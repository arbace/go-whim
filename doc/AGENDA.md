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
functions left plain); several run at once in one process. And the C's gotos
went where a generic rule takes them: phases 170-173 leave 49 of 185, each
one out of a loop or switch -- the backend says those as labeled breaks. In
order, each step
held to `make whim-test` (and a Java editor, once there is one, to the same
cases):

1. **A Java backend for `togo`**, from the same analysis -- which pointers walk
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
