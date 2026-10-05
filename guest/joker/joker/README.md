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
