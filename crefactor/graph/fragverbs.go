package graph

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// FRAG's VERBS: the text verbs that put C text in -- Body, Literal, Sub's
// replacement, Splice -- on the graph, each act's matches made in ONE
// synthesized import (SpliceC), counted and reported as the B0 verbs are.
//
//	edit.E                               Verbs
//	Body(fn, body, what)                 BodyC(fn, src, what)
//	Literal(old, new, n, what)           LiteralC(old, new, n, what): runs of whole items
//	Sub(re, repl, n, what)               ReplaceC(pat, src, n, what), ReplaceAtC
//	Splice(from, through, with, what)    SpliceRunC(from, through, src, what)
//	(an insertion by Literal)            BeforeC, AfterC, TopBeforeC, TopAfterC
//
// A pattern's bindings are the fragment's holes: `$x` in the C for `?x`.

// BodyC replaces the whole body of the function name by the items src.
func (v *Verbs) BodyC(name, src, what string) {
	if v.Err != nil {
		return
	}
	d := v.e.Defn(name)
	if d == nil {
		v.Die("%s is not defined at file scope", name)
		return
	}
	v.spliceC(what, Frag{At: v.e.SpotBody(d), Src: src})
}

// ReplaceC replaces each of the pattern's n matches by the C src, its holes
// the match's bindings.
func (v *Verbs) ReplaceC(pat, src string, n int, what string) { v.replaceC(pat, "", src, n, what) }

// ReplaceAtC replaces, in each of the pattern's n matches, the node bound
// to at by the C src, the rest of the match kept.
func (v *Verbs) ReplaceAtC(pat, at, src string, n int, what string) {
	v.replaceC(pat, at, src, n, what)
}

func (v *Verbs) replaceC(pat, at, src string, n int, what string) {
	ms, p := v.counted(pat, n, what, nil)
	if p == nil {
		return
	}
	var fs []Frag
	for _, m := range ms {
		b, _ := Match(p, m)
		target := m
		if at != "" {
			if target = b[at]; target == nil {
				v.Die("%s -- the pattern binds no ?%s", what, at)
				return
			}
		}
		fs = append(fs, Frag{At: v.e.SpotOf(target), Src: src, Holes: b})
	}
	v.spliceC(what, fs...)
}

// BeforeC puts the items src before each of the pattern's n matches, which
// are items; AfterC after each.
func (v *Verbs) BeforeC(pat, src string, n int, what string) { v.besideC(pat, src, n, what, false) }

// AfterC puts the items src after each of the pattern's n matches.
func (v *Verbs) AfterC(pat, src string, n int, what string) { v.besideC(pat, src, n, what, true) }

func (v *Verbs) besideC(pat, src string, n int, what string, after bool) {
	ms, p := v.counted(pat, n, what, nil)
	if p == nil {
		return
	}
	var fs []Frag
	for _, m := range ms {
		b, _ := Match(p, m)
		at := v.e.SpotBefore(m)
		if after {
			at = v.e.SpotAfter(m)
		}
		fs = append(fs, Frag{At: at, Src: src, Holes: b})
	}
	v.spliceC(what, fs...)
}

// SpliceRunC replaces the run of items from the one from matches through
// the one through matches by the items src.
func (v *Verbs) SpliceRunC(from, through, src, what string) {
	if v.Err != nil {
		return
	}
	a := v.One(from, what+" (its start)")
	z := v.One(through, what+" (its end)")
	if v.Err != nil {
		return
	}
	v.spliceC(what, Frag{At: v.e.SpotRun(a, z), Src: src})
}

// TopBeforeC puts the external declarations src before the file's first
// top-level declaration of name; TopAfterC after its last.  name is an
// ordinary name, or `struct T` (`union T`, `enum T`) for the form that
// defines that tag.
func (v *Verbs) TopBeforeC(name, src, what string) { v.topC(name, src, what, false) }

// TopAfterC puts the external declarations src after the file's last
// top-level declaration of name.
func (v *Verbs) TopAfterC(name, src, what string) { v.topC(name, src, what, true) }

func (v *Verbs) topC(name, src, what string, after bool) {
	if v.Err != nil {
		return
	}
	var forms []*Node
	if kw, tag, ok := strings.Cut(name, " "); ok {
		if d := v.e.tagDef(kw, tag); d != nil && v.e.inFile(d) {
			forms = append(forms, v.e.topOf(d))
		}
	} else {
		for _, f := range v.e.g.Forms {
			if topName(f) == name {
				forms = append(forms, f)
			}
		}
	}
	if len(forms) == 0 {
		v.Die("%s -- no top-level declaration of %s", what, name)
		return
	}
	at := v.e.SpotBefore(forms[0])
	if after {
		at = v.e.SpotAfter(forms[len(forms)-1])
	}
	v.spliceC(what, Frag{At: at, Src: src})
}

// LiteralC replaces each of the n runs of whole items in the scope whose C
// is old -- spaces aside, outside literals -- by the items new.  A text
// Literal that matched part of an item has no run here: the count says so.
func (v *Verbs) LiteralC(old, new string, n int, what string) {
	if v.Err != nil {
		return
	}
	key := normC(old)
	if key == "" {
		v.Die("%s -- the literal is empty", what)
		return
	}
	var fs []Frag
	for _, r := range v.roots() {
		Walk(r, func(l *Node) bool {
			lo := itemsFrom(l)
			if lo < 0 {
				return true
			}
			items := l.Kids[lo:]
			norm := make([]string, len(items))
			for i := 0; i < len(items); i++ {
				var acc string
				for k := i; k < len(items); k++ {
					if norm[k] == "" {
						c, err := clisp.PrintItems([]*clisp.Node{Lisp(items[k])})
						if err != nil {
							v.Die("%s -- %v", what, err)
							return false
						}
						norm[k] = normC(c)
					}
					if acc == "" {
						acc = norm[k]
					} else {
						acc += " " + norm[k]
					}
					if len(acc) >= len(key) || !strings.HasPrefix(key, acc) {
						if acc == key {
							fs = append(fs, Frag{At: v.e.SpotRun(items[i], items[k]), Src: new})
							i = k
						}
						break
					}
				}
			}
			return true
		})
	}
	if v.Err != nil {
		return
	}
	if len(fs) != n {
		v.Die("%s -- occurs %d times, expected %d", what, len(fs), n)
		return
	}
	v.spliceC(what, fs...)
}

// itemsFrom is where a list's items start: a block's, a statement
// expression's, a function's body; -1 for a list that holds none.
func itemsFrom(l *Node) int {
	switch {
	case l.Is("block"):
		if len(l.Kids) > 1 && l.Kids[1].Is("@") {
			return 2
		}
		return 1
	case l.Is("stmt-expr"):
		return 1
	case l.Is("defn"):
		return defnItemsAt(l)
	}
	return -1
}

// normC is C with every run of spaces outside a literal one space, and
// none at the ends.
func normC(s string) string {
	var b strings.Builder
	space := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			space = true
		case c == '"' || c == '\'':
			k := skipLiteral(s, i)
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteString(s[i:k])
			i = k - 1
		default:
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteByte(c)
		}
	}
	return b.String()
}

// A fragBatch is the FRAG acts Together defers: each act's fragments, and
// what it reports.
type fragBatch struct {
	frags []Frag
	acts  []string
	of    []int // the act each fragment is
}

// Together runs acts with every FRAG act among them -- BodyC, ReplaceC,
// LiteralC, ... -- deferred to its end and made in ONE synthesized import:
// a phase's literals for the cost of one.  Each deferred act finds its
// matches and counts them as it runs, on the graph as the batch found it,
// so an act must not look for what another act of the batch writes, and
// no two may touch one place (FRAG refuses a spot inside another's).  The
// deferred acts are reported at the end, in their order, after any other
// act the batch made; the first refusal names its act.
func (v *Verbs) Together(acts func(*Verbs)) {
	if v.Err != nil {
		return
	}
	if v.batch != nil {
		acts(v)
		return
	}
	b := &fragBatch{}
	inner := &Verbs{Tag: v.Tag, W: v.W, e: v.e, scope: v.scope, batch: b}
	acts(inner)
	if inner.Err != nil {
		v.Err = inner.Err
		return
	}
	if len(b.frags) > 0 {
		if _, err := v.e.SpliceC(b.frags...); err != nil {
			what := "together"
			if m := fragNumRE.FindStringSubmatch(err.Error()); m != nil {
				if k, _ := strconv.Atoi(m[1]); k >= 1 && k <= len(b.of) {
					what = b.acts[b.of[k-1]]
				}
			}
			v.Die("%s -- %v", what, err)
			return
		}
	}
	for _, what := range b.acts {
		v.Say(what)
	}
}

var fragNumRE = regexp.MustCompile(`frag (\d+)`)

// spliceC splices the fragments in one import and reports the act; in a
// batch, it defers them.
func (v *Verbs) spliceC(what string, fs ...Frag) {
	if v.Err != nil {
		return
	}
	for _, f := range fs {
		if f.At.err != nil {
			v.Die("%s -- %v", what, f.At.err)
			return
		}
	}
	if b := v.batch; b != nil {
		for _, f := range fs {
			b.frags = append(b.frags, f)
			b.of = append(b.of, len(b.acts))
		}
		b.acts = append(b.acts, what)
		return
	}
	if len(fs) == 0 {
		v.Say(what)
		return
	}
	if _, err := v.e.SpliceC(fs...); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}
