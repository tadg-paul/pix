Usage: pix generate [flags] [reference-images...] <output-file>

Alias: gen

Reads a text prompt from stdin and generates an image via the FAL API.

If earlier positional arguments are existing image files, they are sent
as references to the model's edit endpoint (max 3). The last positional
is always the target output filename.

Flags:
  -h, --help          Show this help message
  --dry-run           Show what would happen without calling the API
  -p, --preview       Open the image after generation
  --load-prompt       Pick a saved prompt via fzf (or configured picker)
  --no-load-prompt    Disable load-prompt mode for this invocation
  --pick-model        Pick a FAL model from the catalogue via the picker
  --no-pick-model     Disable model-picker mode for this invocation

Global flags (place before subcommand):
  -q, --quiet         Suppress non-error output

Interactive features (--load-prompt and --pick-model, plus their config
defaults under interactive: in config.yaml) only fire when stdin is a
TTY. Piped or redirected invocations silently bypass them so scripts
stay scripts.
