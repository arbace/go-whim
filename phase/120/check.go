package p120

// Whim phase 120, the check -- the degenerate unions go.
// See phase/120/edit.go, and GOALS.md.
//
// Runs after phase/120/edit.go and the sweep internal/verify runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// THIS PHASE CHANGES NO STATEMENT AND NO LAYOUT, so there is no behavioural probe to
// offer and none is offered.  What it has instead is stronger than any recording: THE
// BINARY IS THE SAME BYTES.  That is tier 1 of CLAUDE.md's verification table, and it
// subsumes every screen case, every Ex-command row, every command line and every pty
// scenario at once, because the program that would be run is literally the same program.
// It is phase 99's and phase 106's kind of empty declaration -- THE STRONGEST OF THE FIVE
// and not the weakest -- and this check says which kind it is rather than leaving a
// reader to guess.
//
// WHAT IS CLAIMED, in eight parts:
//
// ARITHMETIC  the whole of it is RECOMPUTED FROM THE INPUT by the edit's own scanner,
// run again here: the input's thirteen unions are classified by member
// count, the six with fewer than two members must be gone and the seven
// with two or more must survive BYTE FOR BYTE, each degenerate name must
// keep its declaration and every `.member` access on it must be a plain
// field reference.  Not one of those numbers is written in this file, which
// is the lesson phase 118 was taught when phase 117 moved its counted
// anchors.
// THE DIALECT the empty union's own argument, MEASURED.  `union { } es_info;` is a GNU
// C extension: gcc reports `union has no members [-Wpedantic]` on the input
// exactly once and on the output not at all, with the rest of the pedantic
// diagnostic set unmoved, and a minimal probe shows the same construct is a
// hard ERROR under `-pedantic-errors` while its non-empty twin is silent.
// SYMBOLS     `nm -u` is THE SAME SET, as a `comm` empty in BOTH directions, and `main`
// is still the only external symbol.  Deleting a wrapper type inside one
// translation unit cannot move either, and a symbol ARRIVING must fail as
// loudly as one leaving.
// THE CUT     `awk '/^ *# *include / { exit }'`, whim.mk's own rule, on the INPUT and on
// the OUTPUT: 0 directives and 0 errors under `-fsyntax-only` either side,
// and the warning set -- which IS the core -> host interface -- IDENTICAL,
// computed at run time from the input and never written down.
// THE BINARY  `cmp` of the input's binary and the output's, both built with
// SOURCE_DATE_EPOCH=0 and the boundary's own flags.  This is the whole
// evidence.
// THE CONTROL and it is the point.  c1 is this phase's own output with the two fields it
// PROMOTED -- `uh_next` and `uh_prev` -- exchanged: a pure layout
// permutation of the very struct this phase rewrites, which compiles and
// must give a DIFFERENT binary.  Measured: 31,038 bytes differ.  Without it
// the `cmp` above is a pair of numbers agreeing, and CLAUDE.md is explicit
// that a test that cannot fail is not evidence.
// TWO THAT MOVE NOTHING, REPORTED RATHER THAN DROPPED.  c2 puts the EMPTY union back
// and c3 puts one SINGLE-MEMBER union back with its `.member` accesses --
// this phase run backwards on one field each.  Both must be byte-identical,
// which is the phase's own claim stated in the other direction: a union of
// one member and a plain field are the same program, and an empty union is
// no program at all.  If either ever moves, this phase's account of itself
// has to be rewritten rather than the number quietly updated.
// STRUCTURE   tools/canon.sh is a no-op on the output, and `zhostonly` -- phase
// 103's structural check -- still passes.
//
// AND TWO FULL RECORDINGS, WHICH ARE A CHECK ON THE HARNESS AND NOT ON THE PHASE.  With a
// byte-identical binary a `tools/st.sh zrecord` of each side compares a program with itself,
// so an empty `diff -r` says the instrument is deterministic and says nothing about the
// edit.  They are run because it is cheaper to measure that than to assert it, and this
// comment is what keeps them from being read as the evidence.  The evidence is the `cmp`.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim120", Check) }

var (
	z37UnionC  = regexp.MustCompile(`\bunion\b`)
	z37TailC   = regexp.MustCompile(`^([ \t]*)([A-Za-z_]\w*)[ \t]*;`)
	z37Member  = regexp.MustCompile(`(?s)^(.*?)([A-Za-z_]\w*)[ \t]*;$`)
	z37DirAny  = regexp.MustCompile(`^ *#`)
	z37IncLine = regexp.MustCompile(`^ *# *include <[A-Za-z0-9_/.]+>$`)
)

type z37U struct {
	Name        string
	members     int
	indent      string
	start, line int
	Body, Text  string
}

// z37Scan is the heredoc's scan(): every `union { ... } name;`, braces
// matched and members counted at depth 1.  A malformed one is an error
// carrying the heredoc's message.
func z37Scan(text, which string) ([]z37U, string) {
	var found []z37U
	for _, loc := range z37UnionC.FindAllStringIndex(text, -1) {
		j := loc[1]
		for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n') {
			j++
		}
		line := strings.Count(text[:loc[0]], "\n") + 1
		if j >= len(text) || text[j] != '{' {
			return nil, fmt.Sprintf("the `union` at line %d of the %s is not followed by a brace", line, which)
		}
		depth, k, members := 0, j, 0
		for ; k < len(text); k++ {
			c := text[k]
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					break
				}
			} else if c == ';' && depth == 1 {
				members++
			}
		}
		if k+1 > len(text) {
			return nil, fmt.Sprintf("the union at line %d of the %s does not end `} <name>;`", line, which)
		}
		tm := z37TailC.FindStringSubmatchIndex(text[k+1:])
		if tm == nil {
			return nil, fmt.Sprintf("the union at line %d of the %s does not end `} <name>;`", line, which)
		}
		start := strings.LastIndex(text[:loc[0]], "\n") + 1
		end := k + 1 + tm[1] + 1
		if end > len(text) {
			end = len(text)
		}
		found = append(found, z37U{
			Name: text[k+1+tm[4] : k+1+tm[5]], members: members, indent: text[start:loc[0]],
			start: start, line: line, Body: text[j+1 : k], Text: text[start:end],
		})
	}
	return found, ""
}

// Whim120 is phase 120's check: the degenerate unions.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim120 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	oldC := filepath.Join(state, "old.c")
	r := &check.Rep{Tag: "unions", W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	raw := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	head := func(ls []string, n int, prefix string) {
		for i, l := range ls {
			if i >= n {
				break
			}
			fmt.Fprintf(w, "%s%s\n", prefix, l)
		}
	}
	fileLines := func(p string) []string {
		s := strings.TrimRight(check.ReadFile(p), "\n")
		if s == "" {
			return nil
		}
		return strings.Split(s, "\n")
	}
	beforeRaw := strings.TrimRight(check.ReadFile(filepath.Join(state, "input-lines")), "\n")
	tmp, err := os.MkdirTemp("", "whim120-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	T := func(n string) string { return filepath.Join(tmp, n) }
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflagsS, ldflagsS := "", ""
	if m := check.Z29CFlags.FindStringSubmatch(mk); m != nil {
		cflagsS = m[1]
	}
	if m := check.Z29LDFlags.FindStringSubmatch(mk); m != nil {
		ldflagsS = m[1]
	}
	link := func(src, Out, logPath string) error {
		a := append(append(strings.Fields(cflagsS), strings.Fields(ldflagsS)...), "-o", Out, src)
		c := exec.Command("gcc", a...)
		c.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
		if logPath != "" {
			lf, _ := os.Create(logPath)
			defer lf.Close()
			c.Stderr = lf
		}
		return c.Run()
	}
	var wgAll sync.WaitGroup
	defer wgAll.Wait()
	type job struct {
		wg  sync.WaitGroup
		Err error
	}
	start := func(fn func() error) *job {
		j := &job{}
		j.wg.Add(1)
		wgAll.Add(1)
		go func() { defer wgAll.Done(); defer j.wg.Done(); j.Err = fn() }()
		return j
	}
	jNew := start(func() error { return link(f, T("new"), "") })

	// --- 0. the three controls, built from the input's own text -------------------
	newT, oldT := check.ReadFile(f), check.ReadFile(oldC)
	IN, msg := z37Scan(oldT, "input")
	if msg != "" {
		return stop("%s", msg)
	}
	var deg, gen []z37U
	for _, u := range IN {
		if u.members < 2 {
			deg = append(deg, u)
		} else {
			gen = append(gen, u)
		}
	}
	if len(deg) < 2 || len(gen) == 0 {
		return stop("the input has %d degenerate unions and %d genuine ones, and this check needs at "+
			"least two of the first and one of the second -- a scanner that stopped matching "+
			"would pass by finding nothing", len(deg), len(gen))
	}
	type prom struct{ Name, decl string }
	var promoted []prom
	for _, u := range deg {
		if u.members == 0 {
			continue
		}
		mm := z37Member.FindStringSubmatch(strings.TrimSpace(u.Body))
		if mm == nil {
			return stop("the single member of `%s` is not one `<type> <name>;`", u.Name)
		}
		promoted = append(promoted, prom{u.Name, u.indent + mm[1] + u.Name + ";\n"})
	}
	var pair *[4]string
	for i := 0; i+1 < len(promoted); i++ {
		a, b := promoted[i], promoted[i+1]
		if strings.Count(newT, a.decl+b.decl) == 1 {
			pair = &[4]string{a.Name, b.Name, a.decl, b.decl}
			break
		}
	}
	if pair == nil {
		return stop("no two promoted declarations are adjacent lines in the output, so the layout " +
			"control cannot be built out of this phase's own subject")
	}
	c1 := strings.Replace(newT, pair[2]+pair[3], pair[3]+pair[2], 1)
	var empty []z37U
	for _, u := range deg {
		if u.members == 0 {
			empty = append(empty, u)
		}
	}
	if len(empty) != 1 {
		return stop("the input has %d empty unions and this check was written for one", len(empty))
	}
	e := empty[0]
	aStart := strings.LastIndex(oldT[:e.start-1], "\n") + 1
	above := oldT[aStart:e.start]
	if strings.Count(newT, above) != 1 || strings.Count(oldT, above+e.Text) != 1 {
		return stop("the line above the empty union is not a unique anchor in both texts, so c2 " +
			"cannot be put back where it was")
	}
	c2 := strings.Replace(newT, above, above+e.Text, 1)
	var single []z37U
	for _, u := range deg {
		if u.members > 0 {
			single = append(single, u)
		}
	}
	s := single[len(single)-1]
	mm := z37Member.FindStringSubmatch(strings.TrimSpace(s.Body))
	if mm == nil {
		return stop("the single member of `%s` is not one `<type> <name>;`", s.Name)
	}
	member := mm[2]
	decl := s.indent + mm[1] + s.Name + ";\n"
	if strings.Count(newT, decl) != 1 {
		return stop("`%s` is not declared exactly once in the output as the promoted field", s.Name)
	}
	i := strings.Index(newT, decl)
	reName := regexp.MustCompile(`\b` + s.Name + `\b`)
	c3 := reName.ReplaceAllLiteralString(newT[:i], s.Name+"."+member) + s.Text +
		reName.ReplaceAllLiteralString(newT[i+len(decl):], s.Name+"."+member)
	for _, x := range []struct{ Name, Text string }{{"c1", c1}, {"c2", c2}, {"c3", c3}} {
		if x.Text == newT {
			return stop("%s changed nothing, so it would not be a control", x.Name)
		}
		os.WriteFile(T(x.Name+".c"), []byte(x.Text), 0o644)
	}
	r.Say("three controls written, every one of them built from the input's own "+
		"text and not from C quoted in the check: c1 the two fields this phase PROMOTED "+
		"(`%s` and `%s`) EXCHANGED -- a pure layout permutation of the struct it rewrites, "+
		"which must move the binary; c2 the empty union `%s` put back; c3 the "+
		"single-member union `%s` put back with its %d `.%s` accesses -- this phase run "+
		"backwards on one field", pair[0], pair[1], e.Name, s.Name,
		len(reName.FindAllStringIndex(newT, -1))-1, member)
	var ctlJobs []*job
	for _, c := range []string{"c1", "c2", "c3"} {
		c := c
		ctlJobs = append(ctlJobs, start(func() error { return link(T(c+".c"), T(c), T(c+".log")) }))
	}
	ped := func(src, Out string) *job {
		return start(func() error {
			a := append(strings.Fields(cflagsS), "-fsyntax-only", "-Wpedantic", src)
			c := exec.Command("gcc", a...)
			lf, _ := os.Create(Out)
			defer lf.Close()
			c.Stderr = lf
			c.Run()
			return nil
		})
	}
	jPO := ped(oldC, T("ped-old.txt"))
	jPN := ped(f, T("ped-new.txt"))
	os.WriteFile(T("canon.c"), []byte(newT), 0o644)
	var canonLog []byte
	jCanon := start(func() error {
		var e error
		canonLog, e = exec.Command("sh", "tools/canon.sh", T("canon.c")).CombinedOutput()
		return e
	})

	// --- 1. the source, as arithmetic on the input ----------------------------------
	beforeLines, _ := strconv.Atoi(strings.TrimSpace(beforeRaw))
	NL, OL := strings.Split(newT, "\n"), strings.Split(oldT, "\n")
	halves := func(lines []string, which string) (int, int, error) {
		var d []int
		for i, l := range lines {
			if z37DirAny.MatchString(l) {
				d = append(d, i)
			}
		}
		ok := len(d) > 0
		for k, i := range d {
			if ok && (i != d[0]+k || !z37IncLine.MatchString(lines[i])) {
				ok = false
			}
		}
		if !ok {
			return 0, 0, stop("the `#include` directives are not a run of consecutive lines "+
				"in the %s -- the first of them IS the boundary and nothing else marks "+
				"it", which)
		}
		return d[0], len(d), nil
	}
	obound, ondir, e2 := halves(OL, "input")
	if e2 != nil {
		return e2
	}
	nbound, nndir, e2 := halves(NL, "output")
	if e2 != nil {
		return e2
	}
	IN, msg = z37Scan(oldT, "input")
	if msg != "" {
		return stop("%s", msg)
	}
	OUT, msg := z37Scan(newT, "output")
	if msg != "" {
		return stop("%s", msg)
	}
	deg, gen = nil, nil
	for _, u := range IN {
		if u.members < 2 {
			deg = append(deg, u)
		} else {
			gen = append(gen, u)
		}
	}
	if len(deg) == 0 || len(gen) == 0 {
		r.Bad("the input has %d degenerate unions and %d genuine ones, and this phase "+
			"needs both kinds to exist -- a scanner that stopped matching would pass "+
			"by finding nothing", len(deg), len(gen))
	}
	for _, u := range IN {
		if u.line > obound {
			r.Bad("a union is defined below the boundary, in the host, and this phase is " +
				"about the CORE")
			break
		}
	}
	for _, u := range gen {
		if strings.Count(newT, u.Text) != 1 || strings.Count(oldT, u.Text) != 1 {
			r.Bad("the genuine union `%s` (%d members) does not occur exactly once in "+
				"both texts: %d in, %d out -- a union with two or more members is "+
				"doing the job a union is for and this phase must not touch it",
				u.Name, u.members, strings.Count(oldT, u.Text), strings.Count(newT, u.Text))
		}
	}
	names := func(us []z37U) string {
		var ns []string
		for _, u := range us {
			ns = append(ns, u.Name)
		}
		return strings.Join(ns, " ")
	}
	if names(OUT) != names(gen) {
		o := names(OUT)
		if o == "" {
			o = "none"
		}
		r.Bad("the output's unions are %s and the input's genuine ones are %s", o, names(gen))
	}
	nKeyIn := len(z37UnionC.FindAllStringIndex(oldT, -1))
	nKeyOut := len(z37UnionC.FindAllStringIndex(newT, -1))
	if nKeyIn != len(IN) || nKeyOut != len(gen) {
		r.Bad("`union` is %d keywords in the input and %d in the output, and the scan "+
			"found %d unions in and %d genuine -- every keyword must be one of the "+
			"definitions the scan classified", nKeyIn, nKeyOut, len(IN), len(gen))
	}
	cnt := func(pat, text string) int { return len(regexp.MustCompile(pat).FindAllStringIndex(text, -1)) }
	linesGone := 0
	for _, u := range deg {
		nIn := cnt(`\b`+u.Name+`\b`, oldT)
		nOut := cnt(`\b`+u.Name+`\b`, newT)
		if u.members > 0 {
			mm := z37Member.FindStringSubmatch(strings.TrimSpace(u.Body))
			if mm == nil {
				r.Bad("the single member of `%s` is not one `<type> <name>;`", u.Name)
				continue
			}
			mem, dcl := mm[2], u.indent+mm[1]+u.Name+";"
			acc := cnt(`\b`+u.Name+`\.`+mem+`\b`, oldT)
			if acc != nIn-1 {
				r.Bad("`%s` has %d mentions in the INPUT of which %d are `.%s` "+
					"accesses, and every mention but its own declaration must be one "+
					"-- the partition is what makes the rewrite safe", u.Name, nIn, acc, mem)
			} else if nOut != nIn {
				r.Bad("`%s` has %d mentions in the output and %d in the input, and the "+
					"two must be EQUAL: the union's own name survives on the "+
					"promoted declaration and on every one of the %d accesses, which "+
					"are now plain field references.  What goes is the MEMBER's "+
					"name, `%s`, and that is asserted below as the access shape "+
					"rather than as a count -- other structs in this file have a "+
					"member of the same name", u.Name, nOut, nIn, acc, mem)
			}
			if strings.Count(newT, dcl+"\n") != 1 {
				r.Bad("`%s` is not declared exactly once in the output as `%s`, which "+
					"is its member's OWN declaration with the union's name -- the "+
					"type, the stars and the spacing are the input's", u.Name, strings.TrimSpace(dcl))
			}
			if regexp.MustCompile(`\b` + u.Name + `\s*\.\s*` + mem + `\b`).MatchString(newT) {
				r.Bad("a `%s.%s` access survives in the output", u.Name, mem)
			}
		} else {
			if nIn != 1 {
				r.Bad("the empty union `%s` has %d mentions in the input, and an empty "+
					"union has no member to access, so its declaration is the only "+
					"one there can be", u.Name, nIn)
			}
			if nOut > 0 {
				r.Bad("`%s` still has %d mentions in the output", u.Name, nOut)
			}
		}
		sub := 0
		if u.members > 0 {
			sub = 1
		}
		linesGone += strings.Count(u.Text, "\n") - sub
	}
	if len(OL)-1 != beforeLines {
		r.Bad("the input is %d lines and the driver recorded %d", len(OL)-1, beforeLines)
	}
	if len(NL)-1 != beforeLines-linesGone {
		r.Bad("the file is %d lines and the input was %d -- the six declarations are "+
			"%d lines shorter between them", len(NL)-1, len(OL)-1, linesGone)
	}
	if nndir != ondir {
		r.Bad("the file has %d directives and had %d: this phase adds none and removes "+
			"none", nndir, ondir)
	}
	if obound-nbound != linesGone {
		r.Bad("the boundary moved by %d lines and the file by %d: every line this "+
			"phase touches is above the first `#include`", obound-nbound, linesGone)
	}
	// tools/create_cmdidxs.py -- named as a PATH so tools/implhash.sh hashes
	// it into this phase's key.  Do not delete it.
	nOld := len(check.Z35CmdRow.FindAllString(oldT, -1))
	nNew := len(check.Z35CmdRow.FindAllString(newT, -1))
	cn, _ := harness.CommandNames(f)
	if nNew != nOld || len(cn) != nOld {
		r.Bad("cmdnames[] is %d rows and the input had %d -- this phase touches no Ex "+
			"command", nNew, nOld)
	}
	if check.Z29RowCount(newT) != check.Z29RowCount(oldT) {
		r.Bad("options[] has %d rows and the input had %d -- this phase removes no "+
			"option", check.Z29RowCount(newT), check.Z29RowCount(oldT))
	}
	if check.Z27Runs(NL) > 0 {
		r.Bad("there is a run of two blank lines, which canon.sh should have taken")
	}
	if err := r.Done(); err != nil {
		return err
	}
	var dd, gg, pp []string
	for _, u := range deg {
		k := "1 member"
		if u.members == 0 {
			k = "empty"
		}
		dd = append(dd, fmt.Sprintf("%s (%s)", u.Name, k))
		pp = append(pp, fmt.Sprintf("%s 1 + %d", u.Name, cnt(`\b`+u.Name+`\b`, oldT)-1))
	}
	for _, u := range gen {
		gg = append(gg, fmt.Sprintf("%s (%d)", u.Name, u.members))
	}
	r.Say("`union` %d -> %d, AND WHICH SIX GO IS COMPUTED AND NOT LISTED: the input "+
		"is scanned, the braces matched and the members counted at depth 1, and a union "+
		"with fewer than two is degenerate.  THE %d THAT GO: %s.  THE %d THAT STAY ARE "+
		"DOING THE JOB A UNION IS FOR and their text occurs once in BOTH files, byte for "+
		"byte: %s", nKeyIn, nKeyOut, len(deg), strings.Join(dd, ", "), len(gen), strings.Join(gg, ", "))
	r.Cont("EACH OF THE SIX IS A PARTITION OF ITS OWN MENTIONS IN THE INPUT -- the "+
		"declaration and `.member` accesses, with nothing left over: %s.  Every access is "+
		"now a plain field reference and not one survives; each promoted field is declared "+
		"by its MEMBER'S own declaration carrying the UNION'S name, so the type, the "+
		"stars and the internal spacing are the input's and not the phase's", strings.Join(pp, ", "))
	r.Cont("%d -> %d lines, %d fewer and every one of them above the boundary, which "+
		"moved by the same %d; %d directives unmoved, cmdnames[] %d and options[] %d "+
		"unchanged, and no run of two blank lines",
		beforeLines, len(NL)-1, linesGone, linesGone, nndir, nNew, check.Z29RowCount(newT))

	// --- 2. the empty union's own argument, measured --------------------------------
	os.WriteFile(T("iso-empty.c"), []byte("typedef struct { long a; union { } b; } S;\nS s;\nint main(void) { return (int)sizeof(S) + (int)s.a; }\n"), 0o644)
	os.WriteFile(T("iso-full.c"), []byte("typedef struct { long a; union { int c; } b; } S;\nS s;\nint main(void) { return (int)sizeof(S) + (int)s.a; }\n"), 0o644)
	iso := func(src, logp string) error {
		c := exec.Command("gcc", "-pedantic-errors", "-fsyntax-only", src)
		lf, _ := os.Create(logp)
		defer lf.Close()
		c.Stderr = lf
		return c.Run()
	}
	if iso(T("iso-empty.c"), T("iso-empty.log")) == nil {
		r.Say("gcc ACCEPTS an empty union under -pedantic-errors, so this phase's")
		raw("               dialect argument for es_info does not hold and the sentence has to")
		raw("               be rewritten rather than the check relaxed.")
		return harness.ErrReported
	}
	if !strings.Contains(check.ReadFile(T("iso-empty.log")), "union has no members") {
		r.Say("the ISO probe failed for some other reason than the empty union:")
		head(fileLines(T("iso-empty.log")), 3, "               ")
		return harness.ErrReported
	}
	if iso(T("iso-full.c"), T("iso-full.log")) != nil {
		r.Say("the control probe -- the SAME struct with a one-member union -- does")
		raw("               not compile under -pedantic-errors either, so the probe above says")
		raw("               nothing about emptiness:")
		head(fileLines(T("iso-full.log")), 3, "               ")
		return harness.ErrReported
	}
	jPO.wg.Wait()
	jPN.wg.Wait()
	countL := func(p string, pred func(string) bool) int {
		n := 0
		for _, l := range strings.Split(check.ReadFile(p), "\n") {
			if pred(l) {
				n++
			}
		}
		return n
	}
	noMem := func(l string) bool { return strings.Contains(l, "has no members") }
	diag := func(l string) bool { return strings.Contains(l, "warning:") || strings.Contains(l, "error:") }
	po, pn := countL(T("ped-old.txt"), noMem), countL(T("ped-new.txt"), noMem)
	to, tn := countL(T("ped-old.txt"), diag), countL(T("ped-new.txt"), diag)
	if po != 1 || pn != 0 || to-tn != 1 {
		r.Say("THE PEDANTIC MEASUREMENT DID NOT COME OUT AS THE PHASE CLAIMS.")
		raw("               'has no members': %d in the input, %d in the output, and the", po, pn)
		raw("               whole diagnostic set went %d -> %d, a difference of %d", to, tn, to-tn)
		raw("               where it must be exactly the one line this phase removes.")
		return harness.ErrReported
	}
	r.Say("THE EMPTY UNION'S OWN ARGUMENT, MEASURED AND NOT ASSERTED: gcc reports `union has no members "+
		"[-Wpedantic]` %d time on the input and %d on the output, and the rest of the pedantic diagnostic set "+
		"does not move -- %d -> %d, a difference of exactly one.  A minimal probe confirms the construct is a "+
		"HARD ERROR under -pedantic-errors and that the identical struct with a ONE-member union is silent, so "+
		"the probe is proven able to pass in the same run.  GOALS.md's core is meant to be read by "+
		"something that is not gcc, and a construct ISO C forbids is exactly the latent exotic that costs a "+
		"reader later", po, pn, to, tn)

	// --- 3. the libc surface ----------------------------------------------------------
	os.WriteFile(T("before.u"), []byte(check.ReadFile(filepath.Join(state, "symbols", "undefined"))), 0o644)
	pc := exec.Command("sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols"))
	pc.Stdout, pc.Stderr = w, w
	if err := pc.Run(); err != nil {
		return harness.ErrReported
	}
	lastU := ".cache/symbols/last/undefined"
	if !check.Z30Same(T("before.u"), lastU) {
		bu, lu := fileLines(T("before.u")), fileLines(lastU)
		r.Say("the libc surface moved, and DELETING A WRAPPER TYPE CANNOT MOVE IT:")
		raw("               gone: %s", check.Z31Words(check.Comm23(bu, lu)))
		raw("               came: %s", check.Z31Words(check.Comm23(lu, bu)))
		return harness.ErrReported
	}
	r.Say("symbols %s -> %s, and the set is IDENTICAL as a cmp -- nothing left and nothing arrived; main is "+
		"still the only external symbol", strings.TrimRight(check.ReadFile(".cache/symbols/last/before"), "\n"),
		strings.TrimRight(check.ReadFile(".cache/symbols/last/after"), "\n"))

	// --- 4. the cut, and the core -> host interface it prints ------------------------
	type cut struct {
		lines, errs, warns int
		names              []string
		log                string
	}
	editorcut := func(src, dst string) (cut, error) {
		lines := check.Z28Cut(check.ReadFile(src))
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		os.WriteFile(dst, []byte(text), 0o644)
		for _, l := range lines {
			if check.Z30Dir.MatchString(l) {
				return cut{}, stop("the cut of %s holds a directive, so it found the wrong line", src)
			}
		}
		if len(lines) <= 70000 {
			return cut{}, stop("the cut of %s is %d lines, and whim.mk's floor is 70,000 -- a cut that found "+
				"line 1 would be empty and every check below would pass on nothing", src, len(lines))
		}
		c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-fsyntax-only", dst)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		var cu cut
		cu.lines, cu.log = len(lines), eb.String()
		for _, l := range strings.Split(cu.log, "\n") {
			if strings.Contains(l, "error:") {
				cu.errs++
			}
			if strings.Contains(l, "warning:") {
				cu.warns++
			}
			if m := check.Z35Warn.FindStringSubmatch(l); m != nil {
				cu.names = append(cu.names, m[1])
			}
		}
		sort.Strings(cu.names)
		return cu, nil
	}
	co, e3 := editorcut(oldC, T("cut-old.c"))
	if e3 != nil {
		return e3
	}
	cnw, e3 := editorcut(f, T("cut-new.c"))
	if e3 != nil {
		return e3
	}
	for _, x := range []struct {
		W string
		c cut
	}{{"old", co}, {"new", cnw}} {
		if x.c.errs != 0 {
			r.Say("the %s cut does not parse: %d errors", x.W, x.c.errs)
			var el []string
			for _, l := range strings.Split(x.c.log, "\n") {
				if strings.Contains(l, "error:") {
					el = append(el, l)
				}
			}
			head(el, 3, "               ")
			return harness.ErrReported
		}
		if x.c.warns != len(x.c.names) {
			r.Say("the %s cut has a warning that is not a 'used but never defined':", x.W)
			var wl []string
			for _, l := range strings.Split(x.c.log, "\n") {
				if strings.Contains(l, "warning:") && !strings.Contains(l, "used but never defined") {
					wl = append(wl, l)
				}
			}
			head(wl, 3, "               ")
			return harness.ErrReported
		}
	}
	cutBytes, _ := os.ReadFile(T("cut-new.c"))
	fb, _ := os.ReadFile(f)
	if len(fb) < len(cutBytes) || string(fb[:len(cutBytes)]) != string(cutBytes) {
		return stop("the cut is not a byte prefix of whim-vim.c")
	}
	if strings.Join(co.names, "\n") != strings.Join(cnw.names, "\n") {
		r.Say("THE CUT'S BOUNDARY SET MOVED, AND THIS PHASE CROSSES NO BOUNDARY:")
		raw("               gone: %s", check.Z31Words(check.Comm23(co.names, cnw.names)))
		raw("               came: %s", check.Z31Words(check.Comm23(cnw.names, co.names)))
		return harness.ErrReported
	}
	r.Say("THE CUT -- `make editor.c`'s own rule, and a byte prefix of the file -- is %d lines in and %d out, 0 "+
		"directives and 0 errors under -fsyntax-only either side, and its warning set, which IS the core -> host "+
		"interface, is the SAME %d names as a cmp.  The input's set is computed here and never written down",
		co.lines, cnw.lines, len(cnw.names))
	jCanon.wg.Wait()
	if !check.Z30Same(T("canon.c"), f) {
		r.Say("tools/canon.sh CHANGED THE OUTPUT, and it must be a no-op:")
		head(check.Z30Diff(f, T("canon.c")), 6, "               ")
		return harness.ErrReported
	}
	canonWord := ""
	for _, l := range strings.Split(string(canonLog), "\n") {
		if check.Z35CanonLn.MatchString(l) {
			canonWord = check.Z35CanonLn.ReplaceAllString(l, "")
			break
		}
	}
	r.Say("tools/canon.sh is a NO-OP on the output (%s)", canonWord)

	// --- 5. the binary, which is the whole of this phase's evidence -----------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	bin := filepath.Join(work, "whim-vim")
	if _, e := os.Stat(bin); e == nil {
		(&check.Rep{Tag: "build", W: w}).Say("the clean did not remove whim-vim, so a 'rebuild' below could be no rebuild at all")
		return harness.ErrReported
	}
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes", beforeRaw, check.CountLines(fb), check.SizeOf(bin))
	jNew.wg.Wait()
	if jNew.Err != nil {
		return stop("the reproducible build of the output failed")
	}
	oldBinP := filepath.Join(state, "old")
	oldSize, newSize := check.SizeOf(oldBinP), check.SizeOf(T("new"))
	if newSize < 500000 || oldSize < 500000 {
		return stop("one of the two binaries is %d / %d bytes, which is not an editor -- a cmp of two files "+
			"nothing wrote passes", oldSize, newSize)
	}
	if newSize != check.SizeOf(bin) {
		return stop("the reproducible build is %d bytes and make produced %d: the two differ by more than a "+
			"timestamp, so the comparison below would not be about this boundary", newSize, check.SizeOf(bin))
	}
	if !check.Z30Same(oldBinP, T("new")) {
		r.Say("THE BINARY MOVED.  A union of ONE member has the size, the")
		raw("               alignment and the offset of that member, and an EMPTY union")
		raw("               contributes no storage, so this phase changes no layout and no")
		raw("               code at all and the two binaries -- the input's and the")
		raw("               output's, both built with SOURCE_DATE_EPOCH=0 and the boundary's")
		raw("               own flags -- must be the same bytes.  THIS IS A FINDING AND NOT")
		raw("               A NUMBER TO UPDATE: something about one of these six is not what")
		raw("               the phase says it is.  %d in, %d out.", oldSize, newSize)
		raw("               The GNU build-id note is a hash of the whole image and sits near")
		raw("               the front, so the first difference below is always that note:")
		Out, _ := exec.Command("cmp", oldBinP, T("new")).CombinedOutput()
		head(strings.Split(strings.TrimRight(string(Out), "\n"), "\n"), 1<<30, "               ")
		return harness.ErrReported
	}
	r.Say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side -- tier 1 of CLAUDE.md's verification table, and "+
		"the whole of this phase's evidence.  A byte-identical binary subsumes every screen case, every "+
		"Ex-command row, every command line and every pty scenario at once, because the program that would be "+
		"run is the same program.  This phase's declared delta is NOTHING AT ALL, and that is the STRONGEST of "+
		"the five kinds of empty declaration and not the weakest: it is phase 99's and phase 106's kind", newSize)

	// --- 6. the controls, and the first of them is the point --------------------------
	for _, j := range ctlJobs {
		j.wg.Wait()
	}
	for _, c := range []string{"c1", "c2", "c3"} {
		if fi, e := os.Stat(T(c)); e != nil || fi.Mode()&0o111 == 0 {
			r.Say("the control %s did not build, and every one of them must:", c)
			head(fileLines(T(c+".log")), 5, "               ")
			return harness.ErrReported
		}
	}
	if check.Z30Same(T("new"), T("c1")) {
		r.Say("THE CONTROL c1 DID NOT SHOW.  This phase's own output with the two")
		raw("               fields it PROMOTED exchanged -- a pure layout permutation of the")
		raw("               very struct it rewrites -- gives a binary IDENTICAL to the output's,")
		raw("               so the cmp above is two numbers agreeing and proves nothing about")
		raw("               layout.  A test that cannot fail is not evidence (CLAUDE.md).")
		return harness.ErrReported
	}
	cmpL := func(a, b string) int {
		x, _ := os.ReadFile(a)
		y, _ := os.ReadFile(b)
		n := 0
		for k := 0; k < len(x) && k < len(y); k++ {
			if x[k] != y[k] {
				n++
			}
		}
		return n
	}
	c1Bytes := cmpL(T("new"), T("c1"))
	if c1Bytes < 1000 {
		r.Say("c1 differs in %d bytes, and exchanging two fields of a struct", c1Bytes)
		raw("               the undo layer touches everywhere must move far more than that --")
		raw("               a difference that small is the build-id note and nothing else.")
		return harness.ErrReported
	}
	for _, c := range []string{"c2", "c3"} {
		if !check.Z30Same(T("new"), T(c)) {
			r.Say("THE CONTROL %s MOVED, AND IT IS DECLARED TO MOVE NOTHING.", c)
			raw("               %s is this phase run BACKWARDS on one field: a union of one", c)
			raw("               member put back around a plain field, or an empty union put")
			raw("               back into a struct.  Either must give the same bytes, because")
			raw("               that equivalence is the whole of the phase's argument.  If it")
			raw("               now moves something, the account has to be rewritten rather")
			raw("               than the number quietly updated.  %d bytes differ.", cmpL(T("new"), T(c)))
			return harness.ErrReported
		}
	}
	r.Say("AND IT CAN FAIL: c1 -- this phase's own output with the two fields it PROMOTED EXCHANGED, a pure layout "+
		"permutation of the struct it rewrites -- builds cleanly and differs from it in %d bytes.  So the binary "+
		"IS sensitive to the layout of this struct, and the byte-identical result above is a measurement of the "+
		"layout not moving rather than of nothing having happened", c1Bytes)
	r.Say("AND TWO CONTROLS MOVE NOTHING, WHICH IS REPORTED RATHER THAN HIDDEN: c2 puts the EMPTY union back and c3 " +
		"puts one SINGLE-MEMBER union back with every one of its `.member` accesses -- this phase run backwards on " +
		"one field each -- and both are byte-identical to the output.  That is the phase's claim stated in the " +
		"other direction, which is the only direction a control can state it in: a union of one member and a " +
		"plain field are the same program, and an empty union is no program at all")

	// --- 7. two recordings, which check the harness and not the phase ------------------
	oldBin, _ := filepath.Abs(oldBinP)
	var wgR sync.WaitGroup
	var errRO, errRN error
	wgR.Add(2)
	go func() {
		defer wgR.Done()
		errRO = check.RecZ(oldBin, oldC, T("REC-old"))
	}()
	go func() {
		defer wgR.Done()
		errRN = check.RecZ(T("new"), f, T("REC-new"))
	}()
	wgR.Wait()
	if check.RecReport(w, errRO, errRN) {
		return harness.ErrReported
	}
	base := check.WalkFiles(T("REC-new"))
	if len(base) < 100 {
		return stop("a recording holds %d records, and a zero recording is 106 -- 102 "+
			"screen cases and four sweeps.  A comparison of two things nothing wrote "+
			"passes", len(base))
	}
	if strings.Join(check.WalkFiles(T("REC-old")), "\x00") != strings.Join(base, "\x00") {
		return stop("the two recordings hold different records")
	}
	var moved []string
	for _, n := range base {
		if !check.Z30Same(filepath.Join(T("REC-new"), n), filepath.Join(T("REC-old"), n)) {
			moved = append(moved, n)
		}
	}
	if len(moved) > 0 {
		show := moved
		if len(show) > 8 {
			show = show[:8]
		}
		return stop("the recording moved in %d of %d records -- from two runs of the "+
			"SAME BYTES, which means the instrument is not deterministic and every "+
			"recording-based Part II phase is in question: %s", len(moved), len(base), strings.Join(show, " "))
	}
	r.Say("two full zrecord recordings, all %d records identical -- 102 "+
		"screen cases, every Ex command typed at `:`, every command line the parser may "+
		"see, the four pty scenarios and the terminal table.  THIS IS A CHECK ON THE "+
		"HARNESS AND NOT ON THE PHASE: the two binaries are the same bytes, so what it "+
		"measures is that the instrument is deterministic, and it is reported in those "+
		"words rather than offered as evidence for the edit", len(base))

	// --- 8. phase 103's structural check ------------------------------------------------
	zh := exec.Command("sh", "tools/st.sh", "zhostonly", f)
	zh.Stdout, zh.Stderr = w, w
	if err := zh.Run(); err != nil {
		return harness.ErrReported
	}
	r.Say("and that is phase 103's check, undisturbed: none of the six names this phase removes is in its " +
		"vocabulary, and the host block is untouched")
	return nil
}
