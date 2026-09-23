# Phase 84 — the stack protector goes

`phase/084/check.go`, one whole program: there is no source edit, so there is nothing
for a sweep to do and a split phase would pay for one. `whim-vim.c` comes out of it
byte for byte as it went in, and what changes is one line of `zero/Makefile`:

```make
CFLAGS  = -O0                       ->  CFLAGS  = -O0 -fno-stack-protector
```

**Why.** gcc 15 on this machine enables `-fstack-protector-strong` by default, so
every function with a local array or an address-taken local gets a canary and the
object calls `__stack_chk_fail`. That is a symbol the core would have to be given by
its host, for a check the editor does not ask for — and *what it must be given* is
the number `GOALS.md` measures. Measured on this input: the undefined symbols of
`gcc -c` on `whim-vim.c` go from **80 to 79**, the one that goes is
`__stack_chk_fail` and nothing comes, and the binary goes from **894,088 to 869,512
bytes**.

**Where the flag lives.** In the boundary — the tree a phase transforms — and not in
`tools/templates/core.mk`, which is the pipeline's *input*: the input rule copies it
into `zero/Makefile`, and editing it would move q83's input digest and invalidate
phase 83's recording. The product rule in `whim.mk` cannot read `zero/`, which does
not exist in a checkout that only builds the committed `whim-vim.c`, so it states the
same flags once as `ZEROCFLAGS` and `ZEROLDFLAGS` — and `whim-pass` refuses to copy
`whim-vim.c` out when they differ from the `CFLAGS` and `LDFLAGS` of the makefile the
last phase left. The two statements cannot drift without a pass saying so; proven by
running `make whim-pass ZEROCFLAGS=-O0`, which refuses and names both. `make score`
passes both variables to `tools/score.sh`, which applies them to the object it counts
symbols in as well as to the binary — the count is otherwise taken with the default
CFLAGS, and would still show `__stack_chk_fail`.

**What the phase proves, in order:** the makefile has exactly one `CFLAGS` line and
it does not already carry the flag; the **old** flags do reference
`__stack_chk_fail` and the new ones do not — both measured with `nm -u` on unstripped
objects of the same source, so the check is one that can fail, and a compiler whose
default changed is reported rather than silently passing; `whim-vim.c` is unchanged;
the binary is still absolutely static (`EXEC`, no `INTERP`, no dynamic section, no
relocation); and `tools/coredelta.sh --phase 84` sees no behaviour case, no Ex command
and no terminal-table row move against whim-vim's baselines. `phase/084/delta.md`
declares nothing for it, because a canary is code around the locals and not
behaviour.

It is `stage 84` and `package build` in `phase/STAGES.md`, with one `uses`:
`build:84 seed:83 mechanical`, because `coredelta.sh` refuses without the
`.reference/core-baselines` phase 83 records. It runs in 9 seconds.
