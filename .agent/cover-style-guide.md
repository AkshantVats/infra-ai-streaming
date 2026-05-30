# Cover Image Style Guide

The agent generates SVG cover images for blog posts. This document defines the exact visual requirements for each series.

---

## Canvas

**1200x630px SVG** — self-contained, no external dependencies.

---

## AI Learning Series (Days 1–150)

### Background
Linear gradient from `#0d2137` (top-left) to `#1e3a5f` (bottom-right).

### Visual Element — Circuit/Node Pattern (top-right quadrant)
- 5–7 small circles: `r=4`, `fill=#4a90d9`, `opacity=0.6`
- Connected by thin lines: `stroke=#4a90d9`, `stroke-width=1`, `opacity=0.4`
- 2–3 larger accent rings (no fill): `r=60–100`, `fill=none`, `stroke=#4a90d9`, `stroke-width=1`, `opacity=0.15`

### Day Badge (top-left corner)
- Shape: pill (`rx=12`)
- Background: `#4a90d9`
- Text: `"Day N of N"`, white, `font-size=16`

### Series Label
- Positioned below the day badge
- Text: `"AI Infrastructure Learning"`
- Color: `#7ab8f5`
- `font-size=14`, `font-family=monospace`

### Title Text
- Position: bottom-left
- Max 2 lines. If the title is longer, reduce `font-size` to 36.
- `font-size=42`, `font-weight=bold`, `color=#f0f4f8`
- First line: `x=60`, `y=520`
- Second line: `x=60`, `y=572`

### Accent Line
- Horizontal rule above the title
- `stroke=#4a90d9`, `stroke-width=3`
- From `x=60` to `x=300`, at `y=495`

### Skeleton Example

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0d2137"/>
      <stop offset="100%" stop-color="#1e3a5f"/>
    </linearGradient>
  </defs>

  <!-- Background -->
  <rect width="1200" height="630" fill="url(#bg)"/>

  <!-- Accent rings (top-right) -->
  <circle cx="980" cy="160" r="100" fill="none" stroke="#4a90d9" stroke-width="1" opacity="0.15"/>
  <circle cx="1050" cy="100" r="75"  fill="none" stroke="#4a90d9" stroke-width="1" opacity="0.15"/>
  <circle cx="1100" cy="200" r="60"  fill="none" stroke="#4a90d9" stroke-width="1" opacity="0.15"/>

  <!-- Node connector lines -->
  <line x1="900" y1="80"  x2="980" y2="140" stroke="#4a90d9" stroke-width="1" opacity="0.4"/>
  <line x1="980" y1="140" x2="1060" y2="90"  stroke="#4a90d9" stroke-width="1" opacity="0.4"/>
  <line x1="1060" y1="90" x2="1120" y2="160" stroke="#4a90d9" stroke-width="1" opacity="0.4"/>
  <line x1="980" y1="140" x2="1010" y2="200" stroke="#4a90d9" stroke-width="1" opacity="0.4"/>
  <line x1="900" y1="80"  x2="940" y2="40"   stroke="#4a90d9" stroke-width="1" opacity="0.4"/>

  <!-- Nodes -->
  <circle cx="900"  cy="80"  r="4" fill="#4a90d9" opacity="0.6"/>
  <circle cx="980"  cy="140" r="4" fill="#4a90d9" opacity="0.6"/>
  <circle cx="1060" cy="90"  r="4" fill="#4a90d9" opacity="0.6"/>
  <circle cx="1120" cy="160" r="4" fill="#4a90d9" opacity="0.6"/>
  <circle cx="1010" cy="200" r="4" fill="#4a90d9" opacity="0.6"/>
  <circle cx="940"  cy="40"  r="4" fill="#4a90d9" opacity="0.6"/>

  <!-- Day badge -->
  <rect x="60" y="48" width="120" height="32" rx="12" fill="#4a90d9"/>
  <text x="120" y="69" text-anchor="middle" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="16" fill="white">Day 1 of 150</text>

  <!-- Series label -->
  <text x="60" y="104" font-family="monospace" font-size="14" fill="#7ab8f5">AI Infrastructure Learning</text>

  <!-- Accent line -->
  <line x1="60" y1="495" x2="300" y2="495" stroke="#4a90d9" stroke-width="3"/>

  <!-- Title (line 1) -->
  <text x="60" y="520" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="42" font-weight="bold" fill="#f0f4f8">Title First Line Here</text>
  <!-- Title (line 2) -->
  <text x="60" y="572" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="42" font-weight="bold" fill="#f0f4f8">Title Second Line Here</text>
</svg>
```

---

## Experience Series

### Background
Linear gradient from `#1a0d2e` (top-left) to `#3a1f5f` (bottom-right).

### Visual Element — Data-Flow Arrows (top-right quadrant)
- 3 curved arrows flowing left-to-right: `stroke=#9b72cf`, `stroke-width=2`, `fill=none`, `opacity=0.5`
- Background horizontal "pipe" lines: `stroke=#9b72cf`, `opacity=0.2`

### Episode Badge (top-left corner)
- Shape: pill (`rx=12`)
- Background: `#9b72cf`
- Text: `"Experience N of N"`, white, `font-size=16`

### Series Label
- Positioned below the episode badge
- Text: `"Engineering Experience"`
- Color: `#c4a8e8`
- `font-size=14`, `font-family=monospace`

### Title Text
- Same position as AI series: bottom-left, `x=60`, `y=520` / `y=572`
- Max 2 lines. If longer, reduce `font-size` to 36.
- `font-size=42`, `font-weight=bold`, `color=#f5f0ff`

### Accent Line
- `stroke=#9b72cf`, `stroke-width=3`
- From `x=60` to `x=300`, at `y=495`

### Skeleton Example

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1a0d2e"/>
      <stop offset="100%" stop-color="#3a1f5f"/>
    </linearGradient>
  </defs>

  <!-- Background -->
  <rect width="1200" height="630" fill="url(#bg)"/>

  <!-- Horizontal pipe lines (top-right) -->
  <line x1="800" y1="100" x2="1180" y2="100" stroke="#9b72cf" stroke-width="1" opacity="0.2"/>
  <line x1="800" y1="160" x2="1180" y2="160" stroke="#9b72cf" stroke-width="1" opacity="0.2"/>
  <line x1="800" y1="220" x2="1180" y2="220" stroke="#9b72cf" stroke-width="1" opacity="0.2"/>

  <!-- Curved data-flow arrows -->
  <path d="M820,90 C880,70 940,130 1000,110" stroke="#9b72cf" stroke-width="2" fill="none" opacity="0.5"
        marker-end="url(#arrowhead)"/>
  <path d="M860,155 C930,135 990,185 1060,165" stroke="#9b72cf" stroke-width="2" fill="none" opacity="0.5"
        marker-end="url(#arrowhead)"/>
  <path d="M900,215 C970,195 1030,245 1100,225" stroke="#9b72cf" stroke-width="2" fill="none" opacity="0.5"
        marker-end="url(#arrowhead)"/>

  <!-- Episode badge -->
  <rect x="60" y="48" width="150" height="32" rx="12" fill="#9b72cf"/>
  <text x="135" y="69" text-anchor="middle" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="16" fill="white">Experience 1 of 10</text>

  <!-- Series label -->
  <text x="60" y="104" font-family="monospace" font-size="14" fill="#c4a8e8">Engineering Experience</text>

  <!-- Accent line -->
  <line x1="60" y1="495" x2="300" y2="495" stroke="#9b72cf" stroke-width="3"/>

  <!-- Title (line 1) -->
  <text x="60" y="520" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="42" font-weight="bold" fill="#f5f0ff">Title First Line Here</text>
  <!-- Title (line 2) -->
  <text x="60" y="572" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="42" font-weight="bold" fill="#f5f0ff">Title Second Line Here</text>
</svg>
```

---

## Rules (apply to both series)

- **NEVER** use a plain solid background with text only. Every cover must have the gradient background and the visual element (circuit nodes or data-flow arrows).
- Title must fit in **2 lines max**. If the title text is too long, reduce `font-size` to 36.
- All fonts must use the SVG `font-family` attribute set to: `-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`
- The file must be **valid, self-contained SVG** — no external images, fonts, or scripts.
