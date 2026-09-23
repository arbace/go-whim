116 DECLARES NOTHING AT ALL, and it is a NINTH kind: the phase changes no source at
all -- q116's zero-vim.c is its input's, byte for byte -- and what moves is the
question one of the five harnesses asks.  Phase 86 is the only other of that kind,
and neither declares anything, for the same reason: nothing about the editor
changed, so nothing about its behaviour can have.

BUT THE COMPARISON ITSELF MOVED, WHICH NO OTHER PHASE HAS DONE, and that is what
has to be argued here rather than merely stated.  `tools/ztermcheck.py` asked
`$TERM`, and phase 19 removed the `getenv("TERM")` the editor used to read it
with, so all nineteen rows of .reference/zero-baselines/ref-term.txt said
`term=xterm-256color t_Co=256` -- nineteen ways of recording that the environment
does nothing.  The table was CONTENT-FREE, and measurably so: a prototype that
deleted eight of the ten built-in terminal names and three of the nine capability
tables passed tools/zcompare.py against the real baselines DECLARING NOTHING.  The
phase asks `+set term={name}` instead, which reaches did_set_term(), and the
baselines are re-recorded from whim-vim.c -- the pipeline's immutable input, with
whim's own compile line, which is phase 83's job and not a re-recording from
zero's own output.

WHY THAT DOES NOT MOVE AN EARLIER PHASE'S DECLARED DELTA, measured rather than
reasoned: ./zero-vim extracted from EVERY recorded boundary tar, plus whim-vim.c
built with whim's line, records the SAME nineteen rows under the new question --
ONE md5 across all of them, exactly as the old question gave one md5 across all of
them.  The baseline and every phase's recording move together, so `term-moved`
stays undeclared at every phase, before this one and after it.  phase/116/make.sh
makes that check rather than citing it.

AND THE INSTRUMENT IS PROVEN ABLE TO FAIL, which the old one was not: with ONE row
deleted from builtin_terminals[] the new table moves exactly one of its nineteen
rows, from resolving to refused, and the question this replaces moves NOTHING AT
ALL.  That pair is the phase.
