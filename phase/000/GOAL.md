# Phase 0 — seed, in the one spelling every later phase reads

`whim-vim.c` starts as `slim-vim.c` printed canonically: `internal/cemit` parses
the input and prints it back in one C23 form per construct — one statement per
line, one declarator per declaration, braces always, a table one element per
line with the brace under the `=`, one space between a type and its declarator.
It changes no token and no declaration, only where the whitespace falls, and it
is a fixpoint, so the sweep can run it after every round and get the same bytes
back.

**Why the seed and not each phase.** The input is macro-expansion residue, and
the spacing in it is no one's choice: `{'Z', nv_Zet,  (0x02|NV_NCH) |NV_NCW, 0}
,` is what `slim-vim.c` really says. Every anchor downstream had to match that
rather than what it meant. Seeding the canonical form once makes an anchor a
statement about the construct, and the 163 edits and the checks are written
against it. This was a `--canonical` flag while they were migrated; it is the
pipeline now, and there is no second spelling to fall back to.

`internal/build`'s `Seed` is the whole of it, and `internal/verify` calls the
same function — a verification that read the file instead would hand every
phase after this one a spelling the pipeline does not produce.

**The evidence is the binary.** There is no `-g`, so whitespace cannot reach the
object: `slim-vim.c` and the seed, both built `gcc -O0 -static -s` with
`SOURCE_DATE_EPOCH=0`, give the same 2,204,344 bytes and the same digest. That
is what `cmp` used to say and says more — and it is why
`.reference/baselines`, recorded from `slim-vim.c`'s own binary, is the seed's
baseline too. The phase declares no delta, and `internal/cemit` refuses a node
it cannot print rather than dropping it, so a construct it does not understand
stops the pipeline instead of quietly leaving the file.

Measured: 180,870 lines in, 174,048 out.
