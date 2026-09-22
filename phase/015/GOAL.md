# Phase 15 — the last two encoding options

**Phase 12 emptied `'fileencodings'` and said so, and it was true at startup and
not afterwards.** `set_option_default()` special-cases the option, so `:set
fencs&` restored `ucs-bom,utf-8,default,latin1` from `fencs_utf8_default` — a
third reference Phase 12 did not find, because it names the *string* rather than
the function the other two called. Measured on the shipped binary before this
was written:

```
  at startup           fileencodings=
  after :set fencs&    fileencodings=ucs-bom,utf-8,default,latin1
```

That is worth recording as a pattern and not just a fix. Phase 12 cut two
callers of `set_fencs_unicode()` and asked whether anything still called it;
nothing did. The question it did not ask was whether anything still used the
*value*, and a search for the function name cannot answer that.

Three readers go, and with them the two options can finally follow.
`set_option_default()` stops special-casing `'fileencodings'`, which is what
makes Phase 12's claim true at every moment rather than one. `readfile()` stops
choosing between an empty list and a list to walk, and takes the buffer's own
`'fileencoding'` — the branch the empty case already took. And
`did_set_encoding()` stops setting up a conversion between `'termencoding'` and
`'encoding'`, which `convert_setup()` has answered `CONV_NONE` to since Phase 12,
so the block could only ever have succeeded at doing nothing.

**`'encoding'` still cannot go, and here that stops being temporary.** `p_enc`
is the *name* of the one encoding, compared against in twenty-nine places.
Removing the option would mean removing the name, and the name is doing work.
Of the six encoding options this fork began with, one remains, and it reports
`utf-8` and refuses everything else.

## The delta

**None.** `:set fencs&` no longer restores a list of encodings this build cannot
convert between, which is a correction rather than a change.
