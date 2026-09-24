package p020

// Whim phase 20, the check -- nothing outside the process is consulted.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim20", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim20", args)
	if err != nil {
		return err
	}
	if !s.Gone("  globals      ", check.GBRE, check.GBRE, true, check.GlobalsExtra,
		`getenv((char \*)((char_u \*)"HOME")`, "homedir", "init_users", "match_user", "getpwnam") {
		return harness.ErrReported
	}
	s.Echo("  home         nothing asks where home is, or who this is")
	if !s.Gone("  environment  ", check.GBREw, check.GBREw, true, nil, "getenv", "setenv", "unsetenv", "environ", "vim_getenv") {
		return harness.ErrReported
	}
	s.Echo("  environment  nothing in the source asks the environment anything")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("getenv", "setenv", "unsetenv", "environ") {
		return harness.ErrReported
	}
	s.Echo("  symbols      getenv, setenv, unsetenv and environ are gone from nm -u")
	return s.Phasebuild()
}
