# SPDX-License-Identifier: MIT
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY traceforge/go.mod traceforge/go.sum ./traceforge/
COPY traceforge/ ./traceforge/
RUN cd traceforge && go build -o /traceforge-collector ./cmd/collector

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /traceforge-collector /usr/local/bin/traceforge-collector
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/traceforge-collector"]
