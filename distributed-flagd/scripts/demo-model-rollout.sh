#!/usr/bin/env bash
# demo-model-rollout.sh — canary flip: gpt-4o-mini → gpt-4o (10% → 50% → 100%)
# Prerequisites: kubectl configured, flagd HTTP at localhost:8080
# Usage: bash scripts/demo-model-rollout.sh [namespace]
set -euo pipefail

NS="${1:-default}"
FLAGKEY="model-rollout:demo-tenant"
FLAGD_URL="${FLAGD_URL:-http://localhost:8080}"

step() { echo ""; echo "── $* ──"; }

apply_flag() {
    local gpt4o_weight=$1
    local mini_weight=$((100 - gpt4o_weight))
    kubectl apply -n "${NS}" -f - <<EOF
apiVersion: flagd.lensai.io/v1alpha1
kind: FlagDefinition
metadata:
  name: demo-tenant-model-rollout
  labels:
    app.kubernetes.io/component: model-rollout
spec:
  flagKey: "${FLAGKEY}"
  enabled: true
  variants:
    - value: "gpt-4o-mini"
      weight: ${mini_weight}
    - value: "gpt-4o"
      weight: ${gpt4o_weight}
EOF
    echo "  Applied: gpt-4o-mini=${mini_weight}%  gpt-4o=${gpt4o_weight}%"
    sleep 2  # allow controller sync to etcd
}

sample_eval() {
    echo "  Sampling 8 evaluations (deterministic by user-id):"
    for i in $(seq 1 8); do
        model=$(curl -sf "${FLAGD_URL}/evaluate" \
            -H "Content-Type: application/json" \
            -d "{\"tenant_id\":\"demo-tenant\",\"user_id\":\"user-${i}\"}" \
            | python3 -c "import sys,json; print(json.load(sys.stdin)['resolved_model_id'])" 2>/dev/null || echo "(error)")
        printf "    user-%d → %s\n" "${i}" "${model}"
    done
}

echo "=== distributed-flagd Day 24 — model canary rollout demo ==="
echo "Flag key: ${FLAGKEY}"
echo "Grafana: open 'Model Cost Split' dashboard to see real-time cost breakdown"

step "Phase 1: 10% traffic to gpt-4o"
apply_flag 10
sample_eval
echo "  Check Grafana → cost panel should show ~90% mini / ~10% gpt-4o spend"

step "Phase 2: 50% traffic to gpt-4o"
apply_flag 50
sample_eval
echo "  Check Grafana → cost panel should show ~50/50 split"

step "Phase 3: 100% traffic to gpt-4o (full cutover)"
apply_flag 100
sample_eval
echo "  Check Grafana → all cost attributed to gpt-4o"

step "Rollback: revert to 100% gpt-4o-mini"
echo "  Run: kubectl apply -f deploy/crd/examples/gpt4o-canary-10pct.yaml"
echo "  Or delete the CR for full fallback to defaultModel:"
echo "  kubectl delete flagdefinition demo-tenant-model-rollout -n ${NS}"

echo ""
echo "Done. See deploy/crd/examples/ for individual phase YAMLs."
