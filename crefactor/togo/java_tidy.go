package togo

import "strings"

// PARENTHESES BY PRECEDENCE.  The printer parenthesises every compound
// operand (jparen), since a value it builds does not carry its precedence:
// `if ((((State & MODE_INSERT) != 0) || (restart_edit != 0)))` for what a
// Java programmer writes `if ((State & MODE_INSERT) != 0 || restart_edit !=
// 0)`.  jtidy takes the finished Java a line at a time and drops a pair of
// grouping parentheses where Java's precedence gives the expression the same
// tree without them -- and keeps, as Java programmers do, a && inside a ||,
// a bitwise operator beside another or beside arithmetic, and a comparison
// beside an equality.  What it removes, javac reads the same: the classes
// are the same code (`whim java --same-classes`), which is the proof.
//
// A pair it cannot classify -- a call's, a cast's, an if's, a lambda's
// parameters, anything holding a comma -- it leaves.

// Java's binary operators by precedence, loosest first; the unary operators
// and casts are 13, a primary and its postfix operators 14.
var jprec = map[string]int{
	"=": 1, "+=": 1, "-=": 1, "*=": 1, "/=": 1, "%=": 1, "&=": 1, "|=": 1, "^=": 1, "<<=": 1, ">>=": 1, ">>>=": 1,
	"?": 2, ":": 2,
	"||": 3, "&&": 4, "|": 5, "^": 6, "&": 7,
	"==": 8, "!=": 8,
	"<": 9, ">": 9, "<=": 9, ">=": 9, "instanceof": 9,
	"<<": 10, ">>": 10, ">>>": 10,
	"+": 11, "-": 11,
	"*": 12, "/": 12, "%": 12,
}

const (
	precUnary   = 13
	precPrimary = 14
)

// the operators and punctuation, longest first
var jops = []string{">>>=", "<<=", ">>=", ">>>", "->", "::", "==", "!=", "<=", ">=", "&&", "||", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>",
	"+", "-", "*", "/", "%", "&", "|", "^", "!", "~", "<", ">", "=", "?", ":", "(", ")", "[", "]", "{", "}", ",", ";", ".", "@"}

var jprimitive = map[string]bool{"int": true, "long": true, "short": true, "byte": true, "char": true, "boolean": true, "float": true, "double": true}

// a keyword whose parentheses are its syntax, not a grouping
var jsyntax = map[string]bool{"if": true, "while": true, "for": true, "switch": true, "catch": true, "synchronized": true, "try": true}

type jtok struct {
	s          string
	start, end int
}

// jtidyFile is jtidy on every line of a Java file.
func jtidyFile(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = jtidy(l)
	}
	return strings.Join(lines, "\n")
}

// jtidy is the line of Java l without the grouping parentheses precedence
// makes redundant; l itself when it cannot read it.
func jtidy(l string) string {
	t := strings.TrimSpace(l)
	if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") || !strings.Contains(l, "(") {
		return l
	}
	toks, ok := jlex(l)
	if !ok {
		return l
	}
	match := make([]int, len(toks))
	var stack []int // braces are a block's or an initializer's: not matched here
	for i, tk := range toks {
		match[i] = -1
		switch tk.s {
		case "(", "[":
			stack = append(stack, i)
		case ")", "]":
			if len(stack) == 0 {
				return l // a line that closes what another opened: left as it is
			}
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if (toks[o].s == "(") != (tk.s == ")") {
				return l
			}
			match[o], match[i] = i, o
		}
	}
	if len(stack) > 0 {
		return l
	}
	gone := make([]bool, len(toks))
	cast := make([]bool, len(toks)) // an open parenthesis that is a cast's
	for i, tk := range toks {
		if tk.s == "(" && jisCast(toks, i, match[i]) {
			cast[i] = true
		}
	}
	// the groups innermost first: in the order of their closing parenthesis
	for c := range toks {
		if toks[c].s != ")" {
			continue
		}
		o := match[c]
		if cast[o] || !jgrouping(toks, o, cast, gone) {
			continue
		}
		if jredundant(toks, o, c, match, cast, gone) {
			gone[o], gone[c] = true, true
		}
	}
	var b strings.Builder
	at := 0
	for i, tk := range toks {
		if gone[i] {
			b.WriteString(l[at:tk.start])
			at = tk.end
		}
	}
	b.WriteString(l[at:])
	return b.String()
}

// jlex splits a line of Java into tokens; false for what it does not know.
func jlex(l string) ([]jtok, bool) {
	var toks []jtok
	for i := 0; i < len(l); {
		c := l[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(l) && l[j] != c {
				if l[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(l) {
				return nil, false
			}
			toks = append(toks, jtok{l[i : j+1], i, j + 1})
			i = j + 1
		case c == '/' && i+1 < len(l) && (l[i+1] == '/' || l[i+1] == '*'):
			return nil, false // a comment: the line is left as it is
		case c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			j := i
			for j < len(l) && (l[j] == '_' || l[j] == '$' || l[j] >= 'a' && l[j] <= 'z' || l[j] >= 'A' && l[j] <= 'Z' || l[j] >= '0' && l[j] <= '9') {
				j++
			}
			toks = append(toks, jtok{l[i:j], i, j})
			i = j
		default:
			found := false
			for _, o := range jops {
				if strings.HasPrefix(l[i:], o) {
					toks = append(toks, jtok{o, i, i + len(o)})
					i += len(o)
					found = true
					break
				}
			}
			if !found {
				return nil, false
			}
		}
	}
	return toks, true
}

func jisIdent(s string) bool {
	c := s[0]
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func jisOperand(s string) bool {
	c := s[0]
	return jisIdent(s) || c >= '0' && c <= '9' || c == '"' || c == '\''
}

// jisCast says the parenthesis at o, closed at c, is a cast: a primitive
// type, or a type's name -- dotted, an array's, a generic's -- followed by
// what begins an operand.
func jisCast(toks []jtok, o, c int) bool {
	if c < 0 || c == o+1 {
		return false
	}
	in := toks[o+1 : c]
	if len(in) == 1 && jprimitive[in[0].s] {
		return true
	}
	for _, t := range in {
		if !jisIdent(t.s) && t.s != "." && t.s != "[" && t.s != "]" && t.s != "<" && t.s != ">" && t.s != "," && t.s != "?" {
			return false
		}
	}
	if len(in) == 1 && jprimitive[in[0].s] || !jisIdent(in[0].s) {
		return false
	}
	if c+1 >= len(toks) {
		return false
	}
	n := toks[c+1].s
	return jisOperand(n) || n == "(" || n == "!" || n == "~"
}

// jgrouping says the parenthesis at o is a grouping: not a call's, a
// declaration's, a syntax's or a cast's.
func jgrouping(toks []jtok, o int, cast, gone []bool) bool {
	p := jprev(toks, o, gone)
	if p < 0 {
		return true
	}
	s := toks[p].s
	switch {
	case s == "return" || s == "throw" || s == "case" || s == "else":
		return true
	case jsyntax[s]:
		return false
	case jisIdent(s) || s[0] >= '0' && s[0] <= '9':
		return false // a call, a declaration, a constructor
	case s == "]" || s == ">":
		return false // an array's constructor; a generic's (new Ptr<>(...))
	case s == ")":
		// after a cast, an operand; after a call or a grouping, nothing Java has
		return cast[jmatchBack(toks, p)]
	}
	return true
}

// jmatchBack is the open parenthesis the ) at c closes.
func jmatchBack(toks []jtok, c int) int {
	d := 0
	for i := c; i >= 0; i-- {
		switch toks[i].s {
		case ")":
			d++
		case "(":
			d--
			if d == 0 {
				return i
			}
		}
	}
	return 0
}

func jprev(toks []jtok, i int, gone []bool) int {
	for i--; i >= 0 && gone[i]; i-- {
	}
	return i
}

func jnext(toks []jtok, i int, gone []bool) int {
	for i++; i < len(toks) && gone[i]; i++ {
	}
	return i
}

// jinner is the precedence of the expression between o and c -- the lowest
// of its operators outside any parentheses left -- and those operators.
func jinner(toks []jtok, o, c int, cast, gone []bool) (int, map[string]bool) {
	prec := precPrimary
	ops := map[string]bool{}
	depth := 0
	prevOperand := false // the token before ends an operand
	for i := o + 1; i < c; i++ {
		if gone[i] {
			continue
		}
		s := toks[i].s
		switch s {
		case "{", "}":
			return 0, ops // an array's initializer: left as it is
		case "(", "[":
			if depth == 0 && s == "(" && cast[i] {
				// a cast: the operand after it is a unary expression's
				if prec > precUnary {
					prec = precUnary
				}
			}
			depth++
			continue
		case ")", "]":
			depth--
			prevOperand = !(s == ")" && cast[jmatchBack(toks, i)])
			continue
		}
		if depth > 0 {
			continue
		}
		switch {
		case s == "->":
			return 0, ops
		case s == ",", s == ";":
			return 0, ops
		case s == "." || s == "::" || s == "new":
			prevOperand = false
			continue
		case s == "++" || s == "--":
			continue // postfix, or prefix: either binds tighter than a binary operator
		case (s == "-" || s == "+" || s == "!" || s == "~") && !prevOperand:
			if prec > precUnary {
				prec = precUnary
			}
			continue
		}
		if p, ok := jprec[s]; ok {
			if p < prec {
				prec = p
				ops = map[string]bool{}
			}
			if p == prec {
				ops[s] = true
			}
			prevOperand = false
			continue
		}
		prevOperand = true
	}
	return prec, ops
}

// jredundant says the grouping parentheses at o and c can go.
func jredundant(toks []jtok, o, c int, match []int, cast, gone []bool) bool {
	prec, ops := jinner(toks, o, c, cast, gone)
	if prec == 0 || c == o+1 {
		return false
	}
	p, n := jprev(toks, o, gone), jnext(toks, c, gone)
	// the left side
	if p >= 0 {
		s := toks[p].s
		switch {
		case s == "(" || s == "[" || s == "," || s == "{" || s == ";" || s == "return" || s == "throw" || s == "->" || s == "else":
			// an argument, an index, a whole expression
		case s == "case":
			if prec <= 2 {
				return false
			}
		case s == ")":
			// the operand of a cast: a primary, or after a primitive type's
			// cast, a unary expression too -- (long) (int) x, (int) -x
			q := jmatchBack(toks, p)
			if prec < precUnary || prec == precUnary && !(q+2 == p && jprimitive[toks[q+1].s]) {
				return false
			}
		case jprec[s] == 1:
			// the right side of an assignment
		case (s == "-" || s == "+" || s == "!" || s == "~" || s == "++" || s == "--") && jisPrefix(toks, p, cast, gone):
			if prec < precPrimary || toks[jnext(toks, o, gone)].s == s || toks[jnext(toks, o, gone)].s == "-" || toks[jnext(toks, o, gone)].s == "+" {
				return false
			}
		default:
			lp, ok := jprec[s]
			if !ok || prec <= lp || jconfusing(ops, prec, s) {
				return false
			}
		}
	}
	// the right side
	if n < len(toks) {
		s := toks[n].s
		switch {
		case s == ")" || s == "]" || s == "," || s == ";" || s == "}":
		case s == ":":
			// the end of a case label, or a conditional's middle
			if prec <= 2 {
				return false
			}
		case s == "." || s == "[" || s == "++" || s == "--" || s == "::":
			if prec < precPrimary {
				return false
			}
		case s == "?":
			if prec <= 2 {
				return false
			}
		default:
			rp, ok := jprec[s]
			if !ok || rp == 1 {
				return false
			}
			if prec < rp || prec == rp && rp <= 2 || jconfusing(ops, prec, s) {
				return false
			}
		}
	}
	return true
}

// jisPrefix says the operator at p is a prefix one: nothing that ends an
// operand is before it.
func jisPrefix(toks []jtok, p int, cast, gone []bool) bool {
	q := jprev(toks, p, gone)
	if q < 0 {
		return true
	}
	s := toks[q].s
	if s == ")" {
		return cast[jmatchBack(toks, q)]
	}
	return !(jisOperand(s) || s == "]") || s == "return" || s == "case" || s == "throw"
}

// the operators' kinds, for what a reader would not want to sort out by
// precedence alone
func jkind(op string) string {
	switch op {
	case "&&", "||":
		return "logical"
	case "&", "|", "^":
		return "bitwise"
	case "<<", ">>", ">>>":
		return "shift"
	case "+", "-", "*", "/", "%":
		return "arith"
	case "==", "!=", "<", ">", "<=", ">=", "instanceof":
		return "compare"
	}
	return ""
}

// jconfusing says an expression of operators ops, at precedence prec, keeps
// its parentheses beside op though precedence would not need them: a &&
// inside a ||, a bitwise operator beside another or beside arithmetic or a
// shift, a shift beside arithmetic, a comparison beside another.
func jconfusing(ops map[string]bool, prec int, op string) bool {
	if prec >= precUnary {
		return false
	}
	ko := jkind(op)
	for in := range ops {
		ki := jkind(in)
		switch {
		case in == "&&" && op == "||":
			return true
		case ki == "bitwise" && (ko == "bitwise" && in != op || ko == "arith" || ko == "shift"):
			return true
		case ko == "bitwise" && (ki == "arith" || ki == "shift" || ki == "bitwise" && in != op):
			return true
		case ki == "shift" && ko == "arith" || ko == "shift" && ki == "arith":
			return true
		case ki == "compare" && ko == "compare":
			return true
		}
	}
	return false
}
