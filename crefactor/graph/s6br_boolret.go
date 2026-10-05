package graph

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
	"sort"
	"strings"
)

// BOOLRET ON THE GRAPH (doc/GRAPH-MIGRATION.md, Step6): crefactor/xform's
// BoolRet -- the step that declares `bool` what only ever holds an answer --
// asked of the forms and their edges instead of cc's tree, and made by
// RETYPE and the editor instead of byte offsets.
//
// THE ANALYSIS is the text's, rule for rule, and as the text it goes BY
// NAME: a member is every member so called, a local is every local of its
// function so called, a file-scope object every use of its spelling.  An
// identifier is an atom whose refers edge goes to an ordinary declaration
// (an object, a function, a parameter, a local, an enumerator, an
// external); a literal is an atom with no edge, and C23's `true` and
// `false` are the literals 1 and 0, as cc's predefined macros made them
// for the text.  A function answers a question when every one of its
// returns is an answer -- a truth constant, a comparison, `&&`, `||`, `!`,
// a `?:` of answers, a call of an answering function, a member, parameter,
// local or object that holds one -- found to a fixed point: the least, or
// with Relax the greatest.
//
// THE EDIT: each declaration found is RETYPE'd `bool` (a function's result
// in every declaration of it, a parameter in every declaration of its
// function, every member of the name, a file-scope object in every
// declaration of it, a local); with Relax `x |= E` of an answer into such
// an object becomes `x = (E) || x` (`&&` for `&=`); then, in up to four
// rounds as the text's four parses, every comparison in the core of an
// answer with a truth constant or 0 is the answer, or its negation: `!f()`,
// `!p->m`, `!(...)` as written, else `!(E)`.

// BoolRetOptions are what BoolRet is told: crefactor/xform's BoolRetKnobs
// on the graph.
type BoolRetOptions struct {
	// True and False are the program's named truth constants.
	True, False []string
	// Keep are functions never retyped (main).
	Keep []string
	// Layout is the collection's options, for the members a positional
	// initialiser fills (they stay int).
	Layout CollectOptions
	// Globals takes the core's file-scope objects too; Relax the greatest
	// fixed point, literals 0 and 1 assigned, and `|=`/`&=` of an answer.
	Globals, Relax bool
}

// BoolRetReport is what BoolRet did, the text's numbers.
type BoolRetReport struct {
	Comparisons                                int
	Funcs                                      []string // the functions retyped, sorted
	Decls                                      int      // declarations retyped
	Locals, Members, Params, Globals, Compound int
}

// Lines are the report, the text step's lines.
func (r BoolRetReport) Lines(opt BoolRetOptions) []string {
	out := []string{
		fmt.Sprintf("%d comparisons of an answer with %s or 0 are the answer, or its negation", r.Comparisons,
			strings.Join(append(append([]string{}, opt.True...), opt.False...), ", ")),
		fmt.Sprintf("%d functions answer a question and return bool now, %d declarations retyped: %s",
			len(r.Funcs), r.Decls, strings.Join(r.Funcs[:min(len(r.Funcs), 12)], ", ")+"..."),
		fmt.Sprintf("%d locals that only ever hold an answer are bool too", r.Locals),
		fmt.Sprintf("%d struct members that only ever hold an answer are bool too", r.Members),
		fmt.Sprintf("%d parameters that are only ever given an answer are bool too", r.Params),
	}
	if opt.Globals {
		out = append(out, fmt.Sprintf("%d file-scope objects that only ever hold an answer are bool too", r.Globals))
	}
	if opt.Relax {
		out = append(out, fmt.Sprintf("%d |= and &= of an answer into one are written x = E || x", r.Compound))
	}
	return out
}

type brRun struct {
	e        *Editor
	opt      BoolRetOptions
	yes, no  map[string]bool
	compound []brCompound
}

type brCompound struct {
	name string
	x    *Node // the `|=` or `&=`
}

// brAssign is a value given, with the facts of the function it is given in.
type brAssign struct {
	e  *Node
	lf *brLocals
}

type brFacts struct {
	decls   []*Node // the declarations retyped when it holds an answer
	assigns []brAssign
	bad     bool
}

// brLocals is what a function does with each of its locals, by name.
type brLocals struct {
	fn      string
	params  map[string]int
	decls   map[string]int
	assigns map[string][]*Node
	bad     map[string]bool
}

func (r *brRun) answer(s string) bool { return r.yes[s] || r.no[s] }

// BoolRet runs the rule on the graph: the core is the forms above the
// first include form, the host every form from it.
func (e *Editor) BoolRet(opt BoolRetOptions) (BoolRetReport, error) {
	r := &brRun{e: e, opt: opt, yes: map[string]bool{}, no: map[string]bool{}}
	for _, s := range opt.True {
		r.yes[s] = true
	}
	for _, s := range opt.False {
		r.no[s] = true
	}
	return r.run()
}

// brWalk walks n's lists with their parents: f(x, parent, index).
func brWalk(n *Node, f func(x, p *Node, i int) bool) {
	var walk func(x, p *Node, i int)
	walk = func(x, p *Node, i int) {
		if !f(x, p, i) {
			return
		}
		for j, k := range x.Kids {
			walk(k, x, j)
		}
	}
	walk(n, nil, -1)
}

// ident is the spelling of an ordinary identifier's use at x, through
// parentheses; "" for anything else.
func (r *brRun) ident(x *Node) string {
	for x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	if r.isIdent(x) {
		return x.Atom
	}
	return ""
}

// isIdent says x is an atom that is an ordinary identifier's use.
func (r *brRun) isIdent(x *Node) bool {
	if x.list || len(x.Refs) != 1 || strings.HasPrefix(x.Atom, ".") {
		return false
	}
	t := x.Refs[0]
	switch t.Head() {
	case "typedef", "struct", "union", "enum", "label", "extern-typedef", "extern-struct", "extern-union",
		"extern-enum", "undeclared-label", "unresolved-member", "member":
		return false
	}
	return !isMemberForm(r.e, t)
}

// member is the member a selection at x names, through parentheses.
func brMember(x *Node) string {
	for x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	if (x.Is("->") || x.Is(".")) && len(x.Kids) >= 3 {
		if m := x.Kids[len(x.Kids)-1]; !m.list {
			return m.Atom
		}
	}
	return ""
}

// literal is an atom's value as cc read it: `true` 1, `false` 0.
func brLiteral(x *Node) string {
	if x.list || len(x.Refs) > 0 {
		return ""
	}
	switch x.Atom {
	case "true":
		return "1"
	case "false":
		return "0"
	}
	return x.Atom
}

func brDigit(c byte) bool { return c >= '0' && c <= '9' }

// literal01 says x is the integer literal 0 or 1.
func brLiteral01(x *Node) bool {
	s := brLiteral(x)
	return s == "0" || s == "1"
}

var brRelational = map[string]bool{"==": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true}

// boolish says x's value is 0 or 1 by its form (the text's boolish).
func (r *brRun) boolish(x *Node, yes map[string]bool, lf *brLocals, depth int) bool {
	if depth > 8 {
		return false
	}
	if !x.list {
		if !r.isIdent(x) {
			return false
		}
		s := x.Atom
		if r.answer(s) {
			return true
		}
		if yes["g:"+s] && (lf == nil || lf.decls[s] == 0 && !lf.isParam(s)) {
			return true
		}
		return lf.boolish(r, s, yes, depth)
	}
	args := x.Args()
	switch h := x.Head(); h {
	case "paren":
		return len(args) == 1 && r.boolish(args[0], yes, lf, depth)
	case "==", "!=", "<", ">", "<=", ">=", "&&", "||", "!":
		return true
	case "?":
		return len(args) == 3 && r.boolish(args[1], yes, lf, depth) && r.boolish(args[2], yes, lf, depth)
	case "->", ".":
		if m := brMember(x); m != "" {
			return yes["."+m]
		}
	case "call":
		if len(args) > 0 && r.isIdent(args[0]) {
			return yes[args[0].Atom]
		}
	}
	return false
}

func (lf *brLocals) isParam(nm string) bool {
	_, ok := lf.params[nm]
	return ok
}

func brParamKey(fn string, i int) string { return fmt.Sprintf("p:%s:%d", fn, i) }

func (lf *brLocals) boolish(r *brRun, nm string, yes map[string]bool, depth int) bool {
	if lf != nil {
		if i, ok := lf.params[nm]; ok && lf.decls[nm] == 0 {
			return yes[brParamKey(lf.fn, i)]
		}
	}
	if lf == nil || lf.decls[nm] != 1 || lf.bad[nm] || len(lf.assigns[nm]) == 0 {
		return false
	}
	for _, x := range lf.assigns[nm] {
		if !r.boolish(x, yes, lf, depth+1) {
			return false
		}
	}
	return true
}

// defValue is a def's initialiser, or nil.
func brDefValue(d *Node) *Node {
	i := defNameAt(d)
	if i == 0 || i+2 >= len(d.Kids) {
		return nil
	}
	v := d.Kids[i+2]
	if isAttrForm(v) {
		return nil
	}
	return v
}

// params are a function form's parameters as cc's list gives them: none
// for a variadic one, the names of the rest ("" unnamed), `(void)` one
// unnamed.
func brParams(fn *Node) ([]*Node, bool) {
	if fn == nil || !fn.Is("fn") || len(fn.Kids) < 2 || !fn.Kids[1].list {
		return nil, false
	}
	ps := fn.Kids[1].Kids
	for _, p := range ps {
		if !p.list && p.Atom == "..." {
			return nil, false
		}
	}
	return ps, true
}

// isInt says a type form is `int` alone.
func brIntForm(t *Node) bool { return t != nil && !t.list && t.Atom == "int" }

func (r *brRun) factsOf(f *Node) *brLocals {
	lf := &brLocals{fn: topName(f), params: map[string]int{}, decls: map[string]int{},
		assigns: map[string][]*Node{}, bad: map[string]bool{}}
	if ps, ok := brParams(defType(f)); ok {
		for i, p := range ps {
			if p.list {
				if nm := paramName(p); nm != "" {
					lf.params[nm] = i
				}
			}
		}
	}
	for _, it := range Body(f) {
		brWalk(it, func(x, _ *Node, _ int) bool {
			switch h := x.Head(); {
			case h == "def" || h == "typedef":
				if i := defNameAt(x); i > 0 {
					nm := x.Kids[i].Atom
					lf.decls[nm]++
					if v := brDefValue(x); v != nil {
						if v.Is("init") {
							lf.bad[nm] = true
						} else {
							lf.assigns[nm] = append(lf.assigns[nm], v)
						}
					}
				}
			case assignOps[h] && len(x.Kids) == 3:
				if nm := r.ident(x.Kids[1]); nm != "" {
					if h == "=" {
						lf.assigns[nm] = append(lf.assigns[nm], x.Kids[2])
					} else {
						lf.bad[nm] = true
					}
				}
			case h == "addr" || incDec[h]:
				if len(x.Kids) == 2 {
					if nm := r.ident(x.Kids[1]); nm != "" {
						lf.bad[nm] = true
					}
				}
			}
			return true
		})
		r.comparedWithCode(it, r.ident, func(s string) { lf.bad[s] = true })
	}
	return lf
}

// otherConst says x is a constant that is not an answer.
func (r *brRun) otherConst(x *Node) bool {
	for x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	if !x.list {
		if r.isIdent(x) {
			s := x.Atom
			if r.answer(s) {
				return false
			}
			return s == strings.ToUpper(s)
		}
		s := brLiteral(x)
		switch {
		case s == "":
			return false
		case brDigit(s[0]):
			if strings.ContainsAny(s, ".eEpP") && !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
				return false // a floating constant
			}
			return s != "0" && s != "1"
		case s[0] == '\'':
			return true
		}
		return false
	}
	return x.Is("-") && len(x.Kids) == 2
}

// comparedWithCode calls mark on the name every comparison with a code
// compares.
func (r *brRun) comparedWithCode(n *Node, name func(*Node) string, mark func(string)) {
	brWalk(n, func(x, _ *Node, _ int) bool {
		if !brRelational[x.Head()] || len(x.Kids) != 3 {
			return true
		}
		l, rr := x.Kids[1], x.Kids[2]
		if r.otherConst(rr) {
			if s := name(l); s != "" {
				mark(s)
			}
		}
		if r.otherConst(l) {
			if s := name(rr); s != "" {
				mark(s)
			}
		}
		return true
	})
}

// hostWords are the identifiers the host spells, on lines that do not open
// with `#include ` (edit.MentionCount's rule).
func brHostWords(text []byte) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(string(text), "\n") {
		if strings.HasPrefix(line, "#include ") {
			continue
		}
		for i := 0; i < len(line); {
			if !brIdentByte(line[i]) {
				i++
				continue
			}
			j := i
			for j < len(line) && brIdentByte(line[j]) {
				j++
			}
			out[line[i:j]] = true
			i = j
		}
	}
	return out
}

func brIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func (r *brRun) run() (BoolRetReport, error) {
	var rep BoolRetReport
	e := r.e
	core := e.Core()
	if len(core) == len(e.g.Forms) {
		return rep, fmt.Errorf("the core does not end where this step was told it does: no include form")
	}
	inCore := map[*Node]bool{}
	for _, f := range core {
		inCore[f] = true
	}
	hostC, err := FormsC(e.Host())
	if err != nil {
		return rep, err
	}
	host := brHostWords(hostC)

	type fnInfo struct {
		name    string
		decls   []*Node // the definition and every prototype
		returns []*Node
		locals  *brLocals
	}
	fns := map[string]*fnInfo{}
	facts := map[*Node]*brLocals{}
	factsOf := func(f *Node) *brLocals {
		if lf := facts[f]; lf != nil {
			return lf
		}
		lf := r.factsOf(f)
		facts[f] = lf
		return lf
	}
	for _, f := range core {
		if !f.Is("defn") {
			continue
		}
		t := defType(f)
		if t == nil || !t.Is("fn") || len(t.Kids) != 3 || !brIntForm(t.Kids[2]) {
			continue
		}
		fi := &fnInfo{name: topName(f), decls: []*Node{f}, locals: factsOf(f)}
		for _, it := range Body(f) {
			Walk(it, func(x *Node) bool {
				if x.Is("return") {
					if len(x.Kids) > 1 {
						fi.returns = append(fi.returns, x.Kids[1])
					} else {
						fi.returns = append(fi.returns, nil)
					}
				}
				return true
			})
		}
		fns[fi.name] = fi
	}
	for _, k := range r.opt.Keep {
		delete(fns, k)
	}
	for _, f := range core {
		if !f.Is("def") {
			continue
		}
		fi := fns[topName(f)]
		if fi == nil {
			continue
		}
		t := defType(f)
		if t == nil || !t.Is("fn") || len(t.Kids) != 3 || !brIntForm(t.Kids[2]) {
			delete(fns, fi.name)
			continue
		}
		fi.decls = append(fi.decls, f)
	}
	plain := map[string]bool{}
	defs := map[string]*Node{}
	for _, f := range core {
		if f.Is("defn") {
			plain[topName(f)] = true
			defs[topName(f)] = f
		}
	}
	for _, k := range r.opt.Keep {
		delete(plain, k)
	}
	for _, f := range e.g.Forms {
		brWalk(f, func(x, p *Node, i int) bool {
			if !x.list && r.isIdent(x) && !(p != nil && p.Is("call") && i == 1) {
				delete(fns, x.Atom)
				delete(plain, x.Atom)
			}
			return true
		})
	}
	for name := range plain {
		if host[name] {
			delete(fns, name)
			delete(plain, name)
		}
	}
	params := r.paramCandidates(core, plain, defs, factsOf)
	fields := r.memberCandidates(core, inCore, host, factsOf)
	var globals map[string]*brFacts
	if r.opt.Globals {
		globals = r.globalCandidates(core, inCore, host, factsOf)
	}
	allAnswers := func(as []brAssign, yes map[string]bool) bool {
		for _, a := range as {
			if !r.boolish(a.e, yes, a.lf, 0) {
				return false
			}
		}
		return true
	}
	cands := map[string]func(map[string]bool) bool{}
	for k, pc := range params {
		if !pc.bad && len(pc.assigns) > 0 {
			as := pc.assigns
			cands[k] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for m, fc := range fields {
		if !fc.bad && len(fc.assigns) > 0 {
			as := fc.assigns
			cands["."+m] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for g, gc := range globals {
		if !gc.bad && len(gc.assigns) > 0 {
			as := gc.assigns
			cands["g:"+g] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for name, fi := range fns {
		if len(fi.returns) > 0 {
			fi := fi
			cands[name] = func(yes map[string]bool) bool {
				for _, x := range fi.returns {
					if x == nil || !r.boolish(x, yes, fi.locals, 0) {
						return false
					}
				}
				return true
			}
		}
	}
	yes := map[string]bool{}
	if !r.opt.Relax {
		for grew := true; grew; {
			grew = false
			for k, check := range cands {
				if !yes[k] && check(yes) {
					yes[k] = true
					grew = true
				}
			}
		}
	} else {
		for k := range cands {
			yes[k] = true
		}
		for shrank := true; shrank; {
			shrank = false
			for k := range yes {
				if !cands[k](yes) {
					delete(yes, k)
					shrank = true
				}
			}
		}
	}

	// what is retyped, in the file's order
	type retype struct {
		d      *Node
		result string // a function's name: its result
		key    string // a parameter's or an object's
		global bool
	}
	var todo []retype
	keys := func(m map[string]*brFacts, prefix string) []string {
		var ks []string
		for k := range m {
			if yes[prefix+k] {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		return ks
	}
	for _, k := range keys(params, "") {
		rep.Decls += len(params[k].decls)
		rep.Params++
		todo = append(todo, retype{d: params[k].decls[0], key: k})
	}
	for _, m := range keys(fields, ".") {
		rep.Decls += len(fields[m].decls)
		rep.Members++
		for _, d := range fields[m].decls {
			todo = append(todo, retype{d: d})
		}
	}
	for _, g := range keys(globals, "g:") {
		rep.Decls += len(globals[g].decls)
		rep.Globals++
		todo = append(todo, retype{d: globals[g].decls[0], key: g, global: true})
	}
	for name := range yes {
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "p:") || strings.HasPrefix(name, "g:") {
			continue
		}
		rep.Funcs = append(rep.Funcs, name)
		rep.Decls += len(fns[name].decls)
	}
	sort.Strings(rep.Funcs)
	for _, name := range rep.Funcs {
		todo = append(todo, retype{result: name})
	}
	// the locals that hold an answer
	for _, f := range core {
		if !f.Is("defn") {
			continue
		}
		lf := factsOf(f)
		for _, it := range Body(f) {
			Walk(it, func(x *Node) bool {
				if !x.Is("def") {
					return true
				}
				i := defNameAt(x)
				if i == 0 || !brIntForm(defType(x)) {
					return true
				}
				if lf.boolish(r, x.Kids[i].Atom, yes, 0) {
					todo = append(todo, retype{d: x})
					rep.Decls++
					rep.Locals++
				}
				return true
			})
		}
	}
	var places []*Node
	for _, t := range todo {
		if t.result != "" {
			places = append(places, fns[t.result].decls...)
			continue
		}
		if isParam(t.d) {
			places = append(places, params[t.key].decls...)
			continue
		}
		places = append(places, t.d)
		if t.global {
			places = append(places, globals[t.key].decls[1:]...)
		}
	}
	if err := e.brRetype(places); err != nil {
		return rep, err
	}
	// Relax: x |= E written x = (E) || x
	for _, c := range r.compound {
		if !yes["g:"+c.name] {
			continue
		}
		op := "||"
		if c.x.Is("&=") {
			op = "&&"
		}
		lhs, val := c.x.Kids[1], c.x.Kids[2]
		x := lhs
		for x.Is("paren") {
			x = x.Kids[1]
		}
		d := x.Ref()
		or := NewList(NewAtom(op), val, e.RefTo(d))
		r.e.g.save(or)
		or.Type = basicType(e.g, []string{"int"})
		set := NewList(NewAtom("="), lhs, or)
		r.e.g.save(set)
		set.Type = typeOf(lhs)
		if err := e.Replace(c.x, set); err != nil {
			return rep, err
		}
		if err := r.parenthesise(val); err != nil {
			return rep, err
		}
		rep.Compound++
	}
	// the comparisons, outermost first, in rounds
	for pass := 0; pass < 4; pass++ {
		n, err := r.rewriteComparisons(e.Core(), yes)
		if err != nil {
			return rep, err
		}
		if n == 0 {
			break
		}
		rep.Comparisons += n
	}
	e.brTypeUntyped()
	return rep, nil
}

// parenthesise puts x in a paren node, where the C view does not write
// parentheses around it by itself.
func (r *brRun) parenthesise(x *Node) error {
	if r.e.parenthesised(x) {
		return nil
	}
	p := NewList(NewAtom("paren"), x)
	r.e.g.save(p)
	p.Type = typeOf(x)
	return r.e.Replace(x, p)
}

// brRetype makes every declaration of places `bool` in one edit: RETYPE's
// retypeForms for a set the analysis has proved fit -- a function only ever
// called, every declaration of it named -- so that its question, asked once
// a declaration of a function, is not asked again (it walks the file).  A
// place that declares a function is its result's.
func (e *Editor) brRetype(places []*Node) error {
	src, err := clisp.Read([]byte("bool"))
	if err != nil {
		return err
	}
	forms := make([]*Node, len(places))
	news := make([]*Node, len(places))
	var fns []*Node
	inFns := map[*Node]bool{}
	addFn := func(f *Node) {
		if f != nil && !inFns[f] {
			inFns[f] = true
			fns = append(fns, f)
		}
	}
	for k, d := range places {
		if isFuncDecl(d) {
			forms[k] = defType(d).Kids[2]
			addFn(d)
		} else {
			forms[k] = declTypeForm(d)
			if isParam(d) {
				addFn(e.Parent(d.up.up))
			}
		}
		p, i := e.declPlace(d)
		b := &builder{e: e, p: p, i: i, used: map[string]bool{}, local: map[string]*Node{}}
		news[k] = b.typ(src[0])
		if b.err != nil {
			return fmt.Errorf("retype: %w", b.err)
		}
	}
	tx := e.typeTx()
	newT := map[*Node]*Node{}
	var changed []*Node
	change := func(d, t *Node) {
		if _, ok := newT[d]; !ok {
			changed = append(changed, d)
		}
		newT[d] = t
	}
	formOf := map[*Node]*Node{}
	for k := range places {
		formOf[forms[k]] = news[k]
	}
	for k, d := range places {
		if !isFuncDecl(d) {
			change(d, tx.formType(news[k], isParam(d)))
		}
	}
	for _, f := range fns {
		ft := defType(f)
		old, variadic, result, ok := funcParts(f.Type)
		if !ok {
			return fmt.Errorf("retype: %s's type is not a function's", label(f))
		}
		var ps []*Node
		for j, p := range paramElems(ft) {
			if !p.list && p.Atom == "..." {
				continue
			}
			if t, ok := newT[p]; ok {
				ps = append(ps, t)
			} else if j < len(old) {
				ps = append(ps, old[j])
			}
		}
		if len(paramElems(ft)) == 0 {
			ps = old
		}
		if nf := formOf[ft.Kids[2]]; nf != nil {
			result = tx.formType(nf, false)
		}
		change(f, tx.function(ps, variadic, result))
	}
	tx.commit()
	for k, f := range forms {
		if err := e.Replace(f, news[k]); err != nil {
			return err
		}
		e.typedAll(news[k])
	}
	act := Act{Op: "retype"}
	for _, d := range changed {
		if d.Type != newT[d] {
			e.g.save(d)
			d.Type = newT[d]
			if d.ID != 0 {
				act.Moved = append(act.Moved, d.ID)
			}
		}
		if d.Type == nil {
			e.untype(d)
		} else {
			e.typed(d)
		}
	}
	e.Log = append(e.Log, act)
	rederive := func(ds []*Node) {
		for _, d := range ds {
			for _, u := range e.Uses(d) {
				if u.up != nil {
					tx.rederiveCount(u.up)
				}
			}
		}
	}
	rederive(changed)
	// a declaration of `typeof(E)` has E's type: as E's changes, so does it
	for {
		var more []*Node
		for _, f := range e.g.Forms {
			Walk(f, func(d *Node) bool {
				if !d.Is("def") {
					return true
				}
				t := defType(d)
				if !(t.Is("typeof") || t.Is("typeof_unqual") || t.Is("__typeof__")) || len(t.Kids) != 2 {
					return true
				}
				if nt := typeOf(t.Kids[1]); nt != nil && nt != d.Type {
					e.g.save(d)
					d.Type = nt
					e.typed(d)
					more = append(more, d)
				}
				return true
			})
		}
		if len(more) == 0 {
			break
		}
		rederive(more)
	}
	tx.commit()
	return nil
}

// constOf says x is a truth constant or 0: "yes" or "no".
func (r *brRun) constOf(x *Node) (string, bool) {
	if !x.list && brLiteral(x) == "0" {
		return "no", true
	}
	if !r.isIdent(x) {
		return "", false
	}
	switch {
	case r.yes[x.Atom]:
		return "yes", true
	case r.no[x.Atom]:
		return "no", true
	}
	return "", false
}

func (r *brRun) rewriteComparisons(core []*Node, yes map[string]bool) (int, error) {
	type site struct {
		cmp, x *Node
		negate bool
	}
	var sites []site
	for _, f := range core {
		Walk(f, func(x *Node) bool {
			if !(x.Is("==") || x.Is("!=")) || len(x.Kids) != 3 {
				return true
			}
			var v *Node
			k, isConst := r.constOf(x.Kids[2])
			if isConst {
				v = x.Kids[1]
			} else if k, isConst = r.constOf(x.Kids[1]); isConst {
				v = x.Kids[2]
			}
			if !isConst || !r.boolish(v, yes, nil, 0) {
				return true
			}
			sites = append(sites, site{x, v, (k == "no") == x.Is("==")})
			return false
		})
	}
	for _, s := range sites {
		// the text kept the parentheses it wrote, and those the C view wrote
		// around the answer where it stood: a paren node where the C view
		// would not write them at the new place (C-lisp's own rule)
		paren := s.negate && !(s.x.Is("call") || s.x.Is("->") || s.x.Is(".") || s.x.Is("paren")) ||
			!s.negate && !s.x.Is("paren") && r.e.parenthesised(s.x)
		with := s.x
		if s.negate {
			with = NewList(NewAtom("!"), s.x)
			r.e.g.save(with)
			with.Type = basicType(r.e.g, []string{"int"})
		}
		if err := r.e.Replace(s.cmp, with); err != nil {
			return 0, err
		}
		if paren {
			if err := r.parenthesise(s.x); err != nil {
				return 0, err
			}
		}
		if !s.negate {
			r.e.Rederive(with)
		}
	}
	return len(sites), nil
}

func (r *brRun) paramCandidates(core []*Node, callers map[string]bool, defs map[string]*Node, factsOf func(*Node) *brLocals) map[string]*brFacts {
	out := map[string]*brFacts{}
	for _, f := range core {
		if !f.Is("defn") || !callers[topName(f)] {
			continue
		}
		name := topName(f)
		lf := factsOf(f)
		ps, _ := brParams(defType(f))
		for i, p := range ps {
			pf := &brFacts{}
			out[brParamKey(name, i)] = pf
			pn := paramName(p)
			if !p.list || pn == "" || len(p.Kids) != 2 || !brIntForm(paramTypeForm(p)) {
				pf.bad = true
				continue
			}
			if lf.bad[pn] || lf.decls[pn] != 0 {
				pf.bad = true
				continue
			}
			pf.decls = append(pf.decls, p)
			for _, x := range lf.assigns[pn] {
				pf.assigns = append(pf.assigns, brAssign{x, lf})
			}
		}
	}
	for _, f := range core {
		if !f.Is("def") || !callers[topName(f)] {
			continue
		}
		ps, _ := brParams(defType(f))
		for i, p := range ps {
			pf := out[brParamKey(topName(f), i)]
			if pf == nil {
				continue
			}
			if !brIntForm(paramTypeForm(p)) {
				pf.bad = true
				continue
			}
			pf.decls = append(pf.decls, p)
		}
	}
	for _, f := range core {
		var lf *brLocals
		if f.Is("defn") {
			lf = factsOf(f)
		}
		Walk(f, func(x *Node) bool {
			if !x.Is("call") || len(x.Kids) < 2 || !r.isIdent(x.Kids[1]) || defs[x.Kids[1].Atom] == nil {
				return true
			}
			for i, a := range x.Kids[2:] {
				if pf := out[brParamKey(x.Kids[1].Atom, i)]; pf != nil {
					pf.assigns = append(pf.assigns, brAssign{a, lf})
				}
			}
			return true
		})
	}
	return out
}

func (r *brRun) memberCandidates(core []*Node, inCore map[*Node]bool, host map[string]bool, factsOf func(*Node) *brLocals) map[string]*brFacts {
	out := map[string]*brFacts{}
	get := func(m string) *brFacts {
		if out[m] == nil {
			out[m] = &brFacts{}
		}
		return out[m]
	}
	for _, f := range r.e.g.Forms {
		Walk(f, func(s *Node) bool {
			if !(s.Is("struct") || s.Is("union")) || !isDefForm(s) {
				return true
			}
			for _, m := range members(s) {
				if !m.list || len(m.Kids) == 0 || m.Kids[0].list || m.Is("static_assert") {
					continue
				}
				fc := get(m.Kids[0].Atom)
				if len(m.Kids) != 2 || !brIntForm(m.Kids[1]) || !inCore[f] {
					fc.bad = true
					continue
				}
				fc.decls = append(fc.decls, m)
			}
			return true
		})
	}
	for _, f := range r.e.g.Forms {
		var lf *brLocals
		if f.Is("defn") {
			lf = factsOf(f)
		}
		Walk(f, func(x *Node) bool {
			switch h := x.Head(); {
			case assignOps[h] && len(x.Kids) == 3:
				if m := brMember(x.Kids[1]); m != "" {
					if h == "=" {
						get(m).assigns = append(get(m).assigns, brAssign{x.Kids[2], lf})
					} else {
						get(m).bad = true
					}
				}
			case (h == "addr" || incDec[h]) && len(x.Kids) == 2:
				if m := brMember(x.Kids[1]); m != "" {
					get(m).bad = true
				}
			case h == "at":
				for _, d := range x.Args() {
					if !d.list && strings.HasPrefix(d.Atom, ".") {
						get(d.Atom[1:]).bad = true
					}
				}
			}
			return true
		})
		r.comparedWithCode(f, brMember, func(s string) { get(s).bad = true })
	}
	for m := range PositionalMembers(r.e.g, r.opt.Layout) {
		get(m).bad = true
	}
	for m, fc := range out {
		if len(fc.decls) == 0 || host[m] {
			fc.bad = true
		}
	}
	return out
}

func (r *brRun) globalCandidates(core []*Node, inCore map[*Node]bool, host map[string]bool, factsOf func(*Node) *brLocals) map[string]*brFacts {
	out := map[string]*brFacts{}
	get := func(m string) *brFacts {
		if out[m] == nil {
			out[m] = &brFacts{}
		}
		return out[m]
	}
	for _, f := range r.e.g.Forms {
		if !(f.Is("def") || f.Is("typedef")) {
			continue
		}
		i := defNameAt(f)
		if i == 0 {
			continue
		}
		t := defType(f)
		if t.Is("fn") {
			continue
		}
		fc := get(f.Kids[i].Atom)
		if !brIntForm(t) || !inCore[f] {
			fc.bad = true
			continue
		}
		fc.decls = append(fc.decls, f)
		if v := brDefValue(f); v != nil {
			switch {
			case v.Is("init"):
				fc.bad = true
			case !brLiteral01(v):
				fc.assigns = append(fc.assigns, brAssign{v, nil})
			}
		}
	}
	for _, f := range r.e.g.Forms {
		if f.Is("defn") {
			continue
		}
		Walk(f, func(x *Node) bool {
			if x.Is("addr") && len(x.Kids) == 2 {
				if m := r.ident(x.Kids[1]); m != "" {
					get(m).bad = true
				}
			}
			return true
		})
	}
	for _, f := range r.e.g.Forms {
		if !f.Is("defn") {
			continue
		}
		lf := factsOf(f)
		for nm := range lf.decls {
			get(nm).bad = true
		}
		for nm := range lf.params {
			get(nm).bad = true
		}
		for _, it := range Body(f) {
			Walk(it, func(x *Node) bool {
				switch h := x.Head(); {
				case assignOps[h] && len(x.Kids) == 3:
					if m := r.ident(x.Kids[1]); m != "" {
						switch {
						case h == "=" && r.opt.Relax && !x.Kids[2].list && brLiteral01(x.Kids[2]):
						case h == "=":
							get(m).assigns = append(get(m).assigns, brAssign{x.Kids[2], lf})
						case r.opt.Relax && (h == "|=" || h == "&="):
							get(m).assigns = append(get(m).assigns, brAssign{x.Kids[2], lf})
							r.compound = append(r.compound, brCompound{m, x})
						default:
							get(m).bad = true
						}
					}
				case (h == "addr" || incDec[h]) && len(x.Kids) == 2:
					if m := r.ident(x.Kids[1]); m != "" {
						get(m).bad = true
					}
				}
				return true
			})
			r.comparedWithCode(it, r.ident, func(s string) { get(s).bad = true })
		}
	}
	// a size or a type taken of one
	for _, f := range r.e.g.Forms {
		Walk(f, func(x *Node) bool {
			switch x.Head() {
			case "sizeof", "sizeof-bare", "typeof", "typeof_unqual", "__typeof__":
				Walk(x, func(y *Node) bool {
					if r.isIdent(y) {
						get(y.Atom).bad = true
					}
					return true
				})
			}
			return true
		})
	}
	for m, fc := range out {
		if len(fc.decls) == 0 || host[m] {
			fc.bad = true
		}
	}
	return out
}

// PositionalMembers are the names of the members of every struct or union a
// positional initialiser fills, and of every one it holds by value: the
// collection's guard (crefactor/sweep's PositionalMembers on the graph).
func PositionalMembers(g *Graph, opt CollectOptions) map[string]bool {
	c := newCollector(g, opt)
	c.collect()
	c.positional()
	out := map[string]bool{}
	for _, s := range c.ents {
		if s.kind == 'S' && s.pinAll {
			for _, m := range s.members {
				out[m.name] = true
			}
		}
	}
	return out
}
