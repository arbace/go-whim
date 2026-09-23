102 DECLARES NOTHING AT ALL, and it is 101's kind: the phase moves no code out and
changes no message.  `mch_exit()`'s last statement stops being `exit(r);` and becomes
`vim_host_exit(r);`, a call through a function pointer the launcher installs; the
launcher lands on `__builtin_setjmp` and RETURNS the status instead of the process
ending inside the editor.  Everything mch_exit does before that line -- the terminal
restored, the screen scrolled, the memfile closed -- is unchanged, so what the editor
DRAWS on its way out cannot move, and the recording is of what the editor draws.

What CAN move is the exit status, and that is measured rather than declared: six
routes on both binaries, `:q!` 0, `:cq 3` 3, EOF 1, a bad option 1, SIGTERM 1,
SIGHUP 1, with the output built again with `host_code = r;` made `host_code = r + 1;`
required to move all six.  A status is not a dimension of this instrument -- zcases
and zexcmds record it per case and zerodelta compares it -- so an unchanged status is
part of "nothing moved" and the probe table is what makes it legible.

101 and 102 are the first two phases in this pipeline whose empty declaration means
NEITHER "nothing ran" NOR "the instrument cannot see it": the code runs, the
instrument sees it, and it does the same thing.
