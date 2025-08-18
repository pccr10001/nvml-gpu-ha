# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o nvml-gpu-ha .

# Runtime stage
FROM nvidia/cuda:12.2-base-ubuntu22.04

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy the binary from builder stage
COPY --from=builder /app/nvml-gpu-ha /usr/local/bin/nvml-gpu-ha

# Create non-root user
RUN useradd -r -s /bin/false nvml-gpu

USER nvml-gpu

# Default command
ENTRYPOINT ["/usr/local/bin/nvml-gpu-ha"]
CMD ["--mqtt-host=localhost", "--mqtt-port=1883", "--polling-period=30"]
