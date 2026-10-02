# whimsy: the editor in Rust

**Being built** (2026-10-02): whimsy is the fifth translation of the core,
beside the Go (`editor/`), the Java (`braaam/`), the Clojure (`vijure/`) and
the Haskell (`caprice/`), and held to the same test: `whim test --rust`. It
was planned here on 2026-09-26 and not scheduled; the plan held, with two
changes (a hand-written host, and the parallel `:%s` from the start), and
what was built is recorded below, milestone by milestone.

```
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
  away of it is free), `*mut c_void` for `void *`; walked with
  `wrapping_add`/`wrapping_offset`, subtracted by `pdiff` (the runtime's, in
  elements), compared with `<` and `==`, null-tested with `is_null()`.
  **No Rust reference to a C object is ever made**: a member is
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
- **Every function is `pub unsafe fn name(ed: *mut Editor, ...)`**, the
  editor named `_ed` where the body does not use it; a parameter the body
  never names is `_name`, one it assigns `mut`.
- **Every local is declared at the function's top, zeroed** -- C at -O0 keeps
  a local's value from one iteration of a loop to the next, which a Rust
  declaration in the body would not, and a goto's labeled block must not end
  a local's scope -- and given its initializer where C declares it. C's
  nested scopes are one Rust scope: a local takes a name of its own
  (`i_2`), and never a constant's or a function's, which a `let` would read
  as a pattern.

## Expressions

- **C's arithmetic is exact** (`rs_expr.go`): the integer types are Rust's of
  C's widths (`char` `i8`, `long` `i64`), a C `bool` a `bool`; every
  operation that can overflow is `wrapping_*` (so no overflow panic, and no
  difference between debug and release: signed overflow wraps, as gcc's -O0
  does), division `wrapping_div`/`wrapping_rem`, a shift by a constant `<<`,
  by a variable `wrapping_shl`; C's usual arithmetic conversions are decided
  as the other backends decide them (`usualK`) and said with `as`. `+= -= *=
  &= |= ^=` are computed in the left side's own type, whose low bits they
  are; the rest in the usual type and converted back. A compound assignment
  whose right side calls computes it first, then reads the left: gcc's order.
- **What C does inside an expression is a Rust block**, which is an
  expression: `x++` as a value is `{ t1 = x; x = t1.wrapping_add(1); t1 }`,
  an assignment's value `{ x = v; x }`, the comma `{ a; b }` -- in C's
  order, left to right, as the Java's and the lowered form's. As a
  statement each is a statement: `x = x.wrapping_add(1);`. An lvalue whose
  evaluation does something is read and written through its address, taken
  once.
- **A condition is a Rust `bool`**: `x != 0`, `!p.is_null()`, `f.is_some()`;
  a comparison, `&&`, `||` and `!` are `bool`s, made an `int` (`as i32`)
  only where C uses one as a number.
- **A constant is spelled as C spells it** where it can be: an enumerator
  its constant (`pub const K_DEL: i32 = ...`, each of its own type), a
  character `b'a'`, else its value, a literal of the place's type.
- **A call of a function declared and not defined is the host's**:
  `crate::host::host_write(ed, ...)`; a variadic one (`vim_snprintf`) takes
  its variadic arguments as a slice, `&[VArg::I(n as i64), VArg::P(s as *mut
  c_void)]` -- a C variadic argument after the default promotions, signed,
  unsigned or a pointer.

## Control flow

- **C's own** (`rs_fn.go`): `if`, `while` (`loop` for a constant condition),
  `return`, `break` and `continue` as they are; a `for`'s step and a
  `do`'s condition run after the body, which a `continue` reaches by leaving
  a labeled block around it (`'c3: { ... break 'c3; ... }`); a `do { ... }
  while (0)` -- phase 173's goto regions -- is a labeled block its breaks
  leave. A `break` or `continue` says its loop's label where a labeled block
  stands between it and its loop, as Rust requires.
- **A goto is a labeled block** that ends at its label, the goto `break
  'g_label;` -- the Java backend's rule (`java_stmt.go`): every goto left in
  the core is a forward jump to a label of a block that holds it.
- **A switch is a `match`** when no case falls into the next, each arm a
  case's statements, the `break` that ends one dropped; one that falls
  through is **a ladder of labeled blocks**, the match at its heart breaking
  to the block whose end its case's statements follow, so that falling
  through is going on:

  ```
  's: { 'c1: { 'c0: { match x { 1 => break 'c0, 2 => break 'c1, _ => break 's } }
              case 1's statements }
        case 2's statements }
  ```

  This is C's shape kept; the plan's lowered form for these is not needed.
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

## Risks, named in advance

- **Undefined behaviour of Rust's own.** Raw pointers only, never a reference
  to a C object, and `wrapping_*` pointer arithmetic, which is defined
  wherever it points; Miri, which could check the rest, is not on the
  machine.
- **Signed overflow** is undefined in C and wraps under gcc `-O0`, which is
  what the suite measures: `wrapping_*` everywhere keeps that, deliberately.
- **Readability**: `unsafe` Rust in C's shape. The price of a faithful
  translation, as for the others.
