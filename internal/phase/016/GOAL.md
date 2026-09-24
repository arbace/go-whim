# Phase 16 — six options that no longer decide anything

`'path'` and `'suffixesadd'` have been inert since the file finder went,
`'tags'` and `'tagcase'` since the tag stack, `'autoread'` since the timestamp
poll, and `'swapfile'` since the swap file. All six were still here, because a
row is what initialises its global and `tools/dropoptions.py` refuses to leave
one dangling — **Phase 10's trap, which this phase clears rather than works
around.**

## The order is the phase, and it is forced rather than chosen

1. the three readers that are not plumbing
2. the rows, with `--local`
3. **the sweep** — which is what removes `did_set_tagcase()` and
   `did_set_swapfile()`, the option callbacks, reachable only from the rows
4. the buffer fields and their plumbing
5. the sweep again

**Steps 3 and 4 cannot swap**, and the reason is a property of how this pipeline
sweeps rather than of the code. The callbacks read the buffer field, so removing
the field first stops the file compiling; the sweep works by reading gcc's
*warnings*, so a file that does not compile is a file the sweep cannot act on,
and the callbacks would stay for ever. Every other phase has been free to order
its cut however it liked; this one is not.

## The three that are not plumbing

`ex_drop()` set `'autoread'` on, checked the timestamp, and set it back —
and Phase 13 took the check out from between, so what was left was a variable
saved and restored across nothing at all. `do_set_option_bool()` special-cased
`:setlocal autoread` to mean "follow the global", the `-1` sentinel, and there
is no global to follow. `ml_open()` asked whether this buffer may have a swap
file; since Phase 11 the answer has been no whatever `'swapfile'` said, so it
now says no directly.

Everything else is the five fixed idioms every buffer-local option has — the
field in `buf_T`, the initialiser in `buf_copy_options()`, `check_buf_options()`,
`free_buf_options()`, and one or two `get_varp()` cases — which is what makes
`tools/droplocal.py` possible at all. It takes the *field* name rather than the
option's, because by the time it runs the row is already gone and there is
nothing left to look the field up from.

## The delta

**None.** All six report `E518: Unknown option` instead of a value that decided
nothing.
