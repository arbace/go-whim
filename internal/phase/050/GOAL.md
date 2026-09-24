# Phase 50 — only LF text files

Every line ends with LF when it is read and when it is written, and a CR is a
character like any other. `-b` goes, and with it `'binary'`, `'fileformat'`,
`'fileformats'`, `'endofline'`, `'fixendofline'`, `'endoffile'`, and the old
spellings `'textmode'` and `'textauto'`; so do the `++bin`, `++nobin`, `++ff` and
`++fileformat` arguments. `tools/lfonly.py` removes what a row cannot:

- **`readfile()` stops choosing and detecting a format.** The choice from `++ff`,
  `'binary'` and `'fileformats'`, the DOS and Mac detection, the loop that split
  lines at CR, CR stripping and its retry as Unix, the CTRL-Z at the end of a DOS
  file and the "[CR missing]" message all fold. A last line with no LF is still
  read and still reported as "[noeol]" — that describes the file.
- **`buf_write()` writes LF after every line, the last included, and no CTRL-Z.**
  Its CR branch goes by a local helper that keeps an `if`'s body and drops its
  `else`, which `cutil.fold_always` rightly refuses to guess.
- **Everything that compared a buffer's format with the one it was read in** —
  `file_ff_differs()`, `save_file_ff()`, `set_file_options()` — has nothing to
  compare, so its callers fold, including `unchanged()`, `bufIsChangedNotTerm()`,
  `set_init_1()` and `did_set_modified()`, and the sweep takes it with
  `get_fileformat()`, `set_fileformat()`, `default_fileformat()`,
  `msg_add_fileformat()` and `set_options_bin()`. stdin and fifos stop being read
  as binary.
- **`'endofline'` and `'endoffile'` had no initialiser in `buf_copy_options()`** —
  only resets, which the cut removed — so `tools/droplocal.py` does not recognise
  their shape, and their fields go by hand. So does `b_no_eol_lnum`, the last-line
  marker a binary write used.

**Several first runs failed, each on a count.** Two `else if (curbuf->b_p_bin)` in
`readfile()` until the format chain folded first; the detection block's
`fileformat == -1` test repeated inside itself, now anchored on what follows it;
two `save_file_ff()` calls outside the functions that die, found once the final
check reported *where* each leftover call sits rather than comparing a total.

The phase checks that `-b` is unknown and `:set ff=dos` refused against a
`:set ts=3` control; that `:%s/$/X/` on a CR LF file writes `one\rX\n`, where a
DOS file gave `oneX\r\n`; and that a last line with no LF gains one.

## The delta

**No Ex command; the behaviour cases `ff_dos` and `binary_mode`**, whose
`:set ff=dos` and `:set binary` are refused. Measured: 115,246 → **114,399
lines**.
