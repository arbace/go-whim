package p075

// Whim phase 75, the check -- no autocommands.
// See phase/075/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim75", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim75", args)
	if err != nil {
		return err
	}
	const p = "  noautocmd    "
	if !s.Cnt0(p, "apply_autocmds", "apply_autocmds_exarg", "apply_autocmds_retval", "apply_autocmds_group",
		"aucmd_prepbuf", "aucmd_restbuf", "aco_save_T", "aubuflocal_remove", "au_cleanup",
		"au_remove_pat", "au_del_cmd", "event_nr2name", "auto_next_pat", "getnextac", "first_autopat",
		"last_autopat", "AutoPat", "AutoCmd", "AutoPatCmd_T", "active_apc_list", "au_need_clean",
		"is_autocmd_blocked",
		"has_cursormovedI", "has_textchangedI", "has_textchangedP", "trigger_cmd_autocmd",
		"ins_apply_autocmds", "event_tab", "EVENT_BUFENTER", "NUM_EVENTS") {
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- it has callers that are not autocommand code", "block_autocmds", "unblock_autocmds") {
		return harness.ErrReported
	}
	if n := check.GrepC(s.Src(), `\bautocmd_blocked\b`, check.GERE); n != 3 {
		s.Echo("  noautocmd    autocmd_blocked has %d mentions, expected 3 (declaration and the ++/-- pair)", n)
		return harness.ErrReported
	}
	if check.GrepQ(s.Src(), `\bis_autocmd_blocked\b`, check.GERE) {
		s.Echo("  noautocmd    autocmd_blocked still has a reader")
		return harness.ErrReported
	}
	cb := s.FnBody(`^close_buffer\(`)
	if check.GrepQ(cb, "aucmd_abort", check.GERE) {
		s.Echo("  noautocmd    close_buffer still has an orphaned abort label")
		return harness.ErrReported
	}
	if !check.GrepQ(cb, "e_autocommands_caused_command_to_abort", check.GERE) {
		s.Echo("  noautocmd    close_buffer lost its abort_if_last arm")
		return harness.ErrReported
	}
	bw := s.FnBody(`^buf_write\(`)
	if !check.GrepQ(bw, `\bbuf_ffname\b`, check.GERE) {
		s.Echo("  noautocmd    buf_write lost buf_ffname -- :w on a renamed buffer would break")
		return harness.ErrReported
	}
	if !check.GrepQ(bw, `\bbuf_fname_s\b`, check.GERE) {
		s.Echo("  noautocmd    buf_write lost buf_fname_s")
		return harness.ErrReported
	}
	if !check.GrepQ(s.FnBody(`^open_buffer\(`), `BF_CHECK_RO \| BF_NEVERLOADED`, check.GERE) {
		s.Echo("  noautocmd    open_buffer lost its flag clearing")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- that was never the autocommand layer", "close_buffer", "buf_freeall", "buf_write", "readfile", "set_curbuf", "buflist_new", "open_buffer",
		"ins_redraw", "getout", "do_one_cmd", "set_termname", "u_save", "curbufIsChanged") {
		return harness.ErrReported
	}
	s.Echo("  noautocmd    no autocommands; the work they were wrapped around is intact")
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
	wt := check.ProbeFile(d, "w.txt", "w1\nw2\n", "+1", "+normal! A-w", "+wq")
	if check.Bar(wt) != "w1-w|w2|" {
		s.Echo("  noautocmd    writing broke: '%s'", check.Bar(wt))
		return harness.ErrReported
	}
	check.Put(filepath.Join(d, "src.txt"), "x1\n")
	os.Remove(filepath.Join(d, "dst.txt"))
	check.InD(d, "-e", "-s", "+w dst.txt", "+q!", "src.txt")
	if check.CatS(filepath.Join(d, "dst.txt")) != "x1" {
		s.Echo("  noautocmd    :w to another name broke: %s", check.CatS(filepath.Join(d, "dst.txt")))
		return harness.ErrReported
	}
	if !s.SwitchesE(d, p, "e1.txt", "e2.txt", "e1\n", "e2\n", "e2-E", "+normal! A-E") {
		return harness.ErrReported
	}
	for _, c := range []struct {
		file, content, want, What string
		Args                      []string
	}{
		{"g.txt", "keep1\ndrop\nkeep2\n", "keep1|keep2|", ":g broke", []string{"+g/drop/d", "+wq"}},
		{"s.txt", "s1\ns2\n", "S1|S2|", ":s broke", []string{"+%s/^s/S/", "+wq"}},
		{"m.txt", "m1\nm2\nm3\n", "m2|m3|m1|", ":m broke", []string{"+1m$", "+wq"}},
		{"u.txt", "u1\nu2\n", "u1|u2|", "undo broke", []string{"+1", "+normal! dd", "+normal! u", "+wq"}},
	} {
		f := check.ProbeFile(d, c.file, c.content, c.Args...)
		if check.Bar(f) != c.want {
			s.Echo("  noautocmd    %s: '%s'", c.What, check.Bar(f))
			return harness.ErrReported
		}
	}
	i := check.ProbeFile(d, "i.txt", "i1\n", "+normal! A-ins", "+wq")
	if check.CatS(i) != "i1-ins" {
		s.Echo("  noautocmd    insert-mode editing broke: %s", check.CatS(i))
		return harness.ErrReported
	}
	s.Echo("  noautocmd    loads, writes, :w name, :e, :g, :s, :m, undo and insert all work")
	return nil
}
