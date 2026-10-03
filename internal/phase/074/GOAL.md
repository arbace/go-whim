# Phase 74 — the allocation-failure branches fold

*Formerly phase 149. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

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

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3g) the step `whim74` runs on the graph: `crefactor/graph`'s `Editor.NeverNull`, the stores and tests found by their forms, the never-NULL set to its fixed point on the defn forms (its locals asked by spelling, as the text asked them: `*v =` is a store of `v`), the folds the verbs' `FoldNever`/`FoldAlways` splices, the unreached labels by edge; its report the text version's, and `whim-build-check` holds q074 to the bytes `crefactor/xform`'s `NeverNull` made (which is in history, `16717ab` and before); its test, moved with it, is `crefactor/graph/dropcalls_test.go`. Part 74a is still text, so the phase imports after it.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 148-149 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
