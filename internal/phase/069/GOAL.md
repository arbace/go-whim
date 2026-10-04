# Phase 69 — `regrepeat()` does not jump into a case

*Formerly phase 141. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

The character classes `\s \S \d \D \x \X \o \O \w \W \h \H \a \A \l \L \u \U`
followed by a multi are eighteen pairs of case labels in `regrepeat()`:
- `\s` set its mask and `testval` and ran into the class loop, labelled
  `do_class`;
- each of the other seventeen set its own and jumped to that label from
  further down the switch.

Go cannot jump into a case, and the transpilation restructured it by hand
(finding 11). Now all thirty-six labels lead to one case. It sets `mask`, and
`testval` for the classes that match rather than exclude, in a switch on the
same opcode, then runs the loop. Every opcode makes the same assignments, the
loop is unchanged, and no `goto` is left in the function. The other jumps
into a case (`edit()`, `regatom()`, `check_termcode()`) are not yet phases.

**Declared delta: nothing.** The check reads an opcode → assignment table from
the input's jumps and another from the output's switch, each with its own
parse, and requires them equal with 18 entries. It requires the loop
unchanged byte for byte. Its probe substitutes with all eighteen classes, one
per line; the control swaps `\s` for `\S` and moves.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the class cases are read as a run of items and the head written anew by FRAG, the loop keeping its nodes, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
