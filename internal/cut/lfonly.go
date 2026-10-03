package cut

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// lfonlyDying are the format functions and the option callbacks whose rows
// lfonly drops (record 50's cut).  Every call left must sit inside one of them.
var lfonlyDying = []string{
	"get_fileformat", "get_fileformat_force", "set_fileformat", "default_fileformat",
	"file_ff_differs", "save_file_ff", "set_file_options", "set_options_bin",
	"msg_add_fileformat", "check_ff_value", "did_set_binary", "did_set_fileformat",
	"did_set_fileformats", "did_set_textmode", "did_set_textauto",
	"did_set_eof_eol_fixeol_bomb",
}

// LfOnly makes every line end with LF, read and written.
//
// readfile went with open_buffer's read arms, and its other callers, by
// phase 1 (readfront, phase 31's move): every format it chose on reading
// went with it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the text version's acts, one
// for one and in its order (history keeps it), each found by its form and
// counted; where the text took several lines with one counted pattern
// (get_varp's two cases, buf_clear_file's four stores, buf_copy_options'
// three), the acts are made one by one and reported once.  Its last check
// is the edges': every call of a dying function left sits inside one.
func LfOnly(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("lfonly", e, w)
	// quiet is v's acts unreported, for a group the text reported as one
	quiet := func(acts func(q *graph.Verbs)) {
		if v.Failed() {
			return
		}
		q := graph.NewVerbs("lfonly", e, io.Discard)
		acts(q)
		if q.Err != nil {
			v.Err = q.Err
		}
	}

	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.Cut("(if (&& (!= eap nullptr) (!= (-> eap force_bin) 0)) (block (= write_bin _)) (block (= write_bin (-> buf b_p_bin))))", 1,
			"buf_write choosing 'binary' or ++bin")
		v.Cut("(= fileformat (call get_fileformat_force buf eap))", 1, "buf_write choosing a format")
		v.FoldNever("(&& (== c CAR) (== fileformat EOL_MAC))", 1, "buf_write writing CR as a line end")
		v.Rewrite("(|| ?a (paren (&& (== lnum end) _*)))", "?a", 1,
			"buf_write leaving the last LF off")
		v.FoldAlwaysElse("(== fileformat EOL_UNIX)", 1, "buf_write writing CR LF or CR")
		v.FoldNever("(&& (! (-> buf b_p_fixeol)) (-> buf b_p_eof))", 1, "buf_write appending CTRL-Z")
		v.DropIf("(call msg_add_fileformat fileformat)", 1, `the "[dos]" and "[mac]" write messages`)
	})

	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.Cut("(call save_file_ff curbuf)", 1, "open_buffer saving the format")
	})
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Cut("(call set_file_options TRUE eap)", 1, "do_ecmd setting ++ff and ++bin")
	})
	quiet(func(q *graph.Verbs) {
		q.InFunction("get_varp", func(q *graph.Verbs) {
			for _, bv := range []string{"BV_EOL", "BV_EOF"} {
				q.DropCase("(case (cast idopt_T (+ PV_BUF (cast int (paren "+bv+")))))", 1,
					"get_varp handing out 'endofline' and 'endoffile'")
			}
		})
	})
	v.Say("get_varp handing out 'endofline' and 'endoffile'")
	v.Cut("(= (-> curbuf b_no_eol_lnum) 0)", 1, "resetting the no-LF line for 'binary'")
	v.InFunction("set_init_1", func(v *graph.Verbs) {
		v.Cut("(call save_file_ff curbuf)", 1, "startup saving the format of the first buffer")
	})
	v.InFunction("did_set_modified", func(v *graph.Verbs) {
		v.DropIf("(! (. (-> args os_newval) boolean))", 1, "'nomodified' saving the format")
	})

	v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
		v.FoldNever("(== (call get_fileformat curbuf) EOL_DOS)", 1, "g CTRL-G counting CR LF as two bytes")
		v.FoldNever("(&& (== lnum (. (-> curbuf b_ml) ml_line_count)) (! (-> curbuf b_p_eol)) (|| (-> curbuf b_p_bin) (! (-> curbuf b_p_fixeol))) _)", 1,
			"g CTRL-G at a last line with no LF")
		v.FoldNever("(&& (! (-> curbuf b_p_eol)) (|| (-> curbuf b_p_bin) (! (-> curbuf b_p_fixeol))))", 1,
			"g CTRL-G counting a missing last LF")
	})

	v.InFunction("unchanged", func(v *graph.Verbs) {
		v.Rewrite("(|| ?a (paren (&& ff (call file_ff_differs buf FALSE))))", "?a", 1,
			"a changed format counting as a change")
		v.DropIf("ff", 1, "unchanged saving the format")
	})
	v.InFunction("bufIsChangedNotTerm", func(v *graph.Verbs) {
		// the text kept its parentheses: `(buf->b_changed)`
		v.Rewrite("(|| ?a (call file_ff_differs buf TRUE))", "(paren ?a)", 1, "a changed format counting as changed")
	})
	quiet(func(q *graph.Verbs) {
		q.InFunction("buf_clear_file", func(q *graph.Verbs) {
			for _, m := range []string{"b_p_eof", "b_start_eof", "b_p_eol", "b_start_eol"} {
				q.Cut("(= (-> buf "+m+") _)", 1, "buf_clear_file resetting 'endofline' and 'endoffile'")
			}
		})
	})
	v.Say("buf_clear_file resetting 'endofline' and 'endoffile'")
	v.InFunction("transchar_nonprint", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= buf nullptr) (== c CAR) (== (call get_fileformat buf) EOL_MAC))", 1, "CR shown as a line end")
	})
	v.InFunction("do_ascii", func(v *graph.Verbs) {
		v.FoldNever("(&& (== c CAR) (== (call get_fileformat curbuf) EOL_MAC))", 1, "ga showing CR as a line end")
	})
	v.InFunction("ml_open", func(v *graph.Verbs) {
		v.Cut("(= (index (-> b0p b0_fname) (- B0_FNAME_SIZE_ORG 2)) (+ (call get_fileformat buf) 1))", 1,
			"block 0 recording the format")
	})
	v.InFunction("ml_setflags", func(v *graph.Verbs) {
		v.Cut("(= (index (-> b0p b0_fname) (- B0_FNAME_SIZE_ORG 2)) _)", 1, "block 0 updating the format")
	})
	v.InFunction("set_init_3", func(v *graph.Verbs) {
		v.DropIf("(paren (&& (== (. (-> curbuf b_ml) ml_line_count) 1) (== (deref (call ml_get (cast linenr_T 1))) NUL)))", 1,
			"startup applying 'fileformats' to an empty buffer")
	})

	strncmp := func(s string, n int) string {
		return fmt.Sprintf(`(== (call strncmp (cast (ptr char) (paren arg)) (cast (ptr char) (paren "%s")) (paren %d)) 0)`, s, n)
	}
	v.InFunction("getargopt", func(v *graph.Verbs) {
		v.DropIf("(|| "+strncmp("bin", 3)+" "+strncmp("nobin", 5)+")", 1, "++bin and ++nobin")
		v.FoldNever(strncmp("ff", 2), 1, "++ff")
		v.FoldNever(strncmp("fileformat", 10), 1, "++fileformat")
		v.FoldNever("(== pp (addr (-> eap force_ff)))", 1, "++ff checking its value")
	})
	v.InFunction("prepare_help_buffer", func(v *graph.Verbs) {
		v.Cut("(= (-> curbuf b_p_bin) FALSE)", 1, "the help buffer clearing 'binary'")
	})

	v.InFunction("buf_copy_options", func(v *graph.Verbs) {
		v.Cut("(switch (deref p_ffs) _*)", 1, "a new buffer's 'fileformat' from 'fileformats'")
		v.DropIf("(!= (-> buf b_p_ff) nullptr)", 1, "a new buffer's remembered format")
	})
	quiet(func(q *graph.Verbs) {
		q.InFunction("buf_copy_options", func(q *graph.Verbs) {
			for _, o := range []string{"tw", "wm", "et"} {
				q.Cut("(= (-> buf b_p_"+o+"_nobin) p_"+o+"_nobin)", 1, "a new buffer's values saved for 'binary'")
			}
		})
	})
	v.Say("a new buffer's values saved for 'binary'")

	// Every call left must be inside code the sweep takes with them.  Counted
	// by WHERE each call sits, not by a tally that has to be guessed.
	if v.Failed() {
		return v.Done()
	}
	var live []string
	for _, n := range lfonlyDying {
		for _, u := range v.UsesOutside(n, lfonlyDying...) {
			if c := e.Parent(u); c == nil || !c.Is("call") || c.Kids[1] != u {
				continue // not a call: an option row's pointer
			}
			owner := "?"
			if f := e.Function(u); f != nil {
				owner = graph.DeclName(f)
			}
			live = append(live, n+" in "+owner)
		}
	}
	if len(live) > 0 {
		return fmt.Errorf("lfonly: still called from live code: %s", strings.Join(live, ", "))
	}

	v.Say("every line ends with LF, read and written")
	return v.Done()
}
