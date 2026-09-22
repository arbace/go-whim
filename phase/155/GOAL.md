# Phase 155 — call arguments with effects are evaluated in gcc's order

C leaves the order of a call's arguments unspecified, and Go evaluates them
left to right. gcc, measured on this file's own code line by line in its
disassembly, evaluates them right to left. Where two arguments of one call
both call a function with an effect outside its frame, that order is part of
what the program does. `internal/ccx`'s `Order` finds eleven such calls:
- nine `vim_strnsave(ml_get...(), ml_get..._len())`, which gcc evaluates
  length first;
- `col_print(..., ml_get_curline_len(), linetabsize_str(p))`, which it
  evaluates width first;
- `fileinfo()`'s message, whose `new_file_message()` it calls before the
  `shortmess()` of an earlier argument.

The argument gcc evaluates first becomes a local computed before the call, so
the order is written, not implied, and a translation that evaluates left to
right is this program.

What `Order` still finds is four pairs of binary operands. gcc evaluates each
left to right, as Go does, so they need nothing. No operand anywhere writes
what another reads; that would be undefined behaviour in C, and there is none.

**Declared delta: nothing.** The check measures gcc's order on the input's
code at every rewritten call, and requires no argument pair left. It requires
each remaining binary pair to be called left to right in the output's code.
Its probes run `:copy`, `:move`, opening a line, CTRL-G, g CTRL-G and a Tab,
and each control moves.
