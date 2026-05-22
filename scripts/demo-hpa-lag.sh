#!/usr/bin/env bash
# Demo: watch consumer HPA driven by kafka_consumer_lag_sum (external metric).
# Prereq: helm release up, prometheus-adapter registered.
set -euo pipefail

NS="${K8S_NAMESPACE:-lensai}"
RELEASE="${HELM_RELEASE:-lensai}"

echo "==> External metrics API (kafka_consumer_lag_sum)"
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/${NS}/kafka_consumer_lag_sum" 2>/dev/null | head -c 500 || \
  echo "WARN: external metric not listed yet — check prometheus-adapter logs"

echo "==> HPA (watch mode — Ctrl+C to stop)"
kubectl describe hpa -n "${NS}" "${RELEASE}-consumer" 2>/dev/null || kubectl describe hpa -n "${NS}"

echo ""
echo "To raise lag: port-forward ingestion and flood /ingest while consumer is slow or scaled to 1."
echo "  kubectl port-forward -n ${NS} svc/ingestion 8080:8080"
echo "  ./scripts/smoke-k8s-e2e.sh  # baseline"
echo ""
kubectl get hpa -n "${NS}" -w
