# Phase 1 — no `$VIMRUNTIME`

**Its first step is the command line** (`argvfront`, the pipeline reform's
first drop package, `doc/PIPELINE-REFORM.md` §7): `command_line_scan()` is cut
on the seed to what the product accepts, `+{command}` and nothing else. So are
the calls of `parse_command_name()` and `early_arg_scan()`, main's `--clean`
prescan, and the error enumerators with `main_errors[]`'s rows. Thirteen phases
had cut it an option or two at a time.

**What the cut leaves unwritten falls out** (`crefactor/xform`'s `FallOutOf`).
These are the 12 objects and members that main reached writes of before the
cut and none after: `has_dash_c_arg` and 11 members of `mparm_T`
(`clean`, `evim_mode`, `use_vimrc`, `not_a_term`, `tty_fail`, `edit_type`,
`pre_commands`, …). Each read of them is its value. The closure follows
only what that makes constant:
- the `if`s it decides;
- `is_not_a_term()`, which returns a constant, so its calls go;
- `set_init_1()`'s parameter, which every call passes as one constant;
- `exe_pre_commands()`, left with nothing to do, so its call goes;
- what then follows: `read_stdin()`, `set_init_clean_rtp()` and the evim
  script.

`read_cmd_fd` is held: the product keeps it. These folds were made by hand
in phases 3, 18, 21, 43, 85 and 88, and the shapes come out the same: the
product is byte for byte what it was.

**Then every Ex command the product has not is retired** (`exfront`, the
reform's second drop package, D2). Its rows point at `ex_ni`, as 21 `retire`
steps and four programs in phases 1-79 did a few at a time. The 489 names are
declared in this phase's `delta.md`; 271 are stubs in the seed already, and 218
are retired here. The handlers then have no row, and the sweep takes them. That
also takes every edit phases 9 to 96 made inside them, the reverse constraint of
`doc/PIPELINE-REFORM.md` §3. What only those handlers wrote falls out the same
way as for the command line: 21 objects, among them the `:sort` state,
`redir_fd` and the `filetype_*` flags.

**Then the rows go** (`extable`, D2b). This was phase 80's first half, done
on the seed:
- the 489 stub rows are deleted from `cmdnames[]`;
- each of the 111 left carries its shortest abbreviation, computed from the
  600-row table and proved over all 2,538 prefixes of the 600 names;
- the prefix index, `command_count` and its check go, and the lookup is a scan;
- the one-character commands are cut to those that exist.

The stub rows' enumerators stay, moved after `CMD_SIZE`, because live code
still names some of them: `window_layout_locked(CMD_close)`, the comparisons
later phases fold. No parsed command can reach them. Phase 80 deletes what is
left of them.

**Then the commands that name a file go** (`filefront`, the reform's D4): `:edit`,
`:enew`, `:ex`, `:exit`, `:file`, `:read`, `:saveas`, `:update`, `:visual`, `:view`,
`:write`, `:wq` and `:xit`. These are the 13 of the 111 rows left that the product
has not, which phases 89, 90, 91 and 93 deleted. Their enumerators go after
`CMD_SIZE` like the stubs', and live code still names some of them until those
phases take the last uses. Their handlers die with the rows, and with them the
write path (`buf_write()`), `:read`'s and `:edit`'s, and `do_bang()`'s filters.
So does every edit phases 8 to 93 made inside them.

**Then `:q` stops refusing** (`quitfront`, phase 94's move): `ex_quit()`'s
test — `check_changed()` on the buffer, `check_more()`, `check_changed_any()` —
folds never, so `:q` takes its else arm and quits whatever was changed. That
was phase 94's one fold. With it go `check_changed()`, `check_changed_any()`
and the switch-buffer/switch-window island its tail was the last caller of
(`set_curbuf()`, `enter_buffer()`, `win_enter_ext()`, `get_winopts()` and
seven more), and every edit phases 62 to 93 made inside them.

**Then nothing reads a byte** (`readfront`, phase 92's move):
`open_buffer()`'s two read arms, the named file and stdin, fold never, so a
buffer opens empty. `readfile()` (787 lines), `read_buffer()`,
`fix_help_buffer()` and the swap-file check go with them once their other
callers have, and so does every edit phases 20 to 91 made inside them:
`lfonly`'s formats, `keepbytes`' `++bad`, `noconv`'s conversions, and
phases 62, 70, 75 and 87's folds.

**Then a command line is one command** (`onecmdfront`, phase 81's move):
`|` and `"` stop being syntax, so neither separates a command nor starts a
comment, and a newline still ends one. These are phase 81's edits on the
seed's spelling: `separate_nextcmd()` splits at a newline only,
`ends_excmd()` and `ends_excmd2()` answer the end of the line (their Vim9
`#` goes with the rest), and `find_nextcmd()`, `check_nextcmd()`,
`do_one_cmd()`, `ex_range_without_command()`'s `:|`, `:substitute`'s tail
and `:append`'s `:a|text` stop knowing either character.

**Then every option the product has not is dropped** (`optfront`, the
reform's D3). There are 375 rows, listed in `internal/cut/optfront.md`, where
53 `dropoptions` steps and phases 54 and 95 dropped them a few at a time:
- each row is found inside `options[]`;
- its name's line goes from every list that held it. That is how
  `dropoptions` always worked, and the product keeps what it did to other
  options' value lists.

No guard is asked: a global its row initialised is zero from here on. **What
that makes constant falls out** (D3b): 135 objects nothing writes after the
cut, 113 of them folded. The 22 held are left to the phases that fold them by
hand with a shape of their own, or with the option's real default.

**Then there is no swap file and nothing to recover** (`noswap`, `norecover`
and `nomemfile`, the reform's D5): phase 11's and phase 21's cuts, run on the
seed after the options are dropped. The memfile is memory, nothing is written
that was not asked for, and `ml_recover()` goes. With it go `readfile()`'s
last callers but phase 13's, so `readfile()` dies at phase 13, and
`edit_type`, `mf_dont_release` and the swap file's timestamps fall out here.

**The first ground truth: there is no runtime directory.** Nothing is installed
beside the binary, so every path that goes looking for one is dead weight and,
worse, a promise the editor cannot keep — `:help` that opens nothing is more
confusing than `:help` that says it is not implemented.

Four entry points are cut, and everything unreachable behind them is *found*
rather than listed:

- **Six command rows point at `ex_ni`**: `:help`, `:helpclose`, `:helptags`,
  `:runtime`, `:exusage`, `:viusage`. All six exist only to read or display
  files from the runtime directory.
- **`'helpfile'` and `'runtimepath'` default to `""`**, in both halves of the
  `{vi, vim}` pair. They named `$VIMRUNTIME/doc/help.txt` and a five-element
  path through `~/.vim` and `$VIM/vimfiles`.
- **The `VIMRUNTIME` branches of `vim_getenv()` and `vim_setenv()` go.** That
  is the layer that *derives* a runtime directory from the executable's own
  path when the variable is unset, which is precisely the behaviour an embedded
  binary must not have.
- **The `help.c` region** — 981 lines, 13 functions — is then unreachable and
  the sweep removes it, along with whatever else it was the only caller of.

**The delta this is allowed to cause**, and nothing else: the six commands
report `E319` instead of acting, and `:set helpfile? runtimepath?` report
empty. Every other behaviour case, every other Ex command, every pty scenario
and the whole terminal table are unchanged. The harness checks exactly that
against `slim-vim`'s recorded baselines, and then records `whim-vim`'s own.

## The trap

`:help` is not the only way in. `'helpfile'` is read by anything that opens
help, `$VIMRUNTIME` is consulted by the vimrc search, and `:runtime` is what
`:packadd` was built on. Cutting the commands without cutting the option
defaults leaves an editor that still tries to open a file it will never find —
which is why the option defaults are part of *this* phase and not a later one.
