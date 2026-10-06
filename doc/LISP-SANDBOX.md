# A Lisp in the sandbox: Joker measured, and a stack sketched

*2026-10-05.  An assessment, not a decision: what Joker is beside Clojure,
what it does in each role Clojure has here and inside the guest, and how
a functional, Clojure-ish language on Go's runtime could be the editor's
own language in the box.  Measured where it could be; the rest is
marked as a sketch.*

## The brief, as understood

- **The box.** The hypervisor guest is a bare register-and-RAM machine
  with no legacy constraints: the monitor (`vmm/`) and its hypercall ABI
  (`guest/abi`) are the kernel interface, kept small. Inside, a runtime
  that is self-sustaining: no compiler outside, no host filesystem.
- **The data.** The program and its data are one homoiconic graph -- the
  file system replaced by the store of immutable, content-addressed nodes
  that `crefactor/graph` already keeps (hashes, EDN, the snapshot store).
- **The language.** A Clojure-ish surface (prefix lists, `[]` and `{}`
  literals, EDN as the data format), Haskell-ish semantics as far as they
  go (purity, immutable values, laziness, a type system that can grow),
  and a REPL in place of vimscript: the editor is written in the language
  it is extended in.
- **The runtime.** Go's: compact, garbage-collected, concurrent, cross-
  compiling, already running here as a guest (TamaGo, `guest/tamago/`).
- **The interface.** Console I/O (UTF-8, EDN, terminal codes for now, an
  arbitrary canvas later) and simple networking, reaching APIs like
  Claude's, without touching the host.
- **The measure.** Simplicity and a small footprint first; abstraction and
  speed are a bonus. (The Rust toolchain's 15 GB for a parser branch is
  the counter-example.)

## Joker beside Clojure

Joker (candid82/joker, v1.10.0, built here into `.tmp/`) is a Clojure
dialect written in Go: about 28,000 lines of hand-written Go, its core
library 7,900 lines of Joker (compiled at build time into Go data, 363,000
generated lines), the standard-library wrappers 13,000; one 29 MB static
binary, started in 20 ms. Since recently it compiles everything to a
bytecode VM (`VM.md`) -- no tree-walking fallback.

| | Clojure (JVM) | Joker |
| --- | --- | --- |
| host | the JVM, Java interop | Go, no interop with arbitrary Go |
| footprint | JVM ~200 MB + clojure.jar 4 MB; start ~1 s | 29 MB binary; start 20 ms |
| evaluation | compiled to JVM bytecode, then the JIT | its own bytecode VM; no JIT |
| data | persistent vector, maps (array, hash, sorted), sets, lists, records | vector, array map, hash map, set, list -- no sorted, no records |
| types | deftype, defrecord, protocols, primitive arrays | none of these: maps and vectors only |
| reader | tagged literals, custom data readers | neither: no reader function for a tag |
| laziness | lazy seqs, chunked | lazy seqs, unchunked |
| concurrency | threads, refs, agents, atoms, futures | single-threaded under a global lock; core.async-style `go` |
| transients | yes | vectors, maps, sets |
| type hints | for interop and speed | none |

**What it found.** Joker's persistent vector fails a `conj` after a `pop`
that pulls a leaf out of its tree: `(conj (pop (vec (range 1057))) :x)` is
a Go panic (`core/vector.go`'s `pushTail`, index out of range) -- 1057 is
33 full leaves and one element in the tail. A vector used as a stack meets
it on any input past a thousand items. Not reported upstream yet.

## Joker in each role Clojure has here

**The views over the graph's EDN** (`crefactor/graph/view/clj`,
`view-clj`). Not runnable as they are: they use `deftype` nodes, Java
collections and object arrays, a `PushbackReader`, and the graph's six
tagged literals, none of which Joker has. Measured on the core of it -- the
EDN of whim-vim.c's graph (9.7 MB, 237,423 nodes) made tag-free by a text
transform, read, indexed (each node's parent and the uses of each), and
the callers of `ml_find_line` answered; the same 9 calls in 7 functions as
the Go and the JVM give:

| | read | index | the view | wall | peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| Clojure, JVM (`view-clj --time`) | 1.17 s | 0.21 s | 10 ms | 2.78 s | 650 MB |
| Joker | 0.90 s | 3.71 s | 20 ms | 4.66 s | 500 MB |
| Go (`whim view`) | 66 ms (the cache) | 25-43 ms | < 1 ms | 0.07 s | |

Joker reads faster than the JVM; its index is 17 times slower, every
table a persistent map where the JVM fills arrays. A port of the views is
feasible -- maps for the deftypes, tags read by a fork's data readers or
written as plain data -- at that cost. Built since: *The views in Joker,
in the box*, below, the index 1.36 s with vectors by id.

**The editor in Clojure** (`vijure/`, the generated `editor.clj`). Not
feasible on Joker: the generated core runs on C's memory as bytes, through
braaam's Java runtime (`BytePtr` and its kin), and Joker has no byte or
other mutable arrays and no interop to borrow them. Translating C's memory
model to persistent vectors would be the wrong layer: this role is the C
baseline carried into a Lisp, not the Lisp the box wants.

**The REPL** (`view-clj --repl`). Joker's REPL starts in 20 ms where the
JVM's takes a second -- and it runs **inside the guest**, below.

## Inside the guest: Joker and yaegi on TamaGo

Both build unmodified as TamaGo guests and run on whim's monitor on KVM
(scratch modules in `.tmp/jokerguest` and `.tmp/yaegiguest`; the code is
in the appendix):

| guest | image | start | `fib 27` | |
| --- | ---: | ---: | ---: | --- |
| the C core (`whim-guest.elf`) | 0.9 MB | | | the baseline |
| the Rust core (`whim-guest-rs.elf`) | 1.3 MB | | | |
| the Go editor (Go guest, arm64) | 5.4 MB | | | |
| **Joker**, core namespaces | **17.7 MB** | 0.06 s | 0.44 s (host: 0.26) | REPL on the console |
| **yaegi** (a Go interpreter) | 27.3 MB | | 0.57 s | |
| compiled Go, for scale | | | ~0.003 s | |

The Joker guest is a REPL on the monitor's console -- forms read through
the same wait-and-read hypercall the Go guest uses, each evaluated and
printed, an error reported with its stack and the REPL going on:

```
joker in the box; a form a line
user=> #'user/xs
user=> 285
user=> {:a [1 2 3], :b {\a 5, \b 2, \r 2, \c 1, \d 1}}
user=> #'user/f
user=> error: <joker.core>:740:34: Eval error: Division by zero
...
user=> "still here 416"
```

So a Clojure dialect, its 416 core functions, with a REPL, already lives
in the box on the Go runtime and the existing ABI: no new hypercall, 18 MB.
Interpreted code is about a hundred times slower than compiled Go for the
same function; the guest runs Joker at ~60% of the host's speed (one vCPU,
where the host's collector has other cores).

gore runs `go` itself behind its REPL, so it needs the toolchain and is out
for the box. replyme is a framework for command REPLs in Go applications --
the interface layer, not a language. yaegi interprets Go, which makes it
the way to run Go code the box has not compiled (the editor's own Go, for
instance), not the language of the box.

## A stack, sketched

From the machine up, each layer what exists here and what it would take.

1. **The machine.** The monitor and its ABI (`vmm/`, `guest/abi`): 17 host
   calls, SMP calls, a doorbell, a watchdog, seccomp; KVM and
   Hypervisor.framework, amd64 and arm64. *To add:* a datagram call (send
   and receive a packet), and later a canvas (a framebuffer page and its
   damage). Nothing else is needed from the host.
2. **The runtime.** Go's, on TamaGo: GC, goroutines on several vCPUs (the
   Go guest's SMP), the standard library where it needs no OS. Proven here
   for the Go editor, Joker and yaegi. A GC is the price of immutable,
   shared data, and Go's is a good one; regions or reference counting
   could serve a strict subset later, but not the persistent structures.
3. **The store.** The graph as immutable nodes keyed by content hash
   (`crefactor/graph`: `hash.go`, the store, EDN): what replaces the file
   system -- sources, data and history (the journal's versions) as one
   content-addressed graph, which syncs between boxes by exchanging the
   hashes the other lacks.
4. **The language.** A Clojure-ish surface on EDN, compiled by stages:
   - *now:* Joker's bytecode VM, in the box (above);
   - *next:* a compiler from the graph's own forms to Go closures (two to
     five times an interpreter's speed, no machine code);
   - *a JIT:* the guest owns its page tables, so the monitor can give it an
     executable arena, and the lowered form `crefactor/togo` already makes
     for the Clojure, Haskell and Scheme backends (a function as basic
     blocks) is what a copy-and-patch code generator takes. The hard part
     is Go's GC: generated code must keep no Go pointers in registers
     across a collection, so it either runs as leaf code over handles or
     declares stack maps as the Go compiler does;
   - *or a compiler in the box:* `cmd/compile` and `cmd/link` are Go
     programs, so the box could build its next image itself, from source in
     the store, given an in-memory file system, and the monitor start it
     beside the old one. A bootstrap rather than a JIT.

   Semantics: pure functions and immutable values by default, effects as
   capabilities handed in (the host's calls are values the editor is
   given, as `editor.Host` is now), laziness for sequences and the graph's
   views. Types: start dynamic, as Clojure; infer a Hindley-Milner core
   where code allows it, as Coalton does for Common Lisp and Hackett for
   Racket -- a type system that can grow without changing the surface.
5. **The editor.** The views, buffers, session and journal built in Go
   (`crefactor/graph/view`) are the model: a vim-like modal editor over
   views of the graph, its commands functions of the language, the REPL
   its command line. In time ported from Go to the language, as it
   stabilises.
6. **The interface.** Console: UTF-8, EDN messages, terminal codes. The
   network: datagrams through the monitor, with a TCP/IP stack in the guest
   (gVisor's netstack runs on TamaGo) and TLS in the guest, so the host
   forwards packets under a policy and sees nothing; HTTPS to the Claude
   API from inside the box. Between boxes, QUIC (UDP, encrypted,
   multiplexed) or a small datagram protocol over the content-addressed
   store -- what one box lacks of another's graph, by hash.

**The internal representation** asked for is, on this evidence, the
graph already here: typed, resolved, content-addressed, printed as C,
C-lisp, EDN and Lisp, and translated to Go, Java, Clojure, Haskell, Rust,
Scheme, OCaml and C++ from one form. What the box adds is that it becomes
the program's only form -- immutable, with views as the way in for a
person and for an agent alike.

## The Joker guest, built (2026-10-05)

`go tool whim guest --joker` builds `bin/whim-guest-joker`: Joker as a
REPL on the console, with TamaGo, on the same monitor (`guest/joker`, a
module of its own as `guest/tamago` is; 23 MB with the standard namespaces
that need no system). Joker is a fork in `guest/joker/joker`
(`joker/README.md`): upstream's core and those namespaces, 1.2 MB of
source, nothing outside Go's standard library; its core library compiled
to Go data by `go generate` at each build (3 s), not tracked. The REPL
keeps `*1`, `*2`, `*3` and `*e`, as Joker's own does.

The fork's changes, each marked `go-whim`:

- **The vector fault fixed.** `popTail` stored the nil slice of a child it
  emptied in an `interface{}` slot, which is then not nil: `Pop` kept a
  level it should have dropped, and the next `conj` read an empty node.
  It stores a real nil now; `(conj (pop (vec (range 1057))) :x)` is a
  vector of 1,057 in the box.
- **Tagged literals.** The reader consults `*data-readers*` (bound with
  `binding`, as Clojure's), then `default-data-readers`, then
  `*default-data-reader-fn*` with the tag and the form; a reader is a
  function or a var. So the graph's EDN reads as it is:

```
user=> (binding [*data-readers* {'g/n (fn [[id form & edges]] {:id id :form form :edges (vec edges)})
                                'g/r (fn [id] [:r id]) 'g/t (fn [id] [:t id])}]
         (read-string "#g/n [6 (struct buf) #g/r 2 #g/t 9]"))
{:id 6, :form (struct buf), :edges [[:r 2] [:t 9]]}
user=> (binding [*default-data-reader-fn* (fn [tag v] [tag v])] (read-string "#c/num \"0x10\""))
[c/num "0x10"]
```

## The views in Joker, in the box (2026-10-05)

The views of `crefactor/graph/view/clj` -- `gview.graph`, `gview.view`,
`gview.main` -- are ported to Joker in `guest/joker/gview/` (`graph.joke`,
`view.joke`, `main.joke`, form for form beside the Clojure, embedded by
the Go package there so that no file system is needed), and read the
graph's EDN as it is, its six tags given reader functions through the
fork's `*data-readers*`. They run in two places:

- **On the host**, `guest/joker/cmd/jokerhost`: the fork's core and the
  guest's standard namespaces built by the ordinary `go` (no TamaGo, no
  board; `guest.JokerHost` builds it after `go generate`). `jokerhost view
  ARGS FILE.edn` is `view-clj`: a view, `--batch CASES OUTDIR FILE.edn`,
  `--serve FILE.edn`; `jokerhost [--gview] [--store DIR] [-e EXPR |
  FILE]...` evaluates, or is a REPL on stdin, `box` the store in DIR.
- **In the box**, `go tool whim guest --joker`: the namespaces loaded at
  start, and a graph's EDN read from the store (*The store*, below; it was
  embedded in the image until the store was built):

```
user=> (def st (gview.main/open-string (box/load "graph")))
user=> (select-keys st [:read-ms :index-ms :nodes])
{:read-ms 2517, :index-ms 1715, :nodes 239125}
user=> (gview.main/show st "callers ml_find_line")
;; 9 calls of ml_find_line in 7 functions
(callers ml_find_line
  (in ml_get_buf
...
```

**How the port differs.** A node is a vector `[id text kids refs type
key]`; the index's object arrays are vectors by id, filled as transients;
`ArrayList`, `HashSet` and `LinkedHashMap` are atoms, sets and an ordered
grouping; the printer's `StringBuilder` a transient vector of strings.
Where the Clojure holds nodes in a set or map by identity, Joker would
compare vectors by value, so the port holds a node's KEY: its id, or a
negative number of its own for a list with no id -- such a list can be a
parent on a use's path and a context's form, and keyed by its id 0 every
one would be the same. `identical?` is kept where the Clojure has it
(Joker compares pointers). Two departures from the Clojure, for speed: the
index is sized by the EDN's `:ids` where the Clojure walks the graph for
the greatest id, and each walk is a `reduce`, the cheapest loop Joker has.

**Held to the Go.** `guest/jokerviews_test.go`, as `clj_test.go` holds
the Clojure: every case's text the same bytes as `whim view`, the same
error where there is one. On the sample, 48 cases (the golden views, each
flag and error, and for every name `uses`, `def` and `callers`, `type`):
48 of 48 on the host and 48 of 48 in the box. On whim-vim.c's graph,
`clj_serve_test.go`'s 24 fixed cases and every 25th generated one, 278 of
278 on the host (25 s); with `JOKER_VIEWS_ALL=1` all 6,370 cases, 6,370 of
6,370 (2 min 9 s); in the box, the 24 fixed cases, 24 of 24
(`TestJokerViewsGuest`, which needs TamaGo and `/dev/kvm`).

**Measured** on whim-vim.c's graph (9.66 MB of EDN, 239,125 nodes),
`callers ml_find_line`, on an idle 64-core host:

| | read | index | the view | wall | peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| Clojure, JVM (`view-clj --time`) | 1.06-1.32 s | 0.17-0.30 s | 14 ms | 2.6-2.9 s | 645-775 MB |
| Joker, host (`jokerhost view --time`) | 2.02-2.24 s | 1.36-1.42 s | 15 ms | 3.5-3.7 s | 367-382 MB |
| Joker, in the box, 4 vCPUs | 2.52-2.56 s | 1.72-1.75 s | | 4.7 s | 478 MB |
| Joker, in the box, 1 vCPU | 4.13-4.17 s | 2.99-3.10 s | | 7.6 s | 549 MB |
| Go (`whim view`) | | | | 1.9 s, the import uncached | |

The box's peak is the launcher's resident set (the guest's memory it
touched); the guest's wall time includes starting it and loading the
namespaces. The image was 33.9 MB with the EDN embedded; with the
store it is 24.3 MB -- Joker, the standard namespaces and the views.

The index is 1.36 s where the measurement above (persistent maps) took
3.71: vectors by id filled as transients (2.1 s with `doseq` walks), sized
by `:ids` rather than a walk (0.6 s saved), walked by `reduce` (1.25-1.4
s). Still five to eight times the JVM's 0.17-0.30 s: a function call is
Joker's dearest instruction (a bare walk of the graph's nodes costs 0.25
s), and the profile is the VM's dispatch and the allocator, no single hot
spot. The read is twice the JVM's: Joker's reader alone, every tag given
`identity`, takes 1.0 s; the reader functions the other 1.1 (1.7 before
`read-node` walked its edges in a loop). The views themselves are ten
times the JVM's when large: six heavy cases (`--depth 0 callers ml_get`,
3.9 MB of text; `--depth 0 callees main`, 4.1 MB) 10.3 s against 1.05 s,
an ordinary view 5-80 ms. In the box, on one vCPU, reading and
indexing run at half the host's speed; on four, where the collector has
the other vCPUs, at 80%.

**What the fork lacked**, worked around in the port and noted in
`guest/joker/joker/README.md` (gaps, not faults: nothing new to fix): no
transducers (`(set (map f xs))` for `(into #{} (map f) xs)`), no
`volatile!` (an atom), and a string's `count` is its characters, so the
printer's widths -- the Go's bytes -- are counted from the code points.

## The store (2026-10-05)

The box's data is no longer in its image: **the monitor keeps a store**,
blobs and refs, and the guest reaches it by five hypercalls of the
monitor's own (`guest/abi`: `BlobPut`, `BlobSize`, `BlobGet`, `RefGet`,
`RefSet`). A blob is immutable, named by its SHA-256; a ref is a name
pointing at a blob, the only thing that changes, by compare-and-set when
the guest asks (an old hash of all zeros: the ref must not exist). What
the guest names is a hash or a ref's name, `[A-Za-z0-9._-]{1,64}` -- never
a path of the host's.

On the host the store is a directory (`vmm.DirStore`, `vmm/store.go`):
`blobs/HASH` and `refs/NAME`, the hash in hex. **It runs inside the
monitor's system-call filter as it stands**: a blob is created
exclusively under its own name in one write, a ref rewritten in one write
of 65 bytes, so it needs `openat`, `read`, `write`, `fstat` and `close` and
nothing the filter did not allow already -- no rename, no unlink, no
`pread`. A blob a write left short is refused when first read, held to its
hash; a blob read is kept, verified, while the monitor runs. Without a
store every call answers -1.

- `WHIM_GUEST_STORE=DIR` gives the launcher a store;
- `go tool whim store DIR put FILE | get HASH | ref NAME [HASH] | graph
  NAME FILE | cc NAME FILE` is it on the host -- `graph` writes FILE's graph
  as EDN (a C file imported, or a graph's Lisp read) and points NAME at
  it, `cc` the C host's bundle (*The editor in the box*);
- in Joker, the namespace `box` (`guest/joker/box`, one package for the
  guest's hypercalls and jokerhost's `--store DIR`):

```
user=> (def h (box/put "from the box"))
user=> [(box/ref! "note" h nil) (box/ref! "note" h nil) (box/get (box/ref "note"))]
[true false "from the box"]
user=> (time (count (box/load "graph")))
"Elapsed time: 40.68358 msecs"
9663341
```

`(box/load NAME)` is `(some-> (box/ref NAME) box/get)`: whim-vim.c's
9.66 MB of EDN in 41 ms, read in 1 MiB `BlobGet`s and hashed once.
`TestJokerViewsGuest` builds one image and runs both its graphs from a
store of its own, 48 of 48 and 24 of 24 as before; `vmm`'s `TestDirStore`
holds the store's refusals and its compare-and-set.

## The editor in the box (2026-10-06)

The editing server -- the session, its buffers and their journal, what
`whim view-serve` answers -- runs in the box, driven from the REPL. It is
`crefactor/graph/view`'s `Server` now (`server.go`, moved out of
`cmd/whim`, which calls it), linked into the guest as the namespace `ed`
(`guest/joker/ed`):

- `(ed/start edn cc)`: a session on the graph `edn`, read by
  `graph.ReadEDN` -- whim-vim.c's, 431,697 nodes, in 132 ms in the box;
- `(ed/req LINE)`: one of view-serve's requests, `{:head H :body B}`, an
  error thrown with its message; `(ed/! LINE)` prints the body;
- `write NAME` puts the C in the store and points ref `NAME` at it. A
  buffer opened `--fallout` closes over a deletion's uses with vim's
  options, which moved from `internal/whim` (it would bring the
  translators into the image) to a package of their own,
  `internal/whim/vimgraph`: `ml_clearmarked` deleted whole is pending in a
  plain buffer, being called, and applied with its call in one opened
  `--fallout`.

**The box has no C compiler**, and an applied edit parses and checks a
fragment's pared unit with cc, whose `NewConfig` asks the host's compiler
what it predefines and where its headers are, then reads the headers.
cc's fork takes a preset now (`crefactor/cc/host.go`, `SetHost`): the
compiler's answers and the headers as a file system. `graph.HostBundle`
records them while it imports a file -- for whim-vim.c 32 headers, 116
KB of JSON -- and `whim store DIR cc NAME FILE` puts it in the store, so
`(ed/start (box/load "graph") (box/load "cc"))` edits as the host does:

```
user=> (ed/req "open def vim_snprintf")
user=> (ed/! (str "change " at " " at " \"\\n  (= str_l 0)\""))    ; a statement typed in
"Elapsed time: 54.638237 msecs"
"applied 147"
user=> (print (:body (ed/req "c vim_snprintf")))   ; ... str_l = 0; va_start(ap, fmt); ...
user=> (ed/! "undo")                                ; 3.8 ms
user=> (ed/! "write edited")
"2082553"
```

The fragment declares a `va_list`, from the bundle's `stdarg.h`. The
whole run, the monitor started and the graph read, is 0.7 s.
`TestJokerEditGuest` holds it: the change applied, the C printed with
it, undone, the C written to the store whim-vim.c byte for byte, and the
same change refused when no bundle is given. `TestHostBundle` holds an
import on a bundle read back to the host compiler's graph. The image is
28.6 MB, the graph packages and cc the 4.3 above the views'.

## The collector in the box (2026-10-06)

The namespace `rt` (`guest/joker/rt`, in the guest and in jokerhost)
reads Go's runtime: `(rt/mem)` the heap and the collector's work -- its
stop-the-world pauses from `runtime/metrics`, a histogram whose buckets
give the quantiles -- `(rt/gc)` a collection, and `(rt/trace-start)`,
`(rt/trace-stop)` the execution tracer into memory, its bytes for
`box/put` and `go tool trace` on the host. Three workloads, each a run of
its own: **index**, the views' index of whim-vim.c's graph (box/load,
read, index, a view); **ed**, the editor in the box started on it and 20
edits each undone; **churn**, a map of a million entries built by
`assoc` at the REPL. Times are the forms' (`time`), the heap live after a
collection, sys the most the runtime held:

| | index | ed: start, 20 edits | churn | live | sys | pauses, total | p50 / p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| box, 1 vCPU | 7.08 s | 155 ms, 1.56 s | 7.58 s | 164 / 97 / 12 MB | 506 / 299 / 691 MB | 0.3 / 0.3 / 0.5 ms | 3.6-5.1 / 28.7-41 µs |
| box, 4 vCPUs, park 1 ms | 4.51 s | 147 ms, 1.21 s | 4.68 s | the same | 386 / 247 / 600 MB | 44.5 / 36.1 / 82.7 ms | 1,311 / 1,573-1,835 µs |
| box, 4 vCPUs, park 50 µs | 4.36 s | 131 ms, 1.19 s | 4.75 s | the same | 415 / 238 / 616 MB | 11.0 / 8.0 / 18.7 ms | 229-328 / 459-524 µs |
| host, GOMAXPROCS=1 | 5.28 s | 99 ms, 1.28 s | 6.58 s | 174 / 106 / 12 MB | 516 / 310 / 697 MB | 0.6 / 0.5 / 1.1 ms | 6.1-10.2 / 57-82 µs |
| host, GOMAXPROCS=4 | 3.62 s | 89 ms, 1.12 s | 3.87 s | the same | 401 / 237 / 620 MB | 1.8 / 2.0 / 3.4 ms | 33-49 / 197-328 µs |

**The heap is the program's, not the box's**: live and sys within a few
per cent of the host's, the graph's index 164 MB live (the EDN's 9.66 MB
read into Joker's vectors and maps), the editor's session 97 MB, and a
collection runs as often in the box as on the host. On one vCPU the
pauses are as short as the host's, shorter even: a stop of the world on
one vCPU has nothing to wait for. **On several they were the monitor's:**
TamaGo's idle Ms keep their Ps and nothing wakes them for a stop of the
world, so each stop waits until every idle vCPU looks again -- at most the
park's limit, 1 ms by default, the pauses' 1.3 ms. A runtime trace taken
in the box showed it (the idle P going Running->Idle 0.9-1.3 ms after the
stop began), and that the limit could not be shortened: the park waited on
the host's Go timers, which a mostly idle Go program serves by
`epoll_wait` in whole milliseconds, so 20 µs lasted 1.1 ms. The park is a
futex now (doc/GUEST.md, *SMP*), and `WHIM_GUEST_PARK=50us` gives pauses of
230-330 µs, a quarter of the total; the default stays 1 ms, which the
editor's exits prefer (43 a key against 63). The collector costs little
either way -- 0.2-2% of the time in pauses -- and on four vCPUs the
concurrent mark runs beside the program: 200 forced collections take 1.67
s on one vCPU, 2.0 s on two at 1 ms, 0.98 s on four at 50 µs.

**The stop wakes them now** (2026-10-06). The runtime has no hook at a
stop of the world, but the stopping M waits for the others in
`semasleep` 100 µs at a time (`stopTheWorldWithSema`, `forEachP`), which
reaches the board's idle with a deadline that near, and an idle vCPU's
wait is a timer's or none. So when the board idles with a deadline
within 200 µs it calls `CPUWake` with -1, which gives every other vCPU a
wake -- the parked look again at once, one about to park finds the token
-- and a timer that near wakes them for nothing. At the default limit,
1 ms, 200 forced collections on two vCPUs take 1.20 s (2.0 before), on
four 0.94 s (1.77), the pauses' p50 82-131 µs (1,311); the views' index
on four vCPUs pauses 6.0 ms in all (44.5), the churn 16 ms (82.7), its
p50 131 µs -- its p99 one pause of 5 ms. The editor pays nothing: the Go
guest's quick suite is 43.40 exits a key, as before.

**The long pause, traced.** In about one run in six the churn has one
pause of 1.3-5 ms. A trace of one, 5.1 ms (mark termination), shows the
other idle vCPUs joining in 47 and 73 µs. The goroutine running the
program is preempted only at +5,076 µs, inside `mallocgc`, on an allocation
path that checks for preemption at every call -- so it was most likely
not running at all: its vCPU's thread was off the host's CPU. The
monitor's threads see 14-31 involuntary context switches in every run,
the short ones as the long, so the count does not prove it, and nothing
in the guest can. It is not the missing asynchronous preemption, which
would matter only for a loop without calls.

An idle M giving its P up, as Go does on other systems, would spare a
stop the wait for idle vCPUs. With the wake that wait is 47-73 µs, and
the change is TamaGo's scheduler ("Ms are bound to P on tamago"), so it
is left.


## Closures (2026-10-06)

The next stage the sketch named -- a compiler to Go closures, held to
the interpreter's answers -- is built as a second backend in the fork
(`guest/joker/joker/core/closure.go`). Joker parses a form into an
expression tree and compiles that to bytecode for its VM; the closure
backend compiles the same tree to Go closures instead, each expression a
function of the frame it runs in, with locals resolved to slots, captures
to the closure's values and vars to their `*Var`. A closure-compiled
function is still a `*Fn`, so `fn?`, metadata, macros and the natives
that take functions see no difference. `--closures`, given to jokerhost
or to the guest, chooses it for everything. The core library is
re-evaluated from its source (`core.joke`, embedded, 0.1 s), since it
ships as bytecode, so a program and the library it calls run on one
backend.

The semantics are the VM's, piece by piece:
- a capture is the slot's value when the closure is made, and a
  `letfn`'s bindings are cells its closures share;
- `recur` assigns the loop's slots and runs the loop again;
- `try` catches the Errors its catches name, and its finally runs on
  every way out;
- an error carries the call site being called, and its stack trace
  lists the frames' calls as the VM lists its own.

Three things make it faster than the VM rather than slower:
- **No allocation for a call.** Frames and their values sit on a stack
  that is reused, one per execution, saved and restored where the VM's
  context is (Suspend, Resume, LockIndependent). A call's arguments are
  temporaries in the caller's frame, at offsets fixed at compile time.
  The first version allocated three objects a call and ran `fib` at 2.6
  times the VM's time.
- **Forwarding arities.** An arity whose body only passes its arguments
  to a var's function -- `([x y] (add__ x y))`, most of the arithmetic
  and comparisons -- calls that native directly while the var still holds
  it. The check is made at each call, so a redefined var is honoured, as
  the VM's comment insists ("Vars are mutable").
- **Resolution at compile time**: no opcode dispatch, no operand
  decoding, no stack shuffling.
- **An inline cache at each call site**: the arity the last closure
  called there took, kept beside the site, so a call of the same function
  chooses none (choosing was 12% of `fib`'s time).

**Held to the VM.**
- Upstream Joker's eval tests (`tests/eval`, 42 files with the standard
  namespaces the guest links): 201 tests and 4,595 assertions, every
  file's output the same bytes on both backends, stack traces included.
  The 8 files that fail to load on a missing classpath fail identically.
- `TestJokerClosures`: a corpus of forms covering each construct and
  five errors, the same bytes.
- `TestJokerViews` with `JOKER_VIEWS_ALL=1`: the 6,370 views on
  whim-vim.c's graph, on both, each the Go's bytes.
- `TestJokerViewsGuest`: in the box, on both, 48 of 48 and 24 of 24.
  It failed once, on closures in the box, in its first run, and the
  message was lost to a filter. It has not failed since: 21 runs, nine
  of them three processes side by side. Run four at once, they found
  something else: each regenerates Joker's core into the source tree, and
  one compiled a file another was writing -- `guest.JokerGenerate` holds
  a lock now (`joker/.generate.lock`) until the build after it ends.

**Measured** (ms, the best of three; host, then the box at 1 and 4 vCPUs):

| | fib 27 | loop 3M | reduce 1M | assoc 200k | sort 200k | strings 200k |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| host, VM | 222 | 1,451 | 1,009 | 765 | 3,205 | 589 |
| host, closures | 96 | 841 | 918 | 641 | 2,072 | 482 |
| box 1 vCPU, VM | 321 | 3,232 | 1,227 | 1,035 | 6,640 | 805 |
| box 1 vCPU, closures | 141 | 1,611 | 1,192 | 1,019 | 4,519 | 729 |
| box 4 vCPUs, VM | 323 | 1,860 | 1,014 | 699 | 3,950 | 632 |
| box 4 vCPUs, closures | 141 | 1,026 | 775 | 673 | 2,652 | 450 |

Calls and arithmetic run 1.7 to 2.3 times as fast (1.4 to 1.9 before
the inline cache); work in the
persistent collections and the natives runs 1.1 to 1.3 times. The views
are 1.3 times as fast on the host (6,370 views: 97 s against 127) and 1.2
in the box (the 24 product views: 42 s against 50). The closures fall
short of the sketch's "two to five times an interpreter's" because
Joker's interpreter is not a tree-walker: its VM is already a compiler,
to bytecode. What is left is Joker's values -- boxed numbers, persistent
maps -- and its natives, which a closure calls as the VM does. In the
loop of a million additions, half the time is the collector's: every
`Int` a native returns is two words boxed in an interface (`convT`), for
the bytecode as for the closures, so a number without a box is the next
step, and a change to Joker's values rather than to either backend.

## Unboxed numbers (2026-10-06)

The closures left the collector half of an arithmetic loop's time. Every
`Int` a native returns is boxed in an `Object`, and an `Int` was four
words: a pointer to its reader position, the value, and the literal's
spelling (`Original`, which only Joker's formatter reads). Each box was a
32-byte allocation the collector had to scan. In the fork it is one
word now, `Int{I int}`:
- no position (`GetInfo` is nil, `WithInfo` the Int itself), no spelling;
- a box is the tiny allocator's 8 bytes, never scanned, and values below
  256 are Go's own static boxes;
- `nil`, boxed afresh at every `OP_NIL` and every recur's return, is
  boxed once (`nilObject`).

A position mattered in one place: an integer literal called, `(1 2)`,
whose "1 is not a Fn" named the literal's column. The parser now puts a
literal with no position just inside its form, where `(1` has it; only
`( 1 2)`, with a space, moves by a column. Upstream's eval corpus is the
same bytes on the old build, the new VM and the closures, 42 files of
42.

**The one-vCPU stall.** Measuring this found a larger cost on one vCPU.
TamaGo has no `sysmon`, so nothing preempts a running goroutine. A
collection that starts during a long computation has its mark workers
waiting for the goroutine to block, so the computation runs with the
write barrier on, assisting the marking. A trace in the box showed the
mark phase active across a whole `fib`. It hit whichever run started a
cycle at the wrong moment: `fib 27` on the VM took 1,280 ms instead of
321. Both backends now yield the processor once every 65,536 calls
(`yieldTick`), and the cycle finishes.

Measured (ms, the best of three):

| | fib 27 | loop 3M | reduce 1M | assoc 200k | sort 200k | strings 200k |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| host, VM | 227 | 1,145 | 852 | 722 | 2,619 | 587 |
| host, closures | 100 | 534 | 800 | 625 | 1,568 | 493 |
| box 1 vCPU, VM | 333 | 1,801 | 1,057 | 1,014 | 3,849 | 785 |
| box 1 vCPU, closures | 143 | 804 | 956 | 913 | 2,630 | 703 |
| box 4 vCPUs, VM | 327 | 1,713 | 806 | 698 | 3,396 | 601 |
| box 4 vCPUs, closures | 146 | 733 | 626 | 646 | 2,063 | 481 |

Against the table in *Closures*:
- closures' `loop` 841 -> 534 ms on the host, 1,611 -> 804 in the box on
  one vCPU;
- `sort` 2,072 -> 1,568 on the host, 4,519 -> 2,630 in the box;
- the VM's `loop` 3,232 -> 1,801 and `sort` 6,640 -> 3,849 on one vCPU.

Reading and indexing whim-vim.c's graph, whose EDN is mostly integers,
peaks at 354 MB on the VM instead of 445, and at 324 instead of 387 on
the closures.

The Go editor guest runs on the same runtime, so it was measured for the
same stall, with the collector on and off (`debug.SetGCPercent(-1)`, an
experiment not kept). `TestGuestScale`'s `:%s` over 200,000 lines takes
6,070 ms against 5,409 at one vCPU, 1,598 against 1,463 at four: the
collector's ordinary 11% and 9%, the same share either way. The suite's
heavy case at one vCPU is 596 ms against 554. It does not stall: its
long work runs as goroutines in chunks, and the end of each chunk is a
point where the mark workers get the processor. Nothing there needs a
yield.

## The editor on the console (2026-10-06)

`gview.vi` (`guest/joker/gview/vi.joke`, 473 lines of Joker) is a modal
editor over the editing server: `(gview.vi/edit "def F")` opens the
view as a buffer of `ed`'s session and edits it with vim's keys until
`:q`, then hands its last state back to the REPL it was called from.
- **Normal mode**, a count before most: `h j k l` and the arrows,
  `0 $ w b gg G`, `x dd yy p P` (the unnamed register, or `"a` before
  one for a named one), the operators `d c y` on a motion or a text
  object (`iw aw`, `i( a(`, `i" a"`), `D C s S r`, `.` (the last change
  again, its keys replayed), marks (`ma`, `'a`, `` `a ``), `i a A I o O`, `v V` (visual, characters or
  lines: a motion, then `d x y`), `/PATTERN n N`, `u` (the session's
  undo), Enter (the view at the cursor: what the node there is about),
  `K` (its callers) and Ctrl-O (back to where Enter or `K` left).
- **Command line:** `:q`; `:w NAME`, the C into the store under ref
  `NAME`; `:e VIEW ARG`, another view; `:rename NAME`, the entity at the
  cursor with every declaration and use; `:undo`; `:back`.
- **`:(FORM)`:** a Joker form evaluated, its value on the status line.
  The REPL is the command line.

The buffer is edited in Joker. An edit -- `x`, `dd`, an insert left by
Esc -- goes to the server as the one change that turns the server's text
into the buffer's (`change FROM TO TEXT CURSOR`). The server's answer is
what the screen shows next: its text and its cursor, and its verdict on
the status line: applied, pending with the reason, layout, or same. On
the views' sample, `0` deleted from `(= opt 0)` is pending ("not yet a
program"), `2` typed is applied, and the C says `opt = 2;`. Renaming
`vim_snprintf` to `n2` in the box is refused, because `win_redr_ruler`
has a local `n2` that a renamed call would name; that is the server's
check, shown on the status line.

The keys are the console's (`term`, `guest/joker/term`): the same
buffered input the REPL reads forms from, so the editor takes the keys
after the form that called it and the REPL the forms after `:q`. Raw
mode and the window's size are the core's own host calls (`TermStart`,
`TermStop`, `GetWinsize`). jokerhost reads its stdin, which is how
`TestJokerVi` drives it on the sample. `TestJokerViGuest` drives the
same session in the box, on whim-vim.c's graph and the C host's bundle:
a statement typed into `vim_snprintf` and applied, undone, the function
renamed, `:w`, and `:(+ 1 2)`. On a pseudo-terminal the guest draws a
screen a key and gives the REPL back after `:q`.

A command that changes the text sends its change when it ends: `5x`,
`2dd` and a visual `d` are each one request. `:e` and Ctrl-O put the
editor's own buffer on the other view (`@B open`), so a session keeps
one. Lines wider than the screen scroll sideways with the cursor, and
a visual selection is shown in reverse. `TestJokerViKeys` holds each
feature as a session of its own on the sample: a line yanked and put
below another and the C with it twice; `2dd` and `Vjd` the same
applied edit; `/opt` then `n` and `x` leaving `pt`, which the server
calls undefined; a named register put above; Enter, then Ctrl-O back
to the offset left; `:e uses opt` on the same buffer; `$` on a
20-column screen showing the line's end.

`.` repeats a change as the keys that made it: each command that changed
the text is kept as its keys, from where normal mode was idle to where
it is again, and `.` types them at the cursor; undo, the command line,
search, Enter, `K`, Ctrl-O and the puts are not repeated. The tests add
a session each: `cw`, `r`, `ci(`, `ciw` on spaces, `diw`, `D`; `r5`
repeated by `.` on another line, both applied; a mark set, left and
jumped back to. Marks are offsets, not moved by an edit before them.

Not done: the counts on `p` beyond repetition, marks that follow edits,
`.` with the count of the change it repeats. Offsets are characters in Joker and bytes in the server,
which agree for the views' ASCII.

## What could be done next

1. ~~The Joker guest~~ and 2. ~~tagged literals~~: built, above, and
   the views ported to Joker over the graph's EDN, in the box (*The views
   in Joker, in the box*), the graph read from the monitor's store (*The
   store*).
3. A datagram hypercall, and a design note for the network: netstack and
   TLS in the guest, the host a packet forwarder under a policy.
4. ~~Measure the collector inside the box~~: measured, above (*The
   collector in the box*).
5. ~~The editor in the box~~: built, above.
6. ~~A compiler from the forms to Go closures~~: built, above
   (*Closures*), held to the VM's answers and faster than it.
7. ~~A modal editor on the console~~: built, above (*The editor on the
   console*).

## Appendix: the scratch guests

The Joker guest's `main.go` (its module requires the repository's
`guest/tamago`, TamaGo v1.27.1 and Joker, with replaces to a local clone
after Joker's `go generate ./...`; built with TamaGo's go, `GOOS=tamago`,
`-ldflags "-T 0x201000 -R 0x1000"`, as `guest/gotamago.go` builds the Go
guest; run with `WHIM_GUEST_IMAGE=jg.elf bin/whim-guest`):

```go
type console struct{ buf [256]byte }

func (c *console) Read(p []byte) (int, error) {
	b := board.Call(abi.WaitRead, -1, board.Addr(c.buf[:]), int64(len(c.buf)), 0, 0)
	n := int(b.A[0])
	if b.Ret == 0 || n <= 0 {
		return 0, io.EOF
	}
	return copy(p, c.buf[:n]), nil
}

func main() {
	GLOBAL_ENV.InitEnv(Stdin, Stdout, Stderr, nil)
	RT.GIL.Lock()
	ProcessCoreData()
	GLOBAL_ENV.ReferCoreToUser()
	reader := NewReader(bufio.NewReader(&console{}), "<repl>")
	pc := &ParseContext{GlobalEnv: GLOBAL_ENV}
	for !form(reader, pc) { // TryRead, Parse, Evaluate, PrintObject; an error printed
	}
	board.Exit(0)
}
```

The yaegi guest is `interp.New`, `Use(stdlib.Symbols)` and `Eval` of each
argument, on the same board.
