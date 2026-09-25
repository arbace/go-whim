package p080

import "regexp"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	tableRe  = regexp.MustCompile(`(?ms)^static struct cmdname cmdnames\[\] =\n\{\n(.*?)^\};\n`)
	rowRe    = regexp.MustCompile(`^    \[CMD_(\w+)\] = \{\(char_u \*\)"([^"]*)", sizeof\("([^"]*)"\) - 1, *(\w+) *, \(long_u\)\(.*\), ADDR_\w+\},$`)
	enumRe   = regexp.MustCompile(`(?ms)^enum CMD_index\n\{\n(.*?)^    CMD_SIZE,\n\};\n`)
	idRe     = regexp.MustCompile(`(?m)^    CMD_(\w+),$`)
	idx1Re   = regexp.MustCompile(`(?s)static const unsigned short cmdidxs1\[26\] =\n\{\n(.*?)\};`)
	idx2Re   = regexp.MustCompile(`(?s)static const unsigned char cmdidxs2\[26\]\[26\] =\n\{\n(.*?)\n\};`)
	countRe  = regexp.MustCompile(`static const int command_count = (\d+);`)
	charsRe  = regexp.MustCompile(`vim_strchr\(\(char_u \*\)"([^"]*)", \*p\) != NULL\)\n`)
	bannerRe = regexp.MustCompile(`(?ms)^static const unsigned short cmdidxs1\[26\] =\n.*?^static const int command_count = \d+;\n`)
	numRe    = regexp.MustCompile(`\d+`)
	labelRe  = regexp.MustCompile(`^([ \t]*)(case \w+:|default:)$`)
	caseRe   = regexp.MustCompile(`^[ \t]*case (\w+):$`)
	fallRe   = regexp.MustCompile(`^[ \t]*(break;|goto \w+;|return\b.*;|\{)$`)
	wordRe   = regexp.MustCompile(`^[A-Za-z]+`)
	skipRe   = regexp.MustCompile(`\b(ea\.|eap->)skip\b`)
	vim9Re   = regexp.MustCompile(`\bvim9\b`)
)
