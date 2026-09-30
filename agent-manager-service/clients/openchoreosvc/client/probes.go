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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/wso2/agent-manager/agent-manager-service/clients/openchoreosvc/gen"
	"github.com/wso2/agent-manager/agent-manager-service/utils"
)

// Probe names, as keyed under componentTypeEnvironmentConfigs.probes.
const (
	ProbeStartup   = "startup"
	ProbeReadiness = "readiness"
	ProbeLiveness  = "liveness"

	envConfigProbesKey = "probes"
)

// Probe check types.
const (
	ProbeTypeTCP  = "tcp"
	ProbeTypeHTTP = "http"
)

// ProbeConfig is one health probe as stored under componentTypeEnvironmentConfigs.probes.<name>.
// Fields are pointers so an override can carry only the fields it changes; the JSON names must
// match the probes schema in component-types/agent-api.yaml.
type ProbeConfig struct {
	Enabled             *bool   `json:"enabled,omitempty"`
	Type                *string `json:"type,omitempty"`
	Path                *string `json:"path,omitempty"`
	InitialDelaySeconds *int32  `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       *int32  `json:"periodSeconds,omitempty"`
	TimeoutSeconds      *int32  `json:"timeoutSeconds,omitempty"`
	FailureThreshold    *int32  `json:"failureThreshold,omitempty"`
}

// ComponentProbeConfigs holds an agent's three health probes. Returned fully resolved by
// GetEnvProbeConfigs; passed as a partial override to UpdateEnvProbeConfigs, where a nil probe
// is left unchanged.
type ComponentProbeConfigs struct {
	Startup   *ProbeConfig `json:"startup,omitempty"`
	Readiness *ProbeConfig `json:"readiness,omitempty"`
	Liveness  *ProbeConfig `json:"liveness,omitempty"`
}

// EnvProbeConfigsResponse is the probes in effect for a component in an environment.
type EnvProbeConfigsResponse struct {
	Probes ComponentProbeConfigs
	// RedeployRequired is true when the release running in the environment was cut from a
	// ComponentType that predates configurable probes. OpenChoreo renders a binding against the
	// ComponentType frozen into its release, which accepts but ignores a probes override, so the
	// overrides only take effect once the agent is deployed again.
	RedeployRequired bool
}

func ptr[T any](v T) *T { return &v }

// defaultAgentProbes returns the probes an agent gets when its environment sets no override.
// They reproduce the fixed probes agents had before probes were configurable, and must match the
// defaults in the probes schema of component-types/agent-api.yaml (TestDefaultAgentProbesMatchChart
// enforces this). A fresh value is returned on every call so callers can modify it.
func defaultAgentProbes() ComponentProbeConfigs {
	probe := func(enabled bool, initialDelay, period, failureThreshold int32) *ProbeConfig {
		return &ProbeConfig{
			Enabled:             ptr(enabled),
			Type:                ptr(ProbeTypeTCP),
			Path:                ptr("/health"),
			InitialDelaySeconds: ptr(initialDelay),
			PeriodSeconds:       ptr(period),
			TimeoutSeconds:      ptr(int32(1)),
			FailureThreshold:    ptr(failureThreshold),
		}
	}
	return ComponentProbeConfigs{
		Startup:   probe(true, 10, 5, 60),
		Readiness: probe(true, 0, 5, 6),
		Liveness:  probe(false, 0, 10, 3),
	}
}

// overlay copies the fields set in src onto dst.
func (dst *ProbeConfig) overlay(src *ProbeConfig) {
	if src == nil {
		return
	}
	if src.Enabled != nil {
		dst.Enabled = src.Enabled
	}
	if src.Type != nil {
		dst.Type = src.Type
	}
	if src.Path != nil {
		dst.Path = src.Path
	}
	if src.InitialDelaySeconds != nil {
		dst.InitialDelaySeconds = src.InitialDelaySeconds
	}
	if src.PeriodSeconds != nil {
		dst.PeriodSeconds = src.PeriodSeconds
	}
	if src.TimeoutSeconds != nil {
		dst.TimeoutSeconds = src.TimeoutSeconds
	}
	if src.FailureThreshold != nil {
		dst.FailureThreshold = src.FailureThreshold
	}
}

// overlay applies every probe set in src onto dst field by field.
func (dst *ComponentProbeConfigs) overlay(src ComponentProbeConfigs) {
	dst.Startup.overlay(src.Startup)
	dst.Readiness.overlay(src.Readiness)
	dst.Liveness.overlay(src.Liveness)
}

// mergeProbeOverrides applies an update to the stored overrides field by field. Only fields set in
// the update change, and only fields that were overridden are stored, so a probe the update does not
// mention keeps following the defaults.
func mergeProbeOverrides(existing, update ComponentProbeConfigs) ComponentProbeConfigs {
	merge := func(stored, incoming *ProbeConfig) *ProbeConfig {
		if incoming == nil {
			return stored
		}
		merged := &ProbeConfig{}
		merged.overlay(stored)
		merged.overlay(incoming)
		return merged
	}
	return ComponentProbeConfigs{
		Startup:   merge(existing.Startup, update.Startup),
		Readiness: merge(existing.Readiness, update.Readiness),
		Liveness:  merge(existing.Liveness, update.Liveness),
	}
}

// probeOverridesFromEnvConfigs reads the probes override from a binding's
// componentTypeEnvironmentConfigs. A missing or empty override yields a zero value.
func probeOverridesFromEnvConfigs(envConfigs *map[string]interface{}) (ComponentProbeConfigs, error) {
	var overrides ComponentProbeConfigs
	if envConfigs == nil {
		return overrides, nil
	}
	raw, ok := (*envConfigs)[envConfigProbesKey]
	if !ok || raw == nil {
		return overrides, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return overrides, fmt.Errorf("failed to read probe overrides: %w", err)
	}
	if err := json.Unmarshal(data, &overrides); err != nil {
		return overrides, fmt.Errorf("failed to parse probe overrides: %w", err)
	}
	return overrides, nil
}

// resolveProbeConfigs returns the probes in effect for a binding: its overrides over the defaults.
func resolveProbeConfigs(binding *gen.ReleaseBinding) (ComponentProbeConfigs, error) {
	resolved := defaultAgentProbes()
	if binding == nil || binding.Spec == nil {
		return resolved, nil
	}
	overrides, err := probeOverridesFromEnvConfigs(binding.Spec.ComponentTypeEnvironmentConfigs)
	if err != nil {
		return resolved, err
	}
	resolved.overlay(overrides)
	return resolved, nil
}

// validateResolvedProbes checks the probes that would be rendered after an update. Field ranges
// are enforced by the API and the ComponentType schema; this covers the rules that span fields or
// depend on the merged result, so a partial update cannot leave an unrenderable combination.
func validateResolvedProbes(probes ComponentProbeConfigs) error {
	for name, probe := range map[string]*ProbeConfig{
		ProbeStartup:   probes.Startup,
		ProbeReadiness: probes.Readiness,
		ProbeLiveness:  probes.Liveness,
	} {
		if *probe.Type == ProbeTypeHTTP && *probe.Path == "" {
			return utils.NewInvalidInputError(
				fmt.Sprintf("The %s check needs a path when it uses HTTP", name),
				fmt.Sprintf("probes.%s.path is required when probes.%s.type is http", name, name))
		}
	}
	window := startupProbeWindow(probes.Startup)
	if window > maxStartupProbeWindow {
		return utils.NewInvalidInputError(
			fmt.Sprintf("The startup check may allow at most %s for the agent to start", maxStartupProbeWindow),
			fmt.Sprintf("initialDelaySeconds + periodSeconds x failureThreshold is %s, above the %s maximum",
				window, maxStartupProbeWindow))
	}
	return nil
}

// maxStartupProbeWindow caps how long a startup probe may keep waiting for an agent. Beyond this a
// pod that will never start holds its replica slot, and the deployment status stays in-progress,
// for longer than anyone would wait on it.
const maxStartupProbeWindow = time.Hour

// startupProbeWindow is how long kubelet keeps retrying the startup probe before it kills the
// container: initialDelaySeconds + periodSeconds x failureThreshold. Zero when the probe is disabled.
func startupProbeWindow(startup *ProbeConfig) time.Duration {
	if startup == nil || startup.Enabled == nil || !*startup.Enabled {
		return 0
	}
	seconds := int64(*startup.InitialDelaySeconds) + int64(*startup.PeriodSeconds)*int64(*startup.FailureThreshold)
	return time.Duration(seconds) * time.Second
}

// GetEnvProbeConfigs returns the health probes in effect for a component in an environment. An
// environment the component was never deployed to has no overrides, so it reports the defaults.
func (c *openChoreoClient) GetEnvProbeConfigs(ctx context.Context, ouID, projectName, componentName, environment string) (*EnvProbeConfigsResponse, error) {
	namespaceName := c.NamespaceFor(ouID)
	binding, err := c.findReleaseBindingForEnv(ctx, namespaceName, componentName, environment)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveProbeConfigs(binding)
	if err != nil {
		return nil, err
	}
	supported, err := c.releaseSupportsProbes(ctx, namespaceName, binding)
	if err != nil {
		return nil, err
	}
	return &EnvProbeConfigsResponse{Probes: resolved, RedeployRequired: !supported}, nil
}

// releaseSupportsProbes reports whether the release a binding runs was cut from a ComponentType
// whose environmentConfigs schema has probes. A binding with no release yet reports true: its
// first release will be cut from the current ComponentType.
func (c *openChoreoClient) releaseSupportsProbes(ctx context.Context, namespaceName string, binding *gen.ReleaseBinding) (bool, error) {
	if binding == nil || binding.Spec == nil || binding.Spec.ReleaseName == nil || *binding.Spec.ReleaseName == "" {
		return true, nil
	}
	resp, err := c.ocClient.GetComponentReleaseWithResponse(ctx, namespaceName, *binding.Spec.ReleaseName)
	if err != nil {
		return false, fmt.Errorf("failed to get component release: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return false, handleErrorResponse(resp.StatusCode(), ErrorResponses{
			JSON401: resp.JSON401,
			JSON403: resp.JSON403,
			JSON404: resp.JSON404,
			JSON500: resp.JSON500,
		})
	}
	if resp.JSON200 == nil || resp.JSON200.Spec == nil {
		return false, fmt.Errorf("empty response from get component release")
	}
	return componentTypeHasProbes(resp.JSON200.Spec.ComponentType), nil
}

// componentTypeHasProbes reports whether a release's frozen ComponentType ({kind, name, spec})
// declares spec.environmentConfigs.openAPIV3Schema.properties.probes.
func componentTypeHasProbes(componentType map[string]interface{}) bool {
	node := interface{}(componentType)
	for _, key := range []string{"spec", "environmentConfigs", "openAPIV3Schema", "properties", envConfigProbesKey} {
		m, ok := node.(map[string]interface{})
		if !ok {
			return false
		}
		if node, ok = m[key]; !ok {
			return false
		}
	}
	return node != nil
}

// UpdateEnvProbeConfigs merges probe overrides into the component's release binding for an
// environment. Only the probes and fields set in req change; the rest of the binding's
// componentTypeEnvironmentConfigs is preserved. The component must already be deployed to the
// environment, since the overrides live on its release binding.
func (c *openChoreoClient) UpdateEnvProbeConfigs(ctx context.Context, ouID, projectName, componentName, environment string, req ComponentProbeConfigs) error {
	namespaceName := c.NamespaceFor(ouID)
	listed, err := c.findReleaseBindingForEnv(ctx, namespaceName, componentName, environment)
	if err != nil {
		return err
	}
	if listed == nil || listed.Metadata.Name == "" {
		return utils.NewInvalidInputError(
			"Deploy the agent to this environment before configuring its health checks",
			fmt.Sprintf("no release binding for component %q in environment %q", componentName, environment))
	}
	bindingName := listed.Metadata.Name

	// Re-read the binding by name rather than writing back the listed copy, so the update starts
	// from its latest spec.
	getResp, err := c.ocClient.GetReleaseBindingWithResponse(ctx, namespaceName, bindingName)
	if err != nil {
		return fmt.Errorf("failed to get release binding: %w", err)
	}
	if getResp.StatusCode() != http.StatusOK {
		return handleErrorResponse(getResp.StatusCode(), ErrorResponses{
			JSON401: getResp.JSON401,
			JSON403: getResp.JSON403,
			JSON404: getResp.JSON404,
			JSON500: getResp.JSON500,
		})
	}
	if getResp.JSON200 == nil || getResp.JSON200.Spec == nil {
		return fmt.Errorf("empty response from get release binding")
	}
	releaseBinding := getResp.JSON200

	if releaseBinding.Spec.ComponentTypeEnvironmentConfigs == nil {
		envConfigs := make(map[string]interface{})
		releaseBinding.Spec.ComponentTypeEnvironmentConfigs = &envConfigs
	}
	overrides, err := probeOverridesFromEnvConfigs(releaseBinding.Spec.ComponentTypeEnvironmentConfigs)
	if err != nil {
		return err
	}
	overrides = mergeProbeOverrides(overrides, req)

	// Validate what will actually render — the merged overrides over the defaults — not just the
	// request, so a change to one field is checked against the fields it left in place.
	resolved := defaultAgentProbes()
	resolved.overlay(overrides)
	if err := validateResolvedProbes(resolved); err != nil {
		return err
	}

	overridesMap, err := structToMap(overrides)
	if err != nil {
		return fmt.Errorf("failed to convert probe overrides to map: %w", err)
	}
	(*releaseBinding.Spec.ComponentTypeEnvironmentConfigs)[envConfigProbesKey] = overridesMap

	releaseBinding.Metadata.Labels = c.withResourceLabels(releaseBinding.Metadata.Labels)
	updateResp, err := c.ocClient.UpdateReleaseBindingWithResponse(ctx, namespaceName, bindingName, *releaseBinding)
	if err != nil {
		return fmt.Errorf("failed to update release binding: %w", err)
	}
	if updateResp.StatusCode() != http.StatusOK {
		return handleErrorResponse(updateResp.StatusCode(), ErrorResponses{
			JSON401: updateResp.JSON401,
			JSON403: updateResp.JSON403,
			JSON404: updateResp.JSON404,
			JSON500: updateResp.JSON500,
		})
	}
	return nil
}
