package p127

import "regexp"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	dlTextRe   = regexp.MustCompile(`\.dl_text\s*=[^=]`)
	interiorRe = regexp.MustCompile(`\(char_u? \*\)dp[a-z_]* *\+`)
	mlFlagsRe  = regexp.MustCompile(`ml_flags \|=`)
)
