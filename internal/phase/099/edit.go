package p099

// Whim phase 99 (formerly 180) -- the host's clock can be held still.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim99", Edit) }

// Edit has the C host's host_time() return WHIM_TIME, when the environment
// holds it, instead of the time: a clock the suite holds still, so that
// undo's "N seconds ago" does not depend on when a run crossed a second.
// getenv() and atol() are <stdlib.h>'s, which phase 88 keeps since it
// asks that the file's own tokens mean the same without a header (phase 43's
// EXIT_FAILURE assertion names it): the include is asserted, not added.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, Step6): the body written by FRAG,
// its calls the header's externs as the import makes them; the include
// asserted among the file's include forms.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("pinnedtime", e, w)
	v.CountIs("(return (call time nullptr))", 1, "host_time() returns WHIM_TIME when it is set -- its body is not `return time(nullptr);`")
	v.BodyC("host_time", "    char *pinned = getenv(\"WHIM_TIME\");\n    if (pinned != nullptr && *pinned != NUL)\n    {\n        return atol(pinned);\n    }\n    return time(nullptr);\n",
		"host_time() returns WHIM_TIME when it is set")
	n := 0
	for _, f := range e.Graph().Forms {
		if f.Is("include") && len(f.Kids) == 2 && f.Kids[1].Atom == `"<stdlib.h>"` {
			n++
		}
	}
	v.Expect(n == 1, "<stdlib.h>, for getenv() and atol(), there once -- %d include forms name it", n)
	v.Say("<stdlib.h>, for getenv() and atol(), there once")
	return v.Done()
}
