# GRAPH-MIGRATION.md -- every phase, and what moving it to the graph takes

2026-10-03. Stage B of `doc/GRAPH.md`'s step 5. Stage A built the hybrid
driver, the graph snapshots, the collection through the editor, and moved
the first 14 phases' cuts to the graph (GRAPH.md, *Step 5 as built*). This
file lists every phase 0-103 and every part, classifies each, and proposes
the batches stage B hands out. The classification comes from reading every
program (`internal/phase/NNN/`, `internal/cut`, `crefactor/xform`) against
what `crefactor/graph` can do at `step5a`. The fenced table is data: one row
per unit, `|`-separated, its columns named in the first row.

## Converting a phase: the recipe

1. Write the cut on the graph: a `phase.GraphFunc` registered with
   `phase.RegisterGraph("whimN", ...)` in the phase's `edit.go` (it REPLACES
   the text program, which history keeps), or a `steps.GraphStep` in
   `internal/steps`' `graphOps` for a cutter (the cutter's text version is
   then deleted, unless another step still calls it). Write it on B0's
   verbs (`graph.NewVerbs(tag, e, w)`: the text program's acts, one for
   one, *B0 as built* below), and below them on the editor (`graph.Editor`:
   `Delete`, `Replace`, `InsertBefore/After`, `Retarget`, `Unwrap`,
   `ReplaceRun`, `Build`, `FallOut`, `Collect`) and on patterns
   (`graph.Match`/`Find`, clisp's pattern forms). Report what the text
   version reported, in its order; the bytes are what is checked, the
   report is the reader's.
2. Mark the step `Graph: true` in `internal/build/plan.go`.
3. `GRAPH_SNAPS=.cache/boundaries go test ./internal/graphcheck/ -run
   PhasesOnGraph` holds every phase with a graph step to its snapshot in
   seconds, imported and handed the graph.
4. `rm -rf .cache/boundaries && make whim-build-check` (in order, which
   writes the graph snapshots), then `make whim-build-check` again (the
   parallel check: a phase that begins on the graph reads `qNNN.g`): both
   `whim-vim.c byte for byte`, every boundary compiling. That is the proof
   the graph program equals the text program it replaced.
5. Measure (`GRAPH_MEASURE=5 ... -run MeasureGraphPhases`) and say what the
   phase costs now in its `GOAL.md`.

**What a conversion costs until its neighbours move too.** A phase whose
graph steps begin after a text step pays an import, 1.7-2.1 s on today's
texts (2.0-2.7 times cc's parse and check), which is what the sweep and the
canonical print it no longer runs cost: such a phase is about as fast as
before (GRAPH.md, *Step 5 as built*). A phase that BEGINS on the graph
after one that ended on it pays nothing: 0.21-0.37 s against 2.4-3.7 s.
And a phase that ends on text after graph steps pays the import AND the
sweep (4, 5 and 34 are 1.3-5 s slower now). So convert in contiguous runs,
and convert a phase's last text step before its first.

**Byte risks every converter meets** (found by the catalogue's readers, not
yet by a build):

- The text's unwrap (`FoldAlways`, keep-then, keep-else) splices a branch's
  items into the block around it even when the branch declares something;
  the closure's if-fold keeps a declaring block whole. B0 took it: the
  verbs splice as the text does, refusing where a name would clash
  (`Editor.Unwrap`, `Clash`), and the closure splices too when the cut opts
  in (`FallOutOptions.SpliceDeclaring`), keeping the block whole on a clash.
- `FALSE` and `TRUE` are references (enumerators) in the graph, not
  integers: `graph.Literal(0)` prints `0`. A template says `FALSE` and BUILD
  makes the use, its edge to the enumerator. `nullptr` is a plain token, an
  atom with no edge: `graph.NewAtom("nullptr")`.
- The editor refuses deleting an enumerator whose successor's value is
  implicit: delete last-first (phase 26), or opt into renumbering (`RENUM`).
- A member deleted while a use survives leaves a dangling record the
  closure must discharge or refuse; the text never checked. Delete the uses
  first, or let a rule take them.

## Capabilities

What exists at `step5a` (**G0**): `Import`; `Read`/`Lisp`; the C view; the
editor's `Delete` (a top-level form, a block or body item, a member, an
enumerator, an if's else), `Replace` (an expression by one expression node,
a body by a block, an item by items, any one-node place -- an initialiser
element, a callee, an operand -- by one node, which may be a node moved out
of what it replaces), `InsertBefore/After`, `Retarget` (to a target of the
same spelling); `FallOut`'s rules (call, store, through, value, constant
folds, `?:`, `if` of a constant, `while (0)`, emptied blocks); `Collect`
and `Editor.Collect`; queries (`Defn`, `FileDecls`, `Uses`, `Item`,
`Sibling`, `Parent`, `Function`, `Body`, `Members`, `DeclName`); clisp
patterns on nodes. A `sweep` step is a collection whenever the program is
held as a graph there.

The new ones, each a batch's job (B0 and B2 below):

| name | what it is | units that need it (about) |
| --- | --- | ---: |
| VERBS | **done (B0)**: crefactor/edit's verb set on the graph, `graph.Verbs`: `InFunction`, `InTable`, `In`; `Cut`, `Rewrite`/`RewriteAt`/`RewriteFunc`, `FoldNever`/`FoldAlways`/`DropIf`/`FoldAlwaysElse`/`KeepThen` (structural, not `Replace(cond, 0/1)`: see *B0 as built*), `DropOperand`, `DropCase` (a label is an item of its own in C-lisp, so a case run is a run of items), `Splice`, `Body`, `DeleteDefinition`, `DropBareBlock`, `FoldWalk`/`FoldWalks`; every act counted and refused as the text verbs refuse | nearly all |
| BUILD | **done (B0)**: `Editor.Build(at, template, holes)`, C-lisp forms made nodes at a place -- names resolved as the importer resolves them, members by type, typed edges where they follow, holes for moved nodes -- and `RefTo`, `Call`, `Return`, `Break`, `Void`, `Literal` | ~60 |
| TEXTQ | **done (B0)**: the text's own counts on the scope's C view (`Mentions`, `TextCount(Is)`, `TextQuery`: the text's numbers exactly), and the graph's questions (`UsesOf`, `UsesOutside`, `Says`, `Strings`, `Rows`/`Row` -- a row by its first element, string or designator, as a pattern -- `Editor.Decls`, `Editor.Before`, `Find`/`Count`/`CountIs`/`One`/`Query` by pattern, `ConstOf`); the text-only ones (line deltas, blank-line runs, "directives on the first N lines") are dropped, not ported | ~70 |
| FRAG | a C fragment -- an expression, statements, a declaration, a function, a body, an `editlit.go` or fenced `.md` literal -- made graph nodes in the context of its place: its names resolved to the graph's declarations (and to the externs: libc, builtins like `__builtin_setjmp`, macro invocations like `FD_SET`), typed, given fresh ids, its holes filled with existing nodes. The likely way: print the context's C view with the fragment in place, import that, and keep the new nodes with their edges retargeted to the graph's ids | ~70 |
| CLONE | a subtree copied with fresh ids and the same refers edges, for a node used twice (0b's format argument, 89's tails, 42's `MIN(x, y)` operands) | 4 |
| MACROX | an opaque `(macro "...")` invocation replaced by its expansion as nodes, its arguments taken from its text (42's `MIN`/`MAX`) | 1 |
| RENAME | a declaration and its uses respelled (a member, a function, a typedef), a use retargeted to a target of another spelling, a string literal respelled (its array type changes) | ~25 |
| INITROW | an initialiser element deleted (a table row: `cmdnames[]`, `options[]`, `key_names_table`, `nv_cmds`), found by its designator or its string; inserted; with positional arrays' indexes said | ~20 |
| RENUM | an enumerator deleted with the implicit values after it moved, opted into (`CMD_index` must follow the rows the front cut) | 5 |
| PARAM | a parameter deleted with the argument at every call, in function-pointer types too; one argument of a variadic call deleted (with its format string) | ~20 |
| RETYPE | a declaration's type changed (`int` to `bool`, `void *` to `T *`, a union member to its one member), the typed edges above it cleared and listed (`Untyped`) until step 6's checker re-derives them | ~20 |
| MOVE | a node or a run of items moved elsewhere keeping their ids (an outlined block, a body inlined, a range wrapped in a loop, a definition moved below the boundary) | ~15 |
| INCLUDE | **done (B2e)**: `#include` forms added, deleted or moved under the extern rule (each name the file takes from the headers provided by an include above its first use; no header macro over the file's own names below it), and the first one as the core/host boundary: `FirstInclude`, `Core`/`Host`, `InCore`/`InHost`, `MoveToHost`/`MoveToCore` (*B2e as built*) | ~15 |
| FOLDX | the fall-out rules the closure lacks, each a rule in `fallout.go` (generic) or a cut's own `Rule`: a walk folded to `v = cur; body` (refusing a break that binds to the loop); `a && K`, `a \|\| K`, `!K`, `K == 0`; a function whose body is `return K;` made K at its calls; a parameter every call passes alike made its value; statements after a jump or `return` go; a label no goto reaches goes; empty blocks taken everywhere (and an empty else, a trailing empty else-if), not only those an edit emptied; an if whose only effect is a store to a deleted location; the value a cut gives a deleted object (not its initialiser); `if (!f()) {}` becomes `(void)f();` (a declaring block spliced where no name clashes: B0's, `SpliceDeclaring`) | ~30 |

## B0 as built (2026-10-03)

`crefactor/graph`'s `verbs.go` (the verbs), `build.go` (BUILD), `textq.go`
(TEXTQ) and `unwrap.go` (the declaring-block splice, a run of items
replaced), and `FallOutOptions.SpliceDeclaring` in `fallout.go`; generic,
naming nothing in vim. 624, 622, 197 and 120 lines of Go, comments and blank
lines aside. A converted phase reads like its text program:

```go
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noswapfile", e, w)
	noswap := "(& (. cmdmod cmod_flags) CMOD_NOSWAPFILE)"
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.DropCase("(case 'n')", 1, "the :noswapfile modifier")
	})
	v.InFunction("ml_open", func(v *graph.Verbs) { v.DropIf(noswap, 1, "ml_open asking for it") })
	v.InFunction("buf_copy_options", func(v *graph.Verbs) { v.FoldNever(noswap, 1, "buf_copy_options asking for it") })
	n := v.Mentions("CMOD_NOSWAPFILE")
	v.Expect(n == 1, "CMOD_NOSWAPFILE outside its enumerator -- %d mentions, expected 1", n)
	return v.Done()
}
```

### The API later batches use

**The driver**, `graph.Verbs` (`NewVerbs(tag, e, w)`): `Say`/`Sayf` (the
text's report column, `  %-12s %s`), `Die`/`Refuse`, `Expect`, `Done`,
`Failed`; the first refusal stops the rest and nothing after it reports, as
`edit.E`. **Scopes**: `InFunction(name, acts)`, `InTable(name, acts)` (the
initialised definition of a file-scope object), `In(node, acts)`.
**Acts**, each counted (`pat, ..., n, what`) and reported as it succeeds:

| verb | does | the text's |
| --- | --- | --- |
| `Cut(pat, n, what)` | deletes the matches: items, members, enumerators, top-level forms, an else | `Cut`, `Lines`, `DropBlocks` |
| `Rewrite(pat, tmpl, n, what)` | each match replaced by the template, its holes the match's bindings | `Literal`, `Sub`, `ReplaceBlock`, `DropWalk` |
| `RewriteAt(pat, at, tmpl, n, what)` | in each match, the node bound to `?at` replaced, the rest kept | `Sub` with `${1}...${2}` |
| `RewriteFunc(pat, n, f, what)` | each match replaced by what f builds | a computed `Sub` |
| `FoldNever`, `FoldAlways`, `DropIf`, `FoldAlwaysElse` | the text's folds: the pattern is the `if`, or, not headed `if`, its condition | the same names |
| `KeepThen(pat, n, what)` | `if (T) A else ...` is A, whatever chain follows | the cutters' `keepThen`/`keepThenChain` |
| `DropOperand(pat, n, what)` | an operand of `&&`, `\|\|`, `\|` dropped | a `Literal` on a condition |
| `DropCase(pat, n, what)` | a case label; with its run of statements when it heads one alone; refused where the case before falls into it | a `Cut` of `Line("case 'n':", ...)` |
| `Splice(from, through, tmpl, what)` | a run of items, each end matching one item, replaced | `Splice` |
| `Body(fn, tmpl, what)` | a function's whole body | `Body` |
| `DeleteDefinition(fn, what)` | refused while something outside it refers to it | `DeleteDefinition` |
| `DropBareBlock(pat, what)` | the block around the one statement, if it is an item and holds only that and bare declarations | `DropBareBlock` |
| `FoldWalk(pat, set, n, what)`, `FoldWalks(pat, ok, set, what)` | a loop folded to `set` and its body, a break or continue that would rebind refused | `FoldWalk`, `FoldWalks` |
| `FallOut(opt)` | the closure over what the acts left dangling, its refusal the acts' | -- |

**Assertions and queries**: `Find`, `Count`, `CountIs`, `One`, `Query(pat,
name)` (the bound nodes), `ConstOf(fn, form)`; `Text` (the scope's C view),
`Mentions`, `TextCount`, `TextCountIs`, `TextQuery` (the text's numbers, by
crefactor/edit's own code on that view); `UsesOf`, `UsesOutside`, `Says`,
`Strings`, `Rows`, `Row`.

**On the editor**: `Build(at, tmpl, holes)` and `BuildIn(p, tmpl, holes)`
(BUILD), `Resolve(at, name)`, `RefTo(decl)`, `Call(f, args...)`, and the
free `Return`, `Break`, `Void`, `Literal`; `Unwrap(old, block)` and
`Clash(p, i, items)`; `ReplaceRun(first, last, with...)`; `Decls(name)`,
`Before(a, b)`.

### Refinements of the catalogue's vocabulary

- **The folds are structural, not `Replace(cond, 0/1)` and the closure.**
  The closure's fold is right for what a cut makes constant, but a text
  fold names one if and says what it keeps; the verb does exactly that, so
  that its bytes are the text's. The closure is still there for a cut that
  writes a constant (`Replace(cond, 0)`, then `FallOut`, as phase 24).
- **A pattern is a form, and an else-if arm is an if.** The text's
  `Head("if (x)")` did not match `else if (x)`, since the spellings differ.
  On the graph both are `(if x ...)`, the arm told apart by where it is
  (an if's else). So a count may differ from the text's, and the converter
  recounts. `FoldAlways` refuses an arm, as the text refused an `else if`
  head.
- **Lines, DropBlocks, ReplaceBlock and DropWalk collapse into `Cut` and
  `Rewrite`**: without lines, a statement and a block are both a node.
  `BodyOf` and `InnerBody` are `Editor.Defn` and `graph.Body`.
- **A `\bname\b` count is not `Uses`.** The text counts declarations,
  string contents and macro texts too. So TEXTQ has both: `Mentions` on the
  scope's C view gives the text's number exactly (a function's view prints
  in well under a millisecond, the file's in 50-90 ms), and `UsesOf` asks
  the edges, which is usually what the count stood for.
- **The declaring-block splice is B0's**, not B2d's. The verbs splice as
  the text does, so that their bytes are the text's, and refuse where a
  name clashes: a moved declaration declared again where it moves, one that
  is a parameter there, or one that would hide another declaration a later
  use names (by edge: a use whose target is declared inside what follows
  is not a clash). The closure splices only when the cut opts in
  (`SpliceDeclaring`), and keeps the block whole on a clash. A block whose
  items define a type or that carries attributes is refused. The text's
  splice was unconditional, and a clash there made a different or a
  broken program, which no gate looked for.
- **BUILD is templates.** A template is C-lisp forms read at a place, and
  is how the text's replacement strings carry over: `"(= bp curbuf)"`,
  `"(return FALSE)"`, `"nv_error"`. Its names resolve as the importer
  resolves them. A local or parameter is a use if it is declared before the
  place; otherwise the name is the file's FIRST declaration above it (cc's
  check takes the first declarator, so a use refers to a function's
  prototype, not its definition), or an enumerator, or an external. A
  member resolves by the type selected from, a tag to its definition, a
  goto to the function's label. The typed edges follow where they are
  plain: a call's result, a comparison or logical int, an assignment's
  left side, a selection's member, `&x`, `*p`, `p[i]`, a cast to a type the
  graph holds. Any other new expression is listed in `Untyped`, as a
  replacement's are. It refuses what is FRAG's: a name declared nowhere
  visible, a macro, a compound literal or initialiser, a function type,
  and a hole used twice (CLONE's).
- **An initialiser element is a one-node place, verified.**
  `internal/graphcheck`'s `TestInitElementPlace` points onebuffer's CTRL-^
  row (on q003) and 15a's `!` row (on q014) at `nv_error` by `RewriteAt`,
  on the graph read back from its Lisp. In each, the row keeps its id, one
  atom is superseded and one given, nothing is left untyped, and the C
  view is the text program's substitution byte for byte. So the rows
  pointed elsewhere (15a, onebuffer, 4f's `nullptr`, nomouse's 22) need no
  INITROW. INITROW stays B2b's for deleting and inserting rows and saying
  positional indexes. A row FOUND by its string or its designator is
  `Row(pat)`.
- **A whole phase on the verbs, outside the plan**:
  `TestVerbsOnPhase15` writes phase 15's two programs (whim15a, whim15) on
  the verbs, runs them on q014's graph read back, and collects: the result
  is q015.c byte for byte, with the text's report line for line. The
  control (the `!` row pointed at another handler) moves it by a byte.
  B1a's conversion of phase 15 is that test's body moved into the phase.

### What the tests prove

`crefactor/graph`'s `verbs_test.go` and `build_test.go`, every case on a
graph read back from its Lisp, no cc node behind it:

- **The text verbs' own cases** (`crefactor/edit`'s `acts_test.go`): each
  shape of if-chain folded (`TestFolds`), the refusals (`TestFoldRefusals`),
  and the driver (`TestEDriver`): order, report, the first refusal stopping
  the rest. Each gives the C the text verb gives, printed canonically, byte
  for byte, with the same report.
- **Every other verb against its text counterpart**, the same way:
  `FoldAlwaysElse` and `KeepThen`, `FoldWalk` and `FoldWalks` (a declaring
  body spliced), `DropBareBlock`, `Splice`, `Body`, `DropCase` and
  `DropOperand` (against the cutters' line cuts and literals), `RewriteAt`
  on a table's row, `ConstOf`. And their refusals: a break that would
  rebind, a declaration that would clash, a case fallen into, a block that
  is a body, a run backwards, a definition still referred to.
- **BUILD**: each kind of name resolved where the place is (a local over
  the file's object of the same name, and the file's before the local; a
  parameter, an enumerator, the prototype, a typedef, members through a
  typedef and a pointer, a label), the typed edges, `Untyped`, fresh ids on
  insertion, and the refusals.
- **The splice**: the clash cases, and the closure with and without
  `SpliceDeclaring`. Where it splices, the result is the text's
  `FoldAlways`.
- **TEXTQ**: `Mentions` is `edit.MentionCount` on the canonical text,
  number for number; uses, strings, rows and order are checked as well.

## The classes

- **(a)** expressible with G0 and the B0 library (VERBS, BUILD, TEXTQ).
- **(b)** needs a capability of B2: named in the `needs` column.
- **(c)** a typed transform: it calls `cc.Translate` -- step 6, on typed
  edges. There are only five: the front's `FallOutOf` wrapper
  (`xform/fallout.go`, phases 1-3), `memberout` (94), `stateparam` (95),
  `localout` and `structscalar` (100), and `plainc`'s three (101). Several
  that live in `crefactor/xform` are NOT typed -- `NullptrUsize`, `Attrs`
  (0a, 0c), `Own` (47), `Unions` (50), `DropCalls` (60), `EmptyBlocks` (62),
  `NeverNull` (74), `DeadStmt` (86a), `BoolRet` (87a, 102, 103) and the four
  goto steps (89-92): they are text or `cc.Parse` only, so they are (a) or
  (b).
- **(d)** stays text: the seed (phase 0: it runs on the input before any
  graph, its work is spelling, and its proof is an identical binary; the
  first import is of q000 or later), and phase 88's `includes` (judged by
  gcc compiling each trial). Phase 43 as built asks gcc three questions the
  graph answers by edges (names the headers still supply, definitions whose
  every use is below the cut, the core's undefined interface): (d) as it
  stands, (b) rewritten. Phase 42's `MIN`/`MAX` expansion reads the
  preprocessor's own text (`@minmax`): MACROX, or a text step left at the
  phase's end.

## The catalogue

`phase` is the plan's; `where` is relative to `internal/` unless it starts
with `crefactor/`; `LOC` is non-blank non-comment lines of Go (an
`editlit.go` or `.md` literal's bytes in `lit`); `class` and `needs` as
above; `batch` is the proposal below. A phase's own row (`unit` = `phase
N`) has its hardest class and says what is already on the graph.

```
unit | phase | where | LOC | lit | class | needs | batch | what
phase 0 | 0 | build/plan.go s00-seed | 343 | 2591 | d | - | - | the seed: canonical print of the input, then 0a-0c; stays text
0a | 0 | phase/000/a + crefactor/xform/nullptr.go | 7+229 | - | d | (RENAME FRAG INCLUDE) | - | NULL->nullptr, size_t->usize, (void *) casts dropped; not typed
0b | 0 | phase/000/b | 330 | 2591 | d | (FRAG CLONE) | - | 7 printf-style wrappers expanded at 297 calls
0c | 0 | phase/000/c + crefactor/xform/attrs.go | 6+309 | - | d | (RETYPE RENAME) | - | 113 unused attributes go, 20 fallthrough respelled; not typed
phase 1 | 1 | front + noruntime | - | - | c | FallOutOf; FRAG INITROW MOVE RENAME RETYPE | B4 | the front's D1-D5, one closure over them
FallOutOf | 1-3 | crefactor/xform/fallout.go; steps.go frontHold | 1686 | - | c | step 6 (or an UNWRITTEN seed over write edges + FOLDX) | B4 | the closure after each front phase's cuts, typed twice a round
argvfront | 1 | cut/argvfront.go | 99 | - | b | FRAG INITROW | B4 | command_line_scan's new body, main_errors rows
exfront | 1 | steps.go exFront + cut/retire.go + phase/001/delta.md | 22+37 | - | b | INITROW TEXTQ | B4 | 489 cmdnames rows' handler to ex_ni, found by name string
extable | 1 | cut/extable.go | 270 | - | b | INITROW MOVE RENAME RETYPE FRAG TEXTQ | B4 | shortest-abbreviation lookup, prefix index gone; the hardest front unit; after exfront
filefront | 1 | cut/extable.go | 41 | - | b | INITROW MOVE | B4 | 13 file-command rows; enumerators moved after CMD_SIZE
quitfront | 1 | cut/extable.go | 12 | - | a | - | B4 | one FoldNever
readfront | 1 | cut/extable.go | 15 | - | a | - | B4 | two FoldNever
onecmdfront | 1 | cut/onecmdfront.go | 45 | - | b | FRAG | B4 | condition shrinks (a), two new small bodies
optfront | 1 | cut/optfront.go, droprow.go, optfront.md | 112 | - | b | INITROW TEXTQ | B4 | 375 options[] rows and their names in string lists
noswap | 1 | cut/noswap.go | 69 | - | b | BUILD | B4 | 4 bodies stubbed, an else-if arm
norecover | 1 | cut/norecover.go | 123 | - | a | - | B4 | recovery cut; add_time's body flattened (the declaring splice: B0's Unwrap)
nomemfile | 1 | cut/nomemfile.go | 241 | - | b | PARAM FRAG | B4 | mf_open's parameters, two bodies, lalloc's retry block
noruntime | 1 | cut/noruntime.go | 34 | - | b | RENAME BUILD TEXTQ | B4 | runtime strings to "", vimruntime = FALSE
phase 2 | 2 | front2 + query-empty | - | - | c | FallOutOf; FRAG FOLDX INITROW MOVE RENAME | B4 | D6-D8
nolocale | 2 | cut/nolocale.go | 66 | - | b | BUILD INITROW | B4 | a call statement, 3 table rows
nostartup | 2 | cut/nostartup.go | 42 | - | a | - | B4 | a body emptied, 2 calls
nocmdopts | 2 | cut/nocmdopts.go | 51 | - | a | - | B4 | 3 ifs, a member (after the ifs)
nosession | 2 | cut/nosession.go | 123 | - | b | BUILD TEXTQ | B4 | 18 stubs, 6 DropIf, a get_varp case
whim18 | 2 | phase/018 (Edit) | 44 | - | a | - | B4 | shell redirection and runtime completion; whim18kp is phase 18's
noenc | 2 | cut/noenc.go | 191 | - | b | FRAG | B4 | mb_init's dispatch, 6 iconv stubs
nofencs | 2 | cut/nofencs.go | 23 | - | a | - | B4 | an else-if arm
nofenc | 2 | cut/nofenc.go | 113 | - | b | BUILD FRAG | B4 | 'fileencoding' reads and stores
utf8only | 2 | cut/utf8only.go | 491 | - | b | FOLDX BUILD | B4 | five encoding flags constant, every test folded
noterm | 2 | cut/noterm.go | 57 | - | b | RENAME TEXTQ | B4 | "xterm" -> "xterm-256color"
nomouse | 2 | cut/nomouse.go | 211 | - | b | INITROW MOVE TEXTQ | B4 | 22 rows retargeted (a), 14 key rows, a body inlined
whim2a | 2 | phase/002/a | 52 | - | a | - | B4 | no title
query-empty whim2 | 2 | phase/002 + steps.go | 43 | - | b | TEXTQ INITROW(read) | B4 | asserts 24 rows are ex_ni
phase 3 | 3 | front3 + nointro + optreaders | - | - | c | FallOutOf; FRAG INITROW MOVE RENAME | B4 | D9-D12
noinert | 3 | cut/noinert.go | 69 | - | b | INITROW TEXTQ | B4 | :browse :confirm :behave
notabs | 3 | cut/notabs.go | 232 | - | b | FRAG | B4 | one tab page; 4 rewritten cases
noarglist | 3 | cut/noarglist.go | 30 | - | b | INITROW | B4 | completion of the arglist
nowindows | 3 | cut/nowindows.go | 328 | - | a | - | B4 | one window: 24 FoldNever, arms, stores
nowinsizes | 3 | cut/nowinsizes.go | 135 | - | b | BUILD FRAG | B4 | 9 initialisers added
nobuflist | 3 | cut/nobuflist.go | 65 | - | a | - | B4 | :bnext/:bprevious only
nonfa | 3 | cut/nonfa.go | 74 | - | b | FRAG | B4 | vim_regcomp's tail
noshellout | 3 | cut/noshellout.go | 39 | - | b | BUILD | B4 | 3 bodies
notags | 3 | cut/notags.go | 53 | - | b | FOLDX INITROW | B4 | `if (!f()) {X}` -> `(void)f();`
nosignals | 3 | cut/nosignals.go | 132 | - | b | INITROW FRAG | B4 | signal_info[] to 6 rows, a block
noequiclass | 3 | cut/small.go | 33 | - | a | - | B4 | [= classes
nocindent | 3 | cut/nocindent.go | 145 | - | a | BUILD | B4 | 'cindent'
noucmd | 3 | cut/noucmd.go | 80 | - | b | INITROW BUILD | B4 | user commands
noident | 3 | cut/noident.go | 148 | - | b | FRAG | B4 | nv_ident's 90-line body (maybe deletions: check)
nofnamemod | 3 | cut/nofnamemod.go | 53 | - | a | - | B4 | % modifiers
nocompl | 3 | cut/nocompl.go | 107 | - | a | BUILD | B4 | 9 stubs
nocomplkeys | 3 | cut/nocomplkeys.go | 309 | - | a | BUILD | B4 | 12 stubs; gotos kept by moved nodes
noabbr | 3 | cut/noabbr.go | 97 | - | a | - | B4 | abbreviations
3a | 3 | phase/003/a | 56 | - | a | - | B4 | lisp
3b | 3 | phase/003/b | 69 | - | a | - | B4 | langmap
3c | 3 | phase/003/c | 53 | - | a | BUILD | B4 | jump list
3d | 3 | phase/003/d | 41 | - | a | - | B4 | rot13
3e | 3 | phase/003/e | 49 | - | b | RENAME | B4 | sentence motions; 2 string respells
3f | 3 | phase/003/f | 33 | 3206 | b | FRAG | B4 | file marks: 2 bodies
nointro | 3 | cut/small.go | 11 | - | a | - | B1a | 2 calls
optreaders | 3 | cut/optreaders.go | 57 | - | a | TEXTQ | B1a | a statement found by its string
phase 4 | 4 | 13 steps, 1 sweep | - | - | b | FRAG FOLDX PARAM INITROW | B3a | droplocal x2 on the graph; 2 imports today
nostat | 4 | cut/nostat.go | 32 | - | a | BUILD | B1a | check_timestamps returns 0
4a | 4 | phase/004/a | 72 | 480 | a | TEXTQ | B1a | AUTOMATIC_ENGINE retries
onebuffer | 4 | cut/onebuffer.go | 176 | - | a | BUILD | B3a | one buffer; a key-table element Replace
4b | 4 | phase/004/b | 70 | - | b | INITROW PARAM | B3a | 19 key rows; win_line's parameter
4c | 4 | phase/004/c | 53 | - | b | FRAG | B3a | aucmd window
4d | 4 | phase/004/d | 80 | 8385 | b | FRAG FOLDX(walk) | B3a | one buffer: walks folded, bodies; members after their uses
4e | 4 | phase/004/e | 26 | - | b | PARAM TEXTQ | B3a | check_tty's parameter
nowild | 4 | cut/small.go | 24 | - | b | FRAG BUILD | B3a | delegations to save_patterns, a prototype
nowildmenu | 4 | cut/nowildmenu.go | 252 | - | b | PARAM | B3a | wildmenu; showmatches' 2 parameters
4f | 4 | phase/004/f | 84 | - | b | BUILD | B3a | completion keys; >=20 options[] callbacks to nullptr (Replace); after nowildmenu
phase 5 | 5 | 12 steps, 1 sweep | - | - | b | FRAG FOLDX RENAME PARAM MOVE | B3a | droplocal x2 on the graph; 1 import today
nobackup | 5 | cut/nobackup.go | 158 | - | a | - | B1a | backups and ACLs
lfonly | 5 | cut/lfonly.go | 310 | - | a | TEXTQ | B1a | LF only
keepbytes | 5 | cut/keepbytes.go | 98 | - | a | TEXTQ | B1a | ++bad gone
noconv | 5 | cut/noconv.go | 290 | - | b | FRAG RENAME | B3a | mb_* pointers become utf_* calls
5a | 5 | phase/005/a | 201 | - | b | PARAM FRAG MOVE | B3a | 'formatoptions', gq, = and !
5b | 5 | phase/005/b | 132 | 3511 | b | FRAG FOLDX(walk) RENAME | B3a | one window and tab: walks, 35 uses to curwin
5c | 5 | phase/005/c | 50 | 4353 | b | FRAG | B3a | the frame tree: 15 bodies
5d | 5 | phase/005/d | 112 | 4261 | b | FRAG TEXTQ | B3a | no autocommands
noglob | 5 | cut/small.go | 27 | - | b | BUILD | B3a | a one-call body; after nowild
phase 6 | 6 | nofind | 45 | - | b | FRAG | B3a | find_file_in_path's 15-line body
phase 7 | 7 | noinertopts, sweep, droplocal | 30 | - | a | - | B1a | droplocal on the graph; noinertopts then makes the sweep a collection
phase 8 | 8 | droplocal | - | - | done | - | A | begins on the graph (q007.g)
phase 9 | 9 | nohome, nogetenv | 138 | - | b | FRAG | B3a | bodies naming libc (externs)
phase 10 | 10 | nochdir | 112 | - | b | FRAG | B3a | 2 bodies with static locals, getcwd, errno
phase 11 | 11 | nofloat | 97 | - | a | TEXTQ | B1a | string literals' conversions; case arms
phase 12 | 12 | noowner, droplocal | 123 | - | a | BUILD | B1a | `(void)f()`; droplocal on the graph
phase 13 | 13 | sweep, droplocal | - | - | done | - | A | begins on the graph (q012.g)
phase 14 | 14 | sweep, droplocal | - | - | done | - | A | begins on the graph (q013.g)
phase 15 | 15 | whim15a, whim15 | 37 | - | a | BUILD | B1a | an nv_cmds element Replace; :noswapfile
phase 16 | 16 | oneoptset, sweep, droplocal | 142 | - | b | RENAME | B3a | a string literal respelled
phase 17 | 17 | droplocal | - | - | done | - | A | begins on the graph (q016.g)
phase 18 | 18 | whim18kp, droplocal | 5 | - | a | - | B1a | a get_varp case; maybe DropLocal's rule already takes it
phase 19 | 19 | whim19, sweep, whim19ep, droplocal | 73 | - | a | BUILD | B1a | options cut; whim19ep maybe DropLocal's rule
phase 20 | 20 | whim20, sweep, whim20bl, droplocal | 115 | - | a | BUILD | B1a | options cut; whim20bl maybe DropLocal's rule
phase 21 | 21 | whim21 | 59 | 2403 | b | FRAG | B3a | one file argument
phase 22 | 22 | whim22 | 38 | 3147 | b | FRAG | B3a | :e in place: a 45-line block with stat()
phase 23 | 23 | whim23 | 21 | - | a | TEXTQ | B1a | one FoldNever and a row query
phase 24 | 24 | edit whim24 | 188 | - | done | - | A | begins on the graph (q023.g)
phase 25 | 25 | whim25 | 129 | 961 | a | BUILD (FOLDX would make it generic) | B1a | constant-return predicates folded
phase 26 | 26 | whim26 | 262 | 450 | b | RENAME TEXTQ | B3a | the Ex table: enumerators last-first, a string respelled
phase 27 | 27 | whim27 | 346 | 1992 | b | PARAM TEXTQ | B3b | no Ex mode: ~28 folds, main_loop's parameter
phase 28 | 28 | whim28 | 67 | 850 | b | RENUM BUILD TEXTQ | B3b | no :write
phase 29 | 29 | whim29 | 115 | 638 | b | RENUM | B3b | no :read
phase 30 | 30 | whim30 | 131 | 844 | b | RENUM TEXTQ | B3b | no :edit, gf
phase 31 | 31 | whim31 | 102 | 569 | b | PARAM | B3b | open_buffer(void)
phase 32 | 32 | whim32 | 476 | 5665 | b | RENUM PARAM TEXTQ FOLDX | B3b | the buffer has no name
phase 33 | 33 | whim33 | 87 | 656 | a | TEXTQ | B1b | ex_quit's dead tail, two members
phase 34 | 34 | whim34, droplocal x2, whim34rows | 155 | 513 | b | PARAM(variadic arg) TEXTQ | B3b | W10 and [RO]; droplocal on the graph; 1 import today
phase 35 | 35 | whim35 | 227 | 1854 | b | PARAM | B3b | the never-opened FILE*s
phase 36 | 36 | whim36 | 242 | 12079 | b | FRAG RENAME | B3c | 18 musl string functions; ~600 uses renamed
phase 37 | 37 | whim37 + musl-*.md | 188 | 15400 | b | FRAG RENAME | B3c | ctype, case tables in-file; after 36
38a | 38 | phase/038/a | 199 | - | a | TEXTQ | B1b | deathtrap's ladder; the assertions are the work
38b | 38 | phase/038/b | 98 | - | b | RENAME RETYPE FRAG | B3c | main -> static vim_main, a launcher; the collection's root moves
phase 38 | 38 | whim38a, whim38b, whim38 | 164 | - | b | FRAG PARAM | B3c | exit through the host; builtins
phase 39 | 39 | whim39 | 254 | 20227 | b | FRAG INITROW RENAME | B3c | the host block: macros, termios, externs
phase 40 | 40 | whim40 | 330 | - | b | FRAG PARAM TEXTQ | B3c | stream writes through vim_host_message
phase 41 | 41 | whim41 | 255 | - | b | PARAM RENAME FRAG INCLUDE | B3c | host calls direct
phase 42 | 42 | whim42 @minmax | 400 | - | b | RETYPE RENAME FRAG MACROX INCLUDE | B3c | header types owned; MIN/MAX may stay a text step
phase 43 | 43 | whim43 @state | 717 | - | d | (MOVE INCLUDE FRAG TEXTQ to drop gcc) | B3d | the first #include becomes the boundary; gcc-judged as built
phase 44 | 44 | whim44 | 272 | - | b | RETYPE FRAG INCLUDE | B3d | the scalar clock
phase 45 | 45 | whim45 | 287 | - | b | INITROW | B3d | case tables merged (rows read as values)
phase 46 | 46 | whim46 | 158 | - | b | FRAG | B3d | 1 statement -> 2: the smallest FRAG
phase 47 | 47 | crefactor/xform/libcown.go | 7+248 | - | b | FRAG INCLUDE | B3d | abs/labs own; text, not typed
phase 48 | 48 | whim48 | 369 | - | b | RENAME MOVE RETYPE FRAG INCLUDE | B3d | the clock crosses; after 43, 47
49a | 49 | phase/049/a | 186 | - | b | FRAG INCLUDE | B3d | realloc leaves the core; before 49b
49b | 49 | phase/049/b | 396 | 828 | b | FRAG INCLUDE | B3d | malloc/free/write leave; before 49
phase 49 | 49 | whim49a, whim49b, whim49 @state | 506 | 508 | b | FRAG INCLUDE | B3d | getpid/kill leave
phase 50 | 50 | crefactor/xform/unions.go | 6+311 | - | b | RETYPE | B3d | degenerate unions; text, not typed
51a | 51 | phase/051/a | 374 | - | b | INITROW RENAME TEXTQ | B3d | terminal names; before 51
phase 51 | 51 | whim51a, whim51 | 389 | - | b | PARAM MOVE FOLDX RENAME | B3d | -T goes
phase 52 | 52 | whim52 @state | 218 | 2918 | b | FRAG | B3d | the arena; header types (max_align_t)
phase 53 | 53 | whim53 @state | 817 | 4723 | b | PARAM FOLDX FRAG | B3e | swap-file residue; mostly deletions
phase 54 | 54 | whim54 @state | 589 | 15478 | b | FRAG RETYPE RENAME PARAM | B3e | a block number becomes a reference; after 53
phase 55 | 55 | whim55 @state | 665 | 4418 | b | FRAG PARAM MOVE | B3e | de-page the leaf; after 54
phase 56 | 56 | whim56 @state | 791 | 4045 | b | FRAG RETYPE PARAM | B3e | fold the node types; after 55
phase 57 | 57 | whim57 | 13 | - | b | RETYPE | B3f | p_emoji int
phase 58 | 58 | whim58 | 13 | - | a | - | B1c | three (pos_T *)-1 tests fold
phase 59 | 59 | whim59 | 23 | - | b | RETYPE RENAME | B3f | garray_T *
phase 60 | 60 | crefactor/xform/dropcalls.go | 7+94 | - | a | BUILD FOLDX(dead stores) | B3g | no-op frees go; not typed
phase 61 | 61 | whim61 | 23 | - | b | FRAG | B3f | buflist_findnr
phase 62 | 62 | crefactor/xform/emptyblocks.go | 7+89 | - | b | FOLDX(empties everywhere) | B3g | empty blocks fold; not typed
phase 63 | 63 | whim63 | 58 | - | b | MOVE RENAME RETYPE | B3f | one regprog type
phase 64 | 64 | whim64 | 21 | - | a | BUILD | B1c | the engine called directly
phase 65 | 65 | whim65 | 14 | - | b | RETYPE RENAME FRAG | B3f | changedtick a number
phase 66 | 66 | whim66 | 58 | - | b | PARAM FOLDX TEXTQ | B3f | parameters never used
phase 67 | 67 | whim67 | 72 | - | b | FRAG RETYPE RENAME | B3f | typed sort and search
phase 68 | 68 | whim68 | 57 | - | b | FRAG | B3f | highlight groups by scan
phase 69 | 69 | whim69 | 66 | - | b | FRAG MOVE | B3f | regrepeat's goto into a case
phase 70 | 70 | whim70 | 17 | - | b | RENAME PARAM | B3f | no build date
71a | 71 | phase/071/a | 98 | - | b | MOVE FRAG | B3f | regatom: an outlined block
71b | 71 | phase/071/b | 207 | - | b | FRAG MOVE | B3f | edit(): 17 gotos
phase 71 | 71 | whim71a, whim71b, whim71 | 59 | - | b | FRAG MOVE | B3f | check_termcode; after 71a, 71b
phase 72 | 72 | whim72 | 58 | - | b | FRAG TEXTQ | B3f | a node names its block
phase 73 | 73 | whim73 | 58 | - | b | INCLUDE FRAG | B3f | deathtrap at the next wait (host)
74a | 74 | phase/074/a | 20 | - | b | FRAG | B3f | lalloc's body
phase 74 | 74 | whim74a, xform/nevernull.go | 7+253 | - | b | FOLDX(label) | B3g | never-NULL tests fold; not typed
phase 75 | 75 | whim75 | 73 | - | b | FRAG INITROW TEXTQ | B3f | three typed stacks
76a | 76 | phase/076/a | 96 | - | b | INITROW RETYPE RENAME FRAG | B3f/step 6 | def_val split; a typed transform in text
phase 76 | 76 | whim76a, whim76 | 294 | - | b | INITROW RETYPE FRAG RENAME | B3f/step 6 | optvar_T: kinds from typed edges
77a | 77 | phase/077/a | 13 | - | b | FRAG | B3f | a comparison
phase 77 | 77 | whim77a, whim77 | 13 | - | a | - | B1c | an if deleted (77 alone)
phase 78 | 78 | whim78 | 27 | - | b | FRAG MOVE | B3f | gcc's argument order through locals
phase 79 | 79 | whim79 | 16 | - | b | FRAG | B3f | a static byte for (char_u *)-1
phase 80 | 80 | whim80 | 23 | - | b | RETYPE | B3f | yankreg_T *
phase 81 | 81 | whim81 | 16 | - | b | FRAG | B3f | a font read guarded
phase 82 | 82 | whim82 | 28 | - | b | RETYPE FRAG INITROW | B3f | flexible arrays become pointers
phase 83 | 83 | whim83 | 29 | - | b | PARAM RETYPE TEXTQ | B3f | no cookie
phase 84 | 84 | whim84 | 37 | - | b | FRAG MOVE | B3f | ml_get_invalid outlined
phase 85 | 85 | whim85 | 19 | - | b | FRAG | B3f | a flag for a pointer comparison
86a | 86 | phase/086/a + xform/deadstmt.go, terminates.go | 6+107 | - | a | - | B1c | statements after a jump; cc.Parse only
phase 86 | 86 | whim86a, whim86 | 35 | - | b | PARAM FRAG | B3f | six dead stores; one parameter becomes a local
87a | 87 | phase/087/a + xform/boolret.go | 7+1020 | - | b | RETYPE BUILD | B3f | bool for 278 functions; cc.Parse only
phase 87 | 87 | whim87a, whim87 | 167 | - | b | FRAG TEXTQ | B3f | 153 key codes named
phase 88 | 88 | crefactor/xform/includes.go | 159 | - | d | - | - | each #include tried under gcc
phase 89 | 89 | crefactor/xform/gototail.go | 366 | - | b | CLONE FOLDX(label) | B3g | goto tails; cc.Parse only
phase 90 | 90 | crefactor/xform/gotobreak.go, gotoflow.go | 84+186 | - | a | BUILD FOLDX(label) | B3g | goto -> break
phase 91 | 91 | crefactor/xform/gotoloop.go, gotoflow.go | 232+186 | - | b | MOVE | B3g | goto back -> loop
phase 92 | 92 | crefactor/xform/gotoblock.go | 404 | - | b | MOVE | B3g | goto out -> do-while(0) break
phase 93 | 93 | whim93 | 71 | - | b | FRAG MOVE TEXTQ | B3f | one_adjust as functions
phase 94 | 94 | crefactor/xform/memberout.go, rewrite.go | 703+147 | - | c | step 6 | 6 | member out-parameters
phase 95 | 95 | crefactor/xform/stateparam.go | 429 | - | c | step 6 | 6 | the engine's state a parameter
phase 96 | 96 | whim96 | 232 | - | b | FRAG MOVE | B3f | a line's match alone; after 95
phase 97 | 97 | whim97 | 19 | - | b | FRAG | B3f | :g asks match_range; after 96
phase 98 | 98 | whim98 | 16 | - | b | FRAG | B3f | ml_clearmarked's guard
phase 99 | 99 | whim99 | 16 | - | b | INCLUDE FRAG | B3f | WHIM_TIME (host)
phase 100 | 100 | crefactor/xform/localout.go, structscalar.go | 1148+420 | - | c | step 6 | 6 | values for out-parameters and struct locals
phase 101 | 101 | crefactor/xform/plainc.go | 308 | - | c | step 6 | 6 | identity, ascii classes, constant ifs
phase 102 | 102 | crefactor/xform/boolret.go (Globals) | 11 | - | b | RETYPE BUILD | B3f | file-scope flags bool
phase 103 | 103 | crefactor/xform/boolret.go (Relax) | 12 | - | b | RETYPE BUILD | B3f | more flags bool
```

190 rows: 45 (a), 126 (b), 8 (c: the `FallOutOf` wrapper and the three
front phases it wraps, 94, 95, 100, 101), 6 (d: phase 0 and its three
parts, 43 as built, 88), and 5 `done` (the phases that begin on the graph
now; the `droplocal` steps of 4, 5, 7, 12, 16, 18-20 and 34 are done too,
inside phases whose other steps are listed). A phase of several units has a
row of its own besides theirs, so the classes overlap by those rows.

## Batches for stage B

Each batch is one agent's work in a worktree of its own
(`.tmp/worktrees/`), merged in the order the arrows say; batches on one line
of the diagram run side by side. Every conversion batch proves itself with
the recipe above, so two that touch different phases merge cleanly and are
re-proved together by one `whim-build-check` after the merge.

```
B0 ──┬── B1a, B1b, B1c                          (a-class conversions)
     └── B2a FRAG, B2b INITROW/RENUM/RENAME, B2c PARAM/RETYPE/MOVE,
         B2d FOLDX, B2e INCLUDE                  (capabilities, side by side)
              └── B3a (4-6, 9-12, 16, 21, 22, 26), B3b (27-35),
                  B3c (36-42), B3d (43-52), B3e (53-56, a chain),
                  B3f (57-87, 93, 96-99), B3g (60, 62, 74, 89-92)
                       └── B4 the front (1-3), with step 6's closure
step 6: 94, 95, 100, 101 (and FallOutOf; 76/76a better there)
```

- **B0, the library** -- **done**, *B0 as built* above: VERBS, BUILD,
  TEXTQ's helpers, and the opt-in declaring-block splice, about 1,560 lines
  of Go in `crefactor/graph`; its tests the text verbs' own cases on graphs
  read back from Lisp; an initialiser element verified a one-node place on
  the snapshots (onebuffer's and 15a's `nv_cmds` rows).
- **B1, the a-class conversions** (after B0; three agents):
  - **B1a**: phases 7 (noinertopts), 11, 12, 15, 18, 19, 20, 23, 25 and the
    steps nostat, 4a, nobackup, lfonly, keepbytes, nointro, optreaders.
    With B1a, phases 12-14 and 17-20 are graph from end to end: 17-20 run
    with no import at all, the graph handed along. ~1,700 lines of text
    programs replaced. Check first whether DropLocal's get_varp rule
    already takes whim18kp's, whim19ep's and whim20bl's cases (then those
    programs go and nothing replaces them).
  - **B1b**: 33, 38a (their assertions as queries).
  - **B1c**: 58, 64, 77, 86a (`Terminates` on nodes).
- **B2, the capabilities** (after B0, side by side, each in
  `crefactor/graph`, generic, with unit tests on read-back graphs):
  - **B2a FRAG** (with CLONE and MACROX): the largest; the externs and
    builtins (`__builtin_setjmp`, `__builtin_offsetof`), macro invocations
    made nodes as the importer makes them, new ids, types.
  - **B2b INITROW, RENUM, RENAME** (string literals included).
  - **B2c PARAM, RETYPE (clearing types, no checker), MOVE.**
  - **B2d FOLDX**: the rules listed above, each with the units that want
    it (B0 took the declaring-block splice).
  - **B2e INCLUDE** and the boundary queries (the first include form,
    "above/below", the host region).
- **B3, the b-class conversions** (each after the capabilities its rows
  name; one agent each; ranges chosen so a batch's phases are contiguous,
  which is when a conversion starts saving time):
  - **B3a** phases 4-6, 9-12, 16, 21, 22, 26 (~2,300 lines): FRAG, INITROW,
    PARAM, FOLDX(walk), RENAME, MOVE. With B1a, 4-26 is graph but for 15
    and 24's neighbours; the in-phase sweeps become collections.
  - **B3b** phases 27-35 (~1,600): RENUM, PARAM, FOLDX, little FRAG.
  - **B3c** phases 36-42 (~2,100, ~50 KB of literal C): FRAG with externs,
    builtins and macros; RENAME; PARAM; INCLUDE. 42's MIN/MAX may stay a
    text step.
  - **B3d** phases 43-52 (~3,900): INCLUDE everywhere; 43 rewritten with
    edge queries in place of gcc's answers, or left text (then 44 imports);
    order 43 -> 47 -> 48; 49a -> 49b -> 49; 51a -> 51.
  - **B3e** phases 53-56 (~2,900, ~29 KB of literal C), a strict chain: one
    agent, in order. Mostly FRAG of whole items and bodies; 53 is mostly
    deletions.
  - **B3f** phases 57-87 but 58/60/62/64/74/77/86a, and 93, 96-99 (~2,100 of
    programs, BoolRet's 1,020): FRAG, RETYPE, PARAM, MOVE, INITROW; 76/76a
    may wait for step 6 (a typed transform written in text); 96 after 95.
  - **B3g** phases 60, 62, 74 and 89-92: the `crefactor/xform` text
    transforms (DropCalls, EmptyBlocks, NeverNull, the gotos: ~1,600 lines)
    on the graph: CLONE, MOVE, FOLDX.
- **B4, the front** (phases 1-3: 47 cutters and parts, ~5,100 lines): last.
  Its cutters run inside one step each, wrapped by `xform.FallOutOf`,
  which types the text before and after the cuts. Until the closure has
  an UNWRITTEN seed over write edges and `xform.FallOut`'s later rules
  (B2d), or step 6 has ported FallOut, a front phase cannot end on the
  graph; its cutters can be converted inside it only if the wrapper is
  given the text before them. Most cutters are (a) or need BUILD/INITROW.
- **Step 6**: phases 94, 95, 100 and 101, and FallOutOf -- the typed
  transforms on typed edges; the graph's `Untyped` lists are where its
  checker starts.
- **Stays text**: phase 0 (the seed and 0a-0c) and phase 88 (gcc).

**What the pipeline would gain, measured on what stage A moved.** A phase
handed the graph costs 0.21-0.37 s (index, cut, closure, collection, C
view) against 2.4-3.7 s on text; the import that a text-to-graph boundary
costs is 1.7-2.1 s and the C view 0.05-0.09 s. With phases 1-103 on the
graph but 88 (and 0), the in-order build would pay two imports (after 0
and after 88) and about 100 phases at ~0.3 s plus their cuts: the
estimate of GRAPH.md (some 230 s of today's ~350 s) stands, and stage A's
numbers are its first measurements.

## B2e as built: INCLUDE and the line (2026-10-03)

`crefactor/graph`'s `include.go` (the include edits, the line, the rule,
top-level moves) and `headers.go` (what a header provides); generic,
naming nothing in vim. 893 and 116 lines of Go, comments and blank lines
aside. Nothing else in the package changed. The snapshot tests are
`internal/graphcheck`'s `include_test.go`.

### The API

**The line.** The first include form is the line between the core above it
and the host from it on; a graph with no include form has no line, and is
all core. `IsInclude(n)`, `IncludeSpec(n)` (`<stdio.h>`), `NewInclude(spec)`;
`Editor.Includes()`, `FirstInclude()`, `Core()`, `Host()` (the forms on
either side, the includes the host's), `InCore(n)`/`InHost(n)` (by n's
top-level form; an external or a type is in neither), `TopForm(n)`, and
`FormsC(forms)`, the C view of some forms alone (the core's is `whim.Cut`'s
text, below).

**What the headers provide.** `HeaderOf(spec)` is what one include line
provides, parsed by cc on its own with the importer's configuration and
cached for the process (5-37 ms a header, 0.34 s for whim's 41): `Names`, the
ordinary names declared at file scope and the tags (`struct NAME`);
`Macros`; `Bare`, the object-like macros whose replacement names nothing; a
unit with no include subtracted (the compiler's own names need no header).
`Editor.HeaderUses()` is every name the file takes from the headers, each
with its first using form and the include above it that provides it (0.26 s
on whim-vim.c); `Missing(incs...)`, what deleting those includes would leave
unprovided; `Collisions()`; `SpareIncludes()`, the includes the rule says the
file can do without, chosen in crefactor/xform's `Includes` order (each alone,
then together, else a fold from the bottom).

**The edits**, each refused as the rule says and leaving the graph as it was
when refused: `InsertIncludeBefore/After(at, spec)` (a fresh id, an
`insert` act); `DeleteInclude(inc)`; `MoveFormsBefore/After(at, ns...)`,
top-level forms moved keeping their ids (one `move` act, every id in
`Moved`), includes or declarations; `MoveToHost(ns...)` (after the include
run the line begins) and `MoveToCore(ns...)` (before the first include) --
moving a declaration across the line. And two that go further on purpose:
`DeleteIncludeRebind(inc)` and `InsertIncludeRebind(at, spec, after)`,
below. A refusal by collision is a `*CollisionError` (`Names()`).

### The extern rule

What the file takes from the headers is a DECLARATION -- an external node
with a live use that spells it (`(extern getenv)`, `(extern-typedef
time_t)`, the `(struct termios)` that names a tag; its uses through a macro
are the macro's) -- or a MACRO -- a token some include's header defines: an
identifier atom with no edge that the file does not declare, an atom whose
edge goes to an external spelled otherwise (`errno`), or an identifier in a
`(macro "...")` text; C23's keywords are the language's (`bool`, which
<stdbool.h> defines for an older C). A name is provided where an include
above its first use has a header that declares it (defines it, for a macro).
A COLLISION is an include's macro over a name the file has as its own below
it: one it declares (a syntax error: phase 43's reason) or one it uses with
an edge to its own declaration (which would silently become the macro).

An edit is refused where it would leave a provided name unprovided, or make
a new collision; the rule is RELATIVE to the graph before the edit, so a name
nothing provides already is never the edit's fault. So a removed header's
names still used must come from another include above their first use (the
external nodes stay, as cc typed them at import: that two headers declare a
name alike is not checked), an added header must not take the file's names,
and a form moved above the line must not take a header's name with it. A
move also keeps C's declare-before-use: a refers edge whose declaration was
above its use stays so (a top-level MOVE; B2c's is the general one).

`DeleteIncludeRebind` is a deletion where a removed header's macro is a name
the file declares itself: its tokens become uses of that declaration (a new
atom with its edge, by `Resolve`), as an import of the text after makes them.
`InsertIncludeRebind` is its inverse: the file's uses below the new include
become the macro's tokens (no edge, which is the importer's for a `Bare`
macro; one whose replacement names something is refused). Each is where
the program still compiles and a token now says something else: what a
compiler's silence accepts and the rule does not.

### What the snapshot tests prove

On the graphs read back from Lisp (no cc behind them), with GRAPH_SNAPS:

- **Phase 88** (`TestIncludesPhase88`): on q087 the rule finds 29 of the 31
  headers gcc removed spare. The two it keeps, `<stdlib.h>` and `<stdint.h>`,
  provide `EXIT_FAILURE` and `SIZE_MAX` to two of phase 43's static_asserts
  in the host, which assert the core's own enumerators against the headers;
  with them gone both still compile, silently, and compare each enumerator
  with itself. **That is a finding**: since phase 88 the `SIZE_MAX` assert
  in whim-vim.c is a tautology (gcc -E confirms no remaining header defines
  it), and the `EXIT_FAILURE` one was until phase 99 brought `<stdlib.h>`
  back. Deleting gcc's 31 on the graph -- `DeleteInclude` where the rule
  allows, `DeleteIncludeRebind` for those two, whose tokens become the core's
  enumerators -- gives **q088.c byte for byte**. The control: `<termios.h>`
  is not spare on q088.
- **Phase 73** (`TestIncludesPhase73`): `<fcntl.h>` moved after
  `<termios.h>` by one `MoveFormsAfter`, its id kept: **the text program's
  two literals byte for byte**; `<termios.h>` moved to the end of the file is
  refused.
- **Phase 99** (`TestIncludesPhase99`): `<stdlib.h>` after `<stddef.h>`: the
  strict insertion is refused as a collision of use (the assert's
  `EXIT_FAILURE`, which phase 88 had made the core's); `InsertIncludeRebind`
  makes it, the token the macro again as q099.c's import has it: **the text
  program's literal byte for byte**, read back the same graph.
- **Phase 43** (`TestIncludesPhase43`): on q043, the includes moved back
  above the core are refused, naming **exactly the twelve** names phase 43
  found by compiling (its GOAL.md: INT_MAX, INT_MIN, LONG_MAX, LONG_MIN,
  LLONG_MAX, LLONG_MIN, ULLONG_MAX, SIZE_MAX, PATH_MAX, EXIT_FAILURE, SIGHUP,
  SIGTERM); the graph unchanged.
- **The line on every snapshot** (`TestTheLine`, q000-q103): the core's C
  view is `whim.Cut`'s text byte for byte from q043 on, and before it the
  first form is an include and Cut finds no core; and nothing above the line
  takes a name from the headers.

`crefactor/graph`'s `include_test.go` covers each piece on samples read back
from Lisp: the line, the uses and their providers, a deletion refused and
made, both rebinds against an import of the text after, insertions (a fresh
id, a duplicate, a collision of declaration, a collision of use), moves
across the line both ways (a header's name refused into the core, a
declaration refused below its use, the two together allowed), an include
moved and refused below its names' use.

### Limits

- A header's set is the header parsed alone. A header whose declarations
  depend on what an earlier one defined (feature macros) is read as it is
  alone; whim's never do (88 matches gcc on all 31 but the two above, for
  the reason given).
- Types are not re-checked: a name another header provides keeps its
  external node and cc's type from the import.
- A macro token is any identifier-shaped token a header defines; a member
  or local spelled like a header's function-like macro counts too. A
  declaring atom of a parameter in a nested function type is found by the
  form's shape (`(fn ((NAME TYPE) ...) R)`).
- A rebind into a macro's own invocation text is refused (the text is not
  nodes). `InsertIncludeRebind` makes tokens without edges, right for a
  `Bare` macro only.
- `MoveForms*` moves top-level forms only; items, runs and bodies are B2c's
  MOVE. Its order rule looks at refers edges, not typed edges.
- Each include edit computes the headers' state twice (about 0.5 s on
  whim-vim.c); a phase making many should batch them (`MoveForms*` takes
  many forms at once, `Missing` many includes).
