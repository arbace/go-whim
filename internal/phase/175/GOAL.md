# Phase 175 — a member's address a call hands back is a local's

A target with no address of a variable -- the Java and the Clojure editors
-- holds a struct member whose address is taken anywhere in a one-element
array, and every read of it pays (`doc/JAVA-IDIOMS.md` 6.2,
`doc/CLOJURE-IDIOMS.md` 5). Phase 174 took `pos_T.lnum` and `.col` out by
hand; this phase is the general rule, `crefactor/xform`'s `MemberOut`: a
call that passes `&s->m` or `&s.m` for its callee to read and write calls a
function written once per callee and member,

```c
    static int
mb_ptr2char_adv__p_extra(winlinevars_T *s0__)
{
    typeof(s0__->p_extra) p_extra0__ = s0__->p_extra;
    int r__ = mb_ptr2char_adv(&p_extra0__);
    s0__->p_extra = p_extra0__;
    return r__;
}
```

which does what the call did through a local. The copy is the call's
exactly when nothing the call runs can reach the member but through the
pointer, and the pointer does not outlive the call, so a site is taken only
when the callee is the core's; no function it can reach names that member of
that struct type (a call through a pointer reaching every function whose
address the core takes, a call to the host every core function the host's
code names) -- or the struct is a local whose address goes nowhere else; the
callee's parameter is only dereferenced, indexed, compared, tested or handed
on to a parameter that is so; the member is reached through what does
nothing; and every address of that member in the core is such a site, since
one kept elsewhere boxes the member whatever is done here.

**Measured:** 4 members come out -- `regprog.regmlen`,
`winlinevars_T.char_attr` and `.p_extra`, `exarg.cmdidx` -- through 4
functions, 5 arguments; about 100 of the Java's 4,874 `[0]` reads. Held, and
reported by the step: 47 sites whose callee keeps the pointer (the option
table's `optvar_*`, which is the option machinery's design); 26 whose member's
address is kept elsewhere too (`cp = &cap->nchar`, `flagsp =
&options[i].flags`); and 18 whose callee can reach code that names the
member -- `getvcol` and kin with `w_virtcol` and the column members,
`get_address` with `exarg_T.cmd` and `.arg` -- through vim's error paths,
which reach the redraw. Those are provably unsafe to copy as far as the
analysis sees, and are left: taking them would rest on a judgement the suite
may not reach.
