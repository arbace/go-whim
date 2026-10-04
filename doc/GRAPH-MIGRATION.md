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
| RENAME | **done (B2b)**: `Editor.Rename` -- a declaration, the entity it is one of (a function's prototypes and definition), and every use by edge respelled: a function, an object, a typedef, an enumerator, a local, a parameter, a member (selections and designators), a tag, a label -- refused where a name would change meaning; `RetargetAs`/`RetargetUses`, a use pointed at a target of another spelling; `RespellString`, a string literal written anew, each one named whole (*B2b as built*) | ~25 |
| INITROW | **done (B2b)**: `Editor.ArrangeRows`/`DeleteRows`/`InsertRows`/`BuildRows`, a table's rows deleted, inserted, reordered, every position the file names said again -- subscripts by constants (by edge), the index enumerators and permutation arrays a cut names (`RowIndex`) -- and the table's typed edge resized; verbs `DeleteRows`, `DeleteRowsEach`, `InsertRows` | ~20 |
| RENUM | **done (B2b)**: `Editor.ArrangeEnum`/`DeleteEnumerators`/`MoveEnumerators`/`InsertEnumerators` under a policy -- `HoldValues` (no value moves), `Renumber` (they move, reported: `CMD_index` following the rows the front cut), `PinValues` (the sweep's pin, byte for byte) -- and `BuildValue`, `EnumValues` | 5 |
| FRAG | **done (B2a)**: a C fragment -- statements, a body, an expression, external declarations, an `editlit.go` literal -- made graph nodes in the context of its place, as the importer makes them: `Editor.SpliceC(Frag{At, Src, Holes}...)` at a spot (`SpotOf`, `SpotRun`, `SpotBefore`/`SpotAfter`, `SpotBody`, `SpotEnd`), on a synthesized unit cc parses, checks and imports; the verbs `BodyC`, `LiteralC`, `ReplaceC`/`ReplaceAtC`, `BeforeC`/`AfterC`, `SpliceRunC`, `TopBeforeC`/`TopAfterC`, and `Together` (a phase's FRAG acts in one import): *B2a as built* | ~70 |
| CLONE | **done (B2a)**: `graph.Clone(n)`, a subtree copied without ids, its edges inside it to the copies and the rest where n's go; BUILD's `?h` and FRAG's `$h` used twice are a copy the second time | 4 |
| MACROX | **done (B2a)**: a macro's invocation in a fragment made a node as the importer makes it (with its expansion's edges), and an opaque `(macro "...")` expanded: `MacroCall(n)` reads its name and arguments from its text, `ExpandMacros` replaces each by the C a caller makes of them, through FRAG (42's `MIN`/`MAX`: an argument written twice is parsed twice, no CLONE needed) | 1 |
| RENAME | a declaration and its uses respelled (a member, a function, a typedef), a use retargeted to a target of another spelling, a string literal respelled (its array type changes) | ~25 |
| INITROW | an initialiser element deleted (a table row: `cmdnames[]`, `options[]`, `key_names_table`, `nv_cmds`), found by its designator or its string; inserted; with positional arrays' indexes said | ~20 |
| RENUM | an enumerator deleted with the implicit values after it moved, opted into (`CMD_index` must follow the rows the front cut) | 5 |
| PARAM | a parameter deleted with the argument at every call, in function-pointer types too; one argument of a variadic call deleted (with its format string) | ~20 |
| RETYPE | a declaration's type changed (`int` to `bool`, `void *` to `T *`, a union member to its one member), the typed edges above it cleared and listed (`Untyped`) until step 6's checker re-derives them | ~20 |
| MOVE | a node or a run of items moved elsewhere keeping their ids (an outlined block, a body inlined, a range wrapped in a loop, a definition moved below the boundary) | ~15 |
| INCLUDE | **done (B2e)**: `#include` forms added, deleted or moved under the extern rule (each name the file takes from the headers provided by an include above its first use; no header macro over the file's own names below it), and the first one as the core/host boundary: `FirstInclude`, `Core`/`Host`, `InCore`/`InHost`, `MoveToHost`/`MoveToCore` (*B2e as built*) | ~15 |
| PARAM | **done (B2c)**: `Editor.DropParams`/`DropParam` -- a parameter dropped from every declaration of a function and from the fn forms of the pointers, members, parameters and typedefs of its family, the argument from every call, through pointers and tables too; every use of what changes type a call, a flow to the same new type, or a test, else refused; `DropArg` (one argument through `...`), `ParamToLocal`, `AddParam` (*B2c as built*) | ~20 |
| RETYPE | **done (B2c)**: `Editor.Retype` (an object in every declaration, a local, a member, a typedef to a fixed point, a function's parameter in every declaration) and `RetypeResult`; the typed edges above every use typed again where plain, else cleared into `Untyped`; `Editor.Rederive` (*B2c as built*) | ~20 |
| MOVE | **done (B2c)**: `Editor.MoveBefore`/`MoveAfter`/`MoveRun` (items beside an item, ids and edges kept) and `MoveTo` (a node into a placeholder's place, a fill in its own), refused where a use would not resolve, a declaration would hide or repeat one, or a jump would bind elsewhere (*B2c as built*) | ~15 |
| INCLUDE | `#include` forms added, deleted or moved, and the first one as the core/host boundary: "is X above it", "the host region" | ~15 |
| FOLDX | **done (B2d)**: `Editor.FoldX` -- xform.FallOut's closure on the graph: the UNWRITTEN seed over write edges (`Editor.Unwritten`, `FoldX.Before`/`Hold`), the reads of objects and members nothing writes, `a && K`/`a \|\| K` and the constants (cc's evaluation: `!K`, `K == 0`, sizeof, offsetof), the branches, the statements after a revealed jump, a function whose body is `return K;` K at its calls, a parameter every call passes alike, a function left empty uncalled; a cut's own literals as marks (`Marks`, `NoSeeds`); and beside the closure, opted into (`FallOutOptions`): the empty blocks everywhere with the locals only given values (`EmptyBlocks`, phase 62's), a label no goto reaches, `if (!f()) {}` made `(void)f();` (`KeepCondition`), an if whose only effect was a store to a deleted location (`StoreIfs`), the value a cut gives a deleted object (`Values`). The walk fold and the declaring splice are B0's: *B2d as built* | ~30 |
| DRAFT | **done (B3e)**: `Editor.Draft()`/`Draft.Commit(text)` -- a program's text acts on the C view made the graph's: the changes widened to the innermost runs of whole items, written anew in one FRAG import (fragments naming one another allowed) or deleted, the uses of a struct's members, enumerators and tags written anew carried by name (*B3e as built*) | 4 |

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
  visible, a macro, a compound literal or initialiser, a function type.
  A hole used twice is a copy the second time (CLONE, B2a). (B2a found
  that "the FIRST declaration" is not quite cc's rule: a use after a
  function's definition whose prototype names no parameter refers to the
  definition -- `crefactor/cc`'s `Scope.ident`, *B2a as built*. BUILD
  still says the first; FRAG says what cc says.)
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

## B1b as built (2026-10-03)

Phase 33 and part 38a are graph steps (`phase.RegisterGraph`, `Graph: true`
in the plan); their text programs are gone (history keeps them, and 33's
`editlit.go`). Written on B0's library alone -- `One`, `Cut`, `InFunction`,
`InTable`/`Rows`, `UsesOf`, `Decls`, `Strings`, `Text`, the editor's
`Delete`, `Sibling`, `Parent`, `Item`, `Uses` -- with no change to
`crefactor/graph`. 96 and 247 lines of Go (78+8 and 198 before), the
growth 38a's, whose assertions now say what each anchor stood for.

**How the assertions moved.** One rule for both programs:

- **A `\bname\b` count stays the text's question**, on the C view, number
  for number (`edit.MentionCount` on `v.Text()`): such a count includes
  prototypes, strings and labels, and both phases' tables of counts were
  written against exactly that. **Take the view once** for a table of
  counts: `Verbs.Mentions` prints the scope's view on every call (50-90 ms
  for the file), and 33's 40 counts would cost 2-3 s that way, against two
  views here.
- **An exact line or block becomes a pattern**: 33's four-line tail is a
  run of four items (found once in the file, each next one its sibling);
  38a's `signal_info[]` is its six rows by `Rows`, catch_signals()'s
  deadly arm and deathtrap()'s head are forms, and the ladder is one node,
  found once in the file and checked between `full_screen = FALSE;` and the
  `entered == 2` arm, as the text checked it in context.
- **A regexp or a count that stood for uses becomes an edge query**:
  "not_exiting 2 mentions" is two declarations and no use; "the one
  `catch_signals(deathtrap, SIG_ERR)`" is deathtrap's one use; the writes
  of `entered` (a regexp over deathtrap's lines, with a byte test for
  RE2's missing lookahead) are the uses of the static's declaration that
  write it -- `++`/`--`, an assignment's left side, `&` -- of which there
  must be one, `pre++`; "statements beginning with `exit(`" are the uses of
  the external `exit`, each a call standing as a statement, told by its
  function. The counting trap is now a partition: the six mentions of
  `exit` are two string literals (`Strings`), `(goto exit)` and `(label
  exit)` in vim_regsub_both, and two calls of the external.
- **The text-only ones are dropped**: 38a's "the file lost 9 lines" and
  "N runs of two blank lines, exactly as before" have no counterpart; its
  last report line no longer says the blank-line count.

The reports are the text's line for line but for "goes in the sweep",
which says "the collection" where the collection now takes it. Each
refusal was tried on a mutated snapshot: `sa_flags = 4`, `entered += 1`,
`&entered` and `entered = ...` inside deathtrap, a third deadly row, a third
`exit()` call, a second use of deathtrap, and `exiting = FALSE` in ex_quit's
tail each refuse with the phase's own words.

**The proof.** `rm -rf .cache/boundaries; make whim-build-check`, in order:
whim-vim.c byte for byte, every boundary compiling, 347 s (348 s for stage
A), seven graph snapshots written (q032.g and q037.g new). Again, in
parallel: byte for byte, 89 s (77 s the links, 12 s the compiles, under the
other batches' load), 7 of 7 links that begin on the graph read from their
snapshot. The controls -- 33 deleting `getout(0);` too, 38a deleting
`full_screen = FALSE;` with the ladder, neither seen by an assertion -- are
each named: `phase 33 gives 77498 lines where q033.c holds 77499`, `phase
38 gives 78188 lines where q038.c holds 78189`, `2 of 103 phases do not
reproduce their snapshot`. `whim-editor-check`, `whim-test` (80 cases,
the Go editor all 80), `go test ./...` here and in `crefactor/`, and
`TestPhasesOnGraph`/`TestGraphSnapshots` pass.

**Measured** (5 runs each, medians, load 3-9; the text in, the graph
imported, as a run in order does; and handed q(N-1)'s graph read back, as
the parallel check does, the read apart):

| phase | text before, wall / CPU ms | imported, wall / CPU ms | handed the graph, wall / CPU ms |
| --- | ---: | ---: | ---: |
| 33 | 2,251 / 4,358 | 2,112 / 4,225 (import 1,757, C view 67, collection 121) | **366 / 562** |
| 38 (38a on the graph, 38b and 38 text) | 2,077 / 3,443 | 3,839 / 7,138 (import 1,877, C view 86) | 1,966 / 3,255 |

- **33 begins and ends on the graph**: an import where 32 hands it text
  (1.6-1.8 s, about the sweep and print it no longer runs), 0.37 s handed
  the graph, of which the program's own two file views are about 0.14 s.
  Once 32 ends on the graph (B3b) it imports nothing.
- **38 is 1.8 s slower until 38b and 38 move (B3c)**: it imports for 38a
  and prints the C view for 38b, then sweeps as before -- the cost
  *Converting a phase* warns of, taken because 38a is this batch's and
  38b's FRAG/RENAME is B2's. In the parallel check it reads q037.g and costs
  what it did.
- **Boundaries**: two imports added to the run in order (q032, q037), one
  C view inside phase 38 (to 38b), and two graph snapshots (q032.g,
  q037.g, 6.3-6.5 MB each). Phase 33's collection replaces its sweep.

**Refinements to the catalogue.** Both rows were (a) and TEXTQ as said,
and needed nothing else: no BUILD (33 deletes, the call to `getout` keeps
its node), no FOLDX. A text literal of several statements is a run of
items, which a verb could say (`Run(pats...)`: the first found once, the
rest its siblings), and "the writes of x" by edge is a TEXTQ question
other phases will ask (`Writes(decl)`): both are written locally here and
left for B2 to lift if a second phase wants them. A converter of a phase
with several steps should expect 38's cost until its last text step moves.
## B2a as built (2026-10-03): FRAG, CLONE, MACROX

`crefactor/graph`'s `frag.go` (FRAG), `fragverbs.go` (its verbs and
`Together`), `clone.go` (CLONE), `macrox.go` (MACROX), `same.go`
(`SameGraph`, what the tests hold FRAG to), and `crefactor/clisp`'s
`printnode.go` (`PrintExpr`, `PrintItems`); generic, naming nothing in
vim. Changed beside them, each additively: `import.go` (`ImportParsed`'s
body is `importAST`, which keeps the importer: what each node came from),
`verbs.go` (a `batch` field, carried into a scope), `build.go` (a hole used
twice is a `Clone`, no longer a refusal; two messages say frag.go) and its
test's refusal row.

### How a fragment gets its names and types in context

There is one authority on how cc resolves and types C, the importer on cc's
check, so FRAG runs it on a **synthesized translation unit**: the graph's C
view with each fragment's text written at its spot, pared to what a
fragment can see. Every function's body is left out but the ones a
fragment goes into; a definition that a prototype declared first is left
out when nothing the unit resolves comes after it; a table's rows are left
out, its length written where its initialiser said it (from the def's typed
edge). Everything else stays, in its order: the includes, the types, the
prototypes, the objects, so that every name is declared where it was and
cc resolves the fragment's names exactly as in the file -- locals and
parameters of the enclosing function (printed whole), the file's
declarations, members by the type selected from, labels, tags, the
headers' declarations, builtins, macro invocations. On q021 the unit is
217 KB of the file's 2.18 MB.

A fragment of items sits between two marker statements
(`__whim_frag_mark;`, declared at the unit's top), external declarations
between two marker declarations; an expression or a body takes its
node's own place. The unit is parsed, checked and imported; the import and
the graph are walked side by side, every context node mapped to the node it
is a printing of, and the markers say which nodes are the fragment's. Those
are kept, their ids cleared (the splice gives fresh ones: the id rule), and
their edges carried over: into the context to the mapped node; to a
header's declaration to this graph's external node, added if it has none
(`strlen`, `errno`, a member of `struct stat`); typed edges to this graph's
type node of the same structure, added if it has none. A macro's
invocation is the node the importer makes, a `(macro "...")` with an edge to
every name its expansion uses (MACROX's first half: `va_arg`,
`__builtin_offsetof(struct win, n)` to struct win's `n`, `FD_SET`).

The graph after the splice is **what an import of its C view gives**,
uses outside the fragment included: a later use that a new local now
shadows, a goto to a label the fragment brings, the calls of a function a
top-level fragment declares anew or redefines are retargeted, as the
synthesized import resolves them (for a top-level fragment declaring a name
the file has, the unit is made again with every form using it printed
whole); and the expressions above an expression's spot are typed as the
import types them, not cleared. `SameGraph(a, b)` (ids aside: every refers
edge to the corresponding node, every typed edge to a type of the same
structure) is how the tests hold it.

**cc's rule is not "the first declaration".** `crefactor/cc`'s
`Scope.ident` takes the first visible declaration, unless that is a
prototype naming no parameter and a definition naming them is visible: then
the definition. So a use after `deathtrap`'s definition refers to the
definition, one before to its prototype (phase 39's host block found it).
The importer and FRAG say this; BUILD's templates (B0) still resolve to the
first declaration, which differs from the import there.

### The API later batches call

```go
e := v.Editor()
// spots: where a fragment goes
e.SpotOf(n)              // n's place: an item (replaced by items), an expression or
                         // initialiser element (one expression), a body or an else (a block)
e.SpotRun(first, last)   // a run of items
e.SpotBefore(at), e.SpotAfter(at)  // an insertion beside an item (a top-level form too)
e.SpotBody(fn)           // a function's items
e.SpotEnd(p)             // after p's last item; p nil: the end of the file
// fragments, one unit for all of them
ns, err := e.SpliceC(graph.Frag{At: e.SpotBody(fn), Src: body},
	graph.Frag{At: e.SpotOf(x), Src: "2 * $x", Holes: graph.Bindings{"x": x}})
e.MakeC(fs...)           // the nodes, not placed (no retargeting): for a caller that places them
graph.Clone(n)           // CLONE: a copy without ids, its inner edges to the copies
graph.MacroCall(n)       // `(macro "MIN(a, f(b, c))")` is MIN, [a, f(b, c)]
e.ExpandMacros(ns, func(name string, args []string) (string, error) {...})
graph.SameGraph(a, b)    // a graph against the import of its C view, ids aside
```

`$name` in the C is a hole, the node bound to name (a pattern's binding),
moved in: it must be in what the fragment replaces, or a copy. It is
written into the unit as its own C in parentheses, so cc types the fragment
with its type, and the node replaces what those parentheses made; the C
view writes the parentheses an operator needs (`$x * 2`, x being `a + b`, is
`(a + b) * 2`; a text splice would have written `a + b * 2`).

**The verbs**, each act's matches in one unit, counted and reported as
B0's:

| verb | does | the text's |
| --- | --- | --- |
| `BodyC(fn, src, what)` | a function's whole body | `Body` |
| `LiteralC(old, new, n, what)` | each of the n runs of whole items whose C is old (spacing aside, outside literals) replaced by new | `Literal` (a match of part of an item is not a run: the count says so) |
| `ReplaceC(pat, src, n, what)`, `ReplaceAtC(pat, at, src, n, what)` | each match (or its node bound to at) replaced by C, the bindings as holes | `Sub`, `Literal` |
| `BeforeC`, `AfterC(pat, src, n, what)` | items put beside each match | a `Literal` that keeps its anchor |
| `SpliceRunC(from, through, src, what)` | a run of items, each end a pattern | `Splice` |
| `TopBeforeC`, `TopAfterC(name, src, what)` | external declarations before the first, after the last, top-level declaration of name (`struct T` for a tag's definition) | a file-level `Literal` |
| `ExpandMacros(names, expand, n, what)` | the scope's invocations of the macros named, expanded | phase 42's text expansion |
| `Together(acts)` | the FRAG acts among acts deferred to ONE unit: a phase's literals for the cost of one; each finds and counts its matches on the graph as the batch found it, the deferred acts reported at the end in order, a refusal naming its act | -- |

### What the tests prove

`crefactor/graph`'s `frag_test.go`, on graphs read back from their Lisp,
each result held to the text verb's C printed canonically where there is
one, and every result to the import of its C view (`SameGraph`, no edge
dangling, `Check`):

- a whole body naming a parameter, a local, the file's objects, members by
  type of two structs sharing names, a label and its goto, `errno`, a
  header's function, `__builtin_offsetof`; fresh ids above the graph's
  greatest;
- a run of items replaced whose new local shadows the file's object for the
  use after the run (retargeted), and items inserted;
- expressions with holes: the parentheses kept and dropped, a hole used
  twice (a copy), the expressions above typed as imported;
- external declarations: a new prototype that takes the calls of the old
  one, a new type (`char ***`), new externs (`strspn`);
- several fragments in one unit, two in one list, a body's place;
- macros: invocations made as imported (`va_arg`), MIN and MAX expanded;
  `MacroCall` on nested commas and strings;
- cc's rule: a use after a definition refers to it; a definition replaced
  by a top-level fragment takes the uses of the old one;
- `Together`: the same C and report as the acts one by one; a refusal
  naming its act;
- the refusals (a name declared nowhere, a parse error and a check error at
  the fragment's line, a `#` line, an unbound hole, a fragment that does not
  stand in its place), each leaving the graph its import; CLONE's edges;
  BUILD's hole used twice;
- the control: one edge moved to another declaration of its name, and
  `SameGraph` says so.

`internal/graphcheck`'s `TestFragOnSnapshots` (GRAPH_SNAPS), literals read
from the phases' own `editlit.go`/`edit.go` (go/parser, not retyped), on
q(N-1)'s graph read back, held to the text program's act printed
canonically **byte for byte**, and to the import of the result
(`SameGraph`):

| case | what | on | FRAG, median of 5 (load 45-62) | the file's import |
| --- | --- | --- | ---: | ---: |
| body/68 | `syn_name2id_len`'s body, spaced by hand | q067 | 252 ms | 1.9 s |
| body/61 | `buflist_findnr`'s body | q060 | 255 ms | 2.3 s |
| run/22 | `:edit`'s 45-line block in do_ecmd: `stat()`, `stat_T` and `st_dev`/`st_ino` of the header's struct, a `dev_t` cast, `goto theend` | q021 | 256 ms | 2.9 s |
| together/21 | phase 21's ten literal acts (2 bodies, 8 runs in 6 functions) in one unit | q020 | 301 ms | 2.4 s |
| top/39 | the host block: 16 definitions and objects, `FD_ZERO`/`FD_SET`/`FD_ISSET`, `errno`, `SIG_IGN`, `struct termios`, `sig_atomic_t`, `fd_set`, libc | q038 | 308 ms | 2.1 s |
| builtin/38 | the launcher: `__builtin_setjmp`, `__builtin_longjmp` | q038 without it | 287 ms | 2.0 s |
| macros/42 | 7 MIN and 16 MAX expanded to the header's text (`steps.MinMax`) | q041 | 394 ms | 2.1 s |
| refused/72 | `ml_new_data`'s body before the member it names exists: refused, *frag 1 line 16:7: ... has no member named bh_data* | q071 | -- | -- |

A fragment costs about an eighth of an import of the file, and a batch
about what one fragment costs (together/21: ten acts, 301 ms). Before the
unit was pared (bodies only), it was 419 KB on q021 and FRAG 400-460 ms.
`whim-build-check` (parallel) is unchanged: B2a converts no phase.

### Limits

- **Holes are expressions.** A statement or a run moved into a fragment is
  MOVE's (B2c); a fragment cannot be a member or an enumerator (INITROW,
  RENUM: B2b) or hold a `#` line (INCLUDE: B2e).
- **What is retargeted** is what the unit resolves: uses in the forms
  printed whole -- the function a fragment goes into, and for a top-level
  fragment declaring a name the file has, every form using it. A top-level
  fragment defining a struct, union or enum tag that the file uses as a
  header's (an `extern-struct`) does not move those uses to it. `MakeC`
  retargets nothing.
- **cc's check is asked of the fragment's lines only**: a conflict it
  reports at a line of the context (a redeclaration reported at the later
  declaration) is not a refusal. A fragment's expression cc left untyped is
  in `Untyped`, as BUILD's are.
- **A splice the editor refuses midway** (a body place given more than a
  block) leaves the splices before it made, as a run of edits does; the
  type and external nodes added stay, for the collection.
- **`LiteralC` matches whole items**; a text `Literal` that matched part of
  a line has no run, and its count refuses.
- **The unit is printed, parsed and imported each call** (0.25-0.4 s here,
  a fifth of the file or less, the system headers' parse about 50 ms of
  it); `Together` and `SpliceC(fs...)` are how a phase pays it once.
## B2d as built (2026-10-03)

`crefactor/graph`'s `foldx.go` (`Editor.FoldX`, the closure), `foldx_eval.go`
(cc's constant evaluation on the forms, type layout, the parentheses C
needs), `unwritten.go` (`Editor.Unwritten`, the seed) and `foldmore.go`
(`EmptyBlocks` and the closure's opt-in rules), and `FallOutOptions`' second
half in `fallout.go` -- ADDITIVE: every new field is off by default, so the
closure of the phases already on the graph is what it was (the build check
holds them). 1,304, 858, 620 and 276 lines of Go, comments and blank lines
aside. Beside them `internal/steps/frontcut.go` (`FrontCut`: a front
phase's cuts without the closure their step wraps them in) and the tests.
Generic: what a program must say -- its roots, the names a phase folds by
hand, a caller's own test of a condition's text -- is an option.

### The rules

| rule | does | option | the text's |
| --- | --- | --- | --- |
| the unwritten seed | what nothing writes, by write edges -- a use in a store's left side, `++`, `--`, counted by the name it spells in the code the roots reach and the file's initialisers: a file-scope scalar, defined, address never taken, a constant initialiser at most; a member no live code stores, steps, takes the address of or designates, of a struct every instance of which is a static object without an initialiser reached through pointers from it, a null constant or a zeroing memset (the conversions asked of the typed edges) | `Editor.Unwritten(roots)`; `FoldX{Before, Hold}`: the seeds are what is unwritten now and was not before the cut, to a fixed point over what the folds leave unwritten in turn | `FallOutOf`'s `unwrittenNames`, `unwrittenObjects`, `unwrittenMembers`, `unsafeOwners` |
| reads of objects / members nothing writes | each read its value (a member's 0); a pointer's `((void *)0)`, only where tested | (the seed) | `seeds` |
| reads of locals so initialised | a local initialised with a value the closure wrote, written never again, is that value | `Off: locals` | `locals` |
| constant expressions | an integer expression holding a written value and constant with it is its value: cc's evaluation (each operand converted, unsigned wrapping, `0 && f()` false, a store's value, `sizeof`/`_Alignof` by the layout, `offsetof` by the member its invocation refers to, the standard headers' limits), so `!K` and `K == 0` too | -- | `constExprs` |
| `a && K`, `a \|\| K` | K the identity goes where only the truth is read (or the other operand is a truth); K absorbing is the result, the other operand pure | `Off: logic` | `logic` |
| ifs, whiles, `?:` of a constant | the branch taken: a braced branch alone in a block spliced (whole where it declares), none taken gone, an else-if taking none gone from its chain, `?:` its branch in parentheses; not where a branch holds a label or a case | -- | `branches` |
| statements after a revealed jump | up to the next label, but a run holding a declaration | `Off: jumps` | `deadAfterJump` |
| calls of functions left empty | a static void function only called, left with nothing to do, is called no more | `Off: empty` | `emptyFunctions` |
| calls of functions returning a constant | a static function only called whose body is `return K;`, K written, is K at each call whose arguments are pure | `Off: returns` | `constReturns` |
| a parameter every call passes alike | that value in the body, gone from the definition, its prototypes and every call; one a round | `Off: params` | `constParams` |
| a cut's own writes | the literals a cut wrote count as the closure's: `return 0;` made by a cut is 0 at the calls, with no seed | `FoldX{Marks, NoSeeds}` | -- |
| empty blocks everywhere, and dead locals | in the forms named, an if with an empty block, no else, a pure condition goes; an empty else goes; an empty else-if ending its chain goes; alternating to a fixed point with: a local only ever given a value (every use `x = E;` alone, E and its initialiser pure, no other mention of its name after it) goes with its stores | `Editor.EmptyBlocks(EmptyOptions{In, Cond})`, or `FallOutOptions.Empties` | `EmptyBlocks` (phase 62), `edit.DeadStores` |
| labels | a label an edit took the last goto of goes | `Labels` | the gotos' steps (89, 90) |
| a condition kept | an if whose only branch an edit emptied, its condition doing something, is `(void)E;`, a `!` taken off | `KeepCondition` | notags' two literals |
| store ifs | an if whose only branch held only stores the store rule took, its condition pure, goes, KeepEmpty or not | `StoreIfs` | -- |
| values | a read of an object a cut deleted is the value the cut gives it, not its initialiser's | `Values` | utf8only's flags |
| the walk fold, the declaring splice | B0's: `Verbs.FoldWalk`/`FoldWalks`, `Unwrap` and `SpliceDeclaring` | | |
| statements after ANY jump | 86a's, B1c's (`Terminates` on nodes); FoldX's is the revealed one | | `DeadStmt` |

`FallOutOptions.X` runs FoldX after the closure's own rules, and `Empties`
EmptyBlocks last; the stats say what each did (`X`, `Empty`, `Kept`).

### How it is the text's, byte for byte

- **Marks.** The text marks what it wrote with enumerators (`fallout_N`)
  and the branches it took with comments, and its rules after the seeds
  fire only in the functions holding one. FoldX keeps the values' nodes and
  puts MARK items where the comments stood (one before a spliced branch, one
  in place of an if taking nothing, an `(empty)` after it where a label held
  the if, the mark of an else arm gone at its chain's end), asks "marked" by
  containment and "a mark right before" by the item before, and takes the
  marks out when done. A round finds every rewrite, outermost first, none
  inside another -- the text's overlap of spans as the graph's containment,
  a run's prefix (`(&& a b c)` is cc's `(a && b) && c`) a virtual node --
  and then makes them.
- **The parentheses C needs** are not in the forms; the C view writes them.
  The text rewrites the expression inside them and keeps them, and moves an
  operand with the ones it stood in: so a replacement of what the printer
  parenthesises is `(paren ...)`, and so is a moved operand that needed
  them -- but for the constant rule, which takes cc's node for the
  parenthesised expression whole (`(a && 1)`, not `(a && (1))`).
- **cc's tree is flattened**: an operand is the node that says something,
  so `isTruth` of a parenthesised operand is false, whatever is inside.
- **cc's evaluation**, not the forms' arithmetic: the operand types by C's
  conversions (a literal's by its suffix and size, an enumerator int, a
  cast its form's, through typedefs to their typed edges), `nullptr` a
  pointer, `true`/`false`, the header limits (`SIZE_MAX`, `INT_MAX`, `EOF`:
  atoms with no edge, LP64 values), a store's value as its left side's
  type, the comma's last, `sizeof` and `offsetof` from the layout.
- **The text's parameter order is Go's map order.** It drops one parameter
  a round, the first its map yields: on phase 3 its report says 19 to 23
  parameters, run after run, with the same bytes. FoldX takes the file's
  order. The texts before the sweep then differ, in functions the sweep
  takes (a parameter dropped from a function whose last call went the same
  round); after it they are the same. Where the text would drop a parameter
  and refuse its own next round (a use of it left, a call still passing
  it: "does not type-check"), FoldX refuses that round, naming it.
- **EmptyBlocks' condition test** is the closure's "free of side effects";
  the text's is a regexp on the condition's text (`edit.PureCond`), which
  refuses `regname == '='` for its `=`. `EmptyOptions.Cond` is a caller's
  own test of the C text (`ExprText`): phase 62 on the graph passes
  `edit.PureCond` and is q062 byte for byte; without it, it is not.

### What the tests prove

`crefactor/graph`'s `foldx_test.go`, on graphs read back from Lisp: 13
samples through `xform.FallOut` (every seed) and through FoldX, the same C
byte for byte -- seeds in ifs and in `&&` chains, a predicate, a parameter,
a revealed jump, locals, pointers, members, unsigned arithmetic, labels and
cases, the parentheses, the truths, sizes, `offsetof` and `SIZE_MAX`; a
seed taken from a cut (`Before`); `EmptyBlocks` against
`xform.EmptyBlocksRule`, the C and its numbers; the opt-in rules with and
without their options; a cut's marks with no seed.

`internal/graphcheck`'s `foldx_test.go`, on the snapshots (GRAPH_SNAPS):

| test | holds | result |
| --- | --- | --- |
| `TestFoldXFront` | each front phase's cuts run as text on q(N-1), then the text's `FallOutOf` and FoldX (seeded with q(N-1)'s graph's Unwritten, the step's Hold) on the cut's graph read back, each swept | **phases 1, 2 and 3 byte for byte**, the report line for line but the parameter count and the rounds; phase 2 is q002.c; the control (`Off: logic`) moves phase 2 |
| `TestFoldXEverySeed` | `xform.FallOut` on q040, q070, q103 and FoldX{}, each swept (before q040 the text refuses itself: "undefined: fd") | **3 of 3 byte for byte** |
| `TestEmptyBlocksPhase62` | EmptyBlocks on q061's core (the forms above the first include) with `edit.PureCond`, collected | **q062.c byte for byte**, 49 blocks and the 5 locals in the text's order; the control (no `Cond`) moves it |
| `TestKeepConditionNoTags` | notags' two calls cut from their ifs on q002's graph, the closure with `KeepCondition` | `nv_help` and `nv_tagpop` are the text cutter's; without it, the emptied ifs stay |

**Measured** (the 64-core machine, load 5-15): the front's closures, the
text's 18.3, 9.8 and 16.7 s (two analyses' parses and a type check every
round) against FoldX's 2.2, 1.2 and 1.6 s on the graph; `xform.FallOut`
with every seed 7.7-9.0 s against 0.67-0.77 s.

### What B4 and step 6 still need

B4 can end a front phase on the graph with `FoldX{Before: e.Unwritten(nil)
asked before the cuts, Hold: frontHold}` once its cutters are on the graph:
the closure is the text's on all three. What it does not have:

- **The cutters** (47, `internal/cut` and the parts): FRAG, INITROW, MOVE,
  RENAME, RETYPE, PARAM (B2a-c) and the verbs.
- **Typed edges on what the cutters build.** FoldX reads types: every
  declaration's (an object's to be a seed, a pointer seed only where
  tested), a member's, a typedef's (a cast's integer type), and the
  expressions' a conversion asks (`=`, casts, a call's result and the
  callee's parameter types, a return, an initialiser) for a member's
  struct to stay safe. Where an edit cleared them it derives what a form
  says (a parenthesis, `?:`, a comma, a store, a call, `*`, `[]`, a
  member) and nothing else: a new arithmetic or cast expression from FRAG
  or BUILD without its typed edge counts as no conversion. Step 6's checker
  re-deriving `Untyped` makes that exact; until then FRAG must type what it
  makes, as the importer does.
- **The text's refusal.** `FallOutOf` type-checks its text every round and
  dies on what does not check; the graph has no checker (step 6): FoldX
  refuses only a dangling edge or a call passing a parameter it dropped.
- **Ids.** The parameter rule rebuilds a function type's parameter list and
  each call (new ids for the list and the call, the rest moved): B2c's
  PARAM, deleting a parameter in place, would keep them.
- **Phase 1's rows**: the test sets `REMOVED` from `internal/phase/001/
  delta.md` as the build's `Declared` does; B4's graph step needs the same.

## The classes

- **(a)** expressible with G0 and the B0 library (VERBS, BUILD, TEXTQ).
- **(b)** needs a capability of B2: named in the `needs` column.
- **(c)** a typed transform: it calls `cc.Translate` -- step 6, on typed
  edges. There are only five: the front's `FallOutOf` wrapper
  (`xform/fallout.go`, phases 1-3), `memberout` (94), `stateparam` (95),
  `localout` and `structscalar` (100), and `plainc`'s three (101). Several
  that live in `crefactor/xform` are NOT typed -- `NullptrUsize`, `Attrs`
  (0a, 0c), `Own` (47) and `Unions` (50) (on the graph since B3d), `DropCalls` (60), `EmptyBlocks` (62),
  `NeverNull` (74), `DeadStmt` (86a, on the graph since B1c), `BoolRet` (87a, 102, 103) and the four
  goto steps (89-92): they are text or `cc.Parse` only, so they are (a) or
  (b). (60, 62, 74 and 89-92 are on the graph since B3g, their text
  versions gone.)
- **(d)** stays text: the seed (phase 0: it runs on the input before any
  graph, its work is spelling, and its proof is an identical binary; the
  first import is of q000 or later), and phase 88's `includes` (judged by
  gcc compiling each trial). Phase 43 as built asked gcc three questions the
  graph answers by edges (names the headers still supply, definitions whose
  every use is below the cut, the core's undefined interface): (d) as it
  stood, rewritten on the graph by B3d. Phase 42's `MIN`/`MAX` expansion reads the
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
phase 1 | 1 | front + noruntime | - | - | done | - | B4 | the front's D1-D5 on the graph, FoldX their closure; imports q000 (phase 0 ends on text)
FallOutOf | 1-3 | crefactor/xform/fallout.go; steps.go frontHold | 1686 | - | done | - | B4 | replaced by FoldX (B2d) on the graph; FallOutOf deleted, FallOut kept as FoldX's tests' oracle
argvfront | 1 | cut/argvfront.go | 99 | - | done | - | B4 | command_line_scan's new body, main_errors rows
exfront | 1 | steps.go exFront + cut/retire.go + phase/001/delta.md | 22+37 | - | done | - | B4 | 489 cmdnames rows' handler to ex_ni, found by name string
extable | 1 | cut/extable.go | 270 | - | done | - | B4 | shortest-abbreviation lookup, prefix index gone; the hardest front unit; after exfront
filefront | 1 | cut/extable.go | 41 | - | done | - | B4 | 13 file-command rows; enumerators moved after CMD_SIZE
quitfront | 1 | cut/extable.go | 12 | - | done | - | B4 | one FoldNever
readfront | 1 | cut/extable.go | 15 | - | done | - | B4 | two FoldNever
onecmdfront | 1 | cut/onecmdfront.go | 45 | - | done | - | B4 | condition shrinks (a), two new small bodies
optfront | 1 | cut/optfront.go, droprow.go, optfront.md | 112 | - | done | - | B4 | 375 options[] rows and their names in string lists
noswap | 1 | cut/noswap.go | 69 | - | done | - | B4 | 4 bodies stubbed, an else-if arm
norecover | 1 | cut/norecover.go | 123 | - | done | - | B4 | recovery cut; add_time's body flattened (the declaring splice: B0's Unwrap)
nomemfile | 1 | cut/nomemfile.go | 241 | - | done | - | B4 | mf_open's parameters, two bodies, lalloc's retry block
noruntime | 1 | cut/noruntime.go | 34 | - | done | - | B4 | runtime strings to "", vimruntime = FALSE
phase 2 | 2 | front2 + query-empty | - | - | done | - | B4 | D6-D8 on the graph, handed phase 1's; query-empty asks the C view
nolocale | 2 | cut/nolocale.go | 66 | - | done | - | B4 | a call statement, 3 table rows
nostartup | 2 | cut/nostartup.go | 42 | - | done | - | B4 | a body emptied, 2 calls
nocmdopts | 2 | cut/nocmdopts.go | 51 | - | done | - | B4 | 3 ifs, a member (after the ifs)
nosession | 2 | cut/nosession.go | 123 | - | done | - | B4 | 18 stubs, 6 DropIf, a get_varp case
whim18 | 2 | phase/018 (Edit) | 44 | - | done | - | B4 | shell redirection and runtime completion
noenc | 2 | cut/noenc.go | 191 | - | done | - | B4 | mb_init's dispatch, 6 iconv stubs
nofencs | 2 | cut/nofencs.go | 23 | - | done | - | B4 | an else-if arm
nofenc | 2 | cut/nofenc.go | 113 | - | done | - | B4 | 'fileencoding' reads and stores
utf8only | 2 | cut/utf8only.go | 491 | - | done | - | B4 | five encoding flags constant, every test folded: DRAFT (its text simplifier on the C view, committed as FRAG), the one front unit not on the verbs
noterm | 2 | cut/noterm.go | 57 | - | done | - | B4 | "xterm" -> "xterm-256color"
nomouse | 2 | cut/nomouse.go | 211 | - | done | - | B4 | 22 rows retargeted (a), 14 key rows, a body inlined
whim2a | 2 | phase/002/a | 52 | - | done | - | B4 | no title
query-empty whim2 | 2 | phase/002 + steps.go | 43 | - | done | - | B4 | asserts 24 rows are ex_ni
phase 3 | 3 | front3 + nointro + optreaders | - | - | done | - | B4 | D9-D12 on the graph, handed phase 2's; hands phase 4 the graph
noinert | 3 | cut/noinert.go | 69 | - | done | - | B4 | :browse :confirm :behave
notabs | 3 | cut/notabs.go | 232 | - | done | - | B4 | one tab page; 4 rewritten cases
noarglist | 3 | cut/noarglist.go | 30 | - | done | - | B4 | completion of the arglist
nowindows | 3 | cut/nowindows.go | 328 | - | done | - | B4 | one window: 24 FoldNever, arms, stores
nowinsizes | 3 | cut/nowinsizes.go | 135 | - | done | - | B4 | 9 initialisers added
nobuflist | 3 | cut/nobuflist.go | 65 | - | done | - | B4 | :bnext/:bprevious only
nonfa | 3 | cut/nonfa.go | 74 | - | done | - | B4 | vim_regcomp's tail
noshellout | 3 | cut/noshellout.go | 39 | - | done | - | B4 | 3 bodies
notags | 3 | cut/notags.go | 53 | - | done | - | B4 | `if (!f()) {X}` -> `(void)f();`
nosignals | 3 | cut/nosignals.go | 132 | - | done | - | B4 | signal_info[] to 6 rows, a block
noequiclass | 3 | cut/small.go | 33 | - | done | - | B4 | [= classes
nocindent | 3 | cut/nocindent.go | 145 | - | done | - | B4 | 'cindent'
noucmd | 3 | cut/noucmd.go | 80 | - | done | - | B4 | user commands
noident | 3 | cut/noident.go | 148 | - | done | - | B4 | nv_ident's 90-line body (maybe deletions: check)
nofnamemod | 3 | cut/nofnamemod.go | 53 | - | done | - | B4 | % modifiers
nocompl | 3 | cut/nocompl.go | 107 | - | done | - | B4 | 9 stubs
nocomplkeys | 3 | cut/nocomplkeys.go | 309 | - | done | - | B4 | 12 stubs; gotos kept by moved nodes
noabbr | 3 | cut/noabbr.go | 97 | - | done | - | B4 | abbreviations
3a | 3 | phase/003/a | 56 | - | done | - | B4 | lisp
3b | 3 | phase/003/b | 69 | - | done | - | B4 | langmap
3c | 3 | phase/003/c | 53 | - | done | - | B4 | jump list
3d | 3 | phase/003/d | 41 | - | done | - | B4 | rot13
3e | 3 | phase/003/e | 49 | - | done | - | B4 | sentence motions; 2 string respells
3f | 3 | phase/003/f | 33 | 3206 | done | - | B4 | file marks: 2 bodies
nointro | 3 | cut/small.go | 11 | - | done | - | B1a | 2 calls cut, counted
optreaders | 3 | cut/optreaders.go | 57 | - | done | - | B1a | a statement found by its string; the leftover check TextCount; phase 3 ends on the graph
phase 4 | 4 | 13 steps, 1 sweep | - | - | done | - | B3a | graph end to end, handed phase 3's graph; the in-phase sweep a collection; no import
nostat | 4 | cut/nostat.go | 32 | - | done | - | B1a | check_timestamps returns 0 (Body); phase 4 begins on the graph, handed phase 3's
4a | 4 | phase/004/a | 72 | 480 | done | - | B1a | AUTOMATIC_ENGINE retries; the one-engine proof the text's code on the C view
onebuffer | 4 | cut/onebuffer.go | 176 | - | done | - | B3a | DropCaseRun, DropIf, FoldNever, DropOperand, Rewrite, the CTRL-^ row RewriteAt; the leftover counts the text's on the C view
4b | 4 | phase/004/b | 70 | - | done | - | B3a | 18 key_names_table rows deleted by name (INITROW), the [MOUSE] row, win_line's parameter (PARAM)
4c | 4 | phase/004/c | 53 | - | done | - | B3a | Splice, Rewrite, the bodies by template; no FRAG
4d | 4 | phase/004/d | 80 | 8385 | done | - | B3a | FoldWalk x7, 3 bodies and 4 address arms in two FRAG units (Together), the list members cut
4e | 4 | phase/004/e | 26 | - | done | - | B3a | an arm folded, check_tty's parameter (PARAM); counts the text's on the C view
nowild | 4 | cut/small.go | 24 | - | done | - | B3a | the prototype and 3 calls in one FRAG unit
nowildmenu | 4 | cut/nowildmenu.go | 252 | - | done | - | B3a | 23 acts; showmatches' 2 parameters (PARAM, Dangle for the unread locals); report at the end in the text's order
4f | 4 | phase/004/f | 84 | - | done | - | B3a | DropCaseRun, Run, SpliceFirst; 22 options[] callbacks to nullptr (Replace)
phase 5 | 5 | 12 steps, 1 sweep | - | - | done | - | B3a | graph end to end, handed phase 4's graph; the in-phase sweep a collection; no import
nobackup | 5 | cut/nobackup.go | 158 | - | done | - | B1a | backups and ACLs; quiet acts, a line a group; reproduces the text's bodiless-if accident (B1a as built)
lfonly | 5 | cut/lfonly.go | 310 | - | done | - | B1a | LF only; the dying functions' callers asked of the edges
keepbytes | 5 | cut/keepbytes.go | 98 | - | done | - | B1a | ++bad gone; KeepThen on a chain; phase 5 begins on the graph and imports again after noconv
noconv | 5 | cut/noconv.go | 290 | - | done | - | B3a | HeadFold (heads, arms apart, last first); 386 callees through the mb_* pointers replaced by the functions, by edge; the startup loop FRAG; EndLabels
5a | 5 | phase/005/a | 201 | - | done | - | B3a | 77 acts on the verbs; comp_textwidth's flag (PARAM, also at format_lines' dead call, which the text left ill-typed for the sweep); CutRun
5b | 5 | phase/005/b | 132 | 3511 | done | - | B3a | FoldWalks by pattern (a repeated ?name is the backreference), 9 bodies in 3 FRAG units, firstwin/lastwin's 35 uses RetargetUses to curwin
5c | 5 | phase/005/c | 50 | 4353 | done | - | B3a | 15 bodies in one FRAG unit
5d | 5 | phase/005/d | 112 | 4261 | done | - | B3a | the proof by edges (no store to first_autopat), 2 FRAG literals, 56 dispatches cut by callee
noglob | 5 | cut/small.go | 27 | - | done | - | B3a | a one-call body (BUILD)
phase 6 | 6 | nofind | 45 | - | done | - | B3a | the delegation by edge, the body FRAG; graph end to end
phase 7 | 7 | noinertopts, sweep, droplocal | 30 | - | done | - | B1a | graph end to end: a FoldNever, a RewriteAt, a Cut; the sweep a collection
phase 8 | 8 | droplocal | - | - | done | - | A | begins on the graph (q007.g)
phase 9 | 9 | nohome, nogetenv | 138 | - | done | - | B3a | two bodies FRAG, a body BUILD, cuts and drops; graph end to end
phase 10 | 10 | nochdir | 112 | - | done | - | B3a | two bodies in one FRAG unit (static locals, getcwd, errno); graph end to end
phase 11 | 11 | nofloat | 97 | - | done | - | B1a | the conversion a Splice, the arms DropCase; literals and libm calls counted on the C view
phase 12 | 12 | noowner, droplocal | 123 | - | done | - | B1a | handed the graph: DropOperand, FoldAlways, DropIf, FoldAlwaysElse, `(void)f()` by Rewrite
phase 13 | 13 | sweep, droplocal | - | - | done | - | A | begins on the graph (q012.g)
phase 14 | 14 | sweep, droplocal | - | - | done | - | A | begins on the graph (q013.g)
phase 15 | 15 | whim15a, whim15 | 37 | - | done | - | B1a | handed the graph: an nv_cmds element RewriteAt; :noswapfile's DropCase
phase 16 | 16 | oneoptset, sweep, droplocal | 142 | - | done | - | B3a | the string RespellString, three arms folded, DropOperandAsText; graph end to end
phase 17 | 17 | droplocal | - | - | done | - | A | begins on the graph (q016.g)
phase 18 | 18 | droplocal | 5 | - | done | - | B1a | whim18kp gone: DropLocal's get_varp rule takes `&curbuf->f` unparenthesised now
phase 19 | 19 | whim19, sweep, droplocal | 73 | - | done | - | B1a | handed the graph; whim19ep gone as whim18kp
phase 20 | 20 | whim20, sweep, whim20bl, droplocal | 115 | - | done | - | B1a | handed the graph; whim20bl a DropCase (b_p_bl is the collection's, not droplocal's)
phase 21 | 21 | whim21 | 59 | 2403 | done | - | B3a | 8 literal acts in one FRAG unit, ## by BUILD, runs and a member cut
phase 22 | 22 | whim22 | 38 | 3147 | done | - | B3a | the two block literals in one FRAG unit; Run, CutRun
phase 23 | 23 | whim23 | 21 | - | done | - | B1a | one FoldNever; the row query TextQuery on the C view
phase 24 | 24 | edit whim24 | 188 | - | done | - | A | begins on the graph (q023.g)
phase 25 | 25 | whim25 | 129 | 961 | done | - | B1a | handed the graph; its own DropOperand keeps the text's parentheses; one MIN() macro's text respelled (MACROX's)
phase 26 | 26 | whim26 | 262 | 450 | b | RENAME TEXTQ | B3a | the Ex table: enumerators last-first, a string respelled
phase 27 | 27 | whim27 | 346 | 1992 | done | - | B3b | no Ex mode: the folds by place (FoldNeverAt/FoldAlwaysAt), main_loop's parameter PARAM's; begins on the graph where 26 ends on it
phase 28 | 28 | whim28 | 67 | 850 | done | - | B3b | no :write: RENUM (Renumber), RespellString, a Cut; the residue on the C view
phase 29 | 29 | whim29 | 115 | 638 | done | - | B3b | no :read: RENUM, DropOperand and the operand moved out of its parentheses
phase 30 | 30 | whim30 | 131 | 844 | done | - | B3b | no :edit, gf: RENUM, two DropCase, a fold by condition
phase 31 | 31 | whim31 | 102 | 569 | done | - | B3b | open_buffer(void): PARAM's three parameters in one edit
phase 32 | 32 | whim32 | 476 | 5665 | done | - | B3b | the buffer has no name: CutRun, PARAM (Dangle for the local the text left naming a gone parameter), RENUM; the writes on the C view
phase 26 | 26 | whim26 | 262 | 450 | done | - | B3a | HeadFold, DropOperandAsText in their conditions, the dead ADDR_* labels and 35 arms from their runs, a message RespellString; no RENUM needed (0 enumerators go)
phase 27 | 27 | whim27 | 346 | 1992 | b | PARAM TEXTQ | B3b | no Ex mode: ~28 folds, main_loop's parameter
phase 28 | 28 | whim28 | 67 | 850 | b | RENUM BUILD TEXTQ | B3b | no :write
phase 29 | 29 | whim29 | 115 | 638 | b | RENUM | B3b | no :read
phase 30 | 30 | whim30 | 131 | 844 | b | RENUM TEXTQ | B3b | no :edit, gf
phase 31 | 31 | whim31 | 102 | 569 | b | PARAM | B3b | open_buffer(void)
phase 32 | 32 | whim32 | 476 | 5665 | b | RENUM PARAM TEXTQ FOLDX | B3b | the buffer has no name
phase 33 | 33 | whim33 | 87 | 656 | done | - | B1b | begins on the graph (q032.g); ex_quit's dead tail, two members
phase 34 | 34 | whim34, droplocal x2, whim34rows | 155 | 513 | done | - | B3b | W10 and [RO]: DeleteDefinition, RespellString, DropArgPure; graph end to end
phase 35 | 35 | whim35 | 227 | 1854 | done | - | B3b | the never-opened FILE*s: CutRun, DropOperand, PARAM
phase 36 | 36 | whim36 | 242 | 12079 | b | FRAG RENAME | B3c | 18 musl string functions; ~600 uses renamed
phase 37 | 37 | whim37 + musl-*.md | 188 | 15400 | b | FRAG RENAME | B3c | ctype, case tables in-file; after 36
38a | 38 | phase/038/a | 199 | - | done | - | B1b | begins phase 38 on the graph (q037.g), 38b and 38 text after it; deathtrap's ladder, the assertions the work
38b | 38 | phase/038/b | 98 | - | b | RENAME RETYPE FRAG | B3c | main -> static vim_main, a launcher; the collection's root moves
phase 38 | 38 | whim38a, whim38b, whim38 | 164 | - | b | FRAG PARAM | B3c | exit through the host; builtins
phase 39 | 39 | whim39 | 254 | 20227 | b | FRAG INITROW RENAME | B3c | the host block: macros, termios, externs
phase 40 | 40 | whim40 | 330 | - | b | FRAG PARAM TEXTQ | B3c | stream writes through vim_host_message
phase 41 | 41 | whim41 | 255 | - | b | PARAM RENAME FRAG INCLUDE | B3c | host calls direct
phase 42 | 42 | whim42 @minmax | 400 | - | b | RETYPE RENAME FRAG MACROX INCLUDE | B3c | header types owned; MIN/MAX may stay a text step
phase 43 | 43 | whim43 | 717 | - | done | - | B3d | the first #include becomes the boundary; gcc's three answers asked of the edges, the move one act (MoveFormsOwning)
phase 44 | 44 | whim44 | 272 | - | done | - | B3d | the scalar clock; Retype, ReplaceC, two FRAG units
phase 45 | 45 | whim45 | 287 | - | done | - | B3d | case tables merged (rows read as values); ArrangeRowsTyped, RetargetAs
phase 46 | 46 | whim46 | 158 | - | done | - | B3d | 1 statement -> 2: one LiteralC
phase 47 | 47 | crefactor/graph/b3db_own.go | 7+248 | - | done | - | B3d | abs/labs own: graph.Own (xform's Own deleted)
phase 48 | 48 | whim48 | 369 | - | done | - | B3d | the clock crosses: Rename, MoveForms, RetypeResult, FRAG
49a | 49 | phase/049/a | 186 | - | done | - | B3d | realloc leaves the core; DeleteForHeader
49b | 49 | phase/049/b | 396 | 828 | done | - | B3d | malloc/free/write leave; RetargetAs, DeleteForHeader
phase 49 | 49 | whim49a, whim49b, whim49 | 506 | 508 | done | - | B3d | getpid/kill leave; graph end to end
phase 50 | 50 | crefactor/graph/b3db_unions.go | 6+311 | - | done | - | B3d | degenerate unions: graph.DegenerateUnions (xform's Unions deleted)
51a | 51 | phase/051/a | 374 | - | done | - | B3d | terminal names; ArrangeRowsTyped, RespellString
phase 51 | 51 | whim51a, whim51 | 389 | - | done | - | B3d | -T goes; FoldNever/FoldAlways, DropParam, ReplaceRun
phase 52 | 52 | whim52 | 218 | 2918 | done | - | B3d | the arena; one SpliceC of six fragments
phase 53 | 53 | whim53 @state | 817 | 4723 | b | PARAM FOLDX FRAG | B3e | swap-file residue; mostly deletions
phase 54 | 54 | whim54 @state | 589 | 15478 | b | FRAG RETYPE RENAME PARAM | B3e | a block number becomes a reference; after 53
phase 55 | 55 | whim55 @state | 665 | 4418 | b | FRAG PARAM MOVE | B3e | de-page the leaf; after 54
phase 56 | 56 | whim56 @state | 791 | 4045 | b | FRAG RETYPE PARAM | B3e | fold the node types; after 55
phase 34 | 34 | whim34, droplocal x2, whim34rows | 155 | 513 | b | PARAM(variadic arg) TEXTQ | B3b | W10 and [RO]; droplocal on the graph; 1 import today
phase 35 | 35 | whim35 | 227 | 1854 | b | PARAM | B3b | the never-opened FILE*s
phase 36 | 36 | whim36 | 242 | 12079 | done | - | B3c | 18 musl string functions; 610 uses retargeted (RetargetUses), 23 sites by SubstituteC
phase 37 | 37 | whim37 + musl-*.md | 188 | 15400 | done | - | B3c | ctype, case tables in-file; calls retargeted, macro classifiers by MACROX
38a | 38 | phase/038/a | 199 | - | done | - | B1b | begins phase 38 on the graph (q037.g), 38b and 38 text after it; deathtrap's ladder, the assertions the work
38b | 38 | phase/038/b | 98 | - | done | - | B3c | main -> static vim_main (Rename, SetStorage), a launcher by FRAG
phase 38 | 38 | whim38a, whim38b, whim38 | 164 | - | done | - | B3c | exit through the host; builtins; AddParamC; graph end to end
phase 39 | 39 | whim39 | 254 | 20227 | done | - | B3c | the host block: its 47-act table by Substitutor, delfuncs as Delete
phase 40 | 40 | whim40 | 330 | - | done | - | B3c | stream writes through vim_host_message; AddParamC
phase 41 | 41 | whim41 | 255 | - | done | - | B3c | host calls direct: DropParams, RetargetAs
phase 42 | 42 | whim42 @minmax | 400 | - | done | - | B3c | header types owned; MIN/MAX/offsetof by MACROX on the graph
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
phase 53 | 53 | whim53 @state | 817 | 4723 | b | DRAFT (done, B3e) | B3e | swap-file residue; mostly deletions -- on the graph: its text acts on the C view, committed as FRAG
phase 54 | 54 | whim54 @state | 589 | 15478 | b | DRAFT (done, B3e) | B3e | a block number becomes a reference; after 53 -- on the graph, as 53
phase 55 | 55 | whim55 @state | 665 | 4418 | b | DRAFT (done, B3e) | B3e | de-page the leaf; after 54 -- on the graph, as 53
phase 56 | 56 | whim56 @state | 791 | 4045 | b | DRAFT (done, B3e) | B3e | fold the node types; after 55 -- on the graph, as 53
phase 57 | 57 | whim57 | 13 | - | b | RETYPE | B3f | p_emoji int
phase 58 | 58 | whim58 | 13 | - | done | - | B1c | three (pos_T *)-1 tests fold; on the graph (B1c), a FoldNever by form
phase 59 | 59 | whim59 | 23 | - | b | RETYPE RENAME | B3f | garray_T *
phase 60 | 60 | phase/060 + graph/dropcalls.go | 34+126 | - | done | - | B3g | no-op frees go: Editor.DropCalls, the locals by Editor.DeadLocals (PureCond), the host's calls RetargetAs
phase 61 | 61 | whim61 | 23 | - | b | FRAG | B3f | buflist_findnr
phase 62 | 62 | phase/062 + graph/foldmore.go | 34 | - | done | - | B3g | empty blocks fold: B2d's Editor.EmptyBlocks on the core, PureCond
phase 63 | 63 | whim63 | 58 | - | b | MOVE RENAME RETYPE | B3f | one regprog type
phase 53 | 53 | whim53 @state | 817 | 4723 | b | PARAM FOLDX FRAG | B3e | swap-file residue; mostly deletions
phase 54 | 54 | whim54 @state | 589 | 15478 | b | FRAG RETYPE RENAME PARAM | B3e | a block number becomes a reference; after 53
phase 55 | 55 | whim55 @state | 665 | 4418 | b | FRAG PARAM MOVE | B3e | de-page the leaf; after 54
phase 56 | 56 | whim56 @state | 791 | 4045 | b | FRAG RETYPE PARAM | B3e | fold the node types; after 55
phase 57 | 57 | whim57 | 13 | - | done | - | B3f | p_emoji int; on the graph (B3f): RETYPE: the declaration retyped
phase 58 | 58 | whim58 | 13 | - | done | - | B1c | three (pos_T *)-1 tests fold; on the graph (B1c), a FoldNever by form
phase 59 | 59 | whim59 | 23 | - | done | - | B3f | garray_T *; on the graph (B3f): RETYPE x3, RetargetAs/RENAME, RenamePrototypeParams (new)
phase 60 | 60 | crefactor/xform/dropcalls.go | 7+94 | - | a | BUILD FOLDX(dead stores) | B3g | no-op frees go; not typed
phase 61 | 61 | whim61 | 23 | - | done | - | B3f | buflist_findnr; on the graph (B3f): BodyC, three Cuts
phase 62 | 62 | crefactor/xform/emptyblocks.go | 7+89 | - | b | FOLDX(empties everywhere) | B3g | empty blocks fold; not typed
phase 63 | 63 | whim63 | 58 | - | done | - | B3f | one regprog type; on the graph (B3f): casts by Rewrite; three declarations RetargetAs; the two typedefs one by FRAG with the offsetof
phase 64 | 64 | whim64 | 21 | - | done | - | B1c | the engine called directly; on the graph (B1c): 4 statements rebuilt by template, 1 cut
phase 65 | 65 | whim65 | 14 | - | done | - | B3f | changedtick a number; on the graph (B3f): RETYPE + RENAME of the member, BodyC, 18 Rewrites
phase 66 | 66 | whim66 | 58 | - | done | - | B3f | parameters never used; on the graph (B3f): folds by form, PARAM (B2c's test moved in)
phase 67 | 67 | whim67 | 72 | - | done | - | B3f | typed sort and search; on the graph (B3f): comparators' forms written anew by FRAG (RETYPE refuses a function used as a value), the searches rebuilt by Rewrite, sort by ReplaceC
phase 68 | 68 | whim68 | 57 | - | done | - | B3f | highlight groups by scan; on the graph (B3f): BodyC x2, LiteralC, Cuts, one unit
phase 69 | 69 | whim69 | 66 | - | done | - | B3f | regrepeat's goto into a case; on the graph (B3f): the class run read as items, its head FRAG, the loop kept
phase 70 | 70 | whim70 | 17 | - | done | - | B3f | no build date; on the graph (B3f): RespellString, two Rewrites
71a | 71 | phase/071/a | 98 | - | done | - | B3f | regatom: an outlined block; on the graph (B3f): outlined by FRAG (MOVE refuses: uses become parameters), switch moved into a BUILD loop
71b | 71 | phase/071/b | 207 | - | done | - | B3f | edit(): 17 gotos; on the graph (B3f): jumps classified by ancestors; one FRAG unit
phase 71 | 71 | whim71a, whim71b, whim71 | 59 | - | done | - | B3f | check_termcode; after 71a, 71b; on the graph (B3f): FRAG for the jump, MOVE (MoveRun) for the skipped items into a BUILD else
phase 72 | 72 | whim72 | 58 | - | done | - | B3f | a node names its block; on the graph (B3f): struct, bodies and 19 casts by FRAG (selection chains as cc reads them), assertion by Rewrite and RespellString
phase 73 | 73 | whim73 | 58 | - | done | - | B3f | deathtrap at the next wait (host); on the graph (B3f): one FRAG unit, then INCLUDE's MoveFormsAfter
74a | 74 | phase/074/a | 20 | - | b | FRAG | B3f | lalloc's body
phase 74 | 74 | whim74a, phase/074 + graph/nevernull.go | 35+381 | - | b | FRAG (74a) | B3g, B3f | whim74 on the graph (B3g): Editor.NeverNull, the set by forms, its locals by spelling as the text's; 74a still text, so the phase imports after it
phase 75 | 75 | whim75 | 73 | - | b | FRAG INITROW TEXTQ | B3f | three typed stacks
76a | 76 | phase/076/a | 96 | - | b | INITROW RETYPE RENAME FRAG | B3f/step 6 | def_val split; a typed transform in text
phase 76 | 76 | whim76a, whim76 | 294 | - | b | INITROW RETYPE FRAG RENAME | B3f/step 6 | optvar_T: kinds from typed edges
77a | 77 | phase/077/a | 13 | - | b | FRAG | B3f | a comparison
phase 77 | 77 | whim77a, whim77 | 13 | - | b | FRAG (77a) | B1c, B3f | whim77 on the graph (B1c): an if cut; 77a still text, so the phase imports after it
phase 78 | 78 | whim78 | 27 | - | b | FRAG MOVE | B3f | gcc's argument order through locals
phase 79 | 79 | whim79 | 16 | - | b | FRAG | B3f | a static byte for (char_u *)-1
phase 80 | 80 | whim80 | 23 | - | b | RETYPE | B3f | yankreg_T *
phase 81 | 81 | whim81 | 16 | - | b | FRAG | B3f | a font read guarded
phase 82 | 82 | whim82 | 28 | - | b | RETYPE FRAG INITROW | B3f | flexible arrays become pointers
phase 83 | 83 | whim83 | 29 | - | b | PARAM RETYPE TEXTQ | B3f | no cookie
phase 84 | 84 | whim84 | 37 | - | b | FRAG MOVE | B3f | ml_get_invalid outlined
phase 85 | 85 | whim85 | 19 | - | b | FRAG | B3f | a flag for a pointer comparison
86a | 86 | phase/086/a + graph/deadstmt.go | 21+104 | - | done | - | B1c | statements after a jump; on the graph (B1c): Editor.DeadStmt, StmtTerminates on nodes; xform.Terminates stays for fallout.go (the gotos are on the graph since B3g)
phase 86 | 86 | whim86a, whim86 | 35 | - | b | PARAM FRAG | B3f | six dead stores; one parameter becomes a local
87a | 87 | phase/087/a + xform/boolret.go | 7+1020 | - | b | RETYPE BUILD | B3f | bool for 278 functions; cc.Parse only
phase 87 | 87 | whim87a, whim87 | 167 | - | b | FRAG TEXTQ | B3f | 153 key codes named
phase 88 | 88 | crefactor/xform/includes.go | 159 | - | d | - | - | each #include tried under gcc
phase 89 | 89 | graph/gototail.go, gotoflow.go | 297+256 | - | done | - | B3g | goto tails: Editor.GotoTail, the copies Clone's; begins on text (88), imports
phase 90 | 90 | graph/gotobreak.go, gotoflow.go | 90+256 | - | done | - | B3g | goto -> break: Editor.GotoBreak; handed the graph
phase 91 | 91 | graph/gotoloop.go, gotoflow.go | 211+256 | - | done | - | B3g | goto back -> loop: Editor.GotoLoop, the region moved; handed the graph
phase 92 | 92 | graph/gotoblock.go | 314 | - | done | - | B3g | goto out -> do-while(0) break: Editor.GotoBlock, the region moved; handed the graph
phase 93 | 93 | whim93 | 71 | - | b | FRAG MOVE TEXTQ | B3f | one_adjust as functions
phase 74 | 74 | whim74a, xform/nevernull.go | 7+253 | - | b | FOLDX(label) | B3g | never-NULL tests fold; not typed
phase 75 | 75 | whim75 | 73 | - | done | - | B3f | three typed stacks; on the graph (B3f): one FRAG unit: LiteralC, LiteralExprC (new), ReplaceEachC (new), FragAt (new)
76a | 76 | phase/076/a | 96 | - | b | INITROW RETYPE RENAME FRAG | B3f/step 6 | def_val split; a typed transform in text; B3f: left text, for step 6
phase 76 | 76 | whim76a, whim76 | 294 | - | b | INITROW RETYPE FRAG RENAME | B3f/step 6 | optvar_T: kinds from typed edges; B3f: left text, for step 6
77a | 77 | phase/077/a | 13 | - | done | - | B3f | a comparison; on the graph (B3f): ReplaceC; phase 77 is graph end to end
phase 77 | 77 | whim77a, whim77 | 13 | - | done | - | B3f | an if cut (B1c) and a comparison (B3f): graph end to end
phase 78 | 78 | whim78 | 27 | - | done | - | B3f | gcc's argument order through locals; on the graph (B3f): BUILD + MoveTo (B2c's test moved in)
phase 79 | 79 | whim79 | 16 | - | done | - | B3f | a static byte for (char_u *)-1; on the graph (B3f): one FRAG unit
phase 80 | 80 | whim80 | 23 | - | done | - | B3f | yankreg_T *; on the graph (B3f): RetypeResult, Retype x3, two Rewrites (B2c's test moved in)
phase 81 | 81 | whim81 | 16 | - | done | - | B3f | a font read guarded; on the graph (B3f): LiteralExprC
phase 82 | 82 | whim82 | 28 | - | done | - | B3f | flexible arrays become pointers; on the graph (B3f): RETYPE x3, one FRAG unit
phase 83 | 83 | whim83 | 29 | - | done | - | B3f | no cookie; on the graph (B3f): one DropParams over the family (B2c's test moved in)
phase 84 | 84 | whim84 | 37 | - | done | - | B3f | ml_get_invalid outlined; on the graph (B3f): one FRAG unit
phase 85 | 85 | whim85 | 19 | - | done | - | B3f | a flag for a pointer comparison; on the graph (B3f): one FRAG unit
86a | 86 | phase/086/a + graph/deadstmt.go | 21+104 | - | done | - | B1c | statements after a jump; on the graph (B1c): Editor.DeadStmt, StmtTerminates on nodes; xform.Terminates stays for fallout.go and the gotos
phase 86 | 86 | whim86a, whim86 | 35 | - | done | - | B3f | six dead stores; one parameter becomes a local; on the graph (B3f): 86a (B1c) and whim86 (B3f): Cuts by form and context, Rewrite, ParamToLocal
87a | 87 | phase/087/a + xform/boolret.go | 7+1020 | - | b | RETYPE BUILD | B3f | bool for 278 functions; cc.Parse only; B3f: left text (B3f as built)
phase 87 | 87 | whim87a, whim87 | 167 | - | b | FRAG TEXTQ | B3f | 153 key codes named; B3f: left text (B3f as built)
phase 88 | 88 | crefactor/xform/includes.go | 159 | - | d | - | - | each #include tried under gcc
phase 89 | 89 | crefactor/xform/gototail.go | 366 | - | b | CLONE FOLDX(label) | B3g | goto tails; cc.Parse only
phase 90 | 90 | crefactor/xform/gotobreak.go, gotoflow.go | 84+186 | - | a | BUILD FOLDX(label) | B3g | goto -> break
phase 91 | 91 | crefactor/xform/gotoloop.go, gotoflow.go | 232+186 | - | b | MOVE | B3g | goto back -> loop
phase 92 | 92 | crefactor/xform/gotoblock.go | 404 | - | b | MOVE | B3g | goto out -> do-while(0) break
phase 93 | 93 | whim93 | 71 | - | done | - | B3f | one_adjust as functions; on the graph (B3f): expansions by form, one FRAG unit with the lvalue a hole twice
phase 94 | 94 | crefactor/xform/memberout.go, rewrite.go | 703+147 | - | c | step 6 | 6 | member out-parameters
phase 95 | 95 | crefactor/xform/stateparam.go | 429 | - | c | step 6 | 6 | the engine's state a parameter
phase 96 | 96 | whim96 | 232 | - | done | - | B3f | a line's match alone; after 95; on the graph (B3f): one FRAG unit (WrapEachC, AfterEachC, BeforeEachC new)
phase 97 | 97 | whim97 | 19 | - | done | - | B3f | :g asks match_range; after 96; on the graph (B3f): one FRAG unit
phase 98 | 98 | whim98 | 16 | - | done | - | B3f | ml_clearmarked's guard; on the graph (B3f): ReplaceC
phase 99 | 99 | whim99 | 16 | - | b | INCLUDE FRAG | B3f | WHIM_TIME (host)
phase 100 | 100 | crefactor/xform/localout.go, structscalar.go | 1148+420 | - | c | step 6 | 6 | values for out-parameters and struct locals
phase 101 | 101 | crefactor/xform/plainc.go | 308 | - | c | step 6 | 6 | identity, ascii classes, constant ifs
phase 102 | 102 | crefactor/xform/boolret.go (Globals) | 11 | - | b | RETYPE BUILD | B3f | file-scope flags bool; B3f: left text (B3f as built)
phase 103 | 103 | crefactor/xform/boolret.go (Relax) | 12 | - | b | RETYPE BUILD | B3f | more flags bool; B3f: left text (B3f as built)
```

190 rows: 23 (a), 119 (b), 8 (c: the `FallOutOf` wrapper and the three
front phases it wraps, 94, 95, 100, 101), 6 (d: phase 0 and its three
parts, 43 as built, 88), and 34 `done` (stage A's 5 phases that began on
the graph, B1a's 16 rows, B1b's, B1c's and B3b's; counted after B3b; the
`droplocal` steps of 4, 5, 16 and 34 are done too, inside phases whose
other steps are listed). A phase of several units has a
190 rows: 28 (a), 104 (b), 8 (c: the `FallOutOf` wrapper and the three
front phases it wraps, 94, 95, 100, 101), 6 (d: phase 0 and its three
parts, 43 as built, 88), and 44 `done` (stage A's 5 phases that began on
the graph, B1a's 16 rows and B3a's 23; the `droplocal` steps of 4, 5, 16 and 34 are
done too, inside phases whose other steps are listed). A phase of several units has a
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
    programs go and nothing replaces them). **Done**, *B1a as built* below:
    the rule took neither as it stood, and takes both with one more
    spelling; whim20bl's field is no droplocal's.
  - **B1b**: 33, 38a (their assertions as queries) -- **done**, *B1b as built* above.
  - **B1c**: 58, 64, 77, 86a (`Terminates` on nodes).
  - **B1b**: 33, 38a (their assertions as queries).
  - **B1c**: 58, 64, 77, 86a (`Terminates` on nodes) -- **done**, *B1c as built* below.
- **B2, the capabilities** (after B0, side by side, each in
  `crefactor/graph`, generic, with unit tests on read-back graphs):
  - **B2a FRAG** (with CLONE and MACROX) -- **done**, *B2a as built*: the
    externs and builtins (`__builtin_setjmp`, `__builtin_offsetof`), macro
    invocations made nodes as the importer makes them, new ids, types, on a
    synthesized unit; 8 literal splices of real phases byte for byte.
  - **B2b INITROW, RENUM, RENAME** (string literals included).
  - **B2c PARAM, RETYPE (clearing types, no checker), MOVE.**
  - **B2d FOLDX** -- **done**, *B2d as built* above: the rules listed
    above, each with the units that want it (B0 took the declaring-block
    splice).
  - **B2e INCLUDE** and the boundary queries (the first include form,
    "above/below", the host region).
- **B3, the b-class conversions** (each after the capabilities its rows
  name; one agent each; ranges chosen so a batch's phases are contiguous,
  which is when a conversion starts saving time):
  - **B3a** phases 4-6, 9-12, 16, 21, 22, 26 (~2,300 lines): FRAG, INITROW,
    PARAM, FOLDX(walk), RENAME, MOVE. With B1a, 4-26 is graph but for 15
    and 24's neighbours; the in-phase sweeps become collections.
  - **B3b** phases 27-35 (~1,600): RENUM, PARAM, FOLDX, little FRAG --
    **done**, *B3b as built* below: no FOLDX or FRAG was needed.
    and 24's neighbours; the in-phase sweeps become collections. **Done**,
    *B3a as built* below: phases 4-26 run on the graph end to end.
  - **B3b** phases 27-35 (~1,600): RENUM, PARAM, FOLDX, little FRAG.
  - **B3c** phases 36-42 (~2,100, ~50 KB of literal C): FRAG with externs,
    builtins and macros; RENAME; PARAM; INCLUDE. 42's MIN/MAX may stay a
    text step.
  - **B3d** phases 43-52 (~3,900): INCLUDE everywhere; 43 rewritten with
    edge queries in place of gcc's answers, or left text (then 44 imports);
    order 43 -> 47 -> 48; 49a -> 49b -> 49; 51a -> 51.
  - **B3e** phases 53-56 (~2,900, ~29 KB of literal C), a strict chain: one
    agent, in order. Mostly FRAG of whole items and bodies; 53 is mostly
    deletions. **Done**, *B3e as built* below: DRAFT, the text acts
    committed as FRAG.
  - **B3f** phases 57-87 but 58/60/62/64/74/77/86a, and 93, 96-99 (~2,100 of
    programs, BoolRet's 1,020): FRAG, RETYPE, PARAM, MOVE, INITROW; 76/76a
    may wait for step 6 (a typed transform written in text); 96 after 95. **Done** but 76a/76, 87a/87, 102 and 103, *B3f as built* below.
  - **B3g** phases 60, 62, 74 and 89-92: the `crefactor/xform` text
    transforms (DropCalls, EmptyBlocks, NeverNull, the gotos: ~1,600 lines)
    on the graph: CLONE, MOVE, FOLDX -- **done**, *B3g as built* below.
- **B4, the front** (phases 1-3: 47 cutters and parts, ~5,100 lines): last.
  Its cutters run inside one step each, wrapped by `xform.FallOutOf`,
  which types the text before and after the cuts. The closure is on the
  graph (B2d: `FoldX` with `Before` and `Hold`, the text's on all three
  phases byte for byte, *B2d as built*); a front phase ends on the graph
  once its cutters are, and FoldX then needs their new expressions typed
  (FRAG/BUILD, or step 6). Most cutters are (a) or need BUILD/INITROW.
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
## B2b as built (2026-10-03)

`crefactor/graph`'s `renum.go` (RENUM), `initrow.go` (INITROW) and
`rename.go` (RENAME), generic, naming nothing in vim: 408, 579 and 420
lines of Go, comments and blank lines aside. Two existing files changed,
minimally: `edit.go`'s `splice` is split in two, `splice` computing the
place and `spliceAs` doing the rest with the place given (what the rows and
enumerators splice through, as items; no behaviour moves), and `build.go`'s
`exprType` types `(index a i j)` -- `a[i][j]`, one form -- as what `a`
points at once a subscript, where it took one level for any chain (a wrong
typed edge, not an unknown one: B0's).

### The API

**RENUM.** `Editor.ArrangeEnum(enum, order, how)` makes an enum's
enumerators `order`: its own kept (moved, their ids kept), those left out
deleted (their uses left dangling, for the closure or the cut), new ones
(`NewEnumerator(name, value)`, a value from `BuildValue(after, src)`, which
sees the enumerators before it) inserted with fresh ids. `how` is a
`Values`:

- `HoldValues` refuses if any enumerator of the file would change its value
  -- the editor's `Delete` rule, but for a set at once (a run and the
  implicit ones after it, which `Delete` asks for last-first);
- `Renumber` lets the implicit values move, as a text program's deleted
  line moves them, and reports every value that moved, in this enum and
  any other whose values name it (`Renumbered.Moved`);
- `PinValues` writes a survivor with the value it had where its implicit
  value would follow another enumerator than before -- after a deleted run,
  the sweep's rule, even when the value would not move (`{A, B = 0, C}`
  less B pins `C = 1`, as Prune does) -- or would move; spelled as Prune and
  `Collect` spell it, decimal, hexadecimal from 65536 up. A value that
  cannot be computed is refused, where the sweep keeps the run.

The values are the sweep's: every enumerator outside function bodies, in
the file's order, by name, through `evalForm` (`EnumValues`). Conveniences:
`DeleteEnumerators(ens, how)` (several enums), `MoveEnumerators(ens, after,
how)`, `InsertEnumerators(after, ens, how)`; `IsEnumerator`,
`Enumerators`, `EnumeratorName`. Verbs: `DeleteEnumerators(names, how,
what)`, `MoveEnumerators(names, after, how, what)`. One act, `renum`, its
Moved the kept enumerators; a pin is an act `pin`.

**INITROW.** `Editor.ArrangeRows(def, order, ix)` makes a table's rows
`order` (kept, moved, deleted, inserted as above) and says again every
position the file names. A row's position is its index in the array: one
after the row before, or its designator's (`(at (idx K) ...)`, K evaluated
as an enumerator's value; `[K ... L]`). What names a position:

- every subscript of the table by a decimal constant, anywhere, found by
  the edges (`&highlight_tab[6]`, `(index T 6)`): written with the new
  position, refused when its row goes;
- `RowIndex.Enumerators`, the enumerators the cut names as the table's
  indexes (no edge says that `main_errors[]` is read at `ME_EXTRA_CMD`, a
  parameter carries it): one with a constant value is written with the new
  position; an implicit one must come out there by itself, every other
  value held; one whose row goes is deleted with it, and its declaration
  when it was its enum's only enumerator (untagged, a form of its own);
- `RowIndex.Arrays`, the permutations the cut names (`nv_cmd_idx[]` for
  `nv_cmds[]`): every element written anew, refused when its row goes.

A row designated by a name keeps its place when another goes: nothing to
say. An array whose size is written is refused; a struct's initialiser is
not a table (no positions). The table's typed edge, an array of so many,
is pointed at the type node of the new count when the graph holds one,
else cleared and listed in `Untyped`. Everything is checked before
anything changes. `DeleteRows(def, rows, ix)`, `InsertRows(def, before,
rows, ix)`, `BuildRows(def, src, holes)` (rows from C-lisp: `(init E...)`,
`(at .member V)`, `(at (idx K) V)`, an expression; members by the element
type), `TableInit(def)`. Verbs, in `InTable`: `DeleteRows(pat, n, ix,
what)` (rows the pattern matches, counted), `DeleteRowsEach(pats, ix,
what)` (each pattern one row, all in one arrangement), `InsertRows(before,
tmpl, ix, what)`. One act, `rows`.

**RENAME.** `Editor.Rename(d, to)` respells a declaration, every
declaration of its entity (a function's prototypes and definition, an
object's declarations, a tag's forward declarations) and every use, found
by the refers edges: a function, an object, a typedef, an enumerator, a
local, a parameter, a member (its selections and designators `.m`, a
member of an anonymous struct checked against its container's), a tag
(every `(struct TAG)` that refers to it), a label. Refused: a name already
declared where the new one would be (the file's ordinary names and
externals, a member of the same struct, any tag, a label of the same
function, a local of the same scope or a parameter); a use the new name
would resolve elsewhere (`Resolve` at the use finds a local of that name);
an outer declaration's use that a renamed local would capture; a
block-scope `extern` declaration of the entity; an external (the
headers': retarget its uses instead); a use inside a macro's text
(`(macro "...")`). `RetargetAs(use, i, to)` points a use at a declaration
of another spelling and respells it, checked to resolve there to `to` (to
the first declaration of a function, the importer's edge);
`RetargetUses(from, to)` every use at once, all or none (phase 36's
`strlen` to `musl_strlen`, once FRAG has written the definitions). Verbs:
`Rename(name, to, what)` (a local or parameter in `InFunction`, else the
file's). Ids are kept: an act `rename` (or `retarget-as`) whose Moved are
the nodes respelled in place.

**Strings: the rule.** A string literal is never renamed because it
contains a name, and nothing replaces text across literals.
`RespellString(s, spelling)` writes ONE literal anew, the new spelling a
whole literal token (checked: prefix, quotes, escapes); the verb
`RespellString(old, new, n, what)` finds the scope's literals spelled
exactly `old`, counts them against `n`, and respells each: the explicit
opt-in list, each literal named whole. A typed edge above it that is an
array is cleared (its length changed) -- none on the importer's graphs,
which type a parenthesised literal as the pointer it decays to, as cc does.

### What the tests prove

`crefactor/graph`'s `renum_test.go`, `initrow_test.go` and
`rename_test.go`, every case on a graph read back from its Lisp, the C
view held to C written by hand and printed canonically (and read back
again): the three policies and their refusals; `PinValues` against
crefactor/sweep's `Prune` and the graph's `Collect` on four samples (after
a run, the value it had already, the first enumerators, hexadecimal), byte
for byte; moves and insertions with `BuildValue`; subscripts and a
permutation renumbered, the table's type pointed at an existing array type
or cleared; index enumerators, explicit (`main_errors[]`'s shape: three
deleted with their declarations, one written anew) and implicit (renumbered
by their enum, or refused), and a non-index value held; designated rows;
rows built and inserted with designators resolved to members; every kind of
rename and every refusal, a macro's text included (on a graph written by
hand); `RetargetAs`/`RetargetUses` with a capture refused; string respells
counted, the literal token checked.

`internal/graphcheck`'s `renum_test.go`, on the snapshots (`GRAPH_SNAPS`),
each graph imported, written as Lisp and read back, its C view held to the
text program's result printed canonically:

- **argvfront** (phase 1's D1, on q000): command_line_scan's new body
  (`Body`, a 40-line template), the calls and the `--clean` prescan cut,
  `main_errors[]`'s three rows with `RowIndex{Enumerators: ME_*}`, and
  `mainerr_arg_missing` with its prototype -- **byte for byte**, nothing
  dangling. The control: the same rows with no index said leave
  `ME_EXTRA_CMD = 4` and the three enums, and the bytes move.
- **phase 51a** (on q050): eight `builtin_terminals[]` rows, the
  xterm-family clause of `find_builtin_term()`, the fallback `"xterm"` and
  `report_term_error()`'s two messages respelled -- **byte for byte**.
- **filefront** (phase 1's D4, on the text argvfront, exfront and extable
  leave of q000): thirteen designated `cmdnames[]` rows deleted (no
  position moves) and their enumerators moved after `CMD_SIZE` with
  `Renumber`, 99 values moving -- **byte for byte**; the same move held is
  refused.
- **a rename by edges** (on q050): `find_builtin_term` (definition and
  three calls) and `mparm_T.term`, one use among 125 mentions of `term`
  that are other things -- against the replacement written out, byte for
  byte.

`cd crefactor && go test ./...`, `go test ./...`, gofmt, go vet and
staticcheck clean; `make whim-build-check` (parallel) byte for byte.

### Limits

- Values are evaluated by name, as the sweep does: two enumerators of one
  name in different scopes would share it. Enums inside function bodies
  are not arranged (the sweep does not evaluate them either).
- A position is named only by a decimal subscript, a named index
  enumerator or a named permutation; a position computed otherwise (`T[i +
  1]`, a `#define`'d constant gone to an integer elsewhere, a pointer
  difference) is the cut's to know. An implicit index enumerator that would
  not come out right is refused rather than written explicitly.
- A rename does not reach into a macro's text, a string, or a comment
  (there are none); a block-scope `extern` of the entity is refused rather
  than respelled. Renaming a parameter respells the definition's, not a
  prototype's (a name of its own).
- `RespellString` respells; it does not re-derive a `sizeof` of the
  literal elsewhere, which is the cut's to respell too.
- No phase is converted: argvfront and filefront are B4's, 51a B3d's;
  the tests are their conversions' proofs in advance.
## B2c as built (2026-10-03)

PARAM, RETYPE and MOVE in `crefactor/graph`: `param.go`, `retype.go`,
`move.go`, what the three share in `typeedit.go` (type nodes made and
interned, an expression's type derived, the typed edges above a change
walked again), and their verbs in `b2cverbs.go`; generic, naming nothing in
vim. 904, 287, 346, 349 and 74 lines of Go, comments and blank lines aside.
Two existing files changed: `edit.go` gains one field, `argLists`, and a
guard in `place` and `insertPlace` -- **the sanctioned path past "a
function's parameters are its type"**: while PARAM's splices run, a fn
form's parameter list and a call's arguments are item places, and nowhere
else, so `Delete` still refuses a parameter; `build.go`'s `basicType` is
split, its words in `basicWords`, so that a basic type the file does not
use yet (`long` in a file that had none) can be made.

### The API

**PARAM.** `DropParams([]ParamDrop{{Decl, I}...}, ParamOptions)` is one edit:
each drop names a function type form by its declaration -- a function (any
declaration of it: every one changes, the block-scope prototypes too), a
parameter of a function (that parameter in every declaration), or a member,
an object, a local or a typedef of a pointer-to-function type or an array of
them -- and the index lost; several indices of one form, and several forms,
at once. `DropParam(fn, param, opt)` is the common case;
`ParamIndex(fn, param)` the index. `ParamOptions.Dangle` leaves a live use
of a dropped parameter dangling, for the closure or the collection (an
unused local's initialiser), instead of refusing. `DropArg(call, i)` drops
one argument a callee takes through `...` (its format string is the
caller's). `ParamToLocal(fn, param)` is a drop whose parameter moves into
the definition's body as its first item, `(def NAME TYPE ATTR...)`, the
same node with the same id, so that its uses need nothing. `AddParam(fn, i,
"(NAME TYPE ATTR...)", arg)` adds one, read in each declaration's place,
and at every call the argument `arg(call)` says, read at the call
(direct calls only; a name that would hide the file's, or repeat one at the
top of the body, refused). `ParamStats`: parameters, arguments, calls,
declarations retyped.

**RETYPE.** `Retype(decl, "TYPE")` -- an object (every file-scope
declaration of it), a local, a member, a typedef, or a function's parameter
(in every declaration of the function) -- and `RetypeResult(fn, "TYPE")`,
the type a C-lisp form read at each declaration's place (BUILD's typedef
names and tags). `RetypeStats`: forms, declarations retyped, expressions
typed again, left untyped. `Rederive(n)`, for any edit: the expressions
from n up typed again where plain -- what a converter calls after a
`Replace` the editor cleared conservatively (phase 66's `FALSE` in an
`int`'s place: 20 cleared, 0 after).

**MOVE.** `MoveBefore(n, at)`, `MoveAfter(n, at)`, `MoveRun(first, last, at,
after)`: items beside an item -- statements among statements, the file's
forms among the file's. `MoveTo(n, to, fill)`: any node into the place of
`to`, a placeholder that goes; `fill` (new, built where n stands) takes the
place n leaves, or nil where an item or an else may be left out.

**Verbs**: `DropParam`, `DropParams`, `ParamToLocal`, `Retype(pat, typ)`,
`RetypeResult`, `MoveBefore(pat, at)`, `MoveAfter`, each reported and
refused as B0's acts are.

### The invariants each keeps

- **PARAM, checked before anything moves.** What changes type is closed to
  a fixed point: a declaration whose form names a typedef that changes, a
  function one of whose parameters changes. Each one's new type is what its
  form will say, made by `formType` with the drops and the typedefs' new
  types in it -- and its form as it stands must first give its type node
  now, so that a form PARAM cannot read (`typeof`, a computed array size)
  is refused rather than mistyped. Then every use of every one of them,
  through `()`, `*`, `&`, `[i]` and a member's selection, must be a callee
  (the call loses the arguments its callee's fn form loses), an argument
  dropped with it, a flow -- an argument to a parameter, either side of `=`,
  `==`, `!=`, a return, a declaration's value, an initialiser's element
  (positional: an array's element, a struct's k-th member) -- whose other
  side has the same new type node (a function decays to its pointer) or is
  a null constant, or a truth test; anything else (a cast, arithmetic, a
  designated initialiser, a function passed through `...`) is refused,
  named. A call keeps every other argument only where its type and its
  parameter's, as the edit leaves them, agree. A dropped argument must be
  free of side effects (no store, no call, nothing kept as text): the text
  dropped whatever was there. A dropped parameter of a definition may have
  no live use but in a dropped argument (or Dangle).
- **PARAM, made.** Arguments and parameters go through the splice (ids
  superseded, `delete` acts); a list that empties says `void`; the new type
  nodes are made in one `type` act and interned (one node per structure:
  phase 83's member and the five functions it points at share one); each
  declaration's typed edge is its new type (a `retype` act); and the
  expressions above every use are typed again. A drop on directly called
  functions leaves nothing untyped: a call's type is its callee's result.
- **RETYPE.** A function whose type changes (a parameter, the result, a
  typedef a parameter names) must be used only as a callee, else refused.
  The old forms are superseded and the new given; the declarations keep
  their ids, their changed typed edges logged. A declaration reached
  through a typedef whose form RETYPE cannot read loses its typed edge,
  listed in `Untyped`: unknown, never wrong. Above every use, and above a
  cast that names a changed typedef, each expression is typed again where
  its type follows plainly (a call, a selection, `&`, `*`, `[i]`, an
  assignment, a comparison, a cast, a sizeof), otherwise cleared and listed,
  up to the statement, stopping where it finds a type unchanged. It is not
  a checker: a cast made redundant, or a value that no longer fits, is the
  caller's to rewrite (phase 80's two casts are `Rewrite`s).
- **MOVE.** Made on the graph, checked, and undone when it refuses: nothing
  logged, no id given. Every edge across the moved nodes' boundary must
  resolve from where it now stands to the declaration it refers to
  (BUILD's resolution, the importer's: the scopes before it, innermost
  first, then the file's first declaration of the name): a use inside of a
  declaration outside, a use outside of a declaration inside, and, where
  they go, a use of another declaration of a name they declare (which they
  would hide, or which is no longer the first). Where it resolves to
  another file-scope declaration of its name -- one entity in C -- the
  edge is retargeted there, logged, so that the graph stays the importer's
  (a definition moved above its prototype takes its uses). A label stays
  in its function; a member or a tag is used after its definition; a name
  is not declared twice in one block or as a parameter there; a `break`,
  `continue`, `case` or `default` binds to the same loop or switch, a
  `return` to the same function. A move keeps every id (a `move` act);
  MoveTo's placeholder is superseded and its fill given, as a Replace's,
  and the expressions above both places are typed again.

### What the tests prove

`crefactor/graph`'s `param_test.go`, `retype_test.go` and `move_test.go`,
every case on a graph read back from its Lisp: each edit's C view against
C written by hand, printed canonically, byte for byte, the invariants
(`Check`) and the graph read back the same (`Equal`); the types (a function
type interned once, the selections of a retyped member, a typedef's reach
to a parameter, its function and a cast, a sum cleared and listed); the ids
(a moved local, `ParamToLocal`'s parameter and its uses, `MoveTo`'s
argument); and each refusal to its message with the graph untouched: a
parameter still used (and Dangle's dangling record), an argument with a
side effect, a function stored in a table of the old type, a typedef's
pointer named by its object, a `...`; a function whose address is taken;
a use before its declaration, a declaration that would hide another, one
declared twice, a break that would rebind, a goto out of its function, a
node into itself, a local's use moved out of its scope.

`internal/graphcheck`'s `b2c_test.go`, on the real snapshots
(`GRAPH_SNAPS`): whole phases whose rows need the three, written on the
graph outside the plan, on q(N-1)'s graph read back from its Lisp, held to
the text program twice -- before the sweep, the graph's C view against the
text program's output printed canonically; and collected, against qN.c --
**byte for byte, every one**, with nothing left untyped:

| phase | on the graph | the graph's acts |
| --- | --- | ---: |
| 31 | a condition rewritten, `open_buffer`'s three parameters dropped at its three calls | 31 ms |
| 66 | five functions' eval parameters dropped (two of `find_ex_command` at once, the formatter's and the parse's it hands `tvs` to in one edit), after the folds and the ten `FALSE`s | 143 ms |
| 83 | the cookie: five functions, three parameters and a member of the getter's pointer type, one family -- 18 parameters, 13 calls, among them the calls through `eap->ea_getline` and `fgetline` | 143 ms |
| 57 | `p_emoji` an `int` | 3 ms |
| 80 | `get_register`'s result, `put_register`'s parameter, two locals; the two casts rewritten | 42 ms |
| 78 | 11 arguments moved into new locals' values (`MoveTo`), the places they leave taken by the locals | 10 ms |
| 86 (its parameter alone) | `cmdline_handle_ctrl_bsl`'s `c` made a local, against the text's two literals | -- |

And MOVE on whim-vim.c (q103): `ml_get_buf` before the function of its
first use is refused (`DATA_BL`, a typedef its body names, is defined
between); the first definition that can go there (`lalloc`), and the first
that can go above its own prototype (`ga_init`, two refused before it), each
give the text cut and pasted, printed canonically, gcc accepts the result
(`-fsyntax-only`), and the graph imported from the C view refers as many
uses to the moved definition as the move retargeted.

The pipeline is unchanged: `make whim-build-check` (parallel) gives
whim-vim.c byte for byte, 103 links, 5 begun on their graph snapshot,
every boundary compiling.

### Limits

- **No checker.** RETYPE and MOVE keep the edges right and the types never
  wrong, but whether C accepts the result -- a value that no longer fits its
  new type, a cast now redundant -- is not asked; `Untyped` is where step 6
  starts.
- PARAM follows a function's value through the flows listed and nothing
  else: a cast of a function pointer, one passed through `...`, a
  designated initialiser, a struct copied whole that holds one, a typedef
  named in a cast or a sizeof -- each refused. `AddParam` takes direct calls
  only. Dropped arguments must be free of side effects; there is no option
  to keep one as a statement before the call (no row needs it).
- RETYPE makes no basic type the words do not name, reads no `typeof` or
  computed array size (the declaration is then untyped, listed), and does
  not retype a function wholesale (its parameters and result, one by one).
- MOVE moves items among items, the file's forms among the file's, and any
  node by `MoveTo`; not a member or an enumerator (an enumerator's value is
  RENUM's). A tag or a member used at file scope before its definition is
  refused even where C allows a pointer to an incomplete type. Moving a
  function above its prototype retargets every use of its name, which on
  the snapshots asks the whole file.
## B1c as built (2026-10-03)

Phases 58 and 64, phase 77's own step (`whim77`) and part 86a run on the
graph; their text programs are replaced (history keeps them, `4c8b3a7` and
before). Each is a graph step in the plan (`Graph: true`), registered with
`phase.RegisterGraph`, and reports what the text version reported, line for
line.

| unit | on the graph | Go (non-blank, non-comment) |
| --- | --- | ---: |
| phase 58 | one `FoldNever` of `(== ?p (cast (ptr pos_T) (- 1)))`, 3 | 13 |
| phase 64 | four `Rewrite`s -- each statement the text's `Literal` named, found by its form and rebuilt from a template naming the callee -- and one `Cut`; the collection takes the table, the field and `regengine_T` | 22 |
| whim77 | one `Cut` of the `if` and its call, the whole form as the pattern (the text's literal was the whole statement); the collection takes `free_one_termoption()` | 15 |
| 86a | `Editor.DeadStmt` (`crefactor/graph/deadstmt.go`, new, 104 lines): `StmtTerminates` and `ItemTerminates` on nodes, the runs found first, outermost deleted by `ReplaceRun`, "N runs ... ; M held" as the text said it | 21 + 104 |

**crefactor/graph**: one new file, `deadstmt.go` (`StmtTerminates`,
`ItemTerminates`, `Editor.DeadStmt`), and its test, `deadstmt_test.go`
(crefactor/xform's `TestDeadStmt` and `TestStmtTerminates` moved, the text's
result printed canonically held to the C view, plus a dead run inside a dead
block cut once). Nothing existing changed. `crefactor/xform`'s `DeadStmt` is
deleted (86a was its one caller) and its test with it; `Terminates`,
`StmtTerminates` and `labeled` stay, moved into `terminates.go`, for
`fallout.go`'s closure and the goto transforms (89, 91), which still run on
text.

**Refinement: C-lisp's labels change what an item is.** cc's block item
`case 1: return x;` is ONE labeled statement, which does not terminate; in
C-lisp it is two items, `(case 1) (return x)`. So the rule on nodes asks
whether a label (or the attributes before one) stands before the jump, and a
dead run ends at the first label item. With that, the graph's `DeadStmt` and
the text's agree on all 104 boundaries of a whole run, q000-q103: the text
version's output printed canonically is the graph's C view byte for byte, and
the counts are the same everywhere (0, 3, 15, 16, 17 or 18 runs, 0 held;
measured before the text version was deleted). A declaration-holding run
occurs on none of them: the unit test is what holds `held`.

**Refinement: a template rebuilds what the text's literal named.** 64's
literals named the whole statement, arguments spelled; the patterns name
them too (`(= prog (call (. bt_regengine regcomp) expr re_flags))`), so the
assertion is the text's, and the template's names resolve as the importer
would. 27 ids given, 110 superseded.

### The proof

- `rm -rf .cache/boundaries; make whim-build-check`, in order: whim-vim.c
  byte for byte, 77,634 lines, every boundary compiling, 397 s at a load of
  34-54 (not comparable with stage A's 348 s at 3-13); eight graph snapshots
  now, q057, q063 and q085 added (7 MB each).
- `make whim-build-check` again, in parallel: byte for byte, 103 links, 8
  of them begun on their graph snapshot, every boundary compiling, 85 s at a
  load of ~54.
- The controls, all four at once on the parallel check: 58's `FoldNever`
  made `KeepThen`, 64's `Cut` left out, 77's pattern narrowed to the call
  (leaving the empty if), 86a's deletions stopped after 17 runs. The check
  names exactly the four: `phase 58 gives 76529 lines where q058.c holds
  76549`, 64 76064/76042, 77 75169/75166, 86 75137/75128; `4 of 103 phases do
  not reproduce their snapshot`.
- `GRAPH_SNAPS=... -run 'TestPhasesOnGraph|TestGraphSnapshots'`, `make
  whim-editor-check`, `make whim-test` (80 as HEAD, the Go editor 80 as
  the C), `go test ./...` in both modules, gofmt, go vet and staticcheck on
  what changed: clean.

### Measured

One phase at a time, 5 runs, medians, two rounds alternating with main
`4c8b3a7` on the same snapshots, at a load of 15-50 (so ±0.3 s):

| phase | main (text), wall / CPU ms | B1c from text, wall / CPU ms | handed the graph, wall / CPU ms (the read) |
| --- | ---: | ---: | ---: |
| 58 | 1,511-1,829 / 2,426-2,828 | 1,920-1,966 / 3,542-3,891 | **207 / 309** (45) |
| 64 | 2,158-2,168 / 4,084-4,291 | 1,992-2,026 / 3,871-4,014 | **255 / 384** (57) |
| 77 | 2,120-2,186 / 4,121-4,322 | 1,943-1,956 / 3,657-3,797 | -- (begins on text: 77a) |
| 86 | 2,832-2,867 / 5,960-5,983 | 3,941-3,973 / 8,140-8,150 | 2,201 / 4,398 (63) |

- **What each costs in the plan as it stands.** 58, 64 and 77 import
  (1.7-1.9 s) and end on the graph, so they are as fast as on text or a
  little faster (64, 77: 0.1-0.2 s less; 58 0.1-0.4 s more, its text sweep
  being the cheapest of the four). Their neighbours 57, 63 and 76 end on
  text, so no phase is handed the graph in order: the 0.2-0.26 s a phase
  costs handed the graph arrives only when 57 and 63 (B3f) end on the graph
  too. In the parallel check 58, 64 and 86 read their graph snapshot.
- **86 is 1.1 s slower**: 86a imports and whim86 (B3f) is text, so the phase
  pays the import, a C view, and the text sweep after it. Handed the graph
  it is still 2.2 s, since the sweep stays text. It turns into a saving when
  whim86 moves (B3f), so that the phase ends on the graph; 86a runs first in
  it, as the text did, and moving it after whim86 was not tried.
- **Import/C-view boundaries**: the plan had 11 imports and 15 C views at
  stage A; B1c adds 4 imports (before 58, 64, 77's second step and 86a,
  about 7.5 s in a run in order) and 4 C views (three the boundary's print
  where cemit's was, one before whim86), and saves 3 text sweeps and
  canonical prints (58, 64, 77), about what the imports cost. Net: nothing,
  until the ranges around them convert.
## B1a as built (2026-10-03)

Sixteen units on the graph, byte for byte, their text programs replaced
(history keeps them, `4c8b3a7` and before): the cutters `nointro`,
`optreaders` (phase 3), `nostat` (4), `nobackup`, `lfonly`, `keepbytes`
(5), `noinertopts` (7), `nofloat` (11) and `noowner` (12), graph steps in
`internal/steps`' `graphOps` (`plainGraph`), their text ops gone; and the
programs of part 4a and phases 15 (`whim15a`, `whim15`), 19 (`whim19`),
20 (`whim20`, `whim20bl`), 23 and 25, registered with `RegisterGraph`. Two
programs went and nothing replaces them: `whim18kp` and `whim19ep`. About
1,520 lines of text programs became about 1,210 on the verbs (non-blank,
non-comment). No change to `crefactor/graph` was needed.

**The get_varp finding.** DropLocal's rule did NOT take whim18kp's and
whim19ep's cases as it stood: `get_varp()` writes them `(char_u
*)&curbuf->b_p_kp`, without the parentheses the rule's two patterns
required (`droplocal b_p_kp` on q017 refused: "2 mentions ... no rule
takes it"). One more spelling of each pattern
(`internal/cut/droplocal.go`, `matchAny`) takes both, and the two programs
went. whim20bl's case is not the rule's to take: `b_p_bl` is no
droplocal's field -- whim20 takes its readers and the collection its
member -- so whim20bl stays a program of one `DropCase`. The rule's
report counts the case's `return` among the plumbing sites (`b_p_kp` 6
where the text's said 5); the bytes are the same.

**Phases 12-15, 17-20, 24 and 25 are handed the graph** in a run in order
and end on it, with no import and one C view each; with 8 (stage A's)
that is eleven phases. 3, 7, 11 and 23 import and end on the graph; 4 is
handed phase 3's graph and imports once more after `onebuffer` (two
imports before); 5 imports at its start and again after `noconv`, since
phase 4 ends on text (one import before): the cost the recipe predicts
until B3a. 16 graph snapshots now (q003, q004, q006, q007, q010-q014,
q016-q019, q022-q024), every one gate 2's (`TestGraphSnapshots`).

**The proof.** `rm -rf .cache/boundaries; make whim-build-check` in order:
whim-vim.c byte for byte, 104 boundaries compiling, the 16 graph snapshots
written. Again, in parallel: byte for byte, 103 links, 16 begun on their
graph snapshot, 0 imported. The control, nine faults at once -- one act
changed or dropped in nointro, nostat, nobackup (the accident below),
noinertopts, noowner, 15a, droplocal's new spelling, whim20bl and whim25
-- named exactly phases 3, 4, 5, 7, 12, 15, 18, 19 (the droplocal fault
refuses in both), 20 and 25: `10 of 103 phases do not reproduce their
snapshot`. `GRAPH_PHASES=N` now picks the phases `TestPhasesOnGraph` holds
and `TestMeasureGraphPhases` times (the latter text phases too, for a
before and after); `TestVerbsOnPhase15` runs the registered programs and
holds their report. Every converted unit's report was diffed against the
text program's on its input: the same lines, in order, but phase 18's
droplocal count above.

**Measured**, B1a beside main `4c8b3a7`, each pair side by side at a load
of 20-35 (other agents' builds beside them):

| | main | B1a |
| --- | ---: | ---: |
| the in-order build check, wall | 408 s | 382 s |
| its CPU (user + system, gcc's included) | 982 s | 976 s |
| the parallel check, wall | 113 s | 98 s |
| its CPU | 1,495 s | 1,340 s |

Phase by phase (`TestMeasureGraphPhases`, 5 runs, medians; main's text,
and B1a's the way a run in order takes it -- handed the graph where the
phase before ends on it, else imported):

| phase | main, wall / CPU ms | B1a, wall / CPU ms | how |
| --- | ---: | ---: | --- |
| 3 | 30,134 / 60,275 | 29,456 / 55,263 | an import after front3, a collection for the sweep |
| 4 | 16,545 / 31,484 | 12,212 / 27,448 | handed; 1 import (2 before) |
| 5 | 10,090 / 24,932 | 12,070 / 29,514 | 2 imports (1 before) |
| 7 | 4,493 / 9,309 | 2,542 / 4,931 | imported (0.49 s handed) |
| 11 | 2,565 / 5,624 | 2,477 / 5,276 | imported (0.44 s handed) |
| 12 | 2,528 / 4,838 | 479 / 750 | handed |
| 15 | 2,407 / 5,201 | 302 / 639 | handed |
| 18 | 2,280 / 4,843 | 230 / 481 | handed |
| 19 | 5,522 / 11,908 | 499 / 746 | handed |
| 20 | 4,604 / 10,677 | 442 / 727 | handed |
| 23 | 2,432 / 5,325 | 2,205 / 4,713 | imported (0.27 s handed) |
| 25 | 4,242 / 10,185 | 618 / 750 | handed |

Their sum, about 26 s less in a run in order, is the in-order build's
difference. A phase handed the graph costs 0.23-0.62 s against 2.3-5.5 s
on text; one that imports costs what the text did, the import standing
where the sweep's parse and the print were.

### Refinements of the catalogue

- **The verbs report every act; the text often reported once for several.**
  nofloat's arms, nobackup's groups, noowner's swap-file acts and lfonly's
  multi-place regexps run on a second, quiet `Verbs` (`io.Discard`) whose
  refusal is returned, and the line is written after. A verb option that
  silences an act's report would say it directly.
- **`DropOperand` drops parentheses the text kept.** C-lisp has no `paren`
  where the printer adds one, so `if ((a || b) && !f())` with `!f()`
  dropped prints `if (a || b)` where the text's literal left `if ((a ||
  b))`. Phase 25 has its own `dropOperand`, wrapping a lone `||` or `?:`
  left under `&&` in a `paren`; lfonly writes `(paren ?a)` in its template.
  An option on the verb would serve the next converter.
- **A macro's text respelled.** One of phase 25's five `(Rows - p_ch -
  tabline_height())` is inside an unexpanded `MIN()`: the phase replaces
  the `macro` node by one with the text respelled, its edges kept but the
  one to `tabline_height`. MACROX's (B2a) in miniature.
- **The text's accidents are reproduced, said as acts.** nobackup's text
  cut the lines `{`, `acl = mch_get_acl(fname);`, `}` and left `if
  (!newfile)` without its block; once the statements after it went, the if
  took `prev_got_int = got_int;` as its body, and q005-q019 hold that (the
  product does not: `buf_write` goes at phase 20). The graph version deletes the
  call and moves the statement into the emptied block by a `Splice`, its
  comment saying why. The control drops that act and phase 5 is named. A
  later phase could undo it; byte for byte forbids doing it here.
- **"Count 1" sometimes meant "the first".** The text's `cutCounted` cut
  the first N matches and never refused on more, and `Line()` matches any
  indentation; on the graph `mch_free_acl(acl)` and `vim_free(backup)` are
  scoped to `buf_write`, where there is one each.
- **`DeleteDefinition` refuses while a call to a function with no
  prototype remains** (`set_init_default_backupskip`): cut the call first.
- **A pattern that binds a node and constrains its shape** (`?x:pat`)
  would have made whim20's two `fileinfo` acts, whose `&&`s share an
  operand, one `Rewrite` each; they are a `One` and a scoped `DropOperand`.
- **The text's own questions are kept exactly** where the count was the
  assertion: nofloat's literal scan, 4a's one-engine proof, keepbytes'
  `bad_char`, 23's rows, noowner's and nobackup's leftover mentions run the
  text version's expressions on the C view (`Text`, `TextCount`,
  `TextQuery`), 50-110 ms for the file. Where the count stood for an edge
  question (lfonly's dying functions' callers, whim15's enumerator), the
  edges answer.
- **No count differed from the text's.** No else-if arm moved a fold's
  count; where a bare name matched more than the text's head (`p_fic`,
  `nofile_err`), `DropOperand`'s operand rule or an `InFunction` scope
  made the count the text's.
## B3b as built (2026-10-03)

Phases 27-32, 34 and 35 run on the graph, byte for byte, their text
programs replaced (history keeps them and their `editlit.go`s, `16717ab`
and before): each a graph step in the plan (`Graph: true`), registered with
`phase.RegisterGraph`, its report the text version's line for line. With
B1b's 33, phases 27-35 run with no import between them: 27 imports where 26
ends on text, and 28-35 are each handed the graph the phase before ends on.
Phase 34 is graph end to end (whim34, its two droplocal steps, whim34rows).
About 1,500 lines of text programs (non-blank, non-comment; 1,990 with
their literals) became about 1,280 on the verbs.

| phase | on the graph |
| --- | --- |
| 27 | the folds by condition and place, the operands, the statements cut, the `Q` row's handler `RewriteAt`, gQ's `DropCase`, main_loop's parameter PARAM's |
| 28 | the enumerators still named deleted (RENUM, `Renumber`), ZZ's `"x"` respelled, the `:w>>` parse cut; the residue `vimtext.CoreResidue` on the C view |
| 29 | the enumerator, the `:r!` parse cut, five operands |
| 30 | the enumerators, an operand, gf/gF's two case labels, `[f`'s if folded with its else kept, `++opt` cut |
| 31 | the `&&` rewritten; open_buffer's three parameters in one `DropParams` (the definition and three calls) |
| 32 | runs of items cut, ifs folded by condition, `return K;` bodies, operands, CMD_file (RENUM), buflist_new's and shorten_fnames' parameters (PARAM) |
| 34 | change_warning's six calls and its definition, the CTRL-G format respelled and its argument dropped through `...`, two operands, a block |
| 35 | four operands, a run, a return rewritten, three folds, the writes and calls cut, ui_write's parameter |

**crefactor/graph**: one new file, `b3bverbs.go` (177 lines), and its test
`b3bverbs_test.go`; nothing existing changed. Each verb is a shape of the
text's that B0's set did not say:

- **`FoldNeverAt(IfAt, cond, n, what)`, `FoldAlwaysAt(cond, n, noJump,
  what)`**, `Editor.IsArm`: a fold by where its if stands. The text's
  `line("if (X)")` did not match `else if (X)`; on the graph both are `(if
  X ...)`, so B0's FoldNever counted both and phase 27's counts (2 and 1 for
  use_plus_cmd's three ifs, wait_return's arm) were not the text's.
  `IfNotArm`/`IfArm` count only the place asked for. `noJump` is phase 27's
  own refusal of a kept branch holding a `break` or `continue`, kept as an
  assertion.
- **`CutWhere(pat, ok, n, what)`**: a Cut of the matches a test takes --
  phase 27's two literals of `ex_no_reprint = TRUE;` told apart by their
  indentation (a function's own item, or one level in), phase 34's six
  calls that stand as statements, phase 35's five writes of TRUE or FALSE.
- **`Run(what, pats...)`, `CutRun`**: a text literal of several whole
  statements as the run of items it was -- the first found once, each next
  its following sibling, matching. B1b wrote it locally; phase 32 has five.
- **`Muted(acts)`**: acts whose report is the caller's -- the text's
  `within` with no `what`, or several acts reported once (phase 30's two
  case labels, 32's folded `return K;` bodies). B1a's quiet `Verbs` as a verb.
- **`Editor.DropArgPure(call, i, names...)`**: DropArg with the functions
  named taken as free of side effects. Phase 34's `[RO]` argument is
  `curbuf->b_p_ro ? (shortmess(SHM_RO) ? _("[RO]") : _("[readonly]")) :
  ""`, which DropArg refuses for its calls; the cut names `shortmess` and
  `_`, as the text's deletion of them said, and any other call still
  refuses.

### Refinements of the catalogue

- **Nothing of FOLDX or FRAG was needed.** The catalogue's FOLDX (32) was
  the text's own folds, each naming its if; its "little FRAG" were literals
  that delete or keep: every new node is a moved one, `FALSE` (BUILD), a
  `nullptr` atom (32's three, untyped), or a respelled string.
- **PARAM where the text replaced a prototype, a definition and a call
  apart**: one edit (`DropParam`/`DropParams`), reported as the text's three
  literals, at the place of the last. 32's buflist_new: the text left
  `char_u *ffname = ffname_arg;` naming a parameter it had taken, for the
  sweep; the graph leaves that use dangling (`ParamOptions.Dangle`) and the
  collection takes the local, as the sweep did.
- **The assertions are the text's own questions on the C view** where the
  count was the assertion (the anchors' mentions, 28-30's residue, 32's
  writes and the functions holding them, 34's options[] rows, 35's
  invariant): a phase takes the file's view once per table of counts, 2-4
  views a phase (60-100 ms each), which is most of what a phase costs
  handed the graph.
- **The parentheses the printer writes are not `paren` nodes.** A pattern
  copied from the C (`(paren (| EX_CTRLV EX_XFILE))`) matches nothing; one
  the source wrote (`(&& (paren (& (. ea argt) EX_CMDARG)) ...)`) is a
  node, and where the text's literal took it with the operand, the
  expression it held is moved into the `&&`'s place (`RewriteFunc`).
- **Two of the text's reports name the wrong function**: 32's "the one call
  site, create_windows'" is in `win_alloc_firstwin`, and 35's "nv_visual"
  conjunct is `clear_showcmd`'s. The text matched them file-wide; the graph
  scopes each to where it is, the report kept and a comment saying so.
- **"the sweep" is "the collection"** in seven report lines, as in B1b's;
  nothing else in any report moved.
- B2c's `TestB2cPhase31` held phase 31 written on the graph to the text
  program; the phase is that program now, held by `TestPhasesOnGraph`, so
  the test went.
## B3d as built (2026-10-03)

Phases 43-52 run on the graph end to end -- part 49a, 49b, 51a and every
phase's own program -- each a graph step (`Graph: true`), its text program
replaced (history keeps them, `16717ab` and before); `crefactor/xform`'s
`Own` (47) and `Unions` (50) went with them, their logic now
`crefactor/graph`'s. Phase 43 imports (42 ends on text); 44-52 are each
handed the graph the phase before leaves, with no import between them.
About 4,420 lines of text programs (non-blank, non-comment; 559 of them
xform's) became about 3,510 on the verbs, beside 940 of library.

**Phase 43: rewritten, not left text.** The text program compiled the cut
eight times and read gcc's diagnostics; each of its three questions has an
answer in the edges, and the answers are gcc's:

- *the twelve names* are what the move would leave unprovided --
  `MoveWouldLose`, B2e's headers' rule as a query: exactly the twelve
  macros, and no declaration (the core takes no function or type from a
  header). B2e's `TestIncludesPhase43` had shown the same twelve the other
  way round on q043.
- *defined but not used* is a static function or object of the core no use
  in what stays refers to, **a function's own recursive calls not
  counting** -- gcc's rule (`c-typeck.cc`: "Recursive call does not count
  as usage"; a `sizeof`, an address in a dead object's initialiser, a dead
  function's call all count; measured with gcc on a sample before it was
  written). The enum blocks are asked the same of their enumerators and
  tag, by edge, where the text counted words. The rounds are the text's:
  6, the same names in each, 14 functions, 18 objects, 3 enum blocks.
- *used but never defined*, the boundary: the static functions the core
  declares and uses and does not define -- the same 13 names.

The move is ONE act, `MoveFormsOwning` (below): the 41 include forms, the
enum blocks, the objects, the two `va_list` prototypes and the functions
before the host block in the text's order, the twelve enumerators written
by FRAG beneath `usize`, the core's 100 tokens of them made uses; then the
twelve static_asserts by FRAG below the last include, where their names are
the headers' again. What compiling the finished cut proved -- 0 errors,
nothing dead -- is the rule (nothing in the core takes a name from a header,
no collision) and the fixpoint's last round. 1.2 s handed the graph, where
the text's compiles took 14 s of wall time (6 s of CPU in this process,
gcc's beside it). Its `moved`/`boundary` files and `@state` went: nothing
read them. So did 49b's, 49's and 52's.

**The chain holds to the import.** `TestB3dHandsOnTheImport`
(`internal/graphcheck/b3d_test.go`) runs each of 43-52 on q(N-1)'s graph
read back, its programs called directly, collects, and holds the result to
qN.c's import by `SameGraph`: every refers edge where the importer puts it,
every typed edge to a type of the same structure, ids aside -- stronger than
the bytes, and what a phase handed the graph needs of the one before it.
All ten pass; three gaps it found are fixed (below).

### Library added (crefactor/graph, new files, generic, with tests)

| file | what | for |
| --- | --- | --- |
| `moveowning.go` | `MoveWouldLose(at, after, ns)`: what a move of top-level forms would leave unprovided, as a query. `MoveFormsOwning(at, after, ns, own)`: the move, ids kept; every edge it crosses resolved again as the importer resolves it (cc's rule, the definition where the prototype names no parameter) and retargeted, else refused; a lost declaration refused; the lost macros handed to `own`, which declares them (FRAG); every token that was such a macro made a use of the declaration it now resolves to, the typed edges above it kept where cc gives the macro the type of the declaration (a probe unit of the header alone) and cleared otherwise; the rule held against the graph before the move | 43 |
| `b3da_rows.go` | `ArrangeRowsTyped`: `ArrangeRows` that types the table's new length (an array type made and interned) and a new row's signed literal, as the import types them | 45, 51a |
| `b3db_own.go` | `Own(knobs)`: libc functions the core declared and called made its own -- prototypes deleted, definitions by FRAG, every use retargeted by edge, no literal naming one | 47 (xform's `Own` deleted) |
| `b3db_unions.go` | `DegenerateUnions`: a one-member union field takes its member's type, every access rewritten without the member (`x.m.only`, `x->m.only`), the retype first so nothing above an access loses its type | 50 (xform's `Unions` deleted) |
| `b3db_include.go` | `SystemIncludeRun`: the include forms, contiguous, all `<...>` | 47, 48 |
| `b3dc_header.go` | `DeleteForHeader(ds, dangle, also)`: the file's own prototypes of header functions deleted, each use below an include declaring the name remade by FRAG as the import makes it (the external, typed from the header); a use above every such include refused, or left dangling for the collection when asked | 49a, 49b, 49 |

`internal/whim/vimtext/graph.go` holds the shapes 49b and 49 share
(`IncludeRun`, `OrdinaryBlock`, ...); 49's `shapes.go` and the text shapes
nothing else used went.

**Fixes to existing code, each found by the chain's `SameGraph`:**

- **`(void)` typed as imported** (`typeedit.go` +6, `retype.go` +3,
  `param.go` +3): cc, and so the importer, types `f(void)` with one `void`
  parameter; B2c's `formType` (PARAM's and RETYPE's new types) wrote none,
  so a function whose last parameter PARAM dropped (51's `termcapinit`, 31's
  `open_buffer` once B3b converts it), or one RETYPE retyped (48's
  `host_time`), was typed unlike its import. `AddParam` takes the void out
  before inserting. `TestVoidParamsAsImported`.
- **A table's new length** (51a): INITROW cleared the typed edge of a
  table whose new length the graph had no array type for; 51a now uses
  `ArrangeRowsTyped`.
- **The tokens' types** (43): a `Replace` of a macro token by a use clears
  the typed edges above it (the editor cannot tell the type is the same);
  `MoveFormsOwning` keeps them where the probe says it is.

### Refinements

- **A count that stood for uses is asked of the edges; the text's own
  counts stay on the C view.** 43's constant counts, 44's and 48's
  mention tables, 47's and 50's rename counts are `edit.MentionCount` on
  the view; "every one a call", "the four sites", "only the declaration or
  an access" are edges. 45's "utf_convert at 6 calls" was 5 calls and the
  definition's head: the edges say 5, and the report does.
- **"Eleven directives on eleven lines" is the include forms**: contiguous,
  `<...>`, nothing above the first but the core. The text-only checks (line
  deltas, blank-line runs) are dropped, as B0 drops them; 48's two line
  deltas are restated for the C view, core -8 and host +8.
- **FRAG units that name each other's declarations are two units** (44: a
  fragment cannot refer to what another fragment of the same unit
  declares); 49's six units cost 0.3 s each, its largest cost.
- **Deleting the file's prototype of a header function** is a deletion and
  a FRAG of each use where the header provides it (`DeleteForHeader`; 48
  does it by hand for `time`): the use then refers to the external, typed
  from the header, as the import has it.
- **Reports** are the text's line for line but where a line said lines:
  43's "lines -> lines" and "THE CUT IS N LINES" say forms; 52's last line
  counts the C view's lines; 49's literal count is `Strings()`'s (no
  character literals).

### The proof

- `rm -rf .cache/boundaries; make whim-build-check`, in order: whim-vim.c
  byte for byte, 77,634 lines, every boundary compiling, 332 s (893 s of
  CPU, gcc's included) at a load of 10-60 with the other B3 batches beside
  it; 29 graph snapshots now, q026-q031, q033 and q034 new (q032 was B1b's).
- `make whim-build-check` again, in parallel: byte for byte, 103 links, 29
  of them begun on their graph snapshot and none imported, every boundary
  compiling, 71 s.
- The control, a fault in each of the eight at once on the parallel check:
  27's msg_start fold made never, 28's `"q!"` spelled `"q"`, 29's EX_TRLBAR
  operand kept, 30's `case 'F'` kept, 31's `&&` not rewritten, 32's `b_fname
  = b_sfname` kept, 34's format given a space, 35's `return retesc` kept.
  The check names exactly the eight -- 27, 28, 29, 30 and 34 by their bytes,
  31, 32 and 35 by their own refusals (PARAM's "still used", the holders of
  the fields, retesc's count): `8 of 103 phases do not reproduce their
  snapshot`.
- `TestPhasesOnGraph` and `TestGraphSnapshots` (GRAPH_PHASES=27-35), `make
  whim-editor-check`, `make whim-test` (80 as HEAD, the Go editor 80 as the
  C), `go test ./...` in both modules, gofmt, go vet and staticcheck on what
  changed: clean. Every converted program's C before the collection was
  diffed against its text program's output printed canonically, on
  q(N-1): byte for byte, all eight.

### Measured

`TestMeasureGraphPhases`, 5 runs, medians: main `16717ab`'s text, then
B3b's, one after the other on the same snapshots, at a load of 10 rising to
60 (other batches' builds) during B3b's -- so B3b's are if anything high:

| phase | main (text), wall / CPU ms | B3b imported | B3b handed the graph |
| --- | ---: | ---: | ---: |
| 27 | 3,150 / 6,163 | 2,845 / 4,435 | 630 / 767 |
| 28 | 2,892 / 5,212 | 3,261 / 4,609 | 890 / 1,037 |
| 29 | 2,693 / 5,272 | 3,368 / 5,113 | 758 / 962 |
| 30 | 2,924 / 5,455 | 3,194 / 4,816 | 836 / 1,006 |
| 31 | 3,288 / 5,860 | 3,402 / 5,308 | 1,252 / 1,482 |
| 32 | 4,074 / 9,139 | 3,041 / 5,724 | 1,002 / 1,368 |
| 33 | 2,296 / 4,735 (B1b's) | 2,655 / 4,055 | 497 / 658 |
| 34 | 4,384 / 8,608 | 3,022 / 4,544 | 593 / 819 |
| 35 | 3,597 / 6,165 | 3,438 / 5,242 | 1,008 / 1,402 |

A run in order takes 27 imported (26 ends on text until B3a) and 28-35
handed: about 9.7 s against main's 29.3 s for the nine, and two fewer
imports (before 33 and 34) and seven fewer sweeps and canonical prints.
What a phase costs handed the graph is mostly its own views of the file
for the text's counts (2-4 of them, 50-100 ms each at this load) and the
collection (110-270 ms); 31's three file views are why it is the dearest.
36 begins on text (B3c), so phase 35's graph is printed for it: the C
view that was 33's before.
  byte for byte, 104 boundaries compiling, **330 s** (918 s of CPU); main
  `16717ab` beside it the same minutes, 353 s (957 s). 31 graph snapshots
  now, q042-q051 added.
- Again, in parallel: byte for byte, 103 links, 31 begun on their graph
  snapshot and 0 imported, every boundary compiling, 83 s at a load of
  about 40.
- The control, six faults at once -- 43's objects moved before the enum
  blocks, 44's `* 1000L` made `* 1000`, 46's `msg_didout = TRUE` made
  `FALSE`, 47's `musl_labs` testing `>= 0`, 48's assert `default: 0 + 0`,
  52's message buffer `m[161]` -- names exactly phases 43, 44, 46, 47, 48
  and 52: `6 of 103 phases do not reproduce their snapshot`.
- `GRAPH_PHASES=43-52` `TestPhasesOnGraph` (imported and handed) and
  `TestB3dHandsOnTheImport`; `make whim-editor-check`; `make whim-test` (80
  as HEAD, the Go editor 80 as the C); `go test ./...` in both modules;
  gofmt, go vet and staticcheck on what changed: clean.

**Measured** (`TestMeasureGraphPhases`, 3 runs, medians; main's text at a
load of 10-25, the graph at 30-45):

| phase | main (text), wall / CPU ms | graph, as a run in order takes it, wall / CPU ms |
| --- | ---: | ---: |
| 43 | 13,564 / 5,786 (gcc's eight compiles apart) | 3,325 / 6,069 imported (1,291 / 1,640 handed) |
| 44 | 2,875 / 4,632 | 1,089 / 1,576 handed |
| 45 | 3,345 / 5,119 | 618 / 1,110 |
| 46 | 3,060 / 4,688 | 603 / 840 |
| 47 | 2,324 / 3,395 | 579 / 844 |
| 48 | 2,762 / 3,785 | 2,172 / 2,710 |
| 49 | 4,381 / 6,056 | 3,207 / 4,465 |
| 50 | 2,307 / 3,373 | 817 / 1,028 |
| 51 | 4,988 / 7,621 | 1,938 / 2,298 |
| 52 | 2,784 / 4,337 | 993 / 1,198 |

42.5 s of wall time on text against 15.3 s, 23 s less in a run in order:
the in-order build's difference. 48 and 49 cost most handed: 48's two
moves across the includes recompute what the headers provide (about 0.5 s
a time), 49's six FRAG units 0.3 s each. Phase 43 imports until phase 42
(B3c) ends on the graph.
## B3g as built (2026-10-03)

Phases 60, 62 and 89-92 and phase 74's own step (`whim74`) run on the
graph; `crefactor/xform`'s `DropCalls`, `EmptyBlocks`, `NeverNull`,
`GotoTail`, `GotoBreak`, `GotoLoop` and `GotoBlock` are deleted (history
keeps them, `16717ab` and before), with their tests, which moved to
`crefactor/graph`. 60, 62 and 74 are `RegisterGraph` programs
(`internal/phase/0NN/edit.go`); 89-92 are graph steps of `internal/steps`
(`b3ggotos.go`, registered into `graphOps` in an `init`), their floors
`--at-least` as before. Each reports what the text version reported, line
for line. `xform.Terminates`, `StmtTerminates` and `labeled` stay, for
`fallout.go`'s closure; `xform.Flags` is the text steps' argument parser,
exported for the graph versions.

| unit | on the graph | Go (non-blank, non-comment) |
| --- | --- | ---: |
| phase 60 | `Editor.DropCalls`: the call statements of `vim_free`/`host_free` in the core found by their form, each deleted or replaced by its argument's one `++`/`--` (moved, not rebuilt), then `Editor.DeadLocals` (new: EmptyBlocks' half that takes the locals, with `PureCond` as the text's `DeadStores`), the host's three calls pointed at `host_free` by `RetargetAs` | 34 + 126 |
| phase 62 | B2d's `Editor.EmptyBlocks` on the core (`whim.GraphCore`), `PureCond` | 34 |
| whim74 | `Editor.NeverNull`: the never-NULL set to its fixed point on the defn forms; `if ((v = f()) == nullptr)` split by `Clone` of v; the store, an optional `w = v;`, and the test found as consecutive items; FoldNever/FoldAlways as the verbs make them (`Unwrap`); the labels no goto reaches by edge | 35 + 381 |
| phase 89 | `Editor.GotoTail`: a tail's statements copied over each goto by `Clone`, a block where cc's goto was a labeled statement | 297 |
| phase 90 | `Editor.GotoBreak` | 90 |
| phase 91 | `Editor.GotoLoop`: the region's items moved into `(for () () () (block ...))` or `(do (block ...) c)`, ids kept | 211 |
| phase 92 | `Editor.GotoBlock`: the region moved into `(do (block ...) 0)`, in rounds | 314 |
| the walk | `gotoflow.go`: the frames, `gfNext`, `gfMarks`, the groups, positions, names spelled | 256 |

Against about 1,710 lines of text transforms deleted (94, 89, 253, 366,
84, 232, 404 and 186).

**crefactor/graph**, new files only: `gotoflow.go`, `gototail.go`,
`gotobreak.go`, `gotoloop.go`, `gotoblock.go`, `nevernull.go`,
`dropcalls.go` (with `DeadLocals`), and their tests `gotos_test.go`
(crefactor/xform's `goto_test.go`, `gototail_test.go`,
`gotoblock_test.go` and `TestGotoTailNoStatements`, moved: each source
imported, read back from its Lisp, rewritten, `Check`ed, its C view held
to the text step's result printed canonically, the programs compiled and
run and required to print the same, the controls kept) and
`dropcalls_test.go` (`TestDropCalls`, `TestNeverNull`). Nothing existing
changed but one test: `foldx_test.go`'s `TestEmptyBlocksAgainstText` holds
the text's result on its sample as recorded (4 blocks, the locals `b` and
`p`), since `xform.EmptyBlocksRule` is gone; `internal/graphcheck`'s
`TestEmptyBlocksPhase62` likewise holds the text's numbers on q061 (49, five
locals).

### Refinements

- **A rule's "item" is a group.** cc's `L: x;` is one labeled block item;
  C-lisp's is `(label L) x`. Every rule that counted items counts groups
  -- the label items in a row (goto labels, cases, the attributes before
  one) and the statement after them: a region's start is its group's
  first label, a label's "statement" the item after it, a goto "after a
  label" (`labeledAt`) is cc's goto inside a labeled statement (GotoBreak
  writes `;` there, GotoTail a block of the copies, as the text did).
- **Every body is a block.** The text steps had cases for an unbraced
  `if (c) goto L;` that canonical text never holds; on the graph they do
  not arise. Three of the moved tests fed the text step such sources: their
  expectations are now what canonical text gives (TestGotoBreakFalls'
  emptied else-if is `{ }`, not `{ ; }`; "the label is not a block item"
  under an unbraced if is "a goto is not in the label's block"; the
  GotoBlock control edits the canonical spelling).
- **`attributed` is cc's empty statement**: `__attribute__((fallthrough));`
  in a tail is skipped and not counted, as the text skipped it.
- **NeverNull asks its locals by spelling, as the text did.** By edge, a
  local `dest` whose every store is a never-NULL call makes its function
  never-NULL; the text read `*dest = NUL;` as a store of `dest` (its
  `\bdest\s*=`), and `x && dest` as `&dest`. The edge rule gave two more
  functions (21 against 19) and one more fold (153 against 152), so the
  local's questions are asked of the forms the way the text spelled them:
  a store is an `=` whose left side prints ending in the name, a step one
  printed against it, an address an `&` (or `&&`) printed before it
  (`nnFirst`, `nnLast`). With that the sets agree at every round. The
  rest is by form: a cast is a pointer to a type of one word, a path is
  what `[\w.>\[\]-]+` spells.
- **Positions for offsets.** Where the text compared byte offsets (a goto
  before its label, regions overlapping), the graph compares preorder
  positions in the function (`gfOrder`).
- **"Named after it" stays a spelling.** The text's `\bname\b` over the
  rest of a block counts strings, members and macro texts: `gfNames` asks
  every atom of the spelling and every literal or macro text holding it as
  a word.

### The proof

- Before the text versions were deleted, the four goto transforms and
  NeverNull were run text and graph on every boundary (q000-q103; NeverNull
  from q043, where the core has a line): the graph's C view the text's
  output printed canonically **byte for byte on all 104**, the reports line
  for line (a test kept out of the commit: it needs the text versions).
- `GRAPH_PHASES=N ... -run TestPhasesOnGraph`: 60, 62, 74, 89-92 each qN
  from q(N-1), imported, and 60, 62, 89-92 handed the graph read back too.
- `rm -rf .cache/boundaries; make whim-build-check`, in order: whim-vim.c
  byte for byte, 77,634 lines, every boundary compiling, 351 s (5m53 wall,
  13m49 user) at a load of 30-75; 27 graph snapshots now, q059, q061 and
  q088-q091 added. Again, in parallel: byte for byte, 70 s, 27 links begun
  on their graph snapshot, 0 imported.
- The control, seven faults at once, one in each unit (60 dropping the
  `--` it should keep, 62 without `PureCond`, 74 refusing every `!=`
  fold, 89 never deleting a dead tail, 90 writing `;` for every fall, 91
  never writing the do form, 92 writing `while (1)`): the parallel check
  names exactly 60, 62, 74, 89, 90, 91 and 92, `7 of 103 phases do not
  reproduce their snapshot`.
- `make whim-editor-check`; `make whim-test` (80 as HEAD, the control seen
  by 76, the Go editor 80 as the C); `go test ./...` here and in
  `crefactor/`; gofmt, go vet and staticcheck on what changed: clean.

### Measured

`TestMeasureGraphPhases`, 3 runs, medians, at a load of 30-40, beside
main's text on the same snapshots:

| phase | main (text), wall / CPU ms | B3g as a run in order takes it, wall / CPU ms | handed the graph, wall / CPU ms |
| --- | ---: | ---: | ---: |
| 60 | 2,875 / 4,503 | 1,981 / 3,595 (imports: 59 is text) | 256 / 336 |
| 62 | 4,294 / 5,675 | 3,037 / 3,961 (imports: 61 is text) | 576 / 687 |
| 74 | 5,139 / 7,939 | 2,587 / 3,856 (imports after 74a) | -- |
| 89 | 2,153 / 4,362 | 2,042 / 3,443 (imports: 88 is text) | 282 / 427 |
| 90 | 2,085 / 4,237 | **335 / 434** (handed) | 335 / 434 |
| 91 | 3,266 / 4,345 | **279 / 426** (handed) | 279 / 426 |
| 92 | 3,800 / 7,159 | **339 / 558** (handed) | 339 / 558 |

About 13 s less in a run in order: 90-92 are handed the graph (88 stays
text, so 89 imports once for the four), and 60, 62 and 74 import where
their text sweep and print stood and still save the text transforms' own
cost (74's regexps most). When B3f converts 59, 61 and 74a, 60, 62 and 74
are handed the graph too (0.26-0.58 s).
## B3e as built (2026-10-03): DRAFT, phases 53-56

Phases 53-56 run on the graph, a chain: 53 imports (52 ends on text), 54-56
are handed the graph the phase before collected, and each ends on it. Their
text programs are no longer registered (`phase.RegisterGraph`, `Graph: true`
in the plan); history keeps them (`16717ab` and before).

**What they are written on is not the verbs.** The four programs are ~2,900
lines of literal C (29 KB, `editlit.go`) located by text and asserted by line
partitions (where a name is said, by function; every mention in exactly one
class). Their acts stay those text acts, made on the graph's C view, and the
graph takes their difference: a new capability, **DRAFT**, in
`crefactor/graph`'s `draft.go` and `linediff.go` (generic, new files; and
`crefactor/clisp`'s `printspans.go` with a span hook of three lines in
`toc.go`'s printer):

- `e.Draft()` prints the C view, keeping where each top-level form and each
  block or body item was printed; the program edits `d.Text()` as its text
  version edited the file -- same literals, same counts, same partitions, same
  report -- and `d.Commit(text)` makes the result the graph's.
- **Commit** diffs the view and the text line by line (Myers), slides a pure
  insertion or deletion along the lines that repeat around it to where it
  closes every bracket it opens, widens each change to the innermost run of
  whole items holding it (a statement in a nested block, a definition whose
  head changed, a struct), merges runs that touch or nest, widens a run whose
  new text does not close its brackets to the item around it, deletes the
  runs whose new text is empty, and writes the rest anew in ONE synthesized
  import (FRAG). Everything outside the runs keeps its ids and edges.
- **Fragments that name one another** (a function written anew calling
  another, a struct written anew pointing at another) are spliced with the
  edges between them held at a live node and put back, indexed, once all are
  in: SpliceC alone refused them ("a refers edge ... which the graph does not
  hold").
- **Uses carried.** FRAG retargets what cc resolves in the forms it prints
  whole and the top-level names a fragment declares; not the members,
  enumerators and tags of a struct written anew, which a hundred untouched
  functions select. Commit points every live use and typed edge of a node it
  removed at the new node of the same kind and name (a tag's definition, a
  member of that tag, an enumerator, a top-level declaration); a tag nothing
  defines any more resolves to `(extern-struct T)`, as the importer resolves
  it (56's `typedef struct memfile memfile_T`, left for the collection). A use
  with nothing to go to is a refusal.

`draft_test.go`: the diff (random cases), a nested statement (the function
beside it keeps its ids), a struct written anew with members dropped, added
and kept and used elsewhere plus a function's head, its prototype, a call, a
deletion and two insertions, two functions calling each other both written
anew, a tag left undefined, nothing changed, and the refusal (a member still
named); each result's C view the text printed canonically and the graph the
import of that C view (`SameGraph`, `Check`, nothing dangling).
`internal/graphcheck`'s `TestB3eDrafts`: each phase on q(N-1)'s graph read
back, `Check`, read back equal, collected to qN.c byte for byte, and the
collected graph the import of its C view, every refers and typed edge -- so
the carried uses (54: 676) are the importer's. 0 left untyped.

| phase | acts logged | ids given / superseded |
| --- | ---: | ---: |
| 53 | 102 | 612 / 1,804 |
| 54 | 604 (66 changes: 34 runs replaced, 1 inserted, 22 deleted, 95 items; 676 uses carried) | 375 / 1,412 |
| 55 | 30 | 1,199 / 1,820 |
| 56 | 510 | 993 / 1,535 |

**The proof.** `rm -rf .cache/boundaries; make whim-build-check` in order:
whim-vim.c byte for byte, 104 boundaries compiling, 361 s wall and 891 s of
CPU at a load of 25-60 (q052.g-q055.g new among 25 graph snapshots). Again,
in parallel: byte for byte, 67 s, 25 links begun on their graph snapshot, 0
imported. The control, four faults at once -- 53's root pointer `= 7`, a
page size doubled in 54, a `dl_marked = FALSE` in 55, a `return FAIL` in 56,
each in the text a commit takes -- names exactly 53, 54, 55 and 56: `4 of
103 phases do not reproduce their snapshot`. `whim-editor-check`,
`whim-test` (80 as HEAD, the Go editor 80 as the C), `go test ./...` in both
modules, gofmt, go vet and staticcheck on what changed: clean.

**Measured** (`TestMeasureGraphPhases`, 5 runs, medians, load 25-55, so
noisy): main's text phases against the graph's, imported (as 53 is in order)
and handed the graph (54-56 in order; all four in the parallel check):

| phase | main (text), wall / CPU ms | imported, wall / CPU ms | handed, wall / CPU ms |
| --- | ---: | ---: | ---: |
| 53 | 5,773 / 8,442 | 4,362 / 9,062 | 2,183 / 3,002 |
| 54 | 5,319 / 7,250 | 4,650 / 8,528 | 2,757 / 3,547 |
| 55 | 3,270 / 5,091 | 3,647 / 5,511 | 1,346 / 1,632 |
| 56 | 3,788 / 6,450 | 4,446 / 6,643 | 2,204 / 2,662 |

Handed the graph they are 1.3-2.8 s, not the 0.2-0.4 s of a phase on the
verbs: the text acts themselves cost what they did (54's partitions are 1.9
s of its 2.5), and the commit's FRAG import 0.3-0.6 s.

**Refinement, and what it leaves.** DRAFT is a bridge, not the end: the
acts are located by text, not by pattern, and a run is written anew whole
where a pattern would have rewritten a node (54 supersedes 1,412 ids where
RENAME/RETYPE/PARAM would keep most). Rewriting these four on the verbs --
the partitions as edge questions, the signatures as PARAM, the structs'
members as RETYPE/RENAME, the literals as LiteralC/ReplaceC -- is left; the
bytes, the assertions and the report would not move, and DRAFT's tests are
how such a rewrite would be held. The state files the programs write
(`@state`) are written as before; nothing reads them.
## B3a as built (2026-10-04)

Phases 4-6, 9-12, 16, 21, 22 and 26 on the graph: every remaining text step
of theirs converted, byte for byte, their text programs replaced (history
keeps them). Ten cutters are graph steps in `internal/steps`' `graphOps`
(`onebuffer`, `nowild`, `nowildmenu`, `noconv`, `noglob`, `nofind`,
`nohome`, `nogetenv`, `nochdir`, `oneoptset`), and parts 4b-4f, 5a-5d and
phases 21, 22 and 26 register with `RegisterGraph`. 2,308 lines of text
programs became 2,361 on the verbs (non-blank, non-comment; 26's
`editlit.go` folded in). **Phases 4-26 now run on the graph end to end**:
phase 3 hands its graph to 4, and no phase from 4 to 26 imports, prints
for a text step or sweeps -- the in-phase sweeps of 4, 5, 16, 19 and 20
are collections.

**Library** (`crefactor/graph/runverbs.go`, new, 224 lines, generic; its
tests `runverbs_test.go`, each against the text verb's C printed
canonically): `Run`/`CutRun` (a run of consecutive items by their
patterns, found once: a text literal of several lines); `SpliceFirst`
(from an item through the first after it of a shape: the text's splice to
a line); `DropCaseRun` (DropCase judging the statement before by
`StmtTerminates`, so a case after a block ending in a jump goes, as the
text's line cut did); `DropOperandAsText` (the operand left keeps the
parentheses the C view wrote around it in its operator, which a text cut
of ` && x` leaves: `while ((a, b))`); `HeadFold` (a text fold of a line
head: plain ifs or else-if arms of a condition, apart, folded one at a
time from the last, `what (N)` reported as noconv's text did); and
`Editor.EndLabels` (the `;` the print puts after a label left last in its
block). Nothing existing changed.

**Refinements of the catalogue.**
- No RENUM, INITROW-by-index or MOVE was needed. 4b's 18 mouse rows and
  the `[MOUSE]` row are `DeleteRows` with no position to say; 26's
  enumerators past `CMD_SIZE` are all still named here (0 go, as the text
  reported); 5b's 35 `firstwin`/`lastwin` uses are `RetargetUses` to
  `curwin`, the two declarations then cut.
- PARAM did what the text's literals did: win_line's, check_tty's,
  showmatches' two (with `Dangle`: the locals they initialised go to the
  collection, as the text left them to the sweep), comp_textwidth's. The
  text missed comp_textwidth's call in `format_lines` (left `(TRUE)`, an
  ill-typed call in a function the sweep takes); PARAM drops it too,
  so the C view before the collection differs there by those bytes and
  after it is the same.
- noconv's 386 calls through the ten mb_* pointers are each callee
  replaced by the function, found by the pointers' uses (edges), every use
  a call or refused; `Rederive` types the call again.
- A text head (`if (x)`) did not match `else if (x)`; where both occur
  (noconv's `converted`, 26's skip folds) `HeadFold` tells them apart.
- 5d's proof that nothing registers an autocommand is asked of the edges
  (no use of `first_autopat` is stored to); 5b's backreferences are a
  repeated `?name` in the walk's pattern.
- Each unit was proved alone before its phase (`internal/graphcheck`'s
  `TestB3aRecord`/`TestB3aUnit`, B3A_REF): the text step's output printed
  canonically against the graph step's C view on the same input, and its
  report line for line -- all 22 units the same but 5a's one call above.

**The proof**, rebased on main `4f96189`: `rm -rf .cache/boundaries; make
whim-build-check` in order, whim-vim.c byte for byte (77,635 lines), 104
boundaries compiling, 323 s; again in parallel, byte for byte, 28 links
begun on their graph snapshot (0 imported), 91 s. The control -- one act
dropped or changed in onebuffer, part 5b, nochdir and oneoptset -- is
named by the parallel check: phases 4 (onebuffer's own leftover count
refuses), 5, 10 (refuses) and 16, `4 of 103 phases do not reproduce their
snapshot`; a fifth, 22's `b_may_swap` store kept, fails
`TestPhasesOnGraph` (50 bytes). `whim-editor-check`, `whim-test` (80 as
HEAD, the Go editor 80), `go test ./...` in both modules, gofmt, vet and
staticcheck on what changed: clean. In order beside main at the same hour
(load 20-30): **313 s wall, 902 s CPU against main's 340 s and 978 s**;
phase 4 5 s (12 s), 5 4 s (10 s), 6, 9, 10, 21, 22 and 26 0-1 s (2-3 s):
three imports and one C view fewer (before 4's onebuffer, 5's start and
noconv, 16), and nine sweeps and canonical prints gone.
## B3c as built (2026-10-04)

Phases 36-42 run on the graph end to end -- 36's program, 37's, 38's three
(38a was B1b's; 38b and 38 now), 39's, 40's, 41's and 42's, every one a
graph step registered with `phase.RegisterGraph`, its text program
replaced. Phase 36 imports while 35 ends on text (B3b's); 37-42 are
handed the graph and import nothing; 43 (B3d's) begins on the text.
42's MIN/MAX are not left a text step: with offsetof they are MACROX's,
each invocation's arguments read from its text and the fragment the text
made of them.

**Library, in new files** (`crefactor/graph`; 687, 131 and 43 lines of Go,
comments and blank lines aside, with tests):

- `substc.go`, **SUBSTITUTE**: a text program's `sub(old, new, n)` on the
  graph. `Verbs.SubstituteC(Subst{In|Near, Old, New, N, What}...)` finds
  each place the old C stands (spaces aside, outside literals) as the
  INNERMOST run of items holding it -- in a function (`In`), among the
  top-level forms around a declaration (`Near`), or the scope -- then
  narrows to the smallest node whose C holds every byte the substitution
  changes (an expression, an item, a body; an insertion between two items,
  or after a run, is new items there and the run keeps its ids), shows that
  node stands where the old C stood (the run printed with a marker for it),
  and replaces it by its C substituted, through FRAG, all places in one
  synthesized import. `SubstituteSeq` and the `Substitutor` (`Add`,
  `Flush`, `Pending`, `Touched`) make a table of literals in the text's
  order, batching consecutive independent ones into one import: apart in
  the C (one run may hold several), none whose old C another's new C
  holds, none naming what another's new C declares and the graph does not
  resolve yet. LiteralC took whole items only; most of B3c's ~110 literals
  begin or end inside one.
- `addparamc.go`, **AddParamC**: AddParam (B2c) with the parameter in C,
  made by FRAG in each declaration's place -- a pointer to a function,
  which BUILD does not make (`void (*exit_fn)(int)`, 38 and 40).
- `storage.go`, **SetStorage**: a file-scope def's or defn's storage class
  (`static`, `extern`, none) in every declaration of its entity (38b's
  `main` -> `static vim_main`, after `Rename`).

**The conversions.** Every count is the text's question on the C view;
every edit the graph's: 36's thirteen sites and nine arms SUBSTITUTE, its
eighteen definitions a top-level fragment, the 610-identifier rename
`RetargetUses` from the sixteen externals to the musl_ definitions (a
literal cannot be reached); 37's block and tables fragments, its calls
`RetargetAs` + `Rederive` (towupper's wint_t result becomes int) and the
five classifiers musl's <ctype.h> spells as macros MACROX'd; 38b `Rename`,
`SetStorage` and a launcher fragment; 38 two fragments, a `Rewrite`,
`AddParamC`, a body; 39's 47-act table through one `Substitutor`, its
`delfunc`s `Delete`, its two index splices top-level forms (M1, the
prototypes, made first: FRAG resolves names where it writes them, and the
text wrote calls before their prototypes); 40 SUBSTITUTE, fragments and
`AddParamC`; 41 a fragment, `ReplaceRun`, `DropParams` and nine calls
`RetargetAs` the new prototypes; 42 fragments for the libc prototypes,
`time_T`, `elapsed_T` and musl_gettimeofday, the nine libc calls'
uses retargeted to the prototypes, `RetargetAs` for time_t and uintptr_t,
`Retype` for full_screen, got_int and every elapsed_T object and member,
the five gettimeofday calls rebuilt from their operand's C, and MACROX.
The reports are the text's line for line but where the text counted its
own lines before the canonical print (39, 40, 41's line numbers and
counts, 42's "adds exactly 24 lines", 38b/38's line arithmetic: dropped
or the view's), and "sweep" is "collect".

**What holds them.** `internal/graphcheck`'s `TestB3cAsImported`: each of
36-42 handed q(N-1)'s graph read back gives qN byte for byte AND a graph
that is the import of its C view (`SameGraph`: every refers edge where
cc's check puts it, every typed edge of the same structure) -- which is
what found 37's call types, 42's typedef and members and the libc calls'
edges, each fixed in the program rather than in the test.

**The proof.** After rebasing on main `4f96189`: `rm -rf .cache/boundaries;
make whim-build-check` in order, whim-vim.c byte for byte, 77,635 lines,
every boundary compiling, 405 s at a load of 30-60. Again, in parallel:
byte for byte, 77 s, 27 links begun on their graph snapshot. The control,
five faults at once (36's %p arm, 38b's launcher, 38's call, 40's
host_message, 42's offsetof), named phases 36, 40 and 42 and refused 38:
`4 of 103 phases do not reproduce their snapshot`. `whim-editor-check`,
`whim-test` (80 as HEAD, the Go editor all 80), `go test ./...` in both
modules, gofmt, go vet and staticcheck on what changed: clean.

**Measured** (`TestMeasureGraphPhases`, medians of 3; the text before at a
load of 3-6, the graph after at 23-33, so the graph is the slower side):

| phase | text, wall ms | imported, wall ms | handed the graph, wall ms |
| --- | ---: | ---: | ---: |
| 36 | 2,162 | 3,410 | 1,512 |
| 37 | 2,061 | 3,402 | 1,504 |
| 38 | 3,822 (38a on the graph) | 3,989 | 1,958 |
| 39 | 2,868 | 5,745 | 3,928 |
| 40 | 1,932 | 3,798 | 1,986 |
| 41 | 2,372 | 3,118 | 1,117 |
| 42 | 3,257 | 4,721 | 2,683 |

Handed the graph, the seven take 14.7 s against the text's 18.5 s; 38 no
longer pays B1b's import and C view. What they cost is FRAG's synthesized
imports (0.25-0.4 s each: 39 makes eight, as its table's dependencies
allow), not the edits. In a run in order 36 imports until 35 ends on the
graph.

**Refinements.** A text literal is not always items: SUBSTITUTE is the
general form of LiteralC, and keeps ids outside what changed. FRAG's
names resolve where a fragment is written, so a program whose text
inserted a call above its callee's prototype must order its acts
(39's M1). A call retargeted to a function of another result type needs
`Rederive`; a typedef made anew needs the declarations naming it typed
again (42); RETYPE on a typedef whose result is a function's leaves it
untyped where a fragment in its place does not.
## B3f as built (2026-10-03)

Thirty units of phases 57-98 run on the graph, byte for byte, their text
programs replaced (history keeps them, `16717ab` and before): phases 57,
59, 61, 63, 65-70, 72, 73, 75, 78-85, 93 and 96-98, phase 71 with its
parts 71a and 71b, part 77a and phase 86's own step (`whim86`). With
B1c's 58, 64, 77 and 86a, **phases 57-59, 61, 63-73, 75, 77-86, 93 and
96-98 are graph end to end**; in a run in order 58-59, 64-73, 78-86 and
97-98 are handed the graph by the phase before, and 57, 61, 63, 75, 77, 93
and 96 import (after a text phase: 56, 60, 62, 74, 76, 92, 95). Once B3g's
60, 62 and 74 are on the graph, 57-75 is one run.

**Left text, and why.**

- **76a and 76** (`optvar_T`, 390 lines of regular expressions): every
  option variable's kind is read off its declared type and every read
  rewritten by the kind -- a typed transform written in text. It waits for
  step 6, as the catalogue allowed; it is the one break in 57-86.
- **87a, 102 and 103** (`xform.BoolRet`, 1,020 lines): untyped (cc.Parse
  only), but an analysis over cc's tree -- the greatest fixed point of what
  only ever holds an answer -- whose port is a rewrite of the analysis on
  the graph's edges, the size of a B2 capability, not a conversion. Not
  attempted in this batch's budget.
- **87** (the key names): after 87a in the same phase, so on the graph it
  would import and end on the graph only to hand phase 88 (text) a C view:
  no saving until 87a moves. Not attempted.

**crefactor/graph, additively but one fix**:

- `protoparam.go`: `Editor.RenamePrototypeParams(fn, i, to)` -- a
  prototype's parameter names are its own (Rename respells the
  definition's); phases 59 and 67 rename one.
- `fragmore.go`: `Verbs.LiteralExprC(pat, old, new, n, what)` (a text
  literal that named part of an item: the pattern's matches whose C,
  printed alone, is old), `FragAt(spot, src, what)` (C at a spot no pattern
  names: before a definition that follows its prototype), `ReplaceEachC`,
  `AfterEachC`/`BeforeEachC` and `WrapEachC` (the caller's nodes; Wrap moves
  the node into its replacement as a hole, its ids kept: 96's error calls
  in their else). Each one act, counted, deferred in `Together`.
- **The fix, in `frag.go` and `edit.go`** (nine lines): SpliceC refused a
  fragment whose node referred to another fragment's node of the same unit
  when that one was spliced later ("a refers edge ... which the graph does
  not hold") -- order-dependent: 71a's helper and its calls passed, 71b's
  `esc_now` and 73's objects did not. The editor now holds the unit's nodes
  as pending while it splices (`Editor.pending`), so a unit's fragments
  name each other whichever goes first. `TestFragPendingEdges` is its test;
  its control (pending not set) fails it.
- Tests: `fragmore_test.go`, every verb and refusal against the text
  verb's C and the import of the result.

**Refinements of the catalogue's vocabulary.**

- **A member is no FRAG spot, so a struct that gains members is written
  whole**: 63's two typedefs, 72's `block_hdr`, 96's engine state -- FRAG
  at the definition's spot, its C the form's own C view (`FormsC`) with the
  text's literal applied. FRAG resolves every use again in the unit; but
  the typed edges of declarations elsewhere that name the old definition
  keep pointing at its type node, which the collection then empties:
  `internal/graphcheck`'s `TestB3fSame` (each phase on q(N-1)'s graph, held
  to the import of qN.c by `SameGraph`) names `typedef bhdr_T` (72) and
  `re_multiline`'s parameter (63). Unknown, not wrong; the bytes are the
  text's on every boundary and through the run. A MEMBER capability (or
  FRAG retargeting type nodes) is what would close it.
- **RETYPE refuses a function used as a value** (67's comparators, passed
  to the search), so their declarations are written anew by FRAG in one
  unit with the typed searches, which retargets every use.
- **RETYPE's and PARAM's `f(void)`** is a function type of no parameters
  where the importer's is one `void` parameter (`TestB3fSame` on 59 and
  66): a B2c difference in the type node, bytes unaffected.
- **BUILD leaves arithmetic untyped** (`(- 1)`, `'[' - 256`); where the
  type matters to the graph being the import's, FRAG writes it (71, 71a).
- **A selection chain is one form**: `x->bh_ptr->pb_pointer` is `(-> x
  bh_ptr pb_pointer)` as cc reads it, so a cast that was the operand of
  `->` is replaced together with that selection (72); the parentheses the
  text kept -- `(hp->bh_data)->db_...`, its cast matched inside the
  printer's parentheses -- are written where the text wrote them.
- **The text's literals that named part of an item** (75's grow
  conditions, 81's, 85's calls) are `LiteralExprC`; those whose overlap
  the text resolved by order (75's two `rp = ...` statements, 96's two
  `nmatch = ...`) are scoped or told apart by their context.
- **`?x:pat` would have helped** (a binding with a shape, B1a's wish too):
  73, 75, 82, 96 find the nodes and hand them to the `...EachC` verbs.
- **A text program that computed C** (69's switch, 71a's helper, 71b's
  calls) is a graph program that reads the shapes from the forms and
  writes the new C by FRAG; the nodes it keeps it moves (71's skipped
  code by MoveRun into a BUILD else; 71a's switch into a BUILD loop; 78's
  arguments by MoveTo).
- B2c's whole-phase tests for 57, 66, 78, 80 and 83 became those phases'
  programs and left `b2c_test.go`.

**The proof** (rebased on main `4f96189`). `rm -rf .cache/boundaries; make
whim-build-check` in order: whim-vim.c byte for byte, 77,635 lines, 317 s
wall, 1,073 s CPU (868 user, 205 system) at a load of 10-22, 48 graph
snapshots written (21 before); before the rebase, on 16717ab, 310 s and
869 s. Again, in parallel: byte for byte, 104 boundaries compiling, 107 s at
a load of 37-69; 48 links began on the graph, 36 read from their snapshot,
12 imported. The control, five faults at once -- 59's cast kept, 69's
switch on `((int)*(p)) + 0`, 75's count `0`, 86's `col = 0` kept, 96's
report alone -- named exactly 59, 69, 75 and 86 (`4 of 103 phases do not
reproduce their snapshot`): a report is not a byte. `whim-editor-check`,
`whim-test` (80 as HEAD; the Go editor 80 as the C), `go test ./...` in
both modules, gofmt, go vet and staticcheck on what changed: clean.

**Measured**, one phase at a time (`TestMeasureGraphPhases`, 3 runs,
medians, load 10-30), main 16717ab's text beside B3f the way a run in
order takes each phase: the 28 converted phases 59.2 s against 27.7 s.
Handed the graph a phase costs 0.21-1.0 s against 1.6-4.6 s on text (86:
4.6 s to 0.22 s; 67, 71, 72, 73, 82: 0.8-1.5 s, their FRAG units); where
it imports (57, 61, 63, 75, 77, 93, 96) it costs what the text did, 1.9-2.7
s (63 and 75 about 1 s more: an import against a cheap sweep).
## B4 as built (2026-10-04): the front

Phases 1-3 run on the graph, byte for byte, every text program of theirs
replaced (history keeps them, `19c8e86` and before): the 47 cutters and
parts the three front steps call -- D1-D5, D6-D8 with part 2a and phase
18's program, D9-D12 with parts 3a-3f -- each a function of
`*graph.Editor` (`internal/cut`) or a program registered with
`phase.RegisterGraph`, and phase 1's `noruntime` and phase 2's
`query-empty` graph steps. **`front`, `front2` and `front3` are graph
steps** (`internal/steps/front.go`): what is unwritten asked first
(`Editor.Unwritten`), the cutters in order, and then **FoldX** (`Before`
that, `Hold: frontHold`) where `xform.FallOutOf` was, its report the text
closure's line for line but the order-dependent counts B2d named. Phase 0
ends on text; **phase 1 imports q000, and phases 2-26 are each handed the
graph** -- no import, no sweep, no canonical print between phase 1 and
phase 27's own text neighbours. `xform.FallOutOf` is deleted (and its
`unwrittenNames`/`unwrittenOf`), with `steps.FrontCut` and
`TestFoldXFront`; **`xform.FallOut` stays**: `crefactor/graph`'s
`foldx_test.go` and graphcheck's `TestFoldXEverySeed` hold FoldX to it.
About 5,150 lines of text programs became 4,200 on the graph (non-blank,
non-comment; `utf8only`'s text simplifier counted in both).

**How each unit was proved.** Before any conversion, the front's cutters
ran one at a time on q(N-1) as text, each one's raw output and its
canonical print kept; each graph cutter was then run on the import of the
raw text before it and its C view -- no closure, no sweep -- held to the
text cutter's output printed canonically, byte for byte, its report line
for line (`TestB4Record`/`TestB4Unit`, gone with the text cutters; history
keeps them). All 47 matched, but nomemfile at one call, below.

**Written on the verbs** (B0, B3a's runverbs, FRAG, INITROW, RENUM,
RENAME, PARAM), but one:
- **extable** reads the table, the `CMD_index` enum and the prefix index
  as forms and runs the text's old-and-new lookup model on them; the stub
  rows' enumerators move after `CMD_SIZE` FIRST (`MoveEnumerators`,
  Renumber) and then the rows go by `ArrangeRowsTyped`, so the table is
  typed for its new length (rows deleted first left `cmdnames`' typed edge
  cleared); 111 abbreviations in one FRAG unit; the member retyped and
  renamed before the `(int)` cast is dropped. argvfront, filefront: B2b's
  proofs (`renum_test.go`) now hold the graph cutters. exfront is
  `cut.ExFront`, `cut.Retire` and steps' `exFront` gone.
- **utf8only is a DRAFT** (B3e): its acts are a text simplifier's, ~600
  one-rewrite rounds over marker tokens in 231 functions, and its report
  counts those rounds (321 simplifications, 309 folds), which a port to
  forms could not keep without reimplementing it. It runs on the C view and
  commits as FRAG (0 untyped), plus `labelsBeforeDecls`: the `(empty)`
  statement the print writes between a label and a declaration the folds
  leave adjacent. It costs about 14 s of phase 2's 17 -- the one front unit
  worth rewriting on the verbs next.
- **Typed tables.** `DeleteRows` leaves a table's definition untyped (no
  array type of the new length); FoldX's member seed then cannot see the
  table as an initialised instance, and phase 2 folded `tab[i].context` to
  0 in ExpandOther (116 bytes off q002.c). Every front table deletion is
  typed (`ArrangeRowsTyped`; `cut`'s `deleteRowsTyped`, which took the four
  `DeleteRows` of nolocale, nosignals and noucmd): each unit leaves 0
  expressions untyped.
- **The text's ill-typed accidents are not reproduced where PARAM does
  better**: nomemfile's text rewrote only the first `mf_open` call and left
  ml_recover's passing two arguments to a function of none; PARAM drops
  them at every call, so the C view differs at that line before the
  collection (ml_recover goes) and not after -- as B3a's 5a.
- **Order.** The graph refuses deleting a definition still called
  (`mf_release`, `set_init_default_maxmemtot`, nocomplkeys' `docomplete`
  label after its gotos): the calls go first, the bytes the same.

**Library**: `crefactor/graph/b42_rows.go` (`Editor.DeleteRowsAsWritten`,
with a test): a table's rows deleted with no constant subscript renumbered
-- `options[0]` and `&options[0]` mean the first row whatever it is, which
`DeleteRows` refuses. Nothing else in `crefactor/graph` changed. Two gaps
the converters met and worked around: a pattern that binds a node and
constrains its shape (`?x:pat`), and a verb for a typed row deletion.

**The proof.** `rm -rf .cache/boundaries; make whim-build-check` in order:
whim-vim.c byte for byte (77,635 lines), 104 boundaries compiling, **182
s** wall, 10 min 24 s of CPU, at a load of 12-34 (92 graph snapshots).
Again, in parallel: byte for byte, **49 s**, 92 phases begun on the graph,
80 from their snapshot. The control -- noruntime writing `vimruntime =
TRUE`, noterm respelling `"xterm-88color"`, nofnamemod keeping
`skip_mod = TRUE` -- is named exactly: phases 1, 2 and 3, `3 of 103 phases
do not reproduce their snapshot`. `TestPhasesOnGraph` 1-3 (imported and
handed), `whim-editor-check`, `whim-test`, `go test ./...` in both
modules, gofmt, vet and staticcheck on what changed.

**Measured** (one run each, the text in, as a run in order takes phase 1;
main `19c8e86` beside it at the same hour):

| phase | main (text), wall / CPU s | B4, wall / CPU s | how |
| --- | ---: | ---: | --- |
| 1 | 30.0 / 73 | 15.1 / 33 (13 in order) | an import (3.5 s), the cutters, FoldX, a collection |
| 2 | 23.0 / 43 | 21.2 / 48 (17 in order, handed) | handed; utf8only's DRAFT 14 s |
| 3 | 28.4 / 52 | 11.9 / 25 (8 in order, handed) | handed |

In order the three are 38 s against main's ~81; with B3's phases on the
graph beside them the whole build in order is 182 s.
