# Could deadenums be done inside the AST?

A survey, not an implementation. Every number below was measured in this tree on
2026-09-23, on the machine the repository is checked out on (64 cores, warm page
cache). It continues `.tmp/deadsweep-survey.md` and uses the same standard:
nothing is estimated from general knowledge about C, and where a thing could not
be measured it says so. Read-only throughout: `git status` was clean before and
after, and every probe wrote under `.tmp/de/`.

Short answer: **yes for the values, and the front end gives exactly the numbers
DWARF gives — measured, name by name, on 17 texts and 28,608 DWARF values, with
zero disagreements. The mention analysis agrees too: on 13 texts and 21,331
enumerator observations the textual rule and the tree's reference count name the
same 123 dead enumerators, with no difference in either direction.** What the
tree buys is not speed — deadenums alone in the AST is *more* expensive per build
than it is now — but the removal of a `-g` build, three measured blind spots, and
a whole class of silent renumbering the current tool can only decline to touch.
And unlike deadsweep, **deadenums' position in the round is safe**: measured,
every text reaching sixth place parses, even when deadsweep is removed from the
round entirely.

---

## 1. What deadenums does today, exactly

`internal/dead/deadenums.go` is 222 lines. It is the fifth deleting tool of the
seven in `internal/sweep/sweep.go`'s round:

```
deadsweep, deadprotos, typereach, funcreach, deadfields, deadenums, canon
```

### Which enumerators it deletes

`AnalyseEnums(text, vals)`:

1. `b := cutil.Blank(text)` — comments and string literals blanked.
2. `dead.Definitions(text, b)` — every **top-level** type definition, the same
   scanner `typereach` uses (`internal/dead/typereach.go:61`).
3. `counts[name]` — every identifier occurrence in the blanked text, counted.
4. For each definition whose body matches `enumHead = ^\s*(typedef\s+)?enum\b`,
   split the body between `{` and the last `}` on **top-level commas**
   (`cutil.SplitTop`), take the first identifier of each fragment as the
   enumerator's name, and call it dead when **`counts[name] == 1`** — i.e. the
   only occurrence in the whole file is its own declaration.

So "nothing mentions it" is a whole-file identifier count, not a scope-aware one.

### Why pinning is necessary at all

An enumerator with no `=` takes its value from its **position**. Delete
`HLF_TP` from a list and every enumerator after it drops by one — and several of
these enums are the index of a parallel table (`hl_flags[HLF_COUNT]`,
`first_autopat[NUM_EVENTS]`, `main_errors[]` indexed by `ME_*`). The compile is
perfectly happy; every later row is silently re-pointed. `tools/enumvals.sh`'s
own header says so, and `GOALS.md` II says it again.

So the tool **pins the first survivor after each deleted run** to the value it
had, by splicing ` = <value>` after its name, and everything after that survivor
follows implicitly as before. Only a survivor written *without* an `=` is pinned
(`bytes.IndexByte(raw, '=') < 0`); one that already carries an explicit value
cannot move.

Measured, in the pre-41 round reconstructed below, the pins look like this
(`.tmp/de/seq/pre41/`):

```
-     , BV_COT ... , BV_TSR , BV_EOF          ->   , BV_EOF = 22
-     , WV_CRBIND , WV_WCR , WV_EIW , WV_NU   ->   , WV_WCR = 4 , WV_NU = 6
-     GETLINE_NONE, GETLINE_CONCAT_CONT,      ->   GETLINE_CONCAT_CONT = 1,
```

### The two numbers in the report line

`internal/sweep/tools.go:130`:

```
  deadenums    %d enumerators nothing mentions, %d survivors pinned%s%s
```

- **N — `EnumStats.DeadTotal`**: entries actually **deleted**, counted only in
  enums that keep at least one entry that declares something.
- **M — `EnumStats.Pinned`**: surviving enumerators given an explicit `= value`
  so that nothing after them moves. M is not "how many survived"; it is how many
  needed an anchor.

### The trailing clause, precisely

```
; %d in enums where every constant is dead and the type is in use, which cannot
  be expressed
```

That is `EnumStats.Stuck`. The code that decides it (`deadenums.go:150-167`)
counts what would be **left** after the deletions, and refuses when nothing left
declares anything:

```go
declares := false
for _, k := range keep { if identRe.Match(k) { declares = true; break } }
if !declares { st.Stuck += deleted + keptBack; continue }
```

The case is exactly: **every constant of one enum is dead, and the enum
definition itself survives.** The rewrite would have to emit `enum { }`, which is
not C, so the tool leaves the whole enum alone and counts the constants apart —
*a sweep has to tell "you did not finish" from "this cannot be expressed"*. Note
what makes the definition survive: `typereach` deletes an unreachable type
definition in the same round, so an enum whose constants are all dead is only
still there when its **type** is reached — a typedef used as a field or a
parameter type.

Measured, this is one enum for nearly the whole pipeline. `.tmp/de/stuck.bin`
(a probe that replays `AnalyseEnums`' classification and prints what it declines):

| text | stuck enums | the enum |
|---|---|---|
| q12 (153,882 lines) | 2 | `except_type_T {ET_USER, ET_ERROR, ET_INTERRUPT}` and `omacc_T` |
| q41, q63, q77, q82, q115 | 1 | `typedef enum { VIM_ACCESS_PRIVATE, VIM_ACCESS_READ, VIM_ACCESS_ALL } omacc_T;` |
| committed `whim-vim.c` | 0 | — |

`omacc_T` survives because `omacc_T ocm_access;` is a field
(`.tmp/de/src/q82.c:1540`). Across a whole build (`.tmp/build-full3.log`, 200
sweep rounds) the clause appears in **150** rounds, and in **122** of them the
number is 3 — those three `VIM_ACCESS_*`. `internal/phase/070/GOAL.md` calls them "the
three enums"; measured, it is one enum with three constants. (Reported, not
edited — this survey touches no tracked file.)

The second optional clause, `; %d kept before a survivor DWARF has no value for`
(`EnumStats.Unpinnable`), **never fired once in the 200 rounds of a full build**.
§2 shows why, and why an AST implementation removes the possibility entirely.

### When the values are computed — and what the AST must reproduce

`internal/sweep/sweep.go:47-54` reserves a *name* and deletes the file:

```go
valsFile, err := os.CreateTemp("", "enumvals.")
vals := valsFile.Name(); valsFile.Close(); os.Remove(vals)
```

`runDeadenums` (`tools.go:108`) then dumps **on first need only**:

```go
if _, err := os.Stat(vals); err != nil && (st.DeadTotal > 0 || st.Unpinnable > 0) {
    dead.DumpVals(path, vals)     // sh tools/enumvals.sh
```

and `tools/enumvals.sh` is

```sh
gcc -O0 -g -o "$tmp/vim" "$tmp/vim.c"
readelf --debug-dump=info "$tmp/vim" | awk '/DW_TAG_enumerator/{...}' | sort -u
```

So: **one file per sweep, written the first time something is dead, read by every
later round.** Every pin in the whole sweep is to that one numbering. At the end,
`sweep.go:156` dumps DWARF again and requires that no surviving name moved —
*this check's exit status is the ONE in the whole sweep that is not masked*.

Measured across a whole build: **101 sweeps, 200 rounds, 19 DWARF dumps.**
deadenums ran in 180 rounds (20 were skipped by the `passed this text already`
cache) and paid for a `-g` build in 19 of them — phases 12, 21, 28, 32, 41, 49,
50, 53, 57, 58, 60, 62, 63, 65, 79, 80, 93, 95 and 137.

**What breaks if an AST implementation does not reproduce the snapshot.** The
tempting simplification is to compute the values from the tree it already has,
every round. That is wrong for one specific reason: the snapshot is not an
optimisation, it is the *reference*. If round 3 silently renumbered an unpinned
survivor, a round-4 recompute would pin the survivor to its **new** value, making
the damage permanent, and the final `VerifyEnums` — which compares the final text
against that same recomputed reference — would then agree by construction. It is
the *"never regenerate a baseline from a binary a later phase can reach"* rule at
sweep scale. An AST implementation must snapshot once and hold it. The good news
is that with the AST the snapshot is ~0.9 s rather than a 4–7 s debug build, so
it can be taken **unconditionally on round 1** — the sweep's input numbering,
which is strictly stronger than today's "the first round that finds something
dead", where earlier rounds may already have removed whole enums.

---

## 2. Do the front end's values agree with DWARF?

### Method

`.tmp/de/ev/main.go` parses with `internal/cc` exactly as `internal/ccx.Parse`
and `tx/skel` do, walks the tree with the reflect walker from
`internal/ccx/ccx.go:31`, and for every `EnumSpecifierDef` emits `NAME=VALUE`
from `Enumerator.Value()` (`Int64Value` / `UInt64Value`, via
`internal/cc/check.go:3148`). `.tmp/de/cmpvals/main.go` compares the two files
**numerically** with `math/big`, because `readelf` prints decimal below 65,536
and hex from `0x10000` up (measured: the largest decimal in the committed
product's dump is 65,535; 29 of its 1,155 values are hex). That formatting is the
*only* textual difference on the committed product; every value is equal.

### Result

| text | lines | DWARF | AST | agree | DWARF-only | AST-only | different |
|---|---|---|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 1,155 | 1,155 | 1,155 | 0 | 0 | **0** |
| `whim-vim.c`, `gcc -c` (control, same text, not counted in the total) | 77,306 | 1,155 | 1,155 | 1,155 | 0 | 0 | **0** |
| `editor.c` (`-c`; link refuses) | 75,311 | 1,141 | 1,141 | 1,141 | 0 | 0 | **0** |
| q82 boundary | 86,583 | 1,331 | 1,331 | 1,331 | 0 | 0 | **0** |
| q86 boundary | 86,555 | 1,331 | 1,331 | 1,331 | 0 | 0 | **0** |
| q115 boundary | 79,794 | 1,190 | 1,190 | 1,190 | 0 | 0 | **0** |
| q136 boundary | 78,121 | 1,167 | 1,167 | 1,167 | 0 | 0 | **0** |
| q162 boundary | 77,306 | 1,155 | 1,155 | 1,155 | 0 | 0 | **0** |
| q77 boundary | 89,375 | 1,838 | 1,842 | 1,838 | 0 | 4 | **0** |
| q79 boundary | 88,605 | 1,835 | 1,839 | 1,835 | 0 | 4 | **0** |
| q63 boundary | 100,317 | 2,057 | 2,061 | 2,057 | 0 | 4 | **0** |
| q41 boundary | 117,460 | 2,269 | 2,273 | 2,269 | 0 | 4 | **0** |
| q12 boundary | 153,882 | 2,515 | 2,519 | 2,515 | 0 | 4 | **0** |
| phase 78 pre-sweep | 89,297 | 1,837 | 1,841 | 1,837 | 0 | 4 | **0** |
| phase 79 pre-sweep | 88,892 | 1,836 | 1,841 | 1,836 | 0 | 5 | **0** |
| **phase 41's deadenums input** | 118,003 | 2,297 | 2,333 | 2,297 | 0 | **36** | **0** |
| phase 35's deadenums input | 123,668 | 2,316 | 2,346 | 2,316 | 0 | **30** | **0** |
| phase 80's deadenums input | 87,126 | 1,338 | 1,342 | 1,338 | 0 | 4 | **0** |
| **total** | | **28,608** | **28,707** | **28,608** | **0** | **99** | **0** |

**Not one value disagreed, on any text.** DWARF never had a name the tree lacked.

### Every disagreement, with the construct behind it

All 99 differences are in one direction — names the tree has and DWARF does not —
and they fall into exactly two classes:

**(a) 40 header enumerators.** `P_ALL`, `P_PID`, `P_PGID`, `P_PIDFD` (`idtype_t`,
`sys/wait.h`) in ten texts, from the host half's `#include`s. gcc emits no debug
info for an enum type the translation unit never uses. They are irrelevant to
deadenums, which only looks at enums in the file's own text; an implementation
filters on `Position().Filename == path`, which the probe does.

**(b) 59 dead enumerators gcc dropped.** `SHM_FILE` at phase 79; 32 names on
phase 41's deadenums input; 26 on phase 35's. Checked one by one: **every single
one occurs exactly once in the file** — they are the dead ones. gcc emits DWARF
only for enum types something uses, so the moment an enum's constants become
unreferenced its debug entry disappears. This is the mechanism behind
`deadenums.go`'s comment *"A survivor DWARF has no value for cannot be pinned"*:
gcc's dump is not a function of the declarations, it is a function of the uses.
Measured here, the names it drops happen to be exactly the ones deadenums does
not need — which is why `Unpinnable` was 0 in all 200 rounds — but nothing
guarantees that, and the tool carries a whole branch (`keptBack`) to survive the
case. **The AST has a value for every enumerator, so that branch and its risk
disappear.**

### The construct checklist, counted on the real source

`.tmp/de/detail-*.txt` records, per enumerator, whether it was written with an
initialiser, what the initialiser's source text is, whether that initialiser
names another enumerator or a declarator, the enum's tag, its fixed underlying
type, and the enum type's representation.

| construct | committed | `editor.c` | q82 | q12 | phase 41's deadenums input | agree? |
|---|---|---|---|---|---|---|
| implicit successor values (no `=`) | 325 | 315 | 365 | 1,102 | 1,049 | yes |
| explicit initialisers | 830 | 826 | 966 | 1,417 | 1,284 | yes |
| initialiser over another enumerator | 0 | 0 | 0 | **1** | **1** | yes |
| initialiser over a declarator | 0 | 0 | 0 | 0 | 0 | no case |
| negative values | 4 | 3 | 1 | 3 | 3 | yes |
| values outside `int` | 7 | 7 | 1 | 1 | 1 | yes |
| values above `int64` | 2 | 2 | 0 | 0 | 0 | yes |
| enum with a fixed underlying type (`enum : T`) | **9** | **9** | 1 | 1 | 1 | yes |
| anonymous enums (enumerators in) | 855 | 841 | 1,018 | 1,584 | 1,398 | yes |
| a name declared twice | 0 | 0 | 0 | 0 | 0 | no case |
| unknown (uncomputable) value | 0 | 0 | 0 | 0 | 0 | no case |

The hard cases, and both instruments' answers (committed product):

```
INT_MIN     -(int)(~0u >> 1) - 1            AST -2147483648            DWARF -2147483648
LONG_MIN    -(long)(~0ul >> 1) - 1          AST -9223372036854775808   DWARF -9223372036854775808
LLONG_MAX   (long long)(~0ull >> 1)         AST  9223372036854775807   DWARF  9223372036854775807
SIZE_MAX    (usize)-1                       AST 18446744073709551615   DWARF 0xffffffffffffffff
ULLONG_MAX  ~0ull                           AST 18446744073709551615   DWARF 0xffffffffffffffff
P_COLON     0x80000000L, in `enum : long`   AST  2147483648            DWARF 0x80000000
TYPE_UNKNOWN -1                             AST -1                     DWARF -1
VIM_VERSION_100  VIM_VERSION_MAJOR * 100 + VIM_VERSION_MINOR   AST 902  DWARF 902
```

**The fixed underlying type (`enum : long`) was the construct most likely to
split them, and it does not.** `internal/cc/check.go:3113` applies
`EnumTypeSpecifier`'s type to every enumerator of the enum, and `P_COLON`,
`SIZE_MAX`, `LLONG_MIN` and the rest come out with the same numeric value gcc
records. Counted: the committed product and `editor.c` each hold nine such enums
(the `limits.h` replacements plus `P_COLON`); q82 holds one.

### Two facts about the instrument, measured here

- **`tools/enumvals.sh` cannot be run on `editor.c` at all.** It *links*
  (`gcc -O0 -g -o "$tmp/vim"`), and the core has no host:
  `undefined reference to 'host_message'`. Replacing the link with `gcc -c` (a
  probe, `.tmp/de/enumvals-c.sh`) makes it work and — control — produces the
  **identical** 1,155 values on the committed product. So today the enumerator
  values of the core cannot be asked for by the tool that is supposed to answer
  that question; the AST answers it in 0.83 s.
- **A text that does not compile gets no answer either.** On phase 41's
  unswept accumulation, `enumvals.sh` refuses, so `DumpVals` errors, so
  `runDeadenums` returns an error and the round records nothing. **deadenums is
  already a no-op on a broken text today** — the same behaviour an AST
  implementation would have on a text that does not parse. §4 shows why it never
  matters.

---

## 3. Can the mention analysis be done in the tree?

### What the front end offers — and what it does not

The deadsweep survey found `ReadCount`, `WriteCount`, `SizeofCount`,
`AddressTaken` and `Linkage` on `*cc.Declarator`. **There is no equivalent on
`*cc.Enumerator.`** The type is

```go
type Enumerator struct {
    typer; resolver; valuer; visible
    Case               EnumeratorCase
    ConstantExpression ExpressionNode
    Token  Token     // the IDENTIFIER
    Token2 Token
}
```

— a value and a position, no counters. The one place a use is recorded is
`internal/cc/check.go:5107`, in `PrimaryExpressionIdent`:

```go
case *Enumerator:
    n.resolvedTo = x
    n.val = x.val
    n.typ  = x.Type()
    break out                 // note: no x.read++, unlike the *Declarator arm
```

So the equivalent of `ReadCount` for an enumerator is **a walk**: collect every
`*cc.PrimaryExpression` whose `ResolvedTo()` is the `*cc.Enumerator`. It is
semantic (it follows scopes, and a shadowing local does not count), it covers
case labels, array bounds and initialisers because all of those are expressions,
and it costs one extra traversal of the tree — measured at ~0.16 s on the
committed product on top of a 0.70 s parse.

### The two answers, measured against each other

`.tmp/de/mention/main.go` runs both on the same bytes: `deadenums.go`'s exact
textual rule (`Blank` → `Definitions` → `enumHead` → `SplitTop` →
`counts[name] == 1`) and the tree's (`refs == 0`, merged over every declaration
of a name).

| text | lines | enum defs seen: text / tree | names: text / tree | called dead: text / tree | text only | tree only |
|---|---|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 700 / 709 | 1,146 / 1,155 | 0 / 0 | 0 | 0 |
| `editor.c` | 75,311 | 696 / 705 | 1,132 / 1,141 | 0 / 0 | 0 | 0 |
| q82 | 86,583 | 845 / 846 | 1,330 / 1,331 | 3 / 3 | 0 | 0 |
| q115 | 79,794 | 730 / 739 | 1,181 / 1,190 | 3 / 3 | 0 | 0 |
| q136 | 78,121 | 709 / 718 | 1,158 / 1,167 | 3 / 3 | 0 | 0 |
| q41 | 117,460 | 1,120 / 1,123 | 2,263 / 2,269 | 3 / 3 | 0 | 0 |
| q12 | 153,882 | 1,299 / 1,304 | 2,496 / 2,515 | 6 / 6 | 0 | 0 |
| phase 78 pre-sweep | 89,297 | 854 / 855 | 1,836 / 1,837 | 3 / 3 | 0 | 0 |
| phase 79 pre-sweep | 88,892 | 854 / 855 | 1,836 / 1,837 | 5 / 5 | 0 | 0 |
| phase 139 pre-sweep | 77,963 | 703 / 712 | 1,149 / 1,158 | 0 / 0 | 0 | 0 |
| phase 147 pre-sweep | 77,776 | 699 / 708 | 1,145 / 1,154 | 0 / 0 | 0 | 0 |
| **phase 41's deadenums input** | 118,003 | 1,158 / 1,161 | 2,323 / 2,329 | **56 / 56** | 0 | 0 |
| phase 35's deadenums input | 123,668 | 1,171 / 1,174 | 2,336 / 2,342 | **41 / 41** | 0 | 0 |
| **total** | | | **21,331 observations** | **123 / 123** | **0** | **0** |

**The set difference is empty in both directions, on every text.** For every one
of 21,331 enumerator observations, `counts[name] == 1` held exactly when the
tree's reference count was zero. The textual rule's known weakness — a name that
collides with a field, a local or a macro token would be counted as mentioned —
has no instance in this tree.

Sanity: on phase 41's deadenums input the tree and the text both say 56 dead, and
the tool reports `21 enumerators nothing mentions … 35 in enums where every
constant is dead`. 21 + 35 = 56. The accounting closes.

### Where they differ: the three blind spots in the scanner

The tree sees enum definitions the textual scanner does not — 9 on the committed
product, 6 on phase 41's input, 19 on q12. Every one was identified:

1. **`enum : T { … }` — a fixed underlying type.** The source is written
   ```c
   enum :
       long { P_COLON = 0x80000000L };
   ```
   `typereach.go`'s `startRe` wants `enum\s+\w*\s*\{` and its `headOnly` wants a
   line that is exactly `enum` or `enum <tag>`; `enum :` is neither, so
   `Definitions` never produces the definition. Counted: **9 in the committed
   product and in `editor.c`** (the six `limits.h` replacements, `SIZE_MAX`,
   `ULLONG_MAX`, `P_COLON`), **1** at q82 and earlier.
2. **`static enum { … } var;`** — found by `Definitions` (the `static` prefix is
   in `headOnly`) but rejected by deadenums' own `enumHead`,
   `^\s*(typedef\s+)?enum\b`, which has no room for `static`. Counted: **4
   enumerators** (`EXP_FILETYPECMD_ALL|PLUGIN|INDENT|ONOFF`) in every text before
   the phase that removes them.
3. **Block-scope enums.** `Definitions` only accepts definitions at brace depth
   0, so `enum {CACHE_SIZE = 12};` inside `event_nr2name()` is invisible.
   Counted: **1** at q41 and phase 41's input, **3** at q12 (`CACHE_SIZE`,
   `RELOAD_*`, `ct_*`).

None of these is currently dead, so none of them costs a deletion today. They are
listed because **an AST implementation sees them, and would therefore delete more
— which moves the product.** §6 turns that into a design decision rather than an
accident.

A fourth, different kind of gap: the `Stuck` case. The tree knows whether the
enum's *type* is reached and the text only infers it; but neither can write
`enum { }`, so the AST does not unstick it. What the AST does add is the ability
to *say so precisely* — the current message asserts "the type is in use" without
testing it.

---

## 4. The parse blocker, for this tool's position

deadsweep runs first and, the earlier survey measured, is handed a text that does
not parse exactly where it removes the most (phase 35's edit leaves two uses of
`wo_eiw`, a field it deleted, inside a function nothing calls). deadenums runs
**sixth**, after four other tools have cut. The question is whether that changes
the picture. It does, completely.

### Method

`.tmp/de/seq.sh`: run the seven tools through `tools/st.sh` in `sweep.go`'s
order, and after each ask three questions — does `internal/cc` parse it, can
`tools/enumvals.sh` answer about it, how many lines are left. The inputs are
produced by `.tmp/de/round/main.go`, which runs a run of phases' edit steps
through `internal/build.RunPhase` with no closing sweep (inner `{Op:"sweep"}`
steps run as they really do).

### Four sequences

**Phases 13–41 from q12, the real input of the stage-41 sweep** (this is the
failing case; it reproduces the build log's `deadenums 21 enumerators nothing
mentions, 8 survivors pinned; 35 … cannot be expressed` exactly):

```
pre41  00-input       cc FAIL  enumvals REFUSED            127328   type winopt_T has no member named wo_eiw
pre41  01-deadsweep   cc ok    enumvals ok (2329 values)   124365
pre41  02-deadprotos  cc ok    enumvals ok (2329 values)   124293
pre41  03-typereach   cc ok    enumvals ok (2329 values)   124256
pre41  04-funcreach   cc ok    enumvals ok (2297 values)   118031
pre41  05-deadfields  cc ok    enumvals ok (2297 values)   118003
pre41  06-deadenums   cc ok    enumvals ok (2276 values)   117980
pre41  07-canon       cc ok    enumvals ok (2276 values)   117889
```

**Phases 13–35 from q12** (the earlier survey's minimal failing text):

```
to35   00-input       cc FAIL  enumvals REFUSED            128641
to35   01-deadsweep   cc ok    enumvals ok (2342 values)   127600
to35   .. 05-deadfields  cc ok  enumvals ok                123668
to35   06-deadenums   cc ok    enumvals ok (2304 values)   123655
       deadenums: 12 enumerators nothing mentions, 5 survivors pinned; 29 stuck
```

**Phase 80's edit alone from q79** (a single-edit pre-sweep text; reproduces the
log's `7 enumerators nothing mentions, 1 survivors pinned`):

```
pre80  00-input .. 07-canon    cc ok, enumvals ok throughout   87231 -> 87111
```

**Phase 78's edit alone from q77**:

```
pre78  00-input .. 07-canon    cc ok, enumvals ok throughout   89297 -> 89201
```

### The decisive control: the round with deadsweep removed

If deadsweep itself went into the AST and declined on an unparseable text (the
deadsweep survey's option 2), does deadenums still get a tree? `.tmp/de/seq2.sh`
runs the same round with step 01 skipped:

```
pre41-nods  00-input       cc FAIL  enumvals REFUSED            127328
pre41-nods  01-deadsweep   SKIPPED
pre41-nods  02-deadprotos  cc FAIL  enumvals REFUSED            127328
pre41-nods  03-typereach   cc FAIL  enumvals REFUSED            127307
pre41-nods  04-funcreach   cc ok    enumvals ok (2303 values)   118091
pre41-nods  05-deadfields  cc ok    enumvals ok (2303 values)   118063
pre41-nods  06-deadenums   cc ok    enumvals ok (2282 values)   118040
            deadenums: 21 enumerators nothing mentions, 8 survivors pinned; 36 stuck
```

**funcreach repairs the text at step 04 whatever deadsweep did** — it deletes
`check_window_scroll_resize`, which holds the broken reference, as unreachable —
and deadenums in sixth place finds the same 21 enumerators and the same 8 pins.
funcreach is text-based and cannot be blocked by a parse failure.

So, measured:

- deadenums' position is **after the two tools that can repair a broken text**,
  and in every sequence measured the text at step 06 both parses and compiles.
- Where the text does *not* parse at step 00, deadenums is **already** a no-op
  today, because `enumvals.sh` links and refuses. Moving to the AST loses nothing
  there and gains one case: a text that parses but does not *link*, like
  `editor.c`.
- Unmeasured: whether any text anywhere in the 101 sweeps fails to parse at step
  06. The 28 single-edit pre-sweep texts in the earlier survey all parse at step
  00, and the two stage accumulations measured here parse from step 01 (or 04)
  onward. No counter-example was found, and none is claimed to be impossible.

---

## 5. The merge

### Per call, measured (three runs each unless noted)

| text | lines | `tools/enumvals.sh` | `whimtools parse` | parse + walk + values + refs |
|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 3.78 / 3.76 / 3.75 s | 0.70 / 0.70 / 0.65 s | 0.86 / 0.90 / 0.83 s |
| q82 | 86,583 | 4.20 / 4.06 / 4.08 s | 0.72 / 0.72 / 0.73 s | 0.95 / 0.92 / 0.93 s |
| q63 | 100,317 | 4.84 / 4.93 s | — | 1.19 s |
| phase 41's deadenums input | 118,003 | 5.62 / 5.62 / 5.69 s | 1.18 / 0.99 / 1.19 s | 1.27 / 1.32 / 1.35 s |
| q12 | 153,882 | 7.51 / 7.34 s | — | 1.88 s |

**The whole AST analysis is 4.2–4.4× faster than the DWARF dump it replaces.**
Both are near-linear in lines; fitted, `enumvals.sh ≈ 4.78e-5·L + 0.08 s` and
`AST ≈ 1.33e-5·L − 0.17 s`.

For completeness, today's textual analysis costs `0.16–0.19 s` at 77k lines and
`0.21–0.27 s` at 118k — deadenums without a dump is nearly free.

### Per build, measured on `.tmp/build-full3.log` (a whole `make whim-build`, 1,221 s)

```
101 sweeps          200 rounds          1.98 rounds per sweep
deadsweep ran in 200 rounds (never skipped)
deadenums ran in 180 rounds (20 hit `passed this text already`)
19 rounds dumped DWARF
150 of 200 rounds reported the `cannot be expressed` clause
  0 of 200 rounds reported the `a survivor DWARF has no value for` clause
 84 of 200 rounds had all five deleters report nothing -- the text deadsweep
    saw and the text deadenums saw were the same bytes
```

(The earlier survey estimated ~180 rounds from segment timings; the direct count
on the full log is **200**.)

Applying the fitted curve to the 19 dumping phases' own line counts (from the
log's per-phase `N lines`, so a slight under-estimate — the dump happens
mid-sweep on a slightly larger text):

```
19 dumps over 2,058,130 lines
  tools/enumvals.sh   modelled   99.9 s per build   (8.2 % of 1,221 s)
  the same from the AST          24.2 s per build
deadenums' textual pass          180 x ~0.18 s = 32 s
-------------------------------------------------------------------
deadenums today                  ~132 s per build  (10.8 %)
```

### The arithmetic on the merge

Per-round mean text size, over the 112 of 200 rounds whose phase had both a round
count and a recorded size: **79,215 lines**. At that size a parse costs 0.88 s
and gcc's `-flto` warning call costs ~1.42 s (interpolated from the earlier
survey's 1.38 s at 77,306 and 2.29 s at 118,499 lines).

| arrangement | parses / gcc calls | modelled s per build |
|---|---|---|
| **today** — deadsweep gcc + deadenums text + 19 DWARF dumps | 200 gcc + 19 `-g` builds | 284 + 32 + 100 = **416 s** |
| deadsweep AST, deadenums unchanged | 200 parses + 19 `-g` builds | 178 + 32 + 100 = **310 s** |
| deadenums AST, deadsweep unchanged | 200 gcc + 180 parses | 284 + 158 = **442 s** ← *worse* |
| both AST, each parsing its own text | 380 parses | **334 s** |
| both AST, sharing the parse in the 84 rounds the text did not change | 296 parses | **281 s** |
| the whole round in the tree, one parse | 200 parses | **178 s** |

Two things fall out of that table.

**deadenums alone in the AST is a loss of ~26 s per build.** It pays for gcc only
19 times today; making it parse 180 times costs more than the dumps it removes.
Its case is correctness, not cost.

**One parse cannot generally serve both.** Measured on phase 41's round, the text
between deadsweep's input and deadenums' input goes 127,328 → 124,365 → 124,293 →
124,256 → 118,031 → 118,003 lines: four of the five tools moved it. Across the
build, **84 of 200 rounds** (42 %) had all five deleters report nothing, and only
in those is the text deadenums sees byte-identical to the text deadsweep saw. A
shared parse must therefore be a *cache keyed on the digest of the current text*,
not a per-round object — which is exactly the shape `sweep.go`'s `passed` map
already has.

### What `sweep.go`'s note about round order implies

```go
// canon runs at the END of each round and not before the loop ... Moving it
// before the loop was measured to commute on one phase and then moved 27 of 32
// boundaries when the whole pass ran.
```

The round order is load-bearing to the *bytes*. For deadenums this cuts two ways:

- **In its favour**: deadenums does not need to move. Its position is already
  after the repairs, so unlike deadsweep there is no reordering to pay for.
- **Against a naive swap**: anything that changes *what* deadenums deletes —
  the three blind spots of §3, or a differently formatted pin — moves
  `whim-vim.c`, and with it `slim.sha`, `editor/editor.go` and every recorded
  boundary, plus hours of `make whim-verify`.

On the pin's format, measured: `readelf` prints decimal below 65,536, and **the
largest implicitly-valued enumerator anywhere measured is 600** (committed
product 98, q82 111, q12 and phase 41's input 600). A pin is only ever applied to
an enumerator written without `=`, so in this tree every pin is a small decimal
and an AST implementation emitting `%d` produces byte-identical text. That is a
fact about this tree, not a guarantee; an implementation should assert it
(`value < 65536`) rather than assume it.

---

## 6. Recommendation

**Split the change in two, and do only the free half.**

deadenums is really two questions wearing one coat:

- **Where do the values come from?** Today: a 4–7 s `gcc -O0 -g` build plus
  `readelf`, 19 times a build, which cannot answer about `editor.c` at all and
  which silently omits the enums gcc did not need. From the tree: 0.9 s, every
  enumerator, **measured identical on 28,608 values across 17 texts with zero
  disagreements**.
- **Which enumerators are dead?** Today: `counts[name] == 1` over blanked text.
  From the tree: `refs == 0`. **Measured identical on 21,331 observations across
  13 texts, 123 dead names, no difference in either direction** — but the tree
  additionally *sees* three classes of enum the scanner does not, so adopting it
  wholesale deletes more and moves the product.

The first is free and strictly better. The second is a behaviour change dressed
as a refactor.

### Options, in cost order

1. **Values from the AST, mention analysis untouched.** Replace
   `dead.DumpVals` (and `LoadVals`, and `VerifyEnums`' second dump) with the
   front end; leave `AnalyseEnums` byte for byte as it is.
   *What it costs*: ~75 s of build time saved; the `-g` build and `readelf`
   leave the sweep; the `Unpinnable` branch becomes unreachable because the tree
   has a value for every enumerator; `editor.c` becomes answerable.
   *What would prove it*: `make whim-build-check`. The committed `whim-vim.c`
   comes back byte for byte, or it does not. The only way it can move is a pin
   whose textual form differs, and the measurement above (every implicit
   enumerator ≤ 600, `readelf` decimal below 65,536) says it cannot — assert it
   and let the assertion fail loudly if a future tree breaks it.
   *Deliberately not included*: the twelve `internal/phase/*/check.go` that call
   `tools/enumvals.sh` directly (088, 091–098, 105, 112, 122). They compare a
   before-dump with an after-dump and would see the extra names the tree has;
   leaving them on DWARF keeps them as an independent control, which is the
   point of a control.

2. **Option 1, plus the mention analysis from the tree, with the blind spots
   deliberately preserved.** Keep skipping `enum : T`, `static enum` and
   block-scope enums, so the deletions are identical, and use the tree only for
   the reference count. *Cost*: +126 s of build time (180 parses against a 0.18 s
   textual scan) for no change in output. *Not recommended on its own* — it is
   the price of a shared parse with no shared parse to pay for it. It becomes
   sensible only inside option 4.

3. **Option 1, plus the mention analysis with the blind spots fixed.** The
   honest version: deadenums would then see 9 more enum definitions in the
   committed product and 19 more at q12, and would delete more when they die.
   *Cost*: the product moves. `whim-vim.c`, `slim.sha`, `editor/editor.go` and
   every boundary are re-derived, `make whim-verify` runs for hours, and every
   phase whose `GOAL.md` quotes a line count is re-measured. Worth doing only if
   something in those enums is actually dead and wanted gone — measured, nothing
   in them is dead today.

4. **The whole round in the tree, one parse per text digest.** deadsweep,
   deadprotos, typereach, funcreach, deadfields and deadenums all reading one
   `cc.Translate`, cached by the digest of the current bytes exactly as `passed`
   already is. *Modelled*: 178 s against today's 416 s — a 238 s saving, 19 % of
   the build. *Cost*: five more tools' answers change, the round order question
   reopens, and the product certainly moves. This is a rewrite of the sweep, not
   a substitution, and it should not be attempted before options 1 and the
   deadsweep survey's staged plan have each been proven on their own.

### Is deadenums a better or worse candidate than deadsweep for going first?

**Better on safety, worse on payoff. It should go first anyway, because its safe
half is free.**

- **Safety: much better.** deadsweep is the *first* tool in the round and was
  measured being handed a text that does not parse, on the round where it removes
  the most; it needs either a reordering (which moves 27 of 32 boundaries' worth
  of product) or a declining path whose cost is unknown until it is run.
  deadenums is *sixth*, after funcreach, and every text measured at that point
  parses — including with deadsweep removed from the round entirely. And where a
  text does not parse, deadenums is **already** a no-op today, because
  `enumvals.sh` links. There is no position risk to buy off.
- **Agreement: better.** deadsweep's comparison was 474 of 475 names, with one
  documented gcc rule (`static inline`) that the AST must be taught. deadenums'
  comparison is **exact in both dimensions** — 28,608 of 28,608 values, 123 of
  123 dead names — with no rule to reproduce and no exemption to encode.
- **Payoff: worse.** deadsweep's gcc call is 200 rounds × 1.42 s ≈ 284 s a build
  and an AST replacement returns ~106 s of it. deadenums' DWARF dump is 19 × ~5.3
  s ≈ 100 s and an AST replacement returns ~75 s — but only if the mention
  analysis stays textual. Make it parse too and deadenums becomes *more*
  expensive than it is now.
- **Correctness argument: different in kind.** deadsweep's case is that the AST
  replaces two hand-written extent scanners whose comments record three occasions
  on which they took the wrong brace. deadenums' case is that DWARF is not a
  function of the declarations — it is a function of the *uses* — so the tool's
  own reference for "did anything move?" is missing precisely the names gcc
  decided it did not need. Measured: 32 names absent from the dump on one real
  input, and a whole `keptBack` branch in the tool to survive the day one of them
  is a survivor.

### A staged plan

1. **Write the value extractor as a `ccx`-shaped check, not a tool.** A file in
   `internal/ccx` in the `Result`/`Finding` partition shape: classify every
   enumerator into *implicit successor*, *explicit literal*, *explicit
   expression*, *over another enumerator*, *fixed underlying type*, and refuse on
   a leftover. `.tmp/de/ev/main.go` is that program in draft; it has no
   `Partition` and it should. Nothing in the pipeline changes.
2. **Run it against `tools/enumvals.sh` over every boundary in `.build/`** — the
   comparison in §2, widened from 17 texts to all 44 tars and every pre-sweep and
   deadenums-input text that can be produced. The claim to establish is numeric
   equality on every name DWARF has, with the `-c` variant used where the link
   refuses. If any text disagrees, stop: that is a finding about the front end
   and belongs in `tx/FINDINGS.md`, not in a sweep.
3. **Assert the pin format**: every enumerator with no initialiser has a value
   below 65,536, on every text in step 2. That is the whole of what makes the
   swap byte-neutral, and it should be a check that can fail rather than a
   sentence in a survey.
4. **Then option 1**, and run `make whim-build-check`. One command answers it:
   either the committed bytes come back — and the `-g` build leaves the sweep for
   nothing — or they do not, and the diff names exactly which pin moved.
5. **Keep the DWARF path as the control.** Not as a fallback in the loop —
   *there is no tier below a phase to fall through to* — but as the thing that
   makes step 2 repeatable: a `--dwarf` flag on the same subcommand, and the
   twelve phase checks left exactly as they are. A check that cannot fail is not
   evidence, and gcc's dump is what lets this one fail.
6. **Only then reconsider the mention analysis**, and reconsider it together with
   deadsweep, because on its own it costs more than it saves. The measurement to
   take first is the one this survey could not: whether a parse cache keyed on
   the text digest actually hits in the 84 of 200 rounds where the bytes did not
   move between the two tools.

### What not to expect

Not a gcc-free sweep, and not a faster deadenums. `startSpec`'s speculative
`gcc -c -O0` runs every round in the background and has no AST answer; the twelve
phase checks that ask `tools/enumvals.sh` a before-and-after question would stay
on DWARF, by design, because a control that shares an implementation with the
thing it controls is not a control. The honest claim is narrower and firmer than
deadsweep's: **the enumerator values are not the one thing in the pipeline that
is not a function of the bytes alone — that was an artefact of asking gcc. They
are a function of the bytes, the front end computes them, and on 28,608 measured
values it computes the same ones.**
