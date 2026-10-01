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
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/dead"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/crefactor/xform"
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
	"keepbytes": plain(cut.KeepBytes),
	"lfonly":    plain(cut.LfOnly),
	"noabbr":    plain(cut.NoAbbr),
	"noarglist": plain(cut.NoArgList),
	// the command line cut at the front, and what that leaves unwritten
	// folded; read_cmd_fd is held: the product keeps it
	// The reform's drop packages, each a plain cut (doc/PIPELINE-REFORM.md
	// §7): the command line (D1), the Ex commands retired and their rows
	// deleted (D2), the commands that name a file (D4), the options (D3).
	"argvfront": plain(cut.ArgvFront),
	"exfront":   Step(exFront),
	"extable":   plain(cut.ExTable),
	"filefront": plain(cut.FileFront),
	"optfront":  plain(cut.OptFront),
	// and phase 1's one step: the five in order, and ONE fall-out closure
	// over what they leave unwritten together
	"front":       Step(xform.FallOutOf(xform.Step(front), frontHold...)),
	"nobackup":    plain(cut.NoBackup),
	"nobuflist":   plain(cut.NoBufList),
	"nochdir":     plain(cut.NoChdir),
	"nocindent":   plain(cut.NoCindent),
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
	"droplocal": dropLocal,
	"funcreach": funcReach,
	"edit":      runEdit,
	"query":     runQuery,

	"cemit":        Step(pipeline.Canonical),
	"includes":     Step(xform.Includes(whim.Includes)),
	"gototail":     Step(xform.GotoTail(whim.GotoTail)),
	"gotobreak":    Step(xform.GotoBreak()),
	"gotoloop":     Step(xform.GotoLoop()),
	"gotoblock":    Step(xform.GotoBlock()),
	"memberout":    Step(xform.MemberOut(whim.Core)),
	"stateparam":   Step(xform.StateParam(whim.RegEngine)),
	"localout":     Step(xform.LocalOut(whim.Core)),
	"structscalar": Step(xform.StructScalar(whim.Core)),
	"identity":     Step(xform.Identity(whim.Core)),
	"asciiclass":   Step(xform.AsciiClass(whim.Core)),
	"constbranch":  Step(xform.ConstBranch(whim.Core)),
	"query-empty":  queryEmpty,
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

// front runs the five front cuts in order.
func front(t []byte, args []string, w io.Writer) ([]byte, error) {
	var err error
	for _, op := range []Step{plain(cut.ArgvFront), exFront, plain(cut.ExTable),
		plain(cut.FileFront), plain(cut.OptFront)} {
		if t, err = op(t, args, w); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// frontHold is what the closure after the front cuts leaves to the phases
// that fold it by hand: read_cmd_fd, which the product keeps, and the dropped
// options' globals of optfrontHold.
var frontHold = append([]string{"read_cmd_fd"}, optfrontHold...)

// optfrontHold are the dropped options' globals the fall-out closure leaves
// to the phase that folds them by hand: where the closure's shape and the
// hand fold's differ, and the product has the hand fold's.
var optfrontHold = []string{
	"p_wmnu", // nowildmenu (phase 6) writes `a && b && c` flat
	// nobackup (phase 25) cuts the backup machinery around dobackup, whose
	// assignment also dereferences p_pm
	"p_bk", "p_wb", "p_pm", "p_bsk", "p_bex", "p_bkc", "p_bdir",
	// nowinsizes (phase 40) gives these their defaults -- p_ea is TRUE -- and
	// later phases fold their readers with them; the product keeps three
	"p_sb", "p_spr", "p_spk", "p_ea", "p_ead", "p_wh", "p_wmh", "p_wiw", "p_wmw",
	// phase 60 writes `regmatch.rm_ic = FALSE;` where the closure writes 0,
	// and drops a test on `acl_elapsed >= p_acl` whole
	"p_fic", "p_acl",
	// phase 62 cuts :!'s and :stop's whole `autowrite_all()` blocks, where
	// the closure would empty autowrite_all() and leave the blocks
	"p_aw", "p_awa", "p_write",
}

// exFront points every row phase 1 declares (internal/phase/001/delta.md,
// handed as REMOVED) at ex_ni: the 489 commands the product has not, 271 of
// them stubs in the seed already.
func exFront(t []byte, _ []string, w io.Writer) ([]byte, error) {
	names := strings.Fields(os.Getenv("REMOVED"))
	if len(names) == 0 {
		return nil, fmt.Errorf("exfront: no rows declared")
	}
	// a row ex_script_ni already is a stub as it stands
	var want []string
	script := 0
	for _, n := range names {
		q := regexp.QuoteMeta(n)
		if regexp.MustCompile(`\[CMD_\w+\] = \{\(char_u \*\)"` + q + `", sizeof\("` + q + `"\) - 1,\s*ex_script_ni\b`).Match(t) {
			script++
			continue
		}
		want = append(want, n)
	}
	out, done, already, err := cut.Retire(t, want)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "  exfront      %d commands retired, %d were stubs already\n", len(done), len(already)+script)
	return out, nil
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
