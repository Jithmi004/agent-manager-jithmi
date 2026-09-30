# Probe Unhealthy Agent

A minimal agent that listens on its port but reports itself unhealthy. Used to
show the difference between the default TCP health checks and HTTP checks
against the agent's own health endpoint.

Serves the Agent Manager chat contract: `POST /chat` on port `8000`, plus `GET /health`.

## What it demonstrates

`GET /health` always returns **503**, while `POST /chat` keeps answering. With the
default checks, readiness only tests that port `8000` accepts a TCP connection and
there is no liveness check, so:

1. The pod is marked **Ready** (`1/1`) as soon as the port opens.
2. `/chat` keeps receiving traffic even though `/health` says the agent is unhealthy.

## Using its health endpoint

On the agent's **Deploy** page, open **Health Checks → Configure**:

- Set the readiness check to **HTTP** with path `/health` and save. The pod drops to
  `0/1` and stops receiving traffic.
- Turn the liveness check on with **HTTP** `/health`. The container is restarted
  after the allowed failures (`Liveness probe failed: HTTP probe failed with statuscode: 503`).

The equivalent API call is
`PUT /orgs/{org}/projects/{project}/agents/{agent}/probe-configs?environment={env}`:

```json
{ "readiness": { "type": "http", "path": "/health" } }
```

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
