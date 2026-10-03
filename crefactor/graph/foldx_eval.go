package graph

import (
	"math/bits"
	"strconv"
	"strings"
)

// THE CONSTANTS FOLDX WRITES (foldx.go).  xform.FallOut asks cc's check for
// the value of an expression its rounds wrote a constant into, and gets C's
// own answer: each operand converted to the operation's type, the unsigned
// wrapping, a comparison of pointers, `0 && f()` false without f.  The graph
// keeps no checker (step 6's), so this is cc's evaluation on the forms -- its
// constant cases, value.go's, ported -- with the types C gives the operands:
// a literal's by its suffix and size, an enumerator's int, a cast's its type
// form's, `nullptr` a pointer.  What cc does not answer -- sizeof, an object,
// a call -- is no answer here either.

// An xtype is an integer or pointer type, as far as a constant needs one.
type xtype struct {
	size   int // bytes: 1, 2, 4, 8; 0 for no integer or pointer type
	signed bool
	ptr    bool
	bool_  bool
	float  bool // an arithmetic type that is not an integer: no answer here
}

var (
	xInt   = xtype{size: 4, signed: true}
	xUInt  = xtype{size: 4}
	xLong  = xtype{size: 8, signed: true}
	xULong = xtype{size: 8}
	xPtr   = xtype{size: 8, ptr: true}
)

func (t xtype) integer() bool    { return t.size > 0 && !t.ptr }
func (t xtype) arithmetic() bool { return t.integer() || t.float }

// An xvalue is cc's Int64Value or UInt64Value.
type xvalue struct {
	v        int64
	unsigned bool
}

// convert is cc's convert (check.go) for the integer and pointer types.
func (v xvalue) convert(t xtype) (xvalue, bool) {
	switch {
	case t.bool_:
		return xvalue{v: b2i(v.v != 0)}, true
	case t.ptr:
		return xvalue{v: v.v, unsigned: true}, true
	case !t.integer() || t.size > 8:
		return xvalue{}, false
	case t.signed:
		x := v.v
		if t.size < 8 {
			sbit := int64(1) << (t.size*8 - 1)
			if x&sbit != 0 {
				x |= ^(sbit<<1 - 1)
			} else {
				x &= sbit - 1
			}
		}
		return xvalue{v: x}, true
	}
	m := ^uint64(0)
	if t.size < 8 {
		m = uint64(1)<<(8*t.size) - 1
	}
	return xvalue{v: int64(uint64(v.v) & m), unsigned: true}, true
}

func (v xvalue) zero() bool { return v.v == 0 }

// promote is C's integer promotion.
func promote(t xtype) xtype {
	if t.integer() && t.size < 4 {
		return xInt
	}
	return t
}

// usual is C's usual arithmetic conversions of two integer types.
func usual(a, b xtype) xtype {
	a, b = promote(a), promote(b)
	if a == b {
		return a
	}
	if a.signed == b.signed {
		if a.size >= b.size {
			return a
		}
		return b
	}
	u, s := a, b
	if a.signed {
		u, s = b, a
	}
	if u.size >= s.size {
		return u
	}
	return s
}

// litType is an integer literal's value and type, as cc's intConst gives
// them; ok is false for what is not one.
func litType(tok string) (xvalue, xtype, bool) {
	s0 := strings.ReplaceAll(tok, "'", "")
	s := strings.TrimRight(s0, "uUlL")
	suffix := strings.ToLower(s0[len(s):])
	base, digits := 10, s
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		base, digits = 16, s[2:]
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		base, digits = 2, s[2:]
	case strings.HasPrefix(s, "0"):
		base = 8
	}
	if digits == "" && base != 8 {
		return xvalue{}, xtype{}, false
	}
	var u uint64
	for _, c := range digits {
		var d uint64
		switch {
		case c >= '0' && c <= '9':
			d = uint64(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint64(c-'A') + 10
		default:
			return xvalue{}, xtype{}, false
		}
		if d >= uint64(base) {
			return xvalue{}, xtype{}, false
		}
		hi, lo := bits.Mul64(u, uint64(base))
		if hi != 0 {
			return xvalue{}, xtype{}, false
		}
		u = lo + d
	}
	var list []xtype
	switch suffix {
	case "":
		if base == 10 {
			list = []xtype{xInt, xLong, xLong}
		} else {
			list = []xtype{xInt, xUInt, xLong, xULong}
		}
	case "u":
		list = []xtype{xUInt, xULong}
	case "l", "ll":
		if base == 10 {
			list = []xtype{xLong}
		} else {
			list = []xtype{xLong, xULong}
		}
	case "lu", "ul", "llu", "ull":
		list = []xtype{xULong}
	default:
		return xvalue{}, xtype{}, false
	}
	b := bits.Len64(u)
	for _, t := range list {
		sign := 0
		if t.signed {
			sign = 1
		}
		if t.size*8 >= b+sign {
			return xvalue{v: int64(u), unsigned: !t.signed}, t, true
		}
	}
	return xvalue{}, xtype{}, false
}

// basicXType is the type a `(basic ...)` type node, or a list of type
// words, names.
func basicXType(words []string) xtype {
	n := map[string]int{}
	for _, w := range words {
		n[w]++
	}
	switch {
	case n["float"] > 0 || n["double"] > 0 || n["_Complex"] > 0 || n["_Float128"] > 0:
		return xtype{float: true}
	case n["void"] > 0:
		return xtype{}
	case n["_Bool"] > 0 || n["bool"] > 0:
		return xtype{size: 1, bool_: true}
	}
	t := xtype{size: 4, signed: n["unsigned"] == 0}
	switch {
	case n["char"] > 0:
		t.size = 1
	case n["short"] > 0:
		t.size = 2
	case n["long"] > 0:
		t.size = 8
	case n["int"] == 0 && n["unsigned"] == 0 && n["signed"] == 0:
		return xtype{}
	}
	return t
}

// xtypeOfType is the xtype of a type node.
func xtypeOfType(t *Node) xtype {
	switch {
	case t == nil:
		return xtype{}
	case t.Is("pointer"):
		return xPtr
	case t.Is("basic"):
		var ws []string
		for _, k := range t.Kids[1:] {
			ws = append(ws, k.Atom)
		}
		return basicXType(ws)
	case t.Is("enum"), t.Is("extern-enum"):
		return xInt
	}
	return xtype{}
}

// xtypeOfForm is the xtype a type FORM names -- a cast's -- through the
// typedefs it refers to.
func xtypeOfForm(t *Node) xtype {
	for depth := 0; t != nil && depth < 64; depth++ {
		if !t.list {
			if d := t.Ref(); d != nil {
				switch {
				case (d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef")) && d.Type != nil:
					return xtypeOfType(d.Type)
				case d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef"):
					t = defType(d)
					continue
				case d.Is("extern-typedef"):
					return xtypeOfType(d.Type)
				}
			}
			return basicXType([]string{t.Atom})
		}
		switch h := t.Head(); h {
		case "ptr", "array", "fn", "fn-ids":
			return xPtr
		case "paren", "name-attr":
			t = t.Kids[1]
			continue
		case "enum":
			return xInt
		case "struct", "union", "typeof", "typeof-type", "__typeof__", "__typeof__-type", "atomic":
			return xtype{}
		case "spec":
			t = t.Kids[1]
			continue
		}
		// a list of specifiers: the words, or the one typedef name among
		// qualifiers
		var words []string
		var named *Node
		for _, k := range t.Kids {
			switch {
			case k.list:
				if !isAttrForm(k) {
					named = k
				}
			case k.Atom == "const" || k.Atom == "volatile" || k.Atom == "restrict" || k.Atom == "register" || k.Atom == "static":
			case k.Ref() != nil:
				named = k
			default:
				words = append(words, k.Atom)
			}
		}
		if len(words) == 0 && named != nil {
			t = named
			continue
		}
		return basicXType(words)
	}
	return xtype{}
}

// xconst is a constant expression's value and type as cc's check computes
// them, of the forms the closure's rounds may hold; ok is false where cc
// has no answer.  env answers an atom the evaluator does not know (a seed
// read, when the caller counts one), and may be nil.
func (x *xfold) xconst(n *Node) (xvalue, xtype, bool) {
	if n == nil {
		return xvalue{}, xtype{}, false
	}
	if !n.list {
		return x.xatom(n)
	}
	args := n.Args()
	h := n.Head()
	switch {
	case h == "paren" && len(args) == 1:
		return x.xconst(args[0])
	case strings.HasPrefix(h, "sizeof") || strings.HasPrefix(h, "alignof"):
		if len(args) != 1 {
			return xvalue{}, xtype{}, false
		}
		v, ok := x.sizeofValue(n)
		return xvalue{v: v, unsigned: true}, xULong, ok
	case h == "macro":
		v, ok := x.offsetofValue(n)
		return xvalue{v: v, unsigned: true}, xULong, ok
	case h == "comma" && len(args) > 0:
		return x.xconst(args[len(args)-1])
	case h == "=" && len(args) == 2:
		// cc's: the value stored, as the left side's type
		t := xtypeOfType(x.typeNode(args[0]))
		v, _, ok := x.xconst(args[1])
		if !ok || t.size == 0 {
			return xvalue{}, xtype{}, false
		}
		v, ok = v.convert(t)
		return v, t, ok
	case h == "cast" && len(args) == 2:
		t := xtypeOfForm(args[0])
		if t.size == 0 {
			return xvalue{}, xtype{}, false
		}
		v, _, ok := x.xconst(args[1])
		if !ok {
			return xvalue{}, xtype{}, false
		}
		v, ok = v.convert(t)
		return v, t, ok
	case len(args) == 1 && (h == "-" || h == "+" || h == "~"):
		v, t, ok := x.xconst(args[0])
		if !ok || !t.integer() {
			return xvalue{}, xtype{}, false
		}
		t = promote(t)
		if v, ok = v.convert(t); !ok {
			return xvalue{}, xtype{}, false
		}
		switch h {
		case "-":
			v.v = -v.v
		case "~":
			v.v = ^v.v
		}
		v, ok = v.convert(t)
		return v, t, ok
	case len(args) == 1 && h == "!":
		v, t, ok := x.xconst(args[0])
		if !ok || t.size == 0 {
			return xvalue{}, xtype{}, false
		}
		return xvalue{v: b2i(v.zero())}, xInt, true
	case h == "?" && len(args) == 3:
		c, ct, ok := x.xconst(args[0])
		if !ok || ct.size == 0 {
			return xvalue{}, xtype{}, false
		}
		b := args[2]
		if !c.zero() {
			b = args[1]
		}
		v, t, ok := x.xconst(b)
		if !ok {
			return xvalue{}, xtype{}, false
		}
		// the conditional's type: both branches' usual conversion
		_, ta, oka := x.xconst(args[1])
		_, tb, okb := x.xconst(args[2])
		if oka && okb && ta.integer() && tb.integer() {
			t = usual(ta, tb)
		}
		v, ok = v.convert(t)
		return v, t, ok
	case (h == "&&" || h == "||") && len(args) >= 2:
		// cc's: the left operand decides alone where it can
		l, lt, ok := x.xconst(args[0])
		if !ok || lt.size == 0 {
			return xvalue{}, xtype{}, false
		}
		acc := !l.zero()
		for _, a := range args[1:] {
			if h == "&&" && !acc || h == "||" && acc {
				continue
			}
			r, rt, ok := x.xconst(a)
			if !ok || rt.size == 0 {
				return xvalue{}, xtype{}, false
			}
			acc = !r.zero()
		}
		return xvalue{v: b2i(acc)}, xInt, true
	case len(args) >= 2 && binaryOps[h]:
		l, lt, ok := x.xconst(args[0])
		if !ok {
			return xvalue{}, xtype{}, false
		}
		for _, a := range args[1:] {
			r, rt, ok := x.xconst(a)
			if !ok {
				return xvalue{}, xtype{}, false
			}
			if l, lt, ok = xbinary(h, l, lt, r, rt); !ok {
				return xvalue{}, xtype{}, false
			}
		}
		return l, lt, true
	}
	return xvalue{}, xtype{}, false
}

// xatom is an atom's constant value: a literal, an enumerator, nullptr, a
// value the closure wrote.
func (x *xfold) xatom(n *Node) (xvalue, xtype, bool) {
	t := n.Atom
	switch {
	case t == "":
		return xvalue{}, xtype{}, false
	case t[0] >= '0' && t[0] <= '9':
		if strings.ContainsAny(t, ".pP") || strings.ContainsAny(t, "eE") && !strings.HasPrefix(t, "0x") && !strings.HasPrefix(t, "0X") {
			return xvalue{}, xtype{}, false
		}
		v, ty, ok := litType(t)
		return v, ty, ok
	case t[0] == '\'':
		c, ok := parseChar(t[1 : len(t)-1])
		if !ok {
			return xvalue{}, xtype{}, false
		}
		v, _ := xvalue{v: c}.convert(xtype{size: 1, signed: true}) // char is signed here
		return v, xInt, true
	case t == "nullptr" && n.Ref() == nil:
		return xvalue{unsigned: true}, xPtr, true
	case (t == "true" || t == "false") && n.Ref() == nil:
		return xvalue{v: b2i(t == "true")}, xtype{size: 1, bool_: true}, true
	}
	if m, ok := headerConstants[t]; ok && n.Ref() == nil {
		return m.v, m.t, true
	}
	if d := n.Ref(); d != nil {
		if v, ok := x.enumerator(d); ok {
			return xvalue{v: v}, xInt, true
		}
	}
	return xvalue{}, xtype{}, false
}

// enumerator is the value of d when it is an enumerator.
func (x *xfold) enumerator(d *Node) (int64, bool) {
	if v, ok := x.enumVals[d]; ok {
		return v, true
	}
	p := x.e.Parent(d)
	if p == nil || !p.Is("enum") {
		return 0, false
	}
	var prev int64 = -1
	for _, m := range body(p) {
		if !m.list || m.Is("@") {
			continue
		}
		if v := enumValue(m); v != nil {
			c, _, ok := x.xconst(v)
			if !ok {
				return 0, false
			}
			prev = c.v
		} else {
			prev++
		}
		x.enumVals[m] = prev
		if m == d {
			return prev, true
		}
	}
	return 0, false
}

// xbinary is cc's evaluation of one binary operation.
func xbinary(op string, l xvalue, lt xtype, r xvalue, rt xtype) (xvalue, xtype, bool) {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		t1, t2 := lt, rt
		if lt.arithmetic() && rt.arithmetic() {
			if lt.float || rt.float {
				return xvalue{}, xtype{}, false
			}
			t1 = usual(lt, rt)
			t2 = t1
		}
		if t1.size == 0 || t2.size == 0 {
			return xvalue{}, xtype{}, false
		}
		x, ok := l.convert(t1)
		if !ok {
			return xvalue{}, xtype{}, false
		}
		y, ok := r.convert(t2)
		if !ok {
			return xvalue{}, xtype{}, false
		}
		var res bool
		switch {
		case op == "==" || op == "!=":
			res = x.v == y.v
			if op == "!=" {
				res = !res
			}
		case x.unsigned != y.unsigned:
			return xvalue{}, xtype{}, false // cc has no answer
		case x.unsigned:
			a, b := uint64(x.v), uint64(y.v)
			res = op == "<" && a < b || op == ">" && a > b || op == "<=" && a <= b || op == ">=" && a >= b
		default:
			a, b := x.v, y.v
			res = op == "<" && a < b || op == ">" && a > b || op == "<=" && a <= b || op == ">=" && a >= b
		}
		return xvalue{v: b2i(res)}, xInt, true
	case "<<", ">>":
		if !lt.integer() || !rt.integer() {
			return xvalue{}, xtype{}, false
		}
		t := promote(lt)
		x, ok := l.convert(t)
		if !ok || !r.unsigned && r.v < 0 {
			return xvalue{}, xtype{}, false
		}
		s := uint64(r.v)
		if s >= 64 {
			x.v = 0
		} else if op == "<<" {
			x.v <<= s
		} else if x.unsigned {
			x.v = int64(uint64(x.v) >> s)
		} else {
			x.v >>= s
		}
		x, ok = x.convert(t)
		return x, t, ok
	}
	if !lt.integer() || !rt.integer() {
		return xvalue{}, xtype{}, false
	}
	t := usual(lt, rt)
	x, ok1 := l.convert(t)
	y, ok2 := r.convert(t)
	if !ok1 || !ok2 {
		return xvalue{}, xtype{}, false
	}
	var v int64
	switch op {
	case "+":
		v = x.v + y.v
	case "-":
		v = x.v - y.v
	case "*":
		v = x.v * y.v
	case "/", "%":
		if y.v == 0 {
			return xvalue{}, xtype{}, false
		}
		switch {
		case t.signed && op == "/":
			v = x.v / y.v
		case t.signed:
			v = x.v % y.v
		case op == "/":
			v = int64(uint64(x.v) / uint64(y.v))
		default:
			v = int64(uint64(x.v) % uint64(y.v))
		}
	case "&":
		v = x.v & y.v
	case "|":
		v = x.v | y.v
	case "^":
		v = x.v ^ y.v
	default:
		return xvalue{}, xtype{}, false
	}
	res, ok := xvalue{v: v, unsigned: !t.signed}.convert(t)
	return res, t, ok
}

// ---- the parentheses C needs (crefactor/clisp's levels)
//
// A form carries its grouping: the parentheses precedence asks for are not
// in the forms, the C view writes them.  The text closure rewrites a span
// and leaves the parentheses around it, and moves an operand with the ones
// it carried: so a replacement of what was parenthesised is parenthesised,
// and an operand moved from where it needed them keeps them, as `(paren
// ...)`.

const (
	xlvComma = iota + 1
	xlvAssign
	xlvCond
	xlvLOr
	xlvLAnd
	xlvOr
	xlvXor
	xlvAnd
	xlvEq
	xlvRel
	xlvShift
	xlvAdd
	xlvMul
	xlvCast
	xlvUnary
	xlvPostfix
	xlvPrimary
)

var xBinaryLevel = map[string]int{
	"||": xlvLOr, "&&": xlvLAnd, "|": xlvOr, "^": xlvXor, "&": xlvAnd,
	"==": xlvEq, "!=": xlvEq, "<": xlvRel, ">": xlvRel, "<=": xlvRel, ">=": xlvRel,
	"<<": xlvShift, ">>": xlvShift, "+": xlvAdd, "-": xlvAdd, "*": xlvMul, "/": xlvMul, "%": xlvMul,
	"=": xlvAssign, "*=": xlvAssign, "/=": xlvAssign, "%=": xlvAssign, "+=": xlvAssign, "-=": xlvAssign,
	"<<=": xlvAssign, ">>=": xlvAssign, "&=": xlvAssign, "^=": xlvAssign, "|=": xlvAssign,
}

// formLevel is an expression form's level (clisp's level).
func formLevel(n *Node) int {
	if !n.list {
		return xlvPrimary
	}
	h := n.Head()
	switch {
	case h == "comma":
		return xlvComma
	case h == "?":
		return xlvCond
	case h == "cast":
		return xlvCast
	case (h == "-" || h == "+") && len(n.Kids) == 2:
		return xlvUnary
	}
	switch h {
	case "addr", "deref", "!", "~", "pre++", "pre--", "sizeof", "sizeof-bare", "sizeof-type",
		"alignof", "alignof-bare", "alignof-type", "label-addr":
		return xlvUnary
	case "call", "index", ".", "->", "post++", "post--", "literal":
		return xlvPostfix
	}
	if l, ok := xBinaryLevel[h]; ok {
		return l
	}
	return xlvPrimary
}

// placeLevel is the level element i of p asks for (clisp's printer).
func placeLevel(p *Node, i int) int {
	h := p.Head()
	if lv, ok := xBinaryLevel[h]; ok && len(p.Kids) >= 3 {
		switch {
		case lv == xlvAssign && i == 1:
			return xlvUnary
		case lv == xlvAssign:
			return xlvAssign
		case i == 1:
			return lv
		}
		return lv + 1
	}
	switch h {
	case "paren", "sizeof", "alignof":
		return xlvComma
	case "call":
		if i == 1 {
			return xlvPostfix
		}
		return xlvAssign
	case "index":
		if i == 1 {
			return xlvPostfix
		}
		return xlvComma
	case ".", "->", "post++", "post--":
		return xlvPostfix
	case "pre++", "pre--", "sizeof-bare", "alignof-bare":
		return xlvUnary
	case "addr", "deref", "!", "~", "-", "+", "cast":
		return xlvCast
	case "?":
		switch i {
		case 1:
			return xlvLOr
		case 2:
			return xlvComma
		}
		return xlvCond
	case "comma", "generic":
		return xlvAssign
	case "case", "case-range", "static_assert":
		return xlvCond
	case "def", "init", "at", "array":
		return xlvAssign
	}
	return xlvComma
}

// parenthesised says the C view writes n, where it stands, in parentheses.
func (e *Editor) parenthesised(n *Node) bool {
	p, i := e.index(n)
	if p == nil || e.isTop(p) || !isExpr(n) {
		return false
	}
	want := placeLevel(p, i)
	if q := e.Parent(p); q != nil && q.Is("enum") {
		want = xlvCond // an enumerator's value
	}
	return formLevel(n) < want
}

// carried is n as the text moves it: in the parentheses it stood in.
func carried(paren bool, n *Node) *Node {
	if paren {
		return NewList(NewAtom("paren"), n)
	}
	return n
}

// ---- sizes (cc's ABI for linux/amd64, as far as a constant asks)

// typeLayout is a type node's size and alignment: the basic types', a
// pointer's, an array's, a struct's and a union's from their members laid
// out (no bit-fields: no answer for one), an enum's int.
func (x *xfold) typeLayout(t *Node, depth int) (size, align int64, ok bool) {
	if t == nil || depth > 32 {
		return 0, 0, false
	}
	switch {
	case t.Is("pointer"):
		return 8, 8, true
	case t.Is("enum"), t.Is("extern-enum"):
		return 4, 4, true
	case t.Is("function"):
		return 1, 1, true
	case t.Is("basic"):
		var ws []string
		for _, k := range t.Kids[1:] {
			ws = append(ws, k.Atom)
		}
		w := strings.Join(ws, " ")
		switch {
		case w == "void", w == "_Bool", strings.HasSuffix(w, "char"):
			return 1, 1, true
		case strings.HasSuffix(w, "short"):
			return 2, 2, true
		case w == "long double":
			return 16, 16, true
		case strings.Contains(w, "long"), w == "double":
			return 8, 8, true
		case strings.HasSuffix(w, "int"), w == "unsigned", w == "float":
			return 4, 4, true
		}
		return 0, 0, false
	case t.Is("array"):
		if len(t.Kids) != 3 || t.Kids[1].list {
			return 0, 0, false
		}
		n, err := strconv.ParseInt(t.Kids[1].Atom, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		s, a, ok := x.typeLayout(pointee(t), depth+1)
		return n * s, a, ok
	case t.Is("struct"), t.Is("union"):
		var size, align int64 = 0, 1
		for _, m := range members(t) {
			if m.Is("static_assert") {
				continue
			}
			for _, k := range m.Kids {
				if k.Is("bits") {
					return 0, 0, false
				}
			}
			s, a, ok := x.typeLayout(m.Type, depth+1)
			if !ok {
				return 0, 0, false
			}
			if t.Is("union") {
				size = max(size, s)
			} else {
				size = (size+a-1)/a*a + s
			}
			align = max(align, a)
		}
		return (size + align - 1) / align * align, align, true
	}
	return 0, 0, false
}

// offsetOf is a member's offset in the struct s.
func (x *xfold) offsetOf(s *Node, name string) (int64, bool) {
	if !s.Is("struct") {
		return 0, false
	}
	var off int64
	for _, m := range members(s) {
		if m.Is("static_assert") {
			continue
		}
		sz, a, ok := x.typeLayout(m.Type, 1)
		if !ok {
			// a flexible array member, last: its offset is still known
			if m.Type.Is("array") && len(m.Type.Kids) == 2 && declName(m) == name {
				if _, a2, ok2 := x.typeLayout(pointee(m.Type), 1); ok2 {
					return (off + a2 - 1) / a2 * a2, true
				}
			}
			return 0, false
		}
		off = (off + a - 1) / a * a
		if declName(m) == name {
			return off, true
		}
		off += sz
	}
	return 0, false
}

// formTypeNode is the type node a type form names, where the graph holds
// one: a typedef's, a tag's definition.
func formTypeNode(t *Node) *Node {
	for depth := 0; t != nil && depth < 64; depth++ {
		if !t.list {
			if d := t.Ref(); d != nil && d.Type != nil {
				return d.Type
			}
			return nil
		}
		switch t.Head() {
		case "struct", "union", "enum":
			if d := t.Ref(); d != nil {
				return d
			}
			return nil
		case "paren", "spec":
			t = t.Kids[1]
			continue
		}
		var named *Node
		for _, k := range t.Kids {
			if !k.list && k.Ref() != nil || k.list && !isAttrForm(k) {
				named = k
			}
		}
		if named == nil {
			return nil
		}
		t = named
	}
	return nil
}

// sizeofValue is a sizeof's or an alignof's value, as cc gives it.
func (x *xfold) sizeofValue(n *Node) (int64, bool) {
	h := n.Head()
	var t *Node
	if strings.HasSuffix(h, "-type") {
		if w := xtypeOfForm(n.Kids[1]); w.integer() || w.ptr {
			s := int64(w.size)
			return s, s > 0
		}
		t = formTypeNode(n.Kids[1])
	} else {
		t = x.typeNode(n.Kids[1])
	}
	s, a, ok := x.typeLayout(t, 0)
	if strings.HasPrefix(h, "alignof") {
		return a, ok
	}
	return s, ok
}

// offsetofValue is `offsetof(T, m)`'s value: the macro's text, and the
// member by the edge the invocation keeps to it.
func (x *xfold) offsetofValue(n *Node) (int64, bool) {
	if len(n.Kids) != 2 || n.Kids[1].list {
		return 0, false
	}
	s := strings.Trim(n.Kids[1].Atom, `"`)
	if !strings.HasPrefix(s, "offsetof(") || !strings.HasSuffix(s, ")") {
		return 0, false
	}
	parts := strings.Split(s[len("offsetof("):len(s)-1], ",")
	if len(parts) != 2 {
		return 0, false
	}
	member := strings.TrimSpace(parts[1])
	for _, r := range n.Refs {
		// the invocation refers to the member its expansion names
		if declName(r) == member && isMember(x.e, r) {
			return x.offsetOf(x.e.Parent(r), member)
		}
	}
	return 0, false
}

// headerConstants are the integer macros of the standard headers, as their
// invocations stand in the forms -- an atom with no edge -- with the values
// and types cc's headers give them on linux/amd64 (LP64).
var headerConstants = func() map[string]struct {
	v xvalue
	t xtype
} {
	m := map[string]struct {
		v xvalue
		t xtype
	}{}
	add := func(name string, v int64, t xtype) {
		x, _ := xvalue{v: v, unsigned: !t.signed}.convert(t)
		m[name] = struct {
			v xvalue
			t xtype
		}{x, t}
	}
	add("CHAR_BIT", 8, xInt)
	add("SCHAR_MIN", -128, xInt)
	add("SCHAR_MAX", 127, xInt)
	add("UCHAR_MAX", 255, xInt)
	add("CHAR_MIN", -128, xInt)
	add("CHAR_MAX", 127, xInt)
	add("SHRT_MIN", -32768, xInt)
	add("SHRT_MAX", 32767, xInt)
	add("USHRT_MAX", 65535, xInt)
	add("INT_MIN", -1<<31, xInt)
	add("INT_MAX", 1<<31-1, xInt)
	add("UINT_MAX", 1<<32-1, xUInt)
	for _, p := range []string{"LONG", "LLONG", "PTRDIFF", "INTPTR", "INTMAX", "INT64", "SSIZE"} {
		add(p+"_MIN", -1<<63, xLong)
		add(p+"_MAX", 1<<63-1, xLong)
	}
	for _, p := range []string{"ULONG", "ULLONG", "SIZE", "UINTPTR", "UINTMAX", "UINT64"} {
		add(p+"_MAX", -1, xULong)
	}
	add("INT8_MIN", -128, xInt)
	add("INT8_MAX", 127, xInt)
	add("UINT8_MAX", 255, xInt)
	add("INT16_MIN", -32768, xInt)
	add("INT16_MAX", 32767, xInt)
	add("UINT16_MAX", 65535, xInt)
	add("INT32_MIN", -1<<31, xInt)
	add("INT32_MAX", 1<<31-1, xInt)
	add("UINT32_MAX", 1<<32-1, xUInt)
	add("EOF", -1, xInt)
	return m
}()
