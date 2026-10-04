package whim

import "github.com/arbace/go-whim/crefactor/graph"

// StateParam is phase 95's (crefactor/graph's Editor.StateParam, on the
// graph since Step6): the regex engine's state at match time -- the match
// in progress (rex, and its re-entry guard), the backtracking stacks and
// their byte count, the look-behind's and the counted repeats' state, the
// back-reference's copy -- and reg_toolong, which the compiler sets and
// every match clears; one struct, handed down from the four functions the
// rest of the editor calls the engine by.  The host may name none of the
// functions that take it.
var StateParam = graph.StateParamOptions{
	Core: true,
	Objects: []string{
		"rex", "rex_in_use",
		"regstack", "regstack_star", "regstack_behind", "backpos", "regstack_bytes",
		"behind_pos", "bl_minval", "bl_maxval",
		"brace_min", "brace_max", "brace_count",
		"reg_tofree", "reg_tofreelen", "reg_toolong",
	},
	Type:     "regengine_T",
	Instance: "reg_engine",
	Param:    "re",
	Roots:    []string{"vim_regcomp", "vim_regexec_multi", "vim_regexec_string", "vim_regsub_multi"},
}
