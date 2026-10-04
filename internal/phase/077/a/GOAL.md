# Phase 77a — `free_one_termoption()` compares without a cast

*Formerly phase 153, now part a of phase 77. The other phase numbers in this file are the old
numbering, as it was written: `doc/PHASES.md` maps them.*

`free_one_termoption(var)` looks for the option whose variable is `var`, as it
does upstream. It compared each row's variable **address**, cast to
`char_u *`, with the string **value** its one caller hands it:
`term_strings[KS_CCO]`. An option's non-NULL `ov_str` is the address of a
`char_u *` variable (a `p_*` global or a `term_strings` slot), and no code
stores such an address as a string. So the two are equal exactly when both are
NULL: a row with no variable, and a NULL terminal string. In that case the C
then writes through the NULL.

The comparison now says exactly that, `p->var.ov_str == nullptr && var ==
nullptr`, with no pointer cast to another type. What it does is unchanged,
the latent NULL write included. That is vim's bug, recorded in
`internal/gen/FINDINGS.md` and not this phase's to fix. The Go transpilation already
wrote it this way, because Go cannot compare the two types.

**Declared delta: nothing.** The check proves the premise from the input:
every row's variable is NULL or the address of a variable, the one caller
passes `term_strings[KS_CCO]`, and nothing takes a slot's address as a
string. Its probes reach `ttest()` through the terminal colour options, and
each control moves.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 154, as the first of the group 153-154, which was one idea split for history's sake. There is no boundary q153 of its own any more; everything above still says what the steps do and why.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the comparison is found by its form and written anew by FRAG, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
