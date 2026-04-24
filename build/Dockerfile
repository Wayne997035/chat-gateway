# syntax=docker/dockerfile:1
# first stage
FROM --platform=linux/amd64 golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies and cache them
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /chat_gateway ./cmd/api

# second stage
FROM --platform=linux/amd64 alpine:3.21

WORKDIR /app

# Create directories for config and log
RUN mkdir -p ./config ./log

# Add non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /chat_gateway .
COPY --from=builder /app/configs ./config

ENV CONFIG_PATH=/app/config/production.yaml
ENV LOG_PATH=/app/log
ENV ENV=production

USER appuser

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

# start service
CMD ["./chat_gateway"]
