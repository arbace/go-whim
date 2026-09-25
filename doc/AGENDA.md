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
host-side functions, 16 of them the operating system's. Done: `editor/` is
`package editor` with its host behind a `Host` interface, and the generator's
instance pass makes the state an `Editor`'s fields (1,470 methods, 215
functions left plain); several run at once in one process. In order, each step
held to `make whim-test` (and a Java editor, once there is one, to the same
cases):

1. **No `goto` in the C.** Classified (2026-09-25): 158 in 32 functions of
   `editor.c` (the Go's other 5 are `togo`'s own, for a `continue` in a
   `do`-while). All flow is reducible; no goto enters a block, loop or switch,
   or skips a declaration used after its label. 156 jump forward to a label
   in an enclosing block -- Java's `L: { ... break L; }` -- and 2 backward, as
   retry loops. Four generic `crefactor/xform` steps take 109, each a phase:
   a short tail copied over the goto (52, phase 168's rule widened), a loop's
   own exit as `break` and a goto to the next statement deleted (5), a retry
   as a loop (2), a forward exit through no loop or switch as `do { } while
   (0)` and `break` (50, with a `togo` rule printing that as `for { ...;
   break }`). The last 49 are per site -- 30 of them `getcmdline_int`'s
   command-line loop, a state variable -- and pay only if the rule is "no
   goto in the C" rather than "none the target cannot say": a Java backend
   can lower every forward goto as a labeled break. Held until the pipeline
   compaction lands, so the phases go in with their final numbers.
2. **A Java backend for `togo`**, from the same analysis -- which pointers walk
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
