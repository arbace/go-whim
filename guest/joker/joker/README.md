# guest/joker/joker -- Joker, forked

[Joker](https://github.com/candid82/joker) v1.10.0, upstream commit
c87eb9502fd6ea623dbf8f48aebb4ccd6ea1d1a8 (2026-10-05), a Clojure dialect
written in Go, under the Eclipse Public License 1.0 (`LICENSE`), for the
Joker guest (`guest/joker`, doc/LISP-SANDBOX.md): the language in the box.

**What is here.** Upstream's tracked `core/` and the standard namespaces
that need no system -- base64, crypto, csv, hex, hiccup, html, json, math,
mime, strconv, string, time, url, uuid -- and nothing else: no command
line, no REPL's readline, no bolt, git, http, mail, os, pop3, smtp, yaml,
markdown. So the module requires nothing outside Go's standard library.
`core/a_*.go` is generated (`go generate ./...`, which `go tool whim guest
--joker` runs) and not tracked.

**What is changed**, each marked `go-whim` in the source:

- `core/spew_enabled.go` and `core/line_runereader.go` left out (go-spew,
  behind a build tag; liner, the terminal REPL's readline).
- `core/vector.go`: `popTail` stores a real nil in the slot of a child it
  emptied. A nil slice stored in an `interface{}` is not nil, so `Pop` kept
  a level it should have dropped and the next `conj` read an empty node:
  `(conj (pop (vec (range 1057))) :x)` was a Go panic.
- `core/read.go`, `core/data/core.joke`: tagged literals. The reader
  consults `*data-readers*` (dynamic, bound with `binding` as Clojure's),
  then `default-data-readers`, then `*default-data-reader-fn*` with the tag
  and the form; a reader is a function or a var of one. Upstream names
  `default-data-readers` and reads no tag with it. So the graph's EDN reads
  as it is: `(binding [*data-readers* {'g/n f ...}] (read-string s))`, which
  the views in Joker (`../gview`) do.

**What it lacks that the views missed** (gaps, not faults; worked around
in `../gview`, nothing changed here): no transducers -- `map` and
`filter` have no one-argument arity, there is no `transduce` -- so
`(into #{} (map f) xs)` is an arity error and `(set (map f xs))` is
written; no `volatile!` -- an atom; and `count` of a string is its
characters, so a byte length (the Go's widths) is counted from the code
points.
