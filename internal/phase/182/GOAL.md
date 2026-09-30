# Phase 182 — gettext's identity not called, the ASCII tests named, constant ifs their branch

Three things the C spells that no one would write
(`doc/JAVA-IDIOMS.md`, item 8), each a step of `crefactor/xform`
(`plainc.go`) that knows nothing of vim, run on the core.

**`Identity`**: a function whose body returns its one parameter, cast or
not, is not called; each call is its argument, with the body's cast where
the argument's type is not the result's already. vim's `_()` was gettext;
since phase 0 it has been `return (char *)x;`, called around every message:
`emsg(_(e_invalid_argument))` is `emsg(e_invalid_argument)`, and a
`const char *msg` is `(char *)msg`. 399 calls. The function stays: the host
calls it (Printf's messages, `gettext_` in the Go and the Java).

**`AsciiClass`**: `(unsigned)c - 'A' < 26`, C's ASCII class test in one
comparison -- vim's `ASCII_ISUPPER` and kin, expanded by the preprocessor
-- is a call of a function named for it again, defined at the core's top as
the test was written: `ascii_isupper`, `ascii_islower`, `ascii_isdigit`.
136 tests (67, 37 and 32). A language with no unsigned int said each as
`Integer.compareUnsigned(c - 'A', 26) < 0` (the Java), `(< (u32 (- c 65))
26)` (the Clojure). The argument is evaluated once either way, and an int
parameter's conversion and the cast's give the same bits.

**`ConstBranch`**: an `if` whose condition is a constant is the branch it
takes, when neither branch holds a label: `if (0) A else B` is `B`, `else if
(!TRUE) A else B` is `else B`, `if (1) A` is `A`. 8 of them, the survey's two
(`win_update`, `ex_z`) among them.

Measured: 77,631 -> 77,634 lines (the three functions' definitions); the
Java's `gettext_(` 400 -> 1 and `Integer.compareUnsigned` 150 -> 17, the
Clojure's `(< (u32 (- ...` 6, the Go's 139 calls of the three.

Taking `if (1) { len = 0; }` showed what the constant had hidden from
staticcheck: three functions (`vim_str2nr`, `get_buffcont`,
`match_with_backref`) overwrite a parameter before reading it -- the C
ignores the caller's value. The Go generator writes such a parameter `_`
and the name a local of its own (`deadInParams`, `crefactor/togo`), so
`editor/` stays clean under go vet, staticcheck and `gofmt -s`. And the
suite's control, `_(" INSERT")` in the C, is `" INSERT"`, which both
spellings hold once. What
each editor gains is in `doc/JAVA-IDIOMS.md` item 8. Verified by the phase's
own link (`whim-build-check`), the committed product back byte for byte,
and `whim test` against the commit before it, on the C, the Go, and with
`--java --clojure --haskell`, quick and wide: nothing the editor does
moves.
