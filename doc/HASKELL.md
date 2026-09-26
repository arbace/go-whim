# The editor in Haskell: a plan

**A preliminary plan, not scheduled** (2026-09-26): there is no intention to
translate the editor to Haskell. It is kept as what was measured and how it
would be done, should that change.

Written 2026-09-26, when the core's C was translated to Go, Java and Clojure,
each answering every case of `whim test` as the C does. A Haskell editor is
the next target, held to the same test: `whim test --haskell`.

## What is on the machine

GHC 9.14.1 with its boot packages only -- `base`, `array`, `bytestring`,
`containers`, `mtl`, `unix` -- and no `cabal`: the editor must build with
`ghc --make` from those alone, which keeps the build offline. `base` holds
`Foreign` (raw memory, pointers, `Storable`), and `unix` holds the terminal
(`System.Posix.Terminal`) and the signals (`System.Posix.Signals`).

Measured: a synthetic module shaped as the output will be -- 2,000 functions
of memory reads, writes and wrapping arithmetic in `IO`, 22,006 lines --
compiles in **29 s at -O0 and 20 s at -O1, about 1 GB at the peak**. The core
will be some 70,000-80,000 lines: one module would be minutes and several GB.

## The approach

**C's own memory model, through `Foreign`.** The Go, Java and Clojure editors
model C's memory in managed objects -- a pointer class analysis, `Ptr[T]` and
`BytePtr`, structs as classes -- because their languages have no raw memory.
Haskell has: `Foreign.Ptr` is an address with `plusPtr`, `minusPtr` and
ordering, `peekByteOff`/`pokeByteOff` read and write at an offset, and
`copyBytes`/`moveBytes`/`fillBytes` are memcpy, memmove and memset. So the
translation keeps C's memory exactly:

- **every C object lives in raw memory**, laid out as the C lays it out on
  amd64 -- sizes and offsets from the C front end's types (`crefactor/cc`),
  which already computes them;
- **a pointer is an address** (`Ptr ()`, cast as the C casts): walking,
  comparing, subtracting, walking backwards and punning are C's, so the
  pointer-class analysis the other backends need is not needed -- nor the
  string-as-slice question, nor the unions, which are the C's bytes;
- **the file-scope objects are one data segment per editor**, the string
  literals a read-only segment, an address-taken local `allocaBytes`, and
  `host_alloc` an arena, as the C host's is;
- **a function pointer stored in memory is an index** into the editor's table
  of Haskell functions, since a closure cannot live in raw memory.

Unsafe, and not idiomatic Haskell; but exactly the C's semantics, with the
least translation, and the other backends' hardest problems gone.

**Control flow as join points.** The Clojure backend's lowered form
(`crefactor/togo`'s basic blocks in three-address form) is reused: each block
becomes a local function of the live locals, each jump a tail call, each
assignment a new binding. GHC compiles a known tail call to a local function
as a jump, so every construct -- early return, `break`, `continue`,
fall-through and the 49 gotos -- is one mechanism, with no state machine.

**Arithmetic by type.** `Data.Int` and `Data.Word` have C's widths and wrap as
C's unsigned types do; conversions are `fromIntegral`, C's usual conversions
decided as the other backends decide them; signed division is `quot`/`rem`,
shifts `unsafeShiftL`/`unsafeShiftR` on the right type. Every value strict
(bang patterns, `IO` throughout), so no thunks build up.

**The host in Haskell.** Raw mode, the window size and the keys with
`System.Posix.Terminal`; the signals with `installHandler`, whose handlers
run on their own threads and set flags and wake a pipe, as the Go's and the
Java's do; input with a timeout with `threadWaitRead` and `timeout`;
`vim_snprintf` ported from `editor/format.go`, and held to it byte for byte
as the Java's `Printf` is.

## Milestones

Each verified before the next.

1. **A slice on foreign C.** The runtime (the arena, the segments, the
   function table, the literals) and the printer for the slice the Java's and
   the Clojure's first milestones covered, over the lowered form. Verified by
   foreign C programs translated, compiled with `ghc`, and required to print
   what gcc's build prints, with a control. Measured: compile time and memory
   per thousand lines of output, which decides the module layout -- one
   module, or modules by the call graph's strongly connected components,
   with `.hs-boot` files or calls through the function table across them.
   And a coverage report on `editor.c`.
2. **Every function**, and the core compiling within the measured budget.
3. **The host, the launcher and the suite**: `bin/whim-hs`, and `whim test
   --haskell` / `--wide --haskell` answering the 45 and 240 cases as the C
   does, with the Haskell editor's own control.
4. **Kept current**: the generated modules tracked, written by `whim gen`,
   and refused by `whim-editor-check` when stale.

## Risks, named in advance

- **Compile time and memory**, measured above: the module split is decided in
  milestone 1, and `ghc -j` builds modules in parallel.
- **The call graph's cycles** across modules, if it must be split: vim's core
  is mostly one strongly connected component, so a split may need the
  function table for its cross-module calls, at a cost in speed.
- **Unsafety**: a wrong offset is a wrong read, not an exception. The layout
  comes from the C front end, which already gives the C's sizes; a test
  compares the layout of every struct with gcc's `offsetof`.
- **The RTS**: signal handlers on their own threads, as in Java; the stack
  grows as needed (`+RTS -K` if a deep recursion needs more).
