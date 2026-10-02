# How whimsical could be more idiomatic Scheme: a survey

2026-10-02. It covers `whimsical/whimsical/editor.ss` as tracked at
`598b500` (88,474 lines, 1,713 functions of the core and 17 of the host's
glue, written by `crefactor/togo`'s Scheme backend from the core `go tool
whim cut src/whim-vim.c` prints, 75,721 lines of C), and the hand-written
runtime `whimsical/whimsical/rt.ss`, host `host.ss`, terminal `term.ss` and
`printf.ss`. `doc/HASKELL-IDIOMS.md` and `doc/CLOJURE-IDIOMS.md` are its
model: every item says what the pattern is, how many sites it has, what it
would become, who would do it -- the Scheme printer (`crefactor/togo/scm.go`,
`scm_fn.go`, `scm_expr.go`), togo's shared analyses (`hsout.go`,
`hsstruct.go`, `hseffects.go`), the runtime -- its cost (S/M/L), its risk,
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
| `hseffects.go` (the Haskell backend's analysis, which the Scheme backend asks) | the functions that touch no memory |
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
  even through what they call (`hseffects.go`), e.g. `musl_isdigit`.
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
  by phase 181), a struct local of scalars bindings (148) -- `hsout.go`'s
  and `hsstruct.go`'s decisions.
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
  a different shape from the one caprice measured; not taken here.

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
memory, even through what they call (`hseffects.go`), take only their C
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
