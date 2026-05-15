# Local development on macOS

This repository is a Rust workspace (see [`rust-toolchain.toml`](../rust-toolchain.toml) for the pinned toolchain). The `ingestion` crate builds native code via **cmake** (for `rdkafka-sys`).

## 1. Xcode Command Line Tools

Apple’s compiler toolchain is required for some native dependencies:

```bash
xcode-select --install
```

## 2. Homebrew

Install from [https://brew.sh](https://brew.sh), then:

```bash
brew install cmake
```

`cmake` is required for the `rdkafka` **cmake-build** feature used by the ingestion crate.

## 3. Rust (rustup)

Install **rustup** from [https://rustup.rs](https://rustup.rs). The repo pins **Rust 1.86** (and `rustfmt` / `clippy` components) in `rust-toolchain.toml`; rustup will download that toolchain automatically when you run `cargo` in the workspace.

```bash
cd /path/to/infra-ai-streaming
./scripts/test-ingestion.sh
# or: cargo test -p ingestion (after exporting CXXFLAGS below on first rdkafka build)
```

### rdkafka-sys / libc++ on macOS

A clean build of `rdkafka` (cmake-build) can fail with `'string' file not found` when the C++ compiler does not see Apple’s libc++ headers. Point the build at the Command Line Tools SDK:

```bash
export SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk
export CXXFLAGS="-isystem ${SDKROOT}/usr/include/c++/v1"
cargo test -p ingestion
```

`scripts/test-ingestion.sh` sets these automatically on Darwin. Prefer the script over ad-hoc `cargo` so CI-like local runs stay consistent.

## 4. Optional: Redis for future integration tests

Library unit tests today do not require Redis. When integration tests against a real Redis are added, install a local broker:

```bash
brew install redis
brew services start redis   # optional
```

## 5. Optional: Docker for dependency stack

Redis, Redpanda (Kafka API), and ClickHouse can run via Compose — see README **Local dependencies (Docker)** and [`deploy/.env.example`](../deploy/.env.example).

## Caveats

- **Disk / CPU:** `cargo` + `librdkafka` compile is heavy the first time; ensure enough free disk (~2–3 GB under `target/` is common).
- **Apple Silicon:** use native arm64 toolchains; Docker images in this repo are multi-arch where upstream publishes them.
