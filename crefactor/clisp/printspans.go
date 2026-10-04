package clisp

// PrintSpans is Print, telling span where each top-level form and each
// block item was printed: the bytes [start, end) of the result, its
// indentation and its newline included.  A caller maps a place in the text
// back to the form or item that printed it (crefactor/graph's Draft).
func PrintSpans(forms []*Node, span func(n *Node, start, end int)) ([]byte, error) {
	p := &printer{span: span}
	for i, f := range forms {
		if i > 0 && !(f.Is("include") && forms[i-1].Is("include")) {
			p.w("\n")
		}
		p.top(f)
		if p.err != nil {
			return nil, p.err
		}
	}
	return []byte(p.b.String()), nil
}

func (p *printer) spanned(n *Node, start int) {
	if p.err == nil {
		p.span(n, start, p.b.Len())
	}
}
