# Phase 23 — no floating-point library

Three calls are the whole of libm in this editor, and they turn out to be two
different questions.

**`ceil()` and `floor()`** appear once, in the fuzzy matcher, as the two halves
of rounding half away from zero:

```c
(fzy_score < 0) ? (int)ceil(fzy_score * SCORE_SCALE - 0.5)
                : (int)floor(fzy_score * SCORE_SCALE + 0.5)
```

C's double-to-int conversion truncates **toward zero**, which is `ceil` for a
negative value and `floor` for a positive one — so biasing by half in the sign's
own direction and then converting gives the same answer for every input, and the
two arms collapse into one expression.

**`log10()` is not translated, because it cannot be**, and finding that out is
the useful part of this phase. It appears once, as
`max_prec -= (size_t)log10(abs_f)`, and the obvious integer equivalent —
dividing by ten until the value drops below ten — **is a different function**.
Just below a power of ten, `log10()` returns a double that rounds up to the
integer:

```
(size_t)log10(99.999999999999986)  ==  2        counting digits gives 1
```

`tools/nolibm_check.c` swept a million values through both forms and found 79
disagreements, all of that shape. A rounding rewrite that is merely believed is
how an off-by-one reaches a release — and here the check turned a translation
into a removal, which is the better phase.

## Nothing can reach the `%f` branch

So the whole floating-point branch of `vim_vsnprintf()` goes instead. The
premise is checkable and the phase checks it: **there is not one `%f`, `%F`,
`%e`, `%E`, `%g` or `%G` conversion in any format string in the file**, and the
single `vim_snprintf()` call whose format is not a literal takes a local
`char *fmt` that is one of two constants, `"%*ld "` and `"%-*ld "`. Without
`+eval` there is no `printf()` to supply one at run time either.

That takes the conversion case (139 lines), `TYPE_FLOAT` and its three arms,
`infinity_str()` and `typename_float` — and with them `log10`, `isinf` and
`isnan`. `TYPE_FLOAT` is the **last** enumerator, checked before removing it,
because several enums here index a parallel table.

`<math.h>` stays: `INFINITY` is the fuzzy matcher's score sentinel, in thirteen
places. Under musl libm is part of libc, so the link line does not change
either — what changes is that `nm -u` stops naming a floating-point function.

## Where the symbol count moves

**101 → 98**: `ceil`, `floor`, `log10`.

## The delta

**None.**
