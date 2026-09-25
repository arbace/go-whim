package p167

// Whim phase 167 -- a key code has a name.  See GOAL.md.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/sweep"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim167", Edit) }

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
// key_names_table gives the code.  A code named nowhere gets a mechanical
// name from its two characters, K_TC_<a>_<b>, rather than one remembered.
// A code computed from variables is not a constant and stays as it is.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "keynames", W: w}
	cut := bytes.Index(text, []byte("\n#include "))
	if cut < 0 {
		return nil, p.Die("no #include: the core/host line is not where this phase expects it")
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
			return nil, p.Die("no free name for the key code %s", code)
		}
		named[name] = true
		codes[code] = name
		order = append(order, code)
	}
	if len(order) == 0 {
		return nil, p.Die("no constant key code in the core")
	}

	// where the definitions go: before the top-level declaration that holds
	// the first use, which must come after every KS_ and KE_ name they use
	const path = "whim-vim.c"
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: text},
	})
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	first := keyCode.FindIndex(core)[0]
	at := -1
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil {
			continue
		}
		if a, z := sweep.Span(ed, path, text); a <= first && first < z {
			at = a
			break
		}
	}
	if at < 0 {
		return nil, p.Die("the first key code is in no top-level declaration")
	}
	for _, code := range order {
		for _, m := range keyCode.FindStringSubmatch(code)[1:] {
			if m[0] == '\'' {
				continue
			}
			def := regexp.MustCompile(`\b` + m + ` = `).FindIndex(text)
			if def == nil || def[0] > at {
				return nil, p.Die("%s is not defined before the first key code", m)
			}
		}
	}

	var defs strings.Builder
	for _, code := range order {
		fmt.Fprintf(&defs, "enum { %s = %s };\n", codes[code], code)
	}
	defs.WriteString("\n")
	sites := 0
	body := string(text[at:cut])
	for _, code := range order {
		sites += strings.Count(body, code)
		body = strings.ReplaceAll(body, code, codes[code])
	}
	out := append([]byte{}, text[:at]...)
	out = append(out, defs.String()...)
	out = append(out, body...)
	out = append(out, text[cut:]...)
	if rest := keyCode.FindAll(out[:bytes.Index(out, []byte("\n#include "))], -1); len(rest) != len(order) {
		return nil, p.Die("%d constant key codes are left spelled out outside their definitions", len(rest)-len(order))
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
	p.Say(fmt.Sprintf("%d key codes named at %d sites: %d from key_names_table, %d as K_X for KE_X, %d mechanically",
		len(order), sites, fromTable, fromKE, mech))
	return out, nil
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
