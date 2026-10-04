package steps

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/cut"
	"github.com/arbace/go-whim/internal/phase"
)

// THE FRONT, phases 1-3 (doc/PIPELINE-REFORM.md §7; on the graph since B4,
// doc/GRAPH-MIGRATION.md): each a graph step -- its cutters in order, D1-D5,
// D6-D8 and D9-D12 (three phases so that the parallel check runs them side
// by side), and then ONE fall-out closure over what they leave unwritten
// together.  Phase 1 imports phase 0's text; 2 and 3 are handed the graph.

// A FrontUnit is one cutter of a front phase, by name: a cutter of
// internal/cut (Fn), or a phase's program registered on the graph (Prog,
// "whim18").
type FrontUnit struct {
	Name string
	Fn   func(*graph.Editor, io.Writer) error
	Prog string
}

// frontUnits are the front phases' cutters, in the order they run.
var frontUnits = map[int][]FrontUnit{
	1: {{"argvfront", cut.ArgvFront, ""}, {"exfront", cut.ExFront, ""}, {"extable", cut.ExTable, ""},
		{"filefront", cut.FileFront, ""}, {"quitfront", cut.QuitFront, ""}, {"readfront", cut.ReadFront, ""},
		{"onecmdfront", cut.OneCmdFront, ""}, {"optfront", cut.OptFront, ""}, {"noswap", cut.NoSwap, ""},
		{"norecover", cut.NoRecover, ""}, {"nomemfile", cut.NoMemfile, ""}},
	2: {{"nolocale", cut.NoLocale, ""}, {"nostartup", cut.NoStartup, ""}, {"nocmdopts", cut.NoCmdOpts, ""},
		{"nosession", cut.NoSession, ""}, {"whim18", nil, "whim18"}, {"noenc", cut.NoEnc, ""},
		{"nofencs", cut.NoFencs, ""}, {"nofenc", cut.NoFenc, ""}, {"utf8only", cut.Utf8Only, ""},
		{"noterm", cut.NoTerm, ""}, {"nomouse", cut.NoMouse, ""}, {"whim2a", nil, "whim2a"}},
	3: {{"noinert", cut.NoInert, ""}, {"notabs", cut.NoTabs, ""}, {"noarglist", cut.NoArgList, ""},
		{"nowindows", cut.NoWindows, ""}, {"nowinsizes", cut.NoWinSizes, ""}, {"nobuflist", cut.NoBufList, ""},
		{"nonfa", cut.NoNfa, ""}, {"noshellout", cut.NoShellOut, ""}, {"notags", cut.NoTags, ""},
		{"nosignals", cut.NoSignals, ""}, {"noequiclass", cut.NoEquiClass, ""}, {"nocindent", cut.NoCindent, ""},
		{"noucmd", cut.NoUcmd, ""}, {"noident", cut.NoIdent, ""}, {"nofnamemod", cut.NoFnameMod, ""},
		{"nocompl", cut.NoCompl, ""}, {"nocomplkeys", cut.NoComplKeys, ""}, {"noabbr", cut.NoAbbr, ""},
		{"whim3a", nil, "whim3a"}, {"whim3b", nil, "whim3b"}, {"whim3c", nil, "whim3c"},
		{"whim3d", nil, "whim3d"}, {"whim3e", nil, "whim3e"}, {"whim3f", nil, "whim3f"}},
}

// FrontUnits are front phase n's cutters, in order.
func FrontUnits(n int) []FrontUnit { return frontUnits[n] }

// Run is the unit on the graph.
func (u FrontUnit) Run(e *graph.Editor, w io.Writer) error {
	if u.Fn != nil {
		return u.Fn(e, w)
	}
	g, ok := phase.LookupGraph(u.Prog)
	if !ok {
		return fmt.Errorf("front: no graph program %s", u.Prog)
	}
	return g(e, w, nil)
}

// frontGraph is front phase n: what is unwritten asked first, the cutters
// in order, and then the closure -- FoldX, seeded with what is unwritten
// now and was not before, frontHold held: xform.FallOutOf's, on the graph
// (B2d), its report the text's line for line.
func frontGraph(n int) GraphStep {
	return func(e *graph.Editor, _ []string, w io.Writer) error {
		before := e.Unwritten(nil)
		for _, u := range frontUnits[n] {
			if err := u.Run(e, w); err != nil {
				return err
			}
		}
		st, err := e.FoldX(graph.FoldX{Before: before, Hold: frontHold})
		if err != nil {
			return fmt.Errorf("  %-12s %v", "fallout", err)
		}
		for _, l := range st.Report {
			fmt.Fprintf(w, "  %-12s %s\n", "fallout", l)
		}
		return nil
	}
}

// queryEmptyGraph is query-empty on the graph: the phase's query asked of
// the C view, the text's question exactly.
func queryEmptyGraph(e *graph.Editor, args []string, w io.Writer) error {
	t, err := e.Graph().C()
	if err != nil {
		return err
	}
	_, err = queryEmpty(t, args, w)
	return err
}
