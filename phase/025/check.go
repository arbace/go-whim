package p025

// Whim phase 25, the check -- a write is a write, and nobody owns it.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim25", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim25", args)
	if err != nil {
		return err
	}
	if !s.Gone("  backup       ", check.GBRE, check.GBRE, true, nil, `p_bk\b`, `p_wb\b`, `p_bkc\b`, `p_bdir\b`, `p_bex\b`, `p_bsk\b`, `p_pm\b`,
		"b_p_bkc", "vim_rename", "vim_copyfile", "set_file_time", "mch_get_acl",
		"vim_acl_T", "backup_copy", "dobackup") {
		return harness.ErrReported
	}
	s.Echo("  backup       nothing is copied aside, renamed, or timestamped")
	if !s.Gone("  owner        ", check.GBRE, check.GBRE, true, nil, "getuid", "getgid", "get_user_name", "ROOT_UID", "b0_uname") {
		return harness.ErrReported
	}
	s.Echo("  owner        nothing asks who you are")
	if !s.KeptCall("  permissions  ", " went too -- permissions are not ownership", "mch_setperm", "mch_fsetperm", "mch_getperm") {
		return harness.ErrReported
	}
	s.Echo("  permissions  chmod and fchmod stay: a file still has a mode")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("utime", "readlink", "symlink", "rename") {
		return harness.ErrReported
	}
	s.Echo("  symbols      utime, readlink, symlink and rename are gone from nm -u")
	if !s.SymsGone("getuid", "getgid") {
		return harness.ErrReported
	}
	s.Echo("  symbols      getuid and getgid are gone from nm -u")
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".bktest")
	check.Put(filepath.Join(d, "f.txt"), "one\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "%s/one/two/", "-c", "wq", "f.txt")
	bk := fmt.Sprintf("%s:%s", check.LsA(d), check.CatS(filepath.Join(d, "f.txt")))
	os.RemoveAll(d)
	if bk != "f.txt :two" {
		s.Echo("  overwrite    a write left '%s', expected 'f.txt :two'", bk)
		return harness.ErrReported
	}
	s.Echo("  overwrite    overwriting a file leaves the file, and nothing beside it")
	d = s.Sub(".rotest")
	check.Put(filepath.Join(d, "f.txt"), "one\n")
	os.Chmod(filepath.Join(d, "f.txt"), 0o444)
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "%s/one/two/", "-c", "wq!", "f.txt")
	ro := check.CatS(filepath.Join(d, "f.txt"))
	os.Chmod(filepath.Join(d, "f.txt"), 0o644)
	os.RemoveAll(d)
	if ro != "two" {
		s.Echo("  readonly     :w! over a read-only file gave '%s', expected 'two'", ro)
		return harness.ErrReported
	}
	s.Echo("  readonly     :w! over a read-only file still writes it")
	return nil
}
