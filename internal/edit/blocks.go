package edit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
)

// The structural verbs: blocks found by brace matching and cut or rewritten
// whole.  Each was one phase's and is here because it is a verb and not that
// phase's argument; the phase it was written for is named above it.

// From phase 071.
// FoldWalk turns `for ((v) = firstbuf; ...)` and its block into `v = curbuf;`
// followed by the block's Body, n times.
//
// IT REFUSES A BODY WITH A `break` OR `continue` THAT BINDS TO THE WALK.
// Deleting the `for` header rebinds such a statement to whatever encloses it,
// or to nothing at all -- getout()'s walk did exactly that and produced "break
// statement not within loop or switch".  It is the same hazard CLAUDE.md
// records for unwrapping `do { } while (0)`, and it must be a refusal rather
// than a silent miscompile.
//
// The test is "is there an enclosing loop or switch INSIDE the body", not "is
// it at brace depth 0": C binds break to the nearest enclosing loop or switch
// and brace depth has nothing to do with it -- getout()'s break sits two ifs
// deep and still bound to the `for`.  An earlier version tested depth and would
// have passed it.
func (e *E) FoldWalk(fn, v, head string, n int, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		pat := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(head) + `[ \t]*\n([ \t]*)\{\n`)
		if k := len(pat.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the walk matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := pat.FindSubmatchIndex(e.buf)
			b := cutil.Blank(e.buf)
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := cutil.Match(b, o)
			raw := e.buf[IndexFrom(e.buf, []byte("\n"), o)+1 : LastNewlineBefore(e.buf, c)+1]
			if bad := BindsToWalk(raw); bad != "" {
				e.Die("%s -- the body has a `%s;` that binds to the walk being removed, not to anything inside it", what, bad)
				return
			}
			end := IndexFrom(e.buf, []byte("\n"), c) + 1
			out := append([]byte{}, e.buf[:m[0]]...)
			out = append(out, e.buf[m[2]:m[3]]...)
			out = append(out, (v + " = curbuf;\n")...)
			out = append(out, raw...)
			e.buf = append(out, e.buf[end:]...)
		}
		e.Say(what)
	})
}

// From phase 071.
// DropWalk replaces `for (<head>)` and the block it runs with repl, n times.
//
// The head is matched as a REGEX built from the literal, never compared as one:
// these walk lines were macro-expanded and carried a trailing space after the
// closing paren, and a literal written out in a shell heredoc is a bad place to
// depend on invisible whitespace -- one written that way already failed here.
func (e *E) DropWalk(fn, head, repl string, n int, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		pat := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(head) + `[ \t]*\n[ \t]*\{\n`)
		if k := len(pat.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the walk matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := pat.FindIndex(e.buf)
			b := cutil.Blank(e.buf)
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := cutil.Match(b, o)
			end := IndexFrom(e.buf, []byte("\n"), c) + 1
			out := append([]byte{}, e.buf[:m[0]]...)
			out = append(out, repl...)
			e.buf = append(out, e.buf[end:]...)
		}
		e.Say(what)
	})
}

// From phase 072.
// FoldWalks folds every walk of one shape, back to front so the offsets still to
// be processed stay valid, refusing if any Body's break or continue would rebind
// to the loop being removed.  It is the one act whose count is computed -- ok
// chooses the walks from their captures -- and it reports the count it folded.
func (e *E) FoldWalks(headRe string, ok func([]string) bool, subst func([]string) string, what string) {
	if e.Err != nil {
		return
	}
	pat := regexp.MustCompile(`(?m)^([ \t]*)` + headRe + `[ \t]*\n[ \t]*\{\n`)
	var ms [][]int
	for _, m := range pat.FindAllSubmatchIndex(e.buf, -1) {
		g := make([]string, len(m)/2)
		for i := range g {
			if m[2*i] >= 0 {
				g[i] = string(e.buf[m[2*i]:m[2*i+1]])
			}
		}
		if ok(g) {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		e.Die("%s -- no walk of this shape is left to fold", what)
		return
	}
	var unsafe []int
	total := len(ms)
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		b := cutil.Blank(e.buf)
		o := IndexFrom(e.buf, []byte("{"), m[0])
		c := cutil.Match(b, o)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		raw := e.buf[IndexFrom(e.buf, []byte("\n"), o)+1 : LastNewlineBefore(e.buf, c)+1]
		if BindsToWalk(raw) != "" {
			unsafe = append(unsafe, 1+CountNewlines(e.buf[:m[0]]))
			continue
		}
		g := make([]string, len(m)/2)
		for k := range g {
			if m[2*k] >= 0 {
				g[k] = string(e.buf[m[2*k]:m[2*k+1]])
			}
		}
		end := IndexFrom(e.buf, []byte("\n"), c) + 1
		out := append([]byte{}, e.buf[:m[0]]...)
		out = append(out, e.buf[m[2]:m[3]]...)
		out = append(out, (subst(g) + "\n")...)
		out = append(out, raw...)
		e.buf = append(out, e.buf[end:]...)
	}
	if len(unsafe) > 0 {
		e.Die("%s -- %d walk(s) still carry an escaping break/continue and need an explicit rewrite: lines %s",
			what, len(unsafe), JoinInts(unsafe))
		return
	}
	e.Say(fmt.Sprintf("%s (%d)", what, total))
}

// From phase 072.
// ReplaceBlock replaces a brace-matched block, anchored on the line that opens it.
func (e *E) ReplaceBlock(fn, anchorRe, repl, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		m := regexp.MustCompile(anchorRe).FindIndex(e.buf)
		if m == nil {
			e.Die("%s -- no line matches %s", what, cutil.PyRepr(anchorRe))
			return
		}
		k := LastNewlineBefore(e.buf, m[0]) + 1
		b := cutil.Blank(e.buf)
		o := IndexFrom(e.buf, []byte("{"), m[0])
		c := cutil.Match(b, o)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		out := append([]byte{}, e.buf[:k]...)
		out = append(out, repl...)
		e.buf = append(out, e.buf[IndexFrom(e.buf, []byte("\n"), c)+1:]...)
	})
	if !e.Failed() {
		e.Say(what)
	}
}

// From phase 075.
// DropBareBlock deletes the innermost block enclosing a statement, REFUSING if
// that block still does real work -- anything but the statement itself and bare
// declarations.  It is for the husk a removed call leaves behind.
func (e *E) DropBareBlock(fn, stmt, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		b := cutil.Blank(e.buf)
		i := IndexFrom(e.buf, []byte(stmt), 0)
		if i < 0 {
			e.Die("%s -- %s is not in %s", what, cutil.PyRepr(stmt), fn)
			return
		}
		depth, j := 0, i
		for ; j >= 0; j-- {
			if b[j] == '}' {
				depth++
			} else if b[j] == '{' {
				if depth == 0 {
					break
				}
				depth--
			}
		}
		if j < 0 {
			e.Die("%s -- no enclosing block", what)
			return
		}
		c := cutil.Match(b, j)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		body := e.buf[IndexFrom(e.buf, []byte("\n"), j)+1 : LastNewlineBefore(e.buf, c)+1]
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == stmt || BareDeclOnly.MatchString(line) {
				continue
			}
			if len(line) > 60 {
				line = line[:60]
			}
			e.Die("%s -- the block still does real work: %s", what, cutil.PyRepr(line))
			return
		}
		k := LastNewlineBefore(e.buf, j) + 1
		e.Say(what)
		out := append([]byte{}, e.buf[:k]...)
		e.buf = append(out, e.buf[IndexFrom(e.buf, []byte("\n"), c)+1:]...)
	})
}

// From phase 079.
// InnerBody is a definition's Body between the brace on its own line and the
// closing brace -- NOT the definition, which carries the parameter list.
func (e *E) InnerBody(name string) (string, bool) {
	def, ok := e.BodyOf(name)
	if !ok {
		return "", false
	}
	s := string(def)
	i := strings.Index(s, "{\n")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return "", false
	}
	return s[i+2 : j], true
}

// From phase 079.
// ConstOf requires name's whole Body to be `return <expect>;` and nothing else.
func (e *E) ConstOf(name, expect string) {
	if e.Failed() {
		return
	}
	inner, ok := e.InnerBody(name)
	if !ok {
		e.Refuse("%s is not defined", name)
		return
	}
	inner = strings.TrimSpace(inner)
	m := returnStub.FindStringSubmatch(inner)
	if m == nil {
		if len(inner) > 70 {
			inner = inner[:70]
		}
		e.Refuse("%s is no longer a one-line stub: %s", name, pyRepr(inner))
		return
	}
	if got := strings.TrimSpace(m[1]); got != expect {
		e.Refuse("%s returns %s, not %s -- folding it would change behaviour", name, pyRepr(got), pyRepr(expect))
	}
}
