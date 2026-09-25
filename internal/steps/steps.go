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
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/dead"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/cmdtab"
	"github.com/arbace/go-whim/internal/cut"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// A Step is one transformation: the tree in, the tree out, its report on w.
type Step func(text []byte, args []string, w io.Writer) ([]byte, error)

// plain wraps a cutter that takes no arguments of its own.
func plain(f func([]byte, io.Writer) ([]byte, error)) Step {
	return func(t []byte, _ []string, w io.Writer) ([]byte, error) { return f(t, w) }
}

var ops = map[string]Step{
	"keepbytes":   plain(cut.KeepBytes),
	"lfonly":      plain(cut.LfOnly),
	"noabbr":      plain(cut.NoAbbr),
	"noarglist":   plain(cut.NoArgList),
	"noargv0":     plain(cut.NoArgv0),
	"nobackup":    plain(cut.NoBackup),
	"nobuflist":   plain(cut.NoBufList),
	"nochdir":     plain(cut.NoChdir),
	"nocindent":   plain(cut.NoCindent),
	"nocmdargs":   plain(cut.NoCmdArgs),
	"nocmdopts":   plain(cut.NoCmdOpts),
	"nocompl":     plain(cut.NoCompl),
	"nocomplkeys": plain(cut.NoComplKeys),
	"noconv":      plain(cut.NoConv),
	"noenc":       plain(cut.NoEnc),
	"noequiclass": plain(cut.NoEquiClass),
	"nofenc":      plain(cut.NoFenc),
	"nofencs":     plain(cut.NoFencs),
	"nofind":      plain(cut.NoFind),
	"nofloat":     plain(cut.NoFloat),
	"nofnamemod":  plain(cut.NoFnameMod),
	"nogetenv":    plain(cut.NoGetEnv),
	"noglob":      plain(cut.NoGlob),
	"nohome":      plain(cut.NoHome),
	"noident":     plain(cut.NoIdent),
	"noinert":     plain(cut.NoInert),
	"noinertopts": plain(cut.NoInertOpts),
	"nointro":     plain(cut.NoIntro),
	"nolocale":    plain(cut.NoLocale),
	"nomemfile":   plain(cut.NoMemfile),
	"nomouse":     plain(cut.NoMouse),
	"nonfa":       plain(cut.NoNfa),
	"noowner":     plain(cut.NoOwner),
	"norecover":   plain(cut.NoRecover),
	"noruntime":   plain(cut.NoRuntime),
	"nosession":   plain(cut.NoSession),
	"noshellout":  plain(cut.NoShellOut),
	"nosignals":   plain(cut.NoSignals),
	"nostartup":   plain(cut.NoStartup),
	"nostat":      plain(cut.NoStat),
	"noswap":      plain(cut.NoSwap),
	"notabs":      plain(cut.NoTabs),
	"notags":      plain(cut.NoTags),
	"noterm":      plain(cut.NoTerm),
	"noucmd":      plain(cut.NoUcmd),
	"nowild":      plain(cut.NoWild),
	"nowildmenu":  plain(cut.NoWildMenu),
	"nowindows":   plain(cut.NoWindows),
	"nowinsizes":  plain(cut.NoWinSizes),
	"onebuffer":   plain(cut.OneBuffer),
	"oneoptset":   plain(cut.OneOptSet),
	"optreaders":  plain(cut.OptReaders),
	"utf8only":    plain(cut.Utf8Only),

	// The five that take arguments, and the three that only ask a question.
	"dropoptions": dropOptions,
	"droplocal":   dropLocal,
	"retire":      retire,
	"dropopts":    dropOpts,
	"funcreach":   funcReach,
	"cmdidxs":     cmdIdxs,
	"edit":        runEdit,
	"query":       runQuery,

	"cemit":             Step(pipeline.Canonical),
	"includes":          Step(xform.Includes(whim.Includes)),
	"gototail":          Step(xform.GotoTail(whim.GotoTail)),
	"gotobreak":         Step(xform.GotoBreak()),
	"gotoloop":          Step(xform.GotoLoop()),
	"query-empty":       queryEmpty,
	"query-dropoptions": queryDropOptions,
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

// dropOptions is `dropoptions <name>... [--strict] [--local]`.
func dropOptions(t []byte, args []string, w io.Writer) ([]byte, error) {
	strict, local := false, false
	var names []string
	for _, a := range args {
		switch {
		case a == "--strict":
			strict = true
		case a == "--local":
			local = true
		case strings.HasPrefix(a, "--"):
		default:
			names = append(names, a)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("dropoptions: no option named")
	}
	out, whitelisted, err := cut.DropOptions(t, names, strict, local)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "  options      %d rows dropped (%s), %d modeline entries with them\n",
		len(names), strings.Join(names, ", "), whitelisted)
	return out, nil
}

// dropLocal is `droplocal <field>...`, one field at a time, reporting each.
func dropLocal(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("droplocal: no field named")
	}
	for _, bvar := range args {
		var n int
		var err error
		t, n, err = cut.DropLocal(t, bvar)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  droplocal    %-10s %d plumbing sites\n", bvar, n)
	}
	return t, nil
}

// retire is `retire <command>...`.
func retire(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("retire: no command named")
	}
	out, done, already, err := cut.Retire(t, args)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "  retire       %d commands now answer \"not implemented\": %s\n",
		len(done), strings.Join(done, " "))
	if len(already) > 0 {
		fmt.Fprintf(w, "  retire       %d were already stubs: %s\n",
			len(already), strings.Join(already, " "))
	}
	return out, nil
}

// dropOpts is `dropopts <-x|--long>...`, the command-line options.
func dropOpts(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("dropopts: no option named")
	}
	return cut.DropOpts(t, args, w)
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

// cmdIdxs is `cmdidxs --check`: the derived first-two-letters index still
// reproduces byte for byte.  It changes nothing.  The tool reads a path, so the
// text is handed to it as one.
func cmdIdxs(t []byte, args []string, w io.Writer) ([]byte, error) {
	check := false
	for _, a := range args {
		if a == "--check" {
			check = true
		}
	}
	if !check {
		return nil, fmt.Errorf("cmdidxs: only --check is a step")
	}
	f, err := os.CreateTemp("", "cmdidxs.*.c")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(t); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := cmdtab.CheckCmdIdxs(f.Name()); err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "  cmdidxs      ex_cmdidxs block reproduces byte for byte\n")
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

// queryDropOptions asks which option rows have no variable and drops exactly
// those.  The floor is the phase program's: a pattern that stops matching
// returns few names rather than none, and dropping them would be silent.
func queryDropOptions(t []byte, args []string, w io.Writer) ([]byte, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("query-dropoptions: usage <query> <floor>")
	}
	floor, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, err
	}
	out, err := ask(t, args[:1])
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	if len(names) <= floor {
		return nil, fmt.Errorf("  novar        found only %d rows without a variable -- "+
			"the pattern stopped matching", len(names))
	}
	fmt.Fprintf(w, "  novar        %d options have no variable\n", len(names))
	return dropOptions(t, names, w)
}

// ask runs a query and returns what it printed.
func ask(t []byte, args []string) (string, error) {
	var b strings.Builder
	if _, err := runQuery(t, args, &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

// MinMaxProbe is the two lines phase 109 needs from the host's <sys/param.h>:
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
