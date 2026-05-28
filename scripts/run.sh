#!/usr/bin/env bash
# scripts/run.sh — Config-driven entry point for local deploy and E2E.
#
#   ./scripts/run.sh --profile m1
#   ./scripts/run.sh --profile m1 --skip-chaos
#   ./scripts/run.sh --values deploy/helm/lensai/values.mycluster.yaml
#   ./scripts/run.sh --profile m1 --target compose
#   LENSAI_PROFILE=m1 ./scripts/run.sh
#
# Profiles map to Helm values under deploy/helm/lensai/ and Compose env under deploy/compose/.
# Copy deploy/helm/lensai/values.example.yaml → values.mycluster.yaml for custom clusters.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

HELM_CHART="${ROOT}/deploy/helm/lensai"
COMPOSE_FILE="${ROOT}/deploy/docker-compose.yml"

PROFILE="${LENSAI_PROFILE:-m1}"
TARGET="${LENSAI_TARGET:-k3d}"
CUSTOM_VALUES=""
SKIP_CHAOS=0
SKIP_DEPLOY=0
SKIP_PREFLIGHT=0
EXTRA_ARGS=()

usage() {
  cat <<'EOF'
Usage: ./scripts/run.sh [options]

Options:
  --profile NAME       Helm/Compose profile (default: m1, or LENSAI_PROFILE)
                       Built-in: default, dev, m1, k3d
  --values PATH        Custom Helm values file (overrides --profile for Helm)
  --target TARGET      k3d (default) | compose | helm
  --skip-chaos         Skip chaos steps in k3d E2E
  --skip-deploy        Skip cluster deploy (k3d E2E only)
  --skip-preflight     Skip unit tests (k3d E2E only)
  -h, --help           Show this help

Examples:
  ./scripts/run.sh --profile m1
  ./scripts/run.sh --profile dev --target compose
  ./scripts/run.sh --values deploy/helm/lensai/values.mycluster.yaml --target helm
EOF
}

resolve_helm_values() {
  if [[ -n "$CUSTOM_VALUES" ]]; then
    echo "$CUSTOM_VALUES"
    return
  fi
  case "$PROFILE" in
    default|prod) echo "${HELM_CHART}/values.yaml" ;;
    dev)         echo "${HELM_CHART}/values-dev.yaml" ;;
    m1)          echo "${HELM_CHART}/values-m1.yaml" ;;
    k3d)         echo "${HELM_CHART}/values-k3d.yaml" ;;
    *)
      local candidate="${HELM_CHART}/values-${PROFILE}.yaml"
      if [[ -f "$candidate" ]]; then
        echo "$candidate"
      else
        echo "Unknown profile '${PROFILE}'. Use --values PATH or a built-in profile (default, dev, m1, k3d)." >&2
        exit 1
      fi
      ;;
  esac
}

resolve_compose_env() {
  case "$PROFILE" in
    default|prod|dev) echo "${ROOT}/deploy/compose/values-dev.env" ;;
    m1)               echo "${ROOT}/deploy/compose/values-m1.env" ;;
    k3d)              echo "${ROOT}/deploy/compose/values-dev.env" ;;
    *)
      local candidate="${ROOT}/deploy/compose/values-${PROFILE}.env"
      if [[ -f "$candidate" ]]; then
        echo "$candidate"
      else
        echo "${ROOT}/deploy/.env.example"
      fi
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --values)
      CUSTOM_VALUES="$2"
      shift 2
      ;;
    --target)
      TARGET="$2"
      shift 2
      ;;
    --skip-chaos)
      SKIP_CHAOS=1
      shift
      ;;
    --skip-deploy)
      SKIP_DEPLOY=1
      shift
      ;;
    --skip-preflight)
      SKIP_PREFLIGHT=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      EXTRA_ARGS+=("$1")
      shift
      ;;
  esac
done

HELM_VALUES="$(resolve_helm_values)"
COMPOSE_ENV="$(resolve_compose_env)"

export LENSAI_PROFILE="$PROFILE"
export LENSAI_HELM_VALUES="$HELM_VALUES"
export LENSAI_COMPOSE_ENV="$COMPOSE_ENV"

echo "[run] profile=${PROFILE} target=${TARGET}"
echo "[run] helm values=${HELM_VALUES}"
echo "[run] compose env=${COMPOSE_ENV}"

case "$TARGET" in
  k3d)
    export HELM_VALUES_FILE="$HELM_VALUES"
    [[ "$SKIP_CHAOS" == "1" ]] && export SKIP_CHAOS=1
    [[ "$SKIP_DEPLOY" == "1" ]] && export SKIP_DEPLOY=1
    [[ "$SKIP_PREFLIGHT" == "1" ]] && export SKIP_PREFLIGHT=1
    exec "${ROOT}/scripts/e2e-k3d-full.sh" "${EXTRA_ARGS[@]}"
    ;;
  compose)
    if [[ ! -f "$COMPOSE_ENV" ]]; then
      echo "Compose env file not found: ${COMPOSE_ENV}" >&2
      echo "Copy deploy/.env.example to deploy/.env or add deploy/compose/values-${PROFILE}.env" >&2
      exit 1
    fi
    exec docker compose --env-file "$COMPOSE_ENV" -f "$COMPOSE_FILE" up -d "${EXTRA_ARGS[@]}"
    ;;
  helm)
    NS="${K8S_NAMESPACE:-lensai}"
    RELEASE="${HELM_RELEASE:-lensai}"
    helm dependency update "$HELM_CHART"
    exec helm upgrade --install "$RELEASE" "$HELM_CHART" \
      -n "$NS" --create-namespace \
      -f "$HELM_VALUES" \
      --timeout "${HELM_WAIT_TIMEOUT:-2m}" \
      --wait=false \
      --wait-for-jobs=false \
      "${EXTRA_ARGS[@]}"
    ;;
  *)
    echo "Unknown target '${TARGET}'. Use k3d, compose, or helm." >&2
    exit 1
    ;;
esac
