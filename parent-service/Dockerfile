# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy proto module first (needed for replace directive)
COPY proto/ ./proto/

# Copy parent-service module files
COPY parent-service/go.mod parent-service/go.sum* ./parent-service/

# Download dependencies
WORKDIR /app/parent-service
RUN go mod download

# Copy parent-service source code
WORKDIR /app
COPY parent-service/ ./parent-service/

# Build the application
WORKDIR /app/parent-service
RUN CGO_ENABLED=0 GOOS=linux go build -o parent-service ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/parent-service/parent-service .

# Expose ports (gRPC and REST)
EXPOSE 9090
EXPOSE 8090

# Run the binary
CMD ["./parent-service"]
