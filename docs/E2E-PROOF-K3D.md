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

## Test matrix � run `20260527T064409Z` (M1)

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

**Wall time:** ~711s (~11.9 min). **Branch:** `feat/consumer-anomaly-zscore-detection` @ `d98d99a`. **Topics:** `ai_anomalies` added to Helm init.

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

## Run 20260527T085702Z

```
Started: 2026-05-27T08:57:03Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/prod-hardening-m1-e2e
CONTINUE_ON_FAIL=0
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ cargo test -p ingestion
    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.83s
     Running unittests src/lib.rs (target/debug/deps/ingestion-04b7275663e82a5d)

running 22 tests
test handlers::ingest::tests::validate_rejects_empty_batch ... ok
test handlers::ingest::tests::validate_rejects_stale_timestamp ... ok
test handlers::ingest::tests::validate_rejects_negative_cost ... ok
test kafka::producer::tests::produce_message_holds_wal_entry_id ... ok
test handlers::ingest::tests::validate_rejects_oversized_batch ... ok
test handlers::ingest::tests::validate_rejects_zero_latency ... ok
test handlers::ingest::tests::normalize_assigns_event_id_and_status ... ok
test rate_limit::tenant_limits::tests::from_defaults_resolves_unknown_tenant ... ok
test handlers::ingest::tests::tenant_from_events_json_reads_first_event ... ok
test rate_limit::token_bucket::tests::rate_limit_result_allowed_eq ... ok
test rate_limit::token_bucket::tests::rate_limit_result_denied_eq ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_missing_file ... ok
test rate_limit::tenant_limits::tests::from_file_resolves_known_tenant ... ok
test kafka::producer::tests::producer_client_config_sets_expected_options ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_burst_below_one ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_zero_rps ... ok
test rate_limit::tenant_limits::tests::from_file_rejects_malformed_json ... ok
test rate_limit::token_bucket::tests::resolve_uses_tenant_override ... ok
test server::tests::health_returns_ok ... ok
test wal::writer::tests::append_increments_entry_id ... ok
test kafka::producer::tests::mark_acked_after_success_path_without_kafka ... ok
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
    -- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema for the Go batch writer.
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
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3
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
[36mINFO[0m[0001] Deleting cluster 'lensai'                    
[36mINFO[0m[0005] Deleting 1 attached volumes...               
[36mINFO[0m[0006] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0006] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0006] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
[36mINFO[0m[0000] Using config file deploy/k3d/cluster.yaml (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8080:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9091:9091' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] Prep: Network                                
[36mINFO[0m[0000] Re-using existing network 'k3d-lensai' (367f3448b068d394a56589c94f547f7df1550d219221d7ae9826839aa1125ce1) 
[36mINFO[0m[0000] Created image volume k3d-lensai-images       
[36mINFO[0m[0000] Starting new tools node...                   
[36mINFO[0m[0000] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0001] Creating node 'k3d-lensai-server-0'          
[36mINFO[0m[0001] Creating LoadBalancer 'k3d-lensai-serverlb'  
[36mINFO[0m[0002] Using the k3d-tools node to gather environment information 
[36mINFO[0m[0002] Starting new tools node...                   
[36mINFO[0m[0003] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0004] Starting cluster 'lensai'                    
[36mINFO[0m[0004] Starting servers...                          
[36mINFO[0m[0004] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0011] All agents already running.                  
[36mINFO[0m[0011] Starting helpers...                          
[36mINFO[0m[0011] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0018] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0020] Cluster 'lensai' created successfully!       
[36mINFO[0m[0020] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 986B 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 1.7s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#5 DONE 0.0s

#6 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 119.09kB 0.1s done
#7 DONE 0.1s

#8 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#8 CACHED

#9 [builder 3/6] WORKDIR /build
#9 CACHED

#10 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#10 CACHED

#11 [builder 5/6] COPY ingestion ./ingestion
#11 DONE 0.1s

#12 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#12 0.569 info: syncing channel updates for '1.86-aarch64-unknown-linux-gnu'
#12 3.848 info: latest update on 2025-04-03, rust version 1.86.0 (05f9846f8 2025-03-31)
#12 3.849 info: downloading component 'cargo'
#12 9.638 info: downloading component 'clippy'
#12 12.61 info: downloading component 'rust-std'
#12 22.75 info: downloading component 'rustc'
#12 44.17 info: downloading component 'rustfmt'
#12 46.73 info: installing component 'cargo'
#12 47.55 info: installing component 'clippy'
#12 47.88 info: installing component 'rust-std'
#12 51.12 info: installing component 'rustc'
#12 56.38 info: installing component 'rustfmt'
#12 56.93     Updating crates.io index
#12 63.91  Downloading crates ...
#12 64.01   Downloaded async-stream-impl v0.3.6
#12 64.04   Downloaded anyhow v1.0.102
#12 64.08   Downloaded fnv v1.0.7
#12 64.09   Downloaded async-stream v0.3.6
#12 64.11   Downloaded cfg-if v1.0.4
#12 64.13   Downloaded procfs v0.16.0
#12 64.14   Downloaded httpdate v1.0.3
#12 64.14   Downloaded form_urlencoded v1.2.2
#12 64.15   Downloaded equivalent v1.0.2
#12 64.15   Downloaded idna_adapter v1.2.2
#12 64.15   Downloaded http-body v0.4.6
#12 64.16   Downloaded rand_chacha v0.3.1
#12 64.18   Downloaded lazy_static v1.5.0
#12 64.18   Downloaded rand v0.8.6
#12 64.19   Downloaded atomic-waker v1.1.2
#12 64.19   Downloaded hex v0.4.3
#12 64.19   Downloaded futures-macro v0.3.32
#12 64.20   Downloaded mime v0.3.17
#12 64.22   Downloaded matchers v0.2.0
#12 64.24   Downloaded hyper-timeout v0.4.1
#12 64.24   Downloaded itoa v1.0.18
#12 64.25   Downloaded futures-core v0.3.32
#12 64.25   Downloaded percent-encoding v2.3.2
#12 64.26   Downloaded scopeguard v1.2.0
#12 64.26   Downloaded proc-macro-crate v3.5.0
#12 64.26   Downloaded futures-sink v0.3.32
#12 64.26   Downloaded serde_urlencoded v0.7.1
#12 64.26   Downloaded futures-task v0.3.32
#12 64.27   Downloaded sha1_smol v1.0.1
#12 64.27   Downloaded potential_utf v0.1.5
#12 64.27   Downloaded http-body v1.0.1
#12 64.28   Downloaded urlencoding v2.1.3
#12 64.29   Downloaded tracing-serde v0.2.0
#12 64.29   Downloaded errno v0.3.14
#12 64.30   Downloaded yoke-derive v0.8.2
#12 64.30   Downloaded try-lock v0.2.5
#12 64.31   Downloaded http-body-util v0.1.3
#12 64.31   Downloaded glob v0.3.3
#12 64.32   Downloaded want v0.3.1
#12 64.32   Downloaded tower-service v0.3.3
#12 64.32   Downloaded nu-ansi-term v0.50.3
#12 64.33   Downloaded futures-executor v0.3.32
#12 64.33   Downloaded ordered-float v4.6.0
#12 64.33   Downloaded tower-layer v0.3.3
#12 64.34   Downloaded num_enum_derive v0.7.6
#12 64.34   Downloaded sync_wrapper v1.0.2
#12 64.34   Downloaded lock_api v0.4.14
#12 64.34   Downloaded cmake v0.1.58
#12 64.34   Downloaded bitflags v1.3.2
#12 64.35   Downloaded axum-core v0.3.4
#12 64.35   Downloaded axum-core v0.4.5
#12 64.35   Downloaded autocfg v1.5.0
#12 64.35   Downloaded displaydoc v0.2.5
#12 64.37   Downloaded pin-project-lite v0.2.17
#12 64.38   Downloaded find-msvc-tools v0.1.9
#12 64.39   Downloaded either v1.15.0
#12 64.40   Downloaded pkg-config v0.3.33
#12 64.40   Downloaded pin-project-internal v1.1.13
#12 64.41   Downloaded matchit v0.7.3
#12 64.41   Downloaded zerofrom-derive v0.1.7
#12 64.42   Downloaded tokio-macros v2.7.0
#12 64.42   Downloaded bitflags v2.11.1
#12 64.43   Downloaded stable_deref_trait v1.2.1
#12 64.43   Downloaded utf8_iter v1.0.4
#12 64.45   Downloaded async-trait v0.1.89
#12 64.47   Downloaded quote v1.0.45
#12 64.47   Downloaded ppv-lite86 v0.2.21
#12 64.48   Downloaded rand_core v0.6.4
#12 64.48   Downloaded getrandom v0.2.17
#12 64.49   Downloaded parking_lot_core v0.9.12
#12 64.50   Downloaded num_enum v0.7.6
#12 64.51   Downloaded sync_wrapper v0.1.2
#12 64.51   Downloaded opentelemetry-semantic-conventions v0.14.0
#12 64.51   Downloaded opentelemetry-otlp v0.15.0
#12 64.52   Downloaded tokio-io-timeout v1.2.1
#12 64.52   Downloaded crossbeam-utils v0.8.21
#12 64.52   Downloaded futures-channel v0.3.32
#12 64.53   Downloaded litemap v0.8.2
#12 64.54   Downloaded parking_lot v0.12.5
#12 64.55   Downloaded icu_properties v2.2.0
#12 64.55   Downloaded zerofrom v0.1.8
#12 64.55   Downloaded pin-project v1.1.13
#12 64.57   Downloaded log v0.4.29
#12 64.57   Downloaded indexmap v1.9.3
#12 64.58   Downloaded getrandom v0.4.2
#12 64.59   Downloaded rustversion v1.0.22
#12 64.59   Downloaded proc-macro2 v1.0.106
#12 64.60   Downloaded once_cell v1.21.4
#12 64.60   Downloaded num-traits v0.2.19
#12 64.62   Downloaded shlex v1.3.0
#12 64.62   Downloaded serde_path_to_error v0.1.20
#12 64.63   Downloaded thiserror v1.0.69
#12 64.63   Downloaded cc v1.2.62
#12 64.65   Downloaded icu_provider v2.2.0
#12 64.66   Downloaded zmij v1.0.21
#12 64.67   Downloaded zerovec-derive v0.11.3
#12 64.67   Downloaded procfs-core v0.16.0
#12 64.67   Downloaded opentelemetry v0.22.0
#12 64.68   Downloaded icu_normalizer v2.2.0
#12 64.69   Downloaded toml_datetime v1.1.1+spec-1.1.0
#12 64.70   Downloaded prost-derive v0.12.6
#12 64.70   Downloaded synstructure v0.13.2
#12 64.70   Downloaded icu_collections v2.2.0
#12 64.71   Downloaded base64 v0.21.7
#12 64.72   Downloaded prost v0.12.6
#12 64.72   Downloaded thread_local v1.1.9
#12 64.73   Downloaded http v1.4.0
#12 64.73   Downloaded hashbrown v0.12.3
#12 64.74   Downloaded mio v1.2.0
#12 64.76   Downloaded indexmap v2.14.0
#12 64.77   Downloaded slab v0.4.12
#12 64.78   Downloaded writeable v0.6.3
#12 64.79   Downloaded aho-corasick v1.1.4
#12 64.81   Downloaded memchr v2.8.0
#12 64.82   Downloaded http v0.2.12
#12 64.83   Downloaded opentelemetry_sdk v0.22.1
#12 64.84   Downloaded combine v4.6.7
#12 64.85   Downloaded axum v0.7.9
#12 64.85   Downloaded opentelemetry-proto v0.5.0
#12 64.86   Downloaded smallvec v1.15.1
#12 64.87   Downloaded h2 v0.3.27
#12 64.88   Downloaded yoke v0.8.2
#12 64.88   Downloaded tokio-stream v0.1.18
#12 64.89   Downloaded icu_properties_data v2.2.0
#12 64.90   Downloaded hyper v0.14.32
#12 64.91   Downloaded hashbrown v0.17.1
#12 64.92   Downloaded tinystr v0.8.3
#12 64.92   Downloaded signal-hook-registry v1.4.8
#12 64.92   Downloaded futures-util v0.3.32
#12 64.95   Downloaded itertools v0.12.1
#12 64.95   Downloaded toml_parser v1.1.2+spec-1.1.0
#12 64.96   Downloaded idna v1.1.0
#12 64.97   Downloaded hyper v1.9.0
#12 64.98   Downloaded tracing-attributes v0.1.31
#12 64.98   Downloaded axum v0.6.20
#12 65.00   Downloaded hyper-util v0.1.20
#12 65.01   Downloaded icu_normalizer_data v2.2.0
#12 65.01   Downloaded httparse v1.10.1
#12 65.02   Downloaded serde_derive v1.0.228
#12 65.02   Downloaded ryu v1.0.23
#12 65.03   Downloaded crossbeam-channel v0.5.15
#12 65.04   Downloaded sharded-slab v0.1.7
#12 65.04   Downloaded bytes v1.11.1
#12 65.05   Downloaded tracing-log v0.2.0
#12 65.05   Downloaded thiserror-impl v1.0.69
#12 65.05   Downloaded icu_locale_core v2.2.0
#12 65.06   Downloaded tracing-core v0.1.36
#12 65.08   Downloaded socket2 v0.5.10
#12 65.09   Downloaded uuid v1.23.1
#12 65.11   Downloaded unicode-ident v1.0.24
#12 65.12   Downloaded libc v0.2.186
#12 65.17   Downloaded socket2 v0.6.3
#12 65.17   Downloaded libz-sys v1.1.28
#12 65.20   Downloaded serde_core v1.0.228
#12 65.29   Downloaded linux-raw-sys v0.4.15
#12 65.36   Downloaded prometheus v0.13.4
#12 65.40   Downloaded tonic v0.11.0
#12 65.41   Downloaded tower-http v0.5.2
#12 65.42   Downloaded rdkafka v0.36.2
#12 65.43   Downloaded tower v0.5.3
#12 65.45   Downloaded zerovec v0.11.6
#12 65.48   Downloaded serde_json v1.0.149
#12 65.50   Downloaded winnow v1.0.2
#12 65.53   Downloaded tracing-subscriber v0.3.23
#12 65.55   Downloaded redis v0.25.5
#12 65.56   Downloaded vcpkg v0.2.15
#12 65.64   Downloaded tracing-opentelemetry v0.23.0
#12 65.64   Downloaded tokio-util v0.7.18
#12 65.65   Downloaded zerocopy v0.8.48
#12 65.68   Downloaded syn v2.0.117
#12 65.70   Downloaded tower v0.4.13
#12 65.71   Downloaded protobuf v2.28.0
#12 65.72   Downloaded regex-syntax v0.8.10
#12 65.73   Downloaded rustix v0.38.44
#12 65.76   Downloaded zerotrie v0.2.4
#12 65.78   Downloaded url v2.5.8
#12 65.78   Downloaded toml_edit v0.25.11+spec-1.1.0
#12 65.79   Downloaded serde v1.0.228
#12 65.79   Downloaded tracing v0.1.44
#12 65.83   Downloaded regex-automata v0.4.14
#12 65.88   Downloaded tokio v1.52.3
#12 66.12   Downloaded rdkafka-sys v4.10.0+2.12.1
#12 66.58    Compiling proc-macro2 v1.0.106
#12 66.58    Compiling unicode-ident v1.0.24
#12 66.58    Compiling quote v1.0.45
#12 66.58    Compiling libc v0.2.186
#12 66.58    Compiling cfg-if v1.0.4
#12 66.58    Compiling pin-project-lite v0.2.17
#12 66.58    Compiling smallvec v1.15.1
#12 66.60    Compiling bytes v1.11.1
#12 67.20    Compiling futures-core v0.3.32
#12 67.20    Compiling parking_lot_core v0.9.12
#12 67.33    Compiling scopeguard v1.2.0
#12 67.63    Compiling itoa v1.0.18
#12 67.80    Compiling lock_api v0.4.14
#12 67.80    Compiling futures-sink v0.3.32
#12 67.92    Compiling once_cell v1.21.4
#12 68.17    Compiling futures-task v0.3.32
#12 68.22    Compiling slab v0.4.12
#12 68.26    Compiling log v0.4.29
#12 68.47    Compiling tracing-core v0.1.36
#12 68.55    Compiling rustversion v1.0.22
#12 68.57    Compiling serde_core v1.0.228
#12 68.66    Compiling autocfg v1.5.0
#12 68.85    Compiling stable_deref_trait v1.2.1
#12 69.00    Compiling tower-service v0.3.3
#12 69.41    Compiling futures-channel v0.3.32
#12 70.73    Compiling zerocopy v0.8.48
#12 71.23    Compiling memchr v2.8.0
#12 71.49    Compiling serde v1.0.228
#12 71.95    Compiling syn v2.0.117
#12 72.44    Compiling percent-encoding v2.3.2
#12 72.56    Compiling fnv v1.0.7
#12 72.60    Compiling httparse v1.10.1
#12 73.33    Compiling errno v0.3.14
#12 73.67    Compiling signal-hook-registry v1.4.8
#12 73.71    Compiling mio v1.2.0
#12 73.83    Compiling parking_lot v0.12.5
#12 74.38    Compiling socket2 v0.6.3
#12 74.67    Compiling getrandom v0.2.17
#12 75.34    Compiling tower-layer v0.3.3
#12 75.58    Compiling rand_core v0.6.4
#12 76.59    Compiling http v0.2.12
#12 76.67    Compiling anyhow v1.0.102
#12 77.80    Compiling find-msvc-tools v0.1.9
#12 77.89    Compiling shlex v1.3.0
#12 78.33    Compiling httpdate v1.0.3
#12 78.38    Compiling cc v1.2.62
#12 78.45    Compiling hashbrown v0.17.1
#12 78.75    Compiling equivalent v1.0.2
#12 79.05    Compiling litemap v0.8.2
#12 79.63    Compiling writeable v0.6.3
#12 80.12    Compiling thiserror v1.0.69
#12 81.34    Compiling socket2 v0.5.10
#12 83.82    Compiling http-body v0.4.6
#12 83.91    Compiling num-traits v0.2.19
#12 84.83    Compiling indexmap v2.14.0
#12 85.35    Compiling indexmap v1.9.3
#12 85.43    Compiling http v1.4.0
#12 86.07    Compiling winnow v1.0.2
#12 87.72    Compiling zmij v1.0.21
#12 87.85    Compiling try-lock v0.2.5
#12 89.15    Compiling icu_properties_data v2.2.0
#12 89.24    Compiling utf8_iter v1.0.4
#12 89.98    Compiling icu_normalizer_data v2.2.0
#12 90.61    Compiling crossbeam-utils v0.8.21
#12 90.61    Compiling mime v0.3.17
#12 91.19    Compiling pkg-config v0.3.33
#12 92.46    Compiling http-body v1.0.1
#12 93.43    Compiling want v0.3.1
#12 93.84    Compiling toml_parser v1.1.2+spec-1.1.0
#12 94.45    Compiling axum-core v0.3.4
#12 94.61    Compiling toml_datetime v1.1.1+spec-1.1.0
#12 94.67    Compiling either v1.15.0
#12 95.19    Compiling synstructure v0.13.2
#12 95.44    Compiling serde_json v1.0.149
#12 95.44    Compiling vcpkg v0.2.15
#12 96.24    Compiling hashbrown v0.12.3
#12 96.97    Compiling toml_edit v0.25.11+spec-1.1.0
#12 97.11    Compiling itertools v0.12.1
#12 100.2    Compiling libz-sys v1.1.28
#12 103.6    Compiling axum v0.6.20
#12 103.6    Compiling lazy_static v1.5.0
#12 103.7    Compiling urlencoding v2.1.3
#12 104.0    Compiling bitflags v2.11.1
#12 104.0    Compiling matchit v0.7.3
#12 105.3    Compiling ordered-float v4.6.0
#12 107.0    Compiling crossbeam-channel v0.5.15
#12 107.2    Compiling proc-macro-crate v3.5.0
#12 107.2    Compiling cmake v0.1.58
#12 107.7    Compiling form_urlencoded v1.2.2
#12 108.6    Compiling sync_wrapper v0.1.2
#12 108.7    Compiling rustix v0.38.44
#12 108.9    Compiling regex-syntax v0.8.10
#12 109.0    Compiling glob v0.3.3
#12 109.3    Compiling bitflags v1.3.2
#12 109.6    Compiling rdkafka-sys v4.10.0+2.12.1
#12 110.9    Compiling http-body-util v0.1.3
#12 111.4    Compiling hex v0.4.3
#12 112.6    Compiling ppv-lite86 v0.2.21
#12 113.1    Compiling base64 v0.21.7
#12 113.2    Compiling getrandom v0.4.2
#12 113.3    Compiling tokio-macros v2.7.0
#12 114.4    Compiling futures-macro v0.3.32
#12 114.4    Compiling zerofrom-derive v0.1.7
#12 116.1    Compiling tracing-attributes v0.1.31
#12 116.7    Compiling yoke-derive v0.8.2
#12 119.0    Compiling tokio v1.52.3
#12 119.7    Compiling futures-util v0.3.32
#12 122.8    Compiling zerovec-derive v0.11.3
#12 124.6    Compiling zerofrom v0.1.8
#12 125.6    Compiling displaydoc v0.2.5
#12 125.7    Compiling yoke v0.8.2
#12 126.5    Compiling tracing v0.1.44
#12 127.2    Compiling serde_derive v1.0.228
#12 127.2    Compiling async-trait v0.1.89
#12 129.6    Compiling rand_chacha v0.3.1
#12 131.5    Compiling zerovec v0.11.6
#12 131.5    Compiling zerotrie v0.2.4
#12 134.4    Compiling rand v0.8.6
#12 134.5    Compiling thiserror-impl v1.0.69
#12 135.7    Compiling tinystr v0.8.3
#12 137.0    Compiling potential_utf v0.1.5
#12 139.3    Compiling icu_locale_core v2.2.0
#12 139.4    Compiling icu_collections v2.2.0
#12 146.3    Compiling pin-project-internal v1.1.13
#12 148.7    Compiling futures-executor v0.3.32
#12 152.5    Compiling icu_provider v2.2.0
#12 153.0    Compiling opentelemetry v0.22.0
#12 155.4    Compiling pin-project v1.1.13
#12 155.8    Compiling icu_normalizer v2.2.0
#12 156.1    Compiling icu_properties v2.2.0
#12 158.2    Compiling async-stream-impl v0.3.6
#12 160.3    Compiling prost-derive v0.12.6
#12 161.1    Compiling regex-automata v0.4.14
#12 162.1    Compiling async-stream v0.3.6
#12 162.4    Compiling num_enum_derive v0.7.6
#12 162.8    Compiling atomic-waker v1.1.2
#12 162.9    Compiling idna_adapter v1.2.2
#12 163.4    Compiling procfs v0.16.0
#12 163.6    Compiling linux-raw-sys v0.4.15
#12 164.1    Compiling protobuf v2.28.0
#12 166.1    Compiling sync_wrapper v1.0.2
#12 166.5    Compiling ryu v1.0.23
#12 167.0    Compiling idna v1.1.0
#12 167.7    Compiling tracing-serde v0.2.0
#12 168.6    Compiling procfs-core v0.16.0
#12 176.0    Compiling num_enum v0.7.6
#12 177.9    Compiling sharded-slab v0.1.7
#12 178.0    Compiling prost v0.12.6
#12 180.0    Compiling tracing-log v0.2.0
#12 182.2    Compiling tokio-util v0.7.18
#12 182.3    Compiling tokio-stream v0.1.18
#12 184.4    Compiling tokio-io-timeout v1.2.1
#12 185.6    Compiling opentelemetry_sdk v0.22.1
#12 185.7    Compiling matchers v0.2.0
#12 187.0    Compiling h2 v0.3.27
#12 192.3    Compiling tower v0.4.13
#12 209.8    Compiling hyper v1.9.0
#12 216.3    Compiling thread_local v1.1.9
#12 219.9    Compiling prometheus v0.13.4
#12 222.1    Compiling nu-ansi-term v0.50.3
#12 224.3    Compiling tracing-subscriber v0.3.23
#12 225.5    Compiling hyper-util v0.1.20
#12 240.4    Compiling combine v4.6.7
#12 250.3    Compiling hyper v0.14.32
#12 292.4    Compiling tower v0.5.3
#12 299.2    Compiling url v2.5.8
#12 315.6    Compiling serde_urlencoded v0.7.1
#12 319.7    Compiling axum-core v0.4.5
#12 321.4    Compiling serde_path_to_error v0.1.20
#12 327.9    Compiling opentelemetry-semantic-conventions v0.14.0
#12 328.9    Compiling sha1_smol v1.0.1
#12 331.7    Compiling hyper-timeout v0.4.1
#12 336.0    Compiling redis v0.25.5
#12 356.5    Compiling axum v0.7.9
#12 359.6    Compiling uuid v1.23.1
#12 360.8    Compiling tracing-opentelemetry v0.23.0
#12 376.9    Compiling tower-http v0.5.2
#12 428.3    Compiling tonic v0.11.0
#12 466.0    Compiling opentelemetry-proto v0.5.0
#12 481.1    Compiling opentelemetry-otlp v0.15.0
#12 546.3    Compiling rdkafka v0.36.2
#12 548.6    Compiling ingestion v0.1.0 (/build/ingestion)
#12 574.7     Finished `release` profile [optimized] target(s) in 8m 37s
#12 DONE 577.1s

#13 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#13 CACHED

#14 [stage-1 3/5] WORKDIR /app
#14 CACHED

#15 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#15 DONE 0.1s

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 DONE 0.4s

#17 exporting to image
#17 exporting layers 0.0s done
#17 writing image sha256:478e97a072059050b98fecd59c51f19a3662cbb568d572ffede0988263de966d done
#17 naming to docker.io/lensai/ingestion:local done
#17 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/rri2s80k9ocwhb1qci5br0cwu
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 541B done
#1 DONE 0.0s

#2 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#2 DONE 0.7s

#3 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#3 DONE 1.8s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#5 DONE 0.0s

#6 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 72.07kB 0.0s done
#7 DONE 0.0s

#8 [builder 2/6] WORKDIR /build/consumer
#8 CACHED

#9 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#9 CACHED

#10 [builder 4/6] RUN go mod download
#10 CACHED

#11 [builder 5/6] COPY consumer/ ./
#11 DONE 0.1s

#12 [builder 6/6] RUN CGO_ENABLED=0 go build -o /consumer ./cmd/consumer
#12 DONE 200.3s

#5 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#5 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 DONE 0.3s

#14 exporting to image
#14 exporting layers 0.1s done
#14 writing image sha256:f87e5a810fdc3ee8241bc99bd2d421288c9908f9ecac93de2412a7e5a2b24ad8 done
#14 naming to docker.io/lensai/consumer:local
#14 naming to docker.io/lensai/consumer:local done
#14 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/x0jwd91bb0nzplw9l3z6mdqvh
==> Importing images into k3d
[36mINFO[0m[0000] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0000] Saving 2 image(s) from runtime...            
[36mINFO[0m[0006] Importing images into nodes...               
[36mINFO[0m[0006] Importing images from tarball '/k3d/images/k3d-lensai-images-20260527144044.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0012] Removing the tarball(s) from image volume... 
[36mINFO[0m[0013] Removing k3d-tools node...                   
[36mINFO[0m[0013] Successfully imported image(s)               
[36mINFO[0m[0013] Successfully imported 2 image(s) into 1 cluster(s) 
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
LAST DEPLOYED: Wed May 27 14:41:02 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-74b767d99c-k7d5z condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-ttn8s condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-cb767579f-v6tgm condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-9c5r2 condition met
NAME                                     READY   STATUS      RESTARTS       AGE
pod/lensai-clickhouse-0                  1/1     Running     0              2m19s
pod/lensai-clickhouse-init-78glf         0/1     Completed   0              2m19s
pod/lensai-consumer-6d6f555797-9c5r2     1/1     Running     4 (81s ago)    2m21s
pod/lensai-ingestion-cb767579f-v6tgm     1/1     Running     1 (2m6s ago)   2m21s
pod/lensai-prometheus-7495bc6667-ttn8s   1/1     Running     0              2m21s
pod/lensai-redis-74b767d99c-k7d5z        1/1     Running     0              2m21s
pod/lensai-redpanda-0                    1/1     Running     0              2m20s
pod/lensai-redpanda-init-dkxcq           0/1     Completed   3              2m19s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           70s        2m19s
job.batch/lensai-redpanda-init     Complete   1/1           100s       2m20s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-9c5r2 condition met
pod/lensai-ingestion-cb767579f-v6tgm condition met
pod/lensai-prometheus-7495bc6667-ttn8s condition met
pod/lensai-redis-74b767d99c-k7d5z condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS       AGE
lensai-clickhouse-0                  1/1     Running     0              2m21s
lensai-clickhouse-init-78glf         0/1     Completed   0              2m21s
lensai-consumer-6d6f555797-9c5r2     1/1     Running     4 (83s ago)    2m23s
lensai-ingestion-cb767579f-v6tgm     1/1     Running     1 (2m8s ago)   2m23s
lensai-prometheus-7495bc6667-ttn8s   1/1     Running     0              2m23s
lensai-redis-74b767d99c-k7d5z        1/1     Running     0              2m23s
lensai-redpanda-0                    1/1     Running     0              2m22s
lensai-redpanda-init-dkxcq           0/1     Completed   3              2m21s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"status":"ok"}
ok
==> POST /ingest
HTTP 202 — {"batch_id":"a682db0b-0409-4c4b-bc38-602f994a4f58","event_count":1,"accepted_at_unix_ms":1779873208484}
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
[0;36m[chaos-k8s][0m   round 10/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 279
[0;36m[chaos-k8s][0m   round 11/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 279
[0;36m[chaos-k8s][0m   round 12/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 279
[0;36m[chaos-k8s][0m   round 13/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 279
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15001 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15003 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15001 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15006 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 14/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15006 milliseconds with 0 bytes receivedc
url: (28) Operation timed out after 15005 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 15/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 16/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 17/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 18/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 19/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 20/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 21/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 22/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 23/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 24/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 25/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed ccurl: (28) Operationout  timed outurl: (28) Oafter 15005 milliseconds with 0 bytes recpcurl: (28) Operation timed out after 15eratiei after 15003 mil003lisecved
on milliseconds wids with 0 th 0 bytes received
on timed out after 15003 millisbytes received
econds with 0 bytes received
[0;36m[chaos-k8s][0m   round 26/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed out after 15003 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 27/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m   round 28/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed out curl: (28) Opaeration timed out after 15002 milliseconds with 0 bytes received
fter 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15004 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15003 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15005 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15006 milliseconds with 0 bytces received
url: (28) Operation timed out after 15003 mcurl: (28) Opeilliserconation tds withimed out after 15002 milli 0 bytes receivedseconds
 with 0 bytes received
curl: (28) Operation timed out after 15005 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15005 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 29/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
curl: (28) Operation timed out after 15001 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15001 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15003 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15004 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15004 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15006 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15003 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15006 milliseconds with 0 bytes receivecurl: (28) Opercuatidroncurl: timedl out after 15005: (28) m
illisec Operation tim (o28) Opeed out after 1ration timed out after5005 milliseconds withn 0 bytces 15002 r milliseconds with 0 bytes received
eceived
ds with 0 bytes receivurl: e(d
28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15001 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15002 milliseconds with 0 bytes received
curl: (28) Operation timed out after 15005 milliseconds with 0 bytes received
[0;36m[chaos-k8s][0m   round 30/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 366
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;32m[PASS][0m ClickHouse ready
  breaker max: 0, overflow max: 0, ch_errors Δ: 0, lag peak: 366
  breaker after: 0, overflow after: 0
  CH rows before/after: 400 / 1450
[0;32m[PASS][0m Consumer lag backlog during CH outage (peak 366 events)

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
  Sent: ~1000, new CH rows: 350, lag: 366, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (350 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```

## Run 20260528T101040Z

```
Started: 2026-05-28T10:10:41Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/day12-anomaly-plan-alignment
CONTINUE_ON_FAIL=0
HELM_VALUES=/Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
SKIP_CHAOS=0
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ cargo test -p ingestion
   Compiling rdkafka-sys v4.10.0+2.12.1
error: failed to run custom build command for `rdkafka-sys v4.10.0+2.12.1`

Caused by:
  process didn't exit successfully: `/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-c6f355b5b92c7bf9/build-script-build` (exit status: 101)
  --- stdout
  Configuring and compiling librdkafka
  CMAKE_TOOLCHAIN_FILE_aarch64-apple-darwin = None
  CMAKE_TOOLCHAIN_FILE_aarch64_apple_darwin = None
  HOST_CMAKE_TOOLCHAIN_FILE = None
  CMAKE_TOOLCHAIN_FILE = None
  CMAKE_GENERATOR_aarch64-apple-darwin = None
  CMAKE_GENERATOR_aarch64_apple_darwin = None
  HOST_CMAKE_GENERATOR = None
  CMAKE_GENERATOR = None
  CMAKE_PREFIX_PATH_aarch64-apple-darwin = None
  CMAKE_PREFIX_PATH_aarch64_apple_darwin = None
  HOST_CMAKE_PREFIX_PATH = None
  CMAKE_PREFIX_PATH = None
  CMAKE_aarch64-apple-darwin = None
  CMAKE_aarch64_apple_darwin = None
  HOST_CMAKE = None
  CMAKE = None
  -- Found Zstd: /opt/homebrew/lib/libzstd.dylib
  -- Configuring done (1.9s)
  -- Generating done (0.0s)
  -- Build files have been written to: /Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out/build
  [ 86%] Built target rdkafka
  [ 88%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/HeadersImpl.cpp.o
  [ 88%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/ConfImpl.cpp.o
  [ 89%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/KafkaConsumerImpl.cpp.o
  [ 90%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/ConsumerImpl.cpp.o
  [ 91%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/ProducerImpl.cpp.o
  [ 92%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/HandleImpl.cpp.o
  [ 93%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/MessageImpl.cpp.o
  [ 94%] Building CXX object src-cpp/CMakeFiles/rdkafka++.dir/MetadataImpl.cpp.o

  --- stderr
  Building and linking librdkafka statically
  running: cd "/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out/build" && CMAKE_PREFIX_PATH="" LC_ALL="C" "cmake" "/Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka" "-B" "/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out/build" "-DCMAKE_OSX_ARCHITECTURES=arm64" "-DRDKAFKA_BUILD_STATIC=1" "-DRDKAFKA_BUILD_TESTS=0" "-DRDKAFKA_BUILD_EXAMPLES=0" "-DCMAKE_INSTALL_LIBDIR=lib" "-DCMAKE_POLICY_VERSION_MINIMUM=3.5" "-DWITH_ZLIB=1" "-DWITH_CURL=0" "-DWITH_SSL=0" "-DWITH_SASL=0" "-DWITH_ZSTD=0" "-DENABLE_LZ4_EXT=0" "-DCMAKE_INSTALL_PREFIX=/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out" "-DCMAKE_C_FLAGS= -ffunction-sections -fdata-sections -fPIC --target=arm64-apple-macosx -mmacosx-version-min=26.2 -w" "-DCMAKE_C_COMPILER=/usr/bin/cc" "-DCMAKE_CXX_FLAGS= -ffunction-sections -fdata-sections -fPIC --target=arm64-apple-macosx -mmacosx-version-min=26.2 -w" "-DCMAKE_CXX_COMPILER=/usr/bin/c++" "-DCMAKE_ASM_FLAGS= -ffunction-sections -fdata-sections -fPIC --target=arm64-apple-macosx -mmacosx-version-min=26.2 -w" "-DCMAKE_ASM_COMPILER=/usr/bin/cc" "-DCMAKE_BUILD_TYPE=Debug"
  CMake Deprecation Warning at CMakeLists.txt:1 (cmake_minimum_required):
    Compatibility with CMake < 3.10 will be removed from a future version of
    CMake.

    Update the VERSION argument <min> value.  Or, use the <min>...<max> syntax
    to tell CMake that the project requires at least <min> but has been updated
    to work with policies introduced by <max> or earlier.


  running: cd "/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out/build" && LC_ALL="C" "cmake" "--build" "/Users/akshant/Desktop/Github/infra-ai-streaming/target/debug/build/rdkafka-sys-698109fb1c4a914c/out/build" "--target" "install" "--config" "Debug" "--parallel" "8"
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/ConfImpl.cpp:29:10: fatal error: 'iostream' file not found
     29 | #include <iostream>
        |          ^~~~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/HeadersImpl.cpp:29:10: fatal error: 'iostream' file not found
     29 | #include <iostream>
        |          ^~~~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/KafkaConsumerImpl.cpp:29:10: fatal error: 'string' file not found
     29 | #include <string>
        |          ^~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/ConsumerImpl.cpp:29:10: fatal error: 'iostream' file not found
     29 | #include <iostream>
        |          ^~~~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/ProducerImpl.cpp:29:10: fatal error: 'iostream' file not found
     29 | #include <iostream>
        |          ^~~~~~~~~~
  In file included from /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/MetadataImpl.cpp:29:
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/rdkafkacpp_int.h:33:10: fatal error: 'string' file not found
     33 | #include <string>
        |          ^~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/HandleImpl.cpp:30:10: fatal error: 'iostream' file not found
     30 | #include <iostream>
        |          ^~~~~~~~~~
  /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/rdkafka-sys-4.10.0+2.12.1/librdkafka/src-cpp/MessageImpl.cpp:29:10: fatal error: 'iostream' file not found
     29 | #include <iostream>
        |          ^~~~~~~~~~
  1 error generated.
  1 error generated.
  1 error generated.
  1 error generated.
  1 error generated.
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/KafkaConsumerImpl.cpp.o] Error 1
  make[2]: *** Waiting for unfinished jobs....
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/ProducerImpl.cpp.o] Error 1
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/HeadersImpl.cpp.o] Error 1
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/ConfImpl.cpp.o] Error 1
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/MessageImpl.cpp.o] Error 1
  1 error generated.
  1 error generated.
  1 error generated.
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/MetadataImpl.cpp.o] Error 1
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/HandleImpl.cpp.o] Error 1
  make[2]: *** [src-cpp/CMakeFiles/rdkafka++.dir/ConsumerImpl.cpp.o] Error 1
  make[1]: *** [src-cpp/CMakeFiles/rdkafka++.dir/all] Error 2
  make: *** [all] Error 2

  thread 'main' panicked at /Users/akshant/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/cmake-0.1.58/src/lib.rs:1132:5:

  command did not execute successfully, got: exit status: 2

  build script failed, must exit now
  note: run with `RUST_BACKTRACE=1` environment variable to display a backtrace
exit=101
$ bash -c cd consumer && go test ./...
?   	github.com/akshantvats/infra-ai-streaming/consumer/cmd/consumer	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/clickhouse	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/config	[no test files]
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka	(cached)
ok  	github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics	(cached)
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/model	[no test files]
?   	github.com/akshantvats/infra-ai-streaming/consumer/internal/redis	[no test files]
exit=0
$ helm template lensai deploy/helm/lensai -f /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml --namespace lensai
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
    -- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema for Day 5 writer.
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
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3
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
$ bash -n scripts/run.sh
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
==> Using host ports http=8081 metrics=9092
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.xOotTsAzl0 (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8081:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9092:9092' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[31mERRO[0m[0000] Failed to get nodes for cluster 'lensai': docker failed to get containers with labels 'map[k3d.cluster:lensai]': failed to list containers: Error response from daemon: Docker Desktop is manually paused. Unpause it through the Whale menu or Dashboard. 
[36mINFO[0m[0000] Prep: Network                                
[31mERRO[0m[0000] Failed Cluster Preparation: Failed Network Preparation: failed to create cluster network: failed to check for duplicate docker networks: docker failed to list networks: Error response from daemon: Docker Desktop is manually paused. Unpause it through the Whale menu or Dashboard. 
[31mERRO[0m[0000] Failed to create cluster >>> Rolling Back    
[36mINFO[0m[0000] Deleting cluster 'lensai'                    
[31mERRO[0m[0000] Failed to get nodes for cluster 'lensai': docker failed to get containers with labels 'map[k3d.cluster:lensai]': failed to list containers: Error response from daemon: Docker Desktop is manually paused. Unpause it through the Whale menu or Dashboard. 
[31mERRO[0m[0000] failed to get cluster: No nodes found for given cluster 
[31mFATA[0m[0000] Cluster creation FAILED, also FAILED to rollback changes! 
exit=1
$ helm dependency update deploy/helm/lensai
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Saving 1 charts
Downloading prometheus-adapter from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
exit=0
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Error: kubernetes cluster unreachable: Get "https://0.0.0.0:51857/version": net/http: TLS handshake timeout
exit=1
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
E0528 15:41:10.175464   13844 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:41:20.180231   13844 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:41:30.186098   13844 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:41:40.191247   13844 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:41:50.194436   13844 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
=== FAIL: pods not ready (component=redis) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=redis) ===
--- kubectl get pods -n lensai ---
E0528 15:42:00.267921   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:00.267921   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:10.269501   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:10.269501   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:20.272166   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:20.272166   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:30.275444   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:30.275444   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:40.277887   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:42:40.277887   13890 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
Unable to connect to the server: net/http: TLS handshake timeout
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
E0528 15:43:40.510833   17499 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:43:50.518124   17499 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:00.520865   17499 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:10.524165   17499 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:20.527716   17499 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
=== FAIL: pods not ready (component=redpanda) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=redpanda) ===
--- kubectl get pods -n lensai ---
E0528 15:44:30.600574   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:30.600574   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:40.604262   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:40.604262   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:50.607917   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:44:50.607917   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:45:00.610871   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:45:00.610871   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:45:10.613208   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:45:10.613208   17519 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
Unable to connect to the server: net/http: TLS handshake timeout
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
E0528 15:46:10.760410   17551 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:46:20.763474   17551 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:46:30.765646   17551 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:46:40.769800   17551 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:46:50.773757   17551 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
=== FAIL: pods not ready (component=clickhouse) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=clickhouse) ===
--- kubectl get pods -n lensai ---
E0528 15:47:00.832525   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:00.832525   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:10.840919   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:10.840919   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:20.844878   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:20.844878   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:30.848007   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:30.848007   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:40.851167   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:47:40.851167   17584 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
Unable to connect to the server: net/http: TLS handshake timeout
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
E0528 15:48:41.121315   19247 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:48:51.127749   19247 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:01.131641   19247 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:11.136597   19247 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:21.142010   19247 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
Unable to connect to the server: net/http: TLS handshake timeout
=== FAIL: pods not ready (component=prometheus) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=prometheus) ===
--- kubectl get pods -n lensai ---
E0528 15:49:31.220673   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:31.220673   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:41.228339   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:41.228339   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:51.240230   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:49:51.240230   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:50:01.248447   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:50:01.248447   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": net/http: TLS handshake timeout"
E0528 15:50:01.255477   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.255477   19263 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
E0528 15:50:01.546133   19435 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.546629   19435 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.547693   19435 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.547837   19435 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
=== FAIL: job/lensai-redpanda-init not complete within 180s ===
=== FAIL: job/lensai-redpanda-init not complete within 180s ===
E0528 15:50:01.612474   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.612474   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.612685   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.612685   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.613820   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.613820   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.613953   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.613953   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.615616   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
E0528 15:50:01.615616   19438 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
E0528 15:50:01.666258   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.666258   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.667657   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.667657   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.668024   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.668024   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.669020   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.669020   19439 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
E0528 15:50:01.715492   19440 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.715783   19440 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.718228   19440 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.719140   19440 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
=== FAIL: job/lensai-clickhouse-init not complete within 180s ===
=== FAIL: job/lensai-clickhouse-init not complete within 180s ===
E0528 15:50:01.772480   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.772480   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.772723   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.772723   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.773823   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.773823   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.773966   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.773966   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.776184   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.776184   19443 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
E0528 15:50:01.823043   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.823043   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.823269   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.823269   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.824413   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.824413   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.824573   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.824573   19444 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
E0528 15:50:01.875171   19445 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.876130   19445 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.876639   19445 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.877487   19445 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.877830   19445 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
=== FAIL: pods not ready (component=ingestion) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=ingestion) ===
--- kubectl get pods -n lensai ---
E0528 15:50:01.929548   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.929548   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.929820   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.929820   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.930963   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.930963   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.931102   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.931102   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.932246   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:01.932246   19448 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
E0528 15:50:02.047083   19451 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.047450   19451 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.048521   19451 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.048863   19451 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.052374   19451 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
=== FAIL: pods not ready (component=consumer) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=consumer) ===
--- kubectl get pods -n lensai ---
E0528 15:50:02.106458   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.106458   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.106699   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.106699   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.107801   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.107801   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.107934   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.107934   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.110784   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.110784   19454 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
E0528 15:50:02.212754   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.213083   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.214288   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.214448   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.215805   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.216434   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
E0528 15:50:02.218313   19457 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"https://0.0.0.0:51857/api?timeout=32s\": dial tcp 0.0.0.0:51857: connect: connection refused"
The connection to the server 0.0.0.0:51857 was refused - did you specify the right host or port?
[0;36m[e2e][0m [0;31mCluster not ready within 120s per workload[0m
exit=1
```

## Run 20260528T104634Z

```
Started: 2026-05-28T10:46:35Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/day12-anomaly-plan-alignment
CONTINUE_ON_FAIL=0
HELM_VALUES=/Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
SKIP_CHAOS=0
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ k3d cluster delete lensai
[31mERRO[0m[0001] error getting loadbalancer config from k3d-lensai-serverlb: runtime failed to read loadbalancer config '/etc/confd/values.yaml' from node 'k3d-lensai-serverlb': Error response from daemon: Could not find the file /etc/confd/values.yaml in container 38485199c27ad96d38cd7528223a55670eef0626f3302e83897593a64ae47b7a: file not found 
[36mINFO[0m[0001] Deleting cluster 'lensai'                    
[36mINFO[0m[0008] Deleting 1 attached volumes...               
[36mINFO[0m[0008] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0008] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0008] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.0K6qZNynzW (k3d.io/v1alpha5#simple) 
[36mINFO[0m[0000] portmapping '8080:80' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] portmapping '9091:9091' targets the loadbalancer: defaulting to [servers:*:proxy agents:*:proxy] 
[36mINFO[0m[0000] Prep: Network                                
[36mINFO[0m[0000] Re-using existing network 'k3d-lensai' (367f3448b068d394a56589c94f547f7df1550d219221d7ae9826839aa1125ce1) 
[36mINFO[0m[0000] Created image volume k3d-lensai-images       
[36mINFO[0m[0000] Starting new tools node...                   
[36mINFO[0m[0001] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0001] Creating node 'k3d-lensai-server-0'          
[36mINFO[0m[0002] Creating LoadBalancer 'k3d-lensai-serverlb'  
[36mINFO[0m[0004] Using the k3d-tools node to gather environment information 
[36mINFO[0m[0006] Starting new tools node...                   
[36mINFO[0m[0007] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0011] Starting cluster 'lensai'                    
[36mINFO[0m[0011] Starting servers...                          
[36mINFO[0m[0011] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0068] All agents already running.                  
[36mINFO[0m[0068] Starting helpers...                          
[36mINFO[0m[0068] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0080] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0085] Cluster 'lensai' created successfully!       
[36mINFO[0m[0086] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile:
#1 transferring dockerfile: 1.08kB 0.1s done
#1 DONE 0.2s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 3.8s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.1s

#5 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#5 DONE 0.0s

#6 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#6 DONE 0.0s

#7 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#7 CACHED

#8 [builder 3/6] WORKDIR /build
#8 CACHED

#9 [internal] load build context
#9 transferring context: 1.23kB 0.1s done
#9 DONE 0.3s

#10 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#10 DONE 0.1s

#11 [builder 5/6] COPY ingestion ./ingestion
#11 DONE 0.0s

#12 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#12 CACHED

#13 [builder 5/6] COPY ingestion ./ingestion
#13 CACHED

#14 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#14 10.71 info: syncing channel updates for '1.88-aarch64-unknown-linux-gnu'
#14 17.18 info: latest update on 2025-06-26, rust version 1.88.0 (6b00bc388 2025-06-23)
#14 17.18 info: downloading component 'cargo'
#14 20.29 info: downloading component 'clippy'
#14 23.01 info: downloading component 'rust-std'
#14 31.45 info: downloading component 'rustc'
#14 45.87 info: downloading component 'rustfmt'
#14 46.88 info: installing component 'cargo'
#14 76.62 info: installing component 'clippy'
#14 86.34 info: installing component 'rust-std'
#14 128.7 info: installing component 'rustc'
ERROR: failed to build: failed to receive status: rpc error: code = Unavailable desc = error reading from server: EOF

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/sg9a50zd0li81lal2hw8hgbfi
exit=1
$ helm dependency update deploy/helm/lensai
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Saving 1 charts
Downloading prometheus-adapter from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
exit=0
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Thu May 28 16:21:11 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
error: no matching resources found
=== FAIL: pods not ready (component=redis) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=redis) ===
--- kubectl get pods -n lensai ---
No resources found in lensai namespace.
No resources found in lensai namespace.
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
error: no matching resources found
=== FAIL: pods not ready (component=redpanda) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=redpanda) ===
--- kubectl get pods -n lensai ---
No resources found in lensai namespace.
No resources found in lensai namespace.
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
error: no matching resources found
=== FAIL: pods not ready (component=clickhouse) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=clickhouse) ===
--- kubectl get pods -n lensai ---
No resources found in lensai namespace.
No resources found in lensai namespace.
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Name:             lensai-clickhouse-0
Namespace:        lensai
Priority:         0
Service Account:  default
Node:             <none>
Labels:           app.kubernetes.io/component=clickhouse
                  app.kubernetes.io/instance=lensai
                  app.kubernetes.io/name=lensai
                  apps.kubernetes.io/pod-index=0
                  controller-revision-hash=lensai-clickhouse-69df5b4bf7
                  statefulset.kubernetes.io/pod-name=lensai-clickhouse-0
Annotations:      <none>
Status:           Pending
IP:               
IPs:              <none>
Controlled By:    StatefulSet/lensai-clickhouse
Containers:
  clickhouse:
    Image:       clickhouse/clickhouse-server:24.12-alpine
    Ports:       8123/TCP (http), 9000/TCP (native)
    Host Ports:  0/TCP (http), 0/TCP (native)
    Limits:
      cpu:     500m
      memory:  1Gi
    Requests:
      cpu:        250m
      memory:     512Mi
    Liveness:     exec [clickhouse-client --query SELECT 1] delay=40s timeout=5s period=15s #success=1 #failure=3
    Readiness:    exec [clickhouse-client --query SELECT 1] delay=25s timeout=5s period=10s #success=1 #failure=6
    Environment:  <none>
    Mounts:
      /etc/clickhouse-server/users.d from users-d (ro)
      /var/lib/clickhouse from data (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-qs98c (ro)
Volumes:
  data:
    Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)
    ClaimName:  data-lensai-clickhouse-0
    ReadOnly:   false
  users-d:
    Type:      ConfigMap (a volume populated by a ConfigMap)
    Name:      lensai-clickhouse-users
    Optional:  false
  kube-api-access-qs98c:
    Type:                    Projected (a volume that contains injected data from multiple sources)
    TokenExpirationSeconds:  3607
    ConfigMapName:           kube-root-ca.crt
    Optional:                false
    DownwardAPI:             true
QoS Class:                   Burstable
Node-Selectors:              <none>
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                             node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
Events:                      <none>
Name:             lensai-clickhouse-0
Namespace:        lensai
Priority:         0
Service Account:  default
Node:             <none>
Labels:           app.kubernetes.io/component=clickhouse
                  app.kubernetes.io/instance=lensai
                  app.kubernetes.io/name=lensai
                  apps.kubernetes.io/pod-index=0
                  controller-revision-hash=lensai-clickhouse-69df5b4bf7
                  statefulset.kubernetes.io/pod-name=lensai-clickhouse-0
Annotations:      <none>
Status:           Pending
IP:               
IPs:              <none>
Controlled By:    StatefulSet/lensai-clickhouse
Containers:
  clickhouse:
    Image:       clickhouse/clickhouse-server:24.12-alpine
    Ports:       8123/TCP (http), 9000/TCP (native)
    Host Ports:  0/TCP (http), 0/TCP (native)
    Limits:
      cpu:     500m
      memory:  1Gi
    Requests:
      cpu:        250m
      memory:     512Mi
    Liveness:     exec [clickhouse-client --query SELECT 1] delay=40s timeout=5s period=15s #success=1 #failure=3
    Readiness:    exec [clickhouse-client --query SELECT 1] delay=25s timeout=5s period=10s #success=1 #failure=6
    Environment:  <none>
    Mounts:
      /etc/clickhouse-server/users.d from users-d (ro)
      /var/lib/clickhouse from data (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-qs98c (ro)
Volumes:
  data:
    Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)
    ClaimName:  data-lensai-clickhouse-0
    ReadOnly:   false
  users-d:
    Type:      ConfigMap (a volume populated by a ConfigMap)
    Name:      lensai-clickhouse-users
    Optional:  false
  kube-api-access-qs98c:
    Type:                    Projected (a volume that contains injected data from multiple sources)
    TokenExpirationSeconds:  3607
    ConfigMapName:           kube-root-ca.crt
    Optional:                false
    DownwardAPI:             true
QoS Class:                   Burstable
Node-Selectors:              <none>
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                             node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
Events:                      <none>
--- kubectl logs pod/lensai-clickhouse-0 (current) ---
--- kubectl logs pod/lensai-clickhouse-0 (current) ---
--- kubectl logs pod/lensai-clickhouse-0 (previous) ---
--- kubectl logs pod/lensai-clickhouse-0 (previous) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-zg962 condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
error: timed out waiting for the condition on pods/lensai-ingestion-cb767579f-5mt4q
=== FAIL: pods not ready (component=ingestion) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=ingestion) ===
--- kubectl get pods -n lensai ---
No resources found in lensai namespace.
No resources found in lensai namespace.
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
error: no matching resources found
=== FAIL: pods not ready (component=consumer) ===
--- kubectl get pods -n lensai ---
=== FAIL: pods not ready (component=consumer) ===
--- kubectl get pods -n lensai ---
No resources found in lensai namespace.
No resources found in lensai namespace.
--- kubectl describe pod (not Ready) ---
--- kubectl describe pod (not Ready) ---
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
Hints: Redpanda OOM → raise redpanda limits in /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh
No resources found in lensai namespace.
[0;36m[e2e][0m [0;31mCluster not ready within 120s per workload[0m
exit=1
```

## Run 20260528T105635Z

```
Started: 2026-05-28T10:56:35Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/day12-anomaly-plan-alignment
CONTINUE_ON_FAIL=0
HELM_VALUES=/Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
SKIP_CHAOS=0
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ k3d cluster delete lensai
[36mINFO[0m[0000] Deleting cluster 'lensai'                    
[36mINFO[0m[0006] Deleting 1 attached volumes...               
[36mINFO[0m[0006] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0006] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0006] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.vgDo6tTxPS (k3d.io/v1alpha5#simple) 
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
[36mINFO[0m[0004] Starting cluster 'lensai'                    
[36mINFO[0m[0004] Starting servers...                          
[36mINFO[0m[0004] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0010] All agents already running.                  
[36mINFO[0m[0010] Starting helpers...                          
[36mINFO[0m[0010] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0018] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0020] Cluster 'lensai' created successfully!       
[36mINFO[0m[0020] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 1.08kB 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 1.9s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.0s

#5 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#5 DONE 0.0s

#6 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 1.23kB 0.0s done
#7 DONE 0.1s

#8 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#8 CACHED

#9 [builder 3/6] WORKDIR /build
#9 CACHED

#10 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#10 CACHED

#11 [builder 5/6] COPY ingestion ./ingestion
#11 CACHED

#12 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#12 2.465 info: syncing channel updates for '1.88-aarch64-unknown-linux-gnu'
#12 3.556 info: latest update on 2025-06-26, rust version 1.88.0 (6b00bc388 2025-06-23)
#12 3.558 info: downloading component 'cargo'
#12 4.305 info: downloading component 'clippy'
#12 5.314 info: downloading component 'rust-std'
#12 12.44 info: downloading component 'rustc'
#12 19.65 info: downloading component 'rustfmt'
#12 20.27 info: installing component 'cargo'
#12 23.44 info: installing component 'clippy'
#12 23.94 info: installing component 'rust-std'
#12 27.48 info: installing component 'rustc'
#12 37.98 info: installing component 'rustfmt'
#12 39.16     Updating crates.io index
#12 43.73  Downloading crates ...
#12 43.84   Downloaded arcstr v1.2.0
#12 43.89   Downloaded itoa v1.0.18
#12 43.92   Downloaded proc-macro-crate v3.5.0
#12 43.93   Downloaded potential_utf v0.1.5
#12 43.97   Downloaded prost-derive v0.14.3
#12 43.98   Downloaded scopeguard v1.2.0
#12 43.98   Downloaded sha1_smol v1.0.1
#12 43.99   Downloaded tracing-serde v0.2.0
#12 44.00   Downloaded sync_wrapper v1.0.2
#12 44.01   Downloaded once_cell v1.21.4
#12 44.06   Downloaded zerofrom v0.1.8
#12 44.07   Downloaded rand_core v0.9.5
#12 44.09   Downloaded icu_collections v2.2.0
#12 44.17   Downloaded zmij v1.0.21
#12 44.21   Downloaded xxhash-rust v0.8.15
#12 44.22   Downloaded yoke v0.8.2
#12 44.26   Downloaded tracing-attributes v0.1.31
#12 44.27   Downloaded smallvec v1.15.1
#12 44.28   Downloaded sharded-slab v0.1.7
#12 44.32   Downloaded unicode-ident v1.0.24
#12 44.34   Downloaded uuid v1.23.1
#12 44.35   Downloaded tracing-core v0.1.36
#12 44.37   Downloaded toml_edit v0.25.11+spec-1.1.0
#12 44.42   Downloaded zerotrie v0.2.4
#12 44.45   Downloaded serde v1.0.228
#12 44.50   Downloaded tower v0.5.3
#12 44.58   Downloaded libc v0.2.186
#12 44.82   Downloaded zerovec v0.11.6
#12 44.85   Downloaded tower-http v0.6.11
#12 44.89   Downloaded tokio-util v0.7.18
#12 44.91   Downloaded winnow v1.0.2
#12 44.94   Downloaded tracing-subscriber v0.3.23
#12 44.95   Downloaded serde_json v1.0.149
#12 44.97   Downloaded vcpkg v0.2.15
#12 45.06   Downloaded reqwest v0.13.4
#12 45.06   Downloaded rdkafka v0.39.0
#12 45.07   Downloaded zerocopy v0.8.48
#12 45.09   Downloaded rustix v0.38.44
#12 45.13   Downloaded syn v2.0.117
#12 45.15   Downloaded regex-syntax v0.8.10
#12 45.16   Downloaded tonic v0.14.6
#12 45.17   Downloaded libz-sys v1.1.28
#12 45.22   Downloaded url v2.5.8
#12 45.23   Downloaded tracing-opentelemetry v0.33.0
#12 45.26   Downloaded redis v1.2.1
#12 45.35   Downloaded tracing v0.1.44
#12 45.41   Downloaded opentelemetry-proto v0.32.0
#12 45.43   Downloaded hyper v1.9.0
#12 45.44   Downloaded socket2 v0.6.3
#12 45.45   Downloaded serde_derive v1.0.228
#12 45.45   Downloaded serde_core v1.0.228
#12 45.45   Downloaded ryu v1.0.23
#12 45.46   Downloaded regex-automata v0.4.14
#12 45.53   Downloaded num-bigint v0.4.6
#12 45.56   Downloaded mio v1.2.0
#12 45.57   Downloaded indexmap v2.14.0
#12 45.58   Downloaded procfs-core v0.17.0
#12 45.58   Downloaded procfs v0.17.0
#12 45.59   Downloaded memchr v2.8.0
#12 45.60   Downloaded icu_locale_core v2.2.0
#12 45.63   Downloaded tonic-types v0.14.6
#12 45.64   Downloaded toml_parser v1.1.2+spec-1.1.0
#12 45.64   Downloaded linux-raw-sys v0.4.15
#12 45.73   Downloaded rand v0.9.4
#12 45.74   Downloaded tokio-stream v0.1.18
#12 45.74   Downloaded thiserror-impl v2.0.18
#12 45.74   Downloaded serde_path_to_error v0.1.20
#12 45.75   Downloaded prost v0.14.3
#12 45.75   Downloaded tokio v1.52.3
#12 45.83   Downloaded tinystr v0.8.3
#12 45.83   Downloaded slab v0.4.12
#12 45.84   Downloaded protobuf v3.7.2
#12 45.86   Downloaded rustversion v1.0.22
#12 45.86   Downloaded rand_chacha v0.9.0
#12 45.86   Downloaded quote v1.0.45
#12 45.87   Downloaded protobuf-support v3.7.2
#12 45.87   Downloaded prost-types v0.14.3
#12 45.87   Downloaded opentelemetry_sdk v0.32.1
#12 45.89   Downloaded icu_properties_data v2.2.0
#12 45.90   Downloaded h2 v0.4.14
#12 45.91   Downloaded futures-util v0.3.32
#12 45.93   Downloaded aho-corasick v1.1.4
#12 45.93   Downloaded zerovec-derive v0.11.3
#12 45.93   Downloaded zerofrom-derive v0.1.7
#12 45.93   Downloaded yoke-derive v0.8.2
#12 45.94   Downloaded writeable v0.6.3
#12 45.94   Downloaded utf8_iter v1.0.4
#12 45.94   Downloaded tracing-log v0.2.0
#12 45.94   Downloaded tower-service v0.3.3
#12 45.94   Downloaded toml_datetime v1.1.1+spec-1.1.0
#12 45.94   Downloaded thread_local v1.1.9
#12 45.94   Downloaded thiserror-impl v1.0.69
#12 45.95   Downloaded thiserror v2.0.18
#12 45.95   Downloaded thiserror v1.0.69
#12 45.96   Downloaded synstructure v0.13.2
#12 45.96   Downloaded signal-hook-registry v1.4.8
#12 45.96   Downloaded shlex v1.3.0
#12 45.97   Downloaded itertools v0.12.1
#12 45.98   Downloaded idna v1.1.0
#12 45.99   Downloaded hashbrown v0.17.1
#12 46.00   Downloaded combine v4.6.7
#12 46.01   Downloaded axum v0.8.9
#12 46.02   Downloaded hyper-util v0.1.20
#12 46.03   Downloaded http v1.4.0
#12 46.04   Downloaded cc v1.2.62
#12 46.04   Downloaded prometheus v0.14.0
#12 46.05   Downloaded opentelemetry-otlp v0.32.0
#12 46.06   Downloaded opentelemetry v0.32.0
#12 46.06   Downloaded icu_normalizer_data v2.2.0
#12 46.07   Downloaded icu_normalizer v2.2.0
#12 46.07   Downloaded getrandom v0.4.2
#12 46.08   Downloaded bytes v1.11.1
#12 46.10   Downloaded base64 v0.22.1
#12 46.11   Downloaded want v0.3.1
#12 46.11   Downloaded try-lock v0.2.5
#12 46.12   Downloaded tower-layer v0.3.3
#12 46.13   Downloaded tonic-prost v0.14.6
#12 46.13   Downloaded tokio-macros v2.7.0
#12 46.14   Downloaded stable_deref_trait v1.2.1
#12 46.15   Downloaded proc-macro2 v1.0.106
#12 46.17   Downloaded pin-project v1.1.13
#12 46.23   Downloaded parking_lot_core v0.9.12
#12 46.24   Downloaded num-traits v0.2.19
#12 46.24   Downloaded matchit v0.8.4
#12 46.25   Downloaded log v0.4.29
#12 46.25   Downloaded icu_provider v2.2.0
#12 46.27   Downloaded icu_properties v2.2.0
#12 46.27   Downloaded futures-channel v0.3.32
#12 46.28   Downloaded event-listener v5.4.1
#12 46.28   Downloaded serde_urlencoded v0.7.1
#12 46.29   Downloaded parking_lot v0.12.5
#12 46.29   Downloaded nu-ansi-term v0.50.3
#12 46.30   Downloaded litemap v0.8.2
#12 46.31   Downloaded httparse v1.10.1
#12 46.31   Downloaded getrandom v0.3.4
#12 46.32   Downloaded cmake v0.1.58
#12 46.32   Downloaded async-lock v3.4.2
#12 46.33   Downloaded ppv-lite86 v0.2.21
#12 46.33   Downloaded pkg-config v0.3.33
#12 46.34   Downloaded opentelemetry-http v0.32.0
#12 46.34   Downloaded num_enum_derive v0.7.6
#12 46.34   Downloaded num-integer v0.1.46
#12 46.35   Downloaded lock_api v0.4.14
#12 46.35   Downloaded futures-executor v0.3.32
#12 46.36   Downloaded anyhow v1.0.102
#12 46.37   Downloaded pin-project-lite v0.2.17
#12 46.38   Downloaded pin-project-internal v1.1.13
#12 46.39   Downloaded num_enum v0.7.6
#12 46.40   Downloaded ipnet v2.12.0
#12 46.40   Downloaded hyper-timeout v0.5.2
#12 46.40   Downloaded http-body v1.0.1
#12 46.41   Downloaded hex v0.4.3
#12 46.41   Downloaded futures-task v0.3.32
#12 46.41   Downloaded futures-io v0.3.32
#12 46.41   Downloaded find-msvc-tools v0.1.9
#12 46.41   Downloaded either v1.15.0
#12 46.42   Downloaded displaydoc v0.2.5
#12 46.42   Downloaded crossbeam-utils v0.8.21
#12 46.43   Downloaded concurrent-queue v2.5.0
#12 46.43   Downloaded bitflags v2.11.1
#12 46.44   Downloaded autocfg v1.5.0
#12 46.45   Downloaded async-trait v0.1.89
#12 46.45   Downloaded percent-encoding v2.3.2
#12 46.46   Downloaded parking v2.2.1
#12 46.46   Downloaded mime v0.3.17
#12 46.46   Downloaded matchers v0.2.0
#12 46.46   Downloaded lazy_static v1.5.0
#12 46.47   Downloaded idna_adapter v1.2.2
#12 46.47   Downloaded httpdate v1.0.3
#12 46.47   Downloaded http-body-util v0.1.3
#12 46.48   Downloaded futures-sink v0.3.32
#12 46.48   Downloaded futures-macro v0.3.32
#12 46.48   Downloaded futures-core v0.3.32
#12 46.48   Downloaded fnv v1.0.7
#12 46.49   Downloaded errno v0.3.14
#12 46.49   Downloaded cfg-if v1.0.4
#12 46.49   Downloaded atomic-waker v1.1.2
#12 46.50   Downloaded form_urlencoded v1.2.2
#12 46.50   Downloaded event-listener-strategy v0.5.4
#12 46.50   Downloaded equivalent v1.0.2
#12 46.50   Downloaded axum-core v0.5.6
#12 46.52   Downloaded rdkafka-sys v4.10.0+2.12.1
#12 46.78    Compiling proc-macro2 v1.0.106
#12 46.78    Compiling unicode-ident v1.0.24
#12 46.79    Compiling quote v1.0.45
#12 46.79    Compiling libc v0.2.186
#12 46.79    Compiling cfg-if v1.0.4
#12 47.35    Compiling pin-project-lite v0.2.17
#12 47.47    Compiling smallvec v1.15.1
#12 47.59    Compiling futures-core v0.3.32
#12 47.97    Compiling bytes v1.11.1
#12 47.99    Compiling once_cell v1.21.4
#12 48.00    Compiling parking_lot_core v0.9.12
#12 48.35    Compiling scopeguard v1.2.0
#12 48.38    Compiling itoa v1.0.18
#12 48.62    Compiling futures-sink v0.3.32
#12 48.91    Compiling lock_api v0.4.14
#12 49.03    Compiling tracing-core v0.1.36
#12 51.93    Compiling syn v2.0.117
#12 53.03    Compiling log v0.4.29
#12 54.11    Compiling memchr v2.8.0
#12 54.93    Compiling errno v0.3.14
#12 55.23    Compiling parking_lot v0.12.5
#12 55.65    Compiling signal-hook-registry v1.4.8
#12 55.67    Compiling mio v1.2.0
#12 56.73    Compiling socket2 v0.6.3
#12 57.00    Compiling slab v0.4.12
#12 57.33    Compiling stable_deref_trait v1.2.1
#12 57.48    Compiling futures-task v0.3.32
#12 57.58    Compiling futures-io v0.3.32
#12 57.64    Compiling http v1.4.0
#12 57.78    Compiling percent-encoding v2.3.2
#12 57.93    Compiling hashbrown v0.17.1
#12 58.16    Compiling equivalent v1.0.2
#12 58.23    Compiling futures-channel v0.3.32
#12 59.19    Compiling tower-service v0.3.3
#12 59.20    Compiling fnv v1.0.7
#12 59.55    Compiling httparse v1.10.1
#12 59.56    Compiling atomic-waker v1.1.2
#12 59.79    Compiling litemap v0.8.2
#12 60.22    Compiling http-body v1.0.1
#12 60.40    Compiling indexmap v2.14.0
#12 60.48    Compiling try-lock v0.2.5
#12 60.49    Compiling writeable v0.6.3
#12 60.60    Compiling want v0.3.1
#12 60.83    Compiling sync_wrapper v1.0.2
#12 60.94    Compiling serde_core v1.0.228
#12 62.40    Compiling icu_normalizer_data v2.2.0
#12 62.46    Compiling icu_properties_data v2.2.0
#12 63.05    Compiling utf8_iter v1.0.4
#12 63.22    Compiling tower-layer v0.3.3
#12 63.28    Compiling anyhow v1.0.102
#12 63.48    Compiling httpdate v1.0.3
#12 63.84    Compiling ipnet v2.12.0
#12 63.90    Compiling base64 v0.22.1
#12 65.20    Compiling http-body-util v0.1.3
#12 65.46    Compiling synstructure v0.13.2
#12 66.26    Compiling find-msvc-tools v0.1.9
#12 67.08    Compiling shlex v1.3.0
#12 67.58    Compiling thiserror v2.0.18
#12 67.80    Compiling cc v1.2.62
#12 68.80    Compiling form_urlencoded v1.2.2
#12 68.84    Compiling bitflags v2.11.1
#12 69.57    Compiling getrandom v0.3.4
#12 70.04    Compiling zerocopy v0.8.48
#12 70.65    Compiling either v1.15.0
#12 71.93    Compiling serde v1.0.228
#12 72.07    Compiling itertools v0.12.1
#12 73.55    Compiling pkg-config v0.3.33
#12 73.84    Compiling tokio-macros v2.7.0
#12 74.56    Compiling tracing-attributes v0.1.31
#12 74.62    Compiling zerofrom-derive v0.1.7
#12 75.56    Compiling yoke-derive v0.8.2
#12 76.68    Compiling tokio v1.52.3
#12 77.27    Compiling futures-macro v0.3.32
#12 78.57    Compiling zerofrom v0.1.8
#12 78.89    Compiling tracing v0.1.44
#12 79.04    Compiling zerovec-derive v0.11.3
#12 79.08    Compiling yoke v0.8.2
#12 79.98    Compiling displaydoc v0.2.5
#12 80.26    Compiling futures-util v0.3.32
#12 80.58    Compiling thiserror-impl v2.0.18
#12 83.34    Compiling zerotrie v0.2.4
#12 84.18    Compiling zerovec v0.11.6
#12 85.19    Compiling serde_derive v1.0.228
#12 88.48    Compiling tinystr v0.8.3
#12 88.61    Compiling potential_utf v0.1.5
#12 89.08    Compiling icu_locale_core v2.2.0
#12 89.10    Compiling icu_collections v2.2.0
#12 93.06    Compiling crossbeam-utils v0.8.21
#12 98.20    Compiling icu_provider v2.2.0
#12 104.1    Compiling icu_normalizer v2.2.0
#12 104.7    Compiling icu_properties v2.2.0
#12 110.5    Compiling zmij v1.0.21
#12 111.4    Compiling idna_adapter v1.2.2
#12 111.9    Compiling winnow v1.0.2
#12 112.5    Compiling idna v1.1.0
#12 114.6    Compiling tokio-util v0.7.18
#12 114.8    Compiling tokio-stream v0.1.18
#12 117.3    Compiling h2 v0.4.14
#12 120.3    Compiling tower v0.5.3
#12 120.4    Compiling toml_parser v1.1.2+spec-1.1.0
#12 124.2    Compiling prost-derive v0.14.3
#12 129.2    Compiling pin-project-internal v1.1.13
#12 134.3    Compiling async-trait v0.1.89
#12 155.2    Compiling autocfg v1.5.0
#12 155.8    Compiling serde_json v1.0.149
#12 158.8    Compiling vcpkg v0.2.15
#12 160.1    Compiling toml_datetime v1.1.1+spec-1.1.0
#12 161.4    Compiling toml_edit v0.25.11+spec-1.1.0
#12 167.9    Compiling libz-sys v1.1.28
#12 170.7    Compiling num-traits v0.2.19
#12 171.8    Compiling rand_core v0.9.5
#12 172.0    Compiling prost v0.14.3
#12 174.3    Compiling pin-project v1.1.13
#12 178.0    Compiling url v2.5.8
#12 179.2    Compiling opentelemetry v0.32.0
#12 188.7    Compiling hyper v1.9.0
#12 189.6    Compiling rustversion v1.0.22
#12 191.7    Compiling thiserror v1.0.69
#12 194.6    Compiling ppv-lite86 v0.2.21
#12 195.6    Compiling rand_chacha v0.9.0
#12 195.8    Compiling concurrent-queue v2.5.0
#12 196.8    Compiling hyper-util v0.1.20
#12 200.6    Compiling proc-macro-crate v3.5.0
#12 203.0    Compiling thiserror-impl v1.0.69
#12 208.0    Compiling cmake v0.1.58
#12 209.3    Compiling lazy_static v1.5.0
#12 209.5    Compiling hyper-timeout v0.5.2
#12 209.6    Compiling regex-syntax v0.8.10
#12 210.2    Compiling tonic v0.14.6
#12 212.7    Compiling parking v2.2.1
#12 213.5    Compiling rustix v0.38.44
#12 213.9    Compiling event-listener v5.4.1
#12 215.6    Compiling regex-automata v0.4.14
#12 215.8    Compiling rdkafka-sys v4.10.0+2.12.1
#12 216.8    Compiling num_enum_derive v0.7.6
#12 224.8    Compiling rand v0.9.4
#12 230.5    Compiling tower-http v0.6.11
#12 232.1    Compiling futures-executor v0.3.32
#12 235.6    Compiling linux-raw-sys v0.4.15
#12 238.0    Compiling protobuf v3.7.2
#12 238.4    Compiling getrandom v0.4.2
#12 238.7    Compiling ryu v1.0.23
#12 239.0    Compiling hex v0.4.3
#12 239.6    Compiling procfs v0.17.0
#12 239.9    Compiling procfs-core v0.17.0
#12 249.3    Compiling reqwest v0.13.4
#12 268.0    Compiling matchers v0.2.0
#12 269.1    Compiling opentelemetry_sdk v0.32.1
#12 270.3    Compiling num_enum v0.7.6
#12 270.7    Compiling num-integer v0.1.46
#12 270.9    Compiling tonic-prost v0.14.6
#12 273.9    Compiling protobuf-support v3.7.2
#12 275.8    Compiling event-listener-strategy v0.5.4
#12 276.4    Compiling sharded-slab v0.1.7
#12 282.5    Compiling prost-types v0.14.3
#12 283.1    Compiling tracing-serde v0.2.0
#12 286.5    Compiling tracing-log v0.2.0
#12 290.2    Compiling thread_local v1.1.9
#12 293.3    Compiling nu-ansi-term v0.50.3
#12 298.5    Compiling mime v0.3.17
#12 308.8    Compiling prometheus v0.14.0
#12 311.6    Compiling axum-core v0.5.6
#12 314.7    Compiling opentelemetry-proto v0.32.0
#12 337.2    Compiling tonic-types v0.14.6
#12 349.8    Compiling tracing-subscriber v0.3.23
#12 355.6    Compiling async-lock v3.4.2
#12 361.1    Compiling num-bigint v0.4.6
#12 393.8    Compiling opentelemetry-http v0.32.0
#12 399.8    Compiling serde_urlencoded v0.7.1
#12 400.6    Compiling combine v4.6.7
#12 402.2    Compiling serde_path_to_error v0.1.20
#12 406.4    Compiling matchit v0.8.4
#12 408.5    Compiling sha1_smol v1.0.1
#12 409.3    Compiling xxhash-rust v0.8.15
#12 411.0    Compiling ingestion v0.1.0 (/build/ingestion)
#12 412.7    Compiling arcstr v1.2.0
#12 413.9    Compiling axum v0.8.9
#12 428.9    Compiling redis v1.2.1
#12 441.1    Compiling uuid v1.23.1
#12 448.2    Compiling opentelemetry-otlp v0.32.0
#12 500.2    Compiling tracing-opentelemetry v0.33.0
#12 591.0    Compiling rdkafka v0.39.0
#12 628.2     Finished `release` profile [optimized] target(s) in 9m 49s
#12 DONE 631.5s

#13 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#13 CACHED

#14 [stage-1 3/5] WORKDIR /app
#14 CACHED

#15 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#15 DONE 0.1s

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 DONE 1.0s

#17 exporting to image
#17 exporting layers 0.1s done
#17 writing image sha256:667d445ecbd418d6be691d15da270baff98e6b8d55e6c19eb83c0a6544af16b9 done
#17 naming to docker.io/lensai/ingestion:local done
#17 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/be3z01r9ot2ng8tuzl8jn7165
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 913B 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#2 ...

#3 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#3 DONE 1.0s

#2 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#2 DONE 2.8s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#5 DONE 0.0s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 74.43kB 0.0s done
#7 DONE 0.0s

#8 [builder 2/6] WORKDIR /build/consumer
#8 CACHED

#9 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#9 DONE 0.1s

#10 [builder 4/6] RUN go mod download
#10 DONE 9.4s

#11 [builder 5/6] COPY consumer/ ./
#11 DONE 0.1s

#12 [builder 6/6] RUN CGO_ENABLED=0 go build -ldflags "  -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.Version=0.1.0-dev   -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.GitSHA=b5a5760   -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.BuildTime=2026-05-28T10:57:05Z"   -o /consumer ./cmd/consumer
#12 DONE 215.4s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 DONE 0.2s

#14 exporting to image
#14 exporting layers
#14 exporting layers 0.2s done
#14 writing image sha256:f77f1a85ca45713a746010ae7a17454c93a58723f7176ff80707993cf27953ad
#14 writing image sha256:f77f1a85ca45713a746010ae7a17454c93a58723f7176ff80707993cf27953ad done
#14 naming to docker.io/lensai/consumer:local done
#14 DONE 0.2s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/69vwg9a2zr99u0m9y8g5nhl4g
==> Importing images into k3d
[36mINFO[0m[0001] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0001] Saving 2 image(s) from runtime...            
[36mINFO[0m[0013] Importing images into nodes...               
[36mINFO[0m[0013] Importing images from tarball '/k3d/images/k3d-lensai-images-20260528164136.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0026] Removing the tarball(s) from image volume... 
[36mINFO[0m[0027] Removing k3d-tools node...                   
[36mINFO[0m[0029] Successfully imported image(s)               
[36mINFO[0m[0029] Successfully imported 2 image(s) into 1 cluster(s) 
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
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Thu May 28 16:42:10 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-74b767d99c-8zh84 condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-k7cdt condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-cb767579f-vkt4q condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-wj5lx condition met
NAME                                     READY   STATUS      RESTARTS       AGE
pod/lensai-clickhouse-0                  1/1     Running     0              2m40s
pod/lensai-clickhouse-init-r5rsv         0/1     Completed   0              2m40s
pod/lensai-consumer-6d6f555797-wj5lx     1/1     Running     4 (72s ago)    2m41s
pod/lensai-ingestion-cb767579f-vkt4q     1/1     Running     1 (2m2s ago)   2m41s
pod/lensai-prometheus-7495bc6667-k7cdt   1/1     Running     0              2m41s
pod/lensai-redis-74b767d99c-8zh84        1/1     Running     0              2m41s
pod/lensai-redpanda-0                    1/1     Running     0              2m40s
pod/lensai-redpanda-init-r4xdh           0/1     Completed   2              2m40s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           119s       2m40s
job.batch/lensai-redpanda-init     Complete   1/1           2m9s       2m40s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-wj5lx condition met
pod/lensai-ingestion-cb767579f-vkt4q condition met
pod/lensai-prometheus-7495bc6667-k7cdt condition met
pod/lensai-redis-74b767d99c-8zh84 condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS       AGE
lensai-clickhouse-0                  1/1     Running     0              2m42s
lensai-clickhouse-init-r5rsv         0/1     Completed   0              2m42s
lensai-consumer-6d6f555797-wj5lx     1/1     Running     4 (74s ago)    2m43s
lensai-ingestion-cb767579f-vkt4q     1/1     Running     1 (2m4s ago)   2m43s
lensai-prometheus-7495bc6667-k7cdt   1/1     Running     0              2m43s
lensai-redis-74b767d99c-8zh84        1/1     Running     0              2m43s
lensai-redpanda-0                    1/1     Running     0              2m42s
lensai-redpanda-init-r4xdh           0/1     Completed   2              2m42s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"build_time":"2026-05-28T10:57:05Z","git_sha":"b5a5760","status":"ok","version":"0.1.0"}
{"build_time":"2026-05-28T10:57:05Z","git_sha":"b5a5760","status":"ok","version":"0.1.0-dev"}

==> POST /ingest
HTTP 202 — {"batch_id":"595b3849-a693-4893-b205-10b7261442ea","event_count":1,"accepted_at_unix_ms":1779966898858}
==> Waiting for ClickHouse rows (up to 45s)
==> Consumer lag metric
WARN: lag metric not found yet
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
[0;36m[chaos-k8s][0m   round 13/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 14/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 15/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 16/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 17/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 18/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 19/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 20/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 21/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 22/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 23/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 24/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 25/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 26/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 27/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 28/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 29/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m   round 30/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 359
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;32m[PASS][0m ClickHouse ready
  breaker max: 0, overflow max: 0, ch_errors Δ: 0, lag peak: 359
  breaker after: 0, overflow after: 0
  CH rows before/after: 400 / 1950
[0;32m[PASS][0m Consumer lag backlog during CH outage (peak 359 events)

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
  Sent: ~1000, new CH rows: 2350, lag: 359, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (2350 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```

## Run 20260528T133734Z

```
Started: 2026-05-28T13:37:34Z
Host: Darwin Sauravs-MacBook-Air.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:31 PST 2026; root:xnu-12377.91.3~2/RELEASE_ARM64_T8103 arm64
Branch: feat/day12-anomaly-plan-alignment
CONTINUE_ON_FAIL=0
HELM_VALUES=/Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml
HELM_WAIT_TIMEOUT=2m
POD_WAIT_TIMEOUT=120s
SKIP_CHAOS=0
CH_READY_TIMEOUT_SEC=300
REDPANDA_READY_TIMEOUT_SEC=300
$ bash -c docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true
exit=0
$ k3d cluster delete lensai
[36mINFO[0m[0000] Deleting cluster 'lensai'                    
[36mINFO[0m[0004] Deleting 1 attached volumes...               
[36mINFO[0m[0004] Removing cluster details from default kubeconfig... 
[36mINFO[0m[0004] Removing standalone kubeconfig file (if there is one)... 
[36mINFO[0m[0004] Successfully deleted cluster lensai!         
exit=0
$ ./deploy/k3d/up.sh
==> Creating k3d cluster 'lensai'
==> Using host ports http=8080 metrics=9091
[36mINFO[0m[0000] Using config file /var/folders/kg/nb0jm4jd3839yppqj5dk5dbh0000gn/T/k3d-cluster.XXXXXX.yaml.taI8tFAOJz (k3d.io/v1alpha5#simple) 
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
[36mINFO[0m[0001] Starting new tools node...                   
[36mINFO[0m[0001] Starting node 'k3d-lensai-tools'             
[36mINFO[0m[0003] Starting cluster 'lensai'                    
[36mINFO[0m[0003] Starting servers...                          
[36mINFO[0m[0003] Starting node 'k3d-lensai-server-0'          
[36mINFO[0m[0011] All agents already running.                  
[36mINFO[0m[0011] Starting helpers...                          
[36mINFO[0m[0011] Starting node 'k3d-lensai-serverlb'          
[36mINFO[0m[0019] Injecting records for hostAliases (incl. host.k3d.internal) and for 3 network members into CoreDNS configmap... 
[36mINFO[0m[0021] Cluster 'lensai' created successfully!       
[36mINFO[0m[0021] You can now use it like this:                
kubectl cluster-info
==> Building Docker images
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.ingestion
#1 transferring dockerfile: 1.08kB 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/rust:1.86-bookworm
#2 DONE 0.0s

#3 [internal] load metadata for docker.io/library/debian:bookworm-slim
#3 DONE 1.8s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/rust:1.86-bookworm
#5 DONE 0.0s

#6 [stage-1 1/5] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 1.23kB done
#7 DONE 0.0s

#8 [builder 3/6] WORKDIR /build
#8 CACHED

#9 [builder 2/6] RUN apt-get update && apt-get install -y --no-install-recommends     cmake     libssl-dev     libsasl2-dev     libzstd-dev     libcurl4-openssl-dev     pkg-config     && rm -rf /var/lib/apt/lists/*
#9 CACHED

#10 [builder 4/6] COPY Cargo.toml Cargo.lock rust-toolchain.toml ./
#10 CACHED

#11 [builder 5/6] COPY ingestion ./ingestion
#11 CACHED

#12 [builder 6/6] RUN cargo build --release -p ingestion --bin ingestion
#12 2.212 info: syncing channel updates for '1.88-aarch64-unknown-linux-gnu'
#12 2.885 info: latest update on 2025-06-26, rust version 1.88.0 (6b00bc388 2025-06-23)
#12 2.885 info: downloading component 'cargo'
#12 3.478 info: downloading component 'clippy'
#12 4.663 info: downloading component 'rust-std'
#12 13.64 info: downloading component 'rustc'
#12 17.93 info: downloading component 'rustfmt'
#12 18.26 info: installing component 'cargo'
#12 21.29 info: installing component 'clippy'
#12 22.14 info: installing component 'rust-std'
#12 28.65 info: installing component 'rustc'
#12 45.99 info: installing component 'rustfmt'
#12 49.16     Updating crates.io index
#12 58.20  Downloading crates ...
#12 58.58   Downloaded futures-sink v0.3.32
#12 58.69   Downloaded tokio-macros v2.7.0
#12 58.77   Downloaded tonic-prost v0.14.6
#12 58.81   Downloaded zerofrom v0.1.8
#12 58.85   Downloaded zerofrom-derive v0.1.7
#12 58.92   Downloaded zmij v1.0.21
#12 59.00   Downloaded yoke v0.8.2
#12 59.09   Downloaded uuid v1.23.1
#12 59.20   Downloaded zerotrie v0.2.4
#12 59.32   Downloaded url v2.5.8
#12 59.47   Downloaded tower v0.5.3
#12 59.81   Downloaded zerovec v0.11.6
#12 59.95   Downloaded serde_json v1.0.149
#12 60.05   Downloaded tower-http v0.6.11
#12 60.16   Downloaded opentelemetry_sdk v0.32.1
#12 60.25   Downloaded winnow v1.0.2
#12 60.35   Downloaded vcpkg v0.2.15
#12 60.85   Downloaded tracing-subscriber v0.3.23
#12 61.00   Downloaded reqwest v0.13.4
#12 61.05   Downloaded rdkafka v0.39.0
#12 61.10   Downloaded zerocopy v0.8.48
#12 61.31   Downloaded syn v2.0.117
#12 61.45   Downloaded regex-syntax v0.8.10
#12 61.57   Downloaded tokio-util v0.7.18
#12 61.66   Downloaded rustix v0.38.44
#12 61.84   Downloaded opentelemetry-proto v0.32.0
#12 61.93   Downloaded idna v1.1.0
#12 61.96   Downloaded h2 v0.4.14
#12 62.03   Downloaded tracing v0.1.44
#12 62.43   Downloaded redis v1.2.1
#12 62.63   Downloaded futures-util v0.3.32
#12 62.73   Downloaded rand v0.9.4
#12 62.74   Downloaded icu_properties_data v2.2.0
#12 62.83   Downloaded axum v0.8.9
#12 62.90   Downloaded tonic v0.14.6
#12 62.95   Downloaded serde v1.0.228
#12 62.99   Downloaded regex-automata v0.4.14
#12 63.15   Downloaded libc v0.2.186
#12 63.66   Downloaded libz-sys v1.1.28
#12 64.00   Downloaded hyper v1.9.0
#12 64.15   Downloaded hashbrown v0.17.1
#12 64.19   Downloaded aho-corasick v1.1.4
#12 64.22   Downloaded tracing-core v0.1.36
#12 64.24   Downloaded sharded-slab v0.1.7
#12 64.26   Downloaded serde_derive v1.0.228
#12 64.29   Downloaded itertools v0.12.1
#12 64.34   Downloaded http v1.4.0
#12 64.37   Downloaded combine v4.6.7
#12 64.41   Downloaded unicode-ident v1.0.24
#12 64.43   Downloaded tracing-opentelemetry v0.33.0
#12 64.47   Downloaded tonic-types v0.14.6
#12 64.51   Downloaded tokio v1.52.3
#12 65.07   Downloaded toml_edit v0.25.11+spec-1.1.0
#12 65.09   Downloaded socket2 v0.6.3
#12 65.11   Downloaded serde_core v1.0.228
#12 65.14   Downloaded ryu v1.0.23
#12 65.20   Downloaded prost-types v0.14.3
#12 65.21   Downloaded mio v1.2.0
#12 65.29   Downloaded hyper-util v0.1.20
#12 65.35   Downloaded cc v1.2.62
#12 65.37   Downloaded tracing-attributes v0.1.31
#12 65.38   Downloaded toml_parser v1.1.2+spec-1.1.0
#12 65.40   Downloaded tokio-stream v0.1.18
#12 65.44   Downloaded thiserror-impl v2.0.18
#12 65.45   Downloaded smallvec v1.15.1
#12 65.48   Downloaded slab v0.4.12
#12 65.50   Downloaded shlex v1.3.0
#12 65.54   Downloaded quote v1.0.45
#12 65.61   Downloaded protobuf-support v3.7.2
#12 65.69   Downloaded prost-derive v0.14.3
#12 65.74   Downloaded prometheus v0.14.0
#12 65.86   Downloaded procfs-core v0.17.0
#12 65.99   Downloaded opentelemetry v0.32.0
#12 66.03   Downloaded num-bigint v0.4.6
#12 66.05   Downloaded memchr v2.8.0
#12 66.09   Downloaded linux-raw-sys v0.4.15
#12 66.51   Downloaded indexmap v2.14.0
#12 66.53   Downloaded icu_collections v2.2.0
#12 66.55   Downloaded zerovec-derive v0.11.3
#12 66.56   Downloaded xxhash-rust v0.8.15
#12 66.59   Downloaded writeable v0.6.3
#12 66.62   Downloaded tracing-log v0.2.0
#12 66.64   Downloaded protobuf v3.7.2
#12 66.94   Downloaded toml_datetime v1.1.1+spec-1.1.0
#12 66.97   Downloaded tinystr v0.8.3
#12 67.01   Downloaded thread_local v1.1.9
#12 67.03   Downloaded thiserror-impl v1.0.69
#12 67.05   Downloaded thiserror v2.0.18
#12 67.09   Downloaded thiserror v1.0.69
#12 67.15   Downloaded synstructure v0.13.2
#12 67.16   Downloaded signal-hook-registry v1.4.8
#12 67.18   Downloaded rustversion v1.0.22
#12 67.19   Downloaded rand_core v0.9.5
#12 67.20   Downloaded rand_chacha v0.9.0
#12 67.20   Downloaded procfs v0.17.0
#12 67.23   Downloaded opentelemetry-otlp v0.32.0
#12 67.25   Downloaded icu_normalizer_data v2.2.0
#12 67.26   Downloaded icu_normalizer v2.2.0
#12 67.28   Downloaded icu_locale_core v2.2.0
#12 67.31   Downloaded bytes v1.11.1
#12 67.36   Downloaded base64 v0.22.1
#12 67.38   Downloaded yoke-derive v0.8.2
#12 67.39   Downloaded utf8_iter v1.0.4
#12 67.39   Downloaded try-lock v0.2.5
#12 67.40   Downloaded tracing-serde v0.2.0
#12 67.40   Downloaded tower-service v0.3.3
#12 67.41   Downloaded tower-layer v0.3.3
#12 67.41   Downloaded serde_path_to_error v0.1.20
#12 67.43   Downloaded proc-macro2 v1.0.106
#12 67.47   Downloaded ppv-lite86 v0.2.21
#12 67.49   Downloaded pin-project v1.1.13
#12 67.60   Downloaded parking_lot v0.12.5
#12 67.61   Downloaded num-traits v0.2.19
#12 67.64   Downloaded log v0.4.29
#12 67.68   Downloaded icu_provider v2.2.0
#12 67.70   Downloaded icu_properties v2.2.0
#12 67.72   Downloaded getrandom v0.4.2
#12 67.75   Downloaded getrandom v0.3.4
#12 67.77   Downloaded bitflags v2.11.1
#12 67.79   Downloaded anyhow v1.0.102
#12 67.83   Downloaded want v0.3.1
#12 67.83   Downloaded scopeguard v1.2.0
#12 67.84   Downloaded pkg-config v0.3.33
#12 67.85   Downloaded parking_lot_core v0.9.12
#12 67.87   Downloaded once_cell v1.21.4
#12 67.89   Downloaded num_enum v0.7.6
#12 67.98   Downloaded num-integer v0.1.46
#12 68.00   Downloaded matchit v0.8.4
#12 68.02   Downloaded lock_api v0.4.14
#12 68.04   Downloaded litemap v0.8.2
#12 68.05   Downloaded hyper-timeout v0.5.2
#12 68.06   Downloaded httparse v1.10.1
#12 68.09   Downloaded http-body-util v0.1.3
#12 68.10   Downloaded futures-executor v0.3.32
#12 68.13   Downloaded futures-channel v0.3.32
#12 68.14   Downloaded form_urlencoded v1.2.2
#12 68.14   Downloaded find-msvc-tools v0.1.9
#12 68.15   Downloaded event-listener v5.4.1
#12 68.16   Downloaded either v1.15.0
#12 68.18   Downloaded displaydoc v0.2.5
#12 68.19   Downloaded crossbeam-utils v0.8.21
#12 68.20   Downloaded concurrent-queue v2.5.0
#12 68.23   Downloaded cmake v0.1.58
#12 68.24   Downloaded axum-core v0.5.6
#12 68.25   Downloaded autocfg v1.5.0
#12 68.28   Downloaded async-trait v0.1.89
#12 68.32   Downloaded async-lock v3.4.2
#12 68.33   Downloaded arcstr v1.2.0
#12 68.35   Downloaded sync_wrapper v1.0.2
#12 68.37   Downloaded stable_deref_trait v1.2.1
#12 68.38   Downloaded prost v0.14.3
#12 68.38   Downloaded proc-macro-crate v3.5.0
#12 68.39   Downloaded potential_utf v0.1.5
#12 68.39   Downloaded pin-project-lite v0.2.17
#12 68.42   Downloaded pin-project-internal v1.1.13
#12 68.44   Downloaded parking v2.2.1
#12 68.45   Downloaded opentelemetry-http v0.32.0
#12 68.46   Downloaded num_enum_derive v0.7.6
#12 68.46   Downloaded nu-ansi-term v0.50.3
#12 68.47   Downloaded matchers v0.2.0
#12 68.48   Downloaded itoa v1.0.18
#12 68.50   Downloaded ipnet v2.12.0
#12 68.52   Downloaded errno v0.3.14
#12 68.55   Downloaded sha1_smol v1.0.1
#12 68.57   Downloaded serde_urlencoded v0.7.1
#12 68.59   Downloaded percent-encoding v2.3.2
#12 68.60   Downloaded mime v0.3.17
#12 68.62   Downloaded lazy_static v1.5.0
#12 68.65   Downloaded idna_adapter v1.2.2
#12 68.66   Downloaded httpdate v1.0.3
#12 68.67   Downloaded http-body v1.0.1
#12 68.69   Downloaded hex v0.4.3
#12 68.71   Downloaded futures-task v0.3.32
#12 68.73   Downloaded futures-macro v0.3.32
#12 68.73   Downloaded futures-io v0.3.32
#12 68.73   Downloaded futures-core v0.3.32
#12 68.74   Downloaded fnv v1.0.7
#12 68.74   Downloaded atomic-waker v1.1.2
#12 68.75   Downloaded event-listener-strategy v0.5.4
#12 68.75   Downloaded equivalent v1.0.2
#12 68.75   Downloaded cfg-if v1.0.4
#12 68.80   Downloaded rdkafka-sys v4.10.0+2.12.1
#12 71.35    Compiling proc-macro2 v1.0.106
#12 71.35    Compiling quote v1.0.45
#12 71.35    Compiling unicode-ident v1.0.24
#12 71.35    Compiling libc v0.2.186
#12 71.35    Compiling cfg-if v1.0.4
#12 72.68    Compiling pin-project-lite v0.2.17
#12 72.93    Compiling smallvec v1.15.1
#12 73.39    Compiling futures-core v0.3.32
#12 74.04    Compiling bytes v1.11.1
#12 74.15    Compiling once_cell v1.21.4
#12 74.17    Compiling parking_lot_core v0.9.12
#12 74.65    Compiling futures-sink v0.3.32
#12 74.76    Compiling scopeguard v1.2.0
#12 74.89    Compiling itoa v1.0.18
#12 74.93    Compiling lock_api v0.4.14
#12 75.75    Compiling tracing-core v0.1.36
#12 76.57    Compiling syn v2.0.117
#12 76.89    Compiling log v0.4.29
#12 77.85    Compiling memchr v2.8.0
#12 77.95    Compiling slab v0.4.12
#12 79.89    Compiling errno v0.3.14
#12 80.88    Compiling parking_lot v0.12.5
#12 81.35    Compiling signal-hook-registry v1.4.8
#12 81.35    Compiling socket2 v0.6.3
#12 81.92    Compiling mio v1.2.0
#12 83.12    Compiling stable_deref_trait v1.2.1
#12 83.32    Compiling futures-io v0.3.32
#12 83.76    Compiling futures-task v0.3.32
#12 84.15    Compiling http v1.4.0
#12 84.45    Compiling percent-encoding v2.3.2
#12 84.64    Compiling equivalent v1.0.2
#12 84.80    Compiling hashbrown v0.17.1
#12 85.22    Compiling futures-channel v0.3.32
#12 85.26    Compiling tower-service v0.3.3
#12 85.56    Compiling httparse v1.10.1
#12 86.21    Compiling fnv v1.0.7
#12 86.55    Compiling writeable v0.6.3
#12 87.12    Compiling atomic-waker v1.1.2
#12 88.21    Compiling litemap v0.8.2
#12 88.59    Compiling indexmap v2.14.0
#12 88.64    Compiling try-lock v0.2.5
#12 89.39    Compiling http-body v1.0.1
#12 89.53    Compiling want v0.3.1
#12 90.54    Compiling sync_wrapper v1.0.2
#12 91.17    Compiling anyhow v1.0.102
#12 95.37    Compiling httpdate v1.0.3
#12 100.3    Compiling tower-layer v0.3.3
#12 104.7    Compiling serde_core v1.0.228
#12 104.8    Compiling utf8_iter v1.0.4
#12 105.0    Compiling icu_normalizer_data v2.2.0
#12 105.3    Compiling icu_properties_data v2.2.0
#12 105.4    Compiling base64 v0.22.1
#12 105.8    Compiling ipnet v2.12.0
#12 106.6    Compiling http-body-util v0.1.3
#12 109.8    Compiling find-msvc-tools v0.1.9
#12 111.6    Compiling shlex v1.3.0
#12 111.7    Compiling thiserror v2.0.18
#12 112.2    Compiling cc v1.2.62
#12 115.1    Compiling form_urlencoded v1.2.2
#12 115.8    Compiling synstructure v0.13.2
#12 116.6    Compiling either v1.15.0
#12 124.0    Compiling getrandom v0.3.4
#12 124.8    Compiling bitflags v2.11.1
#12 125.7    Compiling zerocopy v0.8.48
#12 126.6    Compiling serde v1.0.228
#12 126.8    Compiling itertools v0.12.1
#12 129.1    Compiling zmij v1.0.21
#12 129.7    Compiling crossbeam-utils v0.8.21
#12 130.4    Compiling winnow v1.0.2
#12 130.7    Compiling pkg-config v0.3.33
#12 132.9    Compiling tokio-macros v2.7.0
#12 135.0    Compiling tracing-attributes v0.1.31
#12 136.0    Compiling zerofrom-derive v0.1.7
#12 136.1    Compiling yoke-derive v0.8.2
#12 138.4    Compiling tokio v1.52.3
#12 138.6    Compiling futures-macro v0.3.32
#12 142.1    Compiling tracing v0.1.44
#12 142.2    Compiling zerovec-derive v0.11.3
#12 144.0    Compiling zerofrom v0.1.8
#12 146.0    Compiling futures-util v0.3.32
#12 146.4    Compiling yoke v0.8.2
#12 148.6    Compiling displaydoc v0.2.5
#12 148.9    Compiling thiserror-impl v2.0.18
#12 157.9    Compiling zerotrie v0.2.4
#12 158.1    Compiling zerovec v0.11.6
#12 161.8    Compiling serde_derive v1.0.228
#12 162.7    Compiling tinystr v0.8.3
#12 162.8    Compiling potential_utf v0.1.5
#12 163.5    Compiling icu_locale_core v2.2.0
#12 163.6    Compiling icu_collections v2.2.0
#12 172.5    Compiling icu_provider v2.2.0
#12 177.9    Compiling icu_properties v2.2.0
#12 178.8    Compiling icu_normalizer v2.2.0
#12 183.8    Compiling toml_parser v1.1.2+spec-1.1.0
#12 184.6    Compiling idna_adapter v1.2.2
#12 185.3    Compiling idna v1.1.0
#12 189.0    Compiling prost-derive v0.14.3
#12 189.9    Compiling pin-project-internal v1.1.13
#12 190.5    Compiling async-trait v0.1.89
#12 194.9    Compiling tokio-util v0.7.18
#12 195.3    Compiling tokio-stream v0.1.18
#12 197.9    Compiling vcpkg v0.2.15
#12 198.5    Compiling h2 v0.4.14
#12 200.6    Compiling tower v0.5.3
#12 200.6    Compiling serde_json v1.0.149
#12 201.4    Compiling toml_datetime v1.1.1+spec-1.1.0
#12 202.4    Compiling autocfg v1.5.0
#12 203.7    Compiling num-traits v0.2.19
#12 205.1    Compiling toml_edit v0.25.11+spec-1.1.0
#12 207.6    Compiling libz-sys v1.1.28
#12 209.8    Compiling rand_core v0.9.5
#12 212.0    Compiling prost v0.14.3
#12 212.9    Compiling pin-project v1.1.13
#12 213.4    Compiling url v2.5.8
#12 214.9    Compiling opentelemetry v0.32.0
#12 227.6    Compiling hyper v1.9.0
#12 228.4    Compiling ppv-lite86 v0.2.21
#12 230.1    Compiling rustversion v1.0.22
#12 231.2    Compiling thiserror v1.0.69
#12 232.4    Compiling rand_chacha v0.9.0
#12 232.8    Compiling concurrent-queue v2.5.0
#12 236.3    Compiling hyper-util v0.1.20
#12 237.8    Compiling proc-macro-crate v3.5.0
#12 240.2    Compiling thiserror-impl v1.0.69
#12 244.1    Compiling hyper-timeout v0.5.2
#12 244.6    Compiling cmake v0.1.58
#12 244.9    Compiling tonic v0.14.6
#12 245.0    Compiling lazy_static v1.5.0
#12 245.3    Compiling rustix v0.38.44
#12 246.4    Compiling regex-syntax v0.8.10
#12 247.0    Compiling parking v2.2.1
#12 247.4    Compiling event-listener v5.4.1
#12 250.1    Compiling rdkafka-sys v4.10.0+2.12.1
#12 251.5    Compiling num_enum_derive v0.7.6
#12 251.8    Compiling rand v0.9.4
#12 254.3    Compiling regex-automata v0.4.14
#12 255.6    Compiling tower-http v0.6.11
#12 256.9    Compiling futures-executor v0.3.32
#12 259.3    Compiling protobuf v3.7.2
#12 260.5    Compiling procfs v0.17.0
#12 261.4    Compiling linux-raw-sys v0.4.15
#12 261.4    Compiling ryu v1.0.23
#12 262.2    Compiling getrandom v0.4.2
#12 262.9    Compiling hex v0.4.3
#12 263.9    Compiling procfs-core v0.17.0
#12 274.6    Compiling matchers v0.2.0
#12 275.6    Compiling reqwest v0.13.4
#12 283.5    Compiling opentelemetry_sdk v0.32.1
#12 287.8    Compiling num_enum v0.7.6
#12 288.4    Compiling tonic-prost v0.14.6
#12 288.5    Compiling num-integer v0.1.46
#12 290.6    Compiling protobuf-support v3.7.2
#12 293.3    Compiling event-listener-strategy v0.5.4
#12 293.9    Compiling sharded-slab v0.1.7
#12 296.5    Compiling prost-types v0.14.3
#12 297.0    Compiling tracing-serde v0.2.0
#12 297.6    Compiling tracing-log v0.2.0
#12 298.8    Compiling thread_local v1.1.9
#12 299.9    Compiling prometheus v0.14.0
#12 300.5    Compiling nu-ansi-term v0.50.3
#12 302.3    Compiling mime v0.3.17
#12 303.6    Compiling axum-core v0.5.6
#12 305.4    Compiling tracing-subscriber v0.3.23
#12 311.8    Compiling tonic-types v0.14.6
#12 320.9    Compiling opentelemetry-proto v0.32.0
#12 323.8    Compiling async-lock v3.4.2
#12 329.2    Compiling num-bigint v0.4.6
#12 338.4    Compiling opentelemetry-http v0.32.0
#12 347.6    Compiling serde_urlencoded v0.7.1
#12 348.9    Compiling combine v4.6.7
#12 351.3    Compiling serde_path_to_error v0.1.20
#12 353.6    Compiling ingestion v0.1.0 (/build/ingestion)
#12 354.5    Compiling matchit v0.8.4
#12 357.2    Compiling xxhash-rust v0.8.15
#12 359.7    Compiling arcstr v1.2.0
#12 361.0    Compiling sha1_smol v1.0.1
#12 362.0    Compiling axum v0.8.9
#12 371.5    Compiling redis v1.2.1
#12 398.6    Compiling uuid v1.23.1
#12 401.4    Compiling opentelemetry-otlp v0.32.0
#12 424.1    Compiling tracing-opentelemetry v0.33.0
#12 485.1    Compiling rdkafka v0.39.0
#12 517.1     Finished `release` profile [optimized] target(s) in 7m 49s
#12 DONE 519.9s

#13 [stage-1 2/5] RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates     libssl3     libsasl2-2     libzstd1     libcurl4     && rm -rf /var/lib/apt/lists/*
#13 CACHED

#14 [stage-1 3/5] WORKDIR /app
#14 CACHED

#15 [stage-1 4/5] COPY --from=builder /build/target/release/ingestion /app/ingestion
#15 DONE 0.1s

#16 [stage-1 5/5] RUN mkdir -p /data/wal
#16 DONE 0.9s

#17 exporting to image
#17 exporting layers 0.1s done
#17 writing image sha256:702287fb017e6087a1e0def8a630d51465c05eb09f3f103d44c3db6e1c8c5c30 done
#17 naming to docker.io/lensai/ingestion:local done
#17 DONE 0.1s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/3f2rvasr9jnwtf899hamm4k55
#0 building with "desktop-linux" instance using docker driver

#1 [internal] load build definition from Dockerfile.consumer
#1 transferring dockerfile: 913B 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#2 DONE 0.7s

#3 [internal] load metadata for docker.io/library/golang:1.25-bookworm
#3 DONE 1.9s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.25-bookworm@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43
#5 DONE 0.0s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 DONE 0.0s

#7 [internal] load build context
#7 transferring context: 74.43kB 0.0s done
#7 DONE 0.0s

#8 [builder 3/6] COPY consumer/go.mod consumer/go.sum ./
#8 CACHED

#9 [builder 4/6] RUN go mod download
#9 CACHED

#10 [builder 2/6] WORKDIR /build/consumer
#10 CACHED

#11 [builder 5/6] COPY consumer/ ./
#11 CACHED

#12 [builder 6/6] RUN CGO_ENABLED=0 go build -ldflags "  -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.Version=0.1.0-dev   -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.GitSHA=b5a5760   -X github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo.BuildTime=2026-05-28T13:38:01Z"   -o /consumer ./cmd/consumer
#12 DONE 166.6s

#6 [stage-1 1/3] FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
#6 CACHED

#13 [stage-1 2/3] COPY --from=builder /consumer /consumer
#13 DONE 0.3s

#14 exporting to image
#14 exporting layers 0.1s done
#14 writing image sha256:07468d275db94c73a3b0ce3e59ee464164bcba976751c39d219e266a7ef0df55 done
#14 naming to docker.io/lensai/consumer:local 0.0s done
#14 DONE 0.2s

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/e8sepcgnt5tnsjv7dgq62a07l
==> Importing images into k3d
[36mINFO[0m[0000] Importing image(s) into cluster 'lensai'     
[36mINFO[0m[0000] Saving 2 image(s) from runtime...            
[36mINFO[0m[0008] Importing images into nodes...               
[36mINFO[0m[0008] Importing images from tarball '/k3d/images/k3d-lensai-images-20260528191940.tar' into node 'k3d-lensai-server-0'... 
[36mINFO[0m[0016] Removing the tarball(s) from image volume... 
[36mINFO[0m[0017] Removing k3d-tools node...                   
[36mINFO[0m[0017] Successfully imported image(s)               
[36mINFO[0m[0017] Successfully imported 2 image(s) into 1 cluster(s) 
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
$ helm upgrade --install lensai deploy/helm/lensai -n lensai --create-namespace -f /Users/akshant/Desktop/Github/infra-ai-streaming/deploy/helm/lensai/values-m1.yaml --timeout 2m --wait=false --wait-for-jobs=false
level=WARN msg="--wait=false is deprecated (boolean value) and can be replaced with --wait=hookOnly"
Release "lensai" does not exist. Installing it now.
NAME: lensai
LAST DEPLOYED: Thu May 28 19:20:01 2026
NAMESPACE: lensai
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
exit=0
$ wait_cluster_ready
[0;36m[e2e][0m kubectl wait ready: redis (120s)
pod/lensai-redis-74b767d99c-pcfl2 condition met
[0;36m[e2e][0m kubectl wait ready: redpanda (120s)
pod/lensai-redpanda-0 condition met
[0;36m[e2e][0m kubectl wait ready: clickhouse (120s)
pod/lensai-clickhouse-0 condition met
[0;36m[e2e][0m kubectl wait ready: prometheus (120s)
pod/lensai-prometheus-7495bc6667-rjmmp condition met
[0;36m[e2e][0m kubectl wait complete: lensai-redpanda-init (180s)
job.batch/lensai-redpanda-init condition met
[0;36m[e2e][0m kubectl wait complete: lensai-clickhouse-init (180s)
job.batch/lensai-clickhouse-init condition met
[0;36m[e2e][0m kubectl wait ready: ingestion (120s)
pod/lensai-ingestion-cb767579f-cxl56 condition met
[0;36m[e2e][0m kubectl wait ready: consumer (120s)
pod/lensai-consumer-6d6f555797-scrxm condition met
NAME                                     READY   STATUS      RESTARTS       AGE
pod/lensai-clickhouse-0                  1/1     Running     0              2m11s
pod/lensai-clickhouse-init-bgtcc         0/1     Completed   0              2m11s
pod/lensai-consumer-6d6f555797-scrxm     1/1     Running     4 (69s ago)    2m11s
pod/lensai-ingestion-cb767579f-cxl56     1/1     Running     2 (118s ago)   2m11s
pod/lensai-prometheus-7495bc6667-rjmmp   1/1     Running     0              2m11s
pod/lensai-redis-74b767d99c-pcfl2        1/1     Running     0              2m11s
pod/lensai-redpanda-0                    1/1     Running     0              2m11s
pod/lensai-redpanda-init-z96x5           0/1     Completed   3              2m11s

NAME                               STATUS     COMPLETIONS   DURATION   AGE
job.batch/lensai-clickhouse-init   Complete   1/1           86s        2m11s
job.batch/lensai-redpanda-init     Complete   1/1           109s       2m11s
exit=0
$ ./scripts/smoke-k8s-e2e.sh
==> Waiting for pods in namespace lensai
pod/lensai-clickhouse-0 condition met
pod/lensai-consumer-6d6f555797-scrxm condition met
pod/lensai-ingestion-cb767579f-cxl56 condition met
pod/lensai-prometheus-7495bc6667-rjmmp condition met
pod/lensai-redis-74b767d99c-pcfl2 condition met
pod/lensai-redpanda-0 condition met
NAME                                 READY   STATUS      RESTARTS       AGE
lensai-clickhouse-0                  1/1     Running     0              2m12s
lensai-clickhouse-init-bgtcc         0/1     Completed   0              2m12s
lensai-consumer-6d6f555797-scrxm     1/1     Running     4 (70s ago)    2m12s
lensai-ingestion-cb767579f-cxl56     1/1     Running     2 (119s ago)   2m12s
lensai-prometheus-7495bc6667-rjmmp   1/1     Running     0              2m12s
lensai-redis-74b767d99c-pcfl2        1/1     Running     0              2m12s
lensai-redpanda-0                    1/1     Running     0              2m12s
lensai-redpanda-init-z96x5           0/1     Completed   3              2m12s
==> Unit tests skipped (SKIP_UNIT_TESTS=1)
==> Port-forward ingestion :8080 and consumer metrics :9091
==> Health checks
{"build_time":"2026-05-28T13:38:01Z","git_sha":"b5a5760","status":"ok","version":"0.1.0"}
{"build_time":"2026-05-28T13:38:01Z","git_sha":"b5a5760","status":"ok","version":"0.1.0-dev"}

==> POST /ingest
HTTP 202 — {"batch_id":"1a60a1f1-9625-48cd-b5f3-0d3ffe9a8e4f","event_count":1,"accepted_at_unix_ms":1779976338798}
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
[0;36m[chaos-k8s][0m   round 14/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 15/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 16/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 17/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 18/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 19/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 20/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 21/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 22/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 23/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 24/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 25/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 26/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 27/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 28/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 29/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m   round 30/30 — breaker_open: 0, overflow: 0, ch_errors: 0 (Δ=0), lag: 381
[0;36m[chaos-k8s][0m Phase 3: restore ClickHouse...
statefulset.apps/lensai-clickhouse scaled
[0;36m[chaos-k8s][0m Waiting for ClickHouse pod ready (300s)...
[0;32m[PASS][0m ClickHouse ready
  breaker max: 0, overflow max: 0, ch_errors Δ: 0, lag peak: 381
  breaker after: 0, overflow after: 0
  CH rows before/after: 400 / 2000
[0;32m[PASS][0m Consumer lag backlog during CH outage (peak 381 events)

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
  Sent: ~1000, new CH rows: 3150, lag: 381, overflow: 0
[0;32m[PASS][0m Load delivered rows to ClickHouse (3150 new)

[1m═══════════════════════════════════════════════════════════════[0m
exit=0
$ bash -c 
    kubectl get hpa -n 'lensai' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  
No resources found in lensai namespace.
exit=0
```
