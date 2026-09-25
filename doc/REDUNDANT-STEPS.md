# Which steps the canonical print and the sweep made unnecessary

Survey, 2026-09-25. Worktree at HEAD `aa268c4`. No tracked file in the main checkout was changed. Nothing was committed.

**Since written:** it measured `aa268c4`. Phases 166-168 came after it (the
answers typed `bool`, the key names, and the headers dropped last, which moved
the header drops out of 82, 99 and 104), so re-run a finding before acting on
it. The instruments it names under `.tmp/survey/` are not tracked.

## The question and how it was measured

Every phase now ends with two things:

- **the sweep**: `internal/sweep.Prune`, a reachability closure;
- **the canonical print**: `internal/cemit`, which gives one spelling, no comments, 4-space indent, one blank line between top-level declarations and none inside bodies.

A step, or part of a step, counts as **unnecessary** when removing it leaves the phase's output boundary `qN.c` byte-identical.

Method:

1. **Baseline.** The committed boundaries (`.cache/boundaries`, copied to `.tmp/survey/b` at 04:24) were checked against HEAD's code. Every source phase was run alone with `whim build --from N --to N --src q(N-1).c`, and **161/161 reproduced `qN.c` byte for byte.** Phases 0, 83, 84, 86, 116, 123 and 163 change no source. So every comparison below is against a verified pair.
2. **Variants.** Each variant is a copy of the worktree source (`.tmp/survey/src/vNN`), patched and built to `.tmp/survey/bin/vNN`. It was run on the affected phases (or on all 161) with at most 8 jobs at a time, and each output was `cmp`'d with `qN.c`. The scripts are `.tmp/survey/runall.sh`, `patch.py` and `disable.py`. The per-phase logs are in `.tmp/survey/runs/<variant>/`.
3. **Category tests.** Where a category repeats through a shared helper, the helper itself was neutralised and **all 161 phases** were run (G1–G5 below). That covers every call site at once, so no sampling was needed there.
4. **Per-phase tests.** Candidates found by reading were disabled together, phase by phase. If the phase stayed identical, the whole set is proven **jointly**. If not, I bisected until the rest was identical. Where a phase's own assertion counted the thing removed (an `After` map, a `Gone` list, a line count or a blank-run count), I relaxed that assertion in the same variant; the table says so.

"Proven" below means byte-identical output was measured. "Suspected" means read but not run.

## Totals

| | count |
|---|---|
| Whole phases whose edit is a no-op (`q(N-1) == qN` without it) | **2** (82, 99) |
| Plan steps that can go (proven) | **3** (funcreach at 5, inner sweep at 21 and 53). There are also 14 assertion-only steps: `cmdidxs --check` in 13 phases and `query-empty` at phase 2 |
| Shared helpers whose layout work is redundant everywhere (proven on all 161 phases) | **5** (G1–G5), covering about 130 call sites |
| Per-phase hand deletions that the sweep would do anyway (proven) | about **300** declarations: ~72 function definitions, ~23 prototypes, 7 typedefs, ~64 file-scope objects, 11 enumerators or anonymous enums, 41 struct members, ~80 locals |
| Per-phase layout, spacing and blank-line code (proven) | about **35** blocks in 16 phases |
| Phases with at least one proven-redundant part (beyond the assertion-only steps and the helper-wide G1/G3/G4) | **84** of the 161 source phases (counted from the per-phase table) |
| Parts shown NOT redundant by measurement | **12 inner sweeps**, plus about 20 individual deletions (see the last section) |

Variant results, summarised as SAME (identical) or not:

| variant | what | result |
|---|---|---|
| v1 | `cutil.Dedent4` is the identity | 160/161 SAME; 94 refuses |
| v2 | all 14 inner `sweep` steps, `funcreach`, the 13 `cmdidxs --check`, `query-empty`, and the `whim82` and `whim99` edits removed from the plan | 20 SAME; the 12 inner-sweep phases refuse |
| v11 | `cutil.DeleteDefinition` deletes nothing (it still refuses when the function is missing) | 152/161 SAME; 3 25 26 67 72 87 95 96 122 differ |
| v12 | every `\n\n?` in a cutter or edit regex becomes `\n` (62 sites) | 159/161 SAME; 88 and 122 refuse |
| v13 | every blank-line "eat" in `cutil.DropIf`, `FoldNever`'s `tidy`, 10 cutters and 2 `TrimLeft`s removed | 161/161 SAME |
| v3–v10 | per-phase sets, listed in the table | all SAME after the bisection noted |
| v9full, v10full | the two combined cutter variants on all 161 phases | 161/161 SAME each |

## Recurring categories

### G1. Re-indenting a kept body (canonical print): 160 of 161 phases

`cutil.Dedent4` strips one indent level from a body that a fold keeps. The printer re-indents from the AST anyway.

Call sites: `cutil.FoldAlways`, `cutil.FoldNever`, `edit/driver.go:258` (`E.Always`), `edit/shared.go:501,591,827`, and these cutters and edits:
- `norecover.go` ×2, `noenc.go`, `nobackup.go`, `nohome.go`, `noowner.go` ×3
- `keepbytes.go`, `lfonly.go`, `utf8only.go`, `nowildmenu.go` ×2, `noucmd.go`
- `phase/122:289`

**Proven: with `Dedent4` the identity, 160/161 phases are identical.**

The exception is **phase 94**. `ex_quit`'s `FoldNever` is followed in the same phase by the exact-indentation anchor `w94lit1`, which only matches the dedented text. Phase 94 would need that anchor matched modulo whitespace before `Dedent4` could go.

The hand-written re-indents have the same cause, and all were proven separately:
- 93's `foldAlwaysElse` loop
- 143's `W143Helper` and its switch re-indent
- 144's `do_intr` re-indent
- 145's else-body re-indent
- 127's `carry`
- 122's `pad`

### G2. Deleting a function the sweep would take (sweep): 152 of 161 phases

`cutil.DeleteDefinition` has 45 call sites, several of them in loops, covering about 60 functions. Neutralised: **152/161 identical.** Of the 9 that are not:

- **3, 87, 96**: only a phase assertion counts the deleted function. With the assertion relaxed, they are identical (v10, v7).
- **25**: all but `get_bkc_flags` are redundant (v9b). `get_bkc_flags` reads `b_p_bkc`, and `droplocal b_p_bkc` runs in the same phase with no sweep before it.
- **26, 67, 72, 95, 122**: really needed. The reasons are in the last section.

The same holds for the phase-local deleters: 103's `delfunc` rows, 125's `cutDefn`, 111's `elapsed()`, 119's `mch_get_pid()` and 87's `dropDefinition`. All were proven redundant except where noted.

### G3. `\n\n?` suffixes in regexes (canonical print): 60 of 62

Inside a body the canonical text has no blank lines, so the optional newline never matches. At file scope it matches the separator blank line, and the printer rewrites that anyway. **Proven: 159/161 identical with all 62 replaced by `\n`.**

The 2 that are needed are `phase/088:111` `w88EnumRun` and `phase/122:122` `w122EnumRun`. Each matches a *run* of top-level `enum { ME_… };` declarations, which the printer separates with blank lines, so the `\n?` is what joins the run.

### G4. Eating the blank line around a deletion (canonical print): all of them

Proven 161/161 identical (v13) with all of these removed:
- `cutil.DropIf`'s second-newline eat
- `cutil.FoldNever`'s `tidy`
- the newline eats in `dropoptions.go:144`, `nofloat.go:121`, `nofencs.go:55,58`, `noenc.go:128,131`, `nofenc.go:132`, `nomouse.go:163`, `nowildmenu.go:84`, `nobackup.go:36`
- `bytes.TrimLeft(…, "\n")` at `noenc.go:210` and `nomemfile.go:288`

The hand-written versions are proven per phase:
- 110's whole blank-line bookkeeping: collapse loop, drop ranges, appended blanks
- 121:388
- 125's `cutDefn` blank requirement and its "paragraphs emptied" pass
- 126's `\n\n` in the forward-declaration regex
- 127's `repl = append(repl, "")`
- 128's `cut` helper and two double-blank collapses
- 141's `TrimPrefix("\n")`
- 112's `"\n\n"` test before its deleted tables
- 80's `W80Blanks` ×2 and its `ex_ni` blank tweak

### G5. Plan steps

| step | phase | why | result |
|---|---|---|---|
| `funcreach --delete` | 5 | sweep: a strict subset of the closure (it is one of the six deleters Prune replaced) | proven (v2) |
| inner `sweep` | 21, 53 | the end-of-phase sweep does the same job; GOALS.md already says 53 did not need it | proven (v2) |
| inner `sweep` | 16, 24, 28, 32, 42, 49, 50, 57, 58, 60, 62, 64 | **needed**: `droplocal` and `dropoptions --strict` refuse on readers inside dead functions | refuses without it (v2) |
| `cmdidxs --check` | 58, 63, 66, 68–79 | assertion only, returns its input | output-neutral (v2). It checks meaning, not layout, so it is not redundant *as a check* |
| `query-empty whim2` | 2 | assertion only | output-neutral (v2) |
| `edit whim82` (every comment) | 82 | canonical print: the input has no comments, so there is nothing to strip or collapse; `editlit.go` is referenced nowhere | proven: whole phase is a no-op (`q081 == q082`, and v2) |
| `edit whim99` (the includes nothing names) | 99 | sweep: its one cut, the `stat_T` typedef, was already taken by the sweep at phase 93, and the includes are phase 167's now; the rest is assertions | proven: whole phase is a no-op (`q098 == q099`, and v2) |

### Other recurring kinds, each proven in the phases listed in the table

- **Write-only locals, statics and members** whose last write the phase removes. What is left is a bare declaration, which the sweep takes. The earlier gcc-driven sweep could not see these, and that is why the phases name them. Found in 3, 6, 17, 21, 25, 28, 31, 36, 39, 42, 50, 51, 53, 57–81, 85, 87, 88, 90, 92–96, 103, 122, 125–127, 142, 160, 161.
- **Struct members nothing names afterwards.** "A struct field is not a variable, so the sweep cannot see it" was true of `deadfields.py`; Prune takes a member nothing live names. Found in 17, 18, 21, 22, 25, 29, 35, 49, 50, 62, 68–70, 72, 73, 78, 85, 90, 94, 125, 146, 160.
- **Prototypes of functions deleted in the same phase**: 3, 25, 95, 103, 111, 126.
- **Assertions that the canonical print makes true by construction.** These are output-neutral, so they are not tested one by one:
  - blank-run counts: 99:344, 100:314, 103:359, 106:344, 110:269/725, 113:271, 117:324, 120:417, 125:303/962, 126:560, 128:868
  - "the includes are contiguous" and "every directive is an `#include`": 99, 110, 111, 113, 118, 119, 125, 126
  - "a blank line follows X" / "the block is its own paragraph": 110:244,300; 118:198; 119:227; 125:250
  - 93:236 "the body is one level in"
  - 107's doubled-space and ` __attribute__` shape checks
  - 127's blank-line tolerance loops
  - 144:137 and 150's walk back over blank lines
- **Line-count arithmetic that exists only because literals carry blank lines** (100:309, 101:179, 102:264, 108:369, 109:526, 113:242, 114:311, 115:424, 117:289). These are assertions; they tie the spelling of the inserted text to a count the printer then discards. 108's was relaxed in its test.

## Table: phase → step or part → why → proven?

*Why* is **CP** (canonical print), **SW** (sweep), **IMP** (a shape that cannot occur in canonical input) or **ASSERT**. *Proven* names the variant. "Jointly" means proven together with the other rows of the same phase.

| phase | step / part | why | proven |
|---|---|---|---|
| 2 | `query-empty whim2` | ASSERT | yes, v2 |
| 3 | optreaders: `is_not_a_term` and `is_not_a_term_or_gui` definitions, their 2 prototypes, the `p_shm_save` local (with `optreadersAfter` relaxed: `not_a_term` 1→3, two 0-rows dropped) | SW | yes, v10b |
| 5 | `funcreach --delete` | SW | yes, v2 |
| 6 | nowildmenu: the 4 local-deleting closures (noselect/noinsert/cmdline_unchanged, wim_noselect/noinsert, show_menu ×2, skip_pum_redraw); `Dedent4` ×2 | SW; CP | yes, v10a, v1 |
| 12 | noenc `Dedent4`, `TrimLeft`, newline eats | CP | yes, v1, v13 |
| 17 | nofenc: `b_start_fenc` and `b_start_bomb` members, `b0_fenc` and `gvarp` locals | SW | yes, v9a |
| 18 | nocmdopts: `evim_mode` member | SW | yes, v9a |
| 20 | nohome: the whole `expand_env_esc` rewrite (the `~` chain, the `$`/`~` trigger, `at_start`; about 60 lines). nogetenv's `envBody` later in the same phase replaces that function's body | other (overwritten) | yes, v9b |
| 20 | nogetenv: `names[4]` table; `set_init_default_shell` and `set_init_default_cdpath` definitions | SW | yes, v9a, v11 |
| 21 | nomemfile: `swap_mode` and `try_again` locals, `total_mem_used`, `mch_total_mem`, `set_init_default_maxmemtot`, `mch_get_host_name`, 8 memfile_T members, 8 `mf_*` definitions | SW | yes, v9a/v9b |
| 21 | norecover: `swapfile_info`, `recover_names`, `ml_recover` definitions; `Dedent4` ×2 | SW; CP | yes, v10a, v1 |
| 21 | inner `sweep` | SW | yes, v2 |
| 22 | nochdir: `globaldir` field, `start_dir` global, `win_fix_current_dir` together with the `globaldir` global | SW | yes, v9a/v9b |
| 23 | nofloat: removing the `TYPE_FLOAT` enumerator | SW | yes, v9b |
| 24 | nomouse: `ignore_drag_release` local, `check_mouse_termcode`, the 6 option-handler definitions | SW | yes, v9a |
| 25 | nobackup: `backup`, `backup_copy`, `dobackup`, `backup_ext` locals; `vim_rename`, `vim_copyfile`, `set_file_time`; the ACL bundle (3 definitions, the `acl` local, 3 prototypes, `vim_acl_T`); `Dedent4` | SW; CP | yes, v9a/v9b |
| 25 | noowner: `ROOT_UID`, `flen`, `b0_uname` member, `B0_UNAME_SIZE`, `get_user_name`; `Dedent4` ×3 | SW; CP | yes, v10a |
| 28 | nocindent: `do_cindent` and `want_cindent` locals, `parse_cino`, `do_c_expr_indent` | SW | yes, v9a |
| 29 | noucmd: `b_ucmds` member; `Dedent4` | SW; CP | yes, v10a |
| 30 | noident: the first alternative (`case ]:` unquoted) never matches | IMP | yes, v9b |
| 31 | nofnamemod: `tilde_file` and `skip_mod` locals, `modify_fname` | SW | yes, v9a |
| 32 | nocompl: `has_compl_option` | SW | yes, v9a |
| 35 | nosession: `wo_eiw` member | SW | yes, v10a |
| 36 | notabs: `tp`, `had_tab`, `use_tab` locals (the `tpWord` check dropped; the `cmod_tab` count 1→2) | SW | yes, v10a |
| 39 | nowindows: `static int tc`, `split`, `is_split_cmd`, and `int i; win_T *wp;` | SW | yes, v10a |
| 42 | onebuffer: `prev_alt_fnum` (the `w_alt_fnum` count 1→2), `xfname`, `flags` | SW | yes, v10a |
| 49 | oneoptset: `b_p_ml_nobin` member | SW | yes, v10a |
| 50 | lfonly: `write_bin`, `fileformat`, `save_bin` ×2; `b_p_eol` and `b_p_eof` members | SW | yes, v9a |
| 51 | keepbytes: `bad_char_idx` | SW | yes, v9a |
| 53 | noconv: `vimconv` ×2 and `tofree`; inner `sweep` | SW | yes, v9a, v2 |
| 57 | `lispcomm` and `lisp` locals | SW | yes, v6a |
| 58, 63, 66, 68–79 | `cmdidxs --check` | ASSERT | yes, v2 |
| 59 | `key_is_wc` local | SW | yes, v6a |
| 60 | the `delay_pending` / `acl_elapsed` cut | SW | yes, v6a |
| 62 | Whim62BL: the `b_p_bl` field (the `get_varp` case stays) | SW | yes, v6a |
| 64 | 6 open_line declarations, 11 internal_format declarations, `fo_ins_blank`, `remove_comments`, `comments`, `prev_was_comment`, `force_format`; the `auto_format`, `check_auto_format`, `paragraph_start`, `op_format`, `format_lines`, `fmt_check_par` and `set_can_cindent` definitions; the `Insstart_textlen`, `Insstart_blank_vcol`, `end_comment_pending` and `can_cindent` globals. The comment at 064:298 ("the sweep compiles") is stale | SW | yes, v6a |
| 65 | `op_function` and `ins_ctrl_x` definitions | SW | yes, v11 |
| 66 | `prev_pos` local | SW | yes, v6a |
| 67 | `reset_dragwin` and `reset_held_button`; `dragwin`, `held_button`, `mouse_row`/`mouse_col`, `old_mouse_*`; `mouse_index_found`, `looks_like_mouse_start`, `spv`; the `spellvars_T` typedef; the declaration half of `writeOnlyStatics`, plus `frame_locked`, `swap_exists_did_quit`, `did_swapwrite_msg`, `autocmd_nested`, `oldtitle_outdated`, `deadly_signal`, `mr_patternlen`, `was_safe` | SW | yes, v6b |
| 68 | `use_aucmd_win_idx` member; `win_alloc_popup_win`, `win_init_popup_win`, `autocmd_init`; the `aucmd_win[]` table | SW | yes, v6a |
| 69 | `w_alist`, `w_arg_idx`, `w_arg_idx_invalid` members | SW | yes, v6a |
| 70 | `handle_swap_exists`, `check_swap_exists_action`, `check_need_swap`, `ml_open_file`; `swap_exists_action`; the 3 `SEA_*` enums; the `b_may_swap` member | SW | yes, v6a |
| 71 | `au_pending_free_buf`, `firstbuf`, `lastbuf`, `buf_reuse`; the `valid` local | SW | yes, v6a |
| 72 | the `w_next` member; the tabpage locals in 8 functions; `first_tabpage` | SW | yes, v6b |
| 73 | `fr_parent`, `fr_next`, `fr_prev`, `fr_child` members | SW | yes, v6a |
| 74 | `fmarks_check_names`, `fname2fnum`, `namedfm`, `EXTRA_MARKS` | SW | yes, v6a |
| 75 | `cmdline_type`, `aco`, `bufref`, `did_cmd` locals | SW | yes, v6a |
| 76 | `p_re`, `AUTOMATIC_ENGINE` | SW | yes, v6a |
| 78 | `winid` and `w_id` members; `last_win_id`, `LOWEST_WIN_ID`, `autocmd_blocked`, `autocmd_no_enter`/`leave`, `redrawing_for_callback`, `prevwin` | SW | yes, v6a |
| 79 | `vim9script` locals (3 functions), `may_have_range`, `need_check_timestamps` | SW | yes, v6a |
| 80 | `ni` local, `if_level` (w80lit5); `W80Blanks` ×2 and the `ex_ni` blank-line tweak | SW; CP | yes, v6a |
| 81 | `starts_with_colon` ×2 | SW | yes, v6a |
| 82 | the whole `whim82` edit | CP | yes, v2 (the phase is a no-op) |
| 85 | `tty_fail` member | SW | yes, v7a |
| 87 | `nv_exmode`, `do_exmode`, `getexmodeline`, `check_tty`; `ex_pressedreturn`, `ex_no_reprint`, `ex_exitval`; the `exmode_was`, `previous_got_int`, `use_plus_cmd` locals (the `w87After` and "Entering Ex mode" checks relaxed) | SW | yes, v7a |
| 88 | the `p` and `had_minmin` locals | SW | yes, v7a |
| 90 | `usefilter` member | SW | yes, v7a |
| 92 | `read_fifo` and `flags` locals | SW | yes, v7a |
| 93 | the `foldAlwaysElse` dedent loop; the `ffname`/`sfname`/`st` locals (lit11); `readonlymode` | CP; SW | yes, v7a |
| 94 | the `w_topline_was_set` member (lit3) | SW | yes, v7b |
| 95 | the `did_set_readonly` prototype | SW | yes, v7b |
| 96 | the `redir_write` fold (the function is deleted anyway); the `retesc`, `script_char`, `did_return` locals; `redir_off` | other; SW | yes, v7a |
| 99 | the whole `whim99` edit | SW | yes, v2 (the phase is a no-op) |
| 103 | 34 of the 37 rows (every row except R5, W1, W2): W7 and S12 (inside the RealWaitForChar region the T1 splice replaces), M9 (TMODE_SLEEP), 12 prototype rows, 10 global rows, and the delfuncs R7, W3, W5, I1, S1, S2, S5, S9, T1 (the Gone, Want and blank-run checks relaxed) | other; SW | yes, v8a/v8b |
| 105 | the "plain" vs "inline" layout choice | CP | yes, v8a |
| 108 | the `vim_host_message` and `vim_host_exit` objects (2 SwapOnce), with their assertions | SW | yes, v8a |
| 110 | all blank-line bookkeeping in `build`, and the output blank-run assertion | CP | yes, v4a |
| 111 | step 4: the `elapsed_T` typedef, its prototype and `elapsed()` | SW | yes, v4b |
| 112 | the tight-row fallback; section B (the `musl_toUpper` and `musl_toLower` tables) | IMP; SW | yes, v4b |
| 118 | the blank lines in `w118Defs` | CP | yes, v4b |
| 119 | the `mch_get_pid()` definition; the blank line in `w119Def` | SW; CP | yes, v4b |
| 120 | the empty-union arm is dead: replaced by a refusal, it never fires | IMP | yes, v5b |
| 121 | the newline swallow at 388 | CP | yes, v5a |
| 122 | the `want_argument`, `c` and `requested` locals; `Dedent4`; `pad`; the enum `"\n\n"` | SW; CP | yes, v5a |
| 125 | `cutDefn`'s blank-line requirement; the `b0p` declaration; `bnum2`; the `mf_trans`, `mf_blocknr_min`, `mf_neg_count` and `pe_old_lnum` members; the `dirty` declaration; the `lnum_left`/`lnum_right` declarations; `mf_dont_release`; the blank-run pass | SW; CP | yes, v5a |
| 126 | 4 of the 11 forward declarations (`mf_ins_hash`, `mf_rem_hash`, `mf_ins_free`, `mf_rem_free`); the `\n\n` in the regex; s18 (`page_count`) and s26 (`page_count_left`/`right`); the blank-run check | SW; CP | yes, v5b |
| 127 | the unused-locals loop (step 11); the `ML_APPEND_MARK` enum; the blank line in `repl`; `carry`'s re-indent | SW; CP | yes, v5b |
| 128 | the `cut` helper's blank handling; the 2 double-blank collapses | CP | yes, v5b |
| 128 | the `typedef struct memfile memfile_T` deletion (the final Gone check skips `memfile`/`memfile_T`) | SW | yes, v5c |
| 137 | `W137Tick`'s spacing rules | CP | yes, v3 |
| 138 | 2 `LiteralOrGone` calls (`cfunc_T`, `cfunc_free_T`): always "already gone", 0 mentions in q137 | SW | yes, v3 |
| 141 | `TrimPrefix(rest, "\n")` | IMP | yes, v3 |
| 142 | the `date_time` local (and its `__DATE__` assertion) | SW | yes, v3 |
| 143 | `W143Helper`'s dedent loop; the `Inner` TrimSuffix; the switch re-indent | CP | yes, v3 |
| 144 | `W144Enclosing`'s blank walk; the `do_intr` re-indent; the do-while `(?:\n *)?`; the identity `ReplaceAll("\n","\n")` | IMP; CP | yes, v3 |
| 145 | the else-body re-indent | CP | yes, v3 |
| 146 | the `pb_hdr` and `db_hdr` members | SW | yes, v3 |
| 152 | the `lead` whitespace | CP | yes, v3 |
| 160 | the `find_func_t` `LiteralOrGone` (0 mentions in q159); the `cookie` member (and the "still named" check) | SW | yes, v3 |
| 161 | the `static char_u questions[4]` removal | SW | yes, v3 |
| all | G1 `Dedent4` (not 94), G3 `\n\n?` (not 88:111 and 122:122), G4 blank eats | CP | yes, v1, v12, v13 |

### Suspected but not run

- **Blank lines and alignment inside inserted literals.** Examples: 101–109 editlits, 114 `w114Defs`, 115's `\n\n` anchors at 350/366/386, 124/126 `*New`, 139, 143:58, `Whim155`, 147/150/152/161/162, 097 `w97Sites`, 093 `w93lit10/12`, 088:278. This is the same kind as 118, 119 and 122, which were proven. Several are coupled to the line-count assertions listed above.
- **`@state` writes** in 110, 118, 119, 124, 125, 126, 127, 128. They are output-neutral by construction: the scratch directory is thrown away, and plan.go notes no edit reads it.
- **Unused Go constants**: `w82lit*` and the whole of `082/editlit.go`; the `w69`–`w80` editlit constants listed by the reader; the `*Note` constants in `nochdir`, `nogetenv`, `nohome`, `nomemfile`, `norecover`, `nosignals`.
- **Not run:**
  - 96's `redir_write` prototype and definition, together with its `After` counts
  - 87's `save_silent` pair
  - 128's `struct memfile` deletion (beyond the typedef)
  - `nowildmenu`'s `[ \t]*$` tails
  - `includes.go:125-137`: `total`/`keep` is never passed in a build

## What looks redundant and is not, measured

| phase | part | why it stays |
|---|---|---|
| 16, 24, 28, 32, 42, 49, 50, 57, 58, 60, 62, 64 | inner `sweep` | `droplocal` (and at 24 `dropoptions --strict`) refuses while a dead function still names the field or option. The sweep has to take those functions first (v2). |
| 94 | `Dedent4` in `ex_quit`'s FoldNever | the next edit's `w94lit1` is an exact-indentation anchor (v1) |
| 94 | `wi_changelistidx` member and its write in `find_wininfo` (lit5, lit6) | `find_wininfo` is live, so the write and the member survive the sweep (v7a DIFF) |
| 26 | nosignals' 5 definitions and its 3 globals | still reachable: the output keeps `catch_sigusr1`, `catch_sigpwr` and `got_sigusr1` (v10a DIFF) |
| 18 | nostartup emptying `source_startup_scripts` | `dropoptions --strict exrc/viminfo` runs next with no sweep in between, and refuses on the readers in that body (v10a) |
| 25 | `get_bkc_flags` | `droplocal b_p_bkc` runs in the same phase with no sweep before it |
| 67 | `state_no_longer_safe` | its body holds a `was_safe = …` write, which the next `Lines` counts (3, not 2) |
| 72 | `borrow_stl_vsep_hl` | its window walk has an escaping `break` that the walk rewrite refuses |
| 95 | `change_warning` and `did_set_readonly` definitions | `droplocal b_p_ro` in the same phase sees their reads |
| 122 | `mainerr_arg_missing` | keeps `ME_ARG_MISSING` mentioned, which breaks the renumbering |
| 103 | R5, W1, W2 | their bodies hold the second and third occurrences of R6's and W9's counted anchors |
| 88:111, 122:122 | `\n\n?` in `w88EnumRun` / `w122EnumRun` | these match a run of top-level enums, which the printer separates with blank lines |

Kinds the sweep and the printer never do. Readers flagged each of these; I checked each against prune.go and cemit but did not run them:
- **statements**: every write, increment, call, or empty `if {}`
- **labels**: 64 `theend:`, 87 `theend:`, `notabs` `wingotofile:`, 149's labels
- **parameters**
- **members of a struct initialised by position**: `cmdarg_T ca = {0,}` pins 78's `prechar`, and `oparg_T oa = {0}` pins 64's `cursor_start`
- **members whose name another struct still uses**: 69 `fname`, 71 `b_next`
- **renumbering enumerators, and the positional table rows that go with them**: 88, 89–91, 93, 122
- **duplicate prototypes the phase must remove before a rename or move**: 114 `labs`/`abs`, 115 `host_time`
- **parentheses**, which cemit keeps: 146 `w146Paren`
- **the fallthrough respelling at 107**, which changes what the printer emits

`dropopts.go:193`'s newline eat is reported needed: the `else if` promotion after it depends on it.

## Recommended order for removing them

Each step is one commit, checked with `make whim-build-check`; the check runs phase by phase from q(N-1), so it names any phase that moves. Every item below was proven with the same per-phase check that `whim-build-check` runs.

1. **Whole no-ops first.** Phases 82 and 99 become plan steps with no edit, keeping their numbers. Then the `funcreach` step at 5 and the inner sweeps at 21 and 53. These are the biggest deletions with the least risk: about 480 lines of edit, the `dead.FuncReach`/`DeleteFuncs` caller, and 3 plan entries.
2. **The shared helpers (G4, then G3).** The blank-line eats in `cutil.DropIf`, `FoldNever`'s `tidy` and the 10 cutters; then the 60 `\n\n?`s. Both are proven on all 161 phases at once.
3. **`Dedent4` (G1).** First make 94's `w94lit1` anchor whitespace-insensitive (or keep a local dedent there), then make `Dedent4` go away everywhere. That removes about 20 call sites and the hand-written re-indents (93, 122, 127, 143–145).
4. **Blank-line bookkeeping and the assertions it feeds.** This covers:
   - 110, 125, 126, 128 and `BlankRuns` everywhere
   - the "includes are contiguous" / "blank line follows" checks
   - the line-count arithmetic, together with the blank lines in the literals it counts

   It has to be one change per phase, because each literal and its count move together.
5. **Hand deletions the sweep would do, phase by phase** (the table). Do the cutters first (v9b and v10 are each proven on all 161 phases), then 57–81, 85–96, 103, 108–128 and 137–161. Relax or rewrite each phase's `After`/`Gone`/count assertion in the same commit, since several of them count exactly what the sweep now takes. Leave everything in the section above.
6. **Last, the cosmetic literal whitespace and the unused constants.** They are output-neutral and cost nothing to keep.

Artifacts: the variant trees, binaries, scripts and logs are in `/root/go-whim/.tmp/survey/`. The worktree has been removed.
