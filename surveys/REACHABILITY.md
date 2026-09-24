# Could ONE reachability pass replace the whole deleting half?

A survey, not an implementation. Every number below was measured in this tree on
2026-09-23, on the machine the repository is checked out on. Nothing was
estimated; where a thing could not be measured it says so. Read after
`.tmp/deadsweep-survey.md`, whose result about deadsweep-versus-funcreach is
assumed and not re-derived. `.tmp/deadenums-survey.md` did not exist while this
was measured, so nothing here defers to it.

Short answer: **yes for what the closure sees, and it is not close — measured, on
five real texts, the set the current sweep removes is a STRICT SUBSET of the
complement of a reachability closure, with zero exceptions in either direction on
functions, objects and prototypes and 193 extra removals on one 123,740-line
text. The closure is a one-pass answer where the sweep needs up to four rounds.
What stops it being a swap is not the analysis: it is (a) three deliberate
semantic guards the six tools carry which a closure has no notion of, (b) a text
that does not parse, where a closure declines and there is no other tool left to
repair it, and (c) the fact that the product is 163 phases deep and any of this
moves the committed bytes.**

---

## 0. The instruments

Three throwaway programs under `.tmp/`, all untracked, none of which writes a
tracked file:

- **`.tmp/rsx/`** — the closure. `internal/cc` (the forked cc/v4) parses and type
  checks one translation unit; the program builds a graph over eight kinds of
  entity and reports, for every entity, whether it is reachable from the roots.
  Output is one line per entity, `KIND<TAB>ID<TAB>LINE<TAB>1|0`.
- **`.tmp/presweep/`** — `internal/build.RunPhase` on a run of phases with **no
  sweep**, so a phase's raw edit output can be examined.
- **`.tmp/trwhy/`** — replays `internal/dead`'s own `TypeReach` root and frontier
  computation and prints, for a named type, *why* it was called live.
- **`.tmp/hdr/`**, **`.tmp/evx/`** — the header and enumerator-value probes of §4
  and §5.

### The entity kinds

| code | what it is | how it is found |
|---|---|---|
| `F` | a function DEFINITION at file scope | a file-scope `*cc.Declarator` with `IsFuncDef()` |
| `O` | a file-scope object | a file-scope declarator, not a function, not a typedef |
| `P` | a prototype for a function never defined in the TU | a file-scope function declarator with no `IsFuncDef()` anywhere |
| `T` | a typedef | `IsTypename()` |
| `S` | a tagged struct or union definition | `StructOrUnionSpecifierDef` with `Token` (the tag) |
| `E` | a tagged enum definition | `EnumSpecifierDef` with `Token2` |
| `M` | a struct or union member | a `StructDeclarator`'s declarator |
| `N` | an enumerator | an `Enumerator`'s `Token` |

A **name merges all its declarators**, which the deadsweep survey established is
mandatory: `read++` lands on whichever declarator the use resolved to, in this
file on the prototype near the top and never on the definition. So the prototype
block and the definition are ONE `F` entity, and deleting it deletes both — which
is why the closure's `F` set covers both what deadsweep removes and what
deadprotos removes for the same function.

### The edges

Every reference the type checker resolved is charged, **by byte position**, to the
innermost entity span containing it:

- `PrimaryExpressionIdent.ResolvedTo()` → a `*cc.Declarator` (F/O/P/T) or a
  `*cc.Enumerator` (N);
- `PostfixExpression{Select,PSelect}.Field()` → the `*cc.Field`, joined to the
  declaring `M` by the **byte offset of the field's declarator** — exactly the
  resolution `cmd/whimtools/fieldref.go` exists to provide, and the thing that
  makes this survey's field answer different in kind from `deadfields`';
- `TypeSpecifier` of case `TypeName` / `StructOrUnion` / `Enum` → T / S / E.

Structural edges: `M → its owning S or T`, `N → its owning E or T`, `T → the tag
it names`.

### What the instrument's own errors are

Stated so that the numbers can be discounted honestly:

| | product | q82 | q41 | pre78 | five |
|---|---|---|---|---|---|
| field accesses that could not be joined to a declaration **in this file** | **0** | **0** | **0** | **0** | **0** |
| field ids that collided (two members merged into one entity) | 3 | 6 | 6 | 6 | 6 |
| references charged to no entity at all | 12 | 3 | 3 | 3 | 3 |

The zero row is the important one: the member join is total. The collisions merge
two distinct members under one id and can only make the closure *over-keep*, never
over-delete. The nil-owner references are all inside `static_assert`, which §3
turns into a root class rather than an error.

---

## 1. What the six delete, as sets

### The tools, read from the source

| tool | removes | evidence | reachability | needs a compiler | guards it carries |
|---|---|---|---|---|---|
| `deadsweep` | `F` (unused static function), `O` (unused static object, file AND block scope), `P` (`declared 'static' but never defined`) | **gcc's call graph**, `-flto -fno-fat-lto-objects -Wall -Wextra` | **one level** | **yes** | never `static inline`; never external linkage; declines when it cannot find the extent |
| `deadprotos` | `P` | the name occurs **exactly once** in the whole text | one level | no | the `osdef.h` and `xdiff.h` banner blocks are protected — `keepBanners` |
| `typereach` | whole top-level type definitions (`S`, `T`, and the `M`/`N` inside them) | mentions **outside every type definition** are roots; then transitive type→type | **transitive**, type graph only | no | a construct that also declares a variable is `Deletable=false` |
| `funcreach` | `F` | roots are `main` plus every defined name mentioned outside a body once prototype lines are stripped; then transitive | **transitive**, call graph only | no | `MinDefinitions = 100` floor; a name in a live body counts as reached even if it is a variable |
| `deadfields` | `M` | the member's **name** appears nowhere outside a type definition | one level, name-based | no | **refuses entirely while `ml_recover` exists** (a struct layout is a disk format); **skips any type ever initialised positionally**; never empties a struct |
| `deadenums` | `N` | the enumerator's **name** occurs exactly once | one level, name-based | **yes** (`gcc -O0 -g` + `readelf`, for pinning) | pins the first survivor after a deleted run to its DWARF value; keeps a run whose successor DWARF cannot value; refuses an enum where every constant is dead |

### Measured: each tool alone on one 123,740-line text

The text is `five.c` — phases 13–35 applied to `q12` with no sweep, then one round
of the four text-based tools and canon (so that it parses; see §6). Each tool was
run **on its own** on a fresh copy:

```
deadsweep    123499 lines   prototypes 85, functions 13, variables 97 -- 241 lines
deadprotos   123655 lines   85 declared, never defined, never called
typereach    123696 lines   deleted 29 definitions
funcreach    123740 lines   2534 definitions, 2534 reachable, 0 not
deadfields   123740 lines   0 fields nothing outside a type names
deadenums    123727 lines   12 enumerators nothing mentions, 5 survivors pinned
canon        123740 lines   fixpoint after 1 round
```

Pairwise overlap of the **source line numbers each removed**, same text, each run
alone:

| ∩ | deadsweep | deadprotos | typereach | deadenums |
|---|---|---|---|---|
| **deadsweep** (245) | 245 | 85 | 0 | 0 |
| **deadprotos** (85) | 85 | 85 | 0 | 0 |
| **typereach** (44) | 0 | 0 | 44 | 0 |
| **deadenums** (19) | 0 | 0 | 0 | 19 |

So on **this** text `deadprotos ⊂ deadsweep` exactly, and typereach, deadenums and
deadsweep are **pairwise disjoint**. `funcreach` and `deadfields` remove nothing
here at all.

The subset is not a general fact. On the **unparseable** text one step earlier
(`to35.c`, 128,641 lines, two hard C errors) the first round reports

```
deadsweep    prototypes 0, functions 71, variables 12  -- 1041 lines
deadprotos   47 declared, never defined, never called
```

— deadprotos finds 47 prototypes deadsweep found 0 of, because gcc's
`declared 'static' but never defined` is not emitted for the ones whose
definitions the same round is about to delete. **Neither subsumes the other
across texts**, which is the same shape of answer the deadsweep survey reached
for deadsweep-versus-funcreach, and for the same reason: two different questions
that agree on most texts.

`deadfields` and `funcreach` are the two whose sets are usually empty and
occasionally decisive: `deadfields` removed 8 fields in round 1 of the `to35.c`
sweep and 0 everywhere else measured; `funcreach` removed 0 on four of the five
texts and **6,058 lines in one round** on `to35.c`, where it is the only thing in
the sweep that repairs the text (§6).

---

## 2. Does a closure equal their union? — the decisive measurement

### Method

For each text: (a) run `.tmp/rsx` on it, giving the closure-unreachable set
**A**; (b) copy it, run `tools/st.sh sweep` on the copy to a fixpoint, run
`.tmp/rsx` on the result, and take **B** = entities(before) ∖ entities(after) —
what the *current sweep actually removed*, measured with the same instrument so
that the two sets are commensurable. Then compare A and B element by element.

The five texts: the committed product; the q82 boundary; the q41 boundary; the
phase-78 pre-sweep text (`q77` + phase 78's edits, no sweep, 89,297 lines — the
same text the deadsweep survey used); and `five.c`, the phases 13–35 accumulation
repaired by one round of the text tools, which is the only text measured where
every kind moves.

### Result

| text | lines | entities | **sweep removed** | **closure unreachable** | sweep ∖ closure | closure ∖ sweep |
|---|---|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 4,605 | **0** | **16** | **0** | 16 |
| q82 | 86,583 | 5,278 | **0** | **119** | **0** | 119 |
| q41 | 117,460 | 7,336 | **0** | **192** | **0** | 192 |
| phase 78 pre-sweep | 89,297 | 5,879 | **15** | **145** | **0** | 130 |
| phases 13–35, repaired | 123,740 | 7,831 | **283** | **476** | **0** | 193 |

**By kind**, on the two texts where the sweep removes anything:

| kind | pre78 sweep | pre78 closure | five sweep | five closure |
|---|---|---|---|---|
| `F` function | **15** | **15** | **13** | **13** |
| `O` object | 0 | 0 | **98** | **98** |
| `P` prototype | 0 | 0 | **85** | **85** |
| `N` enumerator | 0 | 5 | 51 | 56 |
| `M` member | 0 | 104 | 28 | 185 |
| `S` struct tag | 0 | 6 | 2 | 14 |
| `T` typedef | 0 | 15 | 6 | 25 |
| `E` enum tag | 0 | 0 | 0 | 0 |

And the 15 functions the sweep removed at pre78 are, name for name, the closure's
15:

```
add_b0_fenc clear_chartabsize_arg may_trigger_modechanged
may_trigger_win_scrolled_resized mch_early_init mch_new_shellsize ml_preserve
ml_setname out_flush_check pum_may_redraw set_b0_dir_flag set_init_3
set_init_default_printencoding set_init_lang_env trigger_undo_ftplugin
```

**So: on every text measured, the sweep's set is a subset of the closure's, and
the two are EQUAL on functions, objects and prototypes — 13 = 13, 98 = 98,
85 = 85 on the richest text, and 15 = 15 on pre78.** There is no element the
sweep removes that a closure would keep. `sweep ∖ closure = 0` on all five texts,
1,059 entities of sweep removals in total.

The difference is entirely `M`, `N`, `S`, `T`. Below is every construct behind it.

### 2a. The 16 the closure would remove from the committed product

The product is a sweep fixpoint: the sweep removes nothing from it, in one round.
The closure removes these:

| entity | line | source |
|---|---|---|
| `M:optexpand_T.oe_varp` | 2222 | `char_u *oe_varp;` |
| `M:optexpand_T.oe_opt_value` | 2223 | `char_u *oe_opt_value;` |
| `M:optexpand_T.oe_append` | 2225 | `int oe_append;` |
| `M:optexpand_T.oe_include_orig_val` | 2226 | `int oe_include_orig_val;` |
| `M:optexpand_T.oe_regmatch` | 2228 | `regmatch_T *oe_regmatch;` |
| `M:optexpand_T.oe_xp` | 2229 | `expand_T *oe_xp;` |
| `M:optexpand_T.oe_set_arg` | 2231 | `char_u *oe_set_arg;` |
| `T:optexpand_T` | 2232 | `} optexpand_T;` |
| `T:opt_expand_cb_T` | 2275 | `typedef int (*opt_expand_cb_T)(optexpand_T *args, int *numMatches, char_u ***matches);` |
| `M:vimoption.opt_expand_cb` | 48369 | `opt_expand_cb_T opt_expand_cb;` |
| `T:hash_T` | 1654 | `typedef long_u hash_T;` |
| `M:tabpage_S.tp_snapshot` | 1793 | `frame_T *(tp_snapshot[SNAP_COUNT]);` |
| `N:SNAP_COUNT` | 1784 | `enum { SNAP_COUNT = 3 };` |
| `M:cmdline_info_T.xp_context` | 1529 | `int xp_context;` |
| `M:exarg.skip` | 2422 | `int skip;` |
| `M:termrequest_T.tr_start` | 67187 | `time_T tr_start;` |

Five distinct mechanisms, each verified:

**(i) typereach's tail-identifier ownership.** `internal/dead/typereach.go` gives a
definition a set of "names it owns": the tag from the head, plus **every
identifier after the last `}`** — and when a definition has no brace at all,
`tail` is the whole body. So `typedef long_u hash_T;` owns the names
`{long_u, hash_T}`, and ANY mention of `long_u` anywhere outside a type
definition makes that typedef live for ever. Measured on a nine-line C file
(`.tmp/rs/tr1.c`), with `tools/st.sh typereach`:

```
with `long_u x = 1;` in main():   6 type definitions, 2 unreachable
without it:                       6 type definitions, 4 unreachable
                                     long_u        1 line
                                     hash_T,long_u 1 line
```

`long_u` occurs everywhere in this tree; `hash_T` occurs **once**, its own
definition. `.tmp/trwhy` confirms the same mechanism keeps `opt_expand_cb_T`
alive at q82:

```
def 665 line 2306 names=[args char_u matches numMatches opt_expand_cb_T optexpand_T]
    ROOT mention of "char_u" at offset 6625
def 652 line 2241 names=[optexpand_T]
    reached from def 665 via "optexpand_T"
```

A function-pointer typedef owns its **parameter names**. `char_u` is mentioned
6,000 bytes into the file, and from that one mention the whole `optexpand_T`
island — a typedef, a callback typedef, seven members and a member of `vimoption`
— is immortal. `opt_expand_cb` occurs **once** in the product; `optexpand_T` and
`opt_expand_cb_T` twice each (their own definitions and each other).

**(ii) typereach expands from a live DEFINITION, not from a live FIELD.** Once a
type is live, every identifier in its span marks its owners live. So a live struct
keeps the type of a member nothing ever reads. `.tmp/trwhy` on q82:

```
type_S  <- reached from dictvar_S via "type_T"
class_T <- reached from type_S   via "class_T"
class_S <- reached from class_T  via "class_S"
ufunc_T <- reached from class_S  via "ufunc_T"
ufunc_S <- reached from ufunc_T  via "ufunc_S"
int8_T  <- reached from type_S   via "int8_T"
```

The closure cuts the chain at the first member nothing accesses, because a member
is an entity with its own reachability. This is the single largest source of the
difference: at q82, 6 struct tags, 15 typedefs and 93 members — the whole vim9
class system (`class_S`, `type_S`, `ufunc_S`, `itf2class_S`, `listitem_S`,
`listwatch_S`, `ocmember_T`, `omacc_T`, `cfunc_T`, `cfunc_free_T`,
`class_builtin_T`, `find_func_t`, `int8_T`) plus `VIM_ACCESS_*` and
`CLASS_BUILTIN_MAX`. Verified independently with grep: `class_T` occurs 7 times at
q82 and every one is inside a type definition; `class_S` twice; `ufunc_T` 3 times;
`find_func_t` and `cfunc_T` **once each**.

**(iii) deadfields matches a NAME, not a FIELD.** `M:cmdline_info_T.xp_context`
and `M:exarg.skip`. `xp_context` occurs 5 times in the product: two declarations
(`expand_T` at 1502, `cmdline_info_T` at 1529) and three uses, all of
`expand_T`'s. `skip` occurs 39 times, almost all as a local. `deadfields`'
`outside` set is keyed on the bare name, so a member is safe as soon as *any*
identifier of that spelling appears anywhere. `cmd/whimtools/fieldref.go`'s
docstring is about exactly this failure and phase 122 had to compute the
partition by hand to take `mparm_T.term`. The closure resolves the access to a
`*cc.Field` and therefore to one struct.

**(iv) deadfields' regexp cannot see a parenthesised declarator.**
`frame_T *(tp_snapshot[SNAP_COUNT]);` does not match
`^([A-Za-z_][\w \t]*?[ \t\*])([A-Za-z_]\w*)[ \t]*(\[[^;]*\])?[ \t]*;$`. And
`deadenums` will not touch `SNAP_COUNT` because it occurs **twice** — its
definition and that array bound. Remove the member and the enumerator goes with
it; neither tool can take the first step. Measured: deleting line 1793 from the
product compiles **silently** under phase 82's flags.

**(v) deadfields' positional-initialiser guard — the one place the closure is
WRONG.** `M:termrequest_T.tr_start` occurs **once**, its declaration. It survives
because `termrequest_T` is initialised positionally three times:

```c
static termrequest_T crv_status = {STATUS_GET, -1};
static termrequest_T u7_status  = {STATUS_GET, -1};
static termrequest_T xcc_status = {STATUS_GET, -1};
```

Measured — delete line 67187 and compile the product:

```
.tmp/rs/notr.c:67198:49: warning: excess elements in struct initializer
gcc exit=0
```

A **warning**, and `gcc exits 0`. Here `tr_start` is the last member so the only
casualty is the `-1`; had it been first, `STATUS_GET` would have landed in
`tr_start` and `-1` in `tr_progress`, and gcc would have said the same thing.
That is why `deadfields` keeps every member of any type ever initialised without
designators, and it is a rule that lives in no tree: the closure has no reason to
know it. The pipeline's own gate would catch this one (a check on the sweep is
that gcc prints *nothing*), but only after the fact.

### 2b. The other three guards a closure does not have

- **`ml_recover`.** `DeadFields` returns `recoverable=true` and removes nothing at
  all while `ml_recover` is defined, because until the editor can no longer read a
  swap file *a struct layout is a disk format* and a member nothing reads is still
  a member another vim wrote. A closure would remove those members on the first
  text where `ml_recover` still exists. There is no `ml_recover` in the committed
  product or at q82, so none of the numbers above are affected — but every text
  before that phase would be.
- **Enumerator renumbering.** `deadenums` pins the first survivor after each
  deleted run to the value it had, and **keeps the run** when DWARF has no value
  for that survivor. A closure that deletes `N` entities must reproduce this or it
  silently re-points parallel tables. §5 shows the pinning input is available from
  the tree, which removes the DWARF dependency but not the rule.
- **`keepBanners`.** `deadprotos` protects the `osdef.h` and `xdiff.h` blocks
  because those prototypes describe code that lives elsewhere by design. In the
  closure these are the external-linkage roots (`P:xdl_diff`, `P:xdl_merge`,
  `P:xdl_mmfile_first`, `P:xdl_mmfile_size`, `P:_Xmblen`, `P:hkmap` — 6 at q41,
  5 at pre78), so the closure gets the same answer for a different reason. That
  agreement is measured (`P` removals 85 = 85, with 6 left standing) and not
  assumed.

### 2c. What this section does NOT establish

- The closure was **not applied**. No text was rewritten by it, so the claim
  "removing these 476 entities leaves a program that builds and behaves the same"
  is **unmeasured**. Two of the 476 classes were spot-checked by hand deletion
  (`tp_snapshot` silent, `tr_start` a warning); the rest were not.
- The closure's own over-keeping is unquantified in one place: 3–6 member ids
  collide per text (two members of differently-named anonymous structs merging),
  which can only keep a dead member alive, never delete a live one.
- `E` (tagged enums) never moved in either direction on any text. There are only
  4–5 of them per text.

---

## 3. The roots

A closure that gets these wrong deletes the program, so they were established from
the tree.

### Measured, per text

| text | external linkage | `static_assert`s | entities rooted by a `static_assert` | `__attribute__((used))` | `[[gnu::used]]` | `asm` | `alias`/`section`/`constructor`/`destructor` |
|---|---|---|---|---|---|---|---|
| product | **`main` only** | 17 | 9 | **0** | **0** | **0** | **0** |
| q82 | `main` only | 1 | 2 | 0 | 0 | 0 | 0 |
| q41 | `main` + 6 xdiff/osdef prototypes | 1 | 2 | 0 | 0 | 0 | 0 |
| pre78 | `main` + 5 | 1 | 2 | 0 | 0 | 0 | 0 |
| five | `main` + 6 | 1 | 2 | 0 | 0 | 0 | 0 |

`__attribute__` of any kind: 5 in the product, 137 at q82, 208 at q41, 227 on
`five` — **all `format`/`format_arg`**, none of which affects liveness. This
confirms the deadsweep survey's count of 0 for the liveness-bearing attributes and
extends it to the type, member and enumerator kinds.

### The root classes, named

1. **`main`.** The only external-linkage *definition* in the product.
2. **Every declarator with `Linkage() == External`.** In the product that is
   `main` alone; before phase 82 it also covers the six `osdef.h`/`xdiff.h`
   prototypes that `deadprotos`' `keepBanners` protects by name. **Stating the
   root as "external linkage" replaces a hard-coded list of two file names with a
   property the compiler computes.**
3. **Everything a `static_assert` names.** This was *discovered*, not assumed: the
   first run of the closure reported 11 references charged to no entity, and all
   eleven were inside `static_assert`:
   ```
   31925: static_assert(sizeof(PTR_EN) == 16, "a pointer entry is one node reference and one line count");
   73960: static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE, "cmdnames[] and enum CMD_index have drifted apart");
   75333: static_assert((usize)-1 == SIZE_MAX, "SIZE_MAX");
   75338: static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");
   ```
   Nine entities in the product (`O:cmdnames`, `N:CMD_SIZE`, `N:DB_LINE_MAX`,
   `N:PB_COUNT_MAX`, `T:PTR_EN`, `T:DATA_BL`, `T:DATA_LN`, `T:usize`, `T:time_T`).
   A `static_assert` is a declaration that cannot be deleted and that references
   things; not rooting it deletes the thing the assertion checks.
4. **The core/host interface.** For a closure run on `editor.c` — the core alone,
   cut at the first `#include` — `main` is not in the file and the roots are the
   names the *host* calls. Computed here by charging every reference at a byte
   offset past the cut (offset 2,008,932, line 75,313) and keeping the entities
   defined before it. **29 entities, computed and not listed:**
   ```
   F: _ alloc_clear deathtrap emsg emsg_iobuff_room iemsg iobuff_or musl_memcpy
      musl_memmove musl_memset musl_strchr musl_strlen musl_strncpy utf_ptr2cells
      utfc_ptr2len vim_main
   N: FAIL FALSE NUL OK TRUE
   O: IObuff e_out_of_memory_allocating_nr_bytes e_val_too_large
   T: char_u time_T usize uvarnumber_T varnumber_T
   ```
   16 functions, 5 enumerators, 3 objects, 5 typedefs. This is the mirror of what
   `make editor.c` already computes in the other direction.
5. **What a phase check depends on existing.** Not measured, and it is the one
   root class this survey cannot compute. `internal/check` is 163 programs; some
   assert on names (`cmdnames[]`, `nv_cmds[]`, the musl tables). A closure that
   deletes a name only a check reads would pass the build and fail
   `make whim-verify` hours later. The list would have to be extracted from the
   check programs, which is a separate exercise.

### How a root set would be stated so that it is checked rather than trusted

The tree's own rule: *assert a partition, not a count*. A closure's roots should
be a **classification of every entity**, refusing on a leftover:

```
every entity is exactly one of:
  reachable                              (the closure found it)
  root: external linkage                 (Linkage() == External)
  root: named by a static_assert         (a reference under a StaticAssertDeclaration)
  root: named across the core/host cut   (a reference past the first #include)
  root: declared by the check corpus     (an explicit, dated list, each with the check)
  unreachable                            (to be deleted)
```

and **fail if any entity falls outside**. The first three are computed from the
tree and cannot drift. The fourth is a list, and a list is the thing that rots —
so it should carry, per name, the check that needs it, and the closure should
refuse a name in the list that is not in the file (the list is stale) as loudly as
it refuses a leftover. The control that makes this evidence rather than a
demonstration already exists: **gcc**, which agrees with the front end on `F`, `O`
and `P` (474 of 475 names in the deadsweep survey; 13/98/85 exact here).

---

## 4. The headers

### 4a. The input's 41 `#include`s

Phase 82 deletes each of the 41 `#include <...>` lines in turn and keeps it out if
`gcc -fsyntax-only -O0 -Wall -Wextra -Wno-unused-parameter` prints **nothing**;
the survivors are then tried together, and when that fails they are re-tried one
at a time **from the bottom**. Reproduced here from `q77` through phase 82:

```
includes     28 of 41 can go on their own
includes     together they do not build; 23 removed one by one, from the bottom
includes     removed: limits.h sys/types.h dirent.h sys/time.h pwd.h sys/file.h
             strings.h setjmp.h locale.h float.h math.h inttypes.h stdbool.h
             sys/select.h wchar.h utime.h langinfo.h sys/sysinfo.h sys/wait.h
             stropts.h sys/utsname.h dlfcn.h sys/resource.h
```

— byte for byte what `internal/phase/082/GOAL.md` records. It costs **41 + 1 + up to 28**
compiles.

**The tree answer.** `.tmp/hdr` parses `q81` (87,076 lines, all 41 includes) and
records, for every declaration and every macro the file actually uses, the FILE it
came from: a declaration by `Declarator.Position().Filename`, a macro by finding
every token offset in the file where the token the tree carries is not what the
source text says and looking the source's identifier up in `ast.Macros`, whose
`Position()` is the `#define`. The result is 31 header files. Then `gcc -M` on a
one-line file per include gives each include's transitive file set, and the same
algorithm phase 82 uses is run over sets instead of compiles.

```
used files not reachable from any include: none
can go on their own: 25 of 41
together they do not build -> one at a time from the bottom
REMOVED: 23    KEPT: 18
```

**The same counts, 23 and 18.** The symmetric difference is two each way:

| | phase 82 (gcc) | the tree |
|---|---|---|
| removed | `stdbool.h`, `wchar.h` | `stdlib.h`, `string.h` |
| kept | `stdlib.h`, `string.h` | `stdbool.h`, `wchar.h` |

Both differences have one cause, and it is the same cause: **a position names one
definer, and the question is whether there is another.**

- `wchar.h` supplies exactly one thing the file uses: **`NULL`**. musl defines
  `NULL` identically in `stddef.h` (line 5) and `wchar.h` (line 42). The front end
  charges the expansion to whichever `#define` won; the file-set model then
  believes only `wchar.h` can supply it, pins it — and because musl's `wchar.h`
  includes `stdlib.h` and `string.h`, those two become redundant. That is
  precisely the failure `internal/phase/082/GOAL.md` records for its own first dry run:
  *"Walked top down, it dropped `<string.h>` and `<stdlib.h>`, whose declarations
  happen to arrive through headers further down, and kept `<wchar.h>`."* The tree
  model reproduces the bug phase 82's bottom-up order was written to avoid.
  Verified: `/usr/include/stdlib.h` is reachable from `stdlib.h` and `wchar.h`,
  `/usr/include/string.h` from `string.h` and `wchar.h`, `/usr/include/wchar.h`
  from `wchar.h` alone.
- `stdbool.h` supplies exactly `bool`, `true`, `false` — `#define bool _Bool` and
  friends. In **C23 these are keywords**, so the macros are redundant with the
  language. cc/v4 still records an expansion, so the tree charges the file; gcc
  compiles without it.

**What the tree would need to match gcc**: (1) compare macro *definitions* across
headers for identity, so that two identical `#define NULL` count as one supply —
cc/v4 has `Macro.isSame`, unexported; and (2) know which macro names the language
also provides. Neither is a rewrite; both are real work, and the honest statement
is that **a file-level model reproduces phase 82's counts and two of its 41
decisions differently, so it is not a drop-in for the trial compilation.**

### 4b. How many headers are needed only for a macro

Measured on `q81`, the 31 header files the file takes anything from:

| | count | files |
|---|---|---|
| declarations only, no macro | 11 (+ `<builtin>`) | `bits/alltypes.h` `bits/stat.h` `iconv.h` `string.h` `strings.h` `sys/ioctl.h` `sys/time.h` `termios.h` `time.h` `unistd.h` `wctype.h` |
| **macros only, no declaration** | **11** | `bits/errno.h` `bits/fcntl.h` `bits/ioctl.h` `bits/signal.h` `bits/stdint.h` `limits.h` `stdarg.h` `stdbool.h` `stddef.h` `sys/param.h` `wchar.h` |
| both | 9 | `bits/termios.h` `ctype.h` `errno.h` `fcntl.h` `signal.h` `stdio.h` `stdlib.h` `sys/select.h` `sys/stat.h` |

Of the 41 `#include` lines, four name a file that is in the macros-only column and
supplies nothing else directly: `limits.h`, `stdarg.h`, `stdbool.h`, `stddef.h`.
`sys/param.h` is macros-only for itself (`MAX`, `MIN`) but is kept because of what
it transitively supplies (§4c).

The macros, named, for the macro-only files:

```
bits/errno.h    EFBIG EINTR ENOENT EOVERFLOW
bits/fcntl.h    F_GETFD F_SETFD O_APPEND O_CREAT
bits/ioctl.h    TIOCGWINSZ
bits/signal.h   SA_RESTART SIGALRM SIGCONT SIGHUP SIGINT SIGPIPE SIGTERM SIGTSTP SIGWINCH
bits/stdint.h   SIZE_MAX
sys/param.h     MAX MIN
limits.h        INT_MAX INT_MIN LLONG_MAX LLONG_MIN LONG_MAX LONG_MIN PATH_MAX ULLONG_MAX
stdarg.h        va_arg va_copy va_end va_start
stdbool.h       bool false true
stddef.h        offsetof
wchar.h         NULL
```

**Is `internal/cemit/macro.go`'s property enough to attribute a macro to its
header?** The docstring states it exactly: *EVERY token of an expansion carries
the INVOCATION's position*. That is the position **in `whim-vim.c`**, not in the
header — so by itself it locates the *use* and says nothing about the *definer*.
Two more things are needed, and both exist:

1. the source text at the invocation offset, which gives the macro's NAME (this is
   `cemit`'s `says()` and `atExpansion()`, and it is how `.tmp/hdr` finds the
   expansions at all — a token whose spelling differs from the source at its own
   offset);
2. **`ast.Macros[name].Position()`**, which is the `#define`'s own position, in
   the header. `internal/cc/cpp.go:445`.

So: **yes, with `ast.Macros` beside it.** What it still cannot do is §4a's
problem — `Macro.Position()` names ONE `#define` where two headers carry the same
one, and there is no public way to ask cc/v4 whether the other definition is
identical.

### 4c. The product's 12 `#include`s — which must not move

The twelve, in file order from line 75,313:
`stdlib.h unistd.h sys/param.h time.h signal.h errno.h stdint.h stdarg.h stddef.h
sys/ioctl.h termios.h fcntl.h`.

The tree answer (18 used header files, each include's unique contribution):

```
stdlib.h     uniquely supplies /usr/include/stdlib.h            (EXIT_FAILURE)
unistd.h     uniquely supplies /usr/include/unistd.h            (getpid pipe2 read write)
sys/param.h  uniquely supplies limits.h, sys/select.h, sys/time.h
             (INT_MAX..PATH_MAX; fd_set select FD_SET FD_ISSET FD_ZERO; gettimeofday)
time.h       uniquely supplies /usr/include/time.h              (nanosleep time)
signal.h     uniquely supplies signal.h, bits/signal.h          (kill sigaction ... SIGWINCH)
errno.h      uniquely supplies errno.h, bits/errno.h            (__errno_location errno EINTR)
stdint.h     uniquely supplies /usr/include/bits/stdint.h       (SIZE_MAX)
stdarg.h     uniquely supplies /usr/include/stdarg.h            (va_arg va_copy va_end va_start)
stddef.h     uniquely supplies NOTHING
sys/ioctl.h  uniquely supplies sys/ioctl.h, bits/ioctl.h        (ioctl TIOCGWINSZ)
termios.h    uniquely supplies termios.h, bits/termios.h        (tcgetattr tcsetattr ECHO..XTABS)
fcntl.h      uniquely supplies /usr/include/bits/fcntl.h        (O_CLOEXEC O_NONBLOCK)
```

The gcc answer, asked directly — delete each of the twelve and compile the product
with phase 82's flags:

```
stdlib.h     SILENT -> removable        stdint.h     SILENT -> removable
stddef.h     6 diagnostics, error: unknown type name 'max_align_t'
unistd.h 17   sys/param.h 33   time.h 12   signal.h 61   errno.h 29
stdarg.h 45   sys/ioctl.h 8    termios.h 81  fcntl.h 8
```

**The two answers disagree on 3 of the 12, and the tree is right on two of them.**

- `stddef.h` — gcc right, tree wrong. musl's `bits/alltypes.h` is a multi-pass
  header gated on `__NEED_*` macros: `max_align_t` exists only when `stddef.h`
  asked for it. A file-level model says "alltypes.h is reachable from eight of the
  twelve" and concludes wrongly. **This is the general objection to file-level
  header reachability on musl and it is not repairable by more careful sets.**
- `stdlib.h` and `stdint.h` — **tree right, gcc wrong.** They are "removable" only
  because the thing that uses them degenerates. The core declares its own
  `enum usize { SIZE_MAX = (usize)-1 }` at line 18 and `enum { EXIT_FAILURE = 1 }`
  at line 20 — *before* the includes — and the host half checks them against the
  headers:
  ```
  75333: static_assert((usize)-1 == SIZE_MAX, "SIZE_MAX");
  75335: static_assert(1 == EXIT_FAILURE, "EXIT_FAILURE");
  ```
  Remove `#include <stdint.h>` and `SIZE_MAX` is no longer the header's macro but
  the core's enumerator, the assertion becomes `(usize)-1 == (usize)-1`, and gcc
  is silent. The trial compilation cannot tell "nothing needed it" from "the thing
  that needed it stopped asking". Verified: with `stdint.h` removed the
  preprocessor defines no `SIZE_MAX` at all, and the file still compiles clean.

So for the twelve that must not move, **the tree model is the safer instrument**,
and re-running phase 82's step on the product today would delete two headers and
silently disarm two static_asserts. That is not a hypothetical about a future
refactor; it is a property of the instrument as it stands.

---

## 5. Fixpoint or one pass

### Rounds measured

| segment | boundary in | sweeps | rounds | rounds/sweep | wall |
|---|---|---|---|---|---|
| phases 78–82 (shared) | q77 | 4 | **8** | 2.00 | — |
| phases 140–154 (each) | q139 | 15 | **19** | 1.27 | 63 s |
| phases 13–35 accumulated, **unparseable** | q12 | 1 | **4** | 4.00 | — |
| the same text repaired first | — | 1 | **3** | 3.00 | — |

(The 140–154 figures reproduce the deadsweep survey's 19 rounds / 15 sweeps /
64 s exactly, on a separate run. Its 13–40 = 17/5 and 42–62 = 25/8 are taken as
established and were not re-run.)

In the `each` stages, **12 of the 15 sweeps take exactly one round and remove
nothing at all**; the rounds are spent proving a fixpoint, not reaching one.

### What each round was still removing — the richest case

`to35.c`, 128,641 lines, the phases 13–35 accumulation, 4 rounds,
128,641 → 123,339:

| round | deadsweep | deadprotos | typereach | funcreach | deadfields | deadenums | canon |
|---|---|---|---|---|---|---|---|
| 1 | 0 P, **71 F**, **12 O** (1,041 lines) | **47 P** | 14 defs | **many** (`au_del_group`…) | **8 M** | **12 N**, 5 pinned | changed |
| 2 | **45 P**, 6 F, **85 O** (152 lines) | 5 P | 36 defs | 0 | 0 | 1 N | changed |
| 3 | 1 P, 0 F, 1 O (2 lines) | 0 | 1 def | 0 | 0 | 0 | settled |
| 4 | 0 | 0 | 0 | passed | passed | passed | passed |

**Would a closure have got rounds 2 and 3 in round 1?** Everything in them is a
consequence of round 1's own deletions: the 85 objects were read only by the 71+
functions round 1 deleted; the 45 prototypes are `declared 'static' but never
defined` only because funcreach took the definitions; the 36 type definitions are
orphaned by the removed fields and objects. A transitive closure computes all of
this before deleting anything, so **yes for every row above** — and the argument
is not an empirical one, it is that **removing exactly the unreachable set is
idempotent**: reachability of the survivors is unchanged by deleting nodes no
survivor can reach, so a second closure over the result finds nothing new. That is
a property of the construction, and it was **not verified by running it**, because
the closure was never applied to a text (§2c).

What would still need a second look, even with a closure:

- **`deadenums`' renumbering.** Deleting an enumerator moves every implicit one
  after it. The pinning is an edit, not an analysis, and it is not idempotent in
  the same way — the pinned value has to come from *before* any deletion.
- **`deadfields`' positional guard and `ml_recover`** (§2b), which are refusals
  and do not iterate.
- **canon**, which must still run afterwards because deletion leaves blank runs.
  Measured: canon on the product is **0.154 s**, and it reaches a fixpoint in 1
  round on all five texts.

### Cost per call

| text | lines | one whole sweep round (all 7 tools, gcc included) | **one closure pass** |
|---|---|---|---|
| committed product | 77,306 | **3.28 s** | **1.95 / 2.05 s** |
| q82 | 86,583 | — | 2.31 / 2.19 s |
| q41 | 117,460 | — | 3.31 / 3.28 s |
| five | 123,740 | — | 3.65 / 3.59 s |

The closure includes the parse (0.62 s on the product, per the deadsweep survey),
the type check, three reflective walks and the transitive closure. It is roughly
**1.7× cheaper than one round** and would replace **1.3 to 4 rounds**, so the
saving on the sweep is somewhere between nothing and threefold; the honest
statement is that the deleting half's cost is dominated by *how many rounds* and
the closure makes that number 1.

### One gcc user the deadsweep survey called irreducible can also go

The deadsweep survey concluded that `deadenums`'s DWARF dump is *the one thing in
the pipeline that is not a function of the bytes alone*. Measured here, that is
no longer true. `cc.Enumerator` embeds a `valuer`:

```
enumerators 1155, with a value from the front end 1155
```

and compared against `tools/enumvals.sh` (`gcc -O0 -g` + `readelf
--debug-dump=info`) on the committed product:

```
dwarf 1155   cc 1155
names only in dwarf: []      names only in cc: []
value disagreements: 0
```

**1,155 of 1,155, exact, with no name in either dump the other lacks.** (The raw
files differ on 29 lines only in spelling — DWARF prints `0x10000`, the front end
`65536`.) So the enumerator values a closure needs for pinning can come from the
tree, and `gcc -g` + `readelf` leaves with it. What does **not** leave is
`startSpec`, the speculative `gcc -c -O0` the sweep runs every round in the
background so that the settling round leaves an object `phasebuild` can link.

---

## 6. The blocker, for a pass that replaces the WHOLE deleting half

The deadsweep survey measured 30 of 31 pre-sweep texts parsing, with the one
failure bisected to phase 35. Reproduced here exactly: phases 13–35 from `q12`
with no sweep leaves 128,641 lines, and

```
cc:   .tmp/rs/to35.c:123724:79: type winopt_T has no member named wo_eiw
gcc:  2 errors
```

For a tool that replaces **one** member of the round, declining is cheap: the
other five run, and the deadsweep survey measured funcreach alone repairing the
text. For a pass that replaces **all six**, declining is total. Measured:

| what runs on the unparseable text | lines after | does it parse? |
|---|---|---|
| nothing (the closure declines; the round is empty) | 128,641 | **no**, for ever |
| **canon alone** — the only non-deleting member left | 128,640 | **no** |
| the four text-based deleters + canon, one round | 123,740 | **yes** (`parsed in 1.112s`) |
| the current sweep, 4 rounds | 123,339 | yes |

**canon cannot repair it.** It removed one line (a trailing blank) and the two
errors stand. So a closure-only sweep on this text reaches its fixpoint
immediately, having removed **0** entities where the current sweep removes 5,302
lines — and since the sweep is what the phase leaves behind, that boundary and
every one after it moves.

The repair lives in **funcreach**, and it is not an accident of that text:
funcreach is textual, does not care that the text is broken, and deletes the
unreachable `check_window_scroll_resize` that holds the broken reference. The
pipeline is *designed* around that — the edit cuts, the sweep collects — so
requiring every edit to leave a compiling tree is a different pipeline.

One further fact, measured and mildly reassuring: the sweep's fixpoint does not
depend on which tool repairs the text. Running the four text tools first and then
the full sweep gives a file **byte-identical** to running the full sweep from the
start (`cmp` clean, both 123,339 lines).

And one fact that contradicts the deadsweep survey's "`Other` is empty in
practice": on phase 80's text the sweep reports

```
sweep 1   prototypes 2, functions 4, variables 2, left alone 49 -- 95 lines removed
```

**49 warnings deadsweep classified as `Other` and acted on for none of them.**
What they are was not measured (the kept stderr in `.cache/compile/` is
overwritten each round); that they exist is measured.

---

## 7. Recommendation

**One pass can replace the deleting half's ANALYSIS. It cannot replace the
deleting half.** The closure is strictly better at the question all six tools are
asking, by a measured margin of 193 entities on one text and 119 on a finished
boundary, with zero disagreements in the other direction across 1,059 sweep
removals. What it cannot replace is three refusals that encode facts about disk
formats, positional initialisers and DWARF gaps, and one structural property: that
the sweep must work on text that does not parse.

### Options, in cost order

**Option 1 — a closure as a REPORTER, changing nothing (cheapest, and the only one
that is free).** Ship it as a `whimtools` subcommand and an `internal/ccx`-shaped
partition: classify every entity into *reachable*, *root: external linkage*,
*root: static_assert*, *root: across the core/host cut*, *unreachable*, and
**refuse on a leftover**. Nothing in the pipeline changes.
*What would prove it*: run it on every tar in `.build/` and require
`sweep ∖ closure = ∅` on all 43, as measured here on 5. If any text disagrees,
that is a finding about the front end and belongs in `internal/gen/FINDINGS.md`.
*What it buys*: a standing answer to "what is still dead in the product", which is
today 16 entities and which nothing in the tree reports.

**Option 2 — replace `deadenums`' DWARF dump with the front end's values (cheap,
self-contained).** Measured: 1,155 of 1,155 exact. This removes one of the sweep's
three gcc users and makes the enumerator values a function of the bytes.
*What would prove it*: `make whim-build-check` — the committed bytes back. The
pinning is a textual edit and the input to it is a name→value map, so if the map
is identical the edit is identical, and the product cannot move.
*Risk*: a text where the front end's value and gcc's differ. Run the comparison
over every boundary first.

**Option 3 — let the closure REPLACE typereach + deadfields + deadenums' analysis,
keeping deadsweep, deadprotos and funcreach (moderate).** These three are exactly
where the closure dominates — the 193 extra entities are all `M`, `N`, `S`, `T` —
and funcreach stays, so the unparseable text still repairs itself and the closure
can decline when it must.
*What would prove it*: `make whim-build-check`, and it will **fail**: the closure
removes 119 entities from q82 that the current sweep leaves, so the product moves
by construction. The cost is a re-derived `whim-vim.c`, `slim.sha`,
`editor/editor.go` and every recorded boundary, plus a full `make whim-verify`
(hours), plus `.reference/core-baselines`, which **phase 83 refuses to
overwrite** — so what moved at q82 must be named, one entity at a time, before
the set is removed. 119 names is a long but finite sentence.
*And the three guards must be carried over verbatim*: the `ml_recover` refusal,
the positional-initialiser exclusion, the DWARF-gap run-keeping. Measured, the
second of those is load-bearing on the committed product today: deleting
`termrequest_T.tr_start`, which the closure calls dead and which occurs once,
makes gcc print `excess elements in struct initializer` and exit 0.

**Option 4 — one closure replaces all six (most expensive, not recommended).**
It fails on the one text measured that does not parse, and the failure is not a
degradation: 0 entities where 5,302 lines should go, and every boundary from
phase 35 on moves. To make it work, either every phase edit must leave a
compiling tree (a different pipeline from the one `GOALS.md` describes) or a
textual funcreach must stay in the round as the repair — at which point it is
Option 3.

### The staged plan for Option 1 → 2 → 3

1. **Write the closure as a partition reporter** (`internal/ccx`'s `Result`/
   `Finding` shape), with the five root classes computed and the leftover refusal.
   Nothing changes. The control is gcc: `-Wunused-function`/`-Wunused-variable`
   must name exactly the closure's `F` and `O` sets, which is what makes the
   check able to fail.
2. **Run it over all 43 boundaries in `.build/`** and require `sweep ∖ closure =
   ∅` per class. This survey did 5; the claim is only as good as the widest run.
3. **Swap `tools/enumvals.sh` for the front end** behind `dead.DumpVals`, keeping
   the shell script as a `--dwarf` control. `make whim-build-check` answers it in
   one command: either the bytes come back, and the swap is free, or they do not,
   and the swap is wrong.
4. **Carry the three guards into the closure as REFUSALS, not as filters**, each
   with the measurement that justifies it: the `ml_recover` test on the text, the
   positional-initialiser scan, the DWARF/front-end gap. A guard written as a
   filter silently keeps things; written as a refusal it says what it would not
   touch and why.
5. **Only then** replace typereach/deadfields/deadenums' analysis, and budget for
   a re-derived product: the bytes WILL move, so the work is the naming, not the
   code.

### What it would cost to be wrong

The product is 163 phases deep and its gate is total: `make whim-build-check`
requires the committed `whim-vim.c` back, byte for byte, and `make whim-verify`
costs hours on top. A closure that deletes **one** thing too many is not a
regression that shows up as a diff — it is a binary that differs, and the deltas
are declared in advance, so the phase that carries the change refuses and the pass
stops. That is the good case.

The bad case is the one measured in §2a(v): a deletion that gcc greets with a
**warning** and exit status 0. `excess elements in struct initializer` is not an
error. The pipeline catches it only because a check on the dead-code sweep is that
it prints *nothing* — never that it succeeded. Had `tr_start` been the first
member rather than the last, the initialiser `{STATUS_GET, -1}` would have put
`STATUS_GET` in the wrong field and gcc would have said the same thing. Six of
the 16 entities the closure would take from the committed product are members of
types this survey did not check for positional initialisation.

So the cost of being wrong is bounded by the gate and unbounded by the analysis:
the gate always notices that the bytes moved, and never tells you which of the
119 movements was the mistake. That is the argument for Option 1 first — a
reporter that has agreed with gcc on 43 boundaries is a different proposition from
one that has agreed on 5.
