# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

# ca-certificates for HTTPS outbound (Google OAuth), sqlite3 for diagnostics
RUN apt-get update && apt-get install -y ca-certificates sqlite3 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/server ./server
COPY --from=builder /app/migrations  ./migrations
COPY --from=builder /app/web         ./web

# /data is where the SQLite file lives (mounted as a persistent volume)
RUN mkdir -p /data

EXPOSE 8080

CMD ["./server"]
