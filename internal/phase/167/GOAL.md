# Phase 167 — a key code has a name

vim spells a special key as `TERMCAP2KEY(a, b)`, which is `-(a + (b << 8))`,
and names each one: `K_UP`, `K_DEL`, `K_IGNORE`. The names were macros, so
they went when slim-vim.c's pipeline removed the preprocessor, leaving the
arithmetic at every use: `case (-(('k') + ((int)('D') << 8))):`. This phase
gives every constant key code a name again: one enumerator per code, defined
before the first use, and written wherever the code was spelled out.

**The name is vim's where the file still says it,** and never a remembered
one:
- `TERMCAP2KEY(KS_EXTRA, KE_X)` is `K_X`, which is vim's own rule;
- a code `key_names_table` lists is `K_` plus the shortest name the table
  gives it (`BS`, not `BackSpace`; `DEL`, not `Delete`);
- a code named nowhere in the file gets a mechanical name from its two
  characters, `K_TC_<a>_<b>` (`K_TC_HASH_2` for `'#', '2'`).
A code computed from variables at run time, such as `TERMCAP2KEY(p[1], p[2])`,
is not a constant, and stays as it is.

**The proof is the binary.** A name changes no value, so the product built
with the one compile line is BYTE-IDENTICAL before and after
(`SOURCE_DATE_EPOCH=0`).

It is the survey's item 6 (the Go-idioms survey). The generator writes an
enumerator as a Go constant, so the Go says `case K_INS, K_KINS:` where it said
`case -('k' + (73 << 8)), -(KS_EXTRA + (79 << 8)):`.

**Measured:** 153 codes named at 683 sites -- 72 from `key_names_table`, 75
as `K_X`, 6 mechanically. The Go spells a key code out 24 times, where it
did 707: the definitions and the run-time forms. vet, staticcheck and
`gofmt -s` stay at 0. `whim-test`: 45/45, C and Go.
