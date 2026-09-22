# Phase 31 — file-name modifiers

`eval_vars()` expands `%` and `#` into the current and alternate file names, and
`<cword>`, `<afile>` and the rest. **That stays** — `:w %` and `:e #` are how a
file name is written without typing it.

What goes is the **suffix language** that may follow: `modify_fname()`, 426
lines implementing `:p` (full path), `:h` (head), `:t` (tail), `:r` (root),
`:e` (extension), `:s/from/to/`, `:gs`, `:~` and `:.`, applied left to right so
that `%:p:h:t` means something. It is a small programming language over path
strings, and **most of it asks questions this editor can no longer answer**:

| modifier | what it needed | which phase took it |
| --- | --- | --- |
| `:p` | where the working directory is | 24 — there is one answer now |
| `:~` | the notion of `$HOME` | 22 — nothing outside the process |
| `:s//` | a regexp over a file name | the only place a pattern is applied to something that is not buffer text |

**One caller**, which is why the cut is small: `eval_vars()` reaches it once, in
the arm that runs when the next character is not `<`. That arm goes, and with
it `tilde_file` and `skip_mod`, which existed only to be passed to it. The `<`
arm — which strips one extension and is not part of the modifier language —
stays. After this a modifier is left in the command line as the literal
characters it is written with, which is what an editor that does not know the
syntax does.

**The delta is none, so the phase checks both halves itself**, and only the
pair is a check: `:w %` must still write the file being edited, and `%:t` must
stop being a tail. One without the other passes on a `eval_vars()` that returns
NULL for everything.

Measured: 135,941 → 135,315 lines.
