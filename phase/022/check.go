package p022

// Whim phase 22, the check -- the working directory is where it started.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim22", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim22", args)
	if err != nil {
		return err
	}
	// Counted with grep -c and shown with grep -nw, as the shell does.
	if !s.Gone("  cwd          ", check.GBRE, check.GBREw, true, nil,
		"mch_chdir(", "chdir(", "fchdir(", "win_fix_current_dir(", `\bglobaldir\b`, `\bstart_dir\b`) {
		return harness.ErrReported
	}
	if n := check.GrepC(s.Src(), `getcwd((char \*)`, check.GBRE); n != 1 {
		s.Echo("  cwd          getcwd is called %d times, expected exactly 1", n)
		return harness.ErrReported
	}
	s.Echo("  cwd          nothing moves the process; getcwd is asked once")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("chdir", "fchdir") {
		return harness.ErrReported
	}
	s.Echo("  symbols      chdir and fchdir are gone from nm -u")
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".reltest")
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	check.Put(filepath.Join(d, "sub", "f.txt"), "one\ntwo\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "normal Gothree", "-c", "wq", "sub/f.txt")
	check.VimRC(filepath.Join(d, "sub"), "", "../../whim-vim", "-e", "-s", "-c", "%s/two/2/", "-c", "wq", "../sub/f.txt")
	Rel := strings.ReplaceAll(check.ReadFile(filepath.Join(d, "sub", "f.txt")), "\n", " ")
	os.RemoveAll(d)
	if Rel != "one 2 three " {
		s.Echo("  relative     a relative path gave '%s', expected 'one 2 three '", Rel)
		return harness.ErrReported
	}
	s.Echo("  relative     a relative path with a directory in it opens and writes")
	return nil
}
