package p026

import "regexp"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	deadEnumRe = regexp.MustCompile(`(?m)^    CMD_SIZE,\n((?:    CMD_\w+(?: = \d+)?,\n)+)`)
	deadIDRe   = regexp.MustCompile(`(?m)^    CMD_\w+`)
	liveRowRe  = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{\(char_u \*\)"([^"]*)", \d+,`)
	tableRe    = regexp.MustCompile(`(?ms)^static struct cmdname cmdnames\[\] =\n\{\n(.*?)^\};\n`)
	labelRe    = regexp.MustCompile(`^([ \t]*)(case \w+:|default:)$`)
	caseRe     = regexp.MustCompile(`^[ \t]*case (\w+):$`)
	fallRe     = regexp.MustCompile(`^[ \t]*(break;|goto \w+;|return\b.*;|\{)$`)
	skipRe     = regexp.MustCompile(`\b(ea\.|eap->)skip\b`)
	vim9Re     = regexp.MustCompile(`\bvim9\b`)
)
