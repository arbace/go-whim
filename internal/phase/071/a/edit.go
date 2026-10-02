package p071a

// Whim phase 71a (formerly 143) -- regatom() has no goto.  See GOAL.md.
//
// regatom() jumped into other cases three ways (internal/gen/FINDINGS.md, 11).  The
// delimiter atom becomes regatom_delim(), the multibyte node is written where
// its jump was, and the switch dispatches on sw in a loop that runs once, so
// the collection is reached by dispatching again.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim71a", Edit) }

const (
	w71aDelimHead = "            case 't':\n            delimiter_atom:\n"
	w71aTail      = "\n                break;\n"
	w71aMagic     = "((int)('[') - 256)"
)

// W71aCall is what replaces a jump to delimiter_atom, and the block itself, at
// an indentation: the helper's node, or a return of the NULL it gives after
// its error message.
func W71aCall(indent string) string {
	return indent + "ret = regatom_delim(c, delim_nl, flagp);\n" +
		indent + "if (ret == nullptr)\n" +
		indent + "{\n" +
		indent + "    return nullptr;\n" +
		indent + "}\n" +
		indent + "break;\n"
}

// W71aHelper is regatom_delim(): the delimiter block, taken Out of regatom()
// verbatim, with the node it makes returned.  Its indentation is the canonical
// print's.
func W71aHelper(block string) string {
	return "    static char_u *\nregatom_delim(int c, int delim_nl, int *flagp)\n{\n    char_u      *ret;\n\n" +
		block + "    return ret;\n}\n\n"
}

// Whim71a takes the three jumps Out of regatom().
//
// regatom() jumped into the middle of other cases three ways (internal/gen/FINDINGS.md,
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
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "regatom", W: w}
	blank := edit.Blank(text)
	a, z, ok := edit.FindDefinition(text, blank, "regatom")
	if !ok {
		return nil, p.Die("regatom is not defined")
	}
	s := string(text[a:z])

	// 1. the delimiter atom
	h := strings.Index(s, w71aDelimHead)
	if h < 0 || strings.Count(s, "delimiter_atom:") != 1 {
		return nil, p.Die("the delimiter_atom label is not where this phase expects it")
	}
	bo := h + len(w71aDelimHead)
	if !strings.HasPrefix(s[bo:], "                {\n") {
		return nil, p.Die("delimiter_atom does not label a block")
	}
	bs := edit.Blank([]byte(s))
	bc := edit.Match(bs, bo+strings.Index(s[bo:], "{"))
	if bc < 0 || !strings.HasPrefix(s[bc+1:], w71aTail) {
		return nil, p.Die("the delimiter block does not end in a break")
	}
	Inner := s[bo+len("                {\n") : bc]
	helper := W71aHelper(Inner)
	s = s[:h] + "            case 't':\n" + W71aCall("                ") + s[bc+1+len(w71aTail):]
	n := 0
	for _, ind := range []string{"                ", "                    "} {
		g := "\n" + ind + "goto delimiter_atom;\n"
		if strings.Count(s, g) == 1 {
			s = strings.Replace(s, g, "\n"+W71aCall(ind), 1)
			n++
		}
	}
	if n != 2 || strings.Contains(s, "delimiter_atom") {
		return nil, p.Die("%d jumps to delimiter_atom at the expected places, and this phase was written against 2", n)
	}
	p.Say("the delimiter atom is regatom_delim(), called from its case and from the two jumps")

	// 2. the multibyte node
	mbGoto := "            c = getchr();\n            goto do_multibyte;\n"
	mbLabel := "            do_multibyte:\n                ret = regnode(MULTIBYTECODE);\n                regmbc(c);\n                *flagp |= HASWIDTH | SIMPLE;\n                break;\n"
	if strings.Count(s, mbGoto) != 1 || strings.Count(s, mbLabel) != 1 {
		return nil, p.Die("the do_multibyte jump or label is not what this phase expects")
	}
	s = strings.Replace(s, mbGoto, "            c = getchr();\n            ret = regnode(MULTIBYTECODE);\n            regmbc(c);\n            *flagp |= HASWIDTH | SIMPLE;\n            break;\n", 1)
	s = strings.Replace(s, mbLabel, strings.TrimPrefix(mbLabel, "            do_multibyte:\n"), 1)
	p.Say("`.` with a composing character makes its multibyte node where it was")

	// 3. the collection
	colGoto := "            goto collection;\n"
	colLabel := "    case " + w71aMagic + ":\n    collection:\n"
	if strings.Count(s, colGoto) != 1 || strings.Count(s, colLabel) != 1 {
		return nil, p.Die("the collection jump or label is not what this phase expects")
	}
	s = strings.Replace(s, colGoto, "            sw = "+w71aMagic+";\n            continue;\n", 1)
	s = strings.Replace(s, colLabel, "    case "+w71aMagic+":\n", 1)
	head := "    c = getchr();\n    switch (c)\n    {\n"
	tail := "\n    }\n    return ret;\n}"
	hi := strings.Index(s, head)
	if hi < 0 || !strings.HasSuffix(strings.TrimRight(s, "\n"), strings.TrimPrefix(tail, "\n")) {
		return nil, p.Die("regatom()'s switch is not where this phase expects it")
	}
	ti := strings.LastIndex(s, tail)
	Body := s[hi+len(head) : ti]
	s = s[:hi] + "    c = getchr();\n    sw = c;\n    for (;;)\n    {\n        switch (sw)\n        {\n" +
		Body + "\n        }\n        break;\n    }\n    return ret;\n}" + s[ti+len(tail):]
	decl := "    int c;\n"
	if strings.Count(s, decl) != 1 {
		return nil, p.Die("regatom()'s c is not declared where this phase expects")
	}
	s = strings.Replace(s, decl, decl+"    int sw;\n", 1)
	if strings.Contains(s, "goto ") || strings.Contains(s, "collection:") {
		return nil, p.Die("regatom() still jumps")
	}
	p.Say("regatom() dispatches on sw in a loop that runs once, and `\\_[` dispatches again to the collection: no goto left")

	// the helper goes before regatom(), whose head starts the definition
	Out := string(text[:a]) + helper + s + string(text[z:])
	return []byte(Out), nil
}
