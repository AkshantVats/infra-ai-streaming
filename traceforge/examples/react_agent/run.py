# SPDX-License-Identifier: MIT
"""Run the TraceForge ReAct demo. Set USE_MOCK_OPENAI=1 for offline mode."""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../sdk/python"))

from tools import TOOLS


def build_llm():
    if os.environ.get("USE_MOCK_OPENAI") == "1":
        from mock_openai import MockOpenAI
        return MockOpenAI()
    import openai
    import traceforge
    client = openai.OpenAI(api_key=os.environ["OPENAI_API_KEY"])
    return traceforge.wrap_openai(client)


def main():
    llm = build_llm()
    from agent import ReActAgent

    agent = ReActAgent(llm=llm, tools=TOOLS)
    question = (
        "What is the weather in London, Berlin, and Tokyo? "
        "Convert each temperature from Celsius to USD equivalent (as a thought exercise). "
        "Summarize all results in one sentence."
    )
    print(f"Question: {question}\n")
    print("Running ReAct agent (10-step loop)...\n")
    answer = agent.run(question)

    print(f"\nFinal answer: {answer}")
    print(f"\nTrace ID: {agent.trace_id}")
    print(f"Steps completed: {len(agent.steps)}")

    silent_steps = [s for s in agent.steps if not s.observation]
    if silent_steps:
        print(f"\n⚠️  Silent failures detected at steps: {[s.step for s in silent_steps]}")
        print("   → These steps returned empty observations — agent continued silently.")
        print("   → Check TraceForge Grafana waterfall for EMPTY_RESPONSE spans.")
        print(f"   → Span IDs: {[s.span_id for s in silent_steps]}")
    else:
        print("\n✅ No silent failures detected.")

    print("\nStep-by-step trace:")
    for step in agent.steps:
        status = "⚠️  EMPTY" if not step.observation else "✅"
        obs_preview = step.observation[:60] + "..." if len(step.observation) > 60 else step.observation
        print(f"  Step {step.step:2d} [{status}] {step.action}() → {obs_preview!r}")


if __name__ == "__main__":
    main()
