# The editor in Rust: a plan

**A preliminary plan, not scheduled** (2026-09-26): there is no intention to
translate the editor to Rust. It is kept as what was measured and how it
would be done, should that change.

Written 2026-09-26, when the core's C was translated to Go, Java and Clojure,
each answering every case of `whim test` as the C does, and a Haskell editor
was planned (`doc/HASKELL.md`). A Rust editor is held to the same test:
`whim test --rust`.

## What is on the machine

`rustc` and `cargo` 1.98.1; no crate is cached, and none is needed: the editor
builds from `std` alone, with the few libc calls the terminal needs declared
`extern "C"` (std links libc on Linux), so the build stays offline.

Measured: a synthetic library crate shaped as the output will be -- 2,000
functions of raw-pointer reads, writes and wrapping arithmetic, 26,003 lines
-- compiles in **2.7 s at opt-level 0 and 1.7 s at opt-level 2, about 0.3 GB
at the peak**: a tenth of GHC's time on the same shape (`doc/HASKELL.md`).
The core's size is not a risk here.

## What Rust has, and has not

Rust is the target closest to C of the four so far:

| C | Rust |
|---|---|
| struct and union layout | `#[repr(C)] struct` and `#[repr(C)] union`: the C's layout, checked with `size_of` and `offset_of!` |
| a pointer that walks, is compared, subtracted, cast | a raw pointer, `*mut T`: `.add`, `.offset`, `.offset_from`, `<`, `as` -- C's arithmetic exactly |
| a function pointer in a table | `Option<unsafe fn(...)>`, stored in the struct as it is |
| `return`, `break`, `continue` | the same |
| the 49 forward gotos | labeled blocks, `'l: { ... break 'l; }`, as the Java backend writes them |
| a `switch` | `match` -- **without fall-through** (10 functions fall through) |
| `x++`, `a = b` as a value, the comma operator | **not expressions** in Rust -- as in Go |
| unsigned wrapping, `(uint8_t)x` | `wrapping_add` and kin, `as` -- C's conversions |
| a C variadic function | **not definable** in stable Rust |

So the translation keeps C's memory model, as Haskell's plan does -- but
natively, without offsets computed by hand -- and C's control flow almost as
it is.

## The approach

- **Memory:** every C struct and union a `#[repr(C)]` type; a pointer `*mut
  T` / `*const T`; never a Rust reference to a C object (`&`/`&mut` would
  bring aliasing rules C does not have): fields are reached through raw
  pointers (`addr_of_mut!((*p).f)`). No pointer-class analysis, no `BytePtr`,
  no union handling -- the Go, Java and Clojure backends' hardest parts.
- **The state:** the file-scope objects as the fields of one `#[repr(C)]
  Editor`, boxed and never moved, so pointers into it stay good; every
  function `unsafe fn name(ed: *mut Editor, ...)`. Embeddable as the Go is:
  a process holds as many editors as it makes.
- **Expressions:** side effects taken out of expressions into statements,
  as the Go emitter already does (Go also has no `x++` or assignment as a
  value); its structure is the model for the printer.
- **Control flow:** C's own, with the Java backend's labeled blocks for the
  gotos; the 10 functions that fall through a `switch` go through the
  Clojure backend's lowered form (basic blocks), printed as a `loop`/`match`
  on the block number.
- **Arithmetic:** exact widths (`u8` ... `i64`), `wrapping_*` for every C
  arithmetic operation (so no overflow panic and no difference between debug
  and release), `as` for C's conversions, C's usual arithmetic conversions
  made explicit as the other backends decide them.
- **The host, translated too.** With C's memory model, the host half of
  `whim-vim.c` -- `vim_snprintf`, the terminal, the signals, the arena -- is C
  the same backend can translate, its libc calls becoming `extern "C"`
  declarations: no port of `format.go` by hand, and a signal handler is a
  real one (`sigaction`, an atomic flag and a wake-up pipe), as in the C. By
  hand: the libc declarations and the launcher. `vim_snprintf`'s variadic
  arguments become a slice of an argument enum at its call sites and in its
  translated body.

This is the Rust a C-to-Rust translator writes -- `unsafe` throughout, C's
shape kept -- as c2rust does, which is not on the machine. Idiomatic Rust
(ownership, slices, `String`) would be a later survey, as `GO-IDIOMS.md` was
for the Go.

## Milestones

Each verified before the next.

1. **A slice on foreign C**: the `#[repr(C)]` types and a layout test against
   gcc's `offsetof` for every struct; the printer for the slice the other
   backends' first milestones covered; foreign C programs translated,
   compiled with `rustc`, and required to print what gcc's build prints,
   with a control. A coverage report on `editor.c`.
2. **Every function of the core**, and the host translated, compiling with
   no warning that `#![deny(...)]` would make an error.
3. **The launcher and the suite**: `bin/whim-rs`, and `whim test --rust` /
   `--wide --rust` answering the 45 and 240 cases as the C does, with the
   Rust editor's own control.
4. **Kept current**: the generated crate tracked, written by `whim gen`, and
   refused by `whim-editor-check` when stale.

## Risks, named in advance

- **Undefined behaviour of Rust's own.** Raw pointers only, never a reference
  to a C object; Miri, which could check it, is not on the machine.
- **Signed overflow** is undefined in C and wraps under gcc `-O0`, which is
  what the suite measures: `wrapping_*` everywhere keeps that, deliberately.
- **Readability**: `unsafe` Rust in C's shape. The price of a faithful
  translation, as for the others.
