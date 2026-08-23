# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 works with no C toolchain.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tally-mcp ./cmd/tally-mcp

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 tally && \
    mkdir -p /data && chown tally:tally /data

WORKDIR /app
COPY --from=builder /out/tally-mcp .

USER tally

ENV TALLY_DB_PATH=/data/tally.db
ENV TALLY_LISTEN_ADDR=:8080

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/tally-mcp"]
