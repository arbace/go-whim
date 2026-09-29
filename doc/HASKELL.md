# caprice: the editor in Haskell

**Built** (2026-09-29): caprice is the fourth translation of the core, beside
the Go (`editor/`), the Java (`braaam/`) and the Clojure (`vijure/`), and held
to the same test: `whim test --haskell` and `--wide --haskell` answer all 80
and all 240 cases as the C does, with a control of its own. It was planned here
on 2026-09-26 and not scheduled; the plan held, and what changed is recorded
below.

```
go tool whim caprice        # bin/caprice, built in lib/caprice
make bin/caprice            # the same
make whim-test-hs           # the quick suite with caprice too
go tool whim test --wide --haskell
```

## What is on the machine

GHC 9.14.1 with its boot packages only -- `base`, `array`, `bytestring`,
`containers`, `stm`, `time`, `unix` -- and no `cabal`: caprice builds with
`ghc --make` from those alone, offline.

## The approach: C's own memory

The Go, Java and Clojure editors model C's memory in managed objects -- a
pointer class analysis, `Ptr[T]` and `BytePtr`, structs as classes -- because
their languages have no raw memory. Haskell has: `Foreign.Ptr` is an address,
`peekByteOff`/`pokeByteOff` read and write at an offset. So the translation
keeps C's memory as C keeps it (`caprice/rt/Caprice/Rt.hs`):

- **every C object lives in raw memory**, laid out as the C lays it out on
  amd64 -- the sizes and offsets are the C front end's (`crefactor/cc`);
- **a pointer is an address**, `Ptr ()`: walking, comparing, subtracting and
  punning are C's, so none of the pointer analysis the other backends need
  is used -- nor the unions' discriminants, nor the string-as-slice question;
- **the file-scope objects are one segment per editor**, each at its offset
  (`edSeg ed'`), their initial values written when the editor is made; a
  string literal is a primitive string literal (`Ptr "..."#`), static
  memory GHC stores as its bytes; a compound literal in a file-scope
  initializer gets room of its own in the segment;
- **a local whose address is taken, or that is an array, a struct or a
  union, lives in its call's frame** (`frame n $ \fr' -> ...`,
  `allocaBytes`), zeroed; every other local is a Haskell binding;
- **a function pointer is an index** into the table of the functions whose
  address is taken (`fnTable`), at an address no object has, since a Haskell
  function cannot live in raw memory; a call through one passes its
  arguments as their 64 bits;
- **a struct passed by value is its address**, which the callee copies into
  its frame; **a struct result** is written through an address the caller
  gives (`sret'`), room in the caller's frame.

Unsafe, and not idiomatic Haskell; but exactly the C's semantics, with the
least translation.

## Control flow: join points

The Clojure backend's lowered form (`crefactor/togo/lower.go`: a function as
basic blocks in three-address form) is the input. Each block is a local
function of the variables live at its start, each jump a tail call of one,
and each assignment to a variable a new binding of its name, `!x <- pure e`
(a `let` would be recursive in Haskell: `let !t = t + 1` is a loop, which is
how the first run found out). GHC compiles a known tail call to a local
function as a jump, so every construct -- early return, `break`,
`continue`, fall-through and the 49 gotos -- is one mechanism, with no state
machine and no nesting analysis: none of `clj_shape.go` is needed.

An expression prints as the lines that run first -- each memory read and
each call a binding of its own, in C's order -- and a pure value; an operand
of `&&`, `||` or `?:` that reads or calls runs only where C evaluates it
(`crefactor/togo/hs_expr.go`). The integer types are `Data.Int` and
`Data.Word`, of C's widths; conversions `fromIntegral`, C's usual
conversions decided as the other backends decide them (`usualK`); signed
division `quot` and `rem`; a C bool a `Bool`, a byte in memory.

## The host

The core calls 17 host functions, whose types the backend writes beside the
module (`Editor.hs.host`). As in the Go, the Java and the Clojure, they are
glue to an interface (`caprice/host/Caprice/Host.hs`): a `Host` is a record
of functions -- the window's size, raw mode, the keys, the clocks, the wait
for input, reading it, the signals, output -- `editor/host.go`'s Host, on raw
buffers; an editor's `Ed` carries its own (`edHost`, a `Dynamic`, since the
runtime does not know the host's types), with the arena that is the editor's
and not the host's: 1 GiB `calloc`ed and bumped atomically, as the C host's.
`Caprice.Run.run host args` makes an editor and runs it to its end,
`host_exit` an exception it catches; so a process runs any number of editors,
each on its own host and thread (`caprice/testdata/instances/Main.hs`: four at once,
on hosts of the test's own, none seeing another's text; `go test
./caprice/`). `vim_snprintf` is `editor/format.go` ported
(`caprice/host/Caprice/Printf.hs`).

The terminal host (`caprice/host/Caprice/Term.hs`, `newTerm`) is the C host
half of `whim-vim.c`, function by function, its state the instance's: raw
mode and the keys with `System.Posix.Terminal`, the window's size with an
`ioctl` (a `capi` import), the wait for input a `poll(2)` on the keys and a
pipe of its own (a safe foreign call, so that the signals' handlers, threads
of the threaded RTS, run meanwhile). The signals are the process's, so one
handler a signal tells every terminal alive -- a flag set and a byte down its
pipe, as the C's handlers do.

The host calls back into the core -- the printf's error messages, the death
of a SIGHUP -- so the two modules are mutually recursive: the backend writes
`Editor.hs-boot`, the interface of what the profile names
(`Profile.HsExports`, `internal/whim/gen.go`), which the host imports
`{-# SOURCE #-}`.

Found by the suite on the way: a wait on GHC's I/O manager
(`threadWaitReadSTM`) cannot watch a regular file (the quick suite's keys
are one: epoll says EPERM), and on a pseudo-terminal a zero-timeout wait
raced the manager's notice of the keys already queued, so vim thought none
were and sent a terminal query the C does not; asking the kernel directly,
as the C does, gives the C's answer.

## The parallel :%s

`match_lines` (phases 176-177) has a Haskell body of its own
(`RuntimeBody.Hs`, `internal/whim/gen.go`): the chunks on `forkIO`'s
threads (`chunks`, `Caprice.Rt`), each on a regex engine the core's
`alloc_clear` makes -- its size and its `failed` member's offset asked of the
C front end (`{{sizeof regengine_T}}`, `{{offsetof regengine_T failed}}`,
which the backend fills). The program runs with a capability a core and one
thread collecting (`-with-rtsopts=-N -qg`): the parallel collector made the
sequential work slower.

## Measured

- **The backend writes all 1,710 functions of the core and refuses none**:
  122,743 lines of Haskell, 5.6 MB, tracked as `caprice/Caprice/Editor.hs`
  (with its hs-boot) and refused by `whim-editor-check` when stale.
- **One module, not split**: the plan feared minutes and several GB; GHC
  compiles it in 198 s and 4.9 GB at `-O0`, and 183 s and 4.0 GB at `-O1`,
  which is what caprice uses. No `.hs-boot` for the core's own calls, no
  function table for them. A build whose core has not moved skips it
  (`caprice.Build` writes a source only when it differs), and the suite keeps
  its two builds in `.cache/caprice-suite/`.
- **Speed**: the heavy case 1.4-1.5 times the C (the Go 0.5, the Java 1.8-2.1,
  the Clojure 4.2-4.4). A `:%s` at 500,000 lines (the survey's buffer,
  `doc/PARALLEL-SUBSTITUTE.md`; seconds, median of three less the build):

  | | lit | dense | bt | cls | sel | :g bt |
  | --- | --- | --- | --- | --- | --- | --- |
  | C | 1.80 | 4.01 | 25.7 | 3.95 | .424 | 23.4 |
  | caprice, sequential | 2.37 | 3.93 | 18.0 | 7.97 | .502 | 16.4 |
  | caprice | 1.62 | 2.99 | 1.24 | .869 | .137 | 1.57 |

  Start-up 0.06 s with a capability a core, 0.00 without.
- **Tests**: `crefactor/togo`'s `TestHs*` translate fifteen of the Java and
  Clojure tests' C programs, compile each with GHC and require it to print
  what gcc's build prints; `TestHsControl` undoes one of C's rules at a
  time in the Haskell -- unsigned division, an unsigned char's widening, an
  unsigned shift, a struct's copy -- and each moves the output.

## Not done

- **Idiomatic Haskell**: the core is C in Haskell's syntax -- raw memory, IO
  everywhere, join points. The Go, Java and Clojure editors' idiom surveys
  have no Haskell counterpart yet.
