# The editor in Scheme: a survey

**A survey, not scheduled** (2026-10-02): there is no intention yet to
translate the editor to Scheme. It is kept as what was found and how it would
be done, should that change, and as the list of what must be measured before
an implementation is chosen for good.

Written 2026-10-02, when the core's C was translated to Go (`editor/`), Java
(`braaam/`), Clojure (`vijure/`) and Haskell (`caprice/`), each answering all
80 quick and 240 wide cases of `whim test` as the C does, and a Rust editor was
planned (`doc/RUST.md`). A Scheme editor would be held to the same test --
`whim test --scheme`, `--wide --scheme` -- and to the heavy case: no slower
than 25 times the C (as CLAUDE.md records it: the Go 0.5-0.6, the Haskell 1.0-1.3, the
Java 1.6-2.1, the Clojure 3.4-3.9; the C itself 0.44 s on 2026-09-29,
`doc/CLOJURE-PROFILE.md`).

What the target has to take: the core cut from `src/whim-vim.c` -- 1,710
functions (`doc/HASKELL.md`), a few of them huge (`regmatch`, `win_line`,
`regatom`, `ex_substitute`, `do_put` over 500 lines each in the Go,
`doc/CLOJURE.md`), the 49 gotos in 8 functions, 10 functions that fall
through a `switch` -- and the 17 host functions of `editor/host.go`. No
floating point; but 64-bit integers throughout (`linenr_T` is `long`: 420
uses; `long` 621, `varnumber_T` 34, `uvarnumber_T` 25 -- counted in
`src/whim-vim.c` with `grep -oE`).

## 1. What is on the machine

**No Scheme is installed.** Measured:

```
$ for c in scheme chez petite racket raco guile csc csi gsi gsc gosh \
    chibi-scheme loko cyclone mit-scheme gxi stklos kawa s7 bigloo; do
    command -v $c; done                               # nothing
$ cat /etc/alpine-release; uname -m
3.25.0_alpha20260805
x86_64
```

What Alpine offers (`apk update; apk search -x NAME; apk info -a NAME; apk
policy NAME`; the repositories are edge's main, community and testing):

| package | version | repo | installed size | notes from its APKBUILD |
|---|---|---|---:|---|
| `chez-scheme` | 10.3.0-r2 | community | 4.7 MiB | `--machine=ta6le` (threaded x86_64), `--enable-libffi`; ships `libkernel.a`, `main.o`, `scheme.h` and the boot files (package contents, pkgs.alpinelinux.org) |
| `racket` | 9.3-r0 | community | 177 MiB | Racket CS (`--enable-csonly`), the *minimal* distribution: no `r6rs` collection and no `raco exe` command in its file list |
| `guile` | 3.0.9-r2 | **main** | 50 MiB | plus `guile-bytestructures` 2.0.2 |
| `chicken` | 6.0.0-r0 | community | 5.6 MiB | depends on `gcc` and `libc-dev`: it compiles through C |
| `gambit` | 4.9.5-r1 | **testing** | 76 MiB | `--enable-single-host`, no `--enable-smp` |
| `cyclone` | 0.36.0-r3 | community | 30 MiB | built on `ck` (Concurrency Kit) |
| `chibi-scheme` | 0.12-r0 | community | 4.0 MiB | |
| `tinyscheme` | 1.42-r1 | community | 176 KiB | an R5RS-subset interpreter |
| Gauche, Loko, MIT Scheme, Gerbil, Kawa, s7, Bigloo, STklos | -- | -- | -- | **not packaged** (`apk search` finds nothing; `mosh` is the shell, not Mosh Scheme) |

All the packaged ones are x86_64 musl builds (`so:libc.musl-x86_64.so.1` among
their dependencies). Also on the machine and relevant: gcc (the C builds),
GHC 9.14.1, rustc 1.98.1, OpenJDK 26.

Nothing was installed for this survey (§7 says what to install first, and
what to measure).

## 2. The candidates

Upstream versions and dates are from the projects' release pages, queried on
2026-10-02 (GitHub's API through `gh api repos/OWNER/REPO/releases` or
`/tags`; GitLab's API for Loko and Kawa; the GNU and CHICKEN download
listings). The benchmark column is the place in the *Test ranks* list of the
r7rs-benchmarks, "Generated at 2026-02-03", run in safe mode on an Intel
i3-N305 with Arch Linux's packages (https://ecraven.github.io/r7rs-benchmarks/):
it orders implementations by how often they finished a benchmark in the top
nine, so it is a rough class, not a factor.

| | Alpine | upstream latest | standard | compiles to | parallel OS threads | r7rs-bench place (version) |
|---|---|---|---|---|---|---|
| **Chez Scheme** | 10.3.0 | 10.4.1, 2026-05-12 | R6RS | native code | **yes** (pthreads, no global lock) | **1st** (10.3.0) |
| **Racket CS** | 9.3 | 9.3, 2026-08-13 | its own; `#lang r6rs`/`r7rs` as packages | native (Chez) | yes, since 9.0 (`#:pool`) | 4th (9.0) |
| **Guile** | 3.0.9 | 3.0.11, 2025-11-30 | R6RS, R7RS, its own | bytecode + JIT | **yes**, no global lock | 9th (3.0.11) |
| **CHICKEN** | 6.0.0 | 6.0.0, 2026-08-10 | R5RS, R7RS | C | no (green threads) | 12th (5.4.0) |
| **Gambit** | 4.9.5 (testing) | 4.9.8, tag 2026-08-09 | R7RS mostly, its own | C | no by default (SMP scheduler opt-in, off) | 3rd (4.9.7) |
| **Gauche** | -- | 0.9.15, 2024-04-24 | R7RS | VM bytecode | yes (pthreads) | last of those listed (0.9.15) |
| **Chibi** | 0.12 | 0.12, 2026-03-18 | R7RS | VM interpreter | no (VM threads; separate VMs per OS thread) | not listed (0.11.0) |
| **Loko** | -- | 0.13.0, 2026-06-27 | R6RS, R7RS | native code | no (fibers) | 2nd (0.12.1) |
| **Cyclone** | 0.36.0 | 0.36.0, 2024-02-14 | R7RS | C | yes ("native multithreading") | 11th (0.36.0) |
| **MIT Scheme** | -- | 12.1, 2023-01-07 | R7RS, its own | native code | no (SMP "preliminary", on a branch) | 10th (12.1) |

**Chez Scheme** (https://github.com/cisco/ChezScheme). R6RS, compiled to
native code by a nanopass compiler. Releases 10.0.0 (2024-02-06), 10.1.0
(2024-11-14), 10.2.0 (2025-05-06), 10.3.0 (2025-10-29), 10.4.0 and 10.4.1
(May 2026): about two a year; of the last 40 commits, 14 are Matthew
Flatt's and 12 Bob Burger's. What makes it
fit (release notes, https://cisco.github.io/ChezScheme/release_notes/v10.0/release_notes.html):
threaded builds the default ("Running configure assumes a threaded target
machine type", §2.3), a parallel garbage collector (§2.5), "fx+/wraparound,
fx-/wraparound, fx*/wraparound, fxsll/wraparound" (§2.7) -- C's wrapping
arithmetic on fixnums -- a compiler that "consistently 'lifts' procedures"
with only known call sites into extra arguments (§2.4), executable-relative
and embeddable boot files and vfasl for fast start-up (§2.22-§2.26: `Sregister_boot_file_fd_segment`
"for boot files that are embedded with an executable segment"), and,
since 9.5.1, compile times "lower, sometimes by an order of magnitude or
more, for procedures with thousands of parameters, local variables, and
compiler-introduced temporaries" (§4.10). R6RS gives `bytevector-u32-native-ref`
and kin as standard; Chez adds `foreign-ref`/`foreign-set!` on raw addresses,
`foreign-procedure` (no C compiler needed), `define-ftype` for C structs, and
`register-signal-handler` (CSUG, https://cisco.github.io/ChezScheme/csug/foreign.html,
`system.html`). Threads are pthreads; "most primitive Scheme procedures are
thread-safe" (CSUG chapter 15, `threads.html`). Proper tail calls and cheap
one-shot continuations are the language's.

**Racket CS** (https://racket-lang.org/). Chez underneath, so the same code
generator. A release about every quarter (9.0 2025-11-23, 9.1 2026-02-24, 9.2
2026-05-28, 9.3 2026-08-13). Parallel threads arrived in 9.0
(https://blog.racket-lang.org/2025/11/parallel-threads.html). Against it here:
a linklet larger than `PLT_CS_COMPILE_LIMIT` (default 10000) has its outer
contour *interpreted*, only the functions small enough within it compiled
(https://docs.racket-lang.org/reference/compiler.html) -- the core is far
larger, and its biggest functions may not be "small enough"; the Alpine
package is the minimal distribution, so `#lang r6rs` and `raco exe` would
come from `raco pkg install`; and Racket's own dialect is not "some
standard". Its strengths over plain Chez (contracts, `raco`, a large
library) are not what a generated core needs.

**Guile** (https://www.gnu.org/software/guile/). GNU's extension language:
R6RS and R7RS libraries beside its own modules ("a fully conforming
implementation of R7RS, with the exception of the occasional bug and a couple
of unimplemented features",
https://www.gnu.org/software/guile/manual/html_node/R7RS-Support.html), bytevectors with the R6RS
typed accessors, `(system foreign)` for the FFI through libffi, POSIX
threads "with no global interpreter lock"
(https://wingolog.org/archives/2011/08/30/the-gnu-extension-language). A
bytecode VM with a JIT since 3.0. Releases are slow now: 3.0.9 2023-01-25,
3.0.10 2024-06-23, 3.0.11 2025-11-30 (ftp.gnu.org listing); Alpine is two
behind. The optimizing compiler is CPS-based and costs time and memory on
large inputs; the baseline compiler at `-O1`/`-O0` is "around ten times
faster" to compile (https://wingolog.org/archives/2020/06/03/a-baseline-compiler-for-guile).
No standalone executable: compiled `.go` files run under `guile`.

**CHICKEN** (https://call-cc.org/). R7RS since 6.0.0 (2026-08-10), through
C with Cheney on the M.T.A. The 6.0 manual still says "Native threads that
map directly to the threads provided by the operating system are not
supported ... execution of Scheme code on multiple processor cores is not
available" (https://wiki.call-cc.org/eggref/6/srfi-18). That is phase 177's
parallel `:%s` lost, and every function of the core goes through gcc as one
C file of CPS fragments -- the Java's huge-method trouble in another form.

**Gambit** (https://gambitscheme.org/, Marc Feeley). Fast (3rd), active
(commits this week), but the Alpine package is in *testing* and three
releases behind; its SMP scheduler is `--enable-smp`, "default is NO", and
Alpine's build does not enable it; and its own configure warns that the
`--enable-single-host` build "requires lots of RAM memory (>= 1 GB)"
(configure.ac) -- the same per-module single C function the core would be.

**Gauche** (https://practical-scheme.net/gauche/, Shiro Kawai). R7RS, a VM,
real pthreads; active, but no release since 0.9.15 (2024-04-24), not packaged,
and at the bottom of the benchmark list.

**Chibi** (https://github.com/ashinn/chibi-scheme, Alex Shinn). The R7RS
reference-minded interpreter: small and correct, too slow for the heavy case
by its class (not in the top-nine list at all), threads inside one VM only.

**Loko** (https://scheme.fail/, Göran Weinholt). R6RS and R7RS, native code,
second in the benchmarks, 0.13.0 on 2026-06-27; "Loko builds statically
linked binaries" without libc. But its concurrency is fibers, not processors,
it is not packaged, and one person maintains it.

**Cyclone** (https://github.com/justinethier/cyclone, Justin Ethier). R7RS
through C, with native threads and an on-the-fly collector. Its last release
is 0.36.0 (2024-02-14), its last commit 2026-03-13: slowing. Through C again.

**MIT/GNU Scheme** (https://www.gnu.org/software/mit-scheme/). Native code on
x86-64, a fix for building on musl in its release notes, but 12.1
(2023-01-07) is the last release, it is not packaged, and its SMP support is
"preliminary ... on an alternate branch" (release notes).

Not candidates: **TinyScheme** (an R5RS-subset interpreter); **Kawa** (JVM;
would reuse `braaam/rt` as Clojure does, but last released 3.1.1 in
2020-01 and not packaged); **Gerbil**, **Bigloo**, **s7**, **STklos** (not
packaged).

## 3. The recommendation

**Chez Scheme**, from Alpine's `chez-scheme` package -- R6RS, native code,
real threads -- with **Guile** the fallback.

Tied to the constraints:

- *Standard and idiomatic*: R6RS is the one Scheme standard with typed
  bytevector access (`bytevector-u32-native-ref`, `-s16-`, `-u64-`...) --
  R7RS-small has `bytevector-u8-ref` only -- and that is the vocabulary a C
  core's memory is written in. The generated core can be portable R6RS, with
  what is Chez's own (the raw-memory primitives, threads, the FFI) behind a
  small runtime library.
- *Actively maintained*: two releases a year, the Racket team's compiler.
- *Available here*: in community, threaded, 4.7 MiB, with its kernel
  library for an executable of our own.
- *Fast*: first in the benchmarks; native code, fixnums unboxed, typed
  memory access open-coded; the heavy case's limit (25 times the C) is far
  from what a native-code Scheme should need.
- *Parallel `:%s`*: pthreads without a lock, so `match_lines`' chunks run on
  every core as in the Go, Java, Clojure and Haskell editors.
- *Large code*: the allocator's spilling for huge procedures (§4.10 above),
  and procedure lifting that makes join points free -- but this is the risk to
  measure first (§6).

**Why not Racket**: the same compiler with the compile limit in front of it,
the minimal package without `r6rs` or `raco exe`, and a dialect of its own;
anything that blocks Chez's compiler blocks Racket too.

**Why Guile as the fallback**: an *independent* compiler (so a blocker in
Chez's is not shared), in Alpine's main repository, R6RS bytevectors and
libraries, threads without a lock, an FFI needing no C compiler. Its costs:
slower (9th), an older package (3.0.9), compile time of the CPS optimizer on
a 90,000-line core, and no executable of its own (start-up must be measured).
If Chez's blocker is the heavy case's speed rather than its compiler, Guile
does not help, and Gambit with SMP enabled (a build of our own) would be the
next to measure.

## 4. The approach

caprice's (`doc/HASKELL.md`), printed in Scheme: the Haskell backend is the
closest precedent in everything but syntax, and Scheme makes it simpler (no
`IO`, no types to infer, no pure/effect split).

### Memory: (a), C's memory raw

**(a) caprice's way, chosen**: every C object in raw memory at the C's
layout (the sizes and offsets `crefactor/cc` gives), a pointer an integer, a
load or store a typed access at an address; the file-scope objects one
segment, its initial image of constant bytes copied in; a local whose address
is taken in a frame of its own; string literals in the image; a function
pointer an index into a vector of procedures (caprice's `fnTable`). It needs
**no pointer-class analysis**, no `BytePtr`, no union handling -- the Go,
Java and Clojure backends' hardest parts -- and it is what Chez makes fast:
an address is a fixnum, a load is one machine instruction at
`optimize-level 3`, and the hot paths allocate nothing on the Scheme heap,
so the garbage collector and its thread rendezvous are out of the inner
loops.

Two runtimes for the same generated calls (`(ld-u32 p)`, `(st-u8! p v)`,
names to fix in milestone 1), chosen by measurement:

- **a1, one bytevector**, a pointer an offset: portable R6RS, bounds-checked
  at `optimize-level 2` (a debugging mode the other editors lack), and it can
  grow by copying, since offsets stay valid. R6RS requires a `native`
  accessor's index aligned to the size, so a possibly unaligned access (the
  C's byte-wise copies) goes through `bytevector-u32-ref` with
  `(native-endianness)`.
- **a2, foreign memory**: the arena `mmap`ed or `foreign-alloc`ed, a
  pointer a real address, `foreign-ref`/`foreign-set!` (Chez only;
  unaligned access is the machine's). A 1 GiB arena costs nothing until
  touched, as the C host's does; a 1 GiB bytevector may be zero-filled on
  every start, which the suite would pay 320 times.

**(b), objects per C object (the Java's and Clojure's way), rejected**: it
reuses the Java backend's analyses, but the Clojure editor's profile shows
what it costs (`doc/CLOJURE-PROFILE.md`), Scheme has no `BytePtr` to reuse
(Kawa aside), and in Chez a record per struct and a vector per array would
put every C object on the collected heap, with the parallel `:%s` allocating
in every thread.

**Integers.** Chez's fixnums are 61 bits on 64-bit machines (to confirm:
`(fixnum-width)`, §7). C's 8-, 16- and 32-bit arithmetic is fixnum arithmetic
then a wrap (`fx*/wraparound` keeps the low bits right); 64-bit values
(`long`, `linenr_T`, `varnumber_T`) are exact integers: fixnums for every
value the editor actually holds, bignums only past 2^60, wrapped to 64 bits
where the C type says. A pointer is always a fixnum.

### Control flow: blocks as procedures, jumps as tail calls

The lowered form (`crefactor/togo/lower.go`: basic blocks, three-address
steps) printed in **caprice's shape** (`hsshape.go`): a block one jump reaches
is written where the jump is; a join (several predecessors) or a loop head (a
jump back) is a local procedure of the variables live at its start, bound
with `letrec`/internal `define` around the body, or a named `let` where it is
a loop; every jump a tail call. Chez's lifting makes these procedures free
(no closure: known call sites, their free variables become arguments), and
Scheme's proper tail calls make every jump a jump.

- **The 49 gotos** and the **10 fall-throughs** are jumps like any other: the
  target block, or the next `case` arm, is a join, called in tail position.
  No state machine (the Clojure's `loop`/`case` on a block number is not
  needed: Scheme has what `recur` lacks), no `call/cc`.
- **Locals**: each value a name of its own (`hsnames.go`), no `set!`, so the
  compiler sees only immutable variables -- the condition its lifting and
  type recovery want.
- **A `switch`** is `case` on fixnum constants; whether Chez compiles a dense
  one to a jump table is to be measured, else a `vector` of the arms' joins.
- **Out-parameters and struct results** (phase 181): multiple values,
  `(values r x1)` and `let-values`, which Chez returns without allocating;
  `hsout.go` and `hsstruct.go`'s decisions reused.
- **`host_exit`**: a condition raised and caught by `run`, as caprice's
  exception; Chez's one-shot continuations are cheap, but nothing needs
  `call/cc`.

### Modules

R6RS libraries cannot import each other in a cycle, and the core's largest
cycle of calls is a third of it (caprice's `Part2`). So the core is **one
library** (`(whim editor)`), its parts `include`d as files, or the top-level
program compiled whole. The host and the core call each other (the printf's
error messages, the SIGHUP's death), which caprice solved with an `hs-boot`:
here the core receives the host as a record of procedures, so neither library
imports the other.

### The host

As caprice's: a **Host record** of procedures -- `editor/host.go`'s interface
on addresses and lengths -- carried by the editor, and the 17 functions glue
to it, so a process runs any number of editors, each on its thread. The
terminal host by hand: raw mode with `tcgetattr`/`tcsetattr` on a
`define-ftype termios`, the window's size with `ioctl(TIOCGWINSZ)` (a variadic
foreign call: Chez's `__varargs`), the wait a `poll(2)` on the keys and a
pipe of its own, all `foreign-procedure`s into musl's libc, the call made
`__collect_safe` so that other threads may collect meanwhile. Signals:
`register-signal-handler` runs the handler "at some procedure call boundary"
on the main thread (CSUG `system.html`), so a `poll` in progress returns with
`EINTR` and the handler then runs; whether that is enough, or a few lines of
C (a flag and a byte down the pipe, as the C host does) are needed, is a
milestone-3 measurement. `vim_snprintf` ported from `editor/format.go`, as
`Printf.hs` was (747 lines of Haskell) -- or, with model a2, translated from
the C host by the same backend, as `doc/RUST.md` proposes.

### The parallel `:%s`

`match_lines` gets a Scheme runtime body (`RuntimeBody`,
`internal/whim/gen.go`), as caprice's: the chunks on `fork-thread`s, joined
with a mutex and a condition; each on a regex engine the core's `alloc_clear`
makes (`{{sizeof regengine_T}}`); the arena's bump pointer advanced under a
lock or with `ftype-locked-incr!` on a foreign word (CSUG `threads.html`).

### Packaging

`bin/<name>` an executable of our own: `main.o` and `libkernel.a` from the
package, linked with the boot files (`petite.boot`, `scheme.boot`) and the
compiled core embedded and registered with `Sregister_boot_file_fd_segment`,
in vfasl form for start-up. A script running `chez --program editor.so` is the
first, simpler step. The suite starts an editor for every case; start-up is
measured in milestone 1.

## 5. Milestones

Each verified before the next, as the others' were.

1. **The runtime and a slice on foreign C.** The memory primitives in both
   runtimes (a1, a2), the integer conversions, and the printer for the slice
   the Haskell's first milestone covered: `crefactor/togo`'s `TestHs*` C
   programs translated, run under `chez --program`, required to print what
   gcc's build prints, with controls that undo one C rule at a time (unsigned
   division, a char's widening, an unsigned shift, a struct's copy) as
   `TestHsControl` does. Measured: a1 against a2, start-up, and compile time
   and peak memory on a synthetic core of the output's shape. A coverage
   report on `editor.c`.
2. **Every function of the core**, refusing none; the library compiling at
   `optimize-level 3` with no warning, its time and peak printed beside every
   build, as caprice's GHC is.
3. **The host and the suite**: the Host record, the terminal host, the
   printf, the launcher; `whim test --scheme` and `--wide --scheme` answering
   the 80 and 240 cases as the C does, with the Scheme editor's own control
   (`" INSERT"` changed in the generated core); the heavy case under 25 times
   the C; the parallel `:%s` measured at 500,000 lines beside
   `doc/PARALLEL-SUBSTITUTE.md`'s table.
4. **Kept current**: the generated library tracked, written by `whim gen`,
   and refused by `whim-editor-check` when stale.

## 6. Risks, named in advance

- **Compile time and memory of one large library.** Ninety thousand lines in
  one compilation unit; Chez's register allocator was quadratic before 9.5.1's
  spilling, and the biggest functions lower to hundreds of blocks with many
  live variables. Racket's compile limit exists because of this. Measured
  first (§7); if it is bad, `optimize-level` and `enable-type-recovery` are
  the first levers, splitting the core into separately compiled top-level
  files (cross-file calls through top-level variables) the second.
- **64-bit integers.** Fixnums end at 2^60; a `long` beyond that is a bignum,
  slower, and C's wrap at 2^64 must be explicit. Correctness is the
  controls' to show; the cost should be nil on the editor's values.
- **Unsafe mode is C's safety.** At `optimize-level 3` a wrong address
  corrupts memory as the C would; a1 at level 2 is the debugging build.
- **Signals.** Delivered to the main thread at a call boundary, not in the
  handler's context; a poll on another thread, or a long foreign call, delays
  them. Possibly a small C helper.
- **musl.** Alpine builds Chez and runs `make test-some-fast` against the
  upstream summary (its APKBUILD's `check`), so the base is tested, but
  `load-shared-object` naming musl's libc, and the FFI's varargs, are ours
  to check.
- **Alpine lags upstream** (10.3.0 against 10.4.1; Guile 3.0.9 against
  3.0.11). Nothing found needs the newer, but the package's version is the
  one the editor is held to.
- **Start-up.** Loading two boot files on every case of the suite; vfasl
  and an embedded image are the remedy if it shows.
- **Readability.** C in Scheme's syntax, as caprice is C in Haskell's: the
  price of a faithful translation; an idioms survey would follow, as for the
  others.

## 7. What would have to be measured first

Before a decision, with these installs (none was made for this survey):

1. **`apk add chez-scheme`** (10.3.0-r2, 4.7 MiB):
   - `(fixnum-width)`, `(threaded?)`, `(machine-type)` -- expected 61, `#t`,
     `ta6le`;
   - start-up: `chez --program` of an empty program, and of an executable
     linked from `main.o`/`libkernel.a` with embedded vfasl boot files;
   - **compile time and peak memory** of a synthetic library shaped as the
     output -- 2,000 procedures of typed loads, stores and wrapping
     arithmetic, about 26,000 lines, the shape `doc/RUST.md` measured -- at
     `optimize-level` 2 and 3; and of one procedure shaped as
     `ex_substitute`'s lowered form (hundreds of joins, a hundred live
     variables);
   - the speed of a1 against a2: a loop of `bytevector-u32-native-ref`,
     `bytevector-u32-ref` with `(native-endianness)`, and `foreign-ref
     'unsigned-32`, at level 3; and whether a dense fixnum `case` is a jump
     table (`expand/optimize` and the timing);
   - `fork-thread` on 64 cores: a chunked loop over a shared foreign arena,
     its scaling;
   - the FFI on musl: `load-shared-object` of libc, `poll`, `tcgetattr`,
     `ioctl(TIOCGWINSZ)` by `__varargs`, and a signal arriving during a
     `poll`.
2. **`apk add guile`** (3.0.9-r2, 50 MiB), only if (1) finds a blocker in
   Chez's compiler: the same synthetic library compiled with `guild compile`
   at `-O1` and `-O2`, its time and peak; the same loops' speed; start-up of
   `guile` with the compiled `.go` files.
3. **Racket is not recommended for measurement** unless both fail: it would
   need `apk add racket` (177 MiB) and then `raco pkg install r6rs-lib
   compiler-lib`, a package manager's network install.
