96 DECLARES NOTHING AT ALL, for phase 92's reason and not phase 95's.  Two `static
FILE *` survive in this editor and NOTHING HAS EVER OPENED EITHER OF THEM IN ANY
BUILD OF zero-vim: `scriptin[NSCRIPT]`, which `-s {scriptfile}` filled and for which
whim removed the option, and `redir_fd`, which `:redir > file` filled and for which
whim removed the command.  `scriptin[]` is assigned in exactly one place in the
whole file and that place sets it to NULL; `redir_fd`'s only assignment is its own
declaration.  So removing them is the difference between code that cannot run and
code that is not there, and a recording that MOVED would mean the cut was wrong.

WHICH MEANS THE DELTA CANNOT BE THE EVIDENCE, and phase/096/check.sh says what is
instead: the source this phase was handed, built twice with
`(void)write(2, "FILESTAR-ENTERED\n", 17);` at FIVE places -- the top of
`closescript()`, inside `inchar()`'s `getc(scriptin[curscript])` loop, inside
`redir_write()`'s `redirecting()` block, inside `undo_cmdmod`'s, and the top of
`vim_fsync()` -- and then with the identical instrument on `ui_write()`, which every
byte the editor draws goes through.  Zero of the 106 records carry the marker from
the five and 105 of the same 106 carry it from `ui_write()`, so the zero is a probe
that can fail.  Eighteen adversarial sessions -- every one of them a way of making
the editor PRINT, which is where `redir_write()` sat -- run on both.

Measured twice over: `diff -rq` of two full recordings, the binary the phase was
handed against the one it made, is EMPTY -- all 102 screen cases, all 111 Ex-command
rows, all 30 command lines, the four pty scenarios and the nineteen terminal rows --
and tools/zerodelta.sh --phase 96 finds the same against whim-vim's frozen baselines.

Four libc symbols go with it, `fclose getc putc fsync`, and after them the core has
no `open`, no `stat`, no stdio stream and no fourth descriptor: it can read, write,
close and dup fds 0, 1 and 2 and nothing else.  `fputs` does NOT go and the check
asserts it STILL undefined: the source names it nowhere and gcc lowers
`fprintf(stderr, "...")` to it.
