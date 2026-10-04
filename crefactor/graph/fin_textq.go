package graph

import (
	"strings"
)

// TEXTQ without the whole file (doc/GRAPH-MIGRATION.md, *Fin as built*): a
// text program's question of the whole file -- a word's mentions, a
// regular expression's matches, the lines saying a name and the functions
// they are in -- asked of the C view of only the top-level forms that can
// answer it.  Every token the C view prints is an atom of its form, so a
// match that needs a word within one token is in a form some atom of which
// holds that word, and the answer on those forms is the whole file's.  It
// is the caller's to name, for each question, a word every match holds
// within one token.

// isWordRun: an identifier's, a number's or a keyword's run of word
// characters.
func isWordRun(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

// FormsWith are, for each word, the top-level forms some atom of which
// HOLDS it, in the file's order: one walk of the forms for all of them.  An
// atom holds a word when it is that word, or, not a run of word characters
// -- a literal, an operator, a head -- contains it: a word a string
// mentions is held by the string, and `mfp` is not held by `ml_mfp`.
func (e *Editor) FormsWith(words ...string) map[string][]*Node {
	out := map[string][]*Node{}
	exact := map[string]bool{}
	for _, w := range words {
		exact[w] = true
	}
	for _, f := range e.g.Forms {
		Walk(f, func(x *Node) bool {
			if x.list {
				return true
			}
			if isWordRun(x.Atom) {
				if exact[x.Atom] {
					out[x.Atom] = addForm(out[x.Atom], f)
				}
				return true
			}
			for _, w := range words {
				if strings.Contains(x.Atom, w) {
					out[w] = addForm(out[w], f)
				}
			}
			return true
		})
	}
	return out
}

func addForm(fs []*Node, f *Node) []*Node {
	if len(fs) > 0 && fs[len(fs)-1] == f {
		return fs
	}
	return append(fs, f)
}

// FormsWhere are the top-level forms some atom of which satisfies keep, in
// the file's order.
func (e *Editor) FormsWhere(keep func(atom string) bool) []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		hit := false
		Walk(f, func(x *Node) bool {
			if !hit && !x.list && keep(x.Atom) {
				hit = true
			}
			return !hit
		})
		if hit {
			out = append(out, f)
		}
	}
	return out
}

// Quoted are the texts of the file's forms that may hold a quote, in order,
// as the C view prints them: each string and character literal, and each
// text a form carries verbatim -- an include's operand, a directive, a
// macro's invocation, a line printed verbatim -- unquoted, so that the
// literals a text program found in the file are the ones it finds in these.
func (e *Editor) Quoted() []string {
	var out []string
	for _, f := range e.g.Forms {
		Walk(f, func(x *Node) bool {
			if !x.list {
				return true
			}
			switch x.Head() {
			case "include", "directive", "macro", "macro-decl", "verbatim":
				if len(x.Kids) > 1 && !x.Kids[1].list {
					out = append(out, finUnquote(x.Kids[1].Atom))
				}
				return false
			}
			for _, k := range x.Kids {
				if !k.list && isLiteralAtom(k.Atom) {
					out = append(out, k.Atom)
				}
			}
			return true
		})
	}
	return out
}

// finUnquote is a verbatim text as the C view prints it (clisp's printer):
// the quotes taken off, `\n` a newline and any other escaped byte itself.
func finUnquote(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' {
				b.WriteByte('\n')
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// isLiteralAtom: a string or character literal, its encoding prefix
// (L, u, U, u8) included.
func isLiteralAtom(s string) bool {
	s = strings.TrimLeft(s, "LuU8")
	return len(s) >= 2 && (s[0] == '"' || s[0] == '\'')
}
