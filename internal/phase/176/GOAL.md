# Phase 176 — the regex engine's state is a parameter

The first of the two C phases a parallel `:%s` rests on
(`doc/PARALLEL-SUBSTITUTE.md`, `doc/IR.md`): the regex engine kept the match
in progress in file-scope objects, so no two matches could run at once. They
are now the members of one struct, and every function of the engine is
handed a pointer to it -- `crefactor/xform`'s `StateParam`, told the objects,
the names and where the parameter stops by `internal/whim`'s `RegEngine`:

```c
struct regengine_S
{
    regexec_T rex;             /* the match in progress, and */
    int rex_in_use;            /* its re-entry guard */
    garray_T regstack;         /* the backtracking stacks, */
    garray_T regstack_star;
    garray_T regstack_behind;
    garray_T backpos;
    int regstack_bytes;        /* and their 'maxmempattern' count */
    regsave_T behind_pos;      /* look-behind */
    long bl_minval;
    long bl_maxval;
    long brace_min[10];        /* \{n,m} */
    long brace_max[10];
    int brace_count[10];
    char_u *reg_tofree;        /* a back-reference's copy */
    unsigned reg_tofreelen;
    int reg_toolong;           /* set compiling, cleared matching */
};
```

`rex.reg_ic` is `re->rex.reg_ic`, and so on. The four functions the rest of
the editor calls the engine by -- `vim_regcomp`, `vim_regexec_multi`,
`vim_regexec_string`, `vim_regsub_multi` -- keep their signatures and bind
`re` to the one file-scope instance, `reg_engine`, so nothing their callers
see changes; the 43 functions below them take `regengine_T *re` first. A
caller of the engine's inner functions may now hand it an engine of its
own, which is what phase 177 does.

The step refuses a function that needs the state and is named other than
by a call (its address taken, or named by the host), a need that climbs past
every root, an object with an initialiser that is not zero (the instance is
zero-initialised; the four growarrays' were all-zero), a name it adds that
is taken, and a result that does not type-check. Two things it does to
declare the struct: `regsave_T`, a member's type declared after the first
object, moves up to it; and since `cstrncmp`'s prototype comes before the
struct's place, `regengine_T` is declared there as the typedef of a tagged
struct, defined where the objects were.

**Measured:** 16 objects, 43 functions, 4 roots; 75,381 -> 75,349 lines.
Nothing the editor does moves: the quick suite -- with the 22 `par_*`
cases added for phases 176-177, each a `:%s` over 3,000 numbered lines
built by keys and every line printed after it -- answers as the commit
before on the C, and the Go, Java and Clojure editors answer as the C; the
heavy case's times are the same (Go 0.6, Java 2.3, Clojure 8.0 times the
C).
