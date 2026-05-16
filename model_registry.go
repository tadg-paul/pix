// ABOUTME: Registry of FAL model quirks: explicit base->edit endpoint pairs.
// ABOUTME: Ported in spirit from storyboard-gen/src/storyboard_gen/model_registry.py.

package main

// editSiblings maps a "text-to-image" model id to its corresponding "edit"
// endpoint id. Used when reference images are present.
//
// Why this exists: FAL's endpoint naming for edit pairs is not heuristic-
// friendly. Most models follow the `<model>/edit` suffix pattern, but some
// don't:
//
//   - xai/grok-imagine-image           -> xai/grok-imagine-image/edit       (suffix; OK by heuristic)
//   - fal-ai/glm-image                 -> fal-ai/glm-image/image-to-image   (different suffix)
//   - fal-ai/bytedance/seedream/v4.5/text-to-image -> .../v4.5/edit         (rewrite, not suffix)
//   - fal-ai/flux-pro/kontext          -> fal-ai/flux-pro/kontext           (base IS the edit endpoint)
//
// For kontext, the base endpoint itself accepts `image_url` for editing;
// adding `/edit` produces a 404. The map entry maps the model to itself so
// editEndpointFor() short-circuits the suffix heuristic.
//
// Models not listed here fall through to the suffix heuristic (`<model>/edit`),
// which is correct for the majority of FAL families.
var editSiblings = map[string]string{
	// Kontext family: base endpoint is the i2i endpoint; no suffix.
	"fal-ai/flux-pro/kontext":           "fal-ai/flux-pro/kontext",
	"fal-ai/flux-pro/kontext/max":       "fal-ai/flux-pro/kontext/max",
	"fal-ai/flux-pro/kontext/max/multi": "fal-ai/flux-pro/kontext/max/multi",
	"fal-ai/flux-kontext/dev":           "fal-ai/flux-kontext/dev",

	// GLM Image: edit endpoint is image-to-image, not edit.
	"fal-ai/glm-image": "fal-ai/glm-image/image-to-image",

	// Seedream: t2i -> edit (rewrite the trailing path component).
	"fal-ai/bytedance/seedream/v4.5/text-to-image":      "fal-ai/bytedance/seedream/v4.5/edit",
	"fal-ai/bytedance/seedream/v5/lite/text-to-image":   "fal-ai/bytedance/seedream/v5/lite/edit",

	// Emu: t2i -> edit-image.
	"fal-ai/emu-3.5-image/text-to-image": "fal-ai/emu-3.5-image/edit-image",
}

// editEndpointFor returns the endpoint id to POST to when generating with
// reference images present. Looks up the explicit map first; falls back to
// the suffix heuristic <model>/edit.
func editEndpointFor(model string) string {
	if e, ok := editSiblings[model]; ok {
		return e
	}
	return model + "/edit"
}
