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
- **a pointer is an address**, typed by what it points at: `Ptr Win_T`, a
  struct a phantom type with no values; `Ptr Char_u`, a C typedef of a
  scalar a synonym, `type Char_u = Word8` (an enumeration's too, `type
  State_E = Int32`); `Ptr ()` for `void *`. Walking, comparing, subtracting
  and punning are C's, the runtime's readers and writers take a pointer of
  any type, and the printer writes `castPtr` where the C converts one
  pointer type to another -- so GHC checks what the C checks, and none of
  the pointer analysis the other backends need is used;
- **the file-scope objects are one segment per editor**, each at its offset,
  read, written and addressed by accessors of its name (`curwin ed'`,
  `set'curwin`, `addr'curwin`); a member's offset is a constant of its
  struct's and its own name (`win_T'w_cursor`); their initial values are an
  image of the constant bytes, copied in when the editor is made, and the
  addresses written after it; a string literal is a primitive string literal
  (`Ptr "..."#`), static memory GHC stores as its bytes; a compound literal
  in a file-scope initializer gets room of its own in the segment;
- **a local whose address is taken, or that is an array or a struct used
  as bytes, lives in its call's frame** (`frame n $ \fr' -> ...`,
  `allocaBytes`), zeroed; every other local is a Haskell binding -- a struct
  of scalars used by member and copied whole one binding per member, and a
  local whose address only goes to an out-parameter a binding too;
- **an out-parameter is a value in and a value out**: phase 181 makes it
  so in the C (`LocalOut`), a struct of the result and the values returned,
  which the Haskell returns as a tuple -- a function whose C result is a
  struct of scalars returns its members, `IO (Bool, Ptr Char_u, Int32)` --
  and the printer's own analysis (`hsout.go`) takes the few the C cannot
  (`(r, x1) <- f ... x`);
- **a function that touches no memory is a Haskell function** of its
  arguments, no `IO` (`musl_isdigit :: Int32 -> Int32`), its loops pure
  recursion; one that reaches no file-scope object, no host and no function
  pointer takes no editor;
- **a function pointer is an index** into the table of the functions whose
  address is taken (`fnTable`), at an address no object has, since a Haskell
  function cannot live in raw memory; the editor carries the table (`Ed`'s
  `edFns`), so a call through one (`callPtr`) needs no import of the module
  that makes it; it passes its arguments as their 64 bits;
- **a struct passed by value is its address**, which the callee copies into
  its frame; **a struct result** is written through an address the caller
  gives (`sret'`), room in the caller's frame.

Unsafe underneath -- raw memory -- but exactly the C's semantics, and what
the C lets the types say, they say (`doc/HASKELL-IDIOMS.md`).

## Control flow: blocks in place, joins and loops as local functions

The Clojure backend's lowered form (`crefactor/togo/lower.go`: a function as
basic blocks in three-address form) is the input (`hsshape.go`). A block
that one jump reaches is written where the jump is -- straight on after a
goto, the arm of an `if` or a `case` -- a block that only jumps is where it
jumps, and the entry is the function's body; what is left is a local
function of the variables live at its start: a join, which several places go
on to, or a loop's head, which a jump back reaches (`loop'N`), each jump a
tail call. GHC compiles a known tail call to a local function as a jump, so
every construct -- early return, `break`, `continue`, fall-through and the
49 gotos -- is one mechanism, with no state machine, no block copied, and
none of `clj_shape.go`'s nesting rules: a local function may be called from
anywhere in its scope.

Each value of a variable is a name of its own (`hsnames.go`): `p`, then
`let !p1 = ...`, and a block's parameters new names again, so no binding
shadows another; a value that is a name or a literal is that name, bound to
nothing; a parameter the function never assigns is read from its scope. So
`ghc -Wall` has nothing to say about the module (`whim caprice --lint`).
Enumerators are pattern synonyms of any integer type (`ESC`, `NUL`), in
expressions and in `case`, and character constants `ch '-'`
(`hsnamed.go`).

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
  89,953 lines of Haskell in nine modules -- the top `Caprice.Editor`,
  `Caprice.Editor.Defs` and six parts by the call graph, the largest the
  core's one big cycle of calls (33,735 lines) -- tracked under
  `caprice/Caprice/` and refused by `whim-editor-check` when stale; 122,743
  lines in one module before `doc/HASKELL-IDIOMS.md`'s changes. 56 of the
  functions are pure, 221 take no editor, 97 out-parameters are values in
  and out, 84 struct locals are bindings.
- **GHC**: 145 s and 1.24 GB peak at `-O1`, `-j4` (one module: 213 s and 3.50
  GB), printed beside every build; `ghc -Wall` has nothing to say
  (`whim caprice --lint`). A build whose core has not moved skips it
  (`caprice.Build` writes a source only when it differs), a moved part
  recompiles that part and what imports it, and the suite keeps its two
  builds in `.cache/caprice-suite/`.
- **Speed**: the heavy case 1.0-1.3 times the C (the Go 0.5, the Java 1.8-2.1,
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

- **Idiomatic Haskell, beyond the survey**: the core is still C's memory --
  most functions in `IO` reading and writing raw bytes at named offsets, the
  file-scope objects a segment -- which is what the C is.
  `doc/HASKELL-IDIOMS.md`'s twelve items are done, or done in the part it
  found worth doing (newtypes, `Maybe`, `data` enumerations and list folds
  declined, with why).
