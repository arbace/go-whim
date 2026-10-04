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
(*Done*, below); a second pass the same day re-examined what the first
declined -- the model -- and did items 7-10 (*The second pass*); what
stays declined is at the end, each with the measurement that declines it.

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

## The second pass: the model

Milestone 2 left whiml C in OCaml's syntax: every struct an address,
every enumerator a number, every literal an address. The second pass took
each declined item again with what the repository knows that the first
did not use: the C's own types, through the front end the Scheme printer
already reads (`cc`), and OCaml's type checker as the proof of what the
backend claims -- a variant or a record that meets an integer anywhere
fails the build, the way gcc is the control of the pipeline's cuts. What
the type checker cannot see -- an order, an equality of addresses, memory
-- the backend checks on the C and refuses. Each item held to the
recipe; measured on 2026-10-04, the load 7-33 (another build beside).

### 7. The C's enumerations as variants

A C enumeration the profile names (`MlVariants`, `internal/whim/gen.go`)
is a type of constant constructors, `type paste_mode = Paste_insert |
Paste_cmdline | Paste_ex | Paste_one_char`: its enumerators the
constructors wherever the C names them, its case labels patterns (`|
Addr_lines | Addr_other ->` where the match said `| 0 (* ADDR_LINES *)`),
a field of the state of its type initialized with a constructor. A member
or an object in memory of the type is its number there, read and written
through two conversions (`cmd_addr_of_int`, `int_of_cmd_addr`; a number
no enumerator has fails), so the C's tables keep their layout. A match
naming every constructor has no other arm, and an other arm lists the
constructors left (a wildcard is a fragile match under `-w +a`). The
backend refuses an enumeration a relational operator compares: OCaml
would order the variant by its constructors (`ml_enum.go`).

**7 of the 12 named enumerations of more than one enumerator**:
`cmd_addr_T`, `etype_T`, `flush_buffers_T`, `keyprot_T`, `optmagic_T`,
`paste_mode_T`, `set_op_T` -- 163 uses of their constructors, 13
conversions at memory. The other five, each tried alone: `hlf_T` and
`magic_T` are ordered (2 and 20 operands; refused), `CMD_index` is
counted (`eap->cmdidx = (cmdidx_T)((int)eap->cmdidx + 1)`) and indexes
the command table, `key_extra` and `SpecialKey` are stored as key bytes
and index `term_strings` (the type checker's first error each). The 819
one-enumerator enums are the C's `#define`s, not enumerations: they stay
constants, and a label that names one stays a number with its name
beside it. `TestMlVariants`, `TestMlVariantOrdered`.

### 8. Structs as records

A struct the profile names (`MlRecords`) is an OCaml record of mutable
fields, `type exarg = { mutable nextcmd : int; ...; mutable forceit :
bool; ...; exarg_mem : int }`. A local of the type, which the C gave room
in the call's frame, is a record made where the function starts
(`Exarg_T.make (fr + 16)`); a pointer to one is the record; a member a
field, `eap.line2 <- eap.line2 + 1`; an assignment `copy_into`, an
initializer the fields' stores, a `{0}` or `memset(p, 0, sizeof *p)` a
`clear`. A member whose address is taken, or that is a struct or an
array, stays memory: the record holds the address of a block of its own
in the frame (`exarg_mem`), those members' accessors read there, and
`copy_into` and `clear` take the block too. A table of function pointers
whose functions take a record is a table of its own (`fn_table_1v_0exarg`),
so the ex and normal commands take their records. The Scheme printer says
so in its forms in the OCaml's mode only (`define-c-local ... rec:S`,
`rec-copy!`, `rec-clear!`); `editor.ss` is byte for byte as before.

That is the C's meaning only where no pointer to the struct is ever
memory, so the backend refuses a named struct that a static object, an
array, another struct or a union holds, that a cast converts, that
`sizeof` measures or the functions of bytes are handed but to clear it,
that is passed or returned by value, that has a bit field, or whose
pointers are compared -- a record's `=` is its fields', and one compared
with NULL is an option (`ml_record.go`). The type checker proves the
rest.

Measured with a throwaway closure on the C (`.tmp/`, not tracked): of the
164 structs and unions, 93 are memory -- 38 the type of a static object,
29 cast to from the arena, 6 unions, 20 held by those -- and 47 of the
other 71 are phase 100's out-structs, which the Scheme's forms already
return as tuples. Of the 24 left, **12 are records**: `bufref_T`,
`chartabsize_T`, `cmdarg_T`, `exarg_T`, `incsearch_state_T`, `lineoff_T`,
`optexpand_T`, `optset_T`, `save_state_T`, `searchstat_T`, `ttyinfo_T`,
`winlinevars_T` -- 102 fields, 1,596 field accesses (`eap.line1`) where
there were accessor calls, 49 locals made; the member modules' accessor
calls 13,621 -> 12,094; functions with a frame 228 -> 211. Left memory:
`oparg_T` (714 accessor calls), compared with NULL in 5 places and kept
in the static `current_oap` -- item 3's option, declined below;
`viewstate_T` and `tasave_T`, members of `incsearch_state_T`'s and
`save_state_T`'s memory; `msgchunk_T` (26 comparisons, a cast) and
`searchit_arg_T` (passed as NULL). `chartabsize_T` is the hot loop's
argument (`win_lbr_chartabsize`): the heavy case moved 250 -> 225 ms when
it became a record. `TestMlRecords`, `TestMlRecordMem`,
`TestMlRecordCompared`.

### 9. A string compared with a literal compares with an OCaml string

`musl_strcmp ed key 163639 (* "NONE" *) = 0` is `c_str_is ed key
"NONE"`: a test for 0 of the profile's string comparisons (`MlStrings`:
strcmp, strncmp and their ASCII-folding kin) against a literal is
written on an OCaml string by four functions of the runtime --
`c_str_is`, `has_prefix` (a count the literal's length or less, the
literal cut to it; a longer count compares the terminator too), and
`_ci` for the folding ones. **90 comparisons**; a ladder of them reads
as one now (`c_str_is ed key "TERM"`, `"CTERM"`, `"GUI"`...). The
literals still an address 622 -> 532. `TestMlStrEq`.

### 10. `a + -x` is `a - x`

The Scheme's `(fx+ a (fx- 0 x))` printed `a + -x`: 16 places, now 0.

### After the second pass

| | milestone 2 (`823aad7`) | after items 7-10 |
| --- | ---: | ---: |
| `editor.ml` | 57,584 lines | 57,093 lines |
| variants / their constructors' uses | 0 | 7 / 163 |
| records / their fields' accesses | 0 | 12 / 1,596 |
| member accessor calls (`Win_T.w_topline ed wp`) | 13,621 | 12,094 |
| literals compared as OCaml strings | 0 | 90 |
| functions with a frame | 228 | 211 |
| tables of function pointers | 5 | 7 |
| `ocamlopt` on the core | 9.4-9.6 s, 0.53 GB | 9.6 s, 0.51-0.53 GB |
| the heavy case | 0.5-0.6 times the C (242-259 ms) | 0.5-0.6 (225-257 ms; 296 once at a load of 33) |

## Declined, with why

What the second pass declines it declines measured:

- **Records for the rest of the structs.** 93 of 164 are memory by the
  closure above: the windows, buffers, lines, undo headers and regexp
  programs are allocated from the arena the C never frees, held by
  static objects and by each other, and walked by pointer arithmetic
  (`Win_T`'s 3,554 accessor calls, `Buf_T`'s 1,285, `Pos_T`'s 1,075).
  A record for one of them is the Java's and the Clojure's object model,
  every pointer an object reference and every byte array an object of
  its own: a second backend over the C's tree (the Java's), not a change
  to this one over the Scheme's forms -- and those editors take 1.6-3.9
  times the C's time in the heavy case, whiml 0.5-0.6.
- **`option` for a pointer that may be NULL.** No pointer to a record is
  compared with NULL (the type checker says so: a null pointer met by a
  record fails the build). The pointers that are are offsets into the
  memory, where an `option` would box an `int` and say nothing the C's `p
  = 0` does not; and `oparg_T`, the one struct an option would make a
  record, is tested in 5 places and dereferenced in 714: the OCaml
  printer reads the Scheme's untyped forms, so without a nullness
  analysis each would be `Option.get`, which is no idiom.
- **Out-parameters as tuples**: done before this survey, and not by the
  OCaml printer -- phase 100 made 94 out-parameters of the C values in and
  out, and the Scheme printer's forms return 64 struct results and 2
  out-parameters more as values (`cfacts`' `outparams.go`): `let (r1, r2)
  = one_letter_cmd ed p idx in`.
- **Exceptions for the C's error returns.** The answers are `bool` since
  phases 87a, 102 and 103, and a caller tests each; the 49 gotos the C
  keeps leave a loop or a switch within one function, which the forms
  write as a tail call to a join -- OCaml's own idiom for it. The one jump
  across functions, `host_exit`, is an exception (`Rt.Exit`).
- **`Fun.protect` for a frame's release.** 211 functions keep a frame; as
  milestone 2 said, a closure a call and every join's tail position lost.
- **Labelled arguments.** The C's 1,713 functions take their arguments by
  position, and every call is the C's.
- **Modules for the code.** The functions are one cycle of 410 functions
  and **21,526 lines** (the regexp engine, the screen and the commands
  that call each other), which OCaml can split only into recursive
  modules with signatures written for each; what precedes and follows it
  is ordered by calls, not by subsystem, and the core has no files left to
  name them by. The structs' members are modules (`Win_T`), the records'
  making and copying too (`Exarg_T.make`).
- **Strings for the rest of the literals.** 532 are addresses the C walks,
  stores or hands to a function that reads the memory (`msg`, the
  formats of `vim_snprintf`, `vim_strchr`'s sets, the option names'
  table); an OCaml string there is a copy into the memory at every use.
- **The joins' and loops' names**, **`TRUE`/`FALSE` as `true_`/`false_`**
  and **`Int64` for the 64-bit types**: as milestone 2 declined them
  (`doc/OCAML.md` §4 for the last).
