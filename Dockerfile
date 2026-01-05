# syntax=docker/dockerfile:1

# ============================================================================
# Stage 1: Build frontend (GUI)
# ============================================================================
FROM node:24-alpine AS gui-builder

WORKDIR /build

# Copy only package files first for better layer caching
COPY web/gui/package.json web/gui/package-lock.json* web/gui/pnpm-lock.yaml* web/gui/yarn.lock* ./web/gui/

# Install dependencies
RUN cd web/gui && \
    if [ -f pnpm-lock.yaml ]; then npm install -g pnpm && pnpm install --frozen-lockfile; \
    elif [ -f yarn.lock ]; then yarn install --frozen-lockfile; \
    elif [ -f package-lock.json ]; then npm ci; \
    else npm install; fi

# Copy source and build
COPY web/gui/ ./web/gui/
RUN cd web/gui && npm run build

# ============================================================================
# Stage 2: Build Go binary
# ============================================================================
FROM golang:1.25-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=gui-builder /build/web/gui/dist/ ./web/gui/dist/

# Build arguments for version info
ARG VERSION=unknown
ARG BUILD_TIME=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags "-X 'github.com/1orz/proxy-speedtest/constant.Version=${VERSION}' \
              -X 'github.com/1orz/proxy-speedtest/constant.BuildTime=${BUILD_TIME}' \
              -w -s" \
    -o /build/lite .

# ============================================================================
# Stage 3: Final minimal image
# ============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=go-builder /build/lite /lite

# Expose default port
EXPOSE 10888

# Run as non-root user (distroless/nonroot uses uid 65532)
USER nonroot:nonroot

# Health check
# Note: distroless doesn't have curl/wget, so we use the binary itself if it supports health endpoint
# Or remove this if not applicable
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD ["/lite", "health"] || exit 1

ENTRYPOINT ["/lite"]

