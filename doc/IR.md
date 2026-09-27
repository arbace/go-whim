# IR.md -- where a feature goes, and an intermediate representation

2026-09-27. An assessment, not a plan of record: nothing here is built. It
answers two questions -- where in the chain a feature that matters to the Go,
Java and Clojure editors but not to the C should be introduced (a parallel
`:%s/.../.../g` over a whole buffer on 64 cores is the example), and what an intermediate
representation between the C and the three translations would be. Two
surveys measure what this rests on: `doc/PARALLEL-SUBSTITUTE.md` (how much of a
substitution is matching, and what stands in the way of doing it in
parallel) and `doc/IR-SCHEMA.md` (the representation sketched against what
`crefactor/togo` decides today).

## The chain as it is

```
slim-vim.c --whim (145 phases)--> whim-vim.c --cut--> the core
                                                  |-- togo, Go printer      --> editor/editor.go --> editor.lgo
                                                  |-- togo, Java printer    --> braaam/Editor.java
                                                  `-- togo, Clojure printer --> vijure/src/whim/editor.clj
```

`whim-vim.c` is the one central representation, and it is a stepping stone:
nobody is meant to use the C editor, but it is the **oracle** -- every
translation is held, case by case, to what the C binary does. The three
translations are produced independently and concurrently from it, each
cutting the core for itself.

The fan-out is less separate than the picture: the three printers are about
3,700 (Go), 7,000 (Java) and 5,700 (Clojure) lines, the Clojure printer takes
the Java backend's type and pointer decisions whole (`jgen`), and a lowered
form -- a function as basic blocks, 1,800 lines, printable back as C -- sits
under the Clojure printer alone. There is already one per-target escape
hatch: `Profile.RuntimeBodies`, a function whose body each target writes
natively (used once, for `ga_grow_inner`).

## Should editor.go be the centre?

**For:** Go is the better language to write a feature in (goroutines,
closures, bounds checks, no undefined behaviour), and it is explicit -- every
conversion written, no preprocessor -- so a Go-to-Java translator would be
easier to write than the C-to-Java one was.

**Against, and decisive:**

- **It stops being generated.** `editor.go` is regenerated from the C, so a
  moved upstream flows through by itself. Edit it by hand and every
  regeneration clobbers the features -- unless they are rewritten as
  programs over the Go: a second pipeline.
- **The oracle weakens.** Today each editor is checked against an
  independent implementation, the C binary. With Go as the source, the Java
  and the Clojure would be checked against their own ancestor.
- **The Go is C-shaped.** `Ptr[T]`, `GaData`, an `Editor` of hundreds of
  fields: a Go-to-X translator would have to recover what the C analysis
  already knows, and then also handle goroutines, `defer`, slices and
  interfaces -- a larger surface than the C subset.
- **Clojure is the worst fit** for Go's shared-memory concurrency.

`editor.lgo` works from `editor.go` because it is a change of syntax, not of
meaning; a change of language is not.

## Where a feature goes: two kinds

1. **Behaviour -- what the editor does: in the C, as phases.** That is what
   the pipeline is for (`GOALS.md`, *Adding a phase*). Every target inherits
   a phase, the C stays the oracle, and a phase is a program over upstream,
   so it survives a moved upstream.
2. **Speed that does not change behaviour -- a parallel `:%s`: between the C
   and the targets.** It cannot be written in the C core (no threads, no
   libc) and should not be written three times. Define it in C as a
   sequential **primitive** whose C body *is* its meaning, and let each
   backend give it a native, parallel body -- `RuntimeBodies` is that
   mechanism for one function today. The suites then prove the parallel
   versions equal to the sequential C.

Writing a feature separately in each derivation is right only for
presentation: names, data tables, JIT flags -- what the idiom surveys did.

## The parallel :%s, concretely

It is a range that gives a substitution work to share: `:%s` (every line),
`1,$`, a visual range, `:g/.../s//`. A bare `:s` substitutes in one line and
gains nothing.

It is a refactor of the C first, then one primitive:

1. **A phase: the regex engine's state is a parameter.** Today it is global
   -- `rex` (307 uses) and the `reg_*` flags -- so no two matches can run at
   once. As a struct passed in, the C reads better too.
2. **A phase: `ex_substitute` becomes snapshot, match, apply.** Copy the
   range's lines first (`ml_get` writes its line cache on every read, so it
   is not safe to call concurrently); find the matches with a new function,
   `match_lines(state, lines, n, results)`, pure given its inputs; then apply
   them in order, keeping undo, marks and messages exactly as they are.
3. **Per target: `match_lines`'s parallel body** -- goroutines over chunks in
   Go, a fork-join pool in Java, `pmap` or futures in Clojure.

Only the matching runs in parallel, so the speed-up is bounded by the share
of the time that is matching (Amdahl). A replacement with `\=` expressions,
or a pattern whose matching has side effects, stays on the sequential path,
which the phase detects. `PARALLEL-SUBSTITUTE.md` measures that share.

## An intermediate representation

One exists in pieces already: the analysed C tree with the backends' shared
decisions, and the lowered form. A deliberate IR makes them explicit and
serialisable:

- **Typed:** exact widths and signedness, every conversion written out, C's
  evaluation order fixed (what `crefactor/ccx` works out).
- **Memory resolved:** each object's class decided once -- scalar, struct
  object, array with an offset, a boxed cell -- and places as explicit terms.
- **The instance resolved:** the editor's state against locals, as togo's
  instance pass decides.
- **Structured control flow:** a region tree -- sequence, if, loop, switch,
  a labelled block that can be left -- with the tuple-valued joins and loops
  the Clojure nesting rules built; no `goto`, and a state machine only where
  nesting fails.
- **Primitives as nodes:** the host's calls, and the parallel operations.

**Not a pure AST**, which keeps C's ambiguities and makes every backend
re-derive the answers (today's duplication). **Not SSA**, which destroys the
names and the structure all three targets print. **Data** -- S-expressions
or JSON -- so that a backend is only a printer.

**The strongest part: make it executable.** An interpreter of the IR, run on
the suites against the C, proves the IR correct independently of the three
printers. Then a feature that matters to the targets and not to the C is an
**IR-to-IR pass** -- the parallel map, say -- verified once on the
interpreter and inherited by Go, Java and Clojure, without touching the C.

```
slim-vim.c --phases--> whim-vim.c --lift--> IR --passes--> IR --printers--> Go, Java, Clojure
               behaviour              (interpreter: the suites)   speed         presentation
```

The cost is real: in effect a new middle for togo, the printers rewritten
onto it, and an interpreter. Grow it by extracting the lowered form and the
shared decisions one piece at a time, the suites green throughout.

## Recommendation

- **The C stays the centre** for behaviour.
- **The parallel `:%s` now,** as two C phases and a primitive with per-target
  bodies: the cheapest route to the 64 cores, and fully verifiable.
- **The IR is the long-term architecture:** start it when there are several
  speed features to share, not for one.
- **`editor.go` is not the centre.**
