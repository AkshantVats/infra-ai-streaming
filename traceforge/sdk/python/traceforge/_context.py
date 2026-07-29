# SPDX-License-Identifier: MIT
"""Manual span context for ReAct agents and other non-LLM instrumentation points."""
from __future__ import annotations
import time
from datetime import datetime, timezone
from typing import Any

from ._emit import emit_spans
from ._span import Span, SpanStatus, ToolKind, new_span_id


class ManualSpan:
    """A span you open manually, set attributes on, then close with end().

    Emits to the TraceForge collector on end(). Thread-safe for single-span
    use; not designed for concurrent attribute mutation.
    """

    def __init__(self, name: str, trace_id: str, parent_span_id: str = "") -> None:
        self.span_id = new_span_id()
        self._span = Span(
            trace_id=trace_id,
            span_id=self.span_id,
            parent_span_id=parent_span_id,
            tool_name=name,
            tool_kind=ToolKind.UNKNOWN,
            status=SpanStatus.UNSET,
            start_time=datetime.fromtimestamp(time.time(), tz=timezone.utc).isoformat(),
        )
        self._start_ts = time.time()
        self.attributes: dict[str, Any] = {}

    def set_attribute(self, key: str, value: Any) -> "ManualSpan":
        self.attributes[key] = value
        return self

    def end(self, *, error: str = "") -> None:
        """Finalize and emit the span. Call exactly once."""
        self._span.latency_ms = int((time.time() - self._start_ts) * 1000)
        if error:
            self._span.status = SpanStatus.ERROR
            self._span.error_message = error
        elif self._span.status == SpanStatus.UNSET:
            self._span.status = SpanStatus.OK
        self._span.attributes = {k: str(v) for k, v in self.attributes.items()}
        emit_spans([self._span])


def start_span(name: str, *, trace_id: str, parent_span_id: str = "") -> ManualSpan:
    """Open a manual span. Call span.end() when the work is done.

    Args:
        name: Human-readable span name (e.g. "react.step.7").
        trace_id: Trace this span belongs to.
        parent_span_id: Optional parent for waterfall nesting.

    Returns:
        ManualSpan — call set_attribute() then end().
    """
    return ManualSpan(name=name, trace_id=trace_id, parent_span_id=parent_span_id)
