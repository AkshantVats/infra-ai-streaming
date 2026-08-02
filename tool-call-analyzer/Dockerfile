# SPDX-License-Identifier: MIT
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY tool-call-analyzer/go.mod tool-call-analyzer/go.sum ./tool-call-analyzer/
COPY tool-call-analyzer/ ./tool-call-analyzer/
RUN cd tool-call-analyzer && go build -o /traceforge-analyzer ./cmd/traceforge

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /traceforge-analyzer /usr/local/bin/traceforge
ENTRYPOINT ["/usr/local/bin/traceforge"]
