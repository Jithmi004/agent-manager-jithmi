/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { AgentPathParams } from "./common";

// -----------------------------------------------------------------------------
// Probe config schemas
// -----------------------------------------------------------------------------

/** How a probe checks the agent: port accepts a connection, or GET path returns 2xx/3xx. */
export type AgentProbeType = "tcp" | "http";

export type AgentProbeName = "startup" | "readiness" | "liveness";

export interface AgentProbeConfig {
  enabled?: boolean;
  type?: AgentProbeType;
  path?: string;
  initialDelaySeconds?: number;
  periodSeconds?: number;
  timeoutSeconds?: number;
  failureThreshold?: number;
}

// -----------------------------------------------------------------------------
// Request / Response types
// -----------------------------------------------------------------------------

/** Probes to change. Omitted probes and fields keep their current values. */
export interface UpdateAgentProbeConfigsRequest {
  startup?: AgentProbeConfig;
  readiness?: AgentProbeConfig;
  liveness?: AgentProbeConfig;
}

/** The probes in effect, with every field resolved. */
export interface AgentProbeConfigsResponse {
  startup: AgentProbeConfig;
  readiness: AgentProbeConfig;
  liveness: AgentProbeConfig;
  /** True when saved settings apply only after the agent is deployed again. */
  redeployRequired: boolean;
}

// -----------------------------------------------------------------------------
// Path params and query
// -----------------------------------------------------------------------------

export type GetAgentProbeConfigsPathParams = AgentPathParams;
export type UpdateAgentProbeConfigsPathParams = AgentPathParams;

export interface GetAgentProbeConfigsQuery {
  environment: string;
}

export interface UpdateAgentProbeConfigsQuery {
  environment: string;
}
