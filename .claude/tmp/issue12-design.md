## Solution design

### Language and libraries

Go 1.22+, stdlib only. `gopkg.in/yaml.v3` (already a dep).

### File plan

| File | Change |
|---|---|
| `config.go` | Drop `Always` and `Filter` from `loadPromptConfig` (leaving only `Path`). Introduce `promptPickerConfig` with `Always bool` and `Filter string`. Add `Preselect string` to `modelPickerConfig`. Embed `PromptPicker promptPickerConfig \`yaml:"prompt-picker"\`` into `interactiveConfig`. |
| `prompts.go` | `runLoadPromptFlow`: read `cfg.Interactive.PromptPicker.Filter` (not `cfg.Interactive.LoadPrompt.Filter`) when assembling fzf args. |
| `genimg.go` | `useLoadPrompt` resolution reads `cfg.Interactive.PromptPicker.Always` instead of `cfg.Interactive.LoadPrompt.Always`. |
| `models.go` | After fetching the candidate list and before sending to the picker, if `cfg.Interactive.ModelPicker.Preselect != ""` and the value matches any entry's `endpoint_id`, partition the slice so that entry is first; remaining entries keep their existing (alphabetical / API) order. If no match, no reordering. |
| `docs/helptext/{generate,cost,pix}.md` | Update narrative references where the old keys appear. |
| `README.md` | Update the configuration example to show the new layout. Add a one-paragraph migration note (legacy keys are silently ignored; users move `always`/`filter` from `load-prompt` to `prompt-picker`). |
| `tests/regression/pix_test.go` | Update `loadPromptConfigYAML` and `modelPickerConfigYAML` helpers to emit the new schema. Append `RT-12.x` tests. |

### Patterns to follow

- **YAML struct tags** stay kebab-case (`yaml:"prompt-picker"`) -- consistent with `model-picker`, `load-prompt`.
- **Preselect reordering** uses a stable partition: find the preselect index in the sorted slice; if found, slice it out and prepend. No re-sort of the rest; this keeps the candidate order predictable and deterministic for tests.
- **Test stubs for fzf** continue the established pattern: write a tiny `#!/bin/sh` script literally named `fzf`, log argv to a file, then check the log. Same approach as RT-11.17 / RT-11.18.
- **Test helpers** update in place; per ISSUES.md, helper renames are not test-ID changes and are free to refactor.
- **Real-user test** (TESTING.md): every RT-12.x exercises the compiled binary via `runBinary`. The captured FAL request body and picker argv log are the user-observable artefacts; no Go-internal mocking.

### Anti-patterns to avoid

- **CODING.md "Cross-Language Escaping":** the `Preselect` value (a model endpoint id) never flows into a shell-c string. It is compared as a Go string against `m.EndpointID` from the parsed JSON response. No injection risk.
- **CODING.md "Prohibited Anti-Patterns - Error Suppression":** preselect-match failure is a legitimate "not found" -> log nothing, fall back to default order. Do not swallow other errors.
- **TESTING.md "no mocks":** the preselect tests exercise the real candidate-ordering code by inspecting the picker's stdin via a logging stub. No Go test doubles for `runModelPickerFlow`.
- **TESTING.md "the real-user test":** every RT runs the binary as a subprocess. The preselect-ordering test asserts on what the stub fzf actually received on stdin, not on internal Go state.
- **Avoid silent migration shims** (CODING.md "Avoid backwards-compatibility hacks like ... // removed comments for removed code"). The legacy keys are simply removed from the struct; yaml-v3 ignores unknown fields without error. README carries the migration note.

### Test allocation

- File: `tests/regression/pix_test.go` (append).
- Range: `RT-12.1` through `RT-12.13` plus `UT-12.1`.
- Stub scripts: `t.TempDir()` per test (existing pattern).

### Edge cases baked in

- Preselect value matches a model the FAL `/v1/models` response returned for the active category -> reordered to first.
- Preselect value matches no returned model -> no-op, default alphabetical order.
- Preselect value is empty -> no-op, default order.
- Preselect value is set but `--no-pick-model` overrides everything -> the preselect is moot because the picker never runs.
- Legacy `interactive.load-prompt.always: true` in a user's config -> silently ignored. The picker does not fire on TTY invocations unless they migrate.

### Review against codebase

- The interactiveConfig refactor touches three call sites (prompts.go, genimg.go, models.go). All small, mechanical.
- The preselect ordering is a single sort/partition operation in `runModelPickerFlow` between `fetchModels` and `invokePicker`. No new dependency, no behaviour change to existing tests beyond the helper rename.
- No conflict with #11's flag-parsing changes (already in working tree). The two issues touch different config keys.

AWAITING PROCEED - issue #12

https://github.com/tadg-paul/pix/issues/12
