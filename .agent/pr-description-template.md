# PR Description Template

Use this template verbatim for every blog PR. Fill all placeholders before opening the PR.

---

## Template

```markdown
## Blog Preview

**Hook:** {first-sentence-of-article}
**Word count:** ~{word-count} words
**Diagrams:** {diagram-count} — {diagram-names-comma-separated}
**Code snippets:** {code-block-count}
**Series:** {AI Learning Day N | Experience N}

## Cover

![cover]({relative-path-to-cover-svg})

## Changes
- `{blog-filename}` — new blog post
- `{cover-filename}` — cover image
- `series-index.json` — new entry added

## Self-review checklist
- [ ] All links verified locally (`pre-push-check.sh` passed with exit 0)
- [ ] Every Mermaid diagram uses the standard init block
- [ ] Cover: gradient background + visual element + text (not text-only)
- [ ] Tone: first-person, ≤3 sentences per paragraph, one analogy per concept
- [ ] `series-index.json` updated with correct metadata
- [ ] No placeholder links (example.com, TODO, localhost)
```

---

## Filling Instructions

| Placeholder | How to fill |
|---|---|
| `{first-sentence-of-article}` | Literally the first sentence of the blog post body |
| `{word-count}` | Run `wc -w < blog-file` and round to the nearest 50 |
| `{diagram-count}` | Count of fenced `mermaid` code blocks in the file |
| `{diagram-names-comma-separated}` | Extract the first comment or title line inside each mermaid block (e.g. `%% Flow diagram`), comma-separated |
| `{code-block-count}` | Count of all fenced code blocks (including mermaid) |
| `{relative-path-to-cover-svg}` | Path relative to the repo root, e.g. `posts/day-42/cover.svg` |
| `{blog-filename}` | Filename only, e.g. `day-42-raft-consensus.md` |
| `{cover-filename}` | Filename only, e.g. `cover.svg` |

After running `pre-push-check.sh`, append its verdict as an HTML comment at the very bottom of the PR body:

```
<!-- pre-push-check: {PASS|WARN|FAIL} {N errors, N warnings} -->
```
