package p078

// Whim phase 78, the check -- empty functions, write-only counters, and the window id.
// See phase/078/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim78", Check) }

// termrequestInit is phase 78's `{STATUS_GET, -1}` initialiser as the printer
// writes it: the brace under the `=`, one element per line, a comma after the
// last.  It spans lines, so it cannot go through grepC.
var termrequestInit = regexp.MustCompile(
	`(?m)^static termrequest_T [a-z0-9_]+_status =\n\{\n    STATUS_GET,\n    -1,\n\};$`)

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim78", args)
	if err != nil {
		return err
	}
	const p = "  nostubs      "
	if !s.Cnt0(p, "autocmd_blocked", "autocmd_no_enter", "autocmd_no_leave", "redrawing_for_callback",
		"prevwin", "last_win_id", "LOWEST_WIN_ID", "w_id", "winid", "prechar",
		"clear_chartabsize_arg", "may_trigger_modechanged", "may_trigger_win_scrolled_resized",
		"out_flush_check", "add_b0_fenc", "set_b0_dir_flag", "pum_may_redraw", "ml_setname",
		"ml_preserve", "trigger_undo_ftplugin", "set_init_lang_env",
		"set_init_default_printencoding", "set_init_3", "mch_new_shellsize", "mch_early_init") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\btr_start\b`, check.GERE) {
		s.Echo("  nostubs      tr_start went -- three {STATUS_GET, -1} initialisers supply it")
		return harness.ErrReported
	}
	// THREE OF THEM, AND THE SHAPE IS NOW A TABLE.  The residue wrote each on
	// one line, `termrequest_T crv_status =  {STATUS_GET, -1} ;`; the printer
	// puts the brace under the `=`, one element per line, with a comma after
	// the last -- so this is the only anchor in the file that cannot be a line
	// grep.  Measured: the residue spelling occurs three times in slim-vim.c
	// and this one occurs three times here, at the same three sites.
	if n := len(termrequestInit.FindAllString(s.Src(), -1)); n != 3 {
		s.Echo("  nostubs      the termrequest_T initialisers changed shape (%d, expected 3)", n)
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bnv_nop\b`, check.GERE) {
		s.Echo("  nostubs      nv_nop went -- nv_cmd_idx[] is no longer a permutation")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\+\+breakcheck_count >= BREAKCHECK_SKIP`, check.GERE) {
		s.Echo("  nostubs      breakcheck_count lost its reader")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bvim_ignored\b`, check.GERE) {
		s.Echo("  nostubs      vim_ignored went -- it is the deliberate return-value sink")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- it still has live callers", "block_autocmds", "unblock_autocmds", "init_incsearch_state", "win_enter_ext", "create_windows",
		"redraw_after_callback", "win_alloc", "getcmdline_int") {
		return harness.ErrReported
	}
	s.Echo("  nostubs      no empty calls, no unread counters, no window id; nv_nop and the sinks intact")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") {
		return harness.ErrReported
	}
	i := check.ProbeFile(d, "i.txt", "i1\n", "+normal! A-ins", "+wq")
	if check.CatS(i) != "i1-ins" {
		s.Echo("  nostubs      insert broke: %s", check.CatS(i))
		return harness.ErrReported
	}
	for _, c := range []struct {
		file, content, want, What string
		Args                      []string
	}{
		{"g.txt", "one1\ntwo2\n", "oneN|twoN|", ":g broke", []string{"+g/[0-9]$/s/[0-9]$/N/", "+wq"}},
		{"s.txt", "x\ny\n", "x|y-found|", "search broke", []string{"+/y", "+normal! A-found", "+wq"}},
		{"c.txt", "c1\nc2\n", "c1-ch|c2|", ":set cmdheight broke", []string{"+set cmdheight=2", "+1", "+normal! A-ch", "+wq"}},
	} {
		f := check.ProbeFile(d, c.file, c.content, c.Args...)
		if check.Bar(f) != c.want {
			s.Echo("  nostubs      %s: '%s'", c.What, check.Bar(f))
			return harness.ErrReported
		}
	}
	if !s.MapsLocal(d, p, "m1\n", "m1!") {
		return harness.ErrReported
	}
	s.Echo("  nostubs      loads, inserts, :g, search, cmdheight and mappings all work")
	return nil
}
