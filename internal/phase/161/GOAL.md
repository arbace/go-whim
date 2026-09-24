# Phase 161 — no goto jumps into a block

`ml_get_buf()` answers a line it cannot give with `"???"`. That answer was the
tail of its first `if` block, labelled `errorret`, and a `goto` further down
jumped back into it when the memline could not find a line. Go's `goto` may
not jump into a block. Of the core's 182 gotos, this was the only one that did
(`internal/ccx`'s `Gotos`). Every other goto is legal Go once a function's
locals are declared at its top.

The tail becomes `ml_get_invalid()`, with the static buffer it returns, and
both paths return what it returns.

**Declared delta: nothing.** The check requires `Gotos` to leave exactly that
goto on the input and nothing on the output. It requires the new function to
be the old tail statement for statement. The error path is vim's internal
error for a line the memline cannot give, and no key reaches it, so no probe
can; the textual identity is the evidence for it. The probe draws, joins,
moves through and deletes lines, and the control moves.
