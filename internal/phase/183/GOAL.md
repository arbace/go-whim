# Phase 183 — a file-scope flag is bool

Phase 166 declared `bool` the core's functions whose every return answers
yes or no, and the locals, members and parameters that only ever hold an
answer -- and not the file-scope objects. vim keeps its state in them, and
dozens are flags: `VIsual_active`, `msg_scroll`, `redraw_cmdline`,
`exiting`, `static int x = TRUE;`. Each editor then said a flag as a
number: `VIsual_active != 0` in the Java, `(not (zero? ...))` in the
Clojure (`doc/JAVA-IDIOMS.md`, item 7).

This phase is phase 166's rule on them: `crefactor/xform`'s `BoolRet`,
with vim's knobs, run with `Globals`. A file-scope object is `bool` when:
- every declaration of it is `int` alone in one declarator, not a pointer
  or an array, and not braced;
- its initializer and every value assigned to it is an answer (a truth
  constant, a comparison, a logical operator, another answer), in the
  same fixed point as the functions, members, parameters and locals; an
  initializer of a literal 0 or 1 counts for nothing either way;
- nothing takes its address, in a function or in another initializer
  (the option table's `&p_wiv` keeps `p_wiv` an int);
- nothing increments it, updates it in place, compares it with a code,
  takes its size or its type;
- no function has a local or parameter of its name (the facts go by
  name); and the host never mentions it.
A bool converts to exactly the 0 or 1 an int held wherever an int is
wanted, so no value changes.

Measured, on q182: 79 file-scope objects, and with them 1 function, 7
locals, 4 members and 3 parameters that became answers once the globals
were. 97 declarations retyped; 77,634 lines, as before. In the Java:
`VIsual_active != 0` and `== 0` 115 -> 0, `TRUE`/`FALSE` named 1,195 ->
875, `x != 0)` 618 -> 448, `boolean` fields 301 -> 381. Verified by the phase's own link
(`whim-build-check`), and `whim test` against the commit before it on the
C, the Go, and with `--java --clojure --haskell`, quick and wide. gcc
reports the same diagnostics before and after.
