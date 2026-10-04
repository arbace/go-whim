# whiml: the editor in OCaml

**Built** (2026-10-04): whiml is the seventh translation of the core, beside
the Go (`editor/`), the Java (`braaam/`), the Clojure (`vijure/`), the
Haskell (`caprice/`), the Rust (`whimsy/`) and the Scheme (`whimsical/`),
and held to the same test: `whim test --ocaml` and `--wide --ocaml` answer
all 80 and all 240 cases as the C does, with a control of its own, and the
heavy case runs at 0.6-0.7 times the C. It was built in two milestones, as
whimsical was: faithful (§1-§9), then much more idiomatic
(`doc/OCAML-IDIOMS.md`, §10).

```
go tool whim whiml           # bin/whiml, built in lib/whiml
make bin/whiml               # the same
make whim-test-ml            # the quick suite with whiml too
go tool whim test --wide --ocaml
go tool whim skel editor.c DIR -ml editor.ml   # the backend on a core, by hand
```

## 1. What is on the machine

OCaml 5.5.0, native (`ocamlopt`, amd64, musl), without flambda; the
standard library's `Domain`, `Atomic`, `Mutex` and `Condition` for the
parallel `:%s`, and the `unix` library that ships with the compiler (`-I
+unix unix.cmxa`). No dune, no ocamlfind, no opam: nothing was installed,
and the build is `ocamlopt` called module by module from Go
(`whiml/whiml.go`).

## 2. The approach: the Scheme's forms, printed as OCaml

The Scheme backend (`doc/SCHEME.md`) already decides everything about the
C that a target without C's types has to decide: the memory's layout and
the initial image, which locals live in a frame, the out-parameters and
struct values as values, the order of evaluation (each read and call
bound where Scheme's open argument order could reorder it), the lowered
form's joins and loops as local procedures and every jump a tail call, the
switches, the names. OCaml is, like Scheme, an expression language with
proper tail calls and local recursive functions; what it adds is static
types. So **the OCaml backend does not lower the C again**:
`crefactor/togo/ml.go` runs the Scheme printer on every function, reads
each function back as forms with `scm_tidy.go`'s reader, and prints the
forms as OCaml (`ml_expr.go`), laid out by a Wadler-style printer of its
own (`ml_doc.go`). Every decision about the C is made once, in one place,
for both editors; `editor.ss` is written byte for byte as before (`whim
gen --check`).

The Scheme printer differs in one mode for OCaml (`newSgen(g, true)`),
where OCaml's integers do (below): an unsigned long's `<` is `u64<?` (the
runtime's unsigned order), a call through a function pointer says its
arity and whether it has a value (`call-ptr2v`), and an 8-byte initial
value is written in 63 bits.

What the OCaml printer adds is the types:

- **A form's value that is not looked at is `()`.** Scheme ignores the
  value of a statement; OCaml types it. The printer knows the C type of
  every function it calls -- the core's (the Scheme printer records which
  have no value: no result, no values, a struct written through `sret`),
  the host's, the runtime's -- and of the function a join is in, and
  writes `ignore (f ed x)` where a value is dropped (1,128 places).
- **Everything else OCaml infers.** A C bool is OCaml's `bool`, every
  other scalar `int`, multiple values a tuple, the variadic arguments an
  `int list`; the Scheme printer's conversions (`b->i`, `(not (fxzero? x))`)
  already say where a truth value becomes a number and back. `ocamlopt`
  type-checked the first core printed, all 1,713 functions, with no error.
- **Calls through a function pointer are typed** by one table per arity
  and result: `fn_table_2 : (ed -> int -> int -> int) array`, an index the
  pointer, `call_ptr2 p ed a b`. The tables are made empty at the top of
  the module and filled at its end, so the functions in them may come in
  any order.
- **The functions are ordered by their calls**: OCaml defines a name
  before its use, so the module is the call graph's strongly connected
  components in order, each cycle one `let rec ... and ...` (4 cycles, the
  largest 410 functions -- the regexp engine, the screen and the commands
  that call each other).

## 3. Memory

As whimsical's and caprice's: **every C object of an editor in one
`Bytes`**, laid out as the C lays it out on amd64 (the front end's sizes
and offsets, held to gcc's by `whiml`'s `TestLayout`: 155 structs and
unions, 911 members), and a pointer an `int` offset into it. Below 64 KiB
the null page and the function pointers; then the file-scope objects at
constant addresses, the string literals, the main domain's 8 MiB stack of
frames, the 1 GiB arena and 128 MiB of stacks for the parallel loop's
workers. `Bytes.create` of 1.2 GiB costs nothing until touched (0.02 ms;
musl's malloc maps it, and its pages are the kernel's zeros): the data, a
frame and the arena are zeroed as they are handed out, as in whimsical.

The loads and stores are the compiler's own primitives
(`%caml_bytes_get32u` and kin: native-endian, unchecked, inlined), each
taking the editor: `ld_s32 ed p`, `st_u8 ed p v`. The editor is a record,
`{mem; mutable sp; limit; glue; shared}`, so `ed` reaches the memory and
no function binds it. The C's names read the memory where they are used:

- a file-scope scalar `x` is a function of the editor that reads it, `x
  ed`, `set_x ed v` writes it, `x_addr` is its address; an array or a
  struct is its address, `iobuff`;
- a member is a module per struct: `Win_T.w_cursor_lnum ed wp` reads,
  `Win_T.set_w_topline ed wp v` writes, `Win_T.w_cursor_addr wp` is the
  address;
- a local in the call's frame is its address bound once, `let pos_addr =
  fr + 16 in`, read with `ld_s64 ed pos_addr`.

A C constant is named as the C names it (`let nul = 0`, `mode_insert`),
lowered; a name that OCaml reserves takes a `_` (`true_`), a local that
would shadow a renamed one a prime.

## 4. Integers

C's integers are OCaml's `int`, 63 bits. The 8-, 16- and 32-bit types are
exact, wrapped where C wraps them (`to_i32`, `u32_add`...). `long` and
`unsigned long` are the `int` itself, an unsigned long its 64 bits read as
a signed long: so `-1` is `ULONG_MAX`, and the runtime orders, divides
and shifts right unsigned longs as unsigned (`u64_lt`, `u64_div`,
`u64_shr`). This is **exact for every value under 2^62 in magnitude, and
for the low 63 bits of a sum, a difference, a product or a left shift past
it** -- a hash, say, whose low bits index a table. What it cannot hold
exactly: a value in [2^62, 2^63) or its negative, and the top bit of a
64-bit pattern. The C's constants past 63 bits -- `LONG_MAX`, `LONG_MIN`,
`LLONG_MAX`, `LLONG_MIN`, `MAXLNUM`'s uses (14 places) -- are saturated to
OCaml's `max_int` and `min_int`, in the code and in the image, so that a
line number compared with `MAXLNUM` compares as the C's does; a number
parsed past 2^62 (`musl_strtol`, `vim_str2nr`) overflows there, where the
C's overflows at 2^63. No case of either suite reaches such a value;
`crefactor/togo`'s `TestMlSigned` holds the arithmetic near the edge to
gcc's, and its comparison takes, for a C value past 2^62, its low 63 bits
-- which is what `TestMlIntegers` prints for `ULLONG_MAX / 3`.

The alternative, `Int64` for the 64-bit types, boxes every value that
leaves a register without flambda, and would type every `long` apart from
every `int` in the printer; declined for the editor's values, which are
line numbers, counts and offsets.

## 5. The host

`whiml/host.ml`: the Host record (`editor/host.go`'s interface, as a record
of functions on `Bytes` buffers) and the C host's 17 functions as glue to
it -- a record of the generated module's type `Editor.glue`, which the
editor carries, so the core imports nothing of the host's and the host
imports the core; `host_exit` an exception (`Rt.Exit`) that `run` catches.
`run h args` puts argv on the editor's stack and runs `vim_main`: four
editors run at once in one process, each on a domain and a host of its
own (`whiml_test.go`'s `TestEditorsAreInstances`, on `lib/whiml`'s
modules).

The terminal host, `whiml/term.ml`, is whimsy's `term.rs` in OCaml: `Unix`
for select, read, write, the pipe, the clocks and kill. What `Unix` has
not is the C host's own, in `whiml/c/term_stubs.c` (141 lines): the signal
handlers, real ones installed with sigaction(2) as the C installs them,
which store to C statics and write a byte to the pipe the host made, the
raw mode (`Unix.terminal_io` has no `IEXTEN`, `ONLCR` or `XTABS`), the
window's size (`TIOCGWINSZ`) and `musl_suspend`'s SIGTSTP. Deviations, as
whimsy's: the size a SIGWINCH reports is formatted by `Printf` where the C
calls vim_snprintf (the same digits); and `musl_delay`'s sleep is
`Unix.sleepf`, which a signal does not cut short.

vim's printf (`whiml/snprintf.ml`, 641 lines) is whimsical's `printf.ss`
ported, on the editor's memory: the numbers formatted on `Int64`'s
unsigned division, so `%lu` of `(unsigned long)-1` prints the C's
18446744073709551615.

## 6. The parallel `:%s`

`match_lines` gets its Scheme runtime body (`internal/whim/gen.go`), read
and printed like any function: `chunks ed n (fun ed from to -> ...)`, the
runtime's (`whiml/rt.ml`) -- the chunks on domains, each on a stack of its
own and on an engine the core's `alloc_clear` makes. Measured
(2026-10-04, a load of 1-6), a backtracking `:%s/\v(a|b)+c/X/g` over
500,000 lines of 52 characters, the session's time less the build's:

| workers | 1 | 16 | 32 | 64 |
|---|---:|---:|---:|---:|
| whiml, the `:%s` | 33.6 s | 3.9 s | 3.5 s | 4.5 s |
| the C, one thread | 34.2 s | | | |

and the heavy case 0.35 s on one worker, 0.30-0.32 on 8 or 16, 0.40-0.46
on 64. A domain is dear in OCaml: every domain alive is stopped by every
minor collection of the others. So the workers are the machine's
processors **at most 16** (`WHIML_WORKERS` sets them; 1 runs the C's
loop), made for each loop and joined after it. A pool of domains kept
waiting between loops was measured and declined: the heavy case went
0.39 -> 1.7 s, the waiting domains stopped by every minor collection of
the editor's.

## 7. The build

`whiml/whiml.go` (`go tool whim whiml`, `make bin/whiml`) cuts the core,
writes `editor.ml` with the backend (only when it differs), writes the
embedded sources beside it, and compiles each module with `ocamlopt`
when it or what it depends on moved, then the stubs, then the link:
`bin/whiml`, 8.0 MB, dynamically linked against musl. The suite builds
its two (the candidate and the control) in `.cache/whiml-suite/`.
`ocamlopt`'s defaults: `-inline 200` was measured -- the core's compile
22 s and 1.2 GB against 9.7 s and 0.54 GB, the heavy case unmoved.

## 8. Milestone 1: faithful (2026-10-04)

- **The backend** writes all 1,713 functions of the core, refusing none:
  `whiml/editor.ml`, 59,496 lines, tracked, written by `whim gen` beside
  the other six translations and refused stale by `make
  whim-editor-check`.
- **`ocamlopt` on the core: 9.6-10.2 s, 0.54-0.56 GB at the peak**, with
  no error; the whole build from a changed core about 12 s. Under `-w +a`
  the first printing had 1,759 warnings (1,343 a `let rec` that recurses
  not, 415 a name never used, the missing interface); the printer now
  writes `let rec` only for a cycle of calls and `_name` for a name never
  used, which leaves the interface (`doc/OCAML-IDIOMS.md`).
- **Foreign C** (`crefactor/togo`'s `TestMl*`): the 23 C programs the
  Scheme's and the Haskell's tests translate, and two of its own (signed
  arithmetic near 2^62 and unsigned longs past it, case labels), compiled
  with `ocamlopt` and required to print what gcc's builds print -- the
  values past 2^62 their low 63 bits, as §4 says; `TestMlControl` undoes an
  unsigned long's order, an unsigned char's widening, an unsigned shift
  and a struct's copy, and each moves the output.
- **The suite**: `whim test --ocaml`, all 80 cases as the C, the control
  (`" INSERT"`'s bytes changed in the module's image) seen by 76; `--wide
  --ocaml`, all 240, the control seen by 94 keys and 6 pty cases. Both
  passed once the literals an initializer alone names were placed before
  the memory's end was written (the first run's every case differed: `:q`
  was "Not an editor command", the command table's names overwritten by
  the stack).
- **The heavy case 0.6-0.7 times the C** (C 429-477 ms, whiml 251-306;
  the Go 0.4-0.5), on 16 workers; 0.9 on 64.
- **Start-up**, `:q!` from a file, the median of 20, three times: 4.4-4.8
  ms (the C 2.4-2.6); 11.7 MB resident at the peak.
- **Under load**: §9.

## 9. Under load

`internal/suite/stress_test.go` on `bin/whiml` (as built before the
printer's names of §8 changed; the program's behaviour is the same), under
48 busy loops on the 64 cores (the load average 35-51 at the end), every
run held to its case's first:

| runs differing | quick, file | quick, pipe | wide, file | wide, pipe |
|---|---|---|---|---|
| OCaml | 0 of 3,120 | 40 of 3,120 | 0 of 3,360 | 4 of 3,220 |

Fed from a file, as the suite feeds it, whiml answers every run as it
answered the first, the parallel `:%s`'s domains included. Through a pipe,
the control, it differs as the C does (66 and 106 in `doc/SCHEME.md`'s
table) and the editors that start slower do not: whiml starts in under 5
ms, before one write of the keys has always landed -- 39 of the quick
suite's 40 on `par_branch`, the rest one each on `par_undo`,
`showmode_ins`, `term_report`, `cmd_write` and `cmd_read`.
