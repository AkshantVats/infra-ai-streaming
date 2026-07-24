# SPDX-License-Identifier: MIT
from __future__ import annotations
import time
from datetime import datetime, timezone
from typing import Any

from ._emit import emit_spans
from ._span import Span, SpanStatus, ToolKind, hash_arguments, new_span_id, new_trace_id


def _build_tool_span(
    tool_call: Any,
    trace_id: str,
    parent_span_id: str,
    model: str,
    start_ts: float,
    end_ts: float,
    input_tokens: int,
    output_tokens: int,
) -> Span:
    args_raw = tool_call.function.arguments or ""
    return Span(
        trace_id=trace_id,
        span_id=new_span_id(),
        parent_span_id=parent_span_id,
        tool_name=tool_call.function.name,
        tool_kind=ToolKind.UNKNOWN,
        model=model,
        status=SpanStatus.OK,
        start_time=datetime.fromtimestamp(start_ts, tz=timezone.utc).isoformat(),
        latency_ms=int((end_ts - start_ts) * 1000),
        input_tokens=input_tokens,
        output_tokens=output_tokens,
        total_tokens=input_tokens + output_tokens,
        cost_usd=0.0,
        attributes={
            "traceforge.openai.tool_call_id": tool_call.id,
            "traceforge.args.hash": hash_arguments(args_raw),
        },
    )


def _instrument_response(response: Any, trace_id: str, parent_span_id: str, start_ts: float) -> None:
    """Extract tool_calls from a ChatCompletion and emit one Span per call."""
    end_ts = time.time()

    tool_calls = []
    for choice in response.choices:
        if choice.message.tool_calls:
            tool_calls.extend(choice.message.tool_calls)

    if not tool_calls:
        return

    usage = response.usage or type("U", (), {"prompt_tokens": 0, "completion_tokens": 0})()
    input_tokens = getattr(usage, "prompt_tokens", 0) or 0
    output_tokens = getattr(usage, "completion_tokens", 0) or 0
    model = response.model or ""

    n = len(tool_calls)
    per_call_in = input_tokens // n if n > 1 else input_tokens
    per_call_out = output_tokens // n if n > 1 else output_tokens

    spans = [
        _build_tool_span(tc, trace_id, parent_span_id, model, start_ts, end_ts, per_call_in, per_call_out)
        for tc in tool_calls
    ]
    emit_spans(spans)


class _InstrumentedCompletions:
    def __init__(self, inner: Any, trace_id: str | None, parent_span_id: str) -> None:
        self._inner = inner
        self._trace_id = trace_id
        self._parent_span_id = parent_span_id

    def create(self, *args: Any, **kwargs: Any) -> Any:
        trace_id = self._trace_id or new_trace_id()
        start_ts = time.time()
        response = self._inner.create(*args, **kwargs)
        _instrument_response(response, trace_id, self._parent_span_id, start_ts)
        return response

    async def acreate(self, *args: Any, **kwargs: Any) -> Any:
        trace_id = self._trace_id or new_trace_id()
        start_ts = time.time()
        response = await self._inner.acreate(*args, **kwargs)
        _instrument_response(response, trace_id, self._parent_span_id, start_ts)
        return response

    def __getattr__(self, name: str) -> Any:
        return getattr(self._inner, name)


class _InstrumentedChat:
    def __init__(self, inner: Any, trace_id: str | None, parent_span_id: str) -> None:
        self.completions = _InstrumentedCompletions(inner.completions, trace_id, parent_span_id)
        self._inner = inner

    def __getattr__(self, name: str) -> Any:
        return getattr(self._inner, name)


class _InstrumentedClient:
    def __init__(self, inner: Any, trace_id: str | None, parent_span_id: str) -> None:
        self.chat = _InstrumentedChat(inner.chat, trace_id, parent_span_id)
        self._inner = inner

    def __getattr__(self, name: str) -> Any:
        return getattr(self._inner, name)
