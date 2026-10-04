# The pipeline on a tree: a pilot

2026-10-03, against go-whim `66dd7ee`, on the 64-core machine at a load of
2-7. The question: would the pipeline be simpler and faster if its phases
edited a searchable, editable TREE instead of canonical C text? Today about 70
phase packages and the cutters of `internal/cut` edit the text with
`crefactor/edit`'s verbs and some 382 regular expressions, asserting counts on
it; every phase then ends with the sweep (`crefactor/sweep`, which parses with
`cc.Parse`) and the canonical print (`crefactor/cemit`, which parses again when
the sweep cut anything). The tree tried is C-lisp's (`crefactor/clisp`,
[C-LISP.md](C-LISP.md)), which prints back as cemit's canonical text byte for
byte.

The assessment to test was: the gain is an editable tree -- parse once, print
once -- not s-expressions as such; and the cost is that the tree is untyped
and unresolved, while the sweep and the typed transforms need cc's scopes and
types. Both halves hold, with numbers below. **The recommendation is not to
migrate, and to write a new phase on the tree only where the resolver says
something the text cannot** (*Recommendation*, at the end).

*Afterwards* (2026-10-04): the pipeline did migrate -- not to this untyped
tree but to `doc/GRAPH.md`'s resolved, typed graph, built on C-lisp's forms,
which removed both costs measured here (the conversions and the reparse):
every phase runs on it, one import a run, 128 s in order
(`doc/GRAPH-MIGRATION.md`, *Fin as built*). The "today" below is the
pipeline of `66dd7ee`.

## What was built

**A tree API on C-lisp's `Node`**, in `crefactor/clisp` (it names nothing in
any program):

- `tree.go` -- a `Cursor` is a node in its place: `Node`, `Parent`, `Up`,
  `Index`, `Sibling(k)`, `Enclosing(head)`, `Function`, `IsItem` (a statement
  or block-scope declaration), `Item`, `Live`; and the edits `Delete`,
  `Replace(ns...)` (none deletes, several splice), `InsertBefore`,
  `InsertAfter`, and the one C-level verb the pilot needed, `FoldNever` (an
  `if` that cannot be taken goes, its else standing in its place). A cursor
  finds its node again by identity when an edit elsewhere moved it, so cursors
  collected first can be cut in any order. `Walk(root, pre, post)` visits in
  order and lets either callback edit the node at hand; `Find`, `FindIn` (one
  definition), `Heads`, `Atoms`, `Definition`; the def accessors `DefName`,
  `Prefix`, `HasPrefix`, `DefType`, `DefValue`, `Body`; `Equal`, `Contains`,
  `Clone`; `Mentions` (as the C's `\bname\b` counts: atoms, designators, and
  words in quoted atoms). An `Index` is every atom by its text, as cursors,
  made in one walk: what a cut that starts from a name looks up instead of
  walking the whole tree, as the text verbs find a name's lines with
  `bytes.Index` before a regexp runs. Deletion keeps it good (a cut cursor is
  not `Live`); an insertion or a move does not.
- `pattern.go` -- a pattern is a form in C-lisp: `_` any node, `_*` the rest
  of a list, `?name` a binding (repeated, equal). `Match`, `Matches`,
  `Subst`, `FindPattern`. `(= (-> buf ?f) _)` is every assignment to a member
  of `buf`, the member bound.
- `scope.go` -- `Resolve(forms)`: which declaration each identifier refers to,
  in C's four name spaces -- ordinary (functions, objects, typedef names,
  enumerators), tags, members, labels -- and its scopes: the file, a
  definition's parameters and body as one, every block, a for-statement's
  declaration, a statement expression; each name in scope from its own
  declarator, an inner one shadowing an outer; a prototype and its definition
  one `Decl`, a block-scope `extern` the file's; a label resolved before its
  definition. `Scopes` gives `Refs`, `DeclOf(atom)`, `Uses(decl)`, `File`,
  `Tags`, `Members` and `Opaque` (the forms whose text the tree does not
  structure: a macro's invocation, `verbatim`, an attribute kept as text).
  **It has no types**, and needs none for the pilot: a member use resolves
  only when one member of all the structs has its name (`Ambiguous`
  otherwise).

Tests: `crefactor/clisp/tree_test.go` -- edits on the tree print as the same
edit made to the C, canonically; a walk goes on after a node its callback
deleted or replaced; the patterns and their bindings; the resolver on
shadowing (a parameter, an inner block's local, a for's), a prototype and its
definition, a typedef, an enumerator, a tag, a unique and an ambiguous member,
a goto before its label; `Mentions`.

**The pilot**, `internal/treepilot` (whim's: it knows vim's names), outside
the plan:

- `DropLocal` -- `internal/cut/droplocal.go` as it was then (six regexps, run
  on the lines around the field since `doc/PIPELINE-REFORM.md` §7 step 9;
  a cut on the graph since `doc/GRAPH.md`'s step 5) as forms.
- `P24` -- phase 24, `internal/phase/024`: every call to twelve functions that
  do nothing, the write-only counters `autocmd_blocked`, `autocmd_no_enter`,
  `autocmd_no_leave`, `redrawing_for_callback` and `last_win_id`'s store, the
  two window-id guards folded, and `cmdarg_T.prechar`. Chosen because it is
  the plainest named drop that still has every kind of act a drop has -- calls
  cut by name, statements cut inside named functions, `if`s folded, a member
  cut -- and an assertion the text can only approximate: "every mention of
  the function is a bare call, its prototype or its definition", counted with
  `\bname\b`.
- `RunTree(phase, q(N-1))`: ToLisp once before a run of tree steps, the
  rewrite, Print once after, the phase's other steps as the plan runs them,
  then the same sweep (`sweep.PruneParsed`) and canonical print
  (`cemit.CanonicalParsed`) the pipeline's `finish` runs. `RunText` is the
  phase as the plan runs it, timed the same way.

## The two rewrites, side by side

DropLocal's assignment and get_varp shapes, text:

```go
{0, `(?m)^[ \t]*buf->` + q + ` = [^\n]*;\n`},
{1, `(?m)^[ \t]*case[^\n]*\n[ \t]*return \(char_u \*\)&\(curbuf->` + q + `\);\n`},
{1, `(?m)^[ \t]*case[^\n]*\n[ \t]*return [^\n]*curbuf->` + q +
        `[^\n]*\? \(char_u \*\)&\(curbuf->` + q + `\) : p->var;\n`},
```

tree:

```go
plAssign   = clisp.MustPattern("(= (-> buf ?f) _)")
plVarp     = clisp.MustPattern("(return (cast (ptr char_u) (addr (paren (-> curbuf ?f)))))")
plVarpBoth = clisp.MustPattern("(return (? ?test (cast (ptr char_u) (addr (paren (-> curbuf ?f)))) (-> p var)))")
...
for _, a := range ix.Atoms(bvar) {          // the field's mentions
    it := a.Item()                          // the statement each is in
    ...
    if b, ok := clisp.Match(plVarp, n); ok && isField(b) || isVarpBoth(n, bvar) {
        if prev := it.SiblingCursor(-1); prev != nil && prev.Node().Is("case") {
            add(prev, it)                   // the case label goes with its return
```

Phase 24's call cut, text -- three regexps counted against a fourth:

```go
bare := `(?m)^[ \t]*(?:\(void\))?` + q + `\([^;\n]*\);\n`
allref := len(regexp.MustCompile(`\b`+q+`\b`).FindAll(edit.Blank(e.Text()), -1))
nBare := len(e.Query(bare, 0))
nProto := len(e.Query(`(?m)^static [^\n]*\b`+q+`\(`, 0))
nDefn := len(e.Query(`(?m)^`+q+`\(`, 0))
e.Expect(allref == nBare+nProto+nDefn, ...)
```

tree -- the uses of the file-scope function, each the callee of a call that
is a statement of its own, and its declarations prototypes or the definition:

```go
d := sc.File[fn]
for _, a := range ix.Atoms(fn) {
    if sc.DeclOf(a.Node()) != d { continue }   // a local or member of the name is not it
    it := a.Item()
    b, ok := clisp.Match(bare, it.Node())      // (call ?fn _*), or (cast void (call ?fn _*))
    ...
    if ok && b["fn"] == a.Node() { calls = append(calls, it) }
}
if uses != len(calls) || len(d.Nodes) != proto+defn { refuse }
```

and the two folds, `e.InFunction("getcmdline_int", ... e.FoldNever(edit.Head("if
(is_state.winid != curwin->w_id)"), 2, ...))`, are
`FindIn(root, "getcmdline_int", (if (!= (. is_state winid) (-> curwin w_id)) _*))`,
counted 2, each `c.FoldNever()`.

## Proof: byte for byte, with a control

Every phase with a tree step, on the tree, against the snapshots of the
current input (`/root/go-whim/.cache/boundaries`, its manifest the digest of
`src/slim-vim.c`): **14 of 14 phases give qN byte for byte from q(N-1)** --
the 13 phases with a `droplocal` step (4, 5, 7, 8, 12, 13, 14, 16, 17, 18,
19, 20, 34: 64 fields, the other steps of 4, 5, 18-20 and 34 run as text in
between) and phase 24 (`TestTreeBytes`). Step by step, each of the 64 fields
on the text the plan's steps before it leave: **the same count of plumbing
sites** as `cut.DropLocal`, and the same C canonically
(`TestDropLocalSteps`). Phase 24's report on the tree is the text version's
**line for line**, every count the same (`TestP24Report`). **The control**:
DropLocal leaving the case label of a get_varp case it cuts (valid C, a label
falling through to the next case), and P24 leaving the last call to each
empty function (the same nothing done) -- both caught: phase 17 2,236,904
bytes against q017's 2,236,733, phase 24 2,163,854 against 2,162,569
(`TestTreeControl`).

```sh
TMPDIR=$PWD/.tmp TREEPILOT_SNAPS=.cache/boundaries go test ./internal/treepilot/   # the proofs
TREEPILOT_RUNS=5 ...                     -run Measure -v                           # the tables below
TREEPILOT_PROFILE=D ...                                                            # a CPU profile a run
```

The tree version does assert the same counts, but says some of them better:
the text's "mentions" of a function count a local or a member of the same
name; the resolver's do not.

## Measured

### One phase, A/B

Each phase as the plan runs it (text) and with its tree steps on the tree,
alternately, 5 runs each, one at a time; medians, ms (`TestMeasurePhases`).

| phase 8 (2 fields) | wall | CPU | | phase 17 (3 fields) | wall | CPU |
| --- | ---: | ---: | --- | --- | ---: | ---: |
| text: the step | 11 | 23 | | text: the step | 17 | 45 |
| text: sweep | 1,114 | 2,172 | | text: sweep | 1,061 | 2,167 |
| text: canonical print | 1,354 | 2,796 | | text: canonical print | 1,311 | 2,631 |
| **text: total** | **2,518** | **4,992** | | **text: total** | **2,390** | **4,875** |
| tree: ToLisp | 1,387 | 2,818 | | tree: ToLisp | 1,361 | 2,717 |
| tree: rewrite | 73 | 206 | | tree: rewrite | 77 | 214 |
| tree: Print | 69 | 100 | | tree: Print | 56 | 82 |
| tree: sweep | 1,125 | 2,270 | | tree: sweep | 1,088 | 2,079 |
| tree: canonical print | 1,385 | 2,912 | | tree: canonical print | 1,291 | 2,557 |
| **tree: total** | **4,038** | **8,386** | | **tree: total** | **3,862** | **7,947** |

| phase 18 (whim18kp, 1 field) | wall | CPU | | phase 24 | wall | CPU |
| --- | ---: | ---: | --- | --- | ---: | ---: |
| text: the steps | 9 | 21 | | text: the edit | 860 | 1,110 |
| text: sweep | 1,081 | 2,418 | | text: sweep | 1,105 | 2,344 |
| text: canonical print | 1,301 | 2,674 | | text: canonical print | 1,279 | 2,679 |
| **text: total** | **2,421** | **5,119** | | **text: total** | **3,237** | **6,156** |
| tree: other steps (text) | 2 | 3 | | tree: ToLisp | 1,310 | 2,689 |
| tree: ToLisp | 1,357 | 2,785 | | tree: rewrite | 172 | 519 |
| tree: rewrite | 74 | 230 | | tree: Print | 51 | 149 |
| tree: Print | 55 | 73 | | tree: sweep | 1,110 | 2,396 |
| tree: sweep | 1,064 | 2,281 | | tree: canonical print | 1,309 | 2,754 |
| tree: canonical print | 1,339 | 2,657 | | | | |
| **tree: total** | **3,859** | **8,120** | | **tree: total** | **3,951** | **8,387** |

- **The rewrite itself.** Phase 24 is 860 -> 172 ms wall (1,110 -> 519 CPU),
  five times faster on the tree: its text version scans the whole text for
  each of twelve names several times; the tree looks each up in the index
  and asks the resolver. DropLocal is slower on the tree, 11-17 -> 73-77 ms,
  nearly all of it building the index (one walk of 455,000 nodes, 60-70 ms);
  the text version is a `bytes.Index` and a regexp on a few lines, which step
  9 made it. A first tree version that walked the whole tree per field, and
  allocated the pattern's bindings at every node, took 295-460 ms: on a tree
  as on text, a cut that starts from a name must look the name up.
- **But on today's pipeline a tree phase is slower**, 2.4-3.2 s -> 3.9-4.0 s
  wall and 4.9-6.2 -> 7.9-8.4 s CPU: converting in (ToLisp, a cc parse and
  the conversion, 1.3 s wall) costs more than any edit it replaces; Print
  back is 50-70 ms. The sweep and the print cost what they cost either way.

### Parse once

What each way of having the tree costs on a boundary, 3 runs, medians, ms
(`TestMeasureParse`; q023 is phase 24's input):

| on q023: 2,165,859 bytes, 81,860 lines | wall | CPU |
| --- | ---: | ---: |
| `cc.Parse` (`cemit.Parse`) | 600 | 1,682 |
| ToLisp to forms (`Forms`: the parse and the conversion) | 1,223 | 2,877 |
| `Read` of the `.lc` text | 46 | 66 |
| `Print` (forms to C) | 45 | 45 |
| `Clone` of the forms | 32 | 32 |
| a `Walk` of every node | 32 | 31 |
| `Resolve` | 75 | 75 |
| the sweep (`PruneParsed`, a text it does not cut: one round) | 918 | 2,119 |
| the canonical print, handed that parse | 604 | 982 |

q016 and q103 (the product) are within 10% of these. The tree is 454,587
nodes, 302,537 of them atoms; 99,901 references, 3,843 of them members of a
name more than one struct has; 84 opaque forms.

**Reading C-lisp is 13 times cheaper than parsing the C** (46 against 600 ms
wall, 66 against 1,682 of CPU), and printing it 13 times cheaper than the
canonical print. That is the s-expressions' contribution -- a trivial
grammar, no preprocessor, typedef names decided -- and it is real; but a
pipeline that handed the tree from phase to phase would not read text at all.

**What a phase costs today, profiled** (phase 17 as the plan runs it, one run,
4.64 s of CPU sampled): the collector's mark workers 1.99 s (43%); `cc.Parse`
1.12 s (24%) -- **twice**, the sweep's round and the canonical print's, since
the sweep cut something and so hands no parse on; `cemit.File`, the print,
0.44 s; the sweep's own analysis about 0.3 s (`collect` 0.18); the step 0.05
s. Phase 24 the same with its edit's 0.8 s.

**Handed from phase to phase -- an estimate, not a measurement**: the two
parses and the print go, and with them most of the collector's work, which is
largely theirs; what is left is the rewrite (0.2-0.5 s of CPU here) and a
sweep on the tree, which was not built. Its closure is by name, not by type
(`crefactor/sweep/prune.go`: keys in three name spaces, a member by name), so
the resolver is enough for it; its analysis is about 0.3 s of CPU on the text
and Resolve 75 ms on the tree, so 0.2-0.4 s is a fair guess. A phase would go
from about 2.4 s wall and 4.9 s of CPU to about 0.3-0.5 s and 0.5-1 s. Over
104 phases in order -- 451 s wall, 563 s of CPU -- the finish alone is about
2.4 s wall a phase, 250 s; a tree pipeline might save some 200 s of the 451
in order. The parallel check, 105 s, is bound by the front's phases 1-3
(37, 25 and 33 s), whose closures are `FallOut`, which needs cc's scopes and
types (below); it would gain less.

### Lines of code

Non-blank, non-comment lines:

| | text | tree |
| --- | ---: | ---: |
| DropLocal (and its step's wrapper) | 87 + 16 | 114 (the `Tree` and its index, 12; the control, 4) |
| phase 24 | 69 | 139 |
| the helpers each uses | `crefactor/edit`'s driver, blocks, fold, norm, shared, blank, definition, counted, match and lines: about 1,700 of its 2,241 | `tree.go`, `pattern.go`, `scope.go`: 1,136 |
| to have the tree at all, while the pipeline's currency is text | -- | the converter both ways (`tolisp.go`, `toc.go`, `expr.go`, `sexp.go`): 2,195 |

The tree versions are not shorter. A pattern says a statement more plainly
than its regexp (no backslashes, no layout, no `[^\n]*`), but the text side
has a mature verb set -- `InFunction`, `Lines`, `FoldNever`, a count and a
report in one call -- and the tree has none; phase 24 on the tree spells its
counts and reports by hand. A tree verb set would bring them level, not
below.

## What the tree could not do without types or cc's scopes

- **Members.** Which struct `p->next` names is the type of `p`. The resolver
  resolves a member only when its name is one struct's (3,843 of q023's
  member uses are not). DropLocal and the sweep are content with names --
  the sweep has always kept a member by name -- but anything that must know
  the struct cannot.
- **The typed transforms.** `crefactor/xform`'s `MemberOut`, `LocalOut`,
  `StructScalar`, `StateParam`, `BoolRet` and `plainc` ask cc.Translate for
  types -- a declarator's `Type().Kind()`, a selector's struct, a function's
  parameters, whether a type is a scalar; `FallOut` asks cc's resolution. On
  a tree pipeline the phases that run them (the seed's 0a and 0c, the front's
  three closures, 47, 50, 60, 62, 74, 86a, 87a, 102, 103) would print and
  parse anyway, or the tree would need a type checker, which is cc's work
  again.
- **What the forms keep as text.** A macro's invocation (`va_arg(ap, int)`),
  `verbatim`, an attribute kept as text: 84 forms on q023, listed as
  `Opaque`. The resolver cannot see a name in them; a cut that must know
  nothing hides a mention has to look into them as text (`Mentions` does).
- **The headers.** The forms are the file's; what its `#include`s declare is
  not in them, so libc's names and the host's types are unresolved -- 160
  names on q016, 142 on q023, 63 on the product, every one a header's but
  one each on q016 and q023.

**Those two are a finding.** The resolver left `semsg` unresolved on q016 and
`setfname` on q023: names the file uses and does not declare.
`gcc -fsyntax-only` on every boundary agrees: **q004-q029 do not compile under
gcc 15** -- `implicit declaration of function 'semsg'` in q004-q022 (part 4d's
literal body for `buflist_findpat()` calls it after the front took it) and
`'setfname'` in q022-q029 (phase 22 writes calls to `setfname()`, which phase
20's sweep had taken). The product is not affected -- later phases take both
calls -- and `whim-build-check` proves bytes, not that a boundary compiles. A
text edit can write a name that no longer exists and nothing says so; a
resolver, or a `-fsyntax-only` of each boundary, would. Fixed since, which
moved the snapshots of 4-29: gcc found a third error the resolver could not
(a walk over `b_next`, a member part 4d deleted, in `autowrite_all()`,
q004-q019); part 4d and phase 22 write C that compiles (their `GOAL.md`s),
and `whim build --check` compiles and links every boundary.

## Recommendation

**Stop short of migrating; write a new phase on the tree only where it says
something the text verbs cannot** -- a resolution, a structural count -- and
take the resolver's check of the boundaries (or gcc's) as the one thing worth
having now. The numbers that decide it:

- **On today's pipeline a tree phase costs +1.3-1.6 s wall and +3 s of CPU**
  (ToLisp), more than the five-fold faster rewrite of phase 24 saves (0.7 s),
  and DropLocal, already windowed, is faster as text (11-17 against 73-77
  ms). So converting phases one by one makes the build slower until the last
  one is converted.
- **The win is all-or-nothing, and estimated at about 200 of 451 s in
  order**, nothing like it for the parallel check (bound by the front's typed
  closures). It needs the tree handed from phase to phase, a sweep on the
  tree (1,420 lines to rewrite), and the 79 phase programs and 57 cutters --
  about 15,300 and 6,900 lines -- rewritten and each proved byte for byte as
  these two were; and about 15 phases would still print and parse for cc's
  types.
- **The pipeline's goal is met.** A new phase is now rare and is for the Go
  editor; a build is a moved upstream's cost, not a daily one. 200 s of an
  in-order build does not pay for rewriting 22,000 lines that are proved.

What the pilot does show is that the tree is a sound substrate when one is
wanted: the 14 phases and 64 fields came out byte for byte at the first
attempt, a pattern says a statement more plainly than its regexp, and the
resolver answers exactly the questions the text verbs approximate with
`\bname\b`.

`doc/GRAPH.md` takes the lesson the other way: not a tree beside the text,
but one representation, resolved and typed when it is built and never
reparsed, of which C, C-lisp and every other view are printings.
