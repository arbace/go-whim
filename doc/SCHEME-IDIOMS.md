# How whimsical could be more idiomatic Scheme: a survey

2026-10-02. It covers `whimsical/whimsical/editor.ss` as tracked at
`598b500` (88,474 lines, 1,713 functions of the core and 17 of the host's
glue, written by `crefactor/togo`'s Scheme backend from the core `go tool
whim cut src/whim-vim.c` prints, 75,721 lines of C), and the hand-written
runtime `whimsical/whimsical/rt.ss`, host `host.ss`, terminal `term.ss` and
`printf.ss`. `doc/HASKELL-IDIOMS.md` and `doc/CLOJURE-IDIOMS.md` are its
model: every item says what the pattern is, how many sites it has, what it
would become, who would do it -- the Scheme printer (`crefactor/togo/scm.go`,
`scm_fn.go`, `scm_expr.go`), togo's analyses shared with the Haskell
backend (`cfacts.go`: `outparams.go`, `structvalues.go`, `effects.go`), the runtime -- its cost (S/M/L), its risk,
and how it is verified. `editor.ss` is generated and stays so: every item
done is a change to the backend, the library regenerated.

## The tension, stated first

whimsical was built to be C in Scheme's syntax (`doc/SCHEME.md`): every C
object in one bytevector laid out as amd64 lays it out, a pointer a fixnum
offset, every function a procedure of the editor, control flow as local
procedures and tail calls. That choice is why it was the least translation,
why it answered every case of both suites the first time it ran, and why it
runs at the C's speed (the heavy case 0.9-1.2 times the C, the parallel
`:%s` far faster). It is also most of what a Scheme programmer would object
to. Idiomatic Scheme is expressions nested as they are evaluated, names for
what values are, `cond` and `when` and named `let`, records and strings and
lists. Measured here, the two meet as follows:

- **Most of what reads as foreign is the printer's spelling, and it costs
  nothing at run time.** 29,299 temporaries (`r1`, `r2`...) bind every
  memory read and every call, 28,715 of them used exactly once: Scheme
  leaves the order of an application's arguments open, so the printer
  sequenced everything; but most expressions have one effect or only
  reads, which may be nested as written. 4,624 bindings copy one variable
  into another. The memory is addressed by numbers: 10,697 loads and stores
  of a file-scope object at a constant address, 8,942 of a member at a
  constant offset from a pointer. 8,690 uses of 1,006 named C constants
  and 2,327 character constants are printed as numbers. There is no
  `cond`, `when` or `unless` (509 `if`s have an `if` in their else),
  2,814 `begin`s, 1,608 `(void)`s. Chez compiles each of these to the
  same code either way.
- **Moving data out of the bytevector is not on offer.** The C walks its
  objects by pointer arithmetic (803 `p + n`, 436 `p++`, 3,343 `p[i]`,
  9,269 `p->m`, counted for HASKELL-IDIOMS.md on the same core), puns 50
  pointers, copies structs by bytes and allocates from an arena it never
  frees; a record per struct and a vector per array would be the
  Java's and the Clojure's object model (`doc/CLOJURE-PROFILE.md`), the
  parallel `:%s` allocating on the collected heap in every thread. What can
  change is how the bytevector is *named*: a file-scope object read and
  written like a variable, a member like a record's field.
- **Speed is the constraint that decides the rest**, and every item below is
  held to the heavy case's time, before and after, besides the suites.

## How it was measured

Counted or run, not estimated, unless it says so; the instruments were
throwaway, in `.tmp/scm/idioms/` (not tracked):

| Instrument | What it gave |
| --- | --- |
| `measure.py` (reads `editor.ss`) | the library split into its top-level procedures; the loads and stores by address shape (a constant, a pointer plus a constant, the frame), the printer's temporaries and how often each is used, copy bindings, truth-value conversions, the forms by head (`let`, `if`, `case`, `cond`, `when`, `begin`), the nesting depth |
| `shape.py` (the same) | each local loop and the places outside its body that enter it; the `if`s whose else is an `if` |
| a throwaway `togo` test (`zz_scm_survey_test.go`, removed) | the core parsed: the enumerators and character constants the functions use |
| `hseffects.go` (the Haskell backend's analysis, which the Scheme backend asked; `effects.go`, shared, now) | the functions that touch no memory |
| `whim test --scheme`, `--wide --scheme` | the verification recipe, run at `598b500`: all 80 and all 240 cases as the C, the control seen by 76 and by 94 + 6; the heavy case 0.9-1.2 times the C |

**Verification recipe for every item done** (the suites compare output byte
for byte, so a change that moves one byte of one screen is seen):

- `crefactor/togo`'s `TestScm*` (23 foreign C programs, each run under Chez
  at `optimize-level 2` -- safe: a fixnum operation on what is not one is an
  error -- and required to print what gcc's build prints; the same four with
  every out-parameter in the frame and with the pure functions bare) and
  `TestScmControl`;
- `go tool whim test --scheme` (80 cases, the control seen by 76) and `--wide
  --scheme` (240), and the heavy case's time beside the C's;
- `make whim-editor-check` after the regenerated `editor.ss` is committed;
- Chez's time and peak on the core, printed by every build.

## The shape of the output today

| | count |
| --- | ---: |
| lines | 88,474 |
| top-level procedures | 1,731: the core's 1,713, the host's 17, `new-editor` |
| loads / stores | 20,681 / 6,212 |
| ... of a file-scope object (a constant address) | 10,697 |
| ... of a member (a pointer plus a constant) | 8,942 |
| ... in the call's frame | 2,303 |
| the printer's temporaries `[rN ...]` | 29,299, 28,715 used once |
| copy bindings `[x y]` | 4,624 |
| local procedures: joins / loops | 3,536 / 906 |
| `let`/`let*` forms | 19,804 |
| `if` / `case` / `cond` / `when`+`unless` / `begin` | 8,854 / 70 / 0 / 0 / 2,814 |
| `(void)` results | 1,608 |
| `(not (fxzero? x))` (an int as a truth value) / `(b->i b)` / `(not (fx=? a b))` | 1,714 / 361 / 1,853 |
| multiple values (`let-values`) | 190: 64 struct results and 2 out-parameters as values, 148 struct locals as bindings |
| nesting depth (parentheses) | median 11, at most 36 |

## What a Scheme programmer would find most jarring

Three procedures, as tracked, with the C they come from.

**1. Every read and call bound, every value copied** (`skipwhite`; the C is
`while (*p == ' ' || *p == '\t') ++p; return p;`):

```scheme
(define (skipwhite ed q)
  (let ([mem (ed-mem ed)])
    (define (loop1 p)
      (let* ([r1 (ld-u8 p)]
             [r3 (or (fx=? r1 32)
                     (let ([r2 (ld-u8 p)])
                       (fx=? r2 9)))])
        (if r3
            (let ([p (fx+ p 1)])
              (loop1 p))
            p)))
    (let ([p q])
      (loop1 p))))
```

After items 1, 4, 5 and 6 -- the same reads, in the same order:

```scheme
(define (skipwhite ed q)
  (let ([mem (ed-mem ed)])
    (let loop ([p q])
      (if (or (fx=? (ld-u8 p) (ch #\space)) (fx=? (ld-u8 p) (ch #\tab)))
          (loop (fx+ p 1))
          p))))
```

**2. Addresses for names** (`check_cursor_lnum`; the C is `if
(curwin->w_cursor.lnum > curbuf->b_ml.ml_line_count) curwin->w_cursor.lnum =
curbuf->b_ml.ml_line_count; if (curwin->w_cursor.lnum <= 0)
curwin->w_cursor.lnum = 1;`):

```scheme
(define (check_cursor_lnum ed)
  (let ([mem (ed-mem ed)])
    (define (join2)
      (let* ([r1 (ld-ptr 68264)]
             [r2 (ld-s64 (fx+ r1 16))])
        (if (<= r2 0)
            (let ([r3 (ld-ptr 68264)])
              (st-s64! (fx+ r3 16) 1)
              (void))
            (void))))
    (let* ([r4 (ld-ptr 68264)]
           [r5 (ld-s64 (fx+ r4 16))]
           [r6 (ld-ptr 68304)]
           [r7 (ld-s64 r6)])
      (if (> r5 r7)
          (let* ([r10 (ld-ptr 68264)]
                 [r8 (ld-ptr 68304)]
                 [r9 (ld-s64 r8)])
            (st-s64! (fx+ r10 16) r9)
            (join2))
          (join2)))))
```

`68264` is `curwin`, `68304` `curbuf`, `16` `w_cursor.lnum`. After items 1,
2 and 3 -- the same reads, in the same order (C reads `curwin` again after
the store):

```scheme
(define (check_cursor_lnum ed)
  (let ([mem (ed-mem ed)])
    (define (join2)
      (when (<= (win_T.w_cursor.lnum curwin) 0)
        (win_T.w_cursor.lnum-set! curwin 1)))
    (if (> (win_T.w_cursor.lnum curwin) (buf_T.b_ml.ml_line_count curbuf))
        (begin
          (win_T.w_cursor.lnum-set! curwin (buf_T.b_ml.ml_line_count curbuf))
          (join2))
        (join2))))
```

**3. A loop as a procedure entered once, a call's value copied twice**
(`vim_strchr`'s first loop):

```scheme
    (define (loop3 p)
      (let* ([r1 (ld-u8 p)]
             [b r1])
        (if (not (fx=? b 0))
            (if (fx=? b c)
                p
                (let* ([r2 (utfc_ptr2len ed p)]
                       [t1 r2]
                       [p (fx+ p t1)])
                  (loop3 p)))
            0)))
```

After items 1, 3 and 4:

```scheme
    (define (loop3 p)
      (let ([b (ld-u8 p)])
        (cond
          [(fx=? b 0) 0]
          [(fx=? b c) p]
          [else (loop3 (fx+ p (utfc_ptr2len ed p)))])))
```

## The findings

### 1. Reads and calls where they are used

- **The pattern.** Every memory read and every call is bound to a
  temporary before the expression that uses it (`scm_expr.go`'s `readAt`
  and `call`), since Scheme evaluates an application's arguments in an
  order it chooses (Chez's is right to left) and C's (the Java's, which the
  lowering keeps) is left to right. 29,299 temporaries; 28,715 (98%) used
  once.
- **What it would become.** A value carries what evaluating it does --
  nothing, reads, or a call -- and stays in place, nested where it is used,
  unless putting it there could reorder it against another: an operand
  whose combination has another operand that calls, or reads after one that
  calls, is bound first, in C's order; reads commute with reads. A
  temporary is left only where two effects meet in one expression.
- **Where:** the printer (`scm_expr.go`: a value's level, and one rule that
  orders a combination's operands; `scm_fn.go`: the places that put a value
  somewhere). **Cost:** M. **Risk:** medium -- an order wrongly relaxed is
  a wrong answer; the foreign C tests' `TestScmSteps` and `TestScmIntegers`
  and both suites are built to see one. **Speed:** none expected (Chez's
  register allocator sees the same dataflow).

### 2. The file-scope objects and the members by name

- **The pattern.** 10,697 loads and stores of a file-scope object at its
  constant address (`(ld-ptr 68264)` is `curwin`); 8,942 of a member at a
  constant offset from a pointer (`(ld-s64 (fx+ wp 16))` is
  `wp->w_cursor.lnum`).
- **What it would become.** A file-scope scalar a variable of the library
  as R6RS spells one that is not a location of Scheme's: `identifier-syntax`
  with `set!` -- `curwin` reads it, `(set! curwin wp)` writes it, `&curwin`
  is its address; an array or a struct its address by name. A member read
  through an accessor of its struct's and its path's name, as a record's
  field is: `(win_T.w_cursor.lnum wp)`, `(win_T.w_cursor.lnum-set! wp 1)`,
  `(win_T.w_cursor& wp)` its address. Each name is defined, once, for what
  the library uses, as a macro of the same load or store: the compiled code
  is the same.
- **Where:** the printer (the address it carries keeps the object or the
  struct and the members' path it was made of, as `hsnamed.go` does for
  the Haskell). **Cost:** M. **Risk:** low (a name for a number).

### 3. `cond`, `when` and `unless`; no `begin`, no `(void)`

- **The pattern.** Branches are `if` alone: 509 `if`s whose else is another
  `if` (a `cond`); arms of several forms wrapped in `begin` (2,814); a
  void function's result `(void)` (1,608), often an `if`'s arm.
- **What it would become.** `cond` for a chain, its clauses taking several
  forms; `when`/`unless` for an `if` whose other arm is `(void)`; a void
  result's `(void)` left out after another form.
- **Where:** the printer (`scm_fn.go`'s `scmIf` and the result). **Cost:**
  S. **Risk:** none (the same tree).

### 4. Copies and a value passed on

- **The pattern.** 4,624 bindings copy one variable into another (`[b r1]`,
  `[p q]`, `[t1 r2]`): the C's `p = q`, the lowering's temporaries; and a
  value bound only to be passed to the next jump (`(let ([p (fx+ p 1)])
  (loop3 p))`).
- **What it would become.** A variable that is another's value is that
  variable (`(loop1 q)`), until the other is bound anew; a value used once,
  in the jump that follows, is the jump's argument.
- **Where:** the printer's bindings (`setVar`). **Cost:** S-M. **Risk:**
  low-medium (a name reused after its variable is rebound is a wrong value;
  the suites see it).

### 5. A loop as a named `let`

- **The pattern.** A loop is a local procedure defined at the function's
  top and called by name (`(define (loop1 p) ...)` ... `(loop1 p)`): 906
  loops, of which 464 are entered from one place outside their body.
- **What it would become.** For those, `(let loop1 ([p q]) ...)` where it
  is entered.
- **Where:** the printer's shape. **Cost:** M. **Risk:** low (scope: every
  jump back is inside the body).

### 6. Named constants and characters

- **The pattern.** 8,690 uses of 1,006 enumerators (`NUL`, `ESC`, `K_DEL`,
  `OK`, the preprocessor's constants as phase 0 left them) and 2,327
  character constants are printed as their numbers.
- **What it would become.** An enumerator a constant of the library by its
  name (`(define-syntax ESC (identifier-syntax 27))`), a character
  `(ch #\a)` (the runtime's, its code at expansion time); a `case` label
  keeps its number -- a `case` takes data, not names -- with the name in a
  comment.
- **Where:** the printer. **Cost:** S-M. **Risk:** low.

### 7. Pure functions without the editor

- **The pattern.** Every function takes the editor; 63 touch no memory,
  even through what they call (`effects.go`), e.g. `musl_isdigit`.
- **What it would become.** Those take only their C parameters (the
  backend's `ScmPure`, already held to the foreign C tests).
- **Where:** the profile. **Cost:** S. **Risk:** low.

### 8. Truth values

- **The pattern.** 1,714 `(not (fxzero? x))`, an `int` tested as a truth
  value; 361 `(b->i b)`, a bool made a number; 1,853 `(not (fx=? a b))`.
- **What it would become.** Phases 166, 183 and 184 made the C's answers
  `bool` already; what is left is the C's own ints (`got_int`, counts
  tested for zero) and comparisons. `(fxzero? x)` with the arms swapped is
  shorter; the rest is the C's.
- **Where:** the printer. **Cost:** S. **Risk:** low. **Value:** small.

### Done before this survey

- **Multiple values** (milestone 1): a struct of scalars a function returns
  is `values` (64 functions), an out-parameter a value in and out (2 left
  by phase 181), a struct local of scalars bindings (148) -- the decisions
  of `outparams.go` and `structvalues.go`, shared with the Haskell.
- **The host as a record** (milestone 3): `host.ss`'s `host` record of
  procedures, R6RS's buffer convention.
- **Every value a binding, no `set!`**: each assignment is a new binding of
  the variable's name (`let*`, shadowing), as the lowered form has it.

### Declined, with why

- **Records for the C's structs, vectors for its arrays.** The C reaches its
  objects through pointers into their middle, walks them by arithmetic,
  puns 50 pointers and copies structs as bytes; records would need the
  pointer analysis the Java's and the Clojure's needed and their speed
  (`doc/CLOJURE-PROFILE.md`), and put every object on the collected heap
  under the parallel `:%s`. Item 2 gives the reading of records without
  moving the data.
- **Scheme strings for C strings.** A C string is a mutable array of bytes
  addressed by pointer arithmetic, written in place (`STRCPY`, `IObuff`);
  a Scheme string is neither. The literals already carry their text in the
  library, `(c-str 81234 "text")`.
- **The joins nested where they are used** (each local procedure defined in
  the body of the block that dominates its uses, closing over what is in
  scope there): fewer parameters and a deeper, more Scheme-like nesting, but
  a different shape from the one caprice measured; not taken here. (Taken
  in the second pass, item 18.)

## Ranked

| # | item | sites | cost | risk | value |
| --- | --- | ---: | --- | --- | --- |
| 1 | reads and calls in place -- **done** | 29,299 temporaries | M | medium | high |
| 2 | objects and members by name -- **done** | 10,697 + 8,942 | M | low | high |
| 3 | `cond`, `when`, no `begin`/`(void)` -- **done** | 509 + 2,814 + 1,608 | S | none | medium |
| 4 | copies and values passed on -- **done** | 4,624 | S-M | low-medium | medium |
| 5 | loops as named `let` -- **done** | 464 of 906 | M | low | medium |
| 6 | named constants and characters -- **done** | 8,690 + 2,327 | S-M | low | medium |
| 7 | pure functions without the editor -- **done** | 63 | S | low | low |
| 8 | truth values -- **done** | 1,714 | S | low | low |
| 9 | arms that end alike (found doing 3 and 8) -- **done** | 1,298 | S | none | medium |

## Done

Each item below was built in the backend, the library regenerated and
committed, and held to the recipe above: the foreign C tests, both suites
with `--scheme` (all 80 and all 240 as the C, the control seen as before),
`whim-editor-check`, and the heavy case and Chez's time on the core beside
the C's and the previous build's. The counts are `measure.py`'s.

### 1. Reads and calls in place

A value carries what evaluating it does (`scm_expr.go`'s `lvl`: nothing,
reads, a call), and a combination's operands are readied by one rule
(`seq`): their lines first, in C's order; an operand that reads or calls
stays in place unless another operand's lines run before the combination,
or it would be reordered against a call -- reads commute with reads, so a
call stays in place only when no operand in place comes before it and none
that reads after it, and the rest are bound, in C's order, to a temporary.
A variable another operand's lines bind anew (an out-parameter's value) is
read before them. The left of `and`/`or` and an `if`'s test stay strict;
the right and the arms take their own lines with them. An address read and
written by `+=` or `++` is computed once, a struct's members read in place,
a result read before the frame is given back.

| | before | after |
| --- | ---: | ---: |
| lines | 88,474 | 57,892 |
| the printer's temporaries | 29,299 (28,715 used once) | 660 (433) |
| copy bindings `[x y]` | 4,624 | 1,599 |
| Chez on the core | 29.2 s, 0.72 GB | 26.4 s, 0.70 GB |
| the heavy case | 0.9-1.2 times the C | 0.9-1.0 (C 425-438 ms, whimsical 390-427) |

The loads and stores are the same 20,681 and 6,212: nothing read or written
moved, only where it is written.

### 2. The file-scope objects and the members by name

The runtime has four forms the library defines its names with
(`rt.ss`): `define-c-object` -- a file-scope scalar read by its name and
written by `set!` (a variable transformer, `identifier-syntax`'s kin),
`&name` its address; an array or a struct its address, as C has it --
`define-c-member`, a member's accessor of its struct's name and the
members' path, `(win_T.w_cursor.lnum wp)`, `(win_T.w_cursor.lnum-set! wp
v)`, `(win_T.w_cursor.lnum& wp)`; and `define-c-local`, a local of the C's
that lives in the call's frame, by its name in the function. Each expands
to the load or store it replaces. The printer's address keeps the object or
the local it is, or the struct and the path it was made of
(`scm_expr.go`'s `member`); a struct the C has no name for, a name two
types share, or two kinds at one name keep their numbers. A parameter that
lives in the frame comes in as `name.in` and is copied into `name`.

| | before | after |
| --- | ---: | ---: |
| lines | 57,892 | 60,104 (1,716 lines of definitions) |
| raw loads / stores (`ld-`, `st-`) | 20,681 / 6,212 | 3,324 / 1,076 |
| ... of a file-scope object by address | 10,697 | 0 |
| ... of a member by offset | 7,376 | 870 (a struct with no name, or through an array member's element) |
| ... in the frame by offset | 2,488 | 77 (a struct a call returns) |
| names defined | -- | 803 objects, 913 members, 389 locals |
| Chez on the core | 26.4 s, 0.70 GB | 27.4 s, 0.77 GB |
| the heavy case | 0.9-1.0 times the C | 0.9-1.0 (C 431-436 ms, whimsical 403-429) |

(`measure.py`'s copy bindings went 1,599 -> 2,241: a binding of a named
object's value, `[r1 vcol]`, now reads as one.)

### 3. `cond`, `when` and `unless`; no `begin`, no `(void)`

A branch's arms are body forms (`scm_fn.go`'s `ifForm`): an arm that does
nothing makes it a `when` or an `unless`; an else that is itself a branch
goes on into a `cond`, and an `(if (not x) (if ...) e)` is `(cond [x e]
...)`; an arm of several forms is a `cond` clause, which needs no `begin`;
a `case` clause takes its forms as they are; a void function's `(void)`
after another form goes.

| | before | after |
| --- | ---: | ---: |
| lines | 60,104 | 55,973 |
| `if` / `cond` / `when`+`unless` | 8,854 / 0 / 0 | 3,227 / 3,512 / 489 |
| `begin` | 2,814 | 35 (an expression's effects before its value) |
| `(void)` | 1,608 | 114 (an arm with nothing else to do) |
| Chez on the core | 27.4 s, 0.77 GB | 27.3 s, 0.73 GB |
| the heavy case | 0.9-1.0 times the C | 0.9-1.0 (C 436-442 ms, whimsical 412-428) |

### 4. Copies and values passed on

A variable given another's value, or a constant, is that value, bound to
nothing, until the other is bound anew: then each variable that was a copy
of it gets a binding of its own first (`aliases`). A block that ends in a
jump passes in place what its last lines bind only to pass on -- from the
last line back, while the name is used once in the jump and at most one of
the values moved reads or calls (`jumpOn`) -- and a value bound only to be
the result is the result.

| | before | after |
| --- | ---: | ---: |
| lines | 55,973 | 51,126 |
| bindings of a name to a name (`[x y]`, a named object's read among them) | 2,241 | 776 |
| Chez on the core | 27.3 s, 0.73 GB | 29.5 s, 0.74 GB (the load average 9-17 meanwhile) |
| the heavy case | 0.9-1.0 times the C | 1.0 (C 432-436 ms, whimsical 413-427) |

### 5. Loops as named `let`

A loop's head that one place outside its body enters is written there as
a named `let` of its body, its parameters bound to the jump's arguments,
where it was a procedure of the function's and a call (`namedLets`, after
the printing, until none is left: a loop entered from another loop's body
goes in with it). A loop entered from two places, or from a join, stays a
procedure.

| | before | after |
| --- | ---: | ---: |
| loops as `(define (loopN ...))` / as `(let loopN (...))` | 906 / 0 | 429 / 477 |
| lines | 51,126 | 52,947 (a named let's body indented where it is entered) |
| Chez on the core | 29.5 s, 0.74 GB | 27.5 s, 0.74 GB |
| the heavy case | 1.0 times the C | 0.9-1.0 (C 432-445 ms, whimsical 405-433) |

### 6. Named constants and characters

A constant the C spells as an enumerator is that name, and the library
defines it, `(define ESC 27)`, its value the literal of the type it is
first used at (`SIZE_MAX`, an `unsigned long`, is 18446744073709551615 --
a first build printed it as -1 and the suites refused every case); a
character constant is `(ch #\a)`, the runtime's, its code at expansion. A
name stays while the value is its value: a conversion that changes it, or
makes it a truth value, prints the number. A `case` label keeps its number
(`case` takes data), and so does an expression the front end folded (`A |
B`).

| | before | after |
| --- | ---: | ---: |
| enumerators by name | 0 | 755 defined, 4,673 uses |
| characters by name | 0 | 1,885 |
| lines | 52,947 | 53,890 (the definitions) |
| Chez on the core | 27.5 s, 0.74 GB | 29.0 s, 0.73 GB (the load average 7-8) |
| the heavy case | 0.9-1.0 times the C | 0.9-1.1 (C 427-429 ms, whimsical 398-460) |

### 7. Pure functions without the editor

`whim.Gen` turns on the backend's `ScmPure`: the 63 functions that touch no
memory, even through what they call (`effects.go`), take only their C
parameters -- `(define (musl_isalnum c) (or (musl_isalpha c) (musl_isdigit
c)))` -- and their callers pass no editor; one taken as a function pointer
is wrapped in the table to take the editor a call through a pointer passes.
What the host calls back and what a runtime body names keep it.

| | before | after |
| --- | ---: | ---: |
| functions without `ed` | 0 | 63 |
| Chez on the core | 29.0 s, 0.73 GB | 28.2 s, 0.74 GB |
| the heavy case | 0.9-1.1 times the C | 0.9-1.0 (C 428-435 ms, whimsical 400-412) |

### 8. Truth values

A test that is `(not x)` swaps its arms: `(if (not x) a b)` is `(if x b
a)`, `(when (not x) ...)` an `unless`, and the reverse; `(not (not x))` is
`x`; a truth value compared with 0 or 1 (`FALSE`, `TRUE`, `OK`, `FAIL` the
C's own) is itself or its negation. An `int` the C tests stays `(not
(fxzero? x))` where it is a value, not a test.

| | before | after |
| --- | ---: | ---: |
| `(not ...)` | 4,936 | 2,763 |
| `(not (fxzero? x))` / `(not (fx=? a b))` | 1,714 / 1,853 | 850 / 1,206 |
| `(b->i b)` | 361 | 344 |
| lines | 53,884 | 53,676 |
| Chez on the core | 28.2 s, 0.74 GB | 28.8 s, 0.73 GB |
| the heavy case | 0.9-1.0 times the C | 0.9-1.0 (C 434-457 ms, whimsical 390-447) |

### 9. Arms that end alike (found while doing 3 and 8)

`tails.py` counted 1,298 of the 5,564 two-way branches whose two arms end
in the same form -- most often the same jump to a join, `(cond [c ...
(join16 ...)] [else (join16 ...)])`. A branch's arms are now printed once
up to where they meet, and what both end with follows the branch, once:
`(when c ...) (join16 ...)`.

| | before | after |
| --- | ---: | ---: |
| two-way branches whose arms end alike | 1,298 | 0 |
| lines | 53,676 | 50,838 |
| `if` / `cond` / `when`+`unless` | 3,190 / 3,553 / 489 | 3,328 / 2,328 / 1,860 |
| Chez on the core | 28.8 s, 0.73 GB | 26.6 s, 0.63 GB |
| the heavy case | 0.9-1.0 times the C | 0.9 (C 446-450 ms, whimsical 395-403) |

## The second pass (2026-10-02)

From `2593f8f`, where the nine items above left `editor.ss` at 50,838
lines. What a Scheme programmer would still object to was counted again on
that library (a throwaway reader of its forms, `.tmp/scm2/measure.py`,
`m2.py`-`m4.py`, not tracked), and most of it turned out to be of one
kind: the printer composes its forms as text, a block at a time, and what
is wrong shows only once a function is whole -- a line of a thousand
columns, a join that one place calls, a loop's parameter that every jump
back passes unchanged, a copy bound only to be read once. So most of this
pass is one new piece of the backend, `crefactor/togo/scm_tidy.go`: each
function the printer wrote is read back as forms, rewritten by rules each
of which keeps what the function does, and printed again in a layout of its
own (the Clojure backend's `clj_tidy.go` is its precedent). The rules see
the text's scopes -- what each `let`, named `let`, `let-values`, `define`
and `lambda` binds -- and the names that read memory (the C's objects and
the frame's locals, which are macros of a load); each refuses where a name
it would move could mean another binding, or another value, where it lands.

Each item done was held to the whole recipe of the first pass and to more:
`cd crefactor && go test ./togo` with a test of its own for each rule --
`TestScmTidy*` on texts, and `TestScmTidy` a foreign C program every rule
rewrites, run under Chez and held to gcc's output -- `go test ./whimsical`,
`whim test --scheme` (all 80, the control seen by 76), `--wide --scheme`
(all 240, the control seen by 94 keys and 6 pty cases), `--scheme-debug`
(the safe build at optimize-level 2: all 80), the heavy case, `make
whim-editor-check`, Chez's time and peak on the core, and gofmt, go vet
and staticcheck on `crefactor/togo` and `whimsical/`. No shared analysis
changed: `Editor.hs`, its parts and `editor.rs` are byte for byte what they
were (sha256, before and after every item).

### What was left, measured

| | at `2593f8f` |
| --- | ---: |
| lines | 50,838 |
| lines of the functions over 100 columns / over 160 / the longest | 6,674 / 3,267 / 1,017 |
| named lets begun after other forms on a line, their body under the line's end | 124 |
| joins (local procedures that are not loops) / called from one place | 3,538 / 1,228 |
| named lets' bindings / of a name to itself that every jump back passes on | 2,797 / 2,001 |
| the printer's temporaries `rN` / the lowering's `tN` (an increment's old value, `*d++`) | 660 / 470 |
| `(and (and ...) ...)` / `(or (or ...) ...)` | 803 / 399 |
| a comparison with zero: `(fx=? x 0)` / `(= x 0)`, `(eqv? x 0)` | 1,948 / 132 |
| `(if (not x) ...)` | 175 |
| an `if` whose else is an `if` (an expression's, which item 3 left) | 72 |
| `case` clauses / doing what an earlier clause does / what the else does | 961 / 377 / 8 |
| `(void)` | 109 |
| case labels as numbers | 961 (all) |
| signed `int` and `long` arithmetic through a wrapping helper (`i32+ i32- i32* i64+ i64- i64*`) | 4,340 |
| unsigned (`u32+`, `u64-`...), shifts, division | 748 |
| calls of the core's `musl_memmove` / `musl_memset` / `musl_memcpy`, each a loop a byte at a time | 151 / 69 / 8 |
| raw loads / stores | 3,324 / 1,076: 2,299 `(ld-u8 p)`, 565 `(st-u8! p v)` |
| ... of an array's element, `(ld-ptr (fx+ a (fx* i 8)))` | 464 |
| aggregates in a call's frame / reached only through their members | 230 / 0 |
| the host's glue called by its index, `(vector-ref (ed-glue ed) 3)` | 17 |

### Ranked

| Rank | Item | Who | Cost | Risk | Sites |
| --- | --- | --- | --- | --- | ---: |
| 1 | 10: a layout of the forms' own: lines within 100 columns, a named let under its line -- **done** | `scm_tidy.go` (reader, printer) | M | none: the same forms | 6,674 lines, 124 lets |
| 2 | 11: a join one place calls written where it is called -- **done** | `scm_tidy.go` | M | low: scopes checked | 1,228 joins |
| 3 | 12: a named let's bindings that never change taken out -- **done** | `scm_tidy.go` | S | low: scopes checked | 2,001 bindings |
| 4 | 13: copies, constants and increments in place -- **done** | `scm_tidy.go` | M | low-medium | 470 `tN`, copies |
| 5 | 14: spellings: `and`/`or` flat, `fxzero?`, `(if (not x))` turned, an expression's if-chain a `cond`, a case's clauses that do the same one clause, `(void)` -- **done** | `scm_tidy.go` | S | none | 1,202 + 2,080 + 175 + 72 + 385 + 109 |
| 6 | 15: signed arithmetic as C means it, `fx+` and `+` -- **done** | `scm_expr.go`, `scm_fn.go` | S | the honest one: an overflow C leaves undefined no longer wraps | 4,340 |
| 7 | 16: the memory functions as the bytevector's own -- **done** | `internal/whim/gen.go`'s runtime bodies, `rt.ss` | S | low | 228 calls, 3 bodies |
| 8 | 17: case labels by name -- **done** | `scm_fn.go`, `rt.ss` (`c-case`) | M | low | 961 labels |
| 9 | 18: the joins nested where their calls are (declined in the first pass, re-examined) -- **done** | `scm_tidy.go` | M | low: scopes checked; the speed measured | 13,240 of 16,021 parameters passed unchanged |
| -- | an array's element by index, `(ld-ptr@ a i)` | -- | S | low | 464: declined, below |
| -- | raw byte loads named | -- | -- | -- | declined, below |
| -- | records for struct locals, Scheme strings, the host's glue a record | -- | -- | -- | declined, below |

### 10. A layout of the forms' own

`scm_tidy.go` reads each function the printer wrote back as forms (`scmRead`:
atoms, strings, characters, `'()`; anything else, a comment, refuses the
function) and prints it again (`scmPrint`): a form on one line where it fits
within 100 columns, closing brackets counted, or where it is 32 columns or
fewer; else by what it is -- a `define`, a `let` and a named `let` with
their bindings one to a line under the first and the body indented two, a
`cond` and a `case` a clause to a line, an `if` its arms under its test, a
`when` its body under it, `and` and `or` an operand to a line, a call its
arguments under the first, as many to a line as fit. So a named let is
always a body on lines of its own, where the text the printer composed had
put it after an `if`'s test, its body indented under the line's end
(`del_bytes`, 80 columns in).

| | before | after |
| --- | ---: | ---: |
| lines | 50,838 | 64,591 |
| lines of the functions over 100 columns / over 160 / the longest | 6,674 / 3,267 / 1,017 | 163 / 1 / 195 (a form 30 levels deep) |
| named lets begun after other forms on a line | 124 | 0 |
| Chez on the core | 26.6 s, 0.63 GB (36.8 s under this run's load) | 29.2 s, 0.63 GB |
| the heavy case, nine runs interleaved, the load average 52-54 | C 471-621 ms, whimsical 494-665 | whimsical 490-677 |

The forms are the same, so the compiled code is: what moved is lines.

From here on the heavy case's wall time is the machine's load as much as
the editor's (the load average was 50-55 through this pass, other agents
building): nine runs interleaved moved by 20 % between two builds of the
same forms. So each item is also measured by `perf stat` on the heavy case
with one worker (`WHIMSICAL_WORKERS=1`): the instructions the editor
executes, which the load does not move, and the least cycles of seven runs.
For item 10: 6,782M instructions before and after, at least 1,902M and
1,905M cycles.

### 11. A join one place calls, where it is called

A join -- a block several jumps reach, a local procedure of the function's
-- that the printing left called from one place (most of them because item
9 wrote the two arms' jumps to it as one, after the branch) is written
there (`scmJoinsInPlace`): its body in place of the call, its parameters
bound to the call's arguments by a `let` but where the argument is the
parameter's own name, which is already that value there. It refuses where
a binding around the call would capture a name its body reads, and where a
parameter is a name that reads memory. Several forms where one goes -- an
`if`'s arm -- make the `if` a `cond`.

`lalloc`, before and after:

```scheme
(define (lalloc ed size message)
  (let ([mem (ed-mem ed)])
    (define (join2)
      (host_alloc ed size))
    (when (= size 0) (set! emsg_silent 0) (iemsg ed e_internal_error_lalloc_zero))
    (join2)))

(define (lalloc ed size message)
  (let ([mem (ed-mem ed)])
    (when (= size 0) (set! emsg_silent 0) (iemsg ed e_internal_error_lalloc_zero))
    (host_alloc ed size)))
```

### 12. A named let's bindings that never change

A named let's binding of a name to itself (`[col col]`) that every jump
back passes on unchanged -- the name not bound again on the way -- goes,
from the binding and from every jump (`scmInvariants`): the loop's body
sees the binding outside it, which is the same value. In `del_bytes`, `(let
loop6 ([oldp oldp] [oldlen oldlen] [lnum lnum] [n col]) ... (loop6 oldp
oldlen lnum n))` is `(let loop6 ([n col]) ... (loop6 n))`.

| | before | after 11 and 12 |
| --- | ---: | ---: |
| lines | 64,591 | 59,265 |
| joins as local procedures | 3,538 | 2,310 (1,228 written where they are called; none refused) |
| named lets' bindings / of a name to itself | 2,797 / 2,228 | 771 / 202 (2,026 taken out) |
| Chez on the core | 29.2 s, 0.63 GB | 31.5 s, 0.69 GB (the load average 51-54) |
| the heavy case: instructions / least cycles | 6,782M / 1,905M | 6,777M / 1,907M; in the suite 0.8-0.9x the C |

A loop's invariants read from outside it are free variables of the loop's
procedure where they were its arguments; Chez compiles the two alike here
(the instructions above), though the wall time of nine runs under the
load suggested otherwise until it was measured so.

### 13. Copies, constants and increments in place

The lets are taken apart into lets of one binding each (`scmSplitLets`: a
`let*`'s always, a `let`'s where no value names what the let binds), and
a binding goes where its value can be written where its name is read
(`scmPropagate`): a constant or another binding's name, wherever it is
read; a pure value -- C's arithmetic but division, conversions,
comparisons, a member's address, of names that read no memory -- where it
is read once, and not inside a loop's or a procedure's body. A use where
one of the value's names means another binding refuses it. What is left is
joined again (`scmMergeLets`: a let whose body is one let, one `let*`), and
a let left with no bindings is its body (`scmUnlet`; several forms as an
`if`'s arm make it a `cond`). Item 12's rule then also binds around the
loop a pure value the loop never changes, where no other binding of the
loop names it (`(let ([a s]) (let loop1 ([s s]) ...))` in `musl_strlen`).
So `*d++ = *s++` (`musl_memcpy`) is

```scheme
      (cond
        [(eqv? n 0) dest]
        [else (st-u8! d (ld-u8 s)) (loop1 (u64- n 1) (fx+ d 1) (fx+ s 1))])
```

where it was `(let* ([t1 d] [d (fx+ d 1)] [t2 s] [s (fx+ s 1)]) (st-u8! t1
(ld-u8 t2)) (loop1 (u64- n 1) d s))`.

| | before | after |
| --- | ---: | ---: |
| lines | 59,265 | 59,249 |
| bindings written where they are read | -- | 776 |
| the lowering's temporaries `tN` | 470 | 325 (an old value read after the name is bound anew: `n++ == maxlen`) |
| bindings of a name to a name | 832 | 616 (336 the printer's `rN` of a named object: `(let ([r1 curwin]) (win_T.w_cursor.col-set! r1 ...))`) |
| named lets' bindings | 771 | 649 (122 more bound around their loop) |
| Chez on the core | 31.5 s, 0.69 GB | 31.8 s, 0.70 GB |
| the heavy case: instructions / least cycles | 6,777M / 2,044M | 6,904M (+1.9 %) / 1,957M; in the suite 0.7-1.4x the C under the load |

The instructions grew by 1.9 %: a value bound around a loop is read from
the loop's closure where it was an argument in a register. Kept: the
cycles did not move beyond the load's noise.

### 14. Spellings

The last rules spell shorter what the printer spelled long (`scmSpell`):
`(and (and a b) c)` is `(and a b c)`; `(fx=? x 0)` is `(fxzero? x)` and
`(= x 0)` `(zero? x)`; `(if (not x) a b)` is `(if x b a)`, `(when (not x)
...)` an `unless`; an expression's `if` whose else is an `if` or a `cond`
is a `cond`; a `case`'s clauses that do the same are one clause, their
labels together, and one that does what the else does goes (C's labels are
distinct: no clause's order matters); in a function whose value nothing
reads, a `(void)` after another form goes (`scmDropVoids`).

An unsigned long's `(eqv? n 0)` stays: rewritten as `(zero? n)` it cost
4.8 % more instructions on the heavy case (7,235M against 6,904M) --
`eqv?` with a fixnum is one comparison, `zero?` a generic test, and
`musl_memmove`'s byte loop tests it each byte.

| | before | after |
| --- | ---: | ---: |
| `(and (and ...))` / `(or (or ...))` | 803 / 400 | 0 / 1 (an `or` the tidy made) |
| `(fx=? x 0)` / `(= x 0)` | 1,948 / 91 | 0 / 0: `fxzero?` and `zero?` (178) |
| `(if (not x) ...)` | 175 | 0 |
| an expression's `if` whose else is an `if` | 73 | 1 |
| `case` clauses (labels) | 961 (961) | 576 (953): 377 joined with another, 8 the else's |
| `(void)` | 109 | 12 (an arm with nothing else to do) |
| `if` / `cond` / `when` / `unless` | 3,264 / 2,392 / 1,206 / 654 | 3,200 / 2,424 / 1,206 / 654 |
| lines | 59,249 | 58,642 |
| Chez on the core | 31.8 s, 0.70 GB | 33.1 s, 0.64 GB |
| the heavy case: instructions / least cycles | 6,904M / 1,931M | 6,976M / 1,933M; in the suite 0.9-1.0x the C |

### 15. Signed arithmetic as C means it

- **The pattern.** Every `+ - *` of an `int` was `(i32+ a b)`, a macro of
  `(fx+ a b)` and two shifts that wrap the sum to 32 bits as gcc's -O0
  does; every one of a `long` `(i64+ a b)`, Scheme's `+` and a test that
  wraps it to 64. 4,340 of them. C leaves a signed overflow undefined: a
  correct program makes none, and whimsy's item 4 found that the suites
  reach none.
- **Measured first, here too.** The runtime's `i32+ i32- i32* i64+ i64-
  i64*` made to raise an error on a result outside their type's range (a
  copy of `rt.ss`, not committed), and the suites run on that build: all 80
  and all 240 as the C, and the heavy case -- no signed overflow in any
  path the suites reach.
- **What it is now.** An `int`'s `+ - *` and negation are `fx+ fx- fx*`
  and `(fx- x)` -- its operands' range keeps the result a fixnum -- and a
  `long`'s Scheme's own `+ - *` and `(- x)` (`scm_expr.go`'s `scmSignedOp`,
  `scmNeg`; an increment and a compound assignment come through the same
  `arith`). Unsigned arithmetic still wraps through `u32+`, `u64-`..., as
  C defines it; division and shifts keep their helpers (`MIN / -1`, a
  shift's count). What C leaves undefined no longer wraps: an overflow of
  an `int` is a number outside its range, which a store's conversion then
  wraps -- the honest risk, as whimsy's.

| | before | after |
| --- | ---: | ---: |
| `i32+ i32- i32* i64+ i64- i64*` | 4,340 | 0 |
| `fx+ fx- fx*` / `+ - *` | 4,348 + 185 + 1,248 / 0 | 6,102 + 1,535 + 1,317 / 625 + 476 + 65 |
| the runtime's wrapping helpers left | 5,088 | 745 (unsigned, division, shifts) |
| lines | 58,642 | 58,503 |
| Chez on the core | 33.1 s, 0.64 GB | 30.5 s, 0.65 GB |
| the heavy case: instructions / least cycles | 6,976M / 2,026M | 6,927M / 1,979M; in the suite 1.0-1.1x the C |

`TestScmSigned` holds the arithmetic to gcc's near the types' ends (a
`long` past Chez's 61-bit fixnums and back) and requires no wrapping
helper of a signed type; `TestScmControl`'s mutation of `widen_uchar` is
`(fx+ c 1)`'s now.

### 16. The memory functions as the bytevector's own

`musl_memmove`, `musl_memcpy` and `musl_memset` -- the core's own, a loop a
byte at a time, called from 228 places -- are runtime bodies now
(`internal/whim/gen.go`, as `match_lines` is): `(mem-copy! dest src n)`,
which is R6RS's `bytevector-copy!` of the editor's memory onto itself --
defined for regions that overlap, as `memmove` is -- and `(mem-fill! dest
(->u8 c) n)`, whose loop in `rt.ss` now copies zeros where the byte is 0,
as `mem-zero!` does. The C string functions (`musl_strlen`, `musl_strcmp`
...) stay the core's: R6RS has no procedure that finds a byte in a
bytevector, and Chez none that compares two ranges, so the runtime would
hold the same loops.

| | before | after |
| --- | ---: | ---: |
| the three functions | 31 lines, three loops | 12 lines |
| lines | 58,503 | 58,485 |
| Chez on the core | 30.5 s, 0.65 GB | 30.1 s, 0.66 GB |
| the heavy case: instructions / least cycles | 6,927M / 1,903M | **4,826M / 1,557M** (-30 % / -18 %) |
| ... nine runs interleaved, wall | 451-551 ms (min-median) | 412-513 (the C 466-482); in the suite 0.9-1.2x the C |

`TestScmMemBodies` holds the bodies to gcc's loops on regions overlapping
both ways, a fill of a byte past 255 and of zeros, and counts of 0.

### 17. Case labels by name

A `case` takes data, not names, so item 6 left every label a number. The
runtime has `c-case` now (`rt.ss`): a `case` whose labels are as the C
spells them -- a named constant, a character (its code), a number -- each
read at expansion, the named constant's value through a property its
definition attaches (`define-c-enum`, Chez's `define-property`), so that
what it expands to is the `case` of numbers it was. The printer writes a
label by its name where the C does and the name's value is the switch's
(`scm_expr.go`'s `caseLabel`: an enumerator, `#\a` for a character), and a
switch with one such label a `c-case`:

```scheme
           (c-case c
             [(ESC Ctrl_C)
              (join173 c esc_now lastc ...)]
```

where it was `[(27) (join173 ...)] [(3) (join173 ...)]`.

| | before | after |
| --- | ---: | ---: |
| case labels: by name / a character / a number | 0 / 0 / 953 | 464 / 256 / 233 (the C's own numbers, and labels folded from two names, `A \| B`) |
| switches a `c-case` / a `case` | 0 / 70 | 61 / 9 |
| lines | 58,485 | 58,617 |
| Chez on the core | 30.1 s, 0.66 GB | 26.2 s, 0.65 GB |
| the heavy case: instructions / least cycles | 4,826M / 1,591M | 4,826M / 1,764M (the same code; the cycles the load's); in the suite 0.8-1.0x the C |

`TestScmCaseLabels` switches on enumerators (one negative), characters
(`#\x28` for `(`), a number and a folded label, and on an `unsigned
char`, against gcc.

### 18. The joins nested where their calls are

Declined in the first pass for want of a measure; measured after item 17,
of the 2,309 joins left 13,240 of their 16,021 parameters were passed by
their own name at every call: a value the join is handed only because it
is defined at the function's top, outside the scope that binds it. So a
join several places call moves into the body that holds all its calls --
the innermost `let`, named `let`, `define` or `lambda` whose body every
call is in -- where that drops a parameter: one that every call passes by
its own name, bound to the same binding at each call as where the join
lands (`scmNestJoins`). Its body then reads the name of the scope it is
in. A join whose body names what a binding on the way would capture stays
where it was. Item 11's rule runs again first, after items 13 and 14 left
116 more joins called from one place (a case's clauses made one, a
constant written into a jump).

`del_bytes`'s four joins after it -- each where its calls are, `join19`
inside `join16`, of no parameters where it had three:

```scheme
        (define (join9 count col fixpos)
          (let ([movelen (+ (- (- oldlen col) count) 1)])
            (define (join12)
              (join13 (fx- oldlen col) 1))
            (define (join13 count movelen)
              (let ([alloc_newp (fxzero? (ml_line_alloced ed))])
                (define (join16 newp)
                  (define (join19)
                    (inserted_bytes ed lnum col (->i32 (- count)))
                    (frame-pop! ed fr)
                    #t)
                  ...
```

Found on the way: item 13's `scmSplitLets` had taken the function's own
`(let* ([mem (ed-mem ed)] [fr (frame-push! ed 32)])` apart into two lets in
the 225 functions with a frame, and `scmMergeLets` kept them apart; it
joins them again now.

| | before | after |
| --- | ---: | ---: |
| joins / their parameters | 2,309 / 16,021 | 2,193 / 4,195 (1,492 moved, 11,144 parameters fewer; 116 more in place) |
| lines | 58,617 | 52,677 |
| Chez on the core | 26.2 s, 0.65 GB | 25.1 s, 0.66 GB |
| the heavy case: instructions / least cycles | 4,826M / 1,446M | 4,945M (+2.5 %) / 1,512M (+4.6 %); in the suite 0.8-0.9x the C |

The price is measured and kept: a join defined inside a loop's or another
join's body is a closure Chez makes on each entry, where it was made once
per call of the function. Kept out of loops' and joins' bodies, the rule
(with item 11's second run) left the joins 15,029 parameters, at no cost
(4,826M instructions); everywhere, 4,195, at 2.5 % -- still under the C's
time, and item 16 had taken 30 %.
