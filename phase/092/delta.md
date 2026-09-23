92 DECLARES NOTHING AT ALL, and an empty declaration is a statement here rather
than an omission.  This phase removes the file-reading path -- `readfile()`, 787
lines, `read_buffer()`, the four functions of the message layer that reported what
had been read, and eleven more the sweep finds under them -- and it declares
nothing because THE PATH STOPPED BEING REACHABLE AT PHASE 91.  Phase 88 took the file
argument and the bare `-`, phases 89, 90 and 91 took every command that could name a
file, and after them `open_buffer()`'s two arms need either `b_ffname != NULL` or a
`read_stdin` argument that all four callers pass as FALSE.  gcc kept the code
because it cannot prove the first never holds; nothing the editor can be given
reaches it.  So the difference this phase makes is between code that cannot run and
code that is not there, and a recording that MOVED would mean the cut was wrong.

WHICH MEANS THE DELTA CANNOT BE THE EVIDENCE, and phase/092/check.sh says what is
instead: the source this phase was handed, built twice with
`(void)write(2, "READFILE-ENTERED\n", 17);` as the first statement of `readfile()`
and then of `open_buffer()`, and recorded with tools/zrecord.sh.  Zero of the 106
records carry the marker from `readfile()` and 104 of the same 106 carry it from
`open_buffer()` through the identical instrument -- so the zero is a probe that can
fail, and it is the whole of what this phase can prove.  Eight adversarial sessions
that name the buffer after a real file run on both.

Measured twice over: `diff -rq` of two full recordings, the binary the phase was
handed against the one it made, is EMPTY -- all 102 screen cases, all 111 Ex-command
rows, all 30 command lines, the four pty scenarios and the nineteen terminal rows --
and tools/zerodelta.sh --phase 92 finds the same against whim-vim's frozen baselines.
