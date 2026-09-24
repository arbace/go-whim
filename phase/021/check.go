package p021

// Whim phase 21, the check -- there is nothing to recover, and the memfile is memory.
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

func init() { check.Register("whim21", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim21", args)
	if err != nil {
		return err
	}
	if !s.Gone("  memfile      ", check.GBREw, check.GBREw, true, nil, "mf_fd", "mf_fname", "mf_ffname", "mf_write", "mf_read",
		"mf_release", "total_mem_used", "p_mmt", "p_dir", "mch_total_mem", "mch_get_host_name") {
		return harness.ErrReported
	}
	s.Echo("  memfile      no descriptor, no eviction, no memory budget")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("getpwuid", "localtime_r", "strftime") {
		return harness.ErrReported
	}
	s.Echo("  symbols      getpwuid is gone -- the last of the five password symbols")
	if !s.SymsGone("sysinfo", "getrlimit", "uname") {
		return harness.ErrReported
	}
	s.Echo("  symbols      sysinfo, getrlimit and uname are gone from nm -u")
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".ovtest")
	check.Put(filepath.Join(d, "a.txt"), "one\n")
	check.Put(filepath.Join(d, "b.txt"), "two\n")
	rc := check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "w! b.txt", "-c", "qa!", "a.txt")
	ov := fmt.Sprintf("%d:%s", rc, check.CatS(filepath.Join(d, "b.txt")))
	os.RemoveAll(d)
	if ov != "0:one" {
		s.Echo("  overwrite    :w! over an existing other file gave %s, expected 0:one", ov)
		return harness.ErrReported
	}
	s.Echo("  overwrite    :w! over an existing other file writes it")
	return nil
}
