# whim++: the editor in C++

**Built** (2026-10-04): whim++ is the eighth translation of the core, beside
the Go (`editor/`), the Java (`braaam/`), the Clojure (`vijure/`), the
Haskell (`caprice/`), the Rust (`whimsy/`), the Scheme (`whimsical/`) and
the OCaml (`whiml/`), and held to the same test: `whim test --cpp` and
`--wide --cpp` answer all 80 and all 240 cases as the C does, with a control
of its own. It was built in two milestones, as whiml was: faithful (§1-§8),
then much more idiomatic (`doc/CPP-IDIOMS.md`, §9). The design below is as
it stands, the milestones' measurements as they were taken.

```
go tool whim wpp             # bin/whim++, built in lib/wpp
make bin/whim++              # the same
make whim-test-cpp           # the quick suite with whim++ too
go tool whim test --wide --cpp
go tool whim skel editor.c DIR -cpp editor.cpp   # the backend on a core, by hand
```

## 1. What is on the machine

g++ 15.2 (`-std=c++23`), the standard library it ships (libstdc++, on
musl). GCC 15 has no `std::execution`, no reflection and no contracts; none
is used. Nothing was installed: the build is g++ called unit by unit from Go
(`wpp/wpp.go`), the units side by side.

## 2. The approach: the C, kept

C++ is nearly C's superset, so the C++ backend (`crefactor/togo/cpp*.go`)
does not lower the C or decide its memory again, as the Scheme and the
OCaml do: it prints the core's own syntax tree as C++ -- its statements,
its expressions with their own parentheses and no others, its declarations
as the grammar has them, its types and its memory as they are -- in the
canonical printer's shape (`crefactor/cemit`), and changes only what C++
says otherwise:

- **The editor is an instance.** Every file-scope object and every
  block-scope static is a field of one `class Editor` (`Editor::glue_`, the
  host's, is the one field the C has not), its initial value the field's
  default member initializer, and every function of the core a member
  function -- declared in the class, defined after it as `R Editor::f(...)`
  in `editor.cpp`. The C's names then resolve in the class's scope as they
  did in the file's, so a body is printed as the C wrote it, and an editor
  is `std::make_unique<Editor>()`: value-initialized, so every field the C
  leaves to zero is zero. A `const` object whose value holds nothing of an
  instance's is the class's (`static constexpr`, 9 of them: the tables of
  the regexp classes and the like); a block's `static const` stays in its
  function. A function that reaches nothing of the editor's is a `static`
  member (217); every member is private but `glue_` and `vim_main`, and
  the printf is the class's friend (`doc/CPP-IDIOMS.md`, items 5 and 7). A static of a function is a field named for both
  (`utf_class_buf__classes`), and a struct a function defines is defined at
  namespace scope when a static of it is the editor's.
- **A function pointer is a pointer to a member**: `void (Editor::*)(exarg_T
  *)`, `&Editor::ex_quit` where the C names the function as a value, and a
  call through one `(this->*p)(...)` -- `(*p)(...)` and `p(...)` alike. 18
  declarators say one: the command table's, the normal-mode table's, the
  option table's callbacks, and the parameters that take one. The functions
  are declared in the class before its fields, so that a table's initial
  value may take any one's address.
- **An enumeration is its integer type** unless the C uses it only as a
  type: each enumerator a `constexpr` of the type C gives it (`int`, or
  C23's fixed type), and every use of the enumerated type its integer type
  -- so C's arithmetic on them is C's, and none of C++'s enum conversions
  (an `int` to an enum, `++` of one, `|` of two kinds, which C++20
  deprecates) arises. The 11 of 24 named enumerations whose every use is a
  store, a comparison, a switch, an argument, a result or a cast of the
  type itself are `enum class E : U {...}; using enum E;`
  (`doc/CPP-IDIOMS.md`, item 3). Struct and union definitions are printed
  at namespace scope in the C's order, every tag declared first, a tagged
  one nested in another hoisted before it: C's tags are the file's, where
  C++'s would be the enclosing class's. A struct is named by its tag alone
  (`file_buffer *`), `typedef struct {...} pos_T;` is `struct pos_T`, a
  typedef `using`.
- **What C converts and C++ does not is a cast**, written where C made the
  conversion -- an assignment, an initial value, an argument, a return, a
  conditional's arm, a comparison: a `void *` to another pointer, a string
  literal to `char *`, `char *` and `unsigned char *`, an integer constant
  where a pointer goes, a function to a pointer of another type. 1,103
  casts. A pointer converts implicitly where C++ allows it (to `void *`, to
  a more `const` one, `nullptr`). Every cast, the C's own and these, is a
  named one -- `static_cast`, `reinterpret_cast`, `const_cast` -- and a
  string literal the C makes `char *` or `char_u *` is `"..."_c` or
  `"..."_uc` (`doc/CPP-IDIOMS.md`, item 1).
- **A pointer parameter never null** -- only dereferenced, and every call
  passing `&x` or such a parameter -- is a reference: 141
  (`doc/CPP-IDIOMS.md`, item 6).
- **In a braced list C++ refuses a narrowing**: an element converted to its
  member's type where C converts it, unless it is a constant that fits or a
  widening; and `{0}` for a struct is `{}`.
- **C23's keywords are printed as written**: the front end expands
  `nullptr`, `true` and `false` as macros (`((void*)0)`, 1, 0); the printer
  finds them in the source by position and writes the keyword.
- **A jump past an initial value**, which C++ forbids: a declaration with an
  initial value that a case of an enclosing switch or a goto from outside
  its rest crosses is a declaration and an assignment (1 in the core; the
  `case` labels of a switch the declaration precedes cross nothing).
- **A name C++ reserves** takes a trailing underscore (`new`, `class`,
  `this`, `template`... none in the core now); C++'s `typeof` is
  `decltype`, `_Bool` is `bool`, a GNU attribute is dropped (`format`'s
  argument numbers would count `this`).

Two things C leaves to the compiler C++ defines otherwise, and the backend
keeps the C's:

- **The order of an assignment.** C++17 evaluates an assignment's right
  operand, its side effects with it, before its left; gcc's C evaluates the
  left first. Where it could matter -- the left changes an object the right
  reads, or one a call on the right could reach, or the left calls -- the
  statement takes the left's address first: `{ char_u **lhs__ =
  &((char_u **)(ga.ga_data))[ga.ga_len++]; *lhs__ = vim_strnsave(IObuff,
  len); }`. One statement of the core (`TestCppSteps` holds the rule:
  `buf2[n++] = bump()` where bump reads n).
- **A local read before it is set.** gcc's C at `-O0` reads what the stack
  holds; g++ at `-O2` warns. A scalar a path may read before a store (Java's
  definite assignment, `java_da.go`, no bolder than JLS 16's) is zeroed,
  `int n{};` -- 847 -- and one declared in a loop's body, whose C stack slot
  keeps what the last iteration left (`TestCppLocals` holds it), is declared
  once at the function's top, zeroed: 66. A zeroed one a jump crosses is
  zeroed by an assignment after it.

A compound literal at file scope is C's object of static storage: a field
(`complit__1`, five: the empty strings of the five buffer headers); in a
block, C's lives to the block's end and C++'s temporary to the
expression's: a local declared before the statement.

`editor.hpp` and `editor.cpp` include nothing but `rt.hpp`, which includes
nothing: no C header's macro (`INT_MAX`, `EOF`, `SIGHUP`, all of them the
core's constants too) can reach the core's names. A host includes
`editor.hpp` first.

## 3. Memory and layout

The C's own: a pointer is a pointer, a struct a struct, `sizeof` the C's.
`wpp`'s `TestLayout` holds every struct and union of the core that C names
at file scope (155, 911 members) to the C front end's listing -- gcc's
`sizeof` and `offsetof` on the C must give it, and g++'s on the generated
header too -- but for the four that hold a pointer to a function
(`exarg_T`, `struct cmdname`, `struct nv_cmd`, `struct vimoption`): C++'s
pointer to a member function is two words (Itanium's ABI), where C's is
one, so those are 8 bytes a pointer larger. Nothing in the core depends on
their size but their own `sizeof`s. The arena is the C host's: 1 GiB,
`calloc`'d per editor (its pages the kernel's zeros until touched), the
allocation counted atomically, since the parallel `:%s`'s chunks allocate at
once.

## 4. The host

`wpp/src/host.hpp`: the `Host` interface (`editor/host.go`'s, as whimsy's
trait: `std::optional`, `std::span`, `std::string_view`), and `run(host,
args)`; `host.cpp` the C host's 17 functions as members of the editor, a
line of glue each to its `Glue` (the Host, owned by a `std::unique_ptr`, and
the arena, a `std::unique_ptr` with `free`), `host_exit` an exception
`run` catches -- the C's longjmp back to `main`. `term.cpp` is the terminal
host, the C host's own calls (sigaction with no `SA_RESTART`, the death
pipe, select, termios), its flags `std::atomic`. `printf.cpp` is the C
host's `vim_snprintf`, its 1,383 lines pasted into a struct whose functions
reach the core's through the editor (a forwarder each: `emsg`, `IObuff`...),
changed only where C++ wants a cast. `main.cpp` is the launcher.

Four editors run at once in one process, each on a thread and a host of its
own (`wpp_test.go`'s `TestEditorsAreInstances`, `testdata/instances/
main.cpp`, compiled against `lib/wpp`).

## 5. The parallel `:%s`

`match_lines`' body is the runtime's (`internal/whim/gen.go`'s
`matchLinesCpp`): `chunks(n, [&](long from, long to) {...})`, each chunk on
an engine `alloc_clear` makes. `rt.hpp`'s `chunks` is a template over
`chunks_run` (`rt.cpp`), which runs about four chunks a core, none under 64
lines, on `std::jthread`s joined as their vector goes; a range of one chunk
runs on the caller's thread.

## 6. The build

`go tool whim wpp` (`make bin/whim++`): `whim.Cut` the core, the C++
backend writes `editor.hpp` and `editor.cpp` into `lib/wpp/src` beside the
embedded hand-written sources, and g++ compiles the six units side by side,
each only when its source or a header it includes moved, with `-std=c++23
-O2 -Wall -Wextra` -- **any output a failed build**: the generated core
compiles with no warning. `whim gen` writes `wpp/src/editor.hpp` and
`editor.cpp`, tracked, and `make whim-editor-check` holds them. The C++
sits in `wpp/src/` because Go takes a `.cpp` beside a `.go` for cgo's.

## 7. Milestone 1, measured

- `editor.cpp` 61,921 lines and `editor.hpp` 10,995, all 1,713 functions,
  none refused; the backend writes them in about 3 s.
- g++ on the core: **34-35 s, 0.34 GB** peak (one unit; the others a
  second each, side by side); `-Wall -Wextra`: **0 warnings** (38 at first
  -- 4 `-Wuninitialized` and 34 `-Wmaybe-uninitialized`, the C's own reads
  of unset locals -- before the zeroing above; 131 unused parameters and
  28 missing member initializers before `[[maybe_unused]]` and `{}`).
- `bin/whim++` 1.1 MB, dynamically linked against libstdc++.
- `whim test --cpp`: all 80 cases as the C, the control seen by 76;
  `--wide --cpp`: all 240, the control seen by 94 keys and 6 pty cases.
- The heavy case: **0.2 times the C** (95-99 ms against 445-472; the Go
  0.4-0.6): the C is `-O0`, whim++ `-O2`, and the `:%s` runs in chunks.
- `crefactor/togo`'s `TestCpp*`: the 25 foreign C programs the Scheme's and
  the OCaml's tests use, and one of C++'s own (`cppOwnC`: every rule of §2
  once), compiled by g++ and printing what gcc's builds print;
  `TestCppControl`'s three mutations (an unsigned char read signed, a
  function pointer's call made of another, a block's static made the
  call's) each move the output.
- Under 48 busy loops on the 64 cores (`stress_test.go`, `BINS=bin/whim++`):
  see *Under load*.

## 8. Under load

Under 48 busy loops on the 64 cores (`internal/suite/stress_test.go`,
`BINS=bin/whim++`, a load of 40-50): **from a file, 0 of 3,120 quick runs
(40 a case) and 0 of 3,120 wide (13 a case) differ** from the case's first.
Through a pipe -- the control -- 42 of 3,120 quick (37 of them
`par_vglobal`) and 4 of 2,990 wide: whim++ starts as fast as the C and
reads its keys as they land, as the C does (the C 66 and 106, whimsy 33 and
61).

The same under milestone 2's printing (a load of 60-70, other work
sharing the machine): from a file 0 of 3,120 quick and 0 of 3,120 wide;
through a pipe 15 and 198.

## 9. Milestone 2: much more idiomatic

`doc/CPP-IDIOMS.md` surveys the C++ as milestone 1 left it, counted and
ranked, and items 1-9 are done in the backend (and the host), each held to
`TestCpp*`, both suites, the heavy case and the build's zero warnings:
named casts and literal operators for the 5,719 C casts; `using` and
structs named by their tags; `enum class` for 11 of 24 named enumerations;
`[[nodiscard]]` on 753 functions; 217 `static` member functions; 141
reference parameters; the class private but for `glue_` and `vim_main`,
the printf its friend; `static constexpr` tables; and the host's RAII,
namespace and `std::jthread` from milestone 1. Declined, measured:
`-Wconversion`'s 337 (the C's narrowings), structured bindings at the 104
sites of the 64 struct results (63 inside expressions), `std::span` and
`std::string_view` (no bound proved; 23 tests of `-1` for "to the NUL"),
`std::array` (234 fields that decay), `std::optional` (the null pointer is
the C's "nothing" already).

Measured: editor.hpp 11,090 and editor.cpp 61,921 lines; g++ on the core
34-40 s (under the loads above) and 0.34 GB; `-Wall -Wextra` 0 warnings;
bin/whim++ 1.1 MB; `whim test --cpp` all 80, the control seen by 76;
`--wide --cpp` all 240, the control seen by 94 and 6; the heavy case 0.2
times the C (97-103 ms against 432-446).
