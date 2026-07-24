# SPDX-License-Identifier: MIT
"""Tests for traceforge.wrap_openai()."""
from __future__ import annotations
import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from types import SimpleNamespace
from unittest.mock import MagicMock, patch

import pytest
import traceforge


# ── Fixtures ─────────────────────────────────────────────────────────────────

def _mock_tool_call(id: str, name: str, args: str = '{"q": "test"}') -> SimpleNamespace:
    return SimpleNamespace(
        id=id,
        function=SimpleNamespace(name=name, arguments=args),
    )


def _mock_response(tool_calls, model: str = "gpt-4o", input_tok: int = 100, out_tok: int = 50) -> SimpleNamespace:
    return SimpleNamespace(
        choices=[SimpleNamespace(message=SimpleNamespace(tool_calls=tool_calls))],
        model=model,
        usage=SimpleNamespace(prompt_tokens=input_tok, completion_tokens=out_tok),
    )


class _CollectorHandler(BaseHTTPRequestHandler):
    received: list = []

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)
        _CollectorHandler.received.append(json.loads(body))
        self.send_response(200)
        self.end_headers()

    def log_message(self, *args):
        pass


@pytest.fixture()
def collector():
    _CollectorHandler.received.clear()
    server = HTTPServer(("127.0.0.1", 0), _CollectorHandler)
    port = server.server_address[1]
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    yield f"http://127.0.0.1:{port}/v1/spans"
    server.shutdown()


# ── Tests ─────────────────────────────────────────────────────────────────────

def test_single_tool_call_emits_one_span(collector):
    inner = MagicMock()
    inner.chat.completions.create.return_value = _mock_response(
        [_mock_tool_call("call_1", "get_weather")]
    )

    with patch.dict("os.environ", {"TRACEFORGE_ENDPOINT": collector}):
        client = traceforge.wrap_openai(inner)
        client.chat.completions.create(model="gpt-4o", messages=[], tools=[])

    import time; time.sleep(0.05)
    assert len(_CollectorHandler.received) == 1
    spans = _CollectorHandler.received[0]
    assert len(spans) == 1
    assert spans[0]["tool_name"] == "get_weather"
    assert spans[0]["input_tokens"] == 100
    assert spans[0]["output_tokens"] == 50


def test_parallel_tool_calls_emit_multiple_spans(collector):
    inner = MagicMock()
    inner.chat.completions.create.return_value = _mock_response(
        [
            _mock_tool_call("call_1", "read_file", '{"path": "main.py"}'),
            _mock_tool_call("call_2", "read_file", '{"path": "utils.py"}'),
            _mock_tool_call("call_3", "bash_exec", '{"cmd": "pytest"}'),
        ],
        input_tok=300,
        out_tok=90,
    )

    with patch.dict("os.environ", {"TRACEFORGE_ENDPOINT": collector}):
        client = traceforge.wrap_openai(inner, trace_id="aaaa1111bbbb2222cccc3333dddd4444")
        client.chat.completions.create(model="gpt-4o", messages=[], tools=[])

    import time; time.sleep(0.05)
    assert len(_CollectorHandler.received) == 1
    spans = _CollectorHandler.received[0]
    assert len(spans) == 3

    trace_ids = {s["trace_id"] for s in spans}
    assert trace_ids == {"aaaa1111bbbb2222cccc3333dddd4444"}

    names = {s["tool_name"] for s in spans}
    assert names == {"read_file", "bash_exec"}

    for s in spans:
        assert s["input_tokens"] == 100  # 300 // 3


def test_no_tool_calls_emits_nothing(collector):
    inner = MagicMock()
    inner.chat.completions.create.return_value = _mock_response(tool_calls=None)

    with patch.dict("os.environ", {"TRACEFORGE_ENDPOINT": collector}):
        client = traceforge.wrap_openai(inner)
        client.chat.completions.create(model="gpt-4o", messages=[])

    import time; time.sleep(0.05)
    assert len(_CollectorHandler.received) == 0


def test_unreachable_collector_does_not_raise():
    inner = MagicMock()
    inner.chat.completions.create.return_value = _mock_response(
        [_mock_tool_call("call_1", "search")]
    )

    with patch.dict("os.environ", {"TRACEFORGE_ENDPOINT": "http://127.0.0.1:19999/v1/spans"}):
        client = traceforge.wrap_openai(inner)
        response = client.chat.completions.create(model="gpt-4o", messages=[], tools=[])

    assert response is not None


def test_argument_hashing_is_deterministic():
    from traceforge._span import hash_arguments
    h1 = hash_arguments('{"key": "value"}')
    h2 = hash_arguments('{"key": "value"}')
    assert h1 == h2
    assert len(h1) == 16


def test_argument_hashing_differs_for_different_args():
    from traceforge._span import hash_arguments
    assert hash_arguments('{"a": 1}') != hash_arguments('{"a": 2}')


def test_passthrough_attributes():
    inner = MagicMock()
    inner.models = "models_obj"
    client = traceforge.wrap_openai(inner)
    assert client.models == "models_obj"


def test_latency_recorded(collector):
    import time as _time

    inner = MagicMock()

    def slow_create(*args, **kwargs):
        _time.sleep(0.1)
        return _mock_response([_mock_tool_call("call_1", "slow_tool")])

    inner.chat.completions.create.side_effect = slow_create

    with patch.dict("os.environ", {"TRACEFORGE_ENDPOINT": collector}):
        client = traceforge.wrap_openai(inner)
        client.chat.completions.create(model="gpt-4o", messages=[], tools=[])

    _time.sleep(0.05)
    spans = _CollectorHandler.received[0]
    assert spans[0]["latency_ms"] >= 100
