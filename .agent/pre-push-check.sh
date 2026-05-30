#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# pre-push-check.sh — validate a blog markdown file before pushing
# Usage: ./pre-push-check.sh <markdown-file>
#
# Exit codes:
#   0 — all checks pass (or only soft warnings)
#   1 — soft failures (diagram labels, paragraph length, mermaid init block)
#   2 — hard failures (broken links, placeholder links, missing cover image)
# ---------------------------------------------------------------------------

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <markdown-file>" >&2
  exit 1
fi

MARKDOWN_FILE="$1"

if [[ ! -f "$MARKDOWN_FILE" ]]; then
  echo "ERROR: file not found: $MARKDOWN_FILE" >&2
  exit 1
fi

MARKDOWN_DIR="$(dirname "$MARKDOWN_FILE")"

# Counters
HARD_ERRORS=0   # broken/placeholder links, missing cover  → exit 2
SOFT_ERRORS=0   # diagram labels, paragraph length, init   → exit 1
WARNINGS=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

pass()      { printf "  PASS  %s\n" "$*"; }
fail_hard() { printf "  FAIL  %s\n" "$*"; (( HARD_ERRORS++ )) || true; }
fail_soft() { printf "  FAIL  %s\n" "$*"; (( SOFT_ERRORS++ )) || true; }
warn()      { printf "  WARN  %s\n" "$*"; (( WARNINGS++ )) || true; }

# Emit non-code lines as: "<lineno>\t<line>"
# Toggles on ``` (any fence), so mermaid and plain code blocks are both skipped.
non_code_lines() {
  awk '
    /^```/ { in_code = !in_code; next }
    !in_code { print NR "\t" $0 }
  ' "$MARKDOWN_FILE"
}

# ---------------------------------------------------------------------------
# CHECK 1: Link check
# For each [text](url) in non-code sections, curl http/https URLs.
# Skip relative URLs and anchor-only links (#...).
# Flag any that return 4xx or 5xx.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 1: Link check ==="

LINK_FAILS=0

while IFS=$'\t' read -r lineno line; do
  # Iterate over all markdown links on this line
  while [[ "$line" =~ \[[^]]*\]\(([^)]+)\) ]]; do
    url="${BASH_REMATCH[1]}"
    # Advance past the matched link so the next iteration finds the next one
    line="${line#*"${BASH_REMATCH[0]}"}"

    # Skip anchor-only links
    [[ "$url" == \#* ]] && continue

    # Skip non-http/https (relative paths, mailto:, etc.)
    [[ "$url" != http://* && "$url" != https://* ]] && continue

    http_code=$(curl --head --silent --max-time 5 --location \
      --write-out "%{http_code}" --output /dev/null "$url" 2>/dev/null || echo "000")

    if [[ "$http_code" =~ ^[45] ]]; then
      fail_hard "Line $lineno: HTTP $http_code → $url"
      (( LINK_FAILS++ )) || true
    else
      pass "Line $lineno: HTTP $http_code → $url"
    fi
  done
done < <(non_code_lines)

if [[ $LINK_FAILS -eq 0 ]]; then
  pass "All http/https links returned non-4xx/5xx"
fi

# ---------------------------------------------------------------------------
# CHECK 2: Diagram label length
# Parse all ```mermaid blocks. For each node definition line (containing
# [ ( or {), extract the label text and verify it is ≤ 6 words.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 2: Diagram label length ==="

LABEL_FAILS=0
in_mermaid=0
mermaid_start=0
mermaid_body=()
file_lineno=0

while IFS= read -r line || [[ -n "$line" ]]; do
  (( file_lineno++ )) || file_lineno=1

  if [[ "$line" =~ ^\`\`\`mermaid ]]; then
    in_mermaid=1
    mermaid_start=$file_lineno
    mermaid_body=()
    continue
  fi

  if [[ $in_mermaid -eq 1 && "$line" =~ ^\`\`\`$ ]]; then
    in_mermaid=0
    body_lineno=0
    for body_line in "${mermaid_body[@]}"; do
      (( body_lineno++ )) || true
      actual_lineno=$(( mermaid_start + body_lineno ))

      # Only inspect lines that look like node definitions
      if [[ "$body_line" =~ [\[\(\{] ]]; then
        label=""

        # Try to extract label text in order of specificity (quoted first)
        if   [[ "$body_line" =~ \[\"([^\"]+)\"\] ]];  then label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \[([^\]]+)\] ]];       then label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \(\"([^\"]+)\"\) ]];   then label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \(([^\)]+)\) ]];       then label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \{\"([^\"]+)\"\} ]];   then label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \{([^\}]+)\} ]];       then label="${BASH_REMATCH[1]}"
        fi

        if [[ -n "$label" ]]; then
          word_count=$(printf '%s' "$label" | wc -w)
          if (( word_count > 6 )); then
            fail_soft "Line $actual_lineno: label \"$label\" is $word_count words (max 6)"
            (( LABEL_FAILS++ )) || true
          fi
        fi
      fi
    done
    continue
  fi

  if [[ $in_mermaid -eq 1 ]]; then
    mermaid_body+=("$line")
  fi
done < "$MARKDOWN_FILE"

if [[ $LABEL_FAILS -eq 0 ]]; then
  pass "All mermaid node labels are ≤ 6 words"
fi

# ---------------------------------------------------------------------------
# CHECK 3: Mermaid init block
# Every ```mermaid block must have %%{init: as its first non-empty line.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 3: Mermaid init block ==="

INIT_FAILS=0
in_mermaid=0
mermaid_start=0
first_content_line=""
file_lineno=0

while IFS= read -r line || [[ -n "$line" ]]; do
  (( file_lineno++ )) || file_lineno=1

  if [[ "$line" =~ ^\`\`\`mermaid ]]; then
    in_mermaid=1
    mermaid_start=$file_lineno
    first_content_line=""
    continue
  fi

  if [[ $in_mermaid -eq 1 && "$line" =~ ^\`\`\`$ ]]; then
    in_mermaid=0
    if [[ -z "$first_content_line" || "$first_content_line" != "%%{init:"* ]]; then
      fail_soft "Mermaid block at line $mermaid_start: first line is not '%%{init:' (got: \"$first_content_line\")"
      (( INIT_FAILS++ )) || true
    else
      pass "Mermaid block at line $mermaid_start has %%{init: header"
    fi
    continue
  fi

  # Capture first non-empty content line inside the block
  if [[ $in_mermaid -eq 1 && -z "$first_content_line" && -n "$line" ]]; then
    first_content_line="$line"
  fi
done < "$MARKDOWN_FILE"

if [[ $INIT_FAILS -eq 0 ]]; then
  pass "All mermaid blocks have %%{init: header"
fi

# ---------------------------------------------------------------------------
# CHECK 4: Paragraph length
# For each paragraph in the markdown body (outside code blocks and front
# matter), count sentences (split by ". " "! " or "? "). Flag paragraphs
# with more than 3 sentences.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 4: Paragraph length ==="

PARA_FAILS=0

# Strip fenced code blocks
file_no_code=$(awk '
  /^```/ { in_code = !in_code; next }
  !in_code { print }
' "$MARKDOWN_FILE")

# Strip YAML front matter (--- delimited at top of file)
file_body=$(printf '%s\n' "$file_no_code" | awk '
  NR == 1 && /^---/ { in_fm = 1; next }
  in_fm && /^---/   { in_fm = 0; next }
  !in_fm            { print }
')

para_index=0
current_para=""

while IFS= read -r line || [[ -n "$line" ]]; do
  if [[ -z "$line" ]]; then
    if [[ -n "$current_para" ]]; then
      (( para_index++ )) || true
      # Count sentence boundaries: ". " "! " "? " plus a sentence-ending char at EOL
      sentence_count=$(printf '%s' "$current_para" | grep -oE '\. +|\! +|\? +|[.!?]$' | wc -l)
      if (( sentence_count > 3 )); then
        snippet="${current_para:0:80}"
        fail_soft "Paragraph $para_index: $sentence_count sentences (max 3). Starts: \"$snippet...\""
        (( PARA_FAILS++ )) || true
      fi
      current_para=""
    fi
  else
    if [[ -n "$current_para" ]]; then
      current_para="$current_para $line"
    else
      current_para="$line"
    fi
  fi
done <<< "$file_body"

# Handle final paragraph with no trailing blank line
if [[ -n "$current_para" ]]; then
  (( para_index++ )) || true
  sentence_count=$(printf '%s' "$current_para" | grep -oE '\. +|\! +|\? +|[.!?]$' | wc -l)
  if (( sentence_count > 3 )); then
    snippet="${current_para:0:80}"
    fail_soft "Paragraph $para_index: $sentence_count sentences (max 3). Starts: \"$snippet...\""
    (( PARA_FAILS++ )) || true
  fi
fi

if [[ $PARA_FAILS -eq 0 ]]; then
  pass "All paragraphs are ≤ 3 sentences"
fi

# ---------------------------------------------------------------------------
# CHECK 5: Placeholder links
# Find any URL (inside [text](url)) containing example.com, TODO,
# placeholder, or localhost.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 5: Placeholder links ==="

PLACEHOLDER_FAILS=0

while IFS=$'\t' read -r lineno line; do
  while [[ "$line" =~ \[[^]]*\]\(([^)]+)\) ]]; do
    url="${BASH_REMATCH[1]}"
    line="${line#*"${BASH_REMATCH[0]}"}"

    if [[ "$url" =~ example\.com|TODO|placeholder|localhost ]]; then
      fail_hard "Line $lineno: placeholder URL → $url"
      (( PLACEHOLDER_FAILS++ )) || true
    fi
  done
done < <(non_code_lines)

if [[ $PLACEHOLDER_FAILS -eq 0 ]]; then
  pass "No placeholder links found"
fi

# ---------------------------------------------------------------------------
# CHECK 6: Cover image path
# The markdown must contain an ![cover](...) tag. If the path is relative,
# it must exist on the filesystem.
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 6: Cover image path ==="

cover_line=$(grep -n '!\[cover\]' "$MARKDOWN_FILE" | head -1 || true)

if [[ -z "$cover_line" ]]; then
  fail_hard "No ![cover](...) image tag found in the markdown"
else
  lineno_cover="${cover_line%%:*}"
  cover_text="${cover_line#*:}"

  if [[ "$cover_text" =~ !\[cover\]\(([^)]+)\) ]]; then
    cover_path="${BASH_REMATCH[1]}"

    if [[ "$cover_path" == http://* || "$cover_path" == https://* ]]; then
      pass "Line $lineno_cover: cover image is a remote URL: $cover_path"
    else
      # Resolve relative path against the directory of the markdown file
      if [[ "$cover_path" == /* ]]; then
        full_cover_path="$cover_path"
      else
        full_cover_path="$MARKDOWN_DIR/$cover_path"
      fi

      if [[ -f "$full_cover_path" ]]; then
        pass "Line $lineno_cover: cover image exists at: $full_cover_path"
      else
        fail_hard "Line $lineno_cover: cover image not found: $full_cover_path"
      fi
    fi
  else
    fail_hard "Line $lineno_cover: could not parse path from cover image tag"
  fi
fi

# ---------------------------------------------------------------------------
# SUMMARY
# ---------------------------------------------------------------------------
echo ""
echo "=================================================="
echo "SUMMARY"
echo "  Hard errors (exit 2) : $HARD_ERRORS  — broken links, placeholder links, missing cover"
echo "  Soft errors (exit 1) : $SOFT_ERRORS  — diagram labels > 6 words, paragraphs > 3 sentences, missing init block"
echo "  Warnings             : $WARNINGS"
echo "=================================================="

if (( HARD_ERRORS > 0 )); then
  echo "VERDICT: FAILED (hard errors present — fix before pushing)"
  exit 2
elif (( SOFT_ERRORS > 0 )); then
  echo "VERDICT: FAILED (soft errors present — review before pushing)"
  exit 1
else
  echo "VERDICT: PASSED"
  exit 0
fi
