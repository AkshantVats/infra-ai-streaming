# Email Template

This document defines the exact email the agent sends after each daily run. Fill every placeholder literally — do not add extra content or change the structure.

---

## Subject Line

**Success:**
```
Day {N} ✅ — {ai-blog-title-short}, approve to continue
```

**Error:**
```
Day {N} ❌ — {error-summary-one-line}
```

Subject max 80 characters. Truncate `{ai-blog-title-short}` with `…` if needed to stay within the limit.

---

## Success Body

```
TL;DR: {1-sentence what was accomplished today}

📝 AI Blog: "{ai-blog-title}"
   {first-sentence-of-intro}. {second-sentence-of-intro}.
   → {full-url}

📝 Experience Blog: "{exp-blog-title}"
   {first-sentence-of-intro}. {second-sentence-of-intro}.
   → {full-url}

💻 Code: {repo-name} PR #{pr-number} — {1-line-description}
   → {pr-url}

──────────────────────────────────────
✅ Unlock Day {N+1}: comment "approve" on issue #{issue-number}
   → https://github.com/AkshantVats/infra-ai-streaming/issues/{issue-number}

Tomorrow (Day {N+1}): {1-line preview from plan.json nextDay.description}
──────────────────────────────────────
— auto-agent · {HH:MM} IST · {YYYY-MM-DD}
```

---

## Error Body

```
The Day {N} run encountered an error at phase: {phase-name}

Error: {error-message}

DAILY_PROGRESS.md has been set to phase=error on branch {branch-name}.

Manual steps needed:
1. Check the Claude Code session transcript for details
2. Fix the issue
3. Re-run or skip to Day {N+1} by commenting "approve" on issue #{issue-number}

— auto-agent · {HH:MM} IST · {YYYY-MM-DD}
```

---

## Filling Instructions

| Placeholder | Source |
|---|---|
| `{first-sentence-of-intro}` | Literally the first sentence of the published blog article |
| `{second-sentence-of-intro}` | Literally the second sentence of the published blog article |
| `{1-line-description}` | The first bullet point from the code PR description |
| `{1-line preview from plan.json nextDay.description}` | The `nextDay.description` field from `plan.json` for day N+1 |
| `{HH:MM}` | Current time in IST (UTC+5:30), 24-hour format |
| `{YYYY-MM-DD}` | Current date in IST |

Do not add any other content. Keep the structure exactly as shown above.
