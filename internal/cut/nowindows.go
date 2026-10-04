package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

var (
	nwSplitMod = regexp.MustCompile(`cmod->cmod_split \|=`)
	nwNvWindow = regexp.MustCompile(`\{Ctrl_W, nv_window`)
)

// nwCheck is checkforcmd_noparen on the modifier's name.
func nwCheck(name string, n int) string {
	return fmt.Sprintf(`(call checkforcmd_noparen (addr (-> eap cmd)) %q %d)`, name, n)
}

// nwOnly is a case of the modifier parser that is one split modifier: its
// label, the refusal of any other word, the flag and the continue.
func nwOnly(label, name string, n int, flag string) []string {
	return []string{"(case '" + label + "')", "(if (! " + nwCheck(name, n) + ") (block (break)))",
		"(|= (-> cmod cmod_split) " + flag + ")", "(continue)"}
}

// NoWindows leaves one window: nothing makes, reaches, resizes or binds a
// second.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's line regexps and
// literals are runs of items cut or replaced, folds and drops by
// condition, operands dropped, the CTRL-W row's handler pointed at
// nv_error; the leftover check is the text's own regexps on the C view
// (history keeps the text version).
func NoWindows(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nowindows", e, w)

	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.CutRun(":aboveleft", nwOnly("a", "aboveleft", 3, "WSP_ABOVE")...)
		v.CutRun(":belowright and :botright", "(case 'b')",
			"(if "+nwCheck("belowright", 3)+" (block (|= (-> cmod cmod_split) WSP_BELOW) (continue)))",
			"(if (! "+nwCheck("botright", 2)+") (block (break)))",
			"(|= (-> cmod cmod_split) WSP_BOT)", "(continue)")
		v.Cut("(if "+nwCheck("horizontal", 3)+" (block (|= (-> cmod cmod_split) WSP_HOR) (continue)))", 1,
			":horizontal")
		if run := v.Run(":leftabove", nwOnly("l", "leftabove", 5, "WSP_ABOVE")[1:]...); run != nil {
			nwReplaceRun(v, run, "(break)", ":leftabove")
		}
		v.CutRun(":rightbelow", nwOnly("r", "rightbelow", 6, "WSP_BELOW")...)
		v.CutRun(":topleft", nwOnly("t", "topleft", 2, "WSP_TOP")...)
		v.Cut("(if "+nwCheck("vertical", 4)+" (block (|= (-> cmod cmod_split) WSP_VERT) (continue)))", 1,
			":vertical")
	})
	v.InFunction("has_cmdmod", func(v *graph.Verbs) {
		v.DropOperand("(!= (-> cmod cmod_split) 0)", 1, "has_cmdmod counting a split modifier")
	})

	// A ROW IS NEVER DELETED FROM nv_cmds[], IT IS POINTED AT nv_error.
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init Ctrl_W ?h 0 0)", "h", "nv_error", 1, "CTRL-W's row points at nv_error")
	})

	v.InFunction("nv_record", func(v *graph.Verbs) {
		v.FoldNever("(|| (== (-> cap nchar) ':') (== (-> cap nchar) '/') (== (-> cap nchar) '?'))", 1,
			"q: q/ q? opening it")
	})
	v.InFunction("getcmdline_int", func(v *graph.Verbs) {
		v.FoldNever("(|| (== c cedit_key) (== c _))", 1, "CTRL-F on the command line opening it")
	})
	for _, name := range []string{"ex_quit", "text_locked", "get_text_locked_msg", "nv_normal"} {
		v.InFunction(name, func(v *graph.Verbs) {
			v.FoldNever("(!= cmdwin_type 0)", 1, name+" asking whether it is open")
		})
	}
	v.InFunction("edit", func(v *graph.Verbs) {
		v.FoldNever("(&& (== c Ctrl_C) (!= cmdwin_type 0))", 1, "insert-mode CTRL-C closing it")
		v.FoldNever("(!= cmdwin_type 0)", 1, "insert-mode Enter executing it")
		v.FoldNever("(. (-> curwin w_onebuf_opt) wo_scb)", 1, "insert mode's 'scrollbind'")
		v.FoldNever("(. (-> curwin w_onebuf_opt) wo_crb)", 1, "insert mode's 'cursorbind'")
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= cmdwin_type 0) (! (& (. ea argt) EX_CMDWIN)))", 1, "commands refused inside it")
	})
	v.InFunction("goto_tabpage_tp", func(v *graph.Verbs) {
		v.DropIf("(|| trigger_enter_autocmds trigger_leave_autocmds)", 1, "goto_tabpage_tp refusing inside it")
	})
	v.InFunction("nv_down", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= cmdwin_type 0) (== (-> cap cmdchar) CAR))", 1, "normal-mode Enter executing it")
	})
	v.InFunction("nv_esc", func(v *graph.Verbs) {
		v.DropOperand("(== cmdwin_type 0)", 1, "nv_esc asking before the abandon hint")
		v.FoldNever("(!= cmdwin_type 0)", 1, "normal-mode Esc closing it")
		v.FoldNever("(&& (!= cmdwin_type 0) ex_normal_busy typebuf_was_empty)", 1, ":normal Esc closing it")
	})
	v.InFunction("vgetorpeek", func(v *graph.Verbs) {
		v.DropOperand("(paren (&& (> cmdwin_type 0) (== tc ESC)))", 1, "an interrupted Esc closing it")
		// tc remembered the previous key for that test alone.  Its store goes
		// here and its declaration is the sweep's.
		v.Cut("(= tc c)", 1, "vgetorpeek remembering it")
	})
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.CutRun("do_ecmd hiding it", "(def save_cmdwin_type int cmdwin_type)",
			"(def save_cmdwin_win (ptr win_T) cmdwin_win)", "(= cmdwin_type 0)", "(= cmdwin_win nullptr)")
		v.CutRun("do_ecmd restoring it", "(= cmdwin_type save_cmdwin_type)", "(= cmdwin_win save_cmdwin_win)")
	})
	v.InFunction("win_line", func(v *graph.Verbs) {
		v.FoldNever("(== wp cmdwin_win)", 1, "its column drawn")
	})
	v.InFunction("win_col_off", func(v *graph.Verbs) {
		v.Rewrite("(+ ?a (? (!= wp cmdwin_win) 0 1))", "(paren ?a)", 1, "its column counted")
	})
	v.InFunction("buf_spname", func(v *graph.Verbs) {
		v.FoldNever("(== buf cmdwin_buf)", 1, "its buffer name")
	})
	v.InFunction("comp_textwidth", func(v *graph.Verbs) {
		v.FoldNever("(== curbuf cmdwin_buf)", 1, "its textwidth")
	})
	v.InFunction("main_loop", func(v *graph.Verbs) {
		v.DropOperand("(== cmdwin_result 0)", 1, "main_loop waiting for its result")
	})
	v.InFunction("didset_options", func(v *graph.Verbs) {
		v.Cut("(cast void (call did_set_cedit nullptr))", 1, "startup reading 'cedit'")
	})
	v.InFunction("check_num_option_bounds", func(v *graph.Verbs) {
		v.DropIf("(< p_cwh 1)", 1, "'cmdwinheight' clamped")
	})

	// -o and -O are the command line's own, cut with it (argvfront, the
	// reform's D1)
	v.InFunction("create_windows", func(v *graph.Verbs) {
		v.DropIf("(== (-> parmp window_count) (- 1))", 1, "create_windows defaulting the count")
		v.DropIf("(== (-> parmp window_count) 0)", 1, "create_windows counting the files")
		v.FoldNever("(> (-> parmp window_count) 1)", 1, "create_windows making a window per file")
		v.Cut("(= (-> parmp window_count) 1)", 1, "create_windows settling on one")
	})
	v.Cut("(call edit_buffers _ _)", 1, "startup editing a file in each window")
	v.Cut("(= (. params window_count) (- 1))", 1, "main initialising the window count")

	// 14: four more died with the commands retired at phase 1 (D2)
	if !v.Failed() {
		q := graph.NewVerbs("nowindows", e, io.Discard)
		n := 0
		for _, f := range []string{"scb", "crb"} {
			for _, x := range []string{"curwin", "wp"} {
				pat := "(= (. (-> (paren " + x + ") w_onebuf_opt) wo_" + f + ") FALSE)"
				c := q.Count(pat)
				n += c
				if c > 0 {
					q.Cut(pat, c, "")
				}
			}
		}
		if n != 14 {
			v.Die("every assignment of 'scrollbind' and 'cursorbind' -- expected 14, matched %d", n)
		} else if q.Err != nil {
			v.Err = q.Err
		} else {
			v.Say("every assignment of 'scrollbind' and 'cursorbind'")
		}
	}
	v.InFunction("get_varp", func(v *graph.Verbs) {
		for _, x := range []struct{ wv, fld string }{{"SCBIND", "scb"}, {"CRBIND", "crb"}, {"WFB", "wfb"}} {
			v.CutRun("get_varp for WV_"+x.wv,
				"(case (cast idopt_T (+ PV_WIN (cast int (paren WV_"+x.wv+")))))",
				"(return (cast (ptr char_u) (addr (paren (. (-> curwin w_onebuf_opt) wo_"+x.fld+")))))")
		}
	})
	v.InFunction("copy_winopt", func(v *graph.Verbs) {
		v.CutRun("copy_winopt copying 'scrollbind'",
			"(= (-> to wo_scb) (-> from wo_scb))", "(= (-> to wo_scb_save) (-> from wo_scb_save))")
		v.CutRun("copy_winopt copying 'cursorbind'",
			"(= (-> to wo_crb) (-> from wo_crb))", "(= (-> to wo_crb_save) (-> from wo_crb_save))")
	})
	v.InFunction("normal_cmd", func(v *graph.Verbs) {
		v.FoldNever("(&& (. (-> curwin w_onebuf_opt) wo_scb) toplevel)", 1, "normal mode's 'scrollbind'")
		v.FoldNever("(&& (. (-> curwin w_onebuf_opt) wo_crb) toplevel)", 1, "normal mode's 'cursorbind'")
	})
	v.InFunction("ex_substitute", func(v *graph.Verbs) {
		v.FoldNever("(. (-> curwin w_onebuf_opt) wo_crb)", 1, ":s's 'cursorbind'")
	})
	v.InFunction("set_shellsize_inner", func(v *graph.Verbs) {
		v.FoldNever("(. (-> curwin w_onebuf_opt) wo_scb)", 1, "a resize's 'scrollbind'")
	})
	v.InFunction("scroll_to_fraction", func(v *graph.Verbs) {
		v.DropOperand("(|| (! (. (-> wp w_onebuf_opt) wo_scb)) (== wp curwin))", 1,
			"scroll_to_fraction's 'scrollbind'")
	})
	v.InFunction("check_can_set_curbuf_disabled", func(v *graph.Verbs) {
		v.FoldNever("(. (-> curwin w_onebuf_opt) wo_wfb)", 1, "check_can_set_curbuf_disabled's 'winfixbuf'")
	})
	v.InFunction("check_can_set_curbuf_forceit", func(v *graph.Verbs) {
		v.FoldNever("(&& (! forceit) (. (-> curwin w_onebuf_opt) wo_wfb))", 1,
			"check_can_set_curbuf_forceit's 'winfixbuf'")
	})

	// :drop, do_argfile, goto_buffer and do_buffer_ext splitting a window,
	// before_quit_all asking about the command-line window, and :windo's walk
	// in ex_listdo died with the commands that reached them, retired at phase
	// 1 (exfront, the reform's D2)
	v.InFunction("buflist_getfile", func(v *graph.Verbs) {
		v.DropIf("(& options GETF_SWITCH)", 1, "buflist_getfile's 'switchbuf'")
	})
	v.Cut("(cast void (call opt_strings_flags p_swb p_swb_values (addr swb_flags) TRUE))", 1,
		"didset_string_options reading 'switchbuf'")
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		v.Cut("(case CMD_windo)", 1, "completion for :windo")
	})
	if v.Failed() {
		return v.Done()
	}

	text := v.Text()
	var left []string
	for _, c := range []struct {
		what string
		n    int
		want int
	}{
		{"a split modifier in the parser", len(nwSplitMod.FindAll(text, -1)), 0},
		{"nv_window in the key table", len(nwNvWindow.FindAll(text, -1)), 0},
		// DOBUF_SPLIT and CMD_windo are not counted: this runs at phase 1 (the
		// reform's D9), where the retired commands' handlers still name them
	} {
		if c.n != c.want {
			left = append(left, fmt.Sprintf("(%s, %d)", edit.PyRepr(c.what), c.n))
		}
	}
	if len(left) > 0 {
		return fmt.Errorf("nowindows: still present: [%s]", strings.Join(left, ", "))
	}
	v.Say("nothing makes, reaches, resizes or binds a second window")
	return v.Done()
}

// nwReplaceRun replaces a run of items by a template's, reported.
func nwReplaceRun(v *graph.Verbs, run []*graph.Node, tmpl, what string) {
	e := v.Editor()
	with, err := e.Build(run[0], tmpl, nil)
	if err == nil {
		err = e.ReplaceRun(run[0], run[len(run)-1], with...)
	}
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}
