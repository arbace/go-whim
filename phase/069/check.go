package p069

// Whim phase 69, the check -- one file argument, and no argument list.
// See phase/069/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim69", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim69", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  onearg       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "ex_next", "ex_previous", "do_argfile", "do_arglist", "arglist_del_files", "alist_set", "alist_clear", "alist_add",
		"alist_name", "alist_init", "editing_arg_idx", "check_arglist_locked", "arglist_locked", "arg_had_last",
		"global_alist", "alist_T", "aentry_T", "w_alist", "w_arg_idx", "w_arg_idx_invalid", "AL_SET", "AL_ADD", "AL_DEL") {
		return harness.ErrReported
	}
	if !s.KeptE("  onearg       ", " went too -- the one file would not load", "buflist_add", "buflist_new", "open_buffer", "b_ffname", "readfile", "command_line_scan") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), "buflist_add(p, BLN_CURBUF | BLN_LISTED);", check.GBRE) {
		s.Echo("  onearg       the command line no longer names the buffer")
		return harness.ErrReported
	}
	s.Echo("  onearg       one file argument, no argument list, and the name still reaches curbuf")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, "  onearg       ", "l1\nl2\nl3\n", "l1|l2|LAST l3|") || !s.Edits(d, "  onearg       ", "e.txt", "a\nb\nc\n", "a|c|") {
		return harness.ErrReported
	}
	check.Put(filepath.Join(d, "g1.txt"), "one\n")
	check.Put(filepath.Join(d, "g2.txt"), "two\n")
	if check.InD(d, "-e", "-s", "+normal! iX", "+wq", "g1.txt", "g2.txt") == 0 {
		s.Echo("  onearg       two file arguments were accepted")
		return harness.ErrReported
	}
	if !(check.CatS(filepath.Join(d, "g1.txt")) == "one" && check.CatS(filepath.Join(d, "g2.txt")) == "two") {
		s.Echo("  onearg       a refused command line still wrote")
		return harness.ErrReported
	}
	check.Put(filepath.Join(d, "n.txt"), "n1\n")
	if check.InD(d, "-e", "-s", "+next", "+q!", "n.txt") == 0 {
		s.Echo("  onearg       :next was accepted")
		return harness.ErrReported
	}
	check.Put(filepath.Join(d, "h1.txt"), "h1\n")
	check.Put(filepath.Join(d, "h2.txt"), "h2\n")
	check.InD(d, "-e", "-s", "+e h2.txt", "+normal! iE", "+wq", "h1.txt")
	if !(check.CatS(filepath.Join(d, "h1.txt")) == "h1" && check.CatS(filepath.Join(d, "h2.txt")) == "Eh2") {
		s.Echo("  onearg       :e broke: h1=%s h2=%s", check.CatS(filepath.Join(d, "h1.txt")), check.CatS(filepath.Join(d, "h2.txt")))
		return harness.ErrReported
	}
	s.Echo("  onearg       the file loads and edits; a second argument and :next are refused; :e still opens")
	return nil
}
