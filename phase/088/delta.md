88 leaves the command line as `+{command}` and `-T {term}`.  Three ways of naming
something to edit go -- a file argument, a bare `-` and the `--` that made every
word after it a file name -- and each becomes what every other unknown word
already was, mainerr(ME_UNKNOWN_OPTION).  Six of the 30 command lines move and
every one of them names a file or a stream:

  `-` was the one row that BLOCKED: the editor read the keystroke file as buffer
  text, closed fd 0 and waited on fd 2 for keys that never came (tools/zargv.py
  says so).  It is now `Unknown option argument: "-"`, exit 1, and the row costs
  the timeout no longer.
  `f.txt` and `+q! f.txt` opened a buffer and drew a screen; both are exit 1 with
  an empty stream now.
  `--` and `-- +q!` ended the options, so `+q!` after `--` was a FILE NAME and not
  a command -- which is why the two rows differ from each other in the baselines
  and agree now.
  `f.txt g.txt` is declared although `stderr-moved` would absorb it: it exited 1
  with an empty stream before, for `Too many edit arguments: "g.txt"`, and exits 1
  with an empty stream now for `Unknown option argument: "f.txt"`.  It is the
  two-file-argument row and this is the phase that removes file arguments, so the
  list says so rather than letting a dimension cover it.

Nothing else moves, and the screen corpus is the reason it cannot: every case
seeds itself by TYPING under `'paste'` (GOALS.md II.2d), so not one of the 102
passes a file argument.  Measured: all 102 cases, all 111 Ex-command rows, the
four pty scenarios and the terminal table are identical, as are the other 24
command lines -- including every `+{command}` form and all three `-T` spellings.
```
argv:- argv:-- argv:f.txt argv:f.txt_g.txt argv:+q!_f.txt argv:--_+q!
```
