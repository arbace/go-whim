package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/internal/whim"
)

// runViewServe is an editing session on the program's graph for an editor
// to drive (doc/GRAPH.md, *The editing server*): FILE's graph loaded once
// (as whim view loads it), then a request a line on stdin, an answer on
// stdout, the framing view-clj --serve's -- a header, the body, `;;end`:
//
//	ok N [WORDS]        N the body's bytes; then the body, and ;;end
//	error N             the message as the body
//
// One buffer is open at a time: a view's text being typed into
// (crefactor/graph/view's Buffer), its edits made in place in one session
// and undone in order.  Words are blank-separated; '...' quotes a word as
// it is, "..." as a Go string (\n, \", \\), for a change's text.
//
//	open [--ids] [--fallout] [--depth N] [--show S] [--stop H] VIEW ARG   the buffer on a view;
//	                        body: its text; --fallout closes over the uses a deletion leaves
//	                        (they are refused without it)
//	text                    the buffer's text; header: ok N STATUS [REASON]
//	change FROM TO TEXT [CURSOR]  bytes FROM to TO of the buffer (or LINE:COL)
//	                        replaced by TEXT; header: ok N STATUS CURSOR [REASON] --
//	                        applied, pending, layout or same, CURSOR the change's end
//	                        in the body, or the CURSOR given (a byte of the text after
//	                        the change: the editor's) where it is in the body; an
//	                        applied change that put top-level forms beside the
//	                        view's adds `added NAME...`
//	at POS [VIEW]           the buffer reopened at the cursor: VIEW (the buffer's, by
//	                        default) of what the node at POS is about; header: ok N #ID NAME
//	rename POS NAME         the entity at POS renamed, its every declaration and use, as
//	                        one edit; header: ok N #ID D declarations U uses
//	undo                    the last edit undone; body: the buffer printed again
//	revert                  what is pending dropped; body: the buffer
//	c NAME                  NAME's definition as C, from the graph as it is
//	write FILE              the program's C view written to FILE; header: ok 0 BYTES
//	edits | stats | ping | quit
//
//	whim view-serve [--no-cache] [FILE]
func runViewServe(args []string) int {
	noCache, file := false, "src/whim-vim.c"
	for _, a := range args {
		switch {
		case a == "--no-cache":
			noCache = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(os.Stderr, "usage: whim view-serve [--no-cache] [FILE]")
			return 2
		default:
			file = a
		}
	}
	start := time.Now()
	g, how, err := loadGraph(file, noCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-serve   %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "  view-serve   %s %s %dms; a request a line\n", file, how, time.Since(start).Milliseconds())
	serveViews(g, file, os.Stdin, os.Stdout)
	return 0
}

// a server is one session and its buffer.
type viewServer struct {
	g     *graph.Graph
	file  string
	s     *view.Session
	b     *view.Buffer
	name  string   // the buffer's view
	vargs []string // its arguments, the root last
	opt   view.Options
	p     view.Printer
	fo    *graph.FallOutOptions
}

// serveViews answers requests from in on out until quit or the input's
// end.
func serveViews(g *graph.Graph, file string, in io.Reader, out io.Writer) {
	sv := &viewServer{g: g, file: file, s: view.NewSession(g)}
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	defer w.Flush()
	for {
		line, err := r.ReadString('\n')
		if line = strings.TrimRight(line, "\r\n"); line != "" {
			head, body, rerr := sv.answer(line)
			if errors.Is(rerr, errQuit) {
				answerFrame(w, "ok", "", "")
				return
			}
			if rerr != nil {
				answerFrame(w, "error", "", rerr.Error())
			} else {
				answerFrame(w, "ok", head, body)
			}
			w.Flush()
		}
		if err != nil {
			return
		}
	}
}

var errQuit = errors.New("quit")

// answerFrame writes `KIND N[ HEAD]`, the body, and `;;end` on a line of
// its own.
func answerFrame(w io.Writer, kind, head, body string) {
	h := fmt.Sprintf("%s %d", kind, len(body))
	if head != "" {
		h += " " + strings.ReplaceAll(head, "\n", " ")
	}
	nl := ""
	if body != "" && !strings.HasSuffix(body, "\n") {
		nl = "\n"
	}
	fmt.Fprintf(w, "%s\n%s%s;;end\n", h, body, nl)
}

func (sv *viewServer) answer(line string) (head, body string, err error) {
	ws, err := splitRequest(line)
	if err != nil {
		return "", "", err
	}
	if len(ws) == 0 {
		return "", "", fmt.Errorf("an empty request")
	}
	need := func(n int) error {
		if len(ws)-1 < n {
			return fmt.Errorf("%s takes %d arguments", ws[0], n)
		}
		if ws[0] != "open" && ws[0] != "at" && len(ws)-1 > n {
			return fmt.Errorf("%s takes %d arguments", ws[0], n)
		}
		return nil
	}
	buf := func() error {
		if sv.b == nil {
			return fmt.Errorf("no buffer: open a view first")
		}
		return nil
	}
	switch ws[0] {
	case "quit":
		return "", "", errQuit
	case "ping":
		return "", "pong\n", nil
	case "open":
		if err := sv.open(ws[1:]); err != nil {
			return "", "", err
		}
		return "", sv.b.Text, nil
	case "text":
		if err := buf(); err != nil {
			return "", "", err
		}
		if sv.b.Reason != "" {
			return "pending " + sv.b.Reason, sv.b.Text, nil
		}
		return "same", sv.b.Text, nil
	case "change":
		if len(ws) != 4 && len(ws) != 5 {
			return "", "", fmt.Errorf("change takes FROM TO TEXT [CURSOR]")
		}
		if err := buf(); err != nil {
			return "", "", err
		}
		from, err := textOffsetEnd(sv.b.Text, ws[1])
		if err != nil {
			return "", "", err
		}
		to, err := textOffsetEnd(sv.b.Text, ws[2])
		if err != nil {
			return "", "", err
		}
		typed := sv.b.Text[:min(from, len(sv.b.Text))] + ws[3] + sv.b.Text[min(max(to, from), len(sv.b.Text)):]
		r, err := sv.b.Change(from, to, ws[3])
		if err != nil {
			return "", "", err
		}
		cursor := r.Cursor
		if len(ws) == 5 {
			// the editor's cursor, in the text after the change: where it
			// is in the text the buffer is now
			c, err := strconv.Atoi(ws[4])
			if err != nil || c < 0 || c > len(typed) {
				return "", "", fmt.Errorf("change: a cursor %s not in the text of %d bytes", ws[4], len(typed))
			}
			cursor = view.MapPos(typed, sv.b.Text, c)
		}
		head = fmt.Sprintf("%s %d", r.Status, cursor)
		if r.Reason != "" {
			head += " " + r.Reason
		}
		if r.Result != nil && len(r.Result.Added) > 0 {
			head += " added"
			for _, f := range r.Result.Added {
				head += " " + graph.DeclName(f)
			}
		}
		return head, sv.b.Text, nil
	case "rename":
		if err := need(2); err != nil {
			return "", "", err
		}
		if err := buf(); err != nil {
			return "", "", err
		}
		off, err := textOffset(sv.b.Text, ws[1])
		if err != nil {
			return "", "", err
		}
		rn, d, err := sv.b.Rename(off, ws[2])
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf("#%d %d declarations %d uses", d.ID, len(rn.Decls), len(rn.Uses)), sv.b.Text, nil
	case "at":
		if len(ws) < 2 || len(ws) > 3 {
			return "", "", fmt.Errorf("at takes POS [VIEW]")
		}
		if err := buf(); err != nil {
			return "", "", err
		}
		off, err := textOffset(sv.b.Text, ws[1])
		if err != nil {
			return "", "", err
		}
		e, _ := sv.b.At(off)
		if e == nil {
			return "", "", fmt.Errorf("at %s: no node there", ws[1])
		}
		name := sv.name
		if len(ws) == 3 {
			name = ws[2]
		}
		if name == "follow" {
			return "", "", fmt.Errorf("at: follow needs its steps; name another view")
		}
		old := [2]string{sv.name, strings.Join(sv.vargs, "\x00")}
		sv.name, sv.vargs = name, []string{fmt.Sprintf("#%d", e.ID)}
		if err := sv.b.Reopen(sv.render()); err != nil {
			sv.name, sv.vargs = old[0], strings.Split(old[1], "\x00")
			return "", "", err
		}
		return fmt.Sprintf("#%d %s", e.ID, sv.b.Ix.Name(e)), sv.b.Text, nil
	case "undo":
		if err := buf(); err != nil {
			return "", "", err
		}
		ok, err := sv.b.Undo()
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", fmt.Errorf("nothing to undo")
		}
		return "", sv.b.Text, nil
	case "revert":
		if err := buf(); err != nil {
			return "", "", err
		}
		sv.b.Revert()
		return "", sv.b.Text, nil
	case "c":
		if err := need(1); err != nil {
			return "", "", err
		}
		ix := view.NewIndex(sv.g)
		if sv.b != nil {
			ix = sv.b.Ix
		}
		text, _, err := viewText(ix, "def", ws[1:], view.Options{}, view.Printer{}, true)
		return "", text, err
	case "write":
		if err := need(1); err != nil {
			return "", "", err
		}
		c, err := sv.g.C()
		if err != nil {
			return "", "", err
		}
		if err := os.WriteFile(ws[1], c, 0o644); err != nil {
			return "", "", err
		}
		return strconv.Itoa(len(c)), "", nil
	case "edits":
		return strconv.Itoa(sv.s.Edits()), "", nil
	case "stats":
		n := 0
		sv.g.Walk(func(*graph.Node) bool { n++; return true })
		return "", fmt.Sprintf("file %s\nnodes %d\nedits %d\n", sv.file, n, sv.s.Edits()), nil
	}
	return "", "", fmt.Errorf("no request %s (open, text, change, rename, at, undo, revert, c, write, edits, stats, ping, quit)", ws[0])
}

// open is the buffer on a view: its flags as whim view's, then its name
// and argument.
func (sv *viewServer) open(ws []string) error {
	var opt view.Options
	var p view.Printer
	var rest []string
	fallout := false
	for i := 0; i < len(ws); i++ {
		a := ws[i]
		arg := func() (string, error) {
			if i+1 >= len(ws) {
				return "", fmt.Errorf("%s takes a value", a)
			}
			i++
			return ws[i], nil
		}
		switch a {
		case "--ids":
			p.IDs = true
		case "--fallout":
			fallout = true
		case "--depth", "--show", "--stop":
			v, err := arg()
			if err != nil {
				return err
			}
			switch a {
			case "--depth":
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("--depth %s", v)
				}
				opt.Depth = n
				if n == 0 {
					opt.Depth = -1
				}
			case "--show":
				sh, err := view.ParseShow(v)
				if err != nil {
					return err
				}
				opt.Show = sh
			default:
				opt.Stop = strings.Split(v, ",")
			}
		default:
			rest = append(rest, a)
		}
	}
	need := 2
	if len(rest) > 0 && rest[0] == "follow" {
		need = 3
	}
	if len(rest) != need {
		return fmt.Errorf("open [FLAGS] VIEW ARG (follow STEPS ROOT)")
	}
	sv.name, sv.vargs, sv.opt, sv.p = rest[0], rest[1:], opt, p
	eo := view.EditOptions{IDs: p.IDs}
	if fallout {
		// a deletion's dangling uses closed over, wherever they are: asked
		// for, since a function deleted takes its calls in other functions
		fo := whim.GraphFallOut
		eo.FallOut = &fo
	}
	b, err := sv.s.Open(sv.render(), eo)
	if err != nil {
		return err
	}
	sv.b = b
	// the root pinned by its id, so that an edit renaming it leaves the
	// view (the view by id prints as by name, held below)
	switch sv.name {
	case "def", "callers", "callees", "uses", "member":
		if rs, err := b.Ix.Find(sv.vargs[len(sv.vargs)-1]); err == nil && len(rs) == 1 && rs[0].ID != 0 {
			byName := append([]string(nil), sv.vargs...)
			sv.vargs[len(sv.vargs)-1] = fmt.Sprintf("#%d", rs[0].ID)
			text := b.Text
			if err := b.Reopen(sv.render()); err != nil || b.Text != text {
				sv.vargs = byName
				return b.Reopen(sv.render())
			}
		}
	}
	return nil
}

// render is the buffer's view as a Render.  A root pinned by its id
// (open) whose node an edit replaced -- a function's parameters changed
// make it another definition -- is found again by the name it had when the
// view was last printed.
func (sv *viewServer) render() view.Render {
	name, args, opt, p := sv.name, sv.vargs, sv.opt, sv.p
	pinned := len(args) > 0 && strings.HasPrefix(args[len(args)-1], "#")
	last := ""
	return func(ix *view.Index) (string, []view.Span, error) {
		text, spans, err := viewText(ix, name, args, opt, p, false)
		if err != nil && pinned && last != "" {
			byName := append(append([]string(nil), args[:len(args)-1]...), last)
			text, spans, err = viewText(ix, name, byName, opt, p, false)
		}
		if err == nil && pinned {
			if rs, ferr := ix.Find(args[len(args)-1]); ferr == nil && len(rs) == 1 {
				last = ix.Name(rs[0])
			}
		}
		return text, spans, err
	}
}

// textOffsetEnd is textOffset, the text's end allowed: where a change may
// end, or insert at.
func textOffsetEnd(text, pos string) (int, error) {
	if n, err := strconv.Atoi(pos); err == nil && n == len(text) {
		return n, nil
	}
	if l, c, ok := strings.Cut(pos, ":"); ok {
		line, _ := strconv.Atoi(l)
		col, _ := strconv.Atoi(c)
		if lines := strings.Split(text, "\n"); line == len(lines) && col == len(lines[line-1])+1 {
			return len(text), nil
		}
		if line >= 1 && line <= strings.Count(text, "\n") {
			// a line's end, its newline: where typing at the end of a line goes
			off := 0
			for i := 1; i < line; i++ {
				off += strings.IndexByte(text[off:], '\n') + 1
			}
			if end := strings.IndexByte(text[off:], '\n'); col == end+1 {
				return off + end, nil
			}
		}
	}
	return textOffset(text, pos)
}

// splitRequest is a request's words: blank-separated, '...' a word as it
// is, "..." a Go string.
func splitRequest(line string) ([]string, error) {
	var ws []string
	for i := 0; i < len(line); {
		switch c := line[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '\'':
			j := strings.IndexByte(line[i+1:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("an unclosed '")
			}
			ws = append(ws, line[i+1:i+1+j])
			i += j + 2
		case c == '"':
			j := i + 1
			for ; j < len(line) && line[j] != '"'; j++ {
				if line[j] == '\\' {
					j++
				}
			}
			if j >= len(line) {
				return nil, fmt.Errorf("an unclosed \"")
			}
			s, err := strconv.Unquote(line[i : j+1])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", line[i:j+1], err)
			}
			ws = append(ws, s)
			i = j + 1
		default:
			j := i
			for j < len(line) && line[j] != ' ' && line[j] != '\t' {
				j++
			}
			ws = append(ws, line[i:j])
			i = j
		}
	}
	return ws, nil
}
