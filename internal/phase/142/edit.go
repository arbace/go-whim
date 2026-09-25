package p142

// Whim phase 142 -- the version names no build date or time.  See GOAL.md.
//
// init_longVersion() put __DATE__ " " __TIME__ into the version line a
// command-line error is headed with, so the binary depended on when it was
// built (internal/gen/FINDINGS.md, 13).  The version is its name and release date.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim142", Edit) }

// Whim142 takes the build's date and time Out of the version.
//
// init_longVersion() put __DATE__ " " __TIME__ into the version line that
// mainerr() prints above a command-line error: "VIM - Vi IMproved 9.2 (2026
// Feb 14, compiled <date> <time>)".  So the core's text was a function of when
// it was compiled, and two builds of the same source differed unless
// SOURCE_DATE_EPOCH pinned them; the Go transpilation had no such clock and
// wrote in the constant the pinned build produces (internal/gen/FINDINGS.md, 13).  The
// version is now its name and its release date, "(2026 Feb 14)", and the
// binary is a function of the source alone.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "datetime", W: w}
	var err error
	steps := []struct{ Old, New, What string }{
		{"char *msg = _(\"%s (%s, compiled %s)\");", "char *msg = _(\"%s (%s)\");", "it is the name and the release date"},
		{" + sizeof(VIM_VERSION_DATE_ONLY) - 1 + musl_strlen(date_time);\n", " + sizeof(VIM_VERSION_DATE_ONLY) - 1;\n", "and its length counts no date"},
		{"msg, VIM_VERSION_LONG_ONLY, VIM_VERSION_DATE_ONLY, date_time);", "msg, VIM_VERSION_LONG_ONLY, VIM_VERSION_DATE_ONLY);", "nor formats one"},
	}
	for _, s := range steps {
		if text, err = p.Literal(text, s.Old, s.New, s.What, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
