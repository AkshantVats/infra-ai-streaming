#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Smoke test: builds the real traceforge binary and runs it against a
# checked-in sample event log bundle (testdata/sample_run.jsonl). Unlike
# the unit suite, which exercises pkg/replay, pkg/diff, etc. against
# in-memory fixtures, this runs the actual compiled CLI end to end —
# flag parsing, file I/O, and exit codes included — so a regression that
# only shows up when the pieces are wired together (not caught by any one
# package's tests in isolation) fails CI instead of shipping.
set -euo pipefail

cd "$(dirname "$0")/.."

BIN="$(mktemp -d)/traceforge"
go build -o "$BIN" ./cmd/traceforge
LOG="testdata/sample_run.jsonl"

pass=0
fail=0

check() {
  local desc="$1"
  if eval "$2"; then
    echo "ok — $desc"
    pass=$((pass + 1))
  else
    echo "FAIL — $desc"
    fail=$((fail + 1))
  fi
}

replay_full=$("$BIN" replay --log "$LOG" --trace-id trace-checkout-1001)
check "replay to completion reports recorded final_output" \
  '[[ "$replay_full" == *"order-1001 confirmed: charged \$24.99, confirmation emailed"* ]]'

replay_stopped=$("$BIN" replay --log "$LOG" --trace-id trace-checkout-1001 --stop-at-step 2)
check "replay --stop-at-step halts before the recorded output" \
  '[[ "$replay_stopped" == *"stopped early: halted at --stop-at-step=2"* ]]'

diff_out=$("$BIN" diff --log "$LOG" --trace-a trace-checkout-1001 --trace-b trace-checkout-1002)
check "diff finds the first divergence at the charge_payment step" \
  '[[ "$diff_out" == *"first divergence at step 2"* && "$diff_out" == *"tool_name=charge_payment"* ]]'

set +e
inject_err=$("$BIN" replay --log "$LOG" --trace-id trace-checkout-1001 --inject-timeout 2 2>&1 1>/dev/null)
inject_status=$?
set -e
check "inject-timeout forces the injected step to fail (nonzero exit)" '[[ $inject_status -ne 0 ]]'
check "inject-timeout reports the injected fault" '[[ "$inject_err" == *"injected timeout"* ]]'

echo
echo "smoke test: ${pass} passed, ${fail} failed"
[[ $fail -eq 0 ]]
