package p071a

// Whim phase 71a (formerly 143) -- regatom() has no goto.  See GOAL.md.
//
// regatom() jumped into other cases three ways (internal/gen/FINDINGS.md, 11).  The
// delimiter atom becomes regatom_delim(), the multibyte node is written where
// its jump was, and the switch dispatches on sw in a loop that runs once, so
// the collection is reached by dispatching again.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim71a", Edit) }

// W71aCall is what replaces a jump to delimiter_atom, and the block itself, at
// an indentation: the helper's node, or a return of the NULL it gives after
// its error message.
func W71aCall(indent string) string {
	return indent + "ret = regatom_delim(c, delim_nl, flagp);\n" +
		indent + "if (ret == nullptr)\n" +
		indent + "{\n" +
		indent + "    return nullptr;\n" +
		indent + "}\n" +
		indent + "break;\n"
}

// W71aHelper is regatom_delim(): the delimiter block, taken Out of regatom()
// verbatim, with the node it makes returned.  Its indentation is the canonical
// print's.
func W71aHelper(block string) string {
	return "    static char_u *\nregatom_delim(int c, int delim_nl, int *flagp)\n{\n    char_u      *ret;\n\n" +
		block + "    return ret;\n}\n\n"
}

// Edit takes the three jumps out of regatom().
//
// regatom() jumped into the middle of other cases three ways (internal/gen/FINDINGS.md,
// 11): `\_%)` to the delimiter atom inside the `\%` case's own switch, and
// `\%>` there too when no digit follows; `\_[` to the collection that starts
// the `[` case; and `.` followed by a composing character to the multibyte
// node in the default case.  Go cannot jump into a case or a block.
//
//   - the delimiter atom's block becomes regatom_delim(), verbatim, and its
//     case and both jumps call it: regnode() never returns NULL, so a NULL is
//     the block's own error return, passed on;
//   - the multibyte node's three statements are written where the jump was;
//   - the switch dispatches on sw, not c, in a loop that runs once: the `\_[`
//     path sets sw to the `[` case and continues, and c is '[' as the jump
//     left it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): found by form -- the label, the
// block it marks and the break after it, the jumps, the statements -- and
// written in one FRAG unit: regatom_delim() before regatom() (the block's
// items printed into it: its uses become the helper's parameters, which
// MOVE refuses), the call at the case and at both jumps, the multibyte
// node at its jump; then the loop by BUILD, the switch moved into it whole
// (its nodes kept) and the `\_[` jump a store and a continue.  History keeps
// the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("regatom", e, w)
	fn := e.Defn("regatom")
	if fn == nil {
		v.Die("regatom is not defined")
		return v.Done()
	}
	is := func(n *graph.Node, pat string) bool {
		p, err := clisp.Pattern(pat)
		return err == nil && n != nil && graph.Matches(p, n)
	}
	// sibling is the item k places after n in its list, or nil
	sibling := func(n *graph.Node, k int) *graph.Node {
		ks := e.Parent(n).Kids
		for i, x := range ks {
			if x == n && i+k >= 0 && i+k < len(ks) {
				return ks[i+k]
			}
		}
		return nil
	}
	put := func(at *graph.Node, tmpl string, how string) []*graph.Node {
		if v.Failed() {
			return nil
		}
		ns, err := e.Build(at, tmpl, nil)
		if err == nil {
			switch how {
			case "replace":
				err = e.Replace(at, ns...)
			case "before":
				err = e.InsertBefore(at, ns...)
			case "after":
				err = e.InsertAfter(at, ns...)
			}
		}
		if err != nil {
			v.Die("regatom(): %s -- %v", tmpl, err)
		}
		return ns
	}
	v.In(fn, func(v *graph.Verbs) {
		// 1. the delimiter atom
		lbl := v.One("(label delimiter_atom)", "the delimiter_atom label")
		if v.Failed() {
			return
		}
		blk, brk := sibling(lbl, 1), sibling(lbl, 2)
		if !is(sibling(lbl, -1), "(case 't')") {
			v.Die("the delimiter_atom label is not where this phase expects it")
			return
		}
		if blk == nil || !blk.Is("block") {
			v.Die("delimiter_atom does not label a block")
			return
		}
		if !is(brk, "(break)") {
			v.Die("the delimiter block does not end in a break")
			return
		}
		var inner []*clisp.Node
		for _, x := range blk.Kids[1:] {
			inner = append(inner, graph.Lisp(x))
		}
		c, err := clisp.PrintItems(inner)
		if err != nil {
			v.Die("%v", err)
			return
		}
		jumps := v.Find("(goto delimiter_atom)")
		if len(jumps) != 2 {
			v.Die("%d jumps to delimiter_atom at the expected places, and this phase was written against 2", len(jumps))
			return
		}
		// 2. the multibyte node
		mb := v.Find("(goto do_multibyte)")
		ml := v.Find("(label do_multibyte)")
		if len(mb) != 1 || len(ml) != 1 || !is(sibling(mb[0], -1), "(= c (call getchr))") ||
			!is(sibling(ml[0], 1), "(= ret (call regnode MULTIBYTECODE))") || !is(sibling(ml[0], 2), "(call regmbc c)") ||
			!is(sibling(ml[0], 3), "(|= (deref flagp) (| HASWIDTH SIMPLE))") || !is(sibling(ml[0], 4), "(break)") {
			v.Die("the do_multibyte jump or label is not what this phase expects")
			return
		}
		call := W71aCall("")
		if _, err := e.SpliceC(graph.Frag{At: e.SpotBefore(fn), Src: W71aHelper(string(c))},
			graph.Frag{At: e.SpotRun(lbl, brk), Src: call},
			graph.Frag{At: e.SpotOf(jumps[0]), Src: call},
			graph.Frag{At: e.SpotOf(jumps[1]), Src: call},
			graph.Frag{At: e.SpotOf(mb[0]), Src: "ret = regnode(MULTIBYTECODE);\nregmbc(c);\n*flagp |= HASWIDTH | SIMPLE;\nbreak;\n"},
		); err != nil {
			v.Die("the delimiter atom is regatom_delim() -- %v", err)
			return
		}
		v.Say("the delimiter atom is regatom_delim(), called from its case and from the two jumps")
		if err := e.Delete(ml[0]); err != nil {
			v.Die("%v", err)
			return
		}
		v.Say("`.` with a composing character makes its multibyte node where it was")
		// 3. the collection
		cg := v.Find("(goto collection)")
		cl := v.Find("(label collection)")
		if len(cg) != 1 || len(cl) != 1 || !is(sibling(cl[0], -1), "(case (paren (- (cast int (paren '[')) 256)))") {
			v.Die("the collection jump or label is not what this phase expects")
			return
		}
		var sw *graph.Node
		for _, x := range fn.Kids {
			if x.Is("switch") {
				if sw != nil {
					sw = nil
					break
				}
				sw = x
			}
		}
		if sw == nil || !is(sw, "(switch c _)") || !is(sibling(sw, -1), "(= c (call getchr))") || !is(sibling(sw, 1), "(return ret)") {
			v.Die("regatom()'s switch is not where this phase expects it")
			return
		}
		ds := v.Find("(def c int)")
		if len(ds) != 1 {
			v.Die("regatom()'s c is not declared where this phase expects")
			return
		}
		put(ds[0], "(def sw int)", "after")
		put(sw, "(= sw c)", "before")
		put(sw.Kids[1], "sw", "replace")
		if v.Failed() {
			return
		}
		loop, err := e.Build(sw, "(for () () () (block ?s (break)))", graph.Bindings{"s": sw})
		if err == nil {
			err = e.Replace(sw, loop...)
		}
		if err != nil {
			v.Die("regatom()'s loop -- %v", err)
			return
		}
		if _, err := e.SpliceC(graph.Frag{At: e.SpotOf(cg[0]), Src: "sw = ((int)('[') - 256);\ncontinue;\n"}); err != nil {
			v.Die("regatom(): the `\\_[` jump -- %v", err)
			return
		}
		if err := e.Delete(cl[0]); err != nil {
			v.Die("%v", err)
			return
		}
		if v.Count("(goto _)") != 0 || v.Count("(label _)") != 0 {
			v.Die("regatom() still jumps")
			return
		}
		v.Say("regatom() dispatches on sw in a loop that runs once, and `\\_[` dispatches again to the collection: no goto left")
	})
	return v.Done()
}
