Usage: pix models [filter]

Lists active FAL image-model endpoint_ids, one per line on stdout.
Pipeable: `pix models | grep flux | head`.

Image-model categories included: text-to-image, image-to-image.
Other FAL categories (text-to-video, image-to-3d, training) are excluded.

Arguments:
  filter           Optional regex; only endpoint_ids matching are listed.
                   Bare substrings work too (they compile as literal regex).
                   Same semantics as the model-picker preselect.

Flags:
  -h, --help       Show this help message

Global flags:
  -q, --quiet      Suppress non-error output

Examples:
  pix models                 # list every image model
  pix models flux            # only models containing 'flux'
  pix models '^xai/'         # anchored regex
  pix models | head -20      # pipeable

Run 'pix --help' for the top-level usage and the full subcommand list.
