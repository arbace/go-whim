package graph

// The forms' syntax, as C-lisp writes it (crefactor/clisp/SPEC.md): what
// the importer and the collector ask of a form without its cc node.

// prefixWords are the specifier keywords that stand before a declared name
// (clisp's set).
var prefixWords = map[string]bool{
	"static": true, "extern": true, "typedef": true, "register": true, "auto": true,
	"inline": true, "__inline": true, "__inline__": true, "_Noreturn": true,
	"_Thread_local": true, "thread_local": true, "__thread": true, "constexpr": true,
	"__auto_type": true,
}

func isAttrForm(n *Node) bool { return n.Is("attr") || n.Is("attr-text") || n.Is("std-attr") }

// defNameAt is the index in a def's, typedef's or defn's elements of the
// name it declares, after its prefix; 0 when there is none.
func defNameAt(f *Node) int {
	for i := 1; i < len(f.Kids); i++ {
		k := f.Kids[i]
		if !k.list && prefixWords[k.Atom] || isAttrForm(k) {
			continue
		}
		if k.list {
			return 0
		}
		return i
	}
	return 0
}

// prefixOf is a def's prefix: its storage classes and leading attributes.
func prefixOf(f *Node) []*Node {
	i := defNameAt(f)
	if i == 0 {
		return nil
	}
	return f.Kids[1:i]
}

// hasPrefix says a def's prefix holds the word w; a typedef form's is
// `typedef`.
func hasPrefix(f *Node, w string) bool {
	if w == "typedef" && f.Is("typedef") {
		return true
	}
	for _, p := range prefixOf(f) {
		if !p.list && p.Atom == w {
			return true
		}
	}
	return false
}

// defType is a def's, typedef's or defn's type form.
func defType(f *Node) *Node {
	if i := defNameAt(f); i > 0 && i+1 < len(f.Kids) {
		return f.Kids[i+1]
	}
	return nil
}

// tagOf is a struct, union or enum form's tag, or "".
func tagOf(n *Node) string {
	if len(n.Kids) > 1 && !n.Kids[1].list && n.Kids[1].Atom != "{}" {
		return n.Kids[1].Atom
	}
	return ""
}

// body is a struct, union or enum form's elements after its tag, its
// leading `(@ ...)` and an enum's `(: ...)`: the members (or `{}`, and a
// trailing `(@ ...)`), the enumerators.
func body(n *Node) []*Node {
	args := n.Args()
	if len(args) > 0 && !args[0].list && args[0].Atom != "{}" {
		args = args[1:]
	}
	if len(args) > 0 && args[0].Is("@") {
		args = args[1:]
	}
	if n.Is("enum") && len(args) > 0 && args[0].Is(":") {
		args = args[1:]
	}
	return args
}

// isDefForm says a struct, union or enum form defines its type: it has a
// body.
func isDefForm(n *Node) bool {
	if !(n.Is("struct") || n.Is("union") || n.Is("enum")) {
		return false
	}
	return len(body(n)) > 0
}

// members is a struct or union definition's members, without `{}` and the
// trailing attributes.
func members(n *Node) []*Node {
	var out []*Node
	for _, m := range body(n) {
		if m.Is("@") || !m.list && m.Atom == "{}" {
			continue
		}
		out = append(out, m)
	}
	return out
}

// derivations walk a type form from the name outward: f is called on each
// derivation (ptr, array, fn, fn-ids, paren, name-attr) and the base is
// returned, the specifiers: an atom, a list of specifiers, or one
// specifier's form.
func base(t *Node, f func(d *Node)) *Node {
	for t != nil && t.list {
		switch t.Head() {
		case "ptr", "paren", "name-attr":
			if f != nil {
				f(t)
			}
			t = t.Kids[1]
		case "array", "fn", "fn-ids":
			if f != nil {
				f(t)
			}
			t = t.Kids[len(t.Kids)-1]
		default:
			return t
		}
	}
	return t
}
