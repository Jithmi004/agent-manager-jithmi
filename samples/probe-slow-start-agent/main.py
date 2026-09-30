"""Agent that takes longer to boot than the platform's fixed startup probe allows.

Sleeps for ``STARTUP_DELAY_SECONDS`` before binding its port. The agent-api
ComponentType gives a TCP startup probe ~310s (initialDelaySeconds 10 +
periodSeconds 5 x failureThreshold 60), so the default delay of 330s makes the
pod fail its startup probe, get restarted, and never become Ready.

Used as the AM build's run command: ``python main.py``.
"""

from __future__ import annotations

import logging
import os
import time
import uuid
from typing import Any

import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("probe-slow-start")

STARTUP_DELAY_SECONDS = int(os.environ.get("STARTUP_DELAY_SECONDS", "330"))
PORT = 8000


class ChatRequest(BaseModel):
    message: str
    session_id: str | None = None
    context: dict[str, Any] | None = None


class ChatResponse(BaseModel):
    response: str
    session_id: str | None = None


app = FastAPI(title="Probe Slow Start Agent")


@app.get("/health")
def health() -> dict[str, Any]:
    return {"status": "ok", "startup_delay_seconds": STARTUP_DELAY_SECONDS}


@app.post("/chat", response_model=ChatResponse)
def chat(req: ChatRequest) -> ChatResponse:
    return ChatResponse(
        response=f"Booted after {STARTUP_DELAY_SECONDS}s. You said: {req.message}",
        session_id=req.session_id or str(uuid.uuid4()),
    )


def main() -> None:
    # Simulates a slow boot (loading a model, warming a cache, ...). Nothing is
    # listening on PORT until this finishes, so the TCP startup probe fails.
    log.info("Simulating slow startup: sleeping %ds before binding :%d", STARTUP_DELAY_SECONDS, PORT)
    for elapsed in range(0, STARTUP_DELAY_SECONDS, 30):
        log.info("still starting... %ds elapsed", elapsed)
        time.sleep(min(30, STARTUP_DELAY_SECONDS - elapsed))
    log.info("Startup complete, binding :%d", PORT)
    uvicorn.run(app, host="0.0.0.0", port=PORT)


if __name__ == "__main__":
    main()
