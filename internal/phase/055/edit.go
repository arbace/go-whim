package p055

// Whim phase 55 -- no option nothing reads.  See GOAL.md.
//
// Phase 54 took the options with no variable.  These have one, and nothing but
// the option machinery reads it: the declaration, the row, get_varp() and the
// buffer copy for a local one, set_context_in_set_cmd()'s completion, and a
// did_set_* callback that only validates the value or fills a flag set nothing
// reads (cfc_flags, cia_flags, opfunc_cb).  Setting one of them changed nothing.
//
// autocompletetimeout cdhome cdpath completetimeout imcmdline secure
// shellcmdflag shelltemp shellxescape shellxquote shortname ttybuiltin warn
// xtermcodes commentstring completefuzzycollect completeitemalign helpfile
// lispoptions operatorfunc
//
// And sixteen terminal codes the editor stores and never sends:
//
// t_8b t_8f t_EC t_EI t_GP t_RB t_RC t_RF t_RS t_SC t_SH t_SI t_SR t_WP t_XM t_u7
//
// Their KS_ enumerators stay: the built-in terminal tables still name them.
//
// FOUND, NOT LISTED FROM MEMORY: each was checked for a reader outside that
// machinery by its variable (p_xx, b_p_xx, wo_xx, KS_xx), for a by-name use of
// its long or short name, and for what its callback assigns.  The post-greps
// below are the same check, and a new reader of any of them fails the phase.
//
// THE DELTA: none the harnesses record -- no case sets one.  The probes check
// three of them are now unknown.
// No sweep here.  One stood here, and the lines after it were written for swept text,
// but this phase and every stage it has run in reproduce their boundaries without
// it (GOALS.md, *The inner sweeps*; internal/phase/STAGES.md) -- the stage's one sweep does its work.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// cdpathTest is 'cdpath' being offered as a directory list by command-line
// completion.  The option's row goes in the same edit, below; this is the one
// place that reads the global outside it, and dropoptions --strict refuses a
// row whose global still has a reader.
const cdpathTest = ` || p == (char_u *)&p_cdpath)`

// Whim55 stops 'cdpath' being completed as a directory list, so that the row
// can go.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("unusedopts", text, w)
	e.Literal(cdpathTest, `)`, 1, "'cdpath' is no longer completed as a directory list")
	return e.Done()
}

func init() { phase.Register("whim55", Edit) }
