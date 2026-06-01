# Contributing

## Clone and toolchain

```bash
git clone https://github.com/AkshantVats/infra-ai-streaming.git
cd infra-ai-streaming
```

Install **Rust 1.88** via [rustup](https://rustup.rs). The workspace pins the version in [`rust-toolchain.toml`](rust-toolchain.toml) (currently **1.88**).

Install **Go 1.22+** for the consumer (`consumer/go.mod`). On macOS, install **cmake** (required for `rdkafka-sys`). See [docs/dev-macos.md](docs/dev-macos.md).

## Local CI matrix (matches GitHub Actions)

Run before opening a PR:

```bash
cargo fmt --check
cargo clippy -p ingestion --all-targets -- -D warnings
cargo test -p ingestion
(cd consumer && test -z "$(gofmt -l .)" && go test ./...)
helm dependency update deploy/helm/lensai
helm template lensai deploy/helm/lensai -n lensai -f deploy/helm/lensai/values-m1.yaml >/dev/null
shellcheck -x chaos/*.sh deploy/k3d/*.sh deploy/helm/lensai/files/*.sh deploy/redpanda/*.sh scripts/*.sh
```

PR CI (`.github/workflows/ci.yml`) runs the same gates on every push to `main` and on pull requests, plus an **`e2e-k3d`** job (`./scripts/run.sh --profile m1`, ~25 min) after unit jobs pass.

A **weekly** workflow (`.github/workflows/e2e-k3d-dispatch.yml`) runs the same full stack on a schedule and via manual dispatch — useful when you want a scheduled signal without opening a PR.

## Full M1 E2E (k3d) one-liner

Requires Docker, k3d, helm, kubectl:

```bash
./scripts/run.sh --profile m1
# or: HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh
```

## Tests without Kubernetes

```bash
cargo test -p ingestion
go test ./consumer/...
```

Rust tests cover config, WAL, rate limit, metrics. Go tests cover JSON, breaker, row mapping. Neither requires Docker unless you run compose E2E.

## Local dependencies (optional)

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

See README **Local dependencies (Docker)** for ports.

**Consumer (compose E2E):**

```bash
set -a && source deploy/.env && set +a
export KAFKA_BROKERS=127.0.0.1:9092
go run ./consumer/cmd/consumer
```

Or [`scripts/smoke-e2e.sh`](scripts/smoke-e2e.sh) after Compose is up.

## Build metadata

`/health` on ingestion (`:8080`) and consumer (`:9091`) returns `version`, `git_sha`, and `build_time`. Docker builds accept `GIT_SHA` and `BUILD_TIME` build-args (see [RELEASE.md](RELEASE.md)).

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Please read it before participating.

## Pull requests

Before opening or updating a PR:

1. `git fetch origin`
2. Merge or rebase `origin/main` onto your branch
3. Confirm mergeable locally (`git merge origin/main` with no conflicts) or on GitHub
4. Resolve any conflicts (compose, Helm values, `dashboards/`, shared scripts)
5. Re-run the **local CI matrix** and any smoke/E2E you relied on for the change

Then:

- Keep changes focused; match existing style (imports, error handling, docs level).
- Ensure the **local CI matrix** above passes.
- Do not commit secrets, `.env`, or large generated artifacts (see `.gitignore`).
- Update [CHANGELOG.md](CHANGELOG.md) under `[Unreleased]` for user-visible changes.
- Link operational doc updates when touching deploy or observability (SLOs, runbook, retention).
