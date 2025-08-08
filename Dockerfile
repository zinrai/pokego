# Build stage
FROM golang:1.21-bookworm AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o pokego \
    main.go

# Runtime stage
FROM debian:bookworm-slim

# Install ca-certificates for HTTPS support
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -u 1001 -s /bin/false pokego

# Copy binary from builder
COPY --from=builder /build/pokego /usr/local/bin/pokego

# Make binary executable
RUN chmod +x /usr/local/bin/pokego

# Switch to non-root user
USER pokego

# Set entrypoint
ENTRYPOINT ["pokego"]

# Default command shows help
CMD ["-h"]
