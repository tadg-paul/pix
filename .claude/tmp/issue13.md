Bug-fix issue. References [AC8.8](https://github.com/tadg-paul/pix/issues/8) -- no new AC table is created.

## Bug

After the load-prompt picker exits, pix currently writes this to stderr:

```
Selected prompt:
<saved prompt body>
Add to prompt (Enter to send as-is):
```

Tadg specified the desired wording earlier in the session (see conversation 2026-05-12 / 2026-05-13). The intended display is:

```
PROMPT: (type to add, Enter to send, Ctrl-C to cancel)

<saved prompt body>

⭐ _
```

Differences:
- `Selected prompt:` -> `PROMPT: (type to add, Enter to send, Ctrl-C to cancel)` (all caps, controls on one line, mentions Ctrl-C cancellation).
- `Add to prompt (Enter to send as-is):` -> replaced by the prompt indicator on its own line.
- Star emoji (`⭐`) as the input indicator, with a space after.
- Ctrl-C naturally cancels via SIGINT -- no terminal-mode handling required.

## Affected AC

[AC8.8](https://github.com/tadg-paul/pix/issues/8) -- "After a saved prompt is selected, the prompt contents appear on stderr and pix reads one line from stdin as the additional text (suppressed under `--quiet`, but the read still happens)." The behaviour stays as specified; only the surrounding text changes.

## Solution

In `prompts.go` `runLoadPromptFlow`, when `!globalQuiet`:

- Replace `Selected prompt:` line with `PROMPT: (type to add, Enter to send, Ctrl-C to cancel)`.
- Add a blank line above and below the prompt body.
- Write `⭐ ` (star + space) before the `bufio.NewReader` read instead of `Add to prompt (Enter to send as-is): `.

The bufio read remains line-buffered; Ctrl-C delivers SIGINT to the pix process and terminates it (current default behaviour, no new code needed).

Under `--quiet`, all three lines remain suppressed; the stdin read still happens.

## Tests

The relevant assertion in RT-8.15 (`prompt contents appear on stderr`) continues to hold; the substring "MAGIC-MARKER-12345" remains present. Other existing RT-8.x assertions are unaffected (RT-8.17 quiet-suppresses-display, RT-8.18/19/20 final-prompt-equals checks). No new tests required for a wording-only change in a string already covered by the existing AC.

Optional new RT: explicitly assert the new "PROMPT:" header and the star-emoji indicator appear -- but that's testing literal copy rather than behaviour, and the wording is the kind of thing the user is most likely to tune further. Defer until the copy stabilises.
