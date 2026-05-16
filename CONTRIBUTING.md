# Contributing

## Clone and toolchain

```bash
git clone https://github.com/YOURUSERNAME/infra-ai-streaming.git
cd infra-ai-streaming
```

Install **Rust 1.86+** via [rustup](https://rustup.rs). The workspace reads [`rust-toolchain.toml`](rust-toolchain.toml); running `cargo` here selects the right toolchain.

Install **Go 1.22+** for the consumer (`consumer/go.mod`). On macOS, install **cmake** (required for `rdkafka-sys`). See [docs/dev-macos.md](docs/dev-macos.md) for Xcode CLT, Homebrew, and optional Redis.

## Tests

```bash
cargo test -p ingestion
go test ./consumer/...
```

Rust tests are **library** tests (config, WAL, rate limit, metrics). Go tests cover batch JSON deserialize. Neither requires Docker unless you run the full E2E path.

## Local dependencies (optional)

Compose reads `deploy/.env` via `env_file` (paths are relative to `deploy/`), so copy the example file first; it is gitignored after creation.

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

See README **Local dependencies (Docker)** for ports and caveats.

**Consumer (optional E2E):**

```bash
set -a && source deploy/.env && set +a
export KAFKA_BROKERS=127.0.0.1:9092
go run ./consumer/cmd/consumer
```

Or run [`scripts/smoke-e2e.sh`](scripts/smoke-e2e.sh) after Compose is up.

## Pull requests

- Keep changes focused and consistent with existing style (imports, error handling, docs level).
- Ensure `cargo test -p ingestion` and `go test ./consumer/...` pass locally.
- Do not commit secrets, `.env`, or large generated artifacts. Follow `.gitignore`.
