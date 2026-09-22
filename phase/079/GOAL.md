# Phase 79 — the constant-return predicates

Twenty-eight functions whose whole body is `return <constant>;`. Each was emptied by
an earlier phase and left with its callers in place, so the editor still asks "is the
popup menu visible", "are we in a Vim9 script", "is there more than one window" — and
still branches on an answer that cannot change. No sweep can see this: at `-O0` each
is a real call and a real branch, and the code is *reachable*. Unuseful, not unused.

**The invariant is asserted, not trusted.** Step 1 reads all 28 definitions and
requires each body to be exactly `return <expected>;`, with the expected token written
out per name. If an upstream ever gives one a real body the phase fails instead of
folding a live predicate. That is phase 77's pattern, and it is the only thing between
a fold and a wrong answer.

## Four that look identical and are not

A scan for `return <single token>;` reports 32. Four of those tokens are **variables**:
`get_hislen`→`hislen`, `is_maphash_valid`→`maphash_valid`, `get_search_pat`→`mr_pattern`,
`get_text_locked_msg`→ a static message. My first classifier said *thirteen* of the 32
returned a variable — it had matched the bare tokens `0`, `1` and `NULL` against
unrelated declarations elsewhere in the file. Reading the **definitions** gives four.
Supplying the expected constant per name is what makes that error impossible to repeat
silently, and an assertion requires all four to survive.

**`did_set_number_relativenumber` is a constant and still is not touched.** Its only
two mentions are option-table rows where it appears as a *function pointer* with no
call parentheses. Folding is meaningless and deleting it would leave two rows pointing
at nothing. An assertion requires both rows intact.

## `binds_out` vetoes fold_always, not fold_never

`parse_command_modifiers`' `if (vim9script)` block contains a `break` that binds to the
enclosing `for (;;)` — the shape that made `buflist_findpat` change behaviour silently
in phase 71. But that phase **folded a walk**, keeping the body while removing the loop
around it, so the `break` rebound. `fold_never` **deletes** the body, `break` and all,
and the condition was false, so it never fired. Every `fold_always` site here was
audited separately and none contains an escaping `break`.

## Three second-order cuts, each proved in the phase

**`skip_for_popup`** is not a stub on entry — it has three returns. Once `pum_under_menu`
and `pum_visible` fold, both its guards go and it becomes `return FALSE;`. The phase
re-runs the same `const_of()` check to *prove* the collapse before using it, then takes
nine more sites.

**`may_have_range`** is a local of `do_one_cmd` with two writes. One is inside the
`if (vim9script && …)` block this phase folds; the other is that block's `else` arm,
`may_have_range = TRUE;`. After the fold it has one write, is constantly true, and both
readers fold.

**`wc`** in `option_value2string` is `long wc = 0;` whose only "write" is `&wc` passed to
`wc_use_keyname` — which never dereferences `wcp`. So **both** arms of that chain are
dead, not just the first, and it collapses to the `sprintf`. Read from the body, not
assumed; the phase asserts `wcp` is absent from it.

`need_check_timestamps`, `need_redraw` and `bom_count` each become write-only once the
stub feeding them is gone, so their tests fold and the variables sweep.

## Ordering, and the ternary that spans a line break

Specific literals run before blanket regexes **except where a blanket edit creates the
specific one's target**. Three places turn on it, and the third is the sharp one: the
address ternary in `do_one_cmd` is the rare construct in this tree that spans a line
break, so it must be replaced *before* the blanket `current_win_nr` pass — which would
otherwise rewrite one half and leave `eap->line2 = eap->addr_type == ADDR_WINDOWS`
dangling. All 74 anchors were counted against the q78 tree before the program was
written, and that pre-flight caught the one edit I had never transcribed:
`&& !at_ins_compl_key()`, which I knew about only from a truncated survey line.

**No term edit ends in whitespace.** `only_one_window() && check_changed_any` becomes
`check_changed_any` rather than stripping `only_one_window() && `, because a literal
with a trailing space lost it passing through an editor in phase 71.

## Two bugs, both in the checks rather than the edits

**An assertion that a correct edit could not satisfy.** `vim9script` was in the
zero-mention loop, but four mentions survive and none is the identifier: three
former-file banner comments and the `[CMD_vim9script]` row, where it is the command
*name* — a string literal. A bare word count cannot tell an identifier from a comment
or a string. Phase 78 made this same mistake twice in one script.

**`set -e` killed the phase on the behaviour it was checking.** The quit probes were
written `( … ); rc=$?`, and a subshell whose status is not *tested* aborts under
`set -e`. The second probe runs `:q` on a **modified** file, which exits 1 by design —
so the script died after the build with every probe unrun, and the truncated log made
it look like a clean finish. The form is `rc=0; ( … ) || rc=$?`, which is a tested
context. Phases 77 and 78 avoided this with `|| true` and never needed the status.

## The delta

**None**, and `whimdelta.sh` confirms it. Every fold removes a branch whose condition
cannot hold and every term edit removes a constantly-true conjunct or a constantly-false
disjunct. The quit path is the one place where an error would be silent rather than
fatal — `check_more()` feeds the four `ex_quit`/`ex_exit` conditions that decide whether
`getout(0)` runs, so wrong folding makes the editor refuse to quit or quit without
saving. Five quit probes calibrated on q78 guard it: `:q` refuses a modified file,
`:q!` discards, `:wq` and `:x` write and exit.

Measured: 89,233 → **88,636 lines**.
