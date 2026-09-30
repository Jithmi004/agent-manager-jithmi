"""Agent that opens its port immediately but reports itself unhealthy.

``GET /health`` always returns 503, yet ``POST /chat`` keeps answering. The
agent-api ComponentType only checks that the port accepts TCP connections and
has no liveness probe, so the pod is marked Ready and keeps receiving traffic
even though the agent says it should not.

Used as the AM build's run command: ``python main.py``.
"""

from __future__ import annotations

import logging
import uuid
from typing import Any

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("probe-unhealthy")

PORT = 8000


class ChatRequest(BaseModel):
    message: str
    session_id: str | None = None
    context: dict[str, Any] | None = None


class ChatResponse(BaseModel):
    response: str
    session_id: str | None = None


app = FastAPI(title="Probe Unhealthy Agent")


@app.get("/health")
def health() -> JSONResponse:
    # Simulates an agent whose dependency is down (LLM key missing, database
    # unreachable, ...). An httpGet readiness probe on /health would take this
    # pod out of the Service; the platform's TCP probe cannot see it.
    log.warning("/health called: reporting unhealthy (503)")
    return JSONResponse(status_code=503, content={"status": "unhealthy", "reason": "simulated dependency failure"})


@app.post("/chat", response_model=ChatResponse)
def chat(req: ChatRequest) -> ChatResponse:
    log.warning("/chat received traffic while /health reports unhealthy")
    return ChatResponse(
        response="I report myself as unhealthy on /health, but I am still receiving traffic.",
        session_id=req.session_id or str(uuid.uuid4()),
    )


def main() -> None:
    uvicorn.run(app, host="0.0.0.0", port=PORT)


if __name__ == "__main__":
    main()
