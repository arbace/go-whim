# Phase 129 — `p_emoji` is an `int`

The first phase that comes of transpiling `editor.c` to Go (`tx/FINDINGS.md`,
finding 1). `'emoji'` is a boolean option, and the options table writes and
reads every boolean option through an `int *` — `set_option_default()` stores
`*(int *)varp`, as `do_set_option_bool()` and `set_bool_option()` do — but its
variable was declared `char_u *`. The C got away with it: the static starts
zeroed and the `int` lands in the pointer's low bytes, so
`utf_char2cells()`'s `if (p_emoji && ...)` tests the right thing. The Go
transpilation cannot say that, and its first run panicked in
`set_option_default()`. The phase declares the variable what every writer and
the one reader already take it to be; one line, no line added or removed.

**Declared delta: nothing.** No recorded case types an emoji. The check's
evidence is a partition and a probe:

- **every `P_BOOL` row of `options[]` with a global variable names an `int`**
  on the output; on the input exactly one did not, `'emoji'`. The check fails
  if another appears or this one comes back.
- `p_emoji` is its declaration, its row and its reader, and nothing else.
- the libc surface is the set the stage was handed.
- **the probe**: `iab<U+1F600>cd<Esc>:q!` is written in the same bytes by the
  binary the phase was handed and the one it made; and **the control**,
  `+set noemoji`, writes different bytes, so the probe sees the option at all.
