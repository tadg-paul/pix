> **Discovery framing.** This issue carries a starter AC table; ACs are expected to grow and refine as the work is validated interactively. SATISFIED migrates them to `docs/ACs.md` per ISSUES.md.

## Problem

The `interactive` config block currently conflates two concerns under `load-prompt`:

- **Picker behaviour** (`always`, `filter`) -- whether/how the prompt picker fires.
- **Data source** (`path`) -- where saved prompts live on disk.

Adding fields like `filter` and (next) `preselect` keeps growing under `load-prompt`, but those are picker-behaviour concerns, not properties of the prompt library. The model picker has the same shape on its side (`always`, `filter`, soon `preselect`) but no symmetric naming.

Two missing capabilities motivate the cleanup:

1. **Model preselect.** `interactive.model-picker.preselect: fal-ai/grok-imagine-image` should cause that model to appear as the first line in the picker, so a returning user can press Enter to confirm their habitual choice (and still arrow/type to change). FAL's `/v1/models` does not provide preselect semantics; pix orders the candidate list itself.
2. **Symmetric naming.** A `prompt-picker` block parallel to `model-picker` makes the schema self-explanatory: data goes under `load-prompt`; behaviour goes under `*-picker`.

## Solution

Refactor `interactive`:

```yaml
interactive:
  picker: fzf                          # default picker for both flows
  prompt-picker:
    always: true                       # was: interactive.load-prompt.always
    filter: "image"                    # was: interactive.load-prompt.filter
  load-prompt:
    path: ~/snips/prompts-genai        # data source -- where prompts live
  model-picker:
    always: true                       # unchanged
    filter: ""                         # unchanged
    preselect: fal-ai/grok-imagine-image  # NEW -- ordering hint for the candidate list
```

Behaviour notes:

- `prompt-picker.always` replaces `load-prompt.always`. The old key is dropped; yaml-v3 silently ignores unknown fields, so a stale config does not error but the picker simply does not fire. A README migration note covers this.
- `prompt-picker.filter` replaces `load-prompt.filter`. Same migration shape.
- `model-picker.preselect`, when set, sorts the candidate list so the matching `endpoint_id` is the first line. fzf highlights the first line by default, so the user can confirm with Enter. If the preselect value does not appear in the FAL response (wrong category, deprecated, etc.), the candidate order falls back to the default and pix does not error.
- `interactive.picker` remains the single source of truth for the picker command.

### Acceptance Criteria

| ID | AC | Tests |
|---|---|---|
| AC12.1 | Configuration accepts `interactive.prompt-picker` as a block with keys `always` (bool, default `false`) and `filter` (string, default empty). | ✅ RT-12.1: config containing `interactive.prompt-picker: { always: true, filter: "x" }` parses and drives the load-prompt flow accordingly<br>✅ RT-12.2: config omitting `interactive.prompt-picker` entirely defaults to `always:false`, `filter:""` (picker not fired by default) |
| AC12.2 | Configuration accepts `interactive.model-picker.preselect` (string, default empty). | ✅ RT-12.3: config containing `preselect: "fal-ai/...."` parses<br>✅ RT-12.4: config omitting `preselect` defaults to empty |
| AC12.3 | When `interactive.prompt-picker.always: true` and stdin is a TTY, `pix gen` activates the saved-prompt picker as if `--load-prompt` were supplied. When stdin is piped, the picker is silently bypassed (matching the AC8.17 contract). | ✅ RT-12.5: `always:true` + TTY stdin -> picker fires<br>✅ RT-12.6: `always:true` + piped stdin -> picker not fired; piped content used as prompt |
| AC12.4 | When `interactive.prompt-picker.filter` holds a non-empty string and the configured picker resolves to `fzf`, the fzf invocation includes `--query=<filter>` as an argument. | ✅ RT-12.7: stub fzf logs `--query=<value>` when `prompt-picker.filter` is set<br>✅ RT-12.8: stub fzf does not log `--query=` when `prompt-picker.filter` is empty |
| AC12.5 | When `interactive.model-picker.preselect` matches an `endpoint_id` returned by the `/v1/models` lookup for the active category, that endpoint_id appears as the first line of the candidate list given to the picker. When the preselect value does not match any returned endpoint, the candidate order is the default. When the preselect value is empty, the candidate order is the default. | ✅ RT-12.9: preselect matches a model in the result -> first line is that model<br>✅ RT-12.10: preselect does not match -> candidate order is unchanged<br>✅ RT-12.11: preselect is empty -> candidate order is unchanged |
| AC12.6 | The legacy keys `interactive.load-prompt.always` and `interactive.load-prompt.filter` are not read; their presence has no effect on pix's runtime behaviour. | ✅ RT-12.12: config with `interactive.load-prompt.always: true` (legacy) does not trigger the picker<br>✅ RT-12.13: config with `interactive.load-prompt.filter: "x"` (legacy) does not pass `--query` to fzf |
| AC12.7 | README and the `docs/helptext/*.md` files reference the new schema; the old keys appear only in a migration note. | ⏳ UT-12.1: visual review of README configuration block, generate.md, cost.md, pix.md |

**Key:** ✅ passing · ⏳ pending · ❌ failing · ~~🚫 removed~~

## Out of scope

- Per-flow `picker:` override (e.g. `prompt-picker.picker: sk` while `model-picker` uses `fzf`). Possible future refinement; the single top-level `picker:` covers the current need.
- Pricing in `--pick-model` preview. Tracked separately.
- Deprecation-warning logging when legacy keys are present in config. The silent-drop approach is acceptable given the install base is one user.

## Implementation summary

- `config.go`: `loadPromptConfig` reduced to `{ Path }` only. New `promptPickerConfig { Always, Filter }`. `modelPickerConfig` gains `Preselect string`. `interactiveConfig` embeds `PromptPicker` parallel to `LoadPrompt` and `ModelPicker`.
- `prompts.go`: reads `cfg.Interactive.PromptPicker.Filter` for `--query` plumbing.
- `genimg.go`: `useLoadPrompt` resolution reads `cfg.Interactive.PromptPicker.Always`.
- `models.go`: new `reorderPreselect()` partitions the model slice so the preselected `endpoint_id` (if found) leads. Called between `fetchModels()` and candidate-list assembly.
- `tests/regression/pix_test.go`: helper `loadPromptConfigYAML` emits the new schema. RT-11.17 (filter test) migrated to `prompt-picker.filter`. RT-12.1 through RT-12.13 added.
- `README.md`: configuration example reflects the new layout; migration note explains the move and mentions `preselect`.
- `docs/helptext/generate.md`: TTY-only paragraph updated to reference the new key paths.

Test totals after this change: **148 PASS, 15 SKIP, 0 FAIL**.

Awaiting UT-12.1 + APPROVED 12.
