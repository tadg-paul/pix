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
	// "image_url" (singular) sends the first ref as a string value.
	// "image_urls" (plural, default) sends all refs as an array.
	RefField string
}

// modelHandlers is the dispatch table. Order matters: more-specific patterns
// first; the final entry (empty patterns) is the default that always matches.
var modelHandlers = []modelHandler{
	// Kontext family: singular image_url. Storyboard-gen calls these out as a
	// separate handler class; for pix we encode the difference as a single
	// declarative entry.
	{
		Patterns: []string{"kontext"},
		RefField: "image_url",
	},

	// Reve family: also singular image_url (per storyboard-gen).
	{
		Patterns: []string{"reve"},
		RefField: "image_url",
	},

	// Emu 3.5 image: singular image_url (per storyboard-gen EditHandler entry).
	{
		Patterns: []string{"emu-3.5"},
		RefField: "image_url",
	},

	// Default: plural image_urls. Covers most flux-2 variants, seedream, qwen,
	// grok, glm-image, nano-banana, gpt-image-1.5, firered, and anything new
	// pix hasn't profiled yet.
	{
		Patterns: nil, // empty -> always matches; must remain last
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
// according to the handler's RefField setting. For singular ("image_url"),
// only the first URI is sent; pix warns to stderr when extra refs are
// dropped (matching storyboard-gen's behaviour).
func (h modelHandler) refPayload(uris []string, globalQuiet bool) (string, interface{}) {
	if h.RefField == "image_url" {
		if len(uris) > 1 && !globalQuiet {
			fmt.Fprintf(os.Stderr, "Warning: model accepts a single reference image; using the first of %d (others dropped)\n", len(uris))
		}
		return "image_url", uris[0]
	}
	return "image_urls", uris
}
