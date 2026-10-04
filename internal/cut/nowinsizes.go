package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

var winsizeDefaults = []struct{ typ, form, name, value string }{
	{"int", "int", "p_sb", "FALSE"},
	{"int", "int", "p_spr", "FALSE"},
	{"char_u *", "(ptr char_u)", "p_spk", `(char_u *)"cursor"`},
	{"int", "int", "p_ea", "TRUE"},
	{"char_u *", "(ptr char_u)", "p_ead", `(char_u *)"both"`},
	{"long", "long", "p_wh", "1L"},
	{"long", "long", "p_wmh", "1L"},
	{"long", "long", "p_wiw", "20L"},
	{"long", "long", "p_wmw", "1L"},
}

var woWf = regexp.MustCompile(`\bwo_wf[hw]\b`)

// wfOpt is the window's wo_wfh or wo_wfw, through the pointer named.
func wfOpt(p, h string) string { return "(. (-> " + p + " w_onebuf_opt) wo_wf" + h + ")" }

// NoWinSizes fixes the window-size options at their defaults and stops any
// window being fixed against another.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each declaration written anew
// with its default (FRAG, one unit), the folds by condition, the returns
// rewritten, a loop and get_varp's cases cut; the leftover count is the
// text's on the C view (history keeps the text version).
func NoWinSizes(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nowinsizes", e, w)
	v.Together(func(v *graph.Verbs) {
		for _, d := range winsizeDefaults {
			v.ReplaceC("(def static "+d.name+" "+d.form+")",
				"static "+d.typ+" "+d.name+" = "+d.value+";", 1,
				fmt.Sprintf("%s keeps its default, %s", d.name, d.value))
		}
	})

	v.InFunction("win_split_ins", func(v *graph.Verbs) {
		v.FoldNever(wfOpt("oldwin", "w"), 1, "win_split_ins keeping a fixed width")
		v.FoldNever(wfOpt("oldwin", "h"), 1, "win_split_ins keeping a fixed height")
	})
	v.InFunction("winframe_remove", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= (-> frp2 fr_win) nullptr) "+wfOpt("frp2 fr_win", "h")+")", 1,
			"winframe_remove passing over a fixed height")
		v.FoldNever("(&& (!= (-> frp2 fr_win) nullptr) "+wfOpt("frp2 fr_win", "w")+")", 1,
			"winframe_remove passing over a fixed width")
	})
	for _, f := range []struct{ name, h string }{
		{"frame_fixed_height", "h"}, {"frame_fixed_width", "w"},
	} {
		v.InFunction(f.name, func(v *graph.Verbs) {
			v.Rewrite("(return "+wfOpt("frp fr_win", f.h)+")", "(return FALSE)", 1, f.name+" of a window")
		})
	}
	for _, f := range []struct{ name, h, dim string }{
		{"frame_setheight", "h", "height"}, {"frame_setwidth", "w", "width"},
	} {
		v.InFunction(f.name, func(v *graph.Verbs) {
			v.FoldNever("(&& (!= frp curfrp) (!= (-> frp fr_win) nullptr) "+wfOpt("frp fr_win", f.h)+")", 1,
				fmt.Sprintf("frame_set%s reserving a fixed %s", f.dim, f.dim))
			v.FoldNever("(&& (> room_reserved 0) (!= (-> frp fr_win) nullptr) "+wfOpt("frp fr_win", f.h)+")", 1,
				fmt.Sprintf("frame_set%s sparing a fixed %s", f.dim, f.dim))
		})
	}
	v.InFunction("command_height", func(v *graph.Verbs) {
		v.Cut("(while (&& (!= (-> frp fr_prev) nullptr) (== (-> frp fr_layout) FR_LEAF) "+wfOpt("frp fr_win", "h")+
			") (block (= frp (-> frp fr_prev))))", 1, "command_height stepping over fixed heights")
	})
	v.InFunction("win_enter_ext", func(v *graph.Verbs) {
		v.DropOperand("(! "+wfOpt("curwin", "h")+")", 1, "win_enter_ext sparing a fixed height")
		v.DropOperand("(! "+wfOpt("curwin", "w")+")", 1, "win_enter_ext sparing a fixed width")
	})
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.DropIf("(&& (call bt_quickfix buf) win_valid (== (-> win w_buffer) buf))", 1,
			"close_buffer clearing a quickfix height")
	})
	v.InFunction("get_varp", func(v *graph.Verbs) {
		for _, x := range []struct{ wv, fld string }{{"WFH", "wfh"}, {"WFW", "wfw"}} {
			v.CutRun("get_varp for WV_"+x.wv,
				"(case (cast idopt_T (+ PV_WIN (cast int (paren WV_"+x.wv+")))))",
				"(return (cast (ptr char_u) (addr (paren (. (-> curwin w_onebuf_opt) wo_"+x.fld+")))))")
		}
	})
	if v.Failed() {
		return v.Done()
	}
	if n := len(woWf.FindAll(v.Text(), -1)); n != 2 {
		return fmt.Errorf("nowinsizes: wo_wfh and wo_wfw outside their declarations "+
			"-- %d mentions, expected 2", n)
	}
	v.Say("the sizes are fixed at their defaults, and no window is fixed against another")
	return v.Done()
}
