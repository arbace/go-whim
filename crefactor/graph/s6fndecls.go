package graph

// s6indexBlockFns indexes the block-scope declarations of functions -- a
// prototype inside a function's body -- by name, in the file's order: what
// funcDecls asked of a walk of every function on every call (PARAM's and
// RETYPE's, 155 of them in phase 100's LocalOut).  A declaration is an item,
// so no expression is walked but a statement expression.
func (e *Editor) s6indexBlockFns() {
	e.blockFns = map[string][]*Node{}
	for _, f := range e.g.Forms {
		if !f.Is("defn") {
			continue
		}
		Walk(f, func(n *Node) bool {
			if n != f && n.Is("def") && defType(n).Is("fn") {
				name := topName(n)
				e.blockFns[name] = append(e.blockFns[name], n)
			}
			return !n.list || !isExprForm(n) || n.Is("stmt-expr")
		})
	}
	e.blockFnsOK = true
}
