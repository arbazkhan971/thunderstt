# thunderstt: Design Specification

## 1. Overview

**thunderstt** is a single Go binary that provides a production-grade speech-to-text server with an OpenAI-compatible API. Parakeet TDT V3 as the default engine (fastest, most accurate for English/European languages), Whisper as automatic fallback for 75+ other languages.

**Target audience:** Backend/DevOps engineers who want to self-host transcription without Python, CUDA, or ML tooling.

**Core value props:**
- Single binary, ~50-100MB Docker image
- OpenAI SDK drop-in replacement (swap `base_url`)
- Parakeet V3: 10x faster than Whisper, zero hallucinations
- Automatic language detection + model routing
- CPU-first, optional GPU

## 2. Architecture

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Layer                              │
│  chi router                                                  │
│  ├── POST /v1/audio/transcriptions                          │
│  ├── GET  /v1/models                                        │
│  ├── GET  /health, /ready                                   │
│  ├── GET  /metrics (Prometheus)                             │
│  └── WS   /v1/audio/stream (Phase 2)                       │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                    Job Queue                                  │
│  In-process bounded channel + semaphore                      │
│  Request timeout + context cancellation                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                    Pipeline                                   │
│  Audio decode → Resample 16kHz → VAD → Chunk → Infer → Stitch│
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                    Engine Interface                           │
│  ├── SherpaEngine  (sherpa-onnx: Parakeet + Whisper)        │
│  ├── WhisperEngine (whisper.cpp)                             │
│  └── AutoEngine    (language detect → route)                 │
└─────────────────────────────────────────────────────────────┘
```

### Design: Monolith with Optional Worker Split

Single binary runs in two modes:
- `thunderstt serve` — all-in-one (Phase 1)
- `thunderstt serve --mode=api` + `thunderstt serve --mode=worker` — split (Phase 3)

### Key Decisions

- **chi router** — lightweight, stdlib net/http compatible
- **In-process job queue** for Phase 1 — Go channel + semaphore
- **Pluggable engine interface** — SherpaEngine (sherpa-onnx), WhisperEngine (whisper.cpp), AutoEngine (router)
- **Audio formats** — WAV, MP3, FLAC, OGG natively; ffmpeg subprocess fallback for exotic formats

## 3. Engine Interface

```go
type Engine interface {
    Transcribe(audio []float32, sampleRate int, opts Options) (*Result, error)
    SupportedLanguages() []string
    ModelName() string
    Close() error
}

type Options struct {
    Language       string
    WordTimestamps bool
    VADFilter      bool
    InitialPrompt  string
}

type Result struct {
    Language     string
    LanguageProb float64
    Duration     float64
    Segments     []Segment
}

type Segment struct {
    ID    int
    Start float64
    End   float64
    Text  string
    Words []Word
}

type Word struct {
    Word  string
    Start float64
    End   float64
}
```

### Model Registry

| Model ID | Engine | Size (INT8) | Languages |
|---|---|---|---|
| parakeet-tdt-0.6b-v3 | sherpa-onnx | ~670MB | 25 European |
| parakeet-tdt-0.6b-v2 | sherpa-onnx | ~670MB | English only |
| whisper-large-v3-turbo | sherpa-onnx | ~800MB | 99 languages |
| whisper-large-v3 | whisper.cpp | ~1.5GB | 99 languages |
| whisper-medium | whisper.cpp | ~750MB | 99 languages |

### AutoEngine Routing

1. Language hint provided → route by language support
2. No hint → run language detection on first 30s → route accordingly
3. Parakeet's 25 European languages → Parakeet engine
4. Everything else → Whisper engine

## 4. API Specification

### OpenAI-Compatible Endpoints

| Endpoint | Method | Phase |
|---|---|---|
| /v1/audio/transcriptions | POST | 1 |
| /v1/models | GET | 1 |
| /health | GET | 1 |
| /ready | GET | 1 |
| /metrics | GET | 2 |
| /v1/audio/stream | WebSocket | 2 |
| /v1/audio/translations | POST | 4 |

### Request Format

```
POST /v1/audio/transcriptions
Content-Type: multipart/form-data

file: <audio file>
model: "parakeet-tdt-0.6b-v3" | "whisper-large-v3-turbo" | "auto"
language: "en" (optional)
response_format: "json" | "verbose_json" | "text" | "srt" | "vtt"
timestamp_granularities: ["word", "segment"]
```

### Response Formats

All five OpenAI formats supported: json, verbose_json, text, srt, vtt.

## 5. Audio Handling

- **Native Go:** WAV (go-native), MP3 (go-mp3), FLAC (go-flac), OGG (go-ogg)
- **Fallback:** ffmpeg subprocess for M4A, WebM, etc.
- **Processing:** Resample to 16kHz mono float32
- **Long audio:** Silero VAD → chunk at speech boundaries (~20s) → batch infer → stitch timestamps

## 6. Model Management

```
~/.cache/thunderstt/models/
├── parakeet-tdt-0.6b-v3/
├── whisper-large-v3-turbo/
└── silero-vad/
```

- Auto-download from HuggingFace on first use
- `thunderstt download <model>` for pre-downloading
- `THUNDERSTT_MODELS_DIR` env var override

## 7. CLI Commands

```
thunderstt serve [--host 0.0.0.0] [--port 8000] [--model auto] [--workers 4]
thunderstt transcribe <file> [--model parakeet-tdt-0.6b-v3] [--format text] [--word-timestamps]
thunderstt download <model>
thunderstt models
thunderstt version
```

## 8. Project Structure

```
thunderstt/
├── cmd/thunderstt/main.go
├── internal/
│   ├── api/          (server, routes, handlers, middleware)
│   ├── engine/       (interface, sherpa, whisper, auto, registry)
│   ├── pipeline/     (pipeline, vad, chunker, stitcher)
│   ├── audio/        (decode, resample, ffmpeg fallback)
│   ├── model/        (download, registry, cache)
│   ├── queue/        (job queue, semaphore)
│   ├── format/       (json, verbose_json, text, srt, vtt)
│   └── config/       (config struct, env vars, flags)
├── pkg/thunderstt/   (optional Go client SDK)
├── tests/integration/
├── docker/
├── .github/workflows/
├── .goreleaser.yml
├── go.mod
├── Makefile
├── LICENSE (MIT)
└── README.md
```

## 9. Development Phases

### Phase 1: Core Server (Week 1-2)
- Project scaffolding (go.mod, Makefile, CI)
- Engine interface + SherpaEngine (Parakeet TDT V3)
- Audio decoding (WAV, MP3, FLAC, OGG)
- Pipeline (VAD → chunk → infer → stitch)
- HTTP server with /v1/audio/transcriptions
- All 5 response formats
- CLI (serve, transcribe, download)
- Model auto-download from HuggingFace
- Docker image (multi-stage, distroless)
- Integration tests

### Phase 2: Multi-model + Streaming (Week 3)
- Whisper model support via sherpa-onnx
- AutoEngine (language detection + routing)
- Word-level timestamps
- WebSocket streaming endpoint
- Prometheus metrics
- whisper.cpp engine backend (optional)

### Phase 3: Production Hardening (Week 4)
- Worker split mode (--mode=api / --mode=worker)
- Rate limiting, request size limits
- Kubernetes readiness/liveness probes
- Helm chart
- goreleaser for cross-platform binaries
- Benchmarks (RTFx, WER) vs faster-whisper

### Phase 4: Growth (Ongoing)
- gRPC streaming
- Speaker diarization
- GPU acceleration (ONNX Runtime CUDA/TensorRT)
- Translation endpoint (Canary model)
- WebSocket streaming improvements
- Open WebUI integration guide

## 10. Dependencies

### Core
- github.com/k2-fsa/sherpa-onnx-go (CGo, pre-built libs)
- github.com/go-chi/chi/v5 (HTTP router)
- github.com/spf13/cobra (CLI)
- github.com/hajimehoshi/go-mp3 (MP3 decode)
- github.com/mewkiz/flac (FLAC decode)
- github.com/prometheus/client_golang (metrics)

### NOT dependencies
- Python, PyTorch, NeMo, transformers
- CUDA/cuDNN (for CPU mode)
- Redis, NATS, or any external queue (Phase 1)

## 11. Risks & Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| sherpa-onnx CGo build issues | MEDIUM | Pre-built libs, CI on all platforms |
| Parakeet V3 model files not easily available | LOW | Multiple HuggingFace sources |
| whisper.cpp CGo conflicts with sherpa-onnx | MEDIUM | Build tags to compile one or both |
| Large binary size from two CGo deps | LOW | Build tags for engine selection |
| Long audio OOM | MEDIUM | Streaming pipeline, chunk size limits |
