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

1. **The Java editor kept current** (`doc/JAVA.md`, milestone 4). Milestones
   1-3 are done: `togo`'s Java backend writes every function of the core,
   `jeditor/` is its host on a terminal, and `whim test --java` requires the
   Java editor to answer every case as the C does -- 45 of 45, and 240 of 240
   with `--wide`. `Editor.java` is generated from the candidate at test time
   and not tracked; tracking it, written by `make whim-build` as `editor.go`
   is, with a check that refuses a stale one, is what is left.


## Known stale, not yet scoped


## Declined, with the reason recorded
