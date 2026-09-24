package p009

// Whim phase 9, the check -- the editor stops asking the environment what language it is in.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim9", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim9", args)
	if err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if enc := s.EncodingIn(".enc.txt", ".enc.out"); enc != "encoding=utf-8" {
		s.Echo("  encoding     got '%s', expected encoding=utf-8", enc)
		s.Echo("               the locale used to supply this; the default must now carry it")
		return harness.ErrReported
	}
	s.Echo("  encoding     utf-8 by compiled default, with no locale asked")
	return nil
}
