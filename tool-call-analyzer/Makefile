COVER_OUT := /tmp/tca-cover.out
COVER_HTML := /tmp/tca-cover.html

.PHONY: test test-unit test-cover test-race lint build clean

test-unit:
	go test ./... -short -count=1 -timeout 60s

test-cover:
	go test ./... -short -count=1 -coverprofile=$(COVER_OUT) -covermode=atomic
	go tool cover -func=$(COVER_OUT) | tail -1
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	@echo "Coverage HTML: $(COVER_HTML)"

test-race:
	go test ./... -short -race -count=1 -timeout 120s

## Integration tests — requires running Kafka broker (KAFKA_BROKERS env var)
test-integration:
	go test ./pkg/kafka/... -run Integration -count=1 -timeout 120s -v

test: test-unit test-race

lint:
	go vet ./...

build:
	go build -o /tmp/tca-bin ./cmd/... 2>/dev/null || true

clean:
	rm -f $(COVER_OUT) $(COVER_HTML)
