package graph

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// OWN on the graph: library functions a core declared and called without a
// body of its own become its own -- each prototype gone from the core's
// library declaration block, a definition of the prefixed name written
// before a definition the caller names, and every use of the prototype
// retargeted to it.  crefactor/xform's text Own was this step on text; the
// graph's retargets by edge, so a literal or another declaration that
// spells the name is never reached and needs no scan to be safe from.

// An OwnFunc is one function a core owns: its name, its prototype as the
// C view prints it, and its definition (the prefixed name's), in C.
type OwnFunc struct {
	Name  string
	Proto string
	Def   string
}

// OwnKnobs are what an Own is told: the prefix the owned names take, the
// functions, and the definition (by name) the new ones go before.
type OwnKnobs struct {
	Prefix string
	Funcs  []OwnFunc
	Before string
}

// Own makes the core own k's functions, reported under v's tag as the text
// step did.  want, when it names a function, is how many uses must move.
func (v *Verbs) Own(k OwnKnobs, want map[string]int) {
	if v.Err != nil {
		return
	}
	e := v.e
	if len(k.Funcs) == 0 {
		v.Die("no function was named")
		return
	}
	var names, owned []string
	var defs strings.Builder
	for _, f := range k.Funcs {
		names = append(names, f.Name)
		owned = append(owned, k.Prefix+f.Name)
		defs.WriteString(f.Def)
	}
	incs, at, err := e.SystemIncludeRun()
	if err != nil {
		v.Die("%v", err)
		return
	}
	text := v.Text()
	firstLine := 0
	for i, l := range strings.Split(string(text), "\n") {
		if strings.HasPrefix(l, "#") {
			firstLine = i + 1
			break
		}
	}
	for _, n := range owned {
		if c := edit.MentionCount(text, n); c != 0 {
			v.Die("`%s` already occurs %d times -- this step introduces it, so an existing "+
				"mention means the step has already run or the name is taken", n, c)
			return
		}
	}
	v.Sayf("%d contiguous `#include <...>` directives, the first at line %d and the "+
		"boundary between the core and the host; %s at zero", len(incs), firstLine, strings.Join(owned, " and "))

	// the core's library declarations: the prototypes of functions, not
	// static, above the first definition
	var block []*Node
	for _, f := range e.g.Forms[:at] {
		if f.Is("defn") {
			break
		}
		if f.Is("def") && !hasPrefix(f, "static") && len(f.Kids) > 2 && f.Kids[2].Is("fn") {
			block = append(block, f)
		}
	}
	if len(block) == 0 {
		v.Die("the core declares no library function above its first definition")
		return
	}
	protos := map[string]*Node{}
	for _, f := range k.Funcs {
		var hit []*Node
		for _, d := range block {
			if c, err := FormsC([]*Node{d}); err == nil && strings.TrimSpace(string(c)) == f.Proto {
				hit = append(hit, d)
			}
		}
		if len(hit) != 1 || len(e.FileDecls(f.Name)) != 1 {
			v.Die("`%s` is not in the core's library declaration block exactly once, the one "+
				"declaration of %s in the file -- %d in the block, %d in the file", f.Proto, f.Name, len(hit), len(e.FileDecls(f.Name)))
			return
		}
		protos[f.Name] = hit[0]
	}
	v.Sayf("the core declares %d library functions above the first `static` and will declare %d",
		len(block), len(block)-len(k.Funcs))

	spans, err := edit.LiteralSpans(edit.Ph{Tag: v.Tag}, text)
	if err != nil {
		v.Die("%v", err)
		return
	}
	var holding []string
	for _, sp := range spans {
		for _, n := range names {
			if edit.MentionCount(text[sp[0]:sp[1]], n) > 0 {
				holding = append(holding, string(text[sp[0]:sp[1]]))
				break
			}
		}
	}
	if len(holding) > 0 {
		v.Die("a string or character literal mentions a function this step renames, so a rename "+
			"would change what the program PRINTS: %s", strings.Join(edit.First(holding, 3), " / "))
		return
	}
	v.Expect(e.Defn(k.Before) != nil, "the definition the new ones go before, %s, is not in the file, so there is no place for them", k.Before)
	q := NewVerbs(v.Tag, e, io.Discard) // the text said one line for the whole step, below
	q.TopBeforeC(k.Before, defs.String(), "the owned definitions, before "+k.Before)
	if q.Err != nil {
		v.Err = q.Err
		return
	}
	count := map[string]int{}
	for _, f := range k.Funcs {
		to := e.Defn(k.Prefix + f.Name)
		if to == nil {
			v.Die("`%s` was not defined", k.Prefix+f.Name)
			return
		}
		uses, err := e.RetargetUses(protos[f.Name], to)
		if err != nil {
			v.Die("%v", err)
			return
		}
		count[f.Name] = len(uses)
		if n, ok := want[f.Name]; ok && n != len(uses) {
			v.Die("the rename reached %d `%s`, and this step was told %d -- the prototypes "+
				"are already gone, so what is left is exactly the call sites", len(uses), f.Name, n)
			return
		}
		if err := e.Delete(protos[f.Name]); err != nil {
			v.Die("%v", err)
			return
		}
	}
	v.Sayf("the call sites are the core's own now, in one pass, outside every literal; "+
		"%d literals were scanned and none mentions a renamed function", len(spans))

	after := v.Text()
	if _, _, err := e.SystemIncludeRun(); err != nil {
		v.Die("the %d directives are no longer %d contiguous lines: %v", len(incs), len(incs), err)
		return
	}
	for _, f := range k.Funcs {
		if n := edit.MentionCount(after, f.Name); n != 0 {
			v.Die("`%s` has %d mentions after the cut, expected 0", f.Name, n)
			return
		}
		if n := edit.MentionCount(after, k.Prefix+f.Name); n != 1+count[f.Name] {
			v.Die("`%s` has %d mentions after the cut, expected %d", k.Prefix+f.Name, n, 1+count[f.Name])
			return
		}
		if d := e.FileDecls(k.Prefix + f.Name); len(d) != 1 || !e.InCore(d[0]) {
			v.Die("`%s` is not defined exactly once, above the first `#include`, which is the boundary: it is "+
				"core code and every one of its callers is above the line", k.Prefix+f.Name)
			return
		}
	}
	v.Sayf("%s at 0 mentions in the whole file, %s each a definition and its calls, defined "+
		"above the boundary, %d -> %d lines",
		strings.Join(names, " and "), strings.Join(owned, " and "), strings.Count(string(text), "\n"), strings.Count(string(after), "\n"))
}
