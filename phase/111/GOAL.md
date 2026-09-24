# Phase 111 — the scalar clock

`phase/111/edit.go` and `phase/111/check.go`, `stage 111`, `package boundary`.
The core's whole use of time is *stamp now, then ask how many milliseconds have
passed*. That is four places — `do_sleep`'s `done < msec` loop, `vim_beep`'s 500 ms
rate limit, `handle_osc`'s `>= p_ost` timeout and `inchar_loop`'s deadline — and **not
one of them reads a field, prints a reading or compares two stamps**. So the core never
needed the *layout* of a clock, only a scalar, and this phase gives it one:

```c
    long musl_now_ms(void)          replaces    void musl_gettimeofday(long *, long *)
    X = musl_now_ms();                          a stamp
    musl_now_ms() - X                           a reading
```

Three things phase 109 created go together and are at **0** afterwards: `elapsed_T`, its
tagless mirror of `struct timeval`, whose layout that phase had to `static_assert`
equal; `elapsed()`, whose whole body was one clock read and one subtraction; and
`musl_gettimeofday`, **whose out-parameter pair existed only because a struct could not
cross the boundary**. 80,232 → 80,222 lines.

## What it earns, and the check is careful not to claim more

Every one of the thirteen core → host signatures takes **scalars and byte buffers
only** — `void`, `int`, `long`, `usize`, `char *` and `int *`, with `vim_snprintf`'s
`...` held to printf arguments by `format(printf, 3, 4)` on a `-Wall -Wextra` clean
build. **That was already true at q110**, phase 109 having chosen `long *, long *`
precisely so that `struct timeval` would not cross, and the check computes the property
on the *input* as well as the output for exactly that reason. What is new is that the
**workaround** is gone: no host call's shape is decided any more by a type the core
cannot name.

`nm -u` is the same 17 names and **`gettimeofday` is still one of them**, stated as an
equality because a reader expects a clock phase to free a clock symbol. It cannot: the
host still calls it to implement `musl_now_ms`, and a symbol leaves when its last
*caller* leaves the **file**, which is the split and not this phase.

## Two decisions, both measured

**The origin is the whole second of the first call, not 1970.** Every core use is a
difference, so the origin is free — and on a target where `long` is 32 bits `tv_sec *
1000` is signed overflow on the **first** call and every call after it, measured with
`-fsanitize=signed-integer-overflow` as *`1789797927 * 1000 cannot be represented in
type 'int'`*, where `(tv_sec - base) * 1000` is exact for 2^31 ms, **24.86 days** of
uptime. The base is taken **lazily** rather than in `musl_host_init()`, because an
ordering dependency between two host functions is what a host rewrite breaks silently.
The origin being a whole second is what makes it behaviourally invisible, and the
`epoch` variant's probes and full recording are the product's.

**Precision is not lost and the rounding point moves.** `elapsed()` subtracted and
*then* divided; `musl_now_ms` divides at each reading and the caller subtracts.
Microseconds were already discarded either way — but the two roundings are not the same
function, and the check compiles a probe and runs it over **20,000,000 random pairs**:
the difference is exactly ±1 ms and never more, 24.95 % one lower, 50.08 % equal,
24.97 % one higher, with neither formula closer to the truth. The control is the `ceil`
variant, which rounds every reading **up** — twice the perturbation this change can
cause — and whose probes and full recording are also the product's.

## The declared delta is nothing at all, and it is phase 85's kind

The code runs and the instrument cannot see it, **measured rather than inferred**: of
the 102 screen cases, 95 ring the bell once and 7 not at all, and **not one rings it
twice**, so `vim_beep`'s 500 ms limit — the only clock reading a screen case can reach
— is never asked to suppress anything. Three controls say it from the other side: a
clock that never advances, one that runs backwards and one that runs 1000× fast each
move **0 of the 102**.

So the phase owes probes, and they are built on **`gs`** — `nv_g_cmd`'s `s` arm is
`do_sleep(count * 1000)`, the one call site a keystroke file can drive and the only way
real time passes inside the editor. `1gs` is 1,009 ms on the binary the phase was
handed and 1,008 on its own, `2gs` 2,005 and 2,004, and `hgshh` rings **2** bells on
both — `vim_beep`'s threshold in both directions in one probe. Each half fails on a
control aimed at it: with the clock 1000× fast `2gs` returns after one wait; with a
clock that never advances `1gs` **never returns**; with `vim_beep`'s 500 written
500000 `hgshh` rings 1 bell and with it written −1 it rings 3.

**The sleep assertion was wrong once and the fix is the interesting part** (commit
`6ef24b7`, after the phase landed). It asked for `2gs - 1gs >= 900`, and **both numbers
are wall-clock times taken from outside, around whole editor runs**, so each carries its
own startup jitter: phase 113's verify caught it on a loaded machine with `1gs` inflated
to 1,133 ms against `2gs` at 2,006, and a correct phase failed. Widening the threshold
would move the boundary rather than remove it. **A lower bound on a sleep cannot flake
in that direction** — a sleep takes at least as long as it asks for and load can only
make it longer — so the assertion is `2gs >= 1900`, and the `fast` control still breaks
it at 1,008 ms.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 78,153 | **78,144 (−9)** — re-measured on the canonical text; the core loses 15 and the host gains 6. The rest of this table is not re-measured |
| `elapsed_T` / `elapsed()` / `musl_gettimeofday` | 3 things | **0** |
| `make editor.c` | 78,358 lines | **78,342**, 0 directives, 13 boundary names |
| the boundary | 13 names | **13**, `musl_gettimeofday` → `musl_now_ms`, asserted as a *set* |
| `nm -u` | 17 | **17, the same set**, `gettimeofday` among them |
| external symbols | `main` | `main` |
| binary | 788,488 | **788,488 bytes, and not the same bytes** |
| records that moved | | **0 of 106**, on four recordings — input, output, `ceil`, `epoch` |

## Its placement

`stage 111`, `package boundary` — **not `host`**, and the phase argues it: `boundary` is
not only where the line falls but what the core may **name** at it, which is what 106, 108
and 109 each did, and this is 109's clock item finished; `host` would be wrong more
plainly, since 100 to 104 move code into the launcher and this phase moves none. Four
`uses`: `seed:83` and `harness:86` for the recording — where the check says outright that
the recording is the **weakest** part of the evidence — `host:103`, because `musl_now_ms`
is defined inside the block that phase created and `tools/zhostonly.py` reads the host
region from `host_winch_pending`, and `host:101`, because the probes rest on the editor
being an ordinary program a keystroke file can drive to exit.

`apart 110 111` is measured by running phase 110's check on the tree this phase leaves: it
stops at its first act with *"`elapsed` is not defined exactly once above the boundary,
so the control that moves one core function below it would not be a control"* — **phase
110's boundary argument rests on moving one core function below the cut, and the function
it picked is the one this phase deletes**. One direction only. `need 111 swept` is
measured *not* to be required: run on the unswept tree from phase 110's edit cache, this
edit gives a byte-identical `whim-vim.c` and the sweep after it is a complete no-op.

`tools/zhostonly.py` gains `gettimeofday` as a host word, which this phase is what makes
permanently true, with the five core call sites phase 109 moved named as exceptions at
the counts they had at q103, q104 and q108. Measured: exactly ten keys move — Part II units
and edits 103, 104, 108, 109 and 110 — and **not one slim or whim key of the 107**.
