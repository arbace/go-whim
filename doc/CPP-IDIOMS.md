# How whim++ could be more idiomatic C++: a survey

2026-10-04. It covers `wpp/src/editor.hpp` and `editor.cpp` as milestone 1
left them (`b7efc78`: 10,995 and 61,921 lines, all 1,713 functions of the
core, written by `crefactor/togo`'s C++ backend, `doc/CPP.md`), and the
hand-written runtime, host, terminal host and printf of `wpp/src/`.
`doc/OCAML-IDIOMS.md` and `doc/RUST-IDIOMS.md` are its model: every item
says what the pattern is, how many sites it has, what it becomes, who does
it -- the C++ printer (`crefactor/togo/cpp*.go`) or the hand-written
files -- and how it is held. The generated files stay generated: every
item done is a change to the backend, the files regenerated. Items 1-9
were done the same day (*Done*); the rest are declined, with why.

## The tension, stated first

whim++ at milestone 1 is the C, kept: its statements, expressions,
declarations and memory as they are, in a class. That is what C++ makes
cheap -- and what a C++ programmer reads as C. The objections fall in two
kinds. **The spelling**, which costs nothing at run time and is the
printer's to change: C's casts, `typedef`, `struct T` where C++ says `T`,
enumerators as `int`s, every member public, a pointer where nothing can
be null. **The model**: `std::span`, `std::string_view`, `std::array`,
`std::optional`, tuples for results. The C walks its strings by pointer to
the NUL, writes into them in place, passes `-1` for "to the end", decays
its arrays everywhere and uses null as its "nothing", so the model moves
only where an analysis proves the C does none of that; the analyses
`crefactor/togo` has (effects, definite assignment, out-parameters, the
enumerations' uses) prove the spelling items, and none proves a bound.
Every item is held to both suites, the heavy case, `TestCpp*` and the
build's zero warnings.

## How it was measured

Counted on the generated files and the core (`go tool whim skel
editor.c DIR -cpp editor.cpp`, whose log counts what the backend wrote),
and compiled: `g++ -std=c++23 -O2 -Wall -Wextra` (the build's) and
`-Wconversion -Wsign-conversion` (counted, not the build's).

## The shape at milestone 1

| Pattern | Sites |
| --- | --- |
| C-style casts (the C's own and the 1,052 the backend wrote) | 5,719 |
| of them a string literal made `char *` or `char_u *` | 1,677 |
| `typedef` | 188 (104 of an untagged struct) |
| `struct T` / `union T` naming a type | 182 |
| enumerations as `int`, enumerators as `constexpr int` | 24 named, 1,290 enumerators |
| members, all public | 1,713 functions, 804 fields |
| functions reaching no field, host or function pointer, as members | 217 |
| functions whose every call uses the result | 753 of 883 with one |
| pointer parameters that are never null and only dereferenced | 141 |
| `static inline const` tables | 9 |
| `-Wconversion` (`-Wsign-conversion` too) | 337 (723) |

## The findings, ranked

1. **No C casts** (5,719 casts): each a named cast that says what it
   does; string literals by literal operators. Done.
2. **`using` and bare tags** (188 typedefs, 182 tag references). Done.
3. **`enum class`** where no use is arithmetic (11 of 24). Done.
4. **`[[nodiscard]]`** (753 functions). Done.
5. **`static` member functions** (217). Done.
6. **References** for never-null pointer parameters (141). Done.
7. **Private members**, a friend for the printf. Done.
8. **`constexpr`** constants and tables. Done.
9. **RAII, namespaces, `std::jthread` in the host**. Done at milestone 1.
10. `-Wconversion`: counted (337), declined.
11. Out-parameters as structured bindings (104 sites): declined.
12. `std::span` / `std::string_view` (97 pointer-and-length pairs, 330
    `char_u *` parameters): declined.
13. `std::array` (234 array fields): declined.
14. `std::optional` (1,058 null comparisons): declined.

## Done

### 1. No C casts

Every cast of the C's and every conversion C++ wants written is a named
cast chosen from the two types (`cpp_expr.go`'s `cast`): `static_cast`
between arithmetic types, from or to `void *`, of `nullptr` and to `void`;
`reinterpret_cast` between pointers to different types, between pointers
and integers and between function pointers; `const_cast` where the C drops
a const, around the conversion of the type when both happen. A string
literal the C makes `char *` or `char_u *` -- vim writes through none, but
its types say mutable -- is `"..."_c` or `"..."_uc`, two literal operators
the header defines once, the `const_cast` in them: 1,677 literals. The
front end's type of a C cast loses its pointee's `const`; the spelling of
the cast's type says it, and the printer reads it there.

Measured: C-style casts 5,719 -> **0**: 3,025 `static_cast`, 1,008
`reinterpret_cast` (char * and unsigned char *, mostly: vim's `char_u`),
9 `const_cast`, 1,677 literals. The binary is the same size within 6 KB.

### 2. `using`, and a struct named by its tag

`typedef T N;` is `using N = T;` (81); `typedef struct { ... } pos_T;` is
`struct pos_T { ... };` (104); and a struct or union is named by its tag
alone, `file_buffer *`, where C says `struct file_buffer *` (182 -> 0) --
every tag declared at the top of the namespace, so that a type may name
one before its definition as C's `struct T` may. A tag that is also an
ordinary name of the unit keeps its keyword (none now).

### 3. `enum class` where the C uses an enumeration as a type

`cpp_enum.go` judges every use of a named enumeration's values and
enumerators: stored in, compared with, switched on, passed as, returned
as, cast to and from the enumeration -- or anything else. An enumeration
used only so is `enum class E : U { ... }; using enum E;` -- its
enumerators named as the C names them -- and its values typed `E`
everywhere: **11 of 24** (`SpecialKey`, `cmd_addr_T`, `etype_T`,
`paste_mode_T`, `flush_buffers_T`, `keyprot_T`, `kkpstate_T`, `sb_clear_T`,
`request_progress_T`, `xp_prefix_T`, `estack_arg_T`). The 13 that stay
integers, and the first use that rules each out: `CMD_index` (an array's
index), `hlf_T`, `set_op_T`, `idopt_T`, `reg_getline_flags_T` (bits or
arithmetic), `magic_T` (its address taken), `key_extra` and `set_prefix_T`
(an integer's initial value), `getline_opt_T`, `mokstate_T`,
`optmagic_T`, `regstate_E` (an integer stored in one), `map_result_T` (an
assignment of another type). The 1,290 enumerators of the anonymous
enumerations are `constexpr`s of C's type, as at milestone 1.

### 4. `[[nodiscard]]`

A function with a result that no call of the unit drops -- an expression
statement, a comma's left, a cast to `void`, a for's clauses -- and that
the hand-written C++ does not call: **753 of 883** (`cpp_fx.go`). Dropping
one now warns, and the build fails on a warning.

### 5. `static` member functions

A function that reaches no field of the editor, no host function and no
function pointer, closed over its calls -- effects.go's judgement, the
Rust's and the Scheme's -- and that is not used as a value is a `static`
member: **217** (`musl_strlen`, `utf_char2len`, `vim_isdigit`...).

### 6. References for parameters never null

A pointer parameter to an object whose function only dereferences it
(`*p`, `p->m`) or passes it on to another such parameter, and every call
of which passes `&x` of the parameter's very type or such a parameter of
its own, is `T &p`, used as `p` and `p.m`, and a call passes `x`
(`cpp_refs.go`, a fixed point over the calls): **141** parameters. A
function used as a value, the host's and one the hand-written C++ calls
keep their pointers.

### 7. Private members

The class is `public:` only for what the hand-written C++ names from
outside -- `glue_` and `vim_main` (the profile's `CppExports`) -- and
`private:` for the other 1,712 functions and the 804 fields; the printf
(`printf.cpp`'s `struct Printf`, the profile's `CppFriend`) is its friend.

### 8. `constexpr`

The enumerators were `constexpr` from milestone 1; the 9 constant tables
the editors share (`static inline const` at milestone 1) are `static
constexpr` members.

### 9. The host: RAII, a namespace, `std::jthread`

From milestone 1: the arena a `std::unique_ptr` with `free`, the Host a
`std::unique_ptr<Host>`, `host_exit` an exception, the interface
`std::optional`, `std::span` and `std::string_view`; everything in
`namespace whimpp`; the parallel `:%s` on `std::jthread`.

### Before and after

| | milestone 1 | milestone 2 |
| --- | --- | --- |
| editor.hpp, editor.cpp | 10,995 + 61,921 lines | 11,090 + 61,921 lines |
| C-style casts | 5,719 | 0 |
| g++ -O2 on the core | 34-35 s, 0.34 GB | 34-40 s, 0.34 GB |
| -Wall -Wextra | 0 warnings | 0 warnings |
| -Wconversion (-Wsign-conversion) | | 337 (723) |
| bin/whim++ | 1.1 MB | 1.1 MB |
| heavy case | 0.2 times the C | 0.2 times the C |
| whim test --cpp, --wide --cpp | 80, 240 | 80, 240 |

## Declined, with why

### 10. -Wconversion

337 warnings (723 with `-Wsign-conversion`): the C's own implicit
narrowings -- a `long` line number into an `int` column, a `size_t` into an
`int` length -- each a value C converts as it is defined to. Each fix is a
cast that says nothing new, and a wrong one would change what the editor
does; counted, and left to the build's `-Wall -Wextra`.

### 11. Out-parameters as structured bindings

Phase 100 already made the core's out-parameters values (`crefactor/togo`'s
analysis finds 2 left, in 2 functions; the references of item 6 take
what remains of the idiom), and 64 functions return a struct of their
results. Their 104 call sites read it as the C's comma, `x = (f__o = f(a),
y = f__o.m, f__o.r__)`. A structured binding is a declaration: 63 of the
sites are inside a condition or an expression, where none can stand, and
the other 41 assign locals the function already has, which `auto [r, m]`
would shadow -- `std::tie` would need tuples where the C has named
members. Declined: a struct is C++'s named tuple, and the comma is what
C++ can say where the binding cannot.

### 12. `std::span` and `std::string_view`

97 parameter pairs of a pointer and a length, 330 `char_u *` parameters.
vim's strings are NUL-ended and written in place (a `string_view` is
read-only), its lengths a prefix of what the pointer reaches, or `-1` for
"to the NUL" (23 tests of a negative length in the core): no analysis of
ours proves a pointer's extent, and a span built from a length that is not
it would be a lie the type then repeats. Declined.

### 13. `std::array`

234 array fields (`IObuff`'s kin, the screen's lines): every use decays
to a pointer the C walks and passes on; `std::array` would add `.data()`
at each and change nothing else. Declined.

### 14. `std::optional`

1,058 comparisons with `nullptr`: the C's "nothing" is the null pointer,
which a pointer already says; `std::optional<T *>` would hold two of them.
Declined.

### Kept as it is

The pointer to a member function for the C's function pointers (two words
where C's is one: the four structs that hold one are 8 bytes a pointer
larger, `doc/CPP.md` §3) -- a plain function of an `Editor *` would keep
C's layout and lose C++'s spelling, and nothing depends on those sizes.
