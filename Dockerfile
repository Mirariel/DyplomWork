# ─── Stage 1: build all service binaries ──────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api-gateway  ./cmd/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/market-data  ./cmd/market-data
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/trading      ./cmd/trading
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/analytics    ./cmd/analytics
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/migrate      ./cmd/migrate

# ─── Stage 2: minimal runtime image ───────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/api-gateway  ./api-gateway
COPY --from=builder /bin/market-data  ./market-data
COPY --from=builder /bin/trading      ./trading
COPY --from=builder /bin/analytics    ./analytics
COPY --from=builder /bin/migrate      ./migrate
COPY migrations/                      ./migrations/

# Default: api-gateway.  docker-compose overrides CMD per service.
CMD ["./api-gateway"]
