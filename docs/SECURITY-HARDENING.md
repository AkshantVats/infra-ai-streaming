# Security hardening (lightweight)

Guidance for moving from **local M1/k3d** to a real Kubernetes deployment.
This repo does not ship a production ingress controller or cert-manager chart.

## TLS termination

- **Do not expose ingestion plain HTTP on the public internet.**
- Terminate TLS at an **ingress controller** (nginx, Envoy Gateway, cloud LB) or service mesh.
- Prefer **mTLS** between ingress → ingestion pods when crossing untrusted networks.
- Kafka/Redpanda: enable TLS + SASL in broker config; set `KAFKA_BROKERS` to `SSL://` endpoints in consumer/ingestion env (not wired in M1 values).

## Secrets management

- **Never commit** `.env`, kubeconfig, API keys, or `credentials.json`.
- Use **Kubernetes Secrets** (or External Secrets Operator → Vault/SSM) for:
  - `REDIS_URL` passwords
  - ClickHouse credentials
  - Kafka SASL credentials
- Helm: reference existing secrets via `values` overrides; avoid plain-text secrets in `values-prod.yaml` checked into git.
- **No `.env` in production pods** — inject env from Secret/ConfigMap mounts.

## Ingestion authentication (gap)

`/ingest` has **no AuthN/AuthZ** in-tree. Before production:

- Place an API gateway (API keys, OIDC) in front of ingestion, or
- Add mTLS client cert validation at the ingress.

See [PRODUCTION-READINESS-CHECKLIST.md](PRODUCTION-READINESS-CHECKLIST.md).

## Supply chain

- CI runs **gitleaks** on every PR ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml)).
- Pin image digests in production (`image@sha256:…`) instead of `:latest`.
- Enable Dependabot ([`.github/dependabot.yml`](../.github/dependabot.yml)) and review Rust/Go/Docker bumps.

## Network policy (recommended)

- Restrict ingestion to ingress namespace only.
- Consumer → ClickHouse/Redis/Kafka only; no egress to arbitrary hosts.
- Deny pod-to-pod lateral movement where possible (CNI-dependent).

## Reporting vulnerabilities

See [SECURITY.md](../SECURITY.md) for private disclosure.
