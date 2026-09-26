# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

**The editor in Rust** (`doc/RUST.md`: the plan, its measurements and four
milestones). `#[repr(C)]` types and raw pointers keep C's memory model
natively, `return`/`break`/`continue` and labeled blocks its control flow;
the switch fall-through (10 functions) through the Clojure backend's basic
blocks; even the host translated rather than ported. Built by `rustc` from
std alone. A 26,000-line synthetic crate compiles in 1.7-2.7 s.

**The editor in Haskell** (`doc/HASKELL.md`: the plan, its measurements and
four milestones). C's own memory model through `Foreign` -- raw memory laid
out as the C's, a pointer an address -- so no pointer-class analysis; the
Clojure backend's basic blocks as local functions and tail calls; the host
on the `unix` package; built by `ghc` from its boot packages alone. The risk
to measure first: a 22,000-line synthetic module takes 20-29 s and 1 GB, and
the core is some 75,000 lines.


## Known stale, not yet scoped


## Declined, with the reason recorded
