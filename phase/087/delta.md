87 removes Ex mode, silent mode and the four options that reach them.  Six records
move and every one of them is a way IN to Ex mode: the two keys, and the three
command lines the parser accepted for it.  `key_Q` and `key_gQ` drew "Entering Ex
mode.  Type \"visual\" to go to Normal mode." and now beep once, from nv_error and
from nv_g_cmd's default; `-e`, `-E` and `-v` are mainerr(ME_UNKNOWN_OPTION) like
any other unknown letter, and `-e -s` with them, because `-s` was only ever read
after `-e` had been.

`-s` ALONE IS NOT DECLARED, and the plan's P3 row over-declares it: `case 's'`
set silent mode only `if (exmode_active)` and called mainerr() otherwise, so a
bare `-s` was an unknown option before this phase and is the same unknown option
after it.  Measured: its record is byte-identical.  Nothing else moves -- not the
other 26 command lines, not the 111 Ex-command rows, not the four pty scenarios,
not the terminal table, and not one of the other 100 screen cases.

TWO OF THE FOUR WOULD BE ABSORBED BY `stderr-moved` and are named anyway.  `-e`
and `-e -s` exited 1 with an empty stream before this phase and exit 1 with an
empty stream after it; only their stderr moved, which the line above already
excuses everywhere.  They are ways INTO Ex mode and this is the phase that closes
them, so the list says so rather than letting a dimension cover it -- and if a
later phase ever drops `stderr-moved`, these two are already declared.
```
case:key_Q case:key_gQ
argv:-e argv:-E argv:-e_-s argv:-v
```
