# SPDX-License-Identifier: MIT
"""Offline mock OpenAI client for CI and local dev without an API key."""
from unittest.mock import MagicMock

MOCK_STEPS = [
    'Thought: get london weather\nAction: get_weather\nAction Input: {"city": "london"}',
    'Thought: get berlin weather\nAction: get_weather\nAction Input: {"city": "berlin"}',
    'Thought: get tokyo weather\nAction: get_weather\nAction Input: {"city": "tokyo"}',
    'Thought: convert london temp usd to eur\nAction: convert_currency\nAction Input: {"amount": 15.0, "from_currency": "USD", "to_currency": "EUR"}',
    'Thought: convert berlin temp\nAction: convert_currency\nAction Input: {"amount": 22.0, "from_currency": "USD", "to_currency": "EUR"}',
    'Thought: summarize results so far\nAction: summarize\nAction Input: {"text": "London 15C Berlin 22C Tokyo 28C all gathered"}',
    # Step 7 — agent attempts Tokyo conversion; _act() will return "" regardless
    'Thought: convert tokyo temp\nAction: convert_currency\nAction Input: {"amount": 28.0, "from_currency": "USD", "to_currency": "GBP"}',
    'Thought: all data collected, produce final answer\nAction: finish\nAction Input: {"answer": "Weather: London 15°C (partly cloudy), Berlin 22°C (sunny), Tokyo 28°C (humid). London conversion: 13.80 EUR. Summary complete."}',
]


class MockOpenAI:
    """Drop-in replacement for openai.OpenAI for offline testing."""

    def __init__(self):
        self._calls = iter(MOCK_STEPS)
        self.chat = _MockChat(self._calls)


class _MockChat:
    def __init__(self, calls):
        self.completions = _MockCompletions(calls)


class _MockCompletions:
    def __init__(self, calls):
        self._calls = calls

    def create(self, **kwargs):
        content = next(self._calls, MOCK_STEPS[-1])
        resp = MagicMock()
        resp.choices[0].message.content = content
        resp.usage.prompt_tokens = 200
        resp.usage.completion_tokens = 50
        resp.model = "gpt-4o-mock"
        return resp
