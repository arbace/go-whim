package p076

// Whim phase 76 (formerly 152) -- the option variables are typed.  See GOAL.md.
//
// vimoption_T.var, optset_T.os_varp, get_varp() and every varp held the address
// of an int, a long or a char_u * as a char_u * (internal/gen/FINDINGS.md, 2).  They are
// an optvar_T, a pointer of each kind and a window-local flag, and a
// window-local option's global value is get_varp_allbuf(), not a byte offset.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim76", Edit) }

// W76Type is optvar_T and its helpers, exported so the check requires the
// identical text.
const W76Type = `typedef struct
{
    int         *ov_int;
    long        *ov_long;
    char_u      **ov_str;
    int         ov_win;
} optvar_T;

    static optvar_T
optvar_int(int *p)
{
    optvar_T    v = {p, nullptr, nullptr, 0};

    return v;
}

    static optvar_T
optvar_long(long *p)
{
    optvar_T    v = {nullptr, p, nullptr, 0};

    return v;
}

    static optvar_T
optvar_str(char_u **p)
{
    optvar_T    v = {nullptr, nullptr, p, 0};

    return v;
}

    static optvar_T
optvar_none(void)
{
    optvar_T    v = {nullptr, nullptr, nullptr, 0};

    return v;
}

    static int
optvar_is_null(optvar_T v)
{
    return v.ov_int == nullptr && v.ov_long == nullptr && v.ov_str == nullptr && !v.ov_win;
}

`

// W76Allbuf is get_varp_allbuf(): the global value of a window-local option
// with no global variable, the field of w_allbuf_opt that phase 76a's
// `(char *)get_varp(p) + sizeof(winopt_T)` reached.
const W76Allbuf = `    static optvar_T
get_varp_allbuf(struct vimoption *p)
{
    switch ((int)p->indir)
    {
%s    }
    return optvar_none();
}

`

// Edit types the option variables.
//
// vimoption_T.var, optset_T.os_varp, get_varp() and every `varp` held the
// address of an option's variable -- an int, a long or a char_u * -- as a
// char_u *, cast back at every read: `*(int *)varp`.  A window-local option
// with no global variable held (char_u *)-1, and its global value was reached
// as `(char *)get_varp(p) + sizeof(winopt_T)`, the same field one winopt_T
// further on.  The Go transpilation held them as `any` (internal/gen/FINDINGS.md, 2).
// Now they are an optvar_T: a pointer of each kind, one set, and a flag for
// the window-local sentinel.  Each read names its kind (`*varp.ov_int`), the
// table's rows are typed by the row's P_BOOL, P_NUM or P_STRING, get_varp()'s
// returns by the declared type of the field they name, and the window-local
// global value is get_varp_allbuf(), the w_allbuf_opt field -- in
// get_varp_scope() and in set_string_option_global(), whose two callers hand
// it curwin's w_onebuf_opt field, the one the byte offset stepped from.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, Step6 as built): the rows' variables
// made braced lists in place, the address moved in; optvar_T, its helpers and
// get_varp_allbuf() written by FRAG; the members, the results and every varp
// declaration retyped (RETYPE), first, so that every rewrite after them is
// made in the types it leaves; then every site the text's rules and literals
// took -- found by form, a variable by its edge to vimoption_T.var or
// optset_T.os_varp, a field's kind by its member's typed edge, a global's by
// its declaration's -- rewritten in ONE FRAG unit, which types what it makes
// and what is above it as the import does.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("optvar", e, w)
	o := &opt{v: v, e: e, site: map[*graph.Node]string{}}
	var says []string
	say := func(s string) { says = append(says, s) }

	// the declarations the rewrites name
	vimopt := o.one("(struct vimoption (fullname _) _*)", "struct vimoption")
	optset := o.one("(typedef optset_T _)", "optset_T")
	if v.Failed() {
		return v.Done()
	}
	o.varM = member(vimopt, "var")
	o.osM = member(optset.Args()[1], "os_varp")
	if o.varM == nil || o.osM == nil {
		v.Die("vimoption_T.var or optset_T.os_varp is not where this phase expects it")
		return v.Done()
	}

	// 1. the table's rows
	def := o.one("(def static options _ (init _*))", "options[]")
	if v.Failed() {
		v.Die("options[] is not where this phase expects it")
		return v.Done()
	}
	rows := graph.TableInit(def).Args()
	for _, r := range rows {
		els := r.Args()
		if !r.Is("init") || len(els) < 4 {
			v.Die("a row of options[] has %d fields", len(els))
			return v.Done()
		}
		flags, err1 := graph.ExprText(els[2])
		val, err2 := graph.ExprText(els[3])
		if err1 != nil || err2 != nil {
			v.Die("a row of options[] does not print")
			return v.Done()
		}
		vn := strings.Join(strings.Fields(val), "")
		n := func(s string) *graph.Node { return graph.NewAtom(s) }
		var nv *graph.Node
		switch {
		case vn == "nullptr" || vn == "(char_u*)nullptr":
			nv = graph.NewList(n("init"), n("nullptr"), n("nullptr"), n("nullptr"), n("0"))
		case vn == "((char_u*)-1)" || vn == "(char_u*)((char_u*)-1)":
			nv = graph.NewList(n("init"), n("nullptr"), n("nullptr"), n("nullptr"), n("1"))
		case els[3].Is("cast") && isPtrTo(els[3].Args()[0], "char_u") && els[3].Args()[1].Is("addr"):
			addr := els[3].Args()[1]
			switch {
			case strings.Contains(flags, "P_BOOL"):
				nv = graph.NewList(n("init"), addr, n("nullptr"), n("nullptr"), n("0"))
			case strings.Contains(flags, "P_NUM"):
				nv = graph.NewList(n("init"), n("nullptr"), addr, n("nullptr"), n("0"))
			case strings.Contains(flags, "P_STRING"):
				nv = graph.NewList(n("init"), n("nullptr"), n("nullptr"), addr, n("0"))
			}
		}
		if nv == nil {
			v.Die("a row's variable %q with flags %q has no kind", val, strings.TrimSpace(flags))
			return v.Done()
		}
		if err := e.ReplaceElement(els[3], nv); err != nil {
			v.Die("a row's variable -- %v", err)
			return v.Done()
		}
	}
	say(fmt.Sprintf("the %d rows of options[] name their variable by kind", len(rows)))

	// 2. the type and its helpers, and the window-local global value, by FRAG
	getVarp := e.Defn("get_varp")
	scopeDef := e.Defn("get_varp_scope")
	if getVarp == nil || scopeDef == nil {
		v.Die("get_varp() or get_varp_scope() is not defined")
		return v.Done()
	}
	var cases strings.Builder
	nwin, k := 0, 0
	for _, c := range v.Find("(case (cast idopt_T (+ PV_WIN (cast int (paren _)))))") {
		nwin++
		wv := c.Args()[0].Args()[1].Args()[1].Args()[1].Args()[0]
		ret := next(e, c)
		if ret == nil || !ret.Is("return") || len(ret.Args()) != 1 || !topSwitch(e, c) {
			continue
		}
		x := ret.Args()[0]
		if !x.Is("cast") || !isPtrTo(x.Args()[0], "char_u") || !x.Args()[1].Is("addr") {
			continue
		}
		f := x.Args()[1].Args()[0]
		if !f.Is("paren") || !f.Args()[0].Is(".") || len(f.Args()[0].Args()) != 2 {
			continue
		}
		sel := f.Args()[0]
		if b, _ := graph.ExprText(sel.Args()[0]); b != "curwin->w_onebuf_opt" {
			continue
		}
		kind := kindOf(sel.Args()[1].Ref())
		if kind == "" {
			continue
		}
		k++
		fmt.Fprintf(&cases, "    case (idopt_T)(PV_WIN + (int)(%s)):\n        return %s(&(curwin->w_allbuf_opt.%s));\n", wv.Atom, kind, sel.Args()[1].Atom)
	}
	// EVERY WINDOW-LOCAL CASE, AND THAT IS ASSERTED: get_varp_allbuf() is
	// written from what is read, so a case it misses is an option whose
	// global value silently becomes none.
	if k != nwin || k == 0 {
		v.Die("get_varp() has %d window-local cases and this phase read %d of them -- "+
			"get_varp_allbuf() is written from what is read, so a case it misses is an "+
			"option whose global value silently becomes none", nwin, k)
		return v.Done()
	}
	if _, err := e.SpliceC(
		graph.Frag{At: e.SpotBefore(optset), Src: W76Type},
		graph.Frag{At: e.SpotBefore(scopeDef), Src: fmt.Sprintf(W76Allbuf, cases.String())}); err != nil {
		v.Die("optvar_T and get_varp_allbuf() -- %v", err)
		return v.Done()
	}

	// 3. the declarations, retyped before anything is written in their types
	retype := func(d *graph.Node, t, what string) {
		if v.Failed() {
			return
		}
		if _, err := e.Retype(d, t); err != nil {
			v.Die("%s -- %v", what, err)
		}
	}
	retype(o.varM, "optvar_T", "vimoption_T.var")
	retype(o.osM, "optvar_T", "optset_T.os_varp")
	say("vimoption_T.var is an optvar_T")
	say("and so is optset_T.os_varp, after the type and its helpers")
	for _, f := range []string{"get_varp_scope", "get_option_varp_scope", "get_varp"} {
		if !v.Failed() && o.result(f, "(ptr char_u)") {
			if _, err := e.RetypeResult(f, "optvar_T"); err != nil {
				v.Die("%s() -- %v", f, err)
			}
		}
	}
	say("get_varp() and get_varp_scope() return one")
	if v.Failed() {
		return v.Done()
	}
	// every varp declared char_u *: a local, or a parameter in each
	// declaration of its function (counted as the text counted them)
	ndecl := 0
	done := map[string]bool{}
	var todo []*graph.Node
	for _, f := range e.Graph().Forms {
		if !f.Is("def") && !f.Is("defn") {
			continue
		}
		graph.Walk(f, func(x *graph.Node) bool {
			if x.Is("def") && x != f {
				if len(x.Kids) >= 3 && (x.Kids[1].Atom == "varp" || x.Kids[1].Atom == "varp_arg") && isPtrTo(x.Kids[2], "char_u") {
					ndecl++
					todo = append(todo, x)
				}
				return true
			}
			if !x.IsList() || len(x.Kids) < 2 || x.Kids[0].IsList() || x.Kids[0].Atom != "varp" && x.Kids[0].Atom != "varp_arg" {
				return true
			}
			if !isPtrTo(x.Kids[1], "char_u") || !isParam(e, x) {
				return true
			}
			ndecl++
			key := graph.DeclName(f) + "/" + x.Kids[0].Atom
			if !done[key] {
				done[key] = true
				todo = append(todo, x)
			}
			return true
		})
	}
	for _, x := range todo {
		retype(x, "optvar_T", "a varp declaration")
	}
	if v.Failed() {
		return v.Done()
	}

	// 4. the rewrites, found on the graph the retypes left, made in one unit
	goV := e.Defn("get_option_var")
	if goV == nil || !o.result("get_option_var", "(ptr char_u)") {
		v.Die("get_option_var() is not where this phase expects it")
		return v.Done()
	}
	if _, err := e.RetypeResult("get_option_var", "(ptr (ptr char_u))"); err != nil {
		v.Die("get_option_var() -- %v", err)
		return v.Done()
	}
	o.in(goV, "(return (. (index options opt_idx) var))", 1, "get_option_var()'s return", func(r *graph.Node) {
		o.put(r.Args()[0], "options[opt_idx].var.ov_str")
	})
	o.in(nil, "(cast (ptr (ptr char_u)) (call get_option_var opt_idx))", 1, "get_option_var()'s caller", func(c *graph.Node) {
		o.put(c, "get_option_var(opt_idx)")
	})
	say("get_option_var() returns the string variable its one caller wants")

	// get_varp()'s and get_varp_scope()'s returns, typed by the field
	var bad []string
	nAddr := 0
	for _, c := range v.Find("(cast (ptr char_u) (addr (paren _)))") {
		x := c.Args()[1].Args()[0].Args()[0]
		var m *graph.Node
		switch {
		case x.Is(".") && len(x.Args()) == 2 && text(x.Args()[0]) == "curwin->w_onebuf_opt":
			m = x.Args()[1]
		case x.Is("->") && len(x.Args()) == 2 && text(x.Args()[0]) == "curbuf":
			m = x.Args()[1]
		default:
			continue
		}
		kind := kindOf(m.Ref())
		if kind == "" {
			bad = append(bad, m.Atom)
			continue
		}
		nAddr++
		o.put(c, kind+"(&("+text(x)+"))")
	}
	if len(bad) > 0 {
		v.Die("fields of no known type: %v", bad)
		return v.Done()
	}
	say(fmt.Sprintf("%d addresses of an option's field are typed by the field", nAddr))

	// the window-local global value
	o.in(scopeDef, "(if (== (-> p var) (paren (cast (ptr char_u) (- 1)))) (block (return (cast (ptr char_u) (+ (cast (ptr char) (paren (call get_varp p))) (sizeof-type winopt_T))))))", 1,
		"a window-local option's global value is get_varp_allbuf()", func(f *graph.Node) {
			o.put(f.Args()[0], "p->var.ov_win")
			o.put(f.Args()[1].Args()[0].Args()[0], "get_varp_allbuf(p)")
		})
	say("a window-local option's global value is get_varp_allbuf()")
	o.in(nil, "(return (== (. (index options opt_idx) var) (paren (cast (ptr char_u) (- 1)))))", 1, "and the sentinel is the flag", func(r *graph.Node) {
		o.put(r.Args()[0], "options[opt_idx].var.ov_win")
	})
	say("and the sentinel is the flag")

	// the two functions that reused varp for the string it held
	o.in(e.Defn("get_term_code"), "(if (!= varp nullptr) (block (= varp (deref (cast (ptr (ptr char_u)) (paren varp))))))", 1,
		"get_term_code() returns the string without reusing varp for it", func(f *graph.Node) {
			ret := next(e, f)
			if ret == nil || !ret.Is("return") || len(ret.Args()) != 1 || !isAtom(ret.Args()[0], "varp") {
				v.Die("get_term_code() returns the string without reusing varp for it -- no `return varp;` after the test")
				return
			}
			o.put(f.Args()[0], "!optvar_is_null(varp)")
			o.put(f.Args()[1].Args()[0], "return *varp.ov_str;")
			o.put(ret.Args()[0], "nullptr")
		})
	say("get_term_code() returns the string without reusing varp for it")
	o.in(e.Defn("option_value2string"), "(= varp (deref (cast (ptr (ptr char_u)) (paren varp))))", 1, "nor does option_value2string()", func(a *graph.Node) {
		f := next(e, a)
		if f == nil || lisp(f) != "(if (== varp nullptr) (block (= (index NameBuff 0) NUL)) (if (& (-> opp flags) P_EXPAND) (block (call home_replace nullptr varp NameBuff PATH_MAX FALSE)) (if (== (cast (ptr (ptr char_u)) (-> opp var)) (addr p_pt)) (block (call str2specialbuf p_pt NameBuff PATH_MAX)) (block (call vim_strncpy NameBuff varp (- PATH_MAX 1))))))" {
			v.Die("nor does option_value2string() -- the test after `varp = *(char_u **)(varp);` is not the one this phase expects")
			return
		}
		o.run(a, f, "        char_u      *s = *varp.ov_str;\n\n        if (s == nullptr)\n        {\n            NameBuff[0] = NUL;\n        }\n        else if (opp->flags & P_EXPAND)\n        {\n            home_replace(nullptr, s, NameBuff,  PATH_MAX , FALSE);\n        }\n        else if (opp->var.ov_str == &p_pt)\n        {\n            str2specialbuf(p_pt, NameBuff,  PATH_MAX );\n        }\n        else\n        {\n            vim_strncpy(NameBuff, s,  PATH_MAX  - 1);\n        }\n")
	})
	say("nor does option_value2string()")

	// the callers of get_option_varp_scope() and set_string_option_global()
	o.in(nil, "(def p (ptr char_u) (call get_option_varp_scope opt_idx OPT_LOCAL))", 1, "the callers of get_option_varp_scope() take the string variable", func(d *graph.Node) {
		f := next(e, d)
		var g *graph.Node
		if f != nil {
			g = next(e, f)
		}
		if f == nil || g == nil || lisp(f) != "(call free_string_option (deref (cast (ptr (ptr char_u)) p)))" || lisp(g) != "(= (deref (cast (ptr (ptr char_u)) p)) empty_option)" {
			v.Die("the callers of get_option_varp_scope() take the string variable -- the statements after `p` are not the two this phase expects")
			return
		}
		o.run(d, g, "            char_u **p = get_option_varp_scope(opt_idx, OPT_LOCAL).ov_str;\n            free_string_option(*p);\n            *p = empty_option;\n")
	})
	o.in(e.Defn("set_string_option_global"), "(= p (cast (ptr (ptr char_u)) (+ (cast (ptr char) (paren varp)) (sizeof-type winopt_T))))", 1,
		"set_string_option_global() finds a window-local option's global value by name, not by byte offset", func(a *graph.Node) {
			o.put(a.Args()[1], "get_varp_allbuf(&(options[opt_idx])).ov_str")
		})

	// 5. the rest, by rule: reads, casts, tests
	nDeref, nValue, nNull, nWin := 0, 0, 0, 0
	for _, d := range v.Find("(deref (cast _ _))") {
		c := d.Args()[0]
		kind := ptrKind(c.Args()[0], true)
		x := c.Args()[1]
		if x.Is("paren") {
			x = x.Args()[0]
		}
		if kind == "" || !o.isVar(x, true) || o.taken(d) {
			continue
		}
		nDeref++
		if kind == "char" {
			o.put(d, "(char *)*"+text(x)+".ov_str")
		} else {
			o.put(d, "*"+text(x)+".ov_"+kind)
		}
	}
	for _, c := range v.Find("(cast _ _)") {
		kind := ptrKind(c.Args()[0], false)
		x := c.Args()[1]
		if kind == "" || !o.isValue(x) || o.taken(c) {
			continue
		}
		nValue++
		o.put(c, text(x)+".ov_"+kind)
	}
	gs := 0
	for _, c := range v.Find("(cast (ptr (ptr char_u)) (call get_option_varp_scope _*))") {
		if o.taken(c) {
			continue
		}
		gs++
		o.put(c, text(c.Args()[1])+".ov_str")
	}
	for _, t := range append(v.Find("(== _ _)"), v.Find("(!= _ _)")...) {
		x, y := t.Args()[0], t.Args()[1]
		if o.taken(t) {
			continue
		}
		neg := t.Is("!=")
		switch {
		case y.Atom == "nullptr" && !y.IsList() && o.isNullable(x):
			nNull++
			if neg {
				o.put(t, "!optvar_is_null("+text(x)+")")
			} else {
				o.put(t, "optvar_is_null("+text(x)+")")
			}
		case text(y) == "((char_u *)-1)" && (o.isOptionsVar(x) || o.isVarOf(x, "p")):
			nWin++
			if neg {
				o.put(t, "!"+text(x)+".ov_win")
			} else {
				o.put(t, text(x)+".ov_win")
			}
		case !neg && isAtom(x, "varp") && y.Is("cast") && isPtrTo(y.Args()[0], "char_u") && y.Args()[1].Is("addr") && !y.Args()[1].Args()[0].IsList():
			name := y.Args()[1].Args()[0]
			kind := kindOf(name.Ref())
			o.put(t, "varp."+strings.Replace(kind, "optvar_", "ov_", 1)+" == &"+name.Atom)
		case !neg && o.isOptionsVar(x) && text(y) == "(char_u *)p":
			o.put(t, text(x)+".ov_str == p")
		case !neg && o.isVarOf(x, "p") && isAtom(y, "var"):
			o.put(t, "(char_u *)p->var.ov_str == var")
			say2 := "free_one_termoption() compares as it did"
			o.late = append(o.late, say2)
		}
	}
	if gs != 2 {
		v.Die("%d string reads through get_option_varp_scope(), and this phase was written against 2", gs)
		return v.Done()
	}
	say(fmt.Sprintf("%d reads of an option's variable name its kind", nDeref))
	say(fmt.Sprintf("%d casts of it to a typed pointer name the pointer", nValue))
	say(fmt.Sprintf("%d tests for no variable ask optvar_is_null()", nNull))
	say("the callers of get_option_varp_scope() take the string variable")
	say("set_string_option_global() finds a window-local option's global value by name, not by byte offset")
	say(fmt.Sprintf("%d tests for the window-local sentinel read the flag", nWin))

	// what was nullptr is none
	o.in(scopeDef, "(return nullptr)", 1, "get_varp_scope()'s none", func(r *graph.Node) { o.put(r.Args()[0], "optvar_none()") })
	o.in(getVarp, "(return nullptr)", 1, "get_varp()'s none", func(r *graph.Node) { o.put(r.Args()[0], "optvar_none()") })
	o.in(nil, "(= varp nullptr)", 2, "varp's none", func(a *graph.Node) { o.put(a.Args()[1], "optvar_none()") })
	o.in(nil, "(= (. args os_varp) (cast (ptr char_u) varp))", 1, "os_varp's string", func(a *graph.Node) { o.put(a.Args()[1], "optvar_str(varp)") })
	if len(o.late) != 1 {
		v.Die("free_one_termoption() compares as it did -- %d comparisons, expected 1", len(o.late))
		return v.Done()
	}
	say(o.late[0])
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.SpliceC(o.frags...); err != nil {
		v.Die("the rewrites -- %v", err)
		return v.Done()
	}
	say(fmt.Sprintf("%d varp declarations are optvar_T", ndecl))
	for _, s := range says {
		v.Say(s)
	}
	return v.Done()
}

// opt is the program's state: the members a variable is read through, and
// the sites written, each once.
type opt struct {
	v         *graph.Verbs
	e         *graph.Editor
	varM, osM *graph.Node
	site      map[*graph.Node]string
	frags     []graph.Frag
	runs      [][2]*graph.Node
	late      []string
}

func (o *opt) one(pat, what string) *graph.Node { return o.v.One(pat, what) }

// put writes the node n as the C src, unless a site holds it already.
func (o *opt) put(n *graph.Node, src string) {
	if o.taken(n) {
		o.v.Die("%s is written twice", text(n))
		return
	}
	o.site[n] = src
	o.frags = append(o.frags, graph.Frag{At: o.e.SpotOf(n), Src: src})
}

// run writes the run of items first..last as the C src.
func (o *opt) run(first, last *graph.Node, src string) {
	for x := first; ; x = next(o.e, x) {
		o.site[x] = ""
		if x == last {
			break
		}
	}
	o.runs = append(o.runs, [2]*graph.Node{first, last})
	o.frags = append(o.frags, graph.Frag{At: o.e.SpotRun(first, last), Src: src})
}

// taken says n is a site already, or inside one.
func (o *opt) taken(n *graph.Node) bool {
	for x := n; x != nil; x = o.e.Parent(x) {
		if _, ok := o.site[x]; ok {
			return true
		}
	}
	return false
}

// in calls f on each of the n matches of pat in fn (nil: the file), and
// refuses another count.
func (o *opt) in(fn *graph.Node, pat string, n int, what string, f func(*graph.Node)) {
	if o.v.Failed() {
		return
	}
	var ms []*graph.Node
	if fn == nil {
		ms = o.v.Find(pat)
	} else {
		o.v.In(fn, func(v *graph.Verbs) { ms = v.Find(pat) })
	}
	if len(ms) != n {
		o.v.Die("%s -- occurs %d times, expected %d", what, len(ms), n)
		return
	}
	for _, m := range ms {
		if !o.v.Failed() {
			f(m)
		}
	}
}

// result says the function's every declaration returns the type form t.
func (o *opt) result(fn, t string) bool {
	ds := o.e.FileDecls(fn)
	for _, d := range ds {
		ft := graph.DeclType(d)
		if ft == nil || !ft.Is("fn") || len(ft.Kids) < 3 || graph.Lisp(ft.Kids[2]).String() != t {
			o.v.Die("%s() does not return %s", fn, t)
			return false
		}
	}
	return len(ds) > 0
}

// isVarOf says x is `B->var`, B the atom b, var vimoption_T's.
func (o *opt) isVarOf(x *graph.Node, b string) bool {
	return x.Is("->") && len(x.Args()) == 2 && isAtom(x.Args()[0], b) && x.Args()[1].Ref() == o.varM
}

// isOptionsVar says x is `options[K].var`.
func (o *opt) isOptionsVar(x *graph.Node) bool {
	if !x.Is(".") || len(x.Args()) != 2 || x.Args()[1].Ref() != o.varM {
		return false
	}
	b := x.Args()[0]
	return b.Is("index") && len(b.Args()) == 2 && isAtom(b.Args()[0], "options")
}

// isGetVarp says x is `get_varp(&options[K])` or `get_varp_scope(&options[K], S)`.
func isGetVarp(x *graph.Node) bool {
	if !x.Is("call") || len(x.Args()) < 2 {
		return false
	}
	f := x.Args()[0]
	if !(isAtom(f, "get_varp") && len(x.Args()) == 2 || isAtom(f, "get_varp_scope") && len(x.Args()) == 3) {
		return false
	}
	a := x.Args()[1]
	if !a.Is("addr") {
		return false
	}
	b := a.Args()[0]
	if b.Is("paren") {
		b = b.Args()[0]
	}
	return b.Is("index") && len(b.Args()) == 2 && isAtom(b.Args()[0], "options")
}

// isVar says x is what the text's reads named: varp, p->var, opp->var,
// options[K].var, or get_varp()'s or get_varp_scope()'s value.
func (o *opt) isVar(x *graph.Node, _ bool) bool {
	return isAtom(x, "varp") || o.isVarOf(x, "p") || o.isVarOf(x, "opp") || o.isOptionsVar(x) || isGetVarp(x)
}

// isValue says x is what the text's casts named: varp, opp->var,
// args->os_varp, or get_varp()'s or get_varp_scope()'s value.
func (o *opt) isValue(x *graph.Node) bool {
	if x.Is("->") && len(x.Args()) == 2 && isAtom(x.Args()[0], "args") && x.Args()[1].Ref() == o.osM {
		return true
	}
	return isAtom(x, "varp") || o.isVarOf(x, "opp") || isGetVarp(x)
}

// isNullable says x is what the text's null tests named.
func (o *opt) isNullable(x *graph.Node) bool {
	return isAtom(x, "varp") || o.isVarOf(x, "p") || o.isVarOf(x, "opp") || o.isOptionsVar(x)
}

// ptrKind is the kind a pointer cast's type form reads: int, long, str
// (char_u **), and, for a read, char (char **).
func ptrKind(t *graph.Node, read bool) string {
	switch {
	case isPtrTo(t, "int"):
		return "int"
	case isPtrTo(t, "long"):
		return "long"
	case t.Is("ptr") && len(t.Args()) == 1 && isPtrTo(t.Args()[0], "char_u"):
		return "str"
	case read && t.Is("ptr") && len(t.Args()) == 1 && isPtrTo(t.Args()[0], "char"):
		return "char"
	}
	return ""
}

// kindOf is the optvar_T constructor for what d's declared type is: an
// int, a long or a char_u *.
func kindOf(d *graph.Node) string {
	if d == nil || d.Type == nil {
		return ""
	}
	t := d.Type
	switch {
	case basic(t, "int"):
		return "optvar_int"
	case basic(t, "long"):
		return "optvar_long"
	case t.Is("pointer") && len(t.Kids) == 2 && basic(t.Kids[1].Type, "unsigned", "char"):
		return "optvar_str"
	}
	return ""
}

func basic(t *graph.Node, words ...string) bool {
	if !t.Is("basic") || len(t.Kids) != len(words)+1 {
		return false
	}
	for i, w := range words {
		if t.Kids[i+1].Atom != w {
			return false
		}
	}
	return true
}

// member is the member named name of a struct definition.
func member(def *graph.Node, name string) *graph.Node {
	for _, m := range def.Args() {
		if m.IsList() && len(m.Kids) >= 2 && !m.Kids[0].IsList() && m.Kids[0].Atom == name {
			return m
		}
	}
	return nil
}

// isParam says x, `(NAME TYPE)`, is a parameter of a function's own type.
func isParam(e *graph.Editor, x *graph.Node) bool {
	l := e.Parent(x)
	if l == nil {
		return false
	}
	fn := e.Parent(l)
	if fn == nil || !fn.Is("fn") || len(fn.Kids) < 2 || fn.Kids[1] != l {
		return false
	}
	d := e.Parent(fn)
	return d != nil && (d.Is("def") || d.Is("defn")) && graph.DeclType(d) == fn
}

// next is the item after x in its list, or nil.
func next(e *graph.Editor, x *graph.Node) *graph.Node {
	p := e.Parent(x)
	if p == nil {
		return nil
	}
	for i, k := range p.Kids {
		if k == x && i+1 < len(p.Kids) {
			return p.Kids[i+1]
		}
	}
	return nil
}

// topSwitch says the case c is a label of a switch that is an item of a
// function's own body.
func topSwitch(e *graph.Editor, c *graph.Node) bool {
	b := e.Parent(c)
	s := e.Parent(b)
	if b == nil || !b.Is("block") || s == nil || !s.Is("switch") {
		return false
	}
	f := e.Parent(s)
	return f != nil && f.Is("defn")
}

func isPtrTo(t *graph.Node, name string) bool {
	return t.Is("ptr") && len(t.Args()) == 1 && isAtom(t.Args()[0], name)
}

func isAtom(x *graph.Node, s string) bool { return x != nil && !x.IsList() && x.Atom == s }

func text(n *graph.Node) string {
	s, err := graph.ExprText(n)
	if err != nil {
		return "\x00"
	}
	return s
}

func lisp(n *graph.Node) string { return graph.Lisp(n).String() }
