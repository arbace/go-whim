package graph

import (
	"strconv"
	"unicode"
)

// THE PARED UNIT.  FRAG's unit was every top-level form of the file, each
// definition but those printed whole cut to its header: the whole file's
// declarations parsed, checked and imported for every fragment, 60% of an
// edit through a view on whim-vim.c's graph.  It is pared now to what the
// fragments can name: the forms printed whole, every
// top-level form that declares a name the fragments' text spells (an
// object, a function, a typedef, a tag, an enumerator) or a hole's node
// refers to, and, closed over, every form the printed part of one of these
// refers to and every declaration of a name one of them declares (a tag's
// definition beside its forward declaration), and the include lines before
// the last of them -- in the file's order.  What cc resolves in the unit is then
// what it resolved in the whole: every name the fragments and the forms
// printed whole can reach is declared as it was, and nothing else is.
// fragWhole, which a test sets, prints the whole file instead: the
// control the paring is held to (TestFragPared).
var fragWhole bool

// needed is the top-level forms the unit prints, the rest omitted.
func (s *synth) needed() map[*Node]bool {
	e := s.e
	need := map[*Node]bool{}
	var work []*Node
	// the names each form declares
	byName := map[string][]*Node{}
	declares := map[*Node][]string{}
	for _, f := range e.g.Forms {
		if !IsInclude(f) {
			declares[f] = formDeclares(f)
			for _, n := range declares[f] {
				byName[n] = append(byName[n], f)
			}
		}
	}
	var add func(f *Node)
	add = func(f *Node) {
		if f == nil || need[f] || !e.Live(f) {
			return
		}
		need[f] = true
		work = append(work, f)
		// one entity's every declaration: a tag's forward declaration
		// and its definition, a function's prototypes and its definition
		for _, n := range declares[f] {
			for _, g := range byName[n] {
				add(g)
			}
		}
	}

	for f := range s.keep {
		add(f)
	}
	for _, j := range s.jobs {
		for _, w := range identifiers(j.f.Src) {
			for _, f := range byName[w] {
				add(f)
			}
		}
		for _, h := range j.f.Holes {
			Walk(h, func(x *Node) bool {
				for _, r := range x.Refs {
					add(e.topForm(r))
				}
				return true
			})
		}
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		visit := func(x *Node) bool {
			if s.omit[x] {
				return false // a statement left out of a function printed whole
			}
			if !x.IsList() && len(x.Refs) == 0 {
				// an atom no edge resolves names by its spelling, as the
				// fragments' text does
				for _, g := range byName[x.Atom] {
					add(g)
				}
			}
			if x.Is("macro") {
				// a macro's invocation is its text, which no edge resolves:
				// va_arg(*ap, uvarnumber_T) names a typedef
				for _, k := range x.Args() {
					if !k.IsList() {
						txt := k.Atom
						if u, err := strconv.Unquote(txt); err == nil {
							txt = u
						}
						for _, w := range identifiers(txt) {
							for _, g := range byName[w] {
								add(g)
							}
						}
					}
				}
			}
			for _, r := range x.Refs {
				add(e.topForm(r))
			}
			// a tag spelled names its declarations by name: an edge to a
			// definition an earlier splice of the phase replaced is not
			// retargeted until its re-check
			if (x.Is("struct") || x.Is("union") || x.Is("enum")) && len(x.Kids) >= 2 && !x.Kids[1].IsList() {
				for _, g := range byName[x.Kids[1].Atom] {
					add(g)
				}
			}
			return true
		}
		if f.Is("defn") && !s.keep[f] {
			for _, k := range f.Kids[:defnItemsAt(f)] {
				Walk(k, visit)
			}
			continue
		}
		Walk(f, visit)
	}
	// the include lines before anything printed: a form names only what
	// is declared before it, so a header after the last form the unit
	// prints, or the last fragment's place, cannot change what they say
	// (in whim-vim.c, where the first include is the core's end, an edit
	// in the core parses no header)
	last := -1
	for k, f := range e.g.Forms {
		if need[f] {
			last = k
		}
	}
	for _, j := range s.jobs {
		if j.f.At.p == e.top[0] {
			last = max(last, j.f.At.lo)
		}
	}
	for k, f := range e.g.Forms {
		if IsInclude(f) && k < last {
			need[f] = true
		}
	}
	return need
}

// topForm is the file's top-level form holding n, or nil for a node in no
// form of the file (a header's, a type node).
func (e *Editor) topForm(n *Node) *Node {
	for n != nil && n.up != nil && n.up != e.top[0] {
		n = n.up
	}
	if n == nil || n.up != e.top[0] {
		return nil
	}
	return n
}

// formDeclares is every name a top-level form declares at file scope: its
// own, the tags it defines, its enumerators.
func formDeclares(f *Node) []string {
	var out []string
	if n := topName(f); n != "" {
		out = append(out, n)
	} else if f.Is("def") || f.Is("defn") {
		if n := declName(f); n != "" {
			out = append(out, n)
		}
	}
	// a definition's body declares nothing at file scope: what a function
	// defines in it is named only inside it, and a fragment there prints
	// the function whole
	parts := []*Node{f}
	if f.Is("defn") {
		parts = f.Kids[:defnItemsAt(f)]
	}
	for _, part := range parts {
		formDeclaresIn(part, &out)
	}
	return out
}

func formDeclaresIn(f *Node, out *[]string) {
	Walk(f, func(x *Node) bool {
		if !x.IsList() {
			return false
		}
		if isDefForm(x) {
			if t := tagOf(x); t != "" {
				*out = append(*out, t)
			}
		}
		if x.Is("enum") {
			for _, k := range x.Args() {
				if k.IsList() && len(k.Kids) > 0 && !k.Kids[0].IsList() {
					*out = append(*out, k.Kids[0].Atom)
				}
			}
		}
		return true
	})
}

// identifiers are the words of C text that could name something: its
// identifiers, outside literals and comments.
func identifiers(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		c := rune(src[i])
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(src) && rune(src[j]) != c {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}
			i = j + 2
		case c == '_' || unicode.IsLetter(c):
			j := i
			for j < len(src) && (src[j] == '_' || unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j]))) {
				j++
			}
			out = append(out, src[i:j])
			i = j
		case unicode.IsDigit(c):
			j := i
			for j < len(src) && (src[j] == '_' || src[j] == '.' || src[j] == '\'' || unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j]))) {
				j++
			}
			i = j
		default:
			i++
		}
	}
	return out
}

// pareBodies leaves out of each function printed whole for a fragment in it
// the statements no fragment can see.  In each block on the way from the
// function's body to a fragment's place, an item is printed when it is on
// that way, is a declaration (what a fragment can name; isDeclItem),
// holds a label, a case or a default (what a fragment's goto, or a
// switch's check, can reach), or follows the place in the fragment's own
// block and spells a name the fragment spells (the scope of what the
// fragment declares, where a use printed whole is resolved again:
// retarget -- an item spelling none of its names cannot name what it
// declares); the other statements are left out
// (s.omit), and the walk beside the graph steps past them.  A function
// printed whole for another reason -- one using a name a top-level
// fragment declares -- is printed whole.
func (s *synth) pareBodies() {
	if fragWhole {
		return
	}
	type place struct {
		block *Node
		at    int      // the item on the way, or the fragment's first
		hi    int      // the fragment's end, in its own block; -1 on the way
		words []string // the fragment's identifiers, in its own block
	}
	var places []place
	for _, j := range s.jobs {
		if j.f.At.p == s.e.top[0] {
			continue
		}
		// the fragment's own list, when it is a block of items
		if j.kind == kindItems && pareable(j.f.At.p) {
			places = append(places, place{j.f.At.p, j.f.At.lo, j.f.At.hi, identifiers(j.f.Src)})
		}
		for x := j.f.At.p; x.up != nil && x.up != s.e.top[0]; x = x.up {
			if p := x.up; pareable(p) {
				places = append(places, place{p, indexIn(p.Kids, x), -1, nil})
			}
		}
	}
	keep := map[*Node]map[int]bool{}
	after := map[*Node]int{}               // a fragment's own block: from here on, what it may resolve again
	spelled := map[*Node]map[string]bool{} // the names the fragments in it spell
	for _, pl := range places {
		if keep[pl.block] == nil {
			keep[pl.block] = map[int]bool{}
		}
		keep[pl.block][pl.at] = true
		if pl.hi >= 0 {
			if a, ok := after[pl.block]; !ok || pl.at < a {
				after[pl.block] = pl.at
			}
			if spelled[pl.block] == nil {
				spelled[pl.block] = map[string]bool{}
			}
			for _, w := range pl.words {
				spelled[pl.block][w] = true
			}
		}
	}
	for b, on := range keep {
		if s.e.Parent(b) != nil && s.e.Parent(b).Is("switch") {
			continue // its cases are its check's
		}
		if s.whole[s.e.topOf(b)] {
			continue // a use in it is resolved again (colliding)
		}
		from, own := after[b]
		first := itemsFrom(b)
		for i, k := range b.Kids {
			// the item after a label is the statement it labels
			afterLabel := i > first && b.Kids[i-1].Is("label")
			// after a fragment in its block, what may resolve again: an
			// item that spells a name the fragment spells (no other can
			// name what the fragment declares)
			inScope := own && i >= from && mentions(k, spelled[b])
			if i < first || on[i] || inScope || isDeclItem(k) || k.Is("verbatim") || holdsLabel(k) || afterLabel {
				continue
			}
			s.omit[k] = true
		}
	}
}

// holdsLabel says x is or holds a label, a case or a default.
func holdsLabel(x *Node) bool {
	found := false
	Walk(x, func(n *Node) bool {
		if n.Is("label") || n.Is("case") || n.Is("default") {
			found = true
		}
		return !found
	})
	return found
}

// pareable is a list whose statements may be left out: a block or a
// function's body, not a statement expression, whose last item is its value.
func pareable(l *Node) bool { return (l.Is("block") || l.Is("defn")) && itemsFrom(l) > 0 }

// mentions says x spells one of names: an atom, or a word of a macro's text.
func mentions(x *Node, names map[string]bool) bool {
	found := false
	Walk(x, func(n *Node) bool {
		switch {
		case found:
		case !n.IsList():
			found = names[n.Atom]
		case n.Is("macro"):
			for _, k := range n.Args() {
				if !k.IsList() {
					txt := k.Atom
					if u, err := strconv.Unquote(txt); err == nil {
						txt = u
					}
					for _, w := range identifiers(txt) {
						found = found || names[w]
					}
				}
			}
		}
		return !found
	})
	return found
}
