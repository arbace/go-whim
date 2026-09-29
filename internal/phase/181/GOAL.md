# Phase 181 — an out-parameter a value, a struct local its members

A target with no address of a variable -- the Java and the Clojure editors
-- boxes a local whose address is taken in a one-element array, and every
read of it pays; one with no struct values makes a local struct an object,
allocated each time its declaration runs. The Haskell editor's printer had
learnt to see past both (`doc/HASKELL-IDIOMS.md`, items 6 and 9: out-
parameters as values in and out, struct locals as bindings), for itself
alone. This phase is the same two analyses on the C, so that every editor
gains: `crefactor/xform`'s `LocalOut` and `StructScalar`.

**`LocalOut`** makes an out-parameter a value in and a value out. A
parameter `T *p` whose callee only reads and writes `*p` and tests `p`
against null, and whose every caller passes `&x` of a local of its own that
nothing else takes the address of, takes x's value and returns its new one:

```c
    static void advance(char_u **pp) { (*pp)++; }            -> char_u *advance(char_u *pp) { (pp)++; return pp; }
    advance(&w);                                             -> (w = advance(w));
    if (!get_list_range(&end, &hisidx1, &hisidx2) || ...)    -> if (!(get_list_range__o = get_list_range(end, hisidx1, hisidx2),
                                                                      end = get_list_range__o.str, hisidx1 = ..., hisidx2 = ...,
                                                                      get_list_range__o.r__) || ...)
```

A callee returning nothing with one such parameter returns the value; any
other returns a struct of its result and the values (`get_list_range__out_T`),
since C returns one thing -- which the Go editor returns as a struct, the Java
and the Clojure as an object, the Haskell as a tuple (its printer writes a
function returning a struct of scalars so, `IO (Bool, Ptr Char_u, Int32,
Int32)`). It is exact because x is reachable only through the parameter --
phase 175's proof, for locals: the callee cannot see x by another way, so
when the value comes back is when C's stores through p would be seen. And
a read of x the rewrite would move -- another operand of the same full
expression, which C leaves unordered against the call (`printf("%d %d",
f(&x), x)`, whose x gcc read before the call and would read after) -- holds
the site, unless a sequence point orders it after the call (the right of
`&&` or `||`, an arm of `?:`, a later comma operand).

**`StructScalar`** makes a local struct of scalars one local per member: a
local -- not static, not a parameter, alone in its declaration -- of a
struct whose members are all named scalars or pointers, used only as `s.m`
(not its address), copied whole from or to an expression that does nothing
(`s = *pp;`, `*pp = s;`, read once per member where C read it once), or
initialized so or by braces. A member nothing reads is not a local at all
-- the Go editor refuses a local only written -- and a store to it keeps
only what its value does (`tmp.string = ml_get(lnum);` is `ml_get(lnum);`).
A copy is a block of one statement a member, so that the sweep prunes what
it can.

**Measured:** 94 out-parameters of 61 functions (7 returning the one value,
54 a struct) and 66 struct locals; the product 75,650 -> 77,762 lines (the
result structs, and a declaration a member). The Java editor's one-element
boxes 440 -> 324 and its `[0]` reads 4,802 -> 4,009; its `pos_T` objects 129
-> 78, and 431 result objects where a callee returns. The Clojure editor's
one-element arrays 637 -> 515, its `(aget x 0)` reads 2,177 -> 1,640. The
Haskell editor's own analyses find 2 out-parameters where they found 97, and
287 struct locals as bindings -- the result structs among them -- where 84.
The heavy case: the Java 1.8-2.0 times the C (1.8-2.1 before), the Clojure
4.4 (4.2-4.5), the Haskell 1.1-1.3 (1.0-1.3), the Go 0.5.

**Verified:** `whim test --java --clojure --haskell` -- the 80 cases as HEAD
does, every editor as the C -- and `--wide` the same, 240 cases; the steps'
own tests (`TestLocalOut`, `TestStructScalar`) compile a program before and
after with gcc and require the same output, and name what each holds back.
