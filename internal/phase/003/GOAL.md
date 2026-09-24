# Phase 3 — no introduction, and the command line says only what the editor still decides

**An embedded editor starts in a buffer, not on a title card, and is started by
something that knows what it wants.** This was two phases with a third's worth
of work left undone between them. They were one question — *what may an
invocation say?* — and are answered once.

## The introduction

- **`:intro` and `:version` point at `ex_ni`.**
- **The splash screen's two call sites go.** `maybe_intro_message()` is called
  from the *redraw path* when the buffer is empty and no file was named. It is
  not a command, so an editor whose `:intro` was `ex_ni` would still greet you
  on startup.
- **`-h`, `-?`, `--help` and `--version` go**, and with them `usage()` and
  `list_version()`, which `--version` was the other door to.

That last is where the removal pays. With `list_version()` gone the sweep takes
the version tables, the feature lists, and `pathdef`'s `compiled_user` and
`compiled_sys`, which bake the *building machine's hostname* into the binary.
Measured: the name appears once in this phase's input and nowhere in its output.
That is worth removing on an embedded artifact's account and worth removing
twice on a reproducible one — a binary that names the machine that built it
cannot be byte-identical anywhere else.

## The command line

`tools/dropopts.py` deletes each option's `case` label or `else if` link, so the
option reaches `mainerr(ME_UNKNOWN_OPTION)` — the path anything unrecognised
already takes. `tools/optreaders.py` then removes what only a dropped option
could ever set, and every reader of it, because a field nothing sets is still
*read*: no warning names it and no sweep can take it.

| | options | |
| --- | --- | --- |
| **refusing** | `-A`, `-F`, `-H`, `-g`, `-nb` | print "not enabled at compile time" and exit — what an unknown option does anyway, one message less specifically |
| **inert** | `-f`, `-X`, `-Y`, `-d`, `-U`, `--nofork`, `--literal`, `--gui-dialog-file`, `--startuptime`, `--log` | accepted with an empty body, or an argument that goes nowhere |
| **said another way** | `-l`, `-C`, `-N`, `-V`, `--noplugin` | each is a `:set` — `lisp showmatch`, `compatible`, `nocompatible`, `verbose` and `verbosefile`, `noloadplugins` |
| | `-n` | `'updatecount'` to 0, so no swap file is written; `:set updatecount=0` says the same, and from Phase 11 there is no swap file on disk to avoid |
| | `-p` | the files as tab pages; `-o` and `-O` still lay them out as windows |
| | `--clean` | `-u DEFAULTS`, and empty defaults for `'runtimepath'` and `'packpath'` |
| **a capability** | `--not-a-term` | see below |

**`--not-a-term` goes on purpose, and it is the one that removes something.** It
told a full-screen run with no terminal not to warn, not to wait, and not to
restore a title. Without it that run warns and waits two seconds, as it did
before the option existed. An embedded editor is given a terminal or run with
`-e`, and every harness here runs `-e -s`.

What goes with them, found by `optreaders.py` rather than by the sweep:
`early_arg_scan()`, which existed to refuse `-nb` before anything else ran; the
pre-scan of `argv` at the top of `main()` that set `params.clean` before options
existed, `set_init_1()`'s parameter, and `set_init_clean_rtp()`; `is_not_a_term()`
and `is_not_a_term_or_gui()`, whose eight callers each keep the branch they took
without the option; the reader that turned `-n` into `'updatecount'`;
`WIN_TABS` at seven tests in `create_windows()` and `edit_buffers()`,
`p_shm_save`, and `make_tabpages()`; and `More info with: "vim -h"`, which ended
every usage error by naming a removed option and a binary this one is not.

**Not here:** `-y`, `-Z`, `-t` and `-i` go in Phase 18, and `-r` and `-L` in
Phase 21, each with the capability it selected — a flag is pointless only once
the thing it chose is gone.

## The trap, and the harness it needed

`dropopts.py` removed a long option's `else if` and then asked whether the text
*before* it ended in `else` — which it never did, because the match had already
consumed that `else`. So removing **any** link turned the next `else if` into a
bare `if`, and the chain came apart: `--clean`, `--noplugin` and `--not-a-term`
each matched their own branch, failed every test after it, and reached `mainerr`
anyway. **Three options broken for thirty phases, and nothing noticed, because no
harness passed a single option** — `behaviour.py`, `exsweep.py` and
`termcheck.py` all ran `-u NONE -e -s` and nothing else. The removed link's own
`else` decides now.

`tools/clicheck.py` is the harness that was missing. It runs every option the
parser has. A dropped one must exit 1 naming itself as unknown; a kept one must
not, and must do what it says wherever `:set`, a file or an exit status can show
it — `-c`, `+`, `--cmd`, `-S` and `-u` each set an option the run then reports,
`-b`, `-R`, `-m`, `-M` and `-w7` report theirs, `-W` and `-w` write their file,
`-v` leaves Ex mode and so warns that there is no terminal, and `--ttyfail`
exits 1. "It did not complain" is accepted only for `-s`, `-o`, `-O`, `-T`, `-`
and `--`, whose effects need a terminal to see. **Proven able to fail:** against
`slim-vim` 30 of its 52 cases are wrong, and against the `whim-vim` built before
this phase 12 are — every option this phase newly drops that still worked,
counting `-p2` and `-V9`.

`case 'X':` also appears in more than one switch in this file — the normal-mode
tables and `get_c_indent()` have their own — so everything `dropopts.py` does is
bounded by `command_line_scan()`'s own text, and a label it removes from one of
the parser's two switches it removes from the other.

## The delta

Cumulative against slim-vim's baselines: `:helpclose` from phase 1, and now
`:intro` and `:version`, which succeed in slim-vim and report E319 here. Nothing
else may move — and the pty scenarios are the ones to watch, since a startup
screen is exactly the kind of thing a terminal harness records. The command line
is `clicheck.py`'s to check, because nothing else ever passes an option.

Measured: **180,328 → 178,431 lines**, 1,368 of them taken by the sweep in three
rounds, and libc symbols 146 → 146 — the introduction and the command line were
never what the editor needed from the world.
