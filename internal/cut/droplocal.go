package cut

import (
	"bytes"
	"fmt"
	"regexp"
)

// DropLocal removes a buffer-local option field and its PLUMBING -- the
// declaration, the assignments, the free, and the get_varp case that hands its
// address out -- and refuses when anything else still names it.
//
// The refusal is the point.  Plumbing is what the phase may remove on its own;
// a mention that is left over is a READER, and a reader has to be dealt with
// by the phase before the field can go.  It also refuses when it found fewer
// than three sites, because the field, an initialiser and a get_varp case are
// the minimum shape -- fewer means the shape has moved and a silent partial
// cut would leave the struct and its users disagreeing.
func DropLocal(text []byte, bvar string) (out []byte, n int, err error) {
	if !bytes.Contains(text, []byte(bvar)) {
		return nil, 0, fmt.Errorf("droplocal: there is no %s here", bvar)
	}
	q := regexp.QuoteMeta(bvar)
	// Each pattern matches whole lines -- one, or for the two get_varp
	// shapes two -- and the last holds the field's name, so it runs on the
	// lines around the name and not on the whole text (inLines).
	pats := []struct {
		before int
		re     string
	}{
		{0, `(?m)^[ \t]*(?:char_u[ \t]*\*|int[ \t]+|long[ \t]+)` + q + `;\n`},
		{0, `(?m)^[ \t]*buf->` + q + ` = [^\n]*;\n`},
		{0, `(?m)^[ \t]*curbuf->` + q + ` = -1;\n`},
		{0, `(?m)^[ \t]*(?:check|clear)_string_option\(&buf->` + q + `\);\n`},
		{1, `(?m)^[ \t]*case[^\n]*\n[ \t]*return \(char_u \*\)&\(curbuf->` + q + `\);\n`},
		{1, `(?m)^[ \t]*case[^\n]*\n[ \t]*return [^\n]*curbuf->` + q +
			`[^\n]*\? \(char_u \*\)&\(curbuf->` + q + `\) : p->var;\n`},
	}
	for _, p := range pats {
		ms := inLines(regexp.MustCompile(p.re), text, []byte(bvar), p.before)
		n += len(ms)
		if len(ms) == 0 {
			continue
		}
		out := make([]byte, 0, len(text))
		last := 0
		for _, m := range ms {
			out = append(out, text[last:m[0]]...)
			last = m[1]
		}
		text = append(out, text[last:]...)
	}
	if n < 3 {
		return nil, 0, fmt.Errorf("droplocal: %s: only %d plumbing sites, expected at "+
			"least the field, an initialiser and a get_varp case -- the shape has moved",
			bvar, n)
	}
	left := 0
	if bytes.Contains(text, []byte(bvar)) {
		left = len(regexp.MustCompile(`\b`+q+`\b`).FindAll(text, -1))
	}
	if left > 0 {
		return nil, 0, fmt.Errorf("droplocal: %s still has %d mentions after the plumbing "+
			"went -- those are readers, and the phase has to deal with them before the "+
			"field can go", bvar, left)
	}
	return text, n, nil
}

// inLines is the matches of re in text, as FindAllIndex gives them, for a re
// that matches only a whole run of at most before+1 lines whose last holds
// lit -- found on those lines alone.  The lines holding lit and the before
// lines above each make runs, merged where they meet; a match lies in one
// run, a run starts at the start of a line, and nothing between the runs can
// match, so scanning the runs one by one finds what scanning the text finds,
// in the same order.  It is DropLocal's regexps on a few lines instead of
// the whole multi-megabyte text, which was 44 s of a build's CPU.
func inLines(re *regexp.Regexp, text, lit []byte, before int) [][2]int {
	var runs [][2]int
	for pos := 0; ; {
		rel := bytes.Index(text[pos:], lit)
		if rel < 0 {
			break
		}
		i := pos + rel
		a := bytes.LastIndexByte(text[:i], '\n') + 1
		for k := 0; k < before && a > 0; k++ {
			a = bytes.LastIndexByte(text[:a-1], '\n') + 1
		}
		z := len(text)
		if e := bytes.IndexByte(text[i:], '\n'); e >= 0 {
			z = i + e + 1
		}
		if len(runs) > 0 && a <= runs[len(runs)-1][1] {
			runs[len(runs)-1][1] = max(runs[len(runs)-1][1], z)
		} else {
			runs = append(runs, [2]int{a, z})
		}
		pos = z
		if pos >= len(text) {
			break
		}
	}
	var out [][2]int
	for _, r := range runs {
		for _, m := range re.FindAllIndex(text[r[0]:r[1]], -1) {
			out = append(out, [2]int{r[0] + m[0], r[0] + m[1]})
		}
	}
	return out
}
