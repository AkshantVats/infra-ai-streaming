# shellcheck shell=bash
# Local bind ports for kubectl port-forward (avoid k3d LB host mappings on 8080/9091).

port_is_listening() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

pick_local_port() {
  local start="$1"
  local p="$start"
  for _ in $(seq 0 50); do
    if ! port_is_listening "$p"; then
      echo "$p"
      return 0
    fi
    p=$((p + 1))
  done
  echo "pick_local_port: no free port near ${start}" >&2
  return 1
}

# k3d cluster.yaml maps host 8080/9091; smoke/chaos use port-forward on different locals.
ensure_k8s_smoke_ports() {
  local ing_pref="${SMOKE_ING_LOCAL_PORT:-18080}"
  local con_pref="${SMOKE_CON_LOCAL_PORT:-19091}"
  SMOKE_ING_LOCAL_PORT="$(pick_local_port "$ing_pref")"
  SMOKE_CON_LOCAL_PORT="$(pick_local_port "$con_pref")"
  export SMOKE_ING_LOCAL_PORT SMOKE_CON_LOCAL_PORT
  export INGEST_URL="http://localhost:${SMOKE_ING_LOCAL_PORT}"
  export METRICS_INGEST="${INGEST_URL}/metrics"
  export METRICS_CONSUMER="http://localhost:${SMOKE_CON_LOCAL_PORT}/metrics"
}
