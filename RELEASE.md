# Release process

This repo is optimized for **repeatable local E2E** and lightweight OSS hygiene.
Use this document to ship tagged releases without introducing heavyweight infra.

## Versioning

- Prefer SemVer tags: `v0.1.0`, `v0.2.0`, etc.
- During early development, it’s OK to use pre-release tags: `v0.2.0-alpha.1`.

## Pre-release checklist

- Update `CHANGELOG.md` under `[Unreleased]`.
- Run the fast local matrix:

```bash
cargo fmt --check
cargo clippy -p ingestion -- -D warnings
cargo test -p ingestion
(cd consumer && go test ./...)
helm template lensai deploy/helm/lensai -n lensai -f deploy/helm/lensai/values-m1.yaml >/dev/null
```

- (Recommended) Run the full M1 E2E once:

```bash
HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh
```

## Tagging

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

## Images (optional)

If you publish container images, tag them with both the SemVer and the git SHA:

- `lensai/ingestion:vX.Y.Z` and `lensai/ingestion:<git-sha>`
- `lensai/consumer:vX.Y.Z` and `lensai/consumer:<git-sha>`

Keep `values-m1.yaml` using `pullPolicy: Never` for local k3d; production clusters should use immutable tags.

### Build metadata (git SHA + build time)

Ingestion and consumer embed compile-time metadata for support and rollouts:

| Surface | Fields |
|---------|--------|
| `GET /health` (ingestion `:8080`, consumer `:9091`) | `version`, `git_sha`, `build_time` (JSON) |
| Startup logs | Same fields in structured log line |

**Docker builds** (from repo root):

```bash
export GIT_SHA="$(git rev-parse --short HEAD)"
export BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
docker build --build-arg GIT_SHA="$GIT_SHA" --build-arg BUILD_TIME="$BUILD_TIME" \
  -f deploy/docker/Dockerfile.ingestion -t lensai/ingestion:local .
docker build --build-arg GIT_SHA="$GIT_SHA" --build-arg BUILD_TIME="$BUILD_TIME" \
  -f deploy/docker/Dockerfile.consumer -t lensai/consumer:local .
```

`./deploy/k3d/up.sh` passes these args automatically when building local images.

**Rust (non-Docker):** `ingestion/build.rs` sets `GIT_SHA` / `BUILD_TIME` from env or `git rev-parse` + wall clock.

**Go (non-Docker):** optional `-ldflags` (see `consumer/internal/buildinfo`); defaults are `dev` / `unknown` for `go run`.
