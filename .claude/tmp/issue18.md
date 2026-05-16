> No AC table -- this is a sketchbook. See ISSUES.md §Discovery issues.

## Problem framing

The FAL API surface is not uniform. Different model families accept different argument shapes:

- Ref field naming: `image_url` (singular, kontext, instant-character, reve, emu) vs `image_urls` (plural, most flux-2 variants, seedream, qwen-edit) vs `reference_image_url` (flux-general) vs `reference_image_urls` (ideogram-character).
- Sizing convention: `image_size` preset (`landscape_16_9`) vs `aspect_ratio` passthrough (`16:9`) vs pixel string (`1024x1024`) -- model-dependent.
- Endpoint routing: text-to-image and edit are separate endpoints. The naming is not heuristic-friendly: `xai/grok-imagine-image` -> `xai/grok-imagine-image/edit` (suffix), but `fal-ai/glm-image` -> `fal-ai/glm-image/image-to-image` (different suffix), and `fal-ai/bytedance/seedream/v4.5/text-to-image` -> `fal-ai/bytedance/seedream/v4.5/edit` (rewrite, not suffix).
- Safety options: some families need `enable_safety_checker: false`, others need `safety_tolerance: "6"`, most accept neither.
- Error envelope: the FAL gateway returns `{"error": {"type", "message", ...}}`, but individual model endpoints (the kontext family in particular) return FastAPI-style `{"detail": [{"msg": "...", "loc": [...]}]}`. pix's current envelope parser only handles the gateway shape; the detail envelope falls through to a truncated raw body dump.

pix currently has one heuristic: when reference images are present, append `/edit` to `cfg.Model` and send `image_urls`. This breaks for kontext (wants `image_url` singular at `/edit`), for glm-image (wants `image-to-image` not `edit`), and for any model whose edit-pair isn't a simple suffix.

## Why discovery

The fix shape isn't a single bug or a single feature; it's the introduction of a model-routing abstraction. storyboard-gen (a Python sibling project) already solves the same problem with a declarative `EditHandler` pattern + an `EDIT_SIBLINGS` map + a `clean_api_error` extractor that walks multiple envelope shapes. The discovery question is: what's the right Go shape for pix?

The exploration includes:

- Porting the storyboard-gen handler-pattern in spirit (declarative per-family quirks: patterns, sizing strategy, ref-field, safety defaults) without slavishly copying the OO hierarchy. Go-native shape -- structs and a dispatch slice, not abstract base classes.
- Lifting `EDIT_SIBLINGS` into pix as an explicit map (data, not heuristic).
- Replacing `formatFALErrorBody`'s single-shape parser with the multi-envelope walker from `clean_api_error` (handles `error.message`, `error`, `detail` (str), `detail` ([{"msg":...}])).
- Naming, file layout, package boundaries. Likely: `model_registry.go` (the data), `model_handler.go` (the dispatch).

## Working rules

- Code may be written freely during discovery. Commits are prefixed `wip(discovery): ...` to mark provenance.
- Sketches are throwaway by default. Nothing is canonical until promotion.
- ACs are not drafted upfront. They'll be written at `/end-discovery` once the right shape has emerged from working code.
- Session ends with `/end-discovery` (promote or rule out) -- Tadg drives that, not the agent.

## Out of scope / future

- **Shared model registry across pix and storyboard-gen.** The data (per-family quirks, EDIT_SIBLINGS) is volatile when FAL releases new models, but it lives in two different languages today. A shared YAML registry consumed by both projects is the obvious next step IF the maintenance churn justifies it. Trigger to lift the table out: three or more FAL model additions that require dual updates within a short window. Until then, duplicate the data in each project's source.
- **Cross-language code sharing.** Python/Go interop is more pain than the ~80 lines of duplicated logic costs. Not pursued.

## Reference

- storyboard-gen `providers/fal.py` (the StillHandler pattern + EditHandler declarative form)
- storyboard-gen `model_registry.py` (BACKEND_MODELS + EDIT_SIBLINGS)
- storyboard-gen `errors.py` (clean_api_error: multi-envelope walker)
- FAL gateway error shape: see issue #17
- FastAPI detail envelope: surfaced by the kontext family per Tadg's 2026-05-16 session
