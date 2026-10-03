package graph

import "strings"

// ORPHANED FALLTHROUGHS (crefactor/sweep's fallthrough.go, on the forms).
// An attribute statement whose attributes are all fallthrough, in a live
// function, where control goes next to no case or default label, goes: the
// next item of its block, or at a block's end wherever control goes after
// the statement the block is the body of -- an if's branch, a do-while's
// body, a nested block.  The end of a switch's, a while's or a for's body,
// and of the function, reach none.

// orphans is every live function's orphaned fallthrough.
func (c *collector) orphans() []*Node {
	var out []*Node
	for _, d := range c.decls {
		if d.fn != nil && d.fn.live {
			ftItems(defnBody(d.form), false, &out)
		}
	}
	return out
}

// ftItems walks a block's items, the end of which leads to a case label
// when next.
func ftItems(items []*Node, next bool, out *[]*Node) {
	for i, it := range items {
		after := next
		if i+1 < len(items) {
			after = reachesCase(items[i+1:])
		}
		ftStatement(it, after, out)
	}
}

// reachesCase says the items start with a case or default label, or with
// ordinary labels (and C23's attributes before a statement) on one.
func reachesCase(items []*Node) bool {
	for _, it := range items {
		switch it.Head() {
		case "case", "case-range", "default":
			return true
		case "label", "stmt-attr":
			continue
		}
		return false
	}
	return false
}

func ftStatement(n *Node, next bool, out *[]*Node) {
	switch n.Head() {
	case "block":
		items := n.Kids[1:]
		if len(items) > 0 && items[0].Is("@") {
			items = items[1:]
		}
		ftItems(items, next, out)
	case "attributed":
		if isFallthrough(n) && !next {
			*out = append(*out, n)
		}
	case "if":
		for _, b := range n.Kids[2:] {
			ftStatement(b, next, out)
		}
	case "switch":
		ftStatement(n.Kids[2], false, out)
	case "while":
		ftStatement(n.Kids[2], false, out)
	case "for":
		ftStatement(n.Kids[len(n.Kids)-1], false, out)
	case "do":
		ftStatement(n.Kids[1], next, out)
	}
}

// isFallthrough says an attribute statement's attributes are all
// fallthrough, C23's or GNU's, and there is at least one.
func isFallthrough(n *Node) bool {
	seen := false
	for _, a := range n.Args() {
		switch {
		case a.Is("attr") || a.Is("std-attr"):
			for _, v := range a.Args() {
				if v.list || !isFallthroughName(v.Atom) {
					return false
				}
				seen = true
			}
		case a.Is("attr-text"):
			for _, w := range attrWords(unquote(a.Kids[1].Atom)) {
				if !isFallthroughName(w) {
					return false
				}
				seen = true
			}
		default:
			return false // an expression: not a null statement
		}
	}
	return seen
}

func isFallthroughName(s string) bool {
	if i := strings.LastIndex(s, "::"); i >= 0 {
		s = s[i+2:]
	}
	return s == "fallthrough" || s == "__fallthrough__"
}

// attrWords is the attribute names an attribute's text holds: its
// identifiers but the keyword and a namespace's prefix.
func attrWords(s string) []string {
	var out []string
	b := []byte(s)
	for i := 0; i < len(b); {
		if !(b[i] == '_' || b[i] >= 'a' && b[i] <= 'z' || b[i] >= 'A' && b[i] <= 'Z') {
			i++
			continue
		}
		j := i + 1
		for j < len(b) && identByte(b[j]) {
			j++
		}
		w := string(b[i:j])
		if w != "__attribute__" && w != "__attribute" && !strings.HasPrefix(string(b[j:]), "::") {
			out = append(out, w)
		}
		i = j
	}
	return out
}
