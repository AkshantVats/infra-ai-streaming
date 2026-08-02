# SPDX-License-Identifier: MIT
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY agent-benchmark-runner/go.mod agent-benchmark-runner/go.sum ./agent-benchmark-runner/
COPY agent-benchmark-runner/ ./agent-benchmark-runner/
RUN cd agent-benchmark-runner && go build -o /traceforge-benchmark ./cmd/traceforge

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /traceforge-benchmark /usr/local/bin/traceforge
ENTRYPOINT ["/usr/local/bin/traceforge"]
