# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o wallet-api ./cmd/api

# Final Stage
FROM alpine:3.19

WORKDIR /app

# Install goose for migrations
RUN apk add --no-cache curl \
    && curl -fsSL https://raw.githubusercontent.com/pressly/goose/master/install.sh | sh

# Copy binary
COPY --from=builder /app/wallet-api .

# Copy migrations
COPY db/migrations ./db/migrations

# Setup entrypoint script
COPY scripts/entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080 8081

ENTRYPOINT ["./entrypoint.sh"]
