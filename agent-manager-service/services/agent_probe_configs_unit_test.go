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

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/agent-manager/agent-manager-service/clients/clientmocks"
	"github.com/wso2/agent-manager/agent-manager-service/clients/openchoreosvc/client"
	"github.com/wso2/agent-manager/agent-manager-service/models"
	"github.com/wso2/agent-manager/agent-manager-service/spec"
	"github.com/wso2/agent-manager/agent-manager-service/utils"
)

// probeConfigsTestClient returns an OpenChoreo client mock where the org, project, agent and
// environment all exist. Probe methods are left nil so each test wires only what it expects.
func probeConfigsTestClient() *clientmocks.OpenChoreoClientMock {
	return &clientmocks.OpenChoreoClientMock{
		GetOrganizationFunc: func(_ context.Context, ouID string) (*models.OrganizationResponse, error) {
			return &models.OrganizationResponse{Name: ouID}, nil
		},
		GetProjectFunc: func(_ context.Context, _, name string) (*models.ProjectResponse, error) {
			return &models.ProjectResponse{Name: name}, nil
		},
		GetComponentFunc: func(_ context.Context, _, _, name string) (*models.AgentResponse, error) {
			return &models.AgentResponse{Name: name}, nil
		},
		GetEnvironmentFunc: func(_ context.Context, _, name string) (*models.EnvironmentResponse, error) {
			return &models.EnvironmentResponse{Name: name}, nil
		},
	}
}

func resolvedProbe(enabled bool, probeType string) *client.ProbeConfig {
	path := "/health"
	one, five, six := int32(1), int32(5), int32(6)
	zero := int32(0)
	return &client.ProbeConfig{
		Enabled: &enabled, Type: &probeType, Path: &path,
		InitialDelaySeconds: &zero, PeriodSeconds: &five, TimeoutSeconds: &one, FailureThreshold: &six,
	}
}

func TestUpdateAgentProbeConfigs_PassesOnlyRequestedProbesAndReturnsResolved(t *testing.T) {
	var gotReq client.ComponentProbeConfigs
	oc := probeConfigsTestClient()
	oc.UpdateEnvProbeConfigsFunc = func(_ context.Context, _, _, _, _ string, req client.ComponentProbeConfigs) error {
		gotReq = req
		return nil
	}
	oc.GetEnvProbeConfigsFunc = func(_ context.Context, _, _, _, _ string) (*client.EnvProbeConfigsResponse, error) {
		return &client.EnvProbeConfigsResponse{
			Probes: client.ComponentProbeConfigs{
				Startup:   resolvedProbe(true, client.ProbeTypeTCP),
				Readiness: resolvedProbe(true, client.ProbeTypeHTTP),
				Liveness:  resolvedProbe(false, client.ProbeTypeTCP),
			},
			RedeployRequired: true,
		}, nil
	}
	svc := &agentManagerService{ocClient: oc, logger: discardLogger()}

	httpType := "http"
	resp, err := svc.UpdateAgentProbeConfigs(context.Background(), "ou-1", "proj", "agent-1", "default",
		&spec.UpdateAgentProbeConfigsRequest{Readiness: &spec.AgentProbeConfig{Type: &httpType}})
	require.NoError(t, err)

	assert.Nil(t, gotReq.Startup, "an omitted probe must reach the client as nil, meaning unchanged")
	assert.Nil(t, gotReq.Liveness)
	require.NotNil(t, gotReq.Readiness)
	assert.Equal(t, "http", *gotReq.Readiness.Type)
	assert.Nil(t, gotReq.Readiness.Path, "an omitted field must not be filled in before the merge")

	assert.Equal(t, "http", *resp.Readiness.Type)
	assert.True(t, *resp.Startup.Enabled)
	assert.False(t, *resp.Liveness.Enabled)
	assert.True(t, resp.RedeployRequired)
}

// A rejection from the client (e.g. the agent is not deployed to the environment) is a
// ValidationError that must reach the controller unwrapped, so it becomes a 400 with its
// user-facing message rather than a 500.
func TestUpdateAgentProbeConfigs_KeepsClientValidationError(t *testing.T) {
	notDeployed := utils.NewInvalidInputError("Deploy the agent to this environment before configuring its health checks", "no binding")
	oc := probeConfigsTestClient()
	oc.UpdateEnvProbeConfigsFunc = func(_ context.Context, _, _, _, _ string, _ client.ComponentProbeConfigs) error {
		return notDeployed
	}
	svc := &agentManagerService{ocClient: oc, logger: discardLogger()}

	enabled := true
	_, err := svc.UpdateAgentProbeConfigs(context.Background(), "ou-1", "proj", "agent-1", "sandbox",
		&spec.UpdateAgentProbeConfigsRequest{Liveness: &spec.AgentProbeConfig{Enabled: &enabled}})
	require.Error(t, err)
	assert.Same(t, notDeployed, utils.IsValidationError(err))
	assert.ErrorIs(t, err, utils.ErrInvalidInput)
}

func TestGetAgentProbeConfigs_MissingEnvironmentIsNotFound(t *testing.T) {
	oc := probeConfigsTestClient()
	oc.GetEnvironmentFunc = func(_ context.Context, _, _ string) (*models.EnvironmentResponse, error) {
		return nil, utils.ErrNotFound
	}
	// GetEnvProbeConfigsFunc is nil: reaching it would panic, proving the lookup stops first.
	svc := &agentManagerService{ocClient: oc, logger: discardLogger()}

	_, err := svc.GetAgentProbeConfigs(context.Background(), "ou-1", "proj", "agent-1", "missing")
	assert.ErrorIs(t, err, utils.ErrEnvironmentNotFound)
}

func TestGetAgentProbeConfigs_ClientFailureIsNotMaskedAsNotFound(t *testing.T) {
	oc := probeConfigsTestClient()
	oc.GetEnvProbeConfigsFunc = func(_ context.Context, _, _, _, _ string) (*client.EnvProbeConfigsResponse, error) {
		return nil, errors.New("openchoreo unavailable")
	}
	svc := &agentManagerService{ocClient: oc, logger: discardLogger()}

	_, err := svc.GetAgentProbeConfigs(context.Background(), "ou-1", "proj", "agent-1", "default")
	require.Error(t, err)
	assert.NotErrorIs(t, err, utils.ErrNotFound)
	assert.NotErrorIs(t, err, utils.ErrEnvironmentNotFound)
}
