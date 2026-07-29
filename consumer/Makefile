COVER_OUT := /tmp/consumer-cover.out
COVER_HTML := /tmp/consumer-cover.html

.PHONY: test test-unit test-cover test-race lint build clean

## Unit tests (no infra required)
test-unit:
	go test ./... -short -count=1 -timeout 60s

## Unit tests with coverage report
test-cover:
	go test ./... -short -count=1 -coverprofile=$(COVER_OUT) -covermode=atomic
	go tool cover -func=$(COVER_OUT) | tail -1
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	@echo "Coverage HTML: $(COVER_HTML)"

## Race detector
test-race:
	go test ./... -short -race -count=1 -timeout 120s

## Integration tests — requires Kafka + Redis + ClickHouse (docker compose up -d first)
test-integration:
	go test ./... -run Integration -count=1 -timeout 300s -v

## All: unit + race (integration needs infra)
test: test-unit test-race

lint:
	go vet ./...
	@which staticcheck >/dev/null 2>&1 && staticcheck ./... || true

build:
	go build -o /tmp/consumer-bin ./cmd/consumer

clean:
	rm -f $(COVER_OUT) $(COVER_HTML) /tmp/consumer-bin
