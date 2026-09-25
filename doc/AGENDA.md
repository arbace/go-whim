# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

**The editor as an embeddable component, and a Java editor after it.** Both
want the same thing: the state on an instance and the host behind an
interface, produced by the generator. Measured on `editor/` (2026-09-25): 821
package variables in `editor.go` and 32 in `host.go`, 14,728 references to
them, 1,192 of 1,685 functions touching one directly; the core calls 17
host-side functions, 16 of them the operating system's. In order, each step
held to `make whim-test` (and a Java editor, once there is one, to the same
cases):

1. **A package and a `Host` interface.** `editor/` becomes `package editor`,
   importable; the launcher is a few lines of its own; the 16 OS calls go
   through a `Host` in Go types, the terminal one in a package of its own.
   One editor per process, as the C.
2. **No `goto` in the C.** 163 in 34 functions: Go keeps them, Java has none
   and its labeled break and continue cannot jump backwards. Phases, as 129-168
   were for Go: what the target cannot say goes from the C.
3. **An instance mode in `crefactor/togo`**: the globals as a struct, the
   functions its methods, the function-pointer tables (24) taking it. A
   generic option, written once in the generator. This was declined while
   nothing needed several editors per process; an embeddable component and
   a Java class do.
4. **A Java backend for `togo`**, from the same analysis -- which pointers walk
   (`Ptr[byte]`, 2,212 in the Go, is `byte[]` and an offset), which ints are
   answers, which are unsigned (570 uses: `Integer.*Unsigned`), structs copied
   by value (99 types), function pointers as interfaces -- rather than a
   translation of the Go. `whim test` then compares three editors.


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
