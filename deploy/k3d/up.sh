#!/usr/bin/env bash
# Create k3d cluster and import locally built app images.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

CLUSTER="${K3D_CLUSTER:-lensai}"
HOST_HTTP_PORT="${K3D_HOST_HTTP_PORT:-8080}"
HOST_METRICS_PORT="${K3D_HOST_METRICS_PORT:-9091}"

port_is_listening() {
  local p="$1"
  # lsof returns non-zero if nothing is listening.
  lsof -nP -iTCP:"${p}" -sTCP:LISTEN >/dev/null 2>&1
}

find_free_port() {
  local start="$1"
  local p="$start"
  # Cap search to avoid infinite loops on misconfigured systems.
  for _ in $(seq 0 50); do
    if ! port_is_listening "$p"; then
      echo "$p"
      return 0
    fi
    p=$((p + 1))
  done
  return 1
}

if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER} "; then
  echo "==> k3d cluster '${CLUSTER}' already exists (skip create)"
else
  echo "==> Creating k3d cluster '${CLUSTER}'"
  HTTP_PORT="$(find_free_port "$HOST_HTTP_PORT")"
  METRICS_PORT="$(find_free_port "$HOST_METRICS_PORT")"
  echo "==> Using host ports http=${HTTP_PORT} metrics=${METRICS_PORT}"

  TMP_CONFIG="$(mktemp -t k3d-cluster.XXXXXX.yaml)"
  cp deploy/k3d/cluster.yaml "$TMP_CONFIG"
  # cluster.yaml uses fixed port mappings; rewrite them to avoid collisions (e.g., port 8080 already in use).
  sed -i.bak -e "s/- port: ${HOST_HTTP_PORT}:80/- port: ${HTTP_PORT}:80/" "$TMP_CONFIG"
  sed -i.bak -e "s/- port: ${HOST_METRICS_PORT}:${HOST_METRICS_PORT}/- port: ${METRICS_PORT}:${METRICS_PORT}/" "$TMP_CONFIG"
  rm -f "${TMP_CONFIG}.bak"

  trap 'rm -f "$TMP_CONFIG"' EXIT
  k3d cluster create "${CLUSTER}" --config "$TMP_CONFIG"
fi

echo "==> Building Docker images"
GIT_SHA="${GIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
BUILD_ARGS=(--build-arg "GIT_SHA=${GIT_SHA}" --build-arg "BUILD_TIME=${BUILD_TIME}")
docker build "${BUILD_ARGS[@]}" -f deploy/docker/Dockerfile.ingestion -t lensai/ingestion:local .
docker build "${BUILD_ARGS[@]}" -f deploy/docker/Dockerfile.consumer -t lensai/consumer:local .

echo "==> Importing images into k3d"
k3d image import lensai/ingestion:local lensai/consumer:local -c "${CLUSTER}"

echo "==> Cluster ready. Next:"
echo "    helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-k3d.yaml --wait --timeout 10m"
echo "    ./scripts/smoke-k8s-e2e.sh"
