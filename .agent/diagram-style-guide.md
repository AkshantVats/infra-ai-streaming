# Diagram Style Guide

Concise reference for all Mermaid diagrams used in blog posts on this project.
Every diagram must follow these rules before pushing — the pre-push check
enforces checks 2 and 3 automatically.

---

## Required Init Block

Every mermaid code block **must** begin with this exact init block as its
first line. Copy it verbatim — do not reorder keys or change values.

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

The `'background': 'transparent'` value renders correctly in both GitHub
light and dark themes. Do not replace it with a hex color.

---

## Node Label Rules

| Rule | Limit |
|---|---|
| Max words per label | 6 |
| Max nodes per diagram | 8 |
| Long labels | Use quoted strings (`["label text"]`) to allow wrapping |

Keep labels short so diagrams stay readable at blog widths. If a concept
needs more explanation, add a prose caption below the diagram rather than
cramming text into the node.

---

## When to Use Each Diagram Type

**`flowchart LR`** — system flows and data pipelines

Use when data moves left-to-right through components: ingestion, processing,
output. The horizontal layout mirrors how engineers read pipelines.

**`sequenceDiagram`** — request/response interactions

Use when the order of messages between actors matters: HTTP calls,
event-driven handoffs, client-server protocols. Sequence diagrams make
timing and turn-taking explicit.

**`graph TD`** — hierarchies and tree structures

Use when a parent-child or dependency relationship drives the diagram:
service trees, config inheritance, taxonomy breakdowns. Top-down layout
reflects hierarchy naturally.

---

## Color Palette Reference

| Role | Hex | Used for |
|---|---|---|
| Primary blue | `#4a90d9` | Borders, lines, edge label backgrounds |
| Dark background | `#0d2137` | Node fills, cluster backgrounds |
| Deepest background | `#0a1a2e` | Tertiary / deeply nested backgrounds |
| Node fill (primary) | `#1e3a5f` | Main nodes |
| Text | `#f0f4f8` | All label and title text |
| Canvas | `transparent` | Outer diagram background |

---

## Bad vs Good Node Definitions

### flowchart LR node

```
# BAD — label is 8 words and unquoted
A[Ingest streaming data from Kafka topic partition]

# GOOD — trimmed to 6 words, quoted for safe wrapping
A["Ingest from Kafka partition"]
```

### sequenceDiagram participant

```
# BAD — participant alias reads like a full sentence
participant StreamProcessingWorkerService

# GOOD — short, scannable alias
participant Worker
```

### graph TD decision node

```
# BAD — curly-brace label has 7 words
D{Should we retry the failed request now?}

# GOOD — tight question, quoted, within limit
D{"Retry failed request?"}
```

---

## Transparent Background Note

`'background': 'transparent'` tells Mermaid to leave the canvas color unset,
so the diagram inherits whatever background the page applies. On GitHub this
means white in light mode and dark gray in dark mode — both are readable with
the dark-blue node fills defined above. Replacing `transparent` with a fixed
hex value will break one of the two themes.

---

## Quick Checklist Before Pushing

- [ ] Init block is the **first line** inside every ` ```mermaid ` fence
- [ ] No label exceeds 6 words
- [ ] Diagram has 8 nodes or fewer
- [ ] Diagram type matches the content (LR flow / sequence / TD hierarchy)
- [ ] Background is `'transparent'`, not a hex value
- [ ] `pre-push-check.sh` reports PASS for Check 2 and Check 3
