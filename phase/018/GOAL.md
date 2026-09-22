# Phase 18 — nothing is read at startup, and nothing on the command line decides anything

## nothing is read at startup that was not named on the command line

An editor that goes looking for its own configuration has a filesystem layout in
its head. `source_startup_scripts()` tried, in order:

```
$VIMRUNTIME/evim.vim      $VIMRUNTIME/defaults.vim      $VIM/vimrc
$VIMINIT                  $HOME/.vimrc                  $HOME/.exrc
./.vimrc                  ./.exrc
```

— the last two only with `'exrc'` on, and each guarded by an ownership check,
because reading a config file out of the current directory is a way to be handed
someone else's commands.

All of it goes, **and so does `-u`**. `-u <file>` was the one branch left that
read a file, and `-u NONE` — how every harness kept a vimrc out of a recorded run
— is a no-op once nothing is searched for. With no path to search and no name to
be given, `source_startup_scripts()` has no body, its call goes, and the sweep
takes it; the `-u NONE` test in `main()` that switched `'loadplugins'` off goes
with the option it asked about. `:source` stays until Phase 35.

**It goes here, not later, so that no tool depends on it.** The harnesses are
shared with the slim pipeline, whose editor still searches, so they could not
simply stop passing `-u NONE`. They isolate through the environment instead — an
empty `$HOME`, `$VIM`, `$VIMRUNTIME` and `$XDG_CONFIG_HOME` — which is what `-u
NONE` was for and holds for both editors. Measured against `slim-vim`, with a
real `~/.vimrc` present on the machine: 0 of 67 behaviour cases, 0 of 600 Ex
commands and 0 of 19 terminal rows differ from the baselines recorded with `-u
NONE`, and `tools/verify.sh` is all clear. `tools/clicheck.py` still checks `-u
rc.vim` as an option at Phase 3, where it exists.

`set_init_xdg_rtp()` goes with them. It built a `'runtimepath'` out of
`$XDG_CONFIG_HOME`, and **Phase 1 emptied that option while this was still
filling it back in** — an option reported as empty and rebuilt at startup, which
is the kind of thing only a survey of every `getenv` finds. So does
`process_env()`, which ran `$VIMINIT` or `$EXINIT` as Ex commands, and `'exrc'`,
which selected between two searches that no longer happen.

### The delta

**None, and that is the point rather than a surprise.** No harness passes `-u`,
and under an empty environment none of these paths is taken. What changes is
that the editor no longer needs to be told — and that it can no longer be told.
The phase checks that `-u NONE` is now an unknown option against a control that
still runs.

## command-line options that no longer decide anything

Four outlived what they controlled, each in a different way.

| | why it is inert |
| --- | --- |
| `-y` | evim mode. `parmp->evim_mode` is assigned and read nowhere — its one reader was the line Phase 18 removed |
| `-Z` | restricted mode, whose purpose is to refuse shell commands. `check_restricted()` has two callers left: `do_bang()`, stubbed in Phase 8, and `ex_stop()`. No *live* command carries `EX_RESTRICT` either — the ten that do are all `ex_script_ni` |
| `-t` | jump to a tag at startup, by running `:ta <tag>`. Phase 10 retired `:tag`, so its whole effect is to run a command that reports it is not implemented |
| `-i` | the viminfo file. `'viminfo'` and `'viminfofile'` are wired to `(char_u *)NULL` in **both** editors — the tiny configuration has no viminfo at all |

**`-u` went above**, with the search it used to suppress.

### The harnesses change, and that is the check

All three stop passing `-i NONE`, and `slim-vim` — which still has the option —
must still match its recorded baselines afterwards. It does. That is what proves
the option was a no-op *there* too, rather than only here: `-i NONE` has been
doing nothing for as long as this fork has existed, which is exactly why it was
passed for years without anyone noticing.

`set_init_restricted_mode()` goes with `-Z`, and is a small find of its own: it
read `$SHELL` at startup and turned restricted mode on when the answer was
`nologin` or `false`. An environment read, deciding a mode that restricts
nothing. `EX_RESTRICT` comes out of the twenty-four rows that carry it, because
a flag nothing reads is a concept the table still has and the code does not.

### Two cuts that landed in the wrong place first

Both are the same mistake and both were caught by the compiler rather than by
care. `case 't':` occurs in `get_c_indent()` as well, three thousand lines away
and about `'cinoptions'`, and a substitution with `count=1` takes whichever comes
first *in the file* — the first attempt cut a branch out of the C indenter.
`char_u *tagname;` is also a field of `taggy_T`, seventeen hundred lines
earlier. Everything that edits the option parser is now applied to
`command_line_scan()`'s body alone, and the struct field is anchored on `int
edit_type;`, which sits immediately above it and nowhere else.

### The delta

**None.**
