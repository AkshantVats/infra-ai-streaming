# SPDX-License-Identifier: MIT
"""TraceForge Python SDK — wrap_openai() emits spans per tool call to the TraceForge collector."""

from ._wrap import _InstrumentedClient
from ._span import new_trace_id, new_span_id

__all__ = ["wrap_openai"]


def wrap_openai(client, *, trace_id: str | None = None, parent_span_id: str | None = None):
    """Return an instrumented wrapper around an OpenAI client.

    Args:
        client: An ``openai.OpenAI`` (or ``AsyncOpenAI``) instance.
        trace_id: Optional fixed trace ID. Auto-generated per request if omitted.
        parent_span_id: Optional parent span ID for waterfall nesting.

    Returns:
        Wrapped client with identical interface.
    """
    return _InstrumentedClient(client, trace_id=trace_id, parent_span_id=parent_span_id or new_span_id())
