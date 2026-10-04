package graph

// s6pc_plainc.go is C written plainly where a translation spelled it as the
// preprocessor left it (doc/JAVA-IDIOMS.md, item 8), on the graph: what
// crefactor/xform's plainc.go did on cc's typed tree, asked of the typed
// edges and the forms (doc/GRAPH-MIGRATION.md, Step6).  Three edits, each
// knowing no code base, each on the forms it is handed (a program's core):
//
//   - PlainIdentity: a function whose body returns its one parameter, cast to
//     its result or not -- gettext's `_()` once there was no gettext -- is
//     not called: each call is its argument, cast where the argument's
//     decayed type is not the result's, parenthesised where it is not one
//     operand.  The function stays for whoever names it without calling.
//   - PlainAsciiClass: `(unsigned)c - 'A' < 26`, C's ASCII class test with one
//     comparison, is a call of a function named for it again
//     (ascii_isupper, _islower, _isdigit), defined at the top of the file.
//   - PlainConstBranch: an if whose condition is an integer constant (cc's
//     evaluation, foldx_eval.go) is the branch it takes, a block; none
//     taken is an empty block; not where either branch holds a label.
//
// Each replaces an expression or statement whose value the types above it
// do not depend on, so the typed edges above are kept, not cleared
// (s6pcReplace): a call of `char *_(const char *)` and its `char [N]`
// argument decay alike; `<`'s int and the new call's bool promote alike.

import (
	"fmt"
	"strings"
)

// s6pcReplace replaces old by with, an expression whose value converts in
// old's place as old's did, and keeps the typed edges the editor clears
// above it: each parenthesis directly above with takes its operand's type
// decayed,
// the rest keep theirs.
func (e *Editor) s6pcReplace(old, with *Node) error {
	var chain, types []*Node
	for q := e.Parent(old); q != nil && q.Type != nil && !IsStatement(q) && q.up != e.top[0]; q = e.Parent(q) {
		chain = append(chain, q)
		types = append(types, q.Type)
	}
	if err := e.Replace(old, with); err != nil {
		return err
	}
	inner, parens := with, true
	for i, q := range chain {
		t := types[i]
		if parens && q.Is("paren") && len(q.Kids) == 2 && q.Kids[1] == inner {
			if it := e.s6pcDecay(typeOf(inner)); it != nil {
				t = it
			}
		} else {
			parens = false
		}
		if q.Type != t {
			q.Type = t
			if e.inFile(t) {
				e.typedBy[t] = append(e.typedBy[t], q)
			}
		}
		e.typed(q)
		inner = q
	}
	return nil
}

// s6pcDecay is t after array and function decay, the graph's node for it
// (nil where the graph has none): the type cc gives parentheses around an
// operand of that type.
func (e *Editor) s6pcDecay(t *Node) *Node {
	switch {
	case t.Is("array"):
		return pointerTo(e.g, pointee(t))
	case t.Is("function"):
		return pointerTo(e.g, t)
	}
	return t
}

// s6pcDefns are the function definitions among forms.
func s6pcDefns(forms []*Node) []*Node {
	var out []*Node
	for _, f := range forms {
		if f.Is("defn") {
			out = append(out, f)
		}
	}
	return out
}

// ---- PlainIdentity ---------------------------------------------------------

type s6pcIdent struct {
	result *Node // the result's type node
	cast   *Node // the body's outermost cast's type form, or nil
}

// s6pcBare says n prints as one operand where an argument stood: an atom,
// parentheses, a postfix form but `++` and `--`.
func s6pcBare(n *Node) bool {
	if !n.list {
		return true
	}
	switch n.Head() {
	case "paren", "call", "index", ".", "->", "literal", "generic":
		return true
	}
	return false
}

// s6pcIdentities are the identity functions the forms define, by name.
func s6pcIdentities(forms []*Node) map[string]s6pcIdent {
	ids := map[string]s6pcIdent{}
	for _, f := range s6pcDefns(forms) {
		fn := defType(f)
		if fn == nil || !fn.Is("fn") || f.Type == nil {
			continue
		}
		ps := paramElems(fn)
		if len(ps) != 1 || !ps[0].list || paramName(ps[0]) == "" {
			continue
		}
		params, variadic, result, ok := funcParts(f.Type)
		if !ok || variadic || len(params) != 1 {
			continue
		}
		items := Body(f)
		if len(items) != 1 || !items[0].Is("return") || len(items[0].Kids) != 2 {
			continue
		}
		x := items[0].Kids[1]
		var cast *Node
		for {
			if x.Is("cast") && len(x.Kids) == 3 {
				if cast == nil {
					cast = x.Kids[1]
				}
				x = x.Kids[2]
				continue
			}
			if x.Is("paren") && len(x.Kids) == 2 {
				x = x.Kids[1]
				continue
			}
			break
		}
		if !x.list && x.Ref() == ps[0] {
			ids[DeclName(f)] = s6pcIdent{result: result, cast: cast}
		}
	}
	return ids
}

// s6pcCallee is the function a call's designator names, parentheses
// aside: a file-scope declaration of a function.
func s6pcCallee(c *Node) *Node {
	x := c.Kids[1]
	for x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	d := x.Ref()
	if x.list || d == nil || !(d.Is("defn") || d.Is("def")) || d.Type == nil || !d.Type.Is("function") {
		return nil
	}
	return d
}

// s6pcDecayed is an expression's type after array and function decay, the
// graph's node for it (nil where the graph has none): a plain string
// literal is a `char *`.
func (e *Editor) s6pcDecayed(n *Node) *Node {
	if !n.list && strings.HasPrefix(n.Atom, "\"") {
		return pointerTo(e.g, basicType(e.g, []string{"char"}))
	}
	return e.s6pcDecay(typeOf(n))
}

// s6pcDeclForm is a declaration's type form: a def's, a parameter's or a
// member's.
func s6pcDeclForm(d *Node) *Node {
	switch {
	case d.Is("def") || d.Is("defn") || d.Is("typedef"):
		return defType(d)
	case d.list && len(d.Kids) >= 2:
		return paramTypeForm(d)
	}
	return nil
}

// s6pcQualified says whether the type form t, after strip derivations
// (each a pointer's or an array's), points at or holds a qualified type:
// what cc's type spells `const char *` where the graph's node is `char *`.
// known is false where the form does not say.
func s6pcQualified(t *Node, strip int) (qual, known bool) {
	for depth := 0; t != nil && depth < 16; depth++ {
		switch {
		case !t.list && t.Ref() != nil && t.Ref().Is("typedef"):
			t = defType(t.Ref())
			continue
		case t.Is("paren") || t.Is("name-attr"):
			t = t.Kids[1]
			continue
		case t.Is("ptr") || t.Is("array"):
			el := t.Kids[len(t.Kids)-1]
			if strip > 0 {
				strip--
				t = el
				continue
			}
			for el.Is("paren") {
				el = el.Kids[1]
			}
			if !el.list {
				if d := el.Ref(); d != nil && d.Is("typedef") {
					return s6pcSpecQual(defType(d)), true
				}
				return false, true
			}
			return s6pcSpecQual(el), true
		}
		return false, false
	}
	return false, false
}

// s6pcSpecQual says a specifier list is qualified.
func s6pcSpecQual(t *Node) bool {
	if t == nil || !t.list {
		return false
	}
	switch t.Head() {
	case "ptr", "array", "fn", "fn-ids":
		return false
	}
	for _, k := range t.Kids {
		if !k.list && (k.Atom == "const" || k.Atom == "volatile") {
			return true
		}
	}
	return false
}

// s6pcPointsQualified says an expression of pointer or array type points
// at a qualified type, as its forms say.
func s6pcPointsQualified(n *Node) (qual, known bool) {
	if !n.list {
		if strings.HasPrefix(n.Atom, "\"") {
			return false, true
		}
		if d := n.Ref(); d != nil {
			return s6pcQualified(s6pcDeclForm(d), 0)
		}
		return false, false
	}
	args := n.Args()
	switch n.Head() {
	case "paren":
		return s6pcPointsQualified(args[0])
	case "cast":
		return s6pcQualified(args[0], 0)
	case "?":
		a, ka := s6pcPointsQualified(args[1])
		b, kb := s6pcPointsQualified(args[2])
		return a || b, ka && kb
	case "call":
		if d := s6pcCallee(n); d != nil {
			if fn := s6pcDeclForm(d); fn.Is("fn") && len(fn.Kids) == 3 {
				return s6pcQualified(fn.Kids[2], 0)
			}
		}
	case "index":
		if !args[0].list && args[0].Ref() != nil {
			return s6pcQualified(s6pcDeclForm(args[0].Ref()), len(args)-1)
		}
	case ".", "->":
		if m := args[len(args)-1]; !m.list && m.Ref() != nil {
			return s6pcQualified(s6pcDeclForm(m.Ref()), 0)
		}
	}
	return false, false
}

// PlainIdentity writes each call, in the functions forms defines, of an
// identity function forms defines as its argument: cast as the body casts
// where the argument's type is not the result's, in parentheses where it is
// not one operand.  It says how many calls and functions.
func (e *Editor) PlainIdentity(forms []*Node) (calls, funcs int, err error) {
	ids := s6pcIdentities(forms)
	type site struct {
		call   *Node
		cast   *Node
		paren  bool
		result *Node
	}
	var sites []site
	for _, f := range s6pcDefns(forms) {
		if _, self := ids[DeclName(f)]; self {
			continue
		}
		for _, it := range Body(f) {
			Walk(it, func(x *Node) bool {
				if !x.Is("call") || len(x.Kids) != 3 {
					return true
				}
				d := s6pcCallee(x)
				if d == nil {
					return true
				}
				id, ok := ids[DeclName(d)]
				if !ok {
					return true
				}
				arg := x.Kids[2]
				cast := id.cast
				if t := e.s6pcDecayed(arg); t != nil && t == id.result {
					q, known := s6pcPointsQualified(arg)
					if !known && cast != nil {
						err = fmt.Errorf("identity: cannot tell whether %s points at a qualified type", label(arg))
						return false
					}
					if !q {
						cast = nil
					}
				}
				sites = append(sites, site{call: x, cast: cast, paren: !s6pcBare(arg), result: x.Type})
				return true
			})
			if err != nil {
				return 0, 0, err
			}
		}
	}
	// innermost (last) first: an outer call's argument is then what the
	// inner one became, as the text rendered it
	for i := len(sites) - 1; i >= 0; i-- {
		s := sites[i]
		arg := s.call.Kids[2]
		p, k := e.index(s.call)
		with := arg
		if s.paren && formLevel(arg) >= placeLevelIn(s.cast, p, k) {
			if with, err = e.s6pcBuild(s.call, "(paren ?x)", Bindings{"x": arg}, nil); err != nil {
				return 0, 0, err
			}
			if t := e.s6pcDecay(typeOf(arg)); t != nil {
				with.Type = t // cc's: parentheses decay what they hold
				e.typed(with)
			}
		}
		if s.cast != nil {
			if with, err = e.s6pcBuild(s.call, "(cast "+Lisp(s.cast).String()+" ?x)", Bindings{"x": with}, s.result); err != nil {
				return 0, 0, err
			}
			if formLevel(with) < placeLevel(p, k) {
				return 0, 0, fmt.Errorf("identity: %s would print in parentheses the text does not write", label(s.call))
			}
		}
		if err := e.s6pcReplace(s.call, with); err != nil {
			return 0, 0, err
		}
	}
	return len(sites), len(ids), nil
}

// placeLevelIn is the level an argument asks for: as a cast's operand
// when one is written, else in place k of p.
func placeLevelIn(cast, p *Node, k int) int {
	if cast != nil {
		return xlvCast
	}
	return placeLevel(p, k)
}

// s6pcBuild builds one node at at, typed t where BUILD left it untyped.
func (e *Editor) s6pcBuild(at *Node, src string, holes Bindings, t *Node) (*Node, error) {
	ns, err := e.Build(at, src, holes)
	if err != nil {
		return nil, err
	}
	n := ns[0]
	if n.Type == nil && t != nil {
		n.Type = t
	}
	if n.Type != nil {
		e.typed(n)
	}
	return n, nil
}

// ---- PlainAsciiClass ------------------------------------------------------

// s6pcClasses are the one-comparison tests, by the character and the
// bound, in the order their functions are written.
var s6pcClasses = []struct{ ch, bound, name string }{
	{"'A'", "26", "ascii_isupper"},
	{"'a'", "26", "ascii_islower"},
	{"'0'", "10", "ascii_isdigit"},
}

// PlainAsciiClass writes each `(unsigned)X - 'A' < 26` (and 'a' 26, '0' 10)
// in the functions forms defines as a call of the function named for it,
// and defines the functions used at the top of the file.  It says how many
// of each.
func (e *Editor) PlainAsciiClass(forms []*Node) (map[string]int, error) {
	for _, c := range s6pcClasses {
		if len(e.Decls(c.name)) > 0 {
			return nil, fmt.Errorf("asciiclass: %s is taken", c.name)
		}
	}
	type site struct {
		lt, x *Node
		name  string
	}
	var sites []site
	used := map[string]int{}
	for _, f := range s6pcDefns(forms) {
		for _, it := range Body(f) {
			Walk(it, func(r *Node) bool {
				if !r.Is("<") || len(r.Kids) != 3 || r.Kids[2].list {
					return true
				}
				sub := r.Kids[1]
				if !sub.Is("-") || len(sub.Kids) != 3 || sub.Kids[2].list {
					return true
				}
				name := ""
				for _, c := range s6pcClasses {
					if c.ch == sub.Kids[2].Atom && c.bound == r.Kids[2].Atom {
						name = c.name
					}
				}
				cast := sub.Kids[1]
				if name == "" || !cast.Is("cast") || len(cast.Kids) != 3 ||
					!cast.Type.Is("basic") || len(cast.Type.Kids) != 2 || cast.Type.Kids[1].Atom != "unsigned" {
					return true
				}
				sites = append(sites, site{r, cast.Kids[2], name})
				used[name]++
				return false
			})
		}
	}
	if len(sites) == 0 {
		return used, nil
	}
	var defs strings.Builder
	for _, c := range s6pcClasses {
		if used[c.name] > 0 {
			fmt.Fprintf(&defs, "static inline bool\n%s(int c)\n{\n    return (unsigned)c - %s < %s;\n}\n\n", c.name, c.ch, c.bound)
		}
	}
	if _, err := e.SpliceC(Frag{At: e.SpotBefore(e.g.Forms[0]), Src: defs.String()}); err != nil {
		return nil, fmt.Errorf("asciiclass: %w", err)
	}
	for _, s := range sites {
		// the parentheses the C view wrote around the test stay, as the
		// text kept them, though the call needs none
		tmpl := "(call " + s.name + " ?x)"
		if e.parenthesised(s.lt) {
			tmpl = "(paren " + tmpl + ")"
		}
		call, err := e.s6pcBuild(s.lt, tmpl, Bindings{"x": s.x}, nil)
		if err != nil {
			return nil, fmt.Errorf("asciiclass: %w", err)
		}
		if err := e.s6pcReplace(s.lt, call); err != nil {
			return nil, fmt.Errorf("asciiclass: %w", err)
		}
	}
	return used, nil
}

// ---- PlainConstBranch -----------------------------------------------------

// PlainConstBranch writes each if, in the functions forms defines, whose
// condition is an integer constant as the branch it takes -- a block -- or
// an empty block when it takes none, where neither branch holds a label.
// It says how many.
func (e *Editor) PlainConstBranch(forms []*Node) (int, error) {
	x := &xfold{e: e, enumVals: map[*Node]int64{}}
	type site struct {
		s    *Node
		take *Node
	}
	var sites []site
	for _, f := range s6pcDefns(forms) {
		for _, it := range Body(f) {
			Walk(it, func(s *Node) bool {
				if !s.Is("if") || len(s.Kids) < 3 {
					return true
				}
				v, t, ok := x.xconst(s.Kids[1])
				if !ok || !t.integer() {
					return true
				}
				if hasLabel(s.Kids[2]) || len(s.Kids) > 3 && hasLabel(s.Kids[3]) {
					return true
				}
				var take *Node
				switch {
				case !v.zero():
					take = s.Kids[2]
				case len(s.Kids) > 3:
					take = s.Kids[3]
				}
				sites = append(sites, site{s, take})
				return true
			})
		}
	}
	for i := len(sites) - 1; i >= 0; i-- {
		s := sites[i]
		with := s.take
		if with == nil {
			ns, err := e.Build(s.s, "(block)", nil)
			if err != nil {
				return 0, fmt.Errorf("constbranch: %w", err)
			}
			with = ns[0]
		} else if !with.Is("block") {
			return 0, fmt.Errorf("constbranch: %s takes a branch that is not a block", label(s.s))
		}
		if err := e.Replace(s.s, with); err != nil {
			return 0, fmt.Errorf("constbranch: %w", err)
		}
	}
	return len(sites), nil
}
