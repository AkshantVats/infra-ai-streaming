# distributed-flagd — Design Document

## Problem Statement

LaunchDarkly Pro at 250M+ monthly evaluations exceeds $1,800/month.
We need a self-hosted feature flag control plane optimized for AI model rollouts:
deterministic percentage routing, gRPC streaming for sub-10ms propagation,
and an immutable audit log for every flag change.

## Architecture

### Data model — etcd key schema

| Prefix | Key pattern | Value |
|---|---|---|
| `/flags/` | `/flags/{name}` | FlagData JSON |
| `/audit/` | `/audit/{name}/{unix_ns}` | Entry JSON (90-day TTL) |
| `/locks/` | `/locks/{name}` | CAS lock for concurrent writers |

### Core components

1. **Flag Store** (`internal/etcdstore`) — etcd-backed KV, `/flags/` prefix
2. **Evaluator** (`internal/eval`) — FNV-1a hash mod 10000, O(variants) walk
3. **gRPC Server** (`internal/server`) — EvaluateStream: SNAPSHOT on connect, DELTA via Watch
4. **Audit Logger** (`internal/audit`) — 6-field entries, 90-day TTL via etcd lease

### gRPC streaming protocol

```
Client connects → server sends SNAPSHOT (all current flags)
Server receives etcd Watch event → server sends DELTA (changed flag)
Client reconnects → repeat SNAPSHOT
```

Clients store the flag map in-process (~100ns eval latency with no network hop).
On reconnect, the client re-requests SNAPSHOT to recover missed deltas.

### Percentage rollout algorithm

```
bucket = FNV1a(flag_name + ":" + hash_key) mod 10000
walk variants by cumulative weight (weight × 100 = bucket space units)
return first variant where bucket < cumulative
```

Bucket space is 10,000 so weight precision is 0.01%. Deterministic:
identical inputs always return the same variant. A 10% treatment weight
occupies buckets 0–999 of the 0–9999 space.

### Audit log fields

All six fields are required. Zero values are not valid.

| Field | Type | Description |
|---|---|---|
| `flag_name` | string | Flag identifier |
| `old_value` | string | Value before the change |
| `new_value` | string | Value after the change |
| `changed_by` | string | User or service making the change |
| `changed_at` | int64 | Unix nanoseconds (set by audit logger) |
| `evaluation_count_snapshot` | int64 | Eval count at time of change (caller-provided) |

Entries are stored at `/audit/{flag_name}/{unix_ns}` with a 90-day etcd lease.
This makes the audit log naturally ordered, unique, and self-expiring.

## Acceptance criteria

- [ ] `SetFlag` persists to etcd; `GetFlag` returns correct value within 50ms
- [ ] `EvaluateStream` client receives SNAPSHOT immediately on connect
- [ ] `EvaluateStream` client receives DELTA within 50ms of a `SetFlag` call
- [ ] Percentage rollout: FNV-1a distribution ≤2% deviation from expected weights at 10k samples
- [ ] Audit entry created for every `SetFlag` call; entry expires after 90 days
- [ ] `make docker-up && grpcurl -plaintext localhost:50051 flagd.v1.FlagService/ListFlags` returns empty list

## Why self-hosted over LaunchDarkly

LaunchDarkly Pro at 250M evaluations/month: ~$1,800/month.
Self-hosted etcd + Go server on a single t3.small: ~$30/month.

The primary driver is AI model rollout: routing 10% of inference requests to
`gpt-4o` vs `claude-3-5-sonnet` requires deterministic percentage hashing,
not available on LaunchDarkly's Basic tier. The flag propagation latency
(<10ms via gRPC Watch) matches what Delivery Hero's Route Service required
for config propagation at 10k+ concurrent requests.

See the companion Experience post:
https://akshantvats.github.io/Profile/blog/series/experience/day-21-launchdarkly-build-vs-buy-flagd.html
