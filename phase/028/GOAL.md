# Phase 28 — C indenting

`get_c_indent()` was **1,534 lines** and the largest function left: a model of C
syntax built to answer one question, how far to indent this line. It knows about
labels, scope declarations, `case` bodies, continuation lines, comment blocks
and function arguments, and about `'cinoptions'`, a miniature language for
adjusting all of it. With `in_cinkeys()` and the `cin_*` helpers, **3,007 lines**.

`'autoindent'` stays — it is on by default here — and copies the previous line's
indent. That is what an embedded editor needs; the rest is a C compiler's front
end used for whitespace. `'lisp'` and `'indentexpr'` are different indenters and
are not touched.

Five options go, all `PV_BUF`: `'cindent'`, `'cinkeys'`, `'cinoptions'`,
`'cinscopedecls'`, `'cinwords'`.

## It is spelled in more places than it is named

The first cut found five call sites. The sweep found seven more, and each is a
different way of not being a call to `get_c_indent`:

  * `op_reindent(oap, get_c_indent)` — the `=` operator passes the indenter as a
    **function pointer**, so a grep for `get_c_indent(` does not see it. `=` now
    passes `get_indent`, which sets each line's indent to the indent it already
    has: a no-op, the honest answer for a buffer whose language the editor
    cannot read.
  * `preprocs_left()` and `may_do_si()` — `'smartindent'` **defers to**
    `'cindent'` when both are set, so both read `b_p_cin` without touching the
    indenter.
  * `parse_cino()` has **four** callers and none of them indents anything:
    opening a buffer, resizing a window, setting `'shiftwidth'` (some
    `'cinoptions'` are expressed in shiftwidths), and `check_buf_options()`.
  * `cin_is_cinword()`, reached from `'smartindent'`, because `'cinwords'` told
    it which keywords begin a block.
  * insert completion re-indents on accept, through `want_cindent`.

`cindent_on()` is left, returning FALSE. It has seven callers and five only ask
in order to do something else instead; an editor that answers "no, this buffer
is not C-indented" is telling the truth.

## A tool bug this found

`droplocal.py` counted a field's remaining mentions with `text.count(name)` and
no word boundary, so `b_p_cin` appeared to have 23 readers when it had none —
they were `b_p_cink`, `b_p_cino`, `b_p_cinsd` and `b_p_cinw`. Ordering the
arguments around it would have hidden the bug rather than fixed it.

## The awkward part

Insert mode tests for a re-indent in two places hundreds of lines apart, and the
first **jumps into the second**:

```c
if (cindent_on() && ctrl_x_mode_none())        ... goto force_cindent;
...
if (can_cindent && cindent_on() && ...)  { force_cindent: ... }
```

So the two have to go together or not at all — removing the second alone orphans
the label, and removing the first alone leaves a label nothing reaches.

Measured: 142,170 → 138,178 lines, 3,992 removed against 3,007 predicted; the
option plumbing and the `b_ind_*` fields were the difference.
