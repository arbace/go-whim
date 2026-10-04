// Package steps is every transformation a phase names, as a function from text
// to text.
//
// The phase programs called these through tools/st.sh, one process per call,
// which re-read and re-wrote a two-megabyte file 277 times a pass.  They are
// the same functions; what is new is that a caller can run them in memory and
// in order.  cmd/whim' subcommands and cmd/whim' build both dispatch
// through this table, so there is one definition of what `dropoptions` means.
//
// A step reports on w as it goes -- ORDER IS OUTPUT, the phase programs' rule --
// and refuses with an error rather than returning a half-rewritten tree.  A step
// that only asserts returns its input unchanged.
package steps

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/crefactor/dead"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/cut"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// A Step is one transformation: the tree in, the tree out, its report on w.
type Step func(text []byte, args []string, w io.Writer) ([]byte, error)

var ops = map[string]Step{
	// The four that take arguments, and the three that only ask a question;
	// droplocal is a graph step (graphOps).
	"funcreach": funcReach,
	"edit":      runEdit,
	"query":     runQuery,

	"cemit":       Step(pipeline.Canonical),
	"includes":    Step(xform.Includes(whim.Includes)),
	"query-empty": queryEmpty,
}

// Lookup2 returns the step of that name and panics when there is none: for a
// caller that names a step this package defines, where a missing one is a
// programming error and not a phase's mistake.
func Lookup2(name string) Step {
	s, ok := ops[name]
	if !ok {
		panic("steps: no step named " + name)
	}
	return s
}

// Lookup returns the step of that name, and whether there is one.
func Lookup(name string) (Step, bool) {
	s, ok := ops[name]
	return s, ok
}

// Names returns every step, sorted, for a usage message.
func Names() []string {
	out := make([]string, 0, len(ops))
	for k := range ops {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---- the graph steps -------------------------------------------------------
//
// A GraphStep edits the program as crefactor/graph's graph, through its
// editor (doc/GRAPH.md): the plan names it with Graph set, and the driver
// hands it the graph it holds, importing the text where a run of graph
// steps begins.  Its fall-out is the editor's closure, told vim's
// specifics by whim.GraphFallOut.

// A GraphStep is one transformation on the graph, its report on w.
type GraphStep func(e *graph.Editor, args []string, w io.Writer) error

var graphOps = map[string]GraphStep{
	"droplocal": dropLocal,
	"edit":      runGraphEdit,
	// B1a (doc/GRAPH-MIGRATION.md): cutters converted to the graph
	"noinertopts": plainGraph(cut.NoInertOpts),
	"nofloat":     plainGraph(cut.NoFloat),
	"noowner":     plainGraph(cut.NoOwner),
	"nointro":     plainGraph(cut.NoIntro),
	"optreaders":  plainGraph(cut.OptReaders),
	"nostat":      plainGraph(cut.NoStat),
	"nobackup":    plainGraph(cut.NoBackup),
	"lfonly":      plainGraph(cut.LfOnly),
	"keepbytes":   plainGraph(cut.KeepBytes),
	// B3a: phases 4-6, 9-12, 16, 21, 22 and 26
	"onebuffer":  plainGraph(cut.OneBuffer),
	"nowild":     plainGraph(cut.NoWild),
	"nowildmenu": plainGraph(cut.NoWildMenu),
	"noconv":     plainGraph(cut.NoConv),
	"noglob":     plainGraph(cut.NoGlob),
	"nofind":     plainGraph(cut.NoFind),
	"nohome":     plainGraph(cut.NoHome),
	"nogetenv":   plainGraph(cut.NoGetEnv),
	"nochdir":    plainGraph(cut.NoChdir),
	"oneoptset":  plainGraph(cut.OneOptSet),
	// B4: the front, phases 1-3
	"front":       frontGraph(1),
	"front2":      frontGraph(2),
	"front3":      frontGraph(3),
	"noruntime":   plainGraph(cut.NoRuntime),
	"query-empty": queryEmptyGraph,
}

// LookupGraph returns the graph step of that name, and whether there is one.
func LookupGraph(name string) (GraphStep, bool) {
	s, ok := graphOps[name]
	return s, ok
}

// GraphNames returns every graph step, sorted.
func GraphNames() []string {
	out := make([]string, 0, len(graphOps))
	for k := range graphOps {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// OnText is the step of that name as a function of text, for a caller with
// a file and no graph (`whim droplocal F`, `whim edit whimN F`): a text
// step as it is; a graph step -- or an `edit` of a phase whose program is
// on the graph -- on the text imported, its C view returned.  The plan
// never runs this: it runs a graph step on the graph it holds.
func OnText(name string) (Step, bool) {
	gs, isGraph := graphOps[name]
	if s, ok := ops[name]; ok && name != "edit" {
		return s, true
	} else if !isGraph {
		return nil, false
	}
	return func(t []byte, args []string, w io.Writer) ([]byte, error) {
		if name == "edit" && len(args) > 0 {
			if _, ok := phase.Lookup(args[0]); ok {
				return runEdit(t, args, w)
			}
		}
		g, _, err := graph.Import(filepath.Join(os.TempDir(), "whim-vim.c"), t)
		if err != nil {
			return nil, err
		}
		if err := gs(graph.NewEditor(g), args, w); err != nil {
			return nil, err
		}
		return g.C()
	}, true
}

// dropLocal is `droplocal <field>...`, one field at a time, reporting each:
// internal/cut's DropLocal, a deletion and its fall-out on the graph.
func dropLocal(e *graph.Editor, args []string, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("droplocal: no field named")
	}
	for _, bvar := range args {
		n, err := cut.DropLocal(e, bvar, whim.GraphFallOut)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "  droplocal    %-10s %d plumbing sites\n", bvar, n)
	}
	return nil
}

// runGraphEdit is `edit <phase> [args...]` on the graph: the phase's own
// program, registered with phase.RegisterGraph.
func runGraphEdit(e *graph.Editor, args []string, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("edit: no phase named")
	}
	f, ok := phase.LookupGraph(args[0])
	if !ok {
		return fmt.Errorf("edit: no graph edit for phase %q", args[0])
	}
	return f(e, w, args[1:])
}

// frontHold is what the closure after the front cuts leaves to the phases
// that fold it by hand: read_cmd_fd, sticky_cmdmod_flags,
// aucmd_cmdline_changed_count and skip_win_fix_cursor, which the product
// keeps unwritten (the second and third since D6 took their writers, the last
// since D9), the popup menu's blend state and the completion submode's
// message (since D10's nocompl took their writers), listcmd_busy (since
// whim3e took the '{ and '( addresses that saved and set it), and the dropped
// options' globals of optfrontHold.
var frontHold = append([]string{"read_cmd_fd", "sticky_cmdmod_flags",
	"aucmd_cmdline_changed_count", "skip_win_fix_cursor",
	"screen_pum_blend", "pum_bg_attrs", "pum_bg_lines", "pum_bg_linesUC",
	"pum_bg_linesC", "pum_bg_top", "pum_bg_bot", "pum_bg_cols", "edit_submode",
	"edit_submode_pre", "edit_submode_extra", "edit_submode_highl",
	"listcmd_busy"}, optfrontHold...)

// optfrontHold are the dropped options' globals the fall-out closure leaves
// to the phase that folds them by hand: where the closure's shape and the
// hand fold's differ, and the product has the hand fold's.
var optfrontHold = []string{
	"p_wmnu", // nowildmenu (phase 4) writes `a && b && c` flat
	// nobackup (phase 12's cut, run at phase 5) cuts the backup machinery
	// around dobackup, whose assignment also dereferences p_pm
	"p_bk", "p_wb", "p_pm", "p_bsk", "p_bex", "p_bkc", "p_bdir",
	// nowinsizes (record 40) gives these their defaults -- p_ea is TRUE -- and
	// later phases fold their readers with them; the product keeps three
	"p_sb", "p_spr", "p_spk", "p_ea", "p_ead", "p_wh", "p_wmh", "p_wiw", "p_wmw",
	// phase 19 writes `regmatch.rm_ic = FALSE;` where the closure writes 0,
	// and drops a test on `acl_elapsed >= p_acl` whole
	"p_fic", "p_acl",
	// phase 20 cuts :!'s and :stop's whole `autowrite_all()` blocks, where
	// the closure would empty autowrite_all() and leave the blocks
	"p_aw", "p_awa", "p_write",
}

// funcReach is `funcreach [--delete]`: the floor is the tool's and refusing on
// it is the point, because acting on a broken match would delete the program.
func funcReach(t []byte, args []string, w io.Writer) ([]byte, error) {
	del := false
	for _, a := range args {
		if a == "--delete" {
			del = true
		}
	}
	defs, reachable, deadNames, deadLines := dead.FuncReach(t, whim.Dead.Roots)
	if len(defs) < whim.Dead.MinDefinitions {
		return nil, fmt.Errorf("funcreach: only %d definitions found, which cannot be right "+
			"for this file -- the shape it matches has changed, and acting on the "+
			"answer would delete most of the program", len(defs))
	}
	fmt.Fprintf(w, "  funcreach    %d definitions, %d reachable, %d not (%d lines)\n",
		len(defs), reachable, len(deadNames), deadLines)
	if len(deadNames) > 0 && del {
		t = dead.DeleteFuncs(t, defs, deadNames)
		fmt.Fprintf(w, "  funcreach    %d deleted\n", len(deadNames))
	}
	return t, nil
}

// runEdit is `edit <phase> [args...]`: the phase's own transformation.
func runEdit(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("edit: no phase named")
	}
	f, ok := phase.Lookup(args[0])
	if !ok {
		return nil, fmt.Errorf("edit: no edit for phase %q", args[0])
	}
	return f(t, w, args[1:])
}

// runQuery is `query <phase>`: it asks and changes nothing.  What the phase
// program did with the answer is the caller's -- see internal/build.
func runQuery(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("query: no phase named")
	}
	f, ok := phase.LookupQuery(args[0])
	if !ok {
		return nil, fmt.Errorf("query: no query for phase %q", args[0])
	}
	if err := f(t, w); err != nil {
		return nil, err
	}
	return t, nil
}

// ---- the four steps a phase program expressed as shell around a call --------
//
// These were `live=$(query whim2)` and a test on the answer, a probe compiled
// with the host's headers, a list of option names the query computed.  They are
// steps here for the same reason the rest are: so that a phase is a sequence of
// named transformations and nothing else.

// queryEmpty is `query <phase>` with the phase program's test: an answer at all
// means the phase's premise has stopped holding.
func queryEmpty(t []byte, args []string, w io.Writer) ([]byte, error) {
	out, err := ask(t, args)
	if err != nil {
		return nil, err
	}
	if len(strings.Fields(out)) > 0 {
		return nil, fmt.Errorf("  commands     still implemented, so this phase is incomplete: %s",
			strings.TrimSpace(out))
	}
	fmt.Fprintf(w, "  commands     all 24 menu and spell commands are already ex_ni\n")
	return t, nil
}

// ask runs a query and returns what it printed.
func ask(t []byte, args []string) (string, error) {
	var b strings.Builder
	if _, err := runQuery(t, args, &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

// MinMaxProbe is the two lines phase 42 needs from the host's <sys/param.h>:
// what MIN and MAX expand to, asked of the preprocessor rather than assumed.
// The phase program wrote this file, ran the preprocessor over it and passed
// the result; a build does the same and keeps it in memory.
const minMaxProbe = "#include <sys/param.h>\nMIN(ZZA,ZZB)\nMAX(ZZA,ZZB)\n"

// MinMax returns the preprocessed probe, the two lines exactly.
func MinMax() ([]byte, error) {
	d, err := os.MkdirTemp("", "minmax")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(d)
	src := filepath.Join(d, "minmax-probe.c")
	if err := os.WriteFile(src, []byte(minMaxProbe), 0o644); err != nil {
		return nil, err
	}
	out, err := exec.Command("gcc", "-E", "-P", src).Output()
	if err != nil {
		return nil, fmt.Errorf("the MIN/MAX probe did not preprocess: %v", err)
	}
	var keep []string
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.Contains(ln, "ZZA") {
			keep = append(keep, ln)
		}
	}
	if len(keep) != 2 {
		return nil, fmt.Errorf("the MIN/MAX probe gave %d lines, not 2", len(keep))
	}
	return []byte(strings.Join(keep, "\n") + "\n"), nil
}

// plainGraph wraps a cutter on the graph that takes no arguments of its own.
func plainGraph(f func(*graph.Editor, io.Writer) error) GraphStep {
	return func(e *graph.Editor, _ []string, w io.Writer) error { return f(e, w) }
}
