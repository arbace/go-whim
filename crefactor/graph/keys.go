package graph

import "strings"

// keys are the closure's keys a form mentions (Collect's comment).  skip
// holds the struct, union and enum definitions whose bodies are their own
// entities': their tag and attributes are the form's, their members not.
func (c *collector) keys(n *Node, skip map[*Node]bool) []string {
	k := &keyset{seen: map[string]bool{}}
	c.scan(n, skip, k)
	return k.out
}

type keyset struct {
	seen map[string]bool
	out  []string
}

func (k *keyset) add(s string) {
	if !k.seen[s] {
		k.seen[s] = true
		k.out = append(k.out, s)
	}
}

func (c *collector) scan(n *Node, skip map[*Node]bool, k *keyset) {
	if !n.list {
		c.scanAtom(n, k)
		return
	}
	switch n.Head() {
	case "struct", "union", "enum":
		if t := tagOf(n); t != "" {
			k.add("t:" + t)
		}
		for _, a := range n.Args() {
			switch {
			case !a.list:
				// the tag, or `{}`
			case a.Is("@") || a.Is(":"):
				c.scan(a, skip, k)
			case skip[n]:
			case n.Is("enum"):
				k.add("o:" + a.Kids[0].Atom)
				for _, x := range a.Kids[1:] {
					c.scan(x, skip, k)
				}
			default:
				c.scanMember(a, skip, k)
			}
		}
	case "->", ".":
		c.scan(n.Kids[1], skip, k)
		for _, m := range n.Kids[2:] {
			if m.list {
				c.scanText(m.Kids[1].Atom, true, m, k)
			} else {
				k.add(c.memberKey(m, m.Atom))
			}
		}
	case "at":
		for _, d := range n.Args() {
			switch {
			case d.list:
				c.scan(d, skip, k)
			case strings.HasPrefix(d.Atom, "."):
				k.add(c.memberKey(d, d.Atom[1:]))
			case strings.HasSuffix(d.Atom, ":"):
				k.add("o:" + strings.TrimSuffix(d.Atom, ":"))
			default:
				c.scanAtom(d, k)
			}
		}
	case "goto", "label", "label-addr":
		if len(n.Kids) == 2 && !n.Kids[1].list {
			k.add("o:" + n.Kids[1].Atom)
			return
		}
		c.scanKids(n, skip, k)
	case "def", "typedef", "defn":
		at := defNameAt(n)
		for i, x := range n.Kids {
			if i == 0 || i == at || !x.list && i < at {
				continue
			}
			c.scan(x, skip, k)
		}
	case "fn":
		for _, p := range n.Kids[1].Kids {
			c.scanParam(p, skip, k)
		}
		c.scan(n.Kids[2], skip, k)
	case "fn-ids":
		c.scan(n.Kids[2], skip, k)
	case "attr", "std-attr":
		c.scanAttr(n, k)
	case "attr-text", "macro", "macro-decl", "verbatim", "asm-label":
		c.scanText(n.Kids[1].Atom, false, n, k)
	case "include", "directive":
	default:
		c.scanKids(n, skip, k)
	}
}

func (c *collector) scanKids(n *Node, skip map[*Node]bool, k *keyset) {
	for _, x := range n.Kids {
		c.scan(x, skip, k)
	}
}

// scanMember is a member's form: its name is no key.
func (c *collector) scanMember(m *Node, skip map[*Node]bool, k *keyset) {
	if memberDeclName(m) != "" {
		for _, x := range m.Kids[1:] {
			c.scan(x, skip, k)
		}
		return
	}
	c.scan(m, skip, k)
}

// scanParam is a parameter's form: `(NAME TYPE ATTR...)`, whose name is no
// key, `(TYPE ATTR...)`, or a type alone.
func (c *collector) scanParam(p *Node, skip map[*Node]bool, k *keyset) {
	if p.list && len(p.Kids) >= 2 && !p.Kids[0].list && !isAttrForm(p.Kids[1]) && !p.Kids[1].Is("bits") &&
		len(p.Refs) == 0 && p.Head() != "spec" {
		for _, x := range p.Kids[1:] {
			c.scan(x, skip, k)
		}
		return
	}
	c.scan(p, skip, k)
}

// scanAtom is a use: a file-scope name's key, none for a local's.
func (c *collector) scanAtom(n *Node, k *keyset) {
	if len(n.Refs) == 0 || !isIdentText(n.Atom) {
		return
	}
	t := n.Refs[0]
	if c.isTypedefName(n) || c.file[t] || isExtern(t) || hasPrefix(t, "extern") {
		k.add("o:" + n.Atom)
	}
}

// scanAttr is an attribute's form: its words are names.
func (c *collector) scanAttr(n *Node, k *keyset) {
	for _, x := range n.Kids[1:] {
		if x.list {
			c.scanAttr(NewList(append([]*Node{NewAtom("")}, x.Kids...)...), k)
			continue
		}
		if isIdentText(x.Atom) {
			k.add("o:" + x.Atom)
		}
	}
}

// scanText is a text the forms keep whole, scanned as Prune scans the
// source: each identifier outside a literal, its name space told by the
// token before it, a local's name no key.  member says the text stands
// in a member's place.
func (c *collector) scanText(quoted string, member bool, n *Node, k *keyset) {
	s := unquote(quoted)
	locals := map[string]bool{}
	for _, t := range n.Refs {
		if !c.file[t] && !isExtern(t) && !hasPrefix(t, "extern") {
			locals[declName(t)] = true
		}
	}
	b := []byte(s)
	blankLiterals(b)
	for i := 0; i < len(b); {
		ch := b[i]
		if !(ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z') {
			i++
			continue
		}
		j := i + 1
		for j < len(b) && identByte(b[j]) {
			j++
		}
		word := string(b[i:j])
		tail := i > 0 && identByte(b[i-1])
		ns := classify(b, i)
		if i == firstIdent(b) && member {
			ns = "m:"
		}
		i = j
		if tail || ns == "o:" && locals[word] {
			continue
		}
		k.add(ns + word)
	}
}

func firstIdent(b []byte) int {
	for i, ch := range b {
		if ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			return i
		}
	}
	return -1
}

func identByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// classify is the name space of the identifier at p, from the token before
// it (Prune's).
func classify(b []byte, p int) string {
	i := p - 1
	for i >= 0 && isSpace(b[i]) {
		i--
	}
	if i < 0 {
		return "o:"
	}
	switch b[i] {
	case '.':
		return "m:"
	case '>':
		if i > 0 && b[i-1] == '-' {
			return "m:"
		}
		return "o:"
	}
	if identByte(b[i]) {
		j := i
		for j >= 0 && identByte(b[j]) {
			j--
		}
		switch string(b[j+1 : i+1]) {
		case "struct", "union", "enum":
			return "t:"
		}
	}
	return "o:"
}

// blankLiterals blanks the insides of string and character literals.
func blankLiterals(b []byte) {
	for i := 0; i < len(b); i++ {
		q := b[i]
		if q != '"' && q != '\'' {
			continue
		}
		for i++; i < len(b) && b[i] != q; i++ {
			if b[i] == '\\' && i+1 < len(b) {
				b[i] = ' '
				i++
			}
			b[i] = ' '
		}
	}
}

// unquote is a quoted atom's text (C-lisp's quote reversed).
func unquote(s string) string {
	if len(s) < 2 || s[0] != '"' {
		return s
	}
	s = s[1 : len(s)-1]
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' {
				b.WriteByte('\n')
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
