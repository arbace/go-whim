# Phase 60 — nothing frees

*Formerly phase 132. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

Since phase 124 `host_free()` has an empty body — the host's arena is a bump
allocator, and a garbage collector is assumed — so `vim_free()`, a NULL test
around it, does nothing observable, and neither does any call to either. The
Go transpilation dropped every one (finding 9); this is the C catching up. All
273 calls in the core go: a call whose argument has no side effect goes
entirely, and the two whose argument decrements a counter
(`termcodes[--tc_len].code`, `uep->ue_array[--n].ul_line`) become that
decrement. `vim_free()` is then called by nothing and the sweep takes it; the
host keeps `host_free()`, which its formatter still calls.

A local whose only reader was a free is then only ever given values, which gcc
calls *set but not used* and the sweep leaves: the statements that set it are,
to gcc, its uses. So the phase also applies `edit.DeadStores`: a local declared
alone on its line in a function body, every other mention of which is a
statement `name = E;` with `E` only reading, goes with its stores. On q128 it
finds nothing, on this phase's edit exactly one (`taep` in
`clear_hl_tables()`) -- the same set gcc warns about, measured.

**Declared delta: nothing.** The check proves the premise from the input —
`host_free()`'s body is empty, `vim_free()` only calls it — and then **computes
the whole output**: the input with every call replaced by the same rule
(`edit.W132Rule`), and `vim_free()`'s definition and prototype removed, must be
the output byte for byte, the dead stores taken by the same function. It also
reports the blocks the calls leave empty,
which a later phase can fold once each condition is shown to have no side
effect.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3g) the phase runs on the graph: `crefactor/graph`'s `Editor.DropCalls`, with vim's knobs in `internal/whim/xform.go`, finds the call statements by their form and the host's calls by edge (pointed at `host_free` by `RetargetAs`), takes the locals left only given values with `Editor.DeadLocals` (the text's `edit.DeadStores`, held to it by `PureCond`), and reports what the text version reported; `whim-build-check` holds q060 to the bytes `crefactor/xform`'s `DropCalls` made (which is in history, `16717ab` and before); its test, moved with it, is `crefactor/graph/dropcalls_test.go`.
