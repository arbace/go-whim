# Phase 148 — allocation cannot fail

`lalloc()`, which `alloc()`, `alloc_clear()` and `lalloc_clear()` call,
returned NULL in two cases:
- **`host_alloc()` returning NULL**, which it never does. When the arena is
  full it ends the process through `host_exit()`, which longjmps and never
  returns; otherwise it returns a pointer into the arena. So the
  out-of-memory branches (release the scrollback, say E342) were dead;
- **a request for zero bytes**, which reported E341, an internal error, and
  returned NULL. That case now reports the same error and returns
  `host_alloc(0)`: a pointer to no bytes, where NULL was.

So no allocation in the core can fail, and every failure branch after an
allocation is dead (finding 9). Phase 149 folds them.

**Declared delta: nothing.** The recording never asks for zero bytes and never
runs out. The zero-byte path is an internal error that no input reaches. The
check proves from the input that `host_alloc()` never returns NULL, and
requires the new `lalloc()` body.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 149, in the group 148-149, whose phases share one purpose. There is no boundary q148 of its own any more; everything above still says what the steps do and why.
