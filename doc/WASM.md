# The editor in a browser: a plan

**A preliminary plan, not scheduled** (2026-09-29): there is no intention to
run the editor in a browser. It is kept as what was measured and how it would
be done, should that change -- as `doc/HASKELL.md` and `doc/RUST.md` are.

## What already works

- **The Go editor compiles to WebAssembly as it is.** `GOOS=js GOARCH=wasm
  go build ./editor` succeeds, and so does `GOOS=wasip1`: the generated core,
  its runtime (`crt.go`, `format.go`, `chunks.go`) and the host glue need no
  change.
- **Only the terminal host is the operating system's.** `editor/term` uses
  termios, `ioctl` and signals, and does not build for `js/wasm`.
- **Measured:** the editor with a stub host is 8,952,903 bytes of wasm,
  2,321,022 gzipped.
- **No files is a good fit:** the core has had none since phase 91.

## What it would take

**A browser host** -- a package of its own, say `editor/web`, about 200
lines: the 15 methods of `editor.Host`, each with a browser counterpart.

| Host | In the browser |
| --- | --- |
| `Write` | `term.write` into xterm.js, which reads vim's escape sequences as a terminal does |
| `ReadInput`, `WaitForInput(ms)` | xterm.js's `onData` feeds a channel; the wait is a `select` on it with a timeout |
| `WinSize` | xterm.js's rows and columns; a resize sets the flag and wakes the wait, as `editor/term` does for SIGWINCH |
| `NowMs`, `Time`, `Delay` | `performance.now()`, `Date`, `time.Sleep` (`WHIM_TIME`, phase 180, has no environment to come from) |
| `TTYKeys` | fixed: DEL erases, CTRL-C interrupts |
| `Init`, `Raise`, `Suspend` | no signals in a browser: nothing |
| `Exit`, `Message` | the editor stops and the page shows the message; a page does not exit |

The editor's input loop blocks, which a browser's main thread may not. In
Go's wasm a goroutine that blocks gives the thread back to the browser's
event loop, so `WaitForInput` is a wait on a channel -- no Asyncify, no
worker.

**A page:** `index.html`, the `wasm_exec.js` Go ships, xterm.js, and some 30
lines of JavaScript joining them; and a target, `make web`, building
`editor/cmd/whimweb` for `js/wasm`.

**Its test:** the same host run under Node.js with `wasm_exec.js`, fed each
case's keys through `ReadInput`, so that `whim test` holds the browser
editor to the C as it holds the other three.

## What it would not do

- **Run in parallel.** Go's wasm has one thread: `match_lines`'s chunks run
  one after another (`Chunks`, phase 177). The answers are the same.
- **Stay responsive through a long command.** A large `:%s` runs on the
  page's thread; the page waits, and CTRL-C cannot arrive until it ends.
  Run in a Web Worker, the bytes crossing by `postMessage`, it would not.
- **Have every key.** The browser keeps some (CTRL-W, CTRL-T, CTRL-N)
  before the page sees them.
- **Keep anything.** A buffer ends with the page; saving one (to
  `localStorage`, say) would be a capability of its own, and so a phase.

## The other ways, and why not

- **The C, by clang or emscripten.** The core names no libc and compiles;
  but the C host is POSIX, a browser's would be written in C, and blocking
  input needs Asyncify or a worker waiting on `Atomics.wait`: more work than
  the Go for the same editor.
- **The Java or the Clojure editor** needs a JVM in the browser (CheerpJ,
  TeaVM): heavy, and Clojure's compiled output a poor fit.
- **WASI** (`wasip1`) builds, and runs under wasmtime or Node.js; it is for
  a server, and a raw terminal with timeouts is awkward there.
