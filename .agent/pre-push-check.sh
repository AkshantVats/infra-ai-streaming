#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# pre-push-check.sh — validate a blog markdown file before pushing
# Usage: ./pre-push-check.sh <markdown-file>
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
HARD_ERRORS=0   # broken links, placeholder links, missing cover  → exit 2
SOFT_ERRORS=0   # diagram labels, paragraph length               → exit 1
WARNINGS=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

pass()  { echo "  PASS  $*"; }
fail_hard() { echo "  FAIL  $*"; (( HARD_ERRORS++ )) || true; }
fail_soft() { echo "  FAIL  $*"; (( SOFT_ERRORS++ )) || true; }
warn()  { echo "  WARN  $*"; (( WARNINGS++ )) || true; }

# ---------------------------------------------------------------------------
# Extract lines that are NOT inside fenced code blocks.
# Outputs each non-code line prefixed with its 1-based line number and a tab.
# ---------------------------------------------------------------------------
non_code_lines() {
  awk '
    /^```/ { in_code = !in_code; next }
    !in_code { print NR "\t" $0 }
  ' "$MARKDOWN_FILE"
}

# ---------------------------------------------------------------------------
# 1. LINK CHECK — http/https URLs only; skip relative and anchor-only links
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 1: Link check ==="

LINK_FAILS=0

# Extract all markdown links from non-code sections: [text](url)
while IFS=$'\t' read -r lineno line; do
  # Extract all URLs from this line
  while [[ "$line" =~ \[[^]]*\]\(([^)]+)\) ]]; do
    url="${BASH_REMATCH[1]}"
    # Remove the matched portion so we loop to the next link on the same line
    line="${line#*"${BASH_REMATCH[0]}"}"

    # Skip anchor-only links
    if [[ "$url" == \#* ]]; then
      continue
    fi

    # Skip non-http/https
    if [[ "$url" != http://* && "$url" != https://* ]]; then
      continue
    fi

    http_code=$(curl --head --silent --max-time 5 --location \
      --write-out "%{http_code}" --output /dev/null "$url" 2>/dev/null || echo "000")

    if [[ "$http_code" =~ ^[45] ]]; then
      fail_hard "Line $lineno: HTTP $http_code for $url"
      (( LINK_FAILS++ )) || true
    else
      pass "Line $lineno: HTTP $http_code $url"
    fi
  done
done < <(non_code_lines)

if [[ $LINK_FAILS -eq 0 ]]; then
  pass "All links returned 2xx/3xx"
fi

# ---------------------------------------------------------------------------
# 2. DIAGRAM LABEL LENGTH — labels in mermaid blocks must be ≤ 6 words
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 2: Diagram label length ==="

LABEL_FAILS=0

# Walk the file extracting mermaid blocks with their starting line numbers
in_mermaid=0
mermaid_start=0
mermaid_body=()

while IFS= read -r line || [[ -n "$line" ]]; do
  (( lineno_diag++ )) || lineno_diag=1

  if [[ "$line" =~ ^\`\`\`mermaid ]]; then
    in_mermaid=1
    mermaid_start=$lineno_diag
    mermaid_body=()
    continue
  fi

  if [[ $in_mermaid -eq 1 && "$line" =~ ^\`\`\` ]]; then
    in_mermaid=0
    # Analyze collected body
    body_lineno=0
    for body_line in "${mermaid_body[@]}"; do
      (( body_lineno++ )) || true
      actual_lineno=$(( mermaid_start + body_lineno ))

      # Only inspect node definition lines (contain [ ( or {)
      if [[ "$body_line" =~ [\[\(\{] ]]; then
        # Extract label text: content between [ ] ( ) { }
        label=""
        if [[ "$body_line" =~ \[\"([^\"]+)\"\] ]]; then
          label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \[([^\]]+)\] ]]; then
          label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \(\"([^\"]+)\"\) ]]; then
          label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \(([^\)]+)\) ]]; then
          label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \{\"([^\"]+)\"\} ]]; then
          label="${BASH_REMATCH[1]}"
        elif [[ "$body_line" =~ \{([^\}]+)\} ]]; then
          label="${BASH_REMATCH[1]}"
        fi

        if [[ -n "$label" ]]; then
          word_count=$(echo "$label" | wc -w)
          if (( word_count > 6 )); then
            fail_soft "Line $actual_lineno: label \"$label\" has $word_count words (max 6)"
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
# 3. MERMAID INIT BLOCK — every mermaid block must start with %%{init:
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 3: Mermaid init block ==="

INIT_FAILS=0
in_mermaid=0
mermaid_start=0
first_content_line=""
lineno_diag=0

while IFS= read -r line || [[ -n "$line" ]]; do
  (( lineno_diag++ )) || lineno_diag=1

  if [[ "$line" =~ ^\`\`\`mermaid ]]; then
    in_mermaid=1
    mermaid_start=$lineno_diag
    first_content_line=""
    continue
  fi

  if [[ $in_mermaid -eq 1 && "$line" =~ ^\`\`\` ]]; then
    in_mermaid=0
    if [[ -z "$first_content_line" || "$first_content_line" != "%%{init:"* ]]; then
      fail_soft "Mermaid block at line $mermaid_start does not start with '%%{init:'"
      (( INIT_FAILS++ )) || true
    else
      pass "Mermaid block at line $mermaid_start has %%{init: header"
    fi
    continue
  fi

  if [[ $in_mermaid -eq 1 && -z "$first_content_line" && -n "$line" ]]; then
    first_content_line="$line"
  fi
done < "$MARKDOWN_FILE"

if [[ $INIT_FAILS -eq 0 ]]; then
  pass "All mermaid blocks have %%{init: header"
fi

# ---------------------------------------------------------------------------
# 4. PARAGRAPH LENGTH — paragraphs in body must not exceed 3 sentences
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 4: Paragraph length ==="

PARA_FAILS=0

# Read the file into a variable, strip code blocks, then split on blank lines
# to get paragraphs.
file_no_code=$(awk '
  /^```/ { in_code = !in_code; next }
  !in_code { print }
' "$MARKDOWN_FILE")

# Also skip front-matter (--- delimited)
file_no_frontmatter=$(echo "$file_no_code" | awk '
  NR==1 && /^---/ { in_fm=1; next }
  in_fm && /^---/ { in_fm=0; next }
  !in_fm { print }
')

# Process paragraphs: split on blank lines
para_index=0
current_para=""

while IFS= read -r line || [[ -n "$line" ]]; do
  if [[ -z "$line" ]]; then
    if [[ -n "$current_para" ]]; then
      (( para_index++ )) || true
      # Count sentence endings: ". " or "! " or "? " plus end-of-string
      sentence_count=$(echo "$current_para" | grep -oE '\. |\! |\? |[.!?]$' | wc -l)
      if (( sentence_count > 3 )); then
        snippet="${current_para:0:80}..."
        fail_soft "Paragraph $para_index: $sentence_count sentences (max 3). Starts with: \"$snippet\""
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
done <<< "$file_no_frontmatter"

# Handle final paragraph with no trailing blank line
if [[ -n "$current_para" ]]; then
  (( para_index++ )) || true
  sentence_count=$(echo "$current_para" | grep -oE '\. |\! |\? |[.!?]$' | wc -l)
  if (( sentence_count > 3 )); then
    snippet="${current_para:0:80}..."
    fail_soft "Paragraph $para_index: $sentence_count sentences (max 3). Starts with: \"$snippet\""
    (( PARA_FAILS++ )) || true
  fi
fi

if [[ $PARA_FAILS -eq 0 ]]; then
  pass "All paragraphs are ≤ 3 sentences"
fi

# ---------------------------------------------------------------------------
# 5. PLACEHOLDER LINKS — flag example.com, TODO, placeholder, localhost
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 5: Placeholder links ==="

PLACEHOLDER_FAILS=0

while IFS=$'\t' read -r lineno line; do
  while [[ "$line" =~ \[[^]]*\]\(([^)]+)\) ]]; do
    url="${BASH_REMATCH[1]}"
    line="${line#*"${BASH_REMATCH[0]}"}"

    if [[ "$url" =~ example\.com|TODO|placeholder|localhost ]]; then
      fail_hard "Line $lineno: placeholder URL found: $url"
      (( PLACEHOLDER_FAILS++ )) || true
    fi
  done
done < <(non_code_lines)

if [[ $PLACEHOLDER_FAILS -eq 0 ]]; then
  pass "No placeholder links found"
fi

# ---------------------------------------------------------------------------
# 6. COVER IMAGE PATH — must have ![cover](...) and path must exist
# ---------------------------------------------------------------------------
echo ""
echo "=== CHECK 6: Cover image path ==="

cover_line=$(grep -n '!\[cover\]' "$MARKDOWN_FILE" | head -1 || true)

if [[ -z "$cover_line" ]]; then
  fail_hard "No ![cover] image tag found in the markdown"
else
  lineno_cover="${cover_line%%:*}"
  cover_text="${cover_line#*:}"

  # Extract the path from ![cover](path)
  if [[ "$cover_text" =~ !\[cover\]\(([^)]+)\) ]]; then
    cover_path="${BASH_REMATCH[1]}"

    if [[ "$cover_path" == http://* || "$cover_path" == https://* ]]; then
      pass "Cover image is a remote URL: $cover_path"
    else
      # Resolve relative path against the markdown file's directory
      if [[ "$cover_path" == /* ]]; then
        full_cover_path="$cover_path"
      else
        full_cover_path="$MARKDOWN_DIR/$cover_path"
      fi

      if [[ -f "$full_cover_path" ]]; then
        pass "Cover image exists: $full_cover_path"
      else
        fail_hard "Line $lineno_cover: cover image not found at: $full_cover_path"
      fi
    fi
  else
    fail_hard "Line $lineno_cover: could not parse cover image path"
  fi
fi

# ---------------------------------------------------------------------------
# SUMMARY
# ---------------------------------------------------------------------------
echo ""
echo "=================================================="
echo "SUMMARY"
echo "  Hard errors (exit 2): $HARD_ERRORS  [broken/placeholder links, missing cover]"
echo "  Soft errors (exit 1): $SOFT_ERRORS  [diagram labels, paragraph length, init blocks]"
echo "  Warnings:             $WARNINGS"
echo "=================================================="

if [[ $HARD_ERRORS -gt 0 ]]; then
  echo "VERDICT: FAILED (hard errors present)"
  exit 2
elif [[ $SOFT_ERRORS -gt 0 ]]; then
  echo "VERDICT: FAILED (soft errors present)"
  exit 1
else
  echo "VERDICT: PASSED"
  exit 0
fi
