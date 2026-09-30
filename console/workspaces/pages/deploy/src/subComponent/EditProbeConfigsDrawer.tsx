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

import {
  Alert,
  Box,
  Button,
  Collapse,
  Form,
  FormControlLabel,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Activity } from "@wso2/oxygen-ui-icons-react";
import {
  DrawerWrapper,
  DrawerHeader,
  DrawerContent,
  useSnackBar,
} from "@agent-management-platform/views";
import { useUpdateAgentProbeConfigs } from "@agent-management-platform/api-client";
import type {
  AgentProbeConfig,
  AgentProbeConfigsResponse,
  AgentProbeName,
  AgentProbeType,
  UpdateAgentProbeConfigsRequest,
} from "@agent-management-platform/types";
import { useUnsavedChangesGuard } from "@agent-management-platform/shared-component";
import { useCallback, useEffect, useMemo, useState } from "react";

// Bounds match the AgentProbeConfig schema in agent-manager-service.
const MAX_SECONDS = 3600;
const MAX_FAILURE_THRESHOLD = 1000;
const MAX_PATH_LENGTH = 1024;
/** Longest the startup check may wait for the agent: initialDelay + period x failureThreshold. */
const MAX_STARTUP_WINDOW_SECONDS = 3600;

const PROBE_NAMES: AgentProbeName[] = ["startup", "readiness", "liveness"];

interface ProbeText {
  title: string;
  description: string;
  failure: string;
}

const PROBE_TEXT: Record<AgentProbeName, ProbeText> = {
  startup: {
    title: "Startup check",
    description:
      "Waits for the agent to finish starting. The other checks begin only after this one passes.",
    failure: "If it keeps failing, the agent is restarted.",
  },
  readiness: {
    title: "Readiness check",
    description: "Decides whether the agent receives traffic.",
    failure: "While it fails, requests are not sent to the agent. The agent is not restarted.",
  },
  liveness: {
    title: "Liveness check",
    description: "Detects an agent that has stopped responding after it started.",
    failure: "If it keeps failing, the agent is restarted.",
  },
};

interface ProbeFormValues {
  enabled: boolean;
  type: AgentProbeType;
  path: string;
  initialDelaySeconds: number;
  periodSeconds: number;
  timeoutSeconds: number;
  failureThreshold: number;
}

type ProbesFormValues = Record<AgentProbeName, ProbeFormValues>;
type ProbeErrors = Partial<Record<keyof ProbeFormValues, string>>;
type ProbesErrors = Record<AgentProbeName, ProbeErrors>;

// Fallbacks only for a response missing a field; the backend always returns resolved values.
const FALLBACK: ProbesFormValues = {
  startup: { enabled: true, type: "tcp", path: "/health", initialDelaySeconds: 10, periodSeconds: 5, timeoutSeconds: 1, failureThreshold: 60 },
  readiness: { enabled: true, type: "tcp", path: "/health", initialDelaySeconds: 0, periodSeconds: 5, timeoutSeconds: 1, failureThreshold: 6 },
  liveness: { enabled: false, type: "tcp", path: "/health", initialDelaySeconds: 0, periodSeconds: 10, timeoutSeconds: 1, failureThreshold: 3 },
};

function toProbeFormValues(
  config: AgentProbeConfig | undefined,
  fallback: ProbeFormValues,
): ProbeFormValues {
  return {
    enabled: config?.enabled ?? fallback.enabled,
    type: config?.type ?? fallback.type,
    path: config?.path ?? fallback.path,
    initialDelaySeconds: config?.initialDelaySeconds ?? fallback.initialDelaySeconds,
    periodSeconds: config?.periodSeconds ?? fallback.periodSeconds,
    timeoutSeconds: config?.timeoutSeconds ?? fallback.timeoutSeconds,
    failureThreshold: config?.failureThreshold ?? fallback.failureThreshold,
  };
}

function toFormValues(config: AgentProbeConfigsResponse | undefined): ProbesFormValues {
  return {
    startup: toProbeFormValues(config?.startup, FALLBACK.startup),
    readiness: toProbeFormValues(config?.readiness, FALLBACK.readiness),
    liveness: toProbeFormValues(config?.liveness, FALLBACK.liveness),
  };
}

function checkRange(value: number, min: number, max: number): string | undefined {
  if (!Number.isInteger(value)) return "Enter a whole number";
  if (value < min || value > max) return `Must be between ${min} and ${max}`;
  return undefined;
}

function validateProbe(name: AgentProbeName, probe: ProbeFormValues): ProbeErrors {
  // A disabled probe is not rendered, so its values are kept but not checked.
  if (!probe.enabled) return {};
  const errors: ProbeErrors = {
    initialDelaySeconds: checkRange(probe.initialDelaySeconds, 0, MAX_SECONDS),
    periodSeconds: checkRange(probe.periodSeconds, 1, MAX_SECONDS),
    timeoutSeconds: checkRange(probe.timeoutSeconds, 1, MAX_SECONDS),
    failureThreshold: checkRange(probe.failureThreshold, 1, MAX_FAILURE_THRESHOLD),
  };
  if (probe.type === "http") {
    const path = probe.path.trim();
    if (!path) errors.path = "Path is required for an HTTP check";
    else if (!path.startsWith("/")) errors.path = "Path must start with /";
    else if (path.length > MAX_PATH_LENGTH) errors.path = `At most ${MAX_PATH_LENGTH} characters`;
  }
  if (name === "startup" && !errors.initialDelaySeconds && !errors.periodSeconds && !errors.failureThreshold) {
    const window = startupWindowSeconds(probe);
    if (window > MAX_STARTUP_WINDOW_SECONDS) {
      errors.failureThreshold = `Allows ${formatDuration(window)} to start; the maximum is ${formatDuration(MAX_STARTUP_WINDOW_SECONDS)}`;
    }
  }
  return Object.fromEntries(Object.entries(errors).filter(([, v]) => v)) as ProbeErrors;
}

function startupWindowSeconds(probe: ProbeFormValues): number {
  return probe.initialDelaySeconds + probe.periodSeconds * probe.failureThreshold;
}

function formatDuration(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes === 0) return `${seconds}s`;
  return seconds === 0 ? `${minutes} min` : `${minutes} min ${seconds}s`;
}

/** Sends only the probes the user changed, so untouched probes keep following the defaults. */
function toRequestPayload(
  form: ProbesFormValues,
  initial: ProbesFormValues,
): UpdateAgentProbeConfigsRequest {
  const payload: UpdateAgentProbeConfigsRequest = {};
  for (const name of PROBE_NAMES) {
    if (JSON.stringify(form[name]) === JSON.stringify(initial[name])) continue;
    const probe = form[name];
    payload[name] = {
      enabled: probe.enabled,
      type: probe.type,
      path: probe.path.trim(),
      initialDelaySeconds: probe.initialDelaySeconds,
      periodSeconds: probe.periodSeconds,
      timeoutSeconds: probe.timeoutSeconds,
      failureThreshold: probe.failureThreshold,
    };
  }
  return payload;
}

interface NumberFieldProps {
  id: string;
  label: string;
  helper: string;
  value: number;
  error?: string;
  disabled: boolean;
  min: number;
  onChange: (value: number) => void;
}

function NumberField({
  id,
  label,
  helper,
  value,
  error,
  disabled,
  min,
  onChange,
}: NumberFieldProps) {
  return (
    <Form.ElementWrapper label={label} name={id}>
      <TextField
        id={id}
        type="number"
        size="small"
        fullWidth
        disabled={disabled}
        value={Number.isNaN(value) ? "" : value}
        onChange={(e) => onChange(e.target.value === "" ? NaN : Number(e.target.value))}
        error={!!error}
        helperText={error || helper}
        slotProps={{ input: { inputProps: { min } } }}
      />
    </Form.ElementWrapper>
  );
}

interface ProbeSectionProps {
  name: AgentProbeName;
  probe: ProbeFormValues;
  errors: ProbeErrors;
  disabled: boolean;
  onChange: (field: keyof ProbeFormValues, value: ProbeFormValues[keyof ProbeFormValues]) => void;
}

function ProbeSection({ name, probe, errors, disabled, onChange }: ProbeSectionProps) {
  const text = PROBE_TEXT[name];
  return (
    <Form.Section>
      <Form.Header>{text.title}</Form.Header>
      <Form.Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          {text.description} {text.failure}
        </Typography>
        <FormControlLabel
          control={
            <Switch
              checked={probe.enabled}
              onChange={(_, checked) => onChange("enabled", checked)}
              disabled={disabled}
            />
          }
          label={probe.enabled ? "On" : "Off"}
        />
        <Collapse in={probe.enabled}>
          <Form.Stack spacing={2} sx={{ mt: 1 }}>
            <Form.ElementWrapper label="Check" name={`${name}-type`}>
              <Select
                id={`${name}-type`}
                size="small"
                fullWidth
                value={probe.type}
                disabled={disabled}
                onChange={(e) => onChange("type", e.target.value as AgentProbeType)}
              >
                <MenuItem value="tcp">Port accepts a connection (TCP)</MenuItem>
                <MenuItem value="http">URL returns a success status (HTTP GET)</MenuItem>
              </Select>
            </Form.ElementWrapper>
            <Collapse in={probe.type === "http"}>
              <Form.ElementWrapper label="Path" name={`${name}-path`}>
                <TextField
                  id={`${name}-path`}
                  size="small"
                  fullWidth
                  placeholder="/health"
                  disabled={disabled}
                  value={probe.path}
                  onChange={(e) => onChange("path", e.target.value)}
                  error={!!errors.path}
                  helperText={errors.path || "Called on the agent's port. A 2xx or 3xx status passes."}
                  slotProps={{ htmlInput: { maxLength: MAX_PATH_LENGTH } }}
                />
              </Form.ElementWrapper>
            </Collapse>
            <Stack direction="row" spacing={2}>
              <NumberField
                id={`${name}-initialDelaySeconds`}
                label="Initial delay (seconds)"
                helper="Before the first check"
                value={probe.initialDelaySeconds}
                error={errors.initialDelaySeconds}
                disabled={disabled}
                min={0}
                onChange={(v) => onChange("initialDelaySeconds", v)}
              />
              <NumberField
                id={`${name}-periodSeconds`}
                label="Interval (seconds)"
                helper="Between checks"
                value={probe.periodSeconds}
                error={errors.periodSeconds}
                disabled={disabled}
                min={1}
                onChange={(v) => onChange("periodSeconds", v)}
              />
            </Stack>
            <Stack direction="row" spacing={2}>
              <NumberField
                id={`${name}-timeoutSeconds`}
                label="Timeout (seconds)"
                helper="Per check"
                value={probe.timeoutSeconds}
                error={errors.timeoutSeconds}
                disabled={disabled}
                min={1}
                onChange={(v) => onChange("timeoutSeconds", v)}
              />
              <NumberField
                id={`${name}-failureThreshold`}
                label="Failures allowed"
                helper="In a row, before acting"
                value={probe.failureThreshold}
                error={errors.failureThreshold}
                disabled={disabled}
                min={1}
                onChange={(v) => onChange("failureThreshold", v)}
              />
            </Stack>
            {name === "startup" && !errors.failureThreshold && !errors.periodSeconds && !errors.initialDelaySeconds && (
              <Typography variant="caption" color="text.secondary">
                The agent has {formatDuration(startupWindowSeconds(probe))} to start before it
                is restarted.
              </Typography>
            )}
          </Form.Stack>
        </Collapse>
      </Form.Stack>
    </Form.Section>
  );
}

export interface EditProbeConfigsDrawerProps {
  open: boolean;
  onClose: () => void;
  probeConfigs: AgentProbeConfigsResponse | undefined;
  orgName: string;
  projName: string;
  agentName: string;
  environment: string;
}

export function EditProbeConfigsDrawer({
  open,
  onClose,
  probeConfigs,
  orgName,
  projName,
  agentName,
  environment,
}: EditProbeConfigsDrawerProps) {
  const [formData, setFormData] = useState<ProbesFormValues>(() => toFormValues(probeConfigs));
  const [initial, setInitial] = useState<ProbesFormValues>(() => toFormValues(probeConfigs));

  const { mutate: updateConfigs, isPending } = useUpdateAgentProbeConfigs();
  const { pushSnackBar } = useSnackBar();

  useEffect(() => {
    if (open) {
      const seed = toFormValues(probeConfigs);
      setFormData(seed);
      setInitial(seed);
    }
  }, [open, probeConfigs]);

  const errors = useMemo<ProbesErrors>(
    () => ({
      startup: validateProbe("startup", formData.startup),
      readiness: validateProbe("readiness", formData.readiness),
      liveness: validateProbe("liveness", formData.liveness),
    }),
    [formData],
  );
  const isValid = PROBE_NAMES.every((name) => Object.keys(errors[name]).length === 0);
  const isDirty = open && JSON.stringify(formData) !== JSON.stringify(initial);
  // onClose drops a URL param, so the guard would otherwise block the
  // deliberate Cancel and post-save closes too.
  const { allowNavigation } = useUnsavedChangesGuard(isDirty);

  const handleProbeChange = useCallback(
    (name: AgentProbeName) =>
      (field: keyof ProbeFormValues, value: ProbeFormValues[keyof ProbeFormValues]) => {
        setFormData((prev) => ({ ...prev, [name]: { ...prev[name], [field]: value } }));
      },
    [],
  );

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (!isValid || !isDirty) return;
      updateConfigs(
        {
          params: { orgName, projName, agentName },
          body: toRequestPayload(formData, initial),
          query: { environment },
        },
        {
          onSuccess: (response) => {
            if (response.redeployRequired) {
              pushSnackBar({
                message: "Health checks saved. Deploy the agent again to apply them.",
                type: "info",
              });
            }
            allowNavigation(onClose);
          },
          onError: (error) => {
            const body = (error as { body?: { message?: string } })?.body;
            const message = body?.message ?? "Failed to update health checks";
            pushSnackBar({ message, type: "error" });
          },
        },
      );
    },
    [
      isValid,
      isDirty,
      updateConfigs,
      orgName,
      projName,
      agentName,
      formData,
      initial,
      environment,
      pushSnackBar,
      allowNavigation,
      onClose,
    ],
  );

  return (
    <DrawerWrapper open={open} onClose={onClose}>
      <DrawerHeader
        icon={<Activity size={24} />}
        title="Edit Health Checks"
        onClose={onClose}
      />
      <DrawerContent>
        <form onSubmit={handleSubmit}>
          <Form.Stack spacing={3}>
            <Typography variant="body2" color="text.secondary">
              Health checks tell the platform when the agent in this environment has started,
              can take requests, and is still responding. Checks run against the agent&apos;s port.
            </Typography>
            {probeConfigs?.redeployRequired && (
              <Alert severity="warning">
                This agent was deployed before health checks were configurable. Changes are saved
                now and take effect the next time the agent is deployed.
              </Alert>
            )}
            {PROBE_NAMES.map((name) => (
              <ProbeSection
                key={name}
                name={name}
                probe={formData[name]}
                errors={errors[name]}
                disabled={isPending}
                onChange={handleProbeChange(name)}
              />
            ))}
            <Box display="flex" justifyContent="flex-end" gap={1} mt={2}>
              <Button
                variant="outlined"
                onClick={() => allowNavigation(onClose)}
                disabled={isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="contained"
                color="primary"
                disabled={!isValid || !isDirty || isPending}
              >
                {isPending ? "Saving..." : "Save"}
              </Button>
            </Box>
          </Form.Stack>
        </form>
      </DrawerContent>
    </DrawerWrapper>
  );
}
