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

import { httpGET, httpPUT, SERVICE_BASE } from "../utils";
import type {
  AgentProbeConfigsResponse,
  GetAgentProbeConfigsPathParams,
  GetAgentProbeConfigsQuery,
  UpdateAgentProbeConfigsPathParams,
  UpdateAgentProbeConfigsQuery,
  UpdateAgentProbeConfigsRequest,
} from "@agent-management-platform/types";

function buildBaseUrl(params: GetAgentProbeConfigsPathParams): string {
  const orgName = params.orgName ?? "default";
  const projName = params.projName ?? "default";
  const agentName = params.agentName;
  if (!agentName) {
    throw new Error("agentName is required");
  }
  return `${SERVICE_BASE}/orgs/${encodeURIComponent(orgName)}/projects/${encodeURIComponent(projName)}/agents/${encodeURIComponent(agentName)}/probe-configs`;
}

async function parseResponse(res: Response): Promise<unknown> {
  let body: unknown;
  try {
    body = await res.json();
  } catch {
    body = await res.text().catch(() => "Failed to parse response");
  }
  if (!res.ok) {
    const err = new Error(typeof body === "string" ? body : "Request failed") as Error & { status?: number; statusText?: string; body?: unknown };
    err.status = res.status;
    err.statusText = res.statusText;
    err.body = body;
    throw err;
  }
  return body;
}

export async function getAgentProbeConfigs(
  params: GetAgentProbeConfigsPathParams,
  query: GetAgentProbeConfigsQuery,
  getToken?: () => Promise<string>
): Promise<AgentProbeConfigsResponse> {
  const token = getToken ? await getToken() : undefined;
  const res = await httpGET(buildBaseUrl(params), {
    token,
    searchParams: { environment: query.environment },
  });
  return (await parseResponse(res)) as AgentProbeConfigsResponse;
}

export async function updateAgentProbeConfigs(
  params: UpdateAgentProbeConfigsPathParams,
  body: UpdateAgentProbeConfigsRequest,
  query: UpdateAgentProbeConfigsQuery,
  getToken?: () => Promise<string>
): Promise<AgentProbeConfigsResponse> {
  const token = getToken ? await getToken() : undefined;
  const res = await httpPUT(buildBaseUrl(params), body, {
    token,
    searchParams: { environment: query.environment },
  });
  return (await parseResponse(res)) as AgentProbeConfigsResponse;
}
