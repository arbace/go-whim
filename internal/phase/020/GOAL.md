# Phase 20 — nothing outside the process is consulted

## there is no home directory

`$HOME` is where an editor keeps the things it was told not to keep. This fork
stopped writing them in Phase 11 and stopped looking for them in Phase 18, and
what was left is the *notion* of a home directory — `~/x` meaning a path, `~bob`
meaning someone else's, and `/home/you/x` displayed back as `~/x`.

All three go, and the last is why this is not only a `getenv` removal:
`home_replace()` has **thirteen callers**, every one a place that shows the user
a file name. It becomes a bounded copy, so the thirteen keep working and a name
is shown as what it is.

The user database goes with them — `init_users()`, `add_user()`, `match_user()`
and `get_users()` exist so that `~bob` can complete, and `mch_get_uname()` so
that a swap file could say who wrote it. Two smaller things fall out and had to
be taken by hand, because `-Wunused-but-set-variable` is not a shape the sweep
deletes: `at_start`, which existed only to know whether a `~` began a path, and
`startstr_len`, measured for the one test that used it.

### Where the symbol count moves

`getpwnam`, `getpwent`, `setpwent`, `endpwent` — 119 → 115. CLAUDE.md notes that
`getpwnam()` working under static musl is one of the two things that make this
binary honestly standalone. It no longer needs it.

### Two corrections to what this phase was planned to do

**`getuid` and `getgid` do not go.** `buf_write()` uses them to check ownership
before overwriting a read-only file and to preserve owner and group. That is
file writing, which whim-vim keeps, and the plan was wrong to list them here.

**`getpwuid` does not go either.** `mch_get_uname()` is still reached from
`swapfile_info()`, under `-r`, which lists swap files that cannot exist — Phase
13 removed the swap file and left the option that reads them. That wants a phase
of its own rather than a corner of this one: `ml_recover()` alone is 559 lines,
`recover_names()` 216 and `swapfile_info()` 103.

### The delta

**None the harness records.** `:e ~/notes` opens a file called `~/notes` in the
current directory, which no harness asks for.

`--term-moved` is **cumulative**, like the command list — the comparison is
always against the slim baseline, and Phase 19 collapsed that table for good, so
every phase after it declares the same thing. Discovered by this phase failing
when it did not.

## nothing is read from the environment

The third and last of the standalone phases. Phase 18 stopped reading
configuration files, Phase 20 stopped believing in a home directory, and this
one removes the environment itself — after it, no answer this editor gives
depends on how it was invoked.

**`vim_getenv()` had already been half dead, and that is what makes this
phase small.** Phase 1 folded its `vimruntime` flag to FALSE, so
`vim_getenv("VIMRUNTIME")` had been returning NULL unconditionally ever since,
and `"VIM"` was the only name left that could reach the `$VIM`/`'helpfile'`
fallback chain — which nothing asks for any more. So the function **can only
ever answer "not set"**, and every caller collapses to the branch it was
already taking:

| what it read | what took its place |
| --- | --- |
| `$VAR` in a file name (`expand_env_esc`) | the name, as written |
| `$PATH` (`expand_shellcmd`) | the pattern's own directory |
| `$VIMRUNTIME` (`fix_help_buffer`) | the `*local-additions*` scan, 111 lines, already a no-op |
| `$SHELL`, `$CDPATH`, `$VIM_POSIX` | the compiled-in defaults |
| `$TMPDIR`, `$TEMP`, `$TMP` | `/tmp`, which was always in the list |
| `$COLORFGBG` | what Phase 19 decided the terminal is |
| `$TZ` | `localtime_r`, which does the zone setup itself |
| `$VIM`, `$VIMRUNTIME`, `$MYVIMDIR`, written | nothing writes them |
| `environ`, walked for `$VAR` completion | the row and its `$`-prefix context go, as `~user`'s did |

`expand_env_esc` is the same answer Phase 20 gave `home_replace`: with the `$`
arm gone what remains is `skipwhite`, the backslash escape and the bound on
`dstlen`, and a name reaches its caller intact.

**`vimrc_found()` was already unreachable**, and finding that out is what kept
this phase from being an argument about whether `$VIM` should still be
published. Every `do_source()` call in the file passes `DOSO_NONE`, so the two
arms that called it have been dead since Phase 18. Deleting them takes
`vim_setenv`, `export_myvimdir` and `$MYVIMDIR` with them.

### The check is the object, not the source

`getenv`, `setenv`, `unsetenv` and `environ` leave `nm -u`: **115 → 110**, the
fifth being `tzset`. Grepping the source is not sufficient and the phase does
both — the sweep is what removes `vim_getenv`, so asking before it runs gets
the wrong answer, which this pipeline has now learned four times.

### What stays

`vim_localtime()` still calls `localtime_r()`, and musl reads `$TZ` inside it.
The rule this phase enforces is that *this source* asks the environment
nothing; making a file's timestamp display in UTC would be a different
decision, and not this one.

### The delta

**None the harness records.** `:w $FOO.txt` writes a file called `$FOO.txt`,
`:e $HOME/notes.txt` needs a directory literally named `$HOME`, and
`:set shell?` says `sh` whatever `$SHELL` was — verified by hand, none of it
something a harness asks for.
