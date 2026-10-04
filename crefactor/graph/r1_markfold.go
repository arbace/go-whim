package graph

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
	"regexp"
	"strings"
)

// MARKFOLD (doc/GRAPH-MIGRATION.md, *R1 as built*): a text simplifier's
// constant folding, on the forms.  A cut that knows some objects' values
// -- a flag that is always TRUE, a mode that is always 0 -- marks their
// uses as constants and folds what follows: the operators that see a
// constant through themselves (`||`, `&&`, `!`, `?:`, parentheses, a
// comparison of a zero with a zero or a known nonzero name), and the
// statements whose condition is a constant (an if, an else-if arm, a
// while).  It is written to give, form for form, what a text simplifier
// gave that worked on the C a parenthesis group at a time -- the group a
// constant was innermost in, simplified and, when the result was a bare
// constant and the group no call's, a cast's operand or a keyword's,
// replaced by it; a statement's right side where a constant stood in no
// group -- so it keeps that simplifier's limits as well as its rules: a
// constant behind any other operator is left (`x == TRUE`), an assignment
// at the top of a group stops it, an impure operand before a deciding
// constant stays and what follows it goes, and the parentheses the text
// kept around what it simplified stay, as `(paren ...)` where C does not
// need them, as the import of that text has them.
//
// Each statement's expression is rewritten whole where anything in it
// changed (FRAG's granularity, not an act per operator), its unchanged
// operands moved into the new form with their ids; a statement folded is
// the editor's Unwrap, Replace or Delete.

// A Mark is a constant's kind: true, false, or a zero (which tests false
// and compares equal to 0).
type Mark byte

const (
	MarkTrue Mark = 1 + iota
	MarkFalse
	MarkZero
)

// MarkFold is a fold's state: the marked nodes, how a new one is made, and
// what was done.
type MarkFold struct {
	// Marks are the constants, live in the graph; the fold keeps it: a
	// constant it makes is added, one it drops stays (not live).
	Marks map[*Node]Mark
	// Make is a new constant of the kind, not yet placed.
	Make func(Mark) *Node
	// NonZero says a name compared with a zero constant is known not to
	// be zero (an enumerator other than the zero one).
	NonZero func(atom string) bool
	// Exprs counts the groups and statement right sides rewritten, Folds
	// the statements folded.
	Exprs, Folds int
}

// FoldMarks folds the constants in the function fn: its expressions, then
// its statements, until nothing more changes.
func (e *Editor) FoldMarks(f *MarkFold, fn *Node) error {
	for round := 0; ; round++ {
		if round == 20 {
			return fmt.Errorf("markfold: %s does not settle", DeclName(fn))
		}
		x, err := e.markExprs(f, fn)
		if err != nil {
			return fmt.Errorf("markfold: %s: %w", DeclName(fn), err)
		}
		s, err := e.markStmts(f, fn)
		if err != nil {
			return fmt.Errorf("markfold: %s: %w", DeclName(fn), err)
		}
		f.Folds += s
		if x == 0 && s == 0 {
			return nil
		}
	}
}

// ---- the expressions

// mx is an expression as the text saw it: every pair of parentheses a
// group of its own (`(paren ...)`, and the ones C needs, which the forms
// leave implied), a node of the graph where it is unchanged.
type mx struct {
	n     *Node  // the node it is, unchanged (an implied pair has none)
	head  string // a list's head
	kids  []*mx  // a list's elements after the head; a pair's content
	atom  string // an atom's text
	list  bool
	paren bool // a pair of parentheses: kids[0] inside
	word  bool // a pair a word or `)` precedes: a call's, a cast's operand, return's
	mark  Mark // a constant
	opq   bool // not an expression (a type, a member's name): kept as it is
	stop  bool // a declaration's initialiser in a for's clause: the text's piece assigned at its top
}

type mxState struct {
	e   *Editor
	f   *MarkFold
	err error
}

// opaque heads: their elements are not expressions the fold looks into.
var mxOpaque = map[string]bool{"sizeof-type": true, "alignof-type": true, "literal": true,
	"generic": true, "stmt-expr": true, "macro": true, "label-addr": true}

func (st *mxState) build(n *Node, word bool) *mx {
	if !n.list {
		m := &mx{n: n, atom: n.Atom}
		if k, ok := st.f.Marks[n]; ok {
			m.mark = k
		}
		return m
	}
	h := n.Head()
	m := &mx{n: n, head: h, list: true}
	if h == "paren" {
		m.paren, m.word = true, word
		m.kids = []*mx{st.build(n.Kids[1], false)}
		return m
	}
	if mxOpaque[h] {
		m.opq = true
		return m
	}
	args := n.Args()
	m.kids = make([]*mx, len(args))
	for j, a := range args {
		i := j + 1
		var k *mx
		switch {
		case h == "cast" && i == 1, (h == "." || h == "->") && i > 1:
			k = &mx{n: a, opq: true}
		default:
			kw := false // what precedes the element: a word, or not
			switch h {
			case "cast", "sizeof-bare", "alignof-bare":
				kw = true
			case "!", "~", "-", "+", "deref", "addr", "pre++", "pre--", "sizeof", "alignof":
				kw = h == "-" && len(args) > 1 && i == 1 && word || h == "+" && len(args) > 1 && i == 1 && word
			default:
				// the first element printed inherits what precedes the form
				kw = i == 1 && word
			}
			k = st.build(a, kw)
			if isExpr(a) && formLevel(a) < placeLevel(n, i) {
				k = &mx{paren: true, word: kw, kids: []*mx{k}}
			}
		}
		m.kids[j] = k
	}
	return m
}

// marked says m holds a constant anywhere; direct, one no pair holds (a
// pair, a call's arguments, sizeof's operand: the text's groups).
func (st *mxState) marked(m *mx, direct bool) bool {
	if m.mark != 0 {
		return true
	}
	if m.opq {
		if m.n != nil {
			found := false
			Walk(m.n, func(x *Node) bool {
				if _, ok := st.f.Marks[x]; ok {
					found = true
				}
				return !found
			})
			if found && st.err == nil {
				st.err = fmt.Errorf("a constant inside %s, which the fold does not look into", label(m.n))
			}
		}
		return false
	}
	if direct && m.paren {
		return false
	}
	for j, k := range m.kids {
		if direct && (m.head == "call" && j > 0 || m.head == "sizeof" || m.head == "alignof") {
			continue
		}
		if st.marked(k, direct) {
			return true
		}
	}
	return false
}

func (st *mxState) newMark(k Mark) *mx { return &mx{mark: k} }

func (st *mxState) paren(in *mx, word bool) *mx {
	return &mx{paren: true, word: word, kids: []*mx{in}}
}

func (st *mxState) list(head string, kids []*mx) *mx {
	return &mx{head: head, list: true, kids: kids}
}

// mfConstOf is m's constant: a constant, or one in pairs.
func mfConstOf(m *mx) (Mark, bool) {
	for m.paren {
		m = m.kids[0]
	}
	if m.mark != 0 {
		return m.mark, true
	}
	return 0, false
}

func mfTruth(k Mark) bool { return k == MarkTrue }

// pieces simplifies each operand of a comma, as the text split a group at
// its commas.
func (st *mxState) pieces(c *mx) *mx {
	ch := false
	nk := make([]*mx, len(c.kids))
	for j, k := range c.kids {
		nk[j] = st.simplify(k)
		ch = ch || nk[j] != k
	}
	if !ch {
		return c
	}
	return st.list(c.head, nk)
}

var mfAssignHeads = map[string]bool{"=": true, "*=": true, "/=": true, "%=": true, "+=": true, "-=": true,
	"<<=": true, ">>=": true, "&=": true, "^=": true, "|=": true,
	"pre++": true, "pre--": true, "post++": true, "post--": true}

var mxCallRe = regexp.MustCompile(`[\w\]\)]\s*\(`)

// assignTop says m assigns outside every pair and bracket (the text's
// maskDeep).
func (st *mxState) assignTop(m *mx) bool {
	if m.paren {
		return false
	}
	if m.opq {
		if m.n != nil && m.n.Is("macro") {
			return mfMacroAssigns(m.n, true)
		}
		return false
	}
	if mfAssignHeads[m.head] {
		return true
	}
	for j, k := range m.kids {
		switch {
		case m.head == "call" && j > 0, m.head == "index" && j > 0, m.head == "sizeof", m.head == "alignof":
			continue
		}
		if st.assignTop(k) {
			return true
		}
	}
	return false
}

// pure says m neither assigns nor calls, anywhere (the text's pure: no
// assignment, no `++` or `--`, no word, `)` or `]` before a `(`).
func (st *mxState) pure(m *mx) bool {
	if m.opq {
		if m.n == nil {
			return true
		}
		switch m.n.Head() {
		case "macro":
			return !mfMacroAssigns(m.n, false) && !mxCallRe.MatchString(mfBlankText(mfMacroText(m.n)))
		case "sizeof-type", "alignof-type", "generic", "stmt-expr":
			return false
		}
		return true
	}
	if mfAssignHeads[m.head] {
		return false
	}
	switch m.head {
	case "call", "sizeof", "alignof":
		return false
	case "cast", "sizeof-bare", "alignof-bare":
		if m.kids[len(m.kids)-1].paren {
			return false
		}
	}
	for _, k := range m.kids {
		if !st.pure(k) {
			return false
		}
	}
	return true
}

func mfMacroText(n *Node) string {
	if len(n.Kids) > 1 {
		return mfUnquoteAtom(n.Kids[1].Atom)
	}
	return ""
}

func mfUnquoteAtom(s string) string {
	if len(s) >= 2 && s[0] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `\"`, `"`)
	}
	return s
}

// mfBlankText blanks string and character literals, as the text did.
func mfBlankText(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] != '"' && b[i] != '\'' {
			continue
		}
		q := b[i]
		for j := i + 1; j < len(b); j++ {
			if b[j] == '\\' {
				b[j] = ' '
				if j+1 < len(b) {
					b[j+1] = ' '
				}
				j++
				continue
			}
			if b[j] == q {
				i = j
				break
			}
			b[j] = ' '
		}
	}
	return string(b)
}

// mfMacroAssigns says a macro's text assigns: anywhere, or (top) outside
// its parentheses.
func mfMacroAssigns(n *Node, top bool) bool {
	s := mfBlankText(mfMacroText(n))
	if top {
		b := []byte(s)
		d := 0
		for i, c := range b {
			switch c {
			case '(', '[', '{':
				d++
				b[i] = ' '
				continue
			case ')', ']', '}':
				d--
				b[i] = ' '
				continue
			}
			if d != 0 {
				b[i] = ' '
			}
		}
		s = string(b)
	}
	if strings.Contains(s, "++") || strings.Contains(s, "--") {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '=' || i+1 < len(s) && s[i+1] == '=' {
			continue
		}
		start := i
		if i >= 2 && (s[i-2:i] == "<<" || s[i-2:i] == ">>") {
			start = i - 2
		} else if i >= 1 && strings.IndexByte("+-*/%&|^", s[i-1]) >= 0 {
			start = i - 1
		}
		if start > 0 && strings.IndexByte("=!<>", s[start-1]) >= 0 {
			continue
		}
		return true
	}
	return false
}

// mfLeadsWithNot says m's text begins with `!` though m is no `!`: a binary
// form whose first operand, unparenthesised, is one.
func mfLeadsWithNot(m *mx) bool {
	for m.list && !m.paren && !m.opq && len(m.kids) > 0 {
		switch m.head {
		case "!":
			return true
		case "call", "index", ".", "->", "post++", "post--", "?", "comma":
		default:
			if _, ok := xBinaryLevel[m.head]; !ok {
				return false
			}
		}
		m = m.kids[0]
	}
	return false
}

// simplify is the text's simplify on one expression: m, or what it folds
// to.
func (st *mxState) simplify(m *mx) *mx {
	if st.err != nil || !st.marked(m, false) {
		return m
	}
	if m.stop || m.head == "comma" || st.assignTop(m) {
		return m
	}
	switch {
	case m.head == "?":
		if m.kids[1].head == "?" {
			return m
		}
		c := st.simplify(m.kids[0])
		if k, ok := mfConstOf(c); ok {
			if mfTruth(k) {
				return st.simplify(m.kids[1])
			}
			return st.simplify(m.kids[2])
		}
		y, n := st.simplify(m.kids[1]), st.simplify(m.kids[2])
		if c == m.kids[0] && y == m.kids[1] && n == m.kids[2] {
			return m
		}
		return st.list("?", []*mx{c, y, n})
	case m.head == "||" || m.head == "&&":
		stop := m.head == "||"
		var out []*mx
		ch := false
		for j, p := range m.kids {
			p2 := st.simplify(p)
			if k, ok := mfConstOf(p2); ok {
				if mfTruth(k) == !stop {
					ch = true
					continue
				}
				pure := true
				for _, x := range out {
					if !st.pure(x) {
						pure = false
						break
					}
				}
				want := MarkFalse
				if stop {
					want = MarkTrue
				}
				if pure {
					return st.newMark(want)
				}
				// an impure prefix stays, the constant after it, and
				// what follows goes: the same text when the constant
				// was written so and was the last
				if p2 == p && p.mark == want && !p.paren {
					out = append(out, p)
					ch = ch || j != len(m.kids)-1
				} else {
					out = append(out, st.newMark(want))
					ch = true
				}
				break
			}
			ch = ch || p2 != p
			out = append(out, p2)
		}
		switch {
		case len(out) == 0 && stop:
			return st.newMark(MarkFalse)
		case len(out) == 0:
			return st.newMark(MarkTrue)
		case !ch:
			return m
		case len(out) == 1:
			return out[0]
		}
		return st.list(m.head, out)
	case m.head == "!":
		in := st.simplify(m.kids[0])
		if k, ok := mfConstOf(in); ok {
			if mfTruth(k) {
				return st.newMark(MarkFalse)
			}
			return st.newMark(MarkTrue)
		}
		if in == m.kids[0] {
			return m
		}
		return st.list("!", []*mx{in})
	case m.paren:
		in := st.simplify(m.kids[0])
		if k, ok := mfConstOf(in); ok {
			return st.newMark(k)
		}
		if in == m.kids[0] {
			return m
		}
		return st.paren(in, m.word)
	case mfLeadsWithNot(m):
		if st.err == nil {
			st.err = fmt.Errorf("a constant in an expression whose text begins with `!` but is no `!`: %s", m.head)
		}
		return m
	case (m.head == "==" || m.head == "!=") && len(m.kids) == 2:
		a, b := m.kids[0], m.kids[1]
		zero := func(x *mx) bool { return x.mark == MarkZero || !x.list && x.mark == 0 && x.atom == "0" }
		nonzero := func(x *mx) bool {
			return !x.list && x.mark == 0 && st.f.NonZero != nil && st.f.NonZero(x.atom)
		}
		if a.list || b.list || a.mark == MarkTrue || a.mark == MarkFalse || b.mark == MarkTrue || b.mark == MarkFalse {
			return m
		}
		if !(zero(a) && (zero(b) || nonzero(b)) || (nonzero(a) || zero(a)) && b.mark == MarkZero) {
			return m
		}
		equal := zero(a) && zero(b)
		if (m.head == "==") == equal {
			return st.newMark(MarkTrue)
		}
		return st.newMark(MarkFalse)
	}
	return m
}

// back makes m nodes where want is the level its place asks for.
func (st *mxState) back(m *mx, want int) *Node {
	if m.n != nil {
		return m.n // unchanged
	}
	if m.paren {
		c := st.back(m.kids[0], xlvComma)
		if formLevel(c) < want {
			return c
		}
		return NewList(NewAtom("paren"), c)
	}
	if m.mark != 0 && m.n == nil {
		n := st.f.Make(m.mark)
		st.f.Marks[n] = m.mark
		return n
	}
	if !m.list {
		return NewAtom(m.atom)
	}
	n := NewList(NewAtom(m.head))
	n.Kids = append(n.Kids, make([]*Node, len(m.kids))...)
	for j, k := range m.kids {
		n.Kids[j+1] = st.back(k, placeLevel(n, j+1))
	}
	return n
}

// a statement's expressions, and how the text treated them: an if's,
// while's, switch's or do's condition, a for's clauses (one group, split
// at its `;`), a return's value, an initialiser or an expression
// statement (its right side), a case's value (no group of its own).
type mfStmtRoot struct {
	stmt *Node   // the statement
	ns   []*Node // its expressions
	def  []bool  // a for's clause that is a declaration's initialiser
	kind int
}

const (
	mfRootControl = iota
	mfRootFor
	mfRootRight
	mfRootNone
)

// markExprs rewrites fn's statements' expressions that hold a constant;
// the count of the rewrites, as the text counted its rounds: a group or
// a right side each time it changed.
func (e *Editor) markExprs(f *MarkFold, fn *Node) (int, error) {
	var roots []mfStmtRoot
	has := func(n *Node) bool {
		found := false
		if n == nil {
			return false
		}
		Walk(n, func(x *Node) bool {
			if _, ok := f.Marks[x]; ok {
				found = true
			}
			return !found
		})
		return found
	}
	add := func(stmt, n *Node, kind int) {
		if n != nil && isExpr(n) && has(n) {
			roots = append(roots, mfStmtRoot{stmt: stmt, ns: []*Node{n}, def: []bool{false}, kind: kind})
		}
	}
	var walkItems func(items []*Node)
	var stmt func(s *Node)
	stmt = func(s *Node) {
		switch s.Head() {
		case "block":
			walkItems(blockItems(s))
		case "if":
			add(s, s.Kids[1], mfRootControl)
			stmt(s.Kids[2])
			if len(s.Kids) > 3 {
				stmt(s.Kids[3])
			}
		case "while", "switch":
			add(s, s.Kids[1], mfRootControl)
			stmt(s.Kids[2])
		case "do":
			stmt(s.Kids[1])
			add(s, s.Kids[2], mfRootControl)
		case "for":
			r := mfStmtRoot{stmt: s, kind: mfRootFor}
			any := false
			for _, c := range s.Kids[1:4] {
				d := c.Is("def")
				if d {
					c = mfDefInit(c)
				}
				if c == nil || !isExpr(c) {
					continue
				}
				r.ns = append(r.ns, c)
				r.def = append(r.def, d)
				any = any || has(c)
			}
			if any {
				roots = append(roots, r)
			}
			stmt(s.Kids[4])
		case "return":
			if len(s.Kids) > 1 {
				add(s, s.Kids[1], mfRootRight)
			}
		case "def":
			add(s, mfDefInit(s), mfRootRight)
		case "case":
			add(s, s.Kids[1], mfRootNone)
		case "label", "default", "goto", "break", "continue", "empty", "attributed", "stmt-attr", "typedef", "static_assert":
		default:
			if !IsStatement(s) {
				add(s, s, mfRootRight)
			}
		}
	}
	walkItems = func(items []*Node) {
		for _, it := range items {
			stmt(it)
		}
	}
	walkItems(Body(fn))
	n0 := f.Exprs
	for _, r := range roots {
		if err := e.markRoot(f, r); err != nil {
			return 0, err
		}
	}
	return f.Exprs - n0, nil
}

// markRoot rewrites one statement's expressions as the text did: the
// constants in their order, each one's innermost group simplified, and
// at the first that changes, again from the start.
func (e *Editor) markRoot(f *MarkFold, r mfStmtRoot) error {
	st := &mxState{e: e, f: f}
	var root *mx
	var orig []*mx
	if r.kind == mfRootFor {
		root = &mx{head: "for-clauses", list: true}
		for j, n := range r.ns {
			if !e.Live(n) {
				return nil
			}
			k := st.build(n, false)
			k.stop = r.def[j]
			root.kids = append(root.kids, k)
		}
		orig = append(orig, root.kids...)
	} else {
		if !e.Live(r.ns[0]) {
			return nil
		}
		root = st.build(r.ns[0], r.stmt.Is("return") || r.stmt.Is("case"))
		orig = []*mx{root}
	}
	right := -1 // whether the text found the right side: asked once, when it is needed
	for guard := 0; ; guard++ {
		if guard == 10000 {
			return fmt.Errorf("%s does not settle", label(r.stmt))
		}
		changed := false
		for _, path := range st.markerPaths(root) {
			g := mfGroupOf(path)
			if g != nil {
				ng := st.groupRule(g)
				if ng != g {
					root = mfReplaceIn(root, g, ng)
					changed = true
				}
			} else {
				var nr *mx
				switch r.kind {
				case mfRootControl:
					nr = st.whole(root)
				case mfRootFor:
					nr = st.whole(root)
				case mfRootRight:
					if right < 0 {
						right = 0
						if e.mfTextRight(r.stmt, r.stmt.Is("return")) {
							right = 1
						}
					}
					switch {
					case right == 0:
						nr = root
					case r.stmt.Is("return") || r.stmt.Is("def"):
						nr = st.simplify(root)
					default:
						nr = st.right(root)
					}
				default:
					nr = root
				}
				if nr != root {
					root = nr
					changed = true
				}
			}
			if changed {
				break
			}
		}
		if st.err != nil {
			return st.err
		}
		if !changed {
			break
		}
		f.Exprs++
	}
	var outs []*mx
	if r.kind == mfRootFor {
		outs = root.kids
	} else {
		outs = []*mx{root}
	}
	for j, out := range outs {
		if out == orig[j] {
			continue
		}
		n := r.ns[j]
		p, i := e.index(n)
		nn := st.back(out, placeLevel(p, i))
		if err := e.Replace(n, nn); err != nil {
			return err
		}
	}
	return nil
}

// markerPaths are the constants in m, in the text's order, each with the
// forms from m down to it.
func (st *mxState) markerPaths(m *mx) [][]*mx {
	var out [][]*mx
	var walk func(x *mx, path []*mx)
	walk = func(x *mx, path []*mx) {
		path = append(path, x)
		if x.mark != 0 {
			out = append(out, append([]*mx(nil), path...))
			return
		}
		if x.opq {
			return
		}
		for _, k := range x.kids {
			walk(k, path)
		}
	}
	walk(m, nil)
	return out
}

// mfGroupOf is the innermost group around a constant: a pair, a call's
// arguments, sizeof's operand; nil when it stands in none.
func mfGroupOf(path []*mx) *mx {
	for k := len(path) - 2; k >= 0; k-- {
		a, c := path[k], path[k+1]
		switch {
		case a.paren:
			return a
		case a.head == "call" && c != a.kids[0]:
			return a
		case a.head == "sizeof" || a.head == "alignof":
			return a
		}
	}
	return nil
}

// groupRule simplifies a group: a pair's content (and, the pair no word's
// and the content a constant, the constant in its place), a call's
// arguments, sizeof's operand.
func (st *mxState) groupRule(g *mx) *mx {
	switch {
	case g.paren:
		in := g.kids[0]
		var out *mx
		if in.head == "comma" {
			out = st.pieces(in)
		} else {
			out = st.simplify(in)
			if k, ok := mfConstOf(out); ok && !g.word {
				return st.newMark(k)
			}
		}
		if out == in {
			return g
		}
		return st.paren(out, g.word)
	case g.head == "call":
		nk := []*mx{g.kids[0]}
		ch := false
		for _, a := range g.kids[1:] {
			s := st.simplify(a)
			ch = ch || s != a
			nk = append(nk, s)
		}
		if !ch {
			return g
		}
		return st.list("call", nk)
	}
	in := g.kids[0]
	var s *mx
	if in.head == "comma" {
		s = st.pieces(in)
	} else {
		s = st.simplify(in)
	}
	if s == in {
		return g
	}
	return st.list(g.head, []*mx{s})
}

// whole simplifies a control's condition, or a for's clauses each.
func (st *mxState) whole(m *mx) *mx {
	if m.head == "comma" || m.head == "for-clauses" {
		return st.pieces(m)
	}
	return st.simplify(m)
}

// mfReplaceIn is m with old (once, by identity) replaced by new.
func mfReplaceIn(m, old, new *mx) *mx {
	if m == old {
		return new
	}
	if m.opq || len(m.kids) == 0 {
		return m
	}
	for j, k := range m.kids {
		nk := mfReplaceIn(k, old, new)
		if nk != k {
			c := *m
			c.n = nil
			c.kids = append([]*mx(nil), m.kids...)
			c.kids[j] = nk
			return &c
		}
	}
	return m
}

// mfTextRight says the text found this statement's own right side: it
// took a statement from the `;`, `{` or `}` before it at its depth, so a
// statement after a braced one began inside that one's last block, and one
// after a label began at the label; the right side was then the one after
// the first `return ` at the start, or after the first assignment's `=`
// in all that text -- the statement's own only where nothing before it
// in the text had one.
func (e *Editor) mfTextRight(s *Node, ret bool) bool {
	p, i := e.index(s)
	if p == nil {
		return false
	}
	ks := e.kids(p)
	j := i - 1
	labelled := false
	for j >= 0 && (ks[j].Is("label") || ks[j].Is("case") || ks[j].Is("default") || ks[j].Is("case-range") || ks[j].Is("stmt-attr")) {
		labelled = true
		j--
	}
	var b1 *Node // the block the text began in
	if j >= 0 && (p.Is("block") || p.Is("defn")) && e.place(p, j) == placeItem {
		b1 = mfLastBlock(ks[j])
	}
	if b1 == nil {
		return !ret || !labelled
	}
	items := blockItems(b1)
	if len(items) > 0 && items[0].Is("return") && len(items[0].Kids) > 1 {
		return false // `return ` began it
	}
	if ret {
		return false
	}
	ls := make([]*clisp.Node, len(items))
	for k, it := range items {
		ls[k] = Lisp(it)
	}
	text, err := clisp.PrintItems(ls)
	if err != nil {
		return false
	}
	return !mfFindAssign(mfBlankText(text))
}

// mfLastBlock is the block a braced statement's text ends with, or nil.
func mfLastBlock(s *Node) *Node {
	for {
		switch {
		case s.Is("block"):
			return s
		case s.Is("if"):
			s = s.Kids[len(s.Kids)-1]
		case s.Is("while") || s.Is("switch"):
			s = s.Kids[2]
		case s.Is("for"):
			s = s.Kids[4]
		default:
			return nil
		}
	}
}

// mfFindAssign says the text holds an assignment's `=` (the text's
// findAssign: not `==`, `<=`, `>=`, `!=`).
func mfFindAssign(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '=' || i+1 < len(s) && s[i+1] == '=' {
			continue
		}
		start := i
		if i >= 2 && (s[i-2:i] == "<<" || s[i-2:i] == ">>") {
			start = i - 2
		} else if i >= 1 && strings.IndexByte("+-*/%&|^", s[i-1]) >= 0 {
			start = i - 1
		}
		if start > 0 && strings.IndexByte("=!<>", s[start-1]) >= 0 {
			continue
		}
		return true
	}
	return false
}

// right is an expression statement with its right side simplified: the
// right side of the first assignment the text writes, where that runs to
// the statement's end.
func (st *mxState) right(m *mx) *mx {
	// the first assignment in the text's order: the left side first
	var first *mx
	var find func(x *mx) bool
	find = func(x *mx) bool {
		if x.opq || !x.list && !x.paren {
			return false
		}
		if _, ok := xBinaryLevel[x.head]; ok && xBinaryLevel[x.head] == xlvAssign && !x.paren {
			if find(x.kids[0]) {
				return true
			}
			first = x
			return true
		}
		for _, k := range x.kids {
			if find(k) {
				return true
			}
		}
		return false
	}
	if !find(m) {
		return m
	}
	// rebuild the spine down to it
	var spine func(x *mx) *mx
	spine = func(x *mx) *mx {
		if x == first {
			s := st.simplify(x.kids[1])
			if s == x.kids[1] {
				return x
			}
			return st.list(x.head, []*mx{x.kids[0], s})
		}
		if x.paren || x.opq || !x.list || len(x.kids) == 0 {
			return x
		}
		if _, ok := xBinaryLevel[x.head]; !ok && x.head != "comma" && x.head != "?" {
			return x
		}
		last := x.kids[len(x.kids)-1]
		s := spine(last)
		if s == last {
			return x
		}
		nk := append(append([]*mx{}, x.kids[:len(x.kids)-1]...), s)
		return st.list(x.head, nk)
	}
	return spine(m)
}

// ---- the statements

// markStmts folds fn's ifs, else-if arms and whiles whose condition is a
// constant: an if true is its then arm, false its else (or nothing); an
// arm true is the else it makes of its then block, false its chain's
// rest; a while false goes.
func (e *Editor) markStmts(f *MarkFold, fn *Node) (int, error) {
	n := 0
	for {
		var s *Node
		Walk(fn, func(x *Node) bool {
			if s != nil {
				return false
			}
			if (x.Is("if") || x.Is("while")) && len(x.Kids) > 2 {
				if _, ok := f.Marks[x.Kids[1]]; ok {
					s = x
					return false
				}
			}
			return true
		})
		if s == nil {
			return n, nil
		}
		k := f.Marks[s.Kids[1]]
		p, i := e.index(s)
		arm := p != nil && p.Is("if") && i == 3
		if !mfTruth(k) {
			// the text folded a false one's LAST occurrence of the same
			// line first -- an inner one before the one it is in
			var last *Node
			Walk(fn, func(x *Node) bool {
				if x.Head() == s.Head() && len(x.Kids) > 2 && f.Marks[x.Kids[1]] == k {
					q, j := e.index(x)
					if (q != nil && q.Is("if") && j == 3) == arm {
						last = x
					}
				}
				return true
			})
			s = last
		}
		var err error
		switch {
		case s.Is("while") && mfTruth(k):
			return n, fmt.Errorf("while (TRUE) from a constant -- not expected")
		case s.Is("while"):
			err = e.Delete(s)
		case mfTruth(k) && arm:
			err = e.Replace(s, s.Kids[2])
		case mfTruth(k):
			err = e.Unwrap(s, s.Kids[2])
		case len(s.Kids) < 4:
			err = e.Delete(s)
		default:
			err = e.Unwrap(s, s.Kids[3])
		}
		if err != nil {
			return n, err
		}
		n++
	}
}

// HasStorage says a def's prefix holds the word w (`static`, `extern`).
func HasStorage(f *Node, w string) bool { return hasPrefix(f, w) }

// DeclInit is a def's initialiser, or nil.
func DeclInit(f *Node) *Node { return mfDefInit(f) }

func mfDefInit(f *Node) *Node {
	i := defNameAt(f)
	if !f.Is("def") || i == 0 || i+2 >= len(f.Kids) {
		return nil
	}
	if k := f.Kids[len(f.Kids)-1]; !isAttrForm(k) {
		return k
	}
	return nil
}
