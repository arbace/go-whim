# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started


## Known stale, not yet scoped


## Declined, with the reason recorded

**The C strings as Go slices** (the rest of the Go-idioms survey's item 7). Every
`char *` in the program is one pointer class, 1,561 objects merged by flows, and
it is ordered and compared across pointers into one array: `p < end`, `p == q`,
`p - s`. A slice has no such identity. The rule that makes a forward-only
class a slice (39 classes, 89 objects) cannot reach it, and a string model
that keeps an identity -- a base and an offset, which is `Ptr[byte]` -- is what
the editor already has. Revisit only as a redesign of the string
representation, not as a generator rule.

**In-AST editing** (`GOALS.md`'s *What comes next*). Not on cost: `crefactor/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.

**The Go editor's globals as a struct, and a package split** (the Go-idioms
survey's items 11 and 12). The struct would rewrite 14,720 references to 854 package
variables and 11,560 calls, on a fifth of `editor.go`, to buy several editors
per process, which nothing needs; the split's payoff, in-process tests with a
fake host, needs the struct first, and `whim test` already runs the Go
editor against the C.
