# distributed-flagd

Self-hosted feature flag control plane for AI model rollouts.

- **Deterministic percentage routing** via FNV-1a hashing (`flag_name:request_id`)
- **gRPC streaming** — SNAPSHOT on connect, DELTA on every flag change
- **Immutable audit log** — 90-day TTL via etcd lease
- **etcd backend** — `/flags/`, `/audit/`, `/locks/` key schema

## Quickstart

```bash
# 1. Start etcd
make docker-up

# 2. Generate proto stubs (requires protoc + protoc-gen-go)
make proto

# 3. Build
make build

# 4. Run tests
make test

# 5. Smoke test
grpcurl -plaintext localhost:50051 flagd.v1.FlagService/ListFlags
```

## Architecture

```
client → gRPC GetFlag/EvaluateStream
          ↓
       server.go   ← etcdstore.Client ← etcd /flags/
          ↓
       eval.EvaluatePercentage  (FNV-1a, O(variants))
          ↓
       audit.Logger → etcd /audit/{name}/{unix_ns}
```

See [DESIGN.md](DESIGN.md) for full design rationale, data model, and acceptance criteria.

## Day 21

Built as part of a 150-day distributed systems + AI infrastructure learning series.

- Experience post: https://akshantvats.github.io/Profile/blog/series/experience/day-21-launchdarkly-build-vs-buy-flagd.html
- AI Learning post: https://akshantvats.github.io/Profile/blog/series/ai-learning/day-21-production-reliability-llm-apis.html

> **Note:** This scaffold lives in `infra-ai-streaming/distributed-flagd/` pending
> creation of a dedicated `akshantvats/distributed-flagd` repository.
