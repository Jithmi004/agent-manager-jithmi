# Probe Slow Start Agent

A minimal agent that boots slower than the platform's fixed startup probe allows.
Used to reproduce that agent probes are not configurable.

Serves the Agent Manager chat contract: `POST /chat` on port `8000`, plus `GET /health`.

## What it demonstrates

The `agent-api` ComponentType gives every agent the same TCP startup probe:
`initialDelaySeconds: 10`, `periodSeconds: 5`, `failureThreshold: 60`, so about
**310s** to start listening. This agent sleeps `STARTUP_DELAY_SECONDS` (default
`330`) before binding its port, so:

1. The startup probe fails with `connect: connection refused`.
2. After ~310s Kubernetes kills the container (`failed startup probe`) and restarts it.
3. The restart count keeps climbing and the deployment never becomes active.

There is no setting to give this agent a longer startup budget.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `STARTUP_DELAY_SECONDS` | `330` | Seconds to sleep before binding the port. Set below ~300 to see the agent start normally. |

## Deploy

Create a platform-hosted **Chat Agent** pointing at this directory, with Python 3.11+
and run command `python main.py`, then build and deploy.

## Observe

```bash
kubectl get pods -A | grep probe-slow-start
kubectl get pods -n <ns> -w
kubectl describe pod <pod> -n <ns> | sed -n '/Events:/,$p'
```
