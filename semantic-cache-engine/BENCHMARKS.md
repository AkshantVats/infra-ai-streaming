# semantic-cache-engine — Benchmarks

**Status:** A single offline threshold sweep, run against a small hand-labeled held-out
prompt-pair set. This is not a load-test benchmark (no live traffic, no pgvector instance in
this sandbox) — it validates DESIGN.md §8's shipped `0.92` similarity threshold against
real, reproducible precision/recall/false-positive numbers instead of a single point estimate,
using the method described below. **Read the "Why these numbers aren't the production
signal" section before treating any number here as a claim about `pkg/embedder`'s real
behavior.**

Run on: this build's sandbox environment, `go1.25.0`, 2026-08-04.

---

## Why these numbers aren't the production signal

DESIGN.md §3's `0.88`–`0.96` threshold range is calibrated for **cosine similarity between
real `text-embedding-3-small` vectors** (`pkg/embedder.OpenAIEmbedder`). Producing that would
mean calling the real OpenAI embeddings API for the held-out set below. A live probe against
`api.openai.com/v1/embeddings` in this sandbox returned:

```
HTTP 429
{"error":{"message":"You exceeded your current quota, please check your plan and billing
details.","type":"insufficient_quota","code":"insufficient_quota"}}
```

— the same `OPENAI_API_KEY` billing-quota constraint Day 62 hit for DALL-E cover generation.
`cmd/threshold-sweep` therefore uses `pkg/localsim.TokenCosineSimilarity` — a local,
dependency-free bag-of-words term-frequency cosine similarity — as a stand-in. **This measures
lexical overlap, not semantic intent**, and the two are not the same signal: a paraphrase that
shares few words scores low even though a real embedding model would place it close in vector
space, and a lexically-similar-but-intent-different pair scores high even though a real
embedding model is trained to separate them. Every number below should be read as "what a
crude word-overlap proxy says," not "what the real cache would do in production."

## Held-out prompt-pair set

`testdata/threshold-sweep-pairs.jsonl` — 24 hand-labeled pairs, 12 true near-duplicates
(same intent, different wording — e.g. "Summarize this document for me" / "Give me a summary
of this document") and 12 true distinct pairs, half of which are deliberately confusable
(share almost every word except the one that changes intent — e.g. **"Delete my account" /
"How do I delete my account?"**, DESIGN.md §3's own worked example of a dangerous false-hit
risk). Not a random sample of real traffic — a small adversarial set built to stress the
lexical-overlap-vs-intent distinction §3 warns about.

Run: `go run ./cmd/threshold-sweep --input testdata/threshold-sweep-pairs.jsonl`

## Sweep 1 — DESIGN.md-calibrated range (0.88–0.96)

| Threshold | Precision | Recall | False Positive Rate | TP | FP | FN | TN |
|---|---|---|---|---|---|---|---|
| 0.88 | n/a | 0.000 | 0.000 | 0 | 0 | 12 | 12 |
| 0.90 | n/a | 0.000 | 0.000 | 0 | 0 | 12 | 12 |
| 0.92 | n/a | 0.000 | 0.000 | 0 | 0 | 12 | 12 |
| 0.94 | n/a | 0.000 | 0.000 | 0 | 0 | 12 | 12 |
| 0.96 | n/a | 0.000 | 0.000 | 0 | 0 | 12 | 12 |

**Interpretation: this range produces zero hits at any threshold, on either class.** The
proxy's own similarity scores for this set top out at 0.857 (see the full score list below) and
most true paraphrases score well under 0.88 because they share few literal words
("Convert 100 USD to EUR" / "What is 100 US dollars in euros?" scores 0.169). This is itself a
real, useful finding: it demonstrates concretely that lexical cosine similarity and semantic
embedding cosine similarity are **not the same scale**, so §3's `0.88`–`0.96` range cannot be
sanity-checked against this proxy directly — a threshold tuned against one would be meaningless
applied to the other. It is not evidence about whether `0.92` is well-chosen for the real
embedder.

## Sweep 2 — supplementary sweep at the proxy's own operating range

Run: `go run ./cmd/threshold-sweep --input testdata/threshold-sweep-pairs.jsonl --thresholds 0.55,0.65,0.70,0.75,0.80`

| Threshold | Precision | Recall | False Positive Rate | TP | FP | FN | TN |
|---|---|---|---|---|---|---|---|
| 0.55 | 0.333 | 0.500 | 1.000 | 6 | 12 | 6 | 0 |
| 0.65 | 0.294 | 0.417 | 1.000 | 5 | 12 | 7 | 0 |
| 0.70 | 0.214 | 0.250 | 0.917 | 3 | 11 | 9 | 1 |
| 0.75 | 0.222 | 0.167 | 0.583 | 2 | 7 | 10 | 5 |
| 0.80 | 0.286 | 0.167 | 0.417 | 2 | 5 | 10 | 7 |

**Interpretation: precision never clears ~33% anywhere in this range, and false-positive rate
is catastrophic (up to 100%) at the low end.** This is the adversarial construction of the
held-out set working as intended — the 6 deliberately-confusable distinct pairs ("Delete my
account" / "How do I delete my account?" scores **0.707**, higher than 7 of the 12 true
duplicate pairs) share almost every token with their paired prompt, so a proxy that only counts
shared words cannot tell them apart from real duplicates. **This is DESIGN.md §4's false-hit
correctness risk made concrete and measurable**, even on a proxy: word overlap alone is not a
safe signal for "same intent," which is exactly why the real system embeds prompts into a
learned semantic space rather than comparing tokens. It is also why this benchmark cannot
recommend a specific `0.92`-equivalent number on this scale — the proxy's failure mode here is
structural, not a threshold-tuning problem.

## Full similarity scores

<details>
<summary>All 24 pairs, `pkg/localsim.TokenCosineSimilarity` score and label</summary>

| Similarity | Label | Prompt A | Prompt B |
|---|---|---|---|
| 0.833 | duplicate | Write a haiku about autumn leaves | Compose a haiku about autumn leaves |
| 0.818 | duplicate | Draft a polite follow-up email to a client | Write a polite follow-up email for a client |
| 0.730 | duplicate | Translate this paragraph to French | Please translate this paragraph into French |
| 0.668 | duplicate | What's the weather like in Bangkok today? | Tell me today's weather in Bangkok |
| 0.668 | duplicate | List the top 5 hotels in Phuket | Give me the top five hotels in Phuket |
| 0.571 | duplicate | Give me a recipe for banana bread | Can you share a banana bread recipe? |
| 0.535 | duplicate | Explain how a Kafka consumer group works | Can you explain how Kafka consumer groups work? |
| 0.507 | duplicate | Summarize this document for me | Give me a summary of this document |
| 0.500 | duplicate | What are the cancellation terms for this booking? | Tell me the cancellation policy for this reservation |
| 0.433 | duplicate | How do I reset my password? | What are the steps to reset my password? |
| 0.169 | duplicate | Can you summarize this doc? | Please provide a summary of this document |
| 0.169 | duplicate | Convert 100 USD to EUR | What is 100 US dollars in euros? |
| 0.857 | distinct | What's the cheapest flight to Tokyo? | What's the fastest flight to Tokyo? |
| 0.857 | distinct | List the top 5 hotels in Phuket | List the top 5 restaurants in Phuket |
| 0.857 | distinct | Give me a recipe for banana bread | Give me a recipe for banana smoothie |
| 0.833 | distinct | Summarize chapter one of this book | Summarize chapter two of this book |
| 0.825 | distinct | What's the weather like in Bangkok today? | What's the weather like in Bangkok next week? |
| 0.800 | distinct | Translate this paragraph to French | Translate this paragraph to Spanish |
| 0.772 | distinct | Explain how a Kafka consumer group works | Explain how a Kafka producer works |
| 0.722 | distinct | Increase my account's rate limit | What is my account's current rate limit? |
| 0.722 | distinct | How do I reset my password? | How do I reset my two-factor authentication? |
| 0.707 | distinct | Delete my account | How do I delete my account? |
| 0.707 | distinct | Cancel my subscription | How do I cancel my subscription? |
| 0.667 | distinct | Write a haiku about autumn leaves | Write a haiku about ocean waves |

</details>

## What this does and doesn't validate

| Question | Does this benchmark answer it? |
|---|---|
| Is `0.92` a reasonable default on the real embedding scale? | **No.** Needs a live `OPENAI_API_KEY` with quota, or a local sentence-transformer, run against this held-out set. Tracked as follow-on scope alongside Days 61–62's logged pgvector integration-test gap. |
| Is word-overlap similarity a safe stand-in for semantic similarity in this domain? | **Yes, answered concretely: no.** Confusable-but-distinct pairs outscore true paraphrases on this proxy, matching DESIGN.md §4's stated false-hit risk. |
| Does the threshold-sweep tooling itself work end to end (parsing, classification, precision/recall/FPR math)? | **Yes** — unit-tested (`cmd/threshold-sweep`, `pkg/localsim`) and exercised against this real fixture, not fabricated numbers. |

## Reproduce

```bash
cd semantic-cache-engine
go run ./cmd/threshold-sweep --input testdata/threshold-sweep-pairs.jsonl
go run ./cmd/threshold-sweep --input testdata/threshold-sweep-pairs.jsonl --thresholds 0.55,0.65,0.70,0.75,0.80
```
