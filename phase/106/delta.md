106 DECLARES NOTHING AT ALL, and it is the strongest kind of nothing this pipeline has:
THE BINARY IS BYTE-IDENTICAL.  `NULL` becomes `nullptr`, a C23 keyword that needs no
declaration; `size_t` becomes `usize`, with one new line, `typedef typeof(sizeof(0))
usize;`; and the thirty `(void *)NULL` become plain `nullptr`, the cast having existed
for the variadic hazard of an UNTYPED null constant and `nullptr` being typed.  Three
thousand mechanical edits, +2 lines, and `cmp` of the input's binary against the
output's -- both built with SOURCE_DATE_EPOCH=0 and the boundary's own flags -- finds
no difference at all: 788,488 bytes either side.

SO THE DELTA CANNOT BE THE EVIDENCE AND IS NOT ASKED TO BE.  Tier 1 of CLAUDE.md's
verification table subsumes every screen case, every Ex-command row, every command
line and every pty scenario at once, because the program that would be run is the same
program; tools/zerodelta.sh --phase 106 corroborates.

WHAT COULD HAVE MOVED IS DATA, AND THAT IS THE ONE THING THIS PHASE HAD TO GET RIGHT.
Three string literals in this file contain `NULL` -- the E1507 internal-error message,
`"[NULL]"` and `"NULL"` -- and a line-wise sed rewrites all three.  Measured: the
binary then differs by 1,598 bytes, 1,354 of them in `.rodata`, and `strings` finds
`[nullptr]` and an E1507 message that names a C keyword at the user.  With the three
excluded the binary is `cmp`-identical.  phase/106/check.sh builds the
literal-unaware form as a control and requires it to differ, which is what keeps the
equality above from being two numbers agreeing.
