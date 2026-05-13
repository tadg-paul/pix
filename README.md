# pix

A minimal CLI for generating and pricing images via the [FAL API](https://fal.ai). Pipe a prompt in, get an image out.

<img width="100%" alt="Terminal example" src="https://github.com/user-attachments/assets/de7740f5-4735-47d2-a943-e481b3b9c343" />

## Quickstart

### Prerequisites

- Go 1.22+
- A [FAL API key](https://fal.ai/dashboard/keys)
- ImageMagick (`magick`) -- optional, for format conversion

### Install

```bash
git clone https://github.com/tadg-paul/pix.git
cd pix
make install
```

This compiles the binary to `~/.local/bin/pix` and creates `~/.config/pix/config.yaml` from the template. Edit it to configure the API key and model -- see [Configuration](#configuration).

### Usage

Generate a new image from a prompt:

```bash
> echo "a red cat sitting on a wall" | pix gen cat
Cost: $0.02 (unit: images) for model xai/grok-imagine-image (source: FAL API)
Wrote cat.jpg

> echo "a blueprint" | pix gen blueprint.png
# API returns JPEG, converted to PNG via magick:
Cost: $0.02 (unit: images) for model xai/grok-imagine-image (source: FAL API)
Wrote blueprint.png (converted jpg to png)

> echo "test prompt" | pix gen --dry-run test
POST https://fal.run/xai/grok-imagine-image
{
  "prompt": "test prompt"
}
Output: test
(dry run -- no API call made)

> echo "A spoon eating a man wearing a hat" | pix --quiet gen -p landscape
# generates quietly, opens in default viewer (or preview-command from config)
```

Earlier positionals that exist as image files become reference images (max 3) -- pix sends them to the FAL edit endpoint:

```bash
> echo "make the sky purple" | pix gen photo.jpg edited.jpg
⚠️  Using photo.jpg as reference image (will be sent to FAL)
Cost: $0.02 (unit: images) for model xai/grok-imagine-image (source: FAL API)
Wrote edited.jpg

> echo "merge these" | pix gen a.jpg b.jpg merged.jpg
⚠️  Using a.jpg as reference image (will be sent to FAL)
⚠️  Using b.jpg as reference image (will be sent to FAL)
Wrote merged.jpg
```

Look up cost without generating:

```bash
> pix cost
Model: xai/grok-imagine-image
Unit price: $0.02 per images (source: FAL API)
Estimated cost: $0.0200 per call based on usage history (source: FAL API)
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `generate [refs...] <output>` | Generate an image from a prompt on stdin. Earlier positionals that exist as image files become reference images (max 3) and pix uses the FAL edit endpoint. Alias: `gen`. |
| `cost` | Query pricing for the configured model (no generation) |

Run `pix <subcommand> --help` for subcommand-specific usage.

### Flags

**Global flags** (placed before the subcommand):

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show usage |
| `--version` | Show version |
| `-q`, `--quiet` | Suppress non-error output |

**Subcommand flags** (placed after the subcommand):

| Flag | Subcommand | Description |
|------|------------|-------------|
| `--dry-run` | generate, cost | Show what would happen without calling the API |
| `-p`, `--preview` | generate | Open the image after generation |
| `--load-prompt` | generate | Pick a saved prompt via fzf (or configured picker) |
| `--no-load-prompt` | generate | Disable load-prompt mode (overrides `load-prompt.always`) |
| `--pick-model` | generate | Pick a FAL model from the live catalogue via the picker |
| `--no-pick-model` | generate | Disable model-picker mode (overrides `model-picker.always`) |

`--help` is mutually exclusive with all other flags and arguments.

## Configuration

Configuration lives at `~/.config/pix/config.yaml` (or next to the binary during development).

```yaml
# Model to use for image generation
model: xai/grok-imagine-image

# API key resolution (optional -- falls back to FAL_KEY env var or .env file)
api-keys:
  fal:
    # Run a command to retrieve the key (e.g. password manager)
    command: op read op://vault/fal/credential
    # Or read from a file
    # file: /path/to/fal.key

# Custom preview command (optional -- defaults to open/xdg-open/start)
# preview-command: chafa

# Interactive-only settings: these apply when stdin is a TTY (a user is sitting
# at the keyboard). Piped or redirected invocations silently bypass everything
# in this block -- pix stays scriptable.
# interactive:
#   picker: fzf                          # shared by --load-prompt and --pick-model (default: fzf)
#   load-prompt:
#     path: ~/.config/pix/prompts        # directory of saved prompt files
#     always: false                      # if true, --load-prompt is implicit on every gen
#   model-picker:
#     always: false                      # if true, --pick-model is implicit on every gen
```

> **Config migration (2026-05-13):** the previous flat layout (`picker:`, `load-prompt:`, `model-picker:` at top level) is gone. Move those keys under a single `interactive:` parent block as shown above. The reorganization makes the TTY-only nature of these settings clear at-a-glance.

### Saved prompts

`--load-prompt` opens the configured picker (default `fzf`) on the saved-prompts directory. Pick a file, and pix:

1. Reads the file's contents as the base prompt.
2. Displays it on stderr.
3. Reads one line from stdin -- Enter sends as-is, any text becomes a suffix joined by a blank line.

Cancelling the picker exits 0 without contacting FAL. When stdin is piped with content, pix uses the piped content as the prompt directly and skips the picker -- so `echo "..." | pix gen out.png` works whether or not `--load-prompt` is in play. Set `load-prompt.always: true` to make the flow implicit, with `--no-load-prompt` available per invocation when you want to type a prompt directly.

### Model picker

`--pick-model` fetches FAL's live `/v1/models` catalogue and presents it via the same picker as `--load-prompt`. The category filter adapts to your invocation:

- No reference images on the command line -> `category=text-to-image`.
- One or more reference images -> `category=image-to-image`.

The selected `endpoint_id` is used as the FAL model for this invocation only; `config.yaml` is not modified. Set `model-picker.always: true` to make the flow implicit on every `pix gen`, with `--no-pick-model` to opt out per invocation.

No caching for now -- every invocation hits FAL `/v1/models`. The fetch is fast (one round-trip, ~30s timeout).

### API key resolution priority

1. `FAL_KEY` environment variable
2. `api-keys.fal.command` in config (stdout is the key)
3. `api-keys.fal.file` in config (file contents are the key)
4. `.env` file in the config directory (`FAL_KEY=...`)

## Extension handling

If no file extension is provided, the API response format is used (typically `.jpg`). If the requested extension differs from the API format, ImageMagick (`magick`) converts automatically. If `magick` is not available, the tool exits with an error.

## Documentation

| Document | Description |
|----------|-------------|
| [Vision](docs/vision.md) | Project goals, how it works, technology choices |
| [Architecture](docs/architecture.md) | Component overview, design decisions, [roadmap](docs/architecture.md#roadmap) |

## Project files

| File | Purpose |
|------|---------|
| `main.go` | CLI entry point and subcommand dispatch |
| `genimg.go` | `generate` subcommand handler (alias `gen`) |
| `cost.go` | cost subcommand handler |
| `config.go` | Config loading and API key resolution |
| `fal.go` | FAL API HTTP client helpers |
| `config.yaml` | Default model configuration |
| `Makefile` | Build, test, install, lint targets |
| `tests/regression/` | Regression test suite (54 tests) |
| `tests/one_off/` | One-off tests |

## Development

```bash
make build          # Compile binary
make test           # Lint + run regression tests
make test-one-off   # Run one-off tests
```

All regression tests use local HTTP test servers -- no real API calls, no API key needed.

## Licence

MIT -- Copyright Tadg Paul
