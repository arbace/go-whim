# How the Clojure editor could be more idiomatic Clojure: a survey

2026-09-26. A read-only survey: no tracked file changed but this one. It covers
`vijure/src/whim/editor.clj` as tracked at `2ca3292` (76,975 lines, 2,205
top-level forms, 1,969 `defn`s, 106 `deftype`s; written by `crefactor/togo`'s
Clojure backend from the `src/editor.c` cut from the tracked `whim-vim.c`), and
the hand-written glue `cljhost.clj` (217 lines) and `cljmain.clj` (101 lines).
`doc/GO-IDIOMS.md` is its model: every item says what the pattern is, how many
sites it has, what it would become, who would do it -- the Clojure printer
(`clj_fn.go`, `clj_expr.go`, `clj.go`), the lowering and its nesting
(`lower.go`, `clj_shape.go`), the shared analysis (`jgen`'s types and pointer
classes, which the Java backend uses too), a pipeline phase that changes the
C, or the glue -- its cost (S/M/L), its risk, and how it is verified.
`editor.clj` is generated and stays so: nothing here is a change by hand to
it.

## The tension, stated first

The C is an imperative program over mutable memory: 1,441 of the 1,695 core
functions store something (a file-scope object, a struct member, an array
element or through a pointer), directly or through what they call; 91 of the
105 struct types are changed in place after they are made (2,376 setter calls
in the functions); and the program is one strongly connected knot (366
functions call each other in one cycle). Idiomatic Clojure is values, maps and
sequences. The two meet only part of the way, and the survey's answer is:

- **Idiom that changes only the text is cheap and safe**, and there is a lot
  of it: clj-kondo finds 2,993 warnings in `editor.clj`, and 1,424 + 836 of
  them are redundant `let`s and `do`s the printer writes. A Clojure reader's
  first impression is decided here.
- **Idiom in control flow is real and measured**: a better nesting takes 154
  of the 334 state machines to structured functions without copying code, and
  one step of it (joins that carry several values) was built for this survey
  and passes both suites at no measured cost.
- **Idiom in the data stops at the C's memory model**: immutable state,
  records for structs and Clojure strings would each mean rewriting vim, not
  translating it. What is reachable is at the edges: the 14 struct types and
  242 file-scope objects that are constants after start-up can be Clojure
  data, and the 68 struct members held in one-element arrays because C takes
  their address can be plain fields.
- **Speed is the constraint that decides the rest, and it is not measured by
  anything in the tree.** Measured here for the first time: on a heavy
  session the Clojure editor takes **48 times the C's time**, not because of
  Clojure but because 60 of its hot functions are over HotSpot's 8,000-byte
  limit and are never compiled. One JVM flag makes it 5.6 times faster. Every
  idiom item changes method sizes, so this comes first.

## How it was measured

Everything below was counted or run, not estimated, unless it says so. The
instruments were throwaway, in the worktree's `.tmp/s/` (listed at the end);
none is tracked.

| Instrument | What it gave |
| --- | --- |
| `whim skel .tmp/s/editor.c D -clj F` | `editor.clj` regenerated from the cut `editor.c`: byte-identical to the tracked file (so every count is of what `whim gen` writes) |
| `count.clj` (a throwaway Clojure program: reads the forms with `*read-eval*` off, never evaluates them) | heads of forms, patterns (`(let [_ ...])`, `(if c x nil)`, nested `and`/`or`, ...), type hints, file-scope reads and writes, enumerators, and every structured `loop` classified by how its variables change and whether its body stores |
| `graph.clj` (the same kind) | the call graph, its strongly connected components, the closure of "touches the editor" and "stores", each function's vim source file (by its definition in `/root/vim/src/*.c`, found for 1,611 of 1,675 names) and the file-level graph |
| `clj_xstats.go` + a patch to `clj_shape.go` (reverted) | per function: blocks, loops, states, carried variables, the joins refused and why; and the nesting re-tried with each rule relaxed, counting what would become structured and how long its text would be |
| the same patch, one rule made real (`CLJX_JOINN`) | an `editor.clj` with joins of several variables, built and run through `whim test --clojure-editor` and `--wide` |
| clj-kondo 2026.08.04 | not on the machine and not in `~/.m2`; clojars is reachable, so it was fetched into `.tmp/s/m2` and run on the three files (10 s) |
| `*unchecked-math* :warn-on-boxed` | the namespace loaded from source with boxed-math and auto-boxing warnings on |
| `MSize.java` (the JDK's `java.lang.classfile`) | the bytecode length of every method of the AOT-compiled classes |
| `heavy.keys`, below | a throughput session run on the C, Go, Java and Clojure editors, its output compared by digest, timed; and profiled with JDK Flight Recorder |

The heavy session (20,000 lines, four substitutions, a `:g` with `:normal`):

```
ithe quick brown fox jumps over the lazy dog 0123456789<Esc>yy20000p
:%s/o/0/g<CR>:%s/\v(qu)(i)/\2\1/g<CR>gg:g/f0x/normal wwdw<CR>G:%s/e/E/g<CR>:q!<CR>
```

**Verification recipe for every item** (the suite is exact, so a change that
moves one byte of one screen is seen):

- `go tool whim test --clojure` (45 cases) and `--wide --clojure` (240), the
  Clojure editor held to the C candidate, its own control (`" INSERT"`
  changed) required to move 41 of 45;
- `make whim-editor-check` (`whim gen --check`) after the regenerated
  `editor.clj` is committed, with `Editor.java` and `editor.go` unmoved where
  the change is the Clojure printer's alone;
- and, because the suites cannot see speed, the heavy session's time against
  the one before the change, and the method-size census (methods over 8,000
  bytes; the namespace class's `load()`, 54,382 of 65,535 bytes today).

## The findings

### 0. Speed: the huge-method limit (found on the way; a prerequisite)

- **The pattern.** HotSpot does not compile a method whose bytecode is over
  8,000 bytes (`-XX:+DontCompileHugeMethods`, the default); it runs
  interpreted forever. Measured on the AOT classes: **80 methods of
  `whim.editor` are over 8,000 bytes** -- 20 of them the once-run
  initialisers (`init_globals_10` 56,994) and `load()` (54,382), and 60 of them
  editing code: 13 groups of the split `regmatch` (11,547-25,215 each),
  the 18 of `win_line`, `win_update` 35,225, `nv_g_cmd` 34,130, `regatom`
  33,860, `ex_substitute` 29,660, `vgetc` 25,554, `regrepeat` 21,795,
  `handle_mapping` 11,580, `vgetorpeek` 11,783 ... The Java editor has one
  hot method over the limit (`regmatch`, 9,795).
- **Measured** (the heavy session, output identical on all five):

  | editor | s | with `-XX:-DontCompileHugeMethods` |
  | --- | ---: | ---: |
  | C, `-O0` | 1.62 | |
  | Go | 0.89 | |
  | Java (`bin/braaam`'s flags) | 4.20-4.35 | 2.41 |
  | Clojure (`bin/vijure`'s flags) | 77.8 | **13.9-15.0** |
  | Clojure, C2 (`TieredStopAtLevel` off) | 82.4 | 4.6 (5,000 lines; C1 3.9) |

  The flight recording without the flag: the `regmatch__N` groups hold almost
  every sample. With it, the first frame is `clojure.lang.Numbers.num(long)` --
  boxing -- at 22% of samples, called from the same groups (item 10).
- **The suite does not see it**: its 45 sessions run 0.38 s each whether the
  flag is on or off (17.4 s against 16.9-17.4 s for the 45 in sequence), and
  `--wide --clojure` is 10.3-10.8 s either way.
- **Where:** by hand, `vijure.go`'s and `braaam.go`'s launcher (one flag).
  Better, later: the generator keeps every function it writes under 8,000
  bytes, which the split already does for a machine over 64 KB
  (`Profile.CljSplit`) -- but the split groups are the slowest code there is
  (item 10), so the flag is the right first step. And a throughput number in
  `whim test` (a heavy case's time, reported, not judged), because every item
  below moves method sizes and nothing would notice a regression.
- **Cost:** S. **Risk:** none measured (the flag compiles large methods in
  the background; the short sessions are unchanged). **Value:** 5.6x on the
  Clojure editor, 1.8x on the Java one.

### 1. The printer's noise: what clj-kondo sees

clj-kondo, with the namespace's three macros told to it (`e`, `g`, `g!`, whose
arguments are names, not vars), gives `editor.clj` **0 errors and 2,993
warnings**, and the glue 1 note (`cljmain.clj:29`, a redundant `long`):

| Finding | Count | Example | What it becomes |
| --- | ---: | --- | --- |
| redundant `let` (a `let` whose body is a `let`) | 1,424 | `ml_get_buf` (9260): `(let [...] (let [...] ...))` | one `let` |
| redundant `do` | 836 | `check_cursor_lnum` (8677) | the forms in the enclosing body |
| unused binding | 729 | `ed` 143, `this__` 105, `o__` 104 (the deftypes' methods) | `_`, or the parameter gone (item 7) |
| a var name differing only in case | 3 | `nv_Zet` / `nv_zet` | vim's names, kept |
| unused import | 1 | `BoolPtr` | dropped |

And what the counter finds that no linter reports, in the 1,735 C functions:

| Pattern | Count | What it becomes |
| --- | ---: | --- |
| `(let [_ effect] ...)`: a step done for what it does bound to `_` | 11,240 bindings; 5,847 `let`s bind nothing else | `(do effect ...)`, the `let` only where a name is bound |
| `(let [...] nil)` | 3,119 | the steps, then `nil` once at the function's end, or nothing where the value is not used |
| `(if c x nil)` / `(if c nil x)` | 1,314 / 619 | `(when c x)` / `(when-not c x)` |
| `(do (if ...) ...)` | 1,284 | `(when ...)` as a statement |
| `(and (and a b) c)`, `(or (or ...))` | 793 / 395 | `(and a b c)` |
| `(let [x v] x)` | 890 | `v` (`ml_get_buf`: `(if (<= lnum 0) (let [lnum 1] lnum) lnum)` is `(if (<= lnum 0) 1 lnum)`) |
| `(let [i 0] (loop [i i] ...))` and other name-to-name bindings | 715 | `(loop [i 0] ...)` |
| a conversion of a conversion: `(unchecked-int (i32 x))`, `(unchecked-byte (u8 x))` | 250 + 142 | the outer one |
| a conversion of a constant: `(unchecked-int 0)`, `(unchecked-int (e X))`, `(unchecked-byte (e X))` | 210 + 132 + 199 | the constant |
| `(throw (IllegalStateException. "no state"))`, every machine's default | 332 | nothing: a `case` with no default throws already, and the arm is unreachable |
| `(== b (e NUL))` and `(e NUL)` | 975 uses of `NUL` | `(zero? b)`, `0` |

What `check_cursor_lnum` (8677) becomes with the rules of this item alone:

```clojure
;; now
(defn check_cursor_lnum [^Editor ed]
  (do (if (> (aget ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0) (.ml_line_count ^S_memline (.-b_ml (g ed curbuf))))
        (let [_ (aset ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0 (.ml_line_count ^S_memline (.-b_ml (g ed curbuf))))]
          nil)
        nil)
    (if (<= (aget ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0) 0)
      (let [_ (aset ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0 1)]
        nil)
      nil)))

;; with item 1
(defn check_cursor_lnum [^Editor ed]
  (when (> (aget ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0) (.ml_line_count ^S_memline (.-b_ml (g ed curbuf))))
    (aset ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0 (.ml_line_count ^S_memline (.-b_ml (g ed curbuf)))))
  (when (<= (aget ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0) 0)
    (aset ^longs (.-lnum ^T_pos_T (.-w_cursor (g ed curwin))) 0 1))
  nil)
```

(and with items 3 and 5, `(when (> (.lnum cursor) ...) (.set_lnum cursor ...))`).

- **Where:** the Clojure printer: `clj_shape.go`'s `emit`/`emitTerm` (the
  `let`, `do`, `when`, and joining nested `let`s), `clj_fn.go`'s step printing
  (`_` bindings), `clj_expr.go` (the conversions, `and`/`or`, `NUL`).
- **Cost:** S-M: each is a local rule on the printed form, one sitting each.
- **Risk:** none to behaviour -- each is an identity Clojure's compiler
  checks, and the bytecode of `(let [_ a] b)` and `(do a b)` is the same. Two
  things to watch: **the split's size guess is the text's length**
  (`shaper.cost`: `len(body)` plus the recurs' width, against 110,000), so a
  shorter text leaves a function whole that was split and may meet the 64 KB
  limit -- re-derive the guess from the census of item 0 when this lands; and
  `" INSERT"` must stay once in the file (no block holding a literal is
  copied, which none of these does).
- **Verify:** the suites, `whim-editor-check`, clj-kondo's counts of
  redundant `let`/`do` to 0, the census (no new method over 64 KB).
- **Value:** the largest for the least: it is what a Clojure reader sees on
  every screen.

### 2. Control flow: the 334 state machines

- **The pattern.** 1,358 functions are structured (`let`, `if`, `case`,
  `loop`/`recur`) and 334 are state machines, `(loop [st 0 x x ...] (case st
  ...))`, 2 of them split. The machines are the long ones: 11,721 of the
  19,296 blocks and **708 of the 896 loops of the C are in them**, so only
  188 C loops are Clojure `loop`s today. A machine has a median of 4 states
  and 6 carried variables; `regmatch` has 387 states and its recurs carry 36
  variables each (`regmatch__10`: every jump is a `recur` of 37 arguments).
- **Why they are machines** (`whim skel`'s report, by the first thing that
  would not nest): 232 a join refused, 52 a loop whose changes are read
  after it, 36 a loop left to more than one place, 8 a loop's exit with
  another way in, 6 an irreducible loop. Behind the 232, the joins the
  nesting refused in the machines, by reason: a loop inside the region
  between the branch and its join 277, a join inside the region not written
  in place 271 (a cascade: it falls with the others), more than one variable
  changed 237 (`k=2` 187, `k=3` 37, more 11), a return inside the region 149,
  a block of the region reached from outside it 88.
- **Measured: what each better rule would structure** (the nesting re-tried
  on each machine with the rule relaxed; "size" is the text against the
  machine's):

  | Rule | Machines structured | Loops they hold | Size |
  | --- | ---: | ---: | ---: |
  | J: a join whose arms change several variables: `(let [[s neg] (if ...)] join)` | 57 | 44 | 0.97 |
  | L: a loop whose changes are read after it, or left to several places, has a value: `(let [[i p] (loop ...)] after)`, `(case exit ...)` | 31 | | 0.94 |
  | J + L | 91 | 104 | 0.97 |
  | J + L + P: a loop inside a join's region | 103 | 133 | 0.99 |
  | **J + L + P + R: a return inside a join's region, "returns as values" (a tag in the join's tuple)** | **154** | **170** | **0.99** |
  | all of these, and a loop's exit reached from elsewhere copied, and irreducible loops split | 197 | 293 | 1.24 |
  | copy every block with several ways in (tail duplication) | 241-325 | | 6.5-7.6 |

  So **154 of 334 (46%) become structured with no code copied**, 197 (59%)
  with a quarter more text, and copying past that explodes (7.6 times the
  text; the 64 KB method and `load()` limits forbid it).
- **J was built and run.** Joins of several variables written as a vector
  and taken apart, `(let [j__1 (if ...) ^BytePtr s (nth j__1 0) neg (long
  (nth j__1 1))] ...)`: 1,415 structured and 277 machines (from 1,358 and
  334), `win_line` no longer split, the file 3,004 lines shorter; **`whim
  test --clojure-editor` answers all 45 and all 240 as the C does, the
  control seen by 41**, and the time is unchanged (the wide Clojure group
  10.3-10.7 s against 10.6-10.8 s; the heavy session with item 0's flag
  14.4-15.2 s against 14.6-15.0 s): the vector is made once per join
  executed, and nothing measured it. `musl_atoi` (8387), a machine today:

  ```clojure
  (defn musl_atoi ^long [^Editor ed ^BytePtr s]
    (let [n 0
          neg 0]
      (loop [^BytePtr s s]
        (if (musl_isspace ed (long (.get s)))
          (recur (.add s 1))
          (let [[^BytePtr s neg] (if (== (long (.get s)) 45)
                                   [(.add s 1) 1]
                                   (if (== (long (.get s)) 43) [(.add s 1) neg] [s neg]))]
            (loop [^BytePtr s s
                   n n]
              (if (musl_isdigit ed (long (.get s)))
                (recur (.add s 1) (i32 (- (i32 (* 10 n)) (i32 (- (long (.get s)) 48)))))
                (if (zero? neg) (i32 (- n)) n))))))))
  ```

  (as written by the built rule, with item 1's `let`s folded by hand for
  reading; the C is `while (isspace(*s)) s++; switch (*s) { case '-': neg=1;
  case '+': s++; }` and a digit loop).
- **What is left** (137 functions, 118 without the copying): joins whose
  region is entered from outside (a `goto`, a `break` across a nest), loops
  left from inside a nested loop to two places, and the 49 `goto`s of the C
  (8 functions). The general answer is a local function per join (`letfn`),
  which Clojure cannot `recur` through and calls without primitives -- worse
  than a machine for speed -- so these stay machines.
- **Where:** the nesting (`clj_shape.go`: `findJoins`, `loopForm`, `edge`), on
  the lowered form, which any target without jumps shares.
- **Cost:** M (J is done in a throwaway form; L, P, R are of the same kind).
- **Risk:** low to behaviour (the suite sees every miswritten branch); the
  method sizes move -- a function structured is sometimes longer in bytecode
  than its machine, and J made `win_line` one method of 33,514 bytes where it
  was 18 groups -- so item 0 first.
- **Verify:** the suites, the census, the heavy session's time; and the
  coverage report (`clj: N structured, M state machines`) as the measure.
- **Value:** high: a machine is where the Clojure reads least like Clojure,
  and they are the longest functions.

### 3. Names

- **The pattern.** Every name is the C's: 1,509 of the 1,675 function names
  have an underscore (`ml_get_buf`); struct types are `S_file_buffer` and
  `T_pos_T` behind interfaces `I_S_file_buffer`; members keep C's prefixes
  (`b_ml`, `w_cursor`); a name Clojure has takes an underscore (`inc_`,
  `dec_`, `gettext_`; locals `count_` 1,320 uses, `name_`, `type_`,
  `next_`). No predicate ends in `?`.
- **What it would become.** kebab-case everywhere (`ml-get-buf`,
  `w-cursor`); **0 of the 1,675 names collide with `clojure.core`'s 679 public
  vars once converted** (so the trailing underscores could mostly go too:
  `inc_` stays, `name_` need not, a local may shadow a core name);
  `?` on the **88 functions returning `bool` that store nothing, even through
  what they call** (`buf_valid?`, `musl_isdigit?`, `ends_excmd?`) -- of 285
  `bool` functions, the rest act and report (`ml_append`, `coladvance`) and
  keep their names. **Not `!`**: 1,441 of 1,695 functions store, so a bang
  would mark nearly all of them and say nothing.
- **Where:** the printer's name map (`cljName`, `memberName`); the glue's
  names (`cljhost.clj` resolves `host-of`, `deathtrap`, `gettext_`, `emsg`,
  `iemsg`, `emsg_iobuff_room`, `iobuff_or`, `utfc_ptr2len`, `utf_ptr2cells`;
  `cljmain.clj` calls `vim_main`; the host functions are called by C names),
  and `doc/CLOJURE.md`'s contract, which names them.
- **Cost:** S-M. **Risk:** none to behaviour; the three case-only pairs
  (`nv_Zet`/`nv_zet`) stay distinct. What it costs is what GO-IDIOMS item 13
  weighed: grepping the Clojure for a C name. A comment line per function
  with its C name keeps that.
- **Value:** medium: after item 1, the most visible sign the file is not C.

### 4. The constants as data

- **The pattern.** The C's file-scope objects are per editor, in slots
  filled at `new-editor` by 17 `init-globals-N` and 2 `make-objects-N`
  functions -- 9,535 lines (67,401-76,935), and 20 of item 0's 80 huge
  methods. But much of it is constant: of the 820 slots, **242 are never
  written by any function** (`g!`) -- 216 byte arrays (the messages,
  `e_ml_get_invalid_lnum_nr`), 26 integers -- and **14 of the 105 struct
  types are never changed after start-up**: the tables (`S_nv_cmd`,
  `S_cmdname`, `S_vimoption`, `S_key_name_entry`'s kin, `S_interval` and
  `T_convertStruct` -- the Unicode tables, whose `.set_first_`/`.set_last_`
  are 935 lines each of initialisers).
- **What it would become.** Namespace-level immutable data the editors
  share: the messages Clojure strings or one shared `byte[]` each, the tables
  vectors of maps (`[{:name "append" :fn ex-append :argt ...} ...]`) or of
  records -- read, as the enumerators and slots already are, from strings or
  resources, not built by code in `load()`.
- **Where:** the shared analysis must prove "never written, also not through
  a pointer" (a message array is `char[]`, not `const`, and is passed as
  `char *`) -- `jgen`'s pointer classes know which flows reach a store --
  then the printer.
- **Cost:** M-L. **Risk:** the `load()` limit (54,382 of 65,535 bytes: every
  top-level form costs about 24; 242 `def`s would take 5.8 KB of the 11 KB
  left, so tables in one form, not a `def` each); a table's element read
  today as `(.name_ ^S_cmdname (aget ...))` becomes a map lookup, slower,
  in `find_command`'s loops -- measure with the heavy session. The contract
  already allows a shared constant for `IObuff`'s kin.
- **Value:** medium-high: the part of the file a Clojure programmer would
  expect to be data is 12% of it, and it is code.

### 5. Struct members whose address is taken

- **The pattern.** A member whose address C takes is a one-element array in
  its `deftype` (`^longs lnum` in `T_pos_T`), read `(aget ^longs (.-lnum
  ^T_pos_T p) 0)`: **68 members, their address taken at 91 sites, read that
  way 2,392 times** -- `pos_T.lnum` alone 1,200, because of 11 `&pos.lnum`s.
  The Java has the same (1,119 `.lnum[0]` in `Editor.java`); the Go takes an
  address natively (`w_cursor.lnum`, 492).
- **What it would become.** `(.lnum p)` / `(.set_lnum p v)`, like the other
  members.
- **Where:** either a pipeline phase that removes the `&s.m` (pass the struct,
  or a local copied back) where the C is the cause, which serves the Java
  and the C too; or the shared analysis and the runtime: a field pointer
  (`LongPtr` over an accessor pair, made at the 91 sites), with the member a
  plain field.
- **Cost:** M. **Risk:** low; a field pointer must stay one pointer for
  identity (`Ptr/is`). **Value:** medium: `lnum` and `col` are on every other
  line of the cursor code, and the Java gains the same.

### 6. The functions of bytes: `musl_*`

- **The pattern.** 29 `musl_*` functions are translated byte by byte
  (`musl_strlen` 128 calls, `musl_strcmp`, `musl_strchr`, `musl_atoi`, the
  `is*`/`to*`); 13 of the 42 read-only loops of item 9 are theirs.
- **What it would become.** Calls to the runtime (`BytePtr` methods, as the
  allocators and `musl_mem*` already are: 8 functions "the runtime's",
  `RuntimeBody.Clj`), GO-IDIOMS item 5's answer.
- **Where:** the profile's replacement table (`internal/whim/gen.go`) and
  `braaam/rt`. **Cost:** S. **Risk:** low (a runtime self-test against the
  translation). **Value:** low-medium.

### 7. The editor as the first parameter

- **The pattern.** Every C function takes `ed` first. **224 functions touch no
  file-scope object and no host function, even through what they call**
  (`musl_*`, `getdigits`, `check_cursor_moved`, `vim_strsave`, ...), and 143
  do not use `ed` at all (clj-kondo).
- **What it would become.** Functions of their arguments -- the plainest sign
  of a pure function a Clojure reader has -- except the 165 functions used as
  values, whose signature is their table's.
- **Where:** the shared analysis (the closure) and the printer, at each
  call. **Cost:** S-M. **Risk:** none; a primitive signature keeps one
  parameter fewer to its 4 (item 10). **Value:** low-medium.

### 8. Namespaces

- **The pattern.** One namespace of 2,205 top-level forms, `load()` at
  54,382 of 65,535 bytes.
- **Measured.** By vim's source files it cannot be split: of 55 files, **54
  are one strongly connected component** of the file-level call graph, and
  3,914 of the 5,793 call edges cross files. By the call graph it can: one
  component of 366 functions (22%), 3 of 2, and 1,329 functions in no cycle.
  So: `whim.editor.types` (the 106 deftypes), a `.data` (item 4), layers of
  the acyclic functions by topological order (`.lib` for the leaves: the
  224 of item 7, `charset`, `mbyte`), the knot in one namespace, and
  `whim.editor` the entry the contract names. The 40 `declare`s become
  fewer, not more.
- **Where:** the printer (`fnOrder` already orders by calls). **Cost:** M.
  **Risk:** direct linking across namespaces keeps calls direct; the glue's
  `requiring-resolve` names move. **Value:** modest for reading; needed as
  headroom before anything adds top-level forms (items 3's comment lines, 4).

### 9. Loops that are sequence functions

- **The pattern.** Of the 188 structured `loop`s (the other 708 C loops are
  inside machines, item 2): **42 store nothing** -- a counter to a bound (19:
  `find_termcode` 12582, `name_to_mod_mask` 12570), a pointer walked (21:
  `vim_strchr` 9394, `gettail`), a list followed (2) -- and are `some`,
  `reduce` or an index search; 65 count with effects (`dotimes` where the
  counter's test is the only exit, an upper bound); 7 follow a list with
  effects (`u_freeentries` 32186: `(doseq [e (take-while some? (iterate
  #(.ue_next ^S_u_entry %) first))] ...)`); 38 change only memory, no
  local.
- **The cost.** `range`, `some` and `iterate` box every element; in
  `find_termcode` that is per key typed. A Clojure programmer writes
  `loop`/`recur` in hot code and accepts it; `dotimes` is primitive and free.
- **Where:** the lowering's loops, recognised on the lowered form (a counter
  loop, a walk), printed by the printer. **Cost:** M. **Risk:** order of
  evaluation (`iterate` reads the next element after the body; C reads it
  before -- `u_freeentries` frees the entry). **Value:** low-medium, and
  grows with item 2.

### 10. Arithmetic, hints and boxing

- **The pattern.** `i32` 3,938 (`+` 1,163, `-` 1,144, `inc` 644, `dec` 299),
  `u32` 398, `i8`/`u8`/`i16`/`u16` 217; `unchecked-int` 1,131 and
  `unchecked-byte` 591 on stores; `(long (f ...))` around 2,484 calls;
  18,590 type hints in the functions (`^BytePtr` 4,128, `^ints` 2,157,
  `^T_pos_T` 2,018, `^Editor` 1,729).
- **What is essential.** `i32` on `+ - * inc dec` is C's 32-bit `int`: vim's
  overflow guards are written for it, and gcc wraps where C leaves it
  undefined -- keep it; it is two JVM instructions (`l2i`, `i2l`). A range
  analysis could prove a loop counter bounded by an `int` compare and drop
  its `i32` (most of the 644 `inc`s), at L cost for little. What is noise is
  item 1's conversions of conversions and of constants, and `(long ...)`
  around a call whose result is already a primitive `long` (a no-op kept
  for the functions of more than 4 parameters, which return Objects).
- **What is measured.** `*warn-on-reflection*`: 0 warnings (the build refuses
  one). **`*unchecked-math* :warn-on-boxed`: 0 boxed-math and 0 auto-boxing
  warnings** -- the arithmetic is primitive. The boxing item 0's profile
  shows is at calls: Clojure passes primitives only to functions of at most 4
  parameters, and **200 of the 1,965 functions have more** (the editor
  counts); 216 `Boolean/valueOf` and 79 `Long/valueOf`/`Integer/valueOf` are
  the rest. The split machines' frames (`fl__` `long[]`, `fo__`
  `Object[]`) are the hot case.
- **Where:** the build: add `:warn-on-boxed` to the refused warnings (S), so
  items 1-9 cannot bring boxed math in. The 4-parameter limit is Clojure's;
  passing a struct of arguments is not idiom, so leave it.

## Ranking (value against effort)

| Rank | Item | Who | Cost | Risk to suite / speed / limits | Idiom gained |
| --- | --- | --- | --- | --- | --- |
| 1 | 0: the huge-method flag, a throughput number, `:warn-on-boxed` refused | hand (`vijure.go`, `braaam.go`), `whim test` | S | none measured; 5.6x on the Clojure editor, 1.8x on the Java | none directly -- the measurement every later item needs |
| 2 | 1: the printer's noise | Clojure printer | S-M | none to behaviour; re-derive the split's size guess | 1,424 + 836 kondo warnings, 11,240 `_` bindings, ~2,900 `if`/`when`/`and` forms, ~1,000 conversions |
| 3 | 2: more functions structured (J built; L, P, R) | lowering / nesting | M | low; method sizes move (item 0 first) | 154 of 334 machines (170 loops) with no copying |
| 4 | 3: kebab-case, `?` on 88 predicates | printer + glue + contract | S-M | none; a C name no longer greps | every name |
| 5 | 5: members with their address taken | a C phase, or the shared analysis + runtime | M | low | 2,392 `(aget ^longs (.-m s) 0)`; the Java's 1,119 too |
| 6 | 4: the constants as data | shared analysis + printer | M-L | `load()` (11 KB left); a table lookup's speed | 9,535 lines of initialisers; 14 table types, 242 constant slots |
| 7 | 8: namespaces by the call graph's layers | printer | M | direct linking kept; glue names move | headroom under `load()`; a namespace a reader can open |
| 8 | 7: no `ed` where nothing is touched | shared analysis + printer | S-M | none | 224 functions visibly pure |
| 9 | 6: `musl_*` onto the runtime | profile table + `braaam/rt` | S | low | 29 byte loops |
| 10 | 9: sequence functions for loops | lowering + printer | M | evaluation order; boxing in hot loops | 42 loops certain, up to ~110 |
| -- | 10: arithmetic beyond item 1 | -- | -- | -- | not recommended (below) |

### Recommended first three

1. **Make speed visible, and fix the flag.** `-XX:-DontCompileHugeMethods` in
   both JVM launchers (the heavy session 77.8 s to 13.9 s for Clojure, 4.2 s
   to 2.4 s for Java; the suite unchanged), a timed heavy case reported by
   `whim test --clojure` (and `--java`), `:warn-on-boxed` refused like
   reflection. Without it, items 1-3 cannot be judged: every one of them
   moves method sizes, and the suite would stay green through a tenfold
   slowdown, as it has.
2. **Clean the printed forms.** Item 1's rules in the printer: the file
   loses the forms clj-kondo calls redundant, `_` bindings become `do`, `if
   ... nil` becomes `when`, conversions of conversions go. Nothing can change
   behaviour, and it is the difference between "machine output" and
   "Clojure" on every screen.
3. **Nest more.** Item 2's J (built and passing both suites here), then L, P
   and R: 154 of the 334 state machines structured, 170 of the 708 loops
   inside machines written as `loop`s, at no measured cost. It is also what
   item 9 needs.

## What is not worth doing, and why

- **The state as a map, an atom or records.** The 820 file-scope objects are
  read 11,347 and written 1,504 times in the functions; `(g ed curwin)` is an
  `aget` on a typed array, the fastest the JVM has, and reads as what it is.
  An atom is a CAS and an allocation per write; an immutable map threaded
  through every call is a rewrite of vim, not a translation. A macro or a
  `def` per object (`(curwin ed)`) would add 820 top-level forms, about 20 KB
  of `load()`'s last 11 KB. Grouping the objects by subsystem is possible --
  536 of the 817 the functions use are used by one vim file's functions only
  -- but it reorganises the slots without making them less mutable: cosmetic.
- **Structs as records or maps.** 91 of the 105 types are changed in place
  (504 setters called 2,376 times) and reached through pointers other code
  holds (`curwin->w_cursor`, shared by the window and every caller): a record
  would make each change a new object that no other holder sees. The
  deftype behind an accessor interface was chosen on measurement
  (`doc/CLOJURE.md`: as fast as slot arrays, one object). Only the 14 constant
  table types are records' business (item 4).
- **C strings as Clojure strings.** GO-IDIOMS' *Declined: the C strings as Go
  slices* holds as it stands: every `char *` is one pointer class, compared
  and subtracted across the file; a Clojure `String` has no position and
  cannot be written. The 677 `BytePtr/lit` literals flow into that class.
  Only the constant messages (item 4) could be strings, where the analysis
  proves them read-only.
- **FAIL/OK as exceptions (`ex-info`).** Phase 166 made the 278 yes-or-no
  functions `bool`; what is left is 86 uses of `OK`/`FAIL` (functions whose
  status has a third value) and 893 of `TRUE`/`FALSE` in `int` flags that
  hold other values too. FAIL carries nothing -- the message went through
  `emsg` already -- and an exception would be control flow the C does not
  have, with a cost per throw; the C has no `setjmp` to map it to.
- **`!` on functions that store.** 1,441 of 1,695: it would mark the file,
  not distinguish anything.
- **Tail duplication past item 2's rules.** Copying every block with several
  ways in structures 241-325 of the machines at 6.5-7.6 times their text: the
  64 KB limit, `load()` and the huge-method limit all forbid it.
- **Namespaces by vim's source files.** 54 of the 55 files are one cycle
  (item 8).
- **Dropping `i32` by a range analysis, or C `int` as a Clojure `long`.** GO-IDIOMS
  item 9's reasoning: vim's guards are 32-bit, and the masking is two
  instructions.

## The throwaway files

Under the worktree's `.tmp/s/`, none tracked:

- `count.clj`, `graph.clj`: the form and graph counters (`ONLY=c clojure -M
  count.clj ed.clj`; `clojure -M graph.clj ed.clj vimfns.tsv`), with
  `vimfns.tsv` (vim's function definitions by file), `ed.clj.loops.edn`,
  `pure.txt`, `nowrite.txt`, `nostore.txt`, `boolfns.txt`, `types.txt`;
- `instrument.diff` and `clj_xstats.go.txt`: the nesting instrument and the
  built J rule (the patch to `clj_shape.go`, reverted), `stats.tsv` its
  output, `editorJ.clj` the namespace with J;
- `vb/`, `vj/`, `jv/`: the Clojure editor as tracked, with J, and the Java
  editor, built with `go tool whim clj --out` / `java --out`; `whim-vim` and
  `whim-go` the C and Go editors; `heavy.keys`, `h5.keys` the sessions;
  `vb.jfr`, `vb2.jfr` the flight recordings; `mz/MSize.java` and
  `msize-*.tsv` the method-size census;
- `kondo/`: clj-kondo's output, the tool itself in `m2/`.
