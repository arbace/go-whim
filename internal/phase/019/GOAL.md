# Phase 19 — the terminal is what the build says

Five environment variables describe the terminal and the editor believed all of
them: `$TERM` picks a capability table, `$LINES` and `$COLUMNS` override the size
the kernel reports, `$COLORS` overrides the colour count, `$COLORFGBG` the
background.

## The compiled name is `xterm-256color`, and that is the whole care here

Measured on the shipped binary before choosing:

```
TERM=xterm-256color   -> term=xterm-256color  t_Co=256
TERM=xterm            -> term=xterm           t_Co=8
TERM= (unset)         -> term=xterm           t_Co=8
```

`set_termname()` keeps the *requested* name and tests
`strstr(requested, "256color")` to apply `builtin_256colors` on top of whichever
table it chose. So **the obvious fallback — the one the unset case already took
— would have cost eight of every nine colours the terminal can show**, silently,
for nothing. `xterm-256color` resolves to the same `builtin_xterm` table and
keeps the add-on.

## The size is still autodetected

`ioctl(TIOCGWINSZ)` stays; only the `$LINES`/`$COLUMNS` override goes. Verified
on a pty: with the window at 24×80 and `LINES=9 COLUMNS=9` in the environment,
the editor reports 24×80. An editor that believes `$LINES` over the kernel is
one that draws off the bottom of a resized window.

`-T <term>` stays. It is not the environment, and with one compiled default it
is the only way left to say "this is a dumb terminal"; the ten built-in entries
are still there and `-T` still reaches them.

## The delta

**The terminal table collapses.** Nineteen rows, one per `TERM` the harness
tries, each of which used to resolve to its own entry — now every one of them,
including unset and `no-such-term-9x`, answers `term=xterm-256color t_Co=256`.

That is declared with `--term-moved`, which `tools/whimdelta.sh` grew for this
phase. Until now no phase could move that table, so *"expected unchanged"* was
the whole check; a phase that makes every terminal resolve to one entry has to
be able to say so, and the flag asserts the table moved rather than merely
allowing it to.

## `cutil.drop_if`, extracted here

Four phase tools had written their own "delete an `if` and the block it guards",
and four had written the same bug: a lazy `(?:[^\n]*\n)*?\}` to find the end,
which stops at the first line that is only a brace — an inner block's, whenever
there is one. This phase made it five. It is one function in `cutil.py` now,
brace-matched, refusing a block that has an `else`.
