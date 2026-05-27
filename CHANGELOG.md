# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Build metadata (`version`, `git_sha`, `build_time`) on ingestion and consumer `/health` and startup logs.
- `docs/SLOs.md`, `docs/DATA-RETENTION.md`, `docs/SECURITY-HARDENING.md`.
- Weekly / dispatch k3d E2E workflow (`.github/workflows/e2e-k3d-dispatch.yml`).
- Expanded `CONTRIBUTING.md` with local CI matrix and M1 E2E one-liner.

### Changed

- `docs/PRODUCTION-READINESS-CHECKLIST.md` and `docs/PROJECT-STATUS.md` updated for prod-hardening layers.
- `RELEASE.md` documents build metadata and Docker build-args.

## [0.1.0-dev] — 2026-05-16

Early development preview: Rust ingestion library and binary (WAL, Kafka produce, rate limiting), design docs, and local compose stack. ClickHouse writer and production consumer metrics are not complete yet — see [docs/PROJECT-STATUS.md](docs/PROJECT-STATUS.md).
