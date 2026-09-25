# Phase 173 — a goto out of its block is a break

Most of vim's `goto`s leave early: `goto doend;`, `goto theend;`, `goto skip;`
to a label further down a block that holds the goto, with nothing between
the goto and that block but `if`/`else` and braces. That is a labeled block,
`L: { ... break L; }`, and C has one form of it: `do { ... } while (0);`, which
a `break` leaves. This phase wraps each such label's region -- the items of
the label's block from the first one holding a goto to it, up to the label --
in `do { ... } while (0);`, writes `break;` for each goto, and drops the label,
which nothing jumps to any more.

**Why.** A backend with no `goto` -- Java has none -- must be handed code
without one, and the Go loses them too: `internal/gen` (crefactor/togo) writes
a C `do { ... } while (0)` as `for { ...; break }`, and a function left with no
`goto` has its locals declared where C declares them instead of hoisted. It is
`.tmp/goto-survey/GOTOS.md`'s class E1, and its rule.

**The rule is general** (`crefactor/xform`'s `GotoBlock`, the step
`gotoblock`), and a label is taken whole or not at all. It is held when a goto
to it comes after it or from outside its block (a retry); when a loop or a
switch lies between a goto and the label's block (the `break` would bind to
it); when the region holds a `break` or `continue` of its own that leaves it,
or a `case` whose switch is outside it; when the region declares a name --
object, type, tag, enumerator -- spelled anywhere from the label to the end of
its block (the do-while is a new scope); when a goto from outside the region
jumps to a label inside it (into a block); when the label is inside a
statement rather than one of its block's; and in a function that takes a
label's address. Two labels whose regions overlap are not rewritten in one
round: the text is parsed again, so that a goto now inside the other's
do-while is judged against it (held: a loop between). Rounds run until one
rewrites nothing. `--at-least 50` is the floor: E1's count in the survey.

**It changes no behaviour**; the binary differs, gcc at `-O0` compiling the
jumps as the do-while's.

**Measured**, on q169 (the product before it, phases 170-172 not yet
between): 71 gotos become a `break` out of 18 new `do { } while (0)`s, in
one round (the second finds nothing more), and their 18 labels go: the
survey's 50 of class E1 (`do_one_cmd`'s
19 `goto doend`, `ex_substitute`'s 5 `goto skip`, `do_put`'s 7 `goto end`,
`normal_cmd` 4, `bt_regexec_both` 4, `do_addsub` 3, `screen_fill` 3,
`getcmdline_int`'s 2 `goto theend`, `op_delete`, `stropt_get_newval`,
`undo_time` 1 each), and 21 that class A (a tail copied over the goto, phase
170) and class C (`screenalloc`'s `give_up`, phase 171) would take first:
the three `do_set_option*` 14, `gotchars_add_byte` 3, `open_line` 2, `showmap`
1. Held: 2 labels (2 gotos) for a goto after the label -- `screenalloc`'s
`retry`, `normal_cmd_get_count`'s `getcount`, class D -- and 19 labels (112
gotos) for a loop or switch between, class E2 and the class-A labels whose
gotos leave a loop; none for any other reason. `whim-vim.c`'s `goto`s go from
185 to 114, the functions with one from 34 to 19, and the file from 75,396 to
75,450 lines. It compiles silently with the one compile line.

In the Go, generated from it (`editor/editor.go` rewritten only to measure):
`goto` 163 → 92, labels 40 → 22, functions with a `goto` 34 → 19, 58,141 →
58,028 lines; `go vet` clean. `whim test` on it: 45/45 as HEAD, and the Go
editor answers all 45 as the C does; `--wide`: 240/240, both comparisons.

**In the chain** (after 170-172, measured by the whole build): 50 gotos become a `break` out of a do-while(0), and their 11 labels go -- exactly the survey's class E1; the other 21 it took on q169 were 170's and 171's. 10 labels (49 gotos) are held, each with a loop or switch between a goto and its label's block. The product is 75,539 lines, with 49 gotos, all in the core (185 before 170); `editor.go` has 54 in 10 functions (163 in 34 before), 58,011 lines. `whim test` against the commit before 170: 45/45 as it, the Go editor 45/45 as the C; `--wide`, all 240.
