# Probe Unhealthy Agent

A minimal agent that listens on its port but reports itself unhealthy. Used to
reproduce that agent probes are TCP-only, have no liveness check, and are not
configurable.

Serves the Agent Manager chat contract: `POST /chat` on port `8000`, plus `GET /health`.

## What it demonstrates

`GET /health` always returns **503**, while `POST /chat` keeps answering. The
`agent-api` ComponentType's readiness probe only checks that port `8000` accepts
a TCP connection, and there is no liveness probe, so:

1. The pod is marked **Ready** (`1/1`) as soon as the port opens.
2. `/chat` keeps receiving traffic even though `/health` says the agent is unhealthy.
3. There is no setting to point the readiness or liveness probe at `/health`.

## Deploy

Create a platform-hosted **Chat Agent** pointing at this directory, with Python 3.11+
and run command `python main.py`, then build and deploy.

## Observe

```bash
kubectl get pods -A | grep probe-unhealthy          # READY 1/1
kubectl exec -n <ns> <pod> -- python -c "import urllib.request as u; u.urlopen('http://localhost:8000/health')"
                                                     # raises HTTP Error 503
```

Then send a message in **Try It**. It is answered, and the runtime logs show
`/chat received traffic while /health reports unhealthy`.
