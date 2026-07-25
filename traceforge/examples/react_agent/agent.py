# SPDX-License-Identifier: MIT
"""
TraceForge demo: ReAct agent with silent step-7 failure.

Step 7 is a currency conversion call that returns an empty string.
The agent treats empty == "no data" and loops to step 8 with corrupted context.
Without tracing: the agent appears to complete. Final answer is wrong but no error raised.
With TraceForge: span shows result_bytes=0, status=EMPTY_RESPONSE.
"""
from __future__ import annotations
import json
from dataclasses import dataclass, field
from typing import Any

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../sdk/python"))

import traceforge

MAX_STEPS = 10
SILENT_STEP = 7  # this step always produces an empty tool response


@dataclass
class ReActStep:
    step: int
    thought: str
    action: str
    action_input: dict[str, Any]
    observation: str = ""
    span_id: str = ""


@dataclass
class ReActAgent:
    llm: Any  # wrapped OpenAI client (or mock)
    tools: dict[str, Any]
    trace_id: str = ""
    steps: list[ReActStep] = field(default_factory=list)

    def run(self, question: str) -> str:
        self.trace_id = traceforge.new_trace_id()
        context = question

        for step_num in range(1, MAX_STEPS + 1):
            span = traceforge.start_span(
                f"react.step.{step_num}",
                trace_id=self.trace_id,
            )
            span.set_attribute("step", step_num)
            span.set_attribute("agent.type", "react")

            thought, action, action_input = self._reason(context, step_num)
            observation = self._act(action, action_input, step_num)

            span.set_attribute("tool.name", action)
            span.set_attribute("result_bytes", len(observation.encode()))
            if not observation:
                span.set_attribute("status", "EMPTY_RESPONSE")
                span.set_attribute("error", True)
            else:
                span.set_attribute("status", "OK")
            span.end()

            step = ReActStep(
                step=step_num,
                thought=thought,
                action=action,
                action_input=action_input,
                observation=observation,
                span_id=span.span_id,
            )
            self.steps.append(step)
            context = self._build_context(context, step)

            if action == "finish":
                return observation

        return "Max iterations reached without final answer."

    def _reason(self, context: str, step: int) -> tuple[str, str, dict]:
        prompt = _build_prompt(context, step, list(self.tools.keys()))
        response = self.llm.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": prompt}],
        )
        return _parse_response(response.choices[0].message.content)

    def _act(self, action: str, action_input: dict, step: int) -> str:
        if step == SILENT_STEP:
            # Simulate a tool that swallows its exception and returns ""
            return ""
        tool_fn = self.tools.get(action)
        if tool_fn is None:
            return f"Unknown tool: {action}"
        return tool_fn(**action_input)

    def _build_context(self, prev: str, step: ReActStep) -> str:
        return (
            f"{prev}\n"
            f"Step {step.step}:\n"
            f"  Thought: {step.thought}\n"
            f"  Action: {step.action}({json.dumps(step.action_input)})\n"
            f"  Observation: {step.observation or '[empty — silent failure]'}\n"
        )


def _build_prompt(context: str, step: int, tools: list[str]) -> str:
    tool_list = ", ".join(tools)
    return (
        f"You are a ReAct agent. Available tools: {tool_list}.\n"
        f"Current context:\n{context}\n\n"
        f"Step {step}: respond with:\n"
        f"Thought: <reasoning>\n"
        f"Action: <tool name>\n"
        f"Action Input: <JSON object>\n"
        f"If you have a final answer: Action: finish, Action Input: {{\"answer\": \"...\"}}"
    )


def _parse_response(text: str) -> tuple[str, str, dict]:
    thought = action = ""
    action_input: dict = {}
    for line in text.splitlines():
        if line.startswith("Thought:"):
            thought = line[8:].strip()
        elif line.startswith("Action:"):
            action = line[7:].strip().lower()
        elif line.startswith("Action Input:"):
            try:
                action_input = json.loads(line[13:].strip())
            except json.JSONDecodeError:
                action_input = {"raw": line[13:].strip()}
    return thought, action, action_input
