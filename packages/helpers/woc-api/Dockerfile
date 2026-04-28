# Build stage
FROM golang:1.25-alpine AS builder

# Install git (required for go mod download with private repos)
RUN apk add --no-cache git

WORKDIR /build

# Build argument for GitHub token
ARG TG_READER_TOKEN

# Configure git to use the token for private repositories
RUN git config --global url."https://${TG_READER_TOKEN}@github.com/".insteadOf "https://github.com/"

# Set GOPRIVATE to avoid checksum database for private repos
ENV GOPRIVATE=github.com/teranode-group/*

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version and commit
ARG VERSION=docker
ARG COMMIT=unknown

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    --trimpath \
    -ldflags="-s -w -X main.commit=${COMMIT} -X main.version=${VERSION}" \
    -o woc-api \
    .

# Runtime stage
FROM debian:bookworm-slim

# Install basic shell utilities
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    bash \
    coreutils \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/woc-api .

# Copy configuration files if they exist
COPY --from=builder /build/settings_local.conf* ./

# Make the binary executable
RUN chmod +x /app/woc-api

# Run the application
CMD ["/app/woc-api"]
