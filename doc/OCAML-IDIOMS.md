# How whiml could be more idiomatic OCaml: a survey

2026-10-04. It covers `whiml/editor.ml` as milestone 1 left it (`c2a99dc`:
59,496 lines, all 1,713 functions of the core, written by
`crefactor/togo`'s OCaml backend from the Scheme printer's forms,
`doc/OCAML.md`), and the hand-written runtime `rt.ml`, host `host.ml`,
terminal `term.ml` and printf `snprintf.ml`. `doc/SCHEME-IDIOMS.md` and
`doc/RUST-IDIOMS.md` are its model: every item says what the pattern is,
how many sites it has, what it becomes, who does it -- the OCaml printer
(`crefactor/togo/ml.go`, `ml_expr.go`), the runtime -- and how it is
held. `editor.ml` is generated and stays so: every item done is a change
to the backend, the module regenerated. Items 1-6 were done the same day
(*Done*, below); the rest are declined, with why.

## The tension, stated first

whiml is C in OCaml's syntax: every C object in one `Bytes` laid out as
amd64 lays it out, a pointer an `int` offset, every function a function
of the editor, control flow as local functions and tail calls -- chosen
because it is the least translation (the backend prints the Scheme's
forms, `doc/OCAML.md` §2), and it answered both suites the first time
its memory's end was right. What an OCaml programmer objects to falls in
two kinds. **The spelling**, which costs nothing at run time and is the
printer's to change: `not (x = 0)`, `p + -1`, parentheses OCaml does not
need, `if` ladders where OCaml has `match`, characters compared as their
codes. **The model**: records, variants, options, exceptions, strings
instead of bytes at offsets. The C walks its objects by pointer
arithmetic, puns pointers, copies structs by bytes and allocates from an
arena it never frees, so the model can move only where an analysis proves
the C does none of that to an object: item 3 is the one place it does.
Every item is held to the heavy case's time beside the suites.

## How it was measured

Counted or run, not estimated; the instruments were throwaway, in
`.tmp/idioms/` (not tracked):

| Instrument | What it gave |
| --- | --- |
| `ocamlopt -w +a` on `editor.ml` | the warnings, by number |
| `measure.py` (reads `editor.ml`) | the patterns by regular expression: negations, literals, parentheses, loads and stores, accessors, ladders, matches |
| `objs.py` | the file-scope scalars: their reads and writes, and which have their address taken by a function |
| `ladders.py` (reads `whimsical/whimsical/editor.ss`, the same forms) | the `cond` and `if` chains that test one value against constants |
| `perf record` on the heavy case | where the time goes |
| `whim test --ocaml`, `--wide --ocaml` | the verification recipe |

**Verification recipe for every item done**: `crefactor/togo`'s `TestMl*`
(25 foreign C programs, compiled by `ocamlopt` and required to print what
gcc's builds print) and `TestMlControl`; `go tool whim test --ocaml` (all
80 as the C, the control seen by 76) and `--wide --ocaml` (all 240, the
control seen by 94 keys and 6 pty cases), and the heavy case's time
beside the C's; `make whim-editor-check` on the regenerated module.

## The shape of the output at milestone 1

| | count |
| --- | ---: |
| lines | 59,496 |
| warnings under `-w +a` | 1,759 in the first printing (1,343 `let rec` that does not recurse, 398 unused parameters, 17 unused lets, the missing interface); 1 at `c2a99dc`, the interface (a `[@@@warning "-a"]` silenced the rest) |
| `not (...)` | 3,027, 1,802 of them `not (a = b)` |
| `p + -k` / `0 - x` | 116 / 23 |
| file-scope scalars: reads `x ed` / writes `set_x ed v` | 7,222 / 1,514, of 456 objects, 21 with their address taken by a function |
| `if` ladders testing one value against constants | 80 (176 tests), counted on the Scheme's forms |
| `match` | 68 (the C's switches), its labels numbers: `97 (* 'a' *)` |
| bytes compared with a character's code (`ld_u8 ed p = Char.code 'a'`) | 602 |
| parenthesized `(let` | 2,612 |
| `ref` / `:=` / `Obj.magic` | 0 / 0 / 0 |
| the heavy case | 0.6-0.7 times the C |

## The findings, ranked

1. **Every warning, and an interface** -- S, no risk: the printer writes
   only what is used and the module is compiled `-w +a`, a warning a
   failed build.
2. **Comparisons turned round** -- S: `x <> 0`, `a >= b`, `p - 1`, `-x`.
3. **The file-scope scalars as the editor's fields** -- L: `ed.st.got_int
   <- 0` where nothing takes the object's address.
4. **Ladders as `match`, on characters where they are bytes** -- M.
5. **The parentheses OCaml does not need** -- S.
6. **A byte compared with a character as a char** -- S.

## Done

Measured on 2026-10-04 at a load of 1-6, each with the recipe: all 80 and
all 240 cases as the C, the controls seen as before, `make
whim-editor-check`.

### 1. Every warning, and an interface

The first printing had 1,759 warnings under `-w +a`; milestone 1 took the
`let rec` (`ml_expr.go`'s `mlLocalGroups`: a body's local functions in
the order of their calls, `let rec ... and` only for a cycle) and the
unused names (`_name`, set where the binding's scope is printed), which
left the interface. Now the backend writes `editor.mli` too -- the
host's record, the editor's type with its state abstract, `new_editor`
and what the profile exports, typed from the C (`val vim_main : ed -> int
-> int -> int`) -- and the module defines only the accessors, constants
and host functions its functions use (1,646 lines fewer). `whiml.go`
compiles the generated module with `-w +a`, and a warning fails the
build, as `TestMl*` do on every foreign C program: **0 warnings**.
The hand-written modules keep OCaml's default set (they compile silently
under it; `+a` would add record-field disambiguation notes, 40 and 42,
in the host's records).

### 2. Comparisons turned round

A test's negation is its own operator: the printer builds each
comparison with its negation (`mlCmp`; the unsigned longs' `u64_lt` with
`u64_ge`), so `not (x = 0)` is `x <> 0`, `not (a < b)` `a >= b`,
`not (not c)` `c`; `(if c #t #f)` is `c`; `p + -1` is `p - 1` and `0 - x`
is `-x`. `not (` 3,027 -> 369 (what is left negates an `&&` or `||`, or
a call), `not (a = b)` 1,802 -> 52, `p + -k` 116 -> 0.

### 3. The file-scope scalars as the editor's fields

A C object at file scope was a function of the editor reading its bytes,
`got_int ed`, and one writing them, `set_got_int ed v`. Now a scalar
object **whose address nothing takes** -- no function's `&x` (read off
the Scheme's forms), no initial value's pointer (the Scheme printer
records them in OCaml's mode), and not one the host reads (the exports)
-- **and that some function reads** is a field of the editor's state, a
record the generated module declares (`type state = { mutable got_int :
int; mutable msg_scroll : bool; ... }`, a field `mutable` only where it
is written) and carries in the runtime's editor (`('g, 's) Rt.ed`, its
`st`): `ed.st.curwin`, `ed.st.state land mode_insert <> 0`,
`ed.st.dollar_vcol <- -1`. Its initial value is read from the image the
C's initializer wrote. The domains of the parallel `:%s` share it, as
they share the memory.

376 objects are fields, 7,702 uses (1,376 writes); 107 scalar objects
stay in memory with their readers: the option table's (an initial value
holds their address), the 21 whose address a function takes, the
exported, and three only ever written (a field never read is a warning).
The first build failed every case, and found a fault of the item's own:
the initial values were read by the Scheme's kinds (`i32`) where the
objects have the accessors' (`s32`), so every one was 0 -- bisected to
`dollar_vcol`'s -1 by building with the first n objects as fields.

### 4. Ladders as `match`, on characters where they are bytes

A `cond` -- or an `if` whose else is an `if` -- whose first clauses test
one value for equality with constants, `(fx=? x K)` or an `or` of them,
is a `match` on the value (`ladder`), the clauses after them its last
arm; the value is evaluated once where the tests evaluated it in each,
so it must read and call nothing (`mlPure`), and a label an earlier arm
takes is dropped (OCaml would warn of the unused case). A match whose
value is a byte read and whose labels are all characters matches on the
char: `match ld_char ed p with '\\' -> ... | '\'' -> ...`, where it was
`| 92 (* '\\' *)`. 105 ladders are matches (`match` 68 -> 173, `else
if` 1,341 -> 1,168), 30 matches on a char. A label that is a C
constant's name stays a number with its name beside it: an OCaml pattern
cannot name a constant.

### 5. The parentheses OCaml does not need

A sequence's last expression, and an else, may be a `let` or a `match`:
nothing follows them that they would take. And an `if` with no else
takes only an else. So `e1; let x = v in e2` and `if c then a; b` are
printed bare (`mlParen`, `seq`, `mlIf`): `(let` 2,612 -> 1,108, and the
`(if ... then ...);` statements are `if ... then ...;`.

### 6. A byte compared with a character as a char

`(fx=? (ld-u8 p) (ch #\a))` was `ld_u8 ed p = Char.code 'a'`; it is
`ld_char ed p = 'a'` (`charCmp`, and the runtime's `ld_char`, the byte as
a char), for `=`, `<>` and the orders: 697 comparisons. `Char.code` 1,628
-> 1,026 (what is left compares an int that is not a byte, a key code).

### Before and after

| | milestone 1 (`c2a99dc`) | after items 1-6 |
| --- | ---: | ---: |
| `editor.ml` | 59,496 lines | 57,584 lines, and `editor.mli`, 42 |
| warnings under `-w +a` | 1 (1,759 in the first printing) | 0, enforced |
| `not (...)` | 3,027 | 369 |
| file-scope scalars | 456 by accessor | 376 fields, 107 in memory |
| `match` / `else if` | 68 / 1,341 | 173 / 1,168 |
| `(let` | 2,612 | 1,108 |
| `Char.code` | 1,628 | 1,026 |
| `ocamlopt` on the core | 9.6-10.2 s, 0.54-0.56 GB | 9.4-9.6 s, 0.53 GB |
| the heavy case | 0.6-0.7 times the C (251-306 ms) | 0.5-0.6 (242-259 ms) |

## Declined, with why

- **Records for the C's structs.** The C reaches its structs through
  pointers: 12,646 member reads at a pointer, 992 members' addresses
  taken, structs copied by bytes (`mem_copy`), allocated from an arena
  that is never freed, and the parallel `:%s`'s engines made by
  `alloc_clear`. A record per struct is the Java's and the Clojure's
  object model (`doc/CLOJURE-PROFILE.md`), the pointer analysis
  whimsical and caprice exist to avoid; item 3 is the part that is
  provable without it.
- **`option` for a pointer that may be NULL.** A pointer is an offset, 0
  the C's NULL, compared, stored and walked; an `option` would box every
  pointer and wrap every load and store, and the C's tests (`p = 0`) are
  already its own.
- **Variants for the C's enums.** The enumerators are stored in the
  memory and in the state as numbers, combined with `lor` and `land`
  (`MODE_INSERT` and the rest are bits), ordered and indexed; a variant
  would need a conversion at every load and store. A switch on them is a
  `match` already (item 4, and the C's 70 switches), its labels numbers
  named beside them.
- **Exceptions for the C's error returns.** vim's `OK`/`FAIL` and `-1`
  are values its callers test, store and pass on -- an exception per
  failure would need, for every caller, the proof that it only
  propagates. `host_exit`, the one jump the C makes across functions, is
  an exception (`Rt.Exit`).
- **`Fun.protect` for a frame's release.** 709 `frame_pop`s give a frame
  back where a function returns; `Fun.protect` would allocate a closure
  per call and take the tail position from every join's call, and an
  exception through a frame ends the editor anyway.
- **Labelled arguments.** The C's 1,713 functions take their arguments
  by position, and their calls are the C's; labels would add a name to
  every argument of every call and say nothing the parameters' names do
  not.
- **Modules for the code.** The core has no files left, and its calls
  are one cycle of 410 functions (`doc/OCAML.md` §2); the structs'
  members are modules (`Win_T.w_cursor_lnum`), the state a record.
- **Strings for the C's literals, and the string functions as `Bytes`'.**
  A literal is an address in the editor's memory, which the C compares,
  walks and stores; its text is beside it, `164037 (* "<buffer>" *)`.
  The C's own `musl_strlen` and kin are its loops, translated; `perf` on
  the heavy case puts none of them among the first twenty (the line
  width's loop in `win_linetabsize_cts` is 12 %, the frames' zeroing in
  `caml_fill_bytes` 10 %), and the memory functions are the runtime's
  already (`mem_copy`, `mem_fill`: the Scheme's runtime bodies).
- **The joins' and loops' names.** `join17` and `loop22` say which
  block of the lowered C they are, the same name in the Scheme's
  `editor.ss`; a name of what they compute is not in the C.
- **`TRUE` and `FALSE` as `true_` and `false_`** (459 uses): they are
  the C's int constants -- the flags that are answers are `bool` since
  phases 102-103 -- and OCaml reserves the lowered names.
- **`Int64` for the 64-bit types**: `doc/OCAML.md` §4.
