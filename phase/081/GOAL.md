# Phase 81 — one line, one command

An Ex line could hold several commands separated by `|` and end in a `"` comment.
Both exist for scripts — a vimrc, a sourced file, a function body — and this editor
reads none. Every command it runs was typed, came from `+cmd`, or came from a
mapping's right-hand side. So a line is one command now, and `|` and `"` are ordinary
argument characters. **A newline still ends a command**: that is the rule itself, and
the newline branch of `separate_nextcmd` is kept exactly as it was.

**The machinery was small and in one place.** `separate_nextcmd` split a bar-splitting
command's argument at `|`, `"` or a newline; `check_nextcmd`, `find_nextcmd`,
`ends_excmd` and `ends_excmd2` each knew the same three characters; and a handful of
callers knew them again — `:substitute`'s tail, the trailing-characters check in
`do_one_cmd`, `:a|text`, `:|` printing the line, and the whole-line `:" comment`
with the `starts_with_colon` flag that only existed to feed it.

**Decided before it was written:**

- `a|b` — the bar is argument text. A command without `EX_EXTRA` reports E488; one
  with it takes the bar. `:map Q A|b` now maps `Q` to `A|b`.
- `a " x` — the quote is argument text too, so `:set ts=3 " x` is an error and
  `:" x` is E492.
- `\|` means nothing special: the backslash stays, so `:map Q A\|b` maps to `A\|b`.
  CTRL-V handling is unchanged.

**`EX_NOTRLCOM` stays.** Its comment meaning is gone, but it still decides whether
trailing spaces are stripped, which is what lets a mapping end in a space.

## The delta

No behaviour case, terminal row or swept command uses a bar or a comment, so the
cumulative list is phase 80's, unchanged, and `whimdelta.sh` confirms it. What moves
is probed directly: 43 cases through q80's binary and this one, comparing exit
status, stderr and what was written. Fourteen differ, each declared with its reason;
29 controls must not — `:s/a\|b/…/` and `:g/a\|c/d` (a bar inside a pattern was
never a separator), `:normal! A|x`, `:map Q A"b` (a mapping never took a comment),
CTRL-V before a bar, and two commands separated by a real newline.

One expectation in the corpus was wrong on the first run, and not the edit: the
mapping cases assumed the cursor on line 1, and `-e -s` starts on the last line.

Measured: 87,142 → **87,107 lines**. A small cut by count — the point was the
rule, and the splitter was never large.
