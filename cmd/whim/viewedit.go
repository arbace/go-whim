package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/internal/whim"
)

// runViewEdit is an edit made through a view (crefactor/graph/view's Edit;
// doc/GRAPH.md, *Editable views, the first gate*):
//
//	whim view-edit [FLAGS] VIEW ARG [FILE] < EDITED
//	whim view-edit [FLAGS] -i EDITED VIEW ARG [FILE]
//
// The view is printed as `whim view` prints it (with the same flags), the
// edited text -- that view, changed, with or without its ids -- is aligned
// with it form by form, and the change is made on the graph: a renaming, a
// form replaced, items inserted or deleted, each a C fragment made nodes in
// context (FRAG) and checked where it is made; then the view is printed
// again and must say what was written, the uses a deletion left are
// refused (or closed over, --fallout), the types re-checked and the
// graph's invariants held.  A refusal names its reason and writes nothing.
// What it writes is the edited program: its C view (the default), its
// graph's Lisp (--lisp), or the view printed again (--view); the edits
// made are listed on stderr.
//
//	--ids, --depth N, --show S, --stop HEADS   the view, as whim view's
//	-i EDITED       the edited text from a file, not stdin
//	-o OUT          the output to a file, not stdout
//	--fallout       a deletion's dangling uses closed over by the fall-out
//	                closure (vim's options, internal/whim's GraphFallOut);
//	                its acts listed
//	--lisp, --view  what is written
//	--no-cache      import FILE, and neither read nor write the cache
//	--time          the load, the edit and the print timed, on stderr
func runViewEdit(args []string) int {
	var (
		ids, noCache, timing, fallout bool
		out                           = "c"
		in, outFile                   string
		opt                           view.Options
		rest                          []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, bool) {
			i++
			if i >= len(args) {
				return "", false
			}
			return args[i], true
		}
		var ok = true
		switch a {
		case "--ids":
			ids = true
		case "--no-cache":
			noCache = true
		case "--time":
			timing = true
		case "--fallout":
			fallout = true
		case "--lisp":
			out = "lisp"
		case "--view":
			out = "view"
		case "-i":
			in, ok = next()
		case "-o":
			outFile, ok = next()
		case "--depth":
			var s string
			s, ok = next()
			n, err := strconv.Atoi(s)
			ok = ok && err == nil
			opt.Depth = n
			if n == 0 {
				opt.Depth = -1
			}
		case "--show":
			var s string
			s, ok = next()
			sh, err := view.ParseShow(s)
			ok = ok && err == nil
			opt.Show = sh
		case "--stop":
			var s string
			s, ok = next()
			opt.Stop = strings.Split(s, ",")
		default:
			rest = append(rest, a)
		}
		if !ok {
			return viewEditUsage()
		}
	}
	if len(rest) < 2 {
		return viewEditUsage()
	}
	name, need := rest[0], 2
	if name == "follow" {
		need = 3
	}
	file := "src/whim-vim.c"
	switch len(rest) {
	case need:
	case need + 1:
		file = rest[need]
	default:
		return viewEditUsage()
	}
	var edited []byte
	var err error
	if in != "" {
		edited, err = os.ReadFile(in)
	} else {
		edited, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-edit    %v\n", err)
		return 1
	}
	start := time.Now()
	g, how, err := loadGraph(file, noCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-edit    %v\n", err)
		return 1
	}
	loaded := time.Since(start)
	p := view.Printer{IDs: ids}
	render := func(ix *view.Index) (string, []view.Span, error) {
		return view.Text(ix, name, rest[1:need], opt, p, false)
	}
	eo := view.EditOptions{IDs: ids}
	if fallout {
		fo := whim.GraphFallOut
		eo.FallOut = &fo
	}
	start = time.Now()
	r, err := view.Edit(g, render, string(edited), eo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-edit    %v\n", err)
		return 1
	}
	editTime := time.Since(start)
	for _, o := range r.Ops {
		fmt.Fprintf(os.Stderr, "  edit         %s\n", o)
	}
	for _, rm := range r.FallOut.Removed {
		c, _ := clisp.PrintItems([]*clisp.Node{graph.Lisp(rm.Item)})
		fmt.Fprintf(os.Stderr, "  fall-out     %s in %s (the use of %s): %s\n", rm.Rule, rm.Fn, graph.DeclName(rm.Target), strings.Join(strings.Fields(c), " "))
	}
	if len(r.Closure) > 0 {
		fmt.Fprintf(os.Stderr, "  fall-out     %d acts, %d values, %d folded, %d branches, %d rounds\n",
			len(r.Closure), r.FallOut.Values, r.FallOut.Folded, r.FallOut.Branches, r.FallOut.Rounds)
	}
	fmt.Fprintf(os.Stderr, "  re-check     %s\n", r.Recheck)
	start = time.Now()
	var text []byte
	switch out {
	case "lisp":
		text = r.Graph.Lisp()
	case "view":
		text = []byte(r.Text)
	default:
		text, err = r.Graph.C()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  view-edit    %v\n", err)
			return 1
		}
	}
	if outFile != "" {
		err = os.WriteFile(outFile, text, 0o644)
	} else {
		_, err = os.Stdout.Write(text)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-edit    %v\n", err)
		return 1
	}
	if timing {
		fmt.Fprintf(os.Stderr, "  view-edit    %s %dms, the edit %dms, printed %dms\n",
			how, loaded.Milliseconds(), editTime.Milliseconds(), time.Since(start).Milliseconds())
	}
	return 0
}

func viewEditUsage() int {
	fmt.Fprintln(os.Stderr, "usage: whim view-edit [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--fallout] [--lisp|--view] [-i EDITED] [-o OUT] [--no-cache] [--time]\n"+
		"                      callers F | callees F | uses NAME | member S.M | type T | def NAME | follow 'STEPS' ROOT  [FILE]  < EDITED\n"+
		"  the view as whim view prints it, edited, made an edit of the graph; written: the C view, --lisp the graph, --view the view again")
	return 2
}
