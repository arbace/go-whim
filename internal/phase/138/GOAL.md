# Phase 138 — no parameter carries an eval value

Five functions still took an eval value that every call passed as `nullptr`:
`vim_regsub_both()`'s `typval_T *expr` (substitute() with a funcref),
`match_add()`'s `list_T *pos_list` (matchaddpos(), never even read),
`cursor_pos_info()`'s `dict_T *dict` (wordcount()), the host formatter's
`typval_T *tvs` (printf()'s argument list, also handed to
`parse_fmt_types()`), and `find_ex_command()`'s Vim9 lookup and compile
context, never read. Each goes with its `nullptr`, and each test of it
becomes what it always was: `cursor_pos_info()` always gives its message, the
formatter always clamps an overlong width. The unused `cfunc_T` and
`cfunc_free_T` typedefs go too, since the sweep doesn't take a
function-pointer typedef. Then nothing names the eval layer's value types,
and the sweep takes all of them: `typval_T`, `list_T`, `dict_T`, their items
and watchers, `type_T`, `class_T` and the rest. Phase 137 made this possible, and
`vim_regsub_both()`'s second argument being always `nullptr` was the lead.

**Declared delta: nothing.** The check's partition: in each function every
line naming the parameter is its head, a test against `nullptr`, or its
being handed on, and each function's one call passes `nullptr`; on the output
none of the 16 eval types is named. Its probes cover the paths the tests sat
on — `g CTRL-G`, a `\=` substitution and `:match` — and each control moves.

**Already gone is a class of its own.** The cut of `cfunc_T` and `cfunc_free_T` is a partition, not a
count: the typedef is here and this phase removes it, or nothing at all names
it and the phase says so and cuts nothing (`edit.Ph.LiteralOrGone`). The
second class exists for the sweep's closure (`internal/sweep`,
the sweep itself since the six deleters went), whose closure takes a typedef nothing names in
an earlier sweep. Anything else refuses as it always did.
