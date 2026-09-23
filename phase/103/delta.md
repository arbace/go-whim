103 DECLARES NOTHING AT ALL, and it is the same kind as 101 and 102: the code runs, the
instrument sees it run, and it does the same thing.  Every keystroke now reaches the
core through `musl_read_input()`, every wait is `musl_wait_for_input()`, the terminal
is put in raw mode by `musl_term_start()` and given back by `musl_term_stop()`, and
the editor installs no signal handler of its own -- and two full recordings either
side are byte-identical, 102 screen cases, ref-excmds.txt, ref-argv.txt, ref-pty.txt
and ref-term.txt.

BUT IT IS ALSO PHASE 85'S KIND, and that is the part that matters.  Most of what this
phase changes the instrument CANNOT see: on a pipe `tcgetattr`/`tcsetattr` fail and
change nothing, no recorded case sends a signal, no recorded case resizes a window,
no recorded case types `gs`, and no recorded case reaches end of input with a
terminal on fd 2.  So the phase owes probes, and phase/103/check.sh runs fifteen
of them on both binaries: seven that MUST differ and eight that MUST NOT.

ONE BEHAVIOUR REALLY GOES AND IT IS PROBE-ONLY.  `fill_input_buf`'s `close(0);
vim_ignored = dup(2);` arm reopened the editor's stdin from its stderr when stdin hit
end of file and was not a terminal.  With a TERMINAL on fd 2 the binary this phase
was handed reopens fd 0 and carries on editing -- 2,016 bytes of drawn screen -- and
this one prints `Vim: Finished.` and exits 1, 168 bytes.  It fired 0 times across all
253 recorded rows, which is why the declaration is empty and the probe is the
evidence.  What it buys is GOALS.md II.4b in its strongest form: with `close` and
`dup` gone the core cannot open, close or duplicate any descriptor at all.

THE OTHER SIX MUST-DIFFER PROBES ARE THE PHASE WORKING.  `:set trz?` and `:set
trz=sigwinch` become E518, 'termresize' being the one options[] row this phase takes.
`ESC [ 48;30;100 t` was buffer text and now resizes the editor, because the mode-2048
notification arm is unconditional -- the negotiation that guarded it could never run.
`ESC [ ? 1 z` was nothing and now runs `:stop`, which is the private sequence the
host sends when it catches SIGTSTP.  CTRL-C with stdout a pipe ran `:qa` (E492) and
now draws the message, `stdout_isatty` having folded to TRUE.  And an external
`kill -TSTP` draws a different number of bytes, which is what says it took the
in-band path rather than the core's own `got_tstp` flag.
