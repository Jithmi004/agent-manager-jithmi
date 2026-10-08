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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/agent-manager/agent-manager-service/spec"
)

func healthCheckInt32(v int32) *int32 { return &v }

// startupInEffect is a startup check as an environment runs it: the default 10s + 5s x 60 window.
func startupInEffect() *spec.AgentHealthChecks {
	return &spec.AgentHealthChecks{
		Startup: &spec.AgentHealthCheck{
			Enabled:             spec.PtrBool(true),
			InitialDelaySeconds: healthCheckInt32(10),
			PeriodSeconds:       healthCheckInt32(5),
			TimeoutSeconds:      healthCheckInt32(1),
			FailureThreshold:    healthCheckInt32(60),
		},
	}
}

func TestValidateHealthCheckTimings(t *testing.T) {
	tests := []struct {
		name    string
		timings *spec.AgentHealthCheckTimings
		current *spec.AgentHealthChecks
		wantErr string
	}{
		{
			name:    "nothing sent",
			timings: nil,
			current: startupInEffect(),
		},
		{
			name:    "values in range",
			timings: &spec.AgentHealthCheckTimings{Readiness: &spec.AgentProbeTimings{PeriodSeconds: healthCheckInt32(10)}},
			current: startupInEffect(),
		},
		{
			name:    "initial delay may be zero",
			timings: &spec.AgentHealthCheckTimings{Liveness: &spec.AgentProbeTimings{InitialDelaySeconds: healthCheckInt32(0)}},
			current: startupInEffect(),
		},
		{
			name:    "interval below one",
			timings: &spec.AgentHealthCheckTimings{Readiness: &spec.AgentProbeTimings{PeriodSeconds: healthCheckInt32(0)}},
			current: startupInEffect(),
			wantErr: "readiness periodSeconds must be between 1 and 3600",
		},
		{
			name:    "failures allowed above the maximum",
			timings: &spec.AgentHealthCheckTimings{Liveness: &spec.AgentProbeTimings{FailureThreshold: healthCheckInt32(1001)}},
			current: startupInEffect(),
			wantErr: "liveness failureThreshold must be between 1 and 1000",
		},
		{
			// 10s + 5s x 80 = 410s: within the hour once combined with the values in effect.
			name:    "startup window within an hour with the values in effect",
			timings: &spec.AgentHealthCheckTimings{Startup: &spec.AgentProbeTimings{FailureThreshold: healthCheckInt32(80)}},
			current: startupInEffect(),
		},
		{
			// 10s (in effect) + 60s x 80 = 4810s.
			name:    "startup window over an hour once combined with the values in effect",
			timings: &spec.AgentHealthCheckTimings{Startup: &spec.AgentProbeTimings{PeriodSeconds: healthCheckInt32(60), FailureThreshold: healthCheckInt32(80)}},
			current: startupInEffect(),
			wantErr: "startup check would wait up to 4810 seconds",
		},
		{
			name:    "startup window is not checked for a startup check that is off",
			timings: &spec.AgentHealthCheckTimings{Startup: &spec.AgentProbeTimings{PeriodSeconds: healthCheckInt32(60), FailureThreshold: healthCheckInt32(80)}},
			current: &spec.AgentHealthChecks{Startup: &spec.AgentHealthCheck{Enabled: spec.PtrBool(false)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHealthCheckTimings(tt.timings, tt.current)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateHealthChecks(t *testing.T) {
	str := func(v string) *string { return &v }
	tests := []struct {
		name    string
		checks  *spec.AgentHealthChecks
		wantErr string
	}{
		{
			name:   "nothing sent",
			checks: nil,
		},
		{
			name: "HTTP readiness check on a given port",
			checks: &spec.AgentHealthChecks{Readiness: &spec.AgentHealthCheck{
				Enabled: spec.PtrBool(true), Type: str("http"), Path: str("/health"), Port: healthCheckInt32(8000),
			}},
		},
		{
			name:    "unknown check type",
			checks:  &spec.AgentHealthChecks{Readiness: &spec.AgentHealthCheck{Type: str("grpc")}},
			wantErr: "readiness type must be tcp or http",
		},
		{
			name:    "port out of range",
			checks:  &spec.AgentHealthChecks{Liveness: &spec.AgentHealthCheck{Port: healthCheckInt32(70000)}},
			wantErr: "liveness port must be between 1 and 65535",
		},
		{
			name:    "path without a leading slash",
			checks:  &spec.AgentHealthChecks{Readiness: &spec.AgentHealthCheck{Type: str("http"), Path: str("health")}},
			wantErr: "readiness path must start with /",
		},
		{
			name:    "wait time out of range",
			checks:  &spec.AgentHealthChecks{Startup: &spec.AgentHealthCheck{TimeoutSeconds: healthCheckInt32(0)}},
			wantErr: "startup timeoutSeconds must be between 1 and 3600",
		},
		{
			// 10s + 10s x 400 = 4010s.
			name: "startup window over an hour",
			checks: &spec.AgentHealthChecks{Startup: &spec.AgentHealthCheck{
				InitialDelaySeconds: healthCheckInt32(10), PeriodSeconds: healthCheckInt32(10), FailureThreshold: healthCheckInt32(400),
			}},
			wantErr: "startup check would wait up to 4010 seconds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHealthChecks(tt.checks)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
