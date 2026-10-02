# whimsy: the editor in Rust

**Built** (2026-10-02): whimsy is the fifth translation of the core, beside
the Go (`editor/`), the Java (`braaam/`), the Clojure (`vijure/`) and the
Haskell (`caprice/`), and held to the same test: `whim test --rust` and
`--wide --rust` answer all 80 and all 240 cases as the C does, with a control
of its own. It was planned here on 2026-09-26 and not scheduled; the plan
held, with two changes (a hand-written host, and the parallel `:%s` from the
start), and what was built is recorded below, milestone by milestone.

```
go tool whim whimsy          # bin/whimsy, built in lib/whimsy
make bin/whimsy              # the same
make whim-test-rs            # the quick suite with whimsy too
go tool whim test --wide --rust
go tool whim skel editor.c DIR -rs editor.rs   # the backend on a core, by hand
```

## What is on the machine

`rustc` and `cargo` 1.98.1; no crate is cached, and none is needed: the editor
builds from `std` alone, with the few libc calls the terminal needs declared
`extern "C"` (std links libc on Linux), so the build stays offline. Miri,
which could check the unsafe code for Rust's own undefined behaviour, is not
on the machine and was not installed.

Measured for the plan: a synthetic library crate shaped as the output would
be -- 2,000 functions of raw-pointer reads, writes and wrapping arithmetic,
26,003 lines -- compiled in 2.7 s at opt-level 0 and 1.7 s at opt-level 2,
about 0.3 GB at the peak: a tenth of GHC's time on the same shape.

## What Rust has, and has not

Rust is the target closest to C of the five:

| C | Rust |
|---|---|
| struct and union layout | `#[repr(C)] struct` and `#[repr(C)] union`: the C's layout, held to gcc's by a test |
| a pointer that walks, is compared, subtracted, cast | a raw pointer, `*mut T`: `.wrapping_add`, `.wrapping_offset`, `<`, `as` -- C's arithmetic exactly |
| a function pointer in a table | `Option<unsafe fn(...)>`, stored in the struct as it is |
| `return`, `break`, `continue` | the same, a label said where a labeled block stands between |
| the 49 forward gotos | labeled blocks, `'l: { ... break 'l; }`, as the Java backend writes them |
| a `switch` | `match` -- **without fall-through**: a switch that falls through is a ladder of labeled blocks |
| `x++`, `a = b` as a value, the comma operator | **not expressions** in Rust -- but a block is: `{ x = v; x }` |
| unsigned wrapping, `(uint8_t)x` | `wrapping_add` and kin, `as` -- C's conversions |
| a C variadic function | **not definable** in stable Rust: a slice of `VArg` |

So the translation keeps C's memory model, as the Haskell does -- but
natively, without offsets of its own -- and C's control flow almost as it is.

## The approach: C's own memory, natively

- **Every struct and union is a `#[repr(C)]` type** of the same members in
  the same order (`crefactor/togo/rs_types.go`), so of the same layout; a
  typedef of a scalar or a pointer is a type alias of its name (`char_u`,
  `linenr_T`), the C's `usize` `usize_` (Rust's own name is a primitive); an
  unnamed member type is named after its member (`attrentry_T_ae_u`). The
  backend lists every type's size and every member's offset as the C front
  end lays them out (`editor.rs.layout`), and `whimsy`'s `TestLayout` holds
  gcc's `sizeof`/`offsetof` and Rust's `size_of`/`offset_of!` to the listing,
  as compile-time assertions on both sides.
- **A pointer is a raw pointer**, `*mut T` whatever the C's `const` (a cast
  away of it is free) -- `*const T` where it is never written through
  (`RUST-IDIOMS.md` item 10: 869 of 2,770 pointer slots) -- `*mut c_void`
  for `void *`; walked with
  `wrapping_add`/`wrapping_offset`, subtracted by `pdiff` (the runtime's, in
  elements), compared with `<` and `==`, null-tested with `is_null()`.
  **No Rust reference to a C object is made but where the promise is
  proved** (9 functions' parameters, `RUST-IDIOMS.md` item 7: `pub fn
  ga_init(gap: &mut garray_T)`): a member is
  `(*p).m`, a place, read and written through the pointer, and an address is
  `&raw mut` of a place; an array's value where C converts it is
  `decay(&raw mut a)` (the runtime's: its first element's address), and an
  element `*decay(&raw mut a).wrapping_offset(i)`. No pointer-class analysis
  and no union handling: the Go, Java and Clojure backends' hardest parts.
- **A function pointer is an `Option<unsafe fn(*mut Editor, ...)>`** of the
  C's parameters, `None` for null, called `(f.unwrap())(ed, ...)`, a
  function's value `Some(name as unsafe fn(...))`.
- **The file-scope objects and the block-scope statics are the fields of one
  `#[repr(C)] Editor`** (a static `fn__name`), made zeroed and boxed and never
  moved (`new_editor`), its initial values written into it then
  (`init_globals`, in parts of 400 statements): every initializer's value that
  is not a zero stored at the member or element it initializes, the front
  end's offset of it read back as the path, `(*ed).cmdnames[3].cmd_name =
  b"append\0".as_ptr() as *mut u8;`; a compound literal in a file-scope
  initializer is a field of its own (`lit_N`), as long-lived as the object
  that points at it. One more field is the host's pointer, which the C does
  not have. A process holds as many editors as it makes.
- **A function is `pub unsafe fn name(ed: *mut Editor, ...)`** where it
  needs both (closed over the calls): it takes the editor only
  where it reaches an object of it, the host or a function pointer, or calls
  a function that does (1,497 of the 1,713; the analysis the Haskell and
  the Scheme share, `cfacts.go` and `effects.go`), and is `unsafe` only
  where it does what Rust calls unsafe (`rs_fx.go`, on the same call graph:
  1,647; 66 are safe `pub fn`s, `pub fn
  musl_isdigit(c: i32) -> bool`). What the hand-written crate calls by name
  (`Profile.RsExports`), what is a function pointer and what a runtime body
  names keep the whole signature, the editor `_ed` where unused; a parameter
  the body never names is `_name`, one it assigns `mut`. A body's last
  `return x;` is its tail, `x`.
- **A local is declared where the body first gives it a value**, when that
  is a statement of a block holding every mention of it -- `let col: colnr_T
  = ...;`, `mut` only where the rest of the body writes it again
  (`sinkDecls`): every path to a later mention passes the statement, and
  inside a loop each time round gets a new variable whose first act is the
  store. **Else at the function's top**: with no value, `let x: i32;`, where
  a walk of C's statements in Rust's order sees every read after a store
  (`rs_defer.go`, rustc's own analysis, which checks it), else zeroed -- C at
  -O0 keeps a local's value from one iteration of a loop to the next, and a
  goto's labeled block must not end a local's scope. C's nested scopes are
  one Rust scope: a local takes a name of its own (`i_2`), and never a
  constant's or a function's, which a `let` would read as a pattern.

## Expressions

- **C's arithmetic is exact** (`rs_expr.go`): the integer types are Rust's of
  C's widths (`char` `i8`, `long` `i64`), a C `bool` a `bool`. **Signed
  arithmetic in `int` or `long`, whose overflow C leaves undefined, is Rust's
  plain `+ - * / %` and negation**: an overflow is a bug, which a build with
  overflow checks reports by panicking and the release build -- its profile
  pins `overflow-checks = false` -- wraps as gcc's -O0 does; the suites
  answer every case as the C does on a build with the checks on, so no path
  they reach overflows. An operation whose result provably fits its type
  (`rs_range.go`: `*s as i32 - b'0' as i32`, a mask, a remainder) and an
  unsigned division are plain too; what C defines modular -- unsigned
  arithmetic, a narrow type's increment, a compound assignment computed in a
  wider type -- is `wrapping_*`, as is a pointer's step; a shift by a
  constant `<<`, by a variable `wrapping_shl`; a compound assignment whose
  operation is plain is `x += n`. C's usual arithmetic conversions are decided
  as the other backends decide them (`usualK`) and said with `as`. `+= -= *=
  &= |= ^=` are computed in the left side's own type, whose low bits they
  are; the rest in the usual type and converted back. A compound assignment
  whose right side calls computes it first, then reads the left: gcc's order.
- **What C does inside an expression is a statement where C's order lets
  it** (`rs_hoist.go`): an increment of a local nothing else can see, named
  once in the expression and not in an operand C may not evaluate, is done
  before or after the statement (`*d = *src; d = d.wrapping_add(1);`); an
  assignment or a comma a condition, a return or an initializer evaluates
  first is done before it; a `while` whose condition does something is a
  `loop` that does it, then breaks. **Else it is a Rust block**, which is an
  expression: `x++` as a value is `{ t1 = x; x = t1 + 1; t1 }`, an
  assignment's value `{ x = v; x }`, the comma `{ a; b }` -- in C's order,
  left to right, as the Java's and the lowered form's. An lvalue whose
  evaluation does something is read and written through its address, taken
  once.
- **A condition is a Rust `bool`**: `x != 0`, `!p.is_null()`, `f.is_some()`;
  a comparison, `&&`, `||` and `!` are `bool`s, made an `int` (`as i32`)
  only where C uses one as a number.
- **A constant is spelled as C spells it** where it can be: an enumerator
  its constant (`pub const K_DEL: i32 = ...`, each of its own type), a
  character `b'a'`, else its value, a literal of the place's type; a string
  `c"append".as_ptr() as *mut u8`, a byte string `b"...\0"` where it holds a
  NUL or a byte past ASCII.
- **A call of a function declared and not defined is the host's**:
  `crate::host::host_write(ed, ...)`; a variadic one (`vim_snprintf`) takes
  its variadic arguments as a slice, `&[VArg::I(n as i64), VArg::P(s as *mut
  c_void)]` -- a C variadic argument after the default promotions, signed,
  unsigned or a pointer.

## Control flow

- **C's own** (`rs_fn.go`): `if`, `while` (`loop` for a constant condition),
  `return`, `break` and `continue` as they are; a `for`'s step and a
  `do`'s condition run after the body -- a short step written before each
  `continue`, else reached by leaving a labeled block around the body
  (`'c3: { ... break 'c3; ... }`, 2 of them); a `do { ... }
  while (0)` -- phase 173's goto regions -- is a labeled block its breaks
  leave. A `break` or `continue` says its loop's label where a labeled block
  stands between it and its loop, as Rust requires.
- **A goto is a labeled block** that ends at its label, the goto `break
  'g_label;` -- the Java backend's rule (`java_stmt.go`): every goto left in
  the core is a forward jump to a label of a block that holds it.
- **A switch is a `match`**, each arm a run of cases -- a case's statements
  and those of the cases it falls into -- its patterns the C's constants'
  names where it names them (`ESC =>`, `97 /* 'a' */ =>`), the default's
  arm last, where Rust wants its catch-all. A run of one case is its
  statements, the `break` that ends them dropped; a longer one is **a
  ladder of labeled blocks**, the arm's value bound and matched again at its
  heart, breaking to the block whose end its case's statements follow, so
  that falling through is going on:

  ```
  v @ (1 | 2) => { 'v1: { 'v0: { match v { 1 => break 'v0, _ => break 'v1 } }
                          case 1's statements }
                   case 2's statements }
  ```

  This is C's shape kept; the plan's lowered form for these is not needed.
  A case of no statements is the next case's pattern, and one that falls
  into at most five simple statements has them written again, an arm of
  its own (`RUST-IDIOMS.md` item 15).
- **What Rust's own analysis requires is kept**: a statement after one that
  cannot complete (in Rust's terms: `loop` with no break, a labeled block no
  break leaves) is not written, being dead, and a function whose end Rust
  can reach returns its type's zero there -- C's undefined value.
- **The lowered form for what labeled blocks cannot say** (`rs_lower.go`): a
  goto back or into a statement, a case label inside a statement of its
  switch -- printed as the Clojure and the Haskell backends print every
  function, a `loop` over a `match` on the block's number. The core has none
  such (0 of its functions); the foreign C tests do.

## Milestone 1: the types and the printer on foreign C (2026-10-02)

- **The printer on foreign C**: `crefactor/togo`'s `TestRs*` translate the 21
  C programs the Java, Clojure and Haskell tests translate (integers, flow,
  strings, structs, varargs, pointers, gotos, the lowered form's machines and
  shapes, names, tables, the growarray), compile each with `rustc
  -D warnings` beside a harness host (`out`, `outs`, a printf, an allocator
  and the byte functions) and the runtime, and require it to print what
  gcc's build prints. `TestRsControl` undoes one of C's rules at a time in
  the Rust -- unsigned division done signed, an unsigned char widened with
  its sign, an unsigned shift done arithmetically, a struct's assignment
  not made -- and each moves the output.
- **The layout**: `whimsy`'s `TestLayout` on the core: 156 structs and unions
  C can name (one of them a function's own, which gcc cannot name at file
  scope: Rust's side only), 914 members, gcc's and Rust's layouts both the
  listing's, compile-time assertions on both sides; the control, one offset
  moved by one, is refused by both.
- **Coverage on the core**: the backend writes **all 1,713 functions of the
  core and refuses none**, 0 of them from the lowered form; 70 switches, 59
  matches and 11 ladders; the 49 gotos in 10 labeled blocks; 164 structs and
  unions; an Editor of 811 fields. 65,299 lines of Rust.

## Milestone 2: every function of the core compiles (2026-10-02)

- **The crate** (`whimsy/`): `Cargo.toml` (package `whimsy`, no dependency,
  a library and the program), `src/lib.rs` (`#![deny(warnings)]`: a warning
  is an error), the generated `src/editor.rs`, and by hand the runtime
  (`src/rt.rs`: `VArg`, `decay`, `pdiff`, `fn_addr`, `str_u8`/`str_i8`,
  `Shared` and `chunks`) and the host's glue (`src/host.rs`).
- **The build**: `go tool whim whimsy` cuts the core from `src/whim-vim.c`,
  writes its module into `lib/whimsy/src/editor.rs` (only when it differs),
  writes the embedded hand-written sources beside it the same way, and runs
  `cargo build --release --offline` there (`whimsy.Build`), which must say
  nothing; the program is `bin/whimsy`.
- **Measured**: the 1,713 functions -- 65,299 lines -- compile with no
  warning, the only lints allowed the C's own naming (`non_snake_case`,
  `non_camel_case_types`, `non_upper_case_globals`) and
  `unused_assignments`, which every local's zero at the function's top
  would otherwise raise; **rustc 38.9 s and 0.56 GB at the peak** for the
  release build (opt-level 2), 2.5 s at opt-level 0: a quarter of GHC's time
  on the same core at a sixth of its memory.

## Milestone 3: the host, the launcher and the suite (2026-10-02)

- **The host, by hand** -- the plan had it translated; it follows the other
  four editors instead. `src/host.rs` is the `Host` trait, `editor/host.go`'s
  Host on Rust's types (the window's size, raw mode, the keys, the clocks,
  the wait for input and its reading, the signals, output), every method of
  `&self`, since the core may call the host again from inside a call of it
  (a SIGHUP's deathtrap, run where the host waits); and the C host's 17
  functions as glue to the Host the editor carries in its `host` field, with
  the arena that is the editor's and not the Host's: 1 GiB allocated zeroed
  (calloc's pages, touched as used) and bumped atomically, as the C host's,
  16 bytes aligned -- a larger alignment made `alloc_zeroed` write the whole
  gigabyte, 1.1 s at every start. `host_exit` unwinds to `run(host, args)`,
  which makes an editor, runs `vim_main` to its end and returns the status:
  the C's longjmp to `main`. So a process runs any number of editors, each
  on its own host and thread (`whimsy`'s `TestEditorsAreInstances`: four at
  once, each on a host of the test's own, each its own text on its screen and
  no other's).
- **The terminal host** (`src/term.rs`) is the C host's operating-system half
  function by function over `extern "C"` libc, its structs checked against
  musl's x86_64 layout by gcc: raw mode with `tcgetattr`/`tcsetattr`, the
  size with `ioctl(TIOCGWINSZ)`, the wait a `select` on the keys and a
  wake-up pipe. **The signals are real**: `sigaction` handlers that store to
  static atomics and write a byte down the pipe (`pipe2(O_NONBLOCK |
  O_CLOEXEC)`), as the C's do; a SIGHUP's or SIGTERM's deathtrap is run on
  the editor's thread where the C's `host_deliver_death` runs it. Its
  deviations -- `atol` and the size report spelled by hand, a second
  `init`'s pipe closing the first, SIGPIPE set back to its default before
  `main` (Rust's runtime ignores it) -- are marked `DEVIATION`.
- **vim's printf** (`src/printf.rs`) is the C host's `vim_snprintf` ported by
  hand, as `editor/format.go`, braaam's `Printf.java` and caprice's
  `Printf.hs` are: its `va_list` the call's `[VArg]` and an index, an
  argument read at the type the C's `va_arg` names. Held while it was
  written to the C's own, compiled from `src/whim-vim.c`, on a table of
  77,925 formats and arguments (every conversion and flag, widths and
  precisions, `*` and positional arguments, truncation) and 37 of the error
  paths: byte for byte, return values and buffers.
- **The launcher** (`src/main.rs`): `bin/whimsy [args]`, the editor on the
  terminal.
- **The suite**: `go tool whim test --rust` (`make whim-test-rs`) and `--wide
  --rust` build the candidate's core into `.cache/whimsy-suite/` -- the
  candidate and its control, `" INSERT"` changed in the generated
  `editor.rs`, compiled side by side -- and require every case answered as
  the C candidate answers it. **whimsy answers all 80 quick cases and all
  240 wide ones as the C does**, its control seen by 76 of the 80 and by 94
  keys, 0 Ex, 0 argv and 6 pty cases of the wide suite's, as the Go's is.
- **One bug, found by the suite**: `!(a && b != 0)` was printed `a && b ==
  0` -- the printer turned a negated `x != 0` round without asking whether
  `x` was the whole operand -- and 34 of the 80 cases moved (every count,
  every `par_*` case). Fixed in the printer; none other was found.
- **Speed**: the heavy case 0.25-0.3 times the C's time (C 445-467 ms,
  whimsy 111-131 ms over four runs of the suite; the Go 0.5-0.6) -- the
  release build at opt-level 2, and `match_lines` on every core.

## Milestone 4: kept current (2026-10-02)

- **The generated crate is tracked**: `whimsy/src/editor.rs`, written by `go
  tool whim gen` beside `editor/editor.go`, `braaam/editor/`, `editor.clj`
  and `Editor.hs`, from the same core, and refused outright when the
  backend refuses any part of it; never edited by hand.
- **`make whim-editor-check` refuses it stale**: a byte appended to it is
  named (`editor.rs is NOT what the generator writes`), and `whim gen` puts
  it back.
- **The build builds it**: `make` (`all`) builds `bin/whimsy`, which `go tool
  whim whimsy` makes from the core of `src/whim-vim.c` as it stands, and
  `make clean` removes it, `lib/whimsy` and the suite's cache.

## After the milestones: what the printer took out (2026-10-02)

Four changes of spelling, together held to the whole suite (all 80 and all 240,
the heavy case 0.3 times the C's):

- **a local declared where it is first given a value** at the function's
  own level (above);
- **a pointer's cast of a cast is one cast**: `p as *mut c_void`, not `p as
  *mut i8 as *mut c_void` (the C's `(char *)` before a `void *` parameter);
- **a compound assignment whose right side calls computes it first only
  where the call could change the left side**: not for a local whose address
  is never taken;
- **a switch a match of runs of cases**, its patterns named (above): the 11
  switches that fall through were one ladder each, the largest 40 blocks
  deep; now a ladder is a run's alone.

The module went from 65,299 lines to 62,836; rustc 37.9 s and 0.55 GB at
the peak.

## The idioms (2026-10-02)

`RUST-IDIOMS.md` surveyed what reads as C in the module, ranked it, and
items 0-9 and 11 are done, each a change to the printer held to the whole
recipe -- the crate with `#![deny(warnings)]`, the foreign C tests at
opt-level 0 (where an overflow panics), all 80 and all 240 cases with the
control seen, `whim gen --check`:

- `whim whimsy --lint` counts rustc's warnings on the module with its
  `#[allow]` taken out: 2,817 -> 702, the 482 C names kept;
- safe functions and the editor where it is used (`rs_fx.go`): 66 `pub fn`,
  217 functions of no editor; no `return` at a body's end;
- signed arithmetic as C means it, plain where C's overflow is undefined or
  provably cannot happen (`rs_range.go`), and `x += n`: `wrapping_*` 9,337 ->
  4,798, left for pointers and unsigned arithmetic;
- locals declared in their block, or with no value (`rs_defer.go`): 3,213
  zeros at the top -> 1,098, `unused_assignments` 2,335 -> 220;
- side effects out of expressions (`rs_hoist.go`): blocks 665 -> 153,
  temporaries 528 -> 50;
- c"" literals; a `for`'s step before its `continue` (labels 132 -> 91);
- references for the 9 functions where the promise is provable
  (`rs_refs.go`).

The module went from 62,836 lines to 61,820; rustc 38.0-38.3 s and 0.53 GB
at the peak; the heavy case 0.25-0.3 times the C's, as before.

A second pass (`RUST-IDIOMS.md`, *The second pass*, items 10 and 12-16;
17 declined), each held to the same recipe and to both suites on a build
with overflow checks:

- read-only pointers as `*const` (`rs_const.go`): an inference over every
  pointer slot's writes and flows, which rustc checks -- 869 of 2,770
  slots; `VArg::P` a `*const c_void`, as vim's printf only reads its
  arguments; `decay_const` for an array reached through a `*const`.
- temporaries declared where they are given their value (`let t1: T =
  v;`, item 12);
- every zero rustc does not need, and C's dead stores not written (item
  13): zeros 1,113 -> 932, the module allows no `unused_assignments`, 4
  functions expect it for a store through a pointer rustc does not follow.
- stores out of conditions (item 14): a loop that checks its condition at
  its top, nested ifs for an `&&` that stores, a negated comparison turned
  round; blocks 154 -> 72.
- a switch's fallthrough (item 15): an empty case the next's pattern, a
  few simple statements fallen into written again; labels 91 -> 75.
- `else { if }` as `else if` (item 16), an else if that stores an else
  block; blocks 72 -> 67.

The module went from 61,820 lines to 61,834; rustc 39.0-39.5 s and 0.53 GB
at the peak; the heavy case 0.25-0.3 times the C's, as before.

## Not done

- **What the memory model forbids**, declined in `RUST-IDIOMS.md`: `ed: &mut
  Editor`, or a local reborrow of it (item 17: 1,820 addresses of the
  editor's places are taken, some kept in tables, the parallel `:%s` hands
  the editor to every core, and the host calls the core back), owned data,
  slices for C strings, Rust enums, `p.add(n)`, the 2024 edition. 1,648
  functions stay `unsafe fn`.
- **Miri** could check the module for undefined behaviour of Rust's own; it
  is not on the machine and was not installed.

## Risks, named in advance

- **Undefined behaviour of Rust's own.** Raw pointers, references only where
  a function touches memory through nothing else, and `wrapping_*` pointer
  arithmetic, which is defined wherever it points; Miri, which could check
  the rest, is not on the machine.
- **Signed overflow** is undefined in C and wraps under gcc `-O0`, which is
  what the suite measures. The release build wraps the same (its profile
  pins `overflow-checks = false`); a debug build, or the foreign tests at
  opt-level 0, panics there instead -- the Rust idiom's reading of C's
  undefined behaviour, measured on the suites (none of their paths
  overflows).
- **Readability**: `unsafe` Rust in C's shape where the memory model needs
  it. The price of a faithful translation, as for the others.
