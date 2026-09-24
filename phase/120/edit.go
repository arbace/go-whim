package p120

// Whim phase 120 -- the degenerate unions go.
// See GOAL.md, whose charter is that the core is what a transpiler reads, and
// GOALS.md II.4c, whose rule is that the core's meaning must be on the page.
//
// THIS FILE HAS THIRTEEN `union` KEYWORDS AND SIX OF THEM UNION NOTHING WITH ANYTHING.
// They are not a style that was always there: they are LEFTOVERS of cuts this pipeline
// and whim's already made.  `u_header`'s four link fields were a union of a pointer and
// a swapfile block number, and the arm that named a block went with the swapfile;
// `typval_S.vval` was a union of nine arms -- a string, a list, a dictionary, a funcref,
// a float, a blob, a job, a channel and a number -- and the eval layer took eight of
// them; `estack_T.es_info` was a union of a `ufunc_T *` and a `sctx_T *`, and both went
// with the script stack.  What is left in each case is a variant type with ONE variant,
// which is a value with a longer spelling, and an EMPTY union, which is a value with no
// spelling at all.
//
// WHICH SIX IS COMPUTED AND NOT LISTED.  The edit scans the file for `union`, matches the
// braces, counts the member declarations at depth 1 and takes every union with FEWER THAN
// TWO as degenerate.  It must find both kinds -- at least one degenerate and at least one
// genuine -- so a scanner that stopped matching cannot pass by finding nothing.  Measured
// on the input: 1 member for `uh_next`, `uh_prev`, `uh_alt_next`, `uh_alt_prev` and
// `vval`, 0 for `es_info`, and 2 or 3 for `ae_u`, `lv_u`, `os_oldval`, `os_newval`,
// `rs_u`, `se_u` and `rs_un`, which stay exactly as they are.  Thirteen keywords become
// seven, and the seven that remain are the ones that are doing the job a union is for.
//
// THE EMPTY ONE IS THE ONE WITH A DIALECT ARGUMENT.  `union { } es_info;` is a GNU C
// extension: ISO C requires a struct-declaration-list to be non-empty, and gcc accepts it
// only because it accepts empty structs and unions as an extension -- `-Wpedantic` says
// so, and the check measures that the input draws exactly one such diagnostic and the
// output none.  GOALS.md's core is meant to be readable by something that is not gcc,
// and a construct the C standard forbids is exactly the kind of latent exotic that costs
// a reader later.  It is also the cheapest possible removal: the field has ZERO uses, one
// mention in the whole file, its own declaration.
//
// WHAT THE REWRITE IS, and it is the same rule twice.  A single-member union becomes its
// member, keeping the UNION's name:
//
// union {                              u_header_T *uh_next;
// u_header_T *ptr;         ->
// } uh_next;
//
// and every `uh_next.ptr` becomes `uh_next`.  The replacement text is the member's OWN
// declaration with the member's name replaced by the union's, so the type, the pointer
// stars and the internal spacing are the input's and not this program's.  The empty union
// is deleted outright, there being no member to promote and no use to rewrite.
//
// A PARTITION AND NOT A COUNT (CLAUDE.md, *Rename a name across the whole file*).  For
// each of the six names, EVERY mention outside a literal must classify as either its own
// declaration or a `.member` access on it, and a mention that is neither REFUSES.  That is
// what makes the rewrite safe rather than merely mechanical: a `uh_next` assigned or
// compared as a whole, a `sizeof(vval)`, a designated initialiser `.vval = `, or another
// struct with a field of the same name and a different member would all land in the
// leftover class and stop the phase.  The counts are read off the text here and nowhere
// written down, so this stays true of a file the phase has never seen -- which is the
// lesson phase 118 was taught when phase 117 moved its counted anchors.
//
// LITERAL-AWARE AND SINGLE-PASS, for the reason phase 106 measured.  The file has no
// preprocessor and no comments, so a string or character literal is exactly a quote and
// the escaped bytes to its match, and the scan is exact; no literal in this file holds any
// of the six names, which is asserted rather than assumed.  And every span -- six
// declarations and every accessor -- is computed against the ORIGINAL text and applied in
// ONE pass, because a second pass would index spans computed on the first pass's output
// and every offset after the first replacement is shifted.
//
// THE BINARY MUST NOT MOVE, AND THAT IS THE WHOLE OF THIS PHASE'S EVIDENCE.  A union of
// one member has the size and alignment of that member and its offset is the union's; an
// empty union contributes no storage.  So no structure layout changes, no expression
// changes value, and `uh_next.ptr` and `uh_next` name the same object at the same address.
// The check rebuilds both sides with SOURCE_DATE_EPOCH=0 and the boundary's own flags and
// requires THE SAME BYTES -- tier 1 of CLAUDE.md's verification table, which subsumes
// every screen case, every Ex-command row, every command line and every pty scenario at
// once, because the program that would be run is literally the same program.  The control
// that makes that `cmp` mean something is in the check and is a layout change of the same
// shape, in the same struct.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim120", Edit) }

var (
	z37Inc  = regexp.MustCompile(`^ *# *include <([A-Za-z0-9_/.]+)>$`)
	z37Dir  = regexp.MustCompile(`^ *#`)
	z37Word = regexp.MustCompile(`\bunion\b`)
	z37Tail = regexp.MustCompile(`^([ \t]*)([A-Za-z_]\w*)[ \t]*;`)
	z37Decl = regexp.MustCompile(`(?s)^(.*?)([A-Za-z_]\w*)[ \t]*;$`)
)

type z37Union struct {
	line        int
	Name        string
	members     int
	indent      string
	start, end  int
	Body        string
	member      string
	replacement string
	accessors   int
}

type z37Edit struct {
	a, b int
	rep  string
}

// Whim120 removes the degenerate unions -- the ones that unite nothing with
// anything, five single-member and one EMPTY, which ISO C forbids.
//
// NOTHING HERE IS A NAME THIS PROGRAM KNOWS IN ADVANCE: the braces are matched
// and the members counted at depth 1, so it states a property of the file rather
// than a memory of one.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "unions", W: w}
	t := string(text)

	blankRuns := func(s string) int {
		L := strings.Split(s, "\n")
		n := 0
		for i := 1; i < len(L); i++ {
			if L[i] == "" && L[i-1] == "" {
				n++
			}
		}
		return n
	}
	lineOf := func(s string, pos int) int { return strings.Count(s[:pos], "\n") + 1 }

	lines := strings.Split(t, "\n")
	runsBefore := blankRuns(t)

	// ---- 0. the file this edit was written against ----------------------------
	var d []int
	for i, l := range lines {
		if z37Dir.MatchString(l) {
			d = append(d, i)
		}
	}
	if len(d) == 0 {
		return nil, p.Die("the file has no preprocessor directive, so there is no boundary between the " +
			"core and the host")
	}
	for i := range d {
		if d[i] != d[0]+i {
			return nil, p.Die("the %d directives are not on consecutive lines, so the first `#include` is not "+
				"a boundary", len(d))
		}
	}
	for _, i := range d {
		if !z37Inc.MatchString(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may add " +
				"one")
		}
	}
	bound := d[0]
	p.Sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the core "+
		"is the %d lines above the first of them", len(d), bound+1, bound)

	// ---- 1. the literals ------------------------------------------------------
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	inLiteral := func(pos int) bool {
		lo, hi := 0, len(spans)
		for lo < hi {
			mid := (lo + hi) / 2
			if spans[mid][0] <= pos {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		k := lo - 1
		return k >= 0 && spans[k][0] <= pos && pos < spans[k][1]
	}
	p.Sayf("%d string and character literals scanned, so every span below is over code", len(spans))

	// ---- 2. every union in the file, and which of them union nothing ----------
	var unions []*z37Union
	for _, m := range z37Word.FindAllStringIndex(t, -1) {
		if inLiteral(m[0]) {
			return nil, p.Die("a literal holds the word `union` at line %d, which no literal in this file "+
				"ever has", lineOf(t, m[0]))
		}
		j := m[1]
		for j < len(t) && (t[j] == ' ' || t[j] == '\t' || t[j] == '\n') {
			j++
		}
		if j >= len(t) || t[j] != '{' {
			return nil, p.Die("the `union` at line %d is not followed by a brace, so this is a union type "+
				"named rather than defined and the scanner cannot classify it", lineOf(t, m[0]))
		}
		depth, k, members := 0, j, 0
		for k < len(t) {
			c := t[k]
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
			k++
		}
		if k >= len(t) {
			return nil, p.Die("the union at line %d never closes", lineOf(t, m[0]))
		}
		tail := z37Tail.FindStringSubmatchIndex(t[k+1:])
		if tail == nil {
			return nil, p.Die("the union at line %d does not end `} <name>;`, and this phase rewrites only "+
				"a union declared as one named field", lineOf(t, m[0]))
		}
		start := strings.LastIndex(t[:m[0]], "\n") + 1
		if strings.TrimSpace(t[start:m[0]]) != "" {
			return nil, p.Die("the union at line %d does not begin its line", lineOf(t, m[0]))
		}
		unions = append(unions, &z37Union{
			line: lineOf(t, m[0]), Name: t[k+1+tail[4] : k+1+tail[5]], members: members,
			indent: t[start:m[0]], start: start, Body: t[j+1 : k], end: k + 1 + tail[1],
		})
	}
	coreLen := len(strings.Join(lines[:bound], "\n"))
	for _, u := range unions {
		if u.end > coreLen {
			return nil, p.Die("a union is defined below the boundary, in the host, and this phase is about the " +
				"CORE")
		}
	}
	var degenerate, genuine []*z37Union
	for _, u := range unions {
		if u.members < 2 {
			degenerate = append(degenerate, u)
		} else {
			genuine = append(genuine, u)
		}
	}
	if len(degenerate) == 0 || len(genuine) == 0 {
		return nil, p.Die("the scan found %d degenerate unions and %d genuine ones, and it must find both "+
			"-- a scanner that stopped matching would otherwise pass by finding nothing",
			len(degenerate), len(genuine))
	}
	gs := make([]string, len(genuine))
	for i, u := range genuine {
		gs[i] = fmt.Sprintf("%s (%d)", u.Name, u.members)
	}
	p.Sayf("%d `union` keywords in the core, every one of them a `union { ... } <name>;` field: "+
		"%d with fewer than two members and %d with two or more.  THE SEVEN THAT STAY ARE "+
		"DOING THE JOB A UNION IS FOR: %s",
		len(unions), len(degenerate), len(genuine), strings.Join(gs, ", "))
	ds := make([]string, len(degenerate))
	for i, u := range degenerate {
		kind := "empty"
		if u.members != 0 {
			kind = "1 member"
		}
		ds[i] = fmt.Sprintf("%s (%s)", u.Name, kind)
	}
	p.Sayf("THE %d THAT GO UNION NOTHING WITH ANYTHING: %s", len(degenerate), strings.Join(ds, ", "))

	// ---- 3. the partition -----------------------------------------------------
	var edits []z37Edit
	var report []string
	for _, u := range degenerate {
		if u.members != 0 {
			decl := strings.TrimSpace(u.Body)
			mm := z37Decl.FindStringSubmatch(decl)
			if mm == nil || strings.Contains(decl, "\n") {
				return nil, p.Die("the single member of `%s` is not one `<type> <name>;` on one line: %s",
					u.Name, cutil.PyRepr(decl))
			}
			u.member = mm[2]
			u.replacement = u.indent + mm[1] + u.Name + ";"
		}
		acc := 0
		var leftover []string
		for _, m := range regexp.MustCompile(`\b`+u.Name+`\b`).FindAllStringIndex(t, -1) {
			switch {
			case inLiteral(m[0]):
				leftover = append(leftover, fmt.Sprintf("line %d %s",
					lineOf(t, m[0]), cutil.PyRepr("inside a literal")))
			case u.start <= m[0] && m[0] < u.end:
				// its own declaration, which this edit rewrites
			case u.member != "" && strings.HasPrefix(t[m[1]:], "."+u.member) &&
				!z37IsIdent(z37At(t, m[1]+1+len(u.member))):
				acc++
				edits = append(edits, z37Edit{m[1], m[1] + 1 + len(u.member), ""})
			default:
				lo := m[0] - 40
				if lo < 0 {
					lo = 0
				}
				hi := m[1] + 40
				if hi > len(t) {
					hi = len(t)
				}
				leftover = append(leftover, fmt.Sprintf("line %d %s",
					lineOf(t, m[0]), cutil.PyRepr(strings.ReplaceAll(t[lo:hi], "\n", "|"))))
			}
		}
		if len(leftover) > 0 {
			s := "s"
			if len(leftover) == 1 {
				s = ""
			}
			mem := u.member
			if mem == "" {
				mem = "<no member>"
			}
			return nil, p.Die("`%s` has %d mention%s that is neither its own declaration nor a `.%s` access "+
				"on it, so this phase may not rewrite it: %s",
				u.Name, len(leftover), s, mem, strings.Join(edit.First(leftover, 4), "; "))
		}
		if u.members != 0 && acc == 0 {
			return nil, p.Die("`%s` has no `.%s` access anywhere, so the field this phase would promote is "+
				"read by nothing and belongs to the sweep and not to this edit", u.Name, u.member)
		}
		u.accessors = acc
		report = append(report, fmt.Sprintf("%s 1 + %d", u.Name, acc))
		if u.members != 0 {
			edits = append(edits, z37Edit{u.start, u.end, u.replacement})
		} else {
			if z37At(t, u.end) != '\n' {
				return nil, p.Die("the empty union `%s` does not end its line, so deleting it would take "+
					"code with it", u.Name)
			}
			edits = append(edits, z37Edit{u.start, u.end + 1, ""})
		}
	}
	p.Sayf("THE PARTITION HOLDS FOR ALL %d: every mention outside a literal is the "+
		"declaration or a `.member` access on it, and there is nothing else -- %s",
		len(degenerate), strings.Join(report, ", "))

	// ---- 4. the rewrite: one pass over the original text ----------------------
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].a != edits[j].a {
			return edits[i].a < edits[j].a
		}
		if edits[i].b != edits[j].b {
			return edits[i].b < edits[j].b
		}
		return edits[i].rep < edits[j].rep
	})
	for i := 0; i+1 < len(edits); i++ {
		if edits[i].b > edits[i+1].a {
			return nil, p.Die("two edit spans overlap at offset %d, so applying them in one pass would "+
				"corrupt the text", edits[i+1].a)
		}
	}
	var Out strings.Builder
	last := 0
	for _, e := range edits {
		Out.WriteString(t[last:e.a])
		Out.WriteString(e.rep)
		last = e.b
	}
	Out.WriteString(t[last:])
	t = Out.String()
	p.Sayf("%d spans rewritten in ONE pass over the original text: %d declarations and %d "+
		"`.member` accesses", len(edits), len(degenerate), len(edits)-len(degenerate))

	// ---- 5. what the file is now ----------------------------------------------
	L := strings.Split(t, "\n")
	left := z37Word.FindAllString(t, -1)
	if len(left) != len(genuine) {
		return nil, p.Die("the file has %d `union` keywords and the %d genuine ones are what must remain",
			len(left), len(genuine))
	}
	for _, u := range degenerate {
		n := len(regexp.MustCompile(`\b`+u.Name+`\b`).FindAllString(t, -1))
		want := 0
		if u.members != 0 {
			want = 1 + u.accessors
		}
		if n != want {
			return nil, p.Die("`%s` has %d mentions and must have %d -- its own declaration and the %d "+
				"accesses that are now plain field references", u.Name, n, want, u.accessors)
		}
		if u.members != 0 {
			if strings.Count(t, u.replacement+"\n") != 1 {
				return nil, p.Die("`%s` is not declared exactly once as `%s`",
					u.Name, strings.TrimSpace(u.replacement))
			}
			if regexp.MustCompile(`\b` + u.Name + `\s*\.\s*` + u.member + `\b`).MatchString(t) {
				return nil, p.Die("a `%s.%s` access survives", u.Name, u.member)
			}
		}
	}
	var nd []int
	for i, l := range L {
		if z37Dir.MatchString(l) {
			nd = append(nd, i)
		}
	}
	if len(nd) != len(d) {
		return nil, p.Die("the file has %d directives and had %d: this phase adds none and removes none",
			len(nd), len(d))
	}
	if nd[0]-d[0] != len(L)-len(lines) {
		return nil, p.Die("the boundary moved by %d lines and the file by %d: every line this phase touches "+
			"is above the first `#include`", nd[0]-d[0], len(L)-len(lines))
	}
	if r := blankRuns(t); r != runsBefore {
		return nil, p.Die("the edit left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	p.Sayf("`union` %d -> %d, %d -> %d lines, %d directives unmoved relative to the text, and "+
		"the blank-line runs unchanged at %d",
		len(unions), len(left), len(lines)-1, len(L)-1, len(nd), blankRuns(t))
	return []byte(t), nil
}

func z37At(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// z37IsIdent is the Python's `(c or ' ').isalnum() or c == '_'`: the byte after
// a `.member` access must not continue the identifier.
func z37IsIdent(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
