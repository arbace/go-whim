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
written as plain data -- at that cost.

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

## What could be done next

1. ~~The Joker guest~~ and 2. ~~tagged literals~~: built, above. The
   views ported to Joker over the graph's EDN in the box is the step that
   uses them.
3. A datagram hypercall, and a design note for the network: netstack and
   TLS in the guest, the host a packet forwarder under a policy.
4. Measure the collector inside the box (pauses, heap) on the views' index
   and the REPL.
5. A compiler from the graph's forms to Go closures, held to Joker's
   answers, as each backend here is held to the C's.

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
