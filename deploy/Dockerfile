# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Multi-stage build for KafGraph

# ─── Builder ─────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w \
        -X github.com/scalytics/kafgraph/internal/config.Version=${VERSION} \
        -X github.com/scalytics/kafgraph/internal/config.GitCommit=${GIT_COMMIT} \
        -X github.com/scalytics/kafgraph/internal/config.BuildDate=${BUILD_DATE}" \
    -o /app/kafgraph \
    ./cmd/kafgraph

# ─── Runtime ─────────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S kafgraph && adduser -S kafgraph -G kafgraph

COPY --from=builder /app/kafgraph /usr/local/bin/kafgraph

RUN mkdir -p /var/lib/kafgraph && chown kafgraph:kafgraph /var/lib/kafgraph

USER kafgraph

EXPOSE 7474 7687 9090

ENV KAFGRAPH_DATA_DIR=/var/lib/kafgraph
ENV KAFGRAPH_HOST=0.0.0.0

ENTRYPOINT ["kafgraph"]
