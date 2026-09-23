93 takes the buffer's NAME: the one command that could still set it, `:file`, the
three char_u* fields on every buffer, and the sixteen places that asked what the
name was.  ONE SCREEN CASE AND ONE COMMAND ROW MOVE, and they are the two kinds:

  `cmd_file` types `:file` with no argument, so what the baselines hold is the
  CTRL-G line, `"[No Name]" [Modified] 1 line --100%--` -- an editor that answered
  with the buffer's name.  It is `E492: Not an editor command: file` now: the same
  exit, the same bells and the same snapshot count, with the stream 2,310 -> 2,327
  bytes.
  the ref-excmds.txt row `file` does not change message: it CEASES TO EXIST,
  because tools/zexcmds.py enumerates the table and there are 98 names where there
  were 99.

NOTHING ELSE CAN MOVE, and the reason is stronger than a measurement: every fold
this phase makes takes the branch the code already took at run time.  `b_ffname`,
`b_sfname` and `b_fname` have been NULL since phase 88 took the file argument and
`:file` was the last thing that could set one, so `== NULL` was already TRUE at
every site and `!= NULL` already FALSE.  `[No Name]` survives as the ONLY answer
buf_spname() can give rather than as one of two -- which is why CTRL-G, `g CTRL-G`,
`:registers`, the `%` and `#` registers and the status line are byte-identical
either side, and why `:registers` never printed its `"%` and `"#` lines to begin
with.

Measured with tools/zcompare.py: the other 101 screen cases, the other 97 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical -- `:filter` and `:fixdel` among them, which is the inheritance check a
removed name asks for, and `:q` on a modified buffer (still E37).
```
case:cmd_file
file
```
