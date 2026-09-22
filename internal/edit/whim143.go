package edit

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
)

func init() { register("whim143", Whim143) }

const (
	w143DelimHead = "                case 't':\ndelimiter_atom:\n"
	w143Tail      = "\n                    break;\n"
	w143Magic     = "((int)('[') - 256)"
)

// W143Call is what replaces a jump to delimiter_atom, and the block itself, at
// an indentation: the helper's node, or a return of the NULL it gives after
// its error message.
func W143Call(indent string) string {
	return indent + "ret = regatom_delim(c, delim_nl, flagp);\n" +
		indent + "if (ret == nullptr)\n" +
		indent + "{\n" +
		indent + "    return nullptr;\n" +
		indent + "}\n" +
		indent + "break;\n"
}

// W143Helper is regatom_delim(): the delimiter block, taken out of regatom()
// verbatim and dedented, with the node it makes returned.
func W143Helper(block string) string {
	var b strings.Builder
	b.WriteString("    static char_u *\nregatom_delim(int c, int delim_nl, int *flagp)\n{\n    char_u      *ret;\n\n")
	for _, l := range strings.Split(strings.TrimSuffix(block, "\n"), "\n") {
		if l == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(strings.TrimPrefix(l, strings.Repeat(" ", 20)) + "\n")
	}
	b.WriteString("    return ret;\n}\n\n")
	return b.String()
}

// Whim143 takes the three jumps out of regatom().
//
// regatom() jumped into the middle of other cases three ways (tx/FINDINGS.md,
// 11): `\_%)` to the delimiter atom inside the `\%` case's own switch, and
// `\%>` there too when no digit follows; `\_[` to the collection that starts
// the `[` case; and `.` followed by a composing character to the multibyte
// node in the default case.  Go cannot jump into a case or a block.
//
//   - the delimiter atom's block becomes regatom_delim(), verbatim, and its
//     case and both jumps call it: regnode() never returns NULL, so a NULL is
//     the block's own error return, passed on;
//   - the multibyte node's three statements are written where the jump was;
//   - the switch dispatches on sw, not c, in a loop that runs once: the `\_[`
//     path sets sw to the `[` case and continues, and c is '[' as the jump
//     left it.
func Whim143(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "regatom", w: w}
	blank := cutil.Blank(text)
	a, z, ok := cutil.FindDefinition(text, blank, "regatom")
	if !ok {
		return nil, p.die("regatom is not defined")
	}
	s := string(text[a:z])

	// 1. the delimiter atom
	h := strings.Index(s, w143DelimHead)
	if h < 0 || strings.Count(s, "delimiter_atom:") != 1 {
		return nil, p.die("the delimiter_atom label is not where this phase expects it")
	}
	bo := h + len(w143DelimHead)
	if !strings.HasPrefix(s[bo:], "                    {\n") {
		return nil, p.die("delimiter_atom does not label a block")
	}
	bs := cutil.Blank([]byte(s))
	bc := cutil.Match(bs, bo+strings.Index(s[bo:], "{"))
	if bc < 0 || !strings.HasPrefix(s[bc+1:], w143Tail) {
		return nil, p.die("the delimiter block does not end in a break")
	}
	inner := s[bo+len("                    {\n") : bc]
	inner = strings.TrimSuffix(inner, "                    ")
	helper := W143Helper(inner)
	s = s[:h] + "                case 't':\n" + W143Call("                    ") + s[bc+1+len(w143Tail):]
	n := 0
	for _, ind := range []string{"                ", "                        "} {
		g := "\n" + ind + "goto delimiter_atom;\n"
		if strings.Count(s, g) == 1 {
			s = strings.Replace(s, g, "\n"+W143Call(ind), 1)
			n++
		}
	}
	if n != 2 || strings.Contains(s, "delimiter_atom") {
		return nil, p.die("%d jumps to delimiter_atom at the expected places, and this phase was written against 2", n)
	}
	p.say("the delimiter atom is regatom_delim(), called from its case and from the two jumps")

	// 2. the multibyte node
	mbGoto := "            c = getchr();\n            goto do_multibyte;\n"
	mbLabel := "do_multibyte:\n                ret = regnode(MULTIBYTECODE);\n                regmbc(c);\n                *flagp |= HASWIDTH | SIMPLE;\n                break;\n"
	if strings.Count(s, mbGoto) != 1 || strings.Count(s, mbLabel) != 1 {
		return nil, p.die("the do_multibyte jump or label is not what this phase expects")
	}
	s = strings.Replace(s, mbGoto, "            c = getchr();\n            ret = regnode(MULTIBYTECODE);\n            regmbc(c);\n            *flagp |= HASWIDTH | SIMPLE;\n            break;\n", 1)
	s = strings.Replace(s, mbLabel, strings.TrimPrefix(mbLabel, "do_multibyte:\n"), 1)
	p.say("`.` with a composing character makes its multibyte node where it was")

	// 3. the collection
	colGoto := "            goto collection;\n"
	colLabel := "      case  " + w143Magic + " :\ncollection:\n"
	if strings.Count(s, colGoto) != 1 || strings.Count(s, colLabel) != 1 {
		return nil, p.die("the collection jump or label is not what this phase expects")
	}
	s = strings.Replace(s, colGoto, "            sw = "+w143Magic+";\n            continue;\n", 1)
	s = strings.Replace(s, colLabel, "      case  "+w143Magic+" :\n", 1)
	head := "    c = getchr();\n    switch (c)\n    {\n"
	tail := "\n    }\n\n    return ret;\n}"
	hi := strings.Index(s, head)
	if hi < 0 || !strings.HasSuffix(strings.TrimRight(s, "\n"), strings.TrimPrefix(tail, "\n")) {
		return nil, p.die("regatom()'s switch is not where this phase expects it")
	}
	ti := strings.LastIndex(s, tail)
	body := s[hi+len(head) : ti]
	var ib strings.Builder
	for _, l := range strings.Split(body, "\n") {
		if l == "" {
			ib.WriteString("\n")
		} else {
			ib.WriteString("    " + l + "\n")
		}
	}
	s = s[:hi] + "    c = getchr();\n    sw = c;\n    for (;;)\n    {\n        switch (sw)\n        {\n" +
		strings.TrimSuffix(ib.String(), "\n") + "\n        }\n        break;\n    }\n\n    return ret;\n}" + s[ti+len(tail):]
	decl := "    int             c;\n"
	if strings.Count(s, decl) != 1 {
		return nil, p.die("regatom()'s c is not declared where this phase expects")
	}
	s = strings.Replace(s, decl, decl+"    int             sw;\n", 1)
	if strings.Contains(s, "goto ") || strings.Contains(s, "collection:") {
		return nil, p.die("regatom() still jumps")
	}
	p.say(fmt.Sprintf("regatom() dispatches on sw in a loop that runs once, and `\\_[` dispatches again to the collection: no goto left"))

	// the helper goes before regatom(), whose head starts the definition
	out := string(text[:a]) + helper + s + string(text[z:])
	return []byte(out), nil
}
