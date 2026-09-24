# Phase 164 — no statement follows a jump

A statement after a `return`, `goto`, `break` or `continue` in the same block
is reached by no path, up to the next label or `case`, where one can come in
again. The same holds after an `if` whose two branches both jump, or after a
block that ends in one. vet found 17 such runs in the Go (*unreachable code*),
and all 17 are in the C too: `del_char()` returned `del_chars()` and then
`del_bytes()`; `vim_islower()`, `vim_isupper()`, `vim_toupper()` and
`vim_tolower()` return before a Latin-1 branch that nothing enters any more.

The rule is general and uses the tree only as a locator. `cc.Parse` finds the
runs, and the text is what gets cut. A run inside a dead block goes with the
block, and only the outermost cut is made. A run holding a declaration would
be left alone, since a label after it can be reached with that name in scope;
none does. The sweep then takes what only the dead runs used: three objects
and two tags, the Latin-1 case tables among them.

**Measured:** on the product before it, 18 runs go and 0 are held. `go vet
./editor` goes from 17 findings to 0. `whim-test`: 45/45 as the commit before,
and the Go editor answers all 45 as the C does.
