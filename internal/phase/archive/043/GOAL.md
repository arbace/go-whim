# Phase 43 — no -c, --cmd, -R, -m, -M or -w

**A record now.** Phase 1's first step (`argvfront`, the pipeline reform's
first drop package, `doc/PIPELINE-REFORM.md` §7) cuts the whole command line on
the seed. The fall-out closure then folds what the options set, since nothing
writes it after the cut. So this phase has no plan entry and no program. What
follows is the account of the cut as it was made here.

Six command-line options become what any unknown option is: exit 1, naming
itself. `+{command}` stays, and fills the same list `-c` did.

`tools/dropopts.py` removes `-R`, `-w` and `--cmd`, and the argument switch's
`case 'c':`. It refuses the other two, rightly, and `tools/nocmdargs.py` cuts them
by hand: **`-c` has a body of its own that falls through** into `-T` and `-u` —
`-c{command}` takes the rest of its argument and breaks, `-c {command}` falls
through to ask for the next one — and **`-M` falls through into `-m`**, so the
pair goes together. `--cmd` was the only long option that took an argument, so
the argument switch's `case '-':` goes, the option switch's one
`if (!want_argument)` can no longer be false, and `exe_pre_commands()` loses its
call and goes with the fields it read.

**The harnesses drove the editor with `-c`.** `tools/behaviour.py` and
`tools/exsweep.py` pass `+{command}` now, and — since Phase 18 took `-u` — no
`-u NONE` either. It is the same list in the same order,
so the change moves nothing against any binary either pipeline has made —
measured against `.reference/slim-vim`: 0 of 67 behaviour cases and 0 of 600
Ex-sweep rows differ from the baselines recorded with `-c`. Both are in every
phase's implementation digest, through `whimdelta.sh` and `verify.sh`, so every
boundary in both pipelines was verified again after the change.
`tools/clicheck.py` still passes `-c`: it runs at Phase 3, where `-c` exists.

The phase checks each dropped spelling — `-c qa!`, `-cqa!`, `--cmd qa!`, `-R`,
`-m`, `-M`, `-w7` — against a `+qa!` control.

## The delta

**None the Ex sweep records.** Measured: 117,013 → **116,892 lines**, libc
symbols 88 → 88.
