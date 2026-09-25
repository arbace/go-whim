# Phase 149 — the allocation-failure branches fold

A function whose every return is an allocation, or a local only ever assigned
one, can't return NULL either. The set of such functions is found to a
fixpoint starting from `host_alloc()`, and comes to 19: `vim_strsave()`,
`vim_strnsave()`, `alloc_tabpage()`, `ml_new_data()` and the rest. Each
NULL test of such a call's result that directly follows it can't go the
failure way:
- `== nullptr` is never true, and folds away;
- `!= nullptr` is always true, and folds to its body.

That covers both `v = f(...); if (v == nullptr)` and
`if ((v = f(...)) == nullptr)`. Folding one test can make another function
never-NULL, so the rule recomputes the set and goes on until nothing changes:
152 tests fold. (134 until the rule also took `T *v = f(...);`, a declaration,
and a test with one store of v in between, `wp->w_frame = frp;`. Those were
the shapes it missed, and the Go showed them as nil checks of a new() that
staticcheck calls never true.) Labels that only the folded branches jumped to go too, and
the sweep takes what only those branches used. The Go transpilation never had
these branches.

**Declared delta: nothing.** The check computes the whole output with the same
rule and the real sweep, and requires byte equality. It re-checks the
never-NULL set on the output with a second, simpler test: each function
returns only calls and names, never NULL or a literal. It requires the rule to
fold nothing more.

The transformation now lives in `internal/crefactor/xform` (`NeverNull`), with vim's knobs in `internal/whim/xform.go`.
