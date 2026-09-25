package xform

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	ctext "github.com/arbace/go-whim/crefactor/text"
)

var (
	unInc  = regexp.MustCompile(`^ *# *include <([A-Za-z0-9_/.]+)>$`)
	unDir  = regexp.MustCompile(`^ *#`)
	unWord = regexp.MustCompile(`\bunion\b`)
	unTail = regexp.MustCompile(`^([ \t]*)([A-Za-z_]\w*)[ \t]*;`)
	unDecl = regexp.MustCompile(`(?s)^(.*?)([A-Za-z_]\w*)[ \t]*;$`)
)

// Unions is the step that takes out the core's degenerate unions: a union
// field `union { T m; } u;` with one member is that member, `T u;`, and every
// `u.m` is `u`.  The core is what lies above the first directive, and every
// directive must be an `#include <...>`, all on consecutive lines.  A union
// with no member refuses (it is the sweep's), and so does any mention of a
// degenerate union's field that is neither its declaration nor a `.m`
// access.
//
// Its arguments are floors: `--degenerate N` and `--genuine N` refuse when
// fewer unions of one member, or of two or more, are found.  Without them
// there are none.
func Unions() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		f, err := flags("unions", args, "--degenerate", "--genuine")
		if err != nil {
			return nil, err
		}
		return unions(text, w, f["--degenerate"], f["--genuine"])
	}
}

type unUnion struct {
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

type unEdit struct {
	a, b int
	rep  string
}

// Whim120 removes the degenerate unions -- the ones that unite nothing with
// anything, five single-member and one EMPTY, which ISO C forbids.
//
// NOTHING HERE IS A NAME THIS PROGRAM KNOWS IN ADVANCE: the braces are matched
// and the members counted at depth 1, so it states a property of the file rather
// than a memory of one.
func unions(text []byte, w io.Writer, minDegenerate, minGenuine int) ([]byte, error) {
	p := ctext.Ph{Tag: "unions", W: w}
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

	// ---- 0. the file this edit was written against ----------------------------
	var d []int
	for i, l := range lines {
		if unDir.MatchString(l) {
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
		if !unInc.MatchString(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no step may add " +
				"one")
		}
	}
	bound := d[0]
	p.Sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the core "+
		"is the %d lines above the first of them", len(d), bound+1, bound)

	// ---- 1. the literals ------------------------------------------------------
	spans, err := ctext.LiteralSpans(p, text)
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
	var unions []*unUnion
	for _, m := range unWord.FindAllStringIndex(t, -1) {
		if inLiteral(m[0]) {
			return nil, p.Die("a literal holds the word `union` at line %d, and this step does "+
				"not know it is data", lineOf(t, m[0]))
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
		tail := unTail.FindStringSubmatchIndex(t[k+1:])
		if tail == nil {
			return nil, p.Die("the union at line %d does not end `} <name>;`, and this step rewrites only "+
				"a union declared as one named field", lineOf(t, m[0]))
		}
		start := strings.LastIndex(t[:m[0]], "\n") + 1
		if strings.TrimSpace(t[start:m[0]]) != "" {
			return nil, p.Die("the union at line %d does not begin its line", lineOf(t, m[0]))
		}
		unions = append(unions, &unUnion{
			line: lineOf(t, m[0]), Name: t[k+1+tail[4] : k+1+tail[5]], members: members,
			indent: t[start:m[0]], start: start, Body: t[j+1 : k], end: k + 1 + tail[1],
		})
	}
	coreLen := len(strings.Join(lines[:bound], "\n"))
	for _, u := range unions {
		if u.end > coreLen {
			return nil, p.Die("a union is defined below the boundary, in the host, and this step is about the " +
				"CORE")
		}
	}
	var degenerate, genuine []*unUnion
	for _, u := range unions {
		if u.members < 2 {
			degenerate = append(degenerate, u)
		} else {
			genuine = append(genuine, u)
		}
	}
	if len(degenerate) < minDegenerate || len(genuine) < minGenuine {
		return nil, p.Die("the scan found %d degenerate unions and %d genuine ones, and it was told to find "+
			"at least %d and %d -- a scanner that stopped matching would otherwise pass by finding nothing",
			len(degenerate), len(genuine), minDegenerate, minGenuine)
	}
	gs := make([]string, len(genuine))
	for i, u := range genuine {
		gs[i] = fmt.Sprintf("%s (%d)", u.Name, u.members)
	}
	p.Sayf("%d `union` keywords in the core, every one of them a `union { ... } <name>;` field: "+
		"%d with fewer than two members and %d with two or more.  THOSE THAT STAY ARE "+
		"DOING THE JOB A UNION IS FOR: %s",
		len(unions), len(degenerate), len(genuine), strings.Join(gs, ", "))
	ds := make([]string, len(degenerate))
	for i, u := range degenerate {
		if u.members == 0 {
			return nil, p.Die("`%s` is an EMPTY union, and nothing can name a field with no member: it "+
				"is the sweep's, which takes an unused member, and not this edit's", u.Name)
		}
		ds[i] = fmt.Sprintf("%s (1 member)", u.Name)
	}
	p.Sayf("THE %d THAT GO UNION NOTHING WITH ANYTHING: %s", len(degenerate), strings.Join(ds, ", "))

	// ---- 3. the partition -----------------------------------------------------
	var edits []unEdit
	var report []string
	for _, u := range degenerate {
		decl := strings.TrimSpace(u.Body)
		mm := unDecl.FindStringSubmatch(decl)
		if mm == nil || strings.Contains(decl, "\n") {
			return nil, p.Die("the single member of `%s` is not one `<type> <name>;` on one line: %s",
				u.Name, ctext.PyRepr(decl))
		}
		u.member = mm[2]
		u.replacement = u.indent + mm[1] + u.Name + ";"
		acc := 0
		var leftover []string
		for _, m := range regexp.MustCompile(`\b`+u.Name+`\b`).FindAllStringIndex(t, -1) {
			switch {
			case inLiteral(m[0]):
				leftover = append(leftover, fmt.Sprintf("line %d %s",
					lineOf(t, m[0]), ctext.PyRepr("inside a literal")))
			case u.start <= m[0] && m[0] < u.end:
				// its own declaration, which this edit rewrites
			case strings.HasPrefix(t[m[1]:], "."+u.member) &&
				!unIsIdent(unAt(t, m[1]+1+len(u.member))):
				acc++
				edits = append(edits, unEdit{m[1], m[1] + 1 + len(u.member), ""})
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
					lineOf(t, m[0]), ctext.PyRepr(strings.ReplaceAll(t[lo:hi], "\n", "|"))))
			}
		}
		if len(leftover) > 0 {
			s := "s"
			if len(leftover) == 1 {
				s = ""
			}
			return nil, p.Die("`%s` has %d mention%s that is neither its own declaration nor a `.%s` access "+
				"on it, so this step may not rewrite it: %s",
				u.Name, len(leftover), s, u.member, strings.Join(ctext.First(leftover, 4), "; "))
		}
		if acc == 0 {
			return nil, p.Die("`%s` has no `.%s` access anywhere, so the field this step would promote is "+
				"read by nothing and belongs to the sweep and not to this edit", u.Name, u.member)
		}
		u.accessors = acc
		report = append(report, fmt.Sprintf("%s 1 + %d", u.Name, acc))
		edits = append(edits, unEdit{u.start, u.end, u.replacement})
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
	left := unWord.FindAllString(t, -1)
	if len(left) != len(genuine) {
		return nil, p.Die("the file has %d `union` keywords and the %d genuine ones are what must remain",
			len(left), len(genuine))
	}
	for _, u := range degenerate {
		n := len(regexp.MustCompile(`\b`+u.Name+`\b`).FindAllString(t, -1))
		if want := 1 + u.accessors; n != want {
			return nil, p.Die("`%s` has %d mentions and must have %d -- its own declaration and the %d "+
				"accesses that are now plain field references", u.Name, n, want, u.accessors)
		}
		if strings.Count(t, u.replacement+"\n") != 1 {
			return nil, p.Die("`%s` is not declared exactly once as `%s`",
				u.Name, strings.TrimSpace(u.replacement))
		}
		if regexp.MustCompile(`\b` + u.Name + `\s*\.\s*` + u.member + `\b`).MatchString(t) {
			return nil, p.Die("a `%s.%s` access survives", u.Name, u.member)
		}
	}
	var nd []int
	for i, l := range L {
		if unDir.MatchString(l) {
			nd = append(nd, i)
		}
	}
	if len(nd) != len(d) {
		return nil, p.Die("the file has %d directives and had %d: this step adds none and removes none",
			len(nd), len(d))
	}
	if nd[0]-d[0] != len(L)-len(lines) {
		return nil, p.Die("the boundary moved by %d lines and the file by %d: every line this step touches "+
			"is above the first `#include`", nd[0]-d[0], len(L)-len(lines))
	}
	p.Sayf("`union` %d -> %d, %d -> %d lines, %d directives unmoved relative to the text, and "+
		"the blank-line runs unchanged at %d",
		len(unions), len(left), len(lines)-1, len(L)-1, len(nd), blankRuns(t))
	return []byte(t), nil
}

func unAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// unIsIdent is the Python's `(c or ' ').isalnum() or c == '_'`: the byte after
// a `.member` access must not continue the identifier.
func unIsIdent(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
