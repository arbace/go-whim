package p056

// Whim phase 56, the check -- no shell, runtime or keyword-program options.
// See phase/056/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim56", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim56", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  noshellrtp   ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_kp", "b_p_kp", "p_sh", "p_shq", "p_srr", "p_rtp", "p_pp", "get_isolated_shell_name", "csh_like_shell", "ExpandRTDir",
		"ExpandPackAddDir", "expand_runtime_cmd", "set_context_in_runtime_cmd", "did_set_shellpipe_redir") {
		return harness.ErrReported
	}
	s.Echo("  noshellrtp   no shell, runtime-path or keyword-program option or reader is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  noshellrtp   ", "shell", "shellquote", "shellredir", "runtimepath", "packpath", "keywordprg") {
		return harness.ErrReported
	}
	s.Echo("  noshellrtp   :set sw works; the six are unknown")
	return nil
}
