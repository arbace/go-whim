package p119

import "regexp"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	pidFieldRe    = regexp.MustCompile(`^\s*char_u\s+b0_pid\[\d+\];$`)
	getPidProtoRe = regexp.MustCompile(`^static [\w *]+mch_get_pid\(.*\);$`)
	reraiseRe     = regexp.MustCompile(`^(\s*)kill\(getpid\(\), (\w+)\);$`)
	retTypeRe     = regexp.MustCompile(`^    [\w *]+$`)
	pidWriteRe    = regexp.MustCompile(`^\s*long_to_char\(mch_get_pid\(\), (\w+)->b0_pid\);$`)
)
