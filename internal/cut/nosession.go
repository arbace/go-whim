package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
)

// The four tables are GENERATED from the Python module's own, imported rather
// than retyped: PARSER's third entry carries `\"` and `\n` inside C string
// literals, and transcribing those by hand is how a cut stops matching for a
// reason nobody can see in a diff.
var nosessionStubs = []struct{ name, body string }{
	{"apply_autocmds_group", "    return FALSE;\n"},
	{"has_autocmd", "    return FALSE;\n"},
	{"has_cursorhold", "    return FALSE;\n"},
	{"has_winresized", "    return FALSE;\n"},
	{"has_winscrolled", "    return FALSE;\n"},
	{"has_cursormoved", "    return FALSE;\n"},
	{"has_textchanged", "    return FALSE;\n"},
	{"has_insertcharpre", "    return FALSE;\n"},
	{"has_cmdundefined", "    return FALSE;\n"},
	{"has_tabclosedpre", "    return FALSE;\n"},
	{"trigger_cursorhold", "    return FALSE;\n"},
	{"trigger_undo_ftplugin", ""},
	{"trigger_cmd_autocmd", ""},
	{"trigger_winnewpre", ""},
	{"trigger_winclosed", ""},
	{"trigger_tabclosedpre", ""},
	{"may_trigger_win_scrolled_resized", ""},
	{"in_vim9script", "    return FALSE;\n"},
}

var nosessionDrops = []struct{ what, pat string }{
	{"the legacy modifier", edit.Head(`if (checkforcmd_noparen(&eap->cmd, "legacy", 3))`)},
	{"the noautocmd modifier", edit.Head(`if (checkforcmd_noparen(&eap->cmd, "noautocmd", 3))`)},
	{"the sandbox modifier", edit.Head(`if (checkforcmd_noparen(&eap->cmd, "sandbox", 3))`)},
	{"the vim9cmd modifier", edit.Head(`if (checkforcmd_noparen(&eap->cmd, "vim9cmd", 4))`)},
	{"noautocmd saving 'eventignore'", edit.Head("if ((cmod->cmod_flags & CMOD_NOAUTOCMD) && cmod->cmod_save_ei == nullptr)")},
	{"noautocmd restoring 'eventignore'", edit.Head("if (cmod->cmod_save_ei != nullptr)")},
}

var nosessionLiteral = []struct{ what, old, new string }{
	{"get_varp() handing out 'eventignorewin'", "    case (idopt_T)(PV_WIN + (int)(WV_EIW)):\n        return (char_u *)&(curwin->w_onebuf_opt.wo_eiw);\n", ""},
	{"copy_winopt() copying 'eventignorewin'", "    to->wo_eiw = copy_option_val(from->wo_eiw);\n", ""},
	{"check_winopt() checking 'eventignorewin'", "    check_string_option(&wop->wo_eiw);\n", ""},
	{"clear_winopt() freeing 'eventignorewin'", "    clear_string_option(&wop->wo_eiw);\n", ""},
}

// The parser's half -- -S, -s{file}, -w{file}, -W -- is the command line's
// own, cut with it (argvfront, the reform's D1).

// nosessionBody replaces a definition's body, scoped to the definition's own
// span -- the first `{` inside it is the body opener.
func nosessionBody(text []byte, name, newBody string) ([]byte, error) {
	a, z, ok := edit.FindDefinition(text, edit.Blank(text), name)
	if !ok {
		return nil, fmt.Errorf("nosession: %s is not defined at file scope", name)
	}
	seg := text[a:z]
	b := edit.Blank(seg)
	o := bytes.IndexByte(b, '{')
	c := edit.Match(b, o)
	if o < 0 || c < 0 {
		return nil, fmt.Errorf("nosession: %s is unbalanced", name)
	}
	out := make([]byte, 0, len(text))
	out = append(out, text[:a]...)
	out = append(out, seg[:o]...)
	out = append(out, "{\n"...)
	out = append(out, newBody...)
	out = append(out, '}')
	out = append(out, seg[c+1:]...)
	return append(out, text[z:]...), nil
}

// NoSession removes sessions, autocommands and the Vim9 modifiers.
func NoSession(text []byte, w io.Writer) ([]byte, error) {
	var err error
	stubbed, gone := 0, 0
	for _, s := range nosessionStubs {
		// one only retired commands called is gone with them (exfront, the
		// reform's D2): absent, and named by nothing
		if _, _, ok := edit.FindDefinition(text, edit.Blank(text), s.name); !ok && edit.MentionCount(text, s.name) == 0 {
			gone++
			continue
		}
		if text, err = nosessionBody(text, s.name, s.body); err != nil {
			return nil, err
		}
		stubbed++
	}
	fmt.Fprintf(w, "  nosession    %d doors of the autocommand engine and Vim9 answer "+
		"without it, %d gone with the commands that called them\n", stubbed, gone)

	for _, d := range nosessionDrops {
		n := len(regexp.MustCompile(d.pat).FindAll(text, -1))
		if n != 1 {
			return nil, fmt.Errorf("nosession: %s -- matches %d times, not once", d.what, n)
		}
		if text, err = edit.DropIf(text, d.pat, 1); err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  nosession    %s\n", d.what)
	}

	for _, l := range nosessionLiteral {
		n := bytes.Count(text, []byte(l.old))
		if n != 1 {
			return nil, fmt.Errorf("nosession: %s -- occurs %d times, not once", l.what, n)
		}
		text = bytes.ReplaceAll(text, []byte(l.old), []byte(l.new))
		fmt.Fprintf(w, "  nosession    %s\n", l.what)
	}

	// :file to a new name re-runs filetype detection when the
	// `filetypedetect` group exists -- a group only :augroup or :autocmd made.
	// The test is known now, and with it goes the last caller of do_doautocmd().
	// set_rw_fname's: do_write's went with :write at phase 1 (filefront, the
	// reform's D4), and is still in the unswept text this runs on there (D6)
	var found bool
	if text, found, err = edit.InDefinition(text, "set_rw_fname", func(b []byte) ([]byte, error) {
		return edit.FoldNever(b, edit.Head(`if (au_has_group((char_u *)"filetypedetect"))`), 1)
	}); err != nil || !found {
		return nil, fmt.Errorf("nosession: filetype detection after a rename -- %v (set_rw_fname found: %v)", err, found)
	}
	fmt.Fprintln(w, "  nosession    :write and :file no longer re-detect a filetype no "+
		"group can detect")

	for _, l := range []struct {
		what, pattern string
		want          int
	}{
		{"the four modifiers in the modifier parser",
			`checkforcmd_noparen\([^,]+, "(legacy|noautocmd|sandbox|vim9cmd)"`, 0},
		{"cmod_save_ei outside its declaration", `\bcmod_save_ei\b`, 1},
		{"scriptout opened by the parser", `\bscripterror\b`, 0},
		// p_lpl is not asserted: this runs at phase 1 (D6), before the sweep
		// takes its declaration and its last writes
	} {
		n := len(regexp.MustCompile(l.pattern).FindAll(text, -1))
		if n != l.want {
			return nil, fmt.Errorf("nosession: %s -- %d left, expected %d",
				l.what, n, l.want)
		}
	}

	fmt.Fprintln(w, "  nosession    nothing parses a script modifier, suspends from a key, "+
		"or reads a script file")
	return text, nil
}
