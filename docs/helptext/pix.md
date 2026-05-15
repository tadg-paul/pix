Usage: pix [global flags] <subcommand> [subcommand args]

A minimal CLI for generating images via the FAL API.

Subcommands:
  generate   Generate an image from a text prompt (stdin)
             Accepts optional reference images as earlier positionals (max 3).
             Alias: gen
  cost       Query pricing for the configured model

Global flags:
  -h, --help       Show this help message
  --version        Show version
  -q, --quiet      Suppress non-error output

Recognized flags may appear in any position (before, after, or interleaved
with positional arguments). The subcommand is the first non-flag token.

Run 'pix <subcommand> --help' for subcommand-specific usage.
