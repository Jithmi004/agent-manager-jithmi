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

package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/agent-manager/agent-manager-service/spec"
)

func TestValidateAgentProbeConfigsPayload(t *testing.T) {
	str := func(s string) *string { return &s }
	num := func(n int32) *int32 { return &n }

	tests := []struct {
		name    string
		payload spec.UpdateAgentProbeConfigsRequest
		wantErr string
	}{
		{
			name:    "empty update is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{},
			wantErr: "at least one of",
		},
		{
			name: "http readiness with path is accepted",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Readiness: &spec.AgentProbeConfig{Type: str("http"), Path: str("/health")},
			},
		},
		{
			name: "partial update of one field is accepted",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Startup: &spec.AgentProbeConfig{FailureThreshold: num(120)},
			},
		},
		{
			name: "unknown type is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Liveness: &spec.AgentProbeConfig{Type: str("grpc")},
			},
			wantErr: "liveness probe type",
		},
		{
			name: "path without leading slash is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Readiness: &spec.AgentProbeConfig{Path: str("health")},
			},
			wantErr: "must start with /",
		},
		{
			name: "overlong path is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Readiness: &spec.AgentProbeConfig{Path: str("/" + strings.Repeat("a", maxProbePathLength))},
			},
			wantErr: "at most",
		},
		{
			name: "zero period is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Startup: &spec.AgentProbeConfig{PeriodSeconds: num(0)},
			},
			wantErr: "startup probe periodSeconds",
		},
		{
			name: "zero initial delay is accepted",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Liveness: &spec.AgentProbeConfig{InitialDelaySeconds: num(0)},
			},
		},
		{
			name: "negative initial delay is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Liveness: &spec.AgentProbeConfig{InitialDelaySeconds: num(-1)},
			},
			wantErr: "initialDelaySeconds",
		},
		{
			name: "failure threshold above the cap is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Startup: &spec.AgentProbeConfig{FailureThreshold: num(maxProbeFailureThreshold + 1)},
			},
			wantErr: "failureThreshold",
		},
		{
			name: "timeout above the cap is rejected",
			payload: spec.UpdateAgentProbeConfigsRequest{
				Readiness: &spec.AgentProbeConfig{TimeoutSeconds: num(maxProbeSeconds + 1)},
			},
			wantErr: "timeoutSeconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentProbeConfigsPayload(tt.payload)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
