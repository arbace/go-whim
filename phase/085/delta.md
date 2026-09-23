85 removes the two warnings check_tty() printed when stdin or stdout was not a
terminal, the two seconds it then paused, and --ttyfail.  IT DECLARED NOTHING
UNTIL PHASE 86, and that was true of the old instrument rather than of the
editor: behaviour.py and exsweep.py ran it `-e -s`, where exmode_active is set
before check_tty() and its other branch is taken, so no recorded stderr ever
held the warnings, and termcheck.py drives a pty where both streams ARE
terminals.  The instrument phase 86 installs runs the editor on a pipe, which is
exactly where those warnings were printed, so the difference is visible now and
is declared where it happened: every one of the 102 cases, 109 of the 111 command
rows (:stop and :suspend are skipped and run nothing) and 13 of the 30 command
lines differ in their stderr and in nothing else -- measured with
tools/zcompare.py, which is what "and in nothing else" means here.
```
stderr-moved
```
