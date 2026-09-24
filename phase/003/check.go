package p003

// Whim phase 3, the check -- no introduction, and the command line says only what the editor still decides.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim3", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim3", args)
	if err != nil {
		return err
	}
	if !s.Gone("  cmdline      ", check.GERE, check.GERE, true, nil,
		`\blist_version\b`, `\busage\(`, `\bmaybe_intro_message\b`,
		`\bcompiled_(user|sys)\b`, `\bearly_arg_scan\b`, `\bmake_tabpages\b`,
		`\bset_init_clean_rtp\b`,
		`\bis_not_a_term`, `More info with`, `"-nb"`,
		`"(not-a-term|noplugin|startuptime|gui-dialog-file|nofork|--clean)"`) {
		return harness.ErrReported
	}
	s.Echo("  cmdline      nothing the introduction or a dropped option needed is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if !s.St("clicheck", filepath.Join(s.Work, "whim-vim")) {
		return harness.ErrReported
	}
	return nil
}
