# Contributing

## Clone and toolchain

```bash
git clone https://github.com/YOURUSERNAME/infra-ai-streaming.git
cd infra-ai-streaming
```

Install **Rust 1.86+** via [rustup](https://rustup.rs). The workspace reads [`rust-toolchain.toml`](rust-toolchain.toml); running `cargo` here selects the right toolchain.

On macOS, install **cmake** (required for `rdkafka-sys`). See [docs/dev-macos.md](docs/dev-macos.md) for Xcode CLT, Homebrew, and optional Redis.

## Tests

```bash
cargo test -p ingestion
```

These are **library** tests (config, WAL, rate limit, metrics). They do **not** require Docker or the Compose stack unless/until integration tests are added.

## Local dependencies (optional)

For Redis, Redpanda, and ClickHouse matching the architecture docs:

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

See README **Local dependencies (Docker)** for ports and caveats.

## Pull requests

- Keep changes focused and consistent with existing style (imports, error handling, docs level).
- Ensure `cargo test -p ingestion` passes locally.
- Do not commit secrets, `.env`, or large generated artifacts. Follow `.gitignore`.
