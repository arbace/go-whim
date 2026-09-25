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

## The contract between the generated code and the hand-written

Fixed before milestones 1-3 start, so they can be written side by side:

- **The generated namespace is `whim.editor`**, in `cljeditor/src/whim/editor.clj`
  (written by `togo`'s Clojure backend; milestone 4 tracks it).
- **It provides** `(new-editor host)`, an editor on `host` -- a
  `whim.host.Host`, the Java editor's interface, unchanged -- with the C's
  file-scope objects initialised; `(host-of ed)`, that host back; and every C
  function of the core as a Clojure function of the same name taking the
  editor first: `(vim_main ed argc argv)`, with `argv` a `whim.rt.Ptr` of
  `whim.rt.BytePtr`, as the Java's.
- **The C's host functions are `whim.cljhost`'s** (`cljeditor/src/whim/cljhost.clj`,
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
3. **The host and the suite.** Clojure glue to `jeditor/host` (the `Host`,
   `vim_snprintf`, the terminal), a launcher (`bin/whim-clj`, and an uberjar),
   and `whim test --clojure` / `--wide --clojure`: the 45 and 240 cases
   answered as the C does, with the Clojure editor's own control.
4. **Kept current.** `editor.clj` generated by `whim gen` beside `editor.go`
   and `Editor.java`, tracked, and refused by `whim-editor-check` when stale.

## Milestone 3, as built

Written 2026-09-25, beside milestones 1-2 (in progress in another worktree),
so built against the contract above and proven on a **stand-in**:
`cljeditor/testdata/standin/whim/editor.clj`, a hand-written `whim.editor`
that provides what the contract says the generated one does and calls every
host function as the core would -- no editor, but every line of glue run.

**The pieces**, each the Clojure of a Java file in `jeditor/`:

| Clojure / Go | Java | what |
|---|---|---|
| `src/whim/cljhost.clj` | `Whim.java`'s glue | the 17 host functions, the editor first, each a line of glue to `(whim.editor/host-of ed)`, a `whim.host.Host`; fully type-hinted, `*warn-on-reflection*` clean |
| `src/whim/cljmain.clj` | `Whim.java`'s `main`s | `run` (a new editor on any Host, `vim_main`, an `Exit` caught as its code) and `-main` (`:gen-class`): the terminal host, argv as the Java builds it, a 1 GiB core thread, a throwable reported as `whim-clj: <exception>` and its first 24 frames, status 70 |
| `cljeditor.go` | `jeditor.go` | `Generate` (the cut, the backend), `Compile`, `AOT`, `Jar`, `Train`, `WriteLauncher` |
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
Java. `bin/whim-clj` is `exec java <jeditor's flags> -XX:AOTCache=...
-Xlog:aot=off,cds=off -cp lib/clj/whim-clj.jar -Dwhim.argv0="$0"
whim.cljmain "$@"`.

**The build** (`go tool whim clj [--out DIR] [--editor F] [--jar F] [FILE]`,
`make bin/whim-clj`, `make cljeditor.jar`; `CLJ_EDITOR=F` for a namespace
written already): the core cut from `whim-vim.c` (`jeditor.Cut`), the
namespace generated by `whim skel <editor.c> <dir> -clj <out.clj>` -- a
toolset without that mode writes nothing, and the build says so
(`cljeditor.ErrNoBackend`) rather than compile an empty namespace --
jeditor's `rt/` and `host/` compiled by javac (`jeditor.WriteRuntime`: the
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
| the merged jar, jeditor's flags | 590 |
| `java -jar cljeditor.jar` (no -XX flags) | 700 |
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
terminal. Tests: `cljeditor`'s (the stand-in built and run through every host
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
