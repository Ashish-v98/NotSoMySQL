# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy proto module first (needed for replace directive)
COPY proto/ ./proto/

# Copy sidecar module files
COPY sidecar/go.mod sidecar/go.sum* ./sidecar/

# Download dependencies
WORKDIR /app/sidecar
RUN go mod download

# Copy sidecar source code
WORKDIR /app
COPY sidecar/ ./sidecar/

# Build the application
WORKDIR /app/sidecar
RUN CGO_ENABLED=0 GOOS=linux go build -o sidecar ./cmd/sidecar

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/sidecar/sidecar .

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./sidecar"]
