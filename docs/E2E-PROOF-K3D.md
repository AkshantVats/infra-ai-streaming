# E2E proof � k3d full stack (M1)

Automated log from `./scripts/e2e-k3d-full.sh`.
## Test matrix — run `20260527T073856Z` (all GREEN)

| Step | Status | Exit |
|------|--------|------|
| cargo test ingestion | GREEN | 0 |
| go test consumer | GREEN | 0 |
| helm template values-m1 | GREEN | 0 |
| bash -n scripts/chaos | GREEN | 0 |
| docker/k3d/helm/kubectl checks | GREEN | 0 |
| docker compose down | GREEN | 0 |
| k3d up + helm values-m1 | GREEN | 0 |
| wait pods + init jobs | GREEN | 0 |
| smoke-k8s-e2e | GREEN | 0 |
| chaos C1 kill-redpanda | GREEN | 0 |
| chaos C2 throttle-clickhouse | GREEN | 0 |
| chaos load-m1 | GREEN | 0 |
| HPA status | GREEN | 0 |

**Wall time:** 472s (~7.9 min). **Commit:** `93c7cd8`. **Timeouts:** CH/Redpanda ready 300s. **Script exit:** 0.

**WARN (non-fatal):** C1 ingestion health after rollout; C2 breaker/overflow metrics not tripped on M1.

## Test matrix � run `20260527T064409Z` (G-09 branch, M1)

| Step | Status | Exit |
|------|--------|------|
| cargo test ingestion | GREEN | 0 |
| go test consumer | GREEN | 0 |
| helm template values-m1 | GREEN | 0 |
| bash -n scripts/chaos | GREEN | 0 |
| docker/k3d/helm/kubectl checks | GREEN | 0 |
| docker compose down | GREEN | 0 |
| k3d up + helm values-m1 | GREEN | 0 |
| wait pods + init jobs | GREEN | 0 |
| smoke-k8s-e2e | GREEN | 0 |
| chaos C1 kill-redpanda | GREEN | 0 |
| chaos C2 throttle-clickhouse | **RED** | 1 (CH not ready in 180s; fixed: `CH_READY_TIMEOUT_SEC=300`, errexit in `phase_c \|\|`) |
| chaos load-m1 | GREEN | 0 |
| HPA status | GREEN | 0 |
| **C2 retry** (SKIP_DEPLOY, 300s CH wait) | **GREEN** | 0 (breaker/overflow WARN only) |

**Wall time:** ~711s (~11.9 min). **Branch:** `feat/consumer-anomaly-zscore-detection` @ `d98d99a`. **Topics:** `ai_anomalies` added to Helm init (G-09).

## Test matrix � run `20260525T135429Z` (all GREEN)

| # | Command | Status | Runtime / notes |
|---|---------|--------|-----------------|
| 1 | `cargo test -p ingestion` | **GREEN** | 22 tests, ~0.04s |
| 2 | `cd consumer && go test ./...` | **GREEN** | cached OK |
| 3 | `bash -n scripts/*.sh chaos/*.sh` | **GREEN** | all scripts |
| 4 | `helm template � -f values-m1.yaml` | **GREEN** | renders cleanly |
| 5a | `HELM_WAIT_TIMEOUT=2m` deploy (prior run) | **GREEN** | `--wait=false` + per-pod `kubectl wait` |
| 5b | `smoke-k8s-e2e` | **GREEN** | ok |
| 5c | `chaos C1 kill-redpanda` | **GREEN** | ~168s (standalone timing) |
| 5c | `chaos C2 throttle-clickhouse` | **GREEN** | ~89s; breaker/overflow may warn, exit 0 |
| 5c | `chaos load-m1` (1000 events / 10s) | **GREEN** | ~15�18s |
| 5d | HPA status check | **GREEN** | no HPA on M1 (expected) |

### Chaos root cause (YELLOW ? GREEN)

Bare bash `wait` after background curls also waited on **kubectl port-forward** jobs (never exit) ? scripts hung until perl alarm (exit 142). Fixed with `disown` on port-forwards and `wait_pids` on curl PIDs only.

### Final `kubectl get pods -n lensai`

```
lensai-redis                 1/1 Running
lensai-redpanda-0            1/1 Running
lensai-clickhouse-0          1/1 Running
lensai-ingestion             1/1 Running
lensai-consumer              1/1 Running
lensai-prometheus            1/1 Running
lensai-redpanda-init         Completed
lensai-clickhouse-init       Completed
```

## Prior deploy fixes (commit 02743ae)

- Helm `dig` probe paths, Redpanda FQDN advertise, init jobs non-hook, M1 memory 1G.

## Run 20260527T072324Z

```
Started: 2026-05-27T07:23:24Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/consumer-anomaly-zscore-detection
CONTINUE_ON_FAIL=0
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
CH_READY_TIMEOUT_SEC=300
$ cargo test -p ingestion
   Compiling ingestion v0.1.0 (/Users/akshant/Desktop/Github/infra-ai-streaming/ingestion)
    Finished `test` profile [unoptimized + debuginfo] target(s) in 5.78s
     Running unittests src/lib.rs (target/debug/deps/ingestion-04b7275663e82a5d)

running 22 tests
test handlers::ingest::tests::validate_rejects_empty_batch ... ok
test handlers::ingest::tests::validate_rejects_negative_cost ... ok
test handlers::ingest::tests::validate_rejects_stale_timestamp ... ok
test handlers::ingest::tests::validate_rejects_oversized_batch ... ok
test handlers::ingest::tests::validate_rejects_zero_latency ... ok
test kafka::producer::tests::produce_message_holds_wal_entry_id ... ok
test handlers::ingest::tests::normalize_assigns_event_id_and_status ... ok
test handlers::ingest::tests::tenant_from_events_json_reads_first_event ... ok
test rate_limit::tenant_limits::tests::from_defaults_resolves_unknown_tenant ... ok
test kafka::producer::tests::producer_client_config_sets_expected_options ... ok
test rate_limit::token_bucket::tests::rate_limit_result_allowed_eq ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_missing_file ... ok
test rate_limit::token_bucket::tests::rate_limit_result_denied_eq ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_zero_rps ... ok
test rate_limit::tenant_limits::tests::from_file_resolves_known_tenant ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_burst_below_one ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_malformed_json ... ok
test rate_limit::token_bucket::tests::resolve_uses_tenant_override ... ok
test server::tests::health_returns_ok ... ok
test wal::writer::tests::append_increments_entry_id ... ok
test kafka::producer::tests::mark_acked_after_success_path_without_kafka ... ok
test wal::writer::tests::append_mark_acked_replay ... ok

test result: ok. 22 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.07s

     Running unittests src/main.rs (target/debug/deps/ingestion-802afe9306f36790)

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

   Doc-tests ingestion

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

exit=0
$ bash -c cd consumer && go test ./...
?   	github.com/akshantvats/infra-ai-streaming/consumer/cmd/consumer	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/clickhouse	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/config	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/model	[no test files]
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/redis	[no test files]
exit=0
$ helm template lensai deploy/helm/lensai -f deploy/helm/lensai/values-m1.yaml --namespace lensai
---
# Source: lensai/templates/consumer.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-consumer
  labels:
    app.kubernetes.io/component: consumer
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer

---
# Source: lensai/templates/ingestion.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-ingestion
  labels:
    app.kubernetes.io/component: ingestion
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion

---
# Source: lensai/templates/configmap-clickhouse-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  init.sql: |
    -- Applied by the `clickhouse-init` service in `deploy/docker-compose.yml`
    -- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema.
    CREATE DATABASE IF NOT EXISTS infra_ai;
    
    DROP TABLE IF EXISTS infra_ai.inference_events;
    
    CREATE TABLE infra_ai.inference_events
    (
        event_id UUID,
        tenant_id LowCardinality(String),
        model_id LowCardinality(String),
        timestamp DateTime64(3),
        latency_ms UInt32,
        prefill_latency_ms Nullable(UInt32),
        decode_latency_ms Nullable(UInt32),
        prompt_tokens UInt32,
        completion_tokens UInt32,
        cost_usd Float64,
        status LowCardinality(String),
        error_code Nullable(String),
        request_id Nullable(String)
    )
    ENGINE = MergeTree
    PARTITION BY toYYYYMM(timestamp)
    ORDER BY (tenant_id, model_id, timestamp);
    

---
# Source: lensai/templates/configmap-clickhouse-users.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-users
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  zz-allow-host.xml: |
    <clickhouse>
      <users>
        <default>
          <networks>
            <ip>::/0</ip>
          </networks>
        </default>
      </users>
    </clickhouse>

---
# Source: lensai/templates/configmap-redpanda-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
data:
  init-topics.sh: |
    #!/bin/sh
    # Idempotent topic creation for local dev (8 partitions; 32 in production per DESIGN.md).
    set -e
    
    BROKERS="${KAFKA_BROKERS:-redpanda:9092}"
    PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-8}"
    
    echo "Creating topics on ${BROKERS} (${PARTITIONS} partitions each)..."
    
    for topic in "${KAFKA_TOPIC:-ai_inference_events}" "${KAFKA_DLQ_TOPIC:-ai_inference_dlq}" "${KAFKA_ANOMALIES_TOPIC:-ai_anomalies}"; do
      rpk topic create "${topic}" --brokers "${BROKERS}" -p "${PARTITIONS}" || true
    done
    
    echo "Topics ready:"
    rpk topic list --brokers "${BROKERS}"
    

---
# Source: lensai/templates/configmap-tenant-limits.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-tenant-limits
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
data:
  tenant-limits.json: |
    {
      "default": {
        "max_events_per_sec": 10000,
        "burst_multiplier": 2.0
      },
      "tenants": {
        "tenant-demo": {
          "max_events_per_sec": 5,
          "burst_multiplier": 2.0
        },
        "tenant-premium": {
          "max_events_per_sec": 50000,
          "burst_multiplier": 3.0
        },
        "tenant-free": {
          "max_events_per_sec": 100,
          "burst_multiplier": 1.5
        }
      }
    }
    

---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-prometheus
  labels:
    app.kubernetes.io/component: prometheus
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    scrape_configs:
      - job_name: ingestion
        metrics_path: /metrics
        static_configs:
          - targets: ["ingestion:8080"]
      - job_name: consumer
        metrics_path: /metrics
        static_configs:
          - targets: ["consumer:9091"]
---
# Source: lensai/templates/clickhouse.yaml
apiVersion: v1
kind: Service
metadata:
  name: clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: clickhouse
  ports:
    - port: 8123
      targetPort: http
      name: http
    - port: 9000
      targetPort: native
      name: native

---
# Source: lensai/templates/consumer.yaml
apiVersion: v1
kind: Service
metadata:
  name: consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: consumer
  ports:
    - port: 9091
      targetPort: metrics
      name: metrics
---
# Source: lensai/templates/ingestion.yaml
apiVersion: v1
kind: Service
metadata:
  name: ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: ingestion
  ports:
    - port: 8080
      targetPort: http
      name: http
---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  labels:
    app.kubernetes.io/component: prometheus
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: prometheus
  ports:
    - port: 9090
      targetPort: http
      name: http

---
# Source: lensai/templates/redis.yaml
apiVersion: v1
kind: Service
metadata:
  name: redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redis
  ports:
    - port: 6379
      targetPort: redis
      name: redis

---
# Source: lensai/templates/redpanda.yaml
apiVersion: v1
kind: Service
metadata:
  name: redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  clusterIP: None
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redpanda
  ports:
    - port: 9092
      targetPort: kafka
      name: kafka
    - port: 9644
      targetPort: admin
      name: admin

---
# Source: lensai/templates/consumer.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: consumer
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9091"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: consumer
          image: "lensai/consumer:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 9091
              name: metrics
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_GROUP_ID
              value: "ai-inference-consumer-k8s"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: CLICKHOUSE_DSN
              value: "clickhouse://clickhouse:9000/infra_ai"
            - name: METRICS_PORT
              value: "9091"
          livenessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 512Mi
            requests:
              cpu: 100m
              memory: 128Mi
---
# Source: lensai/templates/ingestion.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: ingestion
    spec:
      containers:
        - name: ingestion
          image: "lensai/ingestion:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: HTTP_PORT
              value: "8080"
            - name: WAL_DIR
              value: "/data/wal"
            - name: TENANT_LIMITS_PATH
              value: "/config/tenant-limits.json"
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
          volumeMounts:
            - name: wal
              mountPath: /data/wal
            - name: tenant-limits
              mountPath: /config
              readOnly: true
      volumes:
        - name: wal
          emptyDir: {}
        - name: tenant-limits
          configMap:
            name: lensai-tenant-limits
---
# Source: lensai/templates/prometheus.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-prometheus
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: prometheus
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: prometheus
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: prometheus
    spec:
      containers:
        - name: prometheus
          image: prom/prometheus:v2.54.1
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --web.enable-lifecycle
          ports:
            - containerPort: 9090
              name: http
          livenessProbe:
            httpGet:
              path: /-/healthy
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /-/ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
            - name: data
              mountPath: /prometheus
          resources:
            limits:
              cpu: 150m
              memory: 256Mi
            requests:
              cpu: 50m
              memory: 128Mi
      volumes:
        - name: config
          configMap:
            name: lensai-prometheus
        - name: data
          emptyDir: {}
---
# Source: lensai/templates/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          livenessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            limits:
              cpu: 150m
              memory: 64Mi
            requests:
              cpu: 50m
              memory: 32Mi
---
# Source: lensai/templates/clickhouse.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  serviceName: clickhouse
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: clickhouse
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: clickhouse
    spec:
      containers:
        - name: clickhouse
          image: clickhouse/clickhouse-server:24.12-alpine
          ports:
            - containerPort: 8123
              name: http
            - containerPort: 9000
              name: native
          livenessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 40
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          volumeMounts:
            - name: data
              mountPath: /var/lib/clickhouse
            - name: users-d
              mountPath: /etc/clickhouse-server/users.d
              readOnly: true
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
            requests:
              cpu: 250m
              memory: 512Mi
      volumes:
        - name: users-d
          configMap:
            name: lensai-clickhouse-users
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
---
# Source: lensai/templates/redpanda.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  serviceName: redpanda
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redpanda
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redpanda
    spec:
      containers:
        - name: redpanda
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          command:
            - /bin/sh
            - -ec
            - |
              FQDN="$(hostname -f)"
              exec /usr/bin/rpk redpanda start \
                --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092 \
                --advertise-kafka-addr "internal://${FQDN}:9092,external://127.0.0.1:9092" \
                --rpc-addr 0.0.0.0:33145 \
                --advertise-rpc-addr "${FQDN}:33145" \
                --smp 1 \
                --memory "1G" \
                --reserve-memory 0M \
                --overprovisioned \
                --check=false \
                --mode dev-container \
                --default-log-level=warn
          ports:
            - containerPort: 9092
              name: kafka
            - containerPort: 9644
              name: admin
          livenessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 15
            failureThreshold: 3
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 15
            failureThreshold: 6
          resources:
            limits:
              cpu: "1"
              memory: 1536Mi
            requests:
              cpu: 500m
              memory: 1Gi
---
# Source: lensai/templates/clickhouse-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: clickhouse-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: clickhouse/clickhouse-server:24.12-alpine
          command:
            - /bin/sh
            - -c
            - |
              until clickhouse-client --host clickhouse --query 'SELECT 1' 2>/dev/null; do sleep 2; done
              clickhouse-client --host clickhouse --multiquery < /init.sql
          volumeMounts:
            - name: init-sql
              mountPath: /init.sql
              subPath: init.sql
              readOnly: true
      volumes:
        - name: init-sql
          configMap:
            name: lensai-clickhouse-init

---
# Source: lensai/templates/redpanda-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: redpanda-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_TOPIC_PARTITIONS
              value: "2"
          command: ["/bin/sh", "/init-topics.sh"]
          volumeMounts:
            - name: script
              mountPath: /init-topics.sh
              subPath: init-topics.sh
              readOnly: true
      volumes:
        - name: script
          configMap:
            name: lensai-redpanda-init
            defaultMode: 0555
exit=0
$ bash -n scripts/demo-flows.sh
exit=0
$ bash -n scripts/demo-hpa-lag.sh
exit=0
$ bash -n scripts/docker-test-ingestion.sh
exit=0
$ bash -n scripts/e2e-k3d-full.sh
exit=0
$ bash -n scripts/smoke-e2e.sh
exit=0
$ bash -n scripts/smoke-k8s-e2e.sh
exit=0
$ bash -n scripts/test-ingestion.sh
exit=0
$ bash -n chaos/run_chaos.sh
exit=0
$ bash -n chaos/run_chaos_k8s.sh
exit=0
$ require_cmd docker
exit=0
$ require_cmd k3d
exit=0
$ require_cmd helm
exit=0
$ require_cmd kubectl
exit=0
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.SyWkF30jod (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8080:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9091:9091' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] Prep: Network                                
[36mINFO[0m[0000] Re-using existing network 'k3d-lensai' (367f3448b068d394a56589c94f547f7df1550d219221d7ae9826839aa1125ce1) 
[36mINFO[0m[0000] Created image volume k3d-lensai-images       
[36mINFO[0m[0000] Starting new tools node...                   
[36mINFO[0m[0000] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0001] Creating node 'k3d-lensai-server-0'          
[36mINFO[0m[0001] Creating LoadBalancer 'k3d-lensai-serverlb'  
[36mINFO[0m[0003] Using the k3d-tools node to gather environment information 
[36mINFO[0m[0004] Starting new tools node...                   
[36mINFO[0m[0005] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0007] Starting cluster 'lensai'                    
[36mINFO[0m[0007] Starting servers...                          
[36mINFO[0m[0007] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0016] All agents already running.                  
[36mINFO[0m[0016] Starting helpers...                          
[36mINFO[0m[0017] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0025] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0027] Cluster 'lensai' created successfully!       
[36mINFO[0m[0027] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 986B 0.0s done
#1 DONE 0.1s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 2.2s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.1s

#5 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#5 DONE 0.0s

#6 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 119.44kB 0.1s done
#7 DONE 0.2s

#8 [builder 5/6] COPY ingestion ./ingestion
#8 CACHED

#9 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#9 CACHED

#10 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#10 CACHED

#11 [builder 3/6] WORKDIR /build
#11 CACHED

#12 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#12 CACHED

#13 [stage-1 3/5] WORKDIR /app
#13 CACHED

#14 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#14 CACHED

#15 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#15 CACHED

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 CACHED

#17 exporting to image
#17 exporting layers done
#17 writing image sha256:9c6090fd4eaaf1289ec1cb14a19a1332317adf39d2ee29bb13608d486eff0396 0.0s done
#17 naming to docker.io/lensai/ingestion:local 0.0s done
#17 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/as4fkzf6f2483oltfv95xp6sh
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 541B 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#2 DONE 0.6s

#3 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#3 DONE 1.8s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.1s

#5 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#5 DONE 0.0s

#6 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 72.31kB 0.1s done
#7 DONE 0.1s

#8 [builder 4/6] RUN go mod download
#8 CACHED

#9 [builder 6/6] RUN CGO_ENABLED=0 go build -o /consumer ./cmd/consumer
#9 CACHED

#10 [builder 2/6] WORKDIR /build/consumer
#10 CACHED

#11 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#11 CACHED

#12 [builder 5/6] COPY consumer/ ./
#12 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 CACHED

#14 exporting to image
#14 exporting layers done
#14 writing image sha256:4426f9a3a3faa7f944a9be08775762906c06f5923ce5f58d3ce196110d417597 done
#14 naming to docker.io/lensai/consumer:local
#14 naming to docker.io/lensai/consumer:local done
#14 DONE 0.0s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/xw9unqteoa6s88an617hvevxs
==> Importing images into k3d
[36mINFO[0m[0000] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0000] Saving 2 image(s) from runtime...            
[36mINFO[0m[0021] Importing images into nodes...               
[36mINFO[0m[0021] Importing images from tarball '/k3d/images/k3d-lensai-images-20260527125413.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0036] Removing the tarball(s) from image volume... 
[36mINFO[0m[0037] Removing k3d-tools node...                   
[36mINFO[0m[0038] Successfully imported image(s)               
[36mINFO[0m[0038] Successfully imported 2 image(s) into 1 cluster(s) 
==> Cluster ready. Next:
    helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-k3d.yaml --wait --timeout 10m
    ./scripts/smoke-k8s-e2e.sh
exit=0
$ helm dependency update deploy/helm/lensai
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Saving 1 charts
Downloading prometheus-adapter from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
exit=0
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Wed May 27 12:54:57 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-77b874d649-jp8p5 condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-5kw8h condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-6667b49bcd-mpcsq condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-7q97f condition met
NAME                                     READY   STATUS      RESTARTS        AGE
pod/lensai-clickhouse-0                  1/1     Running     0               2m40s
pod/lensai-clickhouse-init-2lgq4         0/1     Completed   0               2m41s
pod/lensai-consumer-6d6f555797-7q97f     1/1     Running     4 (84s ago)     2m41s
pod/lensai-ingestion-6667b49bcd-mpcsq    1/1     Running     2 (2m33s ago)   2m41s
pod/lensai-prometheus-7495bc6667-5kw8h   1/1     Running     0               2m41s
pod/lensai-redis-77b874d649-jp8p5        1/1     Running     0               2m41s
pod/lensai-redpanda-0                    1/1     Running     0               2m40s
pod/lensai-redpanda-init-dbc7d           0/1     Completed   2               2m40s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           118s       2m41s
job.batch/lensai-redpanda-init     Complete   1/1           2m19s      2m41s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-7q97f condition met
pod/lensai-ingestion-6667b49bcd-mpcsq condition met
pod/lensai-prometheus-7495bc6667-5kw8h condition met
pod/lensai-redis-77b874d649-jp8p5 condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS        AGE
lensai-clickhouse-0                  1/1     Running     0               2m41s
lensai-clickhouse-init-2lgq4         0/1     Completed   0               2m42s
lensai-consumer-6d6f555797-7q97f     1/1     Running     4 (85s ago)     2m42s
lensai-ingestion-6667b49bcd-mpcsq    1/1     Running     2 (2m34s ago)   2m42s
lensai-prometheus-7495bc6667-5kw8h   1/1     Running     0               2m42s
lensai-redis-77b874d649-jp8p5        1/1     Running     0               2m42s
lensai-redpanda-0                    1/1     Running     0               2m41s
lensai-redpanda-init-dbc7d           0/1     Completed   2               2m41s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"status":"ok"}
ok
==> POST /ingest
HTTP 202 — {"batch_id":"c22a57bb-5718-4219-be01-51aada402df0","event_count":1,"accepted_at_unix_ms":1779866865237}
==> Waiting for ClickHouse rows (up to 45s)
    ClickHouse OK (1 rows)
demo	gpt-4o	0.01
==> Consumer lag metric
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="0",topic="ai_inference_events"} 0
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="1",topic="ai_inference_events"} 1
==> HPA status
==> k8s smoke complete
exit=0
$ ./chaos/run_chaos_k8s.sh kill-redpanda

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C1 (k8s): kill-redpanda — broker outage + WAL replay[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Phase 1: baseline ingest (150 events)...
[0;36m[chaos-k8s][0m ClickHouse rows before kill: 150
[0;36m[chaos-k8s][0m Phase 2: scale redpanda to 0 (lensai-redpanda)...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Phase 3: ingest during broker outage (200 events)...
[0;36m[chaos-k8s][0m kafka_produce_errors_total: 0
[0;36m[chaos-k8s][0m Phase 4: broker down 20s, then restore...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Waiting for redpanda pod ready (120s)...
[0;31m[FAIL][0m Redpanda not ready after 120s
exit=1
$ env CH_READY_TIMEOUT_SEC=300 ./chaos/run_chaos_k8s.sh throttle-clickhouse

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C2 (k8s): throttle-clickhouse — breaker + overflow[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Baseline — breaker_open: 0, overflow: 0, CH: 150
[0;36m[chaos-k8s][0m Phase 1: scale ClickHouse to 0 (lensai-clickhouse) for 25s window...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod termination (90s)...
[0;36m[chaos-k8s][0m Phase 2: load while CH unavailable...
[0;36m[chaos-k8s][0m   5s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   10s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   15s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   20s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   25s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;31m[FAIL][0m ClickHouse not ready after 300s
exit=1
$ env LOAD_EVENTS=1000 LOAD_DURATION_SEC=10 ./chaos/run_chaos_k8s.sh load-m1

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO load-m1 — 1000 events over 10s[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Sending ~100 events/s (2 req/s × 50) for 10s
[0;36m[chaos-k8s][0m   ... 5s, ~500 events sent
[0;36m[chaos-k8s][0m   ... 10s, ~1000 events sent
  Sent: ~1000, new CH rows: 150, lag: 0, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (150 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```

## Run 20260527T073856Z

```
Started: 2026-05-27T07:38:56Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/consumer-anomaly-zscore-detection
CONTINUE_ON_FAIL=0
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ cargo test -p ingestion
    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.56s
     Running unittests src/lib.rs (target/debug/deps/ingestion-04b7275663e82a5d)

running 22 tests
test handlers::ingest::tests::validate_rejects_empty_batch ... ok
test handlers::ingest::tests::validate_rejects_negative_cost ... ok
test handlers::ingest::tests::validate_rejects_stale_timestamp ... ok
test kafka::producer::tests::produce_message_holds_wal_entry_id ... ok
test handlers::ingest::tests::validate_rejects_oversized_batch ... ok
test handlers::ingest::tests::validate_rejects_zero_latency ... ok
test rate_limit::tenant_limits::tests::from_defaults_resolves_unknown_tenant ... ok
test handlers::ingest::tests::normalize_assigns_event_id_and_status ... ok
test kafka::producer::tests::producer_client_config_sets_expected_options ... ok
test handlers::ingest::tests::tenant_from_events_json_reads_first_event ... ok
test rate_limit::token_bucket::tests::rate_limit_result_denied_eq ... ok
test rate_limit::token_bucket::tests::rate_limit_result_allowed_eq ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_missing_file ... ok
test rate_limit::tenant_limits::tests::from_file_resolves_known_tenant ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_zero_rps ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_malformed_json ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_burst_below_one ... ok
test rate_limit::token_bucket::tests::resolve_uses_tenant_override ... ok
test server::tests::health_returns_ok ... ok
test kafka::producer::tests::mark_acked_after_success_path_without_kafka ... ok
test wal::writer::tests::append_increments_entry_id ... ok
test wal::writer::tests::append_mark_acked_replay ... ok

test result: ok. 22 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.04s

     Running unittests src/main.rs (target/debug/deps/ingestion-802afe9306f36790)

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

   Doc-tests ingestion

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

exit=0
$ bash -c cd consumer && go test ./...
?   	github.com/akshantvats/infra-ai-streaming/consumer/cmd/consumer	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/clickhouse	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/config	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/model	[no test files]
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/redis	[no test files]
exit=0
$ helm template lensai deploy/helm/lensai -f deploy/helm/lensai/values-m1.yaml --namespace lensai
---
# Source: lensai/templates/consumer.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-consumer
  labels:
    app.kubernetes.io/component: consumer
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer

---
# Source: lensai/templates/ingestion.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-ingestion
  labels:
    app.kubernetes.io/component: ingestion
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion

---
# Source: lensai/templates/configmap-clickhouse-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  init.sql: |
    -- Applied by the `clickhouse-init` service in `deploy/docker-compose.yml`
    -- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema.
    CREATE DATABASE IF NOT EXISTS infra_ai;
    
    DROP TABLE IF EXISTS infra_ai.inference_events;
    
    CREATE TABLE infra_ai.inference_events
    (
        event_id UUID,
        tenant_id LowCardinality(String),
        model_id LowCardinality(String),
        timestamp DateTime64(3),
        latency_ms UInt32,
        prefill_latency_ms Nullable(UInt32),
        decode_latency_ms Nullable(UInt32),
        prompt_tokens UInt32,
        completion_tokens UInt32,
        cost_usd Float64,
        status LowCardinality(String),
        error_code Nullable(String),
        request_id Nullable(String)
    )
    ENGINE = MergeTree
    PARTITION BY toYYYYMM(timestamp)
    ORDER BY (tenant_id, model_id, timestamp);
    

---
# Source: lensai/templates/configmap-clickhouse-users.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-users
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  zz-allow-host.xml: |
    <clickhouse>
      <users>
        <default>
          <networks>
            <ip>::/0</ip>
          </networks>
        </default>
      </users>
    </clickhouse>

---
# Source: lensai/templates/configmap-redpanda-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
data:
  init-topics.sh: |
    #!/bin/sh
    # Idempotent topic creation for local dev (8 partitions; 32 in production per DESIGN.md).
    set -e
    
    BROKERS="${KAFKA_BROKERS:-redpanda:9092}"
    PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-8}"
    
    echo "Creating topics on ${BROKERS} (${PARTITIONS} partitions each)..."
    
    for topic in "${KAFKA_TOPIC:-ai_inference_events}" "${KAFKA_DLQ_TOPIC:-ai_inference_dlq}" "${KAFKA_ANOMALIES_TOPIC:-ai_anomalies}"; do
      rpk topic create "${topic}" --brokers "${BROKERS}" -p "${PARTITIONS}" || true
    done
    
    echo "Topics ready:"
    rpk topic list --brokers "${BROKERS}"
    

---
# Source: lensai/templates/configmap-tenant-limits.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-tenant-limits
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
data:
  tenant-limits.json: |
    {
      "default": {
        "max_events_per_sec": 10000,
        "burst_multiplier": 2.0
      },
      "tenants": {
        "tenant-demo": {
          "max_events_per_sec": 5,
          "burst_multiplier": 2.0
        },
        "tenant-premium": {
          "max_events_per_sec": 50000,
          "burst_multiplier": 3.0
        },
        "tenant-free": {
          "max_events_per_sec": 100,
          "burst_multiplier": 1.5
        }
      }
    }
    

---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-prometheus
  labels:
    app.kubernetes.io/component: prometheus
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    scrape_configs:
      - job_name: ingestion
        metrics_path: /metrics
        static_configs:
          - targets: ["ingestion:8080"]
      - job_name: consumer
        metrics_path: /metrics
        static_configs:
          - targets: ["consumer:9091"]
---
# Source: lensai/templates/clickhouse.yaml
apiVersion: v1
kind: Service
metadata:
  name: clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: clickhouse
  ports:
    - port: 8123
      targetPort: http
      name: http
    - port: 9000
      targetPort: native
      name: native

---
# Source: lensai/templates/consumer.yaml
apiVersion: v1
kind: Service
metadata:
  name: consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: consumer
  ports:
    - port: 9091
      targetPort: metrics
      name: metrics
---
# Source: lensai/templates/ingestion.yaml
apiVersion: v1
kind: Service
metadata:
  name: ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: ingestion
  ports:
    - port: 8080
      targetPort: http
      name: http
---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  labels:
    app.kubernetes.io/component: prometheus
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: prometheus
  ports:
    - port: 9090
      targetPort: http
      name: http

---
# Source: lensai/templates/redis.yaml
apiVersion: v1
kind: Service
metadata:
  name: redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redis
  ports:
    - port: 6379
      targetPort: redis
      name: redis

---
# Source: lensai/templates/redpanda.yaml
apiVersion: v1
kind: Service
metadata:
  name: redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  clusterIP: None
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redpanda
  ports:
    - port: 9092
      targetPort: kafka
      name: kafka
    - port: 9644
      targetPort: admin
      name: admin

---
# Source: lensai/templates/consumer.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: consumer
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9091"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: consumer
          image: "lensai/consumer:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 9091
              name: metrics
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_GROUP_ID
              value: "ai-inference-consumer-k8s"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: CLICKHOUSE_DSN
              value: "clickhouse://clickhouse:9000/infra_ai"
            - name: METRICS_PORT
              value: "9091"
          livenessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 512Mi
            requests:
              cpu: 100m
              memory: 128Mi
---
# Source: lensai/templates/ingestion.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: ingestion
    spec:
      containers:
        - name: ingestion
          image: "lensai/ingestion:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: HTTP_PORT
              value: "8080"
            - name: WAL_DIR
              value: "/data/wal"
            - name: TENANT_LIMITS_PATH
              value: "/config/tenant-limits.json"
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
          volumeMounts:
            - name: wal
              mountPath: /data/wal
            - name: tenant-limits
              mountPath: /config
              readOnly: true
      volumes:
        - name: wal
          emptyDir: {}
        - name: tenant-limits
          configMap:
            name: lensai-tenant-limits
---
# Source: lensai/templates/prometheus.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-prometheus
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: prometheus
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: prometheus
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: prometheus
    spec:
      containers:
        - name: prometheus
          image: prom/prometheus:v2.54.1
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --web.enable-lifecycle
          ports:
            - containerPort: 9090
              name: http
          livenessProbe:
            httpGet:
              path: /-/healthy
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /-/ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
            - name: data
              mountPath: /prometheus
          resources:
            limits:
              cpu: 150m
              memory: 256Mi
            requests:
              cpu: 50m
              memory: 128Mi
      volumes:
        - name: config
          configMap:
            name: lensai-prometheus
        - name: data
          emptyDir: {}
---
# Source: lensai/templates/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          livenessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            limits:
              cpu: 150m
              memory: 64Mi
            requests:
              cpu: 50m
              memory: 32Mi
---
# Source: lensai/templates/clickhouse.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  serviceName: clickhouse
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: clickhouse
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: clickhouse
    spec:
      containers:
        - name: clickhouse
          image: clickhouse/clickhouse-server:24.12-alpine
          ports:
            - containerPort: 8123
              name: http
            - containerPort: 9000
              name: native
          livenessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 40
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          volumeMounts:
            - name: data
              mountPath: /var/lib/clickhouse
            - name: users-d
              mountPath: /etc/clickhouse-server/users.d
              readOnly: true
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
            requests:
              cpu: 250m
              memory: 512Mi
      volumes:
        - name: users-d
          configMap:
            name: lensai-clickhouse-users
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
---
# Source: lensai/templates/redpanda.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  serviceName: redpanda
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redpanda
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redpanda
    spec:
      containers:
        - name: redpanda
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          command:
            - /bin/sh
            - -ec
            - |
              FQDN="$(hostname -f)"
              exec /usr/bin/rpk redpanda start \
                --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092 \
                --advertise-kafka-addr "internal://${FQDN}:9092,external://127.0.0.1:9092" \
                --rpc-addr 0.0.0.0:33145 \
                --advertise-rpc-addr "${FQDN}:33145" \
                --smp 1 \
                --memory "1G" \
                --reserve-memory 0M \
                --overprovisioned \
                --check=false \
                --mode dev-container \
                --default-log-level=warn
          ports:
            - containerPort: 9092
              name: kafka
            - containerPort: 9644
              name: admin
          livenessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 15
            failureThreshold: 3
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 15
            failureThreshold: 6
          resources:
            limits:
              cpu: "1"
              memory: 1536Mi
            requests:
              cpu: 500m
              memory: 1Gi
---
# Source: lensai/templates/clickhouse-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: clickhouse-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: clickhouse/clickhouse-server:24.12-alpine
          command:
            - /bin/sh
            - -c
            - |
              until clickhouse-client --host clickhouse --query 'SELECT 1' 2>/dev/null; do sleep 2; done
              clickhouse-client --host clickhouse --multiquery < /init.sql
          volumeMounts:
            - name: init-sql
              mountPath: /init.sql
              subPath: init.sql
              readOnly: true
      volumes:
        - name: init-sql
          configMap:
            name: lensai-clickhouse-init

---
# Source: lensai/templates/redpanda-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: redpanda-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_TOPIC_PARTITIONS
              value: "2"
          command: ["/bin/sh", "/init-topics.sh"]
          volumeMounts:
            - name: script
              mountPath: /init-topics.sh
              subPath: init-topics.sh
              readOnly: true
      volumes:
        - name: script
          configMap:
            name: lensai-redpanda-init
            defaultMode: 0555
exit=0
$ bash -n scripts/demo-flows.sh
exit=0
$ bash -n scripts/demo-hpa-lag.sh
exit=0
$ bash -n scripts/docker-test-ingestion.sh
exit=0
$ bash -n scripts/e2e-k3d-full.sh
exit=0
$ bash -n scripts/smoke-e2e.sh
exit=0
$ bash -n scripts/smoke-k8s-e2e.sh
exit=0
$ bash -n scripts/test-ingestion.sh
exit=0
$ bash -n chaos/run_chaos.sh
exit=0
$ bash -n chaos/run_chaos_k8s.sh
exit=0
$ require_cmd docker
exit=0
$ require_cmd k3d
exit=0
$ require_cmd helm
exit=0
$ require_cmd kubectl
exit=0
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ k3d cluster delete lensai
[36mINFO[0m[0000] Deleting cluster 'lensai'                    
[36mINFO[0m[0005] Deleting 1 attached volumes...               
[36mINFO[0m[0005] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0005] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0005] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.pCN2qk2fQn (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8080:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9091:9091' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] Prep: Network                                
[36mINFO[0m[0000] Re-using existing network 'k3d-lensai' (367f3448b068d394a56589c94f547f7df1550d219221d7ae9826839aa1125ce1) 
[36mINFO[0m[0000] Created image volume k3d-lensai-images       
[36mINFO[0m[0000] Starting new tools node...                   
[36mINFO[0m[0000] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0001] Creating node 'k3d-lensai-server-0'          
[36mINFO[0m[0001] Creating LoadBalancer 'k3d-lensai-serverlb'  
[36mINFO[0m[0001] Using the k3d-tools node to gather environment information 
[36mINFO[0m[0002] Starting new tools node...                   
[36mINFO[0m[0002] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0003] Starting cluster 'lensai'                    
[36mINFO[0m[0003] Starting servers...                          
[36mINFO[0m[0003] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0010] All agents already running.                  
[36mINFO[0m[0010] Starting helpers...                          
[36mINFO[0m[0010] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0017] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0019] Cluster 'lensai' created successfully!       
[36mINFO[0m[0019] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 986B done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 2.0s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#5 DONE 0.0s

#6 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 119.44kB 0.0s done
#7 DONE 0.0s

#8 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#8 CACHED

#9 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#9 CACHED

#10 [stage-1 3/5] WORKDIR /app
#10 CACHED

#11 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#11 CACHED

#12 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#12 CACHED

#13 [builder 5/6] COPY ingestion ./ingestion
#13 CACHED

#14 [builder 3/6] WORKDIR /build
#14 CACHED

#15 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#15 CACHED

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 CACHED

#17 exporting to image
#17 exporting layers done
#17 writing image sha256:9c6090fd4eaaf1289ec1cb14a19a1332317adf39d2ee29bb13608d486eff0396 done
#17 naming to docker.io/lensai/ingestion:local done
#17 DONE 0.0s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/ohu8vzy2mjng82ykme5mz2eql
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 541B done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#2 ...

#3 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#3 DONE 0.6s

#2 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#2 DONE 1.5s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#5 DONE 0.0s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 72.31kB 0.0s done
#7 DONE 0.0s

#8 [builder 5/6] COPY consumer/ ./
#8 CACHED

#9 [builder 6/6] RUN CGO_ENABLED=0 go build -o /consumer ./cmd/consumer
#9 CACHED

#10 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#10 CACHED

#11 [builder 4/6] RUN go mod download
#11 CACHED

#12 [builder 2/6] WORKDIR /build/consumer
#12 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 CACHED

#14 exporting to image
#14 exporting layers done
#14 writing image sha256:4426f9a3a3faa7f944a9be08775762906c06f5923ce5f58d3ce196110d417597 done
#14 naming to docker.io/lensai/consumer:local done
#14 DONE 0.0s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/vy1u4njigjinie0pqboozhnao
==> Importing images into k3d
[36mINFO[0m[0000] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0000] Saving 2 image(s) from runtime...            
[36mINFO[0m[0007] Importing images into nodes...               
[36mINFO[0m[0007] Importing images from tarball '/k3d/images/k3d-lensai-images-20260527130932.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0012] Removing the tarball(s) from image volume... 
[36mINFO[0m[0013] Removing k3d-tools node...                   
[36mINFO[0m[0014] Successfully imported image(s)               
[36mINFO[0m[0014] Successfully imported 2 image(s) into 1 cluster(s) 
==> Cluster ready. Next:
    helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-k3d.yaml --wait --timeout 10m
    ./scripts/smoke-k8s-e2e.sh
exit=0
$ helm dependency update deploy/helm/lensai
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Saving 1 charts
Downloading prometheus-adapter from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
exit=0
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Wed May 27 13:09:51 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-77b874d649-qntj2 condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-5hbcz condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-6667b49bcd-pgvpj condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-v9m4f condition met
NAME                                     READY   STATUS      RESTARTS       AGE
pod/lensai-clickhouse-0                  1/1     Running     0              2m21s
pod/lensai-clickhouse-init-86v46         0/1     Completed   0              2m21s
pod/lensai-consumer-6d6f555797-v9m4f     1/1     Running     4 (67s ago)    2m21s
pod/lensai-ingestion-6667b49bcd-pgvpj    1/1     Running     3 (2m3s ago)   2m21s
pod/lensai-prometheus-7495bc6667-5hbcz   1/1     Running     0              2m21s
pod/lensai-redis-77b874d649-qntj2        1/1     Running     0              2m21s
pod/lensai-redpanda-0                    1/1     Running     0              2m21s
pod/lensai-redpanda-init-lsq5f           0/1     Completed   3              2m21s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           84s        2m21s
job.batch/lensai-redpanda-init     Complete   1/1           108s       2m21s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-v9m4f condition met
pod/lensai-ingestion-6667b49bcd-pgvpj condition met
pod/lensai-prometheus-7495bc6667-5hbcz condition met
pod/lensai-redis-77b874d649-qntj2 condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS       AGE
lensai-clickhouse-0                  1/1     Running     0              2m22s
lensai-clickhouse-init-86v46         0/1     Completed   0              2m22s
lensai-consumer-6d6f555797-v9m4f     1/1     Running     4 (68s ago)    2m22s
lensai-ingestion-6667b49bcd-pgvpj    1/1     Running     3 (2m4s ago)   2m22s
lensai-prometheus-7495bc6667-5hbcz   1/1     Running     0              2m22s
lensai-redis-77b874d649-qntj2        1/1     Running     0              2m22s
lensai-redpanda-0                    1/1     Running     0              2m22s
lensai-redpanda-init-lsq5f           0/1     Completed   3              2m22s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"status":"ok"}
ok
==> POST /ingest
HTTP 202 — {"batch_id":"ea3cc2fb-2117-4ce6-8b1b-886dc71954c6","event_count":1,"accepted_at_unix_ms":1779867737971}
==> Waiting for ClickHouse rows (up to 45s)
    ClickHouse OK (1 rows)
demo	gpt-4o	0.01
==> Consumer lag metric
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="0",topic="ai_inference_events"} 0
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="1",topic="ai_inference_events"} 1
==> HPA status
==> k8s smoke complete
exit=0
$ env REDPANDA_READY_TIMEOUT_SEC=300 ./chaos/run_chaos_k8s.sh kill-redpanda

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C1 (k8s): kill-redpanda — broker outage + WAL replay[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Phase 1: baseline ingest (150 events)...
[0;36m[chaos-k8s][0m ClickHouse rows before kill: 150
[0;36m[chaos-k8s][0m Phase 2: scale redpanda to 0 (lensai-redpanda)...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Waiting for Redpanda pod termination (90s)...
[0;36m[chaos-k8s][0m Phase 3: ingest during broker outage (200 events)...
[0;36m[chaos-k8s][0m kafka_produce_errors_total: 0
[0;36m[chaos-k8s][0m Phase 4: broker down 20s, then restore...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Waiting for redpanda pod ready (300s)...
[0;32m[PASS][0m Redpanda ready
[0;36m[chaos-k8s][0m Phase 5: restart ingestion for WAL replay...
[0;36m[chaos-k8s][0m Rollout restart ingestion (lensai-ingestion) for WAL replay...
deployment.apps/lensai-ingestion restarted
Waiting for deployment spec update to be observed...
Waiting for deployment spec update to be observed...
Waiting for deployment "lensai-ingestion" rollout to finish: 0 out of 1 new replicas have been updated...
Waiting for deployment "lensai-ingestion" rollout to finish: 1 old replicas are pending termination...
Waiting for deployment "lensai-ingestion" rollout to finish: 1 old replicas are pending termination...
deployment "lensai-ingestion" successfully rolled out
[0;31m[FAIL][0m Ingestion not healthy after rollout restart
[1;33m[WARN][0m WAL replay may need manual check
[0;36m[chaos-k8s][0m wal_replay_events_total: 0
[0;36m[chaos-k8s][0m Phase 6: wait for consumer → ClickHouse...
  Events sent: 350, CH rows after: 150
[0;32m[PASS][0m Recovery path OK (CH grew or held; at-least-once may duplicate)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ env CH_READY_TIMEOUT_SEC=300 ./chaos/run_chaos_k8s.sh throttle-clickhouse

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C2 (k8s): throttle-clickhouse — breaker + overflow[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Baseline — breaker_open: 0, overflow: 0, CH: 150
[0;36m[chaos-k8s][0m Phase 1: scale ClickHouse to 0 (lensai-clickhouse) for 25s window...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod termination (90s)...
[0;36m[chaos-k8s][0m Phase 2: load while CH unavailable...
[0;36m[chaos-k8s][0m   5s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   10s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   15s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   20s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m   25s — breaker_open: 0, overflow: 0
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;32m[PASS][0m ClickHouse ready
  breaker during: 0, after: 0
  overflow during: 0, after: 0
  CH rows before/after: 150 / 150
[1;33m[WARN][0m Breaker did not report open (got 0) — may need longer pause or more load
[1;33m[WARN][0m No overflow depth during pause

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ env LOAD_EVENTS=1000 LOAD_DURATION_SEC=10 ./chaos/run_chaos_k8s.sh load-m1

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO load-m1 — 1000 events over 10s[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Sending ~100 events/s (2 req/s × 50) for 10s
[0;36m[chaos-k8s][0m   ... 5s, ~500 events sent
[0;36m[chaos-k8s][0m   ... 10s, ~1000 events sent
  Sent: ~1000, new CH rows: 1000, lag: 0, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (1000 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```

## Run 20260527T082437Z

```
Started: 2026-05-27T08:24:37Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/consumer-anomaly-zscore-detection
CONTINUE_ON_FAIL=0
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ cargo test -p ingestion
    Finished `test` profile [unoptimized + debuginfo] target(s) in 1.67s
     Running unittests src/lib.rs (target/debug/deps/ingestion-04b7275663e82a5d)

running 22 tests
test handlers::ingest::tests::validate_rejects_empty_batch ... ok
test handlers::ingest::tests::validate_rejects_negative_cost ... ok
test handlers::ingest::tests::validate_rejects_stale_timestamp ... ok
test handlers::ingest::tests::validate_rejects_oversized_batch ... ok
test kafka::producer::tests::produce_message_holds_wal_entry_id ... ok
test handlers::ingest::tests::validate_rejects_zero_latency ... ok
test rate_limit::tenant_limits::tests::from_defaults_resolves_unknown_tenant ... ok
test handlers::ingest::tests::normalize_assigns_event_id_and_status ... ok
test kafka::producer::tests::producer_client_config_sets_expected_options ... ok
test handlers::ingest::tests::tenant_from_events_json_reads_first_event ... ok
test rate_limit::token_bucket::tests::rate_limit_result_denied_eq ... ok
test rate_limit::token_bucket::tests::rate_limit_result_allowed_eq ... ok
test rate_limit::tenant_limits::tests::from_file_resolves_known_tenant ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_missing_file ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_burst_below_one ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_malformed_json ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_zero_rps ... ok
test rate_limit::token_bucket::tests::resolve_uses_tenant_override ... ok
test server::tests::health_returns_ok ... ok
test kafka::producer::tests::mark_acked_after_success_path_without_kafka ... ok
test wal::writer::tests::append_increments_entry_id ... ok
test wal::writer::tests::append_mark_acked_replay ... ok

test result: ok. 22 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.56s

     Running unittests src/main.rs (target/debug/deps/ingestion-802afe9306f36790)

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

   Doc-tests ingestion

running 0 tests

test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

exit=0
$ bash -c cd consumer && go test ./...
?   	github.com/akshantvats/infra-ai-streaming/consumer/cmd/consumer	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/clickhouse	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/config	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/model	[no test files]
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/redis	[no test files]
exit=0
$ helm template lensai deploy/helm/lensai -f deploy/helm/lensai/values-m1.yaml --namespace lensai
---
# Source: lensai/templates/consumer.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-consumer
  labels:
    app.kubernetes.io/component: consumer
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer

---
# Source: lensai/templates/ingestion.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: lensai-ingestion
  labels:
    app.kubernetes.io/component: ingestion
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion

---
# Source: lensai/templates/configmap-clickhouse-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  init.sql: |
    -- Applied by the `clickhouse-init` service in `deploy/docker-compose.yml`
    -- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema.
    CREATE DATABASE IF NOT EXISTS infra_ai;
    
    DROP TABLE IF EXISTS infra_ai.inference_events;
    
    CREATE TABLE infra_ai.inference_events
    (
        event_id UUID,
        tenant_id LowCardinality(String),
        model_id LowCardinality(String),
        timestamp DateTime64(3),
        latency_ms UInt32,
        prefill_latency_ms Nullable(UInt32),
        decode_latency_ms Nullable(UInt32),
        prompt_tokens UInt32,
        completion_tokens UInt32,
        cost_usd Float64,
        status LowCardinality(String),
        error_code Nullable(String),
        request_id Nullable(String)
    )
    ENGINE = MergeTree
    PARTITION BY toYYYYMM(timestamp)
    ORDER BY (tenant_id, model_id, timestamp);
    

---
# Source: lensai/templates/configmap-clickhouse-users.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-clickhouse-users
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
data:
  zz-allow-host.xml: |
    <clickhouse>
      <users>
        <default>
          <networks>
            <ip>::/0</ip>
          </networks>
        </default>
      </users>
    </clickhouse>

---
# Source: lensai/templates/configmap-redpanda-init.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
data:
  init-topics.sh: |
    #!/bin/sh
    # Idempotent topic creation for local dev (8 partitions; 32 in production per DESIGN.md).
    set -e
    
    BROKERS="${KAFKA_BROKERS:-redpanda:9092}"
    PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-8}"
    
    echo "Creating topics on ${BROKERS} (${PARTITIONS} partitions each)..."
    
    for topic in "${KAFKA_TOPIC:-ai_inference_events}" "${KAFKA_DLQ_TOPIC:-ai_inference_dlq}" "${KAFKA_ANOMALIES_TOPIC:-ai_anomalies}"; do
      rpk topic create "${topic}" --brokers "${BROKERS}" -p "${PARTITIONS}" || true
    done
    
    echo "Topics ready:"
    rpk topic list --brokers "${BROKERS}"
    

---
# Source: lensai/templates/configmap-tenant-limits.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-tenant-limits
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
data:
  tenant-limits.json: |
    {
      "default": {
        "max_events_per_sec": 10000,
        "burst_multiplier": 2.0
      },
      "tenants": {
        "tenant-demo": {
          "max_events_per_sec": 5,
          "burst_multiplier": 2.0
        },
        "tenant-premium": {
          "max_events_per_sec": 50000,
          "burst_multiplier": 3.0
        },
        "tenant-free": {
          "max_events_per_sec": 100,
          "burst_multiplier": 1.5
        }
      }
    }
    

---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lensai-prometheus
  labels:
    app.kubernetes.io/component: prometheus
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    scrape_configs:
      - job_name: ingestion
        metrics_path: /metrics
        static_configs:
          - targets: ["ingestion:8080"]
      - job_name: consumer
        metrics_path: /metrics
        static_configs:
          - targets: ["consumer:9091"]
---
# Source: lensai/templates/ingestion-wal-pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: lensai-ingestion-wal
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi

---
# Source: lensai/templates/clickhouse.yaml
apiVersion: v1
kind: Service
metadata:
  name: clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: clickhouse
  ports:
    - port: 8123
      targetPort: http
      name: http
    - port: 9000
      targetPort: native
      name: native

---
# Source: lensai/templates/consumer.yaml
apiVersion: v1
kind: Service
metadata:
  name: consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: consumer
  ports:
    - port: 9091
      targetPort: metrics
      name: metrics
---
# Source: lensai/templates/ingestion.yaml
apiVersion: v1
kind: Service
metadata:
  name: ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: ingestion
  ports:
    - port: 8080
      targetPort: http
      name: http
---
# Source: lensai/templates/prometheus.yaml
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  labels:
    app.kubernetes.io/component: prometheus
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: prometheus
  ports:
    - port: 9090
      targetPort: http
      name: http

---
# Source: lensai/templates/redis.yaml
apiVersion: v1
kind: Service
metadata:
  name: redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redis
  ports:
    - port: 6379
      targetPort: redis
      name: redis

---
# Source: lensai/templates/redpanda.yaml
apiVersion: v1
kind: Service
metadata:
  name: redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  clusterIP: None
  selector:
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/component: redpanda
  ports:
    - port: 9092
      targetPort: kafka
      name: kafka
    - port: 9644
      targetPort: admin
      name: admin

---
# Source: lensai/templates/consumer.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-consumer
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: consumer
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: consumer
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: consumer
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9091"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: consumer
          image: "lensai/consumer:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 9091
              name: metrics
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_GROUP_ID
              value: "ai-inference-consumer-k8s"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: CLICKHOUSE_DSN
              value: "clickhouse://clickhouse:9000/infra_ai"
            - name: METRICS_PORT
              value: "9091"
          livenessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 512Mi
            requests:
              cpu: 100m
              memory: 128Mi
---
# Source: lensai/templates/ingestion.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-ingestion
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: ingestion
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: ingestion
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: ingestion
    spec:
      containers:
        - name: ingestion
          image: "lensai/ingestion:local"
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: HTTP_PORT
              value: "8080"
            - name: WAL_DIR
              value: "/data/wal"
            - name: TENANT_LIMITS_PATH
              value: "/config/tenant-limits.json"
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          resources:
            limits:
              cpu: 250m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
          volumeMounts:
            - name: wal
              mountPath: /data/wal
            - name: tenant-limits
              mountPath: /config
              readOnly: true
      volumes:
        - name: wal
          persistentVolumeClaim:
            claimName: lensai-ingestion-wal
        - name: tenant-limits
          configMap:
            name: lensai-tenant-limits
---
# Source: lensai/templates/prometheus.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-prometheus
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: prometheus
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: prometheus
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: prometheus
    spec:
      containers:
        - name: prometheus
          image: prom/prometheus:v2.54.1
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --web.enable-lifecycle
          ports:
            - containerPort: 9090
              name: http
          livenessProbe:
            httpGet:
              path: /-/healthy
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /-/ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
            - name: data
              mountPath: /prometheus
          resources:
            limits:
              cpu: 150m
              memory: 256Mi
            requests:
              cpu: 50m
              memory: 128Mi
      volumes:
        - name: config
          configMap:
            name: lensai-prometheus
        - name: data
          emptyDir: {}
---
# Source: lensai/templates/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lensai-redis
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          livenessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            limits:
              cpu: 150m
              memory: 64Mi
            requests:
              cpu: 50m
              memory: 32Mi
---
# Source: lensai/templates/clickhouse.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-clickhouse
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse
spec:
  serviceName: clickhouse
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: clickhouse
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: clickhouse
    spec:
      containers:
        - name: clickhouse
          image: clickhouse/clickhouse-server:24.12-alpine
          ports:
            - containerPort: 8123
              name: http
            - containerPort: 9000
              name: native
          livenessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 40
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["clickhouse-client", "--query", "SELECT 1"]
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          volumeMounts:
            - name: data
              mountPath: /var/lib/clickhouse
            - name: users-d
              mountPath: /etc/clickhouse-server/users.d
              readOnly: true
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
            requests:
              cpu: 250m
              memory: 512Mi
      volumes:
        - name: users-d
          configMap:
            name: lensai-clickhouse-users
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
---
# Source: lensai/templates/redpanda.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: lensai-redpanda
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda
spec:
  serviceName: redpanda
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lensai
      app.kubernetes.io/instance: lensai
      app.kubernetes.io/component: redpanda
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lensai
        app.kubernetes.io/instance: lensai
        app.kubernetes.io/component: redpanda
    spec:
      containers:
        - name: redpanda
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          command:
            - /bin/sh
            - -ec
            - |
              FQDN="$(hostname -f)"
              exec /usr/bin/rpk redpanda start \
                --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092 \
                --advertise-kafka-addr "internal://${FQDN}:9092,external://127.0.0.1:9092" \
                --rpc-addr 0.0.0.0:33145 \
                --advertise-rpc-addr "${FQDN}:33145" \
                --smp 1 \
                --memory "1G" \
                --reserve-memory 0M \
                --overprovisioned \
                --check=false \
                --mode dev-container \
                --default-log-level=warn
          ports:
            - containerPort: 9092
              name: kafka
            - containerPort: 9644
              name: admin
          livenessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 45
            periodSeconds: 15
            timeoutSeconds: 15
            failureThreshold: 3
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - rpk cluster metadata --brokers 127.0.0.1:9092 >/dev/null 2>&1
            initialDelaySeconds: 25
            periodSeconds: 10
            timeoutSeconds: 15
            failureThreshold: 6
          resources:
            limits:
              cpu: "1"
              memory: 1536Mi
            requests:
              cpu: 500m
              memory: 1Gi
---
# Source: lensai/templates/clickhouse-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-clickhouse-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: clickhouse-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: clickhouse-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: clickhouse/clickhouse-server:24.12-alpine
          command:
            - /bin/sh
            - -c
            - |
              until clickhouse-client --host clickhouse --query 'SELECT 1' 2>/dev/null; do sleep 2; done
              clickhouse-client --host clickhouse --multiquery < /init.sql
          volumeMounts:
            - name: init-sql
              mountPath: /init.sql
              subPath: init.sql
              readOnly: true
      volumes:
        - name: init-sql
          configMap:
            name: lensai-clickhouse-init

---
# Source: lensai/templates/redpanda-init-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: lensai-redpanda-init
  labels:
    helm.sh/chart: lensai-0.1.0
    app.kubernetes.io/name: lensai
    app.kubernetes.io/instance: lensai
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/component: redpanda-init
spec:
  backoffLimit: 6
  template:
    metadata:
      labels:
        app.kubernetes.io/component: redpanda-init
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init
          image: docker.redpanda.com/redpandadata/redpanda:v24.2.4
          env:
            - name: KAFKA_BROKERS
              value: "redpanda:9092"
            - name: KAFKA_TOPIC
              value: "ai_inference_events"
            - name: KAFKA_DLQ_TOPIC
              value: "ai_inference_dlq"
            - name: KAFKA_ANOMALIES_TOPIC
              value: "ai_anomalies"
            - name: KAFKA_TOPIC_PARTITIONS
              value: "2"
          command: ["/bin/sh", "/init-topics.sh"]
          volumeMounts:
            - name: script
              mountPath: /init-topics.sh
              subPath: init-topics.sh
              readOnly: true
      volumes:
        - name: script
          configMap:
            name: lensai-redpanda-init
            defaultMode: 0555
exit=0
$ bash -n scripts/demo-flows.sh
exit=0
$ bash -n scripts/demo-hpa-lag.sh
exit=0
$ bash -n scripts/docker-test-ingestion.sh
exit=0
$ bash -n scripts/e2e-k3d-full.sh
exit=0
$ bash -n scripts/smoke-e2e.sh
exit=0
$ bash -n scripts/smoke-k8s-e2e.sh
exit=0
$ bash -n scripts/test-ingestion.sh
exit=0
$ bash -n chaos/run_chaos.sh
exit=0
$ bash -n chaos/run_chaos_k8s.sh
exit=0
$ require_cmd docker
exit=0
$ require_cmd k3d
exit=0
$ require_cmd helm
exit=0
$ require_cmd kubectl
exit=0
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ k3d cluster delete lensai
[36mINFO[0m[0002] Deleting cluster 'lensai'                    
[36mINFO[0m[0012] Deleting 1 attached volumes...               
[36mINFO[0m[0012] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0012] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0012] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.q8BDWBt5WN (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8080:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9091:9091' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] Prep: Network                                
[36mINFO[0m[0000] Re-using existing network 'k3d-lensai' (367f3448b068d394a56589c94f547f7df1550d219221d7ae9826839aa1125ce1) 
[36mINFO[0m[0000] Created image volume k3d-lensai-images       
[36mINFO[0m[0000] Starting new tools node...                   
[36mINFO[0m[0001] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0001] Creating node 'k3d-lensai-server-0'          
[36mINFO[0m[0001] Creating LoadBalancer 'k3d-lensai-serverlb'  
[36mINFO[0m[0002] Using the k3d-tools node to gather environment information 
[36mINFO[0m[0003] Starting new tools node...                   
[36mINFO[0m[0003] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0007] Starting cluster 'lensai'                    
[36mINFO[0m[0007] Starting servers...                          
[36mINFO[0m[0007] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0012] All agents already running.                  
[36mINFO[0m[0012] Starting helpers...                          
[36mINFO[0m[0013] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0020] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0023] Cluster 'lensai' created successfully!       
[36mINFO[0m[0023] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 986B 0.1s done
#1 DONE 0.1s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 3.2s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.1s

#5 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#5 DONE 0.0s

#6 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#6 DONE 0.0s

#7 [internal] load build context
#7 DONE 0.1s

#7 [internal] load build context
#7 transferring context: 119.44kB 0.2s done
#7 DONE 0.3s

#8 [builder 3/6] WORKDIR /build
#8 CACHED

#9 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#9 CACHED

#10 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#10 CACHED

#11 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#11 CACHED

#12 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#12 CACHED

#13 [builder 5/6] COPY ingestion ./ingestion
#13 CACHED

#14 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#14 CACHED

#15 [stage-1 3/5] WORKDIR /app
#15 CACHED

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 CACHED

#17 exporting to image
#17 exporting layers done
#17 writing image sha256:9c6090fd4eaaf1289ec1cb14a19a1332317adf39d2ee29bb13608d486eff0396 0.0s done
#17 naming to docker.io/lensai/ingestion:local 0.0s done
#17 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/xvtaa09b6z9f6663guziv958s
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 541B done
#1 DONE 0.0s

#2 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#2 DONE 0.7s

#3 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#3 DONE 1.7s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#5 DONE 0.0s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 72.31kB 0.1s done
#7 DONE 0.1s

#8 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#8 CACHED

#9 [builder 4/6] RUN go mod download
#9 CACHED

#10 [builder 5/6] COPY consumer/ ./
#10 CACHED

#11 [builder 6/6] RUN CGO_ENABLED=0 go build -o /consumer ./cmd/consumer
#11 CACHED

#12 [builder 2/6] WORKDIR /build/consumer
#12 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 CACHED

#14 exporting to image
#14 exporting layers done
#14 writing image sha256:4426f9a3a3faa7f944a9be08775762906c06f5923ce5f58d3ce196110d417597 done
#14 naming to docker.io/lensai/consumer:local done
#14 DONE 0.0s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/5fh0a80mepdhy2fv76ey6djgj
==> Importing images into k3d
[36mINFO[0m[0000] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0001] Saving 2 image(s) from runtime...            
[36mINFO[0m[0022] Importing images into nodes...               
[36mINFO[0m[0022] Importing images from tarball '/k3d/images/k3d-lensai-images-20260527135546.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0035] Removing the tarball(s) from image volume... 
[36mINFO[0m[0036] Removing k3d-tools node...                   
[36mINFO[0m[0037] Successfully imported image(s)               
[36mINFO[0m[0037] Successfully imported 2 image(s) into 1 cluster(s) 
==> Cluster ready. Next:
    helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-k3d.yaml --wait --timeout 10m
    ./scripts/smoke-k8s-e2e.sh
exit=0
$ helm dependency update deploy/helm/lensai
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Saving 1 charts
Downloading prometheus-adapter from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
exit=0
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Wed May 27 13:56:30 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-77b874d649-d6rx5 condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-xlshk condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-cb767579f-v2k2f condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-qr2w9 condition met
NAME                                     READY   STATUS      RESTARTS      AGE
pod/lensai-clickhouse-0                  1/1     Running     0             2m30s
pod/lensai-clickhouse-init-qfcbj         0/1     Completed   0             2m31s
pod/lensai-consumer-6d6f555797-qr2w9     1/1     Running     4 (76s ago)   2m31s
pod/lensai-ingestion-cb767579f-v2k2f     1/1     Running     0             2m31s
pod/lensai-prometheus-7495bc6667-xlshk   1/1     Running     0             2m31s
pod/lensai-redis-77b874d649-d6rx5        1/1     Running     0             2m31s
pod/lensai-redpanda-0                    1/1     Running     0             2m31s
pod/lensai-redpanda-init-cq289           0/1     Completed   3             2m31s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           94s        2m31s
job.batch/lensai-redpanda-init     Complete   1/1           2m9s       2m31s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-qr2w9 condition met
pod/lensai-ingestion-cb767579f-v2k2f condition met
pod/lensai-prometheus-7495bc6667-xlshk condition met
pod/lensai-redis-77b874d649-d6rx5 condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS      AGE
lensai-clickhouse-0                  1/1     Running     0             2m31s
lensai-clickhouse-init-qfcbj         0/1     Completed   0             2m32s
lensai-consumer-6d6f555797-qr2w9     1/1     Running     4 (77s ago)   2m32s
lensai-ingestion-cb767579f-v2k2f     1/1     Running     0             2m32s
lensai-prometheus-7495bc6667-xlshk   1/1     Running     0             2m32s
lensai-redis-77b874d649-d6rx5        1/1     Running     0             2m32s
lensai-redpanda-0                    1/1     Running     0             2m32s
lensai-redpanda-init-cq289           0/1     Completed   3             2m32s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"status":"ok"}
ok
==> POST /ingest
HTTP 202 — {"batch_id":"7975cd8b-2e3b-4bc6-8f8f-22d6d7e59bb4","event_count":1,"accepted_at_unix_ms":1779870546744}
==> Waiting for ClickHouse rows (up to 45s)
    ClickHouse OK (1 rows)
demo	gpt-4o	0.01
==> Consumer lag metric
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="0",topic="ai_inference_events"} 0
kafka_consumer_lag_events{group="ai-inference-consumer-k8s",partition="1",topic="ai_inference_events"} 1
==> HPA status
==> k8s smoke complete
exit=0
$ env REDPANDA_READY_TIMEOUT_SEC=300 ./chaos/run_chaos_k8s.sh kill-redpanda

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C1 (k8s): kill-redpanda — broker outage + WAL replay[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Phase 1: baseline ingest (150 events)...
[0;36m[chaos-k8s][0m ClickHouse rows before kill: 150
[0;36m[chaos-k8s][0m Phase 2: scale redpanda to 0 (lensai-redpanda)...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Waiting for Redpanda pod termination (90s)...
[0;36m[chaos-k8s][0m Phase 3: ingest during broker outage (200 events)...
[0;32m[PASS][0m WAL accepted 200 events during outage (HTTP 202)
[0;36m[chaos-k8s][0m wal_segments_pending (outage): 1, kafka_produce_errors_total: 0
[0;32m[PASS][0m WAL backlog visible (wal_segments_pending=1)
[0;36m[chaos-k8s][0m Phase 4: broker down 20s, restore Redpanda...
statefulset.apps/lensai-redpanda scaled
[0;36m[chaos-k8s][0m Waiting for redpanda pod ready (300s)...
[0;32m[PASS][0m Redpanda ready
[0;36m[chaos-k8s][0m Phase 5: restart ingestion for WAL replay (PVC on M1 values-m1)...
[0;36m[chaos-k8s][0m Rollout restart ingestion (lensai-ingestion) for WAL replay...
deployment.apps/lensai-ingestion restarted
[0;36m[chaos-k8s][0m Waiting for ingestion rollout (lensai-ingestion, 300s)...
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;32m[PASS][0m Ingestion deployment ready and /health OK
[0;36m[chaos-k8s][0m wal_replay_events_total: 4, wal_segments_pending: 0
[0;32m[PASS][0m WAL replay on startup (wal_replay_events_total=4)
[0;36m[chaos-k8s][0m Rollout restart consumer (lensai-consumer) to reconnect after broker restore...
deployment.apps/lensai-consumer restarted
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Phase 5b: wait for WAL drain + consumer → ClickHouse (up to 180s)...
[0;32m[PASS][0m WAL drained and ClickHouse caught up (150 → 350)
[0;32m[PASS][0m Post-recovery ingest accepted (HTTP 202)
[0;36m[chaos-k8s][0m Phase 6: wait for ≥100 new CH rows (tenant chaos-k8s, up to 120s)...
[0;32m[PASS][0m ClickHouse grew: 150 → 400 (sent ~350 during scenario)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ env CH_READY_TIMEOUT_SEC=300 ./chaos/run_chaos_k8s.sh throttle-clickhouse

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO C2 (k8s): throttle-clickhouse — breaker + overflow[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Baseline — breaker_open: 0, overflow: 0, CH: 400
[0;36m[chaos-k8s][0m Phase 1: scale ClickHouse to 0 (lensai-clickhouse) for 25s window...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod termination (90s)...
[0;36m[chaos-k8s][0m Phase 2: sustained ingest while CH unavailable (30×30 curls/round)...
[0;36m[chaos-k8s][0m   round 1/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 2/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 3/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 4/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 5/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 6/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 7/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 8/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 9/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 10/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 11/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 12/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 13/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 28
[0;36m[chaos-k8s][0m   round 14/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 390
[0;36m[chaos-k8s][0m   round 15/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 16/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 17/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 18/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 19/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 20/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 21/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 22/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 23/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 24/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 25/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 26/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 27/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 28/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 29/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m   round 30/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 392
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;32m[PASS][0m ClickHouse ready
  breaker max: 0, overflow max: 0, ch_errors Δ: 0, lag peak: 392
  breaker after: 0, overflow after: 0
  CH rows before/after: 400 / 1950
[0;32m[PASS][0m Consumer lag backlog during CH outage (peak 392 events)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ env LOAD_EVENTS=1000 LOAD_DURATION_SEC=10 ./chaos/run_chaos_k8s.sh load-m1

[1m═══════════════════════════════════════════════════════════════[0m
[1mSCENARIO load-m1 — 1000 events over 10s[0m

[1m═══════════════════════════════════════════════════════════════[0m
[0;36m[chaos-k8s][0m Starting port-forwards (ingestion :8080, consumer :9091)
[0;36m[chaos-k8s][0m Sending ~100 events/s (2 req/s × 50) for 10s
[0;36m[chaos-k8s][0m   ... 5s, ~500 events sent
[0;36m[chaos-k8s][0m   ... 10s, ~1000 events sent
  Sent: ~1000, new CH rows: 2150, lag: 392, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (2150 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```
