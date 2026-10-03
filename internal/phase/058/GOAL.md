# Phase 58 — the `(pos_T *)-1` tests go

*Formerly phase 130. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

`get_address()`, `nv_gomark()` and `nv_pcmark()` each compared a mark lookup
with `(pos_T *)-1`, the value vim once returned for "a mark in another file".
Nothing in this tree returns it: `getmark()` is `getmark_buf_fnum()`, which
returns a pointer into the buffer or NULL, and `movechangelist()` returns NULL
or an element of `b_changelist`. So each test was an `if` never taken, and
`cutil.FoldNever` folds the three away, keeping the branch that runs. The Go
transpilation had written each as `if false` (`internal/gen/FINDINGS.md`, finding 10).

**Declared delta: nothing.** The check proves the tests were dead from the
input — every mention of `(pos_T *)-1` is one of the three tests, and no
`return` of the four functions can produce it — and that the file lost
exactly the tests, their bodies and their `else` lines, counted from the input.
Its probe drives every way the three sites are reached, `'a`, `` `a ``, `:'a`
and `g;`, on both binaries; each control (an unset mark, an empty change list)
must write different bytes.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B1c) the phase runs on the graph: its one `FoldNever` finds the three ifs by their condition's form, `(== ?p (cast (ptr pos_T) (- 1)))`, on `crefactor/graph`'s verbs, its report the text version's, and `whim-build-check` holds q058 to the bytes the text version made (which is in history, `4c8b3a7` and before).
