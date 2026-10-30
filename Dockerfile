# Multi-stage build for a minimal, secure container
FROM golang:alpine AS builder

WORKDIR /app

# Install git and certs
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o kirocc ./cmd/kirocc

# Production runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/kirocc /app/kirocc
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/kirocc

EXPOSE 10000

ENTRYPOINT ["/app/entrypoint.sh"]

