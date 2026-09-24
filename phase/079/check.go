package p079

// Whim phase 79, the check -- the constant-return predicates.
// See phase/079/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim79", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim79", args)
	if err != nil {
		return err
	}
	const p = "  noconstfn    "
	if !s.Cnt0(p, "in_vim9script", "tabline_height", "pum_visible", "current_win_nr", "current_tab_nr",
		"check_more", "only_one_window", "check_timestamps", "stl_connected", "pum_under_menu",
		"ins_compl_win_active", "ins_compl_lnum_in_range", "ins_compl_active",
		"has_cursormoved", "wc_use_keyname", "script_get", "pum_redraw_in_same_position",
		"has_textchanged", "has_insertcharpre", "get_cellwidth",
		"check_can_set_curbuf_forceit", "check_can_set_curbuf_disabled",
		"bt_terminal", "bt_quickfix", "bomb_size", "at_ins_compl_key", "append_arg_number",
		"skip_for_popup", "may_have_range", "need_check_timestamps") {
		return harness.ErrReported
	}
	nums, ls := check.GrepLines(s.Src(), `\bvim9script\b`, check.GERE)
	keep := check.Gre(`^(// |    \[CMD_vim9script\] = )`, check.GERE)
	var stray []string
	for i := range nums {
		if !keep.MatchString(ls[i]) {
			stray = append(stray, fmt.Sprintf("%d:%s", nums[i], ls[i]))
		}
	}
	if len(stray) > 0 {
		s.Echo("  noconstfn    the vim9script identifier survives outside the banners and the command row")
		skip := check.Gre(`:(// |    \[CMD_vim9script\] = )`, check.GERE)
		for i := range nums {
			if l := fmt.Sprintf("%d:%s", nums[i], ls[i]); !skip.MatchString(l) {
				s.Echo("%s", l)
			}
		}
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- it returns a variable and must stay", "get_hislen", "is_maphash_valid", "get_search_pat", "get_text_locked_msg") {
		return harness.ErrReported
	}
	// THE TWO ROWS THAT SHARE THE FUNCTION, 'number' and 'relativenumber'.  A
	// row of options[] was several lines in the residue and is one line now, so
	// the function and the NULL beside it are read inside the row rather than
	// as a line of their own.  Measured: two rows either way, the same two.
	if n := check.GrepC(s.Src(), `^[ \t]*\{"(number|relativenumber)",.*did_set_number_relativenumber, NULL,`, check.GERE); n != 2 {
		s.Echo("  noconstfn    the two did_set_number_relativenumber option rows are %d, expected 2", n)
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bdid_set_number_relativenumber\(optset_T`, check.GERE) {
		s.Echo("  noconstfn    did_set_number_relativenumber lost its definition")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- :q and :wq still need it", "getout", "ex_quit", "ex_exit", "check_changed", "check_changed_any", "before_quit_autocmds",
		"not_exiting", "do_write", "curbufIsChanged") {
		return harness.ErrReported
	}
	for _, fn := range []string{"ex_quit", "ex_exit"} {
		Body := s.FnBody(`^` + fn + `\(exarg_T`)
		if n := check.GrepC(Body, `getout\(0\);`, check.GERE); n != 1 {
			s.Echo("  noconstfn    %s has %d getout(0) calls, expected 1", fn, n)
			return harness.ErrReported
		}
		if n := check.GrepC(Body, `not_exiting\(save_exiting\);`, check.GERE); n != 2 {
			s.Echo("  noconstfn    %s has %d not_exiting calls, expected 2", fn, n)
			return harness.ErrReported
		}
	}
	s.Echo("  noconstfn    28 constants folded, 4 variable-returning stubs intact, quit guards intact")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	quit := func(file, add string, args ...string) (int, string) {
		f := filepath.Join(d, file)
		check.Put(f, "a\nb\n")
		rc := check.InD(d, append(append([]string{"-e", "-s"}, args...), file)...)
		_ = add
		return rc, check.Bar(f)
	}
	if rc, b := quit("q1.txt", "", "+q"); rc != 0 {
		s.Echo("  noconstfn    :q on an unmodified file exited %d, expected 0", rc)
		return harness.ErrReported
	} else if b != "a|b|" {
		s.Echo("  noconstfn    :q changed the file")
		return harness.ErrReported
	}
	if rc, b := quit("q2.txt", "", "+normal! A-mod", "+q"); rc != 1 {
		s.Echo("  noconstfn    :q on a MODIFIED file exited %d, expected 1 -- it must refuse", rc)
		return harness.ErrReported
	} else if b != "a|b|" {
		s.Echo("  noconstfn    :q wrote a modified file it should have refused")
		return harness.ErrReported
	}
	if rc, b := quit("q3.txt", "", "+normal! A-mod", "+q!"); rc != 0 {
		s.Echo("  noconstfn    :q! exited %d, expected 0", rc)
		return harness.ErrReported
	} else if b != "a|b|" {
		s.Echo("  noconstfn    :q! wrote the file")
		return harness.ErrReported
	}
	if rc, b := quit("q4.txt", "", "+1", "+normal! A-wq", "+wq"); rc != 0 {
		s.Echo("  noconstfn    :wq exited %d, expected 0", rc)
		return harness.ErrReported
	} else if b != "a-wq|b|" {
		s.Echo("  noconstfn    :wq did not write: '%s'", b)
		return harness.ErrReported
	}
	if rc, b := quit("q5.txt", "", "+1", "+normal! A-x", "+x"); rc != 0 {
		s.Echo("  noconstfn    :x exited %d, expected 0", rc)
		return harness.ErrReported
	} else if b != "a-x|b|" {
		s.Echo("  noconstfn    :x did not write: '%s'", b)
		return harness.ErrReported
	}
	s.Echo("  noconstfn    :q refuses a modified file, :q! discards, :wq and :x write and exit")
	t := check.ProbeFile(d, "t.txt", "a\nb\nc\n", "+$", "+s/^/LAST /", "+wq")
	if check.Bar(t) != "a|b|LAST c|" {
		s.Echo("  noconstfn    a range and a substitution broke: '%s'", check.Bar(t))
		return harness.ErrReported
	}
	g := check.ProbeFile(d, "g.txt", "k1\ndrop\nk2\n", "+g/drop/d", "+wq")
	if check.Bar(g) != "k1|k2|" {
		s.Echo("  noconstfn    :g broke: '%s'", check.Bar(g))
		return harness.ErrReported
	}
	sw := check.ProbeFile(d, "sw.txt", "o1\n", "+set shiftwidth=8", "+normal! >>", "+wq")
	if check.CatS(sw) != "        o1" {
		s.Echo("  noconstfn    :set and >> broke: '%s'", check.CatS(sw))
		return harness.ErrReported
	}
	for _, c := range []struct {
		file, content, want, What string
		Args                      []string
	}{
		{"w.txt", "q1\nq2\n", "q1-e|q2|", "writing broke", []string{"+1", "+normal! A-e", "+wq"}},
		{"m.txt", "z1\nz2\n", "z1-mk|z2|", "marks broke", []string{"+1", "+normal! ma", "+2", "+normal! 'aA-mk", "+wq"}},
		{"u.txt", "r1\nr2\nr3\n", "r1|r2|r3|", "undo broke", []string{"+2", "+normal! dd", "+normal! u", "+wq"}},
	} {
		f := check.ProbeFile(d, c.file, c.content, c.Args...)
		if check.Bar(f) != c.want {
			s.Echo("  noconstfn    %s: '%s'", c.What, check.Bar(f))
			return harness.ErrReported
		}
	}
	s.Echo("  noconstfn    ranges, :g, :set, writing, marks and undo all work")
	return nil
}
