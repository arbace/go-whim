# doc/

Investigations that measured something and changed no code. Each was written to
answer one question, each states what it measured and how, and **each is dated
evidence rather than doctrine**: a later measurement may have overturned part of
it, and where one has, this file says so. Read the corrections before acting on
a survey.

They are kept because the measurements cost hours and cannot be recovered from
the tree: a survey records what a text said on a day, and the pipeline moves.

| file | the question | answer |
| --- | --- | --- |
| `DEADSWEEP.md` | could `deadsweep` work from the AST instead of gcc's warnings? | yes on agreement (474/474, one AST-only `static inline`), blocked by its position in the round |
| `DEADENUMS.md` | could `deadenums` take enumerator values from the AST instead of DWARF? | yes, and freely: 28,608 values over 17 texts, zero disagreements |
| `REACHABILITY.md` | could one reachability closure replace the sweep's six tools? | it replaces their ANALYSIS, not their deleting half; `sweep ∖ closure = ∅` over 1,059 removals |
| `AST-EDITING.md` | could the phases edit the AST instead of the text? | **no**, and the reason is a silent one — see below |

## Corrections, newest first

**"The intermediate text does not parse" is FALSE** — it appears in `DEADSWEEP.md`,
`DEADENUMS.md` and `REACHABILITY.md`, and all three mean something weaker than they
say. The text is syntactically valid C throughout; what fails between an edit and
its sweep is SEMANTIC ANALYSIS. `internal/cc` exposes `cc.Parse` separately from
`cc.Translate`, and measured over phases 13-35 applied one at a time from q12 with
no trailing sweep: **`cc.Parse` failed 0 times at all 24 texts**, while `Translate`
failed after 7 of 23, in three windows (25-27, 29-31, 35), each closed by the next
sweep. The real error is `type winopt_T has no member named wo_eiw` — a member
removed by `internal/cut/nosession.go` while three uses survive in functions the
sweep is about to delete as dead. That state is deliberate: the alternative is
every phase computing the transitive closure of what its edit kills, which is what
the sweep does once for all of them.

So there are three bars, not one, and they must be kept apart: **parses** /
**type-checks** / **is reachability-analysable**. The intermediates clear the
first, fail the second, and the third depends on the second.

**`internal/cemit` needs no type information** — verified independently of the
survey that proposed it. Its 1,482 lines contain four hits for `.Type()`,
`.Value()`, `typer`, `IsTypename`, `ResolvedTo` and `.Field(`, and all four are
`reflect` in `macro.go`'s walk. Substituting `cc.Parse` for `cc.Translate` gives
byte-identical output on `whim-vim.c` (80,134 lines) and `editor.c` (78,223), and
canonicalises the non-type-checking reproduction to a fixpoint with the dangling
member reference intact. `enum : long` survives because the printer takes it from
the `EnumSpecifier`'s syntax. NOT a speed change: measured end to end, 1.38 s
against 1.43 s.

**`DEADSWEEP.md`'s "enum values are the one thing not a function of the bytes
alone" is FALSE**, contradicted twice by different instruments: `DEADENUMS.md`
(28,608 DWARF values over 17 texts, zero disagreements) and `REACHABILITY.md`
(1,155/1,155 on the product). gcc emits DWARF only for enum types something USES,
so the dump is a function of the uses, not the declarations — which is the hole
the tool's `keptBack` branch papers over.

**`DEADSWEEP.md`'s "`Other` is always empty" is FALSE** — on phase 80's text
`deadsweep` reports `left alone 49` (`REACHABILITY.md` §6).

**`AST-EDITING.md` reports a stale line in `CLAUDE.md`** ("54 shared cutters,
13,922 lines with cutil") **that is in no tracked `.md` file** — grepped, not
found. Its underlying measurement is right: `cut`+`cutil` is 11,193 lines today.
The same survey quotes `slim-vim.c`'s md5 as though it were `slim.sha`, which
records a sha256; the input was verified intact against the recorded digest.

## Status

- **In-AST editing: assessed, not taken. Revisit later** (decided 2026-09-23).
  The blocker is not cost. `internal/cemit` is a function of (AST, source text)
  joined by byte offset, so a mutation moves the tree while the source stands
  still; deleting an initializer-list element then produces BYTE-IDENTICAL
  output — the tree loses the element, the text keeps it, the result still
  parses and is still the printer's fixpoint. 10 of 10 deletions did this, and
  the class covers 829 hazardous sites of 15,185 in the product. `whim-build-check`
  passes and the edit did nothing, so the pipeline's total gate cannot see it.
  What would unblock it: recording an expansion's extent in `internal/cc` rather
  than inferring it as "the next offset any token has".
- **Print from `cc.Parse`**: handed to the canonicalization branch.
- **Enum values from the front end**: queued, free, provable with one
  `whim-build-check`.
- **The reachability closure as a REPORTER**: queued, proved by
  `sweep ∖ closure = ∅` over the boundary tars, with gcc as the control.
- **The closure replacing typereach/deadfields/deadenums' analysis**: not until
  the reporter has run, and it moves the product (119 entities at q82).
