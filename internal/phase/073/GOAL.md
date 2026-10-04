# Phase 73 — `deathtrap()` runs at the host's next wait

*Formerly phase 147. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

SIGHUP and SIGTERM ran `deathtrap()` as their handler. So the core's whole
way out (preserving, restoring the terminal, writing its message, exiting)
ran inside a signal handler, at whatever point the signal found the core.
That is undefined behaviour in C, since none of it is async-signal-safe, and
Go can't express it: Go's runtime takes the signal and hands it to a
goroutine. The Go host queued the signal and ran `deathtrap()` at its next
wait (finding 12).

The C host now does the same, with a self-pipe:
- **the handler** records the signal and writes a byte to a non-blocking
  pipe;
- **the wait** runs a pending `deathtrap()` first, then selects on the pipe
  beside the input. The read runs a pending one first too;
- **no race:** a signal that lands after the flag is tested and before
  `select()` leaves a byte, which ends the `select()` at once.

Timing doesn't change either. The core already blocks deadly signals
everywhere except the wait in `ui_inchar()`. `vim_handle_signal()` turns a
signal that arrives while the core is busy into an interrupt, and raises it
again when that wait unblocks. So `deathtrap()` only ever ran in that window,
and it still does, at the wait or the read inside it.

It maps line for line onto `editor/host.go`, and from there onto a Go
`select` over an input channel and a `signal.Notify` channel.

**Declared delta: nothing**, and the libc surface grows by `pipe2`, which the
check requires exactly. The host now has twelve `#include`s, `<fcntl.h>` for
the pipe's flags. The check's probes run on a real pty with stderr on its own
pipe: SIGTERM and SIGHUP while the editor waits for a key, and SIGTERM during
a substitution that backtracks for longer than the probe runs. Each gives the
same output, stderr and exit status on both binaries. The controls:
- SIGTERM and SIGHUP differ, because the message names the signal;
- the busy run is interrupted and never finishes its substitution, so the
  signal did land mid-computation. It says `Interrupted`, as it did before.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the objects, functions and statements are written by FRAG in one unit and <fcntl.h> moved by INCLUDE, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
