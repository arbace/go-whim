package sweep

import (
	"strconv"
	"strings"
)

// evalConst evaluates an enumerator's value expression the way the compiler
// would, over what such expressions are written with here: integer and
// character constants, enumerators already seen, the arithmetic, bitwise,
// shift, comparison, logical and conditional operators, parentheses, and casts
// to a builtin integer type.  Anything else -- sizeof, a cast to a typedef, a
// name it does not know -- is not an answer, and it says so: the caller keeps
// what it would otherwise have had to pin.
func evalConst(s string, env map[string]int64) (int64, bool) {
	toks, ok := lexConst(s)
	if !ok {
		return 0, false
	}
	p := &cparser{toks: toks, env: env}
	v, ok := p.cond()
	if !ok || p.i != len(p.toks) {
		return 0, false
	}
	return v, true
}

type cparser struct {
	toks []string
	i    int
	env  map[string]int64
}

func lexConst(s string) ([]string, bool) {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case isSpace(c):
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (isIdent(s[j])) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		case isIdent(c):
			j := i
			for j < len(s) && isIdent(s[j]) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		case c == '\'':
			j := i + 1
			for j < len(s) && s[j] != '\'' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(s) {
				return nil, false
			}
			out = append(out, s[i:j+1])
			i = j + 1
		default:
			for _, op := range []string{"<<", ">>", "<=", ">=", "==", "!=", "&&", "||"} {
				if strings.HasPrefix(s[i:], op) {
					out = append(out, op)
					i += 2
					goto next
				}
			}
			if !strings.ContainsRune("+-*/%&|^~!<>?:()", rune(c)) {
				return nil, false
			}
			out = append(out, string(c))
			i++
		next:
		}
	}
	return out, true
}

func (p *cparser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *cparser) cond() (int64, bool) {
	c, ok := p.binary(0)
	if !ok || p.peek() != "?" {
		return c, ok
	}
	p.i++
	a, ok := p.cond()
	if !ok || p.peek() != ":" {
		return 0, false
	}
	p.i++
	b, ok := p.cond()
	if !ok {
		return 0, false
	}
	if c != 0 {
		return a, true
	}
	return b, true
}

var precedence = map[string]int{
	"||": 1, "&&": 2, "|": 3, "^": 4, "&": 5,
	"==": 6, "!=": 6, "<": 7, ">": 7, "<=": 7, ">=": 7,
	"<<": 8, ">>": 8, "+": 9, "-": 9, "*": 10, "/": 10, "%": 10,
}

func (p *cparser) binary(min int) (int64, bool) {
	l, ok := p.unary()
	if !ok {
		return 0, false
	}
	for {
		op := p.peek()
		prec, isOp := precedence[op]
		if !isOp || prec <= min {
			return l, true
		}
		p.i++
		r, ok := p.binary(prec)
		if !ok {
			return 0, false
		}
		switch op {
		case "||":
			l = b2i(l != 0 || r != 0)
		case "&&":
			l = b2i(l != 0 && r != 0)
		case "|":
			l |= r
		case "^":
			l ^= r
		case "&":
			l &= r
		case "==":
			l = b2i(l == r)
		case "!=":
			l = b2i(l != r)
		case "<":
			l = b2i(l < r)
		case ">":
			l = b2i(l > r)
		case "<=":
			l = b2i(l <= r)
		case ">=":
			l = b2i(l >= r)
		case "<<":
			l <<= uint(r)
		case ">>":
			l >>= uint(r)
		case "+":
			l += r
		case "-":
			l -= r
		case "*":
			l *= r
		case "/":
			if r == 0 {
				return 0, false
			}
			l /= r
		case "%":
			if r == 0 {
				return 0, false
			}
			l %= r
		}
	}
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

var intKeywords = map[string]bool{"int": true, "unsigned": true, "signed": true, "long": true, "short": true}

func (p *cparser) unary() (int64, bool) {
	switch t := p.peek(); t {
	case "-", "+", "~", "!":
		p.i++
		v, ok := p.unary()
		switch t {
		case "-":
			v = -v
		case "~":
			v = ^v
		case "!":
			v = b2i(v == 0)
		}
		return v, ok
	case "(":
		// A cast to a builtin integer type as wide as the value: nothing to do.
		j := p.i + 1
		for j < len(p.toks) && intKeywords[p.toks[j]] {
			j++
		}
		if j > p.i+1 && j < len(p.toks) && p.toks[j] == ")" {
			p.i = j + 1
			return p.unary()
		}
		p.i++
		v, ok := p.cond()
		if !ok || p.peek() != ")" {
			return 0, false
		}
		p.i++
		return v, true
	case "":
		return 0, false
	}
	t := p.toks[p.i]
	p.i++
	switch {
	case t[0] >= '0' && t[0] <= '9':
		return parseInt(t)
	case t[0] == '\'':
		return parseChar(t[1 : len(t)-1])
	}
	v, ok := p.env[t]
	return v, ok
}

func parseInt(t string) (int64, bool) {
	t = strings.TrimRight(t, "uUlL")
	var u uint64
	var err error
	switch {
	case strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X"):
		u, err = strconv.ParseUint(t[2:], 16, 64)
	case strings.HasPrefix(t, "0b") || strings.HasPrefix(t, "0B"):
		u, err = strconv.ParseUint(t[2:], 2, 64)
	case len(t) > 1 && t[0] == '0':
		u, err = strconv.ParseUint(t[1:], 8, 64)
	default:
		u, err = strconv.ParseUint(t, 10, 64)
	}
	return int64(u), err == nil
}

func parseChar(s string) (int64, bool) {
	if len(s) == 1 {
		return int64(s[0]), true
	}
	if len(s) < 2 || s[0] != '\\' {
		return 0, false
	}
	switch s[1] {
	case 'n':
		return '\n', true
	case 't':
		return '\t', true
	case 'r':
		return '\r', true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		v, err := strconv.ParseUint(s[1:], 8, 8)
		return int64(v), err == nil
	case 'x':
		v, err := strconv.ParseUint(s[2:], 16, 8)
		return int64(v), err == nil
	case '\\', '\'', '"', '?':
		return int64(s[1]), true
	case 'a':
		return 7, true
	case 'b':
		return 8, true
	case 'f':
		return 12, true
	case 'v':
		return 11, true
	case 'e':
		return 27, true
	}
	return 0, false
}
