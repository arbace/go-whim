# Could the phases edit the AST instead of the text?

A survey, not an implementation. Nothing tracked was modified; `git status` is
clean and `slim-vim.c` still hashes to `1a4d25021f118c63f5dc5654913e0299`, the
digest `slim.sha` records. Every number below was measured in this tree on
2026-09-23; where a thing could not be measured it says so. The instruments are
untracked programs under `.tmp/astedit/`, `.tmp/astedit2/`, `.tmp/ptrace/` and
`.tmp/ast/`.

Two earlier surveys, `.tmp/reachability-survey.md` and `.tmp/deadenums-survey.md`,
established that AST **analysis** agrees with the text tools. This one is about
**editing**, and keeps their distinction: replacing the analysis is a different
and much cheaper proposition than replacing the edit.

The question is asked of the world the `canon` migration is building, not of
today's: from phase 1 on the text is `cemit`'s canonical form, so
`text == cemit(parse(text))` for every intermediate tree, and an AST edit
followed by a print would land in the form the next phase's anchors expect.

**Short answer: no, not as the printer stands, and the reason is not cost or
taste. `internal/cemit` is a function of (AST, source text), and the map between
the two is BY POSITION. Three of the four mutations a phase would need are
therefore either reverted or corrupted, SILENTLY, with an output that still
parses, still compiles and is still the printer's fixpoint. Measured: deleting an
initializer-list element next to a macro invocation produces BYTE-IDENTICAL
output — 10 of 10 tried; there are 1,321 such elements in the canonical
`slim-vim.c` and 829 in the canonical `whim-vim.c`. Renaming an identifier USE
through the tree changes nothing at all — 6 of 6 tried. Splicing in a parsed
fragment prints `return yyy_probe();` as `return hat were();`. Only two
mutations are clean today: deleting a whole external declaration (294 tried, 294
clean) and deleting a statement (60 tried, 60 clean; 0 of 79,224 block items
hazardous).**

**The good news is narrower and real: `cemit` needs no type information at all —
0 uses of the type system in 1,482 lines — and printing from `cc.Parse` instead
of `cc.Translate` gives BYTE-IDENTICAL output on three large texts. That removes
the hard boundary both earlier surveys assumed, and makes canonicalisation
available on the intermediate texts that fail semantic analysis.**

---

## 0. The instruments

| | what it is |
|---|---|
| `.tmp/astedit/` | `internal/cemit` copied into `package main` (so its unexported `scan`/`expansions` can be instrumented) plus mutation probes: `time`, `exp`, `emit`, `delext`, `delstmt`, `delafter`, `synth`, `synth2`, `hazard`, `hazcheck`, `insert`, `rename`, `renameuse` |
| `.tmp/astedit2/` | the same, against the **canon branch's** `internal/cemit` (`git show worktree-agent-a8c7188a3477181d0:internal/cemit/*.go`), which adds whole-line comment recovery and `specToken`. One accessor the branch adds to `internal/cc` is shimmed with `unsafe` in `.tmp/astedit2/shim.go` rather than editing a tracked file. |
| `.tmp/ptrace/` | applies phases FROM..TO with **no sweep** and, after each, reports whether the text `cc.Parse`s and whether it `cc.Translate`s |
| `.tmp/presweep/` | the earlier survey's probe, reused: a run of phases' edits with no sweep |
| `.tmp/ast/` | the texts: copies of `slim-vim.c`, `whim-vim.c`, `editor.c`, their canonical forms, and two synthetic files built to isolate one mechanism each |

Nothing was run in place on a tracked `.c` file; `whimtools cemit` was run only
on copies under `.tmp/ast/`.

The texts, and the round trip on each (`cc.Translate` + `cemit.File`, main's
printer):

| text | lines in | lines out | parse | print | fixpoint |
|---|---|---|---|---|---|
| `slim-vim.c` canonical | 180,870 → 185,086 | 5,009,241 B | 1.472 s | 1.780 s | yes |
| `whim-vim.c` canonical | 77,306 → 80,134 | 2,097,174 B | 0.745 s | 0.789 s | yes |
| `editor.c` (the core, not canonical) | 2,008,931 B | 2,040,387 B | 0.689 s | 0.707 s | — |

Another agent was running full builds on this machine throughout. The timings
are wall clock on a loaded machine and should be read as upper bounds; the
comparisons between them were taken in the same minutes and are sound.

---

## 1. THE CRUX: can the tree be mutated and printed?

### 1.1 What the printer actually is

`internal/cemit/macro.go` recovers macro invocations from the source text by
position. Its own account is exact and worth restating because everything here
rests on it: every token of an expansion carries the INVOCATION's position, so an
expansion is a run of tokens sharing one offset, **and the text it came from runs
from that offset to the NEXT OFFSET ANY TOKEN HAS**. The canon branch adds
whole-line comment recovery on the same principle, keyed on the line number of
the declaration that follows a comment run; `File` also reads the `#include`
lines out of the source as text.

So the printer is a function of (AST, source text), and the two are joined by
byte offsets into a source that a mutation does not change. The failure mode
follows mechanically: **delete the node that owns the token at an expansion's end
offset, and the expansion's recovered text grows to swallow the deleted source.**

### 1.2 The mechanism, isolated

A ten-line synthetic file (`.tmp/ast/tab2.c`), canonicalised, then mutated in the
tree three ways. Oracles: the changed region of the print, whether the output is
still the printer's fixpoint, and whether the deleted name survives.

```
enum g { P, Q, R };              -- no macro
enum e { A = EINVAL, B, C };     -- a macro in the first enumerator
int tab[] = { EINVAL, 7, 8 };
```

| mutation | printed change | fixpoint | the deleted name |
|---|---|---|---|
| delete enumerator `Q` (no macro nearby) | `"Q,\n    "` → `""` | yes | gone (0 occurrences) |
| delete enumerator `B` (after `A = EINVAL`) | `"\n   "` → `""` | **NO** | **still there (1 occurrence)** |
| delete `7` from `{EINVAL, 7, 8}` | nothing | yes | **still there** |
| delete the statement `int x;` | `"int x;\n    "` → `""` | (does not type check: `x` undefined — correct) | gone |
| delete the statement `x = EINVAL;` | `"EINVAL;\n    x = "` → `""` | yes | gone |

Deleting `B` printed `A = EINVAL, B,` on one line: the expansion's recovered text
ran from `EINVAL` to the first token that survived, which is `C`. Deleting `7`
produced output **identical to the input** — the tree lost an element and the
text did not. That second case is the dangerous one: it is a silent no-op that
passes the fixpoint test, parses, and compiles.

Why statements are safe and list elements are not: a statement's own `;` is a
token of the statement, so an expansion inside it ends inside it. A list
separator `,` belongs to the grammar's list cell, and unlinking the cell removes
the comma as well.

### 1.3 The hazard census, over whole translation units

A node is **hazardous** when some expansion's recovered text ends at its first
token — i.e. unlinking it would extend that expansion. The detector reproduces
exactly the two corruptions of §1.2 on the synthetic file and flags nothing else.

| text | external decls | with expansions | enumerator cells / hazardous | initializer cells / hazardous | block items / hazardous |
|---|---|---|---|---|---|
| canonical `whim-vim.c` | 4,474 | 746 (16.7 %), 3,118 expansions | 1,155 / **0** | 15,185 / **829** | 33,931 / **0** |
| canonical `slim-vim.c` | 8,687 | 1,924 | 2,898 / **0** | 25,891 / **1,321** | 79,224 / **0** |

The expansions whose text a deletion would extend, by name:

- `whim-vim.c`: **829, every one of them `nullptr`.**
- `slim-vim.c`: **1,296 `NULL`**, 3 `false`, 1 `LONG_MAX`, and one each of 20
  `SIG*` names — 24 distinct texts in all.

Then the census was checked by doing it. Ten hazardous elements of canonical
`slim-vim.c` were deleted from the tree one at a time and the file printed:

```
11967:40 (after "NULL"): deleting it from the tree changed NOTHING in the output
12299:35 (after "NULL"): deleting it from the tree changed NOTHING in the output
12299:41 (after "NULL"): ...
28441:20 (after "LONG_MAX"): ...
30118:39 33788:52 33788:58 33788:64 33788:70 37414:38 (after "NULL"): ...
10 tried in 17.0s, 10 silently did nothing, 0 changed the text
```

**10 of 10.** This is exactly the class of edit the pipeline does most: 53 of the
293 step calls are `dropoptions`, which deletes an `options[]` row, and an
`options[]` row is an initializer-list element full of `NULL`.

### 1.4 What IS clean

- **Whole external declarations.** `scan` runs per external declaration, and
  deleting one does not disturb any other's offsets. Measured: 45 deletions on a
  uniform sample of canonical `whim-vim.c` and 249 deletions covering every third
  expansion-bearing declaration — **294 deletions, 294 of them a pure contiguous
  deletion of the printed text, 0 failures**, at 0.73 s per print.
- **Statements.** 60 top-level block items deleted from function bodies that
  contain expansions: **60 clean, 0 failures**. The hazard census agrees: 0
  hazardous block items out of 33,931 and 79,224.
- **Numeric constants.** Changing a `PrimaryExpressionInt` token's text with
  `cc.Token.Set` works: `1` → `999` came out as `999`, because the source at that
  offset does not begin an identifier and the recovery therefore does not fire.
- **A declarator's own name.** `musl_memcpy` → `musl_memcpy_renamed` printed
  correctly: the declarator path prints `tok()` without consulting the recovery.

### 1.5 What is NOT clean

- **An identifier USE.** `cemit/expr.go` consults `fromMacro` before printing a
  `PrimaryExpression`. A token whose text was changed by `Token.Set` keeps its
  original offset, so `says()` fails, `identStart()` succeeds, the node is
  classified as an expansion, and **the original source text is printed**.
  Measured on the synthetic file: 6 renames of identifier uses, **6 with the
  output completely unchanged**, including `EINVAL` (a genuine macro) and `x` (a
  plain local).
- **Insertion.** Two measured failures, both silent:
  1. A fragment parsed under its own file name is **dropped without a word**:
     `File` skips any external declaration whose `pos.Filename != mainFile`.
     Measured: 0 bytes inserted, no error.
  2. A fragment parsed under the host's file name carries the FRAGMENT's line and
     column numbers, which the printer resolves against the HOST's source. Into
     canonical `slim-vim.c`, whose first lines are comments:

     | written | printed |
     |---|---|
     | `int spliced_probe(void) { return 41; }` | `return ro;` |
     | `int g_probe(void) { return yyy_probe(); }` | `g_probe(lati)` … `return hat were();` |

     Into canonical `whim-vim.c`, whose line 1 is
     `typedef typeof(sizeof(0)) usize;`: `return yyy_probe();` printed as
     `return size; en();`.

     A fragment always occupies lines 1..k, so it always lands on the host's
     first lines, and whether it survives depends on what the host happens to say
     there. Eight insertions were tried, four into canonical `whim-vim.c` with
     main's printer and four into canonical `slim-vim.c` with the canon branch's:
     **2 silently dropped, 3 silently corrupted, 3 correct.** The three that came
     out right did so because the host's bytes at those offsets were a newline or
     a digit rather than a letter.
- **Synthesising a token at all.** There is no exported constructor: every
  `Token`-returning function in `internal/cc` is a method on an unexported
  `scanner` or `parser`. The only way to give a node new text from outside the
  package is to copy an existing `Token` and call `Set`, which appends to the
  scanner's buffer and leaves `off` — the position — untouched. That is precisely
  what defeats §1.5's first bullet.

### 1.6 Is this a law, or this printer's choice?

It is this printer's choice, and a good one for what it was written for. The end
of an expansion is INFERRED (`the next offset any token has`) rather than
recorded, which is what makes `macro.go` need no macro table. `cc.Token` does
carry an `m *Macro` field, so a front end that exposed it could give the printer
the invocation's true extent and remove hazard classes 1.3 and 1.5-#1 entirely.
That is a change to `internal/cc` — a fork whose whole discipline is to differ
from upstream in two C23 productions and nothing else (`internal/cc/README.md`).
Nothing measured here says it is impossible; everything measured says it is not
free, and that until it is done, three of the four mutations a phase needs are
silently wrong.

### 1.7 The canon branch behaves the same

The in-flight migration's printer was measured too, since it is the one the
question is about:

- It preserves all **272** whole-line comments of `slim-vim.c` and is a fixpoint
  (`cemit(cemit(x)) == cemit(x)`, checked byte for byte).
- The hazard census on its canonical `slim-vim.c` is identical: 8,687
  declarations, 1,941 with expansions, **1,321 hazardous initializer elements**,
  0 enumerators, 0 block items.
- Its insertion failures are worse, not better, because the recovered text comes
  from the file's comment banners: `(void)` printed as `(lati)`.
- Comments are recovered by ORDER, not by attachment: a comment run is written
  before the first declaration whose line is greater than the run's. So deleting
  a declaration does not lose its comment — it hands it to the next surviving
  declaration. Emptying a whole banner section moves that banner over the next
  section's code. That was not exercised on a real cut; it follows from
  `commentRuns` and `comments(pos.Line)`, and is stated here as a reading of the
  code, not a measurement.

---

## 2. PARSES / TYPE-CHECKS / IS-ANALYSABLE are three different bars

Both earlier surveys, and this survey's own brief, said the text between roughly
phases 13 and 35 "does not parse". **That is wrong, and the correction changes
the shape of the answer.**

### 2.1 The texts parse; they fail semantic analysis

`internal/cc` exposes `cc.Parse` (cc.go:896) separately from `cc.Translate`
(cc.go:913). Measured on the real thing — `.tmp/presweep` applied phases 13
through 35 to the real q12 boundary with **no sweep at all**, producing
`.tmp/ast/q35-nosweep.c`, 3,509,183 bytes:

```
cc.Parse:     ok
cc.Translate: FAIL
  .tmp/ast/q35-nosweep.c:123724:79: type winopt_T has no member named wo_eiw
  .tmp/ast/q35-nosweep.c:123725:78: type winopt_T has no member named wo_eiw
```

Two errors, at `wo_eiw` — the same two `internal/dead/deadsweep.go`'s comment
records from the real phase-35 text. The text is syntactically valid C
throughout. What is unavailable between an edit and its sweep is the RESOLVED
tree, not the tree.

`.tmp/ptrace` then applied phases 13 through 35 one at a time from the real q12
boundary, with **no trailing sweep anywhere** (the five inner sweeps `RunPhase`
runs are all that repairs anything), and asked both questions after every phase.
The whole table, `.tmp/ast/ptrace.log`:

| after phase | `cc.Parse` | `cc.Translate` |
|---|---|---|
| q12 boundary, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24 | ok | ok |
| **25, 26, 27** | ok | FAIL, 2 lines: `undefined: B0_UNAME_SIZE` |
| 28 (has an inner sweep) | ok | ok |
| **29, 30, 31** | ok | FAIL, 39 lines: `type struct file_buffer … has no member` |
| 32 (has an inner sweep), 33, 34 | ok | ok |
| **35** | ok | FAIL, 4 lines: `type winopt_T has no member named wo_eiw` |

**`cc.Parse` did not fail once, at any of the 24 texts. `cc.Translate` failed
after 7 of the 23 phases**, in three short windows, each closed by the next
sweep. Both q12 and q41 — real, post-sweep boundaries — `Translate` without
complaint.

So the "13 to 35 does not parse" of the earlier surveys is a window of **seven
phases where the RESOLVED tree is unavailable**, not twenty-three where the tree
is. Restated for the record: the intermediate texts clear the PARSES bar
everywhere, fail the TYPE-CHECKS bar in three short windows, and
IS-REACHABILITY-ANALYSABLE follows the second, not the first.

### 2.2 The printer needs no types

`internal/cemit` is 1,482 lines and contains **zero** uses of the type system:
`grep` for `.Type()`, `.Value()`, `typer`, `IsTypename`, `ResolvedTo`, `.Field()`
across all five files returns two hits, both `reflect.Value.Type()` inside the
token walk. `Translate` is simply what `Canonical` happens to call.

Measured, with `cc.Parse` substituted for `cc.Translate` in a scratch copy:

| text | Translate vs Parse output |
|---|---|
| canonical `whim-vim.c` (2.1 MB) | **byte-identical** |
| canonical `slim-vim.c` (4.8 MB) | **byte-identical** |
| `editor.c`, the core (2.0 MB) | **byte-identical** |

And on a text that does not type check:

- `.tmp/ppc/bad.c` (the ten-line reproduction): `Translate` fails, `Parse` +
  print produces the correct canonical form, all 343 bytes.
- `.tmp/ast/q35-nosweep.c` (the real intermediate): `Translate` fails, `Parse` +
  main's printer produces **3,530,139 bytes** of correct canonical C.

The `enum : long` underlying type survives because the printer takes it from the
`EnumSpecifier`'s syntax, not from the checked type — which the byte-identical
comparison on all three texts confirms without having to argue about it.

Parse is also cheaper: **1.113 s vs 1.575 s** on canonical `slim-vim.c` (−29 %),
**0.614 s vs 0.721 s** on canonical `whim-vim.c` (−15 %).

### 2.3 One thing the canon branch's printer does refuse there

On `.tmp/ast/q35-nosweep.c` the canon printer refuses with
`cemit: line 6478: a whole-line comment inside a brace`. The comment is one a
phase INSERTED into a function body ("The scrollback is the only memory left to
reclaim…"). This is not a type question and not an AST question; it is a
restriction in the comment recovery, and it is the canon migration's to answer.
Main's printer, which does not recover comments, prints that text without
complaint. Worth naming here only so that "the printer works on non-type-checking
texts" is not overstated: **the type checker is not the obstacle; the comment
rule can still be one.**

### 2.4 Which census classes fall on which side of the line

| needs only `cc.Parse` (available at every intermediate) | needs `cc.Translate` (unavailable between an edit and its sweep) |
|---|---|
| canonicalising the text (`cemit`) | reachability closures — `ResolvedTo`, `Field()` (the whole of `.tmp/reachability-survey.md`) |
| deleting a function definition, a declaration, a prototype | "does anything still READ this option?" — `droplocal`'s refusal |
| deleting a statement, a case arm, a struct member, an enumerator, a table row | enumerator VALUES (`deadenums` takes them from DWARF, which needs a build) |
| locating a construct by shape: "the `options[]` rows whose variable slot is `NULL`" (phase 54's question) | `deadfields`, `typereach`, `funcreach` if they were rebuilt on the tree |
| folding an `if` whose condition is a literal, removing a conjunct | anything that must know a name resolves to THIS declarator rather than a shadowing one |
| scoping an edit to one function (`InFunction`, 221 sites, 21 phases) | — |

So the boundary is real but narrow: it does not stop structural editing or
canonicalisation anywhere in the pipeline, and it does stop every symbol-resolved
question in the window between an edit and the sweep that repairs it. Since the
sweep is exactly the thing that answers those questions today, nothing is lost
that was being asked.

---

## 3. THE CENSUS: what the edits actually do

Two subagents measured this in parallel; I spot-checked the load-bearing counts
myself (`ls phase/*/edit*.go | wc -l` → 141; `ls -d phase/*/ | wc -l` → 163; 108
directories with an `edit*.go`; `cat phase/*/edit*.go | wc -l` → 27,915;
`grep -oE '\{Op: "[a-zA-Z0-9-]+"' internal/build/plan.go | wc -l` → 293;
`grep -ho regexp.MustCompile phase/*/edit*.go | wc -l` → 362, and 190 in
`internal/cut`; `grep -c 'Sweep: *true' internal/build/plan.go` → 87).

### 3.1 The shape of the pipeline

- 163 phases; **106 run a bespoke edit program**, 2 run only a query (002, 054),
  **49 are cutter-only**, 6 change no source (0, 83, 84, 86, 116, 123).
- 293 step calls: **110 `edit`**, 53 `dropoptions`, 21 `retire`, 20 `droplocal`,
  15 `cmdidxs` (assert-only), 14 inline `sweep`, 3 `dropopts`, 2 `query`, and 55
  cutters called once each.
- `internal/cut`: 54 files, 61 exported cutters, 57 reachable as steps.
- `internal/cutil`: 9 files, 1,009 lines — a hand-rolled structure layer over
  text (brace depth, function bodies, definition extents).

CLAUDE.md's "54 shared cutters, 13,922 lines with cutil" is stale: cut+cutil
measures **11,193** lines today, and `internal/edit/phdriver.go` already says 57
cutters. Worth a one-line fix in CLAUDE.md, separately from this survey.

### 3.2 The classification

Over the 437 sites of the polymorphic replace family (`e.Literal/LiteralN/Term/
Sub/Cut/Lines/LinesT`, `p.Literal/SwapOnce`, `edit.Once`), 392 resolved to a
literal argument and 45 are computed at run time and were not classified:

| class | sites | share of the 392 | AST? |
|---|---|---|---|
| **(a)** delete a whole construct — line(s), statement, declaration, struct member, case arm, table row, definition | 216 | 55.1 % | mostly yes, subject to §1 |
| **(b)** rewrite an expression, condition or initialiser in place | 149 | 38.0 % | yes in principle — see §1.5 |
| **(c)** insert new C written as a string | 27 | 6.9 % | **no, as measured in §1.5** |

Plus, outside that family: **221 fold sites in 26 phases** (fold an `if` whose
condition became constant — class (b) with an (a) effect), 137 whole-line
deletions in 17 phases, 18 `DeleteDefinition` sites, 49 line-array splices in
phases 127 and 128, and 226 `InFunction` sites in 24 phases, which are not edits
at all but *scoping a textual match to one AST node* — the single most-used
structural verb in the tree, and the one an AST gives away free.

**(c) INSERTION, measured**: **1,288 lines of new C at 290 sites in 51 phases**,
plus 300 lines through `{old, new}` pair tables in 5 phases, plus 173 lines in
`[]string` blocks in phases 127 and 128, plus the **606** lines phase 98 splices
from `tools/musl-case.txt` and `tools/musl-ctype.txt`, plus 24 lines phase 110
generates from a table. The largest single insertions are
`phase/097/editlit.go`'s 301-line `z14Defs` (whole `musl_*` definitions) and
`phase/103/editlit.go`'s 229-line `z20Host` (the entire host block).

Not every one of those is a valid fragment: `phase/118/edit.go:358` writes
`"    int\nmain(int argc, char **argv)\n{\n"` — an opening brace with no body,
spliced against a following anchor. `e.Splice(from, to, with)` replaces
everything between two byte anchors, a span that need not be a node.

**(d) NOT C AT ALL**: the Makefile (phases 0, 83, 84); `#include` lines (phase
82 deletes them by asking gcc, from the bottom, which is the order the committed
product was cut in; phase 99 removes six; phase 110 MOVES the block, because its
position IS the core/host boundary); every comment in the file (phase 82); blank
runs (41 accounting sites in 14 phases); and the contents of C string literals —
**137 literals in 43 phases**, with 9 phases using `edit.LiteralSpans`
specifically so as NOT to touch them.

**(e) TEXT ORDER, POSITION, LAYOUT**: 1,004 of the 3,529 C-ish string literals in
edit files carry leading indentation, across 83 phases; 432 regex literals spell
`[ \t]*` explicitly in 41 phases; 404 `(?m)` anchors in 52 phases; 76
split-on-newline sites in 33 phases, with 14 phases splitting the whole tree into
a line array; 9 fold sites in 4 phases that take the LAST match. Phase 110
refuses unless the input has exactly eleven preprocessor directives on its first
eleven lines and finds a function as "the line beginning `name(`, whose previous
line starts with `    static `, running to the first line that is exactly `}`,
which must be followed by a blank line". Phase 128 closes a definition by summing
`{` minus `}` per line over a `[]string`.

### 3.3 Phases whose edit is not a tree transformation at all

0, 83, 84 (Makefile only); 86, 116, 123 (the instrument, no source); 82 (comments
and `#include`s, the latter by compiler probe); 99 (six more `#include`s); 110
(the product of the phase is a POSITION in the file, and the set of things that
move is a fixpoint over gcc's unused-symbol diagnostics, reached by the only
`exec.Command` in any phase edit); the 15 `cmdidxs` steps (assert a byte-level
identity, edit nothing); 155 (the transformation is tree-shaped, but its
justification — gcc's right-to-left argument evaluation, read off the
disassembly — is in no tree).

That is **at least 11 phases and 15 steps with no AST formulation**, before any
of §1's problems are counted.

---

## 4. CAN THE AST BE MUTATED AT ALL? (mechanics)

Yes, mechanically, and easily — which is what makes §1 the crux rather than this.

- `internal/cc`'s AST is 85 node types, 20 of them right-recursive list types
  (`TranslationUnit`, `BlockItemList`, `InitializerList`, `EnumeratorList`,
  `InitDeclaratorList`, …), and **every list field is exported**. Deleting an
  element is `prev.Next = cell.Next`, from any package. All the probes here do
  exactly that, and restore it afterwards.
- There is no node-replacement API and none is needed: the fields are exported
  and assignable.
- There is **no way to build a node with new text**. No exported `Token`
  constructor exists. `Token.Set(sep, src)` changes what a token *says* while
  keeping where it *was*, which is the source of §1.5.
- So the practical shape is: **delete freely, rewrite only what `Token.Set` can
  reach, and insert only by parsing a fragment** — and the last of those is
  measured broken in §1.5.

An **edit script over positions** — an AST used to compute byte spans, and the
text spliced — has none of these problems, because the source never stops being
the authority. That is the hybrid, and §6 recommends it.

---

## 5. WHAT IT WOULD COST

### 5.1 The round trip

`cc.Parse` + `cemit.File`, measured at three real sizes:

| text | lines | parse | print | round trip |
|---|---|---|---|---|
| canonical `whim-vim.c` | 80,134 | 0.614 s | 0.784 s | **1.40 s** |
| the phase-35 intermediate, canonical | 133,720 | 1.036 s | 1.327 s | **2.36 s** |
| canonical `slim-vim.c` | 185,086 | 1.113 s | 1.765 s | **2.88 s** |

163 round trips is therefore between **228 s** (every phase at the product's
size) and **470 s** (every phase at the input's size), against `make
whim-build`'s measured **1,220 s**: **+19 % to +38 %** if every phase re-parses.

### 5.2 Against what the text edits cost today

Single phases applied to the 153,882-line q12 boundary with no sweep, measured
individually with a prebuilt binary:

| phase | 13 | 14 | 15 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|---|---|
| seconds | 0.74 | 0.07 | 1.93 | 3.86 | 3.69 | 0.83 | 2.36 |

Phases 13–20 together cost 48.1 s, of which phase 16's inner sweep — a gcc
compile — is about 34 s. So **a parse-and-print round trip at that size (2.4 s)
costs as much as the pipeline's most expensive pure text edit, and thirty times
its cheapest**, while the build's real cost is the 101 sweeps.

This is the mildest of the objections. A 19–38 % build is affordable.

### 5.3 Could a tree be threaded between phases?

Measured from `internal/build`'s Plan:

- **87 of 163 phases are followed by a sweep**; **14 more sweeps run inside a
  phase**, between its own steps (phases 16, 21, 24, 28, 32, 42, 49, 50, 53, 57,
  58, 60, 62, 64). 101 sweeps on a full build.
- In Part I (0–82) only **12** phase boundaries sweep. From 85 on, **every
  source-changing phase sweeps**, and all 74 `Each`-stage phases both sweep and
  are checked individually.

A sweep is a fixpoint loop, max 15 rounds, of seven tools: `deadsweep` (parses
gcc's `-Wunused-*` warnings), `deadprotos`, `typereach`, `funcreach`,
`deadfields`, `deadenums` (pinned against DWARF from a `-g` build), and
`internal/canon` — **which is a seven-pass reformatter, itself looped to a
fixpoint**, and which must run at the END of each round: moving it before the
loop moved 27 of 32 boundaries.

So:

- **Phases 13–41 and 42–63 are the only real threading opportunity**: runs of up
  to 29 consecutive phases with one sweep at the end. A tree could in principle
  be threaded across those, paying one parse and one print per run instead of per
  phase.
- **From 85 on, threading is impossible as things stand.** The sweep is defined
  as a transformation of *text on disk that gcc has been asked about*. Two of its
  seven members — `deadsweep` and `deadenums` — have no AST-level answer at all;
  `internal/build`'s own doc says the sweep "is the one thing in the pipeline that
  is not a function of the bytes alone".

Threading therefore saves the round trip on roughly the half of the pipeline
where it is cheapest to pay anyway, and saves nothing on the half where every
phase already pays for a gcc compile.

---

## 6. WHAT IT WOULD BUY

### 6.1 The honest case for it, counted

The brief's example is real and was measured this week: phase 54's anchor
`\{"(\w+)",` missed two `options[]` rows for the life of the pipeline because vim
writes them `{"statusline"  ,"stl",`, so the phase computed 172 where the truth
was 174.

How much of that is there?

- **156 sites in `slim-vim.c`** where a string literal is followed by whitespace
  and then a comma — the exact shape that defeated the anchor. **59 survive in
  `whim-vim.c`.** Two of them are at the start of a braced row
  (`^\s*\{"[a-z]+"[ \t]+,`), which is the measured pair.
- **585 `regexp.MustCompile` patterns** across `internal/cut`, `internal/dead`,
  `internal/edit` and `phase/*/edit*.go`. **211 pin C punctuation** (an
  unescaped-meta-excluded `,` `;` `=` or an escaped brace or paren). Of those,
  **23 allow whitespace before at least one such punctuation and 188 do not** —
  across 62 files, **35 of them phase edit files**.
- **325 `FindAll*` call sites across 116 files, 62 of the 108 phase edit files.**
  These are the phase-54 shape specifically: an anchor that computes a SET by
  scanning, where a miss is not a refusal but a smaller set. CLAUDE.md's rule is
  "assert a partition, not a count"; these 325 sites are where the rule is
  hardest to keep, and phase 54's floor-of-100 is a count, not a partition.

That is the real argument, and it points at AST **locating**, not AST **editing**.

### 6.2 The argument against, counted, and it is stronger

- **`internal/cutil/norm.go` records that 148 of the 163 phases' anchors stop
  matching if the same C is printed canonically.** The canon migration is 136
  files and ~1,850 changed lines of re-anchoring, and is not finished.
- **`internal/edit/driver.go` records the experiment that settles it**: routing
  anchors through the whitespace-insensitive matcher moved the canonical build's
  refusals from 148 of 163 to 147, **and changed the committed product in at
  least two places**, because a widened match picks a DIFFERENT site where two
  sites differ only in spacing (`&p_rtp )`; phase 71's wiped-fnum branch). An AST
  is a maximally widened match. Being insensitive to layout is not neutral here:
  it changes which of two spacing-variant sites is edited.
- **The pipeline's gate is byte identity.** `make whim-build-check` requires the
  committed `whim-vim.c` back from the committed `slim-vim.c`, byte for byte.
  Indentation, blank runs and residual macro spacing are part of the product.
- **`ORDER IS OUTPUT`** (`internal/cut/edits.go`): a phase's log is read by a
  human comparing two phase commits, and grouping same-shaped edits into a loop
  is a failure even when the bytes are identical. An AST rewrite that visits
  nodes in tree order does not visit them in the order the report was written in.

### 6.3 Where the text anchor is the RIGHT tool

Not a concession — a result. The text is the authority for: `#include` lines and
their POSITION (phase 110: the boundary is "marked by nothing else"); comments
(phase 82 deletes all of them; a C AST contains none); the Makefile (0, 83, 84);
the contents of C string literals, where `internal/edit/literals.go` exists
because a line-wise `NULL`→`nullptr` in phase 106 rewrote three string literals
and moved 1,598 bytes of the binary, 1,354 of them in `.rodata`, *which neither
verification tier can see*; blank-line runs, which the deltas count; and phase
110's fixpoint over gcc's diagnostics.

That last one is also the strongest argument FOR a lexer: the reason the literal
scanner had to be hand-written is that the text tools do not know where a string
ends. A parser hands that over free — and does so from `cc.Parse` alone, at every
intermediate text, per §2.

---

## 7. RECOMMENDATION, IN COST ORDER

**The answer to "should the phases edit the AST?" is no, and the answer to
"should the phases USE the AST?" is yes, for a named subset.** In cost order:

### Step 1 — print from `cc.Parse`, not `cc.Translate` (cheapest, do it anyway)

Change one call in `internal/cemit/emit.go`. **What it buys:** canonicalisation
becomes available on every intermediate text, including the whole
edit-before-sweep window that both earlier surveys called unparseable; and the
parse gets 15–29 % cheaper. **What proves it:** the three byte-identical
comparisons of §2.2, re-run in the tree; then `make whim-build-check`. **What
could go wrong:** a type-dependent printing decision I did not find. Three
byte-identical whole-file comparisons on 9 MB of C is decent but not a proof; the
gate catches it.

This one belongs to the canon migration, not to this survey, and is worth passing
to whoever owns that branch.

### Step 2 — AST as a LOCATOR, text as the EDITOR (the hybrid; recommended)

Use `cc.Parse` to compute BYTE SPANS, and keep the existing text splice. Concretely:

- replace `cutil.FindDefinition`'s backwards line walk — which stops on a blank
  line, a `;}{:`, a `#` or a comment and is a guess about which preceding text
  belongs to a definition — with the declaration node's real extent;
- replace `cutil.Body`/`ReplaceBody`'s brace matching from a `^name(` head with
  the `CompoundStatement`'s span;
- give the 325 `FindAll*` sites an AST-computed denominator to assert against, so
  that a scan producing 172 where the tree says 174 REFUSES instead of
  proceeding. This is the phase-54 fix, and it needs no mutation at all.

**What it buys:** every one of §6.1's 188 undefended anchors gets a partition to
be checked against, and the `(?:[^\n]*\n)*?\}` family of bugs — which
`internal/cutil/fold.go`, `internal/dead/funcreach.go` and
`internal/dead/typereach.go` each record finding the hard way — stops being
possible. **What it does not disturb:** the bytes. The splice is still the text's,
so layout, order and the report are unchanged. **What proves it:**
`make whim-build-check` gives the committed `whim-vim.c` back byte for byte; a
locator that returns a different span moves the product and is caught at once.
**What could go wrong:** a span the parser computes differs from the one the text
tool computed — which is the point, and each difference has to be looked at
rather than accepted.

### Step 3 — AST mutation for whole-declaration and statement deletion only

The two operations measured clean in §1.4: 294 whole-declaration deletions and 60
statement deletions, 0 failures, 0 hazardous block items in 113,155 measured.
That covers `DeleteDefinition` (18 sites, 7 phases), `funcreach --delete`, and
the statement half of class (a).

**What it must NOT cover, on the measurements here:** initializer-list elements
(`dropoptions`, 53 step calls — 1,321 hazardous sites in the input, 829 in the
product, 10 of 10 verified silently no-ops), enumerators (0 hazardous today, but
the mechanism fires and one macro in an enum initialiser turns it on), any
identifier rewrite, and any insertion. **What proves it:** the hazard census,
re-run on the actual text each phase is handed, as a refusal: if any node the
phase is about to unlink is hazardous, refuse. That check is cheap — 1.6 s on the
product, 3.4 s on the input — and it turns a silent wrong answer into a stop.

### Step 4 — full AST mutation: NOT recommended, and here is what it would take

In order:

1. **Record expansion extents at parse time** instead of inferring them from the
   next surviving token (`cc.Token` already carries `*Macro`). That kills the
   1,321/829 hazard and makes `Token.Set` usable on identifiers. It is a change
   to `internal/cc`, whose discipline is to differ from upstream in two
   productions.
2. **An exported way to build tokens and nodes with no position**, so that a
   spliced fragment does not resolve against the host's source. Without it,
   insertion is measured broken, and insertion is 1,288 lines of new C at 290
   sites in 51 phases, plus 606 spliced from outside the tree.
3. **A story for comments and `#include`s**, which no C AST holds and which
   phases 82, 99 and 110 exist to move.
4. **Re-anchoring all 106 bespoke edit programs** by node identity — 27,915 lines
   — on top of the canon migration's 136 files, with the knowledge that the one
   experiment already run (widening the match) *changed the product*.
5. **A tree that survives a sweep**, or the acceptance that 87 phases re-parse.
   Two of the sweep's seven members ask gcc and DWARF; they have no AST answer.

Against that: §3.3's 11 phases and 15 steps have no AST formulation at all, so
even a complete success leaves two kinds of phase in the tree.

### What the gate would catch, and what it would not

`make whim-build-check` is total for anything that moves a byte, and every
failure in §1 that CHANGES the output would be caught by it instantly. The one
that would not is the one measured 10 times out of 10: **a deletion that produces
byte-identical output**. The tree loses the element, the text keeps it, the
product is unchanged, the check passes, and the phase's declared delta — which is
measured on the text — agrees. That is why §1, and not §5, is the answer to this
survey.

---

## 8. Summary of the numbers

| | measured |
|---|---|
| whole-declaration deletions clean | 294 / 294 |
| statement deletions clean | 60 / 60; 0 of 113,155 block items hazardous |
| enumerator cells hazardous | 0 of 1,155 (product), 0 of 2,898 (input) |
| initializer-list elements hazardous | **829 of 15,185** (product), **1,321 of 25,891** (input) |
| hazardous deletions that silently did nothing | **10 / 10** |
| identifier-use renames that silently did nothing | **6 / 6** |
| fragment insertions: dropped / corrupted / correct | **2 / 3 / 3** of 8 |
| declarations containing a macro expansion | 746 of 4,474 (product), 1,924 of 8,687 (input) |
| intermediate texts where `cc.Parse` failed | **0 of 24** |
| intermediate texts where `cc.Translate` failed | 7 of 23 phases, in three windows |
| printer's use of type information | 0 |
| `Parse` vs `Translate` output | byte-identical on 3 texts, 9 MB |
| `Parse` vs `Translate` cost | −29 % (input), −15 % (product) |
| round trip | 1.40 s / 2.36 s / 2.88 s at 80k / 134k / 185k lines |
| 163 round trips against a 1,220 s build | +19 % to +38 % |
| phases followed by a sweep | 87 of 163, plus 14 inner sweeps |
| anchors pinning C punctuation with no whitespace allowance | 188 of 211, in 35 phase edit files |
| `FindAll*` sites (a set computed by scanning) | 325, in 62 of 108 phase edit files |
| sites in the input where a tight `",` anchor misses | 156 (59 in the product) |
| phases whose anchors stop matching under canonical text | 148 of 163 (the tree's own measurement) |
| phases with no AST formulation at all | ≥ 11, plus 15 assert-only steps |
