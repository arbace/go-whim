100 DECLARES NOTHING AT ALL, and it is phase 96's kind rather than phase 92's.  Phase 92
removed code that had BECOME unreachable -- `readfile()` was live at the start of this
pipeline and phases 88 to 91 took every way to reach it.  This phase, like 13, removes a
POSSIBILITY THAT HAS NEVER EXISTED IN ANY BUILD OF zero-vim: `deathtrap()`'s
`entered >= 3` ladder, which is `reset_signals()`, `_exit(8)` and `exit(7)`.

TWO FACTS MAKE IT UNREACHABLE AND NEITHER IS PART II'S DOING.  `catch_signals()` installs
the deadly handler with `sigemptyset(&sa.sa_mask)` and `sa.sa_flags = 0` -- no
`SA_NODEFER` -- so a deadly signal is blocked for the duration of its own handler; and
`signal_info[]` carries exactly TWO `deadly = TRUE` rows, SIGHUP and SIGTERM, because
whim removed SIGSEGV, SIGBUS, SIGILL and SIGFPE and all four are at zero mentions.  Two
signals, each blocked in its own handler, lets `entered` reach 2 -- the
`Vim: Double signal, exiting` arm, which calls getout(1) and never returns -- and no
further.

WHERE PHASE 96'S ARGUMENT WAS TEXTUAL, THIS ONE IS ABOUT SIGNAL MASKS, so it is
measured rather than read.  phase/100/check.sh instruments the source this phase was
HANDED with `write(2, "DTn\n", 4)` after `++entered;` and `write(2, "DTLADDER\n", 9)`
inside the ladder, and builds it five ways.  Bombarded -- eight concurrent sessions,
sixty alternating SIGTERM/SIGHUP each at full speed -- the maximum `entered` ever
observed is 2 and DTLADDER never appears.  Forced -- a `raise()` inside
`preserve_exit()` and another in the `entered == 2` arm -- it stops at 2 and exits 1.
THE SAME SOURCE WITH ONE FIELD CHANGED, `sa.sa_flags = SA_NODEFER`, reaches depth 3,
enters the ladder and EXITS 7, and one forced signal further EXITS 8: both statements
this phase deletes are live code that only the signal mask keeps out of reach.  That
pair is the probe proving it can fail.

AND THE DEADLY SIGNALS THEMSELVES DO NOT MOVE.  A single SIGTERM and a single SIGHUP
give the binary this phase was handed and the one it made the same exit status and the
same stdout and stderr byte for byte, with `Vim: Caught deadly signal TERM`/`HUP` and
`Vim: Finished.` required to be in what was drawn -- so the equality is not two empty
streams agreeing.  tools/zerodelta.sh --phase 100 finds the corpus unmoved, as it must.

`_exit` goes with the nine lines and `exit` does NOT: `mch_exit`'s `exit(r);` is the
one call left in the file, and demoting it is a later phase's.  The word `exit` is
useless as a source assertion -- two string literals, a `goto exit;` and its `exit:`
label in vim_regsub_both() carry four of its five remaining mentions -- so what this
phase asserts is `nm -u`.
