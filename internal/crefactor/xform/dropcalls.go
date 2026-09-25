package xform

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	ctext "github.com/arbace/go-whim/internal/crefactor/text"
)

// DropCallsKnobs is what DropCalls is told.
type DropCallsKnobs struct {
	// Core is where the core ends; the calls go from the core only.  With no
	// end found the whole text is the core.
	Core Core
	// Funcs are the functions whose calls do nothing: an empty body, or one
	// that only calls another of them.
	Funcs []string
	// Redirect, pair by pair, renames in the host the calls of a core
	// function -- one the sweep will take once its core calls are gone -- to
	// the host's own: {from, to}.
	Redirect [][2]string
}

var (
	dropCast  = regexp.MustCompile(`\((?:const\s+)?(?:struct\s+)?[A-Za-z_]\w*\s*\*+\s*\)`)
	dropFn    = regexp.MustCompile(`[A-Za-z_]\w*\s*\(`)
	dropStep  = regexp.MustCompile(`(\+\+|--)[A-Za-z_]\w*|[A-Za-z_]\w*(\+\+|--)`)
	dropWrite = regexp.MustCompile(`[^=!<>]=[^=]`)
)

// DropCallRule is the statement that replaces a call statement to a no-op
// function, or ok=false if the argument does something this rule cannot
// keep.  A side-effect-free argument: nothing.  One whose only effect is one
// ++ or --: that step, as a statement of its own.
func DropCallRule(indent, arg string) (repl string, ok bool) {
	a := dropCast.ReplaceAllString(arg, "")
	a = regexp.MustCompile(`__builtin_offsetof\s*\(`).ReplaceAllString(a, "(")
	if dropFn.MatchString(a) || dropWrite.MatchString(a) {
		return "", false
	}
	steps := dropStep.FindAllString(a, -1)
	switch len(steps) {
	case 0:
		return "", true
	case 1:
		return indent + steps[0] + ";\n", true
	}
	return "", false
}

// DropCalls is the step that removes from the core every call statement,
// alone on its line, of a function that does nothing (k.Funcs).  A call goes
// when its argument has no side effect, and becomes its one ++ or -- when
// that is its only effect; any other argument refuses.  A local then only
// given values goes with its stores (ctext.DeadStores), and the host's calls
// are redirected as k.Redirect says.
//
// Its arguments are counts it requires exactly: `--calls N`, the calls that
// go, and `--redirected N`, the host's calls redirected.  Without them
// there is no count.
func DropCalls(k DropCallsKnobs) Step {
	alt := make([]string, len(k.Funcs))
	for i, f := range k.Funcs {
		alt[i] = regexp.QuoteMeta(f)
	}
	call := regexp.MustCompile(`^([ \t]*)(` + strings.Join(alt, "|") + `)\((.*)\);[ \t]*$`)
	named := strings.Join(k.Funcs, "(), ") + "()"
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := ctext.Ph{Tag: "free", W: w}
		f, err := flags(p.Tag, args, "--calls", "--redirected")
		if err != nil {
			return nil, err
		}
		if len(k.Funcs) == 0 {
			return nil, p.Die("no function was named")
		}
		var out bytes.Buffer
		n, steps := 0, 0
		core := text
		if i := k.Core(text); i >= 0 {
			core = text[:i+1]
		}
		for _, line := range bytes.SplitAfter(core, []byte("\n")) {
			m := call.FindSubmatch(bytes.TrimRight(line, "\n"))
			if m == nil {
				out.Write(line)
				continue
			}
			repl, ok := DropCallRule(string(m[1]), string(m[3]))
			if !ok {
				return nil, p.Die("%s(%s) does something this step cannot keep", m[2], m[3])
			}
			n++
			if repl != "" {
				steps++
			}
			out.WriteString(repl)
		}
		swept, took := ctext.DeadStores(out.Bytes())
		out.Reset()
		out.Write(swept)
		host := text[len(core):]
		redirected := 0
		for _, r := range k.Redirect {
			redirected += bytes.Count(host, []byte(r[0]+"("))
			host = bytes.ReplaceAll(host, []byte(r[0]+"("), []byte(r[1]+"("))
		}
		if want, ok := f["--redirected"]; ok && redirected != want {
			return nil, p.Die("the host makes %d calls to redirect, and this step was told %d", redirected, want)
		}
		out.Write(host)
		if want, ok := f["--calls"]; ok && n != want {
			return nil, p.Die("%d calls to %s in the core, and this step was told %d", n, named, want)
		}
		p.Say(fmt.Sprintf("%d calls to %s in the core go -- %d of them keep the ++ or -- their argument did, as a statement of its own -- and %d of the host's calls are redirected", n, named, steps, redirected))
		p.Say(fmt.Sprintf("%d locals were only passed to them and given values, and go with their stores: %s", len(took), strings.Join(took, " ")))
		return out.Bytes(), nil
	}
}
