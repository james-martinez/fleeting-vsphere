# Multi-stage build for fleeting-vsphere plugin
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=latest
ARG BUILD_INFO=docker

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o fleeting-vsphere \
    .

# Final stage - minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S fleeting && \
    adduser -u 1001 -S fleeting -G fleeting

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/fleeting-vsphere .

# Change ownership to non-root user
RUN chown fleeting:fleeting /app/fleeting-vsphere

# Switch to non-root user
USER fleeting

# Set environment variables
ENV VERSION=${VERSION}
ENV BUILD_INFO=${BUILD_INFO}

# Expose any necessary ports (if needed)
# EXPOSE 8080

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ps aux | grep fleeting-vsphere || exit 1

# Run the binary
ENTRYPOINT ["./fleeting-vsphere"]