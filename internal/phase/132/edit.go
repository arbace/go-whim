package p132

// Whim phase 132 -- nothing frees.  See GOAL.md.
//
// host_free() has had an empty body since phase 124, so vim_free() -- a NULL
// test around it -- does nothing observable.  All 273 calls to either in the
// core go; the two whose argument decrements a counter keep the decrement.
// vim_free() is then called by nothing and the sweep takes it
// (internal/gen/FINDINGS.md, 9).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim132", Edit) }

var (
	w132Call  = regexp.MustCompile(`^([ \t]*)(vim_free|host_free)\((.*)\);[ \t]*$`)
	w132Cast  = regexp.MustCompile(`\((?:const\s+)?(?:struct\s+)?[A-Za-z_]\w*\s*\*+\s*\)`)
	w132Fn    = regexp.MustCompile(`[A-Za-z_]\w*\s*\(`)
	w132Step  = regexp.MustCompile(`(\+\+|--)[A-Za-z_]\w*|[A-Za-z_]\w*(\+\+|--)`)
	w132Write = regexp.MustCompile(`[^=!<>]=[^=]`)
)

// W132Rule is the rule, exported so the check applies the identical one: the
// statement that replaces a call to vim_free() or host_free(), or ok=false if
// the argument does something this rule cannot keep.  A side-effect-free
// argument: nothing.  One whose only effect is one ++ or -- (two, in this
// tree: termcodes[--tc_len].code, uep->ue_array[--n].ul_line): that step, as a
// statement of its own.
func W132Rule(indent, arg string) (repl string, ok bool) {
	a := w132Cast.ReplaceAllString(arg, "")
	a = regexp.MustCompile(`__builtin_offsetof\s*\(`).ReplaceAllString(a, "(")
	if w132Fn.MatchString(a) || w132Write.MatchString(a) {
		return "", false
	}
	steps := w132Step.FindAllString(a, -1)
	switch len(steps) {
	case 0:
		return "", true
	case 1:
		return indent + steps[0] + ";\n", true
	}
	return "", false
}

// Whim132 removes every call to vim_free() and host_free() from the core.
//
// Since phase 124 host_free() has an empty Body -- the host's arena is a bump
// allocator and the garbage collector is assumed -- so vim_free(), a NULL test
// around it, does nothing observable, and neither does any call to either.
// The Go transpilation dropped every one (internal/gen/FINDINGS.md, 9); this is the C
// catching up.  A call goes when its argument has no side effect, and becomes
// its one ++ or -- when that is its only effect; any other argument refuses.
// vim_free() itself is then called by nothing and the sweep takes it; the
// host keeps host_free(), which its formatter still calls.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "free", W: w}
	var Out bytes.Buffer
	n, steps := 0, 0
	// the host starts at the first #include: it is not the core's to change
	core := text
	if i := bytes.Index(text, []byte("\n#include")); i >= 0 {
		core = text[:i+1]
	}
	for _, line := range bytes.SplitAfter(core, []byte("\n")) {
		m := w132Call.FindSubmatch(bytes.TrimRight(line, "\n"))
		if m == nil {
			Out.Write(line)
			continue
		}
		repl, ok := W132Rule(string(m[1]), string(m[3]))
		if !ok {
			return nil, p.Die("%s(%s) does something this phase cannot keep", m[2], m[3])
		}
		n++
		if repl != "" {
			steps++
		}
		Out.WriteString(repl)
	}
	// a local that was freed and otherwise only given values is now only given
	// values, and gcc says so; it goes with them
	swept, took := edit.DeadStores(Out.Bytes())
	Out.Reset()
	Out.Write(swept)
	// the host's formatter called the core's vim_free() three times: across the
	// boundary, and for nothing, host_free() being empty.  It calls its own.
	host := text[len(core):]
	if k := bytes.Count(host, []byte("vim_free(")); k != 3 {
		return nil, p.Die("the host calls vim_free() %d times, and this phase was written against 3", k)
	}
	Out.Write(bytes.ReplaceAll(host, []byte("vim_free("), []byte("host_free(")))
	if n != 273 {
		return nil, p.Die("%d calls to vim_free() and host_free() in the core, and this phase was written against 273", n)
	}
	p.Say(fmt.Sprintf("273 calls to vim_free() and host_free() in the core go -- %d of them keep the -- their argument did as a statement of its own -- and the host's three calls to the core's vim_free() call its own host_free()", steps))
	p.Say(fmt.Sprintf("%d locals were only freed and given values, and go with their stores: %s", len(took), strings.Join(took, " ")))
	return Out.Bytes(), nil
}
