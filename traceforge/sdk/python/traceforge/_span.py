# SPDX-License-Identifier: MIT
from __future__ import annotations
import hashlib
import secrets
from dataclasses import dataclass, field
from enum import Enum


class ToolKind(str, Enum):
    MODEL_CALL = "model_call"
    RETRIEVAL = "retrieval"
    CODE_EXEC = "code_execution"
    FILE_IO = "file_io"
    BROWSER = "browser"
    SUB_AGENT = "sub_agent"
    UNKNOWN = "unknown"


class SpanStatus(str, Enum):
    OK = "OK"
    ERROR = "ERROR"
    UNSET = "UNSET"


@dataclass
class Span:
    trace_id: str
    span_id: str
    parent_span_id: str = ""
    tool_name: str = ""
    tool_kind: ToolKind = ToolKind.UNKNOWN
    model: str = ""
    status: SpanStatus = SpanStatus.UNSET
    start_time: str = ""
    latency_ms: int = 0
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    cost_usd: float = 0.0
    error_message: str = ""
    attributes: dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "trace_id": self.trace_id,
            "span_id": self.span_id,
            "parent_span_id": self.parent_span_id,
            "tool_name": self.tool_name,
            "tool_kind": self.tool_kind.value,
            "model": self.model,
            "status": self.status.value,
            "start_time": self.start_time,
            "latency_ms": self.latency_ms,
            "input_tokens": self.input_tokens,
            "output_tokens": self.output_tokens,
            "total_tokens": self.total_tokens,
            "cost_usd": self.cost_usd,
            "error_message": self.error_message,
            "attributes": self.attributes,
        }


def new_trace_id() -> str:
    return secrets.token_hex(16)


def new_span_id() -> str:
    return secrets.token_hex(8)


def hash_arguments(arguments: str) -> str:
    """SHA-256 the tool arguments; return first 16 hex chars as a privacy-safe fingerprint."""
    return hashlib.sha256(arguments.encode()).hexdigest()[:16]
