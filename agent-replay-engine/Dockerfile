# SPDX-License-Identifier: MIT
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY agent-replay-engine/go.mod agent-replay-engine/go.sum ./agent-replay-engine/
COPY agent-replay-engine/ ./agent-replay-engine/
RUN cd agent-replay-engine && go build -o /traceforge-replay ./cmd/traceforge

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /traceforge-replay /usr/local/bin/traceforge
ENTRYPOINT ["/usr/local/bin/traceforge"]
