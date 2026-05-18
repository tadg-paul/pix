Usage: pix cost [model-id-or-substring] [flags]

Queries pricing for a FAL image model without generating an image.
Reports both the unit price and the historical estimate based on past usage.

Model resolution order:
  1. Positional argument, if given: used verbatim.
     e.g. `pix cost xai/grok-imagine-image`
  2. `model:` in config.yaml, if it resolves to ONE match in the live
     /v1/models catalogue (substring/regex against image-only models).
  3. Ambiguous or no match:
     * Interactive terminal: opens the model picker for selection.
     * Scripted: errors out with the resolver's reason.

Use `pix models` to list available endpoint_ids for scripting.

Flags:
  -h, --help       Show this help message
  --dry-run        Show what URLs would be queried without making API calls

Global flags:
  -q, --quiet      Suppress output (exits zero with no stdout/stderr)

Recognised flags may appear in any position.

Run 'pix --help' for the top-level usage and the full subcommand list.
