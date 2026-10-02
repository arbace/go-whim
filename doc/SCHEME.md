# The editor in Scheme: a survey

**Measured; the choice is Chez Scheme -- not scheduled** (2026-10-02): there
is no intention yet to translate the editor to Scheme. It is kept as what was
found and how it would be done, should that change. The measurements §7 asked
for were made on Alpine's Chez Scheme 10.3.0 the same day (§8), and none of
them blocks: should a Scheme editor be written, it is written for Chez, and
Guile was not installed or measured.

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

**No Scheme was installed** when this survey was written; `chez-scheme`
10.3.0-r2 was installed afterwards for §8's measurements, and nothing else.
Measured before it:

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

Nothing was installed for §1-§7; §8 is measured on `apk add chez-scheme`
alone.

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
real threads -- **confirmed by measurement** (§8): a 93,000-line library of
the core's shape compiles at `optimize-level 3` in 57 s and 1.5 GB (GHC takes
caprice's core in two and a half minutes); typed loads and stores at level 3
run faster than the suite's C at `-O0`; `fork-thread` scales as C's pthreads
do on this machine; the terminal's foreign calls work on musl with no C of our
own; and a linked executable carrying the whole core starts in 38 ms. **Guile**
stays the fallback on paper only: nothing measured calls for it.

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
- *Large code*: measured (§8.3): time grows as lines^1.2 for one library,
  linearly for top-level forms compiled each on its own; one procedure of
  3,700 lines, 300 local join procedures and about 140 live variables -- three
  and a half times `ex_substitute`'s lowered form (1,042 lines of Haskell, 57
  joins, at most 39 parameters) -- compiles in 2.1 s and 175 MB.

**Why not Racket**: the same compiler with the compile limit in front of it,
the minimal package without `r6rs` or `raco exe`, and a dialect of its own;
anything that blocks Chez's compiler blocks Racket too.

**Why Guile as the fallback** (not needed, §8): an *independent* compiler (so a blocker in
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
names to fix in milestone 1), chosen by measurement -- **a1**, §8.4: at level 3
it is the faster (u32: 172 ms against a2's 216 for the same 512 Mi accesses),
and at level 2 a2's `foreign-ref` is not open-coded (16 to 45 times a1), so a2
would have no debugging build:

- **a1, one bytevector**, a pointer an offset: portable R6RS, bounds-checked
  at `optimize-level 2` (a debugging mode the other editors lack), and it can
  grow by copying, since offsets stay valid. R6RS requires a `native`
  accessor's index aligned to the size, so a possibly unaligned access (the
  C's byte-wise copies) goes through `bytevector-u32-ref` with
  `(native-endianness)`.
- **a2, foreign memory**: the arena `mmap`ed or `foreign-alloc`ed, a
  pointer a real address, `foreign-ref`/`foreign-set!` (Chez only;
  unaligned access is the machine's). A 1 GiB arena costs nothing until
  touched, as the C host's does; a 1 GiB bytevector filled with zeros costs
  125 ms to 1.1 s and a gigabyte resident (§8.4), so a1 starts small and grows
  by copying. Unfilled, `make-bytevector` is 1-2 ms but its bytes are not
  promised to be zero, and the C's segment needs them zero.

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
- **A `switch`** is `case` on fixnum constants. Chez compiles a dense one to a
  chain of compares, not a jump table (§8.4: 16 arms, 16 `cmpi` in the
  assembly); a 64-arm `case` costs 700 ms per 50 M dispatches against 610 for
  a `vector` of the arms' procedures, so the vector is for the few hot wide
  switches only, if the profile asks.
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
`__collect_safe` so that other threads may collect meanwhile -- and a
bytevector handed to `read` or `write` made with `make-immobile-bytevector`
(or locked), since a collection may move an ordinary one during such a call.
Signals, measured (§8.6): `register-signal-handler`'s handler runs on the
main thread at a call boundary; a `poll` *on the main thread* returns with
`EINTR` and the handler runs within a millisecond, at the next call
boundary, but a `poll` on any other
thread is not interrupted, and the handler waits until the main thread runs
Scheme again (3.9 s later in the test). So the terminal host takes its signals
the Linux way instead, **with no C of our own**: SIGWINCH, SIGINT and the rest
blocked with `pthread_sigmask` before any thread starts, a `signalfd` beside
the keys in the one `poll`, read as a `struct signalfd_siginfo` -- measured
working on a worker thread, woken 500 ms in as the signal was sent. A host on
the main thread could use the handler instead; signalfd does not care which
thread polls, which is what several editors in one process need. `vim_snprintf` ported from `editor/format.go`, as
`Printf.hs` was (747 lines of Haskell) -- or, with model a2, translated from
the C host by the same backend, as `doc/RUST.md` proposes.

### The parallel `:%s`

`match_lines` gets a Scheme runtime body (`RuntimeBody`,
`internal/whim/gen.go`), as caprice's: the chunks on `fork-thread`s, joined
with a mutex and a condition; each on a regex engine the core's `alloc_clear`
makes (`{{sizeof regengine_T}}`); the arena's bump pointer advanced under a
lock or with `ftype-locked-incr!` on a foreign word (CSUG `threads.html`).

### Packaging

`bin/<name>` an executable of our own: `libkernel.a` from the package and a
`main` of a dozen lines (§8.2), linked with `petite.boot` (not `scheme.boot`:
the editor needs no compiler at run time) and a boot file of the compiled core
made with `make-boot-file`, both converted with `vfasl-convert-file`, embedded
with `.incbin` and registered with `Sregister_boot_file_bytes`, the core
compiled with `generate-inspector-information` off. Measured: **38 ms** to
start with a 93,000-line core (90 with inspector information, 200 without
vfasl), against caprice's 69 and braaam's 265. Dynamically linked: the
package has no `liblz4.a`, so it cannot be `-static` as the C is. A script
running `petite --program editor.so` (178 ms with that core) is the first,
simpler step.

## 5. Milestones

Each verified before the next, as the others' were.

1. **The runtime and a slice on foreign C.** The memory primitives on a1 (a2
   is measured out, §8.4), the integer conversions, and the printer for the slice
   the Haskell's first milestone covered: `crefactor/togo`'s `TestHs*` C
   programs translated, run under `chez --program`, required to print what
   gcc's build prints, with controls that undo one C rule at a time (unsigned
   division, a char's widening, an unsigned shift, a struct's copy) as
   `TestHsControl` does. A coverage report on `editor.c`. (What this
   milestone was to measure first -- a1 against a2, start-up, compile time
   and peak memory on a synthetic core -- is measured, §8.)
2. **Every function of the core**, refusing none; the library compiling at
   `optimize-level 3` with no warning, its time and peak printed beside every
   build, as caprice's GHC is -- expected about a minute and 1.5 GB (§8.3);
   over three minutes is the signal to compile the core as top-level forms
   instead (13 s at 93,000 lines, calls between them about twice as dear).
3. **The host and the suite**: the Host record, the terminal host (its
   signals through `signalfd`, §4), the printf, the launcher -- the linked
   executable with vfasl boot files; `whim test --scheme` and `--wide --scheme` answering
   the 80 and 240 cases as the C does, with the Scheme editor's own control
   (`" INSERT"` changed in the generated core); the heavy case under 25 times
   the C; the parallel `:%s` measured at 500,000 lines beside
   `doc/PARALLEL-SUBSTITUTE.md`'s table.
4. **Kept current**: the generated library tracked, written by `whim gen`,
   and refused by `whim-editor-check` when stale.

## 6. Risks, named in advance

- **Compile time and memory of one large library** -- *measured, not a
  blocker* (§8.3): 57 s and 1.5 GB at 93,000 lines and level 3, growing as
  lines^1.2 (about 100 s and 2.2 GB at 150,000 by that fit); level 2 is 90 s
  and 2.3 GB. If the real core is worse than its synthetic stand-in, the
  levers measured are `compile-program` (half the library's time) and
  top-level forms compiled one at a time (linear: 13 s and 330 MB at 93,000
  lines), whose calls through top-level variables cost about twice a call
  inside one library. Inspector information off changes the compile time
  nothing but halves the start-up.
- **64-bit integers.** Fixnums end at 2^60 (`(fixnum-width)` is 61); a
  `long` beyond that is a bignum, and C's wrap at 2^64 must be explicit.
  Measured (§8.7): `+` with a `fixnum?` test costs 1.3 ns an addition on the
  editor's values, as C's at `-O0` (1.1 ns); past 2^60 it is about 100 ns, 80
  times as dear -- fine for the rare hash or `varnumber_T` at its limits,
  ruinous if a hot loop's values were ever that big. Correctness is the
  controls' to show.
- **Unsafe mode is C's safety.** At `optimize-level 3` a wrong address
  corrupts memory as the C would; a1 at level 2 is the debugging build.
- **Signals** -- *measured, settled* (§8.6): the handler runs on the main
  thread only, and a `poll` elsewhere is not woken; `signalfd` in the poll
  set works from any thread with no C helper.
- **musl** -- *measured* (§8.6): `load-shared-object` takes
  `libc.musl-x86_64.so.1` (or `libc.so`), `define-ftype` lays out musl's
  `termios` (60 bytes, `c_cc` at 17) as gcc does, and `ioctl` through
  `(__varargs_after 2)` returns the pseudo-terminal's size.
- **Alpine lags upstream** (10.3.0 against 10.4.1; Guile 3.0.9 against
  3.0.11). Nothing found needs the newer, but the package's version is the
  one the editor is held to.
- **Start-up** -- *measured* (§8.2): 66 ms for `chez --program`, 27 ms for a
  linked executable with vfasl boot files, 38 ms carrying a 93,000-line
  core: between the Go (8) and the Haskell (69).
- **Readability.** C in Scheme's syntax, as caprice is C in Haskell's: the
  price of a faithful translation; an idioms survey would follow, as for the
  others.

## 7. What would have to be measured first

Before a decision, with these installs (none was made for this survey). Item
1 is measured, §8; items 2 and 3 were not needed:

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

## 8. Measured, 2026-10-02

On Chez Scheme 10.3.0 from Alpine's `chez-scheme` 10.3.0-r2 (`/usr/bin/chez`,
`petite`, `chez-script`; boot files, `libkernel.a`, `main.o` and `scheme.h` in
`/usr/lib/csv10.3.0/ta6le/`), nothing else installed. The machine: Alpine
x86_64, musl, 64 vCPUs (AMD EPYC 7763), 43 GiB. **Load caveat**: two other
agents were building in other worktrees meanwhile; the 1-minute load average
ran from 2 to 23 during these runs (23 at the first round of §8.3, 2-5 for
most of the rest). Every timing is therefore a median of three -- of three
`hyperfine` runs of 20 for start-up, of three runs for the rest -- and the
runs are printed where they scattered. The scratch files were in
`.tmp/scheme-measure/`, run with `TMPDIR=.tmp`; none is kept.

### 8.1 The implementation

```
$ echo '(printf "~a ~a ~a ~a\n" (scheme-version) (fixnum-width) (threaded?) (machine-type))' | chez -q
Chez Scheme Version 10.3.0 61 #t ta6le
```

As expected: 61-bit fixnums (`most-positive-fixnum` 2^60-1), threaded,
`ta6le`. `chez` is dynamically linked to musl, `libncursesw`, `libz` and
`liblz4`; `libkernel.a` needs the same three and libc, and no libffi.
`compile-compressed` is not bound in 10.3; `(compress-format)` is `lz4` and
`(fasl-compressed)` `#t`.

### 8.2 Start-up

A trivial program, `(import (rnrs)) (display "hi\n")`; `hyperfine -N -w 3 -r
20`, three times, the median of the three medians:

| how | ms |
|---|---:|
| `chez --program hello.ss` (compiled on the fly) | 71.2 |
| `chez --program hello.so` (`compile-program`) | 66.4 |
| `petite --program hello.so` | 42.4 |
| `chez-script hello.ss` | 67.7 |
| `main.o` + `libkernel.a` linked, `-b petite.boot --program hello.so` | 46.1 |
| own `main`, petite.boot + app boot **embedded** | 47.9 |
| own `main`, the same **vfasl**, embedded | **27.0** |
| own `main`, petite.boot + app boot **beside** the executable | 48.6 |
| own `main`, the same vfasl, beside | 26.7 |
| `/bin/true`, for the floor | 2.0 |

The linked executable. `main.o` is Chez's own `main` (the `chez` command
line); linking it with the kernel is one line:

```
K=/usr/lib/csv10.3.0/ta6le
gcc -o myscheme $K/main.o $K/libkernel.a /usr/lib/liblz4.so.1 -lz -lncursesw -lpthread -ldl -lm
```

(`liblz4.so.1` by path: the package `lz4-dev` with its `liblz4.so` and
`liblz4.a` is not installed, and was not, so no `-static`.) For an editor,
a `main` of our own, with the application a boot file whose last form sets
`scheme-start`:

```c
#include "scheme.h"
extern char petite_start[], petite_end[], app_start[], app_end[];  /* .incbin */
int main(int argc, const char *argv[]) {
  Sscheme_init(NULL);
  Sregister_boot_file_bytes("petite", petite_start, petite_end - petite_start);
  Sregister_boot_file_bytes("app", app_start, app_end - app_start);
  Sbuild_heap(NULL, NULL);
  int r = Sscheme_start(argc, argv);
  Sscheme_deinit();
  return r;
}
```

```
  .section .rodata
  .balign 16
  .globl petite_start, petite_end, app_start, app_end
petite_start: .incbin "petite-v.boot"
petite_end:
  .balign 16
app_start: .incbin "app-v.boot"
app_end:
  .section .note.GNU-stack,"",@progbits
```

```scheme
;; app.ss
(suppress-greeting #t)
(scheme-start (lambda args (display "hi\n") (flush-output-port (current-output-port)) (exit 0)))
;; mk.ss, run with chez -q --script mk.ss
(compile-file "app.ss" "app.so")
(make-boot-file "app.boot" '("petite") "app.so")
(vfasl-convert-file "/usr/lib/csv10.3.0/ta6le/petite.boot" "petite-v.boot" '())
(vfasl-convert-file "app.boot" "app-v.boot" '("petite"))
```

Beside the executable, the same `main` with `Sregister_boot_file("petite-v.boot")`
and `Sregister_boot_file("app-v.boot")`. Two traps on the way: without
`suppress-greeting` the banner is printed, and `vfasl-convert-file` with `#f`
for petite's base list makes a boot file that crashes (`invalid memory
reference`); `'()` is right.

**With a core of the editor's size** -- the 93,000-line library of §8.3,
`(synth core)`, in the app boot, the start-up procedure importing it:

| how | ms |
|---|---:|
| `petite --libdirs . --program run.so`, the library's `.so` beside | 178.0 |
| own `main`, boots embedded | 202.3 |
| own `main`, boots embedded, vfasl | 95.3 (vfasl app boot 17.9 MB) |
| own `main`, boots beside, vfasl | 90.5 |
| the same, the core compiled with `generate-inspector-information` and `generate-procedure-source-information` off, embedded, vfasl | **37.5** (app boot 1.1 MB, executable 4.7 MB) |
| the same, beside, vfasl | 37.4 |
| the same, embedded, not vfasl | 75.6 |

Beside the existing editors, on the suite's own terms (`:q!\r` on stdin from
a file, `WHIM_TIME` set; `hyperfine --input keys`, median of three medians):

| editor | ms |
|---|---:|
| C, `src/whim-vim.c` built `gcc -O0 -fno-stack-protector -static -no-pie -s` | 2.4 |
| Go, `bin/whim` | 7.9 |
| Haskell, `bin/caprice` | 68.9 |
| Java, `bin/braaam` | 265.5 |
| Clojure, `bin/vijure` | 327.8 |

So a Chez editor linked as above would start in about 40 ms -- between the Go
and the Haskell -- which is nothing to the suite's 320 runs.

### 8.3 Compile time and peak memory

A generator (110 lines of Python, scratch) writes an R6RS library `(synth
core)` -- or the same as a top-level program -- of N procedures and H huge
ones, on one bytevector `mem` of 1 MiB through macros of the generated core's
vocabulary:

```scheme
(define-syntax ld-u32 (syntax-rules () [(_ p) (bytevector-u32-native-ref mem p)]))
(define-syntax st-u8! (syntax-rules () [(_ p v) (bytevector-u8-set! mem p v)]))
(define-syntax w32 (syntax-rules () [(_ e) (fxand e #xffffffff)]))   ; and ld-u8, ld-u16, ld-s64, st-u16!, st-u32!, ptr
```

Each ordinary procedure is 13 lines, calling four others chosen at random
(so the call graph is one big cycle, as the core's is), in tail position and
not:

```scheme
(define (f17 p0 q0 d)
  (let ([p (ptr p0)] [q (ptr q0)])
    (if (fx<= d 0)
        (ld-u32 p)
        (let* ([a (ld-u32 p)] [b (ld-u16 (fx+ q 2))] [c (w32 (fx+ a (fx* b 3)))])
          (st-u32! (fx+ p 4) c)
          (st-u8! (fx+ q 5) (fxand c 255))
          (cond
            [(fx= (fxand c 7) 1) (f57 (fx+ p 8) q (fx- d 1))]
            [(fx< a b) (let loop ([k 0] [s c]) (if (fx< k 9) (loop (fx+ k 1) (fx+/wraparound s (ld-u8 (fx+ q k))))
                         (begin (st-u32! p (w32 s)) (f1647 p (fx+ q 4) (fx- d 1)))))]
            [else (case (fxand b 15) [(0) (f470 q p (fx- d 1))] [(1 2 3) (fx+ a 3)]
                    [(4 5) (fxsrl c 7)] [(6) (fx+ (f1301 p q (fx- d 1)) 1)] [else (fxxor a b)])])))))
```

The huge one is `ex_substitute`'s lowered form exaggerated: 1,506 lines, 60
`let*`-bound values, then 120 local procedures in one `letrec*`, each of 20
parameters (and free in the 60 values: about 80 live), each a nested `if` /
`cond` / `case` ending in tail calls to the next one to three joins, or back
to one 2-7 earlier, with loads, stores and calls of ordinary procedures between.
The real one, in caprice's `Part4.hs`, is 1,042 lines, 57 joins, 4 to 39
parameters (median 28).

Compiled with

```scheme
;; chez -q --script compile.ss LEVEL IN OUT lib|prog
(let* ([a (cdr (command-line))]
       [lvl (string->number (car a))] [in (cadr a)] [out (caddr a)] [mode (cadddr a)])
  (parameterize ([optimize-level lvl] [compile-imported-libraries #t])
    (if (string=? mode "lib") (compile-library in out) (compile-program in out))))
```

under `/usr/bin/time -f "%e s %M KB"`, three rounds (load 23, 8, 5 at their
starts):

| source | lines | how | level | s (runs) | peak MB |
|---|---:|---|---:|---|---:|
| 200 procedures | 2,618 | library | 3 | 0.56 | 64 |
| 200 + 1 huge | 4,124 | library | 3 | 1.69 | 159 |
| 200 + 1 huge | 4,124 | library | 2 | 2.45 | 206 |
| 2,000 + 1 huge | 27,524 | library | 3 | **12.7** (14.05 12.68 12.59) | 573 |
| 2,000 + 1 huge | 27,524 | library | 2 | 20.1 (22.10 20.12 19.81) | 734 |
| 2,000 + 1 huge | 27,522 | program | 3 | 6.9 (6.89 6.82 7.08) | 460 |
| 2,000 + 1 huge | 27,522 | program | 2 | 10.3 (10.43 10.21 10.27) | 668 |
| 4,000 + 2 huge | 55,030 | library | 3 | **30.0** (28.86 30.00 30.53) | 898 |
| 4,000 + 2 huge | 55,028 | program | 3 | 15.1 (15.06 15.02 15.84) | 815 |
| 6,800 + 3 huge | 92,936 | library | 3 | **56.8** (55.20 56.81 56.87) | **1,491** |
| 6,800 + 3 huge | 92,934 | program | 3 | 30.6 (30.36 30.57 31.47) | 1,391 |
| 6,800 + 3 huge | 92,936 | library | 2 | 90.4 (two runs: 90.49 90.35) | 2,265-2,383 |
| 6,800 + 3 huge | 92,934 | program | 2 | 52.7 (one run) | 2,475 |
| 2,000 + 1 huge | 27,521 | top-level forms, `compile-file` | 3 | 3.8 (3.77 3.70 3.78) | 134 |
| 6,800 + 3 huge | 92,933 | top-level forms, `compile-file` | 3 | 13.0 (13.03 13.02 13.23) | 333 |

**The 90,000-line library was measured directly, not extrapolated: 57 s and
1.5 GB at level 3.** Fitted over the three sizes, the library's time grows as
lines^1.23 and its peak as lines^0.79 (the program's: ^1.22 and ^0.91; top-level
forms, each compiled on its own: ^1.02 and ^0.75). By those fits, 150,000 lines
would be about 100 s and 2.2 GB as one library, 55 s as a program, 21 s as
top-level forms -- an extrapolation, not measured. For scale, GHC takes
caprice's nine modules in two and a half minutes.

What the levers did:

- `compile-library` is about twice `compile-program` on the same body.
- `generate-inspector-information` and `generate-procedure-source-information`
  off, with `enable-cross-library-optimization` off: 12.3 s (12.60 12.02 12.28)
  and 579 MB for the 27,524-line library -- no change in compile time, but the
  compiled core is 726 KB instead of 1.69 MB and starts twice as fast (§8.2).
- Top-level forms (the program without its `import`, through `compile-file`,
  so that each `define` is compiled alone) are linear and four times faster
  at 93,000 lines; but every call between procedures then goes through a
  top-level variable: `fib 34` through two mutually recursive procedures, 24
  ms as a program, 45-48 as top-level forms.
- **A harsher huge procedure**: 3,706 lines, 100 values, 300 joins of 40
  parameters (about 140 live), in a program of 50 ordinary procedures: 2.1 s
  (2.17 2.14 2.02) and 175 MB, against 0.14 s and 46 MB without it. Nothing
  quadratic shows in one procedure of that size.

The generator's output was run to its end at level 3 (`(run 4)`) at 200
procedures with three huge ones, and with the 3,706-line one; the 93,000-line
library was loaded and imported by the executables of §8.2.

### 8.4 Raw memory

One 64 MiB arena, eight passes of `v = load u32 at i; store v+k at i; s += v`
over all of it (128 Mi loads and stores), then the same with bytes (512 Mi),
compiled with `compile-program` at level 3 and at level 2; the loops are
named `let`s on fixnums:

```scheme
(let loop ([i 0] [s s])
  (if (fx< i size)
      (let ([v (bytevector-u32-native-ref bv i)])
        (bytevector-u32-native-set! bv i (fxand (fx+ v k) #xffffffff))
        (loop (fx+ i 4) (fxand (fx+ s v) #xffffffff)))
      (pass (fx+ k 1) s)))
;; and the same with (bytevector-u32-ref bv i (native-endianness)), with 'little,
;; with (foreign-ref 'unsigned-32 addr i) on (foreign-alloc size), and the u8 kin
```

and the same in C, `gcc -O0`, on `malloc`ed memory through `*(uint32_t *)(m + i)`.
Medians of three runs, ms:

| access | level 3 | level 2 |
|---|---:|---:|
| a1 `bytevector-u32-native-ref`/`-set!` | **172** (173 172 172) | 764-828 |
| a1 `bytevector-u32-ref` with `(native-endianness)` | 173 | 1,186-1,251 |
| a1 `bytevector-u32-ref` with `'little` | 172 | 1,230-1,252 |
| a2 `foreign-ref 'unsigned-32` | 216 (216 219 214) | 12,406-12,896 |
| a1 `bytevector-u8-ref`/`-set!` | 1,013 (1,010 1,013 1,032) | 2,576-2,590 |
| a2 `foreign-ref 'unsigned-8` | 1,026 (1,026 1,013 1,027) | 45,878 |
| C `-O0`, u32 | 242 (246 712 237 242 248) | |
| C `-O0`, u8 | 1,587 (1,220 3,220 1,587 1,235 1,593) | |

At level 3 every a1 accessor is the same open-coded load (a possibly
unaligned access through `bytevector-u32-ref` with `(native-endianness)`
costs nothing extra), and Chez is **faster than the suite's C reference at
`-O0`** (0.71 of it for u32, 0.64 for u8). a2 is 25 % slower on u32 and even
on u8. At level 2 a1 is 3-7 times slower, bounds-checked -- a debugging build
-- but a2's `foreign-ref` is not open-coded there: 16 to 45 times a1's. (The
C runs scattered the most: other builds were running.)

Making an arena: `(make-bytevector (* 1024 1024 1024) 0)` took 1,085, 125 and
374 ms in three runs, and the process peaked at 1.08 GB; unfilled, 1-2 ms;
`foreign-alloc` of 1 GiB, 0 ms. A 64 MiB filled bytevector: 10-68 ms.

A dense `case`: `(case x [(0) (x 1)] ... [(15) (x 16)] [else 0])` at level 3,
printed through `#%$assembly-output`, is 16 `cmpi`s and branches -- no jump
table. Timed, 50 M dispatches on `(fxand (fx* i 37) 63)`: a 64-arm `case`
700 ms (701 688 700), `((vector-ref arms x) s)` 610 ms (626 610 607).

### 8.5 `fork-thread` on 64 cores

A 512 MiB arena of bytes `(fxand (fx* i 7) 63)`; count the bytes equal to 10,
four passes, split into N chunks on N threads, each `fork-thread`ed and
`thread-join`ed, the totals added under a mutex; level 3. The same in C with
pthreads at `-O0`, for scale. Medians of three, ms (the speed-up against N=1):

| N | a1 bytevector | a2 foreign | C `-O0` pthreads |
|---:|---|---|---|
| 1 | 2,348 | 2,371 | 2,856 |
| 2 | 1,191 (2.0) | 1,212 (2.0) | 1,448 (2.0) |
| 4 | 634 (3.7) | 607 (3.9) | 738 (3.9) |
| 8 | 317 (7.4) | 308 (7.7) | 379 (7.5) |
| 16 | 169 (13.9) | 159 (14.9) | 191 (15.0) |
| 32 | 143 (16.4) | 142 (16.7) | 165 (17.3) |
| 64 | 93 (25.2) | 96 (24.7) | 98 (29.2) |

Chez scales as C does: linear to 8, then both flatten alike past 16 -- the
machine (64 vCPUs shared with two builds, and a scan of 2 GiB in 93 ms is 23
GB/s), not Chez. A first version, joined by a condition variable, failed with
*cannot collect when multiple threads are active* when the next `(collect)`
came before the threads had exited: join with `thread-join`, or collect only
on one thread.

### 8.6 The FFI on musl, and signals

A program (`term.ss`, 110 lines) on a pseudo-terminal of 30x100 that a Python
driver opened with `pty.fork()` (no `script` is installed):

```scheme
(load-shared-object "libc.musl-x86_64.so.1")   ; "libc.so" and "/lib/ld-musl-x86_64.so.1" work too
(define-ftype termios
  (struct [iflag unsigned-32] [oflag unsigned-32] [cflag unsigned-32] [lflag unsigned-32]
          [line unsigned-8] [cc (array 32 unsigned-8)]
          [ispeed unsigned-32] [ospeed unsigned-32]))
(define-ftype winsize (struct [row unsigned-16] [col unsigned-16] [xpixel unsigned-16] [ypixel unsigned-16]))
(define-ftype pollfd (struct [fd int] [events short] [revents short]))
(define tcgetattr (foreign-procedure "tcgetattr" (int (* termios)) int))
(define tcsetattr (foreign-procedure "tcsetattr" (int int (* termios)) int))
(define ioctl-ws (foreign-procedure (__varargs_after 2) "ioctl" (int unsigned-long (* winsize)) int))
(define poll (foreign-procedure "poll" ((* pollfd) unsigned-long int) int))
(define poll/safe (foreign-procedure __collect_safe "poll" ((* pollfd) unsigned-long int) int))
(define errno-loc (foreign-procedure "__errno_location" () uptr))
```

What it printed (`\r` removed):

```
sizeof termios 60, winsize 8, pollfd 8                  -- gcc on musl's headers: 60, c_cc at 17, 8, 8
tcgetattr 0, lflag #o105073, VMIN 1 VTIME 0
tcsetattr raw 0
read back: lflag #o105061, ICANON 0 ECHO 0
TIOCGWINSZ (30 100)
poll for a key: (1 0 300)                               -- result, errno, ms: the key the driver typed at 300 ms
read 1: 120
main/poll: poll -> (-1 4 501); handled right after return: (); after a loop: ((winch 501))
main/poll-collect-safe: poll -> (-1 4 500); handled right after return: (); after a loop: ((int 500))
thread/poll-collect-safe: poll -> (0 0 3004); handled: ()
thread case, after a loop on the main thread: ((winch 4396))
pthread_sigmask 0
signalfd 3
thread/signalfd: (poll ms revents[sfd] read signo) -> (1 500 1 128 28)
TIOCGWINSZ now (45 120)
```

So: musl's libc loads; the `termios` layout is gcc's; raw mode takes; the
variadic `ioctl` through `(__varargs_after 2)` reads the size, and the new size
after the driver's `TIOCSWINSZ`. Signals, with `register-signal-handler` for
SIGWINCH (28) and SIGINT (2), each sent by the driver 500 ms into a `poll` of
3 s:

- a `poll` **on the main thread**, plain or `__collect_safe`, returns at 500 ms
  with -1 and `EINTR` (4); the handler has not run when `poll` returns, and
  has by the next few procedure calls, stamped the same millisecond;
- a `poll` **on another thread** (the main thread in `condition-wait`) is not
  interrupted: it times out at 3 s, and the handler runs only when the main
  thread runs Scheme again, 3.9 s after the signal;
- **`signalfd`**: SIGWINCH blocked with `pthread_sigmask(SIG_BLOCK)` on the main
  thread before `fork-thread` (the new thread inherits the mask), `signalfd(-1,
  set, 0)` polled beside fd 0 on the other thread: woken at 500 ms, `revents`
  POLLIN, 128 bytes read, `ssi_signo` 28. All through `foreign-procedure`, no
  C compiled.

### 8.7 64-bit `long` on 61-bit fixnums

100 M iterations of `s = s + i*k`, level 3 and 2, medians of three, ms:

| how | level 3 | level 2 |
|---|---:|---:|
| `fx+/wraparound` (wraps at 2^60: wrong for a `long`, the floor) | 64 | 142 |
| `fx+`, masked to 48 bits (unchecked at 3, checked at 2) | 85 | 158 |
| generic `+`/`*`, `logand`ed to 48 bits | 127 | 158 |
| `(let ([r (+ s (* i k))]) (if (fixnum? r) r (wrap64 r)))`, values small | 126 | 163 |
| the same, values past 2^60 (k = 2^44) | 10,177 | 10,394 |
| bignums throughout, `wrap64` each step | 10,273 | 11,081 |
| C `long`, `gcc -O0` | 111 (111 111 105) | |

```scheme
(define (wrap64 x)  ; C's wrap of an exact integer to a signed 64-bit long
  (let ([y (logand x #xffffffffffffffff)]) (if (>= y #x8000000000000000) (- y #x10000000000000000) y)))
```

The generic path on fixnum values is twice `fx+/wraparound` and a little
slower than C at `-O0`: about 1.3 ns an addition. Past the fixnums every
operation allocates a bignum, about 100 ns: 80 times. So `long` as `+` with a
`fixnum?` test, and `wrap64` on the rare overflow, costs nothing on the
editor's values; only a loop that really holds values past 2^60 (a hash
mixed in 64 bits) would want `fx*/wraparound` on two halves instead.

### 8.8 What it settles

Nothing blocks. The one risk named first -- one library of 90,000 lines --
compiles in under a minute at level 3 in 1.5 GB, a 3,700-line procedure of
300 joins in 2 s; the fallback lever, top-level forms, is measured too. So
the recommendation stands, sharpened: Chez Scheme 10.3 from Alpine; memory
**a1**, one bytevector (faster than a2 at level 3, and the only one with a
debugging build at level 2); `case` for `switch`; `long` as `+` with a
`fixnum?` test; the parallel `:%s` on `fork-thread` joined with
`thread-join`; the terminal's signals through `signalfd`; and the editor an
executable of our own `main` with vfasl boot files and no inspector
information, starting in under 40 ms. Guile was not measured: the fallback's
measurement would have been §7's item 2, `guild compile` of the same
synthetic library at `-O1` and `-O2`, and nothing here called for it.
