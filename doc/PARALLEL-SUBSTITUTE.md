# PARALLEL-SUBSTITUTE.md -- how much of a :%s is matching, and what stands in the way

2026-09-27. A survey; nothing in it is built. It measures what `doc/IR.md`'s
proposal for `:%s/.../.../g` on many cores rests on: (1) a phase that makes the
regex engine's state a parameter, (2) a phase that splits `ex_substitute` into
snapshot / match / apply, (3) a parallel body of the match step per target. The
feature is a substitution over a **range of many lines** -- `:%s`, `:1,$s`,
`:'<,'>s`. A bare `:s` works on one line and gains nothing from more cores;
`:g/pat/s//x/` runs `ex_substitute` once per marked line, so it too is one line
at a time, and what it could gain is in `ex_global`'s own marking pass, a
second match-every-line loop that the same primitive would serve.

Measured at `16bb5d2` in the worktree `survey-parsub`, on the 64-core machine,
shared: the 1-minute load average read between 3.4 and 18.7 during the runs
(another project's), so every time below is a median and the runs of each are
kept (`.tmp/ps/run/*.txt`). No tracked file changed; the instruments are
throwaways under the worktree's `.tmp/ps/` (listed at the end).

**The short answer.** On a 500,000-line buffer the matching is 64-99% of a
`:%s` in the C and 49-95% in the Go, and that share does not grow with the
buffer: it is the same at 20,000, 100,000 and 500,000 lines, because both the
matching and the rest are linear in the lines. So the ceiling depends on the
pattern, not the size: a pattern that does real regex work per line (a class,
an alternation) is 93-99% matching, and 16 cores could make it 8-13 times
faster; a literal that hits most lines is about half matching, and no number
of cores makes it more than 2-3 times as fast. Undo is 5-10% of a dense
substitution and nearly nothing of the others; `:set ul=-1` moves the bound
little. And the survey found a bug worth more than the feature: **the Go editor
matches no pattern with `\|` or a repeated group** (below, *What the suite
misses*).

## The workloads

The editors have no files, so a buffer is built inside the editor: eight varied
lines typed in Insert mode (prose, code, SQL, a log line, `abab abc bab cab,
abba acdc cabbage`, an indented line), `ggVGy`, `{k}P` to make N lines, and
`Goneedle` for one line nothing else contains -- N = 20,001, 100,001 and
500,001. Five substitutions, each the only command after the build:

| name | command | matches |
| --- | --- | --- |
| lit | `:%s/the/THE/g` | a literal on 1 line in 8, twice |
| dense | `:%s/e/E/g` | every line, several times |
| bt | `:%s/\v(a|b)+c/X/g` | alternation under repetition: backtracking at every `a`/`b` |
| cls | `:%s/[ab]\+c/X/g` | the same matches without the alternation |
| sel | `:%s/needle/pin/g` | 1 line in 500,001 |

Each session ran twice: once ending `:q!` right after the build, once with the
substitution before `:q!`; **the `:%s` time is the difference of the two
medians** (7 runs each for C and Go, 5 for Java, 3 for Clojure, 1 for
Clojure's `bt`), so the build and the start-up are not counted. Keys come from
a regular file and output goes to a file, as `internal/suite` feeds them. The
binaries are `make bin/whim-vim bin/whim bin/braaam bin/vijure` (the C at the
product's `-O0`; the JVM launchers' C1-only, serial-GC flags). Every run's
output was hashed: **all four editors print the same bytes for every workload
and size, undo on and off, except the Go on `bt`**.

## 1. The ceiling

### The time of a :%s (seconds, median; undo on)

| | C | Go | Java | Clojure |
| --- | --- | --- | --- | --- |
| lit 20k / 100k / 500k | .016 / .068 / .370 | .017 / .056 / .246 | .076 / .335 / .786 | .35 / 1.36 / 7.65 |
| dense | .082 / .396 / 1.94 | .051 / .218 / .986 | .217 / .677 / 2.89 | .82 / 4.09 / 17.4 |
| bt | .815 / 3.88 / 19.6 | *wrong* .200 / .920 / 4.60 | .701 / 3.01 / 16.6 | 14.9 / 67.9 / 301 |
| cls | .118 / .565 / 2.86 | .110 / .508 / 2.50 | .265 / 1.05 / 4.85 | 2.78 / 15.9 / 64.4 |
| sel | .009 / .032 / .191 | .007 / .036 / .167 | .057 / .110 / .382 | .15 / .82 / 3.76 |

From 100k to 500k lines the C's times grow 4.9-6.0 times, the Go's 4.4-5.0,
the Clojure's 4.1-5.6 (`bt` 4.4): linear. The Java's grow 2.3-5.5 times, less where the
work is short, because C1 compiles during the first 100k. Clojure at 20k moved
by up to 2 times between two batches of 3 (lit .35 and .64); its 100k and 500k
are steadier; its `bt` at 500k is one run (301 s), the others at 500k three. Memory is not
the limit anywhere measured: the largest resident set was Clojure's, 5.5 GB on
`cls` and `dense` at 500k, on a 62 GB machine; the C's 1 GiB arena, which never
frees, held every workload at 500k (199 MB resident at most).

### How much of it is matching

The share of `ex_substitute`'s own time spent in `vim_regexec_multi` and
everything it calls (the engine, and `reg_getline` -> `ml_get_buf`), at 500k,
undo on (undo off in parentheses):

| | C `-O0` (perf) | Go (pprof) | Java (JFR) | Java, GC counted | Clojure 100k (JFR) | Clojure, GC counted |
| --- | --- | --- | --- | --- | --- | --- |
| lit | .72 (.78) | .735 (.775) | .68 (.69) | .44 | .76 | .70 |
| dense | .64 (.70) | .485 (.536) | .39 (.42) | .27 | .55 | .52 |
| bt | .986 | *wrong* | .98 | .91 | not profiled | -- |
| cls | .93 (.95) | .93 (.93) | .94 | .77 | .97 | .95 |
| sel | .975 | .945 | .97 | .73 | .93 | .91 |

What the rest is, in the C at 500k, dense: building the new line
(`vim_regsub_multi`) 8.3%, undo (`u_savesub` and kin) 8.8%, `ml_replace` 3.7%,
and 15.7% in `ex_substitute`'s own loop (`vim_strnsave` of each line, the
`alloc_clear`s, the copy). Java and Clojure: the same parts, plus the copying
(`vim_strnsave` is 18.8% of the Java's dense). **Size does not move the
share**: the C's lit is .734 / .730 / .718 at 20k / 100k / 500k, dense .63 /
.64 / .64, cls .94 / .94 / .93.

**GC.** The JVM editors run the serial collector, whose pauses stop the one
thread and are not in JFR's execution samples. Counted from JFR's
`jdk.GarbageCollection` events and set against the base session's, a `:%s` at
500k in the Java spends about .29 s of its .79 s in pauses on lit, .87 of 2.89
on dense, .8 of 4.85 on cls; the Clojure at 100k .11 of 1.36 (lit), .27 of
4.09 (dense). The "GC counted" columns treat those pauses as sequential; with a
parallel collector some of them would not be. The Go's collector already runs
on other cores (`gcBgMarkWorker`, 9-15% of the samples, outside
`ex_substitute`), so the Go's shares are of the editor's own thread.

**The C at -O2** (not the product, for reference, built with frame pointers):
the same `:%s` at 500k takes .104 s (lit), .297 (dense), 2.02 (bt), .848
(cls), .062 (sel) -- 3.1 to 9.7 times the product's speed -- and matching is
.67, .39, .90, .94 of it. At `-O0`, 62% of the C's `bt` is `musl_memset`
called from `cleanup_subexpr`, which clears ten start and end positions before
every attempt at every column.

**Undo off.** `:set ul=-1` typed before the `:%s` (and in the base session)
takes undo from 8.8% to 0.4% of the C's dense and from 5.9% to 0.3% of its
lit; measured times: C dense 1.94 -> 1.80 s, Java dense 2.89 -> 2.44, Go dense
.986 -> .851; lit, cls and sel moved by less than the noise. It raises the
matching share by about the undo share, which moves the bound on dense and lit
by 0.2-0.6x at 16 cores, and nothing on the others.

### Amdahl's bound

Speed-up at 8 / 16 / 64 cores if the matching share *p* ran perfectly in
parallel and nothing else changed: 1 / ((1 - p) + p / n).

| | C `-O0` | Go | Java (GC counted) | Clojure (GC counted) |
| --- | --- | --- | --- | --- |
| lit | 2.7 / 3.1 / 3.4 | 2.8 / 3.2 / 3.6 | 1.6 / 1.7 / 1.8 | 2.6 / 2.9 / 3.2 |
| lit, undo off | 3.1 / 3.7 / 4.3 | 3.1 / 3.7 / 4.2 | -- | -- |
| dense | 2.3 / 2.5 / 2.7 | 1.7 / 1.8 / 1.9 | 1.3 / 1.3 / 1.4 | 1.8 / 2.0 / 2.0 |
| dense, undo off | 2.6 / 2.9 / 3.2 | 1.9 / 2.0 / 2.1 | -- | -- |
| bt | 7.3 / 13.2 / 34 | -- | 4.9 / 6.8 / 9.6 | -- |
| cls | 5.4 / 7.8 / 11.8 | 5.4 / 7.8 / 11.8 | 3.1 / 3.6 / 4.1 | 5.9 / 9.1 / 15.4 |
| sel | 6.8 / 11.6 / 24.9 | 5.8 / 8.8 / 14.3 | 2.8 / 3.2 / 3.6 | 4.9 / 6.8 / 9.6 |

Without counting its GC the Java's bounds are the C's to within a row (cls 5.6
/ 8.4 / 13.4, dense 1.5 / 1.6 / 1.6). These are ceilings: the snapshot, the
hand-back of the results and the memory bandwidth of 64 cores all come off
them, and `sel`'s large bound is of a small time (.17-3.8 s at 500k).

## 2. The obstacles, counted

### (a) The engine's file-scope state

Only the backtracking engine is left (`vim_regcomp` calls `bt_regcomp`; there
is no NFA). A crude call graph (`.tmp/ps/cg`, text-level, stopped at the error
and interrupt paths) gives 101 functions reachable from `vim_regexec_multi`.
The file-scope objects they write or read, with their uses in `src/whim-vim.c`:

| object | uses | lifetime | what it is |
| --- | --- | --- | --- |
| `rex` (`regexec_T`, 18 members) | 502 (on 314 lines; 446 in the matcher) | per call | the match in progress: the result arrays, the buffer and window, the first line and last, the current line and input pointer, `reg_ic`, `reg_maxcol` |
| `rex_in_use` | 16 | per call | re-entry guard: `rex` is saved and restored around a nested match |
| `regstack`, `regstack_star`, `regstack_behind`, `backpos` | 21, 9, 10, 36 | scratch, cached across calls | the backtracking stacks, growarrays kept between matches |
| `regstack_bytes` | 14 | per match | `'maxmempattern'` accounting |
| `behind_pos`, `bl_minval`, `bl_maxval` | 13, 3, 3 | per match | look-behind |
| `brace_min`, `brace_max`, `brace_count` | 6, 6, 8 | per match | `\{n,m}` counters |
| `reg_tofree`, `reg_tofreelen` | 8, 4 | scratch | a copy for back-references across lines |
| `reg_toolong` | 12 | per match | set when a pattern is too long to match |
| `got_int` | 132 | global | read in the engine's loops; written by the interrupt check |
| `breakcheck_count` | 5 | global | `fast_breakcheck` from `regmatch` and `reg_nextline`, every 10,000th call polling the host for input (`ui_breakcheck`) |

and two members of the *compiled pattern* written during a match:
`prog->re_in_use` (the recursion guard, set and cleared around every call) and
`prog->regmlen`, which `cstrncmp` may shorten when case is ignored and
`cstrncmp__regmlen` writes back. Read-only during a match, and so no obstacle:
the character tables (`class_tab`, `g_chartab`, `toLower`, `toUpper`,
`utf8len_tab`, `foldCase`, `decomp_table`, `mb_bytelen_tab`), `cmp_flags`, the
options `p_mmp`, `p_cpo`, `p_ambw`, `p_emoji`; and, for four atoms only,
`curwin`'s cursor (`\%#`), the marks (`\%'m`), `VIsual` (`\%V`) and the window
(`\%v`). The compiler's state (`regparse`, `reg_magic`, `curchr`, `regcode`,
and the rest) is used before the loop, once, and is no obstacle.

So "the state is a parameter" is: `rex`, the four stacks, `regstack_bytes`,
the look-behind and brace variables and `reg_tofree` become one struct passed
down (40 functions of 4,223 lines name one of them, `regmatch`'s 1,393 lines
among them); `re_in_use` and `regmlen` move off the program into that struct;
and the interrupt check is left to the caller. Everything is per call or per
match -- nothing carries a result from one line's match to the next -- which is
what makes the lines independent.

### (b) What reading a line writes

The matcher reads lines through `reg_getline` -> `ml_get_buf(rex.reg_buf,
lnum, FALSE)`, and a read writes: the buffer's one-line cache
(`b_ml.ml_line_ptr`, `ml_line_len`, `ml_line_textlen`, `ml_line_lnum`,
`ml_flags`); `ml_flush_line`, which writes a dirty cached line back into its
block first; and `ml_find_line`, which moves the locked block
(`ml_locked`, `ml_locked_low`/`high`/`lineadd`) and the path stack
(`ml_stack`, `ml_stack_top`, `ml_stack_size`). None of it can run on two
threads. There is no memfile or swap any more, so a line's text lives in its
data block and the pointer `ml_get_buf` returns stays valid until that block
changes: **reading the range's lines up front -- a pointer and a length per
line, before the first `ml_replace` -- is enough** for a pattern that reads
only its own line. It costs one `ml_get` per line, sequentially; inside the
matcher today that is 1-4% of the `:%s` (C 3.9% on lit, 1.7% on dense). A
multi-line pattern (`RF_HASNL`: `\n`, `\_x`) reads past its line, up to the
buffer's end, and look-behind (`RF_LOOKBH`) may read before it; those take the
sequential path, or a snapshot of the whole buffer.

### (c) What in ex_substitute's loop cannot be reordered

| | in the product | forces the sequential path |
| --- | --- | --- |
| `\=` expression | the syntax survives, the evaluator does not: `vim_regsub_both`'s `\=` branch is empty, so `:s/b/\=1+1/g` deletes each `b` (`abc abc` -> `ac ac`, all four editors alike) | no: nothing is evaluated |
| `c` confirm | yes: `do_ask`, a redraw and `plain_vgetc` per match | yes (and `'a'` turns it off mid-way) |
| `n` count only | yes: `do_count` | no -- the easiest case: match, count, apply nothing |
| marks | `setpcmark` at the first match, `mark_adjust` only when a match spans or a replacement splits lines, `b_op_start`/`end` after the loop | no: all are in the apply step, in order |
| last substitute | `old_sub`, `save_re_pat`, `regtilde`'s `reg_prev_sub` -- all before the loop | no |
| interrupt | `line_breakcheck` per line, `fast_breakcheck` in the engine, `got_int` read in the loops | no, but the check must stay on the main thread: a worker polls a flag |
| timeout | the `timed_out` argument survives and is `nullptr`; nothing writes it | no: there is none |
| multi-line match | yes: `nmatch > 1`, `nmatch_tl`, lines joined | yes, `re_multiline(prog)` decides |
| a replacement with `\r` | yes: lines split, `lnum` and `line2` shift | no for the matching (later lines' text is unchanged), but `\%23l` would see the shift: yes if the pattern has `RE_LNUM` |
| `\%#`, `\%'m`, `\%V`, `\%v` | yes (`CURSOR`, `RE_MARK`, `RE_VISUAL`, `RE_VCOL`) | yes: the cursor moves as the loop goes |
| `&`, `~`, `:&&` | yes: `CMD_and`, `CMD_tilde` call `ex_substitute`; `~` is compiled in, `&` expanded by `vim_regsub` | no |
| `g` and empty matches | the next match of a line is searched on the *unchanged* line at `matchcol`, with the empty-match step (`prev_matchcol`) | no, but that logic (about 20 lines) moves into the match step |

## 3. The shape

The match step returns, for each line of the range, what the loop would find on
it, on the original text:

```c
typedef struct { colnr_T start, end; lpos_T sub_start[10], sub_end[10]; } linematch_T;
/* Match prog against lines[0..n): for each line, every match the :s loop
 * would take (all of them with do_all, the first without), empty matches
 * stepped over as the loop does. Pure: reads lines and prog, writes st and
 * out. Returns the number of lines with a match, or -1 if got_int was seen. */
static long match_lines(regstate_T *st, regprog_T *prog, bool do_all,
                        const string_T *lines, linenr_T n,
                        linematch_T **out, int *nout);
```

`ex_substitute` keeps its first 300 lines (parsing, flags, `search_regcomp`,
`regtilde`) and splits its loop (lines 15798-16228, 431 of its 805) in two:
before it, when the pattern is single-line and has no cursor, mark, visual or
line-number atom and neither `c` nor a multi-line match is possible, snapshot
the range and call `match_lines`; the loop then walks the results instead of
calling `vim_regexec_multi`, and keeps everything else -- `vim_regsub_multi`,
`u_savesub`, `ml_replace`, the splits, marks, the count and the messages -- in
order and unchanged. Otherwise it runs today's loop. The first phase (the
state a parameter) touches the 40 functions above, 4,223 lines, and the 13
call sites of the `vim_regexec*` entry points outside the engine, or none of them if the old
signatures stay as wrappers over a file-scope default state. The second
touches `ex_substitute`'s loop (431 lines) and adds `match_lines` (by the
loop's matching lines, about 60). `vim_regsub_multi` reads `rex` too, but only
in the sequential apply step.

## 4. What the suite misses

Running `bt` showed the Go editor answering `E486: Pattern not found` where the
C, the Java and the Clojure substitute. It is not the workload: in the Go,
`a\|c`, `\(b\)\+c` and `\%(a\|b\)c` match nothing, in `:s` and in `/`, while
`[ab]\+c` and `\v(ab)c` work. Building the Go editor from `git archive` of
each commit that touched `editor/editor.go`: correct at `a2c2e32` and at the
eight commits after it, wrong from **`643c037` (gen: a pointer into an array
that never walks is a plain \*T)** to HEAD. None of the 45 quick cases, the
240 wide ones or the heavy case has a `\|`. Its time above is the time to fail
at every position; it is not comparable.

This is the risk of the parallel `:%s` too, in small: the suites hold each
editor to the C on the cases they have, and a regex construct no case uses can
be wrong in one target unseen. A parallel match step would add a chunking
boundary, a result hand-back and a fall-back rule; the cases to prove them
are one per row of 2(c), each across a chunk boundary, on a buffer large
enough to be split -- the suite's sessions are 5 lines, the heavy case 5,000.

## 5. Verdict

**Do a cheaper thing first.** In order:

1. **Fix the Go's alternation** and add cases with `\|`, `\(..\)\+`, `\%(..\)`,
   `\{n,m}`, look-behind and back-references to the suite: a correctness bug
   outweighs any speed-up.
2. **The Clojure editor's matcher.** Clojure is 22 times the C on `cls` at
   500k (64.4 s against 2.86) and 15 times on `bt` (301 s against 19.6); its top frames
   are `clojure.lang.Numbers.num` (boxing: 3,301 of 11,450 samples on `cls`)
   and `regmatch`. That is `doc/CLOJURE-IDIOMS.md`'s business: a matcher as fast
   as the Java's would gain 13 times on one core, about what 64 cores gain at
   the Clojure's `cls` bound (15.4).
3. **Undo off for big edits** is already there (`:set ul=-1`): 7-16% on dense.

**Then the parallel `:%s`, if the patterns that matter are regex work.** For a
pattern that is a class, an alternation or backtracking over a 500,000-line
buffer, matching is 93-99% of the time (the Java's 77-91% counting its serial
GC's pauses), the bound at 16 cores is 7.8-13 times (the Java's 3.6-6.8), and
the seconds are real: the Java's `bt`, 16.6 s, would become 1.4-2.4 s, the
Go's `cls`, 2.5 s, about .3 s. For a literal or a pattern that hits most lines,
matching is 27-78% and the bound is 1.3-4.3 times at any number of cores --
what a `:%s/foo/bar/g` usually is, and already .25-.8 s at 500k in the Go and
the Java. The obstacles are all removable: the state is per call, the lines
are independent once snapshotted, and every order-dependent feature is either
gone (`\=`, timeouts), in the apply step (marks, undo, messages), or
detectable up front (`c`, multi-line, the four position atoms) and sent down
today's path. The cost is two C phases of about 4,700 lines touched and a
per-target body; the risk is what 4 found, and it needs the cases before the
code.

## The instruments

All under the worktree's `.tmp/ps/`: `gen.sh` (the key files), `run/` (the
timer: runs, medians, SHA-256 of each output, peak RSS), `whim-vim-sym` (the C
product's line without `-s`, with frame pointers) and `whim-vim-o2`, `perf`
record/script with `stacks.awk` and `stacks2.awk` (the share of each part
under `ex_substitute`), `prof/` (bin/whim with `runtime/pprof` at 2 kHz) and
`pp.sh`, `jfr/fast.jfc` (JFR's profile settings with a 1 ms execution sample)
and `jfr.awk`, `cg/` (the text-level call graph and object uses),
`fnuse.awk` (the functions naming a set of objects), `tryrev.sh` (the Go
editor built at a revision).
