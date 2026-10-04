package p087

// Whim phase 87 (formerly 167) -- a key code has a name.  See GOAL.md.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim87", Edit) }

// keyCode is vim's TERMCAP2KEY(a, b) as the preprocessor left it, in the
// canonical spelling: two constant operands, a character or a name.
var keyCode = regexp.MustCompile(`\(-\(\(('[^'\\]'|[A-Z][A-Z0-9_]*)\) \+ \(\(int\)\(('[^'\\]'|[A-Z][A-Z0-9_]*)\) << 8\)\)\)`)

// tableRow is one row of key_names_table: the code and its name.
var tableRow = regexp.MustCompile(`\{TRUE, (\(-\(\([^{}]*?<< 8\)\)\)), \{\(char_u \*\)\("([^"]+)"\)`)

var ident = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)

// Edit gives every constant key code a name, an enumerator defined once
// before its first use, and writes the name wherever the code was spelled
// out.  The name is vim's where the file still says it: K_X for
// TERMCAP2KEY(KS_EXTRA, KE_X), and K_ plus the shortest name
// key_names_table gives the code.  A code computed from variables is not a
// constant and stays as it is.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, Step6): the codes and the names are
// read off the C view, as the text read them; each code spelled out is a
// node of the core whose C is the code, replaced by a use of its
// enumerator (Editor.ReplaceByUse), and the enumerators are one FRAG unit
// before the top-level form that holds the first.  History keeps the text
// version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("keynames", e, w)
	text, err := e.Graph().C()
	if err != nil {
		return err
	}
	cut := bytes.Index(text, []byte("\n#include "))
	if cut < 0 {
		v.Die("no #include: the core/host line is not where this phase expects it")
		return v.Done()
	}
	core := text[:cut]

	// the names the table gives, shortest first
	byTable := map[string][]string{}
	for _, m := range tableRow.FindAllSubmatch(core, -1) {
		byTable[string(m[1])] = append(byTable[string(m[1])], string(m[2]))
	}
	taken := map[string]bool{}
	for _, m := range ident.FindAll(text, -1) {
		taken[string(m)] = true
	}
	codes := map[string]string{} // spelling -> name
	var order []string
	named := map[string]bool{}
	for _, m := range keyCode.FindAllSubmatch(core, -1) {
		code := string(m[0])
		if _, ok := codes[code]; ok {
			continue
		}
		a, b := string(m[1]), string(m[2])
		var cands []string
		if a == "KS_EXTRA" && strings.HasPrefix(b, "KE_") {
			cands = append(cands, "K_"+strings.TrimPrefix(b, "KE_"))
		}
		names := append([]string{}, byTable[code]...)
		sort.SliceStable(names, func(i, j int) bool { return len(names[i]) < len(names[j]) })
		for _, n := range names {
			cands = append(cands, "K_"+keyWord(n))
		}
		cands = append(cands, "K_TC_"+charWord(a)+"_"+charWord(b))
		name := ""
		for _, c := range cands {
			if !taken[c] && !named[c] {
				name = c
				break
			}
		}
		if name == "" {
			v.Die("no free name for the key code %s", code)
			return v.Done()
		}
		named[name] = true
		codes[code] = name
		order = append(order, code)
	}
	if len(order) == 0 {
		v.Die("no constant key code in the core")
		return v.Done()
	}

	// the codes spelled out: the nodes of the core whose C is one, in the
	// file's order, none inside another
	sites := map[string][]*graph.Node{}
	var first *graph.Node
	nsites := 0
	coreForms := e.Core()
	for _, f := range coreForms {
		graph.Walk(f, func(x *graph.Node) bool {
			if !x.Is("paren") || len(x.Kids) != 2 || !x.Kids[1].Is("-") || len(x.Kids[1].Kids) != 2 {
				return true
			}
			c, err := graph.ExprText(x)
			if err != nil || codes[c] == "" {
				return true
			}
			if first == nil {
				first = f
			}
			sites[c] = append(sites[c], x)
			nsites++
			return false
		})
	}
	// where the definitions go: before the top-level form that holds the
	// first use, which must come after every KS_ and KE_ name they use
	at := slices.Index(coreForms, first)
	for _, code := range order {
		if len(sites[code]) == 0 {
			v.Die("the key code %s is spelled out in no expression of the core", code)
			return v.Done()
		}
		x := sites[code][0]
		bad := ""
		graph.Walk(x, func(y *graph.Node) bool {
			if d := y.Ref(); d != nil && !y.IsList() {
				if i := slices.Index(coreForms, e.TopForm(d)); i < 0 || i >= at {
					bad = y.Atom
				}
			}
			return true
		})
		if bad != "" {
			v.Die("%s is not defined before the first key code", bad)
			return v.Done()
		}
	}
	// the text's count: every spelling from the first use to the core's end
	body := string(text[bytes.Index(text, []byte(order[0])):cut])
	want := 0
	for _, code := range order {
		want += strings.Count(body, code)
		body = strings.ReplaceAll(body, code, codes[code])
	}
	if nsites != want {
		v.Die("%d key codes spelled out in the core's expressions, %d in its text", nsites, want)
		return v.Done()
	}

	var defs strings.Builder
	for _, code := range order {
		fmt.Fprintf(&defs, "enum { %s = %s };\n", codes[code], code)
	}
	v.Muted(func(v *graph.Verbs) { v.FragAt(e.SpotBefore(first), defs.String(), "the key codes' enumerators") })
	if v.Failed() {
		return v.Done()
	}
	for _, code := range order {
		if err := e.ReplaceByUse(sites[code], codes[code]); err != nil {
			v.Die("%s: %v", codes[code], err)
			return v.Done()
		}
	}
	out, err := e.Graph().C()
	if err != nil {
		return err
	}
	if rest := keyCode.FindAll(out[:bytes.Index(out, []byte("\n#include "))], -1); len(rest) != len(order) {
		v.Die("%d constant key codes are left spelled out outside their definitions", len(rest)-len(order))
		return v.Done()
	}
	fromTable, fromKE, mech := 0, 0, 0
	for _, code := range order {
		switch n := codes[code]; {
		case strings.HasPrefix(n, "K_TC_"):
			mech++
		case len(byTable[code]) > 0 && !strings.HasPrefix(keyCode.FindStringSubmatch(code)[2], "KE_"):
			fromTable++
		default:
			fromKE++
		}
	}
	v.Say(fmt.Sprintf("%d key codes named at %d sites: %d from key_names_table, %d as K_X for KE_X, %d mechanically",
		len(order), nsites, fromTable, fromKE, mech))
	return v.Done()
}

// keyWord is a key_names_table name as an identifier: upper case, with every
// character that cannot be in one spelled as _.
func keyWord(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// charWord is one operand of a code as part of an identifier: a letter or
// digit as itself, a KS_ name without its prefix, a punctuation mark by name.
func charWord(op string) string {
	if op[0] != '\'' {
		return strings.TrimPrefix(op, "KS_")
	}
	c := op[1]
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
		return string(c)
	}
	names := map[byte]string{'#': "HASH", '%': "PCT", '*': "STAR", '&': "AMP", '@': "AT", '!': "BANG",
		'<': "LT", '>': "GT", ';': "SEMI", ':': "COLON", '+': "PLUS", '-': "MINUS", '=': "EQ", '|': "BAR",
		'/': "SLASH", '.': "DOT", ',': "COMMA", '?': "QUEST", '^': "CARET", '~': "TILDE", '$': "DOLLAR"}
	if n, ok := names[c]; ok {
		return n
	}
	return fmt.Sprintf("X%02X", c)
}
