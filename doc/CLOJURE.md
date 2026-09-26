# The editor in Clojure: a plan

The Clojure editor is called **vijure** (`vijure/`; `bin/vijure`, `vijure.jar`).

Written 2026-09-25, when the core's C was translated to Go (`editor/`) and to
Java (`braaam/`), and the Go was also written as go-lisp (`doc/GO-LISP.md`).
A Clojure editor is the next target, held to the same test as the others:
`whim test --clojure`, every case answered as the C answers it.

## What Clojure asks that Java did not

Measured on `editor/editor.go`, whose control flow is the C's (2026-09-25):

| of 1,685 functions | |
|---|---:|
| a `return` that is not the body's last statement | 663 |
| a `break` | 243 |
| a `continue` | 51 |
| a `goto` (the 49 left in the C) | 8 |
| a `fallthrough` | 10 |
| **none of these** -- straight-line or nested code only | **923** |
| over 500 lines (`initGlobals` 3,542; `regmatch` 1,007; `win_line` 783; `regatom` 736; `win_update` 643; `ex_substitute` 631; `do_put` 582) | 7 |

- **No statements that jump.** Clojure has no `return`, `break`, `continue`,
  `goto` or labeled break: control leaves a form only by its value, and a loop
  repeats only by `recur` in tail position. 762 functions (45%) jump.
- **No mutable locals.** A local is a binding; C assigns to locals in nearly
  every function. `loop`/`recur` rebinding, or a box, is what Clojure offers.
- **No switch fall-through.** `case` takes one branch.
- **The same JVM.** Clojure calls Java directly, so the Java editor's runtime
  (`braaam/rt`: `BytePtr` and its kin, `Ptr<T>`, the growarray) and its host
  (`braaam/host`: the `Host`, `vim_snprintf`, the terminal through the
  Foreign Function & Memory API) serve a Clojure editor as they are. The
  JVM's limits hold too: 64 KB of bytecode a method, and Clojure writes more
  bytecode per statement than javac.
- **Dynamic by default.** Without type hints every call is reflective: the
  output must be fully hinted (`^long`, `^bytes`, `^whim.rt.BytePtr`), compiled
  with `*warn-on-reflection*` and `*unchecked-math*`, and a reflection
  warning refused.

Clojure is on the machine: the `clojure` CLI (1.12.5) on JDK 26, starting in
0.8 s.

## The approach

**A lowered form between the C and the printer.** The Java backend prints
Java straight from the C's tree, because Java has every statement C has.
Clojure does not, so the backend first lowers each function to **basic
blocks**: straight-line statements in three-address form (each C expression
with side effects taken apart, each assignment its own step), ended by a jump,
a branch or a return. The types and pointer kinds are the Java backend's
decisions, reused. This form is the part other targets without `goto` or
labeled break (Python, Lua, Scheme) would share.

**Two ways to print a function**, chosen per function:

1. **Structured**, for a function whose blocks nest (the 923, and those whose
   early returns and breaks lower to nested `if`s): `let` for each
   assignment (a new binding shadowing the old), `if`/`cond`/`case` for
   branches, `loop`/`recur` for loops, with the locals a loop changes as its
   bindings. Readable, and close to the C's shape.
2. **A state machine**, for the rest: `(loop [block 0, x ..., y ...] (case block
   0 (let [...] (recur 3 x' y)) 1 ...))` -- each basic block a branch of one
   `case`, each jump a `recur` with the block's number and the live locals.
   It says any control flow, `goto` and fall-through included, in one
   mechanism, and `case` on ints compiles to a table switch.

An address-taken local is a box, as in Java (`IntPtr` over one element). A
function over the 64 KB limit is split: its state machine's blocks into
groups, each a function, dispatched by the block number.

**The state.** The C's file-scope objects are an editor's, as in the Go and
the Java. Candidates, to be measured in milestone 1: a `deftype` with
`^:unsynchronized-mutable` fields and every function a method (fast; one class
of 1,470 methods and 821 fields to compile), or typed slot arrays per kind
(`long-array`, `object-array`) behind accessor macros. Structs likewise:
`deftype`s behind generated accessor interfaces, or slot arrays.

## The contract between the generated code and the hand-written

Fixed before milestones 1-3 start, so they can be written side by side:

- **The generated namespace is `whim.editor`**, in `vijure/src/whim/editor.clj`
  (written by `togo`'s Clojure backend; milestone 4 tracks it).
- **It provides** `(new-editor host)`, an editor on `host` -- a
  `whim.host.Host`, the Java editor's interface, unchanged -- with the C's
  file-scope objects initialised; `(host-of ed)`, that host back; and every C
  function of the core as a Clojure function of the same name taking the
  editor first: `(vim_main ed argc argv)`, with `argv` a `whim.rt.Ptr` of
  `whim.rt.BytePtr`, as the Java's.
- **The C's host functions are `whim.cljhost`'s** (`vijure/src/whim/cljhost.clj`,
  by hand), called as `(whim.cljhost/host_write ed p n)` and so on: the names
  and C argument types of the Java editor's 17 abstract methods (`host_alloc`,
  `host_exit`, `host_message`, `host_raise`, `host_time`, `host_write`,
  `musl_delay`, `musl_get_winsize`, `musl_host_init`, `musl_now_ms`,
  `musl_read_input`, `musl_suspend`, `musl_term_start`, `musl_term_stop`,
  `musl_tty_keys`, `musl_wait_for_input`, `vim_snprintf`), the editor first.
  `vim_snprintf`'s variadic arguments are one `Object` array, boxed as the
  Java editor boxes them (Printf's rules). `whim.cljhost` reaches the core
  functions its printf needs (`emsg`, `gettext_` ...) through
  `requiring-resolve`, so the two namespaces do not require each other.
- **The launcher** is `whim.cljmain/-main`: the terminal host
  (`whim.host.Term`), `new-editor`, `vim_main`, its status the exit status.
- **Primitive C types** are Clojure's: `long` for every integer (narrowed and
  masked by the C type where C does), `boolean` for C's `bool`; pointers the
  Java runtime's classes.

## Milestones

Each verified before the next, as the Java's were.

1. **The lowered form and a slice on foreign C.** Lowering to basic blocks
   and three-address form, with its own tests: every foreign C program of the
   Java tests, lowered and then printed back as C, compiles and prints what
   the original prints. Then the Clojure printer for the slice the Java's
   milestone 1 covered (integers signed and unsigned, strings, arrays,
   structs, control flow, fields and methods), both printing strategies, and
   the state representation chosen by measurement. Verified by the same
   programs run through `clojure` and required to print what gcc's build
   prints, with a control. And a coverage report on `editor.c`: functions
   written structured, as state machines, and refused, by reason.
2. **Every function.** The coverage report's refusals, by frequency: function
   pointers (Clojure `fn`s or the Java backend's interfaces), unions, the
   growarray, varargs, the 64 KB split. Measured by every function written,
   no reflection warning, and the namespace compiling.
3. **The host and the suite.** Clojure glue to `braaam/host` (the `Host`,
   `vim_snprintf`, the terminal), a launcher (`bin/vijure`, and an uberjar),
   and `whim test --clojure` / `--wide --clojure`: the 45 and 240 cases
   answered as the C does, with the Clojure editor's own control.
4. **Kept current.** `editor.clj` generated by `whim gen` beside `editor.go`
   and `Editor.java`, tracked, and refused by `whim-editor-check` when stale.
   **Done**: `whim gen` writes `vijure/src/whim/editor.clj` (76,975 lines)
   from the same `editor.c`, refusing outright if the backend refused any
   part of it; `whim gen --check` refuses a stale one -- measured with a
   control, one byte of the tracked file changed. `whim gen` writes the three
   translations in 13.8 s.

## Milestone 3, as built

Written 2026-09-25, beside milestones 1-2 (then in progress in another worktree; *Milestones 1 and 2, as built*),
so built against the contract above and proven on a **stand-in**:
`vijure/testdata/standin/whim/editor.clj`, a hand-written `whim.editor`
that provides what the contract says the generated one does and calls every
host function as the core would -- no editor, but every line of glue run.

**The pieces**, each the Clojure of a Java file in `braaam/`:

| Clojure / Go | Java | what |
|---|---|---|
| `src/whim/cljhost.clj` | `Whim.java`'s glue | the 17 host functions, the editor first, each a line of glue to `(whim.editor/host-of ed)`, a `whim.host.Host`; fully type-hinted, `*warn-on-reflection*` clean |
| `src/whim/cljmain.clj` | `Whim.java`'s `main`s | `run` (a new editor on any Host, `vim_main`, an `Exit` caught as its code) and `-main` (`:gen-class`): the terminal host, argv as the Java builds it, a 1 GiB core thread, a throwable reported as `vijure: <exception>` and its first 24 frames, status 70 |
| `vijure.go` | `braaam.go` | `Generate` (the cut, the backend), `Compile`, `AOT`, `Jar`, `Train`, `WriteLauncher` |
| `internal/suite/clojure.go` | `internal/suite/java.go` | `whim test --clojure`: the build and its control |

**The glue.** `whim.cljhost` does not require `whim.editor` (the core
requires it): it reaches `host-of`, `deathtrap` and what the printf needs
through `requiring-resolve`, each resolved once, on its first call.
`musl_host_init` hands the Host an `IntConsumer` that calls
`(whim.editor/deathtrap ed sig)`. `host_alloc` keeps the arena's accounting
(1 GiB, 16-byte rounding, the exhaustion message and `host_exit 1`) per
editor, in a weak map keyed by the editor's Host -- a Java object, so an
identity, and one a host -- whose value (a `long[1]`) refers to neither, so
the map lets both go. `vim_snprintf` is `whim.host.Printf` on a `Printf.Core`
reified over the editor per call; its `iobuff` and `eValTooLarge` read the
core's `IObuff` and `e_val_too_large`, which only the printf's own error paths
do. The host functions with at most four arguments take and return primitive
`long`s (`^long`), so a direct-linked caller that sees their arglists invokes
them without boxing; `musl_tty_keys` and `vim_snprintf` have more and take
Objects.

**The launcher.** `whim.cljmain/-main` reads the arguments' bytes from
`/proc/self/cmdline` as `Whim.argBytes` does (the bytes the launcher was
handed, not their decoding), `argv[0]` from the system property `whim.argv0`
(the script's `$0`), and runs the core on a thread of 1 GiB of stack, as the
Java. `bin/vijure` is `exec java <braaam's flags> -XX:AOTCache=...
-Xlog:aot=off,cds=off -cp lib/vijure/vijure.jar -Dwhim.argv0="$0"
whim.cljmain "$@"`.

**The build** (`go tool whim clj [--out DIR] [--editor F] [--jar F] [FILE]`,
`make bin/vijure`, `make vijure.jar`; `CLJ_EDITOR=F` for a namespace
written already): the core cut from `whim-vim.c` (`braaam.Cut`), the
namespace generated by `whim skel <editor.c> <dir> -clj <out.clj>` -- a
toolset without that mode writes nothing, and the build says so
(`vijure.ErrNoBackend`) rather than compile an empty namespace --
braaam's `rt/` and `host/` compiled by javac (`braaam.WriteRuntime`: the
Java sources without `Whim.java`, which needs an `Editor.java`), Clojure's
jars found by `clojure -Spath` and copied into `lib/`, and every namespace
AOT-compiled from `whim.cljmain` down (it requires `whim.editor` and
`whim.cljhost`) with `*warn-on-reflection*` and `*unchecked-math*` on,
direct linking, and `:doc`/`:file`/`:line`/`:added` metadata elided. **A
reflection warning anywhere fails the build**, the compiler's lines quoted
(tested). Then the classes and Clojure's jars are merged into one executable
jar, every entry keeping its time: Clojure loads a namespace's AOT class only
when it is NEWER than the `.clj` beside it, and a jar whose entries all had
one time compiled `clojure.core` from source at every start (2.1 s against
0.6 s -- measured, then fixed). Measured on the stand-in: 6.7 s, of which the
AOT 2 s and the training run below 2.7 s.

**Startup**, measured on the stand-in (Clojure's runtime and the glue load;
the real namespace adds its own classes), a run to `q`, JDK 26:

| how | ms |
|---|---:|
| classes directory and jars on the class path | 620-680 |
| the merged jar, braaam's flags | 590 |
| `java -jar vijure.jar` (no -XX flags) | 700 |
| the merged jar and an **AOT cache** (`-XX:AOTCache`, JEP 483/514) | **205-250** |
| the same with a dynamic CDS archive (`-XX:ArchiveClassesAtExit`; the JVM also printed an error of a flag mismatch) | 255-280 |
| the Java editor to a stub, for scale (`doc/JAVA.md`) | 188 |

So the build trains an AOT cache: one run of the editor on `ihello<Esc>:q!<CR>`
with `-XX:AOTCacheOutput`, which JDK 26 writes at the JVM's exit **even when
the editor ends by `System/exit`** -- unlike the AppCDS archive the Java
editor tried (`-XX:+AutoCreateSharedArchive`). It needs a class path of jars
only (a directory refuses the dump: "Cannot have non-empty directory in
paths"), which is why the launcher runs the merged jar. A cache the JVM
cannot use -- the jar rebuilt, another JDK -- is dropped silently by
`-Xlog:aot=off,cds=off` (otherwise the JVM says so on the editor's screen),
and costs only the time. A JDK without AOT caches writes none, and the
launcher runs without it. Direct linking is on by reasoning (a var deref and
an interface call saved on every call between namespaces), not measured: the
stand-in makes too few calls to show it.

**The suite**: `whim test --clojure` and `--wide --clojure` (`make
whim-test-clj`), after the C and the Go comparisons and beside `--java`: the
namespace generated once from the candidate, it and its control (`" INSERT"`
changed in `editor.clj`, required there exactly once) compiled side by side,
and every case run on the C candidate and the Clojure editor, held to the
same bytes and status, the control required to move the Clojure editor's own
answers. `internal/suite/java.go` became the JVM editors' common part (a
`jvmEditor`: its name, its generated file, its launcher's name and the frame
of its core -- `whim.editor$fn` for the Clojure, `Editor.fn` for the Java);
what stopped an editor is named by the exception and the first frame of the
core among those printed. `--clojure-editor F` runs the suite on the
namespace in F. On the stand-in the whole path runs: 45 cases differ (it is
no editor), its control seen by 45, and `1 thrown in vim_main:
java.lang.IllegalStateException: ...`; wide, the pty cases run it on a real
terminal. Tests: `vijure`'s (the stand-in built and run through every host
function, statuses 0, 3, 5, 1 and 70, the argument bytes `\xff\xfe` intact,
`java -jar`; a reflective call refused; a generator that writes nothing
named) and `internal/suite`'s `TestClojureStandIn` (the build and control of
the suite, the control moving only the case that types an `i`, the report of
an exception read back).

**What the glue assumes of `whim.editor`**, beyond the contract's words --
for the merge to check:

- `(new-editor host)`, `(host-of ed)` returning the `whim.host.Host`, and
  `(vim_main ed argc argv)`, `argc` a long, `argv` a `whim.rt.Ptr` of
  `BytePtr`s ending in nil; its value a number, the exit status.
- As core functions of the editor first, resolved by name: `deathtrap`
  `(ed sig)`, `gettext_` `(ed BytePtr) -> BytePtr`, `emsg` and `iemsg` `(ed
  BytePtr)`, `emsg_iobuff_room` `(ed) -> number`, `iobuff_or` `(ed BytePtr) ->
  BytePtr`, `utfc_ptr2len` and `utf_ptr2cells` `(ed BytePtr) -> number`.
- **The file-scope objects `IObuff` and `e_val_too_large`** -- the contract
  says nothing of the state's form -- as a var `whim.editor/IObuff`
  (`e_val_too_large`) holding either a function of the editor that returns
  the object, or the object itself (a constant the editors share); a
  `BytePtr` or a `byte[]`. Only the printf's error paths read them.
- The host functions called with the C's argument types as the contract has
  them: integers as longs (a boxed Integer or Long is converted), pointers the
  runtime's classes, `vim_snprintf`'s variadic arguments one `Object[]`.
  Their values: `long` for the C's `int` and `long` results (`musl_get_winsize`
  and `musl_tty_keys` 1 for OK, 0 for FAIL; `musl_wait_for_input` 1 or 0),
  nil for `void`, and `host_alloc` a `BytePtr` as Object.
- `whim.editor` requires `whim.cljhost` (so the glue is compiled first and the
  core's calls see its primitive signatures), and compiles with no
  reflection warning under `*warn-on-reflection*`, which the build sets for
  every namespace.
- `" INSERT"`, the suite's control, appears in the generated `editor.clj`
  exactly once, spelled so (a Clojure string literal is spelled as the C's).

## Milestones 1 and 2, as built

Written 2026-09-25. Both milestones were built together, in
`crefactor/togo`, and the result met milestone 3's glue on main: **every
function of `editor.c` written, none refused, the namespace compiling ahead
of time with no reflection warning, and `whim test --clojure` answering all
45 cases, and `--wide --clojure` all 240, exactly as the C does** -- the
control seen by 41 of the 45 and, wide, as the Go's is (94 keys, 6 pty).

### Where it lives

| file | what |
|---|---|
| `crefactor/togo/lower.go` | the lowered form: a function as basic blocks |
| `crefactor/togo/lower_c.go` | the lowered form printed back as C: `whim skel <editor.c> <dir> -lowerc <out.c>` |
| `crefactor/togo/clj.go` | the namespace: struct types, the editor's state, initial values, the order, the report |
| `crefactor/togo/clj_expr.go` | expressions: `java_expr.go`'s decisions on long-valued integers |
| `crefactor/togo/clj_fn.go` | a function: its steps and terminators printed, its header, initializers, the split |
| `crefactor/togo/clj_shape.go` | the nesting: joins, loops, state machines |
| `crefactor/togo/lower_test.go`, `clj_test.go` | the tests |

`whim skel <editor.c> <dir> -clj <out.clj>` writes the namespace and beside it
`<out.clj>.refused` (empty on `editor.c`), and logs the coverage:

```
clj: 1693 of 1701 functions written (1358 structured, 334 state machines, 2 of them split), 8 the runtime's, 0 refused
clj: state machines, by the first thing that would not nest:
    232  a jump to a block with ways in: a join whose arms change more than one variable it reads, or leave it
     52  a loop whose changes are read after it
     36  a loop left to more than one place
      8  a loop's exit with another way in
      6  an irreducible loop
```

The 8 are the Java's: the allocators and `musl_mem*`, whose calls are the
runtime's. `ga_grow_inner` is a rule of the profile's, as for the Go and the
Java (`RuntimeBody.Clj`, `internal/whim/gen.go`). The profile also names what
the host reads of the core's state (`CljExports`: `IObuff`,
`e_val_too_large`, each a function of the editor).

### Milestone 1: the lowered form

`lowerFunction` turns a C function into basic blocks, entry first and then
in reverse postorder, each a list of **steps** -- an assignment (`=`, `op=`,
`++`, `--`, each its own step), a call evaluated for what it does, a
declared object's initializer -- ended by a jump, a branch, a switch (its
case values and targets, the default last) or a return. What an expression
does besides giving its value is taken apart before it into temporaries: an
assignment inside it (its value the lvalue, read back), `x++` (the old value
in a temporary), the comma's left operands, a compound literal (storage made
there), `&&`, `||` and `?:` whose later operands do something (a truth value
or the chosen arm in a temporary, the operand's steps only where C evaluates
them). The expressions left are the C's own nodes, read through the
function's substitutions, so the backends' pattern rules (a cast of the
growarray's member, `k * sizeof(T)`, a null constant) see what they saw.

**Deviation: calls stay where C has them**, not each in a temporary
(three-address form in the strict sense): the Java evaluates them in place,
left to right, and that order is the one the Java editor is proven with. A
call is taken apart only where it would be evaluated twice -- an lvalue
read and then written (`*f() += 1`, `a[g()]++`). One order the tests found
where gcc and Java differ: gcc reads a compound assignment's left side
after the right side's calls (`g += bump(3)`, `bump` changing `g`); the
lowering does too (a right side that calls is a temporary first).

**Its tests** (`lower_test.go`): every program of the Java tests, and one of
its own (if-else arms that both go on, `?:` `&&` `||` with effects as values
and statements, a goto backwards, a switch in a loop that falls through and
continues, a case label in a block), lowered, printed back as C, compiled by
gcc and required to print what the original prints. **And the whole core**:
`whim skel editor.c D -lowerc L.c`, `L.c` with `whim-vim.c`'s host appended,
built with the one compile line -- `whim test L` and `whim test --wide L`
find all 45 and all 240 cases behaving exactly as HEAD's. The first run found
an if-else whose then-arm jumped into its else-arm, which the programs had
not reached.

### Milestone 1 and 2: the Clojure printer

**Values.** Every C integer is a Clojure `long` holding the C value -- a
signed type sign-extended, an unsigned one narrower than 64 bits
zero-extended, a 64-bit unsigned one its bits -- so C's comparisons,
division and right shift are Clojure's own, but for 64-bit unsigned
(`Long/compareUnsigned`, `divideUnsigned`, `remainderUnsigned`,
`unsigned-bit-shift-right`). What can leave a type's range -- `+ - * <<` and
unary `-` of a type narrower than 64 bits, `~` of an unsigned one, a
conversion to a narrower type -- is brought back by the namespace's macros
`i8 u8 i16 u16 i32 u32` (`(i32 (+ a b))`); what cannot (`& | ^`, a widening,
a shift right) is written bare. A Java array or runtime pointer holds bits in
the element's primitive: a load widens them (`(bit-and (.at p i) 0xff)` for
an unsigned char), a store narrows (`unchecked-byte`). `*unchecked-math*` is
on. C's `bool` is a Clojure boolean; a test of an integer is `(zero? x)` with
the `if`'s arms swapped, of a pointer `(nil? p)`. Pointers, arrays, the
allocators, the functions of bytes, the growarray (`GA_IntPtr` and kin, over
`Ga`), memcmp of structs, `Ptr/is`: the Java backend's decisions (`jgen`,
reused, not re-derived), on the same runtime classes.

**Structs: a deftype behind an interface of accessors** (`definterface
I_S_pos`), its scalar and pointer members mutable fields read `(.lnum p)` and
written `(.set_lnum p v)`, its struct, array and boxed members final fields
`(.-w_cursor wp)`; it is a `whim.rt.Struct` (`set`, `zero`, for
`Rt/moveStructs` and `zeroStructs`) with `copy` and, where some code compares
it, `eq`; `new-S_x` and `array-S_x` make them. A union is the same, every
member its own field. **Why a deftype, measured**: a loop of 10^8
read-modify-writes of a long field and every 1,024th of a pointer field took
32 ms through the accessor interface and 32 ms through a struct of typed
slot arrays (`(aget ^longs (.-l r) 3)`) once the JIT settled (51 and 60 ms
the first round); so the deftype, which is one object where slot arrays are
three, and reads as the C (`(.lnum pos)` against `(aget (.-l pos) 0)`). A
deftype's mutable fields are not public -- Clojure's compiler resolves `.-x`
only on public fields -- hence the interface; a pointer member's accessor
returns Object, since two struct types that point at each other cannot name
each other's class in their definitions, and the reader hints it. The 106
types compile in about 1 s (105 types of 40 fields each, measured apart:
1.9 s with the JVM's start).

**The editor's state: typed slot arrays**, `(deftype Editor [host ^longs L
^booleans Z ^objects O])` -- integers in `L`, booleans in `Z`, everything
else (pointers, arrays, structs, boxes, functions) in `O` -- read `(g ed
curwin)` and written `(g! ed curwin v)` by macros over a table of every
object's slot and class. **Why, measured**: a deftype's constructor takes
every field, and a JVM method takes at most 255 parameter slots (a long two),
so the 821 objects cannot be one type's fields; several deftypes behind one
would be a second dereference and a type per group, and the accessor
interface measured no faster than the slots (above). `(new-editor host)` makes
the arrays, the objects the slots start with (`make-objects-N`) and the
initial values (`init-globals-N`, 300 forms each); `(host-of ed)` is the
host.

**Functions.** `(defn name ^long [^Editor ed ^long a ^BytePtr p] ...)`: the
editor first, then the C's parameters. Clojure takes primitive parameters and
results only in functions of at most four parameters: those have `^long`
integers; a longer one takes Objects and makes its integers `long` as it
starts. A boolean parameter or result is a Boolean. A call's integer result
is `(long ...)` (a no-op on a primitive one); a struct is passed and returned
as a copy. A function used as a value is the function itself (its var's
value), so two uses compare `identical?` as C's pointers compare; where the
pointer's parameter classes are not the function's (a `Ptr` against a plain
struct), a def'd adapter, made once. A call through a pointer is `(fp ed a
b)`. A function declared and not defined is the host's: `(whim.cljhost/f ed
...)`. The variadic `vim_snprintf`'s arguments past the format are one
`(object-array [...])`, boxed as the Java boxes them (an int an `Integer`, an
unsigned int a `Long` of its value, a pointer itself, NULL nil). Locals: a
scalar or pointer whose address is taken is a one-element Java array (the
Java's box), a struct or array local an object made as the function starts;
neither is ever rebound. Every other local is a binding **rebound by `let`**
at each assignment. A C name Clojure has (`inc`, `dec` -- vim's own -- and
any of `clojure.core`'s, the special forms, the namespace's macros) takes a
trailing underscore, as a Java reserved word does in `Editor.java`: `inc_`,
`dec_`; `_` is `gettext_` by the profile.

**The nesting** (`clj_shape.go`). Each block's steps are one `let` (a step
done for what it does bound to `_`), its terminator the `let`'s body: an
`if`, a `case`, a value. Blocks are then written in place of the one edge
into them; a **join** of a branch -- where an if's or a switch's arms meet
again -- is written once after the branch when every path from it reaches the
join and nothing else, and the arms change at most one variable the join
reads: `(let [r (if (> x 0) 1 -1)] ...)`, or `(do (if ...) ...)` with none; a
**natural loop**, where the function is otherwise structured, is `(loop [t t
i i] ...)` over the variables it changes that its head reads, its back edges
`recur`, the code after it where it leaves (each exit the one way into what
follows) -- or, nested in a loop reachable from there, `(do (loop ...)
after)`, which needs one exit and nothing changed that is read after; a
small block that returns is written at each way into it. A function all of
whose blocks nest so is **structured** (1,358); the others (334) are **state
machines**: `(loop [st 0, x x, ...] (case st 0 ... 1 ...))`, each block no
rule nests a state, each jump to one `(recur k x ...)` with the variables
live at some state, and the joins still written in place inside the states.
A function whose machine is too large for a method is **split** (2:
`win_line`, `regmatch`): every block a state, the states in groups, each
group `(defn- f__N ^long [ed fl__ fo__ st])` running its own states on a
frame of the carried variables (`fl__` their longs, `fo__` the rest and the
result) and returning the next group's state or -1; the function fills the
frame and runs the groups. The size is guessed from the text and the
recurs' width (`Profile.CljSplit`, default 110,000); the largest function
left whole, `ex_substitute`, compiles.

**The namespace's own limit, found on the way**: compiled ahead of time
(the glue's build), every top-level form adds about 24 bytes to one method
of the namespace's class, `load()` -- and 64 KB hold about 2,700. A declare
of every function (1,700), a def per enumerator (926) and a literal map of
the slots (built by code in one static initializer) did not fit. So the
functions are written each after the functions it names (a `declare` only
for the 40 a cycle or an adapter reaches first -- which also lets a direct-linked
call see the callee's primitive signature), the enumerators are one table
read by the macro `(e FAIL)`, and both tables are read from strings. The
namespace now has about 2,200 top-level forms; a core much larger would need
the namespace split across files (`load`).

**Measured** (2026-09-25, `editor.c` of 73,632 lines): `whim skel -clj` 4 s;
`editor.clj` 76,975 lines, 4.6 MB, 1,969 defns and 106 deftypes (105 struct and union types and `Editor`); loaded from
source (JIT-compiled) in 10-12 s, compiled ahead of time in 9.4 s, 2,357
classes (11 MB), no reflection warning either way; `go tool whim clj` builds
`bin/vijure` in 23 s; a run to `:q!` in 0.34 s with the AOT cache. `whim
test --clojure`: the Clojure cases 2.6 s (the Java's 1.1 s); `--wide
--clojure` 11-13 s.

**The tests** (`clj_test.go`, skipped without gcc, javac, java or clojure;
each program's run bounded and killed with the test): the programs of the
Java tests and the lowering's, translated with every function written,
loaded by `clojure.main` with the runtime and a `whim.cljhost` of the tests'
(`out`, `outs`, a printf of the boxed arguments), any output on stderr -- a
reflection warning -- a failure, and required to print what gcc's build
prints: integers, flow, strings, structs, the profile's allocators and
functions of bytes, varargs, the growarray, member addresses, function
pointers and unions, gotos, a dead label. `TestCljSplit` runs four with
every state machine split; `TestCljShapes` requires a loop, a goto backwards
and a join structured and a join of two variables a state machine;
`TestCljControl` undoes one rule at a time -- an unsigned int's range, a
short's, an unsigned char's widening, 64-bit unsigned division, a struct's
copy, a function pointer, a join's test -- and each moves the output;
`TestCljRefuses` names floating point and a variadic definition, the rest of
the namespace loading and running. `whim gen --check` is clean: the Go and
the Java do not move.

### For the glue, as built

What the glue assumed (*Milestone 3, as built*) holds: `(new-editor host)`,
`(host-of ed)`, `(vim_main ed argc argv)` -- argc a long, argv a `Ptr` of
`BytePtr`s, its value a long; `deathtrap`, `gettext_`, `emsg`, `iemsg`,
`iobuff_or`, `emsg_iobuff_room`, `utfc_ptr2len`, `utf_ptr2cells` as core
functions of the editor first; `IObuff` and `e_val_too_large` functions of
the editor; host functions called with longs, the runtime's pointers and one
`Object[]`, their integer results read as `(long ...)`; `whim.editor`
requires `whim.cljhost`; `" INSERT"` once (a block holding a string literal
is never copied).

## Risks, named in advance

- **Compile time and size.** One namespace of about 80,000 lines, compiled by
  Clojure's single-pass compiler: measured in milestone 1 on the slice, and
  split into namespaces if it is slow. (As built: 77,000 lines, 10 s; the
  limit met was not the time but the namespace class's one load method,
  above.)
- **Speed.** Boxed arithmetic or reflection anywhere in a hot path is orders
  of magnitude slower; the reflection check refuses it, and the suite's
  timings show the rest.
- **Readability.** The state-machine functions will not read as Clojure, as
  the Go and the Java read as C. That is the price of a faithful translation;
  the structured strategy keeps it to the functions that jump.
