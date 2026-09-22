# Phase 158 — a highlight's terminal font is read only from a colour entry

A highlight attribute's entry, `attrentry_T`, keeps what it draws in a union.
A term entry holds the `start` and `stop` strings; a cterm entry holds the
colours and a font. Which one an entry holds is its table, and which table a
highlight reads is `t_colors > 1`. `internal/ccx`'s `Unions` checks every access
to every union in the core against its discriminant:
- `ae_u` by `t_colors` or the table;
- the regexp's saved positions by `rex.reg_match == NULL`;
- a regstack item by its state;
- an option's old and new value by the kind of the row whose callback reads it.

It also requires that nothing a reader can call before its read writes the
discriminant. A function that saves `rex` and restores it is a barrier.

It finds one pun. `screen_start_highlight()` tests `cterm.font` before it looks
at `t_colors`, so on a terminal without colours it reads the font out of a term
entry. The font is the two bytes six past `term.start`, which are the top two
bytes of an x86-64 pointer. They are 0 in every user-space address and in NULL,
so the test read 0 and failed. The test now asks `t_colors > 1` first and fails
without reading.

With it, every union is used as if each member were its own field, which is
what a translation into a language without unions gives it.

**Declared delta: nothing.** The check has gcc measure the layout on the
input's own types; the control, offset 4, is refused. It requires `Unions` to
leave exactly those three reads on the input and nothing on the output. It
probes a term entry with a `start` string on a terminal without colours, and a
cterm entry with a font; each control moves.
