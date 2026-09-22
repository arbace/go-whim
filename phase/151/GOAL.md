# Phase 151 — the option table's defaults are typed

Each row of `options[]` held its defaults, for Vi and for Vim, in
`def_val[2]`, a pair of `char_u *`. A string option kept strings there. A
number or boolean option kept the number itself cast to a pointer, such as
`(char_u *)80L` or `(char_u *)TRUE`, cast back with `(long)(long_i)` wherever it
was read. The Go transpilation held them as `any` (finding 2, its first half).

Now a row has `def_str[2]` and `def_num[2]`. The pair its kind uses holds the
defaults, and the other is empty. Every read and write names the one it
means, so no number is stored as a pointer, and the `long_i` typedef that
existed for the cast is swept.

**Declared delta: nothing.** The check recomputes every row's two pairs from
the input's pair with the same rule, row for row, and requires no `def_val`
field left. The silent compile is the proof that no read takes a string where
a number is. Its probes set every option to its default and list them
(`:set all&` then `:set all`), list the terminal options, and reset one
option. Each control moves.
