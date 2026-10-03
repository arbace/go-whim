# CLAUDE.md

Guidance for Claude Code working in this repository. **This file describes the
tree as it now is**; when the two disagree, this file is wrong -- fix it, by
editing the sentence that became wrong rather than appending one that disagrees
with it. The numbers here are measurements: re-measure rather than adjust them by
reasoning.

## What this is

One pipeline, and the Go toolset it runs:

```
slim-vim.c  --whim-->  whim-vim.c
```

- **The input is one file**, `slim-vim.c`: vim 9.2 as a single C23 translation
  unit, produced by [arbace/slim-vim](https://github.com/arbace/slim-vim), whose
  pipeline removes the preprocessor, the comments and dead code and **changes
  nothing the editor does**. `make` fetches it and vim's `LICENSE` at the commit
  that repository's `main` points to, and records the commit in `src/upstream.sha`.
  It is not tracked here. **Never edit it**; a change to the input belongs in
  arbace/slim-vim.
- **whim** (the `Makefile`) removes capability on purpose, phases 0-103 from
  180,870 lines to 77,634, numbered in the order they run (`doc/PHASES.md`
  maps them to the numbers they were written under, 0-184 with gaps, which
  `GOALS.md`, every `GOAL.md` and the records still use). It is two arcs, a
  coda, five for the Go's sake, the headers, the gotos, the parallel `:%s`,
  the out-parameters and struct locals as values, the C spelled plainly, and
  its flags bool:
  - **phases 0-26** (`GOALS.md` Part I, where they are 0-82) leave an editor
    with no runtime to install, 80,809 lines at q026;
  - **phases 27-56** (`GOALS.md` Part II, 83-128 there) turn it into an
    embeddable core: no filesystem, the host behind a line in the file, no
    libc the core names, the text a tree. `GOALS.md` Part II, *Phases 83 to
    128 as they stand*, is the account read across and belongs there, not
    here;
  - **phases 57-85** (Part II too, 129-162 there) remove from the core what
    transpiling it to Go (`editor/editor.go`, `internal/gen/FINDINGS.md`) had
    to work around. All but 70 were meant to change nothing the editor does;
    70 drops the build date from the version line. `internal/gen/FINDINGS.md`
    maps each finding to its phase, in the old numbers.
  - **phase 86** takes what the Go's linters found dead in the C: the
    statements after a jump (its part 86a, a general rule) and six stores
    nothing reads (86, named). With the generator's own fixes, `go vet`,
    staticcheck and `gofmt -s` are clean on `editor/`.
  - **phase 87**'s part 87a declares `bool` the 278 core functions whose
    every return answers yes or no -- OK and FAIL included, OK being `true` --
    and the locals, struct members and parameters that only ever hold an
    answer; `f() == FAIL` is `!f()`. So the Go says `if f()` where it said
    `if f() != 0`. Then 87 names the 153 constant key codes (`K_DEL`,
    `K_IGNORE`) the preprocessor's removal left as arithmetic; the binary is
    byte-identical.
  - **phase 88** drops every system header nothing needs, in one step:
    each `#include` is tried and kept out while the file compiles silently.
    Records 82 and 99 and phase 40 once did it piecemeal, and the phases
    between them asserted the counts it left; now they assert only that every
    directive is a contiguous `#include` and that they add and remove none.
  - **phase 89** copies a label's tail -- at most three statements and the
    return, or a void function's end -- over each of the 98 `goto`s that reach
    one (`crefactor/xform`'s `GotoTail`), and drops the 21 labels left
    unreached: 21 of the 41 functions with a `goto` have none left. Record
    168 wrote `return x;` for each `goto` whose label marks `return x;`, which
    is this rule with a tail of no statements, and 89 takes them all (the
    reform's G, measured byte for byte).
  - **phases 90-91** take the gotos a loop says: one to the statement after
    its own loop or switch is `break;`, one to where control goes next anyway
    is deleted (90, `GotoBreak`); a goto back to a label is a loop and
    `continue;` (91, `GotoLoop`): 4 breaks, 1 deleted, 2 loops.
  - **phase 92** (after the headers: it touches none) wraps the region a
    forward `goto` leaves -- to a label of a block that holds it, no loop or
    switch between -- in `do { ... } while (0);` and writes the goto `break;`
    (`crefactor/xform`'s `GotoBlock`); togo writes that do-while as Go's
    `switch { default: ... }`, a break leaving it as the C's does (`for { ...;
    break }` before, which staticcheck reads as a loop unconditionally ended). After 89-91: 50 gotos, 11 labels. The C keeps 49
    gotos, all in the core (185 before 89): each leaves a loop or switch and
    needs a flag or a state variable, which a Java backend need not have (a
    labeled break says them). The Go keeps the same 49, in 8 functions, from 163 in 34:
    togo writes no goto of its own (a continue that must reach a loop's end
    is `break contN` out of a once-loop around the body).
  - **phase 93** takes the address of a position's line and column out of
    the two functions that took it -- `mark_adjust_internal()`'s 13
    expansions of vim's `one_adjust()` macros become calls of two functions
    of the value, and `cursor_pos_info()`'s columns come back through locals
    -- so the Java and Clojure editors no longer box `pos_T.lnum` and `.col`
    (the Java's `[0]` reads 7,032 -> 4,874).
  - **phase 94** is that rule in general (`crefactor/xform`'s `MemberOut`):
    a call passing `&s->m` for its callee to read and write calls a function
    written once per callee and member that does it through a local --
    where the copy is provably the call's (nothing the callee can reach names
    that member, the pointer is kept nowhere, every address of the member is
    such a site): 4 members, `[0]` reads 4,874 -> 4,774; the rest held and
    reported (the option table's pointers; vim's error paths reach code that
    names the others).
  - **phases 95-96** are the parallel `:%s` (`doc/PARALLEL-SUBSTITUTE.md`).
    95 makes the regex engine's state -- `rex`, its stacks, the look-behind
    and brace variables -- one struct, `regengine_T`, handed down as a
    parameter from the four functions the editor calls the engine by
    (`crefactor/xform`'s `StateParam`: 16 objects, 43 functions). 96 lets
    the engine match one line handed to it and nothing else, failing where a
    match would need more (another line, the cursor, a mark, a message), and
    adds `match_lines`, which says for each line of a range whether it holds
    a match; `ex_substitute` skips the lines it clears. The C runs it line
    after line; the Go, Java, Clojure, Haskell, Rust and Scheme editors run it in
    chunks on every core, each on an engine of its own (a runtime body,
    `internal/whim/gen.go`). Exact whatever the pattern: nothing the editor
    does moves.
  - **phase 97** has `:g`'s marking pass ask `match_range` too, one search
    a line (`:g/\v(a|b)+c/d` at 500,000 lines: Go 9.1 -> 0.8 s, Clojure 106
    -> 5.6); **phase 98** returns from `ml_clearmarked` when nothing is
    marked -- its loop read line 0's slot, index -1, which the Go, Java and
    Clojure editors failed on for any `:g` that matched nothing.
  - **phase 99** has the C host's `host_time()` return `WHIM_TIME` when it
    is set (the Go and Java hosts do the same), and the suite sets it on
    every editor it runs: undo's "N seconds ago" counted wall-clock seconds
    crossed, and the JVM editors failed its cases now and then under load.
  - **phase 100** makes an out-parameter a value in and a value out
    (`crefactor/xform`'s `LocalOut`: a parameter `T *p` its callee only
    reads, writes and null-tests, every caller passing `&x` of a local
    nothing else reaches, x not read unsequenced beside the call; the value
    returned, or with the result a struct of them) and a local struct of
    scalars its members' locals (`StructScalar`): 94 out-parameters of 61
    functions (14 taking no value in), 66 structs. The Java's `[0]` reads
    4,802 -> 4,009, the Clojure's one-element arrays 637 -> 515; the Java
    takes a struct a call returns as it is, the Haskell as a tuple.
  - **phase 101** spells plainly what the preprocessor left
    (`crefactor/xform`'s `plainc.go`): gettext's identity `_()` is not
    called, each call its argument (399); `(unsigned)c - 'A' < 26` and its
    kin are `ascii_isupper(c)`, `_islower`, `_isdigit` again (136); an `if`
    of a constant condition is the branch it takes (8). The Java's
    `gettext_(` 400 -> 1 and `Integer.compareUnsigned` 150 -> 17. Taking
    the constant `if (1) { len = 0; }` showed three parameters whose value
    the function never reads; the Go writes them `_` with a local of the
    name (`deadInParams`), which keeps `editor/` staticcheck-clean.
  - **phase 102** is part 87a's rule on the file-scope objects
    (`BoolRet` with `Globals`): 79 flags that only ever hold an answer --
    `VIsual_active`, `msg_scroll`, `exiting` -- are `bool`, and with them 1
    function, 7 locals, 4 members and 3 parameters; one whose address a
    table takes (`&p_wiv`), one compared with a code, sized, or shadowed by
    a local stays `int`. The Java's `VIsual_active != 0` 115 -> 0, its
    `TRUE`/`FALSE` 1,195 -> 875.
  - **phase 103** takes what 102's rule left int (`BoolRet` with `Relax`):
    the greatest fixed point, so a flag saved in a local and restored is
    an answer; a literal 0 or 1 assigned; `x |= E` of an answer, written
    `x = (E) || x`. 64 declarations, 11 of them flags (`need_wait_return`,
    `did_cursorhold`); the Java's `TRUE`/`FALSE` 871 -> 779. Left int: flags
    saved in a local its function reuses (`msg_scroll`), and `got_int`,
    which the host sets.

  The line between the two arcs falls between phases 26 and 27, where record
  83 stands in the old numbers.

  **The pipeline reform** (`doc/PIPELINE-REFORM.md`) reordered what the bytes
  allow.
  - **Phases 1-5 are the front.** They cut every interface and feature the
    product has not, as twelve packages (D1-D12; phase 1 runs D1-D5, phase 2
    D6-D8, phase 3 D9-D12, each part followed by its own closure, so that the
    parallel check runs the three side by side): the command line, the Ex commands,
    the files, `:q`'s refusal, reading, the command syntax, the options, the
    swap file, startup, the encoding, the terminal, one window and buffer,
    the editing features, one regexp engine, the process; and, after the
    reform, every drop that stayed: the editing-feature programs, phase 3's
    parts 3a-3f, at the end of its part, and first in phases 4 and 5, on the
    text the phase before swept, what counts or anchors on swept text --
    `nostat` (record 13's), part 4a, `onebuffer` (record 42's) and parts
    4b-4e (4f after `nowildmenu`) at phase 4; `nobackup` (phase 12's),
    `lfonly`, `keepbytes` and `noconv` (records 50, 51 and 53's) and parts
    5a-5d at phase 5. The fall-out closure (`crefactor/xform`'s `FallOut`)
    folds what each of phases 1-3 left unwritten.
  - **The rest are named in blocks.** `d02`-`d13` are the drops that count
    text only their predecessors leave, `r01`-`r14` the rewires, and
    `g01`-`g11` the generic steps (`doc/GOALS.md`, *The pipeline as it runs*).
  - **The phases that edit nothing are records** in `internal/phase/archive/`,
    under their old numbers.

  **The test suite is minimal.** Each phase was verified, while it was written, by a
  check program of its own and a delta declared in advance against recorded
  baselines; that whole suite (`internal/check`, `internal/verify`,
  `internal/harness`, every `check.go`, every `delta.md` but the one phase 1 now holds, the
  baselines and `make whim-verify`) was removed after `448e9a8`, the last commit
  that has it. What proves a change now is two things: `make whim-build-check`,
  the committed product back byte for byte and every boundary compiled, which
  sees the text; and `make
  whim-test` (`internal/suite`), which sees the editor -- 80 key sessions
  (`internal/suite/cases.md`) fed from a file on stdin, so that a run's output
  never depends on timing, to a build of the working tree's
  `src/whim-vim.c` and to one of HEAD's, required to print the same screens and
  exit the same way, with a control (`" INSERT"` spelled `" INSERX"`) that must
  move at least one of them, and a case that runs out of keys before its `:q!`
  refused. Then the same cases on the GO editor (`editor/` built), required to
  answer exactly as the C candidate does -- the one check that `editor.go` is
  the editor and not only what `internal/gen` writes (the same control moves 76
  of the 80 there; six cases of the regex engine -- alternation, groups,
  counts, back-references, look-behind, classes -- since the Go editor
  was found matching no `\|` in `doc/PARALLEL-SUBSTITUTE.md`; and 29 `par_*`
  cases, each a `:%s` or a `:g` over 3,000 numbered lines built by keys with every
  line printed after it, one for each thing that sends phase 96's
  matching back to the loop, across the parallel editors' chunks). About
  11 s, the four builds side by side. **`make
  whim-test-wide`** is optional and wider: 240 cases in four groups -- the 102
  keystroke cases of the archived suite with their startup arguments, every Ex
  command by name, 30 command lines, and 10 runs on a real pseudo-terminal of
  several sizes and TERMs -- held to the same two comparisons and the same
  control, in about 4 s. **`--java`**, on either, adds the JAVA editor
  (`braaam/`, built from the candidate): the same cases, required to answer as
  the C candidate does, with a control of its own (`" INSERT"` changed in the
  generated `braaam/editor/Editor.java`) that must move the Java editor's own answers; the
  Java editor answers all 80 and all 240 as the C does (`doc/JAVA.md`,
  milestone 2), its control seen as the Go's is. **`--clojure`** adds the
  CLOJURE editor (`vijure/`) the same way, its control `" INSERT"` changed
  in the generated `editor.clj`, written by `crefactor/togo`'s Clojure backend
  (`doc/CLOJURE.md`, milestones 1-2); the Clojure editor answers all 80 and
  all 240 as the C does, and `--clojure-editor F` runs it on a namespace
  written already. **`--haskell`** adds the HASKELL editor, caprice
  (`caprice/`, `doc/HASKELL.md`), the same way, its control `" INSERT"`
  changed in the generated `Editor.hs`; it answers all 80 and all 240 as the
  C does, its two builds kept in `.cache/caprice-suite/` between runs, since
  its core is nine modules and two and a half minutes of GHC's time;
  `--haskell-bin P` runs a caprice built already instead (`make
  caprice.hsl`'s, `doc/GHC-LISP.md`), its control P itself with its one
  `" INSERT"` changed to `" INSERX"` in its bytes, so that what the control
  moves is that program's own answers. **`--limit D`**, on either suite, is
  how long one run of a case may take on the editors the flags add and on
  their controls (10 s by default, and always for the C builds and the Go
  editor, the oracle): the `-O0` ghc-lisp caprice takes 43 s on `par_mmp`,
  and answers all 80 and all 240 as the C does with `--limit 2m`.
  **`--rust`** adds the RUST editor, whimsy (`whimsy/`, `doc/RUST.md`), the
  same way, its control `" INSERT"` changed in the generated `editor.rs`; it
  answers all 80 and all 240 as the C does, its two builds kept in
  `.cache/whimsy-suite/`. **`--scheme`** adds the SCHEME editor, whimsical
  (`whimsical/`, `doc/SCHEME.md`), the same way, its control `" INSERT"`
  changed in the generated `editor.ss` (the literal's bytes in its image);
  it answers all 80 and all 240 as the C does, its two builds kept in
  `.cache/whimsical-suite/`; `--scheme-debug` runs its debugging build
  (`whim whimsical --debug`: optimize-level 2, safe) instead, which answers
  them all too. Both suites are exact under load (`internal/suite/stress_test.go`,
  48 busy loops on the 64 cores, every case's runs held to its first): fed
  from a file, 0 differing runs of 6,720 for the wide suite on the C and the
  Go editors, and on whimsy, whimsical and whimsical's debugging build, the
  Java, the Clojure and the Haskell editors 0 of 3,120 quick and 0 of 3,360
  wide each; through a pipe, the control, whimsy differs 33 and 61 times,
  the C 66 and 106, and whimsical, the Java, the Clojure and the Haskell
  never, since they start after one write of the keys has landed -- the keys
  in 16 pieces 10 ms apart (`MODES=trickle`) move whimsical 326 of 720 quick
  runs and 307 of 920 wide, the Haskell 547 and 464, and the JVM editors,
  which start after all 16 have landed, in 16 pieces 100 ms apart
  (`MODES=slow`) the Java 105 of 320 and 125 of 460, the Clojure 190 and 134
  (`doc/RUST.md`, `doc/SCHEME.md`, `doc/JAVA.md`, `doc/CLOJURE.md`,
  `doc/HASKELL.md`, *Under load*). **Both end with the heavy
  case** (`internal/suite/heavy.go`), the one that times: 5,000 lines, three
  substitutions and a `:g`, run on every editor of the run one at a time,
  required to answer as the reference does, each time reported beside the
  C's, and an editor over 25 times the C's time failing the run (one built
  already, `--haskell-bin`'s, is timed but not held to it: the `-O0`
  ghc-lisp caprice 81-84) -- measured,
  Rust 0.25-0.3, Go 0.5-0.6, Scheme 0.8-0.9, Haskell 1.0-1.3, Java 1.6-2.1, Clojure 3.4-3.9 (4.2-4.5 before
  its big functions were compiled sooner, `doc/CLOJURE-PROFILE.md`) since the
  parallel `:%s` (Java 2.3 and Clojure 9-10 before), and the Clojure 55 with the JIT's
  huge-method limit left on, which is what it refuses.

## What a file is called

**Code is `.go`, data is `lowercase.md`, prose is `UPPERCASE.md`**, and the three
mix freely in one directory: `internal/phase/036/` holds `edit.go` and `GOAL.md`. A
data file in Markdown puts its data in a FENCED BLOCK and its notes around it,
and the reader takes the fence and ignores the rest -- `internal/build`'s
`declared()` reads `internal/phase/001/delta.md` that way (the command rows phase 1
retires at the front, which phase 26 then deletes), and phase 37's edit embeds `internal/phase/037/musl-ctype.md` and
`musl-case.md` and splices their fenced C in byte for byte. What is left outside
the rule is what Markdown would only obscure: `src/slim.sha` and `src/upstream.sha`, one
digest each, read by `make`.

**A phase is a directory, `internal/phase/NNN/`**, its number -- its place in
the plan, 0 to 103 -- in three digits so that they sort: `GOAL.md`, which opens
`# Phase N — …` and says what the phase removes, why and what was measured (in
the numbers it was written under, which the line under its title gives), and
-- for the 79 phases whose cut is a program of their own -- `edit.go`, which
makes that directory **a Go package**, `pNNN`, registered as `whimN`
(`editlit.go` beside it where the literals are long). **A part** is a program a
phase runs among its steps that had a number of its own -- a compaction
group's earlier member, or a program the front calls -- in a directory inside
the phase's, `internal/phase/NNN/x/`: package `pNNNx`, registered as `whimNx`,
lettered in the order the phase runs them; there are 33. The other phases are
plan steps only (`internal/steps`). An edit is written in `crefactor/edit`'s verb set
(`edit.E`, `edit.Ph`) and `internal/whim/vimtext`'s shared shapes, registers
itself with `internal/phase` (`phase.Register`) in an `init()` -- or, for a
phase converted to the graph (phases 24, 33, 58, 64 and 77 and parts 38a and 86a so far, `doc/GRAPH-MIGRATION.md`),
is written on `crefactor/graph`'s editor and verbs and registers with
`phase.RegisterGraph`, its text program replaced -- and
`cmd/whim/phases.go` is what links them in: it imports every phase blank.

**`doc/GOALS.md`** is what holds for every phase, in the old numbers but for its
section on the pipeline as it runs: Part I (phases 0-82: the charter,
what was measured and the declared delta -- a record now, see its opening -- the rules, the sweep, the concept index,
an index of the phases) and Part II (phases 83 on: the core's charter, what is
measured from 83 on, the core's rules -- cited as *core rule N*, and holding beside
Part I's -- *Phases 83 to 128 as they stand*, *Adding a phase*, an index of the
phases, and an appendix: the plan phases 83 onwards were built from, its sections
numbered II.1-II.6 and cited `GOALS.md II.4c`), then *What comes next*.

`src/whim-vim.c` is **produced, not edited**. `src/slim.sha` records the digest of the
`slim-vim.c` the committed `whim-vim.c` came from.

**There is no Python and no agent here.** Every phase is a program; a phase that
refuses stops the pass with its own report, and there is no tier below it to fall
through to. arbace/slim-vim keeps both, for its own pipeline.

## Layout

```
cmd/whim/         the toolset, every tool a subcommand: go tool whim <subcommand>
                   (README.md: each tool, and what each retired script became)
internal/          whim's Go: cut (the cutters), steps (every transformation a phase names, as
                   one table), treepilot (doc/C-LISP-TREE.md's pilot,
                   outside the plan), graphcheck (crefactor/graph's sweep
                   held to the pipeline's on every phase, and the phases
                   with a graph step held to their snapshots, timed, the
                   graph snapshots read back, and the graph's verbs on the
                   snapshots -- an initialiser element replaced, phase 15
                   written on them, the include edits of phases 43, 73, 88
                   and 99 and the line on every snapshot, B2b's rows,
                   enumerators and renames held to argvfront, filefront and
                   phase 51a, eight phases' literal C spliced by FRAG, PARAM,
                   RETYPE and MOVE on phases 31, 57, 66, 78, 80, 83 and
                   86's parameter and on whim-vim.c's functions:
                   GRAPH_SNAPS), build (whim's pipeline: the plan -- what each
                   phase does to the source -- and the Config that tells the
                   generic driver whim-vim.c, .cache/boundaries, vim's sweep,
                   @state, @minmax and phase 1's delta.md), cmdtab (the Ex command
                   table: its names), score (bytes and
                   symbols, the input beside the product)
crefactor/         the generic C machinery, A GO MODULE OF ITS OWN
                   (github.com/arbace/go-whim/crefactor, its own go.mod; this
                   module requires it and replaces it with ./crefactor), knowing
                   no code base: it cannot import this module, so the boundary
                   between generic C and vim is the compiler's. cc/: the forked C
                   front end (c23.go: the C23 it adds; check_export.go:
                   Check, the type check of a parsed tree, kept whatever it
                   resolved). c23conf/: its
                   conformance test, a file per C23 feature, held to gcc 15
                   through the parser, cemit and C-lisp (doc/C23.md).
                   cemit/: the canonical printer (recover.go: its
                   parse, include lines and macro recovery, exported for
                   clisp). clisp/: C-lisp, C as s-expressions and back, byte
                   for byte on canonical text (`whim c2lisp`, `whim lisp2c`;
                   SPEC.md, every form; doc/C-LISP.md), and a tree API on
                   its forms (tree.go: cursors, edits, an atom index;
                   pattern.go: patterns as forms; scope.go: a resolver of
                   C's name spaces and scopes, untyped; Options.Origin tells a
                   caller the cc node each form came from; printnode.go an
                   expression's or items' C alone). graph/: the
                   program as one resolved, typed graph (doc/GRAPH.md):
                   import.go cc's parse and check into C-lisp's forms as
                   nodes with ids, refers edges (members by type) and typed
                   edges, types.go the type and external nodes, cview.go the
                   C view, byte for byte, lisp.go the graph as Lisp and its
                   reader, collect.go, keys.go and cut.go the sweep as
                   garbage collection, Prune's rules on the nodes, edit.go
                   the editor (Delete, Replace, Insert, Retarget, checked
                   where made: places, containment, dangling records, the
                   id rule, types kept or cleared), fallout.go the fall-out
                   closure over dangling edges (call, store, through,
                   value, constant folds, empties; a program's specifics
                   through FallOutOptions, which internal/whim/graph.go
                   gives vim's), pattern.go clisp's patterns on nodes,
                   names.go what the forms say of a declaration, exported;
                   editcollect.go the collection through the editor (its
                   index kept, the act logged); and the library the phases
                   are converted on (doc/GRAPH-MIGRATION.md, B0): verbs.go
                   crefactor/edit's verb set on the graph (Verbs: the
                   scopes, the counted acts by C-lisp pattern, the text's
                   report and refusals), build.go new nodes from C-lisp
                   templates at a place (names resolved as the importer
                   resolves them, members by type, typed edges where they
                   follow), textq.go the text's assertions (its counts on
                   the scope's C view, and the edges' answers), unwrap.go a
                   branch's items spliced where no name clashes, and a run
                   of items replaced; its tests hold each verb to its text
                   verb's C, on graphs read back from Lisp; include.go
                   (B2e) the include forms added, deleted and moved under
                   the extern rule -- every name the file takes from the
                   headers provided by an include above its first use, no
                   header's macro over the file's own names below it -- and
                   the first include form as the line (Core, Host, InCore,
                   top-level forms moved across it), headers.go what one
                   header provides, parsed by cc alone;
                   and B2b's:
                   renum.go an enum's enumerators deleted, moved, inserted
                   under a values policy (held, renumbered and reported,
                   pinned as the sweep pins), initrow.go a table's rows
                   deleted, inserted, reordered with every position the
                   file names said again (subscripts by constants, the
                   index enumerators and permutations a cut names),
                   rename.go a declaration and its uses respelled by
                   edge, a use retargeted to another spelling, a string
                   literal respelled whole, named by the cut;
                   B2a's
                   frag.go, C text made nodes in context (FRAG: SpliceC
                   at a Spot, on a synthesized unit -- the C view pared to
                   what a fragment sees, the fragment between markers --
                   that cc parses, checks and imports, its nodes kept and
                   their edges carried over, the uses it now declares
                   retargeted, holes `$x`), fragverbs.go its verbs
                   (BodyC, LiteralC, ReplaceC, TopBeforeC, ...) and
                   Together (a phase's literals in one unit), clone.go
                   (CLONE), macrox.go (MACROX: an invocation's name and
                   arguments, expanded through FRAG), same.go (SameGraph:
                   a graph against the import of its C view, ids aside);
                   and B2c's
                   capabilities: param.go PARAM (a parameter dropped from
                   every declaration and every fn form of its family --
                   pointers, members, typedefs -- with the argument at every
                   call, every use of what changes type a call, a flow to
                   the same new type or a test, else refused; DropArg,
                   ParamToLocal, AddParam; the editor's one sanctioned path
                   past "a function's parameters are its type"), retype.go
                   RETYPE (a declaration's type in every declaration of it,
                   a typedef's reach, the typed edges above every use typed
                   again or cleared into Untyped), move.go MOVE (items and
                   nodes moved, ids and edges kept, refused where a use
                   would no longer resolve or a jump would rebind),
                   typeedit.go what they share (type nodes interned,
                   types derived, Rederive), b2cverbs.go their verbs;
                   deadstmt.go the statements after a jump (StmtTerminates
                   on nodes, part 86a's rule);
                   its corpus tests run on GRAPH_CORPUS. graph/view/: its
                   views, read-only (`go tool whim view`): index.go the
                   edges the other way round and the roots by name, view.go
                   the steps, the specs and the tree built with links for
                   revisits, named.go callers, callees, uses, member, type
                   and def, print.go the tree as C-lisp, ids on demand.
                   sweep/: the closure.
                   reach/: what nothing reaches, as a partition with gcc as its
                   control -- a reporter, `go tool whim reach FILE`; it deletes
                   nothing. ccx/: a core's pointer casts and evaluation order,
                   partitioned. dead/: funcreach and gcc's unused warnings.
                   pipeline/: the driver -- Phase, Step, Plan, Run, Advance,
                   Check, the snapshots, Seed -- told everything through a
                   Config; hybrid.go its text and graph steps, the
                   conversions between them, and the graph snapshots. xform/: the generic transforms (fallout.go: the
                   fall-out closure, what a drop's cut leaves unwritten
                   folded, and what that makes constant). edit/: the C-text
                   substrate that was cutil, and the one verb set -- E, every act
                   counted (driver.go, blocks.go); Ph, the driver of the phases
                   whose cut is a computation; the counted acts both and
                   internal/cut's `ed` are written on (counted.go); and in
                   shared.go the generic helpers more than one phase uses.
                   togo/: the C-to-Go translator internal/gen runs, and its
                   instance pass (instance.go: the state a struct's fields,
                   the functions reaching it its methods), its Java
                   backend (java*.go: `whim skel ... -java F.java`,
                   doc/JAVA.md), and its Clojure backend (lower*.go, the
                   lowered form -- a function as basic blocks, `-lowerc
                   F.c` prints it back as C -- and clj*.go: `whim skel ...
                   -clj F.clj`, doc/CLOJURE.md; clj_tidy.go reads the
                   printed forms back and takes their noise out, as
                   java_tidy.go the Java's redundant parentheses), and its
                   Haskell backend (hs*.go: `whim skel ... -hs F.hs`, the
                   lowered form printed on C's own memory, doc/HASKELL.md:
                   hsshape.go the blocks in place, hsnames.go a name for
                   each binding, hsnamed.go the C's names for objects,
                   members and constants, hstypes.go typed pointers,
                   hsout.go out-parameters printed as results, hsstruct.go
                   struct locals printed as values, hssplit.go the
                   modules), and what it decides about the C with the
                   Scheme and Rust backends, named for what it computes
                   (cfacts.go: the cfacts each makes with newCFacts -- the
                   segment's layout, the tags' typedefs, the callers
                   written by hand
                   -- outparams.go the out-parameters, structvalues.go the
                   struct locals that are values, effects.go the pure
                   functions and those without the editor; and cquery.go
                   the small questions more backends ask), and
                   its Rust backend (rs*.go: `whim skel ... -rs F.rs`, C's
                   memory natively, doc/RUST.md: rs_types.go the #[repr(C)]
                   types and the layout listing, rs_fn.go C's statements --
                   a goto a labeled block, a switch a match or a ladder of
                   labeled blocks -- rs_expr.go the expressions, rs_init.go
                   the initializers, rs_lower.go the lowered form where
                   labeled blocks cannot say a function; and the idioms,
                   doc/RUST-IDIOMS.md: rs_fx.go the safe functions, closed
                   over the calls of the cfacts it makes, whose effects.go
                   says which take the editor, rs_range.go the arithmetic
                   that provably fits, rs_hoist.go side effects out of
                   expressions, rs_refs.go references where provable,
                   rs_defer.go locals declared with no value,
                   rs_const.go read-only pointers as *const), and its Scheme
                   backend (scm*.go: `whim skel ... -scm F.ss`, C's memory
                   as one bytevector, the lowered form in caprice's shape,
                   doc/SCHEME.md: scm.go the library, its image and its
                   names, scm_fn.go the blocks as local procedures and named
                   lets, scm_expr.go the expressions in C's order,
                   scm_tidy.go each function read back as forms, rewritten
                   by rules that see its scopes and laid out again,
                   scm_layout.go the layout listing; its cfacts are the
                   Haskell's analyses, shared). Its tests
                   run in it: `cd crefactor && go test ./...`
internal/whim/     what the generic side is told about vim: profile.go (the
                   sweep), xform.go, analysis.go (dead's roots, reach's and ccx's
                   names), gen.go (togo's profile, whim.Gen), graph.go
                   (crefactor/graph's collection and fall-out closure),
                   and vimtext/ (what more than one phase uses that knows vim: the
                   buffer walks, Key, the command table's residue check, the
                   prototype and #include shapes part 49b and phase 49 share, and the
                   Python-port helpers of phases 54-56; a shape one phase alone uses
                   is in that phase's shapes.go)
internal/phase/    the registry the phases' programs join (registry.go, query.go),
                   and the phases: NNN/ (GOAL.md, and edit.go where its cut is a
                   program; NNN/x/ its parts; cmd/whim/phases.go imports each
                   blank), archive/NNN/ (the records, under their old numbers:
                   a GOAL.md each, for a phase that edits nothing now),
                   numbers.md (every live directory's old number and new) and
                   renumber/ (the program that moved them, spent), STAGES.md (the record of
                   the stages there were, and of the measurement that retired them --
                   prose, not a manifest a program reads) and boundaries.md
                   (every boundary's lines, entity counts, binary and nm -u, as
                   `go tool whim build --keep D` and `measure D` give them)
editor/            the core in Go, package editor, a library of instances: an
                   Editor's fields are the C's file-scope objects and the
                   functions reaching them its methods (receiver ed); editor.go
                   GENERATED (make editor/editor.go; never edit it); by hand,
                   its runtime crt.go, format.go (vim_snprintf), chunks.go (a loop
                   over lines in parallel chunks: match_lines), and host.go --
                   the Host interface an Editor runs on, editorHost (the
                   host's state, which Editor embeds), New and Main(host,
                   args); term/ is the terminal host and cmd/whim/ the
                   launcher bin/whim is
internal/gen/      the generator of editor/editor.go (`go tool whim gen`; `whim
                   skel` runs it by hand, with -bodies for the bodies alone):
                   crefactor/togo, the C-to-Go translator, which names
                   nothing in vim, told vim's names by internal/whim/gen.go
                   (whim.Gen), which cmd/whim hands it. Beside its docs:
                   pre/ (`whim pre`: crefactor/ccx's partitions on an
                   editor.c), sigs.md, CONVENTIONS.md and FINDINGS.md
braaam/           the editor in Java (doc/JAVA.md), by hand but for editor/,
                   package whim.editor, GENERATED by the Java backend and
                   tracked (whim gen; never edit it): Editor.java (the
                   fields, their initial values, the methods: 55,415
                   lines), Constants.java (imported statically), and a file
                   for each struct class and function interface, 173:
                   rt/ the runtime it is written against (package whim.rt:
                   BytePtr and its kin, Ptr<T>, Rt, Ga -- the growarray --
                   and Struct; SelfTest.java, which crefactor/togo's tests
                   run); host/ (package whim.host) the
                   Host interface, Exit, Printf (vim_snprintf, format.go's
                   port) and the terminal host Term and Signals (the Foreign
                   Function & Memory API and sun.misc.Signal); Whim.java the
                   glue (Editor's subclass, in whim.editor) and the
                   launcher's main (whim.editor.Whim); and
                   braaam.go, the Go that builds it all (`go tool whim java`,
                   make bin/braaam: lib/braaam/classes and bin/braaam; make
                   braaam.jar: the same as ./braaam.jar)
vijure/         the editor in Clojure (doc/CLOJURE.md), all by hand but the
                   namespace whim.editor, GENERATED by the Clojure backend and
                   tracked (whim gen; never edit it): src/whim/editor.clj,
                   which loads its functions from the four files of
                   src/whim/editor/ (clojure.core's own split: a file's
                   load() is one 64 KB method); src/whim/cljhost.clj the glue
                   (the C's 17 host functions, in kebab-case, the editor first, to braaam's
                   Host and Printf through interop), src/whim/cljmain.clj the
                   launcher's -main; vijure.go the Go that builds it (`go
                   tool whim clj`, make bin/vijure: AOT-compiled on braaam's
                   rt/ and host/ into lib/vijure/, merged with Clojure's jars into
                   lib/vijure/vijure.jar, an AOT cache trained, and bin/vijure;
                   make vijure.jar); testdata/standin/ a hand-written
                   stand-in for whim.editor, which its tests and the suite's
                   (TestClojureStandIn) build and run
caprice/           the editor in Haskell (doc/HASKELL.md), by hand but
                   Caprice/Editor.hs, its hs-boot and Caprice/Editor/ (Defs
                   and the parts), GENERATED by the Haskell backend and
                   tracked (whim gen; never edit them):
                   rt/Caprice/Rt.hs the runtime (C's memory, raw: the
                   segment, the frame, function pointers as indices into a
                   table the editor carries, the parallel chunks), host/Caprice/Host.hs the Host interface
                   (a record of functions, editor/host.go's) and the C host's
                   17 functions as glue to an editor's own Host and arena,
                   Term.hs the terminal host (termios, poll, the signals told
                   to every terminal alive), Run.hs `run host args` (an
                   editor, run to its end: several may run at once) and
                   Printf.hs (vim_snprintf, format.go's port), Main.hs the
                   launcher; caprice.go the Go that builds it (`go tool whim
                   caprice`, make bin/caprice: GHC into lib/caprice and the
                   program bin/caprice); testdata/instances/Main.hs four editors at
                   once on hosts of its own, which its test compiles against
                   lib/caprice
whimsy/            the editor in Rust (doc/RUST.md), the crate whimsy (std
                   alone, offline), by hand but src/editor.rs, the module
                   editor, GENERATED by the Rust backend and tracked (whim
                   gen; never edit it): src/rt.rs the runtime (VArg, decay,
                   pdiff, the parallel chunks), src/host.rs the Host trait
                   (editor/host.go's) and the C host's 17 functions as glue
                   to an editor's own Host and arena, and run(host, args),
                   src/term.rs the terminal host (termios, select, real
                   signal handlers over extern "C" libc), src/printf.rs
                   (vim_snprintf, the C's ported), src/main.rs the
                   launcher; whimsy.go the Go that builds it (`go tool whim
                   whimsy`, make bin/whimsy: cargo into lib/whimsy and the
                   program bin/whimsy); whimsy_test.go the layout test (gcc's
                   and Rust's layouts held to the backend's listing) and
                   testdata/instances/main.rs, four editors at once, which
                   its test compiles against lib/whimsy
whimsical/         the editor in Scheme (doc/SCHEME.md), R6RS for Chez Scheme
                   10.3, by hand but whimsical/editor.ss, the library
                   (whimsical editor), GENERATED by the Scheme backend and
                   tracked (whim gen; never edit it): whimsical/rt.ss the
                   runtime (the editor's one bytevector, the accessors and
                   the C's names for it, C's integers, frames, the parallel
                   chunks on fork-thread), whimsical/host.ss the host record
                   (editor/host.go's Host) and the C host's 17 functions as
                   glue, and run, whimsical/term.ss the terminal host
                   (Chez's foreign procedures into musl: termios, poll,
                   signals by signalfd), whimsical/printf.ss (vim_snprintf,
                   the C's ported), main.ss the launcher, c/ the main and
                   the boot files' assembly; whimsical.go the Go that builds
                   it (`go tool whim whimsical`, make bin/whimsical: Chez at
                   optimize-level 3 into one vfasl boot file, linked with
                   Chez's kernel into bin/whimsical; --debug, the debugging
                   build at level 2, into bin/whimsical-debug); whimsical_test.go the
                   layout test and testdata/instances/main.ss, four editors
                   at once, run on lib/whimsical
Makefile           the whole build: fetches the input, runs the pipeline, builds the
                   binaries and the editor
src/               the input and the product: slim-vim.c (fetched, not tracked),
                   whim-vim.c (produced, tracked), upstream.sha and slim.sha;
                   their binaries are bin/slim-vim and bin/whim-vim
doc/               GOALS.md (what holds for every phase), PHASES.md (the phases'
                   numbers, new and old), AGENDA.md (what is not
                   done, in order, and what was declined, with why), GO-IDIOMS.md (how
                   the Go editor could be idiomatic, measured and ranked; done or
                   declined), JAVA.md (the Java backend: its design and milestones),
                   JAVA-IDIOMS.md, CLOJURE-IDIOMS.md, HASKELL-IDIOMS.md, RUST-IDIOMS.md and SCHEME-IDIOMS.md (how the
                   Java, Clojure, Haskell, Rust and Scheme editors could be idiomatic,
                   measured and ranked; surveys; done: CLOJURE-IDIOMS.md's items 0-3, 4's tables (its messages declined), 5 in part, 6, 7, 8's headroom (9 declined), and
                   JAVA-IDIOMS.md's items 1-3, 4's masks, 5's tables, 6.1 (phase 93) and 11's files, HASKELL-IDIOMS.md's all but what it declines, RUST-IDIOMS.md's items 0-16 (17 declined), SCHEME-IDIOMS.md's items 1-18),
                   PIPELINE-COMPACTION.md (which phases could be dropped, merged,
                   split or reordered, measured byte for byte), CLOJURE.md (the
                   Clojure editor), CLOJURE-PROFILE.md (where its time goes in
                   the heavy case, beside the C's, and the change it chose), HASKELL.md (caprice, the Haskell editor:
                   its design and what was measured), RUST.md (whimsy, the Rust
                   editor: its design, its milestones and its idioms),
                   SCHEME.md (whimsical, the Scheme editor: the survey and measurements that chose Chez Scheme, its design and its milestones), IR.md (where a feature goes in the chain,
                   and an intermediate representation: an assessment),
                   IR-SCHEMA.md (that representation sketched against togo:
                   what is shared and duplicated, a schema, a migration path),
                   PARALLEL-SUBSTITUTE.md (how much of a :%s is matching,
                   measured on every editor, and what stands in the way), GO-LISP.md
                   (the Go editor in go-lisp syntax: an experiment, and make
                   editor.lgo), GHC-LISP.md (caprice's core as one ghc-lisp
                   module, converted, checked and compiled: make caprice.hsl),
                   C23.md (the front end against C23 -- ISO/IEC 9899:2024,
                   N3220 -- the conformance test, how it judges, its score
                   before and after, what stays out), C-LISP.md (whim-vim.c
                   as s-expressions, crefactor/clisp:
                   back byte for byte and compiled to the same binary: make
                   whim-vim.lc), C-LISP-TREE.md (a pilot: phases editing
                   that tree instead of the text -- DropLocal and phase 24
                   rewritten in internal/treepilot, byte for byte on their
                   14 phases, measured; not to migrate) and GRAPH.md (a
                   design, steps 1-4 built and measured, step 5's stage A
                   -- text and graph phases side by side -- built, and its views
                   read-only: the program as one resolved, typed graph,
                   cuts as deletions whose fall-out is the constraints'
                   closure, its views trees printed as Lisp, and an editor
                   over them), GRAPH-MIGRATION.md (every phase classified
                   for the move to the graph, the batches that do it, and
                   B0's library as built)
```

**The toolset is `go tool whim`**: `go.mod` declares `cmd/whim` as a tool, so Go
builds it, caches it and rebuilds it when any `.go` moves; every tool is a
subcommand, `go tool whim <name>`, and the `Makefile` calls it the same way.
**The C front end is a fork**, `crefactor/cc`: modernc.org/cc/v4 v4.29.7 with
the C23 its parser lacks added -- `[[...]]` attributes in every position,
`constexpr`, `_BitInt`, `typeof_unqual`, `static_assert` without a message,
digit separators, `u8'a'`, storage classes in a compound literal, a label at
a block's end -- and three faults corrected, tracked as ordinary source
(`crefactor/cc/README.md`, which says how to diff it against upstream;
`crefactor/c23conf` holds it, cemit and C-lisp to gcc 15, 49 of 49 C23
features, 22 before: `doc/C23.md`). It was composed at build time under `.cache/gofork/`
before, because a patched `vendor/` fails `go mod verify`; a fork under its own
import path has neither problem. Measured: with the patch reversed, `whim
parse whim-vim.c` says *unexpected `<EOF>`, expected `}`*, and `internal/gen` writes
the same `editor/editor.go` byte for byte either way.

## Build

```sh
make                 # all: through whim-vim.c (produced only when slim-vim.c moved)
                     # and the generated editors, bin/whim (the Go editor), the
                     # C binaries bin/whim-vim and bin/slim-vim, bin/braaam and
                     # braaam.jar, bin/vijure and vijure.jar (packed from its build),
                     # bin/caprice, bin/whimsy, bin/whimsical
make whim-build      # the 104 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same, required to give the committed bytes back, every boundary compiled
make whim-editor-check # refuse a tracked editor.go, braaam/editor/, editor.clj, Editor.hs, editor.rs or editor.ss that is not what the generator writes
make whim-test        # the quick suite: 80 key sessions, required to behave as HEAD's does
make whim-test-wide   # the optional wide suite: 240 cases, keys, Ex commands, argv, a terminal
make bin/braaam       # the editor in Java: whim.editor generated, compiled, and a launcher
make whim-test-java   # the quick suite with the Java editor too (whim test --java; --wide --java)
go tool whim java --same-classes  # HEAD's braaam/editor/ and the tree's compile to the same code: a spelling change's proof
make bin/vijure       # the editor in Clojure: whim.editor generated, AOT-compiled, a launcher (CLJ_EDITOR=F: F's)
make whim-test-clj    # the quick suite with the Clojure editor too (whim test --clojure; --wide --clojure)
make bin/caprice      # the editor in Haskell: Caprice.Editor generated, compiled by GHC (three minutes when the core moved; its time and peak printed)
go tool whim caprice --lint  # the same, then ghc -Wall's warnings on the generated module, by flag (0 now)
make whim-test-hs     # the quick suite with the Haskell editor too (whim test --haskell; --wide --haskell; --haskell-bin P: a program built already)
make bin/whimsy       # the editor in Rust: the module editor generated, compiled by cargo, offline (forty seconds when the core moved; its time and peak printed)
go tool whim whimsy --lint  # the same, then rustc's warnings on the generated module, its #[allow]s and #[expect]s taken out, by lint (487 now: 482 the C's names, 5 dead stores the module expects)
make whim-test-rs     # the quick suite with the Rust editor too (whim test --rust; --wide --rust)
make bin/whimsical    # the editor in Scheme: the library (whimsical editor) generated, compiled by Chez (half a minute when the core moved; its time and peak printed)
make whim-test-scm    # the quick suite with the Scheme editor too (whim test --scheme; --wide --scheme)
go tool whim whimsical --debug  # its debugging build, bin/whimsical-debug: optimize-level 2, safe, inspectable (39 s, 1.1 GB)
make editor.lgo       # the Go editor as one go-lisp file, compiled (GOLISP_ROOT=.../go-lisp; doc/GO-LISP.md)
make caprice.hsl      # the Haskell core as one ghc-lisp module, checked, compiled, and the program run on the quick suite (GHCLISP_ROOT=.../ghc-lisp; doc/GHC-LISP.md)
make whim-vim.lc      # the C product as s-expressions (C-lisp): back byte for byte, compiled to bin/whim-vim's bytes (7 s; doc/C-LISP.md)
go tool whim graph --check FILE  # the graph of FILE: its C view FILE byte for byte, written as Lisp and read back the same graph (doc/GRAPH.md)
go tool whim view callers F      # a read-only view of whim-vim.c's graph as Lisp: callers, callees, uses, member S.M, type T, def; 0.07 s once .cache/graph holds it (doc/GRAPH.md)
make go-test          # the Go packages' tests, this module's and crefactor/'s (go test ./... skips it)
make bin/whim-vim    # the C product's binary
make bin/slim-vim    # the input's binary, with the same one line
make score           # bytes to store and symbols to provide, input beside product
make help            # every target, with a line each
```

- **One path.** `make whim-build` is what a moved upstream runs:
  `internal/build`'s plan -- each phase's steps (`internal/steps`), **the sweep,
  after every phase** (there are no stages), and **the canonical print of what is left**
  (`crefactor/cemit`: one spelling per construct, and NO COMMENTS, of any kind;
  for a phase that ends on the graph, the collection and the C view, the same
  text -- a step is a text step or a graph step, below),
  so every boundary that is C is in the one spelling phase 0 seeds with -- C23's,
  `nullptr` and `usize` and the attributes included, since phase 0 runs its
  parts 0a's rename, 0b's variadic collapse and 0c's attributes on the canonical
  input and every later phase is written for them, 0c's 37 `[[fallthrough]];`
  among them (20 reach the product; the print wrote them `;` until the front
  end held them, `doc/C23.md`) -- applied
  in one process, in memory. **Its log is a line a phase** -- the name, the acts its
  steps reported, the lines its edits and the sweep took, the lines left, the
  time, under a heading for each block (`block  d02-outside`); `-v` writes every act, and a phase that refuses writes its whole report
  before the reason. Measured: 104 phases, **348 s** with every boundary compiled after (936 s of CPU, gcc's included), 77,634 lines, at a load of 3-13 (main before step 5: 352 s and 958 s
  the same hour; 451 s and 563 s of CPU under a load of 54-62; 815 s and 1,335 s before the profile of `doc/PIPELINE-REFORM.md` §7, step
  9, and the regexps and phase 43 made cheaper in step 11, and phase 43 guarded and the cutters made cheaper in step 12; `--cpuprofile F` writes one). A
  whole run keeps every boundary in `.cache/boundaries/` (qNNN.c), and beside
  the boundary before each phase that begins on the graph the graph it
  handed that phase, as Lisp (qNNN.g, headed by qNNN.c's digest: eight now), and seals the
  set with the input's digest (`manifest`).
- **The sweep is one closure** (`crefactor/sweep`'s `Prune`): the text parsed
  (`cc.Parse`, no type-checking, no gcc), everything reachable from `main` and
  the static_asserts found by name in C's three name spaces, and everything else
  cut -- functions, objects, prototypes, typedefs, tags, members, enumerators,
  the locals nothing reads (resolved by the parser's scopes), and a
  `fallthrough` attribute statement that precedes no case label, gcc's test
  (`fallthrough.go`; on whim's pipeline it finds none). Its guards: no
  member goes while `ml_recover` is defined; a struct filled by position keeps
  every member; nothing is emptied; an enumerator's deletion pins the survivor
  after it to its value. About 1.1 s of CPU a phase (135 s over a run in order, profiled). It replaced six deleters looped around
  gcc, and keeps nothing they cut (measured on the product: 5,657 entities
  against 5,662, the five it adds all unused).
  It needs every text it is handed to PARSE, and every one does.
- **`whim-build-check` runs phase by phase, in parallel.** With a sealed set of
  snapshots for the input on disk, it checks that phase 0 seeds the input into
  q000 and that EVERY phase N, run on q(N-1), gives qN -- all phases at once,
  `--jobs N` at a time (default: every core) -- and that the last snapshot is the
  committed `whim-vim.c`. Measured: **105 s** under a load of 54-69 (118 s at 107-139, 76-77 s at a
  lighter one), 103 links 64 at a time, bound by
  the machine's load and no longer by one link (in order, the front's three closures, phases 1-3, take 37, 25 and 33 s, phases 4 and 5 12 and 7 s, the seed 12 s and phase 43 15 s; phase 1 alone was 89 s), against 451 s in
  order then (348-352 s now, at a load of 3-13); and a phase whose program was changed -- on purpose (a control),
  or phase 96's while it was being written -- is named and fails the check. That is
  the induction a run in order walks, so it proves the same thing; a phase whose
  program changed breaks its own link and is named. With no snapshots of this
  input it runs the pipeline in order, which writes them. A phase that begins
  on the graph begins on q(N-1).g read back (75-90 ms; a tenth of the
  import, which it falls back to where there is no graph snapshot of that
  text). Measured since: 73 s (61 s the links, 10 s the compiles) at a load
  of 10-16, main's 74 s beside it. **Either way it then
  compiles and links every boundary**, q000-q103, with the one compile line
  (`internal/build`'s `compileBoundary`, the driver's `Config.Compile`), 64 at
  a time, an error failing the check and naming the boundary -- warnings
  allowed, as for the product, and none printed now. Snapshots q004-q029 did
  not compile until part 4d and phase 22 were fixed (their `GOAL.md`s): a
  boundary reproduced is not yet a program. Measured: 8-10 s of a parallel
  check of 71-73 s at a load of 7, 6.6 min of CPU; `-fsyntax-only` would be
  under a second but cannot see a function declared and defined nowhere, which
  only the link does; the control, the unfixed phases on their old snapshots,
  names all 26. It proves the text,
  not the editor; `make whim-test` is what sees the editor (see *What this is*).

- **`editor/editor.go` is generated** (`go tool whim gen`, `internal/gen` on the
  core it cuts from `src/whim-vim.c` into a directory of its own -- nothing is
  written under `src/`) and tracked. `whim-build` writes it after producing `whim-vim.c`;
  `make editor/editor.go` writes it on its own; `whim-editor-check` refuses a
  tracked file that is not what the program writes. `go tool whim gen` writes only
  when the content differs and never runs make: through `whim-vim.c`'s rule it
  could start a build. The binary is `bin/whim`. **`braaam/editor/`, the
  Java editor's package, is generated and tracked the same way**, a stale
  or extra file refused, by the same `whim gen`, from the same
  core (`crefactor/togo`'s Java backend), held to the same check, and
  refused outright if the backend refused any part of the core -- **and so are
  `vijure/src/whim/editor.clj`**, the Clojure backend's, **and
  `caprice/Caprice/Editor.hs`** with its hs-boot and its parts
  (`caprice/Caprice/Editor/`), the Haskell backend's, **and
  `whimsy/src/editor.rs`**, the Rust backend's, **and
  `whimsical/whimsical/editor.ss`**, the Scheme backend's.

- **The compile line is one line**, `gcc -O0 -fno-stack-protector -static -no-pie
  -s`, for the input, the product and every boundary: an ordinary static
  executable, no stack protector. `internal/build/compile.go` states it for the
  tools (`FlagsFor`, `score`, `measure`), and the `Makefile` as `CFLAGS`/`LDFLAGS`
  for its two binary rules. It moved at the old phases 83 (`-no-pie`) and 84
  (`-fno-stack-protector`) until those became the line for all; the two are
  records now.
- **No `-g`**, so a formatting change leaves the binary byte-identical -- the
  cheapest comparison there is. `SOURCE_DATE_EPOCH=0` pins `__DATE__`/`__TIME__`
  when two builds are compared.
- **`gcc` exits 0 with warnings**; the dead-code sweep's compile is judged by
  whether it prints *nothing*, never by its status.

## Temporaries

Every temporary goes in `.tmp/` (gitignored), never the shared `/tmp`: the
`Makefile` exports `TMPDIR` there, so `mktemp`, Go's `os.MkdirTemp` and a
build's work tree all land in it. **Worktrees go in `.tmp/worktrees/`** -- a
subagent's too: `git worktree add .tmp/worktrees/<name>` before launching it,
rather than an isolation flag that chooses its own place, because nothing of
ours belongs inside `.claude/`. Outside `make`, run with `TMPDIR=$PWD/.tmp`.

## The pipeline

A phase is a function of the tree it is handed, so the pipeline is
`p_N = f_N(p_{N-1})` -- 104 of them, in order, numbered 0-103 by their place
in the plan (`TestPlanNumbers` holds the plan to it; `doc/PHASES.md` maps the
numbers they were written under, 0-184 with gaps). 48 phases of the old
numbering that edit nothing any more are records only, a `GOAL.md` each in
`internal/phase/archive/` under the old number (`doc/PHASES.md` lists them and
says where each one's work went): most because the front runs their cut now,
168 because `GotoTail` takes its gotos (`doc/PIPELINE-REFORM.md` §7). 33
programs that had numbers of their own run inside another phase, as its parts:
the seed's three (0a-0c), the programs the front calls (2a; 3a-3f; 4a-4f and
5a-5d, each on the text the phase before swept), and the earlier members of the
same-purpose groups of `doc/PIPELINE-COMPACTION.md` §3d that run as one phase,
the group's last: 15, 38, 49, 51, 71, 74, 76, 77, 86 and 87 (of the two other
groups still running before, 72-73 are phase 5's parts 5b and 5c, and 51-53
are records). One program runs before its own
phase: phase 2's front calls `whim18`, whose package's second entry,
`whim18kp`, runs in phase 18. **There is no memoize**: its key
was the input boundary's digest and the implementation's together, so a moved
`slim-vim.c` missed every entry by construction.

- **The driver is generic, the plan is whim's.** `crefactor/pipeline`
  runs a plan (Run, the parallel Check, Advance, the snapshots, `--keep-going`)
  and knows no code base; `internal/build` hands it a `pipeline.Config` -- the
  plan, the op table (`internal/steps`) and its graph table (`graphOps`), the
  collection's options (`whim.GraphCollect`), the work file `whim-vim.c`, `SnapDir`
  `.cache/boundaries`, the sweep's options (`internal/whim`'s `Profile`), the
  `@state`/`@minmax` arguments and phase 1's `delta.md` -- and keeps its old
  names (`build.Run`, `Check`, `Advance`, `Options`) as wrappers.
- **There are no stages.** Every phase is its steps, the sweep, and the
  canonical print; a `sweep` step inside a phase's steps is for an edit that
  reads its own earlier steps' text swept.
- **A step is a text step or a graph step** (`Step.Graph`; doc/GRAPH.md,
  *Step 5 as built*). The driver holds the program as text or as
  `crefactor/graph`'s graph and converts only where the kind changes (the C
  view before a text step, an import before a graph step); a `sweep` is the
  collection where the graph is held, and a phase that ends on the graph is
  collected and printed by the C view, cemit's text byte for byte. The graph
  goes on to the next phase only when that phase begins on the graph, with a
  fresh editor, so a phase takes one path in order and in the check. On the
  graph now: every `droplocal`, phases 24, 58 and 64, phase 77's own step
  and part 86a; phases 8, 13, 14, 17, 24, 58, 64 and 86 begin on it. The rest is `doc/GRAPH-MIGRATION.md`'s. `internal/phase/STAGES.md` is the
  record of the schedule there was, and of the measurement that retired it.
- `go tool whim build --to N --work D` leaves the tree after phase N; `--keep D` writes every boundary, and `go tool whim measure
  D` counts them (`internal/phase/boundaries.md`).
- **A binary is only ever the build of its source as it stands.** `bin/slim-vim`
  and `bin/whim-vim` stamp the digest of the `src/*.c` they were built from
  (`.cache/stamps/`); as make starts, and whenever a rule rewrites a source, a
  binary whose source no longer matches its stamp is deleted and not rebuilt --
  `make bin/slim-vim` or `make bin/whim-vim` builds it again.
- **A phase touches no shared state**: its scratch and its sweep's file are
  temporary directories of its own, which is what lets the check run phases side
  by side. Two WHOLE builds in one checkout would still both write
  `.cache/boundaries/`; a second one goes in a worktree.
- **The analysis tools report, they do not cut**: `go tool whim reach FILE` is
  what nothing reaches in a text, typed, with gcc as its control and struct
  casts held -- the survey instrument the sweep's closure grew from.
  Like the sweep, `crefactor/reach`, `crefactor/ccx` and `crefactor/dead`'s
  funcreach name nothing in vim: they are told it -- `ml_recover`, the
  allocators, the functions of bytes, the growarray, the unions'
  discriminants, `main` -- by `internal/whim/analysis.go` (`whim.Reach`,
  `whim.CCX`, `whim.Dead`), which their callers pass in -- reach's core/host
  cut and funcreach's floor of 100 definitions among them.

## The core and the host

`whim-vim.c`'s ten `#include`s are not at the top: **the first one is the line
between the editor core and its host**, marked by nothing else. `internal/whim`'s
`Cut` cuts there -- every translation cuts the core for itself (`whim gen`,
`whim java`, `whim clj`, `whim caprice`, `whim whimsy`, `whim whimsical`), and `go tool whim cut` prints it: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `GOALS.md` §II.4 is the
design. On the graph the line is `crefactor/graph`'s first include form
(`Editor.Core`, `Host`, `InCore`; doc/GRAPH-MIGRATION.md, *B2e as built*),
whose core prints as `Cut`'s text on every snapshot from q043.

## Adding a phase

`GOALS.md` Part II, *Adding a phase*, has the process; the next phase is 104.
The pipeline's goal is met; a phase now is for the Go editor, where the C is
the cause of what the generator cannot make idiomatic. What a new one takes:
`internal/phase/NNN/` with `GOAL.md`, and `edit.go` in package `pNNN` registering
itself as `whimN` if its cut is a program (a line in `cmd/whim/phases.go`); and
an entry at the end of `internal/build`'s `Plan`, numbered N, naming its steps
(the sweep follows every phase), and its row in `doc/PHASES.md`. Then `make whim-build` (the product moves, so the tracked `whim-vim.c`
and `editor/editor.go` are rewritten), `make whim-test` against the commit
before it (a phase that removes capability moves cases on purpose: name them),
and whatever further evidence the phase needs, stated in its `GOAL.md`.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
