# Phase 30 — `K` and the tag jumps, keeping `*` and `#`

`nv_ident()` is not one command, it is five, and they have nothing in common but
the first step — read the identifier under the cursor:

| | | |
| --- | --- | --- |
| `*` `#` `g*` `g#` | search for that word | **stay** |
| `K` | run `'keywordprg'` on it | goes |
| `]` `CTRL-]` `g]` | jump to its tag | goes |

`*` and `#` are among the most used keys in vim and are pure search, so this
phase **rewrites** the function rather than deleting it. `K` runs `'keywordprg'`
through `:!` and Phase 8 took the process behind it; the tag jumps build `ta `, `tj `,
`ts ` or `he! ` and hand them to `do_cmdline_cmd()`, and Phase 10 made every one
of those `ex_ni`. Both arms have been building commands that fail.

## Rule 3 applies to normal-mode commands too

The first version **deleted** the `K` and `CTRL-]` rows from `nv_cmds[]`. It
built, it swept clean, it passed the linkage and symbol checks — and **39 of the
67 behaviour cases moved**: CTRL-A, joins, macros, marks, multibyte motions,
nothing to do with `K` or tags.

`nv_cmd_idx[]` is a `static const` array of **indices into `nv_cmds[]`**,
precomputed and sorted by command character, with `nv_max_linear` marking how
far a direct lookup works. Deleting two rows shifts every later index while the
precomputed table still points at the old positions, so every normal command
after them dispatches to the wrong function. It is the parallel-table trap the
enumerators have, one table over.

So **Rule 3 — a command is never deleted from the table, it is pointed at
`ex_ni`** — extends to `nv_cmds[]`, where the equivalent is `nv_error()`, the
handler already used for keys that do nothing.

## And the check had to be a pty

The first `*` check ran under `-e -s` and compared the file. The two binaries
disagreed — before the phase `:normal *dd` did nothing at all, after it the `*`
was ignored and the `dd` deleted line 1. **Neither is what `*` does.**
`normal_search()` wants a screen, so silent Ex mode measures something that is
not the feature. In a real pty both binaries give the same correct answer, and
`tools/starcheck.py` now asks it there: from `foo` on line 1, `*` must land on
the `foo` on line 5 and **skip `foobar`**, which `dd` then proves.

Measured: 136,190 → 135,941 lines; `nv_ident()` from 227 lines to the search
half. **The delta is none** — these are normal-mode keys, so no Ex command
moves.
