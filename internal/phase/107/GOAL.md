# Phase 107 — the attributes

`internal/phase/107/edit.go` and `internal/phase/107/check.go`, `stage 107`, `package dialect`.
`__attribute__` is a GNU extension, and a core on its way to another runtime was
carrying 139 of them. This phase looks at all 139, in three groups, and takes a
different decision on each:

```
    113   __attribute__((unused))         deleted
     20   __attribute__((fallthrough));   respelled  [[fallthrough]];
      6   format / format_arg             kept
```

139 → 6, **117 lines changed, not one line added or removed**, and not one statement
changed. None of the three kinds emits code — `unused` suppresses a diagnostic,
`fallthrough` gives one a hint, `format` decides what gcc will check — so the binary is
byte-identical, 788,488 bytes either side.

## Why the 113 said nothing, and why upstream needs them

Every dead-code compile in this pipeline is `-Wall -Wextra -Wno-unused-parameter`
(`tools/deadsweep.py`, `tools/phasecheck.sh`), so an unused **parameter** is not
diagnosed whatever is written on it. Upstream carries the marker because upstream
compiles this file in configurations where a parameter is used and others where it is
not; there are no configurations here, and have not been since slim's Phase 5. Measured:
with all 113 gone the sweep's own command line prints nothing at all.

## All 113 were on parameters, computed twice rather than assumed

Every one sits inside a parenthesised group whose innermost enclosing `(` is preceded by
exactly a function name, and the **97 lines** that carry them are all function
*definition* headers, each followed by a line that is `{` — so not one is on a variable,
an object, a type or a field. No parenthesised group in this file spans a line break
(`CLAUDE.md`), so that is a computation on one line and not a parse of C.

The compiler says it a second way. With `-Wunused-parameter` turned back **on**, the
output warns **135** times against the input's **43**, and every one of the 92 new
warnings is `-Wunused-parameter` at a line that carried an attribute — **not one
`-Wunused-variable`**, which is what a deletion that had reached an object would have
produced.

## Twenty-one of the 113 were false, and that is the find of the phase

113 sites, 92 warnings. **The difference is 21 attributes that marked a parameter this
build uses**:

```
    ex_cquit(exarg_T *eap)         reads eap->addr_count on its first line
    check_winopt(winopt_T *wop)    dereferences wop five times
    deathtrap(int sigarg)          compares sigarg against SIGHUP
```

There the attribute was not redundant, it was a **claim the code contradicts** — a
statement some earlier whim or Part II phase made untrue, and nothing in either pipeline
checked. So the phase does not delete 113 redundant markers: it deletes 92 unnecessary
ones and **21 wrong statements**, and the check names them rather than letting the
arithmetic swallow them.

## `[[fallthrough]]` is not refused under `-std=c11`, which is an argument against the swap

The swap is one-for-one and textual: all 20 sites were standalone statements on lines of
their own, `[[` occurs **0** times in the input and **20** in the output, and
`-Wimplicit-fallthrough` is silent either side, so not one suppression was lost. It is
not a directive either — C23 attribute syntax is a *statement* in the grammar, spelled
with brackets, and the charter's rule is about preprocessor syntax.

What it costs is stated rather than glossed, and it is **weaker than phase 106's
`typeof`**, which `-std=c11` refuses outright. Measured: gcc accepts `[[fallthrough]]`
under every `-std` it has, as an extension; below C23 `-Wpedantic` says *ISO C does not
support '[[]]' attributes before C23* and **`-pedantic-errors` refuses it** — where the
GNU spelling it replaces is accepted even there, being a reserved identifier. So taken
alone **the swap narrows the dialects this file compiles under**. It costs nothing in
practice, because the file has been C23 by four other routes since before this phase,
and that too is computed rather than argued: `-std=c11` on the whole file gives the
**same 729 errors** before and after.

An honest argument against a change belongs in the phase that makes it, not in the
commit of the phase that has to undo it.

## Two of the twenty are already redundant, and are named rather than pruned

Lines 11480 and 20181, after `case ESC:` and `case Ctrl_P:`: each follows a case label
with **no statement at all**, where C falls through silently and gcc has nothing to
diagnose. They are computed from the structure and required to agree with the control's
count — two independent methods for one number — and then **kept**. What makes a
fallthrough deliberate is the author saying so, not the compiler currently asking, and a
phase that quietly drops what it noticed was unnecessary is how a real suppression goes
missing later.

## The evidence splits in two, and the phase says so

For the **133** that go or change spelling, the binary is the evidence: `cmp`-identical,
which is `CLAUDE.md`'s tier 1 and subsumes every screen case, every Ex-command row,
every command line and every pty scenario at once, because the program that would be run
is the same program.

For the **6** that stay, **the binary is blind** — measured, removing all six as well
leaves it *still* `cmp`-identical while `-Wformat=2` goes from **115**
`-Wformat-nonliteral` warnings to **zero**. So the warnings are their evidence, and the
form is phase 105's invariant reused verbatim: the identical 115 warnings in the
identical **53** functions before and after, compared as list equality, with two controls
that break it in opposite directions — `vim_snprintf`'s `format(printf, 3, 4)` removed
gives **0**, and `_()`'s `format_arg(1)` removed gives **135**, gcc having lost its way
through the translation wrapper.

That is what the six are for. Phase 105 expanded seven wrappers into 129 direct calls, so
**one attribute now type-checks 201 `vim_snprintf` mentions**, and `format_arg` on `_()`
and `NGETTEXT` is why `CLAUDE.md` records that those two were never macro-expanded. The
rule the phase applies is therefore not *remove GNU extensions* but **remove every
attribute whose job another flag already does, and keep every attribute that IS the
flag**.

## Four controls, and one of them is the one that makes `cmp` mean something

c1 and c2 are the two above. c3 blanks all 20 `[[fallthrough]];` and gets **18** warnings
where there were none — which is also how the two redundant sites are confirmed from the
other end. c4 replaces **one** of them by `break;` — the smallest change at those sites
that is a change to the *program* rather than to a diagnostic — and moves **512,594
bytes** of binary. Without c4 the byte-identical binary is two numbers agreeing.

## One fact about the tools, which is why the check is seconds and not minutes

`CLAUDE.md` records that `-fsyntax-only` does not report `-Wunused-function`. It does not
report **`-Wimplicit-fallthrough`** either, which needs the CFG — measured, the c3
control warns 18 times under `-c` and **zero** times under `-fsyntax-only`. A fallthrough
section written with it would have passed while checking nothing. It *does* report
`-Wunused-parameter` and `-Wformat-nonliteral`, which is what the other sections use.

## The trap was the whitespace, not the attribute

Each attribute was written with **two** spaces before it and **one** after, so deleting
the text alone leaves a doubled space or a space before a paren — and `tools/canon.sh`
does not fix either. The check measures it: the count of doubled spaces before a `,` or
a `)` is **639 either side**, and canon is a no-op on the output.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 80,178 | **80,178** — 117 lines changed, none added, none removed |
| bytes | 2,139,325 | **2,136,127** |
| `__attribute__` | 139 | **6** — three `format`, three `format_arg` |
| `__attribute__((unused))` | 113 | **0** — on 97 definition headers, 21 of them false |
| `__attribute__((fallthrough))` | 20 | **0** |
| `[[fallthrough]]` | 0 | **20** |
| `-Wunused-parameter` when asked for | 43 | **135** — 92 new, not one `-Wunused-variable` |
| `-Wformat-nonliteral` under `-Wformat=2` | 115 in 53 functions | **115 in the same 53** |
| `-std=c11` errors on the whole file | 729 | **729** |
| functions / type definitions / DWARF enumerators | 1,756 / 905 / 1,177 | 1,756 / 905 / 1,177 |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 — `cmp`-identical** |
| sweep | | takes nothing, canon a no-op |
| phase | | **22 s** |

## Its placement

`stage 107`, and **a package of its own, `dialect`**, which is the one placement decision
here. The boundary is where the line between core and host falls, and this phase moves
no line: it asks which dialect of C the core is written in and which of its extensions
are still being paid for. `boundary` would have made the package mean two things.
Its `uses` are `dialect:107 seed:83 mechanical`; `dialect:107 tidy:96 rationale`, because
phase 96 dropped `ui_write()`'s `console` parameter rather than leaving
`__attribute__((unused))` on it and said so in as many words — that was this phase's
judgement one phase at a time; `dialect:107 format:105 mechanical`, phase 105's 129 direct
calls being why one `format` attribute now carries 201 mentions; and
`dialect:107 boundary:106 mechanical`, the `%d` probe being built from the output's own
lines — `vim_snprintf`'s prototype and the `typedef typeof(sizeof(0)) usize;` it needs to
compile — both of which read `usize` because of phase 106.

**`apart 106 107` and `need 107` are both measured NOT to be required**, in one run:
`tools/phaserun.sh 106-107` on q105 runs both edits, one sweep and both checks, and
every part passes. This phase touches no `NULL`, no `size_t` and none of the helpers
phase 106's controls quote, and phase 106 neither creates nor destroys an attribute; the
edit asserts no count of its input that a sweep could move, what it asserts being a
partition and a shape.

**This phase renumbered the three that follow it, and this document had the old numbers
until now.** `GOALS.md` §II.4c's reorganisation steps 24, 25 and 26 are phases 108, 109
and 110, which `internal/phase/STAGES.md` states; *Phase 106 — `nullptr` and `usize`* above said
*"26, the move itself"* and has been corrected to say 27. The phase *programs* are
deliberately **not** corrected — `internal/phase/106/*.sh` still says "phase 109" where it means
the move — because every byte of a phase program is in its unit's implementation digest,
and a comment fixed there re-keys a boundary to change nothing.

## What whim-vim is after twenty-four phases

```
whim-vim.c        80,178 lines          from whim-vim.c's 86,614  (-6,436, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
GNU attributes    6, all format or format_arg
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**Four phases in a row have now declared nothing**, and 106 and 107 are the same kind: the
binary is the same bytes. The difference between them is what that kind can and cannot
carry. Phase 106's whole content was inside the `cmp`; a sixth of this phase's — the six
attributes it keeps — is outside it, and the phase had to go and get a second instrument
for that part rather than let the strongest evidence it had cover a decision the evidence
cannot see.

The transformation now lives in `crefactor/xform` (`Attrs`).

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 105-107 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
