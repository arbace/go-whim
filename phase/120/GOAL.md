# Phase 120 — the degenerate unions go

`phase/120/edit.go` and `phase/120/check.go`, `stage 120`, `package tidy`. Thirteen
`union` keywords in `whim-vim.c`, and **six of them union nothing with anything**. Five
are single-member — `u_header`'s `uh_next`, `uh_prev`, `uh_alt_next` and `uh_alt_prev`,
each `union { u_header_T *ptr; }`, and `typval_S.vval`, `union { varnumber_T v_number;
}` — and the sixth is **empty**, `union { } es_info;`, with one mention in the whole file
and no use at all.

Every one is a leftover of a cut already made. The `u_header` unions had an arm that named
a **swapfile block number** and it went with the swapfile; `vval` had nine arms and has had
one since the eval layer went; `estack_T.es_info` was a `ufunc_T *` beside an `sctx_T *`
and both went the same way. **A variant type with one variant is a value with a longer
spelling, and a variant type with no variants is a GNU C extension ISO C forbids** —
`GOALS.md`'s core is what a transpiler reads, so both cost a reader something and
neither buys anything.

## Which six is computed, not listed

The edit scans for `union`, matches the braces, counts the member declarations at depth 1
and takes every union with fewer than two as degenerate — and it must find **both kinds**,
so a scanner that stopped matching cannot pass by finding nothing. Measured on q119: 1
member for `uh_next`, `uh_prev`, `uh_alt_next`, `uh_alt_prev` and `vval`, **0** for
`es_info`, and 2 or 3 for `ae_u`, `lv_u`, `os_oldval`, `os_newval`, `rs_u`, `se_u` and
`rs_un`, which stay byte for byte and whose text is required to occur once in both files.
Thirteen keywords become **seven**.

A single-member union becomes its member **carrying the union's name** — the replacement
text is the member's own declaration, so the type, the stars and the spacing are the
input's — and every `uh_next.ptr` becomes `uh_next`. The empty one is deleted outright.

## A partition and not a count

For each of the six, every mention outside a string literal must classify as **its own
declaration** or a **`.member` access on it**, and a mention that is neither refuses: a
whole-union assignment, a `sizeof`, a designated initialiser, or another struct with a
field of the same name and a different member would each stop the phase. Measured on q119:
`uh_next` 1 + 27, `uh_prev` 1 + 23, `uh_alt_next` 1 + 28, `uh_alt_prev` 1 + 26, `vval`
1 + 19, `es_info` 1 + 0, with nothing left over — **identical to what the same computation
gives on q118**, so phase 119 moved nothing of this phase's. Those numbers are read off the
text by both the edit and the check and written into neither, which is the lesson phase 118
was taught when phase 117 moved its counted anchors.

The rewrite is literal-aware and **single-pass**: 129 spans computed against the original
text and applied together, over 6,707 literals none of which holds any of the six names,
because a second pass would index spans computed on the first pass's output — phase 106
measured five of 437 `size_t` left behind that way.

## The evidence is that the binary is byte-identical, and the control is what makes it evidence

A union of one member has the size, the alignment and the offset of that member, and an
empty union contributes no storage, so no layout moves and `uh_next.ptr` and `uh_next`
name the same object at the same address. **782,760 bytes either side**, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags — tier 1 of `CLAUDE.md`'s verification
table, which subsumes every screen case, Ex-command row, command line and pty scenario at
once, because the program that would be run is literally the same program.

Two numbers agreeing prove nothing on their own, so **`c1` is built from the input's own
text**: this phase's output with the two fields it *promotes*, `uh_next` and `uh_prev`,
**exchanged** — a pure layout permutation of the very struct it rewrites. It builds and
differs in **31,056 bytes**, so the binary is demonstrably sensitive to that struct's
layout. Two further controls move nothing and are **reported rather than dropped**: `c2`
puts the empty union back and `c3` runs the phase backwards on `vval` with all 19
`.v_number` accesses, and both are byte-identical. That is the claim stated in the only
direction a control can state it in. All three are built from the input's own text and
never from C quoted in the check, because **a check that quotes C is a dependency on
spelling** (`apart 105 106`).

## The empty union's dialect argument is measured, not asserted

gcc reports `union has no members [-Wpedantic]` **once** on the input and not at all on
the output, and the rest of the pedantic diagnostic set does not move — 199 → 198, a
difference of exactly one. A minimal probe confirms the construct is a hard **error**
under `-pedantic-errors` and that the identical struct with a one-member union is silent:
the probe proving it can pass, in the same run.

## The declared delta is nothing at all, and it is the strongest kind and not the weakest

`phase/120/delta` gets a comment and no line. This is phase 99's and phase 106's kind — a
`cmp` of the binary — so the phase owes no probes, unlike 103 and 105, and asks nobody to
believe a replacement does what an original did, unlike 97 and 98. Two full
`tools/zrecord.sh` recordings are identical across all 106 records, and the check says in
those words that **with a byte-identical binary that is a check on the harness and not on
the phase**; it is not offered as the evidence.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,799 | **79,786 (−13)**, every one above the boundary |
| `union` keywords | 13 | **7**, computed |
| `make editor.c` | 77,888 | **77,875**, 0 directives, 0 errors |
| the boundary | 18 names | **18**, computed from the input at run time |
| `nm -u` | 17 | **17**, a `cmp` in both directions |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and the same bytes** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | 0 of 106 — the harness, not the evidence |

## Its placement

`stage 120`, `package tidy 96 120 125` and **not `dialect`**: five of the six are leftovers of
earlier cuts, which is phase 96's kind, and only the sixth has a dialect argument.

**It was for a while the one phase after the seed with no `uses` line at all**, and that
looked defensible — its evidence is a `cmp` and not the recording, so it does not rest on
phase 83's baselines the way every other empty declaration does. **It was still wrong.**
`phase/120/check.go` names `tools/coredelta.sh` twice, the delta check runs at its stage
end like every other phase's, and the two **other** `cmp`-evidenced phases, 99 and 106, both
declare the dependency — five `uses` lines and six respectively. So the line is written now,
with the reason it was missing recorded in it. It was found by a documentation pass and not
by anything failing, which is the property of `phase/STAGES.md` worth saying plainly: the
file is read by `tools/stages.sh` and `tools/packages.sh` and **named by no phase program**,
so `tools/implhash.sh` never hashes it. **A manifest edit is free, which is why package and
`uses` data can be kept honest without paying for a repass — and it is also why a wrong one
is never caught by anything running. The only guard on that file is a reader.** Measured
either side of the correction: slim 9, whim 13-41, 120 and 127 are byte-identical,
and both manifest checks pass.

**`apart 119 120` is written and was measured in both directions**, as a real shared stage on
q118, which 119 and 120 make possible by both being split. Phase 119's check stops first, with
*the file is 79786 lines and the input was 79804 (79804 recorded) — expected 79799* and
*the boundary moved from line 77901 to line 77877*: 119 states its arithmetic as a line
count and 120 takes 13 more. The other direction was **observed rather than reasoned**, by
running phase 120's check on exactly the tree and state directory the driver would have
handed it next: *the file is 79786 lines and the input was 79801 — the six declarations are
13 lines shorter between them*, then *the boundary moved by 15 lines and the file by 13*.
**The two extra lines are phase 119's residue** — `static long mch_get_pid(void);` and the
`b0_pid` member, which 36 deliberately leaves for the sweep — landing inside phase 120's
arithmetic because a stage sweeps **once**, at the end. The general form is now in the
manifest: *a phase that states its line count against its own edit cannot share a sweep
with a phase that leaves work for it.*

No `need 120`, measured in the same run: the edit was handed phase 119's **unswept** output
and every part held, at exactly the counts it gets on swept text. It asserts no count a
sweep can move. Adding the phase moved no existing implementation key — 144 whim, slim and
Part II keys identical either side, with only z37 new.
