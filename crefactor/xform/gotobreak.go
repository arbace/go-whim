package xform

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// GotoBreak is the step that takes the gotos a structured statement already
// says:
//
//   - a goto to the label control reaches next anyway -- it would fall
//     through to it, leaving only blocks, ifs, labels and the end of a
//     switch -- goes;
//   - a goto whose label is where control goes when the innermost loop or
//     switch around it ends is `break;`, which goes there.
//
// A label no goto reaches any more goes.  A goto that leaves more than one
// loop or switch to reach its label is held: C has no break for it.  A
// function with a computed goto, a label's address, a local label, a nested
// function or a jump inside an expression is left as it is.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// gotos go.  Without it there is none.
func GotoBreak() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "gotobreak", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		return gotoBreak(p, text, f["--at-least"])
	}
}

func gotoBreak(p edit.Ph, text []byte, floor int) ([]byte, error) {
	ast, err := parse(text)
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	var rw []rewrite
	breaks, falls, held, labels, odd := 0, 0, 0, 0, 0
	functions(ast, func(body *cc.CompoundStatement) {
		fl := flowOf(body)
		if fl.odd {
			odd++
			return
		}
		took := map[string]int{}
		for _, j := range fl.jumps {
			if j.js.Case != cc.JumpStatementGoto {
				continue
			}
			name := j.js.Token2.SrcStr()
			a, z := sweep.Span(j.js, file, text)
			if f, ok := next(j.stack); ok && marks(f, name) {
				// control falls to the label: the goto goes (a lone
				// statement under an if keeps its place as `;`)
				with := ""
				if j.stack[len(j.stack)-1].kind != inBlock {
					with = ";"
				}
				rw = append(rw, rewrite{a, z, with})
				took[name]++
				falls++
				continue
			}
			b := breakable(j.stack)
			if b < 0 {
				continue
			}
			if f, ok := next(j.stack[:b]); ok && marks(f, name) {
				rw = append(rw, rewrite{a, z, "break;"})
				took[name]++
				breaks++
				continue
			}
			for o := breakable(j.stack[:b]); o >= 0; o = breakable(j.stack[:o]) {
				if f, ok := next(j.stack[:o]); ok && marks(f, name) {
					held++
					break
				}
			}
		}
		for name, n := range took {
			if n == fl.gotos[name] {
				a, z := dropLabel(fl.labels[name].ls, text)
				rw = append(rw, rewrite{a, z, ""})
				labels++
			}
		}
	})
	out, ok := apply(text, rw)
	if !ok {
		return nil, p.Die("two rewrites overlap")
	}
	p.Say(fmt.Sprintf("%d gotos are break; %d gotos to where control falls anyway go; %d held, leaving more than one loop or switch; %d labels go; %d functions left, doing what the walk does not model",
		breaks, falls, held, labels, odd))
	if breaks+falls < floor {
		return nil, p.Die("%d gotos go, fewer than the %d this step was told to expect", breaks+falls, floor)
	}
	return out, nil
}
