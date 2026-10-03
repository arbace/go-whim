package graph

import (
	"strconv"
	"strings"
)

// evalForm evaluates an enumerator's value form as Prune evaluates its text
// (crefactor/sweep's evalConst): integer and character constants,
// enumerators already seen, the arithmetic, bitwise, shift, comparison,
// logical and conditional operators, parentheses, and casts to a builtin
// integer type spelled with int, unsigned, signed, long and short.
// Anything else is not an answer.
func evalForm(n *Node, env map[string]int64) (int64, bool) {
	if n == nil {
		return 0, false
	}
	if !n.list {
		t := n.Atom
		switch {
		case t == "":
			return 0, false
		case t[0] >= '0' && t[0] <= '9':
			return parseInt(t)
		case t[0] == '\'':
			return parseChar(t[1 : len(t)-1])
		}
		v, ok := env[t]
		return v, ok
	}
	args := n.Args()
	switch h := n.Head(); {
	case h == "paren" && len(args) == 1:
		return evalForm(args[0], env)
	case h == "cast" && len(args) == 2:
		if !intCast(args[0]) {
			return 0, false
		}
		return evalForm(args[1], env)
	case len(args) == 1 && (h == "-" || h == "+" || h == "~" || h == "!"):
		v, ok := evalForm(args[0], env)
		switch h {
		case "-":
			v = -v
		case "~":
			v = ^v
		case "!":
			v = b2i(v == 0)
		}
		return v, ok
	case h == "?" && len(args) == 3:
		c, ok := evalForm(args[0], env)
		if !ok {
			return 0, false
		}
		a, ok := evalForm(args[1], env)
		if !ok {
			return 0, false
		}
		b, ok := evalForm(args[2], env)
		if !ok {
			return 0, false
		}
		if c != 0 {
			return a, true
		}
		return b, true
	case len(args) >= 2 && binaryOps[h]:
		l, ok := evalForm(args[0], env)
		if !ok {
			return 0, false
		}
		for _, a := range args[1:] {
			r, ok := evalForm(a, env)
			if !ok {
				return 0, false
			}
			if l, ok = binary(h, l, r); !ok {
				return 0, false
			}
		}
		return l, true
	}
	return 0, false
}

var binaryOps = map[string]bool{
	"||": true, "&&": true, "|": true, "^": true, "&": true, "==": true, "!=": true,
	"<": true, ">": true, "<=": true, ">=": true, "<<": true, ">>": true,
	"+": true, "-": true, "*": true, "/": true, "%": true,
}

func binary(op string, l, r int64) (int64, bool) {
	switch op {
	case "||":
		return b2i(l != 0 || r != 0), true
	case "&&":
		return b2i(l != 0 && r != 0), true
	case "|":
		return l | r, true
	case "^":
		return l ^ r, true
	case "&":
		return l & r, true
	case "==":
		return b2i(l == r), true
	case "!=":
		return b2i(l != r), true
	case "<":
		return b2i(l < r), true
	case ">":
		return b2i(l > r), true
	case "<=":
		return b2i(l <= r), true
	case ">=":
		return b2i(l >= r), true
	case "<<":
		return l << uint(r), true
	case ">>":
		return l >> uint(r), true
	case "+":
		return l + r, true
	case "-":
		return l - r, true
	case "*":
		return l * r, true
	case "/":
		if r == 0 {
			return 0, false
		}
		return l / r, true
	case "%":
		if r == 0 {
			return 0, false
		}
		return l % r, true
	}
	return 0, false
}

// intCast says a cast's type is words Prune's evaluator skips.
func intCast(t *Node) bool {
	words := []*Node{t}
	if t.list {
		words = t.Kids
	}
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		switch {
		case w.list:
			return false
		case w.Atom == "int", w.Atom == "unsigned", w.Atom == "signed", w.Atom == "long", w.Atom == "short":
		default:
			return false
		}
	}
	return true
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
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
