# Phase 29 — `:command`, user-defined commands

`:command` lets a user give a name to an Ex command line and have it dispatched
like a built-in. The machinery is **1,451 lines**: a parser for the `-nargs`,
`-range`, `-complete` and `-bang` attributes; a per-buffer and a global growable
array of definitions; `uc_check_code()`, 286 lines, expanding `<args>`,
`<q-args>`, `<line1>`, `<count>`, `<bang>`, `<reg>` and `<mods>`; and a listing
mode.

**Without `+eval` a user command can only invoke built-in commands**, which makes
it a way of writing an alias — and this editor reads no vimrc, so the only way to
define one is to type `:command` by hand in the session where it is used.

What goes beyond the three commands: `do_ucmd()`, which `do_one_cmd()` reaches
when `ea.cmdidx` is negative — the marker for "this name is not in `cmdnames[]`,
try the user table" — so an unknown name is now simply not a command;
`find_ucmd()`'s two callers; **six rows of the completion table**, which kept six
`get_user_cmd_*` functions alive and which no grep for `do_ucmd` would find,
because a table row is a reference the same as a call; the walk past the end of
`cmdnames[]` in `expand_user_command_name()`; and the `b_ucmds` field.

## The delta is two names, not the three retired

`:command` with no arguments lists what is defined, and `:comclear` clears it:
both succeed today, so both move in the Ex sweep. **`:delcommand` does not** — it
is `EX_NEEDARG`, so the sweep's bare call already failed. Declaring three and
being told two is the check working, and it is Rule 3's other half: retiring a
command only shows in the sweep if it used to succeed.
