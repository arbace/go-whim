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
  phases 164-165), and so are the yes/no functions typed `bool` (phase 166, 151
  functions), and so are locals declared where C declares them (the generator:
  1,411 declarations now carry their value, 605 as `x := e`). Next: the 65
  functions that return OK/FAIL, and the 971 generator temporaries still at the
  top. Each step is checked
  by `make whim-test`, which runs the Go editor against the C.

## Known stale, not yet scoped


## Declined, with the reason recorded

**In-AST editing.** `doc/AST-EDITING.md`, and `GOALS.md`'s *What comes next*.
Not on cost: `internal/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.
