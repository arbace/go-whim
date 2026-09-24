package p026

// Whim phase 26, the check -- five signals, not twenty-one.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim26", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim26", args)
	if err != nil {
		return err
	}
	set := map[string]bool{}
	for _, x := range check.GrepO(s.Src(), `\bSIG[A-Z0-9]*\b`, check.GBRE) {
		set[x] = true
	}
	var ns []string
	for k := range set {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	named := strings.Join(ns, " ") + " "
	if len(ns) == 0 {
		named = ""
	}
	if named != "SIGALRM SIGCONT SIGHUP SIGINT SIGPIPE SIGTERM SIGTSTP SIGWINCH " {
		s.Echo("  signals      the file names: %s", named)
		s.Echo("               expected the five kept, plus SIGCONT/SIGALRM/SIGPIPE,")
		s.Echo("               which mch_suspend() sets around the stop")
		return harness.ErrReported
	}
	if !s.Gone("  signals      ", check.GBRE, check.GBRE, false, nil, "sigaltstack", "may_core_dump", "catch_sigpwr", "catch_sigusr1", "got_sigusr1",
		"signal_stack", "sigstk") {
		return harness.ErrReported
	}
	s.Echo("  signals      %d in the table; resize, interrupt,", check.GrepC(s.Src(), "{SIG", check.GBRE))
	s.Echo("               suspend, and a terminal put back on the way out")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("sigaltstack", "sysconf") {
		return harness.ErrReported
	}
	s.Echo("  symbols      sigaltstack and sysconf are gone from nm -u")
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if s.St("termrestore", filepath.Join(s.Work, "whim-vim")) {
		s.Echo("  terminal     SIGTERM still puts the terminal back")
		return nil
	}
	s.Echo("  terminal     SIGTERM left the terminal raw -- the deathtrap is what")
	s.Echo("               this phase kept SIGHUP and SIGTERM for")
	return harness.ErrReported
}
