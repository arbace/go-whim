# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

**The editor in Clojure** (`doc/CLOJURE.md`: the plan, its measurements and
four milestones). 45% of the core's functions jump -- return early, break,
continue, goto, fall through -- which Clojure cannot say, and nearly all
assign to locals: the backend lowers each function to basic blocks first, and
prints it structured where it nests and as a `loop`/`case` state machine where
it does not, on the Java editor's runtime and host.

**Revisit arbace/go-lisp#1** (`doc/GO-LISP.md`). Opened 2026-09-25: the two
go-lisp bugs found while building the Go editor as go-lisp -- make.bash fails
for `internal/golisp` missing from `cmd/dist`'s bootstrap list, and `go tool
golisp` does not exist -- fixed in two commits. When it is merged, point
GO-LISP.md's reproduction at the `go-lisp` branch again (it clones the PR's
branch now), and drop this item; if it is changed or refused, follow what the
review says.


## Known stale, not yet scoped


## Declined, with the reason recorded
