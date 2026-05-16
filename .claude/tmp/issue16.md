Bug-fix issue. References [AC11.8](https://github.com/tadg-paul/pix/issues/11) and [AC12.4](https://github.com/tadg-paul/pix/issues/12) -- no new AC table is created.

## Bug

`interactive.prompt-picker.filter` and `interactive.model-picker.filter` are currently passed to fzf as `--query=<value>`. This pre-fills the search box but lets fzf do its default fuzzy matching against the displayed text. The user reports:

> "prompts were surfacing unfiltered. should have only filtered a substring match at least, regex match better"

In other words: setting `filter: "image"` returned the full list, not just entries whose path/id contains "image". The fzf-fuzzy semantics don't match the user's mental model of "filter" (which is "narrow the candidate list to matches").

## Affected ACs

- [AC11.8](https://github.com/tadg-paul/pix/issues/11) -- "When the configured picker is `fzf` and a `filter` value is set on `interactive.load-prompt` or `interactive.model-picker`, that value is passed to fzf as `--query=<filter>`..."
- [AC12.4](https://github.com/tadg-paul/pix/issues/12) -- "When `interactive.prompt-picker.filter` holds a non-empty string and the configured picker resolves to `fzf`, the fzf invocation includes `--query=<filter>` as an argument."

Both ACs specified the wrong mechanism. The replacement contract:

- `filter` is a Go regular expression (RE2 syntax) compiled at pix's level.
- Candidates (file paths for prompt-picker, endpoint_ids for model-picker) are filtered through `re.MatchString` before being sent to the picker.
- Non-matching candidates are dropped entirely; the picker only ever sees the filtered subset.
- Invalid regex -> warning on stderr, proceed with no filtering.
- Aligns the `filter` semantics with `preselect` (also regex, since #14).

## Solution

`prompts.go::runLoadPromptFlow`:

- Stop appending `--query=<filter>` to fzf args.
- After `listPromptFiles(resolvedPath)`, if `cfg.Interactive.PromptPicker.Filter != ""`, compile it as a regex and retain only entries where `re.MatchString(path)` is true.
- On compile failure, log a one-line warning to stderr and skip the filter (preserve all entries).
- If post-filter the list is empty, error out with "no prompts match filter `<value>`" so the user knows their regex was too tight (rather than seeing an empty fzf).

`models.go::runModelPickerFlow`:

- Stop appending `--query=<filter>` to fzf args.
- After `fetchModels(...)`, if `cfg.Interactive.ModelPicker.Filter != ""`, compile and apply the same way against `m.EndpointID`.
- Same compile-error and empty-result handling.

Both share a small helper `applyRegexFilter(items []T, re *regexp.Regexp, extract func(T) string) []T` -- or inline if the code stays short.

## Test impact

- **RT-11.17, RT-11.18, RT-12.7, RT-12.8** assert on the `--query=` substring in fzf argv. Those assertions become invalid; tests become `t.Skip()` supersession markers per the immutability rule.

New tests:

- **RT-16.1**: prompt-picker.filter `image` against files `[garden.md, image-1.md, image-2.md, sunset.md]` -> picker stdin receives only `image-1.md` and `image-2.md`.
- **RT-16.2**: model-picker.filter `^xai/` against `[fal-ai/aaa, xai/a, xai/b, fal-ai/zzz]` -> picker stdin receives only `xai/a` and `xai/b`.
- **RT-16.3**: invalid regex on prompt-picker.filter -> stderr warning + full unfiltered list passed to picker.
- **RT-16.4**: prompt-picker.filter that matches zero files -> pix exits non-zero with a "no prompts match filter" error, picker not invoked.
- **RT-16.5**: model-picker.filter that matches zero models -> pix exits non-zero with a "no models match filter" error, picker not invoked.

RT-11.17, RT-11.18, RT-12.7, RT-12.8 get `t.Skip()` markers referencing RT-16.1 / RT-16.2.
