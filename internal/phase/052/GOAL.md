# Phase 52 — UTF-8 is not a question

Since Phase 12, `mb_init()` sets the same five globals to the same values every
time: `enc_utf8`, `has_mbyte` and `enc_latin1like` TRUE, `enc_dbcs` and
`enc_unicode` 0. **456 places still asked them**, in every shape C allows — a bare
`if`, a chain of `&&` and `||`, a ternary, a comparison with a DBCS code page, an
argument — and each one was a branch for an encoding this editor cannot have.

`tools/utf8only.py` folds them as constants, on the source, and **never drops a
side effect**:

1. The five lose their declarations and their assignments in `mb_init()`, and every
   other mention becomes a marker — `__T__` for the three that are TRUE, `__Z__`
   for the two that are 0. A marker is an identifier, so the text still parses,
   and a `TRUE` already in the source is never mistaken for one the tool made.
2. Every expression holding a marker is simplified, innermost first, to a
   fixpoint: a ternary on a constant condition becomes its branch; in an `||` list
   a false operand goes and a true one ends the list, in an `&&` list the reverse;
   `!` flips a constant; parentheses around one collapse; `__Z__ == DBCS_x` is
   false. **An operand is dropped only where C would not have evaluated it, or
   where it is pure** — no call, no assignment, no `++` or `--`. Otherwise it stays.
3. Every `if`, `else if` and `while` on a constant marker folds with its else chain,
   by brace matching — from the last occurrence in a function, because the same
   false condition can be nested inside its own block, and folding the outer one
   first makes the inner vanish. A first test run met exactly that.
4. What is left — a marker compared with something that is not a constant, or
   assigned — becomes `TRUE`, `FALSE` or `0` again.

Measured on the phase's input: 239 expression simplifications and 261 statement
folds in 178 functions, 7 constants left as values. The sweep then takes the DBCS
and latin1 paths nothing reaches, and with them two libc symbols, `iswupper` and
`mblen`.

**Tested before it became a phase, against the binary it replaces.** Applied to a
copy of the Phase 49 source, compiled with every warning the sweep does not own
silenced, built, and run through the behaviour and Ex-sweep harnesses beside the
unfolded binary: 0 of 67 cases and 0 of 600 rows differ. That test also caught the
tool's own mistakes twice before it counted — once by crashing, and once by
reporting "0 differ" for a file the crash had left unchanged, which is why the
test now refuses to compare unless the tool succeeded and the file moved.

The phase checks `gUU` over *à é* for `c3 80 c3 89 0a`, byte for byte, and `x` on a
three-byte character.

## The delta

**None.** Folding a constant changes no behaviour, and the harnesses — with their
multibyte motion, case and insertion cases — are the check. Measured: 114,275 →
**112,439 lines**, libc symbols 84 → 82.
