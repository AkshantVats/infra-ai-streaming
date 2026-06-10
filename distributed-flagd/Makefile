.PHONY: proto build test test-integration lint docker-up docker-down openapi-validate

proto:
	protoc --go_out=. --go-grpc_out=. \
	  --go_opt=paths=source_relative \
	  --go-grpc_opt=paths=source_relative \
	  proto/flagd.proto

build: proto
	go build ./...

test:
	go test ./...

test-integration:
	FLAGD_INTEGRATION=1 go test -v -timeout 60s ./tests/integration/...

lint:
	golangci-lint run ./...

openapi-validate:
	docker run --rm -v "$(PWD)/api:/spec" redocly/cli lint /spec/openapi.yaml

docker-up:
	docker compose up -d

docker-down:
	docker compose down
