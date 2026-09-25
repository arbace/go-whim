package xform

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	ctext "github.com/arbace/go-whim/crefactor/text"
)

// OwnFunc is one library function a core takes on as its own.
type OwnFunc struct {
	// Name is the library's name for it: abs.
	Name string
	// Proto is the core's declaration of it, a line of its own in the
	// declaration block above the first `static`: `int abs(int n);`.
	Proto string
	// Def is the core's own definition, named Prefix+Name, with the blank
	// line after it.
	Def string
}

// OwnKnobs is what Own is told.
type OwnKnobs struct {
	// Prefix names the core's own copy: musl_ for musl_abs.
	Prefix string
	// Funcs are the functions, in the order their definitions are written.
	Funcs []OwnFunc
	// Before is the text the definitions are written directly before: the
	// head of a definition of the same family, which must occur once.
	Before string
}

// Own is the step that makes library functions the core's own: each one's
// declaration leaves the core's declaration block, every mention of its name
// outside a literal is renamed Prefix+Name, and the core's definition is
// written above k.Before.  The file's directives must be its `#include <...>`
// lines, contiguous, the first of them the line between the core and the
// host, and every new definition must land above it.  A literal naming a
// function refuses, since a rename would change what the program prints.
//
// Its arguments are counts it requires exactly, as NAME=N: the call sites
// renamed, per function.  Without them there is no count.
func Own(k OwnKnobs) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := ctext.Ph{Tag: "arith", W: w}
		want := map[string]int{}
		for _, a := range args {
			name, n, ok := strings.Cut(a, "=")
			c, err := strconv.Atoi(n)
			if !ok || err != nil || !k.has(name) {
				return nil, fmt.Errorf("%s: unexpected argument %q (want NAME=N for a function this step owns)", p.Tag, a)
			}
			want[name] = c
		}
		return own(p, k, want, text)
	}
}

func (k OwnKnobs) has(name string) bool {
	for _, f := range k.Funcs {
		if f.Name == name {
			return true
		}
	}
	return false
}

var ownInc = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)

func own(p ctext.Ph, k OwnKnobs, want map[string]int, text []byte) ([]byte, error) {
	nInc := ctext.IncludeCount(text)
	if len(k.Funcs) == 0 {
		return nil, p.Die("no function was named")
	}
	names := make([]string, len(k.Funcs))
	owned := make([]string, len(k.Funcs))
	defs := ""
	for i, f := range k.Funcs {
		names[i] = regexp.QuoteMeta(f.Name)
		owned[i] = k.Prefix + f.Name
		defs += f.Def
	}
	word := regexp.MustCompile(`\b(` + strings.Join(names, "|") + `)\b`)
	mentions := ctext.MentionCount
	directives := func(t []byte) ([]int, []string) {
		lines := strings.Split(string(t), "\n")
		var d []int
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
				d = append(d, i)
			}
		}
		return d, lines
	}
	// contiguous: one block, with nothing but blank lines between its lines --
	// the canonical print puts a blank line between two declarations.
	contiguous := func(ls []string, d []int) bool {
		for i := 1; i < len(d); i++ {
			for j := d[i-1] + 1; j < d[i]; j++ {
				if strings.TrimSpace(ls[j]) != "" {
					return false
				}
			}
		}
		return len(d) > 0
	}
	at := func(d []int, n int) string {
		out := make([]string, 0, n)
		for i := 0; i < len(d) && i < n; i++ {
			out = append(out, strconv.Itoa(d[i]+1))
		}
		return strings.Join(out, " ")
	}
	beforeLines := strings.Count(string(text), "\n")

	// ---- 0. the directives: contiguous, and the first is the boundary ------
	d, lines := directives(text)
	if len(d) != nInc || !contiguous(lines, d) {
		return nil, p.Die("the file does not have exactly its %d preprocessor directives, contiguous: "+
			"%d at %s", nInc, len(d), at(d, 4))
	}
	for _, i := range d {
		if !ownInc.MatchString(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no step may " +
				"add one")
		}
	}
	cut := d[0]
	for _, name := range owned {
		if n := mentions(text, name); n != 0 {
			return nil, p.Die("`%s` already occurs %d times -- this step introduces it, so an existing "+
				"mention means the step has already run or the name is taken", name, n)
		}
	}
	p.Sayf("%d contiguous `#include <...>` directives, the first at line %d and the "+
		"boundary between the core and the host; %s at zero", nInc, cut+1, strings.Join(owned, " and "))

	// ---- 1. the prototypes, found as a BLOCK rather than by line number -----
	firstStatic := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "    static") {
			firstStatic = i
			break
		}
	}
	if firstStatic < 0 {
		return nil, p.Die("there is no `    static` line, so the declaration block has no end")
	}
	var proto []int
	for i, l := range lines[:firstStatic] {
		if l == "" || l[0] == ' ' || l[0] == '\t' {
			continue
		}
		if !strings.HasSuffix(l, ";") || !strings.Contains(l, "(") {
			continue
		}
		if strings.HasPrefix(l, "enum") || strings.HasPrefix(l, "typedef") || strings.HasPrefix(l, "static") {
			continue
		}
		proto = append(proto, i)
	}
	if len(proto) == 0 || !contiguous(lines, proto) {
		return nil, p.Die("the core's library declarations are not one contiguous block above the first "+
			"`static`: %d lines at %s", len(proto), at(proto, 4))
	}
	block := make([]string, len(proto))
	for i, j := range proto {
		block[i] = lines[j]
	}
	for _, f := range k.Funcs {
		n := 0
		for _, l := range block {
			if l == f.Proto {
				n++
			}
		}
		if n != 1 {
			return nil, p.Die("`%s` is not in the core's library declaration block exactly once -- the "+
				"block is: %s", f.Proto, strings.Join(block, " | "))
		}
		if strings.Count(string(text), f.Proto+"\n") != 1 {
			return nil, p.Die("`%s` is not a line of its own exactly once in the whole file", f.Proto)
		}
		text = []byte(strings.Replace(string(text), f.Proto+"\n", "", 1))
	}
	p.Sayf("the core declares %d library functions above the first `static` and will declare %d",
		len(block), len(block)-len(k.Funcs))

	// ---- 2. the call sites, by a literal-aware single pass -------------------
	// Literal-aware, because a name in a string is DATA, and single-pass,
	// because a literal span is an OFFSET and every offset after the first
	// replacement is wrong.
	spans, err := ctext.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var holding []string
	for _, s := range spans {
		if word.Match(text[s[0]:s[1]]) {
			holding = append(holding, string(text[s[0]:s[1]]))
		}
	}
	if len(holding) > 0 {
		return nil, p.Die("a string or character literal mentions a function this step renames, so a rename "+
			"would change what the program PRINTS: %s", strings.Join(ctext.First(holding, 3), " / "))
	}
	inSpan := func(off int) bool {
		lo, hi := 0, len(spans)
		for lo < hi {
			mid := (lo + hi) / 2
			if spans[mid][0] <= off {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		i := lo - 1
		return i >= 0 && spans[i][0] <= off && off < spans[i][1]
	}
	var out strings.Builder
	last := 0
	count := map[string]int{}
	for _, m := range word.FindAllSubmatchIndex(text, -1) {
		if inSpan(m[0]) {
			continue
		}
		name := string(text[m[2]:m[3]])
		out.Write(text[last:m[0]])
		out.WriteString(k.Prefix + name)
		last = m[1]
		count[name]++
	}
	out.Write(text[last:])
	text = []byte(out.String())
	for _, f := range k.Funcs {
		if n, ok := want[f.Name]; ok && count[f.Name] != n {
			return nil, p.Die("the rename reached %d `%s`, and this step was told %d -- the prototypes "+
				"are already gone, so what is left is exactly the call sites", count[f.Name], f.Name, n)
		}
	}
	p.Sayf("the call sites are the core's own now, in one pass, outside every literal; "+
		"%d literals were scanned and none mentions a renamed function", len(spans))

	// ---- 3. the definitions ----------------------------------------------------
	if strings.Count(string(text), k.Before) != 1 {
		return nil, p.Die("the definition the new ones go before is not in the file exactly once, so there " +
			"is no unambiguous place for them")
	}
	text = []byte(strings.Replace(string(text), k.Before, defs+k.Before, 1))

	// ---- 4. what the file is now -----------------------------------------------
	d, lines = directives(text)
	if len(d) != nInc || !contiguous(lines, d) {
		return nil, p.Die("the %d directives are no longer %d contiguous lines", nInc, nInc)
	}
	for _, f := range k.Funcs {
		if n := mentions(text, f.Name); n != 0 {
			return nil, p.Die("`%s` has %d mentions after the cut, expected 0", f.Name, n)
		}
		if n := mentions(text, k.Prefix+f.Name); n != 1+count[f.Name] {
			return nil, p.Die("`%s` has %d mentions after the cut, expected %d", k.Prefix+f.Name, n, 1+count[f.Name])
		}
	}
	for _, name := range owned {
		var where []int
		for i, l := range lines {
			if strings.HasPrefix(l, name+"(") {
				where = append(where, i)
			}
		}
		if len(where) != 1 {
			return nil, p.Die("`%s` is not defined by exactly one line beginning at column 0, which is "+
				"how a definition is read", name)
		}
		if where[0] > d[0] {
			return nil, p.Die("`%s` is defined BELOW the first `#include`, which is the boundary: it is "+
				"core code and every one of its callers is above the line", name)
		}
	}
	grow := strings.Count(defs, "\n") - len(k.Funcs)
	if n := strings.Count(string(text), "\n"); n != beforeLines+grow {
		return nil, p.Die("the file is %d lines and the input was %d -- expected exactly %d more, the "+
			"definitions less the prototypes", n, beforeLines, grow)
	}
	p.Sayf("%s at 0 mentions in the whole file, %s each a definition and its calls, defined at "+
		"column 0 above the boundary, %d -> %d lines",
		strings.Join(names, " and "), strings.Join(owned, " and "), beforeLines, strings.Count(string(text), "\n"))
	return text, nil
}
