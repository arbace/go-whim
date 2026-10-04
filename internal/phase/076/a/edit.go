package p076a

// Whim phase 76a (formerly 151) -- the option table's defaults are typed.  See GOAL.md.
//
// def_val[2] held a string option's defaults and, cast to char_u *, a number's
// or a boolean's (internal/gen/FINDINGS.md, 2).  A row now has def_str[2] and def_num[2],
// the one its kind uses filled and the other empty, and every read names one.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim76a", Edit) }

// Edit gives the option table typed defaults.
//
// vimoption_T.def_val[2] held each option's default, for Vi and for Vim: a
// string for a string option, and for a number or boolean option the number
// itself cast to char_u * -- `(char_u *)80L`, `(char_u *)TRUE` -- cast back
// with (long)(long_i) where it was read.  The Go transpilation held them as
// `any` (internal/gen/FINDINGS.md, 2).  Now a row has def_str[2] and def_num[2]: a string
// option's defaults in the first, a number's or a boolean's in the second, the
// other pair empty; every read and write names the one it means.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, Step6 as built): each row's kind is
// its flags' (P_STRING, or no flags at all, is a string's), its default pair
// written in place and the other pair inserted beside it
// (Editor.InsertElements); def_val renamed def_str by edge and def_num
// inserted after it (Editor.InsertMember), so the struct keeps its node; the
// number reads and the one number store found by their form, their member
// pointed at def_num (RetargetAs) and their casts taken off, the types above
// derived again.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("defaults", e, w)
	def := v.One("(def static options _ (init _*))", "options[]")
	if v.Failed() {
		v.Die("options[] is not where this phase expects it")
		return v.Done()
	}
	// the member: def_str, and def_num beside it
	m := v.One("(def_val (array 2 (ptr char_u)))", "the default pair's member")
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.Rename(m, "def_str"); err != nil {
		v.Die("def_val as def_str -- %v", err)
		return v.Done()
	}
	num, err := e.InsertMember(m, true, "(def_num (array 2 long))")
	if err != nil {
		v.Die("def_num -- %v", err)
		return v.Done()
	}
	rows := graph.TableInit(def).Args()
	for _, r := range rows {
		if !r.Is("init") || len(r.Args()) < 4 {
			v.Die("a row of options[] is not a braced list of its fields")
			return v.Done()
		}
		els := r.Args()
		pair := els[len(els)-1]
		if !pair.Is("init") || len(pair.Args()) != 2 {
			v.Die("a row of options[] has no default pair last")
			return v.Done()
		}
		flags, err := graph.ExprText(els[2])
		if err != nil {
			v.Die("%v", err)
			return v.Done()
		}
		a, b := pair.Args()[0], pair.Args()[1]
		if strings.Contains(flags, "P_STRING") || strings.TrimSpace(flags) == "0" {
			// the string pair is the defaults, nullptr for none; the number pair empty
			for _, x := range []*graph.Node{a, b} {
				if !isAtom(x, "nullptr") && isAtom(uncast(x), "0L") {
					v.Expect(e.Replace(x, graph.NewAtom("nullptr")) == nil, "a default could not be written nullptr")
				}
			}
			v.Expect(e.InsertElements(r, len(els), graph.NewList(graph.NewAtom("init"), graph.NewAtom("0L"), graph.NewAtom("0L"))) == nil,
				"the number pair could not be added to a row")
		} else {
			for _, x := range []*graph.Node{a, b} {
				if y := uncast(x); y != x {
					v.Expect(e.Replace(x, y) == nil, "a number default could not be uncast")
				}
			}
			v.Expect(e.InsertElements(r, len(els)-1, graph.NewList(graph.NewAtom("init"), graph.NewAtom("nullptr"), graph.NewAtom("nullptr"))) == nil,
				"the string pair could not be added to a row")
		}
		if v.Failed() {
			return v.Done()
		}
	}
	v.Sayf("the %d rows of options[] give their defaults as a string pair and a number pair", len(rows))

	v.Say("a row holds its string defaults and its number defaults apart")

	// the one store of a number default
	toNum := func(sel *graph.Node) bool {
		// sel is (index (. B def_str) I) or (index (-> p def_str) I)
		s := sel.Args()[0]
		u := s.Args()[len(s.Args())-1]
		if err := e.RetargetAs(u, 0, num); err != nil {
			v.Die("a number default's member -- %v", err)
			return false
		}
		e.RederiveValue(s)
		return true
	}
	stores := v.Find("(= (index (. (index options opt_idx) def_str) VI_DEFAULT) (cast (ptr char_u) (cast long_i val)))")
	if len(stores) != 1 {
		v.Die("a number default is stored as a number (%.60q) -- occurs %d times, expected 1", "options[opt_idx].def_val[VI_DEFAULT] = (char_u *)(long_i)val;", len(stores))
		return v.Done()
	}
	asg := stores[0]
	rhs := asg.Args()[1]
	val := rhs.Args()[1].Args()[1]
	if e.Replace(rhs, val) != nil || !toNum(asg.Args()[0]) {
		v.Die("a number default is stored as a number -- the store could not be rewritten")
		return v.Done()
	}
	v.Say("a number default is stored as a number")

	// the number reads: (int)(long)(long_i)X and (long)(long_i)X, X a default
	// of options[...] or of p
	reads := 0
	for _, c := range v.Find("(cast long (cast long_i (index _ _)))") {
		sel := c.Args()[1].Args()[1]
		if !isDefault(sel) {
			continue
		}
		if e.Replace(c, sel) != nil || !toNum(sel) {
			v.Die("a number read could not be rewritten")
			return v.Done()
		}
		reads++
	}
	strs := len(e.Uses(m))
	if reads != 6 || strs != 12 {
		v.Die("%d number reads and %d string uses of def_val, and this phase was written against 6 and 12", reads, strs)
		return v.Done()
	}
	v.Sayf("%d reads take the number without a cast, and %d uses of a string default name def_str", reads, strs)
	return v.Done()
}

// isDefault says sel is `options[K].def_str[I]` or `p->def_str[I]`.
func isDefault(sel *graph.Node) bool {
	if !sel.Is("index") || len(sel.Args()) != 2 {
		return false
	}
	s := sel.Args()[0]
	if len(s.Args()) != 2 || s.Args()[1].Atom != "def_str" {
		return false
	}
	b := s.Args()[0]
	switch {
	case s.Is("."):
		return b.Is("index") && len(b.Args()) == 2 && b.Args()[0].Atom == "options" && !b.Args()[0].IsList()
	case s.Is("->"):
		return !b.IsList() && b.Atom == "p"
	}
	return false
}

// uncast is x without a `(char_u *)` cast in front, as the text's regular
// expression took it off.
func uncast(x *graph.Node) *graph.Node {
	if x.Is("cast") && len(x.Args()) == 2 {
		if t := x.Args()[0]; t.Is("ptr") && len(t.Args()) == 1 && t.Args()[0].Atom == "char_u" {
			return x.Args()[1]
		}
	}
	return x
}

func isAtom(x *graph.Node, s string) bool { return !x.IsList() && x.Atom == s }
