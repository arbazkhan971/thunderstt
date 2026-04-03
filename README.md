# ThunderSTT

Lightning-fast speech-to-text server powered by sherpa-onnx and NVIDIA NeMo Parakeet models.

## Features

- **OpenAI-compatible API** -- drop-in replacement for `/v1/audio/transcriptions`
- **Translation endpoint** -- `/v1/audio/translations` for speech-to-English translation
- **WebSocket streaming** -- real-time transcription via `/v1/audio/stream`
- **Multiple model support** -- Parakeet TDT, Whisper Large V3 Turbo, and more
- **High performance** -- concurrent worker pool with sherpa-onnx inference
- **Voice Activity Detection** -- built-in Silero VAD to skip silence and reduce hallucinations
- **Multiple output formats** -- text, JSON, SRT, VTT with word-level timestamps
- **Automatic model management** -- downloads models from HuggingFace Hub on first use
- **GPU acceleration** -- optional CUDA support via NVIDIA container images
- **Rate limiting** -- per-client request throttling to protect shared deployments
- **API key authentication** -- optional bearer-token auth via `--api-key`
- **Prometheus metrics** -- `/metrics` endpoint for monitoring and alerting
- **Go SDK** -- first-party client library at `pkg/thunderstt`
- **CLI and server modes** -- transcribe files locally or run as an HTTP service
- **Structured logging** -- JSON logs with zerolog
- **Small footprint** -- single binary, minimal dependencies

## Quick Start

### Using a pre-built binary

Download the latest release from the [Releases](https://github.com/arbazkhan971/thunderstt/releases) page.

```bash
# Download and extract (example for Linux amd64)
curl -L https://github.com/arbazkhan971/thunderstt/releases/latest/download/thunderstt_linux_amd64.tar.gz | tar xz

# Start the server (downloads the default model on first run)
./thunderstt serve --model parakeet-tdt-0.6b-v3 --port 8000
```

### Using Docker

```bash
# CPU
docker run --rm -p 8000:8000 \
    -v thunderstt-models:/root/.cache/thunderstt/models \
    ghcr.io/arbazkhan971/thunderstt:latest

# GPU (requires NVIDIA Container Toolkit)
docker run --rm --gpus all -p 8000:8000 \
    -v thunderstt-models:/root/.cache/thunderstt/models \
    ghcr.io/arbazkhan971/thunderstt:latest-gpu
```

### Using Docker Compose

```bash
cd docker
docker compose up -d
```

### From source

```bash
git clone https://github.com/arbazkhan971/thunderstt.git
cd thunderstt
make build
./bin/thunderstt serve --model parakeet-tdt-0.6b-v3
```

## API Usage

ThunderSTT exposes an OpenAI-compatible transcription API.

### Transcribe an audio file

```bash
curl -X POST http://localhost:8000/v1/audio/transcriptions \
    -H "Content-Type: multipart/form-data" \
    -F "file=@recording.wav" \
    -F "model=parakeet-tdt-0.6b-v3"
```

### Transcribe with options

```bash
curl -X POST http://localhost:8000/v1/audio/transcriptions \
    -H "Content-Type: multipart/form-data" \
    -F "file=@recording.mp3" \
    -F "model=parakeet-tdt-0.6b-v3" \
    -F "response_format=verbose_json" \
    -F "language=en" \
    -F "timestamp_granularities[]=word"
```

### Get available models

```bash
curl http://localhost:8000/v1/models
```

### Translate audio to English

```bash
curl -X POST http://localhost:8000/v1/audio/translations \
    -F "file=@audio.mp3" \
    -F "model=whisper-large-v3-turbo"
```

### WebSocket streaming

Connect to `ws://localhost:8000/v1/audio/stream` and send raw PCM audio frames. The server streams back partial transcription results as JSON messages. See `docs/` for the full protocol spec.

### Health check

```bash
curl http://localhost:8000/health
```

### Metrics

```bash
curl http://localhost:8000/metrics
```

### Version

```bash
curl http://localhost:8000/version
```

### API endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/audio/transcriptions` | Transcribe audio file |
| POST | `/v1/audio/translations` | Translate audio to English |
| GET | `/v1/audio/stream` | WebSocket streaming transcription |
| GET | `/v1/models` | List available models |
| GET | `/health` | Liveness check |
| GET | `/ready` | Readiness check |
| GET | `/version` | Server version info |
| GET | `/metrics` | Prometheus metrics |

### Response formats

| Format | Description |
|--------|-------------|
| `json` | `{"text": "transcribed text"}` |
| `verbose_json` | Full response with segments, timestamps, and metadata |
| `text` | Plain text |
| `srt` | SubRip subtitle format |
| `vtt` | WebVTT subtitle format |

## OpenAI SDK Example

ThunderSTT is compatible with the official OpenAI Python SDK:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8000/v1",
    api_key="your-key",  # omit if auth is not enabled
)

with open("recording.wav", "rb") as audio_file:
    transcript = client.audio.transcriptions.create(
        model="parakeet-tdt-0.6b-v3",
        file=audio_file,
        response_format="verbose_json",
        timestamp_granularities=["word"],
    )

print(transcript.text)
for word in transcript.words:
    print(f"  [{word.start:.2f} - {word.end:.2f}] {word.word}")
```

## Go SDK

ThunderSTT ships a Go client library under `pkg/thunderstt`:

```go
import "github.com/arbazkhan971/thunderstt/pkg/thunderstt"

client := thunderstt.NewClient("http://localhost:8000", thunderstt.WithAPIKey("your-key"))
resp, err := client.Transcribe(thunderstt.TranscribeRequest{
    FilePath: "recording.wav",
    Model:    "parakeet-tdt-0.6b-v3",
})
```

## CLI Usage

### Start the server

```bash
thunderstt serve \
    --host 0.0.0.0 \
    --port 8000 \
    --model parakeet-tdt-0.6b-v3 \
    --workers 4 \
    --log-level info
```

### Transcribe a file locally

```bash
thunderstt transcribe recording.wav \
    --model parakeet-tdt-0.6b-v3 \
    --format text \
    --language en \
    --word-timestamps
```

### Download a model

```bash
thunderstt download parakeet-tdt-0.6b-v3
```

### List available models

```bash
thunderstt models
```

### Print version

```bash
thunderstt version
```

## Configuration

All configuration can be set via CLI flags or environment variables. Environment variables use the `THUNDERSTT_` prefix.

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--host` | `THUNDERSTT_HOST` | `0.0.0.0` | Address to bind the HTTP server |
| `--port` | `THUNDERSTT_PORT` | `8080` | Port to listen on |
| `--model` | `THUNDERSTT_MODEL` | `base` | Model to load |
| `--workers` | `THUNDERSTT_WORKERS` | CPU count | Number of concurrent workers |
| `--log-level` | `THUNDERSTT_LOG_LEVEL` | `info` | Log level (trace, debug, info, warn, error) |
| `--api-key` | `THUNDERSTT_API_KEY` | *(none)* | Require this bearer token for all requests |
| `--rate-limit` | `THUNDERSTT_RATE_LIMIT` | `0` (unlimited) | Max requests per minute per client |
| `--max-file-size` | `THUNDERSTT_MAX_FILE_SIZE` | `25MB` | Maximum upload file size |
| -- | `THUNDERSTT_MODELS_DIR` | `~/.cache/thunderstt/models` | Model cache directory |

## Supported Models

| Model ID | Type | Engine | Size | Languages |
|----------|------|--------|------|-----------|
| `parakeet-tdt-0.6b-v3` | Parakeet | sherpa-onnx | ~700 MB | en, es, fr, de, it, pt, nl, pl, uk, ro, hu, el, sv, cs, bg, sk, hr, da, fi, lt, sl, lv, et, mt, ru |
| `parakeet-tdt-0.6b-v2` | Parakeet | sherpa-onnx | ~700 MB | en |
| `whisper-large-v3-turbo` | Whisper | sherpa-onnx | ~1.6 GB | 90+ languages |
| `silero-vad` | VAD | sherpa-onnx | ~2 MB | Language-agnostic |

Models are downloaded automatically on first use from HuggingFace Hub. Use `thunderstt download <model-id>` to pre-download.

## Performance

Performance varies by model, hardware, and audio characteristics. The following are rough guidelines:

| Model | Hardware | Real-time Factor | Notes |
|-------|----------|-------------------|-------|
| parakeet-tdt-0.6b-v3 | Apple M2 | ~0.02x | 50x faster than real-time |
| parakeet-tdt-0.6b-v3 | Intel i7 (4 workers) | ~0.05x | 20x faster than real-time |
| whisper-large-v3-turbo | Apple M2 | ~0.08x | 12x faster than real-time |
| whisper-large-v3-turbo | NVIDIA A100 | ~0.03x | 33x faster than real-time |

*Benchmarks are approximate and will be updated with formal measurements.*

## Architecture

```
cmd/thunderstt/     CLI entry point (cobra commands)
internal/
  api/              HTTP server, routes, handlers, middleware
  audio/            Audio decoding (WAV, MP3, FLAC, OGG) and resampling
  config/           Configuration management (flags, env vars)
  engine/           Transcription engine interface (sherpa-onnx, auto-routing)
  format/           Output formatting (JSON, verbose_json, text, SRT, VTT)
  metrics/          Prometheus metrics instrumentation
  model/            Model registry, HuggingFace downloader, and cache
  pipeline/         Audio pipeline (VAD, chunking, stitching)
  queue/            Bounded concurrency job queue
pkg/thunderstt/     Go SDK client library
deploy/helm/        Helm chart for Kubernetes deployment
docker/             Docker build files (CPU + GPU)
.github/workflows/  CI/CD pipelines (lint, test, release)
```

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Build Docker image
make docker-build
```

## Kubernetes

Deploy with Helm:

```bash
helm install thunderstt deploy/helm/thunderstt/
```

Override values as needed:

```bash
helm install thunderstt deploy/helm/thunderstt/ \
    --set model=whisper-large-v3-turbo \
    --set replicaCount=3
```

## License

See [LICENSE](LICENSE) for details.
