# Phase 64 — no formatting, comment or nroff-macro options

Five options, each dropped with the machinery that only it gave a meaning to.

- **`'comments'`** — no comment leader is recognised. `get_leader_len()` and
  `get_last_leader_offset()` would answer 0 and -1 everywhere, so every reader
  folds that way. The following all go:
  - `open_line()` copying, replacing, right-aligning and padding a leader;
  - `insertchar()` completing a `*/`, through `end_comment_pending`;
  - `J` removing leaders, through `skip_comment()`;
  - `same_leader()`, and the leader a formatted or wrapped line keeps;
  - `gd` skipping comment lines;
  - `%` skipping a `//` comment when `buf_has_cstyle_comments()` said the buffer
    looked like C.

  `check_linecomment()` stays, because `findmatchlimit()` still uses it.
- **`'formatoptions'`** — fixed at its default, `tcq`. With no leader, `c` and `q`
  have nothing to act on, so what is left is `t`:
  - Typing still wraps at `'textwidth'`, and `'paste'` still stops it, since
    `has_format_option()` answered FALSE under paste.
  - `gq` still formats.

  Every other flag was off, and its code goes:
  - `a`: `auto_format()`, `check_auto_format()`, `did_add_space` and all 18
    calls;
  - `w`, `n`, `2`, `b`, `l`, `v`, `m`, `M`, `B`, `1`, `p`, `]`, `j`, `r`, `o` and
    `/`.

  `format_lines()` is left as a plain paragraph loop with no second-line indent.
  `internal_format()` breaks only at blanks, which removes the multibyte branch,
  and `Insstart_textlen` and `Insstart_blank_vcol` go too.
- **`'formatlistpat'`** — only `n` read it, through `get_number_indent()`.
- **`'paragraphs'`** and **`'sections'`** — no nroff macro starts a paragraph or a
  section. `{`, `}`, `[[`, `]]`, `(`, `)` and the `ip`/`ap` objects stop at blank
  lines, form feeds and braces; `inmacro()` goes. These were not folded to their
  default, which would have kept a table of nroff macro names. Dropping the
  recognition is the same choice as for `'comments'`, whose default would have
  kept all of the leader machinery.

**And the two mechanisms that were left reading what those options described.**
An option and its only consumer are one cut, not two:

- **The format operator.** `gq` and `gw`, their doubled `gqq`/`gqgq`/`gww`/`gwgw`,
  `op_format()`, `format_lines()` and `fmt_check_par()`. A paragraph was only a
  paragraph in order to decide where a format stopped. What stays is the wrap
  while typing: `'textwidth'` and `'wrapmargin'` still break a line through
  `insertchar()` and `internal_format()`, and `'paste'` still stops it. With no
  `gq`, `INSCHAR_FORMAT` is never set, so `comp_textwidth()` loses the flag that
  chose the screen width for it, and `insertchar()` loses its `c == NUL` entry.
- **Go to local declaration.** `gd` and `gD`, `nv_gd()` and `find_decl()`, which
  searched from the start of the block the cursor was in. `gd` was the only
  caller. `gq`, `gw`, `gd` and `gD` now fall to `nv_g_cmd()`'s default and beep.
- **The `=` operator.** `==`, `=G` and the rest. `op_reindent()` re-applied
  `get_indent()` — the indent the line already has — because `'equalprg'` went in
  phase 60 and C-indenting is off, so `=` had nothing left to compute. Its
  `nv_cmds` row points at `nv_error`.
- **The `!` operator, which was already dead.** Its `nv_cmds` row has been
  `nv_error` for phases, and `get_op_type()` is reached only from `nv_operator()`,
  so `OP_FILTER` could no longer be set at all. What goes is the dispatch nothing
  reached: the `OP_FILTER` case, the `!` that `op_colon()` typed after a range,
  and `do_bang()`'s `bangredo` block — the only thing that set it. **`:w !cmd` and
  `:r !cmd` still reach `do_bang()`**, and `do_filter()` still says the command is
  not available in this version, so the filter commands are untouched.
- **What C-indenting left behind.** The engine went phases ago — no
  `get_c_indent()`, no `cin_*` anything, and none of `'cindent'`, `'cinoptions'`,
  `'cinkeys'`, `'cinwords'`, `'indentexpr'` or `'indentkeys'`. What stayed was a
  switch wired to `FALSE` and its plumbing: `cindent_on()`, which is
  `return FALSE`, and **`can_cindent`, written in ten places and read in none.**
  gcc does not warn about that — a static that is assigned counts as used — which
  is the same blind spot `deadfields.py` exists for, one level up. `cindent_on()`'s
  two callers fold: CTRL-U in `ins_bs()` keeps the indent for `'autoindent'` alone,
  and the multi-character insert in `insertchar()` stops asking. `set_can_cindent()`
  goes with the flag. Three of the ten writes are the whole body of an `if`, so the
  test goes too — and each is scoped to its function, because `if (inindent(0))`
  also guards `do_pending_operator()`'s `oap->motion_type = MLINE`, which stays.

**`'smartindent'` is kept, and checked rather than assumed.** `may_do_si()`,
`did_si`/`can_si`/`can_si_back`/`no_si` and `open_line()`'s `{`, `}`, `#` and `)`
rules are a different mechanism from `'cindent'`, and this build switches it on by
default. The phase greps that all four survive, and a probe indents `y;` by one
`'shiftwidth'` after a line ending in `{` and brings `}` back out.

The `opchars[]` rows for `g`+`q` and `g`+`w` stay. The table is positional — its
index *is* the `OP_*` value — so a removed row would renumber every operator
after it. Nothing reaches them: `nv_g_cmd()` reaches the default first.

**Deleted outright rather than left to the sweep**: `auto_format()`,
`check_auto_format()`, `paragraph_start()`, `op_format()`, `format_lines()` and
`fmt_check_par()`. The last three matter for order — `comp_textwidth()` loses its
argument in the same phase, and `format_lines()` would still be calling it with
one when the sweep compiles.

**The options half does not fold `format_lines()` or `fmt_check_par()` first.** An
earlier version did, twenty-odd edits deep, and then the operator half deleted
both. Folding a function that is about to go is work the phase throws away; the
output is identical either way, and that was checked rather than assumed.

**Seven dry runs, and not one failure was a broken build.**

1. **The script failed its own counts.** `open_line()`'s leader block holds
   `lead_len = 0` statements and an `if (lead_len > 0)` of its own, so it is
   dropped first, by a pattern anchored on its first declaration.
2. **`phasecheck.sh` found `extra_len` set and not read** — it sized the leader's
   allocation and nothing else.
3. **The post-condition grep found `oparg_T`'s `cursor_start`**, which was `gw`'s
   alone. `deadfields.py` will not touch it: `pagescroll()` has
   `oparg_T oa = { 0 };`, and a **positional** initialiser names no field, so the
   tool keeps every field of a type that has one. It goes by hand, and `{ 0 }`
   fills only the first field, so removing a later one is safe.
4. **`do_bang()`'s `theend:` label went unused.** The `bangredo` block held the
   only `goto` that reached it, and a label nothing jumps to is a warning. The
   free below it runs either way, so only the marker goes.
5. **The `=` probe asserted the wrong thing**, and the measurement is the useful
   part. A retired operator does not let the motion through: it abandons the rest
   of the sequence. Measured on the q63 boundary, `=jix` gave `xa|b|c|` — `=`
   took `j`, came back to line 1 and inserted — while `!jix`, already `nv_error`,
   left the file alone. So the check is that **both** keys now leave it alone,
   with `!` as the invariant that says what a retired operator looks like, and a
   bare `ix` as the control that proves the binary still inserts.

## The delta

Three behaviour cases: `format_gq` (`gqq` with `tw=20`, which now beeps and
changes nothing), and the two that set the options, `format_comment` and
`open_comment`. No Ex command moves — these are all Normal-mode keys, and
`:center`, `:left` and `:right` were `ex_ni` long before this phase.

The phase checks:
- the five options are unknown to `:set`;
- typing `aaa bbb ccc ddd` with `tw=10` still wraps to two lines;
- `gqq` and `gqj` leave a long line exactly as it was;
- `gd` on `x` leaves the cursor where it is;
- `=jix` and `!jix` leave the file alone, while `ix` still inserts;
- `'smartindent'` still indents `y;` after a line ending in `{`, and `}` comes back;
- `}` from line 1 passes `.PP` to the last line.

Two more failures came from the checks rather than the cuts:

6. **`if (inindent(0))` matched twice.** It guards a `can_cindent` write in `edit()`
   and `oap->motion_type = MLINE` in `do_pending_operator()`, which stays. Each of
   the three `if`-bodied writes is now scoped to its own function.
7. **The `'smartindent'` probe sent no carriage return**, so `GAy;` appended to the
   same line and the probe failed where the editor was right. Calibrated against
   the q63 binary, which gives `if (x) {` / `    y;` / `}` for the corrected keys.

Measured: 100,354 → **97,734 lines**.
