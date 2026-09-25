# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started


- **The generic C refactorings, separated from vim's** (`doc/VIM-VS-GENERIC.md`).
  12 phases are generic C, 93 mixed (most apply a generic kernel to targets vim
  chooses), 57 vim-specific. The proposal: a library `internal/crefactor/`
  (printer, sweep, text verbs, transformations, analysis, pipeline, Go
  generation) and a vim side holding the plan, the cutters and one `Profile`
  value that passes in what the library must not hard-code: the sweep's roots,
  where the core ends, the truth constants, the allocator and free functions.
  Nine steps, each byte-identical under `whim-build-check`, about 8-11 days;
  the first is the sweep's roots as a parameter. Also found: the canonical
  printer drops `#define` and resolves `#if` silently, and phases 164 and 168
  each have their own test of whether a statement ends in a jump.

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
