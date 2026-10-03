# GRAPH.md -- the program as a graph, its views as trees, printed as Lisp

2026-10-03. A design, not a plan of record: nothing here is built. It asks
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

`#id` marks a node and `@id` refers to one; a view may print names and hide
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

1. **The importer**: cc's parse and types into the graph, with ids and
   refers-, typed- and contains-edges. *Gate:* the C view of the imported
   graph is the input byte for byte, on every snapshot q000-q103.
2. **Serialisation**: the graph as Lisp, and a reader. *Gate:* write, read,
   print C: byte for byte, and the read under a tenth of `cc.Parse`.
3. **The sweep as collection.** *Gate:* on every phase's output before its
   sweep, the collected graph's C view is the swept text byte for byte.
4. **One cut as deletion plus closure**, a drop phase the pilot measured
   (24, or the `DropLocal` phases). *Gate:* qN byte for byte from
   q(N-1), with its counts.
5. **The pipeline on the graph**, phase by phase, the graph handed from one
   to the next and C printed only for the snapshots. *Gate:*
   `whim-build-check` unchanged; the in-order time measured against 451 s.
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

## Open questions

- Is the derived layer cached by the graph's digest, or kept incrementally
  under edits -- and which analyses are cheap enough to recompute per view?
- Does the lowered layer of `doc/IR.md` live in the same store from the
  start, or join it once steps 1-3 stand?
- What is the smallest editor that proves the view idea -- a read-only
  browser of views over whim-vim.c's graph, before any editing?
