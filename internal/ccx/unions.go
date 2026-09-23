package ccx

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
	"modernc.org/token"
)

// A union read through a member other than the one last written is a pun: the
// bytes of one value read as another.  Go has no unions; an emitter gives each
// member a field of its own, which is faithful exactly when nothing puns.
// Unions proves that by discriminant: each union here has a value that says
// which member holds, every access must sit where that value is known, and the
// value must not change between the write and the read.
//
// What is known where is a set of guards, collected on the way down:
//
//	+C, -C      the then and else of if (C), each operand of a && chain, the
//	            negation of each operand of a || chain, the arms of C ? : ;
//	            and -C after an if (C) whose then ends in a jump
//	case:X      the case labels a statement falls under since the last jump
//	after:S     an expression statement S earlier in an enclosing block
//	before:S    an expression statement S later in an enclosing block
//
// and a function every direct call to which is under a guard has that guard
// on entry.  A guard is the condition's source with the parentheses around it
// stripped and a leading ! turned into the other sign.

type guards []string

func (g guards) with(s ...string) guards {
	return append(append(guards{}, g...), s...)
}

func (g guards) has(re *regexp.Regexp) bool {
	for _, s := range g {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// cond turns a condition into the guards its being true, or false, gives.
func cond(e cc.ExpressionNode, truth bool) []string {
	e = unparen(e)
	if u, ok := e.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionNot {
		return cond(u.CastExpression, !truth)
	}
	if a, ok := e.(*cc.LogicalAndExpression); ok && a.Case == cc.LogicalAndExpressionLAnd && truth {
		return append(cond(a.LogicalAndExpression, true), cond(a.InclusiveOrExpression, true)...)
	}
	if o, ok := e.(*cc.LogicalOrExpression); ok && o.Case == cc.LogicalOrExpressionLOr && !truth {
		return append(cond(o.LogicalOrExpression, false), cond(o.LogicalAndExpression, false)...)
	}
	if c, ok := e.(*cc.CastExpression); ok && c.Case == cc.CastExpressionUnary {
		return cond(c.UnaryExpression, truth)
	}
	s := srcOrdered(e)
	if truth {
		return []string{"+" + s}
	}
	return []string{"-" + s}
}

// A UnionAccess is one member access of a union, with what is known there.
type UnionAccess struct {
	Fn, Where     string
	Union, Member string // the field holding the union, and the member used
	Base          string
	Guards        guards
	at            int
}

type callSite struct {
	caller string
	g      guards
	at     int // line*10000+column, for order within a function
}

func at(p interface{ Position() token.Position }) int {
	q := p.Position()
	return q.Line*10000 + q.Column
}

type assign struct {
	fn, lhs, rhs, where string
	g                   guards
}

type span struct{ from, to int }

func (s span) holds(p int) bool { return s.from <= p && p <= s.to }

type unionWalk struct {
	deadEnds map[string][]span // the thens that end in a jump, by function
	loops    map[string][]span
	acc      []UnionAccess
	assigns  []assign
	indirect map[string]bool // functions that call through a pointer
	calls    map[string][]callSite
	uses     map[string]int // every use of a name as an expression
}

// unionOf is the union a member access selects from, or nil.
func unionOf(x *cc.PostfixExpression) bool {
	t := typeOf(x.PostfixExpression)
	if t == nil {
		return false
	}
	if x.Case == cc.PostfixExpressionPSelect {
		p, ok := t.(*cc.PointerType)
		if !ok {
			return false
		}
		t = p.Elem()
	}
	return t.Kind() == cc.Union
}

func stmtOf(bi *cc.BlockItem) *cc.Statement {
	if bi.Case == cc.BlockItemStmt {
		return bi.Statement
	}
	return nil
}

// stmtSpan is where a block or a jump statement starts and ends.
func stmtSpan(s *cc.Statement) (span, bool) {
	switch {
	case s == nil:
	case s.Case == cc.StatementCompound:
		return span{at(&s.CompoundStatement.Token), at(&s.CompoundStatement.Token2)}, true
	case s.Case == cc.StatementJump:
		j := s.JumpStatement
		end := at(&j.Token2)
		if p := at(&j.Token3); p > end {
			end = p
		}
		return span{at(&j.Token), end}, true
	}
	return span{}, false
}

// precedes says whether a call at c can run before an access at a in fn:
// earlier in the text or in a loop with it, and not in a then that ends in a
// jump the access is outside.
func (w *unionWalk) precedes(fn string, c, a int) bool {
	for _, d := range w.deadEnds[fn] {
		if d.holds(c) && !d.holds(a) {
			return false
		}
	}
	if c < a {
		return true
	}
	for _, l := range w.loops[fn] {
		if l.holds(c) && l.holds(a) {
			return true
		}
	}
	return false
}

// jumps says whether a statement always ends in a jump.
func jumps(s *cc.Statement) bool {
	if s == nil {
		return false
	}
	switch s.Case {
	case cc.StatementJump:
		return true
	case cc.StatementCompound:
		var last *cc.BlockItem
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			last = l.BlockItem
		}
		return last != nil && jumps(stmtOf(last))
	}
	return false
}

func (w *unionWalk) node(n cc.Node, fn string, g guards) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *cc.FunctionDefinition:
		w.node(x.CompoundStatement, x.Declarator.Name(), nil)
		return
	case *cc.CompoundStatement:
		w.block(x, fn, g)
		return
	case *cc.SelectionStatement:
		w.node(x.ExpressionList, fn, g)
		switch x.Case {
		case cc.SelectionStatementIf:
			w.node(x.Statement, fn, g.with(cond(x.ExpressionList, true)...))
		case cc.SelectionStatementIfElse:
			w.node(x.Statement, fn, g.with(cond(x.ExpressionList, true)...))
			w.node(x.Statement2, fn, g.with(cond(x.ExpressionList, false)...))
		default:
			w.node(x.Statement, fn, g.with("switch:"+srcOrdered(x.ExpressionList)))
		}
		return
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			w.node(x.LogicalAndExpression, fn, g)
			w.node(x.InclusiveOrExpression, fn, g.with(cond(x.LogicalAndExpression, true)...))
			return
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			w.node(x.LogicalOrExpression, fn, g)
			w.node(x.LogicalAndExpression, fn, g.with(cond(x.LogicalOrExpression, false)...))
			return
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			w.node(x.LogicalOrExpression, fn, g)
			w.node(x.ExpressionList, fn, g.with(cond(x.LogicalOrExpression, true)...))
			w.node(x.ConditionalExpression, fn, g.with(cond(x.LogicalOrExpression, false)...))
			return
		}
	case *cc.IterationStatement:
		if fn != "" {
			if sp, ok := stmtSpan(x.Statement); ok {
				w.loops[fn] = append(w.loops[fn], sp)
			}
		}
	case *cc.AssignmentExpression:
		if fn != "" && x.Case != cc.AssignmentExpressionCond {
			w.assigns = append(w.assigns, assign{fn, srcOrdered(x.UnaryExpression), srcOrdered(x.AssignmentExpression), x.Token.Position().String(), g})
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionCall:
			if name := callee(x); name != "" && fn != "" {
				w.calls[name] = append(w.calls[name], callSite{fn, g, at(&x.Token)})
			} else if fn != "" {
				w.indirect[fn] = true
			}
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if fn != "" && unionOf(x) {
				w.acc = append(w.acc, UnionAccess{
					Fn: fn, Where: x.Token2.Position().String(),
					Union: memberName(x.PostfixExpression), Member: x.Token2.SrcStr(),
					Base: srcOrdered(x.PostfixExpression), Guards: g, at: at(&x.Token2),
				})
			}
		}
	}
	for _, c := range children(n) {
		w.node(c, fn, g)
	}
}

// block visits a compound statement's items in order, each with the case
// labels it falls under, the negation of every earlier if whose then jumps,
// and its sibling expression statements.
func (w *unionWalk) block(cs *cc.CompoundStatement, fn string, g guards) {
	var items []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		items = append(items, l.BlockItem)
	}
	text := make([]string, len(items))
	for i, it := range items {
		s := stmtOf(it)
		for s != nil && s.Case == cc.StatementLabeled {
			s = s.LabeledStatement.Statement
		}
		if s != nil && s.Case == cc.StatementExpr && s.ExpressionStatement.ExpressionList != nil {
			text[i] = srcOrdered(s.ExpressionStatement.ExpressionList)
		}
	}
	var cases, exits []string
	prevJump := true
	for i, it := range items {
		s := stmtOf(it)
		for s != nil && s.Case == cc.StatementLabeled && s.LabeledStatement.Case == cc.LabeledStatementCaseLabel {
			if prevJump {
				cases = nil
			}
			cases = append(cases, "case:"+srcOrdered(s.LabeledStatement.ConstantExpression))
			prevJump = false
			s = s.LabeledStatement.Statement
		}
		h := g.with(cases...).with(exits...)
		for j := range items {
			if text[j] == "" || j == i {
				continue
			}
			if j < i {
				h = h.with("after:" + text[j])
			} else {
				h = h.with("before:" + text[j])
			}
		}
		if s != nil {
			w.node(s, fn, h)
		} else {
			w.node(it, fn, h)
		}
		if s != nil && s.Case == cc.StatementSelection {
			sel := s.SelectionStatement
			if sel.Case == cc.SelectionStatementIf && jumps(sel.Statement) {
				exits = append(exits, cond(sel.ExpressionList, false)...)
				if sp, ok := stmtSpan(sel.Statement); ok {
					w.deadEnds[fn] = append(w.deadEnds[fn], sp)
				}
			}
		}
		prevJump = s != nil && jumps(s)
	}
}

// entry is what holds on entry to each function: what holds at every direct
// call to it, for a function whose address is never taken.
func (w *unionWalk) entry(funcs map[string]bool) map[string]guards {
	in := map[string]guards{}
	for round := 0; round < 4; round++ {
		next := map[string]guards{}
		for f := range funcs {
			sites := w.calls[f]
			if len(sites) == 0 || w.uses[f] != len(sites) {
				continue
			}
			var common map[string]bool
			for _, s := range sites {
				set := map[string]bool{}
				for _, x := range s.g {
					set[x] = true
				}
				for _, x := range in[s.caller] {
					set[x] = true
				}
				if common == nil {
					common = set
					continue
				}
				for k := range common {
					if !set[k] {
						delete(common, k)
					}
				}
			}
			for k := range common {
				next[f] = append(next[f], k)
			}
			sort.Strings(next[f])
		}
		in = next
	}
	return in
}

// A unionRule says which guards show each member of one union holds.
type unionRule struct {
	union  string
	member map[string]*regexp.Regexp
}

func q(s string) string { return regexp.QuoteMeta(s) }

// The discriminants.  attrentry_T.ae_u holds term in term_attr_table and cterm
// in cterm_attr_table; which table a highlight reads is t_colors > 1.  The
// regexp's saved positions are pos for a multi-line match (rex.reg_match ==
// NULL) and ptr for a string.  A regstack item holds sesave in the states that
// save a subexpression's start or end, regsave in the others.
var unionRules = []unionRule{
	{"ae_u", map[string]*regexp.Regexp{
		"term":  regexp.MustCompile(`^(-` + q("t_colors>1") + `|\+` + q("table==&term_attr_table") + `|before:.*` + q("get_attr_entry(&term_attr_table,&") + `\{base\}\))$`),
		"cterm": regexp.MustCompile(`^(\+` + q("t_colors>1") + `|\+` + q("table==&cterm_attr_table") + `|before:.*` + q("get_attr_entry(&cterm_attr_table,&") + `\{base\}\))$`),
	}},
	{"rs_u", map[string]*regexp.Regexp{
		"pos": regexp.MustCompile(`^\+` + q("rex.reg_match==nullptr") + `$`),
		"ptr": regexp.MustCompile(`^-` + q("rex.reg_match==nullptr") + `$`),
	}},
	{"se_u", map[string]*regexp.Regexp{
		"pos": regexp.MustCompile(`^\+` + q("rex.reg_match==nullptr") + `$`),
		"ptr": regexp.MustCompile(`^-` + q("rex.reg_match==nullptr") + `$`),
	}},
	{"rs_un", map[string]*regexp.Regexp{
		"sesave":  regexp.MustCompile(`^(case:(RS_MOPEN|RS_MCLOSE)|after:.*regstack_push\((RS_MOPEN|RS_MCLOSE),.*)$`),
		"regsave": regexp.MustCompile(`^(case:(RS_BRANCH|RS_BRCPLX_MORE|RS_BRCPLX_LONG|RS_BRCPLX_SHORT|RS_NOMATCH|RS_BEHIND1|RS_BEHIND2|RS_STAR_LONG|RS_STAR_SHORT)|after:.*regstack_push\((RS_BRANCH|RS_BRCPLX_MORE|RS_BRCPLX_LONG|RS_BRCPLX_SHORT|RS_NOMATCH|RS_BEHIND1|RS_STAR_LONG|RS_STAR_SHORT|rst\.minval<=rst\.maxval\?RS_STAR_LONG:RS_STAR_SHORT),.*)$`),
	}},
}

// optKinds maps each option callback to the kinds of the rows that name it,
// read from the options[] table: P_BOOL is boolean, P_NUM number, P_STRING
// string.
func optKinds(ast *cc.AST) map[string]map[string]bool {
	r := map[string]map[string]bool{}
	row := regexp.MustCompile(`P_(BOOL|NUM|STRING)\b`)
	cb := regexp.MustCompile(`\b(did_set_\w+)\b`)
	kind := map[string]string{"BOOL": "boolean", "NUM": "number", "STRING": "string"}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		d, ok := n.(*cc.InitDeclarator)
		if !ok || d.Declarator == nil || d.Declarator.Name() != "options" || d.Initializer == nil {
			return
		}
		for l := d.Initializer.InitializerList; l != nil; l = l.InitializerList {
			t := srcOrdered(l.Initializer)
			k := row.FindStringSubmatch(t)
			if k == nil {
				continue
			}
			for _, m := range cb.FindAllStringSubmatch(t, -1) {
				if r[m[1]] == nil {
					r[m[1]] = map[string]bool{}
				}
				r[m[1]][kind[k[1]]] = true
			}
		}
	})
	return r
}

// optSetters write the one member their kind of option holds, then call the
// row's callback.
var optSetters = map[string]string{"set_bool_option": "boolean", "set_num_option": "number", "did_set_string_option": "string"}

// kindOf is what an option's flags say its kind is where guards g hold: its
// own flag tested, or both the others tested and clear.
var optFlag = map[string]string{"boolean": "P_BOOL", "number": "P_NUM", "string": "P_STRING"}

func impliesKind(g guards, kind string) bool {
	set := map[string]bool{}
	for _, s := range g {
		set[s] = true
	}
	if set["+flags&"+optFlag[kind]] {
		return true
	}
	for k, f := range optFlag {
		if k != kind && !set["-flags&"+f] {
			return false
		}
	}
	return true
}

// runsOnlyFor says every path into fn, by its direct calls and theirs up to
// depth, tests the option's kind to be kind first.
func (w *unionWalk) runsOnlyFor(fn, kind string, depth int) bool {
	sites := w.calls[fn]
	if depth == 0 || len(sites) == 0 || w.uses[fn] != len(sites) {
		return false
	}
	for _, s := range sites {
		if !impliesKind(s.g, kind) && !w.runsOnlyFor(s.caller, kind, depth-1) {
			return false
		}
	}
	return true
}

// Unions partitions every union member access by the discriminant that says
// the member holds there.
func Unions(ast *cc.AST) Result {
	w := &unionWalk{deadEnds: map[string][]span{}, loops: map[string][]span{}, calls: map[string][]callSite{}, uses: map[string]int{}, indirect: map[string]bool{}}
	funcs := map[string]bool{}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionIdent {
			w.uses[x.Token.SrcStr()]++
		}
	})
	for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
		if fd := l.ExternalDeclaration.FunctionDefinition; fd != nil {
			funcs[fd.Declarator.Name()] = true
			w.node(fd, "", nil)
		}
	}
	in := w.entry(funcs)
	kinds := optKinds(ast)
	res := Result{Title: "union members", Classes: map[string]int{}}
	for _, a := range w.acc {
		g := a.Guards.with(in[a.Fn]...)
		class := ""
		switch a.Union {
		case "os_oldval", "os_newval":
			if k, ok := optSetters[a.Fn]; ok && k == a.Member && w.runsOnlyFor(a.Fn, k, 4) {
				class = "an option's old and new value, written by the setter of its kind"
			} else if ks := kinds[a.Fn]; len(ks) == 1 && ks[a.Member] && len(w.calls[a.Fn]) == 0 {
				class = "an option's old and new value, read by a callback only rows of its kind name"
			}
		default:
			for _, r := range unionRules {
				if r.union != a.Union {
					continue
				}
				re := r.member[a.Member]
				if re != nil && strings.Contains(re.String(), `\{base\}`) {
					// a local built for a table names itself when handed to it
					v := strings.TrimSuffix(strings.TrimSuffix(a.Base, "."+a.Union), "->"+a.Union)
					re = regexp.MustCompile(strings.ReplaceAll(re.String(), `\{base\}`, regexp.QuoteMeta(v)))
				}
				if re != nil && g.has(re) {
					class = fmt.Sprintf("%s.%s, where its discriminant says it holds", a.Union, a.Member)
				}
			}
		}
		if class == "" {
			what := a.Union + "." + a.Member + " of " + a.Base
			if os.Getenv("CCX_GUARDS") != "" {
				what += fmt.Sprintf(" %q", g)
			}
			res.Left = append(res.Left, Finding{a.Fn, a.Where, what})
			continue
		}
		res.Classes[class]++
	}

	// Which table a highlight reads is t_colors > 1.
	for name, want := range map[string]string{"syn_cterm_attr2entry": "+t_colors>1", "syn_term_attr2entry": "-t_colors>1"} {
		for _, s := range w.calls[name] {
			if !s.g.with(in[s.caller]...).has(regexp.MustCompile("^" + regexp.QuoteMeta(want) + "$")) {
				res.Left = append(res.Left, Finding{s.caller, "", name + " called where t_colors does not name its table"})
				continue
			}
			res.Classes["a highlight table read where t_colors names it"]++
		}
	}

	// A regstack item's state moves only within the class of its member.
	sesave := regexp.MustCompile(`^(RS_MOPEN|RS_MCLOSE)$`)
	for _, a := range w.assigns {
		if !regexp.MustCompile(`(->|\.)rs_state$`).MatchString(a.lhs) {
			continue
		}
		if a.fn == "regstack_push" && a.rhs == "state" {
			res.Classes["a regstack item's state, set by the push that names it"]++
			continue
		}
		ok := false
		for _, s := range a.g {
			if c, found := strings.CutPrefix(s, "case:"); found && sesave.MatchString(c) == sesave.MatchString(a.rhs) {
				ok = true
			}
		}
		if !ok {
			res.Left = append(res.Left, Finding{a.fn, a.where, "rs_state = " + a.rhs + " may change which member holds"})
			continue
		}
		res.Classes["a regstack item's state, moved within its member's class"]++
	}

	// The discriminants do not change under the functions that read them.
	callees := map[string][]string{}
	for f, sites := range w.calls {
		for _, s := range sites {
			callees[s.caller] = append(callees[s.caller], f)
		}
	}
	var taken []string
	for f := range funcs {
		if w.uses[f] > len(w.calls[f]) {
			taken = append(taken, f)
		}
	}
	// reach is every function a call from `from` can run, each with the
	// function that called it on a shortest path, "*" through a pointer.
	reach := func(from string, barrier map[string]bool) map[string]string {
		seen := map[string]string{from: ""}
		queue := []string{from}
		for len(queue) > 0 {
			f := queue[0]
			queue = queue[1:]
			next := func(c, via string) {
				if _, ok := seen[c]; !ok && funcs[c] && !barrier[c] {
					seen[c] = via
					queue = append(queue, c)
				}
			}
			for _, c := range callees[f] {
				next(c, f)
			}
			if w.indirect[f] {
				for _, c := range taken {
					next(c, f+"*")
				}
			}
		}
		return seen
	}
	path := func(r map[string]string, to string) string {
		p := to
		for f := r[to]; f != ""; f = r[strings.TrimSuffix(f, "*")] {
			p = f + " > " + p
		}
		return p
	}
	// A function that saves rex on entry and restores it before it returns
	// leaves rex.reg_match as it found it, whatever it runs: a barrier.
	saves := map[string]int{}
	for _, a := range w.assigns {
		if (a.lhs == "rex_save" && a.rhs == "rex") || (a.lhs == "rex" && a.rhs == "rex_save") {
			saves[a.fn]++
		}
	}
	barrier := map[string]bool{}
	for f, n := range saves {
		barrier[f] = n == 2
	}
	// Each discriminant, its guards, and the last access each function makes
	// under one.
	vars := []struct {
		v  string
		re *regexp.Regexp
	}{
		{"t_colors", regexp.MustCompile(`^[+-]t_colors>1$`)},
		{"rex.reg_match", regexp.MustCompile(`^[+-]rex\.reg_match==nullptr$`)},
	}
	for _, d := range vars {
		// every read under the discriminant, with where it is
		reads := map[string][]int{}
		for _, a := range w.acc {
			if a.Guards.has(d.re) {
				reads[a.Fn] = append(reads[a.Fn], a.at)
			}
		}
		for _, name := range []string{"syn_cterm_attr2entry", "syn_term_attr2entry"} {
			for _, s := range w.calls[name] {
				if s.g.has(d.re) {
					reads[s.caller] = append(reads[s.caller], s.at)
				}
			}
		}
		writers := map[string]bool{}
		for _, a := range w.assigns {
			if a.lhs == d.v && !barrier[a.fn] {
				writers[a.fn] = true
			}
		}
		for f, ats := range reads {
			if writers[f] {
				res.Left = append(res.Left, Finding{f, "", d.v + " is written in the function that reads it"})
			}
			// what the reader can call before one of its reads
			r := map[string]string{}
			for c, sites := range w.calls {
				for _, s := range sites {
					if s.caller != f || barrier[c] {
						continue
					}
					before := false
					for _, a := range ats {
						before = before || w.precedes(f, s.at, a)
					}
					if !before {
						continue
					}
					for k, v := range reach(c, barrier) {
						if _, ok := r[k]; !ok {
							r[k] = v
						}
					}
				}
			}
			for wr := range writers {
				if _, ok := r[wr]; ok {
					res.Left = append(res.Left, Finding{f, "", d.v + " is written by " + wr + ", which it can reach: " + path(r, wr)})
				}
			}
		}
		last := reads
		var bs []string
		for f, b := range barrier {
			if b && d.v == "rex.reg_match" {
				bs = append(bs, f)
			}
		}
		sort.Strings(bs)
		res.Classes[fmt.Sprintf("%s: %d functions read under it, and nothing they can call before such a read writes it%s", d.v, len(last), map[bool]string{true: " (saved and restored by " + strings.Join(bs, ", ") + ")", false: ""}[len(bs) > 0])] = 0
	}

	// Every option has one kind.
	for _, n := range optRowsWithout(ast) {
		res.Left = append(res.Left, Finding{"options", "", n})
	}

	sort.Slice(res.Left, func(i, j int) bool { return res.Left[i].Where < res.Left[j].Where })
	return res
}

// optRowsWithout names the options[] rows with other than one of P_BOOL, P_NUM
// and P_STRING.
func optRowsWithout(ast *cc.AST) []string {
	var r []string
	kind := regexp.MustCompile(`\bP_(BOOL|NUM|STRING)\b`)
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		d, ok := n.(*cc.InitDeclarator)
		if !ok || d.Declarator == nil || d.Declarator.Name() != "options" || d.Initializer == nil {
			return
		}
		for l := d.Initializer.InitializerList; l != nil; l = l.InitializerList {
			t := srcOrdered(l.Initializer)
			if k := len(kind.FindAllString(t, -1)); k != 1 && strings.HasPrefix(t, "{\"") {
				r = append(r, fmt.Sprintf("an option row with %d kinds: %.40s", k, t))
			}
		}
	})
	return r
}

// srcOrdered is the source of a node with its tokens in source order, which
// srcText's walk of the fields does not keep, and nullptr written as nullptr.
func srcOrdered(n cc.Node) string {
	type tk struct {
		seq int
		s   string
	}
	var ts []tk
	var tok func(v reflect.Value)
	tok = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
			if v.IsNil() {
				return
			}
			tok(v.Elem())
			return
		}
		if t, ok := v.Interface().(cc.Token); ok {
			if s := t.SrcStr(); s != "" {
				ts = append(ts, tk{t.Seq(), s})
			}
			return
		}
		if v.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				tok(v.Field(i))
			}
		}
	}
	tok(reflect.ValueOf(n))
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].seq < ts[j].seq })
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(t.s)
	}
	return strings.ReplaceAll(b.String(), "((void*)0)", "nullptr")
}
