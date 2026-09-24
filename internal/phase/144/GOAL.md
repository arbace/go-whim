# Phase 144 — `edit()` has no goto

Insert mode's loop jumped to three labels (finding 11):
- `doESCkey`, the second half of the Esc case, from nine places, some of them
  before the switch;
- `normalchar`, the default case's insertion, from seven cases;
- `do_intr`, the start of the Esc case, from the default case when the key is
  the interrupt character.

The two labelled blocks become functions: `edit_esc()`, which says whether
Insert mode ends, and `edit_normalchar()`. The locals they wrote (`count`,
`o_lnum`, `inserted_space`) are passed by pointer. Each jump becomes a call:
- **`doESCkey`:** `if (edit_esc(...)) return (c == Ctrl_O); continue;`. A
  `continue` means "the next key" only where the innermost loop is the main
  one, and the edit checks that at every site. The one jump inside a do-while
  sets `esc_now`, breaks out, and the flag is tested (and reset) right after
  the loop;
- **`normalchar`:** a call and a `break`, checked to leave the main switch;
- **`do_intr`:** its `goto_im()` test written out, then the call.

`check_termcode()`'s jump into an `if` body is phase 145's.

**Declared delta: nothing.** The check requires the helpers to be the input's
blocks, line for line apart from the pointers. It requires every `continue`
and `break` at a former jump to bind where the label's own did. It diffs
`edit()` and requires every change to be accounted for. It probes every key
whose case jumped that a terminal can deliver: Esc, CTRL-O, Tab, CTRL-K,
CTRL-], CTRL-F, CTRL-S, CTRL-L, CTRL-Z, CTRL-A and Enter. Three paths are out
of the harness's reach:
- CTRL-C: on its pty it is SIGINT, and both binaries exit before drawing;
- the do-while's site, which needs `stop_insert_mode`;
- `do_intr`'s, which needs an interrupt character other than CTRL-C.

For those three, where their `continue` and `break` bind is the evidence.
