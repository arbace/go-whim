package graph

import (
	"strconv"
	"strings"
)

// THE TYPES BOOLRET'S EDITS LEAVE UNKNOWN.  RETYPE types again the
// expressions above a retyped declaration where the type follows plainly
// (typeedit.go's derive) and clears the rest -- a `?:`, a sum, a shift --
// into Untyped.  brTypeUntyped gives them back the type C's rules give
// them: the integer promotions and the usual arithmetic conversions (LP64:
// cc's linux/amd64), pointer arithmetic, a `?:`'s operands brought to one
// type, a comma's last operand, an integer literal's type by its value and
// suffix.  What it cannot say stays listed: unknown, never wrong.

// brTypeUntyped types every expression form listed in Untyped whose type
// follows from its operands', to a fixed point, and says how many.
func (e *Editor) brTypeUntyped() int {
	tx := e.typeTx()
	done := 0
	for changed := true; changed; {
		changed = false
		for _, n := range append([]*Node{}, e.Untyped...) {
			if !n.list || !e.Live(n) || !isExprForm(n) {
				continue
			}
			t := tx.derive(n)
			if t == nil {
				t = tx.brDerive(n)
			}
			if t == nil {
				continue
			}
			e.g.save(n)
			n.Type = t
			e.typed(n)
			if e.inFile(t) {
				e.typedBy[t] = append(e.typedBy[t], n)
			}
			done++
			changed = true
		}
	}
	tx.commit()
	return done
}

// brOperand is an operand's type as an rvalue: an array its element's
// pointer, a function its pointer; a literal's by C's rule.
func (tx *typeTx) brOperand(x *Node) *Node {
	t := typeOf(x)
	if t == nil && !x.list {
		t = tx.brLiteral(x.Atom)
	}
	switch {
	case t.Is("array"):
		return tx.pointer(pointee(t))
	case t.Is("function"):
		return tx.pointer(t)
	}
	return t
}

// brLiteral is an integer or character constant's type, or nil.
func (tx *typeTx) brLiteral(s string) *Node {
	switch {
	case s == "":
		return nil
	case s == "nullptr": // cc's: ((void *)0)
		return tx.pointer(tx.basic([]string{"void"}))
	case s == "true" || s == "false": // cc's predefined macros: 1 and 0
		return tx.basic([]string{"int"})
	case s[0] == '\'':
		return tx.basic([]string{"int"})
	case s[0] == '"': // an array of char, as an operand its pointer
		return tx.pointer(tx.basic([]string{"char"}))
	case s[0] < '0' || s[0] > '9':
		return nil
	}
	if strings.ContainsAny(s, ".pP") || !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") && strings.ContainsAny(s, "eE") {
		return nil // floating
	}
	i := len(s)
	for i > 0 && strings.ContainsRune("uUlL", rune(s[i-1])) {
		i--
	}
	digits, suffix := strings.ReplaceAll(s[:i], "'", ""), strings.ToLower(s[i:])
	v, err := strconv.ParseUint(digits, 0, 64)
	if err != nil {
		return nil
	}
	decimal := !strings.HasPrefix(digits, "0") || digits == "0"
	unsigned := strings.Contains(suffix, "u")
	long := strings.Count(suffix, "l")
	cands := [][]string{{"int"}, {"long"}, {"long", "long"}}
	if long == 1 {
		cands = cands[1:]
	} else if long == 2 {
		cands = cands[2:]
	}
	for _, c := range cands {
		bits := uint(31)
		if c[0] == "long" {
			bits = 63
		}
		if !unsigned && v <= 1<<bits-1 {
			return tx.basic(c)
		}
		if (unsigned || !decimal) && v <= 1<<(bits+1)-1 {
			if c[0] == "int" {
				return tx.basic([]string{"unsigned"})
			}
			return tx.basic(append([]string{"unsigned"}, c...))
		}
	}
	return nil
}

// brArith is an arithmetic type's words, or nil.
func brArith(t *Node) []string {
	if t.Is("enum") || t.Is("extern-enum") {
		return []string{"int"}
	}
	if !t.Is("basic") {
		return nil
	}
	var w []string
	for _, k := range t.Args() {
		w = append(w, k.Atom)
	}
	switch strings.Join(w, " ") {
	case "void", "invalid":
		return nil
	}
	return w
}

// brPromote is the integer promotions.
func brPromote(w []string) []string {
	switch strings.Join(w, " ") {
	case "_Bool", "char", "signed char", "unsigned char", "short", "unsigned short":
		return []string{"int"}
	}
	return w
}

// brRank is an integer type's rank and whether it is unsigned; a floating
// type's rank is above every integer's.
func brRank(w []string) (int, bool) {
	s := strings.Join(w, " ")
	switch s {
	case "float":
		return 10, false
	case "double":
		return 11, false
	case "long double":
		return 12, false
	}
	u := strings.HasPrefix(s, "unsigned")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "unsigned"), " ")
	switch s {
	case "", "int":
		return 1, u
	case "long":
		return 2, u
	case "long long":
		return 3, u
	}
	return 0, u
}

// brUsual is the usual arithmetic conversions of a and b, promoted.
func brUsual(a, b []string) []string {
	a, b = brPromote(a), brPromote(b)
	ra, ua := brRank(a)
	rb, ub := brRank(b)
	switch {
	case ra >= 10 || rb >= 10:
		if ra >= rb {
			return a
		}
		return b
	case ua == ub:
		if ra >= rb {
			return a
		}
		return b
	}
	if ua { // a unsigned, b signed
		if ra >= rb {
			return a
		}
		if rb > ra && !(rb == 3 && ra == 2) && !(rb == 2 && ra == 2) {
			return b
		}
		return append([]string{"unsigned"}, b...)
	}
	return brUsual(b, a)
}

// brDerive is the type of a form C's conversions give it, or nil.
func (tx *typeTx) brDerive(n *Node) *Node {
	args := n.Args()
	h := n.Head()
	operands := func() ([][]string, bool) {
		var ws [][]string
		for _, a := range args {
			w := brArith(tx.brOperand(a))
			if w == nil {
				return nil, false
			}
			ws = append(ws, w)
		}
		return ws, true
	}
	switch h {
	case "paren":
		if len(args) == 1 {
			return typeOf(args[0])
		}
	case "comma":
		if len(args) > 0 {
			return tx.brOperand(args[len(args)-1])
		}
	case "-", "+":
		if len(args) == 1 {
			if ws, ok := operands(); ok {
				return tx.basic(brPromote(ws[0]))
			}
			return nil
		}
		// (+ a b c) is ((a + b) + c)
		t := tx.brOperand(args[0])
		for _, a := range args[1:] {
			t = tx.brAdd(h, t, tx.brOperand(a))
		}
		return t
	case "*", "/", "%", "&", "|", "^":
		if ws, ok := operands(); ok && len(ws) >= 2 {
			w := ws[0]
			for _, x := range ws[1:] {
				w = brUsual(w, x)
			}
			return tx.basic(w)
		}
	case "<<", ">>", "~":
		if ws, ok := operands(); ok && len(ws) >= 1 {
			return tx.basic(brPromote(ws[0]))
		}
	case "?":
		if len(args) != 3 {
			return nil
		}
		ta, tb := tx.brOperand(args[1]), tx.brOperand(args[2])
		switch {
		case ta.Is("pointer") && brNull(args[2]):
			return ta
		case tb.Is("pointer") && brNull(args[1]):
			return tb
		case ta == nil || tb == nil:
			return nil
		}
		if wa, wb := brArith(ta), brArith(tb); wa != nil && wb != nil {
			return tx.basic(brUsual(wa, wb))
		}
		if ta == tb {
			return ta
		}
	}
	return nil
}

// brAdd is a + b's or a - b's type: pointer arithmetic, or the usual
// conversions.
func (tx *typeTx) brAdd(h string, ta, tb *Node) *Node {
	switch {
	case ta == nil || tb == nil:
		return nil
	case ta.Is("pointer") && tb.Is("pointer") && h == "-":
		return tx.basic([]string{"long"})
	case ta.Is("pointer"):
		return ta
	case tb.Is("pointer") && h == "+":
		return tb
	}
	if wa, wb := brArith(ta), brArith(tb); wa != nil && wb != nil {
		return tx.basic(brUsual(wa, wb))
	}
	return nil
}

// brNull says x is a null pointer constant: nullptr, 0.
func brNull(x *Node) bool {
	for x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	return !x.list && (x.Atom == "nullptr" || x.Atom == "0")
}
