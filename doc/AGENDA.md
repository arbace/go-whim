# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

- **A wider test suite.** `make whim-test` (`internal/suite`) is the minimal
  one: 45 key sessions on stdin, compared against HEAD's build, 24x80, no pty,
  no files, no startup options. What it cannot see -- terminal handling,
  resize, reading and writing files, `argv` -- the archived suite (`448e9a8`:
  `internal/harness`, its corpus and pty harness) covered and is where a wider
  one can be read from. It checks the Go editor against the C on the same
  cases, so a wider corpus widens both.
- **The Go editor, the rest of idiomatic.** `doc/GO-IDIOMS.md`'s items 1-6 are
  done (lint, dead code, `bool`, scoped locals, libc, key names), and item 7's
  `*T` for a pointer that never walks, and item 10 (phase 168); 11 and 12
  are declined (below). Left of item 7: a pointer that walks forward as a
  resliced `[]T`, which for `Ptr[byte]` is a string model -- a redesign, not a
  rule. It is checked by `make whim-test`, which runs the Go editor against
  the C.
- **A tighter verb set for phase edits** (`doc/DSL.md`). A separate language would
  cover only 10-22% of the edit code; the savings come from one verb set in
  `edit.E` (seven spellings of FoldNever today), verbs for what phases
  hand-write, and moving the ~20 mostly-declarative phases onto them. The phases
  are leaner now (the redundant-steps removals landed), so the census is worth
  re-running before starting.

## Known stale, not yet scoped


## Declined, with the reason recorded

**In-AST editing.** `doc/AST-EDITING.md`, and `GOALS.md`'s *What comes next*.
Not on cost: `internal/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.

**The Go editor's globals as a struct, and a package split** (`GO-IDIOMS.md`
items 11 and 12). The struct would rewrite 14,720 references to 854 package
variables and 11,560 calls, on a fifth of `editor.go`, to buy several editors
per process, which nothing needs; the split's payoff, in-process tests with a
fake host, needs the struct first, and `whim test` already runs the Go
editor against the C.
