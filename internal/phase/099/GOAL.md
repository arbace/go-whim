# Phase 99 — the host's clock can be held still

*Formerly phase 180. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

`undo` and `redo` say how long ago the change they reach was made: "1 line
less; before #3  0 seconds ago". The count is `host_time()` now less
`host_time()` then, whole seconds of the wall clock, so it is 1 whenever a
session crossed a second between the change and the undo -- however short
the time between, and more often the slower the editor. Seven of the suites'
cases print it (`insert`, `gg_G`, `undo_redo`, `par_undo`; the wide suite's
`undo_redo`, `undo_block`, `undo_after_ins`), and the Java and Clojure
editors failed them now and then under load (`doc/AGENDA.md` recorded it).

The fix is the clock, not the cases, which would lose the rest of the
message -- the undo's own record of what it did. `host_time()`, in the C
host, returns `WHIM_TIME` when the environment holds it (`atol`'s reading),
and the time otherwise; the Go, Java and Clojure hosts do the same
(`editor/term`, `braaam/host/Term.java`, which the Clojure editor's host
calls), and the suite runs every editor with it set, so the clock stands
still and every undo was "0 seconds ago". Only the seconds clock is held:
`musl_now_ms`, which times the waits for keys, runs.

**Measured:** +6 lines. A session that changes a line, then searches
200,000 lines for what is not there, then undoes, said "5 seconds ago" on
the C, 3 on the Go, 5 on the Java and 21 on the Clojure editor -- each its
own run's time -- and, with `WHIM_TIME` set, "0 seconds ago" on all four.
The seven cases answer as before on every editor.
Under load -- the seven cases 16 times each on the Clojure editor, 48 at a
time, a load average of 37 -- 5 of 112 runs differed from the C with the
clock free, and none of 112 with it held.

Since phase 88 keeps `<stdlib.h>` (its `EXIT_FAILURE` assertion names it),
this phase asserts the include once instead of adding it after `<stddef.h>`;
the header stays where the input had it.

Since step 6 (`doc/GRAPH-MIGRATION.md`, *Step6 as built*) it runs on the graph: `host_time()`'s body is written by FRAG (`BodyC`, its `getenv`, `atol` and `time` the headers' externs as the import makes them) and the include is asserted among the include forms; phase 98 hands it the graph and it hands it to 100, and `whim-build-check` holds it to the bytes the text version made (in history, `19c8e86` and before).
