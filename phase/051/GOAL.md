# Phase 51 — a byte that is not UTF-8 is kept as it is

**Phase 12 changed this without declaring it.** It made UTF-8 the only encoding by
cutting the conversion layer at its entry points, and its table lists what each
cut function now answers — but not what that does to a file that is not valid
UTF-8. `slim-vim` reads such a file by falling back to latin1 and writes its
bytes back unchanged. With no fallback, `readfile()` replaced every invalid byte
with `?` (`bad_char_behavior`'s default, `BAD_REPLACE`) and made the buffer
read-only, and a forced `:w` wrote the `?`s. Measured: `ok\n\xff bad\n` comes back
as `ok\n? bad\n` from every whim binary since Phase 12, and unchanged from
`slim-vim` and whim Phases 0 to 11. No harness case has an invalid byte, which is
how it went unnoticed; it was found planning the UTF-8 phase, and the user was
asked what an editor that only edits UTF-8 should do.

**The answer was what `++bad=keep` already did.** The byte stays in the buffer as a
byte, shows as `<ff>`, is written back as it was, and the buffer is not made
read-only; "[ILLEGAL BYTE in line N]" is still reported, because that describes
the file. So keeping is the only behaviour, and `tools/keepbytes.py` folds every
test of `bad_char_behavior` — in the UTF-8 check and in the conversion loops —
drops `++bad` from `getargopt()`, and lets the sweep take `get_bad_opt()` and the
buffer's `b_bad_char`.

The phase checks, against a UTF-8 edit as control, that a file with `\xff` is
written back byte for byte after an edit to another line, that reading it leaves
`noreadonly`, and that `++bad=keep` is refused. **Its first run failed on the
probe, not the editor**: `+s/ok/OK/` runs on the last line, where Ex mode starts,
and an `:s` that does not match there stops the `:wq` after it. The probe says
`+1s`.

## The delta

**None the harnesses record.** Measured: 114,399 → **114,275 lines**.
