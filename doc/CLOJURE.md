# The editor in Clojure: a plan

Written 2026-09-25, when the core's C was translated to Go (`editor/`) and to
Java (`jeditor/`), and the Go was also written as go-lisp (`doc/GO-LISP.md`).
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
  (`jeditor/rt`: `BytePtr` and its kin, `Ptr<T>`, the growarray) and its host
  (`jeditor/host`: the `Host`, `vim_snprintf`, the terminal through the
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
3. **The host and the suite.** Clojure glue to `jeditor/host` (the `Host`,
   `vim_snprintf`, the terminal), a launcher (`bin/whim-clj`, and an uberjar),
   and `whim test --clojure` / `--wide --clojure`: the 45 and 240 cases
   answered as the C does, with the Clojure editor's own control.
4. **Kept current.** `editor.clj` generated by `whim gen` beside `editor.go`
   and `Editor.java`, tracked, and refused by `whim-editor-check` when stale.

## Risks, named in advance

- **Compile time and size.** One namespace of about 80,000 lines, compiled by
  Clojure's single-pass compiler: measured in milestone 1 on the slice, and
  split into namespaces if it is slow.
- **Speed.** Boxed arithmetic or reflection anywhere in a hot path is orders
  of magnitude slower; the reflection check refuses it, and the suite's
  timings show the rest.
- **Readability.** The state-machine functions will not read as Clojure, as
  the Go and the Java read as C. That is the price of a faithful translation;
  the structured strategy keeps it to the functions that jump.
