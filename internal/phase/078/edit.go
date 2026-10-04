package p078

// Whim phase 78 (formerly 155) -- call arguments with effects are evaluated in gcc's order.  See GOAL.md.
//
// gcc evaluates call arguments right to left; Go, left to right.  Where two
// arguments both have effects (internal/ccx), the one gcc evaluates first
// becomes a local computed before the call -- eleven calls.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim78", Edit) }

// Edit evaluates call arguments with effects in the order gcc does.
//
// C leaves the order of a call's arguments unspecified, Go evaluates them left
// to right, and gcc -- measured on this file's code, line by line in its
// disassembly -- evaluates them right to left.  Where two arguments both call
// a function with an effect outside its frame (internal/ccx's Order), the
// order is part of what the program does, and a translation that evaluated
// them left to right would not be this program.  Eleven calls are such: nine
// vim_strnsave(ml_get...(), ml_get..._len()), which gcc evaluates length
// first; col_print(..., ml_get_curline_len(), linetabsize_str(p)), which it
// evaluates linetabsize_str first; and fileinfo()'s message, whose
// new_file_message() it calls before the shortmess() of an earlier argument.
// Each first-evaluated argument becomes a local computed before the call, so
// the order is written, not implied (internal/gen/FINDINGS.md).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): each statement is put in a
// block that declares the new local first (BUILD), and the argument is
// moved into the local's value with its nodes (MOVE's MoveTo), the local's
// name taking the place it leaves; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("argorder", e, w)
	// hoist moves x, a node of the statement s, into a new local of type
	// typ declared in a block around s
	hoist := func(s, x *graph.Node, name, typ string) {
		if v.Failed() {
			return
		}
		blk, err := e.Build(s, "(block (def "+name+" "+typ+" 0) ?s)", graph.Bindings{"s": s})
		if err == nil {
			err = e.Replace(s, blk...)
		}
		var fill []*graph.Node
		if err == nil {
			fill, err = e.Build(x, name, nil)
		}
		if err == nil {
			def := blk[0].Kids[1]
			err = e.MoveTo(x, def.Kids[len(def.Kids)-1], fill[0])
		}
		if err != nil {
			v.Die("%s, computed first -- %v", name, err)
		}
	}
	// the copies of a line with its length
	pat, err := clisp.Pattern("(= ?x (call vim_strnsave ?g ?l))")
	if err != nil {
		return err
	}
	var saves []*graph.Node
	for _, s := range v.Find("(= ?x (call vim_strnsave ?g ?l))") {
		b, _ := graph.Match(pat, s)
		g, l := b["g"], b["l"]
		if b["x"].IsList() || e.Item(s) != s || !g.Is("call") || !l.Is("call") || g.Kids[1].IsList() || l.Kids[1].IsList() ||
			!strings.HasPrefix(g.Kids[1].Atom, "ml_get") || !strings.HasPrefix(l.Kids[1].Atom, "ml_get") {
			continue
		}
		v.Expect(l.Kids[1].Atom == g.Kids[1].Atom+"_len" && sameForms(g.Kids[2:], l.Kids[2:]),
			"a copy of a line whose length is not that line's: %s", graph.Lisp(s))
		saves = append(saves, s)
	}
	if v.Failed() {
		return v.Done()
	}
	if len(saves) != 9 {
		v.Die("the copies of a line compute its length first, as gcc does -- %d copies, expected 9", len(saves))
		return v.Done()
	}
	for _, s := range saves {
		hoist(s, s.Kids[2].Kids[3], "len", "colnr_T")
	}
	if !v.Failed() {
		v.Say("the 9 copies of a line compute its length first, as gcc does")
	}
	v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
		if s := v.One("(call col_print buf2 _ (call ml_get_curline_len) (call linetabsize_str p))", "the column"); s != nil {
			hoist(s, s.Kids[5], "vcol", "int")
		}
	})
	if !v.Failed() {
		v.Say("cursor_pos_info() computes the line's width before its length, as gcc does")
	}
	v.InFunction("fileinfo", func(v *graph.Verbs) {
		if q := v.One(`(? (paren (& (-> curbuf b_flags) BF_NEW)) (call new_file_message) "")`, "the message"); q != nil {
			hoist(e.Item(q), q, "new_msg", "(ptr char)")
		}
	})
	if !v.Failed() {
		v.Say("fileinfo() asks whether the file is new before whether to shorten the modified flag, as gcc does")
	}
	return v.Done()
}

// sameForms says two runs of forms spell the same, ids aside.
func sameForms(a, b []*graph.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].IsList() != b[i].IsList() || a[i].Atom != b[i].Atom || !sameForms(a[i].Kids, b[i].Kids) {
			return false
		}
	}
	return true
}
