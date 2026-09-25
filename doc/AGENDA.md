# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started


- **`crefactor` as a Go module of its own** (`doc/VIM-VS-GENERIC.md` §4,
  step 9, optional). Steps 1-8 are done: `internal/crefactor/{pipeline,text,
  xform,togo}` import nothing of whim's -- only `internal/cc`, `cemit` and
  `sweep`, which would have to move with it. What it buys: a boundary the
  compiler enforces rather than one kept by review. Nothing needs it yet.

## Known stale, not yet scoped


## Declined, with the reason recorded

**The C strings as Go slices** (`GO-IDIOMS.md` item 7, its last part). Every
`char *` in the program is one pointer class, 1,561 objects merged by flows, and
it is ordered and compared across pointers into one array: `p < end`, `p == q`,
`p - s`. A slice has no such identity. The rule that makes a forward-only
class a slice (39 classes, 89 objects) cannot reach it, and a string model
that keeps an identity -- a base and an offset, which is `Ptr[byte]` -- is what
the editor already has. Revisit only as a redesign of the string
representation, not as a generator rule.

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
