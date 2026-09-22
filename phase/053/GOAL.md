# Phase 53 — no conversion layer, no 'encoding'

Phase 12 cut the conversion layer at its entry points and left its body. Two ways
in were still open: **`++enc`** on `:e`, `:r` and `:w`, and a buffer whose
`'buftype'` is `help`, which `readfile()` read as latin1-or-utf-8. `tools/noconv.py`
closes both, and then everything behind them has one answer: the encoding name is
always empty, `need_conversion("")` is false, and so `converted`, the conversion
flags, the iconv descriptor, the `'charconvert'` temporary file and the retry with
the next encoding never change. Every test of them folds, in `readfile()` and
`buf_write()`; `buf_write_bytes()` loses the UCS-2, UTF-16, UCS-4 and latin1
writers no flag reached; the byte-order-mark check goes, since `check_for_bom()`
has answered "none" since Phase 12; the rewind that retried another encoding goes,
with its `retry` and `failed` labels.

**`'encoding'` goes**, and `mb_init()` stops asking `p_enc` — **but its NULL
branch was taken, once.** `common_init_1()` calls `mb_init()` before any option
exists, and that call filled the byte-length table with 1s and returned;
`set_init_1()` made the real one. Folded as never-taken, the first call ran on into
`init_chartab()` with no `curbuf`, and the editor crashed before its first command.
So `common_init_1()` now does what its call did then. **`'makeencoding'` goes with it** —
it converted `:make` output, `:make` went long ago, and it shared
`did_set_encoding()`, which is why that function survived the first attempt.

**The terminal is not converted either, and this is not tidying.** `input_conv`
and `output_conv` were `CONV_NONE` whenever `'encoding'` was utf-8. The only
assignment of `input_conv.vc_factor` was in the `mb_init()` branch folded above,
and `fill_input_buf()` divides by it: a first version of this phase folded the one
and not the other, and would have built an editor that divided by zero on its
first read of input. The post-condition grep caught the survivor before the build
did. Their tests fold in `ui_write()`, `fill_input_buf()` and `utf_find_illegal()`.

**The ten `mb_*` function pointers are calls.** `mb_init()` pointed all ten at the
UTF-8 implementations every time; 390 calls through them become direct calls to
`utfc_ptr2len()`, `utf_ptr2char()` and the rest, and the latin1 implementations
they were initialised to are swept. `mb_tail_off()` kept two dead returns after
its last live one from Phase 52; they go, and `dbcs_head_off()` with them.

Completion for `++ff`, `++enc` and `++bad`, left behind by Phases 50, 51 and this
one, goes from `expand_argopt()` and `get_argopt_name()`.

**The sweep met a declaration shape it had never deleted.** `enc_canon_table[]`
and `enc_alias_table[]` are written `static struct`, then the whole body on one
line, then the declarator alone — and gcc reports the declarator's line.
`deadsweep.py` walked back over a type only when that line began with `}`, so it
took the table and left `static struct {...}` open at file scope, where the next
declaration became "duplicate 'static'". It now recognises the one-line body too.
The branch is new and the old one untouched, so no earlier boundary could move —
and `slim-verify` and `whim-specpass` were run to show it, since the tool is in
every phase's implementation digest.

Both this and the crash above were found the expensive way: the phase program
failed, the pass fell through to an agent, and the agent's account named the two
causes. Its boundary and its synthesised residue were discarded; the fixes are in
the programs.

The phase checks that `:set enc?`, `:set menc?` and `++enc` are refused, that `gUU`
over *à é* still gives `c3 80 c3 89 0a`, and that an invalid byte is still written
back unchanged.

## The delta

**None the harnesses record** — no case converts. Measured: 112,439 →
**110,672 lines**, libc symbols 82 → 81 (`lseek`, whose two callers were the
retry's rewind and the help buffer's look at a file's first line — the second
already unreachable, behind a `c = TRUE` its own test could never pass).
