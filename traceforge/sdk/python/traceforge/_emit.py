# SPDX-License-Identifier: MIT
from __future__ import annotations
import json
import logging
import os
import urllib.request
from typing import Sequence

from ._span import Span

logger = logging.getLogger(__name__)

_DEFAULT_ENDPOINT = "http://localhost:8080/v1/spans"


def emit_spans(spans: Sequence[Span], endpoint: str | None = None) -> None:
    """POST spans to the TraceForge collector. Fire-and-forget; swallows errors."""
    url = endpoint or os.getenv("TRACEFORGE_ENDPOINT", _DEFAULT_ENDPOINT)
    payload = json.dumps([s.to_dict() for s in spans]).encode()
    req = urllib.request.Request(
        url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=2) as resp:
            if resp.status != 200:
                logger.warning("traceforge: collector returned %d", resp.status)
    except Exception as exc:
        logger.debug("traceforge: emit failed (%s) — continuing", exc)
