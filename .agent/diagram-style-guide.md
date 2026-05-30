# Diagram Style Guide

Concise reference for all Mermaid diagrams used in blog posts on this project.
Every diagram must follow these rules before pushing.

---

## Required Init Block

Every mermaid code block **must** begin with this exact init block (the
pre-push check enforces it):

~~~
```mermaid
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
~~~

The `"transparent"` background value renders correctly in both GitHub light
and dark themes — do not replace it with a hex color.

---

## Node Label Rules

| Rule | Limit |
|---|---|
| Max words per label | 6 |
| Max nodes per diagram | 8 |
| Long labels | Use quoted strings for wrapping |

Keep labels short so diagrams stay readable at blog widths. If a concept
needs more text, add a caption below the diagram instead.

---

## When to Use Each Diagram Type

**`flowchart LR`** — system flows and data pipelines

Use when you need to show data moving left-to-right through components:
ingestion, processing, output. The horizontal layout matches how readers
scan a pipeline.

**`sequenceDiagram`** — request/response interactions

Use when the order of messages between actors matters: HTTP calls,
event-driven handoffs, client-server protocols. Sequence diagrams make
timing and turn-taking explicit.

**`graph TD`** — hierarchies and tree structures

Use when a parent-child or dependency relationship drives the diagram:
service trees, config inheritance, taxonomy breakdowns. Top-down layout
reflects the hierarchy naturally.

---

## Color Palette Reference

| Role | Hex | Usage |
|---|---|---|
| Primary blue | `#4a90d9` | Borders, lines, edge labels |
| Dark background | `#0d2137` | Node fills, cluster backgrounds |
| Deepest background | `#0a1a2e` | Tertiary / nested backgrounds |
| Node fill (primary) | `#1e3a5f` | Main nodes |
| Text | `#f0f4f8` | All label and title text |
| Canvas | `transparent` | Outer background |

---

## Bad vs Good Node Definitions

### flowchart LR node

```
# BAD — label is too long (8 words) and unquoted
A[Ingest streaming data from Kafka topic partition]

# GOOD — label trimmed to 6 words, quoted for safety
A["Ingest from Kafka partition"]
```

### sequenceDiagram participant

```
# BAD — participant name reads like a sentence
participant StreamProcessingWorkerService

# GOOD — short, readable name
participant Worker
```

### graph TD node with shape

```
# BAD — curly-brace label has 7 words
D{Should we retry the failed request now?}

# GOOD — decision label is a tight question
D{"Retry failed request?"}
```

---

## Quick Checklist Before Pushing

- [ ] Init block is the first line inside every ` ```mermaid ` fence
- [ ] No label exceeds 6 words
- [ ] Diagram has 8 nodes or fewer
- [ ] Diagram type matches the content (LR flow / sequence / TD hierarchy)
- [ ] Background is set to `"transparent"`, not a hex value
- [ ] `pre-push-check.sh` reports PASS for Check 2 and Check 3
