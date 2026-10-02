# Phase 103 — more flags are bool

*Formerly phase 184. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

Phase 183 made `bool` the file-scope flags that only ever hold an answer,
by phase 166's rule. A survey of what the Java still wrote `= TRUE` and `=
FALSE` (`doc/JAVA-IDIOMS.md`, item 7) found three things that rule kept
int, and this phase takes them -- `crefactor/xform`'s `BoolRet`, with vim's
knobs, `Globals`, and `Relax`:
- **the greatest fixed point.** The rule grew its answers from none, so
  a flag saved in a local and restored from it (`int save = x; ... x =
  save;`) waited on the local, and the local on the flag, and neither was
  ever shown one. From all, dropping what is shown not, the cycle holds:
  nothing but answers ever reaches it.
- **a literal 0 or 1 assigned** to a file-scope object, `need_wait_return
  = 0;`, is no evidence against it (an initializer never was).
- **`x |= E` or `x &= E`** of an answer E into one is an assignment of an
  answer, and is written `x = (E) || x` (`&&`) where x becomes bool: E is
  evaluated as before, and reading x does nothing -- so that no editor's
  printer meets a compound assignment of a bool.

A bool converts to exactly the 0 or 1 an int held, so no value changes.

Measured, on q183: 64 declarations retyped -- 11 file-scope objects
(`need_wait_return`, `did_cursorhold` among them), 3 functions, 16 locals,
11 members and 14 parameters that are answers with them; no `|=` rewrite
applied (the flags that have one are kept by something else). 77,634
lines, as before. In the Java: `TRUE`/`FALSE` 871 -> 779, `x != 0)` 446 ->
422, `boolean` fields and methods 381 -> 395.
What it does not take, and why: `msg_scroll`, `redraw_cmdline` and
`msg_didout` are saved in a local that its function also uses for other
values (`n`, `i`), and the facts go by name -- a flow-sensitive analysis of
the locals would be the next step; `got_int` the host's signal handler
sets, and the host names it.

Verified by the phase's own link (`whim-build-check`), and `whim test`
against the commit before it on the C, the Go, and with `--java --clojure
--haskell`, quick and wide; gcc reports the same diagnostics before and
after; `TestBoolRetRelax` (a program under gcc before and after).
