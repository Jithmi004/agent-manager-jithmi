//
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

package client

import (
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/agent-manager/agent-manager-service/clients/openchoreosvc/gen"
	"github.com/wso2/agent-manager/agent-manager-service/utils"
)

const agentAPIComponentTypePath = "../../../../deployments/helm-charts/wso2-amp-platform-resources-extension/templates/component-types/agent-api.yaml"

// bindingWithProbes returns a booting binding whose componentTypeEnvironmentConfigs carries the
// given probes override.
func bindingWithProbes(ago time.Duration, probes map[string]interface{}) *gen.ReleaseBinding {
	binding := bindingBootingSince(ago)
	envConfigs := map[string]interface{}{"probes": probes}
	binding.Spec.ComponentTypeEnvironmentConfigs = &envConfigs
	return binding
}

// The defaults the API reports must be the ones the ComponentType renders, or the console would
// show one set of probes while pods run another.
func TestDefaultAgentProbesMatchChart(t *testing.T) {
	raw, err := os.ReadFile(agentAPIComponentTypePath)
	require.NoError(t, err)
	// The template's only Helm directive sets metadata.namespace; blank it so the file parses as YAML.
	raw = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAll(raw, []byte("ns"))

	var ct struct {
		Spec struct {
			EnvironmentConfigs struct {
				OpenAPIV3Schema struct {
					Properties struct {
						Probes struct {
							Properties map[string]struct {
								Properties map[string]struct {
									Default interface{} `yaml:"default"`
								} `yaml:"properties"`
							} `yaml:"properties"`
						} `yaml:"probes"`
					} `yaml:"properties"`
				} `yaml:"openAPIV3Schema"`
			} `yaml:"environmentConfigs"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &ct))
	chart := ct.Spec.EnvironmentConfigs.OpenAPIV3Schema.Properties.Probes.Properties
	require.Len(t, chart, 3, "chart should define startup, readiness and liveness probes")

	defaults := defaultAgentProbes()
	for name, probe := range map[string]*ProbeConfig{
		ProbeStartup:   defaults.Startup,
		ProbeReadiness: defaults.Readiness,
		ProbeLiveness:  defaults.Liveness,
	} {
		fields := chart[name].Properties
		require.NotEmpty(t, fields, "chart has no %s probe", name)
		assert.Equal(t, *probe.Enabled, fields["enabled"].Default, "%s.enabled", name)
		assert.Equal(t, *probe.Type, fields["type"].Default, "%s.type", name)
		assert.Equal(t, *probe.Path, fields["path"].Default, "%s.path", name)
		assert.Equal(t, int(*probe.InitialDelaySeconds), fields["initialDelaySeconds"].Default, "%s.initialDelaySeconds", name)
		assert.Equal(t, int(*probe.PeriodSeconds), fields["periodSeconds"].Default, "%s.periodSeconds", name)
		assert.Equal(t, int(*probe.TimeoutSeconds), fields["timeoutSeconds"].Default, "%s.timeoutSeconds", name)
		assert.Equal(t, int(*probe.FailureThreshold), fields["failureThreshold"].Default, "%s.failureThreshold", name)
	}
}

func TestResolveProbeConfigs(t *testing.T) {
	t.Run("no binding reports the defaults", func(t *testing.T) {
		resolved, err := resolveProbeConfigs(nil)
		require.NoError(t, err)
		assert.Equal(t, defaultAgentProbes(), resolved)
	})

	t.Run("override replaces only the fields it sets", func(t *testing.T) {
		binding := bindingWithProbes(0, map[string]interface{}{
			"readiness": map[string]interface{}{"type": "http", "path": "/ready"},
			"liveness":  map[string]interface{}{"enabled": true},
		})
		resolved, err := resolveProbeConfigs(binding)
		require.NoError(t, err)

		defaults := defaultAgentProbes()
		assert.Equal(t, defaults.Startup, resolved.Startup)
		assert.Equal(t, "http", *resolved.Readiness.Type)
		assert.Equal(t, "/ready", *resolved.Readiness.Path)
		assert.Equal(t, *defaults.Readiness.FailureThreshold, *resolved.Readiness.FailureThreshold)
		assert.True(t, *resolved.Liveness.Enabled)
		assert.Equal(t, "tcp", *resolved.Liveness.Type)
	})

	t.Run("malformed override is an error", func(t *testing.T) {
		binding := bindingWithProbes(0, map[string]interface{}{
			"startup": map[string]interface{}{"failureThreshold": "sixty"},
		})
		_, err := resolveProbeConfigs(binding)
		assert.Error(t, err)
	})
}

func TestMergeProbeOverrides(t *testing.T) {
	existing := ComponentProbeConfigs{
		Readiness: &ProbeConfig{Type: ptr("http"), Path: ptr("/ready")},
	}

	t.Run("update keeps stored fields it does not set", func(t *testing.T) {
		merged := mergeProbeOverrides(existing, ComponentProbeConfigs{
			Readiness: &ProbeConfig{FailureThreshold: ptr(int32(3))},
		})
		assert.Equal(t, &ProbeConfig{Type: ptr("http"), Path: ptr("/ready"), FailureThreshold: ptr(int32(3))}, merged.Readiness)
	})

	t.Run("probes the update does not mention are left as stored", func(t *testing.T) {
		merged := mergeProbeOverrides(existing, ComponentProbeConfigs{
			Liveness: &ProbeConfig{Enabled: ptr(true)},
		})
		assert.Nil(t, merged.Startup, "startup should keep following the defaults")
		assert.Equal(t, existing.Readiness, merged.Readiness)
		assert.Equal(t, &ProbeConfig{Enabled: ptr(true)}, merged.Liveness)
	})

	t.Run("stored override is not modified", func(t *testing.T) {
		_ = mergeProbeOverrides(existing, ComponentProbeConfigs{
			Readiness: &ProbeConfig{Path: ptr("/other")},
		})
		assert.Equal(t, "/ready", *existing.Readiness.Path)
	})
}

func TestValidateResolvedProbes(t *testing.T) {
	t.Run("defaults are valid", func(t *testing.T) {
		assert.NoError(t, validateResolvedProbes(defaultAgentProbes()))
	})

	t.Run("http check without a path is rejected", func(t *testing.T) {
		probes := defaultAgentProbes()
		probes.overlay(ComponentProbeConfigs{Liveness: &ProbeConfig{Type: ptr("http"), Path: ptr("")}})
		err := validateResolvedProbes(probes)
		require.Error(t, err)
		assert.NotNil(t, utils.IsValidationError(err))
	})

	t.Run("startup window over the cap is rejected", func(t *testing.T) {
		probes := defaultAgentProbes()
		probes.overlay(ComponentProbeConfigs{Startup: &ProbeConfig{PeriodSeconds: ptr(int32(10)), FailureThreshold: ptr(int32(400))}})
		err := validateResolvedProbes(probes)
		require.Error(t, err)
		assert.NotNil(t, utils.IsValidationError(err))
	})

	t.Run("disabled startup probe has no window to cap", func(t *testing.T) {
		probes := defaultAgentProbes()
		probes.overlay(ComponentProbeConfigs{Startup: &ProbeConfig{Enabled: ptr(false), PeriodSeconds: ptr(int32(10)), FailureThreshold: ptr(int32(400))}})
		assert.NoError(t, validateResolvedProbes(probes))
	})
}

func TestAgentStartupBudgetFor(t *testing.T) {
	t.Run("default probes keep the default budget", func(t *testing.T) {
		assert.Equal(t, agentStartupBudget, agentStartupBudgetFor(readyBinding()))
	})

	t.Run("longer startup window extends the budget", func(t *testing.T) {
		binding := bindingWithProbes(0, map[string]interface{}{
			"startup": map[string]interface{}{"failureThreshold": 200},
		})
		// 10s initial delay + 5s x 200 = 1010s, plus slack for image pull and init containers.
		assert.Equal(t, 1010*time.Second+agentStartupSlack, agentStartupBudgetFor(binding))
	})

	t.Run("shorter or disabled startup probe keeps the default budget", func(t *testing.T) {
		shorter := bindingWithProbes(0, map[string]interface{}{"startup": map[string]interface{}{"failureThreshold": 5}})
		disabled := bindingWithProbes(0, map[string]interface{}{"startup": map[string]interface{}{"enabled": false}})
		assert.Equal(t, agentStartupBudget, agentStartupBudgetFor(shorter))
		assert.Equal(t, agentStartupBudget, agentStartupBudgetFor(disabled))
	})

	// An agent given a long startup window must not be reported failed while kubelet is still
	// waiting for it; this is what a fixed budget got wrong.
	t.Run("slow agent within its own window is still in progress", func(t *testing.T) {
		booting := runtimeReplicaState{found: true, desired: 1, ready: 0}
		binding := bindingWithProbes(agentStartupBudget+time.Minute, map[string]interface{}{
			"startup": map[string]interface{}{"failureThreshold": 200},
		})
		assert.Equal(t, DeploymentStatusInProgress, determineDeploymentStatus(binding, booting))
	})
}

// A release freezes its ComponentType as {kind, name, spec}; the probes schema sits under spec.
func TestComponentTypeHasProbes(t *testing.T) {
	frozen := func(properties map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"kind": "ComponentType",
			"name": "agent-api",
			"spec": map[string]interface{}{
				"environmentConfigs": map[string]interface{}{
					"openAPIV3Schema": map[string]interface{}{"properties": properties},
				},
			},
		}
	}
	assert.True(t, componentTypeHasProbes(frozen(map[string]interface{}{"probes": map[string]interface{}{"type": "object"}})))
	assert.False(t, componentTypeHasProbes(frozen(map[string]interface{}{"replicas": map[string]interface{}{"type": "integer"}})))
	assert.False(t, componentTypeHasProbes(nil))
}
