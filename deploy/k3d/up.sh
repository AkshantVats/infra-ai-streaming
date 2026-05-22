#!/usr/bin/env bash
# Create k3d cluster and import locally built app images.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

CLUSTER="${K3D_CLUSTER:-lensai}"

if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER} "; then
  echo "==> k3d cluster '${CLUSTER}' already exists (skip create)"
else
  echo "==> Creating k3d cluster '${CLUSTER}'"
  k3d cluster create "${CLUSTER}" --config deploy/k3d/cluster.yaml
fi

echo "==> Building Docker images"
docker build -f deploy/docker/Dockerfile.ingestion -t lensai/ingestion:local .
docker build -f deploy/docker/Dockerfile.consumer -t lensai/consumer:local .

echo "==> Importing images into k3d"
k3d image import lensai/ingestion:local lensai/consumer:local -c "${CLUSTER}"

echo "==> Cluster ready. Next:"
echo "    helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-k3d.yaml --wait --timeout 10m"
echo "    ./scripts/smoke-k8s-e2e.sh"
