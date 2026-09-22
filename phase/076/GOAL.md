# Phase 76 — one regexp engine, so no retry

**Proved by a single assignment.** `prog->re_engine = BACKTRACKING_ENGINE` is the only
place `re_engine` is ever written, so the field can hold no other value — and both

```c
if (rmp->regprog->re_engine == AUTOMATIC_ENGINE && result == -1)
```

blocks, one in `vim_regexec_string` and one in `vim_regexec_multi`, are unreachable.
They exist to recompile a pattern with the backtracking engine when the automatic
choice failed; with one engine there is nothing to fall back to. `nfa_regengine` and
`regexp_engine` were already at zero — the NFA engine went in an earlier phase, and
these two blocks were what remained pointing at its corpse.

**What went with them.** `p_re` entirely: it is an **orphan option** — no row in the
table sets it, so it reads as 0 for ever — and its only uses were a `< 0 || > 2`
validation that could never fire and the save/restore inside the two dead blocks.
`AUTOMATIC_ENGINE`, which had no other reader. And `nfa_regprog_T` with `nfa_state_T`
by cascade: their only non-type mentions were the two
`((nfa_regprog_T *)rmp->regprog)->pattern` casts **inside** the dead blocks — a husk
kept alive purely by unreachable code. The sweep deleted three type definitions.

`orphanopts` independently confirms the claim: its count fell from six orphans to
five, with `p_re` gone from the list.

## The guard was proved against both failure modes

The phase asserts in-flight that `re_engine` has exactly one assignment and that it
is to `BACKTRACKING_ENGINE`. Before relying on it, it was checked three ways: it
reports one on the real file, it **fires** when a second write is injected, and it
does **not** miscount a `!=` comparison as a write — which is exactly the cry-wolf
bug that cost an iteration in phase 75, where a guard matched `name[^\n;]*=`, spanned
the subscript and landed on the comparison.

## Audited before writing, not after

Both blocks are 25 lines, carry no `break` or `continue` that would rebind, contain
no label, and are followed by no `else` — so `fold_never` takes them without any of
the hazards phases 71, 72 and 75 each ran into. No edit's target is created by an
earlier edit either, so the specific-then-blanket ordering problem does not arise.
**It passed its first dry run.**

## The delta

**None**, and `whimdelta.sh` confirms it. The blocks never ran, so removing them
cannot change a match.

The probes exercise **matching**, not editing, because a load-and-edit probe would
pass whatever happened to the regexp layer: a quantified `%s/a\+/X/g`, `:g` over a
pattern driving `vim_regexec_multi`, capture groups with back-references, a counted
non-capturing group `\%(a\|b\)\{2}` — the shape the NFA engine used to be chosen for
— and a plain search. All five were calibrated against q75 first.

Measured: 89,804 → **89,713 lines**.
