# CLAUDE.md — Autonomous Daily Agent Instructions

This file is the master instruction set for the autonomous Claude Code on the web scheduled agent
that runs in this repository. Every section is mandatory. Read this file in full before taking any
action each session.

---

## 1. Identity & Purpose

This agent runs on a daily schedule — 8am IST and 1pm IST — and performs the following work
automatically without human intervention between runs:

- Writes two blog posts per day: one in the **AI Learning series** and one in the **Experience
  series**. Both are pushed to the `AkshantVats/Profile` repository as pull requests.
- Creates a day-specific code project in a dedicated day repo (e.g., `AkshantVats/day-{N}-{slug}`)
  with a matching code PR.
- Opens a **review issue** in this repository (`AkshantVats/infra-ai-streaming`) that gates
  advancement to the next day.
- Sends a **status summary email** to akshant3@gmail.com summarising everything done that day.

The agent reads `plan.json` in this repository for the day-by-day content plan, reads
`DAILY_PROGRESS.md` to understand current state, and writes back to `DAILY_PROGRESS.md` to record
phase transitions. It never makes assumptions about which day it is — it always derives the current
day from `DAILY_PROGRESS.md` and verifies against the plan.

---

## 2. Email — SEND not DRAFT

The agent MUST send emails, not save them as drafts. Use the shell script `.agent/gmail_send.sh`
for all outbound email. Never call any `create_draft` MCP tool or API method.

### Usage

```bash
bash -e .agent/gmail_send.sh \
  --to "akshant3@gmail.com" \
  --subject "Day {N} ✅ — {slug}, PR #{n} open, approve to continue" \
  --body "$(cat /tmp/email-body.md)"
```

The `--body` argument accepts Markdown. The script handles conversion and delivery.

### Failure handling

If `gmail_send.sh` exits with a non-zero code:

1. Log the error to `DAILY_PROGRESS.md` under a `## Email Errors` section with a timestamp.
2. Continue with the rest of the run. Do NOT block or abort the session because email failed.
3. On the next run, attempt to resend any logged unsent emails before proceeding with new work.

### Prohibited

- `mcp__95628396-b1d8-4768-b864-63ad2dac492d__create_draft` — never call this.
- Any other draft-saving API. If uncertain whether a tool drafts or sends, check its name; if it
  contains "draft", do not use it.

---

## 3. Blog Writing Standards

### 3.1 Before Writing — Read Existing Posts

Before drafting either blog post, fetch `series-index.json` from the `main` branch of
`AkshantVats/Profile` and identify the two most recent posts for each series. Read both posts in
full. Match their register, vocabulary level, section length, and sentence rhythm exactly. If the
recent posts use "I" heavily, so should the new post. If they avoid bullet points, so should this
one.

### 3.2 Voice & Tone

- Write in first-person throughout: "I hit this wall when...", "Here's what surprised me...",
  "What I didn't expect was..."
- Maximum 3 sentences per paragraph. If a paragraph reaches 4 sentences, split it.
- Include exactly one concrete analogy per major concept. The analogy must be grounded in something
  physical or everyday (not another software concept).
- Every section must end with a "so what" sentence — a single sentence that states why this matters
  to the reader in practice.
- No bullet lists as a substitute for prose. Lists are permitted only for enumerated steps where
  order is meaningful and prose would be genuinely harder to follow.

### 3.3 Diagrams — Mermaid Init Block

Every Mermaid diagram in every blog post MUST begin with this exact init block — no exceptions,
no variations:

```
%%{init: {
  'theme': 'base',
  'themeVariables': {
    'primaryColor': '#1e3a5f',
    'primaryTextColor': '#f0f4f8',
    'primaryBorderColor': '#4a90d9',
    'lineColor': '#4a90d9',
    'secondaryColor': '#0d2137',
    'tertiaryColor': '#0a1a2e',
    'background': 'transparent',
    'nodeBorder': '#4a90d9',
    'clusterBkg': '#0d2137',
    'titleColor': '#f0f4f8',
    'edgeLabelBackground': '#0d2137'
  }
}}%%
```

Additional diagram rules:

- Node labels longer than 4 words MUST use `["label text here"]` (quoted syntax) to enable
  word-wrap. Single-word and short labels may use plain `[label]` syntax.
- No node label may exceed 6 words. If the concept requires more words, shorten the label and
  explain the full term in the surrounding prose.
- Maximum 8 nodes per diagram. If a concept requires more than 8 nodes, split it into two or more
  diagrams, each with a descriptive heading.
- Reference `.agent/diagram-style-guide.md` for the complete specification including edge label
  rules, subgraph usage, and accessibility alt-text requirements.

### 3.4 Cover Images

Cover images are mandatory for every post. Follow `.agent/cover-style-guide.md` exactly.

Base rules that always apply:

- SVG dimensions: exactly 1200x630px. Never use a different size.
- Must include: (1) a gradient background, (2) at least one technical visual element (circuit
  traces, node graph, data-flow lines, etc.), (3) title text positioned in the bottom-left third
  of the canvas only.
- NEVER generate a cover that is text on a plain or solid-colour background with no visual element.

Series-specific gradients and visual elements:

| Series | Background gradient | Technical visual |
|---|---|---|
| AI Learning | `#0d2137` → `#1e3a5f` (dark navy to steel blue) | Circuit traces or neural node pattern |
| Experience | `#1a0d2e` → `#3a1f5f` (dark purple to mid purple) | Data-flow arrows or streaming pipeline |

After generating the SVG, verify it by checking:

1. The `viewBox` attribute matches `0 0 1200 630`.
2. A `<linearGradient>` or `<radialGradient>` element is present.
3. At least one `<path>`, `<circle>`, `<line>`, or `<polyline>` element exists that is not part of
   the text.
4. All `<text>` elements have an x-coordinate less than 400 (left third).

---

## 4. Pre-Push Validation (MANDATORY)

Before pushing any blog branch to `AkshantVats/Profile`, run:

```bash
bash -e .agent/pre-push-check.sh <path-to-blog-markdown-file>
```

This script checks:

- All internal links resolve to existing files in the repository.
- All external URLs return HTTP 200 (not 3xx or 4xx).
- The Mermaid init block is present in every code fence tagged `mermaid`.
- The cover SVG passes the four verification checks listed in section 3.4.
- Front matter contains required fields: `title`, `date`, `series`, `coverImage`.
- Word count is between 600 and 2500 words.

### On failure

1. Read the script's output to identify each failing check.
2. Fix every issue identified. Do not skip any.
3. Re-run `pre-push-check.sh`. Repeat until the script exits 0.
4. Only then push the branch.

### On unfixable failures

If a specific failure cannot be automated (e.g., an external URL is dead and no archive exists),
log the issue clearly in a `## Pre-Push Issues` section of `DAILY_PROGRESS.md`, then push the
branch with the PR title prefixed `[MANUAL-FIX-NEEDED]`. Do not silently omit the failure.

---

## 5. PR Description Format

Every blog pull request to `AkshantVats/Profile` MUST use this exact description structure. Fill
in all placeholder values — do not leave any bracket placeholder unfilled:

```markdown
## Blog Preview

**Hook:** [first sentence of the article, verbatim]
**Word count:** ~[N] words
**Diagrams:** [count] — [comma-separated diagram names/descriptions]
**Code snippets:** [count]

## Cover
![cover](path/to/cover.svg)

## Self-review checklist
- [ ] All links verified locally (pre-push-check.sh passed)
- [ ] Diagrams use standard Mermaid init block
- [ ] Cover follows style guide (gradient + visual element + text)
- [ ] Tone: first-person, ≤3 sentences/paragraph
- [ ] series-index.json updated
```

The `path/to/cover.svg` must be the actual relative path from the repo root. The self-review
checklist items are checked by the agent itself during the self-review pass (section 8) before
staging the commit — they are not aspirational, they must actually be verified.

---

## 6. Morning Briefing Email Format

Send this email after both blog PRs and the code PR are open and the review issue is created.
Use `.agent/gmail_send.sh` as specified in section 2.

**Subject line:**

```
Day {N} ✅ — {blog-title-slug}, PR #{n} open, approve to continue
```

Where `{blog-title-slug}` is the slug of the AI Learning post and `{n}` is the review issue number.

**Body (plain text, skimmable in 30 seconds):**

```
TL;DR: [1-sentence summary of what was done today]

📝 AI Blog: "{title}"
   {first 2 sentences of intro, verbatim from the post}
   → {full URL to the PR or published post}

📝 Experience Blog: "{title}"
   {first 2 sentences of intro, verbatim from the post}
   → {full URL to the PR or published post}

💻 Code: {repo-name} PR #{n} — {1-line description of what the code does}

✅ To unlock Day {N+1}: Comment "approve" on issue #{n}
   → https://github.com/AkshantVats/infra-ai-streaming/issues/{n}

Tomorrow (Day {N+1}): {1-line preview pulled from plan.json for the next day}

— auto-agent · {time} IST · {date}
```

All URLs must be full absolute URLs. Do not use relative paths or placeholder URLs.
The time must reflect the actual wall-clock time in IST at the moment the email is sent.

---

## 7. CI Failure Auto-Fix (1pm Run)

The 1pm continuation run must first check whether any blog PRs from the 8am run have a failing
`blog-links` CI check before doing any new work.

### Detection

For each open blog PR created today:

1. Fetch the PR's check runs via the GitHub API.
2. If `blog-links` is in a `failure` or `action_required` state, enter the repair flow.

### Repair flow

1. Checkout the PR's branch locally.
2. Run `bash -e .agent/pre-push-check.sh <blog-markdown-file>` to get the list of broken links.
3. For each broken link:
   - Check the Wayback Machine for an archived version (`https://web.archive.org/web/*/{url}`).
   - If an archive exists, replace the URL with the archived URL.
   - If no archive exists, remove the link and replace the anchor text with plain text.
4. Commit with the exact message: `fix(blog): repair broken links detected by CI`
5. Push to the branch.
6. Poll the `blog-links` check status every 60 seconds for up to 5 minutes.

### After polling

- If CI passes: log success in `DAILY_PROGRESS.md` and continue with the normal 1pm tasks.
- If CI still fails after 5 minutes: add a comment to the review issue in
  `AkshantVats/infra-ai-streaming` stating which links remain broken and why they could not be
  fixed automatically. Then continue with the normal 1pm tasks — do not block on this.

---

## 8. Self-Review Pass (MANDATORY Before Any Commit)

After completing a blog draft and before staging any files with `git add`, re-read the entire post
as a senior engineer who is unfamiliar with this specific topic. Check every point below and fix
any issue found before proceeding:

- **Unexplained jargon:** Any term that requires domain-specific knowledge must have a 1-sentence
  definition on first use. If a term was defined in a previous post in the series, a brief reminder
  phrase is acceptable ("...backpressure — the mechanism that slows producers when consumers fall
  behind...").
- **Paragraph length:** Any paragraph with more than 3 sentences must be split. No exceptions.
- **Diagram label length:** Any node label exceeding 6 words must be shortened. Rewrite the label
  and add clarifying prose in the surrounding paragraph if needed.
- **Markdown integrity:** Scan for unclosed backtick fences, mismatched brackets in links,
  image paths that do not exist in the repository, and headings that skip levels (e.g., H2 to H4).
  Fix every instance found.
- **Placeholder links:** Any URL containing `example.com`, `TODO`, `FIXME`, `placeholder`, or
  `your-url-here` must be replaced with a real URL or removed entirely.

After passing all checks, record a one-line self-review summary in the commit message body:
`Self-review: N issues found and fixed.` where N is the actual count (0 is valid).

---

## 9. Plan Advancement Gate

The agent advances `DAILY_PROGRESS.md` to the next day ONLY when all three conditions are
simultaneously true:

1. **Approval gate:** An "approve" comment (case-insensitive, any capitalisation) exists on the
   current day's review issue in `AkshantVats/infra-ai-streaming`.
2. **CI gate:** The current day's code PR has all required CI checks in a `success` state. No
   check may be in `pending`, `failure`, or `action_required`.
3. **Blog merge gate:** Both blog PRs for the current day are in `merged` state (not just
   `closed`).

### If any gate is not met

1. Do not modify `DAILY_PROGRESS.md`.
2. Do not create any new branches, PRs, issues, or commits.
3. Send (not draft) a reminder email using `.agent/gmail_send.sh` with subject:
   `Day {N} ⏳ — waiting on: {comma-separated list of unmet gates}`
4. Exit cleanly. The next scheduled run will re-check the gates.

### Gate check order

Check gates in this order: approval → CI → blog merge. Report all unmet gates in the reminder
email simultaneously — do not send one email per gate.

---

## 10. Error Handling

All scripts in `.agent/` must be invoked with `bash -e` so that any failing command causes the
script to exit immediately rather than continuing in a broken state.

### On any unrecoverable error

An error is unrecoverable if:

- A required file (e.g., `plan.json`, `DAILY_PROGRESS.md`, `series-index.json`) cannot be read
  after one retry.
- A GitHub API call fails with a 4xx or 5xx response after two retries with 10-second backoff.
- A script in `.agent/` exits non-zero and the agent cannot determine a corrective action.

When an unrecoverable error occurs:

1. Send an error email using `.agent/gmail_send.sh` with subject:
   `Day {N} ❌ — {one-line error summary}`
   and body containing: the full error message, the phase the agent was in when the error occurred,
   and the last action taken.
2. Write to `DAILY_PROGRESS.md`, setting the `phase` field to `error` and adding an
   `error_detail` field with the same one-line summary.
3. Exit immediately. Do not attempt to continue with other tasks in the same session.
4. Do not make any further state changes after recording the error.

### Prohibited behaviours

- Silently catching exceptions and continuing as if nothing happened.
- Retrying indefinitely in a tight loop. Maximum 2 retries with exponential backoff for transient
  errors (network timeouts, rate limits), then fail.
- Writing partial state to `DAILY_PROGRESS.md` without setting the phase to `error` when the
  session is aborting.
- Logging errors only to stdout without persisting them to `DAILY_PROGRESS.md`.
