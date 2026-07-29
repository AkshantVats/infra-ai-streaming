.PHONY: proto build test test-integration lint docker-up docker-down openapi-validate

proto:
	protoc --go_out=. --go-grpc_out=. \
	  --go_opt=paths=source_relative \
	  --go-grpc_opt=paths=source_relative \
	  proto/flagd.proto

build: proto
	go build ./...

COVER_OUT := /tmp/flagd-cover.out
COVER_HTML := /tmp/flagd-cover.html

test:
	go test ./internal/... ./sdk/... -short -count=1 -timeout 60s

test-cover:
	go test ./internal/... ./sdk/... -short -count=1 -coverprofile=$(COVER_OUT) -covermode=atomic
	go tool cover -func=$(COVER_OUT) | tail -1
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	@echo "Coverage HTML: $(COVER_HTML)"

test-race:
	go test ./internal/... ./sdk/... -short -race -count=1 -timeout 120s

test-integration:
	FLAGD_INTEGRATION=1 go test -v -timeout 60s ./tests/integration/...
	go test ./chaos/... -count=1 -timeout 300s -v

lint:
	golangci-lint run ./...

openapi-validate:
	docker run --rm -v "$(PWD)/api:/spec" redocly/cli lint /spec/openapi.yaml

docker-up:
	docker compose up -d

docker-down:
	docker compose down
