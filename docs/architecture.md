<!-- Version: 0.1 | Last updated: 2026-05-03 -->

# Architecture

## Overview

`generate-image` is a single-binary CLI tool written in Go. It has no runtime dependencies beyond the binary itself (ImageMagick is optional for format conversion). The architecture is deliberately minimal -- one file, no packages, no abstractions.

## Components

```
stdin (prompt) ─┐
                 ├─► main.go ─► FAL API ─► image file
config.yaml ────┘       │
                         ├─► FAL pricing API ─► cost to stderr
                         └─► preview command ─► image viewer
```

### Entry point (`main.go`)

All logic lives in a single file. Functions are organized by responsibility:

| Function | Purpose |
|----------|---------|
| `run()` | Orchestrator: parse flags, load config, resolve key, call API, write file |
| `resolveFALKey()` | API key resolution chain (env var -> config command -> config file -> .env) |
| `configDir()` | Locate the config directory (binary dir -> `~/.config/generate-image/`) |
| `loadConfig()` | Parse `config.yaml` |
| `loadFALKey()` | Parse `.env` file (fallback only) |
| `generateImage()` | POST to FAL API, download image, return bytes + content type |
| `writeImage()` | Write image to disk, handle extension logic and magick conversion |
| `convertWithMagick()` | Shell out to ImageMagick for format conversion |
| `reportCost()` | GET pricing from FAL API, print to stderr |
| `defaultPreviewCommand()` | Platform-appropriate image viewer (open/xdg-open/start) |

### Configuration

Two files in the config directory (`~/.config/generate-image/` or next to the binary):

- **`config.yaml`** -- model, API key sources, preview command
- **`.env`** -- fallback API key storage (optional, legacy)

The config directory is resolved at runtime by checking for `config.yaml` or `.env` next to the binary first, then falling back to the XDG location. This allows development (config next to binary) and installed (XDG) use without any flag or env var.

### API integration

The tool calls two FAL endpoints:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `https://fal.run/{model}` | POST | Image generation |
| `https://api.fal.ai/v1/models/pricing?endpoint_id={model}` | GET | Cost lookup (best-effort) |

Both use `Authorization: Key {fal_key}` headers. A `FAL_BASE_URL` environment variable redirects both endpoints for testing via `httptest.NewServer`.

### Testing

All 30 regression tests run the compiled binary as a subprocess via `os/exec`. The FAL API is intercepted using Go's `httptest.NewServer` -- no real API calls are made during `make test`. The Makefile sets `HOME` to a temp directory to prevent personal config from leaking into tests.

## Design decisions

| Decision | Rationale |
|----------|-----------|
| Single file, no packages | The tool is ~500 lines. Splitting adds navigation overhead for zero benefit. |
| Go, not Python | Single static binary. No venv, no pip, no runtime. Trivial cross-compilation. |
| No FAL SDK | The FAL API is two HTTP calls. A dependency for that is not justified. |
| `sh -c` for user commands | Config commands (key retrieval, preview) are user-specified shell expressions. |
| Extension from Content-Type | The FAL API returns JPEG by default regardless of what the user requests. Detecting and handling this is better than surprising the user with a misnamed file. |

## Roadmap

Future enhancements, in rough priority order:

| Feature | Description | Complexity |
|---------|-------------|------------|
| `--cost` flag | Query pricing without generating an image. Two FAL endpoints available: unit price lookup and historical cost estimate. See [#2](https://github.com/tadg-paul/generate-image/issues/2). | Small |
| `--model` flag | Override `config.yaml` model per invocation. Enables comparing models without editing config. | Small |
| Reference image / edit mode | Support `--ref image.png` for image-to-image generation. Uses FAL's `/edit` endpoint with `image_urls` parameter. Different endpoint from text-to-image. | Medium |
| Image dimensions | Support `--aspect-ratio` or `--size` presets. FAL API accepts `aspect_ratio` ("1:1", "16:9") and `resolution` ("1k", "2k"). | Small |
| Homebrew formula | Cross-compiled binaries for Darwin/Linux/Windows. `make release` with GitHub releases. See [#3](https://github.com/tadg-paul/generate-image/issues/3). | Medium |
| Batch mode | Accept multiple prompts (one per line), generate in parallel. | Medium |
| Cost tracking | Cumulative cost log for budgeting across sessions. | Medium |
| Prompt templates | Reusable prefix/suffix fragments in config. | Small |
| Provider abstraction | Support APIs beyond FAL (e.g. Replicate, direct vendor APIs). Not planned until a concrete need arises. | Large |
