package p051a

// Whim phase 51a (formerly 121) -- the eight terminal names go, leaving two.  See GOAL.md.
//
// `builtin_terminals[]` is the whole of what the core knows how to draw on: a name
// and a capability table, ten times.  An embeddable core has no business carrying
// ten terminal descriptions -- the host decides what it is attached to -- and this
// phase keeps TWO: `xterm-256color`, which is already the name `termcapinit()`
// substitutes when it is given none, and `debug`, which draws its capabilities as
// text and is the only one that can be read without a terminal at all.
//
// WHICH ROWS GO IS COMPUTED: the table MINUS the two, read out of the source.  So is
// everything that follows from it -- which capability tables die, which string
// literals have to be rewritten, and what the fallback may name.  The two names are
// the only thing written down here, and `KEEP` is where they are written.
//
// THE PARTITION, WHICH IS THE WHOLE OF THE ARGUMENT.  A terminal name in this file
// is a STRING LITERAL, so the cut is a cut inside literals -- and the same words
// appear elsewhere as identifiers (`builtin_xterm`), as prefixes (`vim_is_xterm`'s
// `musl_strncasecmp(name, "xterm", 5)`) and inside other literals
// (`"builtin_xterm"`, `"screen.xterm"`).  A textual `s/xterm//` would wreck all of
// them.  So the edit walks the file's string literals -- `cutil.blank()` preserves
// offsets and blanks literal CONTENT, so a `"` surviving in the blanked text is a
// real delimiter and the quotes pair up in order -- and every literal whose content
// EQUALS a removed name must fall in one of four classes:
//
// row       inside builtin_terminals[]              the row itself; deleted
// family    inside find_builtin_term()              the xterm-family special case
// fallback  inside set_termname()                   retargeted, see below
// prefix    inside vim_is_xterm()                   KEPT, with its reason
//
// A literal that falls in none refuses, which stays true of a file this edit has
// never seen.  Measured on the input: 11 literals, 8 + 1 + 1 + 1.
//
// THE `prefix` CLASS IS KEPT AND IS NOT AN EXCEPTION.  `vim_is_xterm()` asks whether
// a name BEGINS with `xterm` -- `musl_strncasecmp(name, "xterm", 5)`, with the
// length written out -- and `xterm-256color`, which stays, begins with it.  That
// function still has a caller (`term_is_xterm = vim_is_xterm(term)`), so the test is
// live and the literal is not a terminal name at all: it is five characters.  The
// edit requires every `prefix` literal to be an argument of a counted comparison,
// so a naked `musl_strcmp` against a name that no longer exists could not hide here.
//
// THE `family` CLASS IS DEAD CODE THE SWEEP CANNOT SEE, and this is why it goes in
// the EDIT.  `find_builtin_term()` walks the table and returns a row's table when
// `musl_strcmp(name, "xterm") == 0 && vim_is_xterm(term)` -- a test on the ROW's
// name, so the whole clause is a way of saying "the row called xterm serves the
// whole xterm family".  Once no row is called that, the first conjunct is false for
// every row and the clause can never fire; gcc has no warning for a condition that
// is false at run time, and no tool in tools/sweep.sh reads one.  It is the shape
// CLAUDE.md states for a struct field whose only reader a phase deletes: a thing the
// edit knows is dead is the edit's to take.  The edit proves it dead by COMPUTATION
// -- no surviving row carries that name -- and internal/phase/051/a/check.go proves it again
// by instrumenting the clause and finding it entered 0 times where the input enters
// it on every startup.
//
// AND THE INPUT ENTERS IT ON EVERY STARTUP, which is worth knowing before anything
// here is rearranged: the compiled default is `xterm-256color`, the `xterm` row
// comes BEFORE the `xterm-256color` row, and `vim_is_xterm("xterm-256color")` is
// true -- so every startup of the input resolves through the family clause and the
// `xterm-256color` row is never reached.  After this phase it is.  Same table either
// way (`builtin_xterm`), which is why nothing in the recording moves for it.
//
// THE `fallback` CLASS IS THE ONE REPAIR, AND IT IS NOT OPTIONAL.  `set_termname()`
// answers an unknown name in two ways: at run time (`:set term=vt320`) it reports
// and FAILS, leaving the terminal alone, which is the E522 the table records; but
// before there is a screen (`starting == NO_SCREEN`, which is the `-T {term}` path)
// it substitutes a name of its own and carries on.  That name is written in the
// source, and it is `xterm` -- one of the eight this phase deletes.  Left alone, the
// cut would leave the fallback naming a terminal that no longer exists, and MEASURED
// on this phase's own output with the repair left out: `-T xterm` and
// `-T no-such-term-9x` both print `E437: Terminal capability "cm" required` and draw
// 2,045 bytes where the baseline draws 2,117 -- an editor with no cursor motion.
// That is not a capability removed on purpose, it is a dangling name.
//
// So the fallback is retargeted, and the new name is COMPUTED and not written here:
// it is the name `termcapinit()` substitutes when it is given none, which the edit
// reads out of that function and requires to be a row that stays.  After this phase
// there is ONE name the editor falls back to and one compiled default, and they are
// the same name.  The message that announces it is rewritten in the same step -- the
// format string and the name move together, as internal/phase/034/edit.go's `[RO]` did,
// because nothing in the build checks that a message tells the truth.
//
// WHAT THE SWEEP THEN FINDS: three capability tables, `builtin_ansi`,
// `builtin_vt100` and `builtin_dumb`, as -Wunused-variable.  That set is COMPUTED
// too -- a `builtin_*` table whose only mention left, literals excluded, is its own
// definition -- and the edit asserts the set rather than naming three of them.
// `builtin_xterm` is NOT in it: `xterm-256color` still points at it, and the
// mention inside the string literal `"builtin_xterm"` is a literal and not a
// reference, which is why the count is taken on blanked text.
//
// NOT THIS PHASE'S, AND DELIBERATELY LEFT: the 256-colour add-on's test on
// `requested`, and `-T {term}` itself.  Both are the next phase's, and the second is
// what makes the fallback reachable at all -- remove `-T` and `termcapinit()` can
// only ever be handed nothing, so `set_termname()`'s no-screen arm becomes
// unreachable and the sweep takes `report_term_error()` with it.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 4e does, and the source goes with it as
// $state/old.c.  The check needs both: this phase's delta is `term-moved`, and a
// table that moved is only evidence beside the table it moved from.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// cut is the program's graph edited through crefactor/graph's verbs, and
// its report is the text version's, which the plan ran until then (history
// keeps it): B2b's TestRows51a was its proof in advance.
//
//   - the rows are the table's initialiser elements (Rows), the eight that
//     go deleted in one arrangement (INITROW's DeleteRowsEach);
//   - THE PARTITION is the graph's string literals (Strings), each placed by
//     the top-level form it is in -- the table, find_builtin_term(),
//     set_termname(), vim_is_xterm() -- and a `prefix` literal must be an
//     argument of a call of musl_strncasecmp whose next argument is an
//     integer constant: the counted comparison, by its form, where the text
//     looked 80 bytes back and 16 forward;
//   - the family clause is the `if` around its literal, its condition a
//     call of vim_is_xterm and its block one `return`, cut whole;
//   - the fallback and the messages are RespellString, each literal named
//     whole; the text's `'xterm'` inside a message is that literal
//     respelled, never a replacement across literals;
//   - the dead capability tables are still counted the text's way, on the
//     C view with literals blanked, since "builtin_xterm" is a literal too.
//
// The line the clause began at is read off the C view, where the text read
// it off the text: the same file.

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim51a", Edit) }

// THE ONLY THING WRITTEN DOWN IN THIS PHASE.  Everything else is computed from
// it and from the source: which rows go, which capability tables die, which
// literals have to be rewritten and what the fallback may name.
var whim51aKeep = []string{"xterm-256color", "debug"}

// whim51aMentions counts an IDENTIFIER with string literals excluded.
//
// `builtin_xterm` is written inside a string literal as well as being a table,
// and a count that read that as a reference would report a dead table as live.
// That is why this blanks and p.mentions does not.
func whim51aMentions(text []byte, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).
		FindAll(edit.Blank(edit.WithoutIncludes(text)), -1))
}

// whim51aContent is a plain string literal's content, "" for anything else.
func whim51aContent(s *graph.Node) string {
	if a := s.Atom; len(a) >= 2 && a[0] == '"' && a[len(a)-1] == '"' {
		return a[1 : len(a)-1]
	}
	return ""
}

// whim51aStmtC is a statement's C as the C view prints it, one line a
// string, its own indentation taken off.
func whim51aStmtC(n *graph.Node) ([]string, error) {
	f := clisp.L(clisp.A("defn"), clisp.A("f"), clisp.L(clisp.A("fn"), clisp.L(clisp.A("void")), clisp.A("void")), graph.Lisp(n))
	out, err := clisp.Print([]*clisp.Node{f})
	if err != nil {
		return nil, err
	}
	ls := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var body []string
	for i, l := range ls {
		if l == "{" {
			for _, b := range ls[i+1 : len(ls)-1] {
				body = append(body, strings.TrimPrefix(b, clisp.Indent))
			}
			break
		}
	}
	return body, nil
}

// Edit is phase 51a on the graph: eight rows of builtin_terminals[] go, the
// family clause that only a row named "xterm" could satisfy goes, and the
// fallback and its messages name the compiled default.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("terms", e, w)
	quiet := func(acts func(q *graph.Verbs)) {
		if v.Failed() {
			return
		}
		q := graph.NewVerbs("terms", e, io.Discard)
		acts(q)
		if q.Err != nil {
			v.Err = q.Err
		}
	}
	defn := func(name string) *graph.Node {
		d := e.Defn(name)
		if d == nil {
			v.Die("%s() is not defined in this file, and the partition below is drawn against its extent", name)
		}
		return d
	}

	// the table
	var table *graph.Node
	for _, d := range e.FileDecls("builtin_terminals") {
		if d.Is("def") {
			table = d
		}
	}
	if table == nil {
		v.Die("builtin_terminals[] is not in this file")
		return v.Done()
	}
	type row struct {
		name, tab string
		n         *graph.Node
	}
	var rows []row
	v.InTable("builtin_terminals", func(v *graph.Verbs) {
		for _, r := range v.Rows() {
			if r.Is("init") && len(r.Kids) == 3 && whim51aContent(r.Kids[1]) != "" {
				rows = append(rows, row{whim51aContent(r.Kids[1]), r.Kids[2].Atom, r})
			}
		}
	})
	if v.Failed() {
		return v.Done()
	}
	if len(rows) == 0 {
		v.Die("builtin_terminals[] holds no row in the {\"name\", table} shape, so the cut " +
			"below would be vacuous")
		return v.Done()
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.name)
	}
	if len(edit.Uniq(names)) != len(names) {
		v.Die("builtin_terminals[] names a terminal twice: %s", strings.Join(names, " "))
		return v.Done()
	}
	var missing []string
	for _, k := range whim51aKeep {
		if !edit.ContainsStr(names, k) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		v.Die("builtin_terminals[] does not name %s, and that is a row this phase keeps",
			strings.Join(missing, " "))
		return v.Done()
	}
	var gone []string
	for _, n := range names {
		if !edit.ContainsStr(whim51aKeep, n) {
			gone = append(gone, n)
		}
	}
	if len(gone) == 0 {
		v.Die("builtin_terminals[] holds nothing but the two rows that stay: there is " +
			"nothing here to remove, and every assertion below would be vacuous")
		return v.Done()
	}
	v.Sayf("builtin_terminals[] has %d rows.  %s stay and %d go -- %s.  The removed set is "+
		"the table MINUS the two, computed here, and the two are the only names this "+
		"phase writes down",
		len(rows), strings.Join(whim51aKeep, " and "), len(gone), strings.Join(gone, " "))

	// THE PARTITION: every literal spelling a removed name, by the form it is in.
	fam, fb, px := defn("find_builtin_term"), defn("set_termname"), defn("vim_is_xterm")
	tc, rt := defn("termcapinit"), defn("report_term_error")
	if v.Failed() {
		return v.Done()
	}
	classes := []struct {
		kind string
		form *graph.Node
	}{{"row", table}, {"family", fam}, {"fallback", fb}, {"prefix", px}}
	part := map[string][]*graph.Node{}
	var loose []string
	lits := v.Strings()
	for _, s := range lits {
		if !edit.ContainsStr(gone, whim51aContent(s)) {
			continue
		}
		placed := false
		for _, c := range classes {
			if e.TopForm(s) == c.form {
				part[c.kind] = append(part[c.kind], s)
				placed = true
				break
			}
		}
		if !placed {
			where := "?"
			if f := e.TopForm(s); f != nil {
				where = graph.DeclName(f)
			}
			loose = append(loose, fmt.Sprintf("%s in %s", s.Atom, where))
		}
	}
	if len(loose) > 0 {
		v.Die("%d literal(s) spell a removed terminal name outside builtin_terminals[], "+
			"find_builtin_term(), set_termname() and vim_is_xterm(), and this phase has "+
			"no class for them: %s", len(loose), strings.Join(loose, ", "))
		return v.Done()
	}
	if len(part["row"]) != len(gone) {
		v.Die("builtin_terminals[] holds %d literals spelling a removed name where %d "+
			"rows go", len(part["row"]), len(gone))
		return v.Done()
	}
	if len(part["family"]) != 1 || len(part["fallback"]) != 1 {
		v.Die("find_builtin_term() spells a removed name %d times and set_termname() %d, "+
			"and this phase is written against one of each",
			len(part["family"]), len(part["fallback"]))
		return v.Done()
	}
	if len(part["prefix"]) == 0 {
		v.Die("vim_is_xterm() spells no removed name, so the `prefix` class below would " +
			"be vacuous -- read the function before removing this")
		return v.Done()
	}
	// a prefix literal is an argument of musl_strncasecmp, the next argument
	// an integer constant: a counted comparison
	for _, s := range part["prefix"] {
		var call, arg *graph.Node
		for x := s; x != nil; x = e.Parent(x) {
			if p := e.Parent(x); p != nil && p.Is("call") {
				call, arg = p, x
				break
			}
		}
		if call == nil || len(call.Kids) < 2 || call.Kids[1].Atom != "musl_strncasecmp" {
			v.Die("vim_is_xterm() compares %s other than as a counted prefix, so it is a "+
				"terminal NAME there and not five characters", s.Atom)
			return v.Done()
		}
		counted := false
		for i, k := range call.Kids {
			if k == arg && i+1 < len(call.Kids) {
				n := call.Kids[i+1]
				for n.Is("paren") && len(n.Kids) == 2 {
					n = n.Kids[1]
				}
				counted = !n.IsList() && n.Atom != "" && n.Atom[0] >= '0' && n.Atom[0] <= '9'
			}
		}
		if !counted {
			v.Die("the comparison of %s in vim_is_xterm() carries no written length", s.Atom)
			return v.Done()
		}
	}
	v.Sayf("the %d literals that spell a removed name partition exactly: %d rows, 1 in "+
		"find_builtin_term() (the xterm-family special case), 1 in set_termname() (the "+
		"fallback) and %d in vim_is_xterm(), which are counted PREFIX tests -- "+
		"musl_strncasecmp(name, %s, N) -- and %s, which stays, begins with it",
		len(gone)+2+len(part["prefix"]), len(gone), len(part["prefix"]),
		edit.PyRepr(whim51aContent(part["prefix"][0])), edit.PyRepr(whim51aKeep[0]))

	// the compiled default: the one row name termcapinit() spells
	seen := map[string]bool{}
	for _, s := range lits {
		if e.TopForm(s) == tc && edit.ContainsStr(names, whim51aContent(s)) {
			seen[whim51aContent(s)] = true
		}
	}
	compiled := edit.SortedKeys(seen)
	if len(compiled) != 1 {
		j := strings.Join(compiled, " ")
		if j == "" {
			j = "none"
		}
		v.Die("termcapinit() spells %d of builtin_terminals[] names (%s), and this phase "+
			"needs exactly one -- the name it substitutes when it is given none",
			len(compiled), j)
		return v.Done()
	}
	dflt := compiled[0]
	if !edit.ContainsStr(whim51aKeep, dflt) {
		v.Die("termcapinit()'s compiled default is %s, which this phase deletes: the "+
			"fallback cannot be retargeted onto a row that is going", edit.PyRepr(dflt))
		return v.Done()
	}
	fallback := part["fallback"][0]
	oldFallback := whim51aContent(fallback)
	v.Sayf("set_termname()'s no-screen fallback names %s, which goes; termcapinit()'s "+
		"compiled default is %s, which stays.  The fallback is retargeted onto it, so "+
		"after this phase there is ONE name the editor falls back to and one compiled "+
		"default, and they are the same name",
		edit.PyRepr(oldFallback), edit.PyRepr(dflt))

	// the family clause: the if around its literal
	famLit := part["family"][0]
	if edit.ContainsStr(names, oldFallback) && !edit.ContainsStr(gone, oldFallback) {
		v.Die("%s is still a row of builtin_terminals[], so the special case in "+
			"find_builtin_term() is live and must not be removed", famLit.Atom)
		return v.Done()
	}
	var clause *graph.Node
	for x := e.Parent(famLit); x != nil; x = e.Parent(x) {
		if x.Is("if") {
			clause = x
			break
		}
	}
	callsXterm := false
	if clause != nil {
		graph.Walk(clause.Kids[1], func(n *graph.Node) bool {
			if n.Is("call") && len(n.Kids) > 1 && n.Kids[1].Atom == "vim_is_xterm" {
				callsXterm = true
			}
			return true
		})
	}
	if clause == nil || !callsXterm {
		v.Die("the literal %s in find_builtin_term() is not the condition of an `if` that "+
			"calls vim_is_xterm()", famLit.Atom)
		return v.Done()
	}
	blk := clause.Kids[2]
	if len(clause.Kids) != 3 || !blk.Is("block") || len(blk.Kids) != 2 || !blk.Kids[1].Is("return") {
		v.Die("the xterm-family clause does more than return a table, and this phase is " +
			"written against the clause that does")
		return v.Done()
	}
	// its place in the C view, as the text read it off the text
	stmt, err := whim51aStmtC(clause)
	if err != nil {
		v.Die("the C view of the clause: %v", err)
		return v.Done()
	}
	at := 0
	view := strings.Split(string(v.Text()), "\n")
	head := "find_builtin_term("
	for i, l := range view {
		if strings.HasPrefix(l, head) {
			for j := i; j < len(view); j++ {
				if strings.TrimSpace(view[j]) == stmt[0] {
					at = j + 1
					break
				}
			}
			break
		}
	}
	v.Sayf("the xterm-family special case in find_builtin_term() tests the ROW's name "+
		"against %s, and no row will carry that name: the clause can never fire again.  "+
		"gcc has no warning for a condition that is false at run time and no tool in "+
		"tools/sweep.sh reads one, so it is the edit's to take -- %d lines at line %d",
		edit.PyRepr(oldFallback), len(stmt), at)

	quoted := "'" + oldFallback + "'"
	var msgs []*graph.Node
	for _, s := range lits {
		if e.TopForm(s) == rt && strings.Contains(whim51aContent(s), quoted) {
			msgs = append(msgs, s)
		}
	}
	if len(msgs) == 0 {
		v.Die("report_term_error() does not spell %s, so this phase cannot keep its "+
			"message and its fallback in step", quoted)
		return v.Done()
	}
	v.Sayf("report_term_error() spells %s in %d message(s), and each is rewritten in the "+
		"same step as the fallback itself -- nothing in the build checks that a message "+
		"tells the truth", quoted, len(msgs))

	// the edits
	quiet(func(q *graph.Verbs) {
		q.InTable("builtin_terminals", func(q *graph.Verbs) {
			// each row found by its name, all deleted in one arrangement,
			// the table typed again for its new length (ArrangeRowsTyped:
			// what an import of the result gives, which the next phase is
			// handed)
			const what = "the terminals that are not the product's"
			out := map[*graph.Node]bool{}
			for _, g := range gone {
				pat := fmt.Sprintf("(init %q _)", g)
				r := q.Row(pat, what+": "+pat)
				if r == nil {
					return
				}
				out[r] = true
			}
			var order []*graph.Node
			for _, r := range graph.TableInit(q.Scope()).Args() {
				if !out[r] {
					order = append(order, r)
				}
			}
			if _, err := e.ArrangeRowsTyped(q.Scope(), order, graph.RowIndex{}); err != nil {
				q.Die("%s -- %v", what, err)
				return
			}
			q.Say(what)
		})
	})
	if v.Failed() {
		return v.Done()
	}
	if err := e.Delete(clause); err != nil {
		v.Die("the family clause: %v", err)
		return v.Done()
	}
	if err := e.RespellString(fallback, `"`+dflt+`"`); err != nil {
		v.Die("the fallback: %v", err)
		return v.Done()
	}
	for _, s := range msgs {
		was := whim51aContent(s)
		if err := e.RespellString(s, `"`+strings.ReplaceAll(was, quoted, "'"+dflt+"'")+`"`); err != nil {
			v.Die("a message: %v", err)
			return v.Done()
		}
	}

	// what the cut leaves: the capability tables no row points at
	text := v.Text()
	var tabs []string
	for _, r := range rows {
		if !edit.ContainsStr(tabs, r.tab) {
			tabs = append(tabs, r.tab)
		}
	}
	sort.Strings(tabs)
	var dead, live []string
	for _, x := range tabs {
		if whim51aMentions(text, x) == 1 {
			dead = append(dead, x)
		} else {
			live = append(live, x)
		}
	}
	if len(dead) == 0 {
		v.Die("every capability table builtin_terminals[] pointed at still has a row, so " +
			"this cut orphans nothing and the sweep has nothing to find")
		return v.Done()
	}
	for _, x := range dead {
		if !regexp.MustCompile(`(?m)^static \w+ ` + regexp.QuoteMeta(x) + `\[\] =\n\{`).Match(text) {
			v.Die("%s has one mention left and it is not its own definition", x)
			return v.Done()
		}
	}
	v.Sayf("%d of the %d capability tables builtin_terminals[] pointed at have no row "+
		"left and are the sweep's: %s.  %s stay, each still pointed at -- and the "+
		"count is taken on BLANKED text, because \"builtin_xterm\" is also a string "+
		"literal and a count that read that as a reference would report a live table "+
		"dead", len(dead), len(tabs), strings.Join(dead, " "), strings.Join(live, " "))

	var left, stray []string
	for _, s := range v.Strings() {
		if edit.ContainsStr(gone, whim51aContent(s)) {
			left = append(left, s.Atom)
			if e.TopForm(s) != px {
				stray = append(stray, s.Atom)
			}
		}
	}
	if len(stray) > 0 {
		v.Die("%d literal(s) still spell a removed terminal name outside vim_is_xterm(): "+
			"%s", len(stray), strings.Join(stray, ", "))
		return v.Done()
	}
	if len(left) != len(part["prefix"]) {
		v.Die("vim_is_xterm() holds %d literals spelling a removed name and held %d",
			len(left), len(part["prefix"]))
		return v.Done()
	}
	v.Sayf("nothing in the output spells a removed terminal name except the %d counted "+
		"prefix test(s) in vim_is_xterm(), which is the one class this partition keeps",
		len(left))
	return v.Done()
}
