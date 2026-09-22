# Phase 69 — one file argument, and no argument list

**The order is the opposite of the obvious one, and the first attempt at this
phase proved why.** That attempt imposed buffer reuse inside `buflist_new()` and
deleted the argument-list call that reaches it — and that call is **the only thing
that names the first buffer**. `open_buffer()` reads through
`readfile(curbuf->b_ffname, …)`, so with no name it read nothing: the buffer came
up empty, every edit was a silent no-op, and `:wq` wrote the original bytes back.
It compiled cleanly and passed two of its three probes. It was dropped whole.

So this phase limits the command line **first** and leaves the naming path exactly
as it is:

- **One file argument.** A second non-option argument is
  `mainerr(ME_TOO_MANY_ARGS)`, which is what vim already answers for a second `-`.
  One file means one entry, which is what makes the list pointless rather than
  merely unused.
- **The name still goes through `buflist_add()`.** `curbuf` exists and is unnamed
  and empty when `command_line_scan()` runs — `main()` calls `common_init_2()`,
  which calls `win_alloc_first()`, before the scan — so `buflist_new()` reuses it
  and sets `b_ffname`, exactly as before. Only the *list* around that call goes.
- **The argument list.** `:next` and `:previous` point at `ex_ni`; the other 21
  argument commands already did. Gone with them: `ex_next`, `ex_previous`,
  `do_argfile`, `do_arglist`, `arglist_del_files`, `alist_set`, `alist_clear`,
  `alist_add`, `alist_add_list`, `alist_check_arg_idx`, `alist_name`,
  `check_arg_idx`, `editing_arg_idx`, `arg_all`, `check_arglist_locked`,
  `arg_had_last`, `global_alist`, `alist_T`, `aentry_T`, `w_alist`, `w_arg_idx`,
  `w_arg_idx_invalid` and `mparm_T.fname`.

**What folds because the count is always one**: `check_more()`, whose "N more
files to edit" refusal can never fire; `append_arg_number()`, the `(N of M)`
suffix; `##` in a file-name modifier, which had every argument to expand and now
has none; and the seven `ADDR_ARGUMENTS` arms of Ex range parsing.

**`ADDR_ARGUMENTS`'s labels stay, and its bodies go.** Deleting the labels earns
seven *"enumeration value not handled in switch"* warnings — those switches
enumerate `ADDR_*` exhaustively — which is what the sweep kept reporting as "left
alone 7" while never converging. Each arm gets a constant body instead.

## The delta

**None, and that was measured rather than assumed.** `:next` and `:previous` were
declared as moving and did not. An `exsweep` row is `exit= left= err=`, and with
one file argument `do_argfile()` already answered *"there is only one file to
edit"* — so pointing the rows at `ex_ni` changes the message text, which the sweep
does not record, while the exit status, the files touched and stderr all stay the
same. The declaration was **narrowed** to match the measurement; widening one to
fit is what `whimdelta.sh` exists to refuse.

The probes are **load-first**: `+$` then `+s/^/LAST /` proves the buffer holds the
file's lines, which is the check the abandoned attempt lacked and needed. Then
plain editing, a second file argument refused without writing either file, `:next`
refused, and `:e` still opening a second file.

Measured: 94,122 → **93,393 lines**.
