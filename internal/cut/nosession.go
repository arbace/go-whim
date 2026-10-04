package cut

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// nosessionStubs are the doors of the autocommand engine and Vim9 that
// answer without it: each body a template, `(return FALSE)` or nothing.
var nosessionStubs = []struct{ name, body string }{
	{"apply_autocmds_group", "(return FALSE)"},
	{"has_autocmd", "(return FALSE)"},
	{"has_cursorhold", "(return FALSE)"},
	{"has_winresized", "(return FALSE)"},
	{"has_winscrolled", "(return FALSE)"},
	{"has_cursormoved", "(return FALSE)"},
	{"has_textchanged", "(return FALSE)"},
	{"has_insertcharpre", "(return FALSE)"},
	{"has_cmdundefined", "(return FALSE)"},
	{"has_tabclosedpre", "(return FALSE)"},
	{"trigger_cursorhold", "(return FALSE)"},
	{"trigger_undo_ftplugin", ""},
	{"trigger_cmd_autocmd", ""},
	{"trigger_winnewpre", ""},
	{"trigger_winclosed", ""},
	{"trigger_tabclosedpre", ""},
	{"may_trigger_win_scrolled_resized", ""},
	{"in_vim9script", "(return FALSE)"},
}

// nosessionDrops are the ifs dropped whole, by their conditions.
var nosessionDrops = []struct{ what, cond string }{
	{"the legacy modifier", `(call checkforcmd_noparen (addr (-> eap cmd)) "legacy" 3)`},
	{"the noautocmd modifier", `(call checkforcmd_noparen (addr (-> eap cmd)) "noautocmd" 3)`},
	{"the sandbox modifier", `(call checkforcmd_noparen (addr (-> eap cmd)) "sandbox" 3)`},
	{"the vim9cmd modifier", `(call checkforcmd_noparen (addr (-> eap cmd)) "vim9cmd" 4)`},
	{"noautocmd saving 'eventignore'", "(&& (paren (& (-> cmod cmod_flags) CMOD_NOAUTOCMD)) (== (-> cmod cmod_save_ei) nullptr))"},
	{"noautocmd restoring 'eventignore'", "(!= (-> cmod cmod_save_ei) nullptr)"},
}

// The parser's half -- -S, -s{file}, -w{file}, -W -- is the command line's
// own, cut with it (argvfront, the reform's D1).

// NoSession removes sessions, autocommands and the Vim9 modifiers.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the stubs by Body, the ifs by
// DropIf on their conditions, the 'eventignorewin' lines by DropCase and
// Cut, the filetype test by FoldNever; the leftover counts the text's on
// the C view (history keeps the text version).
func NoSession(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nosession", e, w)
	stubbed, gone := 0, 0
	v.Muted(func(q *graph.Verbs) {
		for _, s := range nosessionStubs {
			// one only retired commands called is gone with them (exfront,
			// the reform's D2): absent, and named by nothing
			if e.Defn(s.name) == nil && q.Mentions(s.name) == 0 {
				gone++
				continue
			}
			q.Body(s.name, s.body, s.name)
			stubbed++
		}
	})
	v.Sayf("%d doors of the autocommand engine and Vim9 answer "+
		"without it, %d gone with the commands that called them", stubbed, gone)

	for _, d := range nosessionDrops {
		v.DropIf(d.cond, 1, d.what)
	}

	v.InFunction("get_varp", func(v *graph.Verbs) {
		v.DropCase("(case (cast idopt_T (+ PV_WIN (cast int (paren WV_EIW)))))", 1,
			"get_varp() handing out 'eventignorewin'")
	})
	v.InFunction("copy_winopt", func(v *graph.Verbs) {
		v.Cut("(= (-> to wo_eiw) (call copy_option_val (-> from wo_eiw)))", 1,
			"copy_winopt() copying 'eventignorewin'")
	})
	v.InFunction("check_winopt", func(v *graph.Verbs) {
		v.Cut("(call check_string_option (addr (-> wop wo_eiw)))", 1,
			"check_winopt() checking 'eventignorewin'")
	})
	v.InFunction("clear_winopt", func(v *graph.Verbs) {
		v.Cut("(call clear_string_option (addr (-> wop wo_eiw)))", 1,
			"clear_winopt() freeing 'eventignorewin'")
	})

	// :file to a new name re-runs filetype detection when the
	// `filetypedetect` group exists -- a group only :augroup or :autocmd made.
	// The test is known now, and with it goes the last caller of do_doautocmd().
	// set_rw_fname's: do_write's went with :write at phase 1 (filefront, the
	// reform's D4)
	v.Muted(func(v *graph.Verbs) {
		v.InFunction("set_rw_fname", func(v *graph.Verbs) {
			v.FoldNever(`(call au_has_group (cast (ptr char_u) "filetypedetect"))`, 1,
				"filetype detection after a rename")
		})
	})
	v.Say(":write and :file no longer re-detect a filetype no group can detect")
	if v.Failed() {
		return v.Done()
	}

	text := v.Text()
	for _, l := range []struct {
		what, pattern string
		want          int
	}{
		{"the four modifiers in the modifier parser",
			`checkforcmd_noparen\([^,]+, "(legacy|noautocmd|sandbox|vim9cmd)"`, 0},
		{"cmod_save_ei outside its declaration", `\bcmod_save_ei\b`, 1},
		{"scriptout opened by the parser", `\bscripterror\b`, 0},
		// p_lpl is not asserted: this runs at phase 2 (D6), before the
		// collection takes its declaration and its last writes
	} {
		if n := edit.CountMatches(regexp.MustCompile(l.pattern), text); n != l.want {
			v.Die("%s -- %d left, expected %d", l.what, n, l.want)
			return v.Done()
		}
	}

	v.Say("nothing parses a script modifier, suspends from a key, or reads a script file")
	return v.Done()
}
