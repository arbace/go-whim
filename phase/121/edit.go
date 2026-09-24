package p121

// Whim phase 121 -- the eight terminal names go, leaving two.  See GOAL.md.
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
// -- no surviving row carries that name -- and phase/121/check.go proves it again
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
// format string and the name move together, as phase/095/edit.go's `[RO]` did,
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
// flags, as every Part II edit since phase 85 does, and the source goes with it as
// $state/old.c.  The check needs both: this phase's delta is `term-moved`, and a
// table that moved is only evidence beside the table it moved from.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim121", Edit) }

// THE ONLY THING WRITTEN DOWN IN THIS PHASE.  Everything else is computed from
// it and from the source: which rows go, which capability tables die, which
// literals have to be rewritten and what the fallback may name.
var whim121Keep = []string{"xterm-256color", "debug"}

var (
	whim121Row    = regexp.MustCompile(`(?m)^[ \t]*\{\s*"([^"]*)"\s*,\s*(\w+)\s*\},\n`)
	whim121IfCall = regexp.MustCompile(`^\s*if\s*\(`)
	whim121Len    = regexp.MustCompile(`^[\s)]*,\s*\(\d+\)`)
)

// whim121Mentions counts an IDENTIFIER with string literals excluded.
//
// `builtin_xterm` is written inside a string literal as well as being a table,
// and a count that read that as a reference would report a dead table as live.
// That is why this blanks and p.mentions does not.
func whim121Mentions(text []byte, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).
		FindAll(cutil.Blank(text), -1))
}

type whim121Edit struct {
	a, z int
	rep  string
}

// Whim121 takes the terminal vocabulary: builtin_terminals[] goes from ten rows
// to two, with three capability tables, find_builtin_term()'s xterm-family
// clause and a repair to set_termname()'s no-screen fallback.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "terms", W: w}
	b := cutil.Blank(text)

	lineOf := func(off int) int { return bytes.Count(text[:off], []byte{'\n'}) + 1 }
	defspan := func(name string) (int, int, error) {
		a, z, ok := cutil.FindDefinition(text, b, name)
		if !ok {
			return 0, 0, p.Die("%s() is not defined in this file, and the partition below is drawn "+
				"against its extent", name)
		}
		return a, z, nil
	}

	// ---- 0. the table, and the two rows that stay ------------------------
	i := bytes.Index(text, []byte("static builtin_tcap_T builtin_terminals[] =\n{"))
	if i < 0 {
		return nil, p.Die("builtin_terminals[] is not in this file")
	}
	j := i + bytes.Index(text[i:], []byte("\n};"))
	type row struct {
		Name, tab string
		a, z      int
	}
	var rows []row
	for _, m := range whim121Row.FindAllSubmatchIndex(text[i:j], -1) {
		rows = append(rows, row{
			Name: string(text[i+m[2] : i+m[3]]),
			tab:  string(text[i+m[4] : i+m[5]]),
			a:    i + m[0], z: i + m[1],
		})
	}
	if len(rows) == 0 {
		return nil, p.Die("builtin_terminals[] holds no row in the {\"name\", table} shape, so the cut " +
			"below would be vacuous")
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if len(edit.Uniq(names)) != len(names) {
		return nil, p.Die("builtin_terminals[] names a terminal twice: %s", strings.Join(names, " "))
	}
	var missing []string
	for _, k := range whim121Keep {
		if !edit.ContainsStr(names, k) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, p.Die("builtin_terminals[] does not name %s, and that is a row this phase keeps",
			strings.Join(missing, " "))
	}
	var gone []string
	for _, n := range names {
		if !edit.ContainsStr(whim121Keep, n) {
			gone = append(gone, n)
		}
	}
	if len(gone) == 0 {
		return nil, p.Die("builtin_terminals[] holds nothing but the two rows that stay: there is " +
			"nothing here to remove, and every assertion below would be vacuous")
	}
	p.Sayf("builtin_terminals[] has %d rows.  %s stay and %d go -- %s.  The removed set is "+
		"the table MINUS the two, computed here, and the two are the only names this "+
		"phase writes down",
		len(rows), strings.Join(whim121Keep, " and "), len(gone), strings.Join(gone, " "))

	// ---- 1. THE PARTITION: every literal that IS a removed name ----------
	// cutil.Blank keeps offsets and blanks literal CONTENT, so a `"` left in
	// the blanked text is a real delimiter and the quotes pair up in order.
	// That is what makes this literal-aware: the identifier `builtin_xterm`,
	// the prefix inside `"screen.xterm"` and the substring of
	// `"xterm-256color"` are all invisible to it.
	var quotes []int
	for _, m := range regexp.MustCompile(`"`).FindAllIndex(b, -1) {
		quotes = append(quotes, m[0])
	}
	if len(quotes)%2 != 0 {
		return nil, p.Die("the file has an odd number of string delimiters after blanking, so the " +
			"literal spans below cannot be trusted")
	}
	var lits [][2]int
	for k := 0; k+1 < len(quotes); k += 2 {
		lits = append(lits, [2]int{quotes[k], quotes[k+1]})
	}

	famA, famZ, err := defspan("find_builtin_term")
	if err != nil {
		return nil, err
	}
	fbA, fbZ, err := defspan("set_termname")
	if err != nil {
		return nil, err
	}
	pxA, pxZ, err := defspan("vim_is_xterm")
	if err != nil {
		return nil, err
	}
	classes := []struct {
		kind   string
		lo, hi int
	}{
		{"row", i, j},
		{"family", famA, famZ},
		{"fallback", fbA, fbZ},
		{"prefix", pxA, pxZ},
	}
	part := map[string][][2]int{}
	var loose [][2]int
	for _, s := range lits {
		if !edit.ContainsStr(gone, string(text[s[0]+1:s[1]])) {
			continue
		}
		placed := false
		for _, c := range classes {
			if c.lo <= s[0] && s[0] < c.hi {
				part[c.kind] = append(part[c.kind], s)
				placed = true
				break
			}
		}
		if !placed {
			loose = append(loose, s)
		}
	}
	if len(loose) > 0 {
		var at []string
		for _, s := range loose {
			at = append(at, fmt.Sprintf("%s at line %d", cutil.PyRepr(string(text[s[0]+1:s[1]])), lineOf(s[0])))
		}
		return nil, p.Die("%d literal(s) spell a removed terminal name outside builtin_terminals[], "+
			"find_builtin_term(), set_termname() and vim_is_xterm(), and this phase has "+
			"no class for them: %s", len(loose), strings.Join(at, ", "))
	}
	if len(part["row"]) != len(gone) {
		return nil, p.Die("builtin_terminals[] holds %d literals spelling a removed name where %d "+
			"rows go", len(part["row"]), len(gone))
	}
	if len(part["family"]) != 1 || len(part["fallback"]) != 1 {
		return nil, p.Die("find_builtin_term() spells a removed name %d times and set_termname() %d, "+
			"and this phase is written against one of each",
			len(part["family"]), len(part["fallback"]))
	}
	if len(part["prefix"]) == 0 {
		return nil, p.Die("vim_is_xterm() spells no removed name, so the `prefix` class below would " +
			"be vacuous -- read the function before removing this")
	}
	// The `prefix` class is KEPT, and this is what makes keeping it honest:
	// each one must be an argument of a COUNTED comparison, so a name compared
	// in full could not sit here unnoticed.
	for _, s := range part["prefix"] {
		lo := s[0] - 80
		if lo < 0 {
			lo = 0
		}
		if !bytes.Contains(text[lo:s[0]], []byte("musl_strncasecmp")) {
			return nil, p.Die("vim_is_xterm() compares %s other than as a counted prefix, so it is a "+
				"terminal NAME there and not five characters",
				cutil.PyRepr(string(text[s[0]+1:s[1]])))
		}
		hi := s[1] + 16
		if hi > len(text) {
			hi = len(text)
		}
		if !whim121Len.Match(text[s[1]+1 : hi]) {
			return nil, p.Die("the comparison of %s in vim_is_xterm() carries no written length",
				cutil.PyRepr(string(text[s[0]+1:s[1]])))
		}
	}
	p.Sayf("the %d literals that spell a removed name partition exactly: %d rows, 1 in "+
		"find_builtin_term() (the xterm-family special case), 1 in set_termname() (the "+
		"fallback) and %d in vim_is_xterm(), which are counted PREFIX tests -- "+
		"musl_strncasecmp(name, %s, N) -- and %s, which stays, begins with it",
		len(gone)+2+len(part["prefix"]), len(gone), len(part["prefix"]),
		cutil.PyRepr(string(text[part["prefix"][0][0]+1:part["prefix"][0][1]])),
		cutil.PyRepr(whim121Keep[0]))

	// ---- 2. what the fallback may name, computed from termcapinit() ------
	tcA, tcZ, err := defspan("termcapinit")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, s := range lits {
		if s[0] < tcA || s[0] >= tcZ {
			continue
		}
		v := string(text[s[0]+1 : s[1]])
		if edit.ContainsStr(names, v) {
			seen[v] = true
		}
	}
	var compiled []string
	for k := range seen {
		compiled = append(compiled, k)
	}
	sort.Strings(compiled)
	if len(compiled) != 1 {
		j := strings.Join(compiled, " ")
		if j == "" {
			j = "none"
		}
		return nil, p.Die("termcapinit() spells %d of builtin_terminals[] names (%s), and this phase "+
			"needs exactly one -- the name it substitutes when it is given none",
			len(compiled), j)
	}
	dflt := compiled[0]
	if !edit.ContainsStr(whim121Keep, dflt) {
		return nil, p.Die("termcapinit()'s compiled default is %s, which this phase deletes: the "+
			"fallback cannot be retargeted onto a row that is going", cutil.PyRepr(dflt))
	}
	fa, fz := part["fallback"][0][0], part["fallback"][0][1]
	oldFallback := string(text[fa+1 : fz])
	p.Sayf("set_termname()'s no-screen fallback names %s, which goes; termcapinit()'s "+
		"compiled default is %s, which stays.  The fallback is retargeted onto it, so "+
		"after this phase there is ONE name the editor falls back to and one compiled "+
		"default, and they are the same name",
		cutil.PyRepr(oldFallback), cutil.PyRepr(dflt))

	// ---- 3. the family clause: dead by computation -----------------------
	ca, cz := part["family"][0][0], part["family"][0][1]
	if edit.ContainsStr(names, oldFallback) && !edit.ContainsStr(gone, oldFallback) {
		return nil, p.Die("%s is still a row of builtin_terminals[], so the special case in "+
			"find_builtin_term() is live and must not be removed",
			cutil.PyRepr(string(text[ca+1:cz])))
	}
	start := bytes.LastIndexByte(text[:ca], '\n') + 1
	headEnd := ca + bytes.IndexByte(text[ca:], '\n')
	head := string(text[start:headEnd])
	if !whim121IfCall.MatchString(head) || !strings.Contains(head, "vim_is_xterm") {
		return nil, p.Die("the literal %s in find_builtin_term() is not the condition of an `if` that "+
			"calls vim_is_xterm(): %s", cutil.PyRepr(string(text[ca+1:cz])), strings.TrimSpace(head))
	}
	closeParen := ca + bytes.IndexByte(text[ca:], ')')
	ob := closeParen + bytes.IndexByte(b[closeParen:], '{')
	cb := cutil.Match(b, ob)
	if cb < 0 {
		return nil, p.Die("the xterm-family clause's block is not balanced")
	}
	end := cb + 1
	if end < len(text) && text[end] == '\n' {
		end++
	}
	Body := text[ob+1 : cb]
	if bytes.Count(Body, []byte(";")) != 1 || !bytes.Contains(Body, []byte("return")) {
		return nil, p.Die("the xterm-family clause does more than return a table, and this phase is "+
			"written against the clause that does: %s", strings.Join(strings.Fields(string(Body)), " "))
	}
	p.Sayf("the xterm-family special case in find_builtin_term() tests the ROW's name "+
		"against %s, and no row will carry that name: the clause can never fire again.  "+
		"gcc has no warning for a condition that is false at run time and no tool in "+
		"tools/sweep.sh reads one, so it is the edit's to take -- %d lines at line %d",
		cutil.PyRepr(oldFallback), bytes.Count(text[start:end], []byte{'\n'}), lineOf(start))

	// ---- 4. the message that announces the fallback ----------------------
	// The message and the name move together.  Nothing in the build checks
	// that a message tells the truth, so a retargeted fallback with the old
	// name in its text would be a lie no harness could see.
	rtA, rtZ, err := defspan("report_term_error")
	if err != nil {
		return nil, err
	}
	quoted := "'" + oldFallback + "'"
	var msgs [][2]int
	for _, s := range lits {
		if s[0] >= rtA && s[0] < rtZ && strings.Contains(string(text[s[0]+1:s[1]]), quoted) {
			msgs = append(msgs, s)
		}
	}
	if len(msgs) == 0 {
		return nil, p.Die("report_term_error() does not spell %s, so this phase cannot keep its "+
			"message and its fallback in step", quoted)
	}
	p.Sayf("report_term_error() spells %s in %d message(s), and each is rewritten in the "+
		"same step as the fallback itself -- nothing in the build checks that a message "+
		"tells the truth", quoted, len(msgs))

	// ---- 5. ONE PASS over the original text ------------------------------
	// Every edit below is an offset into the text as it was read.  A second
	// pass would index its spans against the first pass's output; phase
	// 106 measured that and left five of 437 names behind in a file that still
	// compiled.
	var edits []whim121Edit
	for _, r := range rows {
		if edit.ContainsStr(gone, r.Name) {
			edits = append(edits, whim121Edit{r.a, r.z, ""})
		}
	}
	edits = append(edits, whim121Edit{start, end, ""})
	edits = append(edits, whim121Edit{fa, fz + 1, `"` + dflt + `"`})
	for _, s := range msgs {
		was := string(text[s[0]+1 : s[1]])
		edits = append(edits, whim121Edit{s[0], s[1] + 1,
			`"` + strings.ReplaceAll(was, quoted, "'"+dflt+"'") + `"`})
	}
	sort.Slice(edits, func(x, y int) bool { return edits[x].a < edits[y].a })
	for k := 1; k < len(edits); k++ {
		if edits[k].a < edits[k-1].z {
			return nil, p.Die("two of this phase's edits overlap at line %d, so applying them in one "+
				"pass would corrupt the file", lineOf(edits[k].a))
		}
	}
	var Out []byte
	prev := 0
	for _, e := range edits {
		Out = append(Out, text[prev:e.a]...)
		Out = append(Out, e.rep...)
		prev = e.z
	}
	Out = append(Out, text[prev:]...)
	text = Out

	// ---- 6. what the sweep is handed, computed ---------------------------
	// A `builtin_*` table whose only mention left -- literals excluded -- is
	// its own definition is dead.  The rule is what is asserted; the three
	// names it computes are printed, not required.
	var tabs []string
	for _, r := range rows {
		if !edit.ContainsStr(tabs, r.tab) {
			tabs = append(tabs, r.tab)
		}
	}
	sort.Strings(tabs)
	var dead, live []string
	for _, x := range tabs {
		if whim121Mentions(text, x) == 1 {
			dead = append(dead, x)
		} else {
			live = append(live, x)
		}
	}
	if len(dead) == 0 {
		return nil, p.Die("every capability table builtin_terminals[] pointed at still has a row, so " +
			"this cut orphans nothing and the sweep has nothing to find")
	}
	for _, x := range dead {
		if !regexp.MustCompile(`(?m)^static \w+ ` + regexp.QuoteMeta(x) + `\[\] =\n\{`).Match(text) {
			return nil, p.Die("%s has one mention left and it is not its own definition", x)
		}
	}
	p.Sayf("%d of the %d capability tables builtin_terminals[] pointed at have no row "+
		"left and are the sweep's: %s.  %s stay, each still pointed at -- and the "+
		"count is taken on BLANKED text, because \"builtin_xterm\" is also a string "+
		"literal and a count that read that as a reference would report a live table "+
		"dead", len(dead), len(tabs), strings.Join(dead, " "), strings.Join(live, " "))

	// ---- 7. nothing spells a removed name any more, except the prefix tests
	b2 := cutil.Blank(text)
	var q2 []int
	for _, m := range regexp.MustCompile(`"`).FindAllIndex(b2, -1) {
		q2 = append(q2, m[0])
	}
	var left [][2]int
	for k := 0; k+1 < len(q2); k += 2 {
		if edit.ContainsStr(gone, string(text[q2[k]+1:q2[k+1]])) {
			left = append(left, [2]int{q2[k], q2[k+1]})
		}
	}
	lo2, hi2, ok := cutil.FindDefinition(text, b2, "vim_is_xterm")
	if !ok {
		return nil, p.Die("vim_is_xterm() is not defined in this file, and the partition below is " +
			"drawn against its extent")
	}
	var stray [][2]int
	for _, s := range left {
		if !(lo2 <= s[0] && s[0] < hi2) {
			stray = append(stray, s)
		}
	}
	if len(stray) > 0 {
		var at []string
		for _, s := range stray {
			at = append(at, cutil.PyRepr(string(text[s[0]+1:s[1]])))
		}
		return nil, p.Die("%d literal(s) still spell a removed terminal name outside vim_is_xterm(): "+
			"%s", len(stray), strings.Join(at, ", "))
	}
	if len(left) != len(part["prefix"]) {
		return nil, p.Die("vim_is_xterm() holds %d literals spelling a removed name and held %d",
			len(left), len(part["prefix"]))
	}
	p.Sayf("nothing in the output spells a removed terminal name except the %d counted "+
		"prefix test(s) in vim_is_xterm(), which is the one class this partition keeps",
		len(left))
	return text, nil
}
