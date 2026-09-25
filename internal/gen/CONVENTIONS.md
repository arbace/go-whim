# Transpiling editor.c to Go: the conventions

> **These are now the rules of a program.** `internal/gen -editor` applies them to
> write `editor/editor.go` whole (`go tool whim gen`); the text below is kept as their
> statement, and a rule the program applies differently is a bug in one or the
> other. It was written for the one parallel pass that produced the first
> `editor/editor.go` by hand. The per-file pieces it
> names (`types.go`, `globals.go`, the chunk files) and its checking script
> were folded into `editor/editor.go` afterwards; `internal/gen` regenerates the
> generated part and `internal/gen/sigs.md` is the signature list the pass used.

`editor.c` (the core of `whim-vim.c`, cut at its first `#include`) is being
transpiled **by hand** into the Go package `editor/` (`package editor`),
**faithfully**: every C function becomes one Go function with the same name,
the same parameters and the same control flow, so that the Go file can be read
against the C one line by line. Idiomatic Go comes later, in phases; this pass
must first be *right*. When in doubt, choose the translation that behaves
exactly as the C does.

## What the generator is told

The rules below are the generator's; the names they apply to are vim's, and
the generator -- `crefactor/togo`, which `internal/gen` runs -- is
told them, one value, `whim.Gen` (`internal/whim/gen.go`, of type
`togo.Profile`): the C functions `editor/crt.go` replaces, whose calls
are translated and whose bodies are not (`alloc*`, `musl_mem*`, `musl_str*`,
`ga_grow_inner`), `ga_grow_inner`'s body, which is a rule of the runtime's;
the allocators and `vim_free`; `musl_memmove`/`memcpy`/`memset`/`memcmp` as
the functions of bytes; `garray_T` and its `ga_data`; `usize` as sizeof's
type; the `varp` parameters that pun; `_` as `gettext_`; and `editor.go`'s
header. Nothing in `togo` names any of them; its test translates a small C
file told nothing at all. Three of them move
nothing in today's core, measured by dropping each: `vim_free` (phase 132
dropped its calls), the `varp` puns (no `char *` parameter is so named any
more) and `usize` (an alias of `uint64`, which the emitter writes the same).

## What is fixed, and must not be changed

- `editor/crt.go` — the C runtime: `Ptr[T]`, allocation, `mem*`, `qsort`,
  string literals, growarrays. Read it first; it is short.
- `editor/types.go` — every C type, generated. Structs keep their C field
  names. `struct tag` is `S_tag`; a typedef is an alias (`buf_T = S_file_buffer`).
  Every `enum` constant is an untyped Go `const` with its C name.
- `editor/globals.go` — every file-scope object with its Go type (no
  initializers), and every **block-scope `static`** hoisted to a global named
  `<function>_<name>` (e.g. `static int entered` in `deathtrap` is
  `deathtrap_entered`).
- `internal/gen/sigs.md` — **the Go signature of every function**. Copy yours
  verbatim; call everyone else's exactly as written there. A parameter's Go
  type was decided by a whole-file analysis you cannot redo from one chunk.

Never edit these files. If one is wrong for your chunk, write the best adapter
you can at the call site, and **list the problem in your final report** — it is
fixed centrally.

A C identifier that is a Go keyword or predeclared name gets a trailing `_`
(`type` → `type_`, `len` → `len_`, `string` → `string_`, `new` → `new_`,
`copy` → `copy_`, `max` → `max_`…). `_()` (vim's gettext identity) is
`gettext_()`. Apply the same rule to your locals.

## Types

| C | Go |
| --- | --- |
| `char`, `unsigned char`, `char_u` | `byte` |
| `signed char` | `int8` |
| `short` / `unsigned short` | `int16` / `uint16` |
| `int` / `unsigned` | `int32` / `uint32` |
| `long`, `long long` / unsigned | `int64` / `uint64` |
| `usize` | `usize` (= `uint64`) |
| an `enum` type | its underlying integer type |
| `char *`, `char_u *` (a string) | `Ptr[byte]` |
| `T *` for any other T | `*T` **or** `Ptr[T]` — as the skeleton says for fields, globals and parameters; for a local, `Ptr[T]` if the local is ever indexed, incremented, added to, subtracted, ordered with `<`, or assigned from something that is a `Ptr[T]`, else `*T`. Pointing into an array does not make a `Ptr` by itself: only a pointer into an array that is also handed on as a `void *` (`memmove`, `memset`, a callback's cookie) is one, since what receives it may walk it |
| `void *` | `any` |
| a function pointer | a Go `func` value; NULL is `nil` |
| `T a[N]` local | `Mk[T](N)` if it is ever used as a pointer (passed, walked, `strcpy`'d into — the usual case for `char buf[N]`), else `var a [N]T` |
| a struct local | `var s T`; `&s` is `&s` |

Keep C's integer widths. Go makes every conversion explicit: write them all
(`int32(x)`, `int64(n)`, `usize(len)`), and keep C's semantics — a C `int`
expression is `int32` arithmetic, which wraps as C's does in practice. A
character constant `'a'` is untyped in Go and fits wherever C used it.
Integer division and `%` truncate in both languages.

## Pointers that only walk forward: `[]T`

A class of pointers that walks, but only ever forward by constants -- or never
moves and is only indexed, so that its pointers are always at their array's
start, where a negative index is out of bounds in C too -- and is never
compared with another pointer, subtracted, ordered, stepped back, or had its own
address taken, is a Go slice:
`[]T`, or `[]byte` for a C string, its NUL kept at the end. `*p` is `p[0]`,
`p[k]` is `p[k]`, `p->x` is `p[0].x`, `p++` and `p += k` are `p = p[1:]` and
`p = p[k:]`, and NULL is `nil`. Excluded as well: a class that reaches the
hand-written runtime (the profile's `Runtime`), whose signatures are `Ptr`, and one that holds
the element of a `T **`, since the address of a `Ptr` arrives there and
`*Ptr[T]` is no `*[]T`. At the edges a `Ptr` becomes a slice with `p.Tail()`, a
string literal with `[]byte("...\x00")`, a single `*T` with `One(p)`, and a slice
a `Ptr` with `View(s)`. The analysis (`analyze.go`) records the uses a slice
cannot express per class; `gen.forward` decides.

## Pointers: `Ptr[T]`

`Ptr[T]` is a C pointer that may walk. It is a comparable value — `==` and
`!=` between two `Ptr[T]` mean what they mean in C, and the zero `Ptr[T]{}`
is NULL.

| C | Go |
| --- | --- |
| `p == NULL`, `!p` | `p.Nil()` |
| `p != NULL`, `if (p)` | `!p.Nil()` |
| `*p` | `p.Get()` |
| `*p = v` | `p.Put(v)` |
| `p[i]` | `p.At(int(i))` |
| `p[i] = v` | `p.Set(int(i), v)` |
| `&p[i]`, `p + i` | `p.Add(int(i))` (a `Ptr`), or `p.Ref(int(i))` when a `*T` is wanted |
| `p++`, `p += n` | `p = p.Add(1)`, `p = p.Add(int(n))` |
| `p - q` | `p.Sub(q)` (an `int`; convert) |
| `p < q` | `p.Lt(q)` (also `Le`, `Gt`, `Ge`) |
| `p->f` for `Ptr[T]` | `p.P().f` |
| `(*p)++` | `p.Put(p.Get() + 1)` |
| `*p++` (read) | `c := p.Get(); p = p.Add(1)` — in that order |
| `*p++ = c` | `p.Put(c); p = p.Add(1)` |
| a `*T` where a `Ptr[T]` is required | `Addr(x)` — a one-element `Ptr`; **only** when the callee never looks past that element |
| a `Ptr[T]` where a `*T` is required | `p.P()`; `p.Ref(k)` for `p.Add(k)`, and `&a[k]` for an array's element |
| an array where a `*T` is required | `&a[0]` |
| `NULL` for `*T` / `any` / func | `nil` |
| `NULL` for `Ptr[T]` | `Ptr[T]{}` |

`*T` stays a plain Go pointer: `p->f` is `p.f`, `p == NULL` is `p == nil`.

## Strings

- A string literal `"abc"` is `S("abc")` — a `Ptr[byte]`, NUL-terminated.
  Translate C escapes to Go escapes (`\033` is fine in Go; C's `\x1b` is
  greedy in C and not in Go — check the next character). Adjacent literals are
  concatenated in C: concatenate them in one `S(...)`.
- Character tests on a `Ptr[byte]` are on bytes: `*p == NUL` is
  `p.Get() == NUL`.
- The vendored `musl_str*`, `musl_is*`, `musl_to*` functions are ordinary
  functions in the chunks and are transpiled like everything else.

## The functions `crt.go` replaces

These 11 C functions are **not** transpiled (their definitions are not in any
chunk). Translate their *calls*:

| C | Go |
| --- | --- |
| `alloc(n)`, `lalloc(n, …)` returning bytes | `Alloc(int(n))` |
| `alloc_clear(n)`, `lalloc_clear(n, …)` for bytes | `Alloc(int(n))` (always zeroed) |
| `(T *)alloc(sizeof(T))`, `alloc_clear` of one struct | `new(T)` |
| `(T *)alloc(n * sizeof(T))` | `Mk[T](int(n))` |
| `alloc(offsetof(T, field) + n)` — the **struct hack** | `x := new(T); x.field = Mk[E](int(n))` (the trailing array field is a `Ptr` in `types.go`) |
| `vim_free(p)` | nothing — drop the call (the garbage collector owns memory); keep any `p = NULL` that follows |
| `vim_realloc(p, n)` | `Realloc(p, n)` in elements |
| `mch_memmove`, `musl_memmove(d, s, n)`, `musl_memcpy` | `Memmove(d, s, count)` — **count in elements**, not bytes |
| `musl_memset(p, c, n)` on bytes | `Memset(p, c, int(n))` |
| `musl_memset(p, 0, n * sizeof(T))` on structs | `Zero(p, count)`, or `*x = T{}` for one struct |
| `musl_memcmp(a, b, n)` | `Memcmp(a, b, int(n))` |
| `musl_qsort(base, n, sizeof(T), cmp)` | `Qsort(base, int(n), cmp)` — `base` a `Ptr[T]`; a comparator's `const void *` parameters are `any` holding a `*T` |
| `musl_bsearch(key, base, n, sizeof(T), cmp)` | `Bsearch(key, base, int(n), cmp)` |

And nine C string functions are written in Go in `editor/libc.go`, on `bytes`
and `copy` rather than transpiled: `musl_strlen`, `musl_strcpy`,
`musl_strncpy`, `musl_strcat`, `musl_strcmp`, `musl_strncmp`, `musl_strchr`,
`musl_strstr` and `musl_strpbrk`. Their calls are translated as they stand.
Their contracts are musl's exactly: a comparison returns the difference of
the first bytes that differ, not only its sign, and `strchr(s, 0)` is the
terminator. `editor/libc_test.go` holds each to the transpiled original it
replaced, kept as `editor/libc_oracle_test.go`. The case-insensitive pair,
`strtol`/`atoi` and the `is*`/`to*` one-liners stay transpiled: the first
three have no exact Go counterpart, and the one-liners are plain Go already.

`sizeof` has no Go counterpart: it becomes an element count (`sizeof(buf)` of
a `char buf[N]` is `N`), or disappears into `new`/`Mk`. Never use
`unsafe.Sizeof` — `unsafe` is `crt.go`'s alone.

## Growarrays

`garray_T.ga_data` is `any` holding a `Ptr[T]`. Where C casts it —
`((char_u **)gap->ga_data)[i]`, `(char_u *)ga.ga_data` — write
`GaData[Ptr[byte]](gap).At(int(i))`, `GaData[byte](&ga)`. `GaData`
allocates the storage lazily at the first typed access, as large as
`ga_maxlen`. Inside `ga_grow_inner`, after raising `ga_maxlen`, call
`GaGrowTo(gap, int(gap.ga_maxlen))` instead of the realloc. `gap->ga_data =
NULL` is `gap.ga_data = nil`.

## `container_of`

The phase-128 C recovered a struct from a hash key that pointed into it, and
the first pass kept an owner registry for it. Since phases 133 and 140 the C
has no `container_of` and no hash table, and the registry is gone. If one
reappears, it is a finding for a pipeline phase, not something to work around
here.

## Option variables: `varp`

Since phases 151 and 152 the C types them, and so does the Go:
- `vimoption_T.var`, `optset_T.os_varp` and every `varp` are an `optvar_T`
  (`ov_int *int32`, `ov_long *int64`, `ov_str Ptr[Ptr[byte]]`, `ov_win`);
- the defaults are `def_str [2]Ptr[byte]` and `def_num [2]int64`.

A string variable is `Addr(&p_x)`, and a terminal option is
`View(term_strings[:]).Add(int(K))`. Compare an `ov_str` against the same
construction, so that the pointers are equal where C's are.

## Expressions and statements

- **Conditions**: C's `if (x)` on an integer is `if x != 0`; on a pointer,
  `!p.Nil()` / `p != nil`. `!x` is `x == 0`. A comparison or `&&`/`||`
  *used as a value* is `B2i(...)`.
- **Side effects inside expressions** (`x = y = 0`, `a[i++]`, assignment in a
  condition, the comma operator) are split into statements **in C's
  evaluation order**. `while ((c = *p) != NUL)` becomes a `for` whose body
  starts with the assignment and the test.
- **`?:`**: an `if`/`else` into a temporary. Never a helper that evaluates
  both arms.
- **`switch`**: Go's `case` does not fall through; where C's does, end the
  case with `fallthrough`. A C `[[fallthrough]];` is exactly that. `break`
  inside a `switch` leaves the `switch` in both languages; a `break` meant for
  an enclosing loop needs a label in Go.
- **Locals are declared where C declares them**: `x := e` when e has x's Go
  type, `var x T = e` when it may not (an untyped constant), `var x T` with no
  initializer. Three kinds stay at the function's top instead: every local of a
  function with a `goto`, since Go forbids jumping over a declaration; a
  local declared directly in a `switch` case, since C's case is no scope and
  Go's is; and a local without an initializer inside a loop, since C at `-O0`
  keeps its value from one iteration to the next and a Go declaration in the
  body would zero it.
- **`goto`**: keep it. With its function's locals at the top, most C `goto`s are
  legal Go as they stand; restructure the rest with labeled loops and flags,
  preserving behaviour exactly.
- **Temporaries** (`tN`, for a `?:`, a split `&&`, a postfix `++` used as a
  value) are declared where they are made, since each is assigned on the next
  line: `var tN T = e`, or `for tN := 0` for a counter. A function with a `goto`
  keeps them at its top with its locals.
- **`for (;;)`** is `for {`. A C `for` with a comma in it becomes its
  statements.
- **Unused**: Go rejects unused locals and imports; C does not. Delete the
  unused, or `_ = x`.
- `static_assert` disappears.
- A C `union` is a Go struct with every member as its own field: write and
  read the member the C code names. If the C code writes one member and reads
  another (type punning), **say so in your report**.
- A bitfield is an ordinary field; if the C code relies on truncation to its
  width, mask explicitly.

## `vim_snprintf`

`vim_snprintf(buf, size, fmt, ...)` keeps its C format strings. Pass
arguments as their Go values — `Ptr[byte]` for `%s`, the integer as it is
typed for `%d`, `%ld`, `%lld`, `%c`, `%x` — the host's formatter reads them.

## A block-scope `static`

It is the hoisted global `<function>_<name>`. If it has an initializer, give
it one in an `init()` in your file: `func init() { deathtrap_entered = 0 }`.

## Checking your file

`sh internal/gen/check.sh editor/<yourfile>.go` compiles your file alone against the
skeleton, with a panicking stub for every function you do not define. It must
print `check: <name> compiles` before you are done.

## Your report

When your file compiles, end with a short report: what you could not
translate faithfully and how you adapted it; every place `types.go`,
`globals.go` or `sigs.md` looked wrong; every union read through a member it
was not written through; anything you are unsure behaves as the C does.
