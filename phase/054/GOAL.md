# Phase 54 — no option without a variable

A row of `options[]` whose variable is `(char_u *)NULL` is an option `:set` accepts,
reports and ignores: its feature was never compiled in — folding, syntax, the GUI,
printing, cscope, the interpreter DLLs — or went in an earlier phase. **172 of
them.** `phase/054/check.go` computes the set from the table rather than listing it,
so a row upstream adds later without a variable goes too, and hands it to
`dropoptions.py`. None was buffer- or window-local, and no code outside the table
names one by string.

**The pattern met two traps, both worth keeping.** The variable field has to be
matched, not the row: a string default is often `(char_u *)NULL` too, and a first
count by row put `'messagesopt'`, `'wincolor'` and `'winhighlight'` among them. And
the spacing varies — `'termguicolors'` is `(char_u*)NULL` — so a pattern with the
space found 165. **And a row's flags can wrap onto a second line** — `'diffopt'`,
`'foldmarker'`, `'guifont'`, `'guifontwide'`, `'breakindentopt'` and `'undodir'` — so
a flag list without whitespace in it left those six behind; they were found only
when the options that survived were read by eye. The post-condition had a trap of its own: a row's flags and its
variable are on two lines, so a `grep` for rows left counted 0 whatever was left.
It reads across lines now, and was checked to count 172 on the phase's input.

The phase checks `:set sw` still works and that `'foldmethod'`, `'cursorline'`,
`'undofile'` and `'clipboard'` are unknown.

## The delta

**None the harnesses record** — no case sets an option without a variable.
Measured: 110,672 → **110,025 lines**.
