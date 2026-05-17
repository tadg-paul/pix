// ABOUTME: Per-model-family quirks (declarative). Currently captures the
// ABOUTME: image_url vs image_urls difference; will grow as more quirks land.

package main

import (
	"fmt"
	"os"
	"strings"
)

// modelHandler captures the declarative quirks for a family of FAL models.
//
// Ported in spirit from storyboard-gen's EditHandler. Storyboard-gen has many
// more fields (sizing strategy, safety defaults, edit_accepts_sizing,
// prompt-rewriting, etc.) -- pix is image-only and simpler, so we keep only
// what we currently differentiate on. New fields land as new quirks emerge.
type modelHandler struct {
	// Patterns: substrings (case-insensitive) matched against the model id.
	// First handler in the registry whose Patterns matches the model wins.
	Patterns []string

	// RefField: the JSON key under which reference image URIs are sent to FAL.
	// "image_url" / "reference_image_url" (singular) send first ref as string.
	// "image_urls" / "reference_image_urls" (plural) send all refs as array.
	RefField string

	// SafetyDefaults: keys/values merged into every request payload for this
	// family. Pix is for private use; default to safety-off wherever the
	// model offers a knob (mirrors storyboard-gen safety_defaults).
	SafetyDefaults map[string]interface{}

	// Sizing: how the model wants the aspect-ratio/size expressed.
	//   "image_size"   -- FAL preset string ("landscape_16_9" etc.)
	//   "aspect_ratio" -- raw "W:H" passthrough
	//   "pixel"        -- explicit "WIDTHxHEIGHT" string
	//   ""             -- model has no sizing knob; nothing is sent
	Sizing string

	// T2IEndpointSuffix: appended to cfg.Model when generating WITHOUT refs.
	// Used by kontext, where the base endpoint is the i2i path; "/text-to-image"
	// is required for prompt-only generation. Empty for the common case where
	// the base endpoint IS the t2i path.
	T2IEndpointSuffix string

	// RequiredFields: keys/values that the model demands in every payload.
	// Example: ideogram models require {"style": "AUTO"}; without it FAL 422s.
	// Distinct from SafetyDefaults so the intent at the call site is clear.
	RequiredFields map[string]interface{}
}

// modelHandlers is the dispatch table. Order matters: more-specific patterns
// first; the final entry (empty patterns) is the default that always matches.
var modelHandlers = []modelHandler{
	// Kontext multi (max/multi): plural image_urls.
	{
		Patterns:       []string{"kontext/max/multi"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"safety_tolerance": "6"},
		Sizing:         "aspect_ratio",
	},

	// Kontext family (single-ref variants): singular image_url. The base
	// endpoint is the i2i path; T2I needs the "/text-to-image" suffix.
	{
		Patterns:          []string{"kontext"},
		RefField:          "image_url",
		SafetyDefaults:    map[string]interface{}{"safety_tolerance": "6"},
		Sizing:            "image_size",
		T2IEndpointSuffix: "/text-to-image",
	},

	// Ideogram Character: refs go to the character channel
	// (reference_image_urls). Style channel left empty (pix has no style-ref
	// concept). Requires style: "AUTO".
	{
		Patterns:       []string{"ideogram/character"},
		RefField:       "reference_image_urls",
		Sizing:         "image_size",
		RequiredFields: map[string]interface{}{"style": "AUTO"},
	},

	// Ideogram V3: typography model. No refs. Required style: "AUTO".
	{
		Patterns:       []string{"ideogram/v3"},
		RefField:       "image_urls", // unused -- v3 has no ref support, but a value is required by the struct
		Sizing:         "image_size",
		RequiredFields: map[string]interface{}{"style": "AUTO"},
	},

	// Flux 1.x flux-general: singular reference_image_url.
	{
		Patterns:       []string{"flux-general"},
		RefField:       "reference_image_url",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Reve and reve/fast/remix: singular image_url, aspect_ratio sizing.
	{
		Patterns: []string{"reve"},
		RefField: "image_url",
		Sizing:   "aspect_ratio",
	},

	// Emu 3.5 image: singular image_url, aspect_ratio sizing.
	{
		Patterns: []string{"emu-3.5"},
		RefField: "image_url",
		Sizing:   "aspect_ratio",
	},

	// Nano Banana: plural image_urls, aspect_ratio sizing.
	{
		Patterns: []string{"nano-banana"},
		RefField: "image_urls",
		Sizing:   "aspect_ratio",
	},

	// GPT Image: plural image_urls, pixel sizing.
	{
		Patterns: []string{"gpt-image"},
		RefField: "image_urls",
		Sizing:   "pixel",
	},

	// Grok Image (xAI): plural image_urls, aspect_ratio sizing.
	{
		Patterns: []string{"grok-imagine-image"},
		RefField: "image_urls",
		Sizing:   "aspect_ratio",
	},

	// Flux 2 family (incl. pro / max): plural image_urls, image_size preset.
	{
		Patterns:       []string{"flux-2"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Seedream (ByteDance): plural image_urls, image_size preset.
	{
		Patterns:       []string{"seedream"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Hunyuan Image (Tencent): plural image_urls, image_size preset.
	{
		Patterns:       []string{"hunyuan-image"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Recraft: plural image_urls, image_size preset.
	{
		Patterns:       []string{"recraft"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Instant Character: singular image_url, image_size preset.
	{
		Patterns:       []string{"instant-character"},
		RefField:       "image_url",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Flux 1.x other variants (fallback before catch-all): image_size preset.
	{
		Patterns:       []string{"flux-pro/v1", "flux/dev"},
		RefField:       "image_urls",
		SafetyDefaults: map[string]interface{}{"enable_safety_checker": false},
		Sizing:         "image_size",
	},

	// Default: plural image_urls, no sizing field passed. Covers everything
	// pix hasn't profiled. New families can be promoted up the list.
	{
		Patterns: nil, // must remain last
		RefField: "image_urls",
	},
}

// handlerFor returns the first modelHandler whose Patterns match the given
// model id. The final entry always matches.
func handlerFor(model string) modelHandler {
	lower := strings.ToLower(model)
	for _, h := range modelHandlers {
		if len(h.Patterns) == 0 {
			return h
		}
		for _, p := range h.Patterns {
			if strings.Contains(lower, p) {
				return h
			}
		}
	}
	// Defensive fallback. The default entry should always match before this.
	return modelHandler{RefField: "image_urls"}
}

// refPayload assembles the reference-image portion of a FAL request payload
// according to the handler's RefField setting. Singular fields ("image_url",
// "reference_image_url") send the first URI as a string value; plural fields
// ("image_urls", "reference_image_urls") send all URIs as an array. Singular
// handlers warn to stderr when extra refs are dropped.
func (h modelHandler) refPayload(uris []string, globalQuiet bool) (string, interface{}) {
	switch h.RefField {
	case "image_url", "reference_image_url":
		if len(uris) > 1 && !globalQuiet {
			fmt.Fprintf(os.Stderr, "Warning: model accepts a single reference image; using the first of %d (others dropped)\n", len(uris))
		}
		return h.RefField, uris[0]
	case "reference_image_urls":
		return h.RefField, uris
	default:
		return "image_urls", uris
	}
}
