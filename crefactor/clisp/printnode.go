package clisp

// PrintExpr is an expression's form as C, in cemit's spelling, at the
// lowest precedence its place could ask (a comma's): what a caller writes
// into C text, parenthesised as it needs.
func PrintExpr(n *Node) (string, error) {
	p := &printer{}
	s := p.expr(n, lvComma)
	return s, p.err
}

// PrintItems is items -- statements and declarations of a block -- as C, in
// cemit's spelling, at no indentation.
func PrintItems(items []*Node) (string, error) {
	p := &printer{}
	for _, it := range items {
		p.item(it)
	}
	return p.b.String(), p.err
}
