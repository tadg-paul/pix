Bug-fix issue. References [AC8.3](https://github.com/tadg-paul/pix/issues/8) and [AC10.4](https://github.com/tadg-paul/pix/issues/10) -- no new AC table is created.

## Bug

Both pickers currently treat any non-zero exit (Esc, Ctrl-C, fzf cancel signals) as "user cancellation -> pix exits 0". This was specced in AC8.3 and AC10.4, but in practice it conflates two different user gestures:

- **Esc** -- "I changed my mind about this picker; fall through to whatever the default would be."
- **Ctrl-C** -- "Kill pix entirely."

The picker UX needs to be consistent with how the rest of pix handles cancellation. Following #13's resolution that Ctrl-C is the canonical hard-exit (no Esc-handling raw-terminal mode for prompt typing), Esc on either picker should fall back to the default rather than terminate pix.

## Affected ACs

- [AC8.3](https://github.com/tadg-paul/pix/issues/8) -- "When the picker exits non-zero (user cancellation), pix exits with status 0 and the FAL API is not contacted."
- [AC10.4](https://github.com/tadg-paul/pix/issues/10) -- "Picker cancellation (non-zero exit) causes pix to exit 0 without calling the generation endpoint."

Both ACs over-specified the cancellation contract. The replacement contract:

- Prompt picker non-zero exit -> fall through to `readPrompt()` (the existing stdin path). The user can type a prompt directly.
- Model picker non-zero exit -> fall through to `cfg.Model`. The user gets their configured default model.
- Ctrl-C continues to kill pix (SIGINT to the foreground process group); no code change there.

## Solution

In `genimg.go`:

- After `runLoadPromptFlow` returns `result.Cancelled == true`, do NOT `return 0`. Instead, fall through to the existing `readPrompt()` branch as if the picker had not run.
- After `runModelPickerFlow` returns `cancelled == true`, do NOT `return 0`. Instead, leave `pickedEndpoint` empty so the existing `endpoint := cfg.Model` path is used.

Both changes are removals of the `if result.Cancelled { return 0 }` early returns. The fall-through logic already exists.

## Test impact

- **RT-8.7** (`picker cancel exits zero, no API call`) -- assertion changes: cancellation no longer causes exit 0; instead, the FAL API IS contacted with whatever stdin/cfg.Model produces. The test needs updating (or supersession + new test).
- **RT-10.7** (model-picker cancel) -- same: assertion needs to change to "cancellation falls through to cfg.Model; FAL is called against the default endpoint".

New tests:

- **RT-15.1**: prompt picker non-zero exit -> pix reads from stdin and uses that as the prompt (no exit 0; image is written).
- **RT-15.2**: model picker non-zero exit -> pix uses `cfg.Model` for the generation endpoint URL.
- **RT-15.3**: both pickers active with `always: true`, both cancelled -> stdin prompt and cfg.Model are used; image still written.

RT-8.7 and RT-10.7 get `t.Skip()` supersession markers pointing at RT-15.1 and RT-15.2 respectively.
