package p051

// Whim phase 51 (formerly 122) -- `-T {term}` goes, and the command line is `+{command}` alone.
// See GOAL.md.
//
// Record 88 left argv as exactly two options: `+{command}`, which is how a host
// tells the editor what to do, and `-T {term}`, which is how a SHELL told it what it
// was attached to.  A core is told that by its host or not at all -- and `-T` has
// had a replacement inside the editor since before this pipeline began: `+set term=`
// reaches did_set_term() and does everything `-T` did, which is why record 116
// rebuilt the terminal harness on it.  So this removes the option, and after it
// `+{command}` is the whole command line: every other word is what every unknown
// word already was, `mainerr(ME_UNKNOWN_OPTION)`.
//
// WHAT THE OPTION LETTER PULLS WITH IT, and every one of these is dead code a sweep
// CANNOT see -- gcc has no warning for a variable that is only ever FALSE, for a
// switch that has lost its cases, or for a statement after a `return`:
//
// want_argument   the flag `-T` was the only setter of.  With no case left to set
// it the whole `if (want_argument)` block is unreachable, and that
// block is where ME_GARBAGE, mainerr_arg_missing() and the second
// switch -- `parmp->term = argv[0]` -- live.
// the two rows    ME_GARBAGE and ME_ARG_MISSING lose their last use, and
// main_errors[] is INDEXED BY THEM, so the enumerator and the row
// are one thing.  deadenums.py would take the enumerator and leave
// the row, and the rows are positional; this is CLAUDE.md's
// deadfields lesson in a table, so the edit takes both and
// renumbers what is left.  mainerr_arg_missing() goes with them
// because it is ME_ARG_MISSING's only other mention: the
// enumerator cannot go while its one reader is still there.
// mparm_T.term    the field nothing assigns now.  It goes in the EDIT for a
// reason phase 51a's dead clause did not have: deadfields.py
// matches by NAME, and this file holds thirty-two mentions of
// another struct's `.term` member (attr_entry's `ae_u.term`), so
// that tool can never see this one dead.  The edit computes that
// partition rather than asserting it.
// termcapinit()   it can only ever be handed what the memset left, so it takes no
// name at all now and the compiled default -- read out of the
// function, not written here -- is its initialiser.  That is
// internal/phase/035/edit.go's ui_write(console) again.
//
// AND THEN set_termname()'s NO-SCREEN ARM CANNOT RUN.  set_termname() has two call
// sites, and the edit partitions them: termcapinit()'s, which reaches it before
// there is a screen, and did_set_term()'s, which is `:set term=` at run time.  The
// first can no longer fail -- the compiled default IS a row of builtin_terminals[],
// which the edit checks -- so when find_builtin_term() answers nullptr the call came
// from the second, where `starting` is NO_BUFFERS or 0 and never NO_SCREEN (the edit
// reads every assignment to `starting` and requires none of them to be NO_SCREEN).
// So `if (starting != NO_SCREEN)` is always true there, the block returns FAIL, and
// the three statements after it are unreachable: the fallback that phase 51a had to
// repair, report_default_term(), and the option write that recorded it.
// on both texts -- the input enters the fallback in exactly the two `-T` records and
// a control built from THIS phase's output enters it in none.
//
// THE MESSAGE GOES WITH THE FALLBACK, for phase 51a's reason read backwards.  That
// phase retargeted `' not known, defaulting to 'xterm''` onto the name it kept
// because nothing in the build checks that a message tells the truth.  There is no
// fallback to name now, so the clause that promised one is cut -- the NAME is read
// out of the assignment this edit deletes, never written here -- and what is left is
// `'vt320' not known`, which is true, with E522 following it exactly as before.
//
// WHAT RIDES ALONG IS `requested` AND NOT THE 256-COLOUR TEST, and the difference is
// the whole of what was measured.  set_termname() keeps the name it was GIVEN in
// `requested` for one test, `musl_strstr(requested, "256color")`, and CLAUDE.md says
// why: the unknown-terminal path reassigned `term`, so testing that would have given
// `alacritty-256color` eight colours.  That path is what this phase removes, so the
// only rewrite of `term` left is the `term += 8` that strips a `builtin_` prefix --
// and a strstr cannot match inside that prefix, because the needle begins with a
// character the prefix does not contain, which the edit checks rather than asserts.
// So `requested` IS `term` for this test and the variable goes.
//
// THE TEST ITSELF DOES NOT FOLD, and this phase declines to pretend it does.  Both
// surviving terminal names disagree on it -- `xterm-256color` matches and `debug`
// does not -- and `:set term=` still names either at run time.  MEASURED, in both
// directions, and the check keeps the measurement: with the name test forced TRUE
// exactly ONE of the nineteen rows moves (`debug` gains t_Co=256) and with it forced
// FALSE EIGHTEEN move (everything the compiled default reaches drops to t_Co=8).
// A fold either way would be a behaviour change, so there is none here.
//
// THE INPUT BINARY AND ITS ENUMERATOR VALUES ARE TAKEN HERE, before the edit, as
// used to do, and the DWARF dump because main_errors[] is indexed by enumerators
// this phase renumbers and a build is perfectly happy to renumber a table index.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// The enumerator values of the text this phase is HANDED: main_errors[] is indexed
// by them and this phase moves one, so the check compares DWARF and not the build.
// NOT create_cmdidxs --check, for internal/phase/004/e/edit.go's reason: the derived
// first-two-letters index went with the command table whim reduced, and the tool
// raises rather than reporting nothing.  Nothing here touches the command table.
//

// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// edit is the program's graph edited through crefactor/graph, its report the
// text version's, which the plan ran until then (history keeps it):
//
//   - termcapinit()'s empty-name test is a FoldNever; the `given none` if's
//     one assignment is read, its value (a copy) becomes `term`'s
//     initialiser and the if goes; the parameter goes by PARAM
//     (DropParam), with the argument `params.term` at the one call;
//   - mparm_T's member goes once no use of it is left, by edge; the other
//     structs' `.term` are still counted on the C view, the text's question;
//   - set_termname()'s callers are the uses of its declarations, by the
//     function each is in; the assignments to `starting` are the stores to
//     it, by edge;
//   - the no-screen test is a FoldAlways inside the `termp == nullptr` arm,
//     and the items after its `return FAIL;` are deleted, their C (as the
//     C view prints them) what the report quotes;
//   - report_term_error()'s promise is each literal respelled whole (the
//     text's regexp applied to the literal alone, which is where it
//     matched);
//   - `requested`'s one use, in the 256-colour test, is pointed at the
//     parameter `term` (a new use with its edge); its declaration is the
//     collection's.

import (
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim51", Edit) }

var (
	w51Owner     = regexp.MustCompile(`(\w+)\s*(?:\.|->)\s*term\b`)
	w51DotTerm   = regexp.MustCompile(`(?:\.|->)\s*term\b`)
	w51FirstLit  = regexp.MustCompile(`"([^"]*)"`)
	w51Requested = regexp.MustCompile(`\brequested\b`)
	w51TermRewr  = regexp.MustCompile(`(?m)^[ \t]*term (\+?=[^;]*);$`)
	w51Prefix    = regexp.MustCompile(`musl_strncmp\(\(char \*\)\(name\), \(char \*\)\("([^"]*)"\), \(\(usize\)(\d+)\)\)`)
	w51Needle    = regexp.MustCompile(`musl_strstr\(\(char \*\)requested, "([^"]*)"\)`)
	// the one declaration `requested` may keep (the collection takes it)
	w51DeclOnly = map[string]*regexp.Regexp{
		"requested": regexp.MustCompile(`\bchar_u +\*requested = term;`),
	}
)

func w51Mentions(text []byte, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).
		FindAll(edit.Blank(edit.WithoutIncludes(text)), -1))
}

// w51ItemsC is items' C as the C view prints them in a function's body.
func w51ItemsC(items []*graph.Node) (string, error) {
	f := clisp.L(clisp.A("defn"), clisp.A("f"), clisp.L(clisp.A("fn"), clisp.L(clisp.A("void")), clisp.A("void")))
	for _, n := range items {
		f.List = append(f.List, graph.Lisp(n))
	}
	out, err := clisp.Print([]*clisp.Node{f})
	if err != nil {
		return "", err
	}
	s := string(out)
	a, z := strings.Index(s, "{\n"), strings.LastIndex(s, "}")
	if a < 0 || z < a {
		return "", nil
	}
	return s[a+2 : z], nil
}

func w51Content(s *graph.Node) string {
	if a := s.Atom; len(a) >= 2 && a[0] == '"' && a[len(a)-1] == '"' {
		return a[1 : len(a)-1]
	}
	return ""
}

// Edit is phase 51 on the graph: `-T` goes -- termcapinit() takes no name,
// mparm_T no `term`, and set_termname()'s no-screen fallback and its promise
// with them.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("cmdline", e, w)
	quiet := func(acts func(q *graph.Verbs)) {
		if v.Failed() {
			return
		}
		q := graph.NewVerbs("cmdline", e, io.Discard)
		acts(q)
		if q.Err != nil {
			v.Err = q.Err
		}
	}
	defn := func(name string) *graph.Node {
		d := e.Defn(name)
		if d == nil {
			v.Die("%s() is not defined in this file, and this phase is drawn against its extent", name)
		}
		return d
	}

	// ---- termcapinit() takes no name
	var dflt string
	tci := defn("termcapinit")
	if v.Failed() {
		return v.Done()
	}
	quiet(func(q *graph.Verbs) {
		q.InFunction("termcapinit", func(q *graph.Verbs) {
			q.FoldNever("(&& (!= term nullptr) (== (deref term) NUL))", 1, "termcapinit()'s empty-name test")
		})
	})
	var givenNone, d, init *graph.Node
	v.InFunction("termcapinit", func(v *graph.Verbs) {
		ifs := v.Find("(if (|| (== term nullptr) (== (deref term) NUL)) _*)")
		if len(ifs) != 1 {
			v.Die("termcapinit() has no `given none` test, so the compiled default cannot " +
				"be read out of it")
			return
		}
		givenNone = ifs[0]
		ds := v.Query("(if _ (block (= term ?d)))", "d")
		if len(ds) != 1 || len(givenNone.Kids) != 3 {
			c, _ := w51ItemsC([]*graph.Node{givenNone})
			v.Die("the compiled default is not one assignment: %s", edit.PyRepr(c))
			return
		}
		d = ds[0]
		inits := v.Query("(def term _ ?v)", "v")
		if len(inits) != 1 || inits[0].Atom != "name" {
			v.Die("termcapinit() does not open with `char_u *term = name;`")
			return
		}
		init = inits[0]
	})
	if v.Failed() {
		return v.Done()
	}
	s, err := graph.ExprText(d)
	if err != nil {
		v.Die("the compiled default's C: %v", err)
		return v.Done()
	}
	dflt = s
	calls := e.Uses(tci)
	if len(calls) != 1 {
		v.Die("termcapinit() is not called with the field this phase just removed")
		return v.Done()
	}
	if c := e.Parent(calls[0]); c == nil || !c.Is("call") || len(c.Kids) != 3 || !c.Kids[2].Is(".") ||
		c.Kids[2].Kids[1].Atom != "params" || c.Kids[2].Kids[2].Atom != "term" {
		v.Die("termcapinit() is not called with the field this phase just removed")
		return v.Done()
	}
	if err := e.Replace(init, graph.Clone(d)); err != nil {
		v.Die("termcapinit()'s initialiser: %v", err)
		return v.Done()
	}
	if err := e.Delete(givenNone); err != nil {
		v.Die("termcapinit()'s `given none` test: %v", err)
		return v.Done()
	}
	quiet(func(q *graph.Verbs) { q.DropParam("termcapinit", "name", "termcapinit() takes no name") })
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("termcapinit() takes no name -- nothing could assign the field it was handed -- "+
		"and the compiled default it substituted when it was given none, %s, is its "+
		"initialiser now.  That is internal/phase/035/edit.go's ui_write(console) again",
		strings.TrimSpace(dflt))

	// ---- mparm_T loses `term`
	var member *graph.Node
	for _, td := range e.Decls("mparm_T") {
		if td.Is("typedef") {
			for _, k := range td.Kids {
				if k.Is("struct") {
					for _, m := range graph.Members(k) {
						if m.Head() == "term" {
							member = m
						}
					}
				}
			}
		}
	}
	if member == nil {
		v.Die("mparm_T has no `char_u *term;` member to remove")
		return v.Done()
	}
	if u := e.Uses(member); len(u) != 0 {
		v.Die("%d uses still name this struct's `term` field", len(u))
		return v.Done()
	}
	if err := e.Delete(member); err != nil {
		v.Die("mparm_T's `term`: %v", err)
		return v.Done()
	}
	view := edit.Blank(v.Text())
	var owners []string
	for _, m := range w51Owner.FindAllSubmatch(view, -1) {
		if !edit.ContainsStr(owners, string(m[1])) {
			owners = append(owners, string(m[1]))
		}
	}
	sort.Strings(owners)
	if len(owners) == 0 {
		v.Die("nothing in the file names a `.term` member at all, so the partition below " +
			"says nothing -- read the file before removing this")
		return v.Done()
	}
	var mine []string
	for _, x := range owners {
		if x == "params" || x == "parmp" {
			mine = append(mine, x)
		}
	}
	if len(mine) > 0 {
		v.Die("%s still names this struct's `term` field", strings.Join(mine, " "))
		return v.Done()
	}
	v.Sayf("mparm_T loses its `term` member, in the EDIT: every one of the %d `.term` "+
		"mentions left belongs to another struct (%s), and deadfields.py matches by "+
		"NAME, so that tool could never see this one dead",
		len(w51DotTerm.FindAll(view, -1)), strings.Join(owners, " "))

	// ---- who calls set_termname(), and what `starting` can be
	stn := defn("set_termname")
	if v.Failed() {
		return v.Done()
	}
	var sites []string
	for _, u := range v.UsesOf("set_termname") {
		f := e.Function(u)
		if f == stn {
			continue
		}
		who := "?"
		if f != nil {
			who = graph.DeclName(f)
		}
		if !edit.ContainsStr(sites, who) {
			sites = append(sites, who)
		}
	}
	sort.Strings(sites)
	if strings.Join(sites, "\x00") != "did_set_term\x00termcapinit" {
		j := strings.Join(sites, " ")
		if j == "" {
			j = "nowhere"
		}
		v.Die("set_termname() is called from %s, and this phase is written against the "+
			"two -- termcapinit(), before there is a screen, and did_set_term(), at run "+
			"time", j)
		return v.Done()
	}
	var tabRows []string
	v.InTable("builtin_terminals", func(v *graph.Verbs) {
		for _, r := range v.Rows() {
			if r.Is("init") && len(r.Kids) == 3 && w51Content(r.Kids[1]) != "" {
				tabRows = append(tabRows, w51Content(r.Kids[1]))
			}
		}
	})
	dn := w51FirstLit.FindStringSubmatch(dflt)
	if dn == nil || !edit.ContainsStr(tabRows, dn[1]) {
		name := ""
		if dn != nil {
			name = dn[1]
		}
		v.Die("the compiled default %s is not a row of builtin_terminals[], so "+
			"termcapinit() can still be refused and the arm below is live", edit.PyRepr(name))
		return v.Done()
	}
	defaultName := dn[1]
	var assigns []string
	for _, u := range v.UsesOf("starting") {
		p := e.Parent(u)
		if p == nil || !p.Is("=") || p.Kids[1] != u || e.Item(p) != p {
			continue
		}
		x, err := graph.ExprText(p.Kids[2])
		if err != nil {
			v.Die("a store to `starting`: %v", err)
			return v.Done()
		}
		if !edit.ContainsStr(assigns, x) {
			assigns = append(assigns, x)
		}
	}
	sort.Strings(assigns)
	if edit.ContainsStr(assigns, "NO_SCREEN") {
		v.Die("something assigns starting = NO_SCREEN, so `starting != NO_SCREEN` is not "+
			"true wherever the arm below is reached: %s", strings.Join(assigns, " "))
		return v.Done()
	}
	v.Sayf("set_termname() is called from %s and from nowhere else; termcapinit() now "+
		"passes %s, which IS a row of builtin_terminals[]; and the only assignments to "+
		"`starting` are %s -- so a refusal can only come from did_set_term(), where "+
		"`starting != NO_SCREEN`",
		strings.Join(sites, " and "), edit.PyRepr(defaultName), strings.Join(assigns, " and "))

	// ---- the no-screen test folds ALWAYS, and the fallback after the refusal goes
	var arm *graph.Node
	v.InFunction("set_termname", func(v *graph.Verbs) {
		arms := v.Query("(if (== termp nullptr) ?b)", "b")
		if len(arms) != 1 {
			v.Die("set_termname() has no `termp == nullptr` arm")
			return
		}
		arm = arms[0]
	})
	if v.Failed() {
		return v.Done()
	}
	quiet(func(q *graph.Verbs) {
		q.In(arm, func(q *graph.Verbs) {
			q.FoldAlways("(!= starting NO_SCREEN)", 1, "the no-screen test")
		})
	})
	if v.Failed() {
		return v.Done()
	}
	k := -1
	for i, it := range arm.Kids {
		if it.Is("return") && len(it.Kids) == 2 && it.Kids[1].Atom == "FAIL" {
			k = i
			break
		}
	}
	if k < 0 || k+1 >= len(arm.Kids) {
		v.Die("nothing follows the refusal, so this phase has already been applied or " +
			"the arm is not the one it was written against")
		return v.Done()
	}
	tailItems := append([]*graph.Node(nil), arm.Kids[k+1:]...)
	tail, err := w51ItemsC(tailItems)
	if err != nil {
		v.Die("the fallback's C: %v", err)
		return v.Done()
	}
	if strings.TrimSpace(tail) == "" {
		v.Die("nothing follows the refusal, so this phase has already been applied or " +
			"the arm is not the one it was written against")
		return v.Done()
	}
	if err := e.ReplaceRun(tailItems[0], tailItems[len(tailItems)-1]); err != nil {
		v.Die("the fallback after the refusal: %v", err)
		return v.Done()
	}
	v.Sayf("the no-screen test folds ALWAYS, and what followed the refusal is unreachable "+
		"and goes: %s", strings.Join(strings.Fields(tail), " "))

	nm := w51FirstLit.FindStringSubmatch(tail)
	if nm == nil || !edit.ContainsStr(tabRows, nm[1]) {
		v.Die("the deleted fallback does not name a row of builtin_terminals[], so the " +
			"message below cannot be kept in step with it")
		return v.Done()
	}
	promised := nm[1]
	if !strings.Contains(tail, "report_default_term") {
		v.Die("the deleted text does not call report_default_term(), which this phase " +
			"leaves for the sweep -- read the arm before removing this")
		return v.Done()
	}

	// ---- report_term_error() stops promising it: each literal respelled whole
	re := regexp.MustCompile(`,[^"]*'` + regexp.QuoteMeta(promised) + `'`)
	v.InFunction("report_term_error", func(v *graph.Verbs) {
		changed := 0
		for _, s := range v.Strings() {
			if n := re.ReplaceAllString(s.Atom, ""); n != s.Atom {
				if err := e.RespellString(s, n); err != nil {
					v.Die("report_term_error()'s message: %v", err)
					return
				}
				changed++
			}
		}
		if changed == 0 {
			v.Die("report_term_error() does not promise %s, so there is nothing here to "+
				"keep in step with the fallback", edit.PyRepr(promised))
			return
		}
		var left []string
		for _, s := range v.Strings() {
			for _, x := range tabRows {
				if strings.Contains(s.Atom, "'"+x+"'") && !edit.ContainsStr(left, x) {
					left = append(left, x)
				}
			}
		}
		if len(left) > 0 {
			v.Die("report_term_error() still names %s after the cut", strings.Join(left, " "))
		}
	})
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("report_term_error() stops promising %s: phase 51a moved the message and the "+
		"fallback together because nothing in the build checks that a message tells the "+
		"truth, and this is that rule with no fallback left to name", edit.PyRepr(promised))

	// ---- `requested` is `term` for the 256-colour test
	v.InFunction("set_termname", func(v *graph.Verbs) {
		s := v.Text()
		if k := len(w51Requested.FindAll(edit.Blank(s), -1)); k != 2 {
			v.Die("`requested` has %d mentions in set_termname(), and this phase is "+
				"written against two -- its declaration and the 256-colour test", k)
			return
		}
		var rew []string
		for _, m := range w51TermRewr.FindAllSubmatch(s, -1) {
			x := strings.TrimSpace(string(m[1]))
			if !edit.ContainsStr(rew, x) {
				rew = append(rew, x)
			}
		}
		sort.Strings(rew)
		if strings.Join(rew, "\x00") != "+= 8" {
			j := strings.Join(rew, " / ")
			if j == "" {
				j = "nothing"
			}
			v.Die("`term` is rewritten as %s inside set_termname(), and `requested` can "+
				"only be folded into it while the prefix strip is the only one", j)
			return
		}
		var pre []string
		v.InFunction("term_is_builtin", func(v *graph.Verbs) {
			if m := w51Prefix.FindStringSubmatch(string(v.Text())); m != nil {
				pre = m
			}
		})
		if pre == nil {
			v.Die("term_is_builtin() does not strip a counted literal prefix, so what " +
				"`term += 8` skips cannot be read off the file")
			return
		}
		if n, _ := strconv.Atoi(pre[2]); len(pre[1]) != n {
			v.Die("term_is_builtin() does not strip a counted literal prefix, so what " +
				"`term += 8` skips cannot be read off the file")
			return
		}
		nd := w51Needle.FindSubmatch(s)
		if nd == nil {
			v.Die("the 256-colour test is not a musl_strstr on `requested`")
			return
		}
		if strings.IndexByte(pre[1], nd[1][0]) >= 0 {
			v.Die("%s begins with a character the stripped prefix %s contains, so a match "+
				"could start inside the prefix and `requested` is NOT `term` here",
				edit.PyRepr(string(nd[1])), edit.PyRepr(pre[1]))
			return
		}
		decl := v.Find("(def requested (ptr char_u) term)")
		if len(decl) != 1 {
			v.Die("`requested` is not declared as `= term`")
			return
		}
		termParam := decl[0].Kids[len(decl[0].Kids)-1].Refs
		uses := e.Uses(decl[0])
		if len(termParam) != 1 || len(uses) != 1 {
			v.Die("`requested` is not declared as `= term`")
			return
		}
		u := uses[0]
		c := e.Parent(u)
		for c != nil && !c.Is("call") {
			c = e.Parent(c)
		}
		if c == nil || c.Kids[1].Atom != "musl_strstr" || c.Kids[3].Atom != `"`+string(nd[1])+`"` {
			v.Die("the 256-colour test is not a musl_strstr on `requested`")
			return
		}
		ref := e.RefTo(termParam[0])
		if err := e.Replace(u, ref); err != nil {
			v.Die("the 256-colour test: %v", err)
			return
		}
		e.Rederive(ref) // the test above it typed again: `term` is what `requested` was
		v.Sayf("`requested` goes: it existed because the fallback reassigned `term`, and "+
			"the only rewrite left is the %s that strips %s -- %s cannot match inside "+
			"that, because it begins with a character the prefix does not hold",
			rew[0], edit.PyRepr(pre[1]), edit.PyRepr(string(nd[1])))
	})
	if v.Failed() {
		return v.Done()
	}

	// ---- what is left
	t := v.Text()
	goneNames := []string{"mainerr_arg_missing", "ME_GARBAGE", "ME_ARG_MISSING"}
	for _, name := range goneNames {
		if k := w51Mentions(t, name); k != 0 {
			v.Die("%s still has %d mentions", name, k)
			return v.Done()
		}
	}
	for _, name := range []string{"requested"} {
		if k := w51Mentions(t, name); k != 1 || !w51DeclOnly[name].Match(t) {
			v.Die("%s still has %d mentions, beyond its declaration", name, k-1)
			return v.Done()
		}
	}
	for _, name := range []string{"ME_UNKNOWN_OPTION", "ME_EXTRA_CMD", "MAX_ARG_CMDS",
		"exe_commands", "p_paste", "did_set_term", "report_term_error"} {
		if w51Mentions(t, name) == 0 {
			v.Die("%s went, and it is not this phase's", name)
			return v.Done()
		}
	}
	if k := w51Mentions(t, "report_default_term"); k != 1 {
		v.Die("report_default_term has %d mentions, and this phase leaves it at one -- "+
			"its own definition, which is what the sweep takes", k)
		return v.Done()
	}
	v.Sayf("0 mentions of %s; requested and report_default_term are down to their declarations and are the "+
		"sweep's; ME_UNKNOWN_OPTION, ME_EXTRA_CMD, MAX_ARG_CMDS, exe_commands and "+
		"'paste' are untouched", strings.Join(goneNames, ", "))
	return v.Done()
}
