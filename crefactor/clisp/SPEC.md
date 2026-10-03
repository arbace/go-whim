# C-lisp: C23 as s-expressions

`crefactor/clisp` converts a C translation unit to s-expressions (`ToLisp`,
`go tool whim c2lisp`) and back (`ToC`, `go tool whim lisp2c`). Files are
`.lc`. It is a converter, not a compiler: C-lisp is the C program in another
spelling, read from `crefactor/cc`'s tree and printed back in
`crefactor/cemit`'s canonical spelling, so that

    ToC(ToLisp(x)) == cemit.Canonical(x)

byte for byte -- and, for a text already canonical (every boundary of whim's
pipeline, `whim-vim.c` among them), `ToC(ToLisp(x)) == x`. The forms hold the
program, not its layout: a text that is not canonical comes back canonical.

This file is every form, with the C beside it. `doc/C-LISP.md` is what was
measured.

## Atoms and lists

An **atom** is C's own token text, never decoded or re-encoded: an identifier
(`buf`), a keyword (`unsigned`), a number (`0x7fUL`, `1.5e3`, C23's `1'000'000`
and `3uwb`), a character or string literal with its prefix and escapes
(`'\''`, `L"x"`, `u8'a'`, `"a\tb"`), and the operator heads (`+`, `->`,
`<<=`). A `'` inside a number is C23's digit separator, not a quote. The reader keeps a quoted literal whole,
whatever it holds, so `'('` and `"a ;b"` are one atom each; outside a literal
`;` starts a comment that runs to the end of the line. Adjacent string literals
are one atom already: the front end joins them (`"a" "b"` is `"ab"`).

A **list** is `(head args...)`. Every compound form has a head that says what it
is, so an identifier in head position is never a question: an identifier is an
atom, and an atom on its own is an identifier or a literal.

Texts the forms carry verbatim -- an include's operand, a macro's invocation,
an attribute the forms do not structure -- are written as a C string literal's
atom (`"<stdio.h>"`, `"va_arg(ap, int)"`), `"` and `\` escaped.

## Top level

| form | C |
| --- | --- |
| `(include "<stdio.h>")` | `#include <stdio.h>` |
| `(directive "#include  <x.h>")` | an include line spelled any other way, verbatim |
| `(def NAME TYPE)` | `TYPE NAME;` |
| `(def static NAME TYPE VALUE)` | `static TYPE NAME = VALUE;` |
| `(typedef NAME TYPE)` | `typedef TYPE NAME;` |
| `(defn PREFIX... NAME (fn PARAMS RESULT) ITEM...)` | a function definition |
| `(defn PREFIX... NAME (fn-ids (ID...) RESULT) (kr-params DECL...) ITEM...)` | an old-style (K&R) definition |
| `(struct ...)`, `(union ...)`, `(enum ...)` | a declaration of that type alone: `struct pt { ... };` |
| `(declare SPEC...)` | any other declaration without a declarator |
| `(static_assert E "msg")` | `static_assert(E, "msg");` |
| `(static_assert E)` | `static_assert(E);`, C23's without a message |
| `(macro-decl "TEXT")` | a declaration that is wholly a macro's invocation |
| `(verbatim "TEXT")` | a line cemit prints from the source's tokens: an `asm` statement, a `__label__` declaration, a for-declaration of more than one declarator |

**One declaration, one declarator.** `int a, *b;` is `(def a int)` and `(def b
(ptr int))`, as cemit prints it on two lines. A blank line separates top-level
forms, as it separates the C's; includes are written together, as cemit writes
them.

**The prefix.** A storage class, a function specifier or an attribute that
comes before every type specifier stands before the name: `static`, `extern`,
`typedef`, `register`, `auto`, `inline`, `_Noreturn`, `_Thread_local`,
`thread_local`, `constexpr` (and the `__inline`, `__inline__`, `__thread`
spellings), `__auto_type`, and `(attr ...)`. `(typedef NAME TYPE)` is `(def typedef NAME
TYPE)`. One that comes after a type specifier stays in the specifier list, in
its place: `const static int x;` is `(def x (const static int))`.

After the type, a `def` may have attributes (`(attr ...)`), an asm label
(`(asm-label "__asm__(\"name\")")`), and then its value -- an expression or an
`(init ...)`:

```
(def static vim_vsnprintf
  (fn ((str (ptr char)) (str_m usize) (fmt (ptr (const char))) (ap va_list)) int)
  (attr (format printf 3 0)))
```
```c
static int vim_vsnprintf(char *str, usize str_m, const char *fmt, va_list ap) __attribute__((format(printf, 3, 0)));
```

**No type specifier.** A declaration whose specifiers are all prefix --
C23's `auto x = 1;`, whose type is inferred, or an old `static x;` -- has the
empty specifier list, `()`, for its type; `__auto_type`, gcc's spelling of the
inference, is a prefix word the same way:

| form | C |
| --- | --- |
| `(def auto x () 1)` | `auto x = 1;` |
| `(def __auto_type x () 1)` | `__auto_type x = 1;` |
| `(def static x ())` | `static x;` |
| `(def x (const auto) 1)` | `const auto x = 1;` -- `auto` after a type word stays in the list |

A definition's head line is cemit's: the specifiers and the result's pointers
indented, the name at column 0 under them.

```
(defn static ml_get_buf (fn ((buf (ptr buf_T)) (lnum linenr_T)) (ptr char_u))
  (return ...))
```
```c
    static char_u *
ml_get_buf(buf_T *buf, linenr_T lnum)
{
    return ...;
}
```

## Types: read in order

A type is a base -- its specifiers -- wrapped in derivations read from the name
outward, never C's inside-out declarator:

| form | C (declaring `x`) |
| --- | --- |
| `int` | `int x` |
| `(unsigned long)` | `unsigned long x` -- more than one specifier is a list of them, in the source's order |
| `(const char)` | `const char x` |
| `(spec T const)` | `T const x` -- a list whose first specifier is a typedef name is headed `spec` |
| `(ptr T)` | `T *x` |
| `(ptr T const)` | `T *const x` -- the pointer's own qualifiers follow its target, in the source's order |
| `(ptr T (attr A))` | `T *__attribute__((A)) x` -- an attribute among them |
| `(array 10 T)` | `T x[10]` |
| `(array T)` | `T x[]` |
| `(array static const 10 T)` | `T x[static const 10]` -- a parameter's `static` and qualifiers, in their order, before the size |
| `(array const T)` | `T x[const]` |
| `(array * T)`, `(array const * T)` | `T x[*]`, `T x[const *]` |
| `(fn PARAMS T)` | `T x(PARAMS)` |
| `(paren T)` | parentheses the source has where C does not need them: `char *(x[])` is `(array (paren (ptr char)))` |
| `(struct TAG)`, `(union TAG)`, `(enum TAG)` | `struct TAG x` |
| `(typeof E)`, `(typeof-type T)` | `typeof(E) x`, `typeof(T) x` |
| `(typeof_unqual E)`, `(typeof_unqual-type T)` | `typeof_unqual(E) x`, `typeof_unqual(T) x` |
| `(_BitInt N)` | `_BitInt(N) x`; `(unsigned (_BitInt 7))` is `unsigned _BitInt(7) x` |
| `(name-attr T ATTR...)` | `T x ATTR...`: C23's attributes after the identifier, so the outermost form -- `(name-attr (array 4 int) (std-attr maybe_unused))` is `int x [[maybe_unused]][4]` |
| `(__typeof__ E)`, `(__typeof__-type T)` | `__typeof__(E) x`, `__typeof__(T) x`; `__typeof` the same |
| `(atomic T)` | `_Atomic(T) x` |
| `(alignas E)`, `(alignas-type T)` | `alignas(E)`, `alignas(T)` as a specifier |

`(ptr (array 10 (ptr (fn (int) int))))` is `int (*(*x)[10])(int)`: a pointer to
an array of ten pointers to functions of an int returning int. The parentheses
a pointer inside an array or a function needs are written by `ToC`, and are not
in the forms; the ones it does not need are `(paren ...)`.

**Old-style definitions.** An identifier list is `(fn-ids (ID...) RESULT)`,
and a K&R definition's parameter declarations are `(kr-params DECL...)`, the
first of its items, one declarator a `def`, as cemit prints them one a line,
an indent in:

```
(defn f (fn-ids (a p) int)
  (kr-params (def a int) (def register p (ptr char)))
  (return a))
```
```c
    int
f(a, p)
    int a;
    register char *p;
{
    return a;
}
```

**Parameters.** `PARAMS` is a list: `(NAME TYPE ATTR...)` a named parameter,
`(TYPE ATTR...)` an unnamed one, an atom type bare (`void`, `int`), and `...`
last. `()` is `f()`.

```
(fn ((s (ptr (const char))) (n int (attr unused)) ...) int)
(fn (void) int)
(fn (((ptr char)) int) void)
```
```c
int (const char *s, int n __attribute__((unused)), ...)
int (void)
void (char *, int)
```

**Struct and union.** `(struct TAG MEMBER...)` defines; `(struct MEMBER...)`
is anonymous; `(struct TAG {})` has no members. A member is `(NAME TYPE)`,
`(NAME TYPE (bits W))` for a bit-field, `(TYPE)` for a member with no
declarator (an anonymous struct or union), `(TYPE (bits W))` for an unnamed
bit-field, or a `(static_assert ...)`; a member's attributes follow, `(NAME TYPE
ATTR...)`. **Attributes** of the struct itself are `(@ ATTR...)`: before the
members when the source has them before the body, after when after, and
`(struct TAG (@ ATTR...))` on a tag alone.

```
(struct pt (x int) (y int) (f unsigned (bits 3)) ((union (i int) (c char))))
(struct s (@ (attr packed)) (a int (attr (aligned 8))) (@ (attr (aligned 16))))
```
```c
struct pt
{
    int x;
    int y;
    unsigned f : 3;
    union
    {
        int i;
        char c;
    };
};
struct __attribute__((packed)) s
{
    int a __attribute__((aligned(8)));
} __attribute__((aligned(16)));
```

**Enum.** `(enum TAG (@ ATTR...) (: SPEC...) ENUMERATOR...)`, the tag, C23's
attributes between `enum` and the tag, and the fixed underlying type each
optional; an enumerator is `(NAME)` or `(NAME VALUE)`, its attributes after
the name, `(NAME ATTR... VALUE)`: `(enum e (@ (std-attr deprecated)) (A) (B
(std-attr deprecated) 2))` is `enum [[deprecated]] e { A, B [[deprecated]] = 2, };`. An
enum of one enumerator is one line, as cemit prints it -- how this tree spells
a constant.

```
(enum (: long) (LONG_MAX (cast long (>> (~ 0ul) 1))))
(enum hue (RED) (GREEN 3) (BLUE))
```
```c
enum : long { LONG_MAX = (long)(~0ul >> 1) };
enum hue
{
    RED,
    GREEN = 3,
    BLUE,
};
```

## Attributes

`__attribute__((unused))` is `(attr unused)`; `__attribute__((cold, format(printf,
1, 2)))` is `(attr cold (format printf 1 2))`. C23's `[[...]]` is `(std-attr
...)` the same way, a namespace's prefix part of the name's atom:
`[[nodiscard("why")]]` is `(std-attr (nodiscard "why"))`, `[[gnu::packed,
deprecated]]` is `(std-attr gnu::packed deprecated)`, `[[]]` is `(std-attr)`.
An attribute specifier that does not print back as its source with that shape
-- other spacing, an argument that is not one token, an attribute left out of
a list (`[[a,]]`), another vendor's arguments (`[[vendor::x(1, [2])]]`) -- is
`(attr-text "...")`, its source text. An attribute stands where the C has
it: in a declaration's prefix, after a declarator, after its identifier
(`name-attr`), on a parameter or a member, on a struct, union or enum (`(@
...)`), on an enumerator, among a pointer's qualifiers, after a function
definition's declarator (`(defn f (fn (void) int) (std-attr unsequenced)
...)`), as a statement (`attributed`), or before one (`stmt-attr`, and a
block's `(@ ...)`).

## Initializers

| form | C |
| --- | --- |
| `(init A B C)` | `{A, B, C}` -- at a declaration's top, one element a line |
| `(at .x 1)` | `.x = 1` |
| `(at (idx 3) V)` | `[3] = V` |
| `(at (idx 1 5) V)` | `[1 ... 5] = V` |
| `(at .a (idx 0) .b V)` | `.a[0].b = V` -- the designators, then the value |

```
(def static t (array (array 2 int)) (init (init 1 2) (at (idx 1) (init 3 4))))
```
```c
static int t[][2] =
{
    {1, 2},
    [1] = {3, 4},
};
```

## Statements

A function's body and a block are their items, a statement or a declaration
each:

| form | C |
| --- | --- |
| `(block ITEM...)` | `{ ... }` |
| `E` (any expression form) | `E;` |
| `(empty)` | `;` |
| `(if C THEN)`, `(if C THEN ELSE)` | `if (C) THEN else ELSE`; an `ELSE` that is an `(if ...)` is `else if`, a block holding one is a block |
| `(switch C BODY)` | `switch (C) BODY` |
| `(while C BODY)` | `while (C) BODY` |
| `(do BODY C)` | `do BODY while (C);` |
| `(for INIT COND STEP BODY)` | `for (INIT; COND; STEP) BODY`; a clause that is not there is `()`; `INIT` may be a `(def ...)` |
| `(return)`, `(return E)` | `return;`, `return E;` |
| `(break)`, `(continue)` | |
| `(goto L)`, `(goto* E)` | `goto L;`, `goto *E;` |
| `(label L)` | `L:` |
| `(case E)`, `(case-range A B)`, `(default)` | `case E:`, `case A ... B:`, `default:` |
| `(attributed ATTR... [E])` | `__attribute__((fallthrough));`, `[[fallthrough]];` and their kin |
| `(stmt-attr ATTR...)` | C23's attributes before the statement that follows, a line of their own: `[[gnu::musttail]]` above `return f(x);`, `[[maybe_unused]]` above `l:` |
| `(block (@ ATTR...) ITEM...)` | `[[likely]] { ... }`, the attributes a line above the `{` |

A body (`THEN`, `BODY`) is a `(block ...)`: cemit braces every body, so a
statement the source left unbraced is the block it prints as.

**A label is an item of its own**, before the statement it labels, as C writes
it: `case 1: case 2: x++;` is `(case 1) (case 2) (post++ x)`. The text is the
same, and a switch reads as one.

```
(switch c
  (block
    (case 'a')
    (case 'b')
    (= n 1)
    (break)
    (default)
    (return 0)))
```

## Expressions

| form | C | | form | C |
| --- | --- | --- | --- | --- |
| `(+ a b c)` | `a + b + c` | | `(call f a b)` | `f(a, b)` |
| `(- a b)` | `a - b` | | `(index a i j)` | `a[i][j]` |
| `(- a)`, `(+ a)` | `-a`, `+a` | | `(-> p a b)` | `p->a->b` |
| `(! a)`, `(~ a)` | `!a`, `~a` | | `(. s a)` | `s.a` |
| `(addr a)` | `&a` | | `(post++ a)`, `(post-- a)` | `a++`, `a--` |
| `(deref p)` | `*p` | | `(pre++ a)`, `(pre-- a)` | `++a`, `--a` |
| `(* a b)`, `(/ a b)`, `(% a b)` | | | `(cast T e)` | `(T)e` |
| `(<< a b)`, `(>> a b)` | | | `(sizeof e)` | `sizeof(e)` |
| `(< a b)` ... `(>= a b)` | | | `(sizeof-bare e)` | `sizeof e` |
| `(== a b)`, `(!= a b)` | | | `(sizeof-type T)` | `sizeof(T)` |
| `(& a b)`, `(^ a b)`, `(\| a b)` | | | `(alignof e)`, `(alignof-bare e)`, `(alignof-type T)` | the same, `alignof` |
| `(&& a b c)`, `(\|\| a b)` | | | `(literal T ITEM...)` | `(T){...}` |
| `(? c a b)` | `c ? a : b` | | `(generic e (T v) (default v))` | `_Generic(e, T: v, default: v)` |
| | | | `(literal static T ITEM...)` | C23's `(static T){...}`: storage classes before the type |
| `(= a b)`, `(+= a b)` ... | `a = b` | | `(stmt-expr ITEM...)` | `({ ... })` |
| `(comma a b)` | `a, b` | | `(label-addr L)` | `&&L` |
| `(paren e)` | `(e)`, see below | | `(macro "TEXT")` | a macro's invocation |

A left-nested run of one operator is one form: `a - b - c` is `(- a b c)`,
which is `(- (- a b) c)`. Assignment nests to the right, as it groups:
`a = b = 1` is `(= a (= b 1))`. A chain of `.`, `->` or `[]` is one form too.

**Parentheses.** A prefix form carries its grouping, so the parentheses C needs
are not written: `(* (+ a b) c)` is `(a + b) * c`. A pair the source has where
precedence does not need one is `(paren ...)`: `(a * b) + c` is `(+ (paren (* a
b)) c)` and `return (x);` is `(return (paren x))`. cemit keeps every pair the
source wrote, and so do the forms.

**`sizeof`.** `(sizeof e)` is the operand parenthesised -- `sizeof(buf)`, as
the source nearly always writes it -- and `(sizeof-bare e)` the operand as it
stands.

**Macros.** The front end expands the preprocessor; cemit recovers an
invocation's text where the whole of a node came from one (`errno`, `nullptr`,
`va_arg(ap, int)`), and so do the forms: an invocation that is one identifier is
that atom, `errno`, and any other is `(macro "va_arg(ap, int)")`. As a statement
it is that text with its `;`. The forms do not structure what is inside one:
the tree does not have it, only the expansion.

## What it refuses

`ToLisp` refuses what cemit refuses -- a nested function definition, a node of
the tree no form says -- with the position, never a silent omission. What the
front end does not parse never reaches either, and is refused there, with its
position: `[*]` without a qualifier but as an unnamed parameter's first
declarator (`int [*]` parses; `int a[*]` and `int (*)[*]` do not), and an
unnamed parameter's `[static 3]` or `[const n]` (`[static const 3]` and
`[const 4]` parse), and C23's `[[...]]` where upstream's parser takes GNU's
attributes and discards them -- just inside a declarator's parenthesis, `int
([[a]] *p)`, and before an abstract declarator -- which the fork refuses
rather than drops. `ToC` refuses a form it does not know, an argument
missing, a value where none can be.
