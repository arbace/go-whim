# Phase 154 — the NULL write in `free_one_termoption()` is gone

`ttest()` called `free_one_termoption(t_Co)` when the terminal had neither
`t_Sb` nor `t_AB`, meaning to clear `'t_Co'`. But it passed `t_Co`'s string
value, and the function looks for the option whose variable **address** is its
argument. Phase 153 showed the two are equal only when both are NULL, and then
the function wrote `empty_option` through the NULL variable of the first option
that has none. So the call never cleared `'t_Co'`; the one thing it ever did
was that write, a crash waiting for a NULL `t_Co`.

The call goes, with the `if` around it, whose condition only reads the two
strings the lines above it already read. The sweep takes
`free_one_termoption()`, which nothing else calls. What the editor does is
unchanged, apart from the crash that can no longer happen.

Doing what vim meant, passing `t_Co`'s address so it really is cleared on a
terminal with no colour-setting codes, would change behaviour, and would need
a declared delta. It is not this phase.

**Declared delta: nothing.** The check proves from the input that the call's
one effect was the NULL write (the function matches only both-NULL, then
writes through the match). It also proves the `if` reads nothing new, and
requires the call and the function gone. Its probes are the paths into
`ttest()`: clearing the colour options, setting `t_Co`, and setting the
terminal. Each control moves.
