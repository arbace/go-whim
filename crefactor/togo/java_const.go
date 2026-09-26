package togo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// CONSTANTS AS THE C SPELLS THEM.  C's constant arithmetic is not Java's --
// unsigned types, the usual arithmetic conversions -- so the backend knows a
// constant expression's value from the C (cc's Value) and wrote that value.
// It writes the C's spelling instead -- `P_STRING | P_VI_DEF`, `case ESC:` --
// when Java's evaluation of what it writes gives the same value: jconst
// evaluates the printed Java by the Java language's rules for a constant
// expression (JLS 15.29), the names being the enumerators' fields, and a
// spelling it cannot evaluate, or that evaluates to anything else, is
// written as the value, as before.  javac folds a constant expression, so the
// class it writes is the one the value gave (`whim java --same-classes`).

// jnum is a Java constant's value and type: an int (size 4), a long (8) or a
// boolean (0), v sign-extended.
type jnum struct {
	v    int64
	size int
}

// constText is the constant expression e, of C value cv and kind k, as the C
// spells it, and whether Java's value of that spelling is jlit(cv, k)'s --
// an int's where k is a long, which Java widens, being one; n is the
// spelling's own value and type.  It
// tries only an expression of names, literals and operators (constOnly);
// what it prints on the way is undone when it fails.
func (f *jfn) constText(e cc.ExpressionNode, k jk, cv int64, to string) (n jnum, s string, ok bool) {
	if k.boolean || k.size != 4 && k.size != 8 || !constOnly(e) || !constNamed(e) {
		return jnum{}, "", false
	}
	mark := len(f.j.newEnums)
	defer func() {
		if r := recover(); r != nil {
			if _, is := r.(unsupported); !is {
				panic(r)
			}
			ok = false
		}
		if !ok {
			for _, n := range f.j.newEnums[mark:] {
				delete(f.j.enums, n)
			}
			f.j.newEnums = f.j.newEnums[:mark]
		}
	}()
	v := f.exprTo1(e, to)
	if v.null || v.t != k.java() {
		return jnum{}, "", false
	}
	n, good := jconst(v.s, f.j.enumVal)
	if !good || n.size == 0 || n.size > k.size || n.v != jwant(cv, k) {
		return jnum{}, "", false
	}
	return n, v.s, true
}

// jwant is the value jlit(cv, k) writes, sign-extended.
func jwant(cv int64, k jk) int64 {
	switch k.size {
	case 1:
		return int64(int8(cv))
	case 2:
		return int64(int16(cv))
	case 4:
		return int64(int32(cv))
	}
	return cv
}

// constOnly says e is made of integer and character constants, enumerators,
// parentheses and the operators of a constant expression: what jconst can
// check once printed, and what prints with no effect -- no sizeof, no string,
// no comma, no object.
func constOnly(e cc.Node) bool {
	switch x := e.(type) {
	case *cc.ConstantExpression:
		return constOnly(x.ConditionalExpression)
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionInt, cc.PrimaryExpressionChar:
			return true
		case cc.PrimaryExpressionIdent:
			_, ok := x.ResolvedTo().(*cc.Enumerator)
			return ok
		case cc.PrimaryExpressionExpr:
			return x.ExpressionList != nil && constOnly(x.ExpressionList)
		}
		return false
	case *cc.ExpressionList:
		return x.ExpressionList == nil && constOnly(x.AssignmentExpression)
	case *cc.AssignmentExpression:
		return x.Case == cc.AssignmentExpressionCond && constOnly(x.ConditionalExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionLOr {
			return constOnly(x.LogicalOrExpression)
		}
		return x.Case == cc.ConditionalExpressionCond && constOnly(x.LogicalOrExpression) && x.ExpressionList != nil && constOnly(x.ExpressionList) && constOnly(x.ConditionalExpression)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return constOnly(x.UnaryExpression)
		}
		_, ok := scalarKind(x.Type())
		return ok && constOnly(x.CastExpression)
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return constOnly(x.PostfixExpression)
		case cc.UnaryExpressionPlus, cc.UnaryExpressionMinus, cc.UnaryExpressionCpl, cc.UnaryExpressionNot:
			return constOnly(x.CastExpression)
		}
		return false
	case *cc.PostfixExpression:
		return x.Case == cc.PostfixExpressionPrimary && constOnly(x.PrimaryExpression)
	case *cc.MultiplicativeExpression:
		if x.Case == cc.MultiplicativeExpressionCast {
			return constOnly(x.CastExpression)
		}
		return constOnly(x.MultiplicativeExpression) && constOnly(x.CastExpression)
	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionMul {
			return constOnly(x.MultiplicativeExpression)
		}
		return constOnly(x.AdditiveExpression) && constOnly(x.MultiplicativeExpression)
	case *cc.ShiftExpression:
		if x.Case == cc.ShiftExpressionAdd {
			return constOnly(x.AdditiveExpression)
		}
		return constOnly(x.ShiftExpression) && constOnly(x.AdditiveExpression)
	case *cc.RelationalExpression:
		if x.Case == cc.RelationalExpressionShift {
			return constOnly(x.ShiftExpression)
		}
		return constOnly(x.RelationalExpression) && constOnly(x.ShiftExpression)
	case *cc.EqualityExpression:
		if x.Case == cc.EqualityExpressionRel {
			return constOnly(x.RelationalExpression)
		}
		return constOnly(x.EqualityExpression) && constOnly(x.RelationalExpression)
	case *cc.AndExpression:
		if x.Case == cc.AndExpressionEq {
			return constOnly(x.EqualityExpression)
		}
		return constOnly(x.AndExpression) && constOnly(x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		if x.Case == cc.ExclusiveOrExpressionAnd {
			return constOnly(x.AndExpression)
		}
		return constOnly(x.ExclusiveOrExpression) && constOnly(x.AndExpression)
	case *cc.InclusiveOrExpression:
		if x.Case == cc.InclusiveOrExpressionXor {
			return constOnly(x.ExclusiveOrExpression)
		}
		return constOnly(x.InclusiveOrExpression) && constOnly(x.ExclusiveOrExpression)
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionOr {
			return constOnly(x.InclusiveOrExpression)
		}
		return constOnly(x.LogicalAndExpression) && constOnly(x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLAnd {
			return constOnly(x.LogicalAndExpression)
		}
		return constOnly(x.LogicalOrExpression) && constOnly(x.LogicalAndExpression)
	}
	return false
}

// constNamed says e holds an enumerator or a character constant: a spelling
// that says more than its value.  An expression of numbers alone (-1,
// 1024 * 1024) is written as its value, as it was.
func constNamed(e cc.Node) bool {
	found := false
	var walk func(cc.Node)
	walk = func(n cc.Node) {
		if found || n == nil {
			return
		}
		if x, ok := n.(*cc.PrimaryExpression); ok && (x.Case == cc.PrimaryExpressionIdent || x.Case == cc.PrimaryExpressionChar) {
			found = true
			return
		}
		walkChildrenFn(n, walk)
	}
	walk(e)
	return found
}

// jcharLit is the C character constant of value cv as a Java char literal of
// the same value, and whether there is one: ASCII, escaped as the C escapes
// it, a control character in octal.
func jcharLit(cv int64) (string, bool) {
	if cv < 0 || cv > 0x7f {
		return "", false
	}
	switch cv {
	case '\\':
		return `'\\'`, true
	case '\'':
		return `'\''`, true
	case '\t':
		return `'\t'`, true
	case '\n':
		return `'\n'`, true
	case '\r':
		return `'\r'`, true
	case '\b':
		return `'\b'`, true
	case '\f':
		return `'\f'`, true
	}
	if cv < 0x20 || cv == 0x7f {
		return fmt.Sprintf(`'\%o'`, cv), true
	}
	return "'" + string(rune(cv)) + "'", true
}

// jconst evaluates s, a Java expression, as javac folds a constant
// expression: int and long arithmetic in two's complement, shifts by their
// distance's low 5 or 6 bits, casts to the integer types, comparisons and
// the logical operators to a boolean.  A name is an enumerator's field, its
// value from names.  ok is false for anything else -- a call, a field of an
// object, a division by zero -- which is then not a constant, or not one
// this knows.
func jconst(s string, names map[string]jnum) (n jnum, ok bool) {
	p := &jparser{names: names}
	if !p.lex(s) {
		return jnum{}, false
	}
	defer func() {
		if r := recover(); r != nil {
			if _, is := r.(jconstFail); !is {
				panic(r)
			}
			ok = false
		}
	}()
	n = p.cond()
	if p.i != len(p.toks) {
		return jnum{}, false
	}
	return n, true
}

type jconstFail struct{}

type jparser struct {
	toks  []string
	i     int
	names map[string]jnum
}

func (p *jparser) fail() { panic(jconstFail{}) }

// lex splits s into tokens: numbers, character literals, names and
// operators.
func (p *jparser) lex(s string) bool {
	ops := []string{">>>", "<<", ">>", "<=", ">=", "==", "!=", "&&", "||",
		"+", "-", "*", "/", "%", "&", "|", "^", "~", "!", "<", ">", "?", ":", "(", ")"}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ':
			i++
		case c == '\'':
			j := i + 1
			for j < len(s) && s[j] != '\'' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(s) {
				return false
			}
			p.toks = append(p.toks, s[i:j+1])
			i = j + 1
		case c >= '0' && c <= '9', c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i
			for j < len(s) && (s[j] == '_' || s[j] == '.' || s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
				j++
			}
			p.toks = append(p.toks, s[i:j])
			i = j
		default:
			found := false
			for _, o := range ops {
				if strings.HasPrefix(s[i:], o) {
					p.toks = append(p.toks, o)
					i += len(o)
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func (p *jparser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *jparser) next() string {
	t := p.peek()
	if t == "" {
		p.fail()
	}
	p.i++
	return t
}

func (p *jparser) expect(t string) {
	if p.next() != t {
		p.fail()
	}
}

// jwiden is the binary numeric promotion of a and b: both ints or both
// longs; a boolean is no number.
func jwiden(a, b jnum) int {
	if a.size == 0 || b.size == 0 {
		panic(jconstFail{})
	}
	if a.size == 8 || b.size == 8 {
		return 8
	}
	return 4
}

// jfit is v as a value of size: an int wraps to 32 bits.
func jfit(v int64, size int) jnum {
	if size == 4 {
		return jnum{int64(int32(v)), 4}
	}
	return jnum{v, size}
}

func jbool(b bool) jnum {
	if b {
		return jnum{1, 0}
	}
	return jnum{0, 0}
}

func (p *jparser) cond() jnum {
	c := p.binary(0)
	if p.peek() != "?" {
		return c
	}
	p.next()
	if c.size != 0 {
		p.fail()
	}
	a := p.cond()
	p.expect(":")
	b := p.cond()
	if a.size == 0 || b.size == 0 {
		if a.size != b.size {
			p.fail()
		}
	} else if w := jwiden(a, b); true {
		a, b = jfit(a.v, w), jfit(b.v, w)
	}
	if c.v != 0 {
		return a
	}
	return b
}

// jlevels are Java's binary operators from the loosest to the tightest.
var jlevels = [][]string{
	{"||"}, {"&&"}, {"|"}, {"^"}, {"&"}, {"==", "!="}, {"<", ">", "<=", ">="},
	{"<<", ">>", ">>>"}, {"+", "-"}, {"*", "/", "%"},
}

func (p *jparser) binary(level int) jnum {
	if level == len(jlevels) {
		return p.unary()
	}
	a := p.binary(level + 1)
	for {
		op := p.peek()
		in := false
		for _, o := range jlevels[level] {
			in = in || o == op
		}
		if !in {
			return a
		}
		p.next()
		b := p.binary(level + 1)
		a = p.apply(op, a, b)
	}
}

func (p *jparser) apply(op string, a, b jnum) jnum {
	switch op {
	case "||", "&&":
		if a.size != 0 || b.size != 0 {
			p.fail()
		}
		if op == "||" {
			return jbool(a.v != 0 || b.v != 0)
		}
		return jbool(a.v != 0 && b.v != 0)
	case "==", "!=":
		if a.size == 0 && b.size == 0 {
			return jbool((a.v == b.v) == (op == "=="))
		}
	case "&", "|", "^":
		if a.size == 0 && b.size == 0 {
			switch op {
			case "&":
				return jbool(a.v != 0 && b.v != 0)
			case "|":
				return jbool(a.v != 0 || b.v != 0)
			}
			return jbool((a.v != 0) != (b.v != 0))
		}
	case "<<", ">>", ">>>":
		if a.size == 0 || b.size == 0 {
			p.fail()
		}
		d := uint(b.v) & 31
		if a.size == 8 {
			d = uint(b.v) & 63
		}
		switch op {
		case "<<":
			return jfit(a.v<<d, a.size)
		case ">>":
			return jfit(a.v>>d, a.size)
		}
		if a.size == 4 {
			return jfit(int64(uint32(a.v)>>d), 4)
		}
		return jnum{int64(uint64(a.v) >> d), 8}
	}
	w := jwiden(a, b)
	x, y := jfit(a.v, w).v, jfit(b.v, w).v
	switch op {
	case "==":
		return jbool(x == y)
	case "!=":
		return jbool(x != y)
	case "<":
		return jbool(x < y)
	case ">":
		return jbool(x > y)
	case "<=":
		return jbool(x <= y)
	case ">=":
		return jbool(x >= y)
	case "&":
		return jfit(x&y, w)
	case "|":
		return jfit(x|y, w)
	case "^":
		return jfit(x^y, w)
	case "+":
		return jfit(x+y, w)
	case "-":
		return jfit(x-y, w)
	case "*":
		return jfit(x*y, w)
	case "/", "%":
		if y == 0 {
			p.fail() // not a constant expression: javac evaluates it at run time
		}
		if w == 4 && x == -1<<31 && y == -1 || w == 8 && x == -1<<63 && y == -1 {
			if op == "/" {
				return jfit(x, w)
			}
			return jfit(0, w)
		}
		if op == "/" {
			return jfit(x/y, w)
		}
		return jfit(x%y, w)
	}
	p.fail()
	return jnum{}
}

// jcasts are the casts a constant's spelling may hold: to what size, and
// how the value is cut.
var jcasts = map[string]func(int64) jnum{
	"int":   func(v int64) jnum { return jnum{int64(int32(v)), 4} },
	"long":  func(v int64) jnum { return jnum{v, 8} },
	"short": func(v int64) jnum { return jnum{int64(int16(v)), 4} },
	"byte":  func(v int64) jnum { return jnum{int64(int8(v)), 4} },
	"char":  func(v int64) jnum { return jnum{int64(uint16(v)), 4} },
}

func (p *jparser) unary() jnum {
	switch t := p.peek(); t {
	case "+", "-", "~", "!":
		p.next()
		a := p.unary()
		switch t {
		case "!":
			if a.size != 0 {
				p.fail()
			}
			return jbool(a.v == 0)
		case "+":
			if a.size == 0 {
				p.fail()
			}
			return a
		case "-":
			if a.size == 0 {
				p.fail()
			}
			return jfit(-a.v, a.size)
		}
		if a.size == 0 {
			p.fail()
		}
		return jfit(^a.v, a.size)
	case "(":
		if p.i+2 < len(p.toks) && p.toks[p.i+2] == ")" {
			if cast, ok := jcasts[p.toks[p.i+1]]; ok {
				p.i += 3
				a := p.unary()
				if a.size == 0 {
					p.fail()
				}
				return cast(a.v)
			}
		}
		p.next()
		a := p.cond()
		p.expect(")")
		return a
	}
	return p.primary()
}

func (p *jparser) primary() jnum {
	t := p.next()
	switch {
	case t[0] == '\'':
		body := t[1 : len(t)-1]
		if len(body) == 1 && body != "\\" {
			return jnum{int64(body[0]), 4}
		}
		if len(body) < 2 || body[0] != '\\' {
			p.fail()
		}
		switch body[1:] {
		case "\\":
			return jnum{'\\', 4}
		case "'":
			return jnum{'\'', 4}
		case "t":
			return jnum{'\t', 4}
		case "n":
			return jnum{'\n', 4}
		case "r":
			return jnum{'\r', 4}
		case "b":
			return jnum{'\b', 4}
		case "f":
			return jnum{'\f', 4}
		}
		v, err := strconv.ParseInt(body[1:], 8, 64)
		if err != nil || v > 0377 {
			p.fail()
		}
		return jnum{v, 4}
	case t[0] >= '0' && t[0] <= '9':
		size := 4
		if strings.HasSuffix(t, "L") || strings.HasSuffix(t, "l") {
			size, t = 8, t[:len(t)-1]
		}
		var u uint64
		var err error
		switch {
		case strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X"):
			u, err = strconv.ParseUint(t[2:], 16, 64)
			if err == nil && size == 4 && u > 0xffffffff {
				p.fail()
			}
		case len(t) > 1 && t[0] == '0':
			p.fail() // octal: the printer writes none
		default:
			u, err = strconv.ParseUint(t, 10, 64)
			// a decimal int literal is at most 2147483648, and that one only
			// after a minus; the printer writes -2147483648 as such, so a
			// larger one is not what it means
			if err == nil && (size == 4 && u > 1<<31 || size == 8 && u > 1<<63) {
				p.fail()
			}
		}
		if err != nil {
			p.fail()
		}
		return jfit(int64(u), size)
	case t == "true":
		return jbool(true)
	case t == "false":
		return jbool(false)
	}
	if n, ok := p.names[t]; ok {
		return n
	}
	p.fail()
	return jnum{}
}
