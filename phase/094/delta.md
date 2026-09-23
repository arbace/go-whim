94 removes the last refusal: `:q` on a modified buffer answered `E37: No write
since last change (add ! to override)` and stayed.  ONE SCREEN CASE AND ONE
COMMAND ROW MOVE, and the second is a THIRD kind of movement, which no Part II phase
has had before:

  `quit_modified` types text and then `:q`, so what the baselines hold is an
  editor that REFUSED.  The E37 line is gone, the one bell with it, and the record
  loses a snapshot -- 2,342 -> 2,213 bytes of stream.  The exit status does NOT
  move, and that is the corpus's limit rather than the phase's: every zcases.py
  case ends with a trailing `:q!`, which quits the old binary too.  The status is
  what phase/094/check.sh's `q_alone` probe is for -- `:q` with NOTHING after
  it, where the old binary draws E37, runs out of stdin, prints `Vim: Finished.`
  and exits 1, and this one quits and exits 0.
  the ref-excmds.txt row `quit` CHANGES MESSAGE AND DOES NOT CEASE TO EXIST,
  unlike every row phases 89 to 93 declared: `:quit` is still a command and still
  has its row, so tools/zexcmds.py enumerates the same 98 names and compares the
  block.  Its `msgs` lose the E37 line -- `:set nopaste / E37... / :q!` becomes
  `:set nopaste / :quit`.  The `cquit` row does not move.

NOTHING ELSE CAN MOVE.  The fold takes the branch the code took whenever the
buffer was unmodified, and `:q!` took it already; `ZZ` and `ZQ` have run
`do_cmdline_cmd("q!")` since phase 89, so all four spellings were already one
thing for an unmodified buffer and are one thing for every buffer now.  The
buffer still KNOWS it is modified -- CTRL-G still prints `[Modified]`, the status
line still draws `[+]`, `:set modified?` still answers -- so what went is the
refusal and not the state.

Measured with tools/zcompare.py: the other 101 screen cases, the other 97 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical -- `:cquit` and `:quit`'s neighbours among them.
```
case:quit_modified
quit
```
