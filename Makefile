## infra-ai-streaming — top-level test runner
## Runs unit tests for all sub-modules. No infra required.
## For e2e/integration tests: use scripts/test-e2e.sh from repo root

.PHONY: test test-unit test-cover test-race test-integration lint

test-unit:
	@echo "\n── ingestion (Rust) ──"
	cargo test --quiet
	@echo "\n── consumer (Go) ──"
	$(MAKE) -C consumer test-unit
	@echo "\n── tool-call-analyzer (Go) ──"
	$(MAKE) -C tool-call-analyzer test-unit
	@echo "\n── traceforge (Go) ──"
	cd traceforge && go test ./... -short -count=1 -timeout 60s
	@echo "\n── distributed-flagd (Go) ──"
	$(MAKE) -C distributed-flagd test

test-cover:
	@echo "\n── ingestion (Rust) ──"
	cargo test --quiet 2>&1 | grep "test result"
	@echo "\n── consumer (Go) ──"
	$(MAKE) -C consumer test-cover
	@echo "\n── tool-call-analyzer (Go) ──"
	$(MAKE) -C tool-call-analyzer test-cover
	@echo "\n── traceforge (Go) ──"
	cd traceforge && go test ./... -short -coverprofile=/tmp/traceforge-cover.out -covermode=atomic && go tool cover -func=/tmp/traceforge-cover.out | tail -1
	@echo "\n── distributed-flagd (Go) ──"
	$(MAKE) -C distributed-flagd test-cover

test-race:
	@echo "\n── consumer (race) ──"
	$(MAKE) -C consumer test-race
	@echo "\n── tool-call-analyzer (race) ──"
	$(MAKE) -C tool-call-analyzer test-race
	@echo "\n── traceforge (race) ──"
	cd traceforge && go test ./... -short -race -count=1 -timeout 120s
	@echo "\n── distributed-flagd (race) ──"
	$(MAKE) -C distributed-flagd test-race

## Integration tests — requires docker compose up -d first
test-integration:
	$(MAKE) -C consumer test-integration
	$(MAKE) -C tool-call-analyzer test-integration
	$(MAKE) -C distributed-flagd test-integration

## Default: unit + race
test: test-unit test-race

lint:
	cargo clippy --deny warnings
	cd consumer && go vet ./...
	cd tool-call-analyzer && go vet ./...
	cd traceforge && go vet ./...
	cd distributed-flagd && go vet ./...
