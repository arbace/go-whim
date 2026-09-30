package p122

// Whim phase 122 -- `-T {term}` goes, and the command line is `+{command}` alone.
// See GOAL.md.
//
// Phase 88 left argv as exactly two options: `+{command}`, which is how a host
// tells the editor what to do, and `-T {term}`, which is how a SHELL told it what it
// was attached to.  A core is told that by its host or not at all -- and `-T` has
// had a replacement inside the editor since before this pipeline began: `+set term=`
// reaches did_set_term() and does everything `-T` did, which is why phase 116
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
// reason phase 121's dead clause did not have: deadfields.py
// matches by NAME, and this file holds thirty-two mentions of
// another struct's `.term` member (attr_entry's `ae_u.term`), so
// that tool can never see this one dead.  The edit computes that
// partition rather than asserting it.
// termcapinit()   it can only ever be handed what the memset left, so it takes no
// name at all now and the compiled default -- read out of the
// function, not written here -- is its initialiser.  That is
// internal/phase/096/edit.go's ui_write(console) again.
//
// AND THEN set_termname()'s NO-SCREEN ARM CANNOT RUN.  set_termname() has two call
// sites, and the edit partitions them: termcapinit()'s, which reaches it before
// there is a screen, and did_set_term()'s, which is `:set term=` at run time.  The
// first can no longer fail -- the compiled default IS a row of builtin_terminals[],
// which the edit checks -- so when find_builtin_term() answers nullptr the call came
// from the second, where `starting` is NO_BUFFERS or 0 and never NO_SCREEN (the edit
// reads every assignment to `starting` and requires none of them to be NO_SCREEN).
// So `if (starting != NO_SCREEN)` is always true there, the block returns FAIL, and
// the three statements after it are unreachable: the fallback that phase 121 had to
// repair, report_default_term(), and the option write that recorded it.
// on both texts -- the input enters the fallback in exactly the two `-T` records and
// a control built from THIS phase's output enters it in none.
//
// THE MESSAGE GOES WITH THE FALLBACK, for phase 121's reason read backwards.  That
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
// NOT create_cmdidxs --check, for internal/phase/085/edit.go's reason: the derived
// first-two-letters index went with the command table whim reduced, and the tool
// raises rather than reporting nothing.  Nothing here touches the command table.
//

import (
	"bytes"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim122", Edit) }

var (
	w122EmptyName = `^[ \t]*if \(term != nullptr && \*term == NUL\)$`
	w122GivenNone = regexp.MustCompile(edit.Head("if (term == nullptr || *term == NUL)"))
	w122Assign    = regexp.MustCompile(`(?s)\A\s*\n[ \t]*term = (.*?);\n[ \t]*\z`)
	w122TermDecl  = regexp.MustCompile(`(?m)^([ \t]*char_u +\*term) = name;$`)
	w122TciProto  = regexp.MustCompile(`(?m)^static void termcapinit\([^)]*\);$`)
	w122Member    = regexp.MustCompile(`(?m)^[ \t]*char_u +\*term;\n`)
	w122Owner     = regexp.MustCompile(`(\w+)\s*(?:\.|->)\s*term\b`)
	w122DotTerm   = regexp.MustCompile(`(?:\.|->)\s*term\b`)
	w122SetTerm   = regexp.MustCompile(`\bset_termname\s*\(`)
	w122FnName    = regexp.MustCompile(`\n[a-zA-Z_]\w*`)
	w122Row       = regexp.MustCompile(`(?m)^[ \t]*\{\s*"([^"]*)"\s*,\s*\w+\s*\},$`)
	w122Starting  = regexp.MustCompile(`(?m)^[ \t]*starting = ([^;]+);$`)
	w122TermpNull = regexp.MustCompile(edit.Head("if (termp == nullptr)"))
	w122NoScreen  = `^[ \t]*if \(starting != NO_SCREEN\)$`
	w122FirstLit  = regexp.MustCompile(`"([^"]*)"`)
	w122Requested = regexp.MustCompile(`\brequested\b`)
	w122TermRewr  = regexp.MustCompile(`(?m)^[ \t]*term (\+?=[^;]*);$`)
	w122Prefix    = regexp.MustCompile(`musl_strncmp\(\(char \*\)\(name\), \(char \*\)\("([^"]*)"\), \(\(usize\)(\d+)\)\)`)
	w122Needle    = regexp.MustCompile(`musl_strstr\(\(char \*\)requested, "([^"]*)"\)`)
	w122ReqDecl   = regexp.MustCompile(`\n[ \t]*char_u +\*requested = term;\n`)
	// w122DeclOnly: a local's declaration, what is left of it for the sweep.
	w122DeclOnly = map[string]*regexp.Regexp{
		"requested": regexp.MustCompile(`\bchar_u +\*requested = term;`),
	}
)

// w122Mentions counts an IDENTIFIER with string literals excluded.
func w122Mentions(text []byte, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).
		FindAll(edit.Blank(edit.WithoutIncludes(text)), -1))
}

// Whim122 removes `-T {term}`: command_line_scan() becomes one `if (argv[0][0]
// == '+')` and one `else` answering mainerr(ME_UNKNOWN_OPTION), with two
// main_errors[] rows and their enumerators, mparm_T.term, and the no-screen
// arm of set_termname() that only -T could reach.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "cmdline", W: w}

	span := func(t []byte, name string) (int, int, error) {
		a, z, ok := edit.FindDefinition(t, edit.Blank(t), name)
		if !ok {
			return 0, 0, p.Die("%s() is not defined in this file, and this phase is drawn against its "+
				"extent", name)
		}
		return a, z, nil
	}
	inFunction := func(t []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
		a, z, err := span(t, name)
		if err != nil {
			return nil, err
		}
		seg, err := edit(t[a:z])
		if err != nil {
			return nil, err
		}
		Out := append([]byte(nil), t[:a]...)
		Out = append(Out, seg...)
		return append(Out, t[z:]...), nil
	}

	// ---- 0-1. the option letters, want_argument's block, mainerr_arg_missing
	// and the two rows they indexed went with the command line (argvfront,
	// the reform's D1)
	t := text
	var err error

	// ---- 2. termcapinit() takes no name ----------------------------------
	var dflt string
	tci := func(s []byte) ([]byte, error) {
		folded, err := edit.FoldNever(s, "(?m)"+w122EmptyName, 1)
		if err != nil {
			return nil, p.Die("termcapinit()'s empty-name test would not fold -- %v", err)
		}
		s = folded
		d := w122GivenNone.FindIndex(s)
		if d == nil {
			return nil, p.Die("termcapinit() has no `given none` test, so the compiled default cannot " +
				"be read out of it")
		}
		b := edit.Blank(s)
		o := d[1] + bytes.IndexByte(b[d[1]:], '{')
		c := edit.Match(b, o)
		ass := w122Assign.FindSubmatch(s[o+1 : c])
		if ass == nil {
			return nil, p.Die("the compiled default is not one assignment: %s",
				edit.PyRepr(string(s[o+1:c])))
		}
		dflt = string(ass[1])
		end := c + bytes.IndexByte(s[c:], '\n') + 1
		k := bytes.LastIndexByte(s[:d[0]], '\n') + 1
		Out := append([]byte(nil), s[:k]...)
		s = append(Out, s[end:]...)

		loc := w122TermDecl.FindSubmatchIndex(s)
		if loc == nil {
			return nil, p.Die("termcapinit() does not open with `char_u *term = name;`")
		}
		repl := string(s[loc[2]:loc[3]]) + " =" + dflt + ";"
		Out = append([]byte(nil), s[:loc[0]]...)
		Out = append(Out, repl...)
		s = append(Out, s[loc[1]:]...)
		s = bytes.ReplaceAll(s, []byte("termcapinit(char_u *name)"), []byte("termcapinit(void)"))
		return s, nil
	}
	t, err = inFunction(t, "termcapinit", tci)
	if err != nil {
		return nil, err
	}
	t = w122TciProto.ReplaceAll(t, []byte("static void termcapinit(void);"))
	if bytes.Count(t, []byte("termcapinit(params.term);")) != 1 {
		return nil, p.Die("termcapinit() is not called with the field this phase just removed")
	}
	t = bytes.ReplaceAll(t, []byte("termcapinit(params.term);"), []byte("termcapinit();"))
	if w122Mentions(t, "name") > 0 && bytes.Contains(t, []byte("termcapinit(char_u")) {
		return nil, p.Die("termcapinit() still takes a name")
	}
	p.Sayf("termcapinit() takes no name -- nothing could assign the field it was handed -- "+
		"and the compiled default it substituted when it was given none, %s, is its "+
		"initialiser now.  That is internal/phase/096/edit.go's ui_write(console) again",
		strings.TrimSpace(dflt))

	// ---- 3. mparm_T loses the field nothing assigns ----------------------
	// THE PARTITION, and it is why this is the edit's: every `.term`/`->term`
	// belongs either to this struct -- and those mentions have just gone --
	// or to another struct with a member of the same name, which is exactly
	// what deadfields.py cannot tell apart, because it matches by NAME.
	iM := bytes.Index(t, []byte("} mparm_T;"))
	if iM < 0 {
		return nil, p.Die("mparm_T's definition is not balanced")
	}
	oM := edit.RMatch(edit.Blank(t), bytes.LastIndexByte(t[:iM+1], '}'))
	if oM < 0 {
		return nil, p.Die("mparm_T's definition is not balanced")
	}
	mem := w122Member.FindIndex(t[oM:iM])
	if mem == nil {
		return nil, p.Die("mparm_T has no `char_u *term;` member to remove")
	}
	Out := append([]byte(nil), t[:oM+mem[0]]...)
	t = append(Out, t[oM+mem[1]:]...)
	var owners []string
	for _, m := range w122Owner.FindAllSubmatch(edit.Blank(t), -1) {
		if !edit.ContainsStr(owners, string(m[1])) {
			owners = append(owners, string(m[1]))
		}
	}
	sort.Strings(owners)
	if len(owners) == 0 {
		return nil, p.Die("nothing in the file names a `.term` member at all, so the partition below " +
			"says nothing -- read the file before removing this")
	}
	var mine []string
	for _, x := range owners {
		if x == "params" || x == "parmp" {
			mine = append(mine, x)
		}
	}
	if len(mine) > 0 {
		return nil, p.Die("%s still names this struct's `term` field", strings.Join(mine, " "))
	}
	p.Sayf("mparm_T loses its `term` member, in the EDIT: every one of the %d `.term` "+
		"mentions left belongs to another struct (%s), and deadfields.py matches by "+
		"NAME, so that tool could never see this one dead",
		len(w122DotTerm.FindAll(edit.Blank(t), -1)), strings.Join(owners, " "))

	// ---- 4. set_termname()'s no-screen arm cannot run --------------------
	// THE ARGUMENT, computed in three parts before a line is cut.
	dA, dZ, err := span(t, "set_termname")
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, m := range w122SetTerm.FindAllIndex(edit.Blank(t), -1) {
		at := m[0]
		if dA <= at && at < dZ {
			continue
		}
		if strings.TrimSpace(string(t[bytes.LastIndexByte(t[:at], '\n')+1:at])) == "static int" {
			continue
		}
		lo := bytes.LastIndex(t[:at], []byte("\n    static "))
		who := "?"
		if lo >= 0 {
			if f := w122FnName.Find(t[lo:at]); f != nil {
				who = strings.TrimSpace(string(f))
			}
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
		return nil, p.Die("set_termname() is called from %s, and this phase is written against the "+
			"two -- termcapinit(), before there is a screen, and did_set_term(), at run "+
			"time", j)
	}
	tabStart := bytes.Index(t, []byte("static builtin_tcap_T builtin_terminals[] =\n{"))
	tabEnd := tabStart + bytes.Index(t[tabStart:], []byte("\n};"))
	var tabRows []string
	for _, m := range w122Row.FindAllSubmatch(t[tabStart:tabEnd], -1) {
		tabRows = append(tabRows, string(m[1]))
	}
	dn := w122FirstLit.FindStringSubmatch(dflt)
	if dn == nil || !edit.ContainsStr(tabRows, dn[1]) {
		name := ""
		if dn != nil {
			name = dn[1]
		}
		return nil, p.Die("the compiled default %s is not a row of builtin_terminals[], so "+
			"termcapinit() can still be refused and the arm below is live", edit.PyRepr(name))
	}
	defaultName := dn[1]
	var assigns []string
	for _, m := range w122Starting.FindAllSubmatch(t, -1) {
		v := strings.TrimSpace(string(m[1]))
		if !edit.ContainsStr(assigns, v) {
			assigns = append(assigns, v)
		}
	}
	sort.Strings(assigns)
	if edit.ContainsStr(assigns, "NO_SCREEN") {
		return nil, p.Die("something assigns starting = NO_SCREEN, so `starting != NO_SCREEN` is not "+
			"true wherever the arm below is reached: %s", strings.Join(assigns, " "))
	}
	p.Sayf("set_termname() is called from %s and from nowhere else; termcapinit() now "+
		"passes %s, which IS a row of builtin_terminals[]; and the only assignments to "+
		"`starting` are %s -- so a refusal can only come from did_set_term(), where "+
		"`starting != NO_SCREEN`",
		strings.Join(sites, " and "), edit.PyRepr(defaultName), strings.Join(assigns, " and "))

	var tail string
	stn := func(s []byte) ([]byte, error) {
		b := edit.Blank(s)
		m := w122TermpNull.FindIndex(s)
		if m == nil {
			return nil, p.Die("set_termname() has no `termp == nullptr` arm")
		}
		o := m[1] + bytes.IndexByte(b[m[1]:], '{')
		c := edit.Match(b, o)
		if c < 0 {
			return nil, p.Die("the refusal arm is not balanced")
		}
		Inner, err := edit.FoldAlways(s[o+1:c], "(?m)"+w122NoScreen, 1)
		if err != nil {
			return nil, p.Die("the no-screen test would not fold -- %v", err)
		}
		k := bytes.Index(Inner, []byte("return FAIL;\n")) + len("return FAIL;\n")
		tail = string(Inner[k:])
		if strings.TrimSpace(tail) == "" {
			return nil, p.Die("nothing follows the refusal, so this phase has already been applied or " +
				"the arm is not the one it was written against")
		}
		Out := append([]byte(nil), s[:o+1]...)
		Out = append(Out, Inner[:k]...)
		return append(Out, s[c:]...), nil
	}
	sA, sZ, err := span(t, "set_termname")
	if err != nil {
		return nil, err
	}
	seg, err := stn(t[sA:sZ])
	if err != nil {
		return nil, err
	}
	Out = append([]byte(nil), t[:sA]...)
	Out = append(Out, seg...)
	t = append(Out, t[sZ:]...)
	p.Sayf("the no-screen test folds ALWAYS, and what followed the refusal is unreachable "+
		"and goes: %s", strings.Join(strings.Fields(tail), " "))

	// The name the fallback promised, read Out of the text just deleted.
	nm := w122FirstLit.FindStringSubmatch(tail)
	if nm == nil || !edit.ContainsStr(tabRows, nm[1]) {
		return nil, p.Die("the deleted fallback does not name a row of builtin_terminals[], so the " +
			"message below cannot be kept in step with it")
	}
	promised := nm[1]
	if !strings.Contains(tail, "report_default_term") {
		return nil, p.Die("the deleted text does not call report_default_term(), which this phase " +
			"leaves for the sweep -- read the arm before removing this")
	}

	message := func(s []byte) ([]byte, error) {
		re := regexp.MustCompile(`,[^"]*'` + regexp.QuoteMeta(promised) + `'`)
		o := re.ReplaceAll(s, nil)
		if bytes.Equal(o, s) {
			return nil, p.Die("report_term_error() does not promise %s, so there is nothing here to "+
				"keep in step with the fallback", edit.PyRepr(promised))
		}
		var left []string
		for _, x := range tabRows {
			if bytes.Contains(o, []byte("'"+x+"'")) {
				left = append(left, x)
			}
		}
		if len(left) > 0 {
			return nil, p.Die("report_term_error() still names %s after the cut", strings.Join(left, " "))
		}
		return o, nil
	}
	t, err = inFunction(t, "report_term_error", message)
	if err != nil {
		return nil, err
	}
	p.Sayf("report_term_error() stops promising %s: phase 121 moved the message and the "+
		"fallback together because nothing in the build checks that a message tells the "+
		"truth, and this is that rule with no fallback left to name", edit.PyRepr(promised))

	// ---- 5. `requested` is `term` for the one test that reads it ---------
	requested := func(s []byte) ([]byte, error) {
		if k := len(w122Requested.FindAll(edit.Blank(s), -1)); k != 2 {
			return nil, p.Die("`requested` has %d mentions in set_termname(), and this phase is "+
				"written against two -- its declaration and the 256-colour test", k)
		}
		var rew []string
		for _, m := range w122TermRewr.FindAllSubmatch(s, -1) {
			v := strings.TrimSpace(string(m[1]))
			if !edit.ContainsStr(rew, v) {
				rew = append(rew, v)
			}
		}
		sort.Strings(rew)
		if strings.Join(rew, "\x00") != "+= 8" {
			j := strings.Join(rew, " / ")
			if j == "" {
				j = "nothing"
			}
			return nil, p.Die("`term` is rewritten as %s inside set_termname(), and `requested` can "+
				"only be folded into it while the prefix strip is the only one", j)
		}
		bA, bZ, err := span(t, "term_is_builtin")
		if err != nil {
			return nil, err
		}
		pre := w122Prefix.FindSubmatch(t[bA:bZ])
		if pre == nil {
			return nil, p.Die("term_is_builtin() does not strip a counted literal prefix, so what " +
				"`term += 8` skips cannot be read off the file")
		}
		if n, _ := strconv.Atoi(string(pre[2])); len(pre[1]) != n {
			return nil, p.Die("term_is_builtin() does not strip a counted literal prefix, so what " +
				"`term += 8` skips cannot be read off the file")
		}
		nd := w122Needle.FindSubmatch(s)
		if nd == nil {
			return nil, p.Die("the 256-colour test is not a musl_strstr on `requested`")
		}
		if bytes.IndexByte(pre[1], nd[1][0]) >= 0 {
			return nil, p.Die("%s begins with a character the stripped prefix %s contains, so a match "+
				"could start inside the prefix and `requested` is NOT `term` here",
				edit.PyRepr(string(nd[1])), edit.PyRepr(string(pre[1])))
		}
		s = bytes.ReplaceAll(s,
			[]byte(`musl_strstr((char *)requested, "`+string(nd[1])+`")`),
			[]byte(`musl_strstr((char *)term, "`+string(nd[1])+`")`))
		if !w122ReqDecl.Match(s) {
			return nil, p.Die("`requested` is not declared as `= term`")
		}
		p.Sayf("`requested` goes: it existed because the fallback reassigned `term`, and "+
			"the only rewrite left is the %s that strips %s -- %s cannot match inside "+
			"that, because it begins with a character the prefix does not hold",
			rew[0], edit.PyRepr(string(pre[1])), edit.PyRepr(string(nd[1])))
		return s, nil
	}
	rA, rZ, err := span(t, "set_termname")
	if err != nil {
		return nil, err
	}
	seg, err = requested(t[rA:rZ])
	if err != nil {
		return nil, err
	}
	Out = append([]byte(nil), t[:rA]...)
	Out = append(Out, seg...)
	t = append(Out, t[rZ:]...)

	// ---- 6. what is left, as a partition ---------------------------------
	goneNames := []string{"mainerr_arg_missing", "ME_GARBAGE", "ME_ARG_MISSING"}
	for _, name := range goneNames {
		if k := w122Mentions(t, name); k != 0 {
			return nil, p.Die("%s still has %d mentions", name, k)
		}
	}
	// requested is left declared and read by nothing, which is what the
	// sweep takes.
	for _, name := range []string{"requested"} {
		if k := w122Mentions(t, name); k != 1 || !w122DeclOnly[name].Match(t) {
			return nil, p.Die("%s still has %d mentions, beyond its declaration", name, k-1)
		}
	}
	for _, name := range []string{"ME_UNKNOWN_OPTION", "ME_EXTRA_CMD", "MAX_ARG_CMDS",
		"exe_commands", "p_paste", "did_set_term", "report_term_error"} {
		if w122Mentions(t, name) == 0 {
			return nil, p.Die("%s went, and it is not this phase's", name)
		}
	}
	if k := w122Mentions(t, "report_default_term"); k != 1 {
		return nil, p.Die("report_default_term has %d mentions, and this phase leaves it at one -- "+
			"its own definition, which is what the sweep takes", k)
	}
	p.Sayf("0 mentions of %s; requested and report_default_term are down to their declarations and are the "+
		"sweep's; ME_UNKNOWN_OPTION, ME_EXTRA_CMD, MAX_ARG_CMDS, exe_commands and "+
		"'paste' are untouched", strings.Join(goneNames, ", "))
	return t, nil
}
