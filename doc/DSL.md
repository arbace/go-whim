# Would a language for phase edits be viable, and worth it?

Survey, 2026-09-25. Main checkout at `ee05eee` with the working-tree changes

**Since written:** the prototype it ran (`.tmp/dsl/`) is not tracked. Its remark that CLAUDE.md said 164 phases and no test suite was checked and is wrong: CLAUDE.md said neither.

**Step 3, line anchors, is done too** (2026-09-25): `cutil.Line` and `cutil.Head`
(aliased in `edit`) spell an anchor as the C it matches, and build the same
regular expression phases wrote by hand. 418 anchors use them: 116 whole-line
literals, 244 fold heads, and 58 `Lines` calls that became `Cut(Line(...))`.
The 271 left as regular expressions use a regex feature. Each conversion was
mechanical, and `whim-build-check` reproduced every link and the product.

**Since written, the embedded half was done (steps 1 and 2 of §4).** The
census was re-run first, on `986e8b9`, after the redundant-steps removals:

- 111 phases have an edit program (099's went), **28,035 lines**, 18,996 of
  them code: 1,198 fewer than below. By the markers alone, D0 13, D1 16 and
  C 82, as before; the removals thinned the C tier's layout code and hand
  deletions (`Lines` 133 -> 70 sites), not the tiers.
- **55 `Text`/`Set` calls** (56 below), and 253 fold calls in 18 spellings.

What changed:

- **One verb set.** `edit.E` went from 51 exported methods to 34, and its
  acts from 37 to 17. The folds are `FoldNever`, `FoldAlways`, `DropIf` and
  `FoldAlwaysElse`. Every act that could match more than once takes its count
  before its `what` (`Literal(old, new, n, what)`); `LiteralN`, `Term`,
  `LinesT`, `BodyTrue`, `Always`, `DropWalkIn` and the *Count/N/Repeat/Many/In*
  folds are gone. The last-match folds turned out to give the same phase
  output as the first-match ones once every boundary is printed canonically,
  so the drift §1.2 describes had no product behind it, only log lines.
  `driver.go`'s doc comment is the list.
- **Verbs for what phases hand-wrote:** `Splice` (two literal ends, each
  counted once), `InTable` (a scope over an initialised table, as
  `InFunction` is over a function), `Expect` (an invariant in the phase's own
  words, one line), `Query` (a capture group of every match). `Rename` was
  added and removed again: the one rename (072) is a counted `Sub`.
- **Moved:** the 20 middle-tier phases but 054 (a query that prints, not an
  edit) and 100-102 (assertion programs: their one act is a literal, and the
  rest is preconditions that `Ph`'s early return serves as well); the
  nineteen declarative phases written against `edit.Ph`; and the E phases
  whose `Text`/`Set` was a splice, a fold or a count (44, 48, 55, 69, 70, 73,
  75, 78, 81, 135). 137's layout function is the 18-count `Sub` measured in
  §3.4.
- **After:** 27,467 lines (18,432 code); by the markers D0 30, D1 14, C 67;
  **16 `Text`/`Set` calls**, 14 of them in 080's command-table rebuild and
  two reads for a computed count (075, 078).

Not done: step 0 (`whim build --from N --to N` and a `cmp` against the
committed `qN` served instead, with a control), and step 3 (line anchors).
`git status` showed at the start (phase 166, `internal/sweep/prune.go`,
`src/whim-vim.c`, the editor). **No tracked file was modified and nothing was
committed.** No build that writes `.cache/boundaries` was run: four boundaries
were *copied* out of it (`q060`, `q136`, `q157`, `q159`) into `.tmp/dsl/b/`, and
every comparison below runs both variants on the same copied input, so it does
not matter whether those boundaries match the running build.

The instruments are untracked, under `.tmp/dsl/`:

| path | what it is |
|---|---|
| `census/` | a `go/ast` program (its own module) that classifies every call in `internal/phase/*/edit*.go` and `internal/cut/*.go`; output in `census/out.txt`, per-phase line split in `census/lines.txt` |
| `anchors/` | a `go/ast` program that classifies the pattern argument of every `edit.E` pattern verb (a C line with its indentation left open, or a real regular expression) |
| `wed/wed.go` | a **working prototype** of the language: a 300-line parser (`text/scanner`) and interpreter that calls the `edit.E` driver |
| `progs/*.md` | phases 158, 61, 160 and 137 written in it |
| `run/main.go` | a differential runner: the Go edit and the language edit on the same `q(N-1)`, compared on the edit's output bytes, on its log, and on the phase's output after the sweep and the canonical print (`build.Advance`) |
| `ctl/` | two controls: a count and a replacement deliberately changed |

## Short answer

- **Viable: yes, and measured.** A 300-line prototype ran phases 158, 61, 160
  and 137 from data files. The edit output and log were **byte-identical** for
  158, 61 and 137. Phase 160 differs by 2 bytes before the canonical print,
  because the prototype writes a call's spacing the canonical way, and the
  **phase output is byte-identical after the canonical print.** Both controls
  were caught: a changed replacement came out different, and a changed count was
  refused with the driver's own message.
- **Coverage is about a third.** Of the 112 phases that run an edit program:
  - **36 (32 %) are declarative now.** Their programs are 2,890 lines, 10 % of
    the edit code.
  - **20 (18 %) are declarative apart from one or two small computations,**
    3,504 lines. A few more verbs would cover these.
  - **56 (50 %) need real code,** 22,839 lines, 78 % of the edit code. They
    compute a set from the text, use the compiler or `internal/cc` as an oracle,
    or restructure control flow.

  Measured by line count, a language covers the part of the pipeline that is
  already small.
- **The Go that `edit.E` gives today is already an embedded language**
  (accumulated errors, one verb per act, a count on every act). A separate
  language would mostly save boilerplate: about 10 lines per phase, around 360
  lines in total. The larger savings found here, the 25-line layout function in
  phase 137 and the helpers in 150 and 160, can be had in Go just as well.
- **Recommendation: do it partially.** Harden the embedded language: one
  orthogonal verb set, the idioms the middle tier keeps writing by hand made
  into verbs, and the layout logic the canonical print made redundant deleted.
  **Do not introduce an external grammar now.** It works, as the prototype
  shows, but it splits "one path" into two phase languages to cover 10 % of the
  code. §4 gives the effort and a migration path in which every step is proved
  byte-identical.

---

## 1. What the edits actually do

### 1.1 The shape

- `internal/build/plan.go` has **169 phases** and 297 steps.
- **112 phases have an `edit.go`**, 33 of them with an `editlit.go` beside it.
  That is 145 files and **29,233 physical lines**, of which **20,014 are code
  lines**, meaning lines that hold a token outside a comment. Phase 155's program
  lives in `internal/edit/shared.go` (`Whim155`) and its `edit.go` is 11 lines of
  comment.
- The other 57 phases have no edit program. Their plan entry names cutters with
  arguments:
  - 97 parametric step calls: `dropoptions` 53, `retire` 21, `droplocal` 20,
    `dropopts` 3;
  - the once-used cutters of `internal/cut`: 54 files, **10,269 lines**.

  **`plan.go` is already a table-driven embedded language** at the level of
  steps.

### 1.2 The operations, by kind

These are call sites counted by `census`, walking every function in
`internal/phase/*/edit*.go`, helpers included:

| kind | verbs | sites |
|---|---|---|
| scope to one function | `InFunction` | **236** |
| fold an `if` | `DropIf` 88, `FoldNever` 88, `FoldAlways` 16, `FoldNeverIn2` 15, `FoldNeverCount` 6, `FoldAlwaysCount` 4, `FoldWalk` 5, `FoldNeverRepeat` 4, `FoldNeverIn` 4, `BodyTrue` 3, `DropWalk` 3, `FoldWalks` 3, and 10 more spellings | **253** |
| literal replace, counted | `Literal` 131, `Term` 55, `ReplaceAnchor(B)` 15, `LiteralN` 8, `SwapOnce` 8, `ReplaceAll` 6, `LiteralOrGone` 3, `Once` 1 | **227** |
| delete whole lines by pattern | `Lines` 133, `LinesT` 5 | **138** |
| regex cut, counted | `Cut` | **71** |
| regex substitute, counted | `Sub` 26, `SubNotAfterWord` 4 | **30** |
| replace a function body | `Body` 23, `ReplaceBody` 7 | 30 |
| delete a definition | `DeleteDefinition` | 23 |
| blocks and splices | `DropBlocks` 5, `ReplaceBlock`, `DropBareBlock`, `Splice` 2 | 9 |
| **assertions** (counts, mentions, once, gone, const) | `Mentions` 52, `Contains` 22, `AssertOnce` 16, `CountAnchor(B)` 17, `Lines(count)` 10, `CallsNotAfterWord` 9, `MentionCount` 7, `BlankRuns` 5, `CoreCalls` 5, `ConstOf` 3, … | **149** |
| **escape hatch**: take the text and put it back | `e.Text()` 36, `e.Set()` 20 | **56** |
| text-structure primitives used by hand | `cutil.Blank` 49, `cutil.Match` 38, `FindDefinition` 22, `LiteralSpans` 10, … | ~125 |
| report lines | `Say` 190, `Sayf` 150 | 340 |
| refusals | `Die` 761, `Refuse` 33, `Refused` 29 | **823** |

The `internal/cut` cutters are the older, lower-level style: raw `cutil` plus
hand splicing with `append`, for example `nofind.go`. Their census:

- `cutil.Blank` 63, `Match` 61, `DropIf` 33, `DeleteDefinition` 21,
  `FindDefinition` 14, `Dedent4` 14, `Body` 12, `FoldNever` 9;
- 237 verb sites, of which 126 have constant arguments.

**The vocabulary has drifted.** A `FoldNever` has seven spellings (plain,
`Count`, `N`, `Repeat`, `Many`, `In`, `In2`). They differ in how the count is
applied (first match n times, or last match) and in whether the log line
carries `(n)`. `driver.go` keeps them apart on purpose, because each
reproduces a Python original's log. An external language would have to either
keep all seven or change the log.

### 1.3 The arguments

For each verb call (reports, refusals and `InFunction` excluded), are the
pattern arguments constant?

| | constant | templated (`fmt.Sprintf` / loop variable over a literal list) | computed |
|---|---|---|---|
| phases | **732 (75 %)** | 57 (6 %) | 184 (19 %) |
| cutters | 126 (49 %) | 18 (7 %) | 111 (44 %) |

`anchors` looked at the 449 pattern arguments of the `edit.E` pattern verbs
(`Sub`, `Cut`, `Lines`, the folds, `CountIs`):

- **381 (85 %) are one or more literal C lines** once the indentation idioms
  (`^[ \t]*`, `[ \t]+`, `\n\n?`) are removed;
- 42 (9 %) use real regular-expression features: alternation, `[^\n]*`,
  capture groups (several of which only capture indentation), `(?s:.*?)`;
- 26 (6 %) are computed.

The counts written are 1 in 234 of 279 cases, then 2 (23), 3 (7), 6 (4), and
single cases up to 15.

So a language whose default anchor is **a C line matched after the indentation**,
with a regular expression as the exception, fits 85 % of what is written. On
canonical text that anchor is exact, because the printer fixes spacing and
indentation (the redundant-steps survey, G1, v12, in git history at 6f8c1e9).

### 1.4 Which phases could be declarative

`census` gives a first tier:

- **D0**: verbs with constant arguments and error plumbing only;
- **D1**: also loops over literal lists, `if count != N { refuse }`, and
  `fmt.Sprintf` messages;
- **C**: anything else: computed loops, logic `if`s, `switch`, maps,
  `FindAll`/`ReplaceAllFunc`, `strings`/`bytes` surgery, `Text`/`Set`,
  `cc`/`sweep`/`exec`.

Every C phase with a low logic score (≤ 24 markers) was then read by hand, and
**re-tiered where its "logic" is a verb spelled by hand**. Examples:

- `bytes.Count` followed by `bytes.ReplaceAll` is `LiteralN` (135, 156);
- `Text`/`Set` around a `cutil.FoldNever` is `FoldNever` (70);
- a local `cutOnce` closure is `Cut 1` (48);
- a local `lit` closure is `LiteralN` (150);
- `if !e.Failed() { e.Say(...) }` is plumbing (68).

| tier | phases | physical lines | code lines | of which data/C payload | report/refuse | verb calls | the rest (logic + boilerplate) |
|---|---|---|---|---|---|---|---|
| **D** declarative | **36** | 2,890 (10 %) | 1,538 | 476 | 35 | 520 | 507 |
| **M** mostly declarative | **20** | 3,504 (12 %) | 2,205 | 546 | 168 | 791 | 700 |
| **C** real code | **56** | 22,839 (78 %) | 16,271 | 2,448 | 1,966 | 541 | **11,316** |
| total | 112 | 29,233 | 20,014 | 3,470 | 2,169 | 1,852 | 12,523 |

The column headings mean:

- **code lines**: lines holding a token outside a comment;
- **data/C payload**: multi-line or `\n`-bearing string literals, lines of
  literal tables, and import blocks;
- **report/refuse**: lines inside `Say`/`Sayf`/`Die`/`Refuse` calls.

**D (36):**

- 033, 044, 048, 055–058, 060, 061, 065, 068, 069, 070, 073, 074, 085;
- 129, 130, 131, 133, 135, 136, 138, 140, 142, 148, 153, 154, 156–162
  and 165.

Several are a single literal replacement (129, 130, 148, 153, 154, 158). Others
run a local `[]struct{Old, New, What}` table through
`p.Literal` (131, 136, 157, 161, 162, …).

**M (20):** these are declarative except for the item given.

| phase | what it needs beyond the verbs |
|---|---|
| 054 | a query: print the capture of a regular expression |
| 059, 062, 063, 064, 066, 071 | one or two `Text`/`Set` splices between anchors |
| 067, 072 | computed name lists from a regular expression, and a word-bounded rename |
| 077 | rows computed from the command table |
| 079 | tables of constants held in Go maps |
| 100, 101, 102 | preconditions: `Mentions == N`, `AssertOnce`, `HasSuffix` |
| 137 | a 25-line layout function, **proved redundant below** |
| 139, 146 | `ReplaceAllFunc` with a capture-group condition |
| 147 | a conditional on `#include` count |
| 150 | a local `lit` helper over templated anchors |
| 155 | a capture-group rewrite with a consistency check between two groups |

**C (56), and why they need code.** The grouping below is by reading and by the
markers `census` found:

1. **The tree as locator** (`internal/cc` and `sweep.Walk`, text cut by span):
   164 (statements after a jump), 166 (functions answering a question become
   `bool`: 870 lines, 97 logic `if`s, 18 `switch`es), 167 (key codes get
   names). 149 does a text-level non-null dataflow (309 lines).
2. **The compiler as oracle, or a fixpoint:**
   - 110: the first `#include` becomes the boundary, and what moves is a
     fixpoint over gcc's unused-symbol diagnostics, through the only
     `exec.Command` in any phase;
   - 082: comments, and the `#include`s;
   - 099: the includes nothing names;
   - 134: empty blocks folded to a fixpoint.
3. **Computed partitions.** The set is derived from the text, not listed, so
   that a moved upstream still gets the right cut. Phase 54's comment says
   *"Nothing here excludes a name and nothing lists one: that is the point of
   computing the set."* The phases: 002, 075, 076, 078, 080 (705 lines: the
   command table against `delta.md`), 081, 087–098, 105–109, 111–115,
   117–121, 124–126.
4. **Structural rewrites no verb says:**
   - goto elimination: 141, 143, 144, 145;
   - line-array surgery and synthesis of definitions: 122 (819 lines), 127
     (909), 128 (1,024);
   - per-call rules: 132;
   - retyping the option table: 151, 152;
   - moving the host block: 103, 104.

A language would not remove this code. It would push it into escape hatches.
The part of it that is *accidental* is:

- layout handling that the canonical print now makes redundant
  (the redundant-steps survey: about 35 blocks in 16 phases, about 300 hand
  deletions the sweep does anyway);
- the Python port's precondition prose, 1,966 report/refusal lines in the C
  tier.

Neither needs a new language to remove.

---

## 2. How a DSL is done in Go, applied here

### 2.1 Embedded: a fluent driver, or Go data (what the repo does now)

**Idioms.**

- **A sticky error.** Every method is a no-op once one has failed, and the
  caller checks once at the end. This is the *"errors are values"* pattern of
  `bufio.Scanner` and the `errWriter` example. `edit.E` is exactly this, and its
  comment says why: *"a port then reads as the phase's argument and nothing
  else"*.
- **Tables as data.** A slice of struct literals walked by one loop:
  `plan.go`'s `[]Phase{{N:…, Steps: []Step{{Op:…, Args:…}}}}`, and phases
  131, 136, 157, 161 and 162's `[]struct{Old, New, What string}`.
- **Closures as scope**: `e.InFunction("f", func(e *edit.E) {…})`.
- Elsewhere in Go:
  - ent's schema builders (`field.String("x").Unique()`);
  - squirrel's SQL builder;
  - Mage, where the build is Go functions;
  - go-ruleguard, where rules are Go syntax that is type-checked but read as
    an AST rather than executed.

**Cost:** none, it exists. **What it gives:**

- all of Go, used by 56 phases;
- typed refusals with computed context: 761 `Die` sites, most of which format
  what was found;
- `go vet`, the IDE and `gofmt`;
- one language for every phase.

**What it loses:**

- about 10 lines of boilerplate per phase (package, import, `init`/`Register`,
  signature, `New`, `Done`) and a line in `registry.go`;
- **nothing stops a phase reaching past the verbs** (56 `Text`/`Set` escapes,
  local `lit`/`cutOnce`/`arm` helpers re-implementing `LiteralN`/`Cut`);
- vocabulary drift (§1.2);
- an error names the phase tag and the "what", but not a source position.

### 2.2 Data files: `lowercase.md` with a fenced block (the repo's own convention)

**Idioms.**

- `//go:embed` plus a small reader of the fence. The repo already does this:
  `build.declared()` reads `internal/phase/080/delta.md`, `internal/suite`
  embeds `cases.md` (one tab-separated line per case), and phase 98 embeds
  `musl-*.md`.
- **Format choice.** JSON is the only stdlib format; YAML/TOML/HCL
  (`hashicorp/hcl/v2`, which carries source ranges into diagnostics) and CUE
  (constraints) are modules. **For this payload the format that fits is Go's own
  string tokens.** The anchors are C with backslashes, quotes and regular
  expressions. In YAML or JSON every `\(` becomes `\\(`; as Go raw strings
  (`` `…` ``) they are written exactly as the phase authors write them today.
  `text/scanner` tokenises Go strings, identifiers and integers, skips `//`
  comments and reports `file:line:col`, which makes a hand-written parser a page
  long. The prototype's parser is about 150 lines.

**Cost:**

- interpreter plus parser: about 300 lines for the verbs used by the prototype
  phases, and an estimated 600–700 for the ~30 verbs the D and M tiers use;
- one `//go:embed */edit.md` in `internal/phase` (embed may reach into
  subdirectories that are other packages);
- one `Register` loop.

**What it gives:**

- **the file is only the argument.** A diff of an anchor is a diff of data;
- **the count becomes syntax** (`cut 6 …`), and the parser can *require* it;
- **errors carry a position** (`061/edit.md:14:5`) as well as the driver's
  message;
- a phase with no Go needs no package and no registry line;
- a checker can dry-run every anchor of every phase against its `q(N-1)` in
  seconds, without the sweep.

**What it loses:**

- arbitrary logic. The first loop or computed set pushes a phase back to Go,
  and the rewrite is a migration of its own;
- typed refusals with computed context. The language can refuse with a
  message, and the driver's `-- matched k times, expected n`, but not "these
  three names are still live";
- a single phase language.

### 2.3 An external grammar

**Hand-written recursive descent.** This is the Go norm: `go/parser` and
`text/template`'s parser are both hand-written, and `text/template`'s lexer is
Rob Pike's state-function scanner (*Lexical Scanning in Go*). It is best done
over `text/scanner` when the tokens are Go-like, as they are here.

**Generators:**

- `goyacc` (`golang.org/x/tools/cmd/goyacc`), LALR;
- `participle`, a grammar written as struct tags, good for small grammars;
- `pigeon` (PEG);
- ANTLR's Go target.

All of them add a build-time or module dependency and give worse error messages
than a hand-written parser. **This grammar has no expressions and no precedence
(verb, count, strings, braces), so a generator buys nothing.**

**`text/template`** is for *producing* text. It could template inserted C (phase
110 generates 24 lines from a table), but it cannot match, and a template with
logic is Go written worse. Not a fit.

**`go/parser` reuse**, the go-ruleguard and `gofmt -r 'a -> b'` style:

- write the rules in a subset of Go syntax, parse them with `go/parser`, and
  interpret the AST;
- gains: `gofmt`, syntax highlighting, no parser to maintain;
- cost: it *looks* like Go and is not, which is confusing next to 56 phases
  that are real Go.

The C analogue, patterns parsed by `internal/cc` and matched on the tree, is
the *locator* half of `doc/AST-EDITING.md`'s recommended hybrid. That is
possible for matching. For editing it is blocked by what that document measured
(cemit maps tree to source by position).

**Starlark** (`go.starlark.net`) is a deterministic, hermetic Python dialect
used by Bazel and embedded from Go. It would give loops and conditions without
Go. Against it: a second general-purpose language, and *"There is no Python
here."* No.

### 2.4 Existing tools (none installed here: `spatch`, `comby` and `semgrep` are absent, so nothing below was run)

**Coccinelle** (SmPL semantic patches):

```
@@ expression X; @@
- ((X)->b_ct_di.di_tv.vval)
+ X->b_changedtick
```

It matches structurally, with metavariables, modulo formatting, and it is the
Linux kernel's tool for exactly this kind of edit. It does not fit here:

- an OCaml binary beside a pure Go and gcc toolchain;
- **no "exactly N" contract**: zero matches is a silent no-op, and
  isomorphisms *widen* matches, the opposite of refusing on a moved anchor;
- C23 support (`nullptr`, `bool` as a keyword, `[[…]]`, `typeof`) is
  uncertain and was not measured;
- its main advantage, matching modulo formatting, is what the canonical print
  already guarantees.

**comby:** `:[x]` holes that match balanced delimiters, aware of strings and
comments. It has the same dependency and counting objections. **The idea is
worth taking**: a hole matcher over canonical text is roughly 200 lines of Go,
and it would replace many of the 42 real regular expressions, for example
`sub 18 "((:[x])->b_ct_di.di_tv.vval)" ":[x]->b_changedtick"`.

**semgrep** supports C but needs Python and OCaml. **clang's
LibTooling/Rewriter** is C++.

**Go-only tools, as idioms:** `gofmt -r`, `eg` (refactoring by example
before/after functions) and `gopls`'s rewrites.

---

## 3. Prototypes on paper, then run

All four ran through `run/main.go` on a copied `q(N-1)`. Line counts:

- **Go** is physical lines of `edit*.go`, with code lines (non-comment,
  non-blank) in brackets;
- **DSL** is the lines inside the fence, with non-comment non-blank lines in
  brackets.

| phase | tier | Go | DSL | edit bytes | edit log | phase output after sweep + canonical print |
|---|---|---|---|---|---|---|
| 158 (trivial) | D | 39 (14) | 6 (5) | identical | identical | identical |
| 61 (medium) | D | 92 (60) | 49 (41) | identical | identical | identical |
| 160 (declarative with a little logic) | C by the markers, D once written with verbs | 83 (57) | 26 (22) | **differs by 2 bytes** (see below) | identical | **identical** |
| 137 (bonus: logic the canonical print made unnecessary) | M | 93 (59) | 7 (6) | **identical** | identical | identical |

**Controls** (`CONTROL=1`, `ctl/`):

- 158 with `t_colors > 2` came out *not* identical, at the edit and at the
  phase;
- 61 with `cut 5` where the text has 6 refused with
  `notitle  changes asking for a title update -- matched 6 times, expected 5`.

Each `build.Advance` took 2.7–5.4 s for either variant, so the interpreter's
cost is not measurable against the sweep.

### 3.1 Phase 158: trivial

Go (the `Edit` and what it needs; 33 non-blank lines with the doc comments):

```go
package p158

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim158", Edit) }

const (
	W158Font  = "        if (aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
	W158Guard = "        if (t_colors > 1 && aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
)

func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "font", W: w}
	return p.Literal(text, W158Font, W158Guard, "screen_start_highlight() reads cterm.font only when t_colors > 1 says the entry is a cterm entry", 1)
}
```

`internal/phase/158/edit.md` (the prose around the fence is the doc comment's
place):

````
```
edit font

literal
    "        if (aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
    "        if (t_colors > 1 && aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
    : "screen_start_highlight() reads cterm.font only when t_colors > 1 says the entry is a cterm entry"
```
````

The only thing gained is the removal of the package, import, register and
signature lines, and the `registry.go` line.

### 3.2 Phase 61: medium (scopes, folds, counted cuts, a mentions check, template loops)

Go (`Edit` only, 60 code lines in the file):

```go
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("notitle", text, w)
	for _, fn := range []string{"showruler", "redraw_cmd", "ui_focus_change", "main_loop"} {
		fn := fn
		e.InFunction(fn, func(e *edit.E) {
			e.DropIf(titleWanted, fmt.Sprintf("%s updating the title", fn))
		})
	}
	for _, fn := range []string{"enter_buffer", "buf_name_changed", "do_ecmd", "set_termname", "set_shellsize_inner", "win_enter_ext"} {
		fn := fn
		e.InFunction(fn, func(e *edit.E) {
			e.Cut(`(?m)^[ \t]*maketitle\(\);\n`, 1, fmt.Sprintf("%s updating the title", fn))
		})
	}
	e.InFunction("do_exedit", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(n != curwin->w_arg_idx_invalid\)$`,
			":edit updating the title when the argument index moved")
		e.Cut(`(?m)^[ \t]*n = curwin->w_arg_idx_invalid;\n`, 1,
			":edit remembering the argument index for the title")
		if k := e.Mentions("n"); !e.Failed() && k != 3 {
			e.Refuse("do_exedit mentions n %d times after the title went, expected 3 (declaration, readonlymode save and restore)", k)
		}
	})
	e.InFunction("ex_stop", func(e *edit.E) {
		e.Cut(restoreTitle, 1, ":stop restoring the title")
		e.Cut(`(?m)^[ \t]*maketitle\(\);\n[ \t]*resettitle\(\);\n`, 1, ":stop setting the title again")
	})
	// … mch_exit, vim_main2, clear_termoptions, value_changed, set_init_3: one Cut each …
	e.Cut(`(?m)^[ \t]*need_maketitle = TRUE;\n`, 6, "changes asking for a title update")
	return e.Done()
}
```

DSL (41 lines):

````
```
edit notitle

in showruler redraw_cmd ui_focus_change main_loop {
    drop-if `(?m)^[ \t]*if \(need_maketitle\)$` : "$fn updating the title"
}
in enter_buffer buf_name_changed do_ecmd set_termname set_shellsize_inner win_enter_ext {
    cut 1 `(?m)^[ \t]*maketitle\(\);\n` : "$fn updating the title"
}

in do_exedit {
    drop-if `(?m)^[ \t]*if \(n != curwin->w_arg_idx_invalid\)$`
        : ":edit updating the title when the argument index moved"
    cut 1 `(?m)^[ \t]*n = curwin->w_arg_idx_invalid;\n`
        : ":edit remembering the argument index for the title"
    // the declaration, :view's readonlymode save and its restore
    expect mentions n 3 : "declaration, readonlymode save and restore"
}

in ex_stop {
    cut 1 `(?m)^[ \t]*mch_restore_title\(\(SAVE_RESTORE_TITLE \| SAVE_RESTORE_ICON\)\);\n`
        : ":stop restoring the title"
    cut 1 `(?m)^[ \t]*maketitle\(\);\n[ \t]*resettitle\(\);\n` : ":stop setting the title again"
}
// … mch_exit, vim_main2, clear_termoptions, value_changed, set_init_3 …

cut 6 `(?m)^[ \t]*need_maketitle = TRUE;\n` : "changes asking for a title update"
```
````

The regular expressions are kept verbatim so that the transliteration is
identical by construction. The next step, a *line anchor*, would shorten
`` `(?m)^[ \t]*maketitle\(\);\n` `` to `line "maketitle();"`. 85 % of the
anchors have that shape (§1.3); each change would be proved on its phase.

### 3.3 Phase 160: mostly declarative, with a little logic

The Go (57 code lines) hand-writes four things the verbs already say:

- a word count asserted at 19 (`regexp.FindAll` plus `if len != 19 { Die }`);
- a counted regex delete (`FindAll`/`ReplaceAll(nil)` with a count check);
- a counted capture substitution (`ReplaceAll("${1}, ")` with a count check);
- a final "no longer named" check.

It also runs a local `[]struct{Old, New, What; n}` table through `p.Literal`.

The DSL (22 lines):

````
```
edit cookie

expect count `\bcookie\b` 19 : "cookie is named 19 times, as this phase was written against"

literal-or-gone 1
    "typedef long (*find_func_t)(const char *line, long line_len, char *buffer, long buffer_size, void *priv);\n" ""
    find_func_t
    : "find_func_t, a typedef nothing names, goes"
literal 9 "(int, void *, int, getline_opt_T)" "(int, int, getline_opt_T)"
    : "a line getter takes no cookie: its type"
cut 9 `,\s*void\s+\*cookie\b`
    : "9 declarations of do_cmdline(), getline_equal(), do_one_cmd(), getexline() and getcmdkeycmd() take no cookie"
sub 6 `(do_cmdline\([^;]*?, (?:nullptr|getexline|getcmdkeycmd)), nullptr, ` "${1}, "
    : "6 calls of do_cmdline() pass none"

literal "    void *cookie;\n" "" : "exarg_T holds none"
literal "eap->ea_getline(NUL, eap->cookie, indent, " "eap->ea_getline(NUL, indent, "
    : ":append's reader passes none"
literal 4 "getline_equal(fgetline, cookie, getexline)" "getline_equal(fgetline, getexline)"
    : "getline_equal() is asked without one"
literal "fgetline(':', cookie, 0, " "fgetline(':', 0, " : "do_cmdline() gets its next line without one"
literal "do_one_cmd(&cmdline_copy, flags, fgetline, cookie);" "do_one_cmd(&cmdline_copy, flags, fgetline);"
    : "and runs a command without one"
literal "    ea.cookie = cookie;\n" "" : "do_one_cmd() keeps none"

expect none `\bcookie\b` : "cookie is still named"
```
````

**The 2-byte difference is deliberate.** The Go writes
`do_one_cmd(&cmdline_copy, flags,  fgetline );`, with a doubled space and a
space before `)`. The DSL writes the canonical spelling. The edit output differs
by those two bytes, **and the phase output is identical**: the canonical print
erases the difference. Every anchor or replacement *can* be written canonically,
and nothing downstream sees a sloppy one.

### 3.4 Phase 137 (bonus): the logic was layout, and the canonical print made it redundant

`W137Tick` is 25 lines. It rewrites the 18 `((X)->b_ct_di.di_tv.vval)` with
rules for the spaces the macro expansion left around them: none after `(` or
`++`, none before `;` or `)`, re-indent at the start of a line. In the DSL:

````
```
edit tick

literal "    dictitem16_T b_ct_di;\n" "    varnumber_T b_changedtick;\n" : "buf_T's changedtick is a varnumber_T"
body init_changedtick "    buf->b_changedtick = 0;\n"
    : "init_changedtick() sets it to 0, and no type, lock or flags"
sub 18 `\(\((\w+)\)->b_ct_di\.di_tv\.vval\)` "${1}->b_changedtick"
    : "its 18 reads and writes name the field"
```
````

**The edit output is byte-identical** to the Go's, even before the canonical
print: on canonical input, none of `W137Tick`'s spacing cases ever arises.
93 lines become 7. The same one-line `e.Sub(…, 18, …)` would do it in Go, so
this is an argument for deleting layout code, not for a new language.

---

## 4. Recommendation

### Do it partially: harden the embedded language, and do not add an external grammar yet

**The case against a separate language now:**

1. **Coverage.** It expresses 36 phases now and 56 with about eight more verbs,
   but those are **10–22 % of the edit code**. The 56 phases that are 78 % of
   the code compute sets, consult gcc or `internal/cc`, or restructure control
   flow. Their computing is deliberate: phase 54 computes its set *so that* a
   moved upstream is still cut right, and a language that invites listing works
   against that.
2. **It saves little over the embedded form.** Phases 61 and 160 shrink by about
   30 % and 60 %. Most of the saving is boilerplate, or hand-spelled verbs that
   Go can call too. The large wins, 137's 93 → 7 and 160's helpers, come from
   *using the verbs and trusting the canonical print*, which needs no new
   language.
3. **"One path."** The repo's rules are one spelling, one pipeline and one
   printer. Two phase languages means a reader needs both, and a phase that
   grows one computation migrates from one to the other.

**What is worth doing (the embedded half), in order:**

| step | what | effort | proof it changes nothing |
|---|---|---|---|
| 0 | **A differential runner**: `.tmp/dsl/run/main.go` made a subcommand. It runs the old and new program of phase N on one `q(N-1)` and requires identical edit bytes, identical log, and identical `build.Advance` output | 0.5 day | a control, as here: one changed count and one changed replacement must fail |
| 1 | **One verb set for `edit.E`.** One fold verb with options (count, last-match, log with `(n)`) in place of seven spellings. Add `ExpectMentions`, `ExpectNone`, `Rename(word, n)`, `SpliceBetween(a, b, with, n)` and `Query(re, group)`, the idioms the M tier writes by hand. Keep the old names as one-line wrappers until no phase calls them | 1–2 days | no phase changes in this step |
| 2 | **Move the 20 M phases and the Ph-style D phases onto the verbs.** Remove `Text`/`Set` (56 sites), the local `lit`/`cutOnce`/`arm` helpers, and layout code like `W137Tick`. One commit per phase | 3–4 days | the step-0 runner on the phase (edit bytes and log identical, or phase output identical where a spelling was deliberately made canonical, as in 160), then `make whim-build-check` (77 s) |
| 3 | **Line anchors.** A `Line("maketitle();")` sugar compiling to the same `(?m)^[ \t]*…\n` regular expression, used where 85 % of anchors already have that shape | 1 day, plus the phases done piecemeal | the regular expression compiled is textually the same, so it is identical by construction |

That is **about a week** for steps 0–3. It removes most of the M tier's hand
code and the fold drift, and it leaves every phase in Go.

### If the external language is wanted anyway (step 4, optional, about 1 week more)

The prototype shows how to do it without risk:

- `internal/phase/NNN/edit.md`, with the program in a fenced block, Go string
  tokens, `text/scanner`, and **every verb a direct call to `edit.E`**, so a
  transliteration is identical by construction;
- one `//go:embed */edit.md` in `internal/phase`, plus a loop that registers
  `whimNNN` for each, so that `plan.go` does not change;
- **the count made mandatory by the grammar**;
- `whim wed check`, which dry-runs every file's anchors against its `q(N-1)`.

Migrate the 36 D phases one at a time:

1. write `edit.md`;
2. the step-0 runner must report identical edit bytes and log, as for 158 and
   61 here;
3. delete `edit.go` and its `registry.go` line;
4. `make whim-build-check`;
5. commit.

The cost is about 600–700 lines of interpreter for ~30 verbs, and about 1,000
DSL lines replacing about 1,500 Go code lines (2,890 physical). **Wait for a
concrete need:** a moved upstream that forces many of the declarative phases to
be re-anchored at once is when data-only diffs and position-bearing errors pay
off.

### Do not

- adopt Coccinelle, comby or semgrep: external runtimes, no count contract, and
  looser matching than the refusal discipline wants. Take comby's *holes* as a
  Go verb instead;
- use Starlark or `text/template` as the phase language;
- use an AST-mutating language (`doc/AST-EDITING.md`: blocked by cemit's
  position mapping).

---

## Caveats and side findings

- **The tiers are a measurement plus a reading.** The census markers are
  mechanical. The move of 8 phases from C to D and of 20 to M is by reading
  their `Edit` bodies. The "why" grouping of the 56 C phases is from their
  `GOAL.md` titles and markers and was not read line by line. The grouping is
  approximate; the tier sizes are not sensitive to it.
- The argument-constness classifier treats a message string like any other
  argument, which slightly inflates "templated".
- **CLAUDE.md is stale in several places.** It says 164 phases and that "the
  next phase is 164", but the plan has 169 entries (phases 0–168). It says
  "There is no test suite", but `internal/suite` (`go tool whim test`, 63-line
  `cases.md`) exists and is tracked (`31928fb`). These are not fixed here.
