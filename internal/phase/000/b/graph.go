package p000b

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// ON THE GRAPH (R3, doc/GRAPH-MIGRATION.md): the same collapse, its sites
// found by edge -- every call of a wrapper's declarations outside the seven
// definitions -- and written by FRAG, the call's arguments moved in as
// holes (the format, written twice, copied the second time).  What the text
// told apart by the character after the call's `)` is told by the node: a
// call that ends its expression statement (the statement, or the operand
// that statement ends with: `(void)semsg(...)`) becomes the formatting call
// in its place and the tail as a statement after it -- the text's two
// statements, `(void)vim_snprintf(...); emsg(...);` -- and any other is in
// value position, the comma shape.  The seven definitions and six
// prototypes are deleted by name, and vim_snprintf's second prototype with
// them, its uses pointed where an import of the file after puts them: the
// first prototype above the definition, the definition below it.
func init() { phase.RegisterGraph("whim0b", EditGraph) }

// EditGraph is phase 0b on the graph.
func EditGraph(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("format", e, w)
	text, err := e.Graph().C()
	if err != nil {
		return err
	}
	mentions := func(t []byte, name string) int { return edit.MentionCount(t, name) }

	// ---- 0. this is the file the phase was written against --------------
	for _, wr := range w0bWrap {
		if k := mentions(text, wr.Name); k != wr.want {
			v.Die("the input has %d mentions of `%s`, expected %d -- this is not the tree "+
				"this phase was written against", k, wr.Name, wr.want)
			return v.Done()
		}
	}
	for _, c := range []struct {
		Name string
		want int
	}{{"va_start", 8}, {"va_list", 15}, {"va_end", 10}} {
		if k := mentions(text, c.Name); k != c.want {
			v.Die("the input has %d mentions of `%s`, expected %d", k, c.Name, c.want)
			return v.Done()
		}
	}
	vsBefore := mentions(text, "vim_snprintf")
	linesBefore := strings.Count(string(text), "\n") + 1 // the text counted the pieces a split makes
	v.Say("the input is the seed's: eight functions call `va_start`, seven of them wrappers over " +
		"the eighth, and their mention totals are 42 4 2 214 15 4 29")

	// ---- the declarations: seven definitions, six prototypes, and two of
	// vim_snprintf's (the second goes: it was there for the wrappers)
	defs := map[*graph.Node]bool{}
	var protos []*graph.Node
	for _, wr := range w0bWrap {
		var d *graph.Node
		np := 0
		for _, f := range e.FileDecls(wr.Name) {
			if f.Is("defn") {
				d = f
			} else {
				np++
				protos = append(protos, f)
			}
		}
		want := 1
		if wr.Name == "smsg_attr_keep" {
			want = 0 // it never had one
		}
		if d == nil || np != want {
			v.Die("D: `%s` has %d prototypes and a definition %v, expected %d and one", wr.Name, np, d != nil, want)
			return v.Done()
		}
		defs[d] = true
	}
	var vsProtos []*graph.Node
	var vsDef *graph.Node
	for _, f := range e.FileDecls("vim_snprintf") {
		if f.Is("defn") {
			vsDef = f
		} else {
			vsProtos = append(vsProtos, f)
		}
	}
	if len(vsProtos) != 2 || vsDef == nil {
		v.Die("P0: `vim_snprintf` has %d prototypes, expected 2, and a definition", len(vsProtos))
		return v.Done()
	}

	// ---- the sites, before anything moves --------------------------------
	type site struct {
		name string
		call *graph.Node
	}
	var sites []site
	for _, wr := range w0bWrap {
		for _, d := range e.FileDecls(wr.Name) {
			for _, u := range e.Uses(d) {
				if inAny(e, u, defs) {
					continue // a wrapper calling another goes with its definition
				}
				c := e.Parent(u)
				if c == nil || !c.Is("call") || len(c.Kids) < 2 || c.Kids[1] != u {
					v.Die("`%s` is used other than called, in %s", wr.Name, graph.DeclLabel(e.TopForm(u)))
					return v.Done()
				}
				sites = append(sites, site{wr.Name, c})
			}
		}
	}
	// in the file's order, as the text met them
	order := map[*graph.Node]int{}
	k := 0
	for _, f := range e.Graph().Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if n.IsList() {
				order[n] = k
				k++
			}
			return true
		})
	}
	sortSites := func(s []site) {
		for i := 1; i < len(s); i++ {
			for j := i; j > 0 && order[s[j].call] < order[s[j-1].call]; j-- {
				s[j], s[j-1] = s[j-1], s[j]
			}
		}
	}
	sortSites(sites)

	// ---- the five helpers, and six declarations below vim_snprintf's first
	if _, err := e.SpliceC(graph.Frag{At: e.SpotAfter(vsProtos[0]), Src: strings.TrimPrefix(w0blit9, w0blit8)}); err != nil {
		v.Die("P1 -- %v", err)
		return v.Done()
	}
	var anchor *graph.Node
	for _, f := range e.FileDecls("last_sourcing_lnum") {
		if anchor == nil {
			anchor = f
		}
	}
	if anchor == nil {
		v.Die("H: no `last_sourcing_lnum` for the helpers to go above")
		return v.Done()
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotBefore(anchor), Src: w0bHelpers}); err != nil {
		v.Die("H -- %v", err)
		return v.Done()
	}

	// ---- 4. the call sites ----------------------------------------------
	shapes := map[string]int{"statement": 0, "value": 0}
	nPlain := 0
	plain := func(f *graph.Node) bool {
		if !f.IsList() && strings.HasPrefix(f.Atom, "\"") && strings.HasSuffix(f.Atom, "\"") && !strings.Contains(f.Atom, "%") {
			nPlain++
			return true
		}
		return false
	}
	counts := map[string]int{}
	var frags []graph.Frag
	for _, s := range sites {
		args := s.call.Kids[2:]
		counts[s.name]++
		holes := graph.Bindings{}
		h := make([]string, len(args))
		for i, a := range args {
			h[i] = fmt.Sprintf("$a%d", i)
			holes[fmt.Sprintf("a%d", i)] = a
		}
		switch s.name {
		case "vim_snprintf_safelen":
			if len(args) < 3 || len(args) == 3 && !plain(args[2]) {
				v.Die("vim_snprintf_safelen with %d arguments", len(args))
				return v.Done()
			}
			frags = append(frags, graph.Frag{At: e.SpotOf(s.call), Holes: holes,
				Src: fmt.Sprintf("safelen_result(%s, %s, vim_snprintf(%s))", h[0], h[1], strings.Join(h, ", "))})
			shapes["value"]++
			continue
		case "vim_snprintf_add":
			if len(args) < 3 || len(args) == 3 && !plain(args[2]) {
				v.Die("vim_snprintf_add with %d arguments", len(args))
				return v.Done()
			}
			src := fmt.Sprintf("vim_snprintf(%s + strlen((char *)(%s)), append_room(%s, %s), %s)",
				h[0], h[0], h[0], h[1], strings.Join(h[2:], ", "))
			if e.Item(s.call) == s.call {
				src += ";" // the call a statement of its own: the text's `;` stays
			}
			frags = append(frags, graph.Frag{At: e.SpotOf(s.call), Holes: holes, Src: src})
			shapes["value"]++
			continue
		}
		nl := w0bLead[s.name]
		if len(args) < nl+1 || len(args) == nl+1 && !plain(args[nl]) {
			v.Die("`%s` with %d arguments -- a site that passes no variadic argument "+
				"must pass a literal without `%%`, which vim_snprintf copies as it is", s.name, len(args))
			return v.Done()
		}
		f := h[nl]
		a := fmt.Sprintf("vim_snprintf((char *)IObuff, %s, %s)", w0bRoom[s.name], strings.Join(h[nl:], ", "))
		var b string
		switch s.name {
		case "smsg":
			b = fmt.Sprintf("msg(iobuff_or(%s))", f)
		case "smsg_attr":
			b = fmt.Sprintf("msg_attr(iobuff_or(%s), %s)", f, h[0])
		case "smsg_attr_keep":
			b = fmt.Sprintf("msg_attr_keep(iobuff_or(%s), %s, TRUE)", f, h[0])
		case "semsg":
			b = fmt.Sprintf("emsg(iobuff_or(%s))", f)
		default:
			b = fmt.Sprintf("iemsg(iobuff_or(%s))", f)
		}
		switch st := endsStatement(e, s.call); {
		case st == s.call:
			// A STATEMENT: two statements where it was.
			frags = append(frags, graph.Frag{At: e.SpotOf(s.call), Holes: holes, Src: a + "; " + b + ";"})
			shapes["statement"]++
		case st != nil:
			// The operand a statement ends with: the call there, and the
			// tail a statement after it -- the text's split at `);`.
			frags = append(frags, graph.Frag{At: e.SpotOf(s.call), Holes: holes, Src: a},
				graph.Frag{At: e.SpotAfter(st), Holes: holes, Src: b + ";"})
			shapes["statement"]++
		default:
			// VALUE POSITION: the comma shape the regexp engine already uses.
			frags = append(frags, graph.Frag{At: e.SpotOf(s.call), Holes: holes, Src: "(" + a + ", " + b + ")"})
			shapes["value"]++
		}
	}
	if len(sites) != w0bSites {
		v.Die("%d call sites, expected %d", len(sites), w0bSites)
		return v.Done()
	}
	if _, err := e.SpliceC(frags...); err != nil {
		v.Die("the call sites -- %v", err)
		return v.Done()
	}

	// ---- 1, 2. the definitions and the prototypes ------------------------
	for _, f := range e.Graph().Forms {
		if defs[f] {
			if err := e.Delete(f); err != nil {
				v.Die("D -- %v", err)
				return v.Done()
			}
		}
	}
	for _, f := range protos {
		if err := e.Delete(f); err != nil {
			v.Die("P -- %v", err)
			return v.Done()
		}
	}
	at := map[*graph.Node]int{}
	for i, f := range e.Graph().Forms {
		at[f] = i
	}
	for _, u := range e.Uses(vsProtos[1]) {
		to := vsProtos[0]
		if at[e.TopForm(u)] > at[vsDef] {
			to = vsDef
		}
		for i, r := range u.Refs {
			if r == vsProtos[1] {
				if err := e.Retarget(u, i, to); err != nil {
					v.Die("P0 -- %v", err)
					return v.Done()
				}
			}
		}
	}
	if err := e.Delete(vsProtos[1]); err != nil {
		v.Die("P0 -- %v", err)
		return v.Done()
	}

	// ---- what the file is now --------------------------------------------
	out, err := e.Graph().C()
	if err != nil {
		return err
	}
	tally := make([]string, len(w0bWrap))
	for i, wr := range w0bWrap {
		tally[i] = fmt.Sprintf("%s %d", wr.Name, counts[wr.Name])
	}
	for _, wr := range w0bWrap {
		if k := mentions(out, wr.Name); k != 0 {
			v.Die("`%s` still has %d mentions after the expansion", wr.Name, k)
			return v.Done()
		}
	}
	for _, c := range []struct {
		Name string
		want int
	}{{"va_start", 1}, {"va_list", 8}, {"va_end", 3}} {
		if k := mentions(out, c.Name); k != c.want {
			v.Die("`%s` has %d mentions after the expansion, expected %d", c.Name, k, c.want)
			return v.Done()
		}
	}
	if k := mentions(out, "vim_snprintf"); k != vsBefore-1+len(sites) {
		v.Die("`vim_snprintf` went from %d to %d, expected %d -- one prototype away and one "+
			"mention at each of the %d sites", vsBefore, k, vsBefore-1+len(sites), len(sites))
		return v.Done()
	}
	v.Sayf("%d call sites expanded: %s", len(sites), strings.Join(tally, ", "))
	v.Sayf("%d statements, %d in value "+
		"position -- `return (semsg(...), rc_did_emsg = TRUE, nullptr)` comma "+
		"expressions, the safelens whose value is consumed, and the appends; %d pass no "+
		"variadic argument, a literal without `%%`",
		shapes["statement"], shapes["value"], nPlain)
	v.Sayf("`va_start` 8 -> 1, `va_list` 15 -> 8, `va_end` 10 -> 3; `vim_snprintf` %d -> %d; "+
		"seven definitions and six prototypes gone, five helpers and six declarations in; "+
		"lines %d -> %d",
		vsBefore, mentions(out, "vim_snprintf"), linesBefore, strings.Count(string(out), "\n")+1)
	return v.Done()
}

// inAny says n is inside one of the forms.
func inAny(e *graph.Editor, n *graph.Node, forms map[*graph.Node]bool) bool {
	return forms[e.TopForm(n)]
}

// endsStatement is the expression statement whose C ends with c's -- c
// itself as an item, or an item that ends with c by its last operand (a
// cast, an assignment, a binary operator, a comma) -- or nil: what the text
// told by a `;` after the call's `)`.
func endsStatement(e *graph.Editor, c *graph.Node) *graph.Node {
	n := c
	for {
		if e.Item(n) == n {
			return n
		}
		p := e.Parent(n)
		if p == nil || !p.IsList() || graph.IsStatement(p) || p.Kids[len(p.Kids)-1] != n {
			return nil
		}
		switch p.Head() {
		case "cast", "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", ",",
			"+", "-", "*", "/", "%", "&&", "||", "&", "|", "^", "<<", ">>",
			"==", "!=", "<", ">", "<=", ">=", "!", "~", "neg", "deref", "addr":
		default:
			return nil
		}
		n = p
	}
}
