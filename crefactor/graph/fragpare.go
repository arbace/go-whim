package graph

import "unicode"

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
	Walk(f, func(x *Node) bool {
		if !x.IsList() {
			return false
		}
		if isDefForm(x) {
			if t := tagOf(x); t != "" {
				out = append(out, t)
			}
		}
		if x.Is("enum") {
			for _, k := range x.Args() {
				if k.IsList() && len(k.Kids) > 0 && !k.Kids[0].IsList() {
					out = append(out, k.Kids[0].Atom)
				}
			}
		}
		return true
	})
	return out
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
