# GRAPH.md -- the program as a graph, its views as trees, printed as Lisp

2026-10-03. A design, not a plan of record; its steps 1-4 are built
(`crefactor/graph`) and measured, in *Steps 1-3 as built* and *Step 4 as
built*, near the end, and so is step 5's foundation, the pipeline running
text and graph phases side by side (*Step 5 as built*), and a first, read-only
taste of its views (`crefactor/graph/view`, `whim view`: *Views,
read-only*); the rest is not. It asks
what representation the pipeline, and later an editor, would hold a C program
in, if the C text were no longer the thing edited. It rests on two measured
pieces: C-lisp (`doc/C-LISP.md`: C23 as s-expressions, byte for byte against
cemit's canonical text) and the pilot of phases editing C-lisp's tree
(`doc/C-LISP-TREE.md`), whose numbers are the case made here.

## The problem: two representations, translated back and forth

Every phase today edits **canonical text** (about 70 phase packages, 382
regexp patterns, `crefactor/edit`'s verbs), and every phase ends by parsing
it again: the sweep parses and resolves names by cc's scopes, the canonical
print parses again when the sweep cut something, and the typed transforms
(`FallOut`, `BoolRet`, `MemberOut`, `StateParam`, `LocalOut`,
`StructScalar`, `plainc`) type-check it with `cc.Translate`. Phase 17,
profiled: the collector 43% of its CPU, `cc.Parse` 24%, two parses.

The pilot put a tree beside the text -- C to C-lisp, an edit on the forms,
back to C -- and measured what a mixed representation costs:

| | text | tree |
|---|---|---|
| reading the program (q023) | `cc.Parse` 600 ms | the `.lc` read, 46 ms |
| printing it | cemit 604 ms | the forms, 45 ms |
| phase 24's own rewrite | 860 ms | 172 ms |
| a phase, end to end (phases 8, 17, 18, 24) | 2.4-3.2 s | 3.9-4.0 s |

The tree wins every operation that happens *on* it and loses the phase:
converting into it (1.3 s) costs more than any edit saves, and the sweep
and print (2.4 s) still run on text. The gain appears only when nothing is
converted -- estimated, not measured, a phase from 2.4 s to 0.3-0.5 s and
about 200 of the in-order build's 451 s. And the pilot's tree, being
C's syntax alone, could not do what a third of the work needs: 3,843 member
uses on q023 are ambiguous by name without types, names inside macro text
are invisible, and the typed transforms still want cc.

So the lesson is not "a tree instead of text" but **one representation,
resolved and typed when it is built, never reparsed**. Names connected to
their definitions make it a graph, not a tree.

## The design

### One graph, the source of truth

- **Nodes with identities, not positions.** Every declaration, statement,
  expression, type, member, enumerator, label and include is a node with a
  stable id. An id survives edits around it; a new node gets a fresh id; a
  node a rewrite replaces says which ids it supersedes.
- **Edges of kinds:**
  - **contains** -- a file its declarations, a function its body, a struct
    its members, a statement its expressions (C's syntax, in order: the
    one spanning tree the C and C-lisp printers walk);
  - **refers** -- a use to its definition: identifiers, members (resolved by
    type, which is what the pilot lacked), tags, labels, typedef names;
  - **typed** -- an expression or declarator to its type node; types are
    nodes too (a struct type is the node its members hang from, so a member
    use refers to the member through the type);
  - **derived** -- calls, reads, writes, address-taken, the facts the
    analyses compute today and throw away: `cfacts`' layouts and effects,
    `ccx`'s evaluation order, the out-parameters, which object a function
    reaches. Each derived edge kind names the analysis that keeps it.
- **Built once, from cc.** The importer is cc's parse plus `cc.Translate`'s
  types and resolution, turned into nodes and edges a single time. From then
  on nothing parses C: the graph is authoritative, and C is printed from it.

### Edits under constraints

- **The constraints are the graph's invariants:** every reference resolves;
  every typed node agrees with its context (the conversions C makes
  implicitly are explicit nodes, as `doc/IR.md` asks of an IR); every
  contains-child is in a place its parent's grammar allows.
- **An edit is a graph operation** -- delete a node, replace a subtree,
  insert, retarget a reference -- and the constraints are checked **where
  the edit was made**, not by the next parse or by gcc phases later. An edit
  that breaks one is refused at the spot, with the violated edge named.
- **Fall-out is the closure of the constraints.** Deleting a definition
  leaves references dangling; rules close over them until none do: a use of
  a deleted constant becomes its value, a call of a deleted function goes,
  a condition that became constant folds, a statement left empty goes, a
  local nothing reads goes. That is `xform.FallOut`'s and the fall-out
  closure's work today, re-derived per phase from reparsed text; here it is
  the constraint system's propagation, and it is the same rules, written
  once, for every cut.
- **The sweep is garbage collection:** what no root reaches -- `main`, the
  static asserts, what the profile pins -- is collected. Reachability over
  refers-edges is exactly what `crefactor/sweep`'s closure computes by
  name; on the graph it needs no parse.

### Views: the graph seen as a tree

A **view** is a projection of the graph to a tree: a **root**, the **edge
kinds** that count as children, and **where to stop** (a depth, a kind of
node, a set of ids). Where following an edge would revisit a node the view
already holds, it prints a link instead, so every view is a tree -- and every
tree prints trivially as an s-expression.

- **The containment view of the file is C-lisp**, with ids. The **C view**
  is the same walk printed in C's syntax, cemit's spelling, for gcc and for
  the snapshots.
- **A feature view**, rooted at an option: its uses, grouped by the function
  holding them, each with the statement around it. The code a cut of that
  option touches, gathered from wherever the file holds it.
- **A callers view**, rooted at a function: who calls it, recursively, to a
  depth. **A member view**: every read and write of `buf_T.b_ml`, with
  the expressions around them. **A type view**: a struct, its members, the
  functions taking it.

```lisp
;; containment: one function, ids shown
(defn #f17 ascii_isupper ((#p31 c int)) int
  (return (< (- (cast unsigned @p31) #\A) 26)))

;; a feature view: rooted at an option, children its uses by function
(view (uses #o88 p_wiv)
  (in #f203 set_option_value  (= @o88 #e9120))
  (in #f511 win_redr_status   (if @o88 #s7741)))
```

`#id` marks a node and `@id` refers to one (as built: `#N` before a node,
`@N` and `@:N` after it, *The Lisp* below); a view may print names and hide
the ids, and show them on demand. **A view is editable**: every printed form
carries its node's id, so an edit made in the view -- delete this use,
replace that condition -- is a graph operation on known nodes, checked
against the constraints like any other. A phase becomes: choose a view,
rewrite it, let the closure run.

### Lisp, not JSON

Every view is a tree, so its serialisation is an s-expression with no
escape hatches: heads are the node kinds, atoms are C's tokens (C-lisp's
rule), `#id`/`@id` are the graph's two additions. The **whole graph** is
serialised as its containment view of the file plus the type nodes, with
every refers-edge as an `@id` -- readable, diffable, and enough to rebuild
the graph without parsing C: the importer's result can be stored and read
back in tens of milliseconds (the pilot's `.lc` read, 46 ms, is the
measure). Derived edges are not serialised; they are recomputed by their
analyses, or cached under the graph's digest.

## How it relates to what is here

- **C-lisp** is the containment view without ids, and its byte-exact round
  trip is the proof obligation the graph inherits: the C view must print
  cemit's text byte for byte, or the 104 snapshots stop being the proof.
- **The pilot's tree API** (`crefactor/clisp`'s cursors, patterns and
  resolver) is a sketch of a view's editing surface; its resolver is the
  refers-edges of the untyped part.
- **`doc/IR.md`'s IR** sits downstream of whim-vim.c, lowered and typed, for
  the backends. The graph sits at C's level, upstream, for the pipeline. They
  can be **one store with layers**: the lowered form, the instance pass and
  `cfacts`' decisions are derived edges and nodes over the same graph, and a
  backend is a printer of a lowered view. `doc/IR-SCHEMA.md`'s schema is a
  candidate for that layer's node kinds.
- **The typed transforms** become graph rewrites that read typed and derived
  edges directly instead of calling `cc.Translate` on text: the 15 or so
  phases the pilot could not reach are where the graph earns most.

## The hard parts

- **C's semantics in the invariants.** Implicit conversions, decay, integer
  promotions, evaluation order, lvalue-ness. Taking the types from
  `cc.Translate` at import covers the start; keeping them right after an
  edit is a local re-check per edit, which is a type checker's rules written
  over the graph -- the largest single piece.
- **Identity across rewrites.** Which ids a rewrite keeps decides what views
  and caches survive it; the rule must be explicit (a replacement names what
  it supersedes) or the derived edges go stale silently.
- **Macro residue.** Recovered macros (`nullptr`, `errno`, the opaque
  invocations) are nodes whose inside is text today; their references must
  become edges, or they stay invisible to the closure as they were to the
  pilot.
- **Exact printing.** The C view must reproduce cemit's choices that are not
  in the tree (the parentheses the source kept, `sizeof x` bare): C-lisp
  already carries them as forms, so the graph carries them as node
  attributes.
- **The headers.** Names the system headers declare are not in the file;
  they are external nodes, typed from `cc.Translate`'s view of the headers.

## The way there, each step with a gate

1. **The importer** (built): cc's parse and types into the graph, with ids and
   refers-, typed- and contains-edges. *Gate:* the C view of the imported
   graph is the input byte for byte, on every snapshot q000-q103.
2. **Serialisation** (built): the graph as Lisp, and a reader. *Gate:* write, read,
   print C: byte for byte, and the read under a tenth of `cc.Parse`.
3. **The sweep as collection** (built). *Gate:* on every phase's output before its
   sweep, the collected graph's C view is the swept text byte for byte.
4. **One cut as deletion plus closure** (built), a drop phase the pilot measured
   (24, or the `DropLocal` phases). *Gate:* qN byte for byte from
   q(N-1), with its counts.
5. **The pipeline on the graph**, phase by phase, the graph handed from one
   to the next and C printed only for the snapshots. *Gate:*
   `whim-build-check` unchanged; the in-order time measured against 451 s.
   Its stage A is built (the hybrid driver, the graph snapshots, 14 phases'
   cuts); stage B is `doc/GRAPH-MIGRATION.md`'s batches.
6. **The typed transforms** onto typed edges, `cc.Translate` gone from them.

Steps 1-3 are worth building on their own: they prove the representation is
lossless and resolved, and they are what an editor needs. Steps 4-6 are the
pipeline's migration, which the pilot's numbers price at about 22,000 lines
re-proven for about 200 s of a build that runs when upstream moves -- so they
are justified by what the graph enables, not by speed.

## The editor at the end

A vim whose buffer is a **view of the graph** rather than a file of text:

- **The view is rooted where the cursor is**, and repositioned on the fly:
  on a function, its body and its callers; on an option, every use of it,
  gathered from wherever the C file scatters them, under one root.
- **Edits in the view are graph edits**, checked against the constraints as
  they are made; a delete that leaves a reference dangling shows its
  fall-out before it is committed.
- **The text is Lisp**, so the motions are structural -- a form, its
  siblings, its parent -- and the printing is trivial; the C view is one
  more view, read-only or edited through the same graph.

Prior art to read before designing it: Code Bubbles (Brown University,
2010: working sets of scattered fragments shown together), JetBrains MPS
(projectional editing), Glamorous Toolkit (views shaped to the object),
Unison (code stored by identity, text only a rendering), Hazel (structure
editing that keeps every state well-formed). None puts a modal editor over
Lisp projections of a C program's graph.

## Decided for steps 1-3 (2026-10-03)

Starting easy, each choice the simpler of two, the other kept as the
direction:

- **Ids are sequential at import and preserved through edits**; a new node
  gets a fresh id. *Later:* content-addressed ids, Unison's choice --
  identical subtrees one node, edits copy-on-write -- are the direction
  preferred once the graph stands.
- **The serialisation has its own reader**, C's tokens as atoms as in
  C-lisp, `#id` and `@id` as written above. *Later:* a Clojure-readable
  (EDN) form is preferred, so that Clojure tools and an editor read the
  graph for nothing; the C tokens would then be strings or tagged literals.
- **Step 1 takes cc's types at import**, so members resolve by type (the
  pilot's 3,843 ambiguous uses) and the typed edges exist from the start.

Defaults, not decisions: Go, a generic crefactor package naming nothing in
vim; the headers' declarations external nodes typed from cc; the serialised
graph not tracked; the lowered layer after steps 1-3; the sweep's guards
reproduced exactly in step 3.

## Steps 1-3 as built (2026-10-03)

`crefactor/graph`, about 3,100 lines of Go (comments and blank lines aside),
naming nothing in vim; `internal/graphcheck` holds its sweep to the
pipeline's, and `go tool whim graph` is the tool (`graph FILE` the Lisp,
`graph --check FILE...` steps 1 and 2 on each, `graph --collect FILE` the
sweep as collection). Against go-whim `03bf458` and its snapshots, on the
64-core machine.

### What the graph is

- **Containment is C-lisp's.** The importer is cc's parse, cc's type check
  of the same tree (`AST.Check`, exported from the fork for this: the check
  `Translate` runs, keeping what it resolved when it fails, since a text
  before its sweep need not type-check), and C-lisp's conversion with a hook
  that tells it, for every form and identifier atom, the cc node it came
  from (`clisp.Options.Origin`). Its forms become the nodes: **every list is
  a node with an id; an atom is a node with an id when an edge starts from
  it** (a use of a name), and otherwise a token of its form. What cemit keeps
  from the source that the tree has not -- the parentheses it kept, `sizeof
  x` bare, a recovered macro's text, the include lines -- C-lisp already has
  as forms (`paren`, `sizeof-bare`, `macro`, `include`), so the graph has them
  as nodes, not as attributes.
- **Refers edges**, a use to its declaration's form: an identifier to its
  `def`, `defn`, parameter, enumerator or local (the declaration cc's check
  found, else the one the parser's scopes say is visible); a member to the
  member, BY THE TYPE of what it selects from (the field cc's check found,
  `PostfixExpression.Field`); a designator `.x` to the member, by the type of
  the object its braces initialise; a tag to its definition; a typedef name
  to its typedef; a label to its `(label L)`; and a macro's invocation --
  `(macro "va_arg(ap, int)")`, `errno`, `nullptr` -- to every name its
  expansion uses, which is how names inside macro text stop being invisible.
  Every use has its edge: what nothing declares is an external node of its
  own (`(undeclared semsg)`, `(unresolved-member b_next)`), so that "every
  reference resolves" is a property of the graph, not of its luck.
- **Typed edges**, from every expression's form and every declaration's
  (a def, a parameter, a member, an enumerator) to a **type node**:
  `(basic int)`, `(pointer @:T)`, `(array 10 @:T)`, `(function (@:P ...)
  @:R)`, interned by structure (1,648 for whim-vim.c). A struct, union or
  enum type is its definition's own form in the file. An atom's type is not
  an edge: an identifier's is its declaration's, a literal's its spelling's.
  Qualifiers and typedef names stay the forms' business; a typedef is its
  type.
- **External nodes**, what the headers declare that the file uses, typed
  from cc's view of the headers: `(extern errno)`, `(extern-typedef
  size_t)`, `(extern-struct stat (member st_size) ...)` with the members
  named, `(extern-enumerator ...)`.
- **Ids** are sequential, in the order of the containment, then the types,
  then the externs, given by one interface (`IDs`: `Next`, `Saw`) that
  `Import` and `Read` use and an edit's `Graph.Fresh` asks; nothing else
  makes an id, so content addresses are a second `IDs` and a pass, not a
  rewrite. `Equal` is identity by id: the same nodes, texts, elements, and
  edges to the same ids.

### The Lisp

The containment view with the graph's marks, then two top-level lists,
`(types ...)` and `(externs ...)`; from q103, whim-vim.c's first function:

```lisp
#1(defn static inline ascii_isupper
  #2(fn #3(#4(c int)@:237424) bool)
  #5(return #6(< #7(- #8(cast unsigned #9:c@4)@:237428 'A')@:237428 26)@:237424))@:237426

(types
  #237424(basic int)
  #237425(basic _Bool)
  #237426(function #237427(@:237424) @:237425)
  ...)
(externs
  #239073(extern-typedef time_t)@:237430
  ...)
```

`#N` before a node is its id -- before a list's `(`, and before an atom with
`:` between, `#9:c`; `@N` after a node is a refers edge, `@:N` a typed edge,
and a type's operand is an edge alone. The four marks are constants in one
file (`lisp.go`), the atoms are C-lisp's (C's tokens), and the reader is its
own: it reads the text as one string and slices the atoms from it, the nodes
and their elements and edges allocated in slabs, 63 allocations for
whim-vim.c's 430,000 nodes and tokens. The `(types` and `(externs` heads
cannot be a top-level form's: C-lisp's are `def`, `defn`, `struct` and the
like. The Lisp is not tracked; `go tool whim graph F` writes it.

### The gates

| gate | on | result |
| --- | --- | --- |
| 1: the C view of the imported graph is the text | q000-q103, the 104 snapshots | **104 of 104 byte for byte** (`TestCorpus`) |
| 2: written, read back: the same graph (ids, every edge), its C view the text | q000-q103 | **104 of 104** (`TestCorpus`) |
| 2: the read under a tenth of cc's parse and check | q000-q103, one at a time, best of 3 | **104 of 104**, 13.8 to 26.1 times faster, median 19.3 (`TestCorpusTimes`) |
| 3: each phase's text before its sweep, imported, collected: its C view is qN | phases 0-103 (0 on slim-vim.c's seed) | **104 of 104 byte for byte**, and on every one the same counts as the sweep's, kind by kind (functions, objects, prototypes, typedefs, tags, members, enumerators, locals, fallthroughs, pinned survivors, kept runs, positional structs, rounds), no edge left dangling (`internal/graphcheck`'s `TestCollect`) |

**The controls.** Step 1: a token changed in the graph changes the C view.
Step 2: one edge retargeted in the Lisp, `@N` made `@N1`, is caught by
`Equal`. Step 3: the collected graph's last form dropped is caught on all
104 phases; and the graph's own member rule (below) is not the sweep's, which
the gate sees on 103 of 104 phases. Each is a test (`TestControls`,
`TestCollectControl`). Unit tests hold the collection to the sweep, its
oracle, on crefactor/sweep's own cases and the graph's (orphaned
fallthroughs, `T *(a[3])` filled by position, anonymous members, headers'
structs and typedefs, designators, labels), every one through Read first, so
the collection is shown to need no cc node behind the graph.

### Measured

Steps 1 and 2, one file at a time, best of 3 (`TestCorpusTimes`):

| | cc's parse and check (`cemit.Parse`, `Check`) | the import | the read of the Lisp | the read, times faster |
| --- | ---: | ---: | ---: | ---: |
| q000, 4.8 MB of C, 14.8 MB of Lisp | 1,439 ms | 3,921 ms | 72 ms | 20.0 |
| q001, 4.2 MB, 13.0 MB | 1,303 ms | 3,287 ms | 68 ms | 19.0 |
| q023, 2.2 MB, 6.7 MB | 823 ms | 1,905 ms | 40 ms | 20.8 |
| whim-vim.c, 2.1 MB, 6.4 MB | 788 ms | 1,654 ms | 31 ms | 25.1 |
| the 104, median | 779 ms | 1,727 ms | 40 ms | 19.3 (13.8 to 26.1) |
| the 104, total | 83.5 s | 188.5 s | 4.3 s | |

The import costs 2.0 to 2.7 times cc's parse and check, median 2.2: C-lisp's conversion
with its origin map, then the resolution. It is made once; the read is what
the graph costs after that.

| graph of | nodes | of them lists | atoms with an id | tokens | refers edges | typed edges | type nodes | externs | Lisp, bytes |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| q000 (the seed, 4.8 MB of C) | 545,058 | 327,547 | 217,511 | 431,788 | 218,004 | 228,853 | 2,683 | 236 | 14,815,474 |
| q023 (2.2 MB) | 253,555 | 153,907 | 99,648 | 207,483 | 99,954 | 110,060 | 1,654 | 121 | 6,732,821 |
| whim-vim.c (2.1 MB) | 239,125 | 142,721 | 96,404 | 192,569 | 96,585 | 102,080 | 1,648 | 54 | 6,397,609 |

**What resolves.** On every snapshot, every use: 0 unresolved identifiers,
members, designators, typedef names, tags or labels, 0 expression forms cc
left untyped, cc's check clean on all 104. Members by type:

| | member uses | resolved by type | of a name more than one struct has | of those resolved |
| --- | ---: | ---: | ---: | ---: |
| q023 | 16,641 | 16,641 | 3,876 (the pilot: 3,843 by its count) | 3,876 |
| whim-vim.c | 17,520 | 17,520 | 4,390 | 4,390 |
| q000 | 30,195 | 30,195 | 7,628 | 7,628 |

whim-vim.c: 72,198 identifier uses (32,604 to the file's declarations, 39,570
to locals, 24 to the headers'), 6,633 typedef names, 107 tags, 49 labels, 26
members of the headers' structs; 3,144 macro invocations (nearly all
`nullptr`), 78 refers edges from their expansions. The pilot counted 3,843
ambiguous uses on q023; this count, 3,876, includes the members of anonymous
structs and unions. **vim has no designated initializer**; the designators
are tested on the samples.

**What does not resolve, and why.** On the snapshots, nothing. Before a
sweep, where a phase's edit has left the text momentarily inconsistent, six
of the 104 texts hold a use nothing declares, and the graph is built all the
same, the use's edge to an `(undeclared NAME)` or `(unresolved-member NAME)`
node, and the collection on each is byte for byte. Three are calls in the
core to a function whose prototype the edit took while the call survives in
code the sweep then takes -- `set_sigwinch_handler` at phase 39,
`musl_gettimeofday` at 44, `getpid` at 49: an implicit declaration, which
cc's check lets by and gcc 15 refuses, and which only the edges show; three
fail cc's check: phase 4's (part 4d deletes `buf_T.b_next` while two uses of it survive,
which the sweep then takes with what holds them: two `(unresolved-member
b_next)`, and two of `wim_flags_arg`, a parameter gone), phase 32's (two locals initialised from parameters the edit
removed, `ffname_arg` and `sfname_arg`, which the sweep then takes as locals
nothing reads: two `(undeclared ...)`), and phase 72's (`static_assert(sizeof(DATA_BL)
== 8 + DB_LINE_MAX * sizeof(DATA_LN), ...)` fails until the sweep has cut the
members the edit left dead; every use resolves). Names
in macro text are resolved through the expansion; what stays opaque is the
text of an attribute the forms keep whole (`attr-text`) and of an `asm`, and
the collection scans those by name, as the sweep does.

Step 3, phase by phase, one at a time (`GRAPH_JOBS=1`), wall:

| | the sweep (`Prune`, its parses) | the import | the collection | the C view |
| --- | ---: | ---: | ---: | ---: |
| over phases 0-103 | 134.1 s | 192.4 s | **15.4 s** | 7.0 s |
| a phase, median | 0.99 s | 1.75 s | **0.13 s** | 0.06 s |
| the slowest (phase 1, the front) | 5.09 s | 3.77 s | 0.41 s | 0.15 s |

The collection is 7-15 times the sweep's speed, median 8, and the C view
replaces the canonical print (0.6-1.3 s a phase today) at 0.05-0.16 s. A
graph handed from phase to phase would pay neither the sweep's parse nor
the print's; the import is what it would pay once.

**Members by type.** The sweep keeps a member while any live use names a
member of that name (`p->next` keeps every `next`); the graph knows which
member each use is (`CollectOptions.MembersByType`). On the snapshots, which
the sweep has swept already, the graph's rule would take 1 member more from
whim-vim.c (`xp_context`, of an anonymous struct, named only as another
struct's) and from 38 more boundaries, 4 from 58 of the middle
(`dictitem16_S.di_flags`, `re_flags`, `re_engine`, `xp_context`), 13 from
q003 and 0 from q000, where `ml_recover` freezes every layout; and moves the text of 103 of the 104
phases' sweeps (`TestCorpusMembersByType`, `TestCollectControl`). It is not
the gate's rule: the gate is the sweep's text.

### How the sweep became collection

The collector is Prune's closure on the nodes: the same entities (a
top-level form, a struct's member, an enumerator), the same keys, the same
guards (no member while `ml_recover` is defined, a struct filled by position
keeps its members and the structs it holds by value, nothing emptied, an
enumerator's deletion pinning the survivor after it to the value Prune's
evaluator gives, now on the forms), the unused locals by their declarations'
incoming edges, the orphaned fallthroughs by where control goes next, the
rounds. A name's key is read off the graph, not the text: a use whose edge is
to a file-scope declaration or an external is `o:NAME`, one to a local is
none -- the parser's scopes, which the sweep consults by offset, are the
edges here. Where the sweep's text rule is not the graph's own, the
collector says the sweep's on purpose, since the gate is the sweep's text:
members by name; a tag by its name wherever it is written; a label's and a
local typedef's name as a file-scope key; an initialiser's array depth
counted after the last parenthesised declarator, as Prune's loop over the
direct declarator stops there, so `static histentry_T *(history[5]) =
{...}` counts as filled by position (a first version counted through the
parentheses, and the gate's counts -- 29 positional structs against 30 --
found it, the text being the same). The collection also drops the type and
external nodes nothing left reaches; an external struct lives while one of
its members does (a first version dropped `fd_set` under three macro
invocations that name `fds_bits`, and the dangling-edge check found it).

### Steps 4-6, given what was measured

- **Step 4 (one cut as deletion and closure)** was ready to start, and is
  built (*Step 4 as built*, below): the graph
  has the edges a cut asks about (phase 24's "every mention is a bare call"
  is an incoming-edge count, members included), and the collection is the
  closure's last half already. What is missing is the edit surface: a
  delete that refuses to leave an edge dangling, a replace that says which
  ids it supersedes, and the fall-out rules (`FallOut`'s: a constant's use
  its value, a folded condition, a statement left empty) on the forms.
  `clisp`'s cursors and patterns carry over; they want the edges.
- **Step 5 (the pipeline on the graph)** is priced better now. Per phase,
  the sweep and the canonical print (about 2.4 s together today, the
  pilot's profile) would be the collection and nothing (0.13 s; the C view only for a snapshot,
  0.06 s): some 230 s of the 451-s in-order build, a little more than the
  pilot's estimate of 200.
  But a phase still written on text costs an import, 1.75 s median, more
  than it saves, so converting phase by phase is slower until the last is
  converted -- the pilot's conclusion stands, and the graph does not change
  it. The import itself could be cheaper by building the nodes directly
  rather than C-lisp's forms and an origin map; it is 2.2 times cc's parse
  and check now.
- **Step 6 (the typed transforms)** has its inputs: 102,080 typed edges on
  whim-vim.c, every expression typed, every member resolved by type. What
  the transforms ask cc today -- a declarator's type kind, a selection's
  struct, a function's parameters, whether a type is a scalar -- is a walk
  of type nodes. Keeping the types right after an edit is the part not
  begun: the check would be local to the edit, and it is a type checker's
  rules written over the graph.
- **Content-addressed ids**: a second `IDs` and a numbering pass over the
  containment bottom-up; the type nodes' cycles (a struct holding a pointer
  to itself) need Unison's treatment of a cycle as one unit. The Lisp then
  writes hashes where it writes numbers; nothing else names an id's form.
- **An EDN form**: the four marks become tagged literals and C's tokens
  strings; the containment, the sections and the edges stay as they are.

## Views, read-only (2026-10-03)

The first taste of the view idea, before any editing: `crefactor/graph/view`
(about 1,200 lines of Go, comments and blank lines aside, naming nothing in vim) projects the graph to
trees, and `go tool whim view` prints them. It reads the graph and changes
nothing; it needed of `crefactor/graph` only five exported wrappers of what
the forms already said (`names.go`: `DeclName`, `DeclType`, `Tag`,
`IsTypeDef`, `Members`), no signature changed.

### What a view is

**An index**, made once in one walk (15-20 ms on whim-vim.c), holds what the
edges say the other way round: each node's parent, each declaration's uses
(its incoming refers edges), the nodes each type node types, and the
top-level forms by the name they declare -- so that a function's
prototypes and its definition, an object's declarations and its definition,
are one entity, as C's linkage makes them (on whim-vim.c every call of
`ml_get` refers to its prototype, none to its definition).

**A spec** is a relation, a depth, the heads where it stops, and how much to
show around each use. The relation is written in steps, each from a set of
nodes to another, and every named view is one:

| step | from a node to |
| --- | --- |
| `refers` / `refers<` | its declaration (the entity's: a function's definition) / every use of the entity |
| `typed` / `typed<` | its type node / every node it types |
| `contains` / `contains<` | its elements / its parent |
| `inside` | every node it contains |
| `^fn`, `^stmt` | the top-level form holding it (the entity's), the statement holding it |
| `call` | itself, if a call calls it: `(call NODE ...)`; else nothing |

`callers` is `refers< call ^fn`, `callees` is `inside call refers`, `uses`
is `refers< ^fn`. A child's **context** is the use the edge was crossed
at -- the referring end of the last refers or typed edge followed, whichever
way -- shown as the use alone (`--show node`: the call, the selection, the
atom), its statement (`stmt`, the default: the blocks and initialiser
elements that hold no use elided as `...`), its whole function (`fn`), or
not at all (`none`).

**The tree** is built depth first, children in id order -- the source's --
and **a child the view already holds is a link**, `@NAME` (`@ID NAME` with
ids), its contexts shown and its own children not: a recursive function's
callers end on itself, and an unlimited view is finite. Printed, every form
is C-lisp's, laid out by C-lisp's rules; `--ids` puts the graph's marks on
it -- `#ID` on every list and every use, `@ID` after a use its refers edge --
so that every printed form names its node (the typed edges are left to
`whim graph`).

### The named views

`whim view [--ids] [--depth N] [--show S] [--stop HEADS] VIEW ARG [FILE]`;
a root is a name (a function's definition, an object, a typedef, an
enumerator, a header's declaration), `S.M` (a member, S a typedef name or a
tag, found in anonymous members too), `struct T`, `F/x` (the parameters and
locals named x in F) or `#ID`.

- **`callers F`**: each function holding a call of F, with the calling
  statements, and its callers, to `--depth` (2; 0 for no limit). A note
  counts the uses that are not calls -- a function stored in a table.
- **`callees F`**: each function a call in F's body names, with the calls,
  and its callees. A call through a pointer is to the pointer (a
  parameter, a member); one through an expression, `table[0](2)`, is to
  nothing the graph resolves, and is not shown.
- **`uses NAME`**: every use, grouped by the top-level form holding it, each
  statement labelled with what its uses do -- `:call`, `:read`, `:write`,
  `:update` (`+=`, `++`), `:addr`, `:init` (a designator), `:type`,
  `:unevaluated` (in `sizeof` or `typeof`), `:label` -- read off the forms
  around the use: C's syntax, not an analysis of what a pointer reaches, so
  a write through `&b->b_ml` is the `:addr` it starts from. Writing a member
  of a struct member, or an element of an array member, writes the member.
- **`member S.M`**: `uses` of a member, which the graph resolves by type:
  struct other's `b_ml` is not buf_T's.
- **`type T`**: a struct's (union's, enum's) members with their uses
  counted by access, the functions taking it (by value or through pointers,
  the parameters shown) and returning it, the other structs' members and
  the file's objects of it.
- **`def NAME`**: the definition's containment view, C-lisp, with ids on
  `--ids`; `--c` prints its C view.
- **`follow 'STEPS' ROOT`**: an ad hoc view, its relation the steps.

On whim-vim.c (q103), from the cache. The feature view of an option, with
the option table's other entries elided:

```lisp
;; 7 uses of p_wiv in 4 top-level forms: 4 read, 2 write, 1 addr
(uses p_wiv
  (in options
    (:addr
      (def static options (array (struct vimoption))
        (init ...
          (init "weirdinvert" "wiv" ...
            (init (addr p_wiv) nullptr nullptr 0)
            PV_NONE
            did_set_weirdinvert
            nullptr
            ...)
          ...))))
  (in did_set_weirdinvert
    (:read
      (if (&& p_wiv (! (. (-> args os_oldval) boolean)))
        ...
        (if (&& (! p_wiv) (. (-> args os_oldval) boolean)) ...)))
    (:write (= p_wiv (paren (!= (deref (paren (index term_strings (cast int (paren KS_XS))))) NUL)))))
  (in screen_line
    (:read (if redraw_this ... (if (&& p_wiv (> (+ col coloff) 0) (> off_to 0)) ...)))
    ...)
  (in ttest
    (:write (= p_wiv (paren (!= (deref (paren (index term_strings (cast int (paren KS_XS))))) NUL))))))
```

Who calls `ml_get` (72 calls in 42 functions; to depth 2, 483 lines), a
function the view holds already a link:

```lisp
;; 72 calls of ml_get in 42 functions
(callers ml_get
  ...
  (in truncate_line
    (= old_line (call ml_get lnum))
    (in @op_delete
      (call truncate_line FALSE)
      (call truncate_line TRUE)))
  ...
  (in match_range
    (= (. (index lines k) string) (call ml_get (+ line1 k)))
    (in ex_substitute
      (def found (ptr linefound_T)
        (? (&& (! (. subflags do_ask)) (> line2 (-> eap line1)))
          (call match_range (addr regmatch) (. subflags do_all) (-> eap line1) line2)
          nullptr)))
    (in ex_global ...))
```

With no limit, `callers ml_get` holds 998 functions (27,026 lines) and
`callees main` 1,362, every other meeting a link. With ids, every form is
its node:

```lisp
;; 27 calls of ml_get_buf in 23 functions
(callers #93556 ml_get_buf
  (in #16343 linetabsize
    #16356(call #16357:ml_get_buf@5479
      #16358(-> #16359:wp@16346 #16360:w_buffer@3133)
      #16361:lnum@16349
      #16362:FALSE@1900))
  ...
```

Members by type, counted (the first line of `member S.M`):

| member | uses | in top-level forms | read | write | update | unevaluated |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `buf_T.b_ml` | 376 | 127 | 310 | 47 | 19 | |
| `pos_T.lnum` | 1,248 | 206 | 902 | 229 | 67 | 50 (phase 100's `typeof`) |
| `memline_T.ml_line_count` | 217 | 105 | 212 | 3 | 2 | |

and `type pos_T`: 3 members, taken by 41 functions, returned by 7, held in 25
members, 6 objects:

```lisp
(type pos_T
  (members
    (member lnum linenr_T (uses 1248 (read 902) (write 229) (update 67) (unevaluated 50)))
    (member col colnr_T (uses 1120 (read 703) (write 253) (update 113) (unevaluated 51)))
    (member coladd colnr_T (uses 419 (read 249) (write 117) (update 5) (unevaluated 48))))
  (taking
    (fn getvcol (pos (ptr pos_T)))
    ...
```

### What was measured and tested

| on whim-vim.c, at a load of 10 | wall |
| --- | ---: |
| the first view, the graph imported (and cached: `.cache/graph/`, 6.4 MB of Lisp) | 1.58-1.66 s (320-340 MB) |
| a later view: the cache read, 39-48 ms; indexed, 15-20 ms; the view built and printed, 0-4 ms | **0.06-0.07 s** (100 MB) |
| the largest, unlimited: `callers ml_get`, `callees main`, built and printed | 44 ms, 74 ms |
| the same through `go tool whim view` | 0.17 s |

**The cache** is the graph's Lisp under `.cache/graph/`, one file a source
file, headed by its key: the source's SHA-256 and the running binary's size
and time, so that a rebuilt `whim` -- which may import differently -- makes
it again rather than read a stale graph; a graph's Lisp handed as FILE is
read as it is. The read is what makes a view fit an editor's keystroke:
re-rooting is a read-free rebuild of the tree, milliseconds, once the
graph is in memory.

**The tests** (`crefactor/graph/view`, on a graph imported and read back,
so no cc node is behind it): every named view on a small C text against
golden output -- a recursive function's callers ending on a link, mutual
recursion closed by one with no depth limit, ids, a member told from
another struct's of the same name by type, each access label; the answers
stated beside the goldens; the same text twice and on the graph imported
and read back. On whim-vim.c when it is there: `uses X` shows exactly the
refers edges into X's declarations, each once, counted by walking the
graph and not the index, for eight roots (a function, an option, a
typedef, a global, two members, an enumerator (`NUL`), a
local); `def F`'s C view is F's text in the file for all 1,755
definitions; `callers F` to depth 1 is exactly the functions holding a
call. The control: `refers<` taking the root's own uses and not its
entity's shows 0 of `ml_get`'s 72 -- each refers to the prototype.

### What an editor would need next

- **Where the cursor is**: the printer knows each form's id as it writes
  it; a span table (id to the text's range, and back) is what roots a view
  at the cursor and keeps the ids out of sight when `--ids` is off.
- **Edits through the view**: every form carries its id, so a changed form
  says which node it was; the edit operations are step 4's (another
  worktree's), and the view is printed again from the edited graph.
- **Derived edges**: calls through function pointers and tables are
  invisible to `callers` and `callees`, and what is written through a
  pointer is the `:addr` it came from; both want the derived layer (a
  points-to answer, `cfacts`' effects) as edges a step can follow.
- **The graph as the source**: once edits land in the graph, the cache is
  no longer a cache of the C text but the program itself, written after
  each edit.
## Step 4 as built (2026-10-03)

`crefactor/graph`'s `edit.go` (the editor), `fallout.go` (the closure) and
`pattern.go` (clisp's patterns matched on nodes), naming nothing in vim;
`internal/whim/graph.go`, what the closure is told about vim; and
`internal/graphcut`, whim's, which wrote `DropLocal` and phase 24 as
deletions on the graph and held them to the snapshots; the plan did not
run it. Step 5 moved the two cuts into the plan (`internal/cut/droplocal.go`,
`internal/phase/024/edit.go`) and removed the package, its gate now the
build check's (*Step 5 as built*). Against go-whim `a329f9b` and its
snapshots, on the 64-core machine.

### The edit API and its invariants

`graph.NewEditor(g)` indexes a graph -- each node's container (a pointer
in the node, set by the index; one editor per graph at a time) and the
refers edges by target id, two walks and no map, 17 ms on q023 -- and makes
four operations, each checked where it is made, refused with the place or
the edge named:

- **`Delete(n)`, `Replace(old, with...)`, `InsertBefore/After(at, ns...)`**,
  one splice underneath. **Places**: a node goes, or is replaced by other
  than one node, only where its container's grammar lets it: an item of a
  block, a statement expression, a function's body or the file; a struct's
  member; an enumerator (not one whose successor's value is implicit:
  deleting it would move that value -- *the value of #8 (B), after it, is
  implicit and would move*); an if's else. A body is replaced by one block;
  any other place -- a condition, an operand, a callee, a function's
  parameters, which are its type -- by exactly one node, an expression by an
  expression (*an expression's place in #63 (call) cannot hold a return*).
  The type and external sections are the collection's. **Containment**: a
  node is contained once -- a replacement may move nodes out of what it
  replaces, and holds no other node the graph has. **References**: a new
  node's edges go to nodes the graph holds. An edge INTO a node an edit
  removes is not refused: it becomes a **dangling record** (`Dangling()`),
  which the closure must discharge or refuse, naming it.
- **`Retarget(use, i, to)`**: a use that spells a name must spell its new
  target's (*retarget #83 `ready` to #14: it declares `counter`*): the C
  view prints the name, not the edge.
- **`Check()`**, for tests: the whole graph against the invariants -- every
  node contained once and indexed where it is, every edge to a node held or
  recorded dangling. The gate runs it after every graph cut.

**The id rule.** A node keeps its id while it is in the graph, wherever an
edit moves it (the items an if's taken branch splices into its block keep
theirs). A node an edit removes takes its id with it: superseded, never given
again, and a removed node brought back is refused. A node an edit brings in
gets a fresh id from the graph's `IDs` (a list, or an atom an edge starts
from). Every edit is logged (`Log`: what it superseded, moved and gave), the
record a view or a cache keyed by id needs. Phase 24 on the graph is 86 acts.

**Types.** A deletion leaves every remaining typed edge right: an item or a
member gone changes no other node's type (a struct's type is its form, which
loses the member); a parameter cannot be deleted (it is the function's type).
A replacement re-derives one thing: an expression replaced by one of the same
type node, or a decimal literal in an `int`'s place, leaves the expressions
above it typed. Otherwise it CLEARS the typed edges above it, up to the
statement, and lists them (`Untyped`): unknown, never wrong; a new expression
without a typed edge is listed too. The re-check that would type them again
is step 6's. On every gate phase: 0 untyped.

### The fall-out rules

`Editor.FallOut(opt)` closes over the dangling records, round after round,
and refuses with the first it cannot discharge (`*Unhandled`: *#15554
`b_p_ro` in fileinfo refers to #2646 (b_p_ro), which an edit deleted, and no
rule takes it (8 such edges)*). At a dangling use, the item holding it, the
cut's own rules first (`opt.Rules`), then:

- **call**: `f(args);` -- a statement, or cast to void -- of a deleted
  function goes, its arguments free of side effects. In an expression
  context, its value used, nothing is taken: the cut must say what the
  value is (`Replace`), and the closure refuses -- which is phase 24's "every
  mention is a bare call" asked of the edges.
- **store**: an expression statement whose only effect is on deleted
  locations goes: `x = E;`, `p->m += E;`, `++x;`, `a[i] = E;`, E free of
  side effects or itself such a store (`new_wp->w_id = ++last_win_id;`,
  both deleted). A call of a function the options call **Pure** -- one that
  only allocates -- is free of them.
- **through**: a call of a function the options say acts only on what its
  arguments point at (**Through**), every argument a deleted location's
  address, goes.
- **value**: a read of a deleted object nothing writes, of a constant value
  (its initialiser; zero at file scope) is that value -- `xform.FallOut`'s
  seed rule; a pointer's is `((void *)V)`.

Then what became constant, starting only from what an edit wrote (as
`xform.FallOut` fires only on its own marks): an `int` expression of
constants is its value, `c ? a : b` of a constant c its branch, an `if` of a
constant condition the branch it takes (a block's items spliced, the block
kept whole when it declares anything), `while (0)` goes; and unless
`KeepEmpty`, a block an edit emptied goes when it is an item or an else, and
an `if` whose only branch it was, its condition free of side effects. Then
the caller collects (step 3). Not written, since no gate exercises them:
`xform.FallOut`'s later rules -- `a && K` losing its K, a function whose body
is `return K;` made K at its calls, a parameter every call passes alike.
Unit tests (`edit_test.go`, every one on a graph read back from its Lisp, no
cc node behind it) hold each rule, and the collection after it, to the sweep
and print of the C written by hand, and each refusal to its message.

**What vim tells it** (`internal/whim/graph.go`, as the sweep is told its
roots): `vim_strsave` is Pure; `check_string_option` and
`clear_string_option` are Through; and `KeepEmpty`, because the text cutters
leave the blocks they empty -- `if (ready) {}` is in q024.

**Where the text cutters do what the generic rules do not**, each said
where it is:

- **DropLocal's get_varp case** -- `case X: return (char_u *)&(curbuf->F);`,
  and the `? ... : p->var` form -- goes with its case label: a cut's own
  `Rule` (`getVarp`), not a generic one, since dropping a case changes where
  the switch goes, which only this cut decides.
- **The text's shapes are narrower than the rules**: DropLocal's regexps
  take `buf->F = ...;` and `curbuf->F = -1;`, the store rule any store to
  the field. On every gate phase the two take the same statements, since
  the text refuses any other (*still has N mentions*); off the gate, the
  graph's would take more, and is the more general by construction.
- **Phase 24's statements cut in named functions** are not statements on the
  graph: the cut DELETES the seven locations they were the last writes of
  (`incsearch_state_T.winid`, `win_T.w_id`, `last_win_id`,
  `autocmd_blocked`, `autocmd_no_enter`, `autocmd_no_leave`,
  `redrawing_for_callback`), the store rule takes exactly the ten
  statements the text cuts -- asserted, function by function and shape by
  shape, none more -- and the collection takes `LOWEST_WIN_ID`.
- **Phase 24's two guards**: the cut says what the text's `FoldNever` says,
  that `is_state.winid != curwin->w_id` is never true with one window, as
  `Replace(condition, 0)`, and the generic fold takes the ifs.
- **The twelve empty functions** are deleted, each checked empty first, and
  the call rule takes their 46 calls.

### Composition

`RunGraph(phase, q(N-1))`: the text steps run as the plan runs them; before
the first of a run of graph steps the text is imported -- or, when it is the
phase's first step, q(N-1)'s graph is handed in, read from its Lisp; the run
edits one graph; a text step after it gets the graph's C view. A phase that
ends on the graph is collected and printed -- no sweep, no canonical print;
one that ends on text (4, 5, 34) is finished as the plan finishes it. An
in-phase `sweep` step stays a text step.

### The gate

| check | on | result |
| --- | --- | --- |
| qN byte for byte from q(N-1), the graph steps on the graph | the 13 DropLocal phases (4, 5, 7, 8, 12-14, 16-20, 34) and 24 | **14 of 14** (`TestGraphBytes`) |
| the same, q(N-1)'s graph handed in, read from its Lisp | 8, 17, 24 (those whose first step is a graph step) | **3 of 3** |
| field by field, on the text the steps before leave: the text's sites and the graph's, the text's result printed canonically and the graph's C view | 64 fields | **64 of 64**, the same counts (3-6 sites each) and the same C (`TestDropLocalSteps`), the invariants checked after each |
| phase 24's report | q023 | **the text's, line for line**: 12 call counts (46 calls), the fold, the ten acts, `prechar` (`TestP24Report`) |
| a reader left | `b_p_ro` on q016 | **both refuse**, both say 8 mentions (`TestDropLocalRefuses`) |

**Controls** (`TestGraphControl`), each moving the bytes: the get_varp
rule leaving the case label (phase 17, `BV_CMS` kept: 2,237,003 bytes
against 2,236,832); the guards taken as always true instead of never (phase
24: the guarded call stays); and the closure's own empty-block rule turned
on (phase 24: `if (ready) {}` goes, 2,163,912 against 2,164,016) -- the one
generic rule the text cutters do not have, measured.

### Measured

A/B, each phase as the plan runs it (text), with its graph steps on a graph
imported from q(N-1) (graph), and on q(N-1)'s graph read from its Lisp
(handed: the read, 51-53 ms, timed apart), alternately, 5 runs each, one at
a time, load 2-3; medians, ms (`TestMeasurePhases`):

| phase | text: the cut | text: total (wall / CPU) | graph: import | graph: index | graph: cut and closure | graph: collection | graph: C view | graph: total | handed: total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 8 (2 fields) | 11 | 2,362 / 4,673 | 2,055 | 27 | 13 | 141 | 80 | 2,318 / 4,296 | **250** / 351 |
| 17 (3 fields) | 18 | 2,387 / 4,581 | 1,943 | 25 | 19 | 137 | 75 | 2,202 / 4,249 | **235** / 317 |
| 18 (1 field, after a text step) | 8 | 2,301 / 4,645 | 1,888 | 26 | 6 | 136 | 77 | 2,140 / 4,311 | -- |
| 24 | 781 | 3,072 / 5,197 | 1,877 | 23 | 13 | 124 | 69 | 2,103 / 4,021 | **216** / 306 |

- **The graph's own work** -- index, cut, closure, collection, C view -- is
  216-262 ms against the text's 2.3-3.1 s: **10-14 times** when the graph is
  handed in. The cut with its closure is 6-19 ms: phase 24's 781 ms of
  regexps is 13 ms (the pilot's tree, 172), DropLocal's 8-18 ms 6-19 (the
  pilot's tree, 73-77: its atom index was most of it). The collection is
  most of what is left.
- **The index** was 160-200 ms in a first version, maps of every node's
  container and every edge's target, more than the cut, the closure and the
  C view together; a container pointer in the node and the edges grouped by
  target id in two walks made it 17-32 ms.
- **With the import, a phase that ends on the graph is no slower than the
  text phase** (2.1-2.3 s against 2.3-3.1 s): the import, 1.9-2.1 s, costs
  what the sweep's parse and the canonical print cost together. The pilot's
  tree, which kept the sweep and the print, was 3.9-4.0 s; so converting a
  phase that ends on the graph no longer costs time on today's pipeline. One
  that ends on text (4, 5, 34) pays the import and the C view on top.

Lines of code, non-blank and non-comment:

| | text | the pilot's tree | the graph |
| --- | ---: | ---: | ---: |
| DropLocal | 87, and its step's wrapper 16 | 114 | 98 (the get_varp rule and the declaration's shapes in it) |
| phase 24 | 69 | 139 | 189 (the ten acts asserted against the closure's removals, the locations found by typedef) |
| the machinery | `crefactor/edit`, about 1,700 of its 2,241 | `tree.go`, `pattern.go`, `scope.go`: 1,136 | `edit.go` 681, `fallout.go` 424, `pattern.go` 77: 1,182, and the importer and collection beside |
| composing it with the plan | -- | `pilot.go` | `run.go`, 166 |
| what vim tells it | -- | -- | 10 |

The graph versions are not shorter either; phase 24 is the longest of the
three, because it asserts that the closure took exactly what the text took --
a cost of proving it equal, which a phase written for the graph first would
not pay. What is shorter is what each cut says it removes: three lists
(twelve functions, seven locations, a member) and two conditions, the rest
the closure's.

### What step 5 now needs

- **Every phase's cut on the graph**, or a bridge: a text step costs an
  import (1.9 s) unless the phase ends on the graph, where the import pays
  for itself; a graph handed from phase to phase pays it never. The 79 phase
  programs and 57 cutters are the work (about 22,000 lines, the pilot's
  count); the closure's rules grow with them -- `xform.FallOut`'s later
  rules for the front, phases 1-3.
- **The collection through the editor.** `Collect` cuts the forms directly
  and leaves an editor's index stale; a pipeline either rebuilds the index
  after each collection (17-32 ms) or makes the collection's cut a client of
  `splice`, which would also log its ids as superseded.
- **In-phase `sweep` steps as collections**, and the C view only for a
  snapshot.
- **The parallel check on the graph**: each phase reads q(N-1)'s graph
  (50 ms) instead of parsing it, so the snapshots would be kept as Lisp
  beside the C, or the C imported once per link (1.9 s).
- **The typed transforms** (step 6) for the 15 or so phases that ask
  `cc.Translate`; until then they import, and the editor's `Untyped` says
  what a replacement left for the re-check.
- **Ids across phases**: the `Log` is the record of what each phase
  superseded; content-addressed ids (*Decided*, later) would make an
  unchanged function the same node in every snapshot.

## Step 5 as built: stage A, the hybrid pipeline (2026-10-03)

The pipeline runs text phases and graph phases side by side, holds the
program as a graph while graph steps follow one another, and keeps the
graph beside the snapshots for the parallel check. 14 phases' cuts are graph
steps now -- every `droplocal` and phase 24 -- byte for byte. Stage B
converts the rest from `doc/GRAPH-MIGRATION.md`'s catalogue. Against go-whim
`291eded` and its snapshots, on the 64-core machine.

### The hybrid driver

`crefactor/pipeline` (`hybrid.go`; generic, it names nothing in vim):

- **A step is a text step or a graph step.** `Step.Graph` marks a graph
  step. Its op is `Config.GraphLookup`'s, a `GraphOp` that edits through
  the `graph.Editor` it is handed. Text is the default, so a plan without
  the mark runs as it did. whim's graph table is `internal/steps`' `graphOps`
  (`droplocal`, and `edit` for a phase registered with
  `phase.RegisterGraph`).
- **The program is held as one kind, converted only where the kind
  changes**: a text step after graph steps gets the graph's C view, and a
  graph step after text steps gets the text imported (`graph.Import`). A
  run of graph steps edits one graph through one editor.
- **The sweep after a phase** is the graph's collection when the phase ended
  on the graph, printed by the C view (cemit's canonical print byte for
  byte, gate 1). It is crefactor/sweep and cemit when the phase ended on
  text. **An in-phase `sweep` step** is a collection when the program is held
  as a graph there, through the editor, so the steps after it edit on the
  same index. Otherwise it is crefactor/sweep's on the text.
- **Between phases the text is always there**: a boundary is C, for the
  snapshot and the line counts. The graph goes on only to a phase that
  BEGINS on the graph, meaning its first step, sweeps aside, is a graph step
  (`BeginsOnGraph`). So a phase takes the same path whether it is run in
  order or from its snapshot: a phase that begins on text gets text either
  way. The graph handed on gets a **fresh editor**, because an editor's
  records (what was written, emptied, removed) belong to one phase's
  closure. Handing the old one across would let phase N's closure fold what
  phase N-1 wrote, which the check, starting from a snapshot, would not do.
- **The log** says what a phase spent on the graph: `...; graph: 1 import
  1707ms, 1 C view 57ms, 1 collection 120ms, 87 acts, 287 ids superseded, 0
  given`. A phase that ended on the graph reports its edits and collection
  together (`-151 edited and collected`), since nothing printed the text
  between them. `AdvanceFrom` is a phase handed a graph, and returns that
  account (`Conv`).

### The graph snapshots

`qNNN.g` sits beside `qNNN.c` in `.cache/boundaries/`. It holds the graph a
whole run handed from boundary NNN to the phase after it, when that phase
begins on the graph. It is the graph's Lisp, headed by a comment line naming
the SHA-256 of `qNNN.c`:

```lisp
;; the graph of q023.c, sha256 83a7754eb7af88b5a00758b3ad9c3a1c3b629022c9ce8b2fea05ef533115752f
;; a C translation unit as a graph: crefactor/graph
#1(include "<limits.h>")
...
(ids 253555)
```

- **Written** by a whole run, before the manifest seals the set; a run first
  removes the old ones. Today there are five, q007, q012, q013, q016 and
  q023, the boundaries before phases 8, 13, 14, 17 and 24, 6.7-7.1 MB each.
  Where a run already held the graph (phase 7, 12, 13 and 16 ended on it),
  the snapshot is that graph, its ids carried from phase to phase. Where the
  phase before ended on text (23), the run imports at the boundary and
  writes what it imported.
- **Read** by the parallel check: a phase that begins on the graph reads
  q(N-1).g (the read, 74-89 ms, a tenth of cc's parse) instead of importing
  q(N-1).c. It reads the file only when its header names q(N-1).c's digest.
  A graph snapshot of another text (left by another plan, or missing) is
  not read: the phase imports the text, which proves the same thing more
  slowly, and the check says how many links did which (`5 of them began on
  the graph: 5 read from its graph snapshot, 0 imported from the text`). It
  still compares every phase's C view with qN.c byte for byte and compiles
  every boundary.
- **Ids carry across.** A graph read back is the graph written: the same
  nodes and edges, and since stage A the same id allocator, through a new
  last section `(ids N)` that records the last id the graph's IDs gave
  (`Lasting`). A graph whose greatest ids an edit superseded would otherwise
  give them again once read back, which the id rule forbids. Every graph
  snapshot passes gate 2 (`internal/graphcheck`'s `TestGraphSnapshots`):
  read back, its C view is qNNN.c, and written again it reads back equal.
- **What each phase retired and created** is in its log line, counted from
  the editor's `Log`: the acts, the ids superseded, the ids given. Stage A's
  14 phases supersede 25-378 ids each and give none, because their fall-out
  adds only tokens (`0` for a folded condition). This is the record
  content-addressed ids would start from. Nothing else is written for it.

### Collecting through the editor

`Editor.Collect` is the collection made through the editor. The collector
records what its cut took: each removed node as a root, the forms a tag's
definition replaced, the enumerators a value was pinned on. The editor
clears each removed root's container, so nothing under it is `Live`. It
places a definition that took a top-level form's place in the file, places
a pinned value in its enumerator, and logs one `collect` act with the ids it
superseded. The refers index needs nothing, because a use the collection
took is filtered out on reading, as an edit's are.

The alternative was to collect and then make a new editor (`NewEditor`).
Both were measured on the text nine phases leave before their sweep, with 5
runs, medians (`TestMeasureCollect`):

| phase | ids collected | through the editor | collected and indexed anew |
| --- | ---: | ---: | ---: |
| 1 | 43,249 | 335 ms | 384 ms |
| 3 | 88,706 | 266 ms | 275 ms |
| 8 | 5 | 114 ms | 153 ms |
| 24 | 2 | 109 ms | 141 ms |
| 30 | 1,925 | 121 ms | 139 ms |
| 53 | 236 | 172 ms | 192 ms |
| 60 | 12 | 100 ms | 133 ms |
| 74 | 25 | 99 ms | 132 ms |
| 89 | 0 | 94 ms | 118 ms |

Through the editor is 9-49 ms faster everywhere, and it is the one that
logs the ids, so it is the driver's. `TestEditorCollect` holds it to
`Collect` on every sample: the same C, the same counts, the index whole
(`Check`), nothing it cut `Live`, and every superseded id in the act.
`TestEditCollectEdit` covers edit, collect, edit again.

### The first batch

The 13 phases with a `droplocal` step (4, 5, 7, 8, 12-14, 16-20, 34) and
phase 24 are graph steps in the plan. Their text programs are gone:
`internal/cut`'s `DropLocal` is the graph's (its six regexps and
`inLines` went), and phase 24's `edit.go` is the graph's, registered with
`RegisterGraph`. Nothing else called either. The CLI keeps
`whim droplocal F` and `whim edit whim24 F`: `steps.OnText` runs a graph
step on a file imported and writes its C view. `internal/graphcut` is
removed. Its byte gate is the build check's now, its control (the guards
taken as always) was repeated against the parallel check, and its
refusal test is `internal/cut`'s `TestDropLocalRefuses`.
`internal/treepilot`'s step test, which held the pilot's tree to the text
`DropLocal`, now holds it to the graph's. `internal/graphcheck`'s
`TestPhasesOnGraph` holds each phase with a graph step to its snapshot in
seconds, both from text and handed the graph.

Phases 8, 13, 14, 17 and 24 begin on the graph. 7, 8, 12-14, 16-20 and 24
end on it, collected and not swept. 4, 5 and 34 end on text.

### The proof

- `rm -rf .cache/boundaries; make whim-build-check`, in order: **whim-vim.c
  byte for byte**, 77,634 lines, every boundary compiling, the five graph
  snapshots written.
- `make whim-build-check` again, in parallel: **byte for byte**, 103 links,
  5 of them begun on their graph snapshot, every boundary compiling.
- The control: phase 24's guards replaced by `1` instead of `0`. The
  parallel check names it: `phase 24 gives 81748 lines where q024.c holds
  81746`, `1 of 103 phases do not reproduce their snapshot`.
- `crefactor/pipeline`'s hybrid tests run the toy plan with a graph phase
  that begins on the graph, one that turns from graph to text, and one that
  begins with a sweep on the graph. They check that the hybrid plan gives the
  text plan's product, that the run writes exactly the graph snapshots its
  plan reads, and that the check reads them (3 of 3), imports where one is
  missing or of another text (1 read, 2 imported), and names a changed graph
  step.

### Measured

At a load of 3-13 (agents reading beside it), main `291eded` against
`step5a`, the same machine and the same hour:

| | main (all text) | step 5a (14 phases' cuts on the graph) |
| --- | ---: | ---: |
| the in-order build check, wall | 352 s | 348 s |
| its CPU (user + system, gcc's compiles included) | 958 s | 936 s |
| the parallel check, wall | 74 s (64 s the links, 9 s the compiles) | 73 s (61 s, 10 s) |
| its CPU | 1,585 s | 1,477 s |

The CLAUDE.md figure of 451 s was measured under a load of 54-62. Re-measured
on main in these conditions it is 352 s.

Phase by phase, one at a time, 5 runs, medians: text on main
(`build.Advance`), and on step5a from the text (the graph imported where its
graph steps begin, as a run in order does after a text phase, and as the
check does without a graph snapshot) and handed the graph (as a run in order
after a graph phase, and as the check from qNNN.g: the read timed apart)
(`TestMeasureGraphPhases`):

| phase | main, wall / CPU ms | step 5a from text, wall / CPU ms | its import / C view / collection ms | handed the graph, wall / CPU ms (and the read) |
| --- | ---: | ---: | ---: | ---: |
| 4 | 10,057 / 23,831 | 15,178 / 35,748 | 2 imports 5,145 / 198 / -- | |
| 5 | 8,002 / 19,703 | 10,200 / 24,101 | 2,188 / 68 / -- | |
| 7 | 4,733 / 9,913 | 4,604 / 9,388 | 2,113 / 81 / 144 | |
| 8 | 2,471 / 5,131 | 2,279 / 4,714 | 2,001 / 63 / 148 | **218 / 315** (89) |
| 12 | 3,800 / 7,862 | 2,554 / 5,156 | 2,119 / 81 / 253 | |
| 13 | 3,726 / 7,677 | 3,292 / 6,132 | 1,961 / 63 / 148 | **368 / 529** (74) |
| 14 | 3,683 / 7,810 | 3,241 / 6,472 | 2,079 / 58 / 129 | **358 / 487** (79) |
| 16 | 3,667 / 7,628 | 3,568 / 7,166 | 2,000 / 80 / 133 | |
| 17 | 2,409 / 5,079 | 2,291 / 4,763 | 2,069 / 92 / 142 | **226 / 335** (85) |
| 18 | 2,538 / 5,337 | 2,315 / 4,765 | 2,082 / 82 / 139 | |
| 19 | 5,889 / 12,300 | 5,552 / 11,762 | 2,036 / 90 / 133 | |
| 20 | 4,867 / 11,231 | 4,633 / 10,436 | 2,016 / 79 / 162 | |
| 24 | 3,286 / 6,432 | 2,121 / 4,170 | 1,888 / 55 / 121 | **214 / 311** (76) |
| 34 | 2,644 / 5,540 | 4,401 / 9,293 | 1,839 / 71 / -- | |

- **A phase handed the graph is 7-15 times faster than on text**: 0.21-0.37
  s against 2.4-3.7 s. 13 and 14 cost more than 8, 17 and 24 because their
  in-phase sweep is a second collection.
- **A phase that imports and ends on the graph is as fast as on text, or a
  little faster** (7, 12, 16-20, 24: 0.1-1.2 s less). The import costs what
  the sweep's parse and the canonical print it replaces cost.
- **A phase that imports and ends on text pays both**: 4 (two imports), 5
  and 34 are 1.8-5.1 s slower, because their text steps after the graph's
  still need the sweep.
- **The conversions in the plan as it stands**, in the in-order run: 11
  imports, 20.2 s in all (at q003-q033, 1.6-2.3 s each); 15 C views, 0.9 s;
  13 collections, 1.7 s. In the run, phases 8, 13, 14 and 17 are handed the
  graph the phase before left: the 4-5 s they gain is about what 4, 5 and 34
  lose. That is why the in-order total barely moves. The gain arrives as
  contiguous runs of phases convert: B1a's would make 17-20 import-free.
- **In the parallel check** (64 links at once, so every time is inflated by
  the load) the five links read from their graph snapshot take 1.6-3.0 s,
  their collections most of it. The links that import take 1.6-18 s for the
  import alone. The check is bound by its longest links (the front's), so
  its wall time does not move.

### What stage B is

`doc/GRAPH-MIGRATION.md`: every phase and part (190 rows) classified as
(a) expressible now, (b) needing a named capability, (c) a typed transform
for step 6, or (d) staying text, with its lines of code, and batches to hand
out:

- **B0**: a shared library on the graph (crefactor/edit's verbs, small node
  constructors, the text's assertions as queries).
- **B1**: the a-class conversions.
- **B2**: five capabilities side by side: FRAG (C fragments as nodes in
  context), INITROW/RENUM/RENAME, PARAM/RETYPE/MOVE, the missing fall-out
  rules, INCLUDE.
- **B3**: the b-class conversions in contiguous ranges.
- **B4**: the front, last, behind `FallOutOf`.

Only the seed (phase 0) and phase 88 (gcc) stay text. Phase 43 stays text
unless gcc's three answers become edge queries.

## Open questions

- Is the derived layer cached by the graph's digest, or kept incrementally
  under edits -- and which analyses are cheap enough to recompute per view?
- Does the lowered layer of `doc/IR.md` live in the same store from the
  start, or join it once steps 1-3 stand?
- What is the smallest editor that proves the view idea? The read-only
  browser is built (*Views, read-only*), its views in milliseconds once the
  graph is read; what it lacks for a buffer -- a span table from the
  printed text to the ids, the edits of step 4 -- is listed there.
