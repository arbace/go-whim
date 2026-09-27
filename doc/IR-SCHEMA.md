# IR-SCHEMA.md -- an intermediate representation, sketched against togo

**Status (2026-09-27): a survey; nothing in it is built.** It supports
`doc/IR.md`, which proposes a representation between the C core and the three
printers: typed, memory resolved, the instance resolved, control flow as a
region tree, primitives as nodes, data rather than code, and executable. This
document measures what `crefactor/togo` decides today and where, sketches the
schema, writes three real functions of the core in it by hand, and prices the
interpreter and the migration. Every count below comes from a command named
beside it; the throwaway instruments are under the worktree's `.tmp/` and
listed at the end. No tracked file changed but this one.

It covers the tree at `16bb5d2`: `src/whim-vim.c`, whose core `go tool whim
cut` prints as 73,474 lines (`editor.c` below), and the tracked translations,
`editor/editor.go` (57,961 lines), `braaam/Editor.java` (63,610) and
`vijure/src/whim/editor.clj` (64,083). `whim skel editor.c D -java F` and
`-clj F` were run and give the tracked Java and Clojure back byte for byte
(`cmp`), so the counts are of the code that writes what is tracked.

**The short answer.** togo is 18,027 lines of Go outside its tests: 1,002
shared (the pointer analysis, the profile, the driver), then 4,288 for
the Go printer, 5,261 for the Java, and 7,476 for the Clojure (5,711, and
the lowered form's 1,765, which only it uses). What is decided once is the
**pointer analysis** (which class of pointer walks) and, for the Java and
the Clojure only, **the Java backend's representation** (`jgen`: types,
boxes, struct classes), which the Clojure takes whole. What is decided
**three times** is nearly everything a statement or an expression needs:
30 methods have the same name and the same job on all three printers'
receivers (`conv`, `compare`, `lval`, `addr`, `assign`, `call`, `alloc`,
`fill` ...), about 1,000 lines each; 18 more are Java's and Clojure's
twice over (about 470 lines each); C's usual arithmetic conversions
are written twice (`usual` on Go type names, `usualK` on Java kinds);
and side effects are taken out of an expression three times (about 1,200
lines together). The three derivations have already drifted once: **plain
`char` is unsigned in the Go and signed in the Java and the Clojure**
(gcc's, on x86-64, is signed) -- measured with a two-line probe below,
at 44 sites of the Java. Seven concerns are derived per printer -- types,
conversions, places, effects, the functions of memory, calls, initial
values -- about 8,100 lines today, and control 2,600 more. An IR would
fold them into one lift and three printers' rules: about 7,300 lines new
(the interpreter's 2,500 among them) against 5,100 deleted, so togo would
end about its size without the interpreter and 2,200 lines larger with it
-- worth it for what it enables and prevents rather than what it deletes.
**Verdict: later, in slices** -- the first slice when a fourth target or
a second speed feature is scheduled; the `char` drift now, by itself.

## 1. What togo decides today, and where

Measured by listing every function of `crefactor/togo/*.go` (not the tests)
with its receiver and its length (`go/parser`, 610 functions), then summing
by concern with a hand-made table of which function serves which
(`.tmp/buckets.txt`). The lengths are the functions' own lines, so the sums
are a little under the files'. A concern's line is **SHARED** when one piece
of code decides it for several printers, **J+C** when the Java decides and the
Clojure takes it, and **DUP** when each printer derives it again.

| concern | Go (`body*.go`, `gen.go`) | Java (`java*.go`) | Clojure (`clj*.go`, `lower.go`) | | lines G / J / C |
|---|---|---|---|---|---:|
| which pointer walks (cursor, forward, punned) | `analyze.go`: the union-find, `gen.cursor`, `gen.forward` | reads it for struct pointers only (`jgen.jt`) | through `jgen` | SHARED | 636 + 41 |
| a C type's target type | `gen.goType`, `scalar`, `structBody`, `canon` | `jgen.jt`, `structClass`, `fnIfaceK`, `scalarKind`, `ptrClass` | `jgen.jt`; its own `cljHint`, deftypes (`cgen.structType`) | DUP (G vs J+C) | 378 / 378 / 233 |
| a union | a struct, every member a field (`gen.go:275`) | the same class (`java.go:464`) | the same deftype | DUP, one rule thrice | in the above |
| boxed local, global, member (address taken) | none: Go has `&x` | `jgen.boxes`, `isBoxed`, `boxedField` | `jgen`'s | J+C | 41 |
| conversions, promotions, unsigned ops, truth | `conv` (86), `usual`, `promoted`, `compare`, `cast` | `conv`, `convK`, `numconv`, `usualK`, `unmasked`; `java_const.go` (525) | `convK`, `convV`, `wrapK`, `arith`; `usualK`, `truncK` | DUP | 356 / 933 / 386 |
| places: lvalues, `&`, members, elements, struct copy and `==` | `lval`, `addr`, `path`, `postfix`, `incdec`, `assign` | the same names, `ptrOver`, `copyStmt`, `eqStmt`, `eqClosure` | the same names, `varLval`, `slotLval`, `copyForm`, `eqForm` | DUP | 484 / 631 / 753 |
| side effects out of an expression; evaluation order | `hasEffect`, `exprStmt`, `newTemp`, `ternary`, `logical` | `hoisted`, `exprStmt`, `discard`, `merge` | `lower.go`'s `effect`, `val`, `operands`, `snapshot` (~430), `hasSub` | DUP | 275 / 314 / ~640 |
| control: loops, switch, goto, fall-through, reachability | native `goto` (49) and `fallthrough` (25); `loopOnce`, `tailBreaks`, `terminate.go` (Go's rule) | labelled blocks for `goto`, `jcomplete` (JLS 14.22), `liveBreak` | `lower.go`'s blocks (~290) and liveness (~150); `clj_shape.go` joins, loops, state machines, the split | DUP | 670 / 593 / ~1,380 |
| allocators, functions of bytes, the growarray | `alloc`, `fill`, `sizeCount`, `gadata` | the same, `bytesCall`, `memmoveElems`, `gaHelpers` | the same | DUP (the profile SHARED) | 142 / 275 / 273 |
| calls, function values, varargs | `call` (131), `adapter`; method values by the instance pass | `call`, `callPtr`, `fnValue`, `vararg`, interfaces per signature | `call`, `callPtr`, `fnValue`, `vararg` | DUP | 175 / 239 / 211 |
| declarations, initial values, tables | `declaration`, `initValue`, `initObject`, `initializers` | `declaration`, `initInto`, `fields` (`initGlobalsN`) | `initInto`, `tableData`, `filler` | DUP | 347 / 302 / 423 |
| the instance: state against locals | `instance.go`, a rewrite of the printed Go (fixpoint: which function reaches state) | none needed: every function a method | `allocSlots`, `slotPlace`, `editorText` | DUP | 355 / -- / 179 |
| host calls (declared, not defined) | the `Host`, by hand | abstract methods (`jgen.abstract`) | `whim.cljhost`, by name | DUP, small | ~10 each |
| runtime bodies (`Profile.RuntimeBodies`) | `RuntimeBody.Body` | `.Java` | `.Clj` | the NAME shared, a body each | -- |
| names | `goName` | `jName`, `memberName` | `cljName`, `kebab`, a question's `?` (`predicates`, 92: an analysis) | DUP, and should stay per printer | 18 / 28 / 262 |
| tidying the text | -- | `java_tidy.go` | `clj_tidy.go` | per printer, and should stay | -- / 366 / 541 |

**The mirrored methods.** 30 method names exist on all three printers'
receivers (`fnEmit`, `jfn`, `cfn`): 1,028 lines in the Go, 1,057 in the Java,
950 in the Clojure. 18 more exist on the Java's and the Clojure's alone
(`callPtr`, `fnValue`, `vararg`, `bytesCall`, `memmoveElems`, `initInto`,
`gaMember`, `structRef` ...): 484 and 456 lines. The Clojure printer calls into
the Java's in 16 places (`c.j.isBoxed`, `f.c.j.boxedField`, `jt`,
`structName`, `sigWhy`, `needEq`, `ifaceByName`) and uses its free helpers
(`scalarKind`, `promote`, `usualK`, `truncK`, `members`, `isAggr`, `elemJ`,
`ptrOfArray`) -- that is the one place where a decision is already SHARED
beyond the analysis, and it is what the first slice of an IR would be (§4).

**The lowered form is the Clojure's alone.** `lower.go` turns a C function
into basic blocks whose steps are single effects and whose expressions are
the C's own nodes read through a substitution map; `lower_c.go` prints it
back as C, which is how it is tested (`whim skel editor.c D -lowerc L.c`: 82,385
lines, run for this survey in 1.9 s). It has fixed evaluation order (a right
side that calls becomes a temporary first, gcc's order for `g += bump()`),
which the Go and the Java each fix for themselves.

**A drift the duplication allowed.** `gen.go`'s `scalar` maps `char` and
`unsigned char` alike to `byte` (`internal/gen/CONVENTIONS.md`, line 72);
`java.go`'s `scalarKind` asks `cc.IsSignedInteger`, which says `char` is
signed, as gcc on x86-64 does. The probe `int widen(const char *s) { return
*s; }`, translated by `whim skel -editor` and `-java`, gives `return
int32(s.Get())` (zero-extended: 233 for the byte 0xE9) and `return (int)
s.get();` (sign-extended: -23, the C's). `Editor.java` has 44 such
sign-extending `(int) p.get()` reads and 3 `(long)`, in 16 functions
(`musl_atoi`, `musl_atol`, `musl_strtol`, `tgoto`, `cstrncmp` ...); those
looked at compare with an ASCII constant or with each other for equality,
where the two agree, which is why no suite sees it. Whether any of the 44 is
observable was not established. In an IR, `conv i8 i32` is one node, printed
three ways; the question would not arise twice.

## 2. The schema

The IR is **the C's own structure with every question answered**: the types
exact, every conversion a node, the memory class of every object decided,
effects taken out of expressions in C's order, and control a region tree.
It is S-expressions (JSON is the same tree); a symbol in a value position is
the load of that local, and `(* e)` in a value position the load through `e`.

```
; types -- representation, decided once
T   ::= i8 | i16 | i32 | i64 | u8 | u16 | u32 | u64 | bool | void
      | (ptr E)       ; an address that never walks (the analysis's non-cursor)
      | (cur E)       ; a cursor: storage and an offset (BytePtr, Ptr[T], Ptr<S>)
      | (obj S)       ; a struct or union object, held by its container, copied by (copy)
      | (arr E n)     ; an array object
      | (fn G)        ; a function value of signature G
      | any           ; a void *, typed where it is used (the growarray's storage)
E   ::= T
S   ::= a struct's name          G ::= a signature's name
; an object's storage: plain, or :cell -- its address is taken
; (Go: a variable and &x; Java: a one-element array; Clojure: the same)

; places (lvalues) -- explicit terms
P   ::= x | (st x)               ; a local or parameter; a slot of the editor's state
      | (* e) | (@ e i)          ; through a pointer; element i of a cursor or array
      | (. P m) | (-> e m)       ; a member of a struct place; through a struct pointer

; expressions -- nothing to do but read and call; calls in C's order
e   ::= P | (k T v [:char "c"]) | (e NAME) | (str "...") | (nil T)
      | (& P)                    ; its type says (ptr E) or (cur E)
      | (+ T a b) | (- T a b) | (* T a b) | (/ T a b) | (% T a b)
      | (<< T a b) | (>> T a b) | (band T a b) | (bor T a b) | (bxor T a b)
      | (== T a b) | (!= T a b) | (< T a b) | (<= T a b) | (> T a b) | (>= T a b)
      | (neg T a) | (bnot T a) | (not a) | (and a b) | (or a b) | (? c a b)
      | (conv T1 T2 a)           ; every conversion; T of an op is after C's usual conversions
      | (p+ e k) | (p- a b) | (p= a b) | (nil? e)
      | (call f e...) | (calli e e...) | (host h e...) | (prim p e...)
      | (fnv f) | (va T e)       ; a function as a value; a variadic argument, promoted

; statements and regions
s   ::= (let x T [e] [:cell])    ; a local where C declares it
      | (set P e) | (op= OP T P e) | (copy S P e) | (do e)
      | (seq s...)
      | (if e s [s] [:out (x...)])
      | (loop L [:test e] [:step s] s [:carry (x...)] [:out (x...)])
      | (switch T e (case (v...) s [:fall])... [(default s)])
      | (block L s)              ; a region that can be left: C's forward goto
      | (leave L) | (next L)     ; to after L; to L's step
      | (return [e])
      | (machine (x...) (state k s)...) | (goto k)   ; IR-V only, see below

; functions and the module
F   ::= (fn name ((x T [:cell])...) T :reaches state|none [:question] body)
M   ::= (module
          (struct S [:union] (m T [:cell])...)...
          (sig G (T...) T [:va])...
          (enum NAME T v [:char "c"])...
          (state x T [:cell] [:init D])...  ; the file-scope objects, hoisted statics
          (table x E (row v...)...)...        ; initial values as data
          (host h (T...) T)...                ; declared, not defined: the Host's 17
          (prim p F)...                       ; a runtime body: its C body is its meaning
          F...)
```

Four decisions carry the design.

- **Facts, and the printer's coarsening.** `(ptr E)` against `(cur E)` is the
  analysis's fact; Java prints both as `BytePtr` when `E` is a scalar (it has
  no other address of one) and only a struct pointer differently, as `jgen.jt`
  does today. The IR carries the finest distinction any printer uses; a
  printer's type is a pure function of the IR type. The same for `:cell`:
  Go prints a cell as a plain variable and `(& x)` as `&x`, Java and Clojure
  as a one-element array.
- **Annotations are computed by passes, not by the lift.** `:out` and
  `:carry` (the variables a region changes that are read after it, or that a
  loop carries) are liveness on the tree -- today `lower.go`'s `live`,
  `usesDefs` and `clj_shape.go`'s joins, about 300 lines. `:reaches` (the
  instance) is the fixpoint `instance.go` computes on the printed Go today.
  `:question` (a `bool` result and no store) is `clj_names.go`'s
  `predicates`. Go and Java ignore `:out` and `:carry`.
- **Two levels.** The lift gives **IR-S**, with `leave`, `next` and `return`
  anywhere: Go and Java print it directly -- every one of the 49 C gotos is a
  forward leave of an enclosing block (`CLAUDE.md`, phase 173), which is how
  the Java already writes them. A pass, **values**, gives **IR-V** for
  targets with no jump: every region yields its `:out` as a value, and where
  nesting fails, a `machine`. Measured today (`whim skel -clj`, run for this
  survey): of 1,699 functions written, 1,554 nest and 144 are state machines
  (91 a join entered from outside or left elsewhere, 45 a loop's exit with
  another way in, 6 irreducible loops, 2 a return that must reach a join).
  That pass is `clj_shape.go` (655 lines) and the split (133) moved, and it is
  what `doc/HASKELL.md` and `doc/RUST.md` would share.
- **Presentation rides along.** `(k i32 45 :char "-")` and `(e BS_NOSTOP)`
  keep what the C spells, because the idiom surveys found that the spelling
  is most of what makes a translation readable (`JAVA-IDIOMS.md`: 961 case
  labels). Names, parentheses, where a local is declared and the tidying stay
  the printers'.

## 3. Three functions, by hand

### `skipwhite` -- a loop on a cursor

```c
static char_u *skipwhite(char_u *q)
{ char_u *p = q; while (((*p) == ' ' || (*p) == '\t')) { ++p; } return p; }
```

```
(fn skipwhite ((q (cur u8))) (cur u8) :reaches none
  (seq (let p (cur u8) q)
       (loop L1 :test (or (== i32 (conv u8 i32 (* p)) (k i32 32 :char " "))
                          (== i32 (conv u8 i32 (* p)) (k i32 9 :char "\t")))
         (set p (p+ p 1))
         :carry (p))
       (return p)))
```

The printers' rules: `(conv u8 i32 (* p))` is Go `int32(p.Get())`, Java
`p.get() & 0xff` (dropped by today's Java when compared with a constant that
fits -- a printer rule), Clojure `(bit-and (.get p) 0xff)`; `(p+ p 1)` is
`p.Add(1)`, `p.add(1)`, `(.add p 1)`; `:reaches none` makes the Go a
function, not a method; `:carry (p)` makes the Clojure `(loop [p p] ...)`.
Today's text, which those rules give back:

```go
func skipwhite(q Ptr[byte]) Ptr[byte] {
	p := q
	for (int32(p.Get()) == (' ')) || (int32(p.Get()) == 9) {
		p = p.Add(1)
	}
	return p
}
```
```clojure
(defn skipwhite ^BytePtr [^Editor ed ^BytePtr q]
  (let [^BytePtr p q]
    (loop [^BytePtr p p]
      (if (or (== (bit-and (.get p) 0xff) 32) (== (bit-and (.get p) 0xff) 9))
        (let [^BytePtr p (.add p 1)]
          (recur p))
        p))))
```

(`del_chars` is the same shape with a step: `(loop L :test (and (< i64 i
count) (!= i32 (conv u8 i32 (* p)) (e NUL))) :step (set i (+ i64 i 1)) ...
:carry (bytes i p))`, which the Go prints as `for ; cond; i++`, the Java as
`for (i = 0L; ...; i++)` and the Clojure as `(loop [bytes_ i p] ...)`.)

### `musl_atoi` -- a join of two variables

```c
static int musl_atoi(const char *s)
{
    int n = 0; int neg = 0;
    while (musl_isspace(*s)) { ++s; }
    if (*s == '-') { neg = 1; ++s; } else if (*s == '+') { ++s; }
    while (musl_isdigit(*s)) { n = 10 * n - (*s++ - '0'); }
    return neg ? n : -n;
}
```

```
(fn musl_atoi ((s (cur i8))) i32 :reaches none
  (seq
    (let n i32 (k i32 0))
    (let neg i32 (k i32 0))
    (loop L1 :test (call musl_isspace (conv i8 i32 (* s)))
      (set s (p+ s 1))
      :carry (s))
    (if (== i32 (conv i8 i32 (* s)) (k i32 45 :char "-"))
        (seq (set neg (k i32 1)) (set s (p+ s 1)))
        (if (== i32 (conv i8 i32 (* s)) (k i32 43 :char "+"))
            (set s (p+ s 1)))
        :out (s neg))
    (loop L2 :test (call musl_isdigit (conv i8 i32 (* s)))
      (seq (let t1 (cur i8) s)                 ; *s++: the old value, once
           (set s (p+ s 1))
           (set n (- i32 (* i32 (k i32 10) n)
                         (- i32 (conv i8 i32 (* t1)) (k i32 48 :char "0")))))
      :carry (s n))
    (return (? (!= i32 neg (k i32 0)) n (neg i32 n)))))
```

`*s++` is taken apart once, by the lift; today the Go (`t1` inside the loop),
the Java (`t1` at the method's top, since it is declared without a value in a
loop) and the lowering (`t1 = s; s++;` in block `b9` of `lowered.c`) each do
it. `(- i32 ...)` is Clojure's `(i32 (- ...))`, the wrap to 32 bits, and Go's
and Java's plain `-`. `(? c a b)` is Java's `?:`, the Clojure's `if`, and in
the Go a temporary and an `if`, since Go has no conditional expression.
`:out (s neg)` is what makes the Clojure a tuple, `[s neg]` at each arm and
taken apart after -- the rule `clj_shape.go` applies to its blocks today.
And `(conv i8 i32 (* s))` is where the Go would change: `int32(int8(s.Get()))`
where it writes `int32(s.Get())` (§1). Today:

```go
	for musl_isdigit(int32(s.Get())) {
		var t1 Ptr[byte] = s
		s = s.Add(1)
		n = (10 * n) - (int32(t1.Get()) - '0')
	}
	var t2 int32
	if neg != 0 {
		t2 = n
	} else {
		t2 = -n
	}
	return t2
```
```clojure
        (let [j__1 (if (== (long (.get s)) 45)
                     (let [neg 1
                           ^BytePtr s (.add s 1)]
                       [s neg])
                     (if (== (long (.get s)) 43)
                       (let [^BytePtr s (.add s 1)]
                         [s neg])
                       [s neg]))
              ^BytePtr s (nth j__1 0)
              neg (long (nth j__1 1))]
          (loop [^BytePtr s s
                 n n]
            (if (musl-isdigit? ed (long (.get s)))
              (let [^BytePtr t1 s
                    ^BytePtr s (.add s 1)
                    n (i32 (- (i32 (* 10 n)) (i32 (- (long (.get t1)) 48))))]
                (recur s n))
              (if (zero? neg) (i32 (- n)) n))))
```

### `can_bs` -- a switch, the editor's state, and a cell

```c
static bool can_bs(int what)
{
    switch (*p_bs) { case '3': return TRUE; case '2': return (what != BS_NOSTOP);
                     case '1': return (what != BS_START); case '0': return FALSE; }
    return vim_strchr(p_bs, what) != nullptr;
}
```

```
(state p_bs (cur u8) :cell)        ; the option table holds &p_bs
(fn can_bs ((what i32)) bool :reaches state
  (seq
    (switch i32 (conv u8 i32 (* (st p_bs)))
      (case ((k i32 51 :char "3")) (return (k bool 1)))
      (case ((k i32 50 :char "2")) (return (!= i32 what (e BS_NOSTOP))))
      (case ((k i32 49 :char "1")) (return (!= i32 what (e BS_START))))
      (case ((k i32 48 :char "0")) (return (k bool 0))))
    (return (not (nil? (call vim_strchr (st p_bs) what))))))
```

`(st p_bs)` with `:cell` is the Go's `ed.p_bs` (and the option table's
`&ed.p_bs`), the Java's `p_bs[0]` (and `new Ptr<BytePtr>(p_bs, 0)`), the
Clojure's `(aget (g ed p-bs) 0)`; `:reaches state` makes the Go a method.
The `switch` has no `:fall`, so the Clojure's `case` takes it whole; a case
that falls through would be `:fall`, printed as Go's `fallthrough`, as
nothing in Java, and in the Clojure through IR-V. Today:

```go
func (ed *Editor) can_bs(what int32) bool {
	switch ed.p_bs.Get() {
	case '3':
		return true
	...
	return !ed.vim_strchr(ed.p_bs, what).Nil()
```
```java
    boolean can_bs(int what) {
        switch (p_bs[0].get() & 0xff) {
            case '3':
                return true;
            ...
        return vim_strchr(p_bs[0], what) != null;
```
```clojure
(defn can-bs [^Editor ed ^long what]
  (case (bit-and (.get ^BytePtr (aget (g ed p-bs) 0)) 0xff)
    51
      true
    ...
    (some? (vim-strchr ed (aget (g ed p-bs) 0) what))))
```

The Go drops the widening where a switch on a `byte` against untyped
constants means the same; that is a printer's peephole, and stays one.

## 4. The interpreter

**The model** is the Java runtime's, in Go: a cursor is a slice and an
offset -- `editor/crt.go`'s own `Ptr[T]` (296 lines with the rest of the Go
runtime) serves as it is, so `vim_snprintf` (`editor/format.go`, 1,184 lines)
and the terminal host (`editor/term`) are reused unchanged; a struct object is
a record of slots, one per member, a union the same (every member its own
slot, as all three printers do); a cell is a one-element slot; the editor's
state a record of the module's `state` slots; a function value a closure over
the interpreter. The host is the Go editor's `Host` interface (17 functions,
`editor/host.go`, 202 lines), called by `(host h ...)`. A `prim` runs its C
body -- the sequential meaning a parallel body is proved against.

**Its size**, by comparison: evaluating a node is about three times printing
it (a value, its type's wrap, and the place), and `lower_c.go` prints the
lowered form of the whole core back as C in 422 lines; the Java runtime
`braaam/rt` is 735 lines without its self-test. So: expressions, places and
regions about 1,200; the memory model about 500 (on `crt.go`); loading a
module, its tables (4,121 rows, `JAVA-IDIOMS.md`) and its initial values
about 300; the S-expression reader about 250; the host bridge about 200 --
**about 2,500 lines**, an estimate. Its speed was not measured: a
tree-walker is commonly 30 to 100 times slower than the compiled Go, which
for a key session of the suite would be tenths of a second to a second.

**Its check** is the others': `whim test --ir`, the 45 cases and `--wide`'s
240, the interpreter's screens required to equal the C candidate's, with a
control of its own -- the string `" INSERT"` changed in the IR, which must
move cases as it moves the Go's 41 of 45. That proves the IR the editor,
independently of the three printers, which is what makes an IR-to-IR pass
(the parallel map of `IR.md`, the `values` pass) checkable once.

## 5. The migration path

Each step leaves the three tracked translations **byte-identical**
(`whim gen --check`) unless it says otherwise, with `whim test --java
--clojure` and `--wide` green; a step whose output must move names every
changed line. Sizes are estimates, from the counts of §1.

| step | what | new | deleted | gate | risk |
|---|---|---:|---:|---|---|
| 0 | **The `char` drift**, alone: the Go printer widens a plain `char` with its sign (`int32(int8(p.Get()))`) | ~20 | -- | the 44-odd sites of the Go that move, named; the suites | low |
| 1 | **The module slice**: IR types and nodes (`crefactor/ir`), the S-expression form, and the lift of the module -- `jgen`'s decisions (`jt`, `boxes`, struct classes, signatures) and the analysis's facts, as `(struct ...)`, `(state ...)`, `(sig ...)`, `(host ...)`. The Java and the Clojure read their types from it | ~1,300 | ~400 moved out of `java.go` | byte-identical Java and Clojure | low |
| 2 | **The function lift**: regions from the C's statements (the Java's labelled-block reading of goto), expressions with every conversion and place explicit, effects taken out by `lower.go`'s rules; an IR-to-C printer replaces `lower_c.go`, checked as it is (the whole core through gcc, then `whim test` on it) | ~2,500 | `lower_c.go` 422 | the IR-to-C editor answers every case | medium |
| 3 | **The interpreter**, and `whim test --ir` with its control | ~2,500 | -- | 45 and 240 cases | medium: its speed |
| 4 | **The Clojure onto the IR**: the passes (`:out`/`:carry`, `values`) from `clj_shape.go` and `lower.go`'s liveness; the blocks built from regions, not from C; `clj_expr.go` on IR nodes | ~600 | `lower.go` ~1,000; `clj_expr.go` ~800 | byte-identical Clojure | medium-high: the tuple shapes must come back exactly |
| 5 | **The Java onto the IR** | ~200 | `java_expr.go` ~900; `java_stmt.go`'s goto and hoisting ~250 | byte-identical Java | medium |
| 6 | **The Go onto the IR**: `:reaches` replaces the fixpoint of `instance.go` | ~200 | `body_expr.go` ~700, `body_stmt.go` ~300, `instance.go` 355 | byte-identical Go but for step 0 | medium: the Go is the idiom-tuned one |
| 7 | **Passes**: the parallel map, and whatever `IR.md` puts between the lift and the printers | per pass | -- | the interpreter, then the three | per pass |

**The order** puts the Clojure first because it already reads a lowered form
and the Java's decisions -- its move is the smallest change of kind -- and the
Go last, because it is the lead translation and the one the idiom work
tuned. Step 1 is the natural first slice: what is SHARED between the Java and
the Clojure today becomes data, and nothing printed moves. The lowered form
is not the first slice: its expressions are C nodes read through a map, and
its blocks lose the structure the Go and the Java print; it becomes the
target of the `values` pass instead.

**In sum**: about 7,300 lines new (IR 1,300, lift 2,500, interpreter 2,500,
passes and printer glue about 1,000) against about 5,100 deleted or moved,
out of 18,027: togo ends at about 20,200 lines, 17,700 without the
interpreter. The printers end at about 3,100 (Go), 3,900 (Java, of which
`java_const.go` and `java_tidy.go`, 891, are presentation) and 4,500
(Clojure, once its shape pass, about 800, has moved to the IR as `values`).

## 6. Verdict

**Later, in slices; step 0 now.**

- **What it removes is real but not large.** Seven concerns are derived per
  printer, about 8,100 lines, of which about 3,000 are the same 30 methods
  written three times. The IR folds them into one lift and three printers'
  rules: about 5,100 lines go, 4,800 come, and the interpreter's 2,500
  besides.
- **What it prevents is the stronger argument.** The `char` drift is the
  kind of thing three derivations produce and one does not, and it hides
  where the suites cannot see. The IR's `conv` nodes and one type mapping are
  the fix for the class; step 0 is the fix for the instance, and needs no IR.
- **What it enables is the reason to build it**: an IR-to-IR pass checked
  once on the interpreter and inherited by three targets, and a fourth
  target (`HASKELL.md`, `RUST.md`) that is a printer of IR-V, not another
  1,000-1,500 lines of the mirrored methods. The parallel `:s` alone does not
  need it: `IR.md`'s route -- two C phases and a primitive with a body per
  target, `Profile.RuntimeBodies` -- is cheaper for one feature.
- **So:** start step 1 when a fourth target or a second speed feature is
  scheduled; steps 2-3 together, since the lift is proved by the interpreter
  and the IR-to-C printer; the printers one at a time, each byte-identical.
  Never as a rewrite: every step has a gate the suites and `whim gen --check`
  already provide.

## Instruments

Under the worktree's `.tmp/`, not tracked: `fl/`, a `go/parser` lister of
every function of `crefactor/togo/*.go` but the tests, with its receiver and
length (`funcs.txt`), and `buckets.txt`, which function serves which concern, summed
by `awk`; `m_fnEmit`, `m_jfn`, `m_cfn`, each receiver's method names, which
`comm` compared; `editor.c` (`go tool whim cut src/whim-vim.c`), `cfn.sh`,
which prints a function of it, and `lowered.c` (`whim skel -lowerc`);
`skel/Editor.java` and `skel/editor.clj` (`whim skel -java`, `-clj`,
`cmp`-equal to the tracked files; their logs gave the coverage); and
`ch/ch.c`, the probe of `char`'s signedness through `-editor` and `-java`.
