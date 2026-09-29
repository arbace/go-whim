# How caprice could be more idiomatic Haskell: a survey

2026-09-29. A read-only survey: no tracked file changed but this one. It
covers `caprice/Caprice/Editor.hs` as tracked at `80db5f3` (122,743 lines,
1,710 functions, written by `crefactor/togo`'s Haskell backend from the core
`go tool whim cut src/whim-vim.c` prints, 73,737 lines of C), its hs-boot, and
the hand-written runtime `caprice/rt/Caprice/Rt.hs` (257 lines), host
`caprice/host/Caprice/Host.hs` (372) and `Printf.hs` (747).
`doc/CLOJURE-IDIOMS.md`, `doc/JAVA-IDIOMS.md` and `doc/GO-IDIOMS.md` are its
model: every item says what the pattern is, how many sites it has, what it
would become, who would do it -- the Haskell printer (`crefactor/togo/hs.go`,
`hs_fn.go`, `hs_expr.go`), the lowering (`lower.go`), a pipeline phase that
changes the C, the runtime (`Rt.hs`) or the host -- its cost (S/M/L), its
risk, and how it would be verified. `Editor.hs` is generated and stays so:
nothing here is a change by hand to it; the experiments below were made on
copies under `.tmp/`, never committed.

## The tension, stated first

caprice was built to be C in Haskell's syntax (`doc/HASKELL.md`): every C
object in raw memory laid out as amd64 lays it out, a pointer an address
(`Ptr ()`), every function in `IO`, control flow as join points. That choice
is why it was the least translation and why it runs at 1.5 times the C. It is
also everything a Haskell programmer would object to. Idiomatic Haskell is
pure functions of immutable data, types that say what a value is, and
control flow a reader can follow. Measured here, the two meet as follows:

- **Purity is scarce, and it is the C's, not the printer's.** Of the 1,710
  functions, 58 touch no memory at all, even through what they call (3%); 198
  more read memory and write none (12%). 1,450 write memory; 978 reach the
  terminal or the process, 314 of them only through the error-message path
  (`emsg` reaches the screen). So the core stays in `IO`, and the idiom
  available is *typed, named* `IO` on memory, not a pure program.
- **Most of what reads as foreign is the printer's spelling, and it is free.**
  GHC's `-Wall` gives **36,565 warnings** (34,916 of them a binding shadowing
  another); memory is addressed by numbers (11,657 reads, writes and
  addresses of a file-scope object are an offset into `(edSeg ed')`, and
  16,052 C member accesses an offset from a pointer); 8,694 named C
  constants and 2,462 char literals are printed as numbers; every pointer is
  `Ptr ()`. Three such changes were built here and compiled -- new names for
  the assignments, the file-scope objects by name, the pure functions pure:
  the heavy case's time did not move, GHC's time moved by 2-6%.
- **Moving data out of raw memory costs speed, measured.** A file-scope
  `Int64` in an `IORef` field is **4.5 times slower** per read-and-write than
  the segment (a boxed value allocated on every write); an unboxed field is
  as fast as the segment, measured -- because it is what the segment
  already is. So the file-scope objects should get names, not new homes.
- **Compile time is GHC's simplifier, not its collector, and the module's
  memory is its size.** At `-O1` the simplifier is 123 of GHC's ~200 s; the
  collector's 94 s cannot be tuned away (`-A`, `-H`, `-F` measured). The one
  change measured to move either number is splitting the module by the call
  graph: **204 s and 4.0 GB -> 172 s and 1.8 GB** (`ghc -j4`), the heavy case
  unchanged.

## How it was measured

Everything below was counted or run, not estimated, unless it says so. The
instruments were throwaway, in the worktree's `.tmp/hsid/` (listed at the
end); none is tracked.

| Instrument | What it gave |
| --- | --- |
| `whim skel core.c D -hs Editor.hs` | `Editor.hs` regenerated from the cut core: **byte-identical** to the tracked file, so every count is of what `whim gen` writes |
| `scan/` (Go, reads `Editor.hs`) | the module split into its functions; each line the printer writes classified -- a read or write of the segment, of the frame, of any other address; a call of the core, of the host, through `callPtr`; join points, frames, bindings -- and the closures over the call graph: each function's effect set |
| `zz_survey_test.go` (a `togo` test, copied into `crefactor/togo` to run, removed after) | the core parsed and every function lowered as the backend lowers it (`lowerFunction`, `placeVars`): why each frame variable is in memory, the file-scope objects by kind and use, pointer arithmetic and casts, switches and their labels, loops and what their bodies do, pointer parameters and nulls, out-parameters, members and typedefs by use |
| `joins/`, `scc/` (Go) | the join points by how many jumps reach them; the call graph's strongly connected components and layers |
| `ghc -fno-code -Wall` | GHC's warnings on `Editor.hs` with its `-w` turned off: the Haskell counterpart of clj-kondo and PMD |
| `ghc -O1 -ddump-timings` | GHC's time and allocation by pass |
| `xf/` (Go) and `build.sh` | five variants of `Editor.hs` made by text transforms (below), each compiled with caprice's own flags (`-O1 -threaded`) into a program, timed with `/usr/bin/time` and GHC's `+RTS -s`; and `initGlobals` alone, three ways |
| `heavy.sh` | the suite's heavy case (`internal/suite/heavy.go`'s keys, a file on stdin, `WHIM_TIME` pinned) on a binary, its output's digest and its time: every variant printed the C's digest (`21dece53`) |
| `bench/Bench.hs`, `bench/Maybe.hs` | micro-benchmarks: a file-scope scalar in the segment, an `IORef` and an unboxed array; a nullable result as `nullPtr` and as `Maybe` |
| `whim test --haskell` | the verification recipe, run once at `80db5f3` in the worktree: 80 cases answered as the C does, the control seen by 76, the heavy case C 458 ms and caprice 698 ms (1.5x); 3 m 47 s |

**GHC's time is measured on this machine, which others share** (load
average 6-7 on 64 cores throughout). The baseline, compiled three times the
way the variants were (`build.sh`: the program with `Editor.hs` swapped in):
**202.8, 203.4 and 204.5 s wall, 4.02 GB peak** -- a little over
`doc/HASKELL.md`'s 183 s and 4.0 GB, measured when caprice was built. Two
variants compiled side by side took up to 8% longer than either alone, so
every comparison below is of builds made alone, unless it says otherwise.
The heavy case, five runs: the C 0.44-0.46 s, caprice 0.65-0.74 s, and
caprice with one capability (`+RTS -N1`) 0.70-0.75 s -- at 5,000 lines the
parallel `:%s` matters little, so the heavy case times the sequential code
the items below change.

**Verification recipe for every item** (the suite compares output byte for
byte, so a change that moves one byte of one screen is seen):

- `go tool whim test --haskell` (80 cases, the control -- `" INSERT"` changed
  in the generated `Editor.hs` -- seen by 76) and `--wide --haskell` (240);
- `crefactor/togo`'s `TestHs*` (fifteen C programs, each compiled by GHC and
  required to print what gcc's build prints) and `TestHsControl`;
- `make whim-editor-check` after the regenerated `Editor.hs` is committed,
  with `editor.go`, `Editor.java` and `editor.clj` unmoved where the change is
  the Haskell printer's alone (a C phase moves all four, and all four suites
  judge it);
- and, because the suites see neither, GHC's time and peak memory on the
  core and the heavy case's time, before and after.

## The shape of the output today

| | count |
| --- | ---: |
| functions | 1,710, all in `IO`: 844 `IO ()`, 299 `IO Int32`, 288 `IO Bool`, 236 `IO P` |
| lines | 122,743: 105,548 in the functions (median 26), 15,283 in `initGlobals` |
| join points (`j'N`, one per basic block) | 19,384; 478 functions are one block, 20 have 100 or more (`regmatch` 418) |
| functions with a frame (`frame n $ \fr' ->`) | 301 (19,001 bytes in all, at most 1,056); the C instrument finds a local in memory in 294, the other 7 not traced |
| the segment (file-scope objects) | 8,407 reads in 1,004 functions, 1,816 writes in 460, 1,434 addresses taken in 448; 959 offsets named |
| other memory (through a pointer) | 10,367 reads in 1,057 functions, 3,606 writes in 703; 497 `copyMem`, 23 `fillMem` |
| the frame | 2,430 reads, 972 writes, 1,063 addresses |
| calls of the host | in 112 functions (`vim_snprintf` in 88); `callPtr` 16 in 11 |
| bindings | 30,768 `r'N <-` (a read or a call), 8,339 `!x <- pure e` (an assignment) |
| string literals | 1,812 `Ptr "..."#`, 1,394 distinct |

**The effects, closed over the call graph** (`scan/`: a function's own lines
and all it calls; a call through a pointer an effect of its own):

| effect set | functions | e.g. |
| --- | ---: | --- |
| none: no memory, no host | 58 | `musl_isdigit`, `ends_excmd`, `utf_char2len`, `hex2nr` |
| reads memory only through its arguments | 61 | `musl_strlen`, `musl_strcmp`, `musl_atoi` |
| reads the segment too | 137 | `buf_valid`, `byte2cells`, `vim_isIDc` |
| writes memory, no host | 462 | `musl_memcpy`, `set_bufref`, `ga_init` |
| allocates, the clock, the terminal, or a function pointer | 992 | `alloc` (whose out-of-memory path reaches `emsg`), everything above it |

So 256 functions (58 + 198) write nothing; with the error-message functions
(`emsg`, `semsg`, `iemsg`, `siemsg`, `emsg_multiline`, `do_outofmem_msg`) cut
from the graph, 268, and the functions reaching the terminal fall from 978 to
664. 221 functions touch no file-scope object and no host even through what
they call, and 147 do not use `ed'` at all.

**What forces raw memory, counted on the C** (`zz_survey_test.go`):

- **Locals in the frame: 604 variables in 294 functions** -- 290 scalars whose
  address is taken (207 integers, 78 pointers, 5 enums; 282 of the addresses
  are handed to a call), 219 structs, 14 structs passed by value, 81 arrays
  (67 of bytes). Every other local -- 3,712 C locals, 2,916 parameters and
  613 temporaries in all -- is already a Haskell binding.
- **File-scope objects: 809** (and block statics): 367 integers, 22 bools, 6
  enums, 79 pointers, 295 arrays, 40 structs, **0 unions** (the core has no
  union type). 453 scalars never have their address taken (91 of them are
  never written by a function either); 268 of the arrays are never written
  by name (194 are byte arrays with a string initializer: the messages
  among them).
- **Pointer arithmetic**: 803 `p + n`, 436 `p++`, 208 `p += n`, 165 `p - q`,
  71 `p < q`, 3,343 `p[i]` on a pointer, 2,272 `*p`, 9,269 `p->m`.
- **Casts between pointers**: 1,374 between pointers to same-sized integers
  (`char_u *` and `char *`: signedness), 176 to or from `void *`, **50 puns**
  of another type (`u8char_T *` to `char *` 16, `pos_T *` to `char *` 6),
  1,833 `NULL`s, 23 pointer to integer. No union.
- **Types**: 103 struct types; 854 enum specifiers, of which 829 are one
  enumerator each (the preprocessor's constants, as the phases leave them)
  and 25 are enumerations proper.

## What a Haskell programmer would find most jarring

Five excerpts, as tracked, with the C they come from.

**1. A pure predicate in `IO`, with the editor it never uses**
(`musl_isdigit`, line 15912; `musl_isalnum`, 15954):

```haskell
musl_isdigit :: Ed -> Int32 -> IO Bool
musl_isdigit ed' c = do
  let
    j'0 !c = do
      pure (((fromIntegral c :: Word32) - (48 :: Word32)) < (10 :: Word32))
  j'0 c

musl_isalnum :: Ed -> Int32 -> IO Bool
musl_isalnum ed' c = do
  let
    j'0 !c = do
      r'1 <- musl_isalpha ed' c
      r'3 <- if r'1 then pure True else (do { r'2 <- musl_isdigit ed' c; pure r'2 })
      pure r'3
  j'0 c
```

After items 1, 2 and 5 (1 and 5 built and measured, below):

```haskell
musl_isdigit :: Int32 -> Bool
musl_isdigit c = fromIntegral c - 48 < (10 :: Word32)

musl_isalnum :: Int32 -> Bool
musl_isalnum c = musl_isalpha c || musl_isdigit c
```

**2. Offsets for names** (`check_cursor_lnum`, 64009; the C is
`if (curwin->w_cursor.lnum > curbuf->b_ml.ml_line_count) curwin->w_cursor.lnum
= curbuf->b_ml.ml_line_count; if (curwin->w_cursor.lnum <= 0)
curwin->w_cursor.lnum = 1;`):

```haskell
    j'0 = do
      r'1 <- rdP (edSeg ed') 2768
      r'2 <- rdI64 r'1 16
      r'3 <- rdP (edSeg ed') 2808
      r'4 <- rdI64 r'3 0
      if (r'2 > r'4)
        then j'1
        else j'2
    j'1 = do
      r'7 <- rdP (edSeg ed') 2768
      r'5 <- rdP (edSeg ed') 2808
      r'6 <- rdI64 r'5 0
      wrI64 r'7 16 r'6
      j'2
```

`2768` is `curwin`, `16` is `w_cursor.lnum`. With the objects and members
by name (item 3), typed pointers (item 4) and the structure (item 2) -- the
same reads, in the same order, since C reads `curwin` again after the store:

```haskell
check_cursor_lnum :: Ed -> IO ()
check_cursor_lnum ed = do
  lnum <- curwin ed >>= w_cursor_lnum
  count <- curbuf ed >>= ml_line_count . b_ml
  when (lnum > count) $ do
    wp <- curwin ed
    curbuf ed >>= ml_line_count . b_ml >>= set_w_cursor_lnum wp
  lnum' <- curwin ed >>= w_cursor_lnum
  when (lnum' <= 0) $ curwin ed >>= \wp -> set_w_cursor_lnum wp 1
```

**3. A loop as blocks, a byte pointer stepped as arithmetic** (`vim_strchr`,
110891; 15 join points, where the C has two `while`s):

```haskell
    j'3 !c !p = do
      r'1 <- rdW8 p 0
      !b <- pure (fromIntegral r'1 :: Int32)
      if (b /= (0 :: Int32))
        then j'5 c p b
        else j'4
    j'4 = do
      pure nullPtr
    j'5 !c !p !b = do
      if (b == c)
        then j'7 p
        else j'6 c p
    j'6 !c !p = do
      r'2 <- utfc_ptr2len ed' p
      !t1 <- pure r'2
      !p <- pure (pAdd p ((fromIntegral (fromIntegral t1 :: Int64) * 1)))
      j'3 c p
    j'7 !p = do
      pure p
```

After item 2 (a block reached by one jump written in place, a loop head a
local `go`) and item 1:

```haskell
    narrow !p = do
      b <- fromIntegral <$> peekByte p
      if b == NUL then pure nullPtr
      else if b == c then pure p
      else utfc_ptr2len ed p >>= \l -> narrow (p `plusPtr` fromIntegral l)
```

**4. A local in memory because its address is an out-parameter**
(`getvcol_nolist`, 20284; the C declares `colnr_T vcol;` and calls
`getvcol(curwin, posp, NULL, &vcol, NULL, 0)`):

```haskell
getvcol_nolist ed' posp = frame 4 $ \fr' -> do
  ...
    j'1 !posp !list_save = do
      r'5 <- rdP (edSeg ed') 2768
      getvcol ed' r'5 posp nullPtr fr' nullPtr (0 :: Int32)
      j'3 list_save
    ...
    j'3 !list_save = do
      r'7 <- rdP (edSeg ed') 2768
      wrI32 r'7 448 list_save
      r'8 <- rdI32 fr' 0
      pure r'8
```

`getvcol`'s `cursor` is an optional out-parameter (tested for `NULL`, stored
through, never read): item 6 would return it, `(_, vcol, _) <- getvcol ...`,
and the frame would go.

**5. A struct as bytes** (`optvar_int`, 16216; the C returns the compound
`optvar_T v = { p, NULL, NULL, 0 }` by value):

```haskell
optvar_int :: Ed -> P -> P -> IO ()
optvar_int ed' sret' p = frame 32 $ \fr' -> do
  let
    j'0 !p = do
      fillMem (pAdd fr' 0) 0 32
      wrP (pAdd fr' 0) 0 p
      copyMem sret' fr' 32
      pure ()
  j'0 p
```

This one stays: `optvar_T` is written into memory the caller owns
(`sret'`) and read there through pointers; a record would be a copy at every
boundary (item 9).

## The findings

### 0. Measure GHC, not only the editor (a prerequisite)

- **The pattern.** Nothing in the tree records GHC's time or memory on the
  core: `caprice.Compile` runs `ghc` and reports only failure. The suite
  reports the heavy case's time, which catches a slowdown of the editor, but
  every item below moves the module's size and GHC's work, and a doubling of
  a three-minute compile would pass unseen.
- **Measured** (`-ddump-timings`, the baseline): the **simplifier is 122.9 s**
  (105 GB allocated), the renamer and type checker 21.0 s, code generation
  11.3 s, Core to STG 9.5 s, demand analysis 9.3 s, float-out 7.5 s, the
  parser 4.2 s. `+RTS -s`: 94-95 s of GHC's 198-199 s is its collector, with a
  1.7 GB residency. Tuning GHC's own RTS does not help: `-A256m` 207.5 s,
  `-H4g` 207.6 s, `-F4` 189.3 s but 5.86 GB. Type checking alone
  (`-fno-code`) is 25.6 s and **3.40 GB**: the memory is the module's size
  before any optimisation.
- **What it would become.** `caprice.Compile` passing `+RTS -s -RTS` to GHC
  and printing its wall time and maximum residency, as the heavy case prints
  its ratio; and `ghc -fno-code -Wall`'s count of warnings (36,565 today;
  item 1) as the printer's lint measure, as clj-kondo's was for the Clojure.
  `-Wall` itself is slow on this module -- 154.6 s against 25.6 s for the
  type check alone (the warnings' own analyses; not traced further) -- so it
  is a survey's instrument, not the build's.
- **Where:** `caprice/caprice.go`. **Cost:** S. **Risk:** none.

### 1. The printer's noise: what `-Wall` sees

**`ghc -fno-code -Wall` on `Editor.hs`: 36,565 warnings**, and a counter of
what no warning names:

| Finding | Count | What it becomes |
| --- | ---: | --- |
| `-Wname-shadowing`: `!x <- pure e` binds `x` again, a block's parameter shadows its caller's | 34,916 | a new name per assignment, `let !x'1 = e` (measured, below) |
| `-Wunused-matches`: a call's result bound and not used (`r'2 <- musl_strcpy ...`) | 1,126 | `_ <- f ...`, or `void`; a `()` result unbound as today |
| `-Wunused-matches`: `ed'` not used | 147 | the parameter dropped (item 5) |
| `-Wunused-matches`: other | 207 | `_` |
| `-Wincomplete-uni-patterns`: `fnTable`'s `\ed' [a0] -> ...` | 165 | a total pattern (`(a0 : _)`), or the table typed per signature |
| a literal with its type, `(0 :: Int32)` | 16,924 | the literal, where the other operand fixes the type -- the printer knows `ht` everywhere |
| `fromIntegral` | 10,151, 2,682 of a `fromIntegral` | one conversion; a conversion of a constant folded |
| a byte pointer stepped as `pAdd p ((fromIntegral (fromIntegral n :: Int64) * 1))` | 1,658 `* 1` | `plusPtr p (fromIntegral n)` |
| `!x <- pure e` (an assignment through `IO`'s bind) | 8,339, 3,954 of them a variable copied (`!t1 <- pure s`) | `let`; a copy is the name itself |
| `(do { ...; pure r })` on the right of `&&`, `||` or `?:` | 4,012 | a local `orM`/`andM`, or item 2's structure |
| a block that only jumps (`j'6 !s !n !neg = do j'7 s n neg`) | 392 | the jump's target |
| a function of one block, still `let j'0 ... in j'0 args` | 478 | its body |
| `c'` before a name that starts upper-case (`c'FreeWild`) | 14 functions, 725 uses | kept: a C name must stay a grep |

- **Measured: the renaming.** `xf/lets.go` rewrote all 8,339 `!x <- pure e`
  as `let !x'vN = e`, each later use and jump renamed to the newest name
  (so none shadows another, and none is a bind in `IO`). GHC **207.9 s**,
  4.02 GB (baseline 202.8-204.5 s); the heavy case the C's digest, 0.64-0.70 s
  against the baseline's 0.66-0.70 s in alternation. The Core is the same
  thing written twice, evidently: GHC removes `pure`'s bind in `IO` either way.
- **Where:** the printer: `hs_fn.go`'s `setVar` (the `let` and the names),
  `step`'s `opEval` (the discarded result), `jump` and `block` (the trivial
  blocks, the one-block functions); `hs_expr.go`'s `conv`, `hsLit` and
  `additive` (the literals, the conversions, the byte steps), `logical` and
  `ternary`.
- **Cost:** S-M: each is a local rule. **Risk:** none to behaviour: each is
  an identity GHC's type checker checks; one to watch -- a literal without
  its type where the other operand is also a literal defaults to `Integer`,
  so the rule is "drop the annotation when the other side is not a
  literal". **Verify:** the suites, `TestHs*`, and the `-Wall` count toward 0.
- **Value:** high for the least: it is what a Haskell reader sees on every
  screen, and `-Wall` clean is what a Haskell project is held to.

### 2. Control flow: join points as structure

- **The pattern.** Every basic block of the lowered form is a local function
  (`j'N`), every jump a tail call. `joins/` classifies the 19,384 blocks by
  what reaches them: 1,709 entries, **12,717 reached by exactly one jump**
  (66%), 4,054 reached by several (joins), 904 reached by a jump back (loop
  heads, where the C has 919 loops). 679 functions have no join and no loop:
  they are trees of `if`s printed as blocks.
- **What it would become.** A block reached by one jump written in place, in
  the arm of the `if` or `case` that jumps to it; a loop head a local
  recursive function with a name (`go`, or the C loop's variable, `narrow`
  above); a join a local function called from its arms -- which is exactly
  how a Haskell programmer writes a join, so the Clojure's difficulty (a
  join Clojure cannot `recur` through, 144 state machines left) does not
  arise: the joins are local functions already, and the rule only moves the
  blocks reached once into place -- every function structured, by
  construction, with **no state machine and no copying**. 12,717 of the
  19,384 local functions go; the 49 `goto`s of the C are joins like any
  other.
- **Where:** a nesting pass on the lowered form for the Haskell printer
  (`hs_fn.go`): the dominator tree the Clojure's `clj_shape.go` computes
  (`findJoins`, `loopForm`) is what it needs, with fewer rules, since a local
  function may be called from anywhere in its scope. **Cost:** M.
- **Risk:** low to behaviour (the suites see a misplaced arm). To GHC's time:
  none expected -- GHC's own Core writes a block reached once in place and a
  join as a join point, so the program it compiles is the same -- **an
  estimate, not measured**; measure GHC's time when it lands. The C's order
  of reads and calls is untouched: only where the lines are printed moves.
- **Verify:** the suites, `TestHsNest`, `TestHsMachine` and `TestHsGoto`
  (which already hold the lowering's awkward shapes to gcc), GHC's time.
- **Value:** high: it is what makes a 400-block function (`regmatch`,
  `win_line`) readable as the C is.

### 3. Names for offsets and numbers

Three patterns of one kind: the C has a name, the Haskell a number.

- **File-scope objects.** 8,407 reads, 1,816 writes and 1,434 addresses of
  the segment are `rdP (edSeg ed') 2768`. The printer knows the object at
  every site (`h.segOff`). **Measured:** `xf/` `names` gave each object an
  accessor, `curwin :: Ed -> IO P; curwin ed = rdP (edSeg ed) 2768`
  (`{-# INLINE #-}`), a setter and an address -- **1,458 accessors** for the
  959 offsets the functions name (an object's member at an offset inside it
  `obj'N`) -- and used them at the 11,657 sites: GHC **215.4 s** alone (231.5
  side by side with another build), 3.98 GB; the heavy case the C's digest
  and time. So 6% of GHC's time for a module that says `curwin` where it said
  `2768`.
- **Members.** The C accesses 764 distinct members, at 16,052 sites (9,269
  `p->m`, 6,783 `s.m`); each is `rdI64 r'1 16`. As an address function per
  (type, member), pure (`w_cursor :: Ptr Win -> Ptr Pos; w_cursor p = p
  `plusPtr` 16`) and a reader/writer per scalar member, 764 small functions
  that GHC inlines; not built here -- a text transform cannot know the type
  at a site; the printer can (`haddr.field`). An estimate from the measured
  accessors: a few percent of GHC's time.
- **Constants.** In the functions, **7,927 uses of a named constant** (one of
  the 829 one-enumerator enums: `ESC`, `K_DEL`, `NUL` 967 of them, `K_*`
  353), 767 of an enumerator of a real enumeration, and 2,462 char literals
  are printed as numbers (`(45 :: Int32)` for `'-'`); of the 961 `case`
  labels, 386 are named constants, 256 char literals, 209 expressions of
  names (`(idopt_T)(PV_BUF + BV_AI)`). They would be top-level constants,
  and **pattern synonyms** in `case` (`pattern ESC :: Int32; pattern ESC =
  27`), and char literals `(fromIntegral (ord '-'))`, or a `c8 '-'` helper
  the runtime inlines.
- **Where:** the printer (`hs.go`'s `layout` has the objects; `hs_expr.go`'s
  `addrNode` the members; `node` sees the enumerator before it folds it to a
  value: the Java printer's `java_const.go` does the same, writing `case
  ESC:` where Java's evaluation of the spelling gives the C's value).
  **Cost:** S for the objects, M for members and constants. **Risk:** none to
  behaviour; a constant's name must be the C's unless it collides, and a
  pattern synonym must start upper-case (`ESC` does; a lower-case enumerator
  takes a prefix, as `hsName`'s `c'` does the reverse). GHC's time: measured
  +6% for the objects; members and constants not measured.
- **Value:** high: this and item 1 are most of what makes the file unreadable.

### 4. Types: what a pointer points at, and what an integer means

- **Typed pointers.** Every pointer is `P = Ptr ()`: 1,413 parameters, 236
  results. The C's pointers point at 110 kinds of thing (`char_u` 1,032
  declarators, `win_T` 144, `cmdarg_T` 91, `buf_T` 80, `pos_T` 78, ...). As
  `Ptr WinT` -- a phantom type, an empty `data WinT` per struct, no cost at
  run time -- GHC would check what the C checks, and a reader would see it.
  Where the C converts, `castPtr`: 1,374 signedness casts (`char_u *` to
  `char *`), 176 through `void *`, 50 puns, and every `void *` parameter a
  `Ptr a`. The runtime's `rdP` becomes `rdP :: Ptr a -> Int -> IO (Ptr b)`
  or, with item 3's members, disappears behind typed readers.
- **Newtypes for `linenr_T` and `colnr_T`.** Declared 306 and 252 times
  (locals 132/153, parameters 114/44, members 46/43). As `newtype LineNr =
  LineNr Int64 deriving (Eq, Ord, Num)` a constant needs nothing (505 and 506
  sites), but where the C mixes one with another integer type -- **340 and
  664 binary expressions or assignments** -- the printer must convert
  explicitly, and the typedef is lost on every arithmetic result the front
  end types as `long`, so these counts are a floor. Value medium-low for
  that cost; a C phase cannot help (C has no newtype).
- **`Maybe` for a nullable pointer.** 94 of the 236 functions returning a
  pointer have a `return NULL`; of 1,383 pointer parameters, 89 are passed a
  `NULL` at some call and 107 are tested for null in the callee (1,231
  neither -- which does not prove them never null: an argument may be a
  variable that is). Measured (`bench/Maybe.hs`, a non-inlined function
  returning either, 2 x 10^8 calls): `nullPtr` 1.95-2.01 s, `Maybe`
  1.70-2.04 s -- no cost seen. But the C compares, stores and walks the same
  pointer that may be null (1,833 `NULL`s in the C, 1,835 `nullPtr` in the
  Haskell), so `Maybe` would be a conversion at every boundary of the 94
  functions, for a type the C does not keep. Declined but at the 94 results
  of a leaf function, if at all.
- **`Bool`** is done: phase 166 made the 278 yes-or-no functions `bool`, and
  the Haskell has 288 `IO Bool` functions and 283 `Bool` parameters.
- **Where:** the printer (`hsType`, `hsParamType`, the casts in `conv`), the
  runtime (`Rt.hs`: typed readers), the host and its `hs-boot` (the host's
  17 signatures, `Editor.hs.host`, and `Profile.HsExports`' types).
  **Cost:** M (pointers), L (newtypes). **Risk:** low to behaviour; the
  typed pointers move every signature the host sees. GHC's time: not
  measured; a phantom type adds no Core.
- **Value:** medium-high for pointers; low for the rest.

### 5. Purity: the pure functions pure, and the editor where it is used

- **The pattern.** Every function is `Ed -> ... -> IO r`. 58 functions touch
  no memory and call only each other (the `musl_is*`, `utf_char2len`,
  `hex2nr`, `ends_excmd`, `one_adjust`, `decode_modifiers`, `trim_to_int`
  ...); 20 of them return `Bool` -- of the 288 `IO Bool` functions, 20 are
  pure, 69 read memory only (14 of them only through their arguments), and
  199 write or reach the host. 221 functions touch no file-scope object and
  no host even through what they call, and 147 never name `ed'`.
- **Measured:** `xf/` `pure` made the 55 of the 58 without a frame
  `f :: A -> B` (`runIdentity` around the body), each call `r <- pure $! f
  a`, a call for its effect nothing, and `fnTable`'s two entries `pure (f
  ...)`: GHC **207.9 s** alone (219.9 side by side), 4.00 GB; the heavy case
  the C's digest and time. The printer can do it by the effect closure
  `scan/` computed, which is the shared analysis's to make (`an`, or the
  instance pass that decides the Go's methods: `instance.go`).
- **The read-only 198 stay in `IO`**: they read memory another function
  writes, so they are functions of the moment they run, not of their
  arguments; `unsafePerformIO` would let GHC float or share a read past a
  write. A Haskell programmer would still recognise them if they took no
  `Ed` they do not use.
- **The editor where it is used.** Dropping `ed'` from the 221 is the
  Clojure's item 7 and needs no analysis beyond the closure. The alternative
  a Haskell programmer would reach for, `ReaderT Ed IO` everywhere (22,128 `
  ed'` passed today), was not built: it is a newtype GHC erases, but its bind
  is inlined at every one of the 39,107 binds (and every statement besides)
  and the simplifier is already 61% of the compile -- measure before
  adopting.
- **Where:** the shared analysis (the closure) and the printer (signatures,
  calls, `fnTable`). **Cost:** S. **Risk:** none to behaviour: a pure call
  has nothing to reorder. `match_lines`' runtime body calls none of the 58.
- **Value:** medium: few functions, but the plainest sign of Haskell there
  is.

### 6. Out-parameters as results, and the frame

- **The pattern.** 290 scalar locals are in memory because their address is
  taken, 282 of those addresses handed to a call (`&vcol`). On the callee's
  side, of the 249 pointer-to-scalar parameters that are not strings: **51
  only stored through (out), 27 tested for null and stored through
  (optional out)**, in 55 functions; 88 read and written (in-out); 5 only
  read; 64 passed on, indexed or walked; 14 unused.
- **What it would become.** A callee whose out-parameters are only that
  returns them, `(r, vcol) <- getvcol ...`; a caller whose every address of a
  local goes to such a parameter keeps the local a binding, and its frame
  goes when nothing else is in it. An in-out parameter could be a value in
  and a value out as well, but the callee must then not reach the local by
  another way: that is phase 175's proof (`crefactor/xform`'s `MemberOut`,
  for members), which for locals is simpler -- a local's address that is
  only passed is reachable only through the parameter.
- **Where:** either a pipeline phase in the manner of 174-175 (the C returns
  a struct, or a helper copies through a local), which the Go, Java and
  Clojure gain from too and all four suites judge; or the Haskell printer
  alone (a tuple result), which C cannot say. **Cost:** M. **Risk:** low; C's
  order of evaluation around the call stays as the lowering has it. How many
  frames would go was not measured.
- **Value:** medium: 141 functions have an address-taken local, and
  `frame n $ \fr' ->` with `rdI32 fr' 0` is the least Haskell thing in them.

### 7. `initGlobals`: the tables and the messages as data

- **The pattern.** `initGlobals` is 15,281 lines: 13,502 writes of a constant
  (`wrI32 (edSeg ed') 2192 (-1 :: Int32)`), of which **6,452 write one byte**
  (5,981 of them in 186 runs of four or more: the strings of the byte
  arrays, the 178 messages `e_*` among them), 1,776 writes of an address (a
  string literal or a function's index) and 1 of a bool.
- **Measured.** GHC on `initGlobals` alone (with the segment's size, the
  runtime and nothing else): **10.0 s, 0.60 GB**. With each run of bytes one
  `copyMemNo` from a string literal (`xf/bytes.go`: 186 runs, 5,981 lines
  folded): 6.2 s. With every constant write folded into one image of the
  segment, a `Ptr "..."#` literal of 96,801 bytes copied in once, and the
  1,776 writes of an address and the bool's kept (`xf/image.go`): **1,779
  lines, 1.2 s, 0.23 GB**. Combined with items 3 and 5 in one build (`ipn`):
  GHC **203.0 s, 3.66 GB**, the heavy case the C's digest and time -- the
  image paying for what the names and the pure functions cost.
- **What it would become.** Idiomatically, the tables as Haskell data (a
  list of rows, as the Clojure's `clj_tables.go` made them) -- but the core
  reads them through pointers into the segment, so the rows must be written
  there anyway, and the image is the cheapest form of that. **The messages**
  could be the literals themselves, shared by every editor, not copied into
  the segment, when nothing writes them: 268 arrays are never written by
  name, which is not a proof (a message is passed as `char *`); the Clojure
  declined the same for the same reason.
- **Where:** the printer (`hs.go`'s `initializers`: it knows the offsets and
  the constants). **Cost:** S. **Risk:** low: an image is the C's own
  initialisation, byte for byte; the byte order is amd64's, which the layout
  already assumes. **Verify:** the suites; the image against the writes it
  replaces (a test that runs both and compares the segments).
- **Value:** low for reading (a reader skips `initGlobals`), a sixth of the
  file gone, and 9 s of GHC.

### 8. One module or several

- **The pattern.** One module of 122,743 lines: GHC holds it all (3.4 GB
  before optimising) and cannot use a second core.
- **Measured: the call graph.** 1,294 of the 1,710 functions are in no cycle;
  one knot of 410 functions (41,290 lines, 39%) calls itself round; three
  pairs. With a call through a pointer counted as reaching every function in
  the table, the knot is 884 functions (78% of the lines): **the function
  table is what would tie the module together**, since `fnTable` names 165
  functions and its callers are everywhere.
- **Measured: a split.** `xf/split.go` wrote the module as `Caprice.Low` (the
  functions that cannot reach the knot, 23,073 lines), `Caprice.Knot`
  (41,700), `Caprice.High` (the rest, 42,485) and the top `Caprice.Editor`
  (the segment, `initGlobals`, the table, and the seven names the host
  imports back, as wrappers so the hs-boot is unchanged), with the table read
  through an `IORef` set when an editor is made (`Caprice.Tbl`), so that no
  part imports the top. Compiled with `ghc -j4`: **172.2 s wall (193.6 s
  CPU), 1.77 GB peak**, against 204 s and 4.02 GB; the heavy case the C's
  digest and time. The wall time gains little because the three parts are a
  chain (Low, then Knot, then High); the memory is what a part holds.
- **What it would become.** The table in `Ed` (a field, not a global `IORef`),
  set by `newEditor` -- a runtime change -- and the modules by the call
  graph's layers, as the Clojure's item 8 found for namespaces. More, smaller
  parts of `High` in parallel would shorten the chain; not measured.
- **Where:** the printer (it orders functions by calls already for the
  Clojure), `Rt.hs` (`Ed`), `caprice.go` (`-j`), `Profile.HsExports` (the
  wrappers). **Cost:** M. **Risk:** a call across modules is inlined only
  through the interface's unfoldings, so a hot small function in `Low` called
  from `Knot` might not be; the heavy case did not move. The suite keeps two
  builds (`.cache/caprice-suite/`) and would recompile only the parts that
  moved.
- **Value:** medium for idiom (modules a reader can open); high for the
  machine: **a 56% smaller peak** on a machine others share, where the
  suite compiles the candidate and its control side by side (two builds of
  4.0 GB today).

### 9. Structs, arrays and file-scope objects as Haskell data

- **Struct locals.** Of the 233 struct locals, 129 have their address taken
  (passed to a function as `&pos`); of the 104 that do not, 21 are only used
  by member and 66 copied whole to or from memory (`pos = curwin->w_cursor`),
  17 passed or returned whole. By type: `pos_T` 56, `optvar_T` 23,
  `string_T` 13. The 87 could be values -- a record, or simply one binding per
  member (scalar replacement, which the lowering can do) -- a copy from memory
  becoming a read of each member, a copy to memory a write of each; the 17
  need a record at a function boundary the callee also takes.
- **File-scope scalars.** 453 of the 474 scalar objects never have their
  address taken; 91 are never written by a function. As fields of an editor
  record, measured (`bench/Bench.hs`, 2 x 10^8 read-and-write each): the
  segment 0.11-0.13 s, **an `IORef Int64` field 0.50-0.54 s** (4.5 times:
  every write allocates the boxed value), an unboxed one-element array
  0.11-0.12 s; `curwin->m` read-and-written with `curwin` in the segment
  0.17-0.20 s, in an `IORef` 0.28 s. The fast form is the segment's own form,
  so item 3's names are the idiom and a record buys nothing. The 91 never
  written could be Haskell constants where their initializer is a constant;
  not counted further.
- **Arrays.** 81 local arrays (67 of bytes: buffers vim formats into) and 295
  file-scope arrays are walked by pointers (803 + 436 + 208 steps) and passed
  as `char *`: raw memory is their form.
- **Constraints any of this meets:** aliasing (a value can leave memory only
  when nothing takes its address: the counts above are that proof's first
  half, and a callee reaching it by name its second); the host's callbacks
  (`hs-boot`: `IObuff`'s address is the host's, `addr'IObuff`); and the
  parallel `:%s` (`match_lines`' runtime body runs the engines on `forkIO`'s
  threads, each on a `regengine_T` in raw memory, reading the segment's
  options concurrently -- which raw memory and an `IORef` both allow, and a
  pure record threaded through the calls would not).
- **Where:** the lowering (scalar replacement) and the printer. **Cost:** M-L.
  **Value:** low-medium: 87 locals, `pos_T` mostly.

### 10. Enumerations and switches

- **The pattern.** 25 enumerations proper (the rest are 829 one-constant
  enums, the preprocessor's `#define`s as the phases left them). Of the 70
  `switch`es, **14 switch over one enumeration** (`{ADDR_LINES..}` 7,
  `regstate_E`, `CLASS_ALNUM`'s 20, `CMD_index`, ...), 7 over one-constant
  enums (key codes), 23 over char literals; the rest mix.
- **What it would become.** A `data` type per enumeration and `case` on its
  constructors would be the idiom, but the values live in memory as C `int`s
  (a member, a file-scope object) and are compared and stored as integers:
  every load a `toEnum`, every store a `fromEnum`, and a value outside the
  enumeration -- which C does not forbid -- an error. Pattern synonyms (item 3)
  give the names in `case` at no cost; that is the part worth doing.
- **Cost:** M for the data types, S with item 3. **Value:** low beyond item 3.

### 11. Loops as folds

- **The pattern.** 919 loops in the C (502 `for`, 380 `while`, 37 `do`). 740
  store to memory or call something that does; 147 store only to locals and
  read memory (a fold over memory: `musl_strlen`, the table searches); 24
  return from inside (a search); **8** read no memory and call nothing
  impure.
- **What it would become.** The 147 are `foldM`/`go` over addresses, which is
  what item 2 prints them as; a `foldl'` over a list would allocate the list
  (the Clojure's boxing, item 9 there). Only the 8 are pure folds.
- **Value:** low: item 2 is the idiom for loops here.

### 12. The runtime and the host

- **`Rt.hs`**: `type P = Ptr ()` and the readers `rdI32` ... are C's view;
  with item 4 they become typed (`Storable` instances per struct are not the
  idiom here: the printer knows each member's offset and type at the site).
  `cBytes` returns a `[Word8]` list, used by the tests' harness only.
- **The host** is idiomatic Haskell already (`System.Posix`, `installHandler`,
  a `capi` import), but for its state: one record of `IORef`s behind
  `unsafePerformIO` (`st`, `Host.hs`), so one editor a process
  (`doc/HASKELL.md`, *Not done*). It would be a `Host` record in `Ed`, which
  item 8's table field needs anyway. **Cost:** S-M. **Value:** low for idiom,
  and what several editors a process need. **Done** (`94b34f3`, while this
  survey was written): `Ed` carries its editor's `Host` record and arena
  (`edHost`), the terminal is `Caprice.Term`, per instance, and
  `go test ./caprice/` runs four editors at once.

## Ranking (value against cost)

"GHC" and "heavy" are measured where a number is given, estimated otherwise.

| Rank | Item | Who | Cost | Risk | GHC time / peak | heavy | Idiom gained |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 0: GHC's time and peak reported | `caprice.go` | S | none | -- | -- | none directly: the measure every later item needs |
| 2 | 1: the printer's noise, SSA names first | printer | S-M | none | lets: 207.9 s (base 202.8-204.5), 4.02 GB | same | 36,565 `-Wall` warnings toward 0; 16,924 typed literals, 2,682 double conversions |
| 3 | 3: objects, members, constants by name | printer | S (objects), M | none | objects: 215.4 s (+6%); rest est. a few % | same | 11,657 segment sites, 16,052 member sites, 8,694 constants |
| 4 | 2: structure: blocks in place, loops as `go` | printer + lowering's nesting | M | low | est. unchanged (same Core) | est. same | 12,717 of 19,384 local functions gone; every function structured |
| 5 | 5: pure functions; `ed'` where used | shared analysis + printer | S | none | 207.9 s | same | 58 pure functions; 221 without `ed'` |
| 6 | 7: `initGlobals` as an image | printer | S | low | its part 10.0 -> 1.2 s; with 3 and 5: 203.0 s, 3.66 GB | same | 15,281 -> 1,779 lines |
| 7 | 8: modules by the call graph | printer + `Rt.hs` + `caprice.go` | M | cross-module inlining | 172.2 s, **1.77 GB** (-j4) | same | three modules and a top, not one |
| 8 | 4: typed pointers | printer + `Rt.hs` + host | M | host signatures move | est. unchanged | est. same | `Ptr WinT` for `Ptr ()` at 1,413 parameters and 236 results |
| 9 | 6: out-parameters as results | a C phase, or the printer | M | low | not measured | est. same | 78 out-parameters in 55 functions |
| 10 | 9: struct locals as values | lowering + printer | M-L | aliasing proof | not measured | not measured | 87 locals |
| 11 | 10: enumerations as `data` | printer | M | values outside the enum | -- | -- | 14 switches (item 3's synonyms first) |
| 12 | 12: the host's state in `Ed` -- **done**, `94b34f3` | host | S-M | none | -- | -- | several editors a process |
| -- | 4's newtypes, 4's `Maybe`, 11's folds, 9's `IORef`s | -- | -- | -- | -- | -- | not recommended (below) |

### Recommended first three

1. **Make GHC's cost visible, then take the noise out.** Item 0 in
   `caprice.go`, and item 1's rules in the printer, the SSA `let`s first --
   measured at GHC's time and the heavy case's, 34,916 of the 36,565 `-Wall`
   warnings. It changes no behaviour and no other backend, and it is the
   difference between "machine output" and "Haskell" on every screen.
2. **Names.** Item 3: the file-scope objects first (measured, 6% of GHC's
   time), then members and constants; with item 7's image in the same
   change, which pays GHC's time back.
3. **Structure.** Item 2 on the lowered form: a block reached once in place,
   loops as named local functions. It is the largest reading gain and, unlike
   the Clojure's, has no state machine left over, because a Haskell local
   function is a join.

Item 8 is the one to do early if memory, not reading, is what hurts: the
suite's two caprice builds side by side hold two peaks of 4.0 GB today.

## What is not worth doing, and why

- **The state in `IORef`s, a `State` monad or an immutable record.** 453
  file-scope scalars could leave the segment, but an `IORef` is measured 4.5
  times slower on a write and a record threaded through the calls would not
  be shared with `match_lines`' threads or the host's callbacks. The
  unboxed form is the segment. Item 3's names give the reading.
- **Structs as records.** 9,269 `p->m` reach structs other code holds
  (`curwin->w_cursor`, shared by the window and every caller): a record would
  make each change a new value no other holder sees. The Clojure declined the
  same (`CLOJURE-IDIOMS.md`); only the 87 locals of item 9 are values.
- **C strings as `ByteString` or `Text`.** A `char *` is walked (803 + 436 +
  208), subtracted (165), compared (71) and written in place, and 1,032
  pointer declarators are `char_u *`; a `ByteString` is immutable and has no
  position. The string literals are already the right form: `Ptr "..."#` is
  static memory, GHC's own primitive string. GO-IDIOMS' *Declined: the C
  strings as Go slices* holds as it stands.
- **`Maybe` for nullable pointers**, beyond perhaps the 94 functions that
  `return NULL`: measured at no speed cost, but the C tests and walks the same
  pointer, and the conversions would be everywhere.
- **Newtypes for `linenr_T`/`colnr_T`**: 1,004 mixed expressions, a floor, for
  a distinction the C does not keep past one arithmetic operation.
- **Pure functions of read-only memory** (`unsafePerformIO` on the 198
  readers): the memory is written by others; GHC would be free to share or
  float a read past a write.
- **`ReaderT Ed IO`**, unmeasured: it would remove 22,128 ` ed'`s, and add a
  bind GHC must inline at each of 39,107 binds (and every statement besides)
  in a compile whose simplifier is already 61%; item 5's narrower change
  first.
- **Folds over lists** for the 147 read-only loops: a list per loop, where the
  local `go` of item 2 is what a Haskell programmer writes in `IO`.
- **Tuning GHC's collector**: `-A256m`, `-H4g` no gain; `-F4` 7% faster for
  46% more memory.
- **Exceptions for FAIL**: as the Clojure found, FAIL carries nothing and the
  C has no `setjmp`; phase 166 made the yes-or-no functions `Bool`.

## The throwaway files

Under the worktree's `.tmp/hsid/`, none tracked:

- `core.c` (the cut core), `Editor.hs` (regenerated, byte-identical),
  `whim-vim` (the C, the one compile line), `base/` (caprice built by `whim
  caprice --out`);
- `scan/` (`main.go`, `effects.go`: the per-function counter and the effect
  closure; `SCAN_CUT` names callees to leave out), its outputs
  `Editor.hs.fns.tsv`, `Editor.hs.eff.tsv`, `Editor.hs.segoffs.tsv`,
  `scan.txt`;
- `zz_survey_test.go` (the C-side instrument, run as `HSID_CORE=core.c
  HSID_EFF=Editor.hs.eff.tsv HSID_OUT=. go test -run TestSurveyHs` in
  `crefactor/togo` and removed after), its outputs `survey.txt`,
  `segnames.tsv`, `globals.tsv`;
- `joins/`, `scc/`: the join-point and call-graph counters;
- `xf/` (`pure`, `names`, `lets`, `bytes`, `image`, `split`: the text
  transforms), their outputs `Editor.{pure,names,lets,image,ipn}.hs`,
  `split/`, `initexp/` (`initGlobals` alone, and folded);
- `build.sh`, `buildsplit.sh`, `exp/*/` (each variant's build: `time.txt`,
  GHC's `+RTS -s` in `ghc.log`, the program), `timings/` (`-ddump-timings`),
  `tc/` (`-fno-code`), `wall/` (`-Wall`'s 36,565 warnings);
- `heavy.sh`, `heavy1.sh` (one capability), `heavy.keys`;
- `bench/`: `Bench.hs` and `Maybe.hs`, the micro-benchmarks;
- `suite-hs.log`: `whim test --haskell` at `80db5f3`.
