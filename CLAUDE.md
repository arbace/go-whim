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
- **whim** (the `Makefile`) removes capability on purpose, phases 0-184 from 180,870
  lines to 77,634. It is two arcs, a coda, an empty phase, five for the Go's
  sake, the headers, the gotos, the parallel `:%s`, the out-parameters
  and struct locals as values, the C spelled plainly, and its flags bool:
  - **phases 0-82** (`GOALS.md` Part I) leave an editor with no runtime to
    install, 80,926 lines at q80, the last of them that edits (81 and 82 are
    records);
  - **phases 83-128** (`GOALS.md` Part II) turn it into an embeddable core:
    no filesystem, the host behind a line in the file, no libc the core names, the
    text a tree. `GOALS.md` Part II, *Phases 83 to 128 as they
    stand*, is the account read across and belongs there, not here;
  - **phases 129-162** (Part II too) remove from the core what transpiling it to
    Go (`editor/editor.go`, `internal/gen/FINDINGS.md`) had to work around. All but 142
    were meant to change nothing the editor does; 142 drops the build date from
    the version line.
    `internal/gen/FINDINGS.md` maps each finding to its phase.
  - **phase 163** printed the product in the one canonical spelling phase 0
    seeds with; it is empty now that every boundary is printed that way.
  - **phases 164-165** take what the Go's linters found dead in the C: the
    statements after a jump (164, a general rule) and six stores nothing reads
    (165, named). With the generator's own fixes, `go vet`, staticcheck and
    `gofmt -s` are clean on `editor/`.
  - **phase 166** declares `bool` the 278 core functions whose every return
    answers yes or no -- OK and FAIL included, OK being `true` -- and the
    locals, struct members and parameters that only ever hold an answer; `f() == FAIL` is `!f()`. So the Go says
    `if f()` where it said `if f() != 0`.
  - **phase 167** names the 153 constant key codes (`K_DEL`, `K_IGNORE`) the
    preprocessor's removal left as arithmetic; the binary is byte-identical.
  - **phase 168** is a record: it wrote `return x;` for each `goto` whose
    label marks `return x;`, which is phase 170's rule with a tail of no
    statements, and 170 takes them all (the reform's G, measured byte for
    byte).
  - **phase 169** drops every system header nothing needs, in one step:
    each `#include` is tried and kept out while the file compiles silently.
    Phases 82, 99 and 104 once did it piecemeal, and the phases between them
    asserted the counts it left; now they assert only that every directive is a
    contiguous `#include` and that they add and remove none.
  - **phase 170** copies a label's tail -- at most three statements and the
    return, or a void function's end -- over each of the 98 `goto`s that reach
    one (`crefactor/xform`'s `GotoTail`), and drops the 21 labels left
    unreached: 21 of the 41 functions with a `goto` have none left.
  - **phases 171-172** take the gotos a loop says: one to the statement after
    its own loop or switch is `break;`, one to where control goes next anyway
    is deleted (171, `GotoBreak`); a goto back to a label is a loop and
    `continue;` (172, `GotoLoop`): 4 breaks, 1 deleted, 2 loops.
  - **phase 173** (after the headers: it touches none) wraps the region a
    forward `goto` leaves -- to a label of a block that holds it, no loop or
    switch between -- in `do { ... } while (0);` and writes the goto `break;`
    (`crefactor/xform`'s `GotoBlock`); togo writes that do-while as Go's
    `switch { default: ... }`, a break leaving it as the C's does (`for { ...;
    break }` before, which staticcheck reads as a loop unconditionally ended). After 170-172: 50 gotos, 11 labels. The C keeps 49
    gotos, all in the core (185 before 170): each leaves a loop or switch and
    needs a flag or a state variable, which a Java backend need not have (a
    labeled break says them). The Go keeps the same 49, in 8 functions, from 163 in 34:
    togo writes no goto of its own (a continue that must reach a loop's end
    is `break contN` out of a once-loop around the body).
  - **phase 174** takes the address of a position's line and column out of
    the two functions that took it -- `mark_adjust_internal()`'s 13
    expansions of vim's `one_adjust()` macros become calls of two functions
    of the value, and `cursor_pos_info()`'s columns come back through locals
    -- so the Java and Clojure editors no longer box `pos_T.lnum` and `.col`
    (the Java's `[0]` reads 7,032 -> 4,874).
  - **phase 175** is that rule in general (`crefactor/xform`'s `MemberOut`):
    a call passing `&s->m` for its callee to read and write calls a function
    written once per callee and member that does it through a local --
    where the copy is provably the call's (nothing the callee can reach names
    that member, the pointer is kept nowhere, every address of the member is
    such a site): 4 members, `[0]` reads 4,874 -> 4,774; the rest held and
    reported (the option table's pointers; vim's error paths reach code that
    names the others).
  - **phases 176-177** are the parallel `:%s` (`doc/PARALLEL-SUBSTITUTE.md`).
    176 makes the regex engine's state -- `rex`, its stacks, the look-behind
    and brace variables -- one struct, `regengine_T`, handed down as a
    parameter from the four functions the editor calls the engine by
    (`crefactor/xform`'s `StateParam`: 16 objects, 43 functions). 177 lets
    the engine match one line handed to it and nothing else, failing where a
    match would need more (another line, the cursor, a mark, a message), and
    adds `match_lines`, which says for each line of a range whether it holds
    a match; `ex_substitute` skips the lines it clears. The C runs it line
    after line; the Go, Java and Clojure editors run it in chunks on every
    core, each on an engine of its own (a runtime body,
    `internal/whim/gen.go`). Exact whatever the pattern: nothing the editor
    does moves.
  - **phase 178** has `:g`'s marking pass ask `match_range` too, one search
    a line (`:g/\v(a|b)+c/d` at 500,000 lines: Go 9.1 -> 0.8 s, Clojure 106
    -> 5.6); **phase 179** returns from `ml_clearmarked` when nothing is
    marked -- its loop read line 0's slot, index -1, which the Go, Java and
    Clojure editors failed on for any `:g` that matched nothing.
  - **phase 180** has the C host's `host_time()` return `WHIM_TIME` when it
    is set (the Go and Java hosts do the same), and the suite sets it on
    every editor it runs: undo's "N seconds ago" counted wall-clock seconds
    crossed, and the JVM editors failed its cases now and then under load.
  - **phase 181** makes an out-parameter a value in and a value out
    (`crefactor/xform`'s `LocalOut`: a parameter `T *p` its callee only
    reads, writes and null-tests, every caller passing `&x` of a local
    nothing else reaches, x not read unsequenced beside the call; the value
    returned, or with the result a struct of them) and a local struct of
    scalars its members' locals (`StructScalar`): 94 out-parameters of 61
    functions (14 taking no value in), 66 structs. The Java's `[0]` reads
    4,802 -> 4,009, the Clojure's one-element arrays 637 -> 515; the Java
    takes a struct a call returns as it is, the Haskell as a tuple.
  - **phase 182** spells plainly what the preprocessor left
    (`crefactor/xform`'s `plainc.go`): gettext's identity `_()` is not
    called, each call its argument (399); `(unsigned)c - 'A' < 26` and its
    kin are `ascii_isupper(c)`, `_islower`, `_isdigit` again (136); an `if`
    of a constant condition is the branch it takes (8). The Java's
    `gettext_(` 400 -> 1 and `Integer.compareUnsigned` 150 -> 17. Taking
    the constant `if (1) { len = 0; }` showed three parameters whose value
    the function never reads; the Go writes them `_` with a local of the
    name (`deadInParams`), which keeps `editor/` staticcheck-clean.
  - **phase 183** is phase 166's rule on the file-scope objects
    (`BoolRet` with `Globals`): 79 flags that only ever hold an answer --
    `VIsual_active`, `msg_scroll`, `exiting` -- are `bool`, and with them 1
    function, 7 locals, 4 members and 3 parameters; one whose address a
    table takes (`&p_wiv`), one compared with a code, sized, or shadowed by
    a local stays `int`. The Java's `VIsual_active != 0` 115 -> 0, its
    `TRUE`/`FALSE` 1,195 -> 875.
  - **phase 184** takes what 183's rule left int (`BoolRet` with `Relax`):
    the greatest fixed point, so a flag saved in a local and restored is
    an answer; a literal 0 or 1 assigned; `x |= E` of an answer, written
    `x = (E) || x`. 64 declarations, 11 of them flags (`need_wait_return`,
    `did_cursorhold`); the Java's `TRUE`/`FALSE` 871 -> 779. Left int: flags
    saved in a local its function reuses (`msg_scroll`), and `got_int`,
    which the host sets.

  Phase 83 is the line between the two arcs.

  **The pipeline reform** (`doc/PIPELINE-REFORM.md`) reordered what the bytes
  allow.
  - **Phases 1-3 are the front.** They cut every interface and feature the
    product has not, as twelve packages (D1-D12; phase 1 runs D1-D5, phase 2
    D6-D8, phase 3 D9-D12, each part followed by its own closure, so that the
    parallel check runs the three side by side): the command line, the Ex commands,
    the files, `:q`'s refusal, reading, the command syntax, the options, the
    swap file, startup, the encoding, the terminal, one window and buffer,
    the editing features, one regexp engine, the process. The fall-out
    closure (`crefactor/xform`'s `FallOut`) folds what each part left
    unwritten.
  - **The rest are named in blocks.** `d02`-`d13` are the drops that count
    text only their predecessors leave, `r01`-`r14` the rewires, and
    `g01`-`g11` the generic steps (`doc/GOALS.md`, *The pipeline as it runs*).
  - **The phases that edit nothing are records** in `internal/phase/archive/`.

  **The test suite is minimal.** Each phase was verified, while it was written, by a
  check program of its own and a delta declared in advance against recorded
  baselines; that whole suite (`internal/check`, `internal/verify`,
  `internal/harness`, every `check.go`, every `delta.md` but the one phase 1 now holds, the
  baselines and `make whim-verify`) was removed after `448e9a8`, the last commit
  that has it. What proves a change now is two things: `make whim-build-check`,
  the committed product back byte for byte, which sees the text; and `make
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
  line printed after it, one for each thing that sends phase 177's
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
  its core is nine modules and two and a half minutes of GHC's time. Both suites are exact under load (`internal/suite/stress_test.go`:
  0 differing runs of 6,720 for the wide suite). **Both end with the heavy
  case** (`internal/suite/heavy.go`), the one that times: 5,000 lines, three
  substitutions and a `:g`, run on every editor of the run one at a time,
  required to answer as the reference does, each time reported beside the
  C's, and an editor over 25 times the C's time failing the run -- measured,
  Go 0.5-0.6, Haskell 1.0-1.3, Java 1.6-2.1, Clojure 3.4-3.9 (4.2-4.5 before
  its big functions were compiled sooner, `doc/CLOJURE-PROFILE.md`) since the
  parallel `:%s` (Java 2.3 and Clojure 9-10 before), and the Clojure 55 with the JIT's
  huge-method limit left on, which is what it refuses.

## What a file is called

**Code is `.go`, data is `lowercase.md`, prose is `UPPERCASE.md`**, and the three
mix freely in one directory: `internal/phase/097/` holds `edit.go` and `GOAL.md`. A
data file in Markdown puts its data in a FENCED BLOCK and its notes around it,
and the reader takes the fence and ignores the rest -- `internal/build`'s
`declared()` reads `internal/phase/001/delta.md` that way (the command rows phase 1
retires at the front, which phase 80 then deletes), and phase 98's edit embeds `internal/phase/098/musl-ctype.md` and
`musl-case.md` and splices their fenced C in byte for byte. What is left outside
the rule is what Markdown would only obscure: `src/slim.sha` and `src/upstream.sha`, one
digest each, read by `make`.

**A phase is a directory, `internal/phase/NNN/`**, its number in three digits so that they
sort: `GOAL.md`, which opens `# Phase N — …` and says what the phase removes,
why and what was measured, and -- for the 112 phases whose cut is a program of
its own -- `edit.go`, which makes that directory **a Go package**, `pNNN`
(`editlit.go` beside it where the literals are long). The other phases are plan
steps only (`internal/steps`). An edit is written in `crefactor/edit`'s verb set
(`edit.E`, `edit.Ph`) and `internal/whim/vimtext`'s shared shapes, registers
itself with `internal/phase` (`phase.Register`) in an `init()`, and
`cmd/whim/phases.go` is what links them in: it imports every phase blank.

**`doc/GOALS.md`** is what holds for every phase: Part I (phases 0-82: the charter,
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
                   one table), build (whim's pipeline: the plan -- what each
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
                   front end. cemit/: the canonical printer. sweep/: the closure.
                   reach/: what nothing reaches, as a partition with gcc as its
                   control -- a reporter, `go tool whim reach FILE`; it deletes
                   nothing. ccx/: a core's pointer casts and evaluation order,
                   partitioned. dead/: funcreach and gcc's unused warnings.
                   pipeline/: the driver -- Phase, Step, Plan, Run, Advance,
                   Check, the snapshots, Seed -- told everything through a
                   Config. xform/: the generic transforms (fallout.go: the
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
                   hseffects.go the pure functions and those without the
                   editor, hsout.go out-parameters as results, hsstruct.go
                   struct locals as values, hssplit.go the modules). Its tests
                   run in it: `cd crefactor && go test ./...`
internal/whim/     what the generic side is told about vim: profile.go (the
                   sweep), xform.go, analysis.go (dead's roots, reach's and ccx's
                   names), gen.go (togo's profile, whim.Gen),
                   and vimtext/ (what more than one phase uses that knows vim: the
                   buffer walks, Key, the command table's residue check, the
                   prototype and #include shapes phases 118-119 share, and the
                   Python-port helpers of 126-128; a shape one phase alone uses
                   is in that phase's shapes.go)
internal/phase/    the registry the phases' programs join (registry.go, query.go),
                   and the phases: NNN/ (GOAL.md, and edit.go where its cut is a
                   program; cmd/whim/phases.go imports each blank), archive/NNN/
                   (the records: a GOAL.md each, for a phase that edits nothing
                   now), STAGES.md (the record of
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
Makefile           the whole build: fetches the input, runs the pipeline, builds the
                   binaries and the editor
src/               the input and the product: slim-vim.c (fetched, not tracked),
                   whim-vim.c (produced, tracked), upstream.sha and slim.sha;
                   their binaries are bin/slim-vim and bin/whim-vim
doc/               GOALS.md (what holds for every phase), AGENDA.md (what is not
                   done, in order, and what was declined, with why), GO-IDIOMS.md (how
                   the Go editor could be idiomatic, measured and ranked; done or
                   declined), JAVA.md (the Java backend: its design and milestones),
                   JAVA-IDIOMS.md, CLOJURE-IDIOMS.md and HASKELL-IDIOMS.md (how the
                   Java, Clojure and Haskell editors could be idiomatic,
                   measured and ranked; surveys; done: CLOJURE-IDIOMS.md's items 0-3, 4's tables (its messages declined), 5 in part, 6, 7, 8's headroom (9 declined), and
                   JAVA-IDIOMS.md's items 1-3, 4's masks, 5's tables, 6.1 (phase 174) and 11's files, HASKELL-IDIOMS.md's all but what it declines),
                   PIPELINE-COMPACTION.md (which phases could be dropped, merged,
                   split or reordered, measured byte for byte), CLOJURE.md (the
                   Clojure editor), CLOJURE-PROFILE.md (where its time goes in
                   the heavy case, beside the C's, and the change it chose), HASKELL.md (caprice, the Haskell editor:
                   its design and what was measured), RUST.md (a preliminary
                   plan for a Rust editor, not scheduled), WASM.md (the same, for the Go editor in a
                   browser: what compiles already, the host it would need), IR.md (where a feature goes in the chain,
                   and an intermediate representation: an assessment),
                   IR-SCHEMA.md (that representation sketched against togo:
                   what is shared and duplicated, a schema, a migration path),
                   PARALLEL-SUBSTITUTE.md (how much of a :%s is matching,
                   measured on every editor, and what stands in the way) and GO-LISP.md
                   (the Go editor in go-lisp syntax: an experiment, and make
                   editor.lgo)
```

**The toolset is `go tool whim`**: `go.mod` declares `cmd/whim` as a tool, so Go
builds it, caches it and rebuilds it when any `.go` moves; every tool is a
subcommand, `go tool whim <name>`, and the `Makefile` calls it the same way.
**The C front end is a fork**, `crefactor/cc`: modernc.org/cc/v4 v4.29.7 with two
C23 productions added, tracked as ordinary source (`crefactor/cc/README.md`,
which says how to diff it against upstream). It was composed at build time under `.cache/gofork/`
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
                     # bin/caprice
make whim-build      # the 124 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same, required to give the committed bytes back
make whim-editor-check # refuse a tracked editor.go, braaam/editor/, editor.clj or Editor.hs that is not what the generator writes
make whim-test        # the quick suite: 80 key sessions, required to behave as HEAD's does
make whim-test-wide   # the optional wide suite: 240 cases, keys, Ex commands, argv, a terminal
make bin/braaam       # the editor in Java: whim.editor generated, compiled, and a launcher
make whim-test-java   # the quick suite with the Java editor too (whim test --java; --wide --java)
go tool whim java --same-classes  # HEAD's braaam/editor/ and the tree's compile to the same code: a spelling change's proof
make bin/vijure       # the editor in Clojure: whim.editor generated, AOT-compiled, a launcher (CLJ_EDITOR=F: F's)
make whim-test-clj    # the quick suite with the Clojure editor too (whim test --clojure; --wide --clojure)
make bin/caprice      # the editor in Haskell: Caprice.Editor generated, compiled by GHC (three minutes when the core moved; its time and peak printed)
go tool whim caprice --lint  # the same, then ghc -Wall's warnings on the generated module, by flag (0 now)
make whim-test-hs     # the quick suite with the Haskell editor too (whim test --haskell; --wide --haskell)
make editor.lgo       # the Go editor as one go-lisp file, compiled (GOLISP_ROOT=.../go-lisp; doc/GO-LISP.md)
make go-test          # the Go packages' tests, this module's and crefactor/'s (go test ./... skips it)
make bin/whim-vim    # the C product's binary
make bin/slim-vim    # the input's binary, with the same one line
make score           # bytes to store and symbols to provide, input beside product
make help            # every target, with a line each
```

- **One path.** `make whim-build` is what a moved upstream runs:
  `internal/build`'s plan -- each phase's steps (`internal/steps`), **the sweep,
  after every phase** (there are no stages), and **the canonical print of what is left**
  (`crefactor/cemit`: one spelling per construct, and NO COMMENTS, of any kind),
  so every boundary that is C is in the one spelling phase 0 seeds with -- C23's,
  `nullptr` and `usize` included, since phase 0 runs phase 106's rename on the
  canonical input and every later phase is written for it -- applied
  in one process, in memory. **Its log is a line a phase** -- the name, the acts its
  steps reported, the lines its edits and the sweep took, the lines left, the
  time, under a heading for each block (`block  d02-outside`); `-v` writes every act, and a phase that refuses writes its whole report
  before the reason. Measured: 124 phases, **815 s**, 77,634 lines. A
  whole run keeps every boundary in `.cache/boundaries/` (qNNN.c) and seals the
  set with the input's digest (`manifest`).
- **The sweep is one closure** (`crefactor/sweep`'s `Prune`): the text parsed
  (`cc.Parse`, no type-checking, no gcc), everything reachable from `main` and
  the static_asserts found by name in C's three name spaces, and everything else
  cut -- functions, objects, prototypes, typedefs, tags, members, enumerators, and
  the locals nothing reads (resolved by the parser's scopes). Its guards: no
  member goes while `ml_recover` is defined; a struct filled by position keeps
  every member; nothing is emptied; an enumerator's deletion pins the survivor
  after it to its value. About 3 s a phase. It replaced six deleters looped around
  gcc, and keeps nothing they cut (measured on the product: 5,657 entities
  against 5,662, the five it adds all unused).
  It needs every text it is handed to PARSE, and every one does.
- **`whim-build-check` runs phase by phase, in parallel.** With a sealed set of
  snapshots for the input on disk, it checks that phase 0 seeds the input into
  q000 and that EVERY phase N, run on q(N-1), gives qN -- all phases at once,
  `--jobs N` at a time (default: every core) -- and that the last snapshot is the
  committed `whim-vim.c`. Measured: **83 s**, 123 links 64 at a time, bound by
  the machine's load and no longer by one link (the front's three parts, phases 1-3, take 40, 43 and 45 s, and the seed 10 s; phase 1 alone was 89 s), against 1,005 s in
  order; and a phase whose program was changed -- on purpose (a control),
  or phase 177's while it was being written -- is named and fails the check. That is
  the induction a run in order walks, so it proves the same thing; a phase whose
  program changed breaks its own link and is named. With no snapshots of this
  input it runs the pipeline in order, which writes them. It proves the text,
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
  (`caprice/Caprice/Editor/`), the Haskell backend's.

- **The compile line is one line**, `gcc -O0 -fno-stack-protector -static -no-pie
  -s`, for the input, the product and every boundary: an ordinary static
  executable, no stack protector. `internal/build/compile.go` states it for the
  tools (`FlagsFor`, `score`, `measure`), and the `Makefile` as `CFLAGS`/`LDFLAGS`
  for its two binary rules. It moved at phases 83 (`-no-pie`) and 84
  (`-fno-stack-protector`) until those became the line for all; the two phases
  change nothing now.
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
`p_N = f_N(p_{N-1})` -- 124 of them, in order, numbered 0-184. 39 phases that
edit nothing any more are records only, a `GOAL.md` each in
`internal/phase/archive/` (`doc/GOALS.md` lists them): most because phase 1's
front runs their cut now, 168 because `GotoTail` takes its gotos
(`doc/PIPELINE-REFORM.md` §7). 61's program is called by the front too, so it
has no plan entry but keeps its directory; 106's is phase 0's one step, the
same. And the
13 same-purpose groups of `doc/PIPELINE-COMPACTION.md` §3d still running -- 44-48,
51-53, 72-73, 100-102, 105 and 107, 117-119, 121-122, 143-145, 148-149, 151-152,
153-154, 164-165, 166-167 -- each run as one phase under the group's last
number. A gap in the numbers is nothing to the driver,
which pairs plan entries by position. **There is no memoize**: its key
was the input boundary's digest and the implementation's together, so a moved
`slim-vim.c` missed every entry by construction.

- **The driver is generic, the plan is whim's.** `crefactor/pipeline`
  runs a plan (Run, the parallel Check, Advance, the snapshots, `--keep-going`)
  and knows no code base; `internal/build` hands it a `pipeline.Config` -- the
  plan, the op table (`internal/steps`), the work file `whim-vim.c`, `SnapDir`
  `.cache/boundaries`, the sweep's options (`internal/whim`'s `Profile`), the
  `@state`/`@minmax` arguments and phase 1's `delta.md` -- and keeps its old
  names (`build.Run`, `Check`, `Advance`, `Options`) as wrappers.
- **There are no stages.** Every phase is its steps, the sweep, and the
  canonical print; a `sweep` step inside a phase's steps is for an edit that
  reads its own earlier steps' text swept. `internal/phase/STAGES.md` is the
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
`whim java`, `whim clj`), and `go tool whim cut` prints it: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `GOALS.md` §II.4 is the
design.

## Adding a phase

`GOALS.md` Part II, *Adding a phase*, has the process; the next phase is 185.
The pipeline's goal is met; a phase now is for the Go editor, where the C is
the cause of what the generator cannot make idiomatic. What a new one takes:
`internal/phase/NNN/` with `GOAL.md`, and `edit.go` in package `pNNN` registering
itself if its cut is a program (a line in `cmd/whim/phases.go`); and an entry at
the end of `internal/build`'s `Plan` naming its steps (the sweep follows
every phase). Then `make whim-build` (the product moves, so the tracked `whim-vim.c`
and `editor/editor.go` are rewritten), `make whim-test` against the commit
before it (a phase that removes capability moves cases on purpose: name them),
and whatever further evidence the phase needs, stated in its `GOAL.md`.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
