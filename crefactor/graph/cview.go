package graph

import (
	"github.com/arbace/go-whim/crefactor/clisp"
)

// C is the C VIEW: the file's containment printed in crefactor/cemit's
// spelling.  It is C-lisp's printer on the forms, which is cemit's text
// byte for byte (doc/C-LISP.md), so the graph imported from a canonical
// text prints that text back.  The types, the externs and every edge are
// not in it: C says them by name.
func (g *Graph) C() ([]byte, error) {
	forms := make([]*clisp.Node, len(g.Forms))
	for i, f := range g.Forms {
		forms[i] = Lisp(f)
	}
	return clisp.Print(forms)
}

// Lisp is n as C-lisp's form, without ids or edges.
func Lisp(n *Node) *clisp.Node {
	if !n.list {
		return clisp.A(n.Atom)
	}
	kids := make([]*clisp.Node, len(n.Kids))
	for i, k := range n.Kids {
		kids[i] = Lisp(k)
	}
	return clisp.L(kids...)
}
