package togo

// hsnames.go is how the Haskell printer names what a function binds: every
// binding a name of its own, so that none shadows another -- GHC's -Wall
// counted 34,916 shadowings when each assignment rebound the C name
// (doc/HASKELL-IDIOMS.md, item 1).  A variable's first name is its C name;
// each later value of it -- an assignment, a block's parameter -- the name
// with a number (p, p1, p2; t1'1 where the C name ends in a digit).  And
// hsTidy takes out what a function binds and never uses.

import (
	"fmt"
	"regexp"
	"strings"
)

// topNames are the names no local may take: Haskell's and the runtime's
// (hsReserved), and the module's top level.
func (h *hgen) topNames() map[string]bool {
	if h.taken != nil {
		return h.taken
	}
	h.taken = map[string]bool{}
	for w := range hsReserved {
		h.taken[w] = true
	}
	for _, n := range h.names {
		h.taken[n] = true
	}
	for _, o := range h.objs {
		h.taken[o.name] = true
	}
	return h.taken
}

// nameVars gives each variable its first name, and says what each binding
// variable is where the function starts: a parameter its name, any other its
// type's zero, as C leaves it unset.
func (f *hfn) nameVars() {
	top := f.h.topNames()
	f.used = map[string]bool{"ed'": true, "fr'": true, "sret'": true}
	f.base = map[*lvar]string{}
	f.ver = map[*lvar]int{}
	f.cur = map[*lvar]string{}
	f.entryCur = map[*lvar]string{}
	// the C names that are free first, so that no number given later takes
	// another variable's name
	var clash []*lvar
	for _, v := range f.lf.vars {
		if top[v.name] || f.used[v.name] {
			clash = append(clash, v)
			continue
		}
		f.used[v.name] = true
		f.base[v] = v.name
	}
	for _, v := range clash {
		f.base[v] = v.name
		f.base[v] = f.fresh(v)
	}
	for _, v := range f.lf.vars {
		if !f.reg(v) {
			continue
		}
		if v.param {
			f.entryCur[v] = f.base[v]
		} else {
			f.entryCur[v] = hsZero(f.vtype(v))
		}
	}
}

// fresh is a new name for a value of v.
func (f *hfn) fresh(v *lvar) string {
	top := f.h.topNames()
	b := f.base[v]
	for {
		f.ver[v]++
		n := hsVerName(b, f.ver[v])
		if !f.used[n] && !top[n] {
			f.used[n] = true
			return n
		}
	}
}

// hsVerName is the nth later name of b.
func hsVerName(b string, n int) string {
	if c := b[len(b)-1]; c >= '0' && c <= '9' || c == '\'' {
		return fmt.Sprintf("%s'%d", strings.TrimSuffix(b, "'"), n)
	}
	return fmt.Sprintf("%s%d", b, n)
}

// valueOf is what v is in cur.
func (f *hfn) valueOf(cur map[*lvar]string, v *lvar) string {
	s, ok := cur[v]
	if !ok {
		f.no(nil, "%s read where it has no value", v.name)
	}
	return s
}

// hsAtomRe is a name, or a literal: a value a variable can be with no
// binding of its own.
var hsAtomRe = regexp.MustCompile(`^(?:[a-z_][A-Za-z0-9_']*|True|False|nullPtr|\((?:-?[0-9]+|[A-Z][A-Za-z0-9_']*|ch '(?:[^'\\]|\\.)') :: (?:Int|Word)[0-9]+\))$`)

func hsAtom(s string) bool { return hsAtomRe.MatchString(s) }

// hsTidy takes out of one function's text what it binds and never uses: a
// result bound and not read is `_ <-`, a let not read `let !_` (still
// forced, as C evaluated it), a parameter not read `_` or `_name`.  Every
// name a function binds is its own (nameVars), so a count of a name's
// occurrences is exact.
func hsTidy(src string) string {
	counts := map[string]int{}
	for _, t := range hsTokens(src) {
		counts[t]++
	}
	lines := strings.Split(src, "\n")
	// a result bound only to be the block's result: the action is the result
	var kept []string
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if i+1 < len(lines) {
			t := strings.TrimLeft(l, " ")
			ind := l[:len(l)-len(t)]
			if j := strings.Index(t, " <- "); strings.HasPrefix(t, "r'") && j > 0 && counts[t[:j]] == 2 && lines[i+1] == ind+"pure "+t[:j] {
				kept = append(kept, ind+t[j+4:])
				i++
				continue
			}
		}
		kept = append(kept, l)
	}
	lines = kept
	for i, l := range lines {
		if i == 1 {
			lines[i] = hsTidyHead(l, counts)
			continue
		}
		t := strings.TrimLeft(l, " ")
		ind := l[:len(l)-len(t)]
		switch {
		case strings.HasPrefix(t, "r'"):
			if j := strings.Index(t, " <- "); j > 0 && counts[t[:j]] == 1 {
				lines[i] = ind + "_" + t[j:]
			}
		case strings.HasPrefix(t, "let !"):
			if j := strings.Index(t, " = "); j > 5 && counts[t[5:j]] == 1 {
				lines[i] = ind + "let !_" + t[j:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

// hsTidyHead is a function's first line, `name ed' a b = ...`, its
// parameters that are never read written `_` (the editor) or `_a`.
func hsTidyHead(l string, counts map[string]int) string {
	eq := strings.Index(l, " =")
	if eq < 0 {
		return l
	}
	ws := strings.Fields(l[:eq])
	for k := 1; k < len(ws); k++ {
		if counts[ws[k]] != 1 {
			continue
		}
		if ws[k] == "ed'" {
			ws[k] = "_"
		} else if ws[k] != "sret'" {
			ws[k] = "_" + ws[k]
		}
	}
	return strings.Join(ws, " ") + l[eq:]
}

// hsTokens are the names in Haskell source, outside its strings and
// comments.
func hsTokens(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '"':
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '\'':
			// a character literal: a quote that no name ends in
			i++
			if i < len(src) && src[i] == '\\' {
				i++
			}
			i++
			if i < len(src) && src[i] == '\'' {
				i++
			}
		case c == '{' && i+1 < len(src) && src[i+1] == '-':
			if j := strings.Index(src[i:], "-}"); j >= 0 {
				i += j + 2
			} else {
				i = len(src)
			}
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_':
			j := i
			for j < len(src) && hsIdentChar(src[j]) {
				j++
			}
			out = append(out, src[i:j])
			i = j
		case c >= '0' && c <= '9':
			for i < len(src) && hsIdentChar(src[i]) {
				i++
			}
		default:
			i++
		}
	}
	return out
}

func hsIdentChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '\''
}
