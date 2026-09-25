package text

import "regexp"

// The negative-lookbehind rewrites: `(?<!\w)name(` is "a call of name, and not
// the tail of a longer identifier", and RE2 has no lookbehind of either sign.
//
// THE BYTE TEST IS WHAT REPLACES IT, which is the shape tools/README.md records
// for `noconv` -- `(?<![\w*])ptr\(` becomes "test the preceding byte".  It is
// exact here for the reason it is exact there: the excluded character is one
// byte BEFORE the match and is consumed by nothing, so reading it changes no
// span.  A capturing rewrite -- `([^\w])name\(` -- would consume that byte, and
// two matches sharing one would lose the second; that is why nobackup and
// noconv test bytes rather than capture, and why these do.
//
// `\w` is [0-9A-Za-z_] and nothing else: these files are ASCII, which the
// tree asserts elsewhere.
func IsWordByte(c byte) bool {
	return c == '_' ||
		(c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z')
}

// callsNotAfterWord returns the byte offsets of every `name(` whose preceding
// byte is not a word character.
func CallsNotAfterWord(text []byte, name string) [][]int {
	re := regexp.MustCompile(regexp.QuoteMeta(name) + `\(`)
	var Out [][]int
	for _, loc := range re.FindAllIndex(text, -1) {
		if loc[0] > 0 && IsWordByte(text[loc[0]-1]) {
			continue
		}
		Out = append(Out, loc)
	}
	return Out
}

// subNotAfterWord rewrites every such call to `repl(` and returns the new text
// and the number of substitutions -- which is Python's re.subn of the same
// pattern.
func SubNotAfterWord(text []byte, name, repl string) ([]byte, int) {
	locs := CallsNotAfterWord(text, name)
	if len(locs) == 0 {
		return text, 0
	}
	var Out []byte
	last := 0
	for _, loc := range locs {
		Out = append(Out, text[last:loc[0]]...)
		Out = append(Out, repl...)
		Out = append(Out, '(')
		last = loc[1]
	}
	Out = append(Out, text[last:]...)
	return Out, len(locs)
}
