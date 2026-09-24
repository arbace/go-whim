# Phase 139 — the core sorts and searches typed arrays

The core sorted one array and searched four through the vendored
`musl_qsort()` and `musl_bsearch()`. These see an array as a `void *` stepped
by a byte width, and hand each element to the comparator as a
`const void *`. The Go transpilation could not follow a pointer through
`void *`: `sort_strings()`'s first Go signature was wrong, and each search
was typed by hand (finding 7).

The four searches — highlight attributes, colour names, key names and
character classes — now call `keyvalue_bsearch()` or `key_name_bsearch()`.
Each is `musl_bsearch()` line for line on a typed pointer, so it probes the
same entries in the same order, and a comparator that matches a prefix finds
the entry it found before. The comparators take the type they always cast
to. `:undolist`'s one sort is an insertion sort by `strcmp()`: two strings
that compare equal are equal byte for byte, so any order of them prints the
same. The sweep takes `musl_qsort()`, `musl_bsearch()` and `sort_compare()`.

**Declared delta: nothing.** The check proves each typed search's body is
the input's `musl_bsearch()` with its two byte steps made element steps. It
requires each search to use the table and comparator it had. Its probes are
`:hi` attributes, a colour name, a key name in a mapping, a character class
in a pattern, and `:undolist` over two branches; each control moves.
