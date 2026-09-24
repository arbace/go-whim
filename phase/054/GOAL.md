# Phase 54 — no option without a variable

A row of `options[]` whose variable is `(char_u *)NULL` is an option `:set` accepts,
reports and ignores: its feature was never compiled in — folding, syntax, the GUI,
printing, cscope, the interpreter DLLs — or went in an earlier phase. **174 of
them.** `phase/054/edit.go`'s `noVarRow` computes the set from the table rather than
listing it, so a row upstream adds later without a variable goes too, and hands it to
`dropoptions`. None is buffer- or window-local, and no code outside the table
names one by string.

**It was 172 until the text became canonical, and the two that arrived are the
reason the set is computed.** `'statusline'` and `'statuslineopt'` are no-variable
rows like the other 172, and the old anchor `\{"(\w+)",` missed them because vim
writes those two rows with spaces before the comma — `{"statusline"  ,"stl",` —
which is upstream's own text, verbatim at the commit `upstream.sha` records. Phase 0
prints one shape per construct, so the row reads `{"statusline", "stl", …` like every
other and the computation sees all 174. **Nothing is excluded and no name is listed**:
a row without a variable goes, and that rule is what found these two the moment the
spelling stopped hiding them.

**The pattern met two traps and the canonical text retired both.** The variable field
has to be matched, not the row: a string default is often `(char_u *)NULL` too, and a
first count by row put `'messagesopt'`, `'wincolor'` and `'winhighlight'` among them —
that one is still live, because it is about which FIELD is null and not about spelling.
The other two were spelling: the spacing varied, `'termguicolors'` being
`(char_u*)NULL`, so a pattern with the space found 165; and a row's flags could wrap
onto a second line — `'diffopt'`, `'foldmarker'`, `'guifont'`, `'guifontwide'`,
`'breakindentopt'` and `'undodir'` — so a flag list without whitespace in it left those
six behind. A canonical row is one line with one spelling, so neither can happen again;
the pattern still tolerates both, because tolerating a shape that no longer occurs
costs nothing and asserting its absence would be a claim about the printer made here.
The post-condition had the same history — a row's flags and its variable were on two
lines, so a `grep` for rows left counted 0 whatever was left — and it reads across
lines still. Measured on the phase's input: **174**.

The phase checks `:set sw` still works and that `'foldmethod'`, `'cursorline'`,
`'undofile'` and `'clipboard'` are unknown.

## The delta

**None the harnesses record**, and that now has to be said of two more names than it
was. No behaviour case sets an option without a variable, and the Ex sweep runs command
NAMES: `set` is one of its 600 rows and its result does not move, because the sweep
never names an option. So `'statusline'` and `'statuslineopt'` becoming unknown to
`:set` is a change the corpus cannot see, and `phase/054/delta` declares nothing —
stated here rather than widened to fit, which is what would have happened if the
declaration grammar had been given an option name it does not take.
