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

import { useQueryClient } from "@tanstack/react-query";
import { useAuthHooks } from "@agent-management-platform/auth";
import { useApiMutation, useApiQuery } from "./react-query-notifications";
import {
  getAgentProbeConfigs,
  updateAgentProbeConfigs,
} from "../apis/probe-configs";
import type {
  AgentProbeConfigsResponse,
  GetAgentProbeConfigsPathParams,
  GetAgentProbeConfigsQuery,
  UpdateAgentProbeConfigsPathParams,
  UpdateAgentProbeConfigsQuery,
  UpdateAgentProbeConfigsRequest,
} from "@agent-management-platform/types";

const QUERY_KEY = "probe-configs";

export function useGetAgentProbeConfigs(
  params: GetAgentProbeConfigsPathParams,
  query: GetAgentProbeConfigsQuery
) {
  const { getToken } = useAuthHooks();
  return useApiQuery<AgentProbeConfigsResponse>({
    queryKey: [QUERY_KEY, params, query],
    queryFn: () => getAgentProbeConfigs(params, query, getToken),
    enabled:
      !!params.orgName && !!params.projName && !!params.agentName && !!query.environment,
  });
}

export function useUpdateAgentProbeConfigs() {
  const { getToken } = useAuthHooks();
  const queryClient = useQueryClient();
  return useApiMutation<
    AgentProbeConfigsResponse,
    unknown,
    {
      params: UpdateAgentProbeConfigsPathParams;
      body: UpdateAgentProbeConfigsRequest;
      query: UpdateAgentProbeConfigsQuery;
    }
  >({
    action: { verb: "update", target: "agent health checks" },
    mutationFn: ({ params, body, query }) =>
      updateAgentProbeConfigs(params, body, query, getToken),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [QUERY_KEY] });
    },
  });
}
