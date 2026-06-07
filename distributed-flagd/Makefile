.PHONY: proto build test lint docker-up docker-down

proto:
	protoc --go_out=. --go-grpc_out=. \
	  --go_opt=paths=source_relative \
	  --go-grpc_opt=paths=source_relative \
	  proto/flagd.proto

build: proto
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
