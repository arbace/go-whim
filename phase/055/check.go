package p055

// Whim phase 55, the check -- no option nothing reads.
// See phase/055/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim55", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim55", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  unusedopts   ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_act", "p_cdh", "p_cdpath", "p_cto", "p_imcmdline", "p_secure", "p_shcf", "p_stmp", "p_sxe", "p_sxq", "p_sn", "b_p_sn",
		"p_tbi", "p_warn", "p_xtermcodes", "p_cms", "b_p_cms", "p_cfc", "cfc_flags", "p_cia", "cia_flags", "p_hf", "p_lop", "b_p_lop",
		"p_opfunc", "opfunc_cb", "did_set_commentstring", "did_set_completefuzzycollect", "did_set_completeitemalign",
		"did_set_helpfile", "did_set_lispoptions", "did_set_operatorfunc") {
		return harness.ErrReported
	}
	s.Echo("  unusedopts   no variable, flag set or callback of the 36 is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  unusedopts   ", "shelltemp", "commentstring", "t_EI") {
		return harness.ErrReported
	}
	s.Echo("  unusedopts   :set sw works; :set shelltemp, commentstring and t_EI are unknown")
	return nil
}
