# Phase 32 — insert completion, the popup menu, and the keys that reached them

CTRL-N, CTRL-P and the whole CTRL-X family — `CTRL-X CTRL-F` for file names,
`CTRL-X CTRL-K` for a dictionary, `CTRL-X CTRL-L` for whole lines — plus the
popup menu that displays the matches. This is the largest single subsystem left
after the regexp engine, and it is the one whose sources are all gone already:
the tag stack went in Phase 10 and the `CTRL-]` key in Phase 30, the shell in
Phases 6 and 8, `'dictionary'` and `'thesaurus'` name files this editor has no
business reading, and `'completefunc'` needs the eval layer.

**Two cuts, with a sweep between them.** The first answers the questions
completion is entered through, so it produces nothing; the second removes the
code that kept asking. This was two phases, and the second existed only because
the first had stopped short.

## The predicates

**Nine predicates become constants**, and the sweep follows them:

```
ins_complete              FAIL        pum_visible                    FALSE
ins_compl_prep            FALSE       pum_redraw_in_same_position    FALSE
ins_compl_active          FALSE       pum_may_redraw   pum_undisplay   pum_display
ins_compl_has_autocomplete FALSE
```

Twelve option rows go with them — `autocomplete complete completefunc
completeopt dictionary infercase pumborder pummaxwidth pumopt pumheight pumwidth
thesaurus` — and six buffer-local fields, `b_p_cpt b_p_cot b_p_dict b_p_tsr
b_p_inf b_p_ac`.

## `didset_string_options()`, for the fourth time

This is the fourth phase to be caught by it, and this time it was a **segfault
before the first keystroke**. The function dereferences every string option's
global at startup, so dropping `'completeopt'`'s row while leaving

```c
opt_strings_flags(p_cot, p_cot_values, &cot_flags, TRUE);
```

hands a NULL to something that reads it. The editor did not mis-complete; it
did not start.

`orphanopts.py` existed precisely to catch this and did not, because it looked
for an explicit `*p_x` dereference and this is a bare argument. **It now counts
any mention at all.** A pointer nothing mentions is harmless — the sweep takes
it — and one that is mentioned while having no row to initialise it is a NULL
going somewhere, which is enough to fail on without judging the shape of the
somewhere. Re-run over every earlier boundary: no new complaints, so the
stricter rule costs nothing and closes the trap that had cost four phases.

## Checking that a key does nothing

Bare CTRL-N in insert mode is **already inert** in a build with nothing to
complete from, so a before/after comparison of it proves nothing either way.
`tools/complcheck.py` uses `CTRL-X CTRL-N` instead, which is unambiguous, and
checks the half that must survive in the same run: **insert mode still
inserts**. A completion check that only proves completion is gone also passes on
a binary that cannot type.

## The callers

**Seventy functions named `ins_compl_*`, `pum_*` or `compl_*` survive the
stubs.** They are *reachable*, so no sweep can touch them, and never *entered*,
because `ins_complete()` returns FAIL before any of them runs. `edit()` does not
reach completion through one door: it calls `ins_compl_addleader()`,
`ins_compl_bs()`, `ins_compl_accept_char()` and twenty-five more directly, and
`update_screen()`, `win_line()`, `showruler()` and `screen_puts_len()` each ask
`pum_visible()` on their own account. That is the shape worth naming: **a stub
answers a question; it does not remove the caller that asks it.** So the second
cut removes the callers, and the second sweep takes the callees.

What goes, all of it inside `edit()`:

| | |
| --- | --- |
| the CTRL-X submode | `ins_ctrl_x()` is empty, so `ctrl_x_mode` never leaves `CTRL_X_NORMAL` and every `ctrl_x_mode_*()` test is decided |
| the per-key completion arm | forty lines feeding each keystroke to the match list |
| `'autocomplete'` | six arming sites, three of them one-line blobs macro expansion left behind |
| the arrow keys | four `if (pum_visible()) goto docomplete;` arms on Up, Down, PageUp and PageDown |
| `docomplete:` | the label itself |

**What stays is the answer the stubs gave**: CTRL-N and CTRL-P are still
insert-mode keys, and they now do nothing, which is what an unbound key does.

**The sweep between the two cuts is kept, and it is not a formality.**
`tools/nocomplkeys.py` counts and matches text in `edit()` as the first sweep
leaves it, and a count taken over code about to be swept is a different count.

## A cut that is not unique is a guess

The first version dropped the autocomplete disarm by matching its condition,
`if (c != KE_CURSORHOLD && c != KE_COMPLETE_DELAY)`. That condition occurs
**three times inside `edit()`**, and the one the search took was

```c
        {
            lastc = c;
        }
```

— the last-character save, which has nothing to do with completion. It
compiled, it swept clean, the island still shrank by 2,400 lines, and **nothing
downstream objected.** `drop_unique()` now refuses any condition that is not
unique in the file; anything genuinely ambiguous is spelled out in full or
anchored to one function with `drop_if_in()`. The phase also asserts the `lastc`
line is still there, because that is the failure that got through.

## Two halves, and only the pair is a check

Completion must be absent — `tools/complcheck.py` — and the arrow keys, whose
`pum_visible()` arms this phase cuts, must still move the cursor. Cutting a
guard and the key's real body together is exactly what no completion check would
notice, so `tools/arrowcheck.py` (since retired) asked in a pty: from `one/two/three`, `A` then
Down then `X` must give `twoX`. **It was proved able to fail first** — with
`ins_down()` removed it reports `oneX`.

## The delta

Measured: **135,315 → 127,131 lines** and symbols 88 → 88, the subsystem being
pure computation over things already removed. The thirteen functions that remain
of the island are constant-answer stubs the redraw layer asks on its own account.
**Merged, the phase reproduces the boundary the two phases recorded byte for
byte**, in 173 seconds against the 203 they took in sequence. **The delta is
none** — no behaviour case types CTRL-N, these are insert-mode keys, and no Ex
command moves.
