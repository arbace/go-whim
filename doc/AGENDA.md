# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

- **A wider test suite.** `make whim-test` (`internal/suite`) is the minimal
  one: 44 key sessions on stdin, compared against HEAD's build, 24x80, no pty,
  no files, no startup options. What it cannot see -- terminal handling,
  resize, reading and writing files, `argv` -- the archived suite (`448e9a8`:
  `internal/harness`, its corpus and pty harness) covered and is where a wider
  one can be read from. It checks the Go editor against the C on the same
  cases, so a wider corpus widens both.
- **The Go editor, idiomatic.** `doc/GO-IDIOMS.md` measured it and ranked the
  work. Lint-clean is done (vet, staticcheck, `gofmt -s` all 0: generator rules,
  phases 164-165), and so are the yes/no functions typed `bool` (phase 166, 278
  functions), and so are locals and the generator's temporaries declared where they
  are made (only functions with a goto keep theirs at the top). OK/FAIL is done too:
  phase 166 counts it a yes/no, success `true`. Struct members and parameters that
  hold an answer are `bool` too. The nine C string functions are Go's own now
  (`editor/libc.go`), and the key codes have their names (phase 167). Each step is checked
  by `make whim-test`, which runs the Go editor against the C.

- **The steps the canonical print and the sweep made redundant.**
  `doc/REDUNDANT-STEPS.md` measured them, each byte-identical when removed. Done:
  the phases that did nothing (82, 99, now `NoSource`), the three plan steps
  (funcreach at 5, the inner sweeps at 21 and 53), and the shared layout
  helpers (the blank-line eats, `\n\n?`, `Dedent4`). Left, one commit each, held
  by `make whim-build-check`: the hand-written layout code in single phases
  (93, 110, 121-128, 141-145), then the about 300 hand deletions the sweep makes
  anyway, relaxing each phase's own counts in the same commit.

- **A tighter verb set for phase edits** (`doc/DSL.md`). A separate language would
  cover only 10-22% of the edit code; the savings come from one verb set in
  `edit.E` (seven spellings of FoldNever today), verbs for what phases
  hand-write, and moving the ~20 mostly-declarative phases onto them. Best done
  together with the REDUNDANT-STEPS removals, which touch the same phases.

## Known stale, not yet scoped


## Declined, with the reason recorded

**In-AST editing.** `doc/AST-EDITING.md`, and `GOALS.md`'s *What comes next*.
Not on cost: `internal/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.
