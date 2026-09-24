package p082

// Whim phase 82, the check -- the system headers nothing needs, and every comment.
// See phase/082/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim82", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim82", args)
	if err != nil {
		return err
	}
	silent := func(p string) bool {
		Out, err := exec.Command("gcc", "-fsyntax-only", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter", p).CombinedOutput()
		return err == nil && len(Out) == 0
	}
	var total int
	fmt.Sscan(check.ReadFile(filepath.Join(s.State, "total")), &total)
	keep := len(check.Lines(check.ReadFile(filepath.Join(s.State, "keep"))))
	left := check.GrepC(s.Src(), `^#include <`, check.GERE)
	if left != total-keep {
		s.Echo("  includes     %d includes left, expected %d", left, total-keep)
		return harness.ErrReported
	}
	if !silent(s.F) {
		s.Echo("  includes     the result does not compile silently")
		return harness.ErrReported
	}
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, _ := os.MkdirTemp("", "whim82")
	defer os.RemoveAll(d)
	build := func(sub, src string) error {
		dir := filepath.Join(d, sub)
		os.MkdirAll(dir, 0o755)
		check.Put(filepath.Join(dir, "whim-vim.c"), check.ReadFile(src))
		c := exec.Command("gcc", "-O0", "-static", "-s", "-o", "vim", "whim-vim.c")
		c.Dir, c.Env = dir, check.EnvWith("SOURCE_DATE_EPOCH=0")
		return c.Run()
	}
	if build("old", filepath.Join(s.State, "old", "whim-vim.c")) != nil || build("new", s.F) != nil {
		return harness.ErrReported
	}
	ob, nb := check.ReadFile(filepath.Join(d, "old", "vim")), check.ReadFile(filepath.Join(d, "new", "vim"))
	if ob != nb {
		s.Echo("  includes     the binary changed -- a header was doing more than declaring")
		return harness.ErrReported
	}
	s.Echo("  includes     %d includes left; the binary is byte-identical (%d bytes)", left, len(nb))
	return nil
}
