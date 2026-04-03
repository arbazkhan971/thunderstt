# =============================================================================
# ThunderSTT -- Multi-stage Docker build
# =============================================================================
#
# Build:
#   docker build -f docker/Dockerfile -t thunderstt:latest .
#
# Run:
#   docker run --rm -p 8000:8000 -v thunderstt-models:/root/.cache/thunderstt/models thunderstt:latest
#
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Build
# ---------------------------------------------------------------------------
FROM golang:1.22-bookworm AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

# Install build dependencies for sherpa-onnx (CGO).
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        cmake \
        pkg-config \
        libonnxruntime-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Cache Go module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree.
COPY . .

# Build the binary with CGO enabled (required for sherpa-onnx bindings).
RUN CGO_ENABLED=1 \
    go build \
        -ldflags "-s -w \
            -X main.Version=${VERSION} \
            -X main.Commit=${COMMIT} \
            -X main.BuildDate=${BUILD_DATE}" \
        -o /usr/local/bin/thunderstt \
        ./cmd/thunderstt

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# ---------------------------------------------------------------------------
FROM debian:bookworm-slim

# Install only the minimal runtime libraries needed by the binary and
# sherpa-onnx shared libraries. ca-certificates is required for HTTPS
# downloads from HuggingFace Hub.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        libgomp1 \
        libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

# Copy the compiled binary.
COPY --from=builder /usr/local/bin/thunderstt /usr/local/bin/thunderstt

# Copy shared libraries that the binary links against. The exact set depends
# on how sherpa-onnx was built; we copy the full onnxruntime library tree.
COPY --from=builder /usr/lib/x86_64-linux-gnu/libonnxruntime* /usr/lib/x86_64-linux-gnu/
RUN ldconfig

# Default model cache location.
ENV THUNDERSTT_MODELS_DIR=/root/.cache/thunderstt/models

# Create the models directory.
RUN mkdir -p ${THUNDERSTT_MODELS_DIR}

# The server listens on port 8000 by default.
EXPOSE 8000

ENTRYPOINT ["thunderstt", "serve"]
CMD ["--host", "0.0.0.0", "--port", "8000"]
