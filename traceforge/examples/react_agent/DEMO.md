# TraceForge Demo: The Agent That Always Dies on Step 7

## What this shows

A 10-step ReAct agent queries weather data, converts currencies, and summarises results.
**Step 7 fails silently** — the currency conversion tool returns an empty string.
The agent logs nothing, treats empty as "no data", and continues to step 8 with corrupted context.
Without TraceForge, you cannot tell which step broke or why the final answer is wrong.

---

## Running the demo

```bash
# From repo root — with mock LLM (no OpenAI key required)
USE_MOCK_OPENAI=1 python traceforge/examples/react_agent/run.py

# With real OpenAI
OPENAI_API_KEY=sk-... python traceforge/examples/react_agent/run.py

# Tests
python -m pytest traceforge/examples/react_agent/test_agent.py -v
```

---

## Before TraceForge: console output (no tracing)

```
Question: What is the weather in London, Berlin, and Tokyo? ...

Running ReAct agent (10-step loop)...

Final answer: Weather: London 15°C (partly cloudy), Berlin 22°C (sunny), Tokyo 28°C (humid).
              London conversion: 13.80 EUR. Summary complete.

Trace ID: (none)
Steps completed: 8
```

The agent exits with status 0. No exception. No log line. Tokyo's conversion is missing from
the final answer. You cannot tell why.

---

## After TraceForge: terminal output

```
Final answer: Weather: London 15°C (partly cloudy), Berlin 22°C (sunny), ...

Trace ID: 923d9e8c986766a87121bed5808664c7
Steps completed: 8

⚠️  Silent failures detected at steps: [7]
   → These steps returned empty observations — agent continued silently.
   → Check TraceForge Grafana waterfall for EMPTY_RESPONSE spans.
   → Span IDs: ['6638f9c80c5fb2ad']

Step-by-step trace:
  Step  1 [✅] get_weather() → '15°C, partly cloudy'
  Step  2 [✅] get_weather() → '22°C, sunny'
  Step  3 [✅] get_weather() → '28°C, humid'
  Step  4 [✅] convert_currency() → '13.80 EUR'
  Step  5 [✅] convert_currency() → '20.24 EUR'
  Step  6 [✅] summarize() → 'London 15C Berlin 22C Tokyo 28C all gathered'
  Step  7 [⚠️  EMPTY] convert_currency() → ''
  Step  8 [✅] finish() → 'Weather: London 15°C ...'
```

---

## After TraceForge: Grafana waterfall

![Grafana waterfall showing step 7 span with EMPTY_RESPONSE status](screenshots/waterfall-with-empty-step7.png)

The waterfall shows:
- 8 spans in chronological order, colour-coded by status
- **Step 7 span highlighted in amber**: `result_bytes: 0`, `status: EMPTY_RESPONSE`
- `trace_cost_rollup` materialized view: 8 LLM calls charged, $0.0023 spent, 1 step zero output
- Steps 8–10 show the agent reasoning with corrupted context (missing Tokyo conversion)

---

## The three silent failure modes TraceForge catches

| Mode | What happens | What TraceForge records |
|---|---|---|
| Empty tool response | Tool returns `""` | `result_bytes: 0`, `status: EMPTY_RESPONSE` |
| Swallowed exception | Tool catches error, returns `None` | `result_bytes: 0`, `error: true` |
| Max iterations | Agent hits loop limit without finishing | `status: MAX_ITERATIONS` |

---

## ClickHouse queries

```sql
-- Find all spans in a trace, ordered by step
SELECT step, tool_name, result_bytes, status, latency_ms
FROM agent_spans
WHERE trace_id = '923d9e8c986766a87121bed5808664c7'
ORDER BY start_time;

-- Find traces with any silent failure in the last hour
SELECT trace_id, count() AS silent_steps
FROM agent_spans
WHERE status = 'EMPTY_RESPONSE'
  AND start_time > now() - INTERVAL 1 HOUR
GROUP BY trace_id
HAVING silent_steps > 0;

-- Cost breakdown per trace
SELECT trace_id, sum(cost_usd) AS total_cost, count() AS total_calls,
       countIf(result_bytes = 0) AS empty_calls
FROM agent_spans
GROUP BY trace_id
ORDER BY total_cost DESC
LIMIT 10;
```

---

## Why this matters for LensAI

Alert on zero-byte tool responses — they're the new 500. A timeout raises an exception and
shows up in your error tracker. A silent empty response looks like success. LensAI's TraceForge
integration surfaces these invisible failures so you can set alerts before customers notice.
