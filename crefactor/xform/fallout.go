package xform

// fallout.go is the fall-out closure (doc/PIPELINE-REFORM.md §5): what a
// drop leaves behind, taken generically, and nothing else.
//
// A drop deletes the code that wrote something -- a command-line option's
// arm, an Ex command's handler -- and leaves its readers.  FallOut starts
// from those readers and follows what they make constant:
//
//   - seeds: a file-scope object nothing writes (its address never taken,
//     its one write its constant initializer, or none: zero) and a struct
//     member nothing writes, of a struct whose every instance is a static
//     object with no initializer, reached only through pointers to it and
//     memset to zero -- each read of either is its value;
//   - a local initialised with such a value and never written again;
//   - an expression that is then constant is its value; `a && K` and
//     `a || K` lose the K where only the truth of the result is read;
//   - an if, a while or a ?: of a constant condition is the branch it
//     takes, a braced branch spliced into the block that held the if;
//   - the statements after a jump that was revealed so;
//   - a function whose body is then `return K;` is not called: each call
//     is K; a parameter every call passes the same constant is that
//     constant in the body, and goes from the definition, its prototypes
//     and its calls; a function left with nothing to do is not called.
//
// The rules after the seeds fire only on what holds a value they wrote:
// each written value is an enumerator `fallout_V` declared at the top while
// the step runs, and each branch it took is marked by a comment, so code
// the drop did not touch is left exactly as it is.  At the end every
// enumerator is its number again and the comments go.  What nothing names
// then -- the object, the member, the function -- is the sweep's.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// FallOut is the step.  It takes no arguments.
func FallOut() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "fallout", W: w}
		if _, err := flags(p.Tag, args); err != nil {
			return nil, err
		}
		return fallOut(p, text, nil, nil)
	}
}

// FallOutOf is the step that runs cut and then the closure, seeded with
// what the cut left unwritten: each object and member nothing writes after
// it that something wrote before, save the names in hold -- what the
// product keeps, and a later phase folds or leaves.
func FallOutOf(cut Step, hold ...string) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "fallout", W: w}
		before, _, err := unwrittenNames(text)
		if err != nil {
			return nil, p.Die("before the cut: %v", err)
		}
		out, err := cut(text, args, w)
		if err != nil {
			return nil, err
		}
		after, ast, err := unwrittenNames(out)
		if err != nil {
			return nil, p.Die("after the cut: %v", err)
		}
		held := map[string]bool{}
		for _, h := range hold {
			held[h] = true
		}
		// A FIXED POINT OVER THE SEEDS TOO: what the closure folds can take an
		// object's last write -- `if (params.no_swap_file) p_uc = 0;` goes with
		// the flag -- so what is unwritten is asked again after every closure,
		// against the text before the cut, until nothing new is.
		seeds := map[string]bool{}
		for pass := 0; ; pass++ {
			var names []string
			for k := range after {
				if !before[k] && !held[k] && !seeds[k] {
					seeds[k] = true
					names = append(names, k)
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				return foUndeclare(out), nil
			}
			if pass == 0 {
				p.Sayf("%d left unwritten by the cut: %s", len(names), strings.Join(names, " "))
			} else {
				p.Sayf("%d more left unwritten by what fell out: %s", len(names), strings.Join(names, " "))
			}
			if out, ast, err = fallOutMarked(p, out, seeds, ast); err != nil {
				return nil, err
			}
			after = unwrittenOf(out, ast)
		}
	}
}

// unwrittenNames is the name of every object, and the struct.member of
// every member, nothing writes in text.
func unwrittenNames(text []byte) (map[string]bool, *cc.AST, error) {
	ast, err := translate(append([]byte(nil), text...))
	if err != nil {
		return nil, nil, err
	}
	return unwrittenOf(text, ast), ast, nil
}

// unwrittenOf is unwrittenNames on a text already parsed.
func unwrittenOf(text []byte, ast *cc.AST) map[string]bool {
	f := newFO(text, ast)
	f.noIndex = true
	f.index()
	out := map[string]bool{}
	for d := range f.unwrittenObjects() {
		out[d.Name()] = true
	}
	for k := range f.unwrittenMembers() {
		out[k] = true
	}
	return out
}

const (
	foMark    = "/*fallout*/"
	foEnumTag = "enum { fallout_values"
)

var foValue = regexp.MustCompile(`\bfallout_(m?)(\d+)\b`)

// foName is the enumerator that stands for v.
func foName(v int64) string {
	if v < 0 {
		return fmt.Sprintf("fallout_m%d", -v)
	}
	return fmt.Sprintf("fallout_%d", v)
}

// foLit is the value v written where a t is read: a null pointer constant
// for a pointer.
func foLit(t cc.Type, v int64) string {
	if t != nil && t.Kind() == cc.Ptr {
		return "((void *)" + foName(v) + ")"
	}
	return foName(v)
}

// foDeclare puts the enumerators the text names in one declaration at its top.
func foDeclare(text []byte) []byte {
	if bytes.HasPrefix(text, []byte(foEnumTag)) {
		text = text[bytes.IndexByte(text, '\n')+1:]
	}
	seen := map[string]bool{}
	var names []string
	for _, m := range foValue.FindAllSubmatch(text, -1) {
		if s := string(m[0]); !seen[s] {
			seen[s] = true
			names = append(names, s)
		}
	}
	if len(names) == 0 {
		return text
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(foEnumTag)
	for _, n := range names {
		m := foValue.FindStringSubmatch(n)
		v := m[2]
		if m[1] == "m" {
			v = "-" + v
		}
		fmt.Fprintf(&b, ", %s = %s", n, v)
	}
	b.WriteString(" };\n")
	return append([]byte(b.String()), text...)
}

// foUndeclare writes every enumerator as its number and removes the
// declaration and the marks.
func foUndeclare(text []byte) []byte {
	if bytes.HasPrefix(text, []byte(foEnumTag)) {
		text = text[bytes.IndexByte(text, '\n')+1:]
	}
	text = foValue.ReplaceAllFunc(text, func(b []byte) []byte {
		m := foValue.FindSubmatch(b)
		if string(m[1]) == "m" {
			return append([]byte("-"), m[2]...)
		}
		return m[2]
	})
	return bytes.ReplaceAll(text, []byte(foMark), nil)
}

// fallOut runs the closure to its fixed point; seeds, when not nil, are
// the only objects and members it starts from.
func fallOut(p edit.Ph, text []byte, seeds map[string]bool, ast0 *cc.AST) ([]byte, error) {
	text, _, err := fallOutMarked(p, text, seeds, ast0)
	if err != nil {
		return nil, err
	}
	return foUndeclare(text), nil
}

// fallOutMarked is fallOut leaving its values as the enumerators it declared,
// with the parse of its last round: what is unwritten afterwards can be asked
// of that parse, without another.
func fallOutMarked(p edit.Ph, text []byte, seeds map[string]bool, ast0 *cc.AST) ([]byte, *cc.AST, error) {
	counts := map[string]int{}
	var ast *cc.AST
	var vals map[string]int64
	rounds := 0
	for round := 0; ; round++ {
		rounds = round
		declared := foDeclare(text)
		ast = ast0 // round 0's text is the one already parsed, when given
		if ast0 != nil && !bytes.Equal(declared, text) {
			ast = nil // the declaration moved: parse again
		}
		ast0 = nil
		text = declared
		if ast == nil {
			var err error
			if ast, err = translate(append([]byte(nil), text...)); err != nil {
				return nil, nil, p.Die("round %d: the text does not type-check: %v", round, err)
			}
		}
		f := newFO(text, ast)
		f.only = seeds
		f.vals = vals
		f.collect()
		vals = f.vals
		if len(f.rws) == 0 {
			break
		}
		for _, r := range f.rws {
			counts[r.rule]++
		}
		var rws []nrw
		for _, r := range f.rws {
			rws = append(rws, r.nrw)
		}
		out, err := applyNested(text, rws)
		if err != nil {
			return nil, nil, p.Die("round %d: %v", round, err)
		}
		text = out
		if round > 200 {
			return nil, nil, p.Die("no fixed point after %d rounds", round)
		}
	}
	var rules []string
	for r := range counts {
		rules = append(rules, r)
	}
	sort.Strings(rules)
	var parts []string
	for _, r := range rules {
		parts = append(parts, fmt.Sprintf("%d %s", counts[r], r))
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing falls out")
	}
	p.Sayf("%s; %d rounds", strings.Join(parts, ", "), rounds)
	return text, ast, nil
}

// --- one round -----------------------------------------------------------------

type foRw struct {
	nrw
	rule string
}

type fo struct {
	src    []byte
	ast    *cc.AST
	parent map[cc.Node]cc.Node
	rws    []foRw
	fns    map[string]*cc.FunctionDefinition
	// every mention of a function's name, and those that are the callee of a call
	mentions map[string]int
	calls    map[string][]*cc.PostfixExpression
	caller   map[*cc.PostfixExpression]string // the function a call is in
	markedFn map[string]bool                  // a body holding a value or a branch this step wrote
	only     map[string]bool                  // the seeds, when not every unwritten object
	vals     map[string]int64                 // the unwritten objects' values, once a pass
	keys     map[cc.Type]string
	all      []cc.Node       // every node, in order
	end      []int           // where the subtree of all[i] ends
	idx      map[cc.Node]int // a node's place in all
	done     map[*cc.FunctionDefinition]bool
	refs     map[string]map[string]bool // the functions a function names; "" the file scope
	live     map[string]bool            // what main reaches
	noIndex  bool                       // analysis only: no rule runs
}

// walk is sweep.Walk over the nodes indexed once: fn on every node under
// n, n included, in order, and nothing below a node it returns false for.
func (f *fo) walk(n cc.Node, fn func(cc.Node) bool) {
	i, ok := f.idx[n]
	if !ok {
		sweep.Walk(n, fn)
		return
	}
	for j := i; j < f.end[i]; {
		if fn(f.all[j]) {
			j++
		} else {
			j = f.end[j]
		}
	}
}

func newFO(src []byte, ast *cc.AST) *fo {
	return &fo{src: src, ast: ast, parent: map[cc.Node]cc.Node{},
		fns: map[string]*cc.FunctionDefinition{}, mentions: map[string]int{},
		calls: map[string][]*cc.PostfixExpression{}, caller: map[*cc.PostfixExpression]string{},
		markedFn: map[string]bool{}, keys: map[cc.Type]string{}, idx: map[cc.Node]int{},
		done: map[*cc.FunctionDefinition]bool{}, refs: map[string]map[string]bool{},
		live: map[string]bool{}}
}

// balanced says a span opens every bracket it closes and closes every one
// it opens: a node a macro's expansion made carries the positions of the
// invocation's tokens, and its span can stop inside the invocation.
func balanced(b []byte) bool {
	depth := 0
	for i := 0; i < len(b); i++ {
		switch c := b[i]; c {
		case '"', '\'':
			for i++; i < len(b) && b[i] != c; i++ {
				if b[i] == '\\' {
					i++
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// add records a rewrite unless it overlaps one already taken this round, or
// its span is not a whole piece of the text.
func (f *fo) add(rule string, a, z int, text func(render func(a, z int) string) string) bool {
	if z < a || !balanced(f.src[a:z]) {
		return false
	}
	for _, r := range f.rws {
		if a < r.z && r.a < z || a == r.a && z == r.z {
			return false
		}
	}
	f.rws = append(f.rws, foRw{nrw{a: a, z: z, text: text}, rule})
	return true
}

func (f *fo) span(n cc.Node) (int, int) { return spanOf(n, f.src) }

func (f *fo) textOf(n cc.Node) string {
	a, z := f.span(n)
	return string(f.src[a:z])
}

// marked says the node's text holds a value or a branch this step wrote.
func (f *fo) marked(n cc.Node) bool {
	a, z := f.span(n)
	if z <= a {
		return false
	}
	s := f.src[a:z]
	return foValue.Match(s) || bytes.Contains(s, []byte(foMark))
}

// markedBefore says a mark stands right before the node: a branch taken.
func (f *fo) markedBefore(n cc.Node) bool {
	a, _ := f.span(n)
	s := bytes.TrimRight(f.src[:a], " \t\n")
	return bytes.HasSuffix(s, []byte(foMark))
}

func intValue(e cc.ExpressionNode) (int64, bool) {
	if e == nil {
		return 0, false
	}
	switch x := e.Value().(type) {
	case cc.Int64Value:
		return int64(x), true
	case cc.UInt64Value:
		return int64(x), true
	}
	return 0, false
}

// isMarker says the expression is one enumerator this step wrote.
func (f *fo) isMarker(e cc.Node) bool {
	return foValue.Match([]byte(f.textOf(e))) && foValue.FindString(f.textOf(e)) == strings.TrimSpace(f.textOf(e))
}

// constOf is a marked expression's value.
func (f *fo) constOf(e cc.ExpressionNode) (int64, bool) {
	if e == nil || !f.marked(e) {
		return 0, false
	}
	return intValue(e)
}

func (f *fo) collect() {
	f.index()
	// the rules, outermost first so that a rewrite of a statement wins over
	// one of an expression inside it
	f.branches()
	f.deadAfterJump()
	f.emptyFunctions()
	f.constReturns()
	f.constParams()
	f.logic()
	f.constExprs()
	f.seeds()
	f.locals()
}

// index finds every node's parent, the functions, and who names them.
func (f *fo) index() {
	for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed != nil && ed.Case == cc.ExternalDeclarationFuncDef && ed.Position().Filename == file {
			fd := ed.FunctionDefinition
			nm := fd.Declarator.Name()
			f.fns[nm] = fd
			// a body's extent is its braces: no walk of its tokens
			a := fd.CompoundStatement.Token.Position().Offset
			z := fd.CompoundStatement.Token2.Position().Offset + 1
			if a >= 0 && z <= len(f.src) && a < z {
				body := f.src[a:z]
				f.markedFn[nm] = foValue.Match(body) || bytes.Contains(body, []byte(foMark))
			}
		}
	}
	// one walk of the file: who names and calls a function, and from
	// where; and which functions read a seed
	fields := map[string]bool{}
	for k := range f.only {
		if i := strings.LastIndexByte(k, '.'); i >= 0 {
			fields[k[i+1:]] = true
		}
	}
	need := map[string]bool{}
	for nm, m := range f.markedFn {
		if m || f.only == nil {
			need[nm] = true
		}
	}
	for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Position().Filename != file {
			continue
		}
		in := ""
		var root cc.Node = ed
		if ed.Case == cc.ExternalDeclarationFuncDef {
			in = ed.FunctionDefinition.Declarator.Name()
			root = ed.FunctionDefinition.CompoundStatement
		}
		sweep.Walk(root, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if x.Case != cc.PrimaryExpressionIdent {
					return true
				}
				nm := x.Token.SrcStr()
				if f.fns[nm] != nil {
					f.mentions[nm]++
					if f.refs[in] == nil {
						f.refs[in] = map[string]bool{}
					}
					f.refs[in][nm] = true
				}
				if f.only[nm] && in != "" {
					need[in] = true
				}
			case *cc.PostfixExpression:
				switch x.Case {
				case cc.PostfixExpressionCall:
					if nm := callee(x); nm != "" && f.fns[nm] != nil && in != "" {
						f.calls[nm] = append(f.calls[nm], x)
						f.caller[x] = in
					}
				case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
					if fields[x.Token2.SrcStr()] && in != "" {
						need[in] = true
					}
				}
			}
			return true
		})
	}
	// what main reaches, and what a file-scope initializer names: the code
	// whose writes count
	if f.fns["main"] == nil {
		for nm := range f.fns {
			f.live[nm] = true
		}
	} else {
		work := []string{"main"}
		for nm := range f.refs[""] {
			work = append(work, nm)
		}
		for len(work) > 0 {
			nm := work[len(work)-1]
			work = work[:len(work)-1]
			if f.live[nm] {
				continue
			}
			f.live[nm] = true
			for r := range f.refs[nm] {
				work = append(work, r)
			}
		}
	}
	if f.noIndex {
		return
	}
	for nm := range need {
		f.ensure(f.fns[nm])
	}
}

// liveWalk walks the file-scope declarations and the functions main
// reaches: where a write is one that can happen.
func (f *fo) liveWalk(fn func(cc.Node) bool) {
	for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Position().Filename != file {
			continue
		}
		if ed.Case == cc.ExternalDeclarationFuncDef {
			if f.live[ed.FunctionDefinition.Declarator.Name()] {
				f.walk(ed.FunctionDefinition, fn)
			}
			continue
		}
		f.walk(ed, fn)
	}
}

// lvalueIdent is the object an lvalue names, through parentheses.
func lvalueIdent(e cc.Node) *cc.Declarator {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			switch x.Case {
			case cc.PrimaryExpressionIdent:
				d, _ := x.ResolvedTo().(*cc.Declarator)
				return d
			case cc.PrimaryExpressionExpr:
				e = x.ExpressionList
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				e = x.AssignmentExpression
				continue
			}
		}
		return nil
	}
}

// ensure indexes a function: every node in order, its parent, and where
// its subtree ends -- done for the functions a rule can fire in, not for
// the whole file.
func (f *fo) ensure(fd *cc.FunctionDefinition) {
	if fd == nil || f.done[fd] {
		return
	}
	f.done[fd] = true
	start := len(f.all)
	sweep.Walk(fd, func(n cc.Node) bool {
		if _, seen := f.idx[n]; !seen {
			f.idx[n] = len(f.all)
			f.all = append(f.all, n)
		}
		sweep.Walk(n, func(c cc.Node) bool {
			if c == n {
				return true
			}
			f.parent[c] = n
			return false
		})
		return true
	})
	f.end = append(f.end, make([]int, len(f.all)-start)...)
	for i := len(f.all) - 1; i >= start; i-- {
		if f.end[i] < i+1 {
			f.end[i] = i + 1
		}
		if par, ok := f.parent[f.all[i]]; ok {
			if j, ok := f.idx[par]; ok && j >= start && f.end[j] < f.end[i] {
				f.end[j] = f.end[i]
			}
		}
	}
}

// callee is the name a call calls, when it calls one by name.
func callee(call *cc.PostfixExpression) string {
	e := call.PostfixExpression
	for {
		switch x := e.(type) {
		case *cc.PostfixExpression:
			if x.Case != cc.PostfixExpressionPrimary {
				return ""
			}
			e = x.PrimaryExpression
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				return x.Token.SrcStr()
			}
			return ""
		default:
			return ""
		}
	}
}

func args(call *cc.PostfixExpression) []cc.ExpressionNode {
	var out []cc.ExpressionNode
	for l := call.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		out = append(out, l.AssignmentExpression)
	}
	return out
}

// inLive calls fn on every node of every function main reaches: code main
// does not reach is the sweep's, and may still write what it reads.
func (f *fo) inLive(fn func(cc.Node) bool) {
	for nm, fd := range f.fns {
		if f.live[nm] {
			f.walk(fd.CompoundStatement, fn)
		}
	}
}

// lvalue says n is written where it stands: assigned, stepped, or its
// address taken.
func (f *fo) lvalue(n cc.Node) bool {
	for {
		switch x := f.parent[n].(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				n = x
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				n = x
				continue
			}
		case *cc.AssignmentExpression:
			return x.UnaryExpression == n
		case *cc.UnaryExpression:
			return x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec || x.Case == cc.UnaryExpressionAddrof
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
				return true
			}
			if x.Case == cc.PostfixExpressionIndex && x.PostfixExpression == n {
				n = x // an element of it: written if the element is
				continue
			}
		}
		return false
	}
}

// inMarked calls fn on every node of every body holding a mark: the only
// ones a rule after the seeds can fire in.
func (f *fo) inMarked(fn func(cc.Node) bool) {
	for nm, fd := range f.fns {
		if f.markedFn[nm] {
			f.walk(fd.CompoundStatement, fn)
		}
	}
}

// underSizeof says n is the operand of a sizeof or a typeof: not a read.
func (f *fo) underSizeof(n cc.Node) bool {
	for p := f.parent[n]; p != nil; p = f.parent[p] {
		switch x := p.(type) {
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionSizeofExpr || x.Case == cc.UnaryExpressionSizeofType ||
				x.Case == cc.UnaryExpressionAlignofExpr || x.Case == cc.UnaryExpressionAlignofType {
				return true
			}
		case *cc.TypeSpecifier:
			return true
		case *cc.Statement, *cc.FunctionDefinition:
			return false
		}
	}
	return false
}

// --- seeds ----------------------------------------------------------------------

// seeds writes each read of an object or a member nothing writes as its value.
func (f *fo) seeds() {
	// The values are the pass's first round's: the rounds only take code out,
	// so what was unwritten stays so, at the same value.
	var objs map[*cc.Declarator]int64
	if f.vals == nil {
		objs = f.unwrittenObjects()
		f.vals = map[string]int64{}
		for d, v := range objs {
			f.vals[d.Name()] = v
		}
	} else {
		objs = map[*cc.Declarator]int64{}
		for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
			ed := tu.ExternalDeclaration
			if ed == nil || ed.Case != cc.ExternalDeclarationDecl || ed.Position().Filename != file ||
				ed.Declaration == nil || ed.Declaration.InitDeclaratorList == nil {
				continue
			}
			for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				if d := l.InitDeclarator.Declarator; d != nil {
					if v, ok := f.vals[d.Name()]; ok {
						objs[d] = v
					}
				}
			}
		}
	}
	var members map[string]bool
	if f.only != nil {
		// a seeded member stays unwritten: the rounds only take code out
		members = f.only
	} else {
		members = f.unwrittenMembers()
	}
	f.inLive(func(n cc.Node) bool {
		if f.lvalue(n) {
			return true
		}
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case != cc.PrimaryExpressionIdent {
				return true
			}
			d, ok := x.ResolvedTo().(*cc.Declarator)
			if !ok {
				return true
			}
			if v, ok := objs[d]; ok && !f.underSizeof(x) && (f.only == nil || f.only[d.Name()]) {
				a, z := f.span(x)
				if d.Type().Kind() == cc.Ptr && !f.tested(x) {
					return true
				}
				lit := foLit(d.Type(), v)
				f.add("reads of objects nothing writes", a, z, func(func(a, z int) string) string { return lit })
			}
		case *cc.PostfixExpression:
			if x.Case != cc.PostfixExpressionSelect && x.Case != cc.PostfixExpressionPSelect {
				return true
			}
			if k := f.memberKey(x); !members[k] || f.underSizeof(x) || f.only != nil && !f.only[k] {
				return true
			}
			fl := x.Field()
			if fl == nil {
				return true
			}
			var target cc.Node = x
			if fl.Type().Kind() == cc.Array {
				// an element read: the index expression around it
				ix, ok := f.parent[x].(*cc.PostfixExpression)
				if !ok || ix.Case != cc.PostfixExpressionIndex || ix.PostfixExpression != x || !ix.ExpressionList.Pure() {
					return true
				}
				target = ix
			} else if !cc.IsScalarType(fl.Type()) {
				return true
			}
			if fl.Type().Kind() == cc.Ptr && !f.tested(target) {
				return true
			}
			a, z := f.span(target)
			lit := foLit(target.(cc.ExpressionNode).Type(), 0)
			f.add("reads of members nothing writes", a, z, func(func(a, z int) string) string { return lit })
			return false
		}
		return true
	})
}

// unwrittenObjects is every file-scope scalar object nothing writes, with
// its value.
func (f *fo) unwrittenObjects() map[*cc.Declarator]int64 {
	type agg struct {
		decls   []*cc.Declarator
		writes  int
		inits   int
		value   int64
		bad     bool
		defined bool
	}
	// the writes main can reach, by the name of what they write
	writes := map[string]int{}
	f.liveWalk(func(n cc.Node) bool {
		var d *cc.Declarator
		switch x := n.(type) {
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				d = lvalueIdent(x.UnaryExpression)
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
				d = lvalueIdent(x.UnaryExpression)
			}
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
				d = lvalueIdent(x.PostfixExpression)
			}
		}
		if d != nil {
			writes[d.Name()]++
		}
		return true
	})
	byName := map[string]*agg{}
	for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != cc.ExternalDeclarationDecl || ed.Position().Filename != file ||
			ed.Declaration == nil || ed.Declaration.InitDeclaratorList == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			id := l.InitDeclarator
			if id == nil || id.Declarator == nil {
				continue
			}
			d := id.Declarator
			t := d.Type()
			if t == nil || t.Kind() == cc.Function {
				continue
			}
			a := byName[d.Name()]
			if a == nil {
				a = &agg{}
				byName[d.Name()] = a
			}
			a.decls = append(a.decls, d)
			a.writes = writes[d.Name()]
			if d.AddressTaken() || !cc.IsScalarType(t) || d.IsVolatile() {
				a.bad = true
			}
			if !d.IsExtern() || id.Initializer != nil {
				a.defined = true
			}
			if id.Initializer != nil {
				a.inits++
				if id.Initializer.Case != cc.InitializerExpr {
					a.bad = true
					continue
				}
				v, ok := intValue(id.Initializer.AssignmentExpression)
				if !ok {
					a.bad = true
				}
				a.value = v
			}
		}
	}
	out := map[*cc.Declarator]int64{}
	for _, a := range byName {
		if a.bad || !a.defined || a.inits > 1 || a.writes != 0 {
			continue
		}
		for _, d := range a.decls {
			out[d] = a.value
		}
	}
	// a local of the same name resolves to its own declarator, so the
	// map is by declarator and needs no exclusion
	return out
}

// memberKey names a member access by its member and the struct it is of.
func (f *fo) memberKey(x *cc.PostfixExpression) string {
	st := f.ownerOf(x)
	if st == nil {
		return ""
	}
	return f.tkey(st) + "." + x.Token2.SrcStr()
}

// ownerOf is the struct type a member access reads from.
func (f *fo) ownerOf(x *cc.PostfixExpression) cc.Type {
	if x.PostfixExpression == nil || x.PostfixExpression.Type() == nil {
		return nil
	}
	t := x.PostfixExpression.Type()
	if x.Case == cc.PostfixExpressionPSelect {
		pt, ok := t.(*cc.PointerType)
		if !ok {
			return nil
		}
		t = pt.Elem()
	}
	if t.Kind() != cc.Struct {
		return nil
	}
	return t
}

// tkey is a type's name, computed once a type: a tagged struct's String
// writes out every member.
func (f *fo) tkey(t cc.Type) string {
	if k, ok := f.keys[t]; ok {
		return k
	}
	k := t.String()
	f.keys[t] = k
	return k
}

func (f *fo) isPtrTo(t cc.Type, s string) bool {
	if t == nil || t.Kind() != cc.Ptr {
		return false
	}
	pt, ok := t.(*cc.PointerType)
	return ok && pt.Elem() != nil && pt.Elem().Kind() == cc.Struct && f.tkey(pt.Elem()) == s
}

// unwrittenMembers is every member nothing writes, of a struct whose
// instances are all zero to begin with and are reached only as themselves.
func (f *fo) unwrittenMembers() map[string]bool {
	written := map[string]bool{} // by member name: any struct
	owners := map[string]bool{}  // the structs a candidate member is read from
	reads := map[string]bool{}
	lhsMember := func(e cc.Node) string {
		for {
			switch x := e.(type) {
			case *cc.PrimaryExpression:
				if x.Case != cc.PrimaryExpressionExpr {
					return ""
				}
				e = x.ExpressionList
			case *cc.ExpressionList:
				if x.ExpressionList != nil {
					return ""
				}
				e = x.AssignmentExpression
			case *cc.PostfixExpression:
				switch x.Case {
				case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
					return x.Token2.SrcStr()
				case cc.PostfixExpressionIndex:
					e = x.PostfixExpression
				default:
					return ""
				}
			default:
				return ""
			}
		}
	}
	f.liveWalk(func(n cc.Node) bool {
		if n.Position().Filename != file {
			return true
		}
		switch x := n.(type) {
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				if m := lhsMember(x.UnaryExpression); m != "" {
					written[m] = true
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionAddrof, cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				if m := lhsMember(x.CastExpression); m != "" {
					written[m] = true
				}
				if m := lhsMember(x.UnaryExpression); m != "" {
					written[m] = true
				}
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
				if m := lhsMember(x.PostfixExpression); m != "" {
					written[m] = true
				}
			case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
				if st := f.ownerOf(x); st != nil {
					reads[f.tkey(st)+"."+x.Token2.SrcStr()] = true
				}
			}
		case *cc.Designator:
			if x.Case == cc.DesignatorField || x.Case == cc.DesignatorField2 {
				written[x.Token2.SrcStr()] = true
				written[x.Token.SrcStr()] = true
			}
		}
		return true
	})
	cands := map[string]bool{}
	for k := range reads {
		i := strings.LastIndexByte(k, '.')
		if !written[k[i+1:]] {
			cands[k] = true
			owners[k[:i]] = true
		}
	}
	if len(cands) == 0 {
		return cands
	}
	bad := f.unsafeOwners(owners)
	for k := range cands {
		if bad[k[:strings.LastIndexByte(k, '.')]] {
			delete(cands, k)
		}
	}
	return cands
}

// unsafeOwners is each struct of owners an instance of which may start
// other than zero, or be written other than member by member.
func (f *fo) unsafeOwners(owners map[string]bool) map[string]bool {
	bad := map[string]bool{}
	of := func(t cc.Type) string { // the owner t is, holds, or is an array of
		for t != nil && t.Kind() == cc.Array {
			at, ok := t.(*cc.ArrayType)
			if !ok {
				return ""
			}
			t = at.Elem()
		}
		if t != nil && (t.Kind() == cc.Struct || t.Kind() == cc.Union) {
			if owners[f.tkey(t)] {
				return f.tkey(t)
			}
			if st, ok := t.(*cc.StructType); ok {
				for i := 0; i < st.NumFields(); i++ {
					if fl := st.FieldByIndex(i); fl != nil {
						ft := fl.Type()
						for ft != nil && ft.Kind() == cc.Array {
							at, ok := ft.(*cc.ArrayType)
							if !ok {
								break
							}
							ft = at.Elem()
						}
						if ft != nil && ft.Kind() == cc.Struct && owners[f.tkey(ft)] {
							return f.tkey(ft)
						}
					}
				}
			}
		}
		return ""
	}
	conv := func(to, from cc.Type, src cc.ExpressionNode, memsetZero bool) {
		for s := range owners {
			pt, pf := f.isPtrTo(to, s), f.isPtrTo(from, s)
			switch {
			case pt == pf:
			case pf && memsetZero:
			case pt && src != nil && func() bool { v, ok := intValue(src); return ok && v == 0 }():
			default:
				bad[s] = true
			}
		}
	}
	f.walk(f.ast.TranslationUnit, func(n cc.Node) bool {
		if n.Position().Filename != file {
			return true
		}
		switch x := n.(type) {
		case *cc.Declarator:
			if x.IsTypename() {
				return true
			}
			if s := of(x.Type()); s != "" {
				if x.StorageDuration() != cc.Static || x.HasInitializer() || x.IsParam() {
					bad[s] = true
				}
			}
		case *cc.StructDeclarator:
			if x.Declarator != nil {
				if s := of(x.Declarator.Type()); s != "" {
					bad[s] = true // held inside another
				}
			}
		case *cc.AssignmentExpression:
			if x.Case == cc.AssignmentExpressionCond {
				return true
			}
			if s := of(x.Type()); s != "" {
				bad[s] = true // a whole struct assigned
			}
			if x.Case == cc.AssignmentExpressionAssign && x.UnaryExpression != nil {
				conv(x.UnaryExpression.Type(), x.AssignmentExpression.Type(), x.AssignmentExpression, false)
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast && x.TypeName != nil {
				conv(x.TypeName.Type(), x.CastExpression.Type(), x.CastExpression, false)
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionComplit:
				if s := of(x.Type()); s != "" {
					bad[s] = true
				}
			case cc.PostfixExpressionCall:
				for o := range owners {
					if f.isPtrTo(x.Type(), o) {
						bad[o] = true // an instance from somewhere else
					}
				}
				ct := x.PostfixExpression.Type()
				if pt, ok := ct.(*cc.PointerType); ok {
					ct = pt.Elem()
				}
				ft, _ := ct.(*cc.FunctionType)
				var ps []*cc.Parameter
				if ft != nil {
					ps = ft.Parameters()
				}
				callee := ""
				if pe, ok := x.PostfixExpression.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent {
					callee = pe.Token.SrcStr()
				}
				as := args(x)
				zero := callee == "memset" && len(as) == 3 && func() bool { v, ok := intValue(as[1]); return ok && v == 0 }()
				for i, a := range as {
					var to cc.Type
					if i < len(ps) {
						to = ps[i].Type()
					}
					conv(to, a.Type(), a, zero && i == 0)
				}
			}
		case *cc.InitDeclarator:
			if x.Initializer != nil && x.Initializer.Case == cc.InitializerExpr && x.Declarator != nil {
				conv(x.Declarator.Type(), x.Initializer.AssignmentExpression.Type(), x.Initializer.AssignmentExpression, false)
			}
		case *cc.FunctionDefinition:
			res := x.Declarator.Type().(*cc.FunctionType).Result()
			f.walk(x.CompoundStatement, func(m cc.Node) bool {
				if j, ok := m.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementReturn && j.ExpressionList != nil {
					conv(res, j.ExpressionList.Type(), j.ExpressionList, false)
				}
				return true
			})
		}
		return true
	})
	return bad
}

// --- locals ---------------------------------------------------------------------

// locals writes each read of a local initialised with a value this step
// wrote, and never written again, as that value.
func (f *fo) locals() {
	f.inMarked(func(n cc.Node) bool {
		x, ok := n.(*cc.PrimaryExpression)
		if !ok || x.Case != cc.PrimaryExpressionIdent {
			return true
		}
		d, ok := x.ResolvedTo().(*cc.Declarator)
		if !ok || d.IsParam() || d.StorageDuration() == cc.Static || d.AddressTaken() || d.WriteCount() != 1 {
			return true
		}
		id, ok := f.parent[d].(*cc.InitDeclarator)
		if !ok || id.Initializer == nil || id.Initializer.Case != cc.InitializerExpr {
			return true
		}
		v, ok := f.constOf(id.Initializer.AssignmentExpression)
		if !ok || !f.isMarker(id.Initializer.AssignmentExpression) || f.underSizeof(x) {
			return true
		}
		a, z := f.span(x)
		f.add("reads of locals so initialised", a, z, func(func(a, z int) string) string { return foName(v) })
		return true
	})
}

// --- expressions ----------------------------------------------------------------

// exprNode says n is an expression the rules may write as a value.
func exprNode(n cc.Node) (cc.ExpressionNode, bool) {
	e, ok := n.(cc.ExpressionNode)
	if !ok {
		return nil, false
	}
	switch n.(type) {
	case *cc.ExpressionList:
		return nil, false
	}
	return e, true
}

// constExprs writes an integer expression holding a value this step wrote,
// and constant with it, as its value: the outermost such.
func (f *fo) constExprs() {
	f.inMarked(func(n cc.Node) bool {
		e, ok := exprNode(n)
		if !ok {
			return true
		}
		if f.isMarker(e) {
			return false
		}
		v, ok := f.constOf(e)
		if !ok || !e.Pure() || e.Type() == nil || !cc.IsIntegerType(e.Type()) || f.underSizeof(e) {
			return true
		}
		a, z := f.span(e)
		f.add("constant expressions", a, z, func(func(a, z int) string) string { return foName(v) })
		return false
	})
}

// boolContext says only the truth of e's value is read.
func (f *fo) boolContext(e cc.Node) bool {
	n := e
	for {
		par := f.parent[n]
		switch x := par.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				n = x
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				n = x
				continue
			}
		case *cc.SelectionStatement:
			return x.ExpressionList == n && x.Case != cc.SelectionStatementSwitch
		case *cc.IterationStatement:
			return x.Case == cc.IterationStatementWhile && x.ExpressionList == n ||
				x.Case == cc.IterationStatementDo && x.ExpressionList == n ||
				x.Case == cc.IterationStatementFor && x.ExpressionList2 == n
		case *cc.ConditionalExpression:
			return x.LogicalOrExpression == n
		case *cc.UnaryExpression:
			return x.Case == cc.UnaryExpressionNot
		case *cc.LogicalAndExpression, *cc.LogicalOrExpression:
			return true
		}
		return false
	}
}

// tested says a pointer's value is only compared or tested for null.
func (f *fo) tested(e cc.Node) bool {
	if f.boolContext(e) {
		return true
	}
	n := e
	for {
		switch x := f.parent[n].(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				n = x
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				n = x
				continue
			}
		case *cc.EqualityExpression:
			return true
		}
		return false
	}
}

// isTruth says e's value is 0 or 1 already.
func isTruth(e cc.Node) bool {
	switch x := e.(type) {
	case *cc.LogicalAndExpression, *cc.LogicalOrExpression, *cc.EqualityExpression, *cc.RelationalExpression:
		return true
	case *cc.UnaryExpression:
		return x.Case == cc.UnaryExpressionNot
	}
	return false
}

// logic takes the constant operand out of && and ||.
func (f *fo) logic() {
	f.inMarked(func(n cc.Node) bool {
		var l, r cc.ExpressionNode
		var or bool
		switch x := n.(type) {
		case *cc.LogicalAndExpression:
			if x.Case != cc.LogicalAndExpressionLAnd {
				return true
			}
			l, r = x.LogicalAndExpression, x.InclusiveOrExpression
		case *cc.LogicalOrExpression:
			if x.Case != cc.LogicalOrExpressionLOr {
				return true
			}
			l, r, or = x.LogicalOrExpression, x.LogicalAndExpression, true
		default:
			return true
		}
		e := n.(cc.ExpressionNode)
		a, z := f.span(n)
		// the identity: K true in &&, K false in ||, keeps the other operand
		keep := func(k cc.ExpressionNode, other cc.ExpressionNode) bool {
			v, ok := f.constOf(k)
			if !ok || (v != 0) == or {
				return false
			}
			if !f.boolContext(e) && !isTruth(other) {
				return false
			}
			oa, oz := f.span(other)
			return f.add("constant operands of && and ||", a, z, func(render func(a, z int) string) string { return render(oa, oz) })
		}
		// the absorbing value: K false in &&, K true in ||, is the result
		absorb := func(k cc.ExpressionNode, other cc.ExpressionNode) bool {
			v, ok := f.constOf(k)
			if !ok || (v != 0) != or || !other.Pure() {
				return false
			}
			res := int64(0)
			if or {
				res = 1
			}
			return f.add("constant operands of && and ||", a, z, func(func(a, z int) string) string { return foName(res) })
		}
		if keep(l, r) || keep(r, l) || absorb(l, r) || absorb(r, l) {
			return false
		}
		return true
	})
}

// --- statements -----------------------------------------------------------------

// asBlockItem is the block item a statement is, if it is one.
func (f *fo) asBlockItem(s cc.Node) *cc.BlockItem {
	n := s
	for {
		switch x := f.parent[n].(type) {
		case *cc.Statement:
			n = x
			continue
		case *cc.BlockItem:
			return x
		}
		return nil
	}
}

// stmtOf is the Statement node that holds a selection or iteration.
func (f *fo) stmtOf(n cc.Node) *cc.Statement {
	s, _ := f.parent[n].(*cc.Statement)
	return s
}

// splice is a taken branch's text: a braced branch standing alone in a
// block gives its items, unless they declare something.
func (f *fo) splice(take *cc.Statement, alone bool) (int, int) {
	if alone && take.Case == cc.StatementCompound {
		its := items(take.CompoundStatement)
		decl := false
		for _, it := range its {
			decl = decl || it.Case != cc.BlockItemStmt
		}
		if !decl {
			if len(its) == 0 {
				return 0, 0
			}
			first, _ := f.span(its[0])
			_, end := f.span(its[len(its)-1])
			return first, end
		}
	}
	return f.span(take)
}

// branches takes the branch an if, a while or a ?: of a constant condition
// takes.
func (f *fo) branches() {
	f.inMarked(func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.SelectionStatement:
			if x.Case != cc.SelectionStatementIf && x.Case != cc.SelectionStatementIfElse {
				return true
			}
			v, ok := f.constOf(x.ExpressionList)
			if !ok || hasLabel(x.Statement) || x.Case == cc.SelectionStatementIfElse && hasLabel(x.Statement2) {
				return true
			}
			st := f.stmtOf(x)
			if st == nil {
				return true
			}
			a, z := f.span(st)
			alone := f.asBlockItem(st) != nil
			var take *cc.Statement
			switch {
			case v != 0:
				take = x.Statement
			case x.Case == cc.SelectionStatementIfElse:
				take = x.Statement2
			}
			if take != nil {
				ta, tz := f.splice(take, alone)
				f.add("ifs of a constant condition", a, z, func(render func(a, z int) string) string {
					if tz <= ta {
						return foMark
					}
					return foMark + render(ta, tz)
				})
				return false
			}
			// no branch taken: an else arm goes from the if that holds it
			if outer, ok := f.parent[st].(*cc.SelectionStatement); ok && outer.Case == cc.SelectionStatementIfElse && outer.Statement2 == st {
				_, sz := f.span(outer.Statement)
				f.add("ifs of a constant condition", sz, z, func(func(a, z int) string) string { return foMark })
				return false
			}
			if alone {
				f.add("ifs of a constant condition", a, z, func(func(a, z int) string) string { return foMark })
			} else {
				f.add("ifs of a constant condition", a, z, func(func(a, z int) string) string { return foMark + ";" })
			}
			return false
		case *cc.IterationStatement:
			if x.Case != cc.IterationStatementWhile {
				return true
			}
			v, ok := f.constOf(x.ExpressionList)
			if !ok || v != 0 || hasLabel(x.Statement) {
				return true
			}
			st := f.stmtOf(x)
			if st == nil {
				return true
			}
			a, z := f.span(st)
			if f.asBlockItem(st) != nil {
				f.add("whiles of a false condition", a, z, func(func(a, z int) string) string { return foMark })
			} else {
				f.add("whiles of a false condition", a, z, func(func(a, z int) string) string { return foMark + ";" })
			}
			return false
		case *cc.ConditionalExpression:
			if x.Case != cc.ConditionalExpressionCond {
				return true
			}
			v, ok := f.constOf(x.LogicalOrExpression)
			if !ok || x.ExpressionList == nil {
				return true
			}
			var take cc.Node = x.ExpressionList
			if v == 0 {
				take = x.ConditionalExpression
			}
			a, z := f.span(x)
			ta, tz := f.span(take)
			f.add("?: of a constant condition", a, z, func(render func(a, z int) string) string { return "(" + render(ta, tz) + ")" })
			return false
		}
		return true
	})
}

// deadAfterJump deletes what follows a jump a taken branch revealed, up to
// the next label.
func (f *fo) deadAfterJump() {
	f.inMarked(func(n cc.Node) bool {
		cs, ok := n.(*cc.CompoundStatement)
		if !ok {
			return true
		}
		items := items(cs)
		for i := 0; i < len(items); i++ {
			if !Terminates(items[i]) || !(f.markedBefore(items[i]) || f.marked(items[i])) {
				continue
			}
			j := i + 1
			decl := false
			for j < len(items) && !labeled(items[j]) {
				decl = decl || items[j].Case == cc.BlockItemDecl
				j++
			}
			if j == i+1 || decl {
				continue
			}
			a, _ := f.span(items[i+1])
			_, z := f.span(items[j-1])
			f.add("statements after a revealed jump", a, z, func(func(a, z int) string) string { return foMark })
			i = j - 1
		}
		return true
	})
}

// items is a block's items, the parser's synthetic __func__ left out.
func items(cs *cc.CompoundStatement) []*cc.BlockItem {
	var out []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		if l.BlockItem != nil && !funcName(l.BlockItem) {
			out = append(out, l.BlockItem)
		}
	}
	return out
}

// onlyCalled says every mention of the function is the callee of a call.
func (f *fo) onlyCalled(nm string) bool {
	fd := f.fns[nm]
	return fd != nil && fd.Declarator.IsStatic() && len(f.calls[nm]) == f.mentions[nm] && len(f.calls[nm]) > 0
}

// emptyFunctions removes the calls of a function this step left with
// nothing to do.
func (f *fo) emptyFunctions() {
	for nm, fd := range f.fns {
		if !f.markedFn[nm] || !f.onlyCalled(nm) {
			continue
		}
		if ft, ok := fd.Declarator.Type().(*cc.FunctionType); !ok || ft.Result().Kind() != cc.Void {
			continue
		}
		empty := true
		for _, it := range items(fd.CompoundStatement) {
			switch it.Case {
			case cc.BlockItemDecl:
				f.walk(it.Declaration, func(n cc.Node) bool {
					if id, ok := n.(*cc.InitDeclarator); ok && id.Initializer != nil &&
						(id.Initializer.Case != cc.InitializerExpr || !id.Initializer.AssignmentExpression.Pure()) {
						empty = false
					}
					return true
				})
			case cc.BlockItemStmt:
				s := it.Statement
				switch {
				case s.Case == cc.StatementJump && s.JumpStatement.Case == cc.JumpStatementReturn && s.JumpStatement.ExpressionList == nil:
				case s.Case == cc.StatementExpr && s.ExpressionStatement.ExpressionList == nil:
				default:
					empty = false
				}
			default:
				empty = false
			}
		}
		if !empty {
			continue
		}
		var sites [][2]int
		for _, call := range f.calls[nm] {
			ok := true
			for _, a := range args(call) {
				ok = ok && a.Pure()
			}
			f.ensure(f.fns[f.caller[call]])
			es, isStmt := f.exprStmtOf(call)
			if !ok || !isStmt {
				sites = nil
				break
			}
			a, z := f.span(es)
			sites = append(sites, [2]int{a, z})
		}
		for _, s := range sites {
			f.add("calls of functions left empty", s[0], s[1], func(func(a, z int) string) string { return foMark })
		}
	}
}

// exprStmtOf is the expression statement a call is the whole of.
func (f *fo) exprStmtOf(call cc.Node) (*cc.ExpressionStatement, bool) {
	n := call
	for {
		switch x := f.parent[n].(type) {
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return nil, false
			}
			n = x
			continue
		case *cc.ExpressionStatement:
			return x, true
		}
		return nil, false
	}
}

// constReturns writes each call of a function whose body is `return K;`,
// K a value this step wrote, as K.
func (f *fo) constReturns() {
	for nm, fd := range f.fns {
		if !f.markedFn[nm] || !f.onlyCalled(nm) {
			continue
		}
		its := items(fd.CompoundStatement)
		if len(its) != 1 || its[0].Case != cc.BlockItemStmt {
			continue
		}
		s := its[0].Statement
		if s.Case != cc.StatementJump || s.JumpStatement.Case != cc.JumpStatementReturn || s.JumpStatement.ExpressionList == nil {
			continue
		}
		v, ok := f.constOf(s.JumpStatement.ExpressionList)
		if !ok {
			continue
		}
		for _, call := range f.calls[nm] {
			pure := true
			for _, a := range args(call) {
				pure = pure && a.Pure()
			}
			if !pure {
				continue
			}
			a, z := f.span(call)
			f.add("calls of functions returning a constant", a, z, func(func(a, z int) string) string { return foName(v) })
		}
	}
}

// constParams takes out a parameter every call passes the same value this
// step wrote: the value in the body, the parameter from the definition and
// its prototypes, the argument from the calls.
func (f *fo) constParams() {
	for nm, fd := range f.fns {
		if !f.onlyCalled(nm) {
			continue
		}
		callersMarked := true
		for _, call := range f.calls[nm] {
			callersMarked = callersMarked && f.markedFn[f.caller[call]]
		}
		if !callersMarked {
			continue
		}
		pds := paramDecls(fd.Declarator)
		for i, pd := range pds {
			if pd.Declarator == nil {
				continue
			}
			var val int64
			same := true
			for k, call := range f.calls[nm] {
				as := args(call)
				if i >= len(as) {
					same = false
					break
				}
				v, ok := f.constOf(as[i])
				if !ok || !f.isMarker(as[i]) || k > 0 && v != val {
					same = false
					break
				}
				val = v
			}
			d := pd.Declarator
			if !same || d.AddressTaken() || d.WriteCount() != 0 {
				continue
			}
			f.dropParam(nm, fd, i, len(pds), val)
			return // one parameter a round: the lists are rewritten by position
		}
	}
}

// dropParam writes the rewrites constParams decided on.
func (f *fo) dropParam(nm string, fd *cc.FunctionDefinition, i, n int, val int64) {
	pd := paramDecls(fd.Declarator)[i]
	d := pd.Declarator
	// the reads in the body
	f.walk(fd.CompoundStatement, func(m cc.Node) bool {
		if pe, ok := m.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(d) {
			a, z := f.span(pe)
			f.add("parameters every call passes one constant", a, z, func(func(a, z int) string) string { return foName(val) })
		}
		return true
	})
	// the definition and every prototype
	lists := [][]cc.Node{}
	addDecl := func(dd *cc.Declarator) {
		var items []cc.Node
		for _, p := range paramDecls(dd) {
			items = append(items, p)
		}
		if len(items) == n {
			lists = append(lists, items)
		}
	}
	addDecl(fd.Declarator)
	for tu := f.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != cc.ExternalDeclarationDecl || ed.Position().Filename != file || ed.Declaration.InitDeclaratorList == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			if l.InitDeclarator.Declarator.Name() == nm {
				addDecl(l.InitDeclarator.Declarator)
			}
		}
	}
	for _, items := range lists {
		f.dropItem(items, i, "void")
	}
	for _, call := range f.calls[nm] {
		var items []cc.Node
		for _, a := range args(call) {
			items = append(items, a)
		}
		f.dropItem(items, i, "")
	}
}

// dropItem removes the i-th of a comma-separated list, or writes the whole
// list as alone when it was the only one.
func (f *fo) dropItem(items []cc.Node, i int, alone string) {
	a, z := f.span(items[i])
	switch {
	case len(items) == 1:
		f.add("parameters every call passes one constant", a, z, func(func(a, z int) string) string { return alone })
	case i+1 < len(items):
		na, _ := f.span(items[i+1])
		f.add("parameters every call passes one constant", a, na, func(func(a, z int) string) string { return "" })
	default:
		_, pz := f.span(items[i-1])
		f.add("parameters every call passes one constant", pz, z, func(func(a, z int) string) string { return "" })
	}
}
