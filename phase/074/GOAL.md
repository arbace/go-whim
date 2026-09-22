# Phase 74 — no file marks

**This is the phase that was abandoned as 70**, and the difference between the two
attempts is the whole lesson.

The first attempt died three times on patterns transcribed from *truncated views* —
`ex_delmarks`' `|| to < from` tail, `clrallmarks`' `static int i = -1` guard over
`26 + 1` (not `26 + EXTRA_MARKS`, which is what I kept writing), and four adjust
sites I had never enumerated. Worse, its probes **measured nothing**: a mark name
like `'A` carries a quote, the shell quoting broke, and the key was never pressed —
so three runs produced no evidence either way. It was dropped on instruction.

This time every site was read with `cat -A` first, **every anchor was pre-flighted
against pristine q73 and came back ALL OK before the script ran**, and both probes
were calibrated on q73. It passed its first dry run.

## What goes

`namedfm[26 + EXTRA_MARKS]` — which is **both** the uppercase A–Z file marks and the
numbered 0–9 marks. One array, one set of code paths, so they cannot be separated;
and with viminfo long gone nothing could ever set the numbered ones anyway, so they
were a store no code could write.

`getmark_buf_fnum`'s A–Z/0–9 arm was the **only** caller of `buflist_getfile()` and of
`fname2fnum()` — the latter already an empty body from phase 70, folded there
precisely because *"the file marks are a separate cut"*. Removing the arm leaves
`posp` NULL, which `check_mark()` already reports as E20, exactly as an unset
lowercase mark does.

The sweep then took `buflist_nr2name`, `fmarks_check_one` and `getfile` by cascade.

## What stays

The lowercase marks `a`–`z` in `buf->b_namedm[]`, and every special mark — `'`,
`` ` ``, `"`, `^`, `.`, `[`, `]`, `<`, `>` — none of which touch `namedfm`. `:marks`
still lists what is left and `:delmarks` still clears it; uppercase and digits now
fall to "invalid argument".

**`fmark_T` stays**: `struct taggy` embeds it, so the tag stack depends on it. Only
`xfmark_T`, which exists to bolt a filename onto a mark, goes.

**`do_join()` has a parameter named `setmark`**, so every edit is scoped by function.
An unscoped pattern or a global rename would have corrupted it — the `b_next` lesson
from phase 71 wearing a different hat.

## The delta

**None**, and `whimdelta.sh` confirms it.

The probes are worth stating because this is where the first attempt failed. The
**discriminator** is a pair: on q73 a lowercase mark gives `K1|K2-kept|K3|` and an
uppercase mark gives `U1-up|U2|U3|`; after the cut lowercase is unchanged and
uppercase gives `U1|U2|U3|`. The uppercase half *changes*, which is what makes it
evidence rather than decoration. The mark name is kept out of shell-metacharacter
position by double-quoting the vim text.

`:marks` and `:delmarks` are **not** discriminators — both write the file either way
and neither reaches stderr — so they are no-crash checks only, and the script says so
rather than letting a later reader mistake them for proof.

Measured: 91,329 → **90,972 lines**.
