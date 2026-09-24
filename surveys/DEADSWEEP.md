# Could deadsweep be done inside the AST?

A survey, not an implementation. Every number below was measured in this tree on
2026-09-23, on the machine the repository is checked out on (64 cores, warm page
cache). Nothing here was estimated from general knowledge about C; where a thing
could not be measured, it says so.

Short answer: **yes, and the front end gives the same answer gcc does — measured,
by set equality, on seven real texts totalling 474 names. What blocks it is not
the analysis but the position deadsweep holds in the round: it is the FIRST tool
in the sweep, and the text it is handed at the start of a sweep does not always
parse.** It was measured not parsing exactly where it removes the most.

---

## 1. What deadsweep removes today, exactly

`internal/dead/deadsweep.go` is 323 lines and does three things: run gcc, read
two warning options out of its stderr, and delete the line ranges they name.

### The gcc call

```go
exec.Command("gcc", "-c", "-O0", "-flto", "-fno-fat-lto-objects",
    "-Wall", "-Wextra", "-Wno-unused-parameter", "-o", "/dev/null", path)
```

The docstring says why `-flto` and not `-fsyntax-only`: the two warnings it reads
*come from gcc's call graph, which `-fsyntax-only` never builds and so reports
neither*, and `-flto -fno-fat-lto-objects` builds the call graph, warns, and
writes GIMPLE instead of compiling. The recorded numbers are *2.4 seconds instead
of 6.0 on a 129,000-line file*.

Verified here on the committed `whim-vim.c` (77,306 lines), three runs each:

| gcc line | time |
|---|---|
| `-c -O0 -flto -fno-fat-lto-objects -Wall -Wextra` | 1.38, 1.38, 1.48 s |
| `-c -O0 -Wall -Wextra` (real object, full codegen) | 3.52, 3.59 s |
| `-fsyntax-only -O0 -Wall -Wextra` | 0.27, 0.28 s |

The docstring's *ratio* reproduces (2.5× vs 2.4× here); the absolute numbers are
this machine's, which is about 1.7× faster than the one the comment was measured
on. `-fsyntax-only` is 5× cheaper still and, as the docstring says, useless: it
emits neither warning.

`gcc`'s exit status is deliberately not consulted — see §3, where that turns out
to be the whole point.

### The three warning classes

```go
neverDefined = `'(\w+)' declared 'static' but never defined`
deadFunction = `'(\w+)' defined but not used \[-Wunused-function\]`
deadVariable = `(?:'(\w+)' defined but not used|unused variable '(\w+)') \[-Wunused(?:-const)?-variable=?\]`
```

- `neverDefined` → delete that one line (a prototype). Counted as `Proto`.
- `deadFunction` → `FunctionExtent()`, delete the range. Counted as `Func`.
- `deadVariable` → `DeclarationExtent()`, delete the range. Counted as `Var`.
- anything else → `Other`, and nothing is deleted.

The comment above them is one of the load-bearing sentences in the file: the
warning *text* is the same for a function and a variable, and only the option in
brackets says which, so *matching on the text alone treats an unused error string
as a function definition and deletes 500 lines*.

Note that `deadVariable` matches **both** the file-scope form (`'X' defined but
not used`) and the block-scope form (`unused variable 'X'`). Measured across every
gcc run in this survey: 172 `-Wunused-variable` warnings, of which **160 file
scope and 12 block scope**. So deadsweep deletes unused *locals* too, not only
file-scope statics.

Measured across the same runs, the only two options gcc ever emitted on this
tree's text were:

```
379  [-Wunused-function]
172  [-Wunused-variable]
```

Nothing else — no `-Wunused-but-set-variable`, nothing from `-Wextra`. That is
why `left alone 0` is what the sweep reports: `Other` is empty in practice.

### What it deliberately leaves alone

- **Parameters**: `-Wno-unused-parameter` is on the command line.
- **Anything with external linkage**: `-Wunused-function` and `-Wunused-variable`
  only fire for `static`. Measured on a 118,499-line mid-pipeline text: 2,529
  static function names, **7 external**, 1,262 static objects, **0 external
  objects**. So in this tree the restriction costs almost nothing.
- **`static inline` functions**: gcc does not warn about an unused `static
  inline`. Measured: 7 in the committed `whim-vim.c`, 7 in `editor.c`, 7 at q82,
  9 in a mid-pipeline text. This is the one place the two notions were measured
  to disagree — see §4.
- **An extent it cannot find**: both `FunctionExtent` and `DeclarationExtent`
  return `ok=false` rather than guess (a runaway declaration over 4,000 lines, an
  unbalanced brace), and the warning is then counted as `Other` and dropped.
- **Its own output**: the kept stderr and its sha go to `.cache/compile/`, never
  the work tree, *because a file left in the work tree is a file the boundary
  digest counts*.

### How the result feeds the fixpoint

`internal/sweep/sweep.go` runs seven tools per round, in this order and no other:

```
deadsweep, deadprotos, typereach, funcreach, deadfields, deadenums, canon
```

with `canon` last *because the sweep DELETES, and deletion leaves blank runs that
only canon removes*; moving canon before the loop *was measured to commute on one
phase and then moved 27 of 32 boundaries when the whole pass ran*.

The round repeats until the text stops changing, with `MaxRounds = 15` a hard
failure. Two economies matter here:

- **`passed`**: a tool is not run again on text it has already passed, since every
  one is a pure function of the bytes. A tool that *changes* something clears the
  memory.
- **`startSpec`**: on every round the sweep starts a *second*, background
  `gcc -c -O0` of the round's starting text, so that the round which changes
  nothing leaves an object `phasebuild` can link. This is a full compile — 3.5 s
  on the committed product — and **an AST deadsweep would not remove it.**
- **`deadenums`** dumps enumerator values through `tools/enumvals.sh`, which is
  `gcc -O0 -g` plus `readelf --debug-dump=info`. Also not removable.

So *"no gcc"* is not on the table for the sweep. Only deadsweep's warning call is.

### deadsweep versus funcreach

`internal/dead/funcreach.go` already computes reachability from text, with no
compiler at all. The two are genuinely different and **neither subsumes the
other**, which was measured rather than reasoned:

| | deadsweep | funcreach |
|---|---|---|
| source of truth | gcc's call graph | regexps over the text |
| what it finds definitions by | gcc | `^name(args)$` at column 0, brace matching |
| reachability | **one level** — "is this static referenced anywhere at all" | **transitive**, from roots |
| roots | none (it is not a reachability question) | `main`, plus every defined name mentioned outside any function body once prototype lines are stripped |
| linkage | `static` only | any file-scope definition |
| objects | yes — file-scope statics *and* block-scope locals | none at all |
| prototypes | `declared 'static' but never defined` | none |
| needs the text to compile | yes (see §3) | **no** |

The measurement, on the text phases 13–40 leave before the stage's sweep
(127,570 lines):

- deadsweep, running first as it does today: **125 functions, 20 variables, 2,772
  lines**. funcreach, later in the same round: **151 definitions, 5,716 lines**.
- funcreach run **alone** on that same text, with deadsweep never run:
  **269 definitions, 8,313 lines**, leaving 118,719 lines. gcc on the result still
  reports **151 unused functions and 143 unused variables**.

So what is left for a deadsweep — AST or gcc — that funcreach does not do:

1. **Every object.** funcreach has nothing to say about a variable. 143 file-scope
   objects on that one text, and 12 block-scope locals across the survey.
2. **151 functions funcreach's roots kept.** funcreach's roots are textual: a
   handler named in a table is a root whether or not the table is itself dead, and
   *a name that merely appears in a live body counts as reached, even where it
   might be a variable of the same name* (the docstring says so, and this tree has
   the case: `call_update_screen` is a file-scope `static int` at line 86535 **and**
   a parameter name in two functions).
3. **Prototype lines** for statics that are declared and never defined.

---

## 2. What the front end offers

`internal/cc` is modernc.org/cc/v4 v4.29.7 forked with two C23 productions added
(`internal/cc/README.md`). It is a full front end: preprocessor, parser, type
checker, scopes — and, relevant here, **it already counts uses.**

### Entering

Exactly what `internal/gen` and `internal/ccx.Parse` do, and what `whimtools parse`
does:

```go
cfg, err := cc.NewConfig("linux", "amd64")
ast, err := cc.Translate(cfg, []cc.Source{
    {Name: "<predefined>", Value: cfg.Predefined},
    {Name: "<builtin>", Value: cc.Builtin},
    {Name: path},
})
```

`cc.Translate` parses *and* type checks; the use counts below are documented as
*valid after Translate*.

### Scopes

```go
type Scope struct {
    Children []*Scope
    Nodes    map[string][]Node
    Parent   *Scope
}
```

`ast.Scope` is the file scope. `Nodes` maps a name to every node declared under
it, and **a name has several declarators**: the prototype block near the top of
this file and the definition further down are two `*Declarator`s under one key.
That is not a detail an implementation can skip — see the trap in §4.

### What a declarator records

`internal/cc/ast2.go`, all of it already public:

```go
func (n *Declarator) Name() string
func (n *Declarator) NameTok() Token          // Position().Offset is a byte offset
func (n *Declarator) Position() token.Position
func (n *Declarator) Type() Type              // Kind() == cc.Function for a function
func (n *Declarator) Linkage() Linkage        // External | Internal | None
func (n *Declarator) IsStatic() bool
func (n *Declarator) IsExtern() bool
func (n *Declarator) IsFuncDef() bool
func (n *Declarator) IsTypename() bool
func (n *Declarator) IsParam() bool
func (n *Declarator) IsSynthetic() bool
func (n *Declarator) HasInitializer() bool
func (n *Declarator) LexicalScope() *Scope

func (n *Declarator) ReadCount() int          // n.read
func (n *Declarator) WriteCount() int         // n.write
func (n *Declarator) SizeofCount() int        // n.sizeof
func (n *Declarator) AddressTaken() bool      // n.addrTaken
```

`ReadCount` is incremented in `check.go` at the one place an identifier resolves
(`PrimaryExpressionIdent`, lines 5137/5149/5164, `d.read++` under `mode&noRead ==
0`), so it is a **semantic** count: it follows scopes, and a parameter shadowing a
file-scope static does not increment the static. `AddressTaken` is set by
`c.takeAddr` (line 4691) for an explicit `&x`. `WriteCount` is incremented for an
*initializer* as well as for an assignment (`check.go:1205`, `check.go:1282`) —
which matters, see §4.

`PrimaryExpression.ResolvedTo()` gives the node an identifier resolved to; that is
how `internal/ccx`'s `takeAddr` and `internal/gen`'s `fnDecls` work.

### Spans to delete

The AST gives exact byte offsets, which is the real prize:

- `FunctionDefinition{ Declarator, DeclarationSpecifiers, CompoundStatement }`,
  and `CompoundStatement{ Token, Token2 }` — `Token2` **is the closing brace**.
- `Declaration{ ..., Token, Token2, Token3 }` carries the terminating `;`.
- `Token.Position()` returns a `go/token.Position`, which has `Offset`.

So the span of a function definition is
`DeclarationSpecifiers.Position().Offset … CompoundStatement.Token2.Position().Offset+1`,
computed rather than scanned. Compare what it would replace:

- `FunctionExtent()` walks *backwards* over the return type line by line and then
  counts braces on blanked text.
- `DeclarationExtent()` counts braces *and* parentheses forward to a terminating
  semicolon, then walks backwards again for two separate cases, each of which is
  documented as having been got wrong once — *gcc reports the unused variable at
  `} mouse_table[] =`, and running forward from there takes the initialiser and
  leaves the struct body open. The next declaration lands inside it and gcc says
  "expected specifier-qualifier-list before 'static'" a hundred lines later — which
  is how this was found, in the phase that removed the mouse* — and the second
  case, the same type written on one line, which *swallowed the next declaration*.

Those two comments are the argument for the AST, and it is a correctness
argument, not a speed one.

### The two existing consumers

- **`internal/gen/analyze.go`** (656 lines) walks with its own reflect-based walker,
  keys declarators by a string, keeps `fnDecls map[string]*cc.Declarator` and an
  `addr map[string]bool` for *functions used as values*. It already does the
  merge-by-name that a deadsweep needs.
- **`internal/ccx/*.go`** (2,099 lines, nine checks) has `Parse(path)`, `walk`
  and `walkDepth` — the same reflect walk — and the `Result`/`Finding` partition
  reporter. A deadsweep would be a tenth file there, or a `Partition` in the same
  shape: *classify every occurrence into the classes a rule serves and refuse on a
  leftover*.

### The concrete API an AST deadsweep would use

```go
ccx.Parse(path)                    // or the three-source cc.Translate above
ast.Scope.Nodes                    // file scope, name -> []Node
n.(*cc.Declarator)
  .IsSynthetic(), .IsTypename()    // skip
  .Position().Filename == path     // skip <predefined>/<builtin>
  .Linkage() == cc.Internal        // gcc only warns about static
  .Type().Kind() == cc.Function
  .IsFuncDef()                     // definition, vs prototype
  .ReadCount() / .WriteCount() / .SizeofCount() / .AddressTaken() / .HasInitializer()
scope.Children                     // for block-scope locals
FunctionDefinition.CompoundStatement.Token2.Position().Offset   // exact span
```

---

## 3. THE BLOCKER, measured

The front end can only answer about text it can parse. The sweep's input is text
six deleting tools have already cut, and **deadsweep runs first** — before
funcreach, before canon, on whatever the phase's edit left.

### Method

For each phase *N* whose input boundary `q(N-1)` is a tar in `.build/`, run that
phase's edit steps **alone, with no sweep** (`internal/build.RunPhase`, driven by
a throwaway program under `.tmp/`), and ask three questions of the text it left:
does `internal/cc` parse it, does gcc accept it, and what does each say is unused.
Then run the sweep's seven tools in `sweep.go`'s order and ask again after each.

### Result: 31 of 32 pre-sweep texts parse

Every text one phase's edit leaves, for all 28 phases whose input boundary is on
disk (80, 81, 82, 84, 85, 116, 117, 124, 129, 137, 139–144, 146–148, 150–155,
158–160), plus 78, 79, 139 and 147 measured separately:

```
phase 80     87231 lines  cc ok  gcc exit 0, 0 errors, 8 unused-warnings
phase 81     87082 lines  cc ok  gcc exit 0, 0 errors, 1 unused-warnings
phase 82     86623 lines  cc ok  gcc exit 0, 0 errors, 0 unused-warnings
...  (all 28: cc ok, gcc exit 0, 0 errors)
phase 160    77307 lines  cc ok  gcc exit 0, 0 errors, 0 unused-warnings
```

And through a whole round, on three of them:

```
pre78   00-input      cc ok  gcc exit 0   89297 lines
pre78   01-deadsweep  cc ok  gcc exit 0   89235 lines
pre78   02-deadprotos cc ok  gcc exit 0   89226 lines
pre78   03-typereach  cc ok  gcc exit 0   89226 lines
pre78   04-funcreach  cc ok  gcc exit 0   89226 lines
pre78   05-deadfields cc ok  gcc exit 0   89226 lines
pre78   06-deadenums  cc ok  gcc exit 0   89226 lines
pre78   07-canon      cc ok  gcc exit 0   89201 lines

pre139  00-input      cc ok  gcc exit 0 (3 warnings)  77963 lines
pre139  01-deadsweep .. 07-canon   cc ok, gcc exit 0 throughout   77907 lines

pre147  00-input .. 07-canon       cc ok, gcc exit 0 throughout   77776 lines
```

No tool in the sweep was ever observed to *break* a text that parsed on the way
in. That was the hypothesis worth testing and it did not hold: it is not the other
five tools that leave rubble.

### The one failure, which is the whole case

The pre-sweep texts above are each **one** edit, and all 28 parse. A *shared*
stage runs every edit and then ONE sweep, so its deadsweep is handed the
accumulation. Phases 13–41 are one shared stage of 29 phases. Running phases
13–40 from `q12` with no closing sweep leaves 127,570 lines, and:

```
wt  00-input       cc FAIL  gcc exit 1 (145 warnings, 2 errors)  127570 lines
       whimtools: wt/x.c:122824:79: type winopt_T has no member named wo_eiw
wt  01-deadsweep   cc ok    gcc exit 0 (156 warnings, 0 errors)  124798 lines
wt  02-deadprotos  cc ok    gcc exit 0 ( 85 warnings)            124727 lines
wt  03-typereach   cc ok    gcc exit 0 ( 85 warnings)            124692 lines
wt  04-funcreach   cc ok    gcc exit 0 (203 warnings)            118634 lines
wt  05-deadfields  cc ok    gcc exit 0 (203 warnings)            118606 lines
wt  06-deadenums   cc ok    gcc exit 0 (203 warnings)            118583 lines
wt  07-canon       cc ok    gcc exit 0 (203 warnings)            118499 lines
```

Read that first row twice. The text has **two hard C errors**. `internal/cc`
returns no AST at all and an AST deadsweep would have nothing to say. **gcc
returns its full answer anyway** — 125 `-Wunused-function` and 20
`-Wunused-variable`, exit status 1 — because `-Wunused-function` is emitted at the
end of the translation unit and gcc's error recovery gets there. deadsweep then
deletes 2,772 lines **including the function that holds the broken reference**,
and from row 2 onward the file both compiles and parses.

The broken reference is:

```c
122824:  int ignore_scroll = event_ignored(EVENT_WINSCROLLED, wp->w_onebuf_opt.wo_eiw);
122825:  int size_changed = !event_ignored(EVENT_WINRESIZED, wp->w_onebuf_opt.wo_eiw)
```

inside `check_window_scroll_resize`, which nothing calls. Phase 35 is the phase
that drops the option (`{Op: "dropoptions", Args: []string{"--local",
"eventignorewin"}}`, `internal/build/plan.go:258`); the field `wo_eiw` goes with
the option row and the two uses in a dead function are left standing **for the
sweep to remove**. That is the pipeline working as designed: *the edit cuts, the
sweep collects*. It is also, precisely, an unparseable intermediate.

### Bisected to one phase, not to the accumulation

Running 13→34 and 13→35 separately:

```
phases 13-34   129,252 lines   cc parses   gcc -fsyntax-only: 0 errors
phases 13-35   128,641 lines   cc FAILS    gcc -fsyntax-only: 2 errors
        .tmp/ds/to35.c:123724:79: type winopt_T has no member named wo_eiw
```

So it is **one phase's edit**, not 28 edits piling up. On that text, with two
hard errors in it:

```
gcc  -flto -Wall -Wextra   exit 1, 2 errors, 71 -Wunused-function, 12 -Wunused-variable
deadsweep                  prototypes 0, functions 71, variables 12 -- 1041 lines removed
result                     parses, 0 errors
AST deadsweep              would have removed 0
```

So the blocker is real, narrow, and not a corner case of the shared stages:

- **30 of 31** measured pre-sweep texts parse. **1 of 29** single-edit pre-sweep
  texts does not, and it is phase 35's.
- Where it fails, it fails on the round that removes the most: 1,041 lines from
  phase 35's own text, 2,772 from the stage's accumulation, against 0–62 lines in
  the 28 single-edit cases that do parse.
- `gcc`'s tolerance of a broken TU is not incidental — it is what lets the phase
  programs leave rubble in the first place. The docstring's *"a file that fails to
  compile yields no warning lines, and the tool is then a no-op"* is **wrong as
  stated**: measured, a file that fails to compile yielded 145 warning lines and
  the tool removed 2,772 lines from it.

### And the repair is already in the sweep

funcreach is text-based and does not care that the text is broken. Run **alone**
on the same 127,570-line text:

```
funcreach  2707 definitions, 2438 reachable, 269 not (8313 lines)
        -> 118719 lines, gcc exit 0, 0 errors
```

It deletes `check_window_scroll_resize` as unreachable and the text compiles. So
an AST deadsweep placed *after* funcreach in the round would have had a parseable
tree in this case too — and gcc on the repaired text still reports 151 unused
functions and 143 unused variables for it to find. That is the staged plan in §6,
and its cost is stated there.

---

## 4. Where the two would disagree — and where they were measured not to

Rather than reason about constructs, the two answers were computed on the same
text and the **sets** compared. `.tmp/astdead/main.go` walks `ast.Scope.Nodes`,
merges every declarator of a name, and calls a name unused by:

- **function**: `Linkage()==Internal && IsFuncDef() && read==0`
- **object**: `Linkage()==Internal && read==0 && sizeof==0 && !addrTaken && write <= (1 if HasInitializer)`
- **prototype**: `Linkage()==Internal && function && no declarator IsFuncDef()`

| text | lines | gcc names | AST names | agree | gcc only | AST only |
|---|---|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 0 | 0 | 0 | 0 | 0 |
| q82 boundary | 86,583 | 0 | 0 | 0 | 0 | 0 |
| phase 78 pre-sweep | 89,297 | 15 | 15 | 15 | 0 | 0 |
| phase 79 pre-sweep | 88,892 | 36 | 37 | 36 | 0 | **1** |
| phase 139 pre-sweep | 77,963 | 3 | 3 | 3 | 0 | 0 |
| 13–40 swept once | 118,499 | 130 | 130 | 130 | 0 | 0 |
| 13–40, funcreach only | 118,719 | 290 | 290 | 290 | 0 | 0 |
| **total** | | **474** | **475** | **474** | **0** | **1** |

The 118,499-line row is broken out by class, and all three match exactly:

```
gcc: 7 unused functions, 123 unused file-scope objects, 73 never-defined prototypes
AST: 7 unused functions, 123 unused file-scope objects, 73 never-defined prototypes
set difference in both directions: empty
```

### The one disagreement

`in_vim9script` at phase 79. It is

```c
    static inline int
in_vim9script(void)
{
    return FALSE;
}
```

**gcc does not warn about an unused `static inline`.** The AST's `read==0` does not
know that rule. Counts in this tree: **7** `static inline` in the committed
`whim-vim.c`, **7** in `editor.c`, **7** at q82, **9** in the 118,499-line
mid-pipeline text. An AST deadsweep must either reproduce gcc's exemption (one
`IsInline()` test) or accept that it deletes them. In this very case the
difference was absorbed within the round: **funcreach deleted `in_vim9script`
anyway**, three tools later, so the fixpoint was the same.

### The construct checklist, counted on the real source

| construct | committed `whim-vim.c` | `editor.c` | q82 | 118,499-line intermediate | does it make them disagree? |
|---|---|---|---|---|---|
| `&func` — address of a function taken | **0** | — | — | **0** (AST `AddressTaken`) | no case in this tree |
| address of an object taken | — | — | — | 233 | both count it as a use |
| function name used as a value (no `&`) | — | — | — | counted as `read` by the AST, as a reference by gcc | agreed on all 474 names |
| `asm` / `__asm__` | **0** | **0** | **0** | **0** | cannot arise |
| `__attribute__((used))` | **0** | **0** | **0** | **0** | cannot arise |
| `[[gnu::used]]` | **0** | **0** | **0** | **0** | cannot arise |
| `__attribute__` of any kind | 5 | 3 | 137 | 209 | all `format`/`format_arg`; none affects liveness |
| `weak`/`alias`/`section`/`constructor`/`destructor` | **0** | **0** | **0** | **0** | cannot arise |
| designated initializer `.field = name` | **0** | **0** | **0** | **0** | cannot arise; this tree's tables are positional |
| `sizeof` of a function | AST `SizeofCount` is available and was included in the object rule; no function had `sizeof>0` in any text measured | | | | no case |
| `if (0)` / `if (FALSE)` | **1** | **1** | **1** | **1** | see below |
| `static inline` | **7** | **7** | **7** | **9** | **yes — the one measured difference** |
| reference only from another dead definition | — | — | — | 151 functions on one text | **neither tool sees it; funcreach does** |

On `if (0)`: there is exactly one in the tree, and at `-O0` with `-flto` gcc's
call graph still contains the reference, so gcc calls the callee used — as does an
AST read count. Both over-keep identically. No disagreement was observed.

On *reference only from another dead definition*: this is where **both** gcc and
an AST read-count are one-level and wrong in the same direction. Measured: after
funcreach had already removed 269 unreachable definitions from the 127,570-line
text, gcc *still* named 151 unused functions, and the AST named the same 151. The
transitive question belongs to funcreach and would stay there.

### The trap an implementation must not fall into

The first version of the probe keyed on a **declarator** and not on a **name**,
and reported **1,555** unused static functions where gcc reported 7. The reason is
in §2: a name has several file-scope declarators, and `read++` lands on whichever
one the use resolved to — in this file, on the prototype in the block near the
top, never on the definition. Merging by name fixed it exactly (7 = 7).

The second version required `write == 0` for an object and reported **21** where
gcc reported 123, because `Declarator.write++` fires for an **initializer**
(`check.go:1282`). Allowing one write per `HasInitializer()` fixed it exactly
(123 = 123). Neither of these is visible from the API docs; both were found by
comparing against gcc.

---

## 5. Cost

### Per call

| text | lines | gcc `-flto` warning call | `cc.Translate` (`whimtools parse`) | parse + check + classify |
|---|---|---|---|---|
| committed `whim-vim.c` | 77,306 | 1.38–1.54 s | 0.616, 0.615, 0.624 s | 0.69 s |
| `editor.c` | 75,311 | — | 0.586, 0.681, 0.698 s | — |
| q82 | 86,583 | 1.73 s | 0.663 s | 0.77 s |
| q63 | 100,317 | 1.82 s | 0.836 s | — |
| q41 | 117,460 | 2.58 s | 1.02 s | — |
| 118,499-line intermediate | 118,499 | 2.29 s | — | 0.97 s |

**The front end is 2.1–2.4× faster than the gcc call it would replace**, whole
analysis included. That is the opposite of what one might expect and it is because
gcc is doing far more: preprocessing 12 system headers, building GIMPLE, writing
an LTO object.

### Per build

- **101 sweep invocations** per `make whim-build`: 87 phases with `Sweep: true`
  plus 14 `{Op: "sweep"}` steps inside phase programs (`internal/build/plan.go`).
  Split by arc: **27** at N≤86 (13 + 14 inner), **74** at N>86.
  (`CLAUDE.md` does not state a number; 93 was the figure in the question and the
  measurement is 101.)
- **Rounds per sweep**, measured by running real segments:

  | segment | sweeps | rounds | rounds/sweep | wall |
  |---|---|---|---|---|
  | phases 13–40 (shared) | 5 | 17 | 3.40 | 208 s |
  | phases 42–62 (shared) | 8 | 25 | 3.13 | 258 s |
  | phases 140–154 (each) | 15 | 19 | 1.27 | 64 s |

  The `each` stages converge fast because each sees one edit; the shared stages see
  a whole stage's accumulation.

- **Rounds per build** ≈ 27 × 3.2 + 74 × 1.27 = **86 + 94 = 180**, so **about 180
  gcc warning calls per build.**
- **gcc time per build** ≈ 86 × 2.3 s (the early arc's 100–127k-line texts) +
  94 × 1.5 s (the late arc's ~77k-line texts) = 198 + 141 = **≈ 340 s**, against
  the recorded 1,220 s build: **about 28 %.**
- The same work in the front end at the measured ratio: **≈ 145 s**. Saving
  **≈ 195 s per build, ≈ 16 %.**

Cross-check against the segments actually timed: 13–40 spent ≈ 17 × 2.6 = 44 s of
208 s on the warning call (21 %); 42–62 ≈ 25 × 2.2 = 55 s of 258 s (21 %);
140–154 ≈ 19 × 1.5 = 28 s of 64 s (**44 %** — in the core arc the warning call is
nearly half the sweep's wall clock, because the edits there are small and the
compile dominates).

**Caveat on the saving**: gcc does not leave. `startSpec` runs a full
`gcc -c -O0` (3.5 s on the product) on every round in the background, and
`deadenums` runs `gcc -O0 -g` plus `readelf` on first need. The 195 s is the
foreground saving only; whether the machine notices depends on whether the
speculative compile was already saturating a core.

### What the parse does NOT cost

`cc.Translate` on `whim-vim.c` reads the twelve system headers the host half
includes. That is already the case for `whimtools parse`, `internal/gen` and
`internal/ccx`, and it is inside the 0.62 s.

---

## 6. Recommendation

**Do it, but not as a swap, and not first.**

The analysis is not the problem. Measured: 474 of 475 names agree exactly across
seven texts and 660,000 lines, the one difference is a documented gcc rule about
`static inline` that is one `IsInline()` test away, the front end is 2.2× cheaper
than the gcc call, and the AST replaces two hand-written extent scanners whose
comments record three separate occasions on which they took the wrong brace. On
its own terms an AST deadsweep is better in every dimension that was measured.

What stops it being a drop-in is one measured fact: **deadsweep is the first tool
in the round, and 1 of the 29 single-edit pre-sweep texts measured does not parse
— phase 35's, where gcc still names 83 things and deadsweep still removes 1,041
lines, and where an AST deadsweep would remove 0.** A tool that needs a parseable
tree cannot hold that position. gcc's willingness to answer about a broken
translation unit is not a convenience here; it is load-bearing, and the docstring
that says otherwise is wrong.

### What would have to be true of the other five tools

An AST deadsweep is coherent in the loop only if, at the point it runs, the text
parses. Three ways, in increasing order of cost:

1. **Move it after funcreach.** Measured: funcreach alone repairs the one failing
   text (269 definitions, 8,313 lines, gcc exit 0 afterwards) and leaves 290 names
   for a deadsweep to find, on which gcc and the AST agree exactly. The other four
   tools are text-based and unaffected.
   **Cost: the product moves.** The round order is load-bearing — `sweep.go`
   records that moving *canon* alone *moved 27 of 32 boundaries* — so
   `make whim-build-check` would no longer return the committed `whim-vim.c`, and
   the committed product, `slim.sha`, `editor/editor.go` and every recorded
   boundary would have to be re-derived and re-verified (`make whim-verify`,
   hours). That is the real price, and it is not a refactor.
2. **Let it decline.** Keep the position; parse, and on failure do nothing, the
   way `FunctionExtent` declines rather than guesses. **Measured cost: 2,772 lines
   in the first round of stage 13–41, which the following rounds would then have
   to pick up** — and after funcreach they largely would, since funcreach repairs
   the text and deadsweep runs again next round. This is the cheapest thing to try
   and it is *measurable before committing to it*: run it, and see whether every
   boundary comes back. If the fixpoint is the same, this costs nothing at all.
   **It is the one that should be tried first.** If the fixpoint is not the same,
   the product moves and we are back at option 1's price.
3. **Make the edits leave parseable text.** Phase 35 leaves two uses of a field it
   deleted. Requiring every edit to leave a compiling tree is a different pipeline
   from the one in `GOALS.md`, where the edit cuts and the sweep collects. Not
   recommended; recorded only to say it was considered.

### A staged plan

1. **Write it as a `ccx` check first, not a tool** — a tenth file in
   `internal/ccx`, in the `Result`/`Finding` partition shape: classify every
   file-scope declarator into *used*, *unused function*, *unused object*, *unused
   prototype*, *exempt (`static inline`, external linkage)*, and refuse on a
   leftover. That is *assert a partition, not a count*, and it is evidence rather
   than a demonstration. Nothing in the pipeline changes.
2. **Run it against gcc over every boundary in `.build/`** — the comparison in §4,
   widened from 7 texts to all 43 tars and every pre-sweep text that can be
   produced. The claim to establish is set equality, per class, with the
   `static inline` exemption written in. If any text disagrees, stop: that is a
   finding about the front end and it belongs in `internal/gen/FINDINGS.md`, not in a sweep.
3. **Add block-scope objects.** §1 measured 12 of them across the survey; the file
   scope walk in §4 does not reach them. `Scope.Children` plus the same counters,
   with the same `HasInitializer` correction, and the same comparison against
   gcc's `unused variable 'X'` form.
4. **Then, and only then, option 2 above**: put the AST behind deadsweep's existing
   interface, declining when the parse fails, and run `make whim-build-check`. That
   one command answers the whole question — either the committed bytes come back,
   and the swap is free, or they do not, and the cost is a re-derived product.
5. **Keep the gcc path.** Not as a fallback in the loop — *there is no tier below
   a phase to fall through to* — but as the control: a `--gcc` flag on the same
   subcommand, so the comparison in step 2 can be re-run on any text at any time.
   A check that cannot fail is not evidence, and the gcc answer is what makes this
   one able to fail.

### What not to expect

Not a gcc-free sweep. `startSpec`'s speculative object and `deadenums`'s DWARF
dump both need gcc and neither has an AST answer — the enumerator values are
*the one thing in the pipeline that is not a function of the bytes alone*.
The honest claim is: one of the sweep's three gcc users can go, it is the one that
runs most often, it costs about 28 % of a build, and replacing it would return
about 16 % — if and only if the fixpoint does not move.
