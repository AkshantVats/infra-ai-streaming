# SPDX-License-Identifier: MIT
"""Tests for ReAct demo agent — TraceForge Day 35."""
import os
import sys
import pytest
from unittest.mock import MagicMock, patch

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../sdk/python"))

from agent import ReActAgent, SILENT_STEP, _parse_response
from tools import TOOLS, get_weather, convert_currency, summarize


# ── helpers ────────────────────────────────────────────────────────────────────

STEP_RESPONSES = [
    'Thought: check weather london\nAction: get_weather\nAction Input: {"city": "london"}',
    'Thought: check weather berlin\nAction: get_weather\nAction Input: {"city": "berlin"}',
    'Thought: check weather tokyo\nAction: get_weather\nAction Input: {"city": "tokyo"}',
    'Thought: convert london\nAction: convert_currency\nAction Input: {"amount": 15.0, "from_currency": "USD", "to_currency": "EUR"}',
    'Thought: convert berlin\nAction: convert_currency\nAction Input: {"amount": 22.0, "from_currency": "USD", "to_currency": "EUR"}',
    'Thought: summarize\nAction: summarize\nAction Input: {"text": "London 15C Berlin 22C Tokyo 28C"}',
    # Step 7 — SILENT_STEP: _act() returns "" regardless of this action
    'Thought: convert tokyo\nAction: convert_currency\nAction Input: {"amount": 28.0, "from_currency": "USD", "to_currency": "GBP"}',
    'Thought: done\nAction: finish\nAction Input: {"answer": "Summary complete"}',
]


def _mock_llm(responses=None):
    responses = responses or STEP_RESPONSES
    calls = iter(responses)

    client = MagicMock()

    def _create(**kwargs):
        resp = MagicMock()
        resp.choices[0].message.content = next(calls, 'Action: finish\nAction Input: {"answer": "done"}')
        resp.usage.prompt_tokens = 200
        resp.usage.completion_tokens = 50
        resp.model = "gpt-4o-mock"
        return resp

    client.chat.completions.create.side_effect = _create
    return client


# ── tests ──────────────────────────────────────────────────────────────────────

def test_step7_returns_empty():
    """Step 7 tool call must return empty string regardless of which tool is called."""
    llm = _mock_llm()
    agent = ReActAgent(llm=llm, tools=TOOLS)
    agent.run("test question")
    assert agent.steps[SILENT_STEP - 1].observation == ""


def test_agent_continues_after_empty():
    """Agent must not raise an exception on step 7 empty response — it continues to step 8."""
    llm = _mock_llm()
    agent = ReActAgent(llm=llm, tools=TOOLS)
    result = agent.run("test question")
    assert result is not None
    assert len(agent.steps) >= SILENT_STEP


def test_span_has_empty_response_attributes():
    """Span for step 7 must carry result_bytes=0 and status=EMPTY_RESPONSE attributes."""
    import traceforge
    captured_spans = []

    original_start = traceforge.start_span

    def capturing_start(name, *, trace_id, parent_span_id=""):
        span = original_start(name, trace_id=trace_id, parent_span_id=parent_span_id)
        captured_spans.append(span)
        return span

    with patch.object(traceforge, "start_span", side_effect=capturing_start):
        llm = _mock_llm()
        agent = ReActAgent(llm=llm, tools=TOOLS)
        agent.run("test")

    step7_span = next(
        (s for s in captured_spans if s.attributes.get("step") == SILENT_STEP),
        None,
    )
    assert step7_span is not None, "No span found for step 7"
    assert step7_span.attributes.get("result_bytes") == 0
    assert step7_span.attributes.get("status") == "EMPTY_RESPONSE"


def test_non_silent_steps_have_observations():
    """All steps except SILENT_STEP must produce non-empty observations (before finish)."""
    llm = _mock_llm()
    agent = ReActAgent(llm=llm, tools=TOOLS)
    agent.run("test question")
    for step in agent.steps:
        if step.step != SILENT_STEP and step.action != "finish":
            assert step.observation, f"Step {step.step} unexpectedly empty"


def test_weather_tool():
    assert "15°C" in get_weather("london")
    assert "22°C" in get_weather("berlin")
    assert "unavailable" in get_weather("mars").lower()


def test_convert_currency_tool():
    result = convert_currency(100.0, "USD", "EUR")
    assert "92.00 EUR" in result
    result_gbp = convert_currency(100.0, "USD", "GBP")
    assert "77.00 GBP" in result_gbp
    result_unknown = convert_currency(100.0, "USD", "JPY")
    assert "No rate" in result_unknown


def test_parse_response_extracts_fields():
    text = 'Thought: check weather\nAction: get_weather\nAction Input: {"city": "london"}'
    thought, action, action_input = _parse_response(text)
    assert thought == "check weather"
    assert action == "get_weather"
    assert action_input == {"city": "london"}
