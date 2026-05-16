Bug-fix issue. References [AC8.3](https://github.com/tadg-paul/pix/issues/8), [AC10.4](https://github.com/tadg-paul/pix/issues/10), [AC11.8](https://github.com/tadg-paul/pix/issues/11), and [AC12.4](https://github.com/tadg-paul/pix/issues/12) -- no new AC table is created.

## Bug 1: Esc terminates pix instead of falling back to default

Both pickers currently treat any non-zero exit (Esc, Ctrl-C, fzf cancel signals) as "user cancellation -> pix exits 0". This was specced in AC8.3 and AC10.4, but in practice it conflates two different user gestures:

- **Esc** -- "I changed my mind about this picker; fall through to whatever the default would be."
- **Ctrl-C** -- "Kill pix entirely."

Following #13's resolution that Ctrl-C is the canonical hard-exit (no Esc-handling raw-terminal mode for prompt typing), Esc on either picker should fall back to the default rather than terminate pix. Replacement contract:

- Prompt picker non-zero exit -> fall through to `readPrompt()` (the existing stdin path). The user can type a prompt directly.
- Model picker non-zero exit -> fall through to `cfg.Model`. The user gets their configured default model.
- Ctrl-C continues to kill pix (SIGINT to the foreground process group); no code change there.

**Affected ACs:**
- [AC8.3](https://github.com/tadg-paul/pix/issues/8) -- "picker non-zero exit -> pix exit 0, no FAL call"
- [AC10.4](https://github.com/tadg-paul/pix/issues/10) -- "picker cancellation -> exit 0, no generation request"

Both ACs over-specified the cancellation contract.

## Bug 2: `filter` is applied as fzf `--query` (post-display) instead of pre-filter (drop non-matches)

`interactive.prompt-picker.filter` and `interactive.model-picker.filter` currently pass the value to fzf as `--query=<value>`. fzf displays every candidate and lets the user type-narrow against the pre-filled query. User feedback:

> "prompts were surfacing unfiltered. should have only filtered a substring match at least, regex match better"

Wrong layer. The user's mental model of "filter" is "narrow the candidate list to matches before the picker opens". Replacement contract:

- `filter` is a Go regular expression (RE2 syntax) compiled at pix's level.
- Candidates (file paths for prompt-picker, endpoint_ids for model-picker) are filtered through `re.MatchString` before being sent to the picker. Non-matching candidates are dropped entirely.
- Invalid regex -> stderr warning, proceed with no filtering.
- Aligns the `filter` semantics with `preselect` (also regex, since #14).

Empty fzf is a poor UX, so:
- If post-filter the list is empty for prompt-picker, error out with "no prompts match filter `<value>`".
- If post-filter the list is empty for model-picker, error out with "no models match filter `<value>`".

**Affected ACs:**
- [AC11.8](https://github.com/tadg-paul/pix/issues/11) -- "filter passed as `--query=<filter>`"
- [AC12.4](https://github.com/tadg-paul/pix/issues/12) -- "filter passed as `--query=<filter>`"

Both ACs specified the wrong mechanism.

## Solution

### `genimg.go`

After `runLoadPromptFlow` returns `result.Cancelled == true`, do NOT `return 0`. Fall through to `readPrompt()` as if the picker had not run.

After `runModelPickerFlow` returns `cancelled == true`, do NOT `return 0`. Leave `pickedEndpoint` empty so `endpoint := cfg.Model` applies.

### `prompts.go::runLoadPromptFlow`

- Stop appending `--query=<filter>` to fzf args.
- After `listPromptFiles(resolvedPath)`, if `cfg.Interactive.PromptPicker.Filter != ""`, compile it as a regex and retain only entries where `re.MatchString(path)` is true.
- On compile failure, log a one-line warning to stderr and skip the filter.
- If post-filter the list is empty, return an error.

### `models.go::runModelPickerFlow`

- Stop appending `--query=<filter>` to fzf args.
- After `fetchModels(...)`, apply the same regex filter against `m.EndpointID`.
- Same compile-error and empty-result handling.

## Test impact

**Superseded** (`t.Skip()` with reference to the new tests):
- **RT-8.7** (prompt picker cancel -> exit 0)
- **RT-10.7** (model picker cancel -> exit 0)
- **RT-11.17, RT-11.18** (`--query` substring in fzf argv -- prompt-picker.filter and model-picker.filter)
- **RT-12.7, RT-12.8** (same shape, prompt-picker variant)

**New tests:**
- **RT-15.1**: prompt picker non-zero exit -> pix reads from stdin and uses that as the prompt; image is written.
- **RT-15.2**: model picker non-zero exit -> pix uses `cfg.Model` for the generation endpoint URL.
- **RT-15.3**: both pickers active with `always: true`, both cancelled -> stdin prompt and cfg.Model are used; image still written.
- **RT-15.4**: prompt-picker.filter `image` against files `[garden.md, image-1.md, image-2.md, sunset.md]` -> picker stdin receives only `image-1.md` and `image-2.md`.
- **RT-15.5**: model-picker.filter `^xai/` against `[fal-ai/aaa, xai/a, xai/b, fal-ai/zzz]` -> picker stdin receives only `xai/a` and `xai/b`.
- **RT-15.6**: invalid regex on prompt-picker.filter -> stderr warning + full unfiltered list passed to picker.
- **RT-15.7**: prompt-picker.filter that matches zero files -> pix exits non-zero with "no prompts match filter"; picker not invoked.
- **RT-15.8**: model-picker.filter that matches zero models -> pix exits non-zero with "no models match filter"; picker not invoked.

(Both bugs share issue #15 since they share the same fix surface and the same user-reported session.)
