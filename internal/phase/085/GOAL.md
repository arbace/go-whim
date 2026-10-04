# Phase 85 — no two function pointers are compared

*Formerly phase 162. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

`do_cmdline()` asked `getline_equal(fgetline, getexline)` four times: whether
its lines come from the command line typed at `:`. That was the one comparison
of two function pointers in the core, and Go's func values compare only with
`nil`. The two callers that pass `getexline`, `nv_colon()` and `ex_at()`, now
say so with a new flag, `DOCMD_GETEXLINE`, and `do_cmdline()` tests the flag.
`do_one_cmd()`, which is handed the flags, reads only `DOCMD_VERBOSE` of them.
`getline_equal()` is left with no caller, and the sweep takes it.

**Declared delta: nothing.** The check requires `internal/ccx`'s
`FuncCompares` to leave exactly that comparison on the input and nothing on
the output. It requires the flag to be set by exactly the calls that pass
`getexline`, its bit to be no other `DOCMD` flag's, and `do_one_cmd()` to read
only `DOCMD_VERBOSE`. It probes `@:` after a command typed at `:`, and after a
`<Cmd>` mapping, which must not replace the last command line. Each control
moves.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the enum, the flags and the four tests are written by FRAG in one unit, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
