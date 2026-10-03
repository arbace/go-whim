package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
)

// viewCacheDir holds the graphs `whim view` imported, as Lisp, one a source
// file, each headed by the key it was made under.
const viewCacheDir = ".cache/graph"

// graphHeader is how the graph's Lisp begins (graph.Lisp's first line).
const graphHeader = ";; a C translation unit as a graph"

// runView is a read-only view of the program's graph (crefactor/graph/view,
// doc/GRAPH.md, *Views, read-only*):
//
//	whim view [FLAGS] VIEW ARG [FILE]
//	whim view [FLAGS] follow 'STEPS' ROOT [FILE]
//
// VIEW is callers F, callees F, uses NAME, member S.M, type T or def NAME;
// follow is an ad hoc view, its relation written as steps.  FILE is
// src/whim-vim.c by default, or a graph's Lisp (whim graph's).  A C file's
// graph is imported once and kept in .cache/graph/, keyed by the file's
// digest and the whim binary's, and read from there after: a view then
// starts in tens of milliseconds, not the import's second and a half.
//
//	--ids         ids on every node, `@ID` on every use, links by id
//	--depth N     levels of children (callers, callees: 2; uses: 1); -1 none
//	--show S      around each use: node, stmt (the default), fn, none
//	--stop HEADS  nodes of these heads, comma-separated, not expanded
//	--c           def: the C view of the definition, not its Lisp
//	--no-cache    import, and neither read nor write the cache
//	--time        the load, the view and the print timed, on stderr
func runView(args []string) int {
	var (
		ids, cOut, noCache, timing bool
		opt                        view.Options
		rest                       []string
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
		switch a {
		case "--ids":
			ids = true
		case "--c":
			cOut = true
		case "--no-cache":
			noCache = true
		case "--time":
			timing = true
		case "--depth":
			s, ok := next()
			n, err := strconv.Atoi(s)
			if !ok || err != nil {
				return viewUsage()
			}
			opt.Depth = n
			if n == 0 {
				opt.Depth = -1
			}
		case "--show":
			s, ok := next()
			sh, err := view.ParseShow(s)
			if !ok || err != nil {
				return viewUsage()
			}
			opt.Show = sh
		case "--stop":
			s, ok := next()
			if !ok {
				return viewUsage()
			}
			opt.Stop = strings.Split(s, ",")
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 2 {
		return viewUsage()
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
		return viewUsage()
	}
	start := time.Now()
	g, how, err := loadGraph(file, noCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view         %v\n", err)
		return 1
	}
	loaded := time.Since(start)
	start = time.Now()
	ix := view.NewIndex(g)
	indexed := time.Since(start)
	start = time.Now()
	roots, err := ix.Find(rest[need-1])
	if err == nil && name == "member" && !isMemberNode(ix, roots[0]) {
		err = fmt.Errorf("%s is not a member: member is S.M", rest[need-1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view         %v\n", err)
		return 1
	}
	var out bytes.Buffer
	p := view.Printer{IDs: ids}
	for _, root := range roots {
		var t *view.Tree
		switch name {
		case "callers":
			t = view.Callers(ix, root, opt)
		case "callees":
			t = view.Callees(ix, root, opt)
		case "uses":
			t = view.Uses(ix, root, opt)
		case "member":
			t = view.Member(ix, root, opt)
		case "type":
			s, err := ix.Aggregate(rest[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "  view         %v\n", err)
				return 1
			}
			t = view.Type(ix, s, opt)
		case "def":
			d := view.Def(ix, root)
			if cOut {
				c, err := clisp.Print([]*clisp.Node{graph.Lisp(d)})
				if err != nil {
					fmt.Fprintf(os.Stderr, "  view         %v\n", err)
					return 1
				}
				out.Write(c)
			} else {
				out.WriteString(p.Form(d))
			}
			continue
		case "follow":
			steps, err := view.ParseSteps(rest[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "  view         %v\n", err)
				return 1
			}
			t = view.Build(ix, root, view.Spec{Name: "follow", Child: "to", Rel: view.Steps(steps),
				Depth: followDepth(opt.Depth), Stop: opt.Stop, Show: opt.Show})
			t.Notes = append(t.Notes, "steps: "+rest[1])
		default:
			return viewUsage()
		}
		out.WriteString(p.Tree(ix, t))
	}
	built := time.Since(start)
	os.Stdout.Write(out.Bytes())
	if timing {
		fmt.Fprintf(os.Stderr, "  view         %s %dms, indexed %dms, the view built and printed %dms\n",
			how, loaded.Milliseconds(), indexed.Milliseconds(), built.Milliseconds())
	}
	return 0
}

func isMemberNode(ix *view.Index, n *graph.Node) bool {
	p := ix.Parent(n)
	return n.Is("member") || p != nil && graph.IsTypeDef(p) && (p.Is("struct") || p.Is("union"))
}

func viewUsage() int {
	fmt.Fprintln(os.Stderr, "usage: whim view [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--c] [--no-cache] [--time]\n"+
		"                 callers F | callees F | uses NAME | member S.M | type T | def NAME | follow 'STEPS' ROOT  [FILE]\n"+
		"  a root is a name, S.M, struct T, F/local or #ID; STEPS are refers, typed, contains (each with <), inside, ^fn, ^stmt, call")
	return 2
}

// loadGraph is file's graph: read, when file is a graph's Lisp or the
// cache holds this file's under the same key, else imported and cached.
func loadGraph(file string, noCache bool) (*graph.Graph, string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	if bytes.HasPrefix(src, []byte(graphHeader)) {
		g, err := graph.Read(src)
		return g, "read", err
	}
	if noCache {
		g, _, err := graph.Import(file, src)
		return g, "imported", err
	}
	key := cacheKey(src)
	abs, _ := filepath.Abs(file)
	sum := sha256.Sum256([]byte(abs))
	cached := filepath.Join(viewCacheDir, filepath.Base(file)+"-"+hex.EncodeToString(sum[:4])+".lisp")
	head := ";; key " + key + "\n"
	if text, err := os.ReadFile(cached); err == nil && bytes.HasPrefix(text, []byte(head)) {
		if g, err := graph.Read(text[len(head):]); err == nil {
			return g, "read from " + cached, nil
		}
	}
	g, _, err := graph.Import(file, src)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(viewCacheDir, 0o755); err == nil {
		tmp := cached + ".tmp"
		if os.WriteFile(tmp, append([]byte(head), g.Lisp()...), 0o644) == nil {
			_ = os.Rename(tmp, cached)
		}
	}
	return g, "imported (and cached)", nil
}

// cacheKey is the source's digest and the running binary's identity: a
// rebuilt whim may import differently, so its graphs are made again.
func cacheKey(src []byte) string {
	sum := sha256.Sum256(src)
	k := hex.EncodeToString(sum[:])
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			k += fmt.Sprintf("-%x-%x", st.Size(), st.ModTime().UnixNano())
		}
	}
	return k
}

// followDepth is an ad hoc view's depth: 1 unless asked, -1 no limit.
func followDepth(d int) int {
	switch {
	case d == 0:
		return 1
	case d < 0:
		return 0
	}
	return d
}
