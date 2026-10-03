package p099

// Whim phase 99 (formerly 180) -- the host's clock can be held still.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim99", Edit) }

// Edit has the C host's host_time() return WHIM_TIME, when the environment
// holds it, instead of the time: a clock the suite holds still, so that
// undo's "N seconds ago" does not depend on when a run crossed a second.
// getenv() and atol() are <stdlib.h>'s, which phase 88 keeps since it
// asks that the file's own tokens mean the same without a header (phase 43's
// EXIT_FAILURE assertion names it): the include is asserted, not added.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("pinnedtime", text, w)
	e.Literal("    static long\nhost_time(void)\n{\n    return time(nullptr);\n}\n",
		"    static long\nhost_time(void)\n{\n    char *pinned = getenv(\"WHIM_TIME\");\n    if (pinned != nullptr && *pinned != NUL)\n    {\n        return atol(pinned);\n    }\n    return time(nullptr);\n}\n", 1,
		"host_time() returns WHIM_TIME when it is set")
	e.Literal("#include <stdlib.h>\n", "#include <stdlib.h>\n", 1,
		"<stdlib.h>, for getenv() and atol(), there once")
	return e.Done()
}
