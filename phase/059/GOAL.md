# Phase 59 — no command-line completion

The command line no longer completes anything. In `getcmdline_int()` the
`'wildchar'` and `'wildcharm'` keys, S-Tab, CTRL-D (list), CTRL-A (insert all),
CTRL-L (longest match) and CTRL-N/CTRL-P over matches go; each of those keys is now
an ordinary character, CTRL-N and CTRL-P browse history as they did when there were
no matches, and CTRL-L still adds a character to an incremental search. The six
wild* options go with them: `'wildchar'`, `'wildcharm'`, `'wildmode'`,
`'wildoptions'`, `'wildignore'` and `'wildignorecase'`.

What completion shared with filename expansion stays: `expand_filename()` →
`ExpandOne()` with `EXPAND_FILES`, and the argument list through
`expand_wildcards()`. So `ExpandFromContext()` keeps its file branch and loses the
rest — options, mappings, buffers, highlight groups, `++opt`, every command's
arguments — and `ExpandOne()` keeps the one mode its last caller asks for. The
phase checks after the sweep that `expand_filename()` is that last caller.

**Three things kept the machinery alive, and each was found by the post-condition
greps rather than by reading.** `didset_options2()` still parsed `'wildmode'` into
`wim_flags` at startup, which nothing read. Every `options[]` row still named the
callback that completes its value — 29 of them — so the table kept `ExpandGeneric()`
and the fuzzy matcher reachable; no code reads that field any more, and the rows
now hold `NULL`. And the check that `ExpandOne()` had one caller ran first before
the sweep, when its dead callers were all still there. The sweep then took
6,208 lines.

**`:e` does not expand a wildcard, and has not since Phase 7 — deliberately.** The
first probe here asked that `:e onlyo*` edit `onlyone.txt`. It failed, and failed
identically on the previous phase's binary, which wrote a file named `onlyo*`. That
is Phase 7's declared delta, not a regression: Phase 7 replaced
`gen_expand_wildcards()` with `save_patterns()`, so `:e *.c` names a file
literally, and said so. Bisecting the boundaries confirms it — q6 expands the
pattern, q7 does not. An earlier draft of this section called the loss silent and
placed it "at or before Phase 12"; that came from testing binaries before reading
Phase 7, and was wrong. This phase does not change it, and the probe checks `:e`
on a plain name instead.

## The delta

**None the harnesses record** beyond Phase 58's. Measured: 108,374 →
**102,166 lines**.
