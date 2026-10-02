# How whimsy could be more idiomatic Rust: a survey

2026-10-02. **Status: items 0-11 done; see *Done* after the ranking, and
*The second pass* for item 10 and the residue.** It covers
`whimsy/src/editor.rs` as tracked at `cb3f2db` (62,836 lines, 1,713
functions, written by `crefactor/togo`'s Rust backend from the core `go
tool whim cut src/whim-vim.c` prints, 75,721 lines of C) and the
hand-written crate around it: `rt.rs` (135 lines), `host.rs` (262),
`printf.rs` (1,008), `term.rs` (516), `main.rs` (30). `HASKELL-IDIOMS.md` is
its model, and `GO-IDIOMS.md`, `JAVA-IDIOMS.md` and `CLOJURE-IDIOMS.md`
before it: every item says what the pattern is, how many sites it has, what
it would become, who would do it -- the Rust printer (`crefactor/togo/rs*.go`),
an analysis it shares or could share with the others, a pipeline phase that
changes the C, or the hand-written crate -- its cost (S/M/L), its risk, and
how it is verified. `editor.rs` is generated and stays so: nothing here is a
change by hand to it.

## The tension, stated first

whimsy was built to be C in Rust's syntax (`doc/RUST.md`): every C object in
`#[repr(C)]` memory, reached through raw pointers; every function an `unsafe
fn` of `ed: *mut Editor`; every operation that can overflow `wrapping_*`; C's
control flow as Rust's, a goto a labeled block. That model is why the
translation is faithful -- no pointer-class analysis, no union handling, the
layout held to gcc's by a test -- and why it is the fastest editor of the
five: **the heavy case at 0.25-0.3 times the C's time**. It is also what a
Rust programmer would object to first. Idiomatic Rust is safe code, owned
data and references whose promises the compiler checks, values that say
what they are, and arithmetic that panics where it overflows by mistake.

Measured here, the two meet as follows:

- **The memory model stays, and most of what reads as foreign does not need
  it.** 2,335 of rustc's own warnings are switched off by the module's one
  `#[allow]` beside the C's names -- 2,332 `unused_assignments`, nearly every one a local's zero
  at the function's top that the body overwrites before reading; 875
  functions end in `return x;`; 682 side effects are written as blocks
  inside expressions (`*({ t3 = s; s = t3.wrapping_add(1); t3 })`); 9,337
  operations are `wrapping_*`, of which about 4,260 are on signed integers,
  whose overflow C leaves undefined. Each is a printer rule, and rustc itself
  is the proof of most: a declaration Rust cannot see initialized, a
  function written safe that does something unsafe, does not compile.
- **Safety is scarce, and it is the C's, not the printer's.** 217 of the
  1,713 functions reach no object of the editor's, no host and no function
  pointer, even through what they call; 57 of them do nothing Rust calls
  unsafe -- no raw pointer dereferenced -- and could be safe `fn`s. The rest
  read or write memory through raw pointers that other code holds: the
  windows, the buffers, the lines.
- **References need a proof the C does not give.** A `&mut T` promises that
  nothing else reaches `*T` while it lives, and rustc then tells LLVM so
  (`noalias`). 1,850 addresses of the editor's own objects are taken
  (`&raw mut (*ed).x`), some kept in tables (the options' `&p_wiv`), and the
  parallel `:%s` hands the editor to every core: an `ed: &mut Editor` would
  be a promise the program breaks. Of pointer parameters, a reference can be
  proved only in a function that touches memory through nothing else --
  measured: 9 functions.
- **What the model forbids is out of scope.** Owned data (`Vec`, `String`,
  `Box` per object), safe references for the editor and its structs, slices
  for C strings: each is a different memory model and a different
  translation, not a change of spelling; each is declined below with what was
  counted.

## How it was measured

Everything below was counted or run, not estimated, unless it says so. The
instruments are throwaways under the worktree's `.tmp/rsid/`; none is
tracked.

| Instrument | What it gave |
| --- | --- |
| `whim skel core.c D -rs editor.rs` | `editor.rs` regenerated from the cut core: byte-identical to the tracked file, so every count is of what `whim gen` writes |
| `scan/` (Go, reads `editor.rs`) | the module split into its functions, and per function: its signature (`unsafe`, the editor named or not, raw pointer parameters), its declarations (zeroed at the top or at their first value), its last statement; and over the text: `wrapping_*` by method, `(*ed).`, `&raw mut`, `decay(`, casts, block expressions, labels by kind, string literals |
| `cargo check` on a copy, the `#[allow]` and `#![deny(warnings)]` taken out | rustc's own lint: **2,817 warnings**, 482 of them the three naming lints on the C's names, 2,332 `unused_assignments` (2,131 of them a top-of-function zero) and 3 `value passed is never read` -- the counterpart of GHC's `-Wall` for caprice and clj-kondo for vijure. Clippy is not on the machine and was not installed: where a clippy lint names a pattern (`needless_return`, `assign_op_pattern`), the pattern is counted by `scan/` |
| the backend's own counts (`RS_SURVEY=1`, a throwaway counter beside the printer, removed after) | the effects closure (the editor, unsafe), the references' fixpoint, `wrapping_*` by operand type (pointer, signed, unsigned) and which provably fit, block expressions by kind |
| `whim test --rust` and `--wide --rust` with `CARGO_PROFILE_RELEASE_OVERFLOW_CHECKS=true` | the suites on a build whose signed arithmetic is Rust's plain `+ - *`, compiled to panic on overflow: what C's undefined behaviour the suites reach (item 4) |
| `go tool whim whimsy` | rustc's wall time and peak memory for the release build, printed by every build: **39.4 s, 0.55 GB** at `cb3f2db` |

**Verification recipe for every item** (the suites compare output byte for
byte, so a change that moves one byte of one screen is seen):

- the crate builds, `#![deny(warnings)]` kept (`go tool whim whimsy`);
- `cd crefactor && go test ./togo`: the 21 foreign C programs translated,
  compiled with `rustc -D warnings` at opt-level 0 -- where an overflow
  panics -- and held to gcc's output, and `TestRsControl`'s mutations;
- `go tool whim test --rust` (80 cases, the control -- `" INSERT"` changed in
  the generated `editor.rs` -- seen by 76) and `--wide --rust` (240);
- `make whim-editor-check` after the regenerated `editor.rs` is committed,
  `editor.go`, `Editor.java`, `editor.clj` and `Editor.hs` unmoved (every
  item here is the Rust printer's alone);
- whimsy's layout test (`go test ./whimsy`), and the heavy case's time and
  rustc's, before and after.

At `cb3f2db`: all 80 and all 240 as the C does, the control seen by 76 and by
94 keys and 6 pty cases; the heavy case C 438-444 ms, whimsy 115-119 ms
(0.3x).

## The shape of the output today

| | count |
| --- | ---: |
| functions | 1,713, every one `pub unsafe fn` of `ed: *mut Editor` (142 of them `_ed`: the body never names it); and 1 `new_editor`, 26 `init_globals` parts |
| lines | 62,836 |
| `(*ed).` | 22,262 reads and writes of the editor's objects |
| `(*p).m` through other pointers | 6,443 |
| `&raw mut` | 3,434: 1,850 of an editor object's place, 470 of a local passed to a call, 397 of a local array (`decay(&raw mut buf)`) |
| `wrapping_*` | 9,337: 4,517 `add`, 2,355 `offset`, 1,970 `sub`, 213 `mul`, 122 `neg`, 114 `div`, 40 `rem`, 6 shifts |
| by operand (the backend's count) | pointers about 4,000 (`p.wrapping_add(1)`, `.wrapping_offset(i as isize)`); signed integers 4,258 (1,167 `x++`/`x--`, 3,091 binary); unsigned 445 |
| casts `as` | 12,021; 569 a cast of a cast |
| locals | 3,213 declared at the top with a zero (`let mut c: i32 = 0;`), 1,356 at their first value (`sinkDecls`), 2 nested |
| block expressions | 682: 280 `x++` as a value, 105 `++x`, 174 an assignment's value, 123 a comma's (phase 181's results unpacked) |
| temporaries `t1`, `t2` ... | 528 declared |
| labels | 132: 30 `'cN` (a `for`'s continue reaching its step), 10 `'g_` (gotos), 31 `'lN` (a loop a `break` names), 13 `'sN` and 48 `'vN` (switches and their ladders) |
| `return` | 2,304; 875 the function's last statement |
| string literals | 1,812 `b"...\0".as_ptr() as *mut u8` |
| `core::mem::zeroed()` | 340; `unsafe { }` blocks: 1 (`new_editor`) |
| `#[allow]` | 1: `non_snake_case, non_camel_case_types, non_upper_case_globals, unused_assignments` |

**The effects, closed over the call graph** (the backend's analysis, item 2):
217 functions take no editor -- they name none of its objects, call no host
function and no function pointer, and call only functions like them; 57 of
those are also safe, dereferencing no raw pointer and calling only safe
functions (`musl_isdigit`, `utf_char2len`, `hex2nr`, `ascii_isupper`). What
the hand-written crate calls by name (`vim_main`, `deathtrap`, `emsg` and
five more: `Profile.RsExports`), what is used as a function pointer, and what
`match_lines`' runtime body names keep the whole signature.

## What a Rust programmer would find most jarring

Five excerpts, as tracked, and what the items below make of them.

**1. A pure predicate as an unsafe function of the editor it does not use**
(`musl_isdigit`, `musl_isalnum`):

```rust
pub unsafe fn musl_isdigit(ed: *mut Editor, c: i32) -> bool {
    return ascii_isdigit(ed, c);
}

pub unsafe fn musl_isalnum(ed: *mut Editor, c: i32) -> bool {
    return musl_isalpha(ed, c) || musl_isdigit(ed, c);
}
```

After items 1 and 2: `pub fn musl_isdigit(c: i32) -> bool { ascii_isdigit(c) }`.

**2. C's increments as blocks inside expressions, and their temporaries
zeroed at the top** (`musl_strncpy`):

```rust
pub unsafe fn musl_strncpy(_ed: *mut Editor, dest: *mut i8, mut src: *mut i8, mut n: usize_) -> *mut i8 {
    let mut t1: *mut i8 = null_mut();
    ...
    let mut t5: *mut i8 = null_mut();
    let mut d: *mut i8 = dest;
    while n != 0 && *src != 0 {
        t2 = { t1 = d; d = t1.wrapping_add(1); t1 };
        *t2 = *({ t3 = src; src = t3.wrapping_add(1); t3 });
        n = n.wrapping_sub(1);
    }
```

The C is `*d++ = *s++;`. After item 6: `*d = *src; d = d.wrapping_add(1);
src = src.wrapping_add(1);` -- each increment of a local nothing else can see
moved after the statement, and no temporary.

**3. Arithmetic that cannot overflow written as if it could** (`musl_atoi`):

```rust
n = 10i32.wrapping_mul(n).wrapping_sub((*({ t1 = s; s = t1.wrapping_add(1); t1 }) as i32).wrapping_sub(b'0' as i32));
```

`*s as i32 - 48` is a byte less 48: it cannot overflow an `i32`, and Rust's
`-` says so (item 3). `10 * n - ...` can -- musl's own atoi accumulates
negatively and overflows for an out-of-range input, which C leaves undefined
-- and Rust's plain operators say *that* too: a debug build panics, a release
build wraps as gcc's does (item 4).

**4. A zero for every local, overwritten before it is read** (`getvvcol`):

```rust
    let mut col: colnr_T = 0;
    let mut coladd: colnr_T = 0;
    let mut endadd: colnr_T = 0;
    let mut ptr: *mut char_u = null_mut();
    let mut c: i32 = 0;
    if virtual_active(ed) != 0 {
        getvcol(ed, wp, pos, &raw mut col, null_mut(), null_mut(), flags);
        coladd = (*pos).coladd;
        endadd = 0;
        ptr = ml_get_buf(ed, (*wp).w_buffer, (*pos).lnum, false);
```

rustc counts 2,131 of these zeros as never read. After item 5: `let coladd:
colnr_T = (*pos).coladd;` inside the `if`, where the C first gives it a value.

**5. Every function ends in `return`** (875 of them: clippy's
`needless_return`):

```rust
pub unsafe fn ga_grow(ed: *mut Editor, gap: *mut garray_T, n: i32) -> bool {
    if (*gap).ga_maxlen.wrapping_sub((*gap).ga_len) < n {
        return ga_grow_inner(ed, gap, n);
    }
    return true;
}
```

## The findings

### 0. rustc's lint, counted (a prerequisite)

- **The pattern.** The module's `#![allow(non_snake_case,
  non_camel_case_types, non_upper_case_globals, unused_assignments)]` hides
  every warning rustc would give, and the crate's `#![deny(warnings)]` sees
  only what is left; nothing counts what the allow hides. Measured on a copy
  with both taken out: **2,817 warnings**, 2,332 `unused_assignments` and 3
  `value passed ... is never read`, and 482 of the three naming lints -- 143
  fields, 122 locals and 42 functions not snake case (`VIsual_active`,
  `fname_casew`), 134 constants not upper case, 41 types not camel case
  (`garray_T`). The names are the C's and stay so, as every backend keeps them:
  a C name must stay a grep. The `unused_assignments` are the printer's.
- **What it would become.** `go tool whim whimsy --lint`: the crate checked
  (`cargo check`) with the module's allow taken out, rustc's warnings counted
  by lint -- caprice's `--lint` for rustc.
- **Where:** `whimsy/whimsy.go`. **Cost:** S. **Risk:** none.

### 1. The last statement's `return`

- **The pattern.** 875 functions end in `return x;`; Rust's style is the
  value as the block's tail (clippy's `needless_return`, warn by default).
- **What it would become.** `x` as the last line of the body; a `return;` at
  a void function's end dropped. Every other `return` stays: an early return
  is Rust's too.
- **Where:** the printer (`rs_fn.go`'s `function`). **Cost:** S. **Risk:**
  none: the tail's value is the return's. `TestRsControl`'s patterns name
  `return` and move with it.

### 2. Safe functions, and the editor where it is used

- **The pattern.** Every function is `pub unsafe fn f(ed: *mut Editor,
  ...)`. 217 take no editor even through what they call, and 57 of them do
  nothing unsafe (the effects closure above); 397 more name `ed` only to
  hand it on.
- **What it would become.** A function that needs no editor does not take
  one, and its calls pass none; one that does nothing unsafe is a safe `pub
  fn`. Rust checks the second claim: a safe function that dereferences a raw
  pointer, or calls an unsafe function, does not compile. HASKELL-IDIOMS.md's
  item 5 made the same 56 functions pure and the same 221 editor-free.
- **Where:** an effects analysis over the C (`rs_fx.go`, the shared effects
  analysis, `effects.go`, on Rust's terms: unsafe where Rust says so, not
  where the Haskell touches memory) and the printer (signatures, calls).
  `Profile.RsExports` names what the hand-written crate calls. **Cost:** S.
  **Risk:** none.

### 3. Arithmetic that provably fits

- **The pattern.** Every `+ - * / %` and negation is a `wrapping_*` method,
  whatever its operands. Some cannot overflow: a byte widened and less a
  character (`*s as i32 - 48`), a value masked (`c & 0x7f`), a remainder by a
  constant; and **unsigned division and remainder never overflow at all**
  (`wrapping_div` of a `u64` is `/`). Measured: 151 sites, 73 of them
  unsigned division.
- **What it would become.** The plain operator where the result's range,
  computed from its operands', fits its type -- a constant is itself, a
  conversion that keeps every value keeps the range, a `u8` is 0..=255, a
  bool converted 0..=1, `&` with a value not negative bounds it, `%` by a
  constant bounds it. Nothing is followed through a variable.
- **Where:** the printer (`rs_range.go`, `rs_expr.go`'s `binop` and `conv`).
  **Cost:** S. **Risk:** low, and visible: a wrong proof is an overflow that
  panics in the foreign tests (compiled at opt-level 0, overflow checks on)
  and in the suite's overflow-checked build (item 4).
- **Value:** low alone (151 of 9,337), but it is what the next item builds on.

### 4. Signed arithmetic as C means it

- **The pattern.** 4,258 operations on signed integers are `wrapping_*`
  (1,167 `x++`/`x--`, 1,111 `+` and 984 `-` on `i32`, 443 and 353 on `i64`,
  129 `*`). C does not define a signed overflow: a correct program never
  makes one, and gcc at -O0 wraps. Rust's plain `+` says exactly that: an
  overflow is a bug, which a build with overflow checks reports by
  panicking and a build without (the release profile's default) wraps.
- **Measured.** The printer writing every signed `+ - *`, negation and
  increment as Rust's plain operator, and the suites run on a build compiled
  **with** overflow checks (`CARGO_PROFILE_RELEASE_OVERFLOW_CHECKS=true`, the
  panic strings checked in the binary): **all 80 and all 240 cases as the C
  does, and the heavy case** -- no signed overflow in any path the suites
  reach -- at the same time, 0.3x the C's even with the checks. `wrapping_add`
  4,484 -> 1,733.
- **What it would become.** Plain operators for signed `+ - *`, negation and
  increments; `wrapping_*` left exactly where C's arithmetic is modular
  (unsigned) and where a pointer may stray, so that it says something. The
  release profile pins `overflow-checks = false`, so the program is gcc's
  -O0 whatever the defaults, and a check build is one variable away.
  Signed division stays `wrapping_div` where `MIN / -1` is possible: Rust's
  `/` panics there even without checks.
- **Where:** the printer (`binop`, `incDec`, `unary`) and `whimsy/Cargo.toml`.
  **Cost:** S. **Risk:** the honest one of this survey: a debug build of the
  crate (or the foreign tests at opt-level 0) panics where C's behaviour is
  undefined and gcc's wraps -- in a path the suites do not reach, a debug
  whimsy could stop where the release one goes on. That is the Rust idiom's
  meaning of C's undefined behaviour, and the release build -- the one the
  suite, the heavy case and `bin/whimsy` run -- is unchanged in behaviour.
- **Value:** high: the largest single change of how the module reads.

### 5. Locals declared where they are first given a value

- **The pattern.** 3,213 locals are declared at the function's top with a
  zero (`let mut col: colnr_T = 0;`) -- the C at -O0 keeps a local's value
  across a loop's iterations, and a goto's labeled block must not end a
  scope, so the backend's rule was the safe one -- and only those first
  assigned at the function's own level are declared there (1,356, `sinkDecls`).
  2,131 of the zeros are never read (rustc).
- **What it would become.** A local whose first mention is an assignment
  statement of some block, all of whose mentions are inside that block, is
  declared there: `let coladd: colnr_T = (*pos).coladd;` inside the `if`.
  Inside a loop's body that is a fresh variable each iteration, which is
  right exactly because the assignment comes first: no iteration can read
  the one before's. rustc checks every one -- a read it cannot see
  initialized, or a name used outside its block, does not compile.
- **Where:** the printer (`rs_fn.go`'s `sinkDecls`, on the printed lines,
  as it is). **Cost:** S-M. **Risk:** none to behaviour: names are unique in
  a function, so nothing is shadowed, and rustc refuses what it cannot see.
- **Value:** high: it is on every screen, and it takes the `unused_assignments`
  count toward 0.

### 6. Side effects out of expressions

- **The pattern.** 682 block expressions: C's `x++`, `++x`, an assignment
  used as a value, a comma's left operands. 528 temporaries `tN` exist for
  them, each zeroed at the top.
- **What it would become.** An expression statement whose increments are of
  locals nothing else can see (no address taken), each named once in the
  statement and none in a branch C may not evaluate (`&&`, `||`, `?:`), is
  the statement with each `x++` read as `x`, followed by the increments --
  `*d = *src; d = d.wrapping_add(1); src = src.wrapping_add(1);` -- and a
  `++x` the increment before it. The same for a condition, a return's value
  and an initializer, where the side effect is the first thing evaluated:
  hoisted before the statement. A `while` whose condition does something is
  a `loop` that does it, then breaks when the condition is false.
- **Where:** the printer (`rs_fn.go`'s statements, `rs_expr.go`).
  **Cost:** M. **Risk:** low: the rules keep C's order where it can be
  observed -- a local nothing else can see cannot be observed by a call, and
  a side effect evaluated first is evaluated first either way.

### 7. Pointer parameters as references

- **The pattern.** 1,261 raw pointer parameters (485 of them bytes: C
  strings). 470 times a local's address is passed (`&raw mut pos`).
- **Measured.** A function is *memory-local* when it takes no editor, every
  pointer it dereferences is a parameter it only reaches through (`p->m`,
  `*p`) -- never compares, steps, stores, casts or tests for null -- and
  every function it calls is memory-local; then while it runs, memory is
  touched only through those parameters, and they can be references:
  `&mut` if written through, `&` if not, with no two references of a
  function overlapping where one writes, and every call passing `&x` of an
  lvalue or a reference of the caller's. The fixpoint finds **9 functions**,
  each with one `&mut` (`ga_init`, `ga_init2`, `ga_clear`,
  `ga_clear_strings`, `ExpandCleanup`, `correct_range`, ...), and 31 calls.
  Every other pointer parameter belongs to a function that reaches the
  editor, so that a reference would be a promise the program could break.
- **What it would become.** `pub fn ga_init(gap: &mut garray_T)`, `gap.ga_len
  = 0;`, called `ga_init(&mut (*buf).b_ga)`.
- **Where:** an analysis (`rs_refs.go`) and the printer. **Cost:** M.
  **Risk:** low: the proof is the fixpoint's; rustc checks the types.
- **Value:** low: 9 functions. It is the most a reference can be proved for
  without a different memory model.

### 8. A `for`'s `continue`

- **The pattern.** 30 loops wrap their body in a labeled block, `'c3: {
  ... break 'c3; ... }`, so that a `continue` reaches the `for`'s step.
- **What it would become.** The step written before each `continue`, which is
  what a Rust programmer writes, where the step is one statement and the
  body's `continue`s are few.
- **Where:** the printer (`rs_fn.go`'s `loopBody`). **Cost:** S. **Risk:**
  none: the step runs once on each path to the next iteration either way.

### 9. C strings as C-string literals

- **The pattern.** 1,812 string literals are `b"...\0".as_ptr() as *mut u8`:
  a byte string with its NUL written in.
- **What it would become.** `c"...".as_ptr() as *mut u8`: Rust's C-string
  literal (stable since 1.77), the NUL implied, where the C string has no
  NUL of its own inside. A literal that initializes a char array
  (`str_u8::<N>(b"...")`) stays a byte string.
- **Where:** the printer, and the suite's control string
  (`internal/suite/rust.go`). **Cost:** S. **Risk:** none: the same bytes.

### 10. Read-only pointers as `*const`

- **The pattern.** Every pointer is `*mut T`, whatever the C's `const`, and
  the core has almost none (70 `const` in 75,721 lines: the canonical
  printer and vim's own C keep few).
- **What it would become.** `*const T` for a pointer never written through.
  With no `const` in the C to follow, it is an inference over every pointer
  variable, parameter and member -- where each one's value flows -- or a cast
  at every flow into a `*mut`, which adds noise to remove it.
- **Cost:** M-L. **Risk:** low (rustc checks a write through `*const`).
  **Value:** low-medium.
- **Done** (`rs_const.go`): the inference, not the casts. A pointer *slot*
  -- a parameter, a local, an object of the editor, a struct's member, a
  function's result -- is `*const` unless a place reached through its value
  is written, incremented or has its address taken, its own address is
  taken, or its value flows (assigned, initializing, passed, returned)
  into a slot that is `*mut` or into a place that is no slot (an element of
  an array of pointers, a pointer's pointee, a host function's or a
  function pointer's parameter, an integer): the least set of `*mut` slots,
  closed backwards over the flows. A value's slots are its *heads*, through
  casts, pointer arithmetic, increments, an assignment's value, the comma
  and both arms of `?:`; a `*mut` value goes into a `*const` slot as Rust
  coerces it, and nothing casts a `*const` to a `*mut` -- so rustc checks
  every claim: a write through a `*const`, or a `*const` where a `*mut` is
  wanted, does not compile. Measured: **869 of 2,770 pointer slots are
  `*const`** -- 448 of 1,317 parameters, 303 of 960 locals, 45 of 222
  results, 58 of 193 members, 15 of 78 objects (`fn
  ga_concat(ed: *mut Editor, gap: *mut garray_T, s: *const char_u)`, `pub xp_pattern: *const
  char_u`). Of the 1,901 left `*mut`, by the first reason found: 600 written
  through, 902 flowing into a slot that is, 244 fixed from outside (what
  the crate calls by name, a function pointer's signature, a typedef's
  alias, a reference), 81 flowing into a place that is no slot, 79 whose
  address is taken. Two rules beside the inference, each measured: a
  variadic argument is read -- the one variadic callee is vim's printf,
  which has no `%n` -- so `VArg::P` holds a `*const c_void` (+110 slots);
  an array reached through a `*const` is `decay_const(&raw const (*p).a)`,
  where it decayed through `&raw mut` before (+11). Declined: the host's
  read-only parameters (`host_write`'s, `host_message`'s, printf's
  format), +9 slots for a profile list and three hand-written signatures. A
  `*mut` compared with a `*const` is written `*const` first, the
  comparison turned round (Rust coerces only the right operand); `null()`
  for a `*const`'s zero. rustc 38.7-39.9 s; the heavy case 0.25-0.3x.

### 11. Compound assignment

- **The pattern.** `x = x + n` where Rust says `x += n` (clippy's
  `assign_op_pattern`): only after items 3-4 is there a plain operator to
  combine.
- **What it would become.** `place op= value;` for a compound assignment and
  an increment whose operation is plain: `n += 1`, `len -= 2`.
- **Where:** the printer (`assigned`, `incDecStmt`). **Cost:** S. **Risk:**
  none.

## Ranking (value against cost)

| Rank | Item | Who | Cost | Risk | Idiom gained |
| --- | --- | --- | --- | --- | --- |
| 1 | 0: rustc's lint counted -- **done**, `82b6f5d` | `whimsy.go` | S | none | the measure: 2,335 of the printer's warnings hidden |
| 2 | 1: no `return` at the end -- **done**, `7899b3c` | printer | S | none | 875 functions |
| 3 | 2: safe functions; the editor where used -- **done**, `7899b3c` | analysis + printer | S | none | 57 safe, 217 without the editor |
| 4 | 5: declarations at their first value -- **done**, `eb69bc1`, `8e8cf7a` | printer | S-M | none (rustc checks) | 3,213 zeros at the top; 2,131 `unused_assignments` |
| 5 | 3 + 4: arithmetic as C means it -- **done**, `5614549` | printer + `Cargo.toml` | S | debug builds panic on C's UB | 4,258 signed `wrapping_*`, 151 provable |
| 6 | 11: compound assignment -- **done**, `5614549` | printer | S | none | after 4 |
| 7 | 6: side effects out of expressions -- **done**, `2946815` | printer | M | low | 682 blocks, 528 temporaries |
| 8 | 9: C-string literals -- **done**, `4b0035d` | printer + suite | S | none | 1,812 literals |
| 9 | 8: a `for`'s `continue` -- **done**, `4b0035d` | printer | S | none | 30 labeled blocks |
| 10 | 7: references -- **done**, `9619d4c` | analysis + printer | M | low | 9 functions |
| 11 | 10: `*const` -- **done**, see *The second pass* | inference | M-L | low (rustc checks) | 869 of 2,770 pointer slots |
| -- | `&mut Editor`, owned data, slices, Rust enums, `.add()`, the 2024 edition | -- | -- | -- | declined (below) |

### Done: items 0-9 and 11 (2026-10-02)

All but item 10, in the Rust printer (`crefactor/togo`: `rs_fx.go` the
effects, `rs_range.go` the ranges, `rs_hoist.go` the side effects,
`rs_refs.go` the references, `rs_defer.go` the declarations with no value,
and `rs_expr.go`, `rs_fn.go`, `rs_init.go`), `whimsy/Cargo.toml` and
`whimsy.go`; each with a C program of its own among the foreign tests
(`TestRsArith`, `TestRsHoist`, `TestRsRefs`, `TestRsDefer`), compiled at
opt-level 0 -- where an overflow panics -- and held to gcc's output. Every
commit was held to the whole recipe above: the crate with
`#![deny(warnings)]`, `go test ./togo`, all 80 and all 240 cases as the C
does with the control seen, `go test ./whimsy`, `whim gen --check`.

| | `cb3f2db` | after |
| --- | ---: | ---: |
| `editor.rs` | 62,836 lines | 61,820 lines |
| `pub unsafe fn` / `pub fn` | 1,714 / 1 | 1,648 / 67 (66 safe functions of the core and `new_editor`) |
| functions taking the editor | 1,714 (142 `_ed`) | 1,497 (7 `_ed`, all kept whole: function pointers, the hand-written crate's) |
| reference parameters | 0 | 9 functions, `&mut` each |
| `wrapping_*` | 9,337 | **4,798**: 2,304 `offset` and most of 2,136 `add` on pointers, the rest unsigned |
| `x op= v` | 0 | 2,006 |
| a body's last `return` | 875 | 0 |
| block expressions (`({ `, `= { `) | 665 | 153, of which 5 init_globals' compound literals |
| temporaries `tN` | 528 | 50 |
| locals zeroed at the top | 3,213 | 1,098; 1,336 declared in a nested block, 396 with no value |
| labels | 132 | 91 (`'cN` 30 -> 2, loops named 31 -> 18) |
| string literals | 1,812 `b"...\0"` | 1,833 `c"..."`, 214 `b"..."` (a NUL inside, a byte past ASCII, a char array's) |
| rustc's warnings (`whim whimsy --lint`) | 2,817: 2,335 `unused_assignments` | 702: **220** `unused_assignments`, 482 the C's names |
| rustc, release | 39.4 s, 0.55 GB | 38.0-38.3 s, 0.53 GB |
| the heavy case | 0.3x the C (115-119 ms) | 0.25-0.3x (112-140 ms against 430-478) |

- **Items 3, 4, 11** (`5614549`). Measured on the result, as on the survey's
  experiment: both suites on a build with overflow checks
  (`CARGO_PROFILE_RELEASE_OVERFLOW_CHECKS=true`) answer every case as the C
  does, the heavy case included. Signed division and remainder are `/`
  and `%` too: `MIN / -1`, which gcc's `idiv` traps on, panics either way.
  What C defines stays `wrapping_*`: unsigned arithmetic, a narrow type's
  increment (`signed char` is promoted, then converted back), a compound
  assignment computed in a wider type (`int x; x += 1LL`).
- **Items 1, 2** (`7899b3c`). `Profile.RsExports` names what `host.rs` and
  `printf.rs` call by name (`vim_main`, `deathtrap`, `emsg` and five more).
- **Item 5** (`eb69bc1`, `8e8cf7a`) is two rules: a local declared in the
  block where it is first given a value, and -- for one first given a value
  in both arms of an `if`, every case of a switch, or before a loop -- a
  declaration with no value, `let x: i32;`, where a walk of C's statements
  in Rust's order sees every read after a store (`rs_defer.go`: rustc's
  definite-initialization analysis, path-insensitive, a loop's body walked
  twice for `mut`). rustc checks both: its first version said `mut` for a
  store whose path always returned, and rustc said so. The 220 warnings
  left are the temporaries' zeros, the C's own initializers never read, and
  loops the walk is too conservative for: the module keeps its `#[allow]`.
- **Item 6** (`2946815`): also an assignment or a comma first evaluated by a
  condition, a return, an initializer or a plain assignment's value -- which
  takes phase 181's results out of their blocks, `let mut o: pr = two(3);
  let mut a2: i32 = o.a; let r2: i32 = o.r;`. Found by the suite: the first
  version hoisted the comma of a statement's own `(void)(o = f(), p =
  o.x)` and then wrote it again -- every case moved; bisected to
  `match_keyprotocol` by building the module with one function at a time
  from either side, and held by `TestRsHoist` since.
- **Item 7** (`9619d4c`): found on the way, in `TestRsPointers`, `return
  &wp->o.so` through what would have been a `&` -- a raw pointer into a
  reference, outliving it -- refused, and held by the test.
- **Items 8, 9** (`4b0035d`): c"" cannot say a byte past ASCII as `\x80`
  (rustc refuses it), so such a string stays a byte string.

The survey's excerpts, as printed now:

```rust
pub fn musl_isdigit(c: i32) -> bool {
    ascii_isdigit(c)
}

pub unsafe fn musl_strncpy(dest: *mut i8, mut src: *mut i8, mut n: usize_) -> *mut i8 {
    let mut d: *mut i8 = dest;
    while n != 0 && *src != 0 {
        *d = *src;
        d = d.wrapping_add(1);
        src = src.wrapping_add(1);
        n = n.wrapping_sub(1);
    }
    ...

    while musl_isdigit(*s as i32) {
        n = 10 * n - (*s as i32 - b'0' as i32);
        s = s.wrapping_add(1);
    }
    if neg != 0 { n } else { -n }

pub fn ga_init(gap: &mut garray_T) {
    gap.ga_data = null_mut();
    gap.ga_maxlen = 0;
    gap.ga_len = 0;
}
```

**What remains**, measured: 1,648 functions are `unsafe fn`, as the memory
model makes them -- every one dereferences a raw pointer or calls one that
does; 21,577 `(*ed).x`; 4,798 `wrapping_*`, pointers' and unsigned; 1,098
zeros at the top, of which rustc sees 175 never read; 153 blocks inside
expressions (increments of the editor's objects and of members, a local
named twice, a lazy operand); item 10 (done in *The second pass*, below).

## The second pass: item 10 and the residue (2026-10-02)

From `8a186a4`, each item a commit of its own, held to the whole recipe
above and more: the crate with `#![deny(warnings)]` (`go tool whim
whimsy`), `--lint`, `go test ./togo` with a foreign C program for each new
rule, `go test ./whimsy`, all 80 and all 240 cases with the control seen
-- on the release build and on one with overflow checks
(`CARGO_PROFILE_RELEASE_OVERFLOW_CHECKS=true`) -- the heavy case, `make
whim-editor-check`, gofmt, go vet and staticcheck. The counts are a
throwaway counter's over `editor.rs` (`.tmp/rsc/count.sh`): `*const` and
`wrapping_` occurrences, zeros a line `let x: T = <zero>;` at a
function's own level, block expressions `({ ` and `= { `, labels.

| | `8a186a4` | after |
| --- | ---: | ---: |
| `editor.rs` | 61,820 lines | 61,719 lines |
| pointer slots `*const` | 0 of 2,770 | 869 (448 parameters, 303 locals, 45 results, 58 members, 15 objects) |
| `*const` in the text | 0 | 1,492 |
| `pub unsafe fn` | 1,648 | 1,648 |
| `wrapping_*` | 4,822 | 4,822 |
| zeros at the top | 1,113 | 932, each one rustc needs (446 `0`, 242 `zeroed()`, 138 `false`, 104 null) |
| locals declared with no value, `let x: T;` | 396 | 499 |
| the module's `#![allow]` | the C's names, `unused_assignments` | the C's names alone; 4 functions `#[expect(unused_assignments)]` |
| block expressions | 154 | 154 |
| labels | 91 | 91 |
| rustc's warnings (`--lint`, every allow and expect out) | 702: 220 `unused_assignments` | 487: 5 `unused_assignments`, 482 the C's names |
| rustc, release | 38.0-38.3 s | 38.7-44.9 s, 0.53-0.54 GB |
| the heavy case | 0.25-0.3x | 0.25-0.3x (112-139 ms against 435-473; twice 154-168 ms, 0.35-0.4x, under load) |

- **Item 10** (`rs_const.go`, `TestRsConst`): above, under the item.
- **Item 12, temporaries where they are given their value** (`rs_fn.go`'s
  `letTemp`, `TestRsTemps`). The 50 temporaries left after item 6 -- an
  increment's old value, a compound assignment's right side that calls,
  an lvalue's address taken once -- were zeroed at the top and stored
  where used; each holds one value, used after it in the same block, so
  each is `let t1: T = v;` there, a block expression's own let where it is
  in one: `(*buf).b_fnum = { let t1: i32 = (*ed).top_file_num;
  (*ed).top_file_num = t1 + 1; t1 };`. A compound literal's variable keeps
  the function's scope (its address outlives the block). Zeros at the top
  1,113 -> 1,063, `unused_assignments` 220 -> 170, 50 lines.
- **Item 13, every zero rustc does not need, and the module's
  `unused_assignments` gone** (`rs_defer.go`, `TestRsLive`). The question
  was whether all 1,063 zeros could go; measured, 932 cannot -- rustc's own
  definite-initialization analysis refuses `let x: T;` for each, since
  some path (feasible or not) reads before a store, or the struct or array
  is filled member by member -- and the 170 `unused_assignments` that
  could, did. The walk of item 5 became as precise as rustc's where it
  was not: a condition's true and false ways (the right of `&&` only
  where the left is true), a loop with no condition left only by its
  breaks, a do's body run once at least, a goto's state joined at its
  label, and structs, arrays and locals whose address is taken followed
  too (an address, a member written, an array reached are reads that need
  `mut`). Then C's own dead stores, which rustc counted: the same walk
  following one store's value finds 70 initializers nothing reads (`int i
  = 0;` then `i = ...`), 15 statements whose value nothing reads, and 3
  parameters assigned before any read (`fn vim_str2nr(..., _len: i32,
  ...)`, `let mut len: i32;`); each is not written, a store's value's
  effects alone are. Five stores to locals whose address is taken (`stat`,
  `vcol`) are dead as rustc sees them, which does not follow a pointer:
  they are kept, and their 4 functions say `#[expect(unused_assignments)]`,
  which rustc holds to its count both ways. The module's `#![allow]` is
  the C's names alone; rustc checks every claim -- a read it cannot see
  initialized, a `mut` missing or not needed, an expectation unmet does
  not compile -- but for a dropped store, whose proof is the walk's and
  the suites'. Zeros 1,063 -> 932, `let x: T;` 396 -> 499,
  `unused_assignments` 170 -> 5 (expected), 51 lines. Found on the way: a
  do-while whose body breaks never had its condition's reads checked.

## What is not worth doing, and why

- **`ed: &mut Editor`.** It would turn 22,262 `(*ed).x` into `ed.x` and most
  of the module's `unsafe` with it -- and it is a promise the program breaks.
  1,850 addresses of the editor's objects are taken (`&raw mut (*ed).x`),
  kept in tables (the options' variables, `&p_wiv`) and in structs, and
  written through while the editor is in use; `match_lines` hands the editor
  to every core's thread. With `&mut`, rustc tells LLVM that nothing else
  reaches the editor (`noalias`), and a write through one of those
  addresses is undefined behaviour that opt-level 2 is free to miscompile.
  It needs the objects out of one struct, or a different memory model.
- **Owned data and safe structs.** 6,443 accesses through pointers other
  code holds (`(*(*ed).curwin).w_cursor`, the buffers' lines): a `Box` or a
  `Vec` per object would make every one a borrow the checker must see, which
  C's pointers into each other's objects (a window's buffer, a buffer's
  windows) do not allow. It is the translation the Go and Java backends'
  pointer-class analyses approximate, at their cost and speed; whimsy is the
  translation that does not, and is fastest for it.
- **C strings as `&[u8]` or `String`.** 485 byte-pointer parameters, 2,355
  `wrapping_offset`s: a C string is walked, subtracted and written in place,
  and its length is where its NUL is. GO-IDIOMS' *Declined: the C strings as
  Go slices* holds as it stands.
- **Rust `enum`s for C's enumerations.** A C enum holds values outside its
  enumerators, and the core stores and compares them as integers; a Rust
  enum would make every load a conversion that must check. The constants
  have their names already (`ESC =>`, `K_DEL`).
- **`p.add(n)` for `p.wrapping_add(n)`.** `add` is undefined behaviour when
  the result leaves the object, and C at -O0 tolerates pointers one before
  an array that gcc never folds; without Miri on the machine to look, the
  wrapping form -- defined wherever it points -- stays.
- **The 2024 edition.** Its `unsafe_op_in_unsafe_fn` requires an `unsafe { }`
  around every unsafe operation inside an `unsafe fn`: in this module,
  around nearly every statement. The narrower `unsafe` that idiom asks for
  needs the safe functions first (item 2), and there are few.
- **`Option<&T>` for a nullable pointer, `Result` for FAIL.** The pointers
  that are tested for null are also walked; FAIL carries nothing (phase 166
  made the yes-or-no functions `bool`).

## The throwaway files

Under the worktree's `.tmp/rsid/`, none tracked: `core.c` (the cut core),
`base.rs` (regenerated, byte-identical), `scan/` (the text counter), `ua/`
(the copy `cargo check`ed without the allow), `exp-signed-*.log` (the suites
with signed arithmetic plain and overflow checks on), `base-*.log` (the
suites at `cb3f2db`).
