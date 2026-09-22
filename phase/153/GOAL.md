# Phase 153 — `free_one_termoption()` compares without a cast

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
`tx/FINDINGS.md` and not this phase's to fix. The Go transpilation already
wrote it this way, because Go cannot compare the two types.

**Declared delta: nothing.** The check proves the premise from the input:
every row's variable is NULL or the address of a variable, the one caller
passes `term_strings[KS_CCO]`, and nothing takes a slot's address as a
string. Its probes reach `ttest()` through the terminal colour options, and
each control moves.
