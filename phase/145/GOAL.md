# Phase 145 — `check_termcode()` has no goto

While an OSC response was arriving over several reads, `check_termcode()`
jumped from the top of its loop to `handle_osc`, a label inside the OSC branch
of the if-chain in `if (key_name[0] == NUL)`, skipping everything in between.
That was the last `goto` of finding 11 that the transpilation had to
restructure. Nothing follows that chain inside its block, so the jump ran
exactly the OSC handling and then the code after the block. Now the jump's
`if` does the handling itself, and everything the jump skipped, from the key's
first byte through the end of the block, becomes its `else`. A `continue` or
`break` in that code binds to the same loop as before, because an `if` catches
neither.

**Declared delta: nothing.** The check proves from the input that the label's
chain is the last thing in its block. It requires the `else` to be the
skipped code byte for byte, indented four spaces further. Its probe sends an
OSC response in two writes and then types, and requires the same output
from both binaries. Two controls move: typing other text, and sending no
response. Whether the pty delivers the two writes as two reads is up to the
kernel, so for that path the byte-for-byte `else` is the evidence.
