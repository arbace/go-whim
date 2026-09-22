package edit

import (
	"io"
)

func init() { register("whim142", Whim142) }

// Whim142 takes the build's date and time out of the version.
//
// init_longVersion() put __DATE__ " " __TIME__ into the version line that
// mainerr() prints above a command-line error: "VIM - Vi IMproved 9.2 (2026
// Feb 14, compiled <date> <time>)".  So the core's text was a function of when
// it was compiled, and two builds of the same source differed unless
// SOURCE_DATE_EPOCH pinned them; the Go transpilation had no such clock and
// wrote in the constant the pinned build produces (tx/FINDINGS.md, 13).  The
// version is now its name and its release date, "(2026 Feb 14)", and the
// binary is a function of the source alone.
func Whim142(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "datetime", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{"    char *date_time = __DATE__ \" \" __TIME__;\n", "", "the version reads no build date or time"},
		{"char *msg = _(\"%s (%s, compiled %s)\");", "char *msg = _(\"%s (%s)\");", "it is the name and the release date"},
		{"        + sizeof(VIM_VERSION_DATE_ONLY) - 1\n        + musl_strlen(date_time);\n", "        + sizeof(VIM_VERSION_DATE_ONLY) - 1;\n", "and its length counts no date"},
		{"msg, VIM_VERSION_LONG_ONLY, VIM_VERSION_DATE_ONLY, date_time);", "msg, VIM_VERSION_LONG_ONLY, VIM_VERSION_DATE_ONLY);", "nor formats one"},
	}
	for _, s := range steps {
		if text, err = p.literal(text, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	if n := p.mentions(text, "__DATE__") + p.mentions(text, "__TIME__"); n != 0 {
		return nil, p.die("__DATE__ or __TIME__ is still named %d times", n)
	}
	return text, nil
}
