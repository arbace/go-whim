# Phase 14 — a file name means the file of that name

`'path'` searching is the last of the three ways this editor knew where files
live, after globbing (Phase 7) and `'tags'` (Phase 10). `vim_findfile()` walks a
path list downward and upward, remembers directories it has visited so a symlink
loop cannot trap it, and can be asked for the second match and the third — 866
lines of filesystem-layout knowledge behind `:find`, `:sfind`, `:tabfind` and
`gf`.

`:find`, `:sfind` and `:tabfind` are retired: the whole of what they do is the
search.

**`gf` is kept, and resolves the name literally.** It is the one place a user
names a file from *inside the buffer* rather than on a command line, and taking
it away would be taking away the naming rather than the searching. So
`find_file_in_path()` stops consulting `'path'` and answers the only question
left — is there a file of this name? Fifteen lines against eight hundred and
sixty-six, and it reaches the filesystem no differently from `:e`.

Two details of the contract it has to keep, both visible in
`find_file_name_in_path()`: `first == FALSE` asks for the *next* match, which is
what `3gf` and `]f` use, and there is never a next one now — so it answers NULL
and the caller's loop ends, which is the same answer the search gave when the
path held one match. And the result is owned by the caller, so it is allocated
even though the name is already in hand.

`'path'` and `'suffixesadd'` cannot go — `PV_BOTH` and `PV_BUF`, and a row is
what initialises its global. They stay, and now decide nothing.

## The delta

`gf` opens the name under the cursor if there is a file of that name rather than
searching `'path'` for one. **No Ex command moves** — Phase 10's lesson again
rather than a surprise: retiring a command only shows in the sweep if it used to
*succeed*, and `:find`, `:sfind` and `:tabfind` already failed for want of an
argument.
