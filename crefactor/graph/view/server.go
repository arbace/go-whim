package view

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// THE EDITING SERVER (doc/GRAPH.md, *The editing server*): a session on a
// graph and its buffers, answering a request a line -- what `whim
// view-serve` runs on stdin, and the Joker guest's namespace ed in the box
// (doc/LISP-SANDBOX.md, *The editor in the box*).  The requests are
// view-serve's, documented there.

// ServerOptions are what a server is told about its program and its
// world: the fall-out closure's options a buffer opened with --fallout
// uses (nil: --fallout refused), and where `write NAME` puts the C (nil:
// a file of that name).
type ServerOptions struct {
	FallOut *graph.FallOutOptions
	Write   func(name string, c []byte) error
}

// ErrQuit is the answer to quit.
var ErrQuit = errors.New("quit")

// ErrUsage is a view named that is not one.
var ErrUsage = errors.New("no such view")

// A Server is one session and its buffers; the request's buffer is the
// one embedded, so that its fields are the server's.
type Server struct {
	g    *graph.Graph
	file string
	s    *Session
	o    ServerOptions
	*sbuf
	bufs map[int]*sbuf
	last int // the buffer the last request named
	next int // the next buffer's number
}

// an sbuf is a buffer and the view it is on.
type sbuf struct {
	id    int
	b     *Buffer
	name  string   // the buffer's view
	vargs []string // its arguments, the root last (pinned: #ID)
	asked string   // the view as asked for, or as `at` reached it: for `buffers`
	opt   Options
	p     Printer
}

// NewServer is a session on g, read from file, and no buffer.
func NewServer(g *graph.Graph, file string, o ServerOptions) *Server {
	return &Server{g: g, file: file, s: NewSession(g), o: o, bufs: map[int]*sbuf{}, next: 1}
}

// Serve answers requests from in on out until quit or the input's end,
// each framed as Frame writes it.
func (sv *Server) Serve(in io.Reader, out io.Writer) {
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	defer w.Flush()
	for {
		line, err := r.ReadString('\n')
		if line = strings.TrimRight(line, "\r\n"); line != "" {
			head, body, rerr := sv.Answer(line)
			if errors.Is(rerr, ErrQuit) {
				Frame(w, "ok", "", "")
				return
			}
			if rerr != nil {
				Frame(w, "error", "", rerr.Error())
			} else {
				Frame(w, "ok", head, body)
			}
			w.Flush()
		}
		if err != nil {
			return
		}
	}
}

// Frame writes `KIND N[ HEAD]`, the body, and `;;end` on a line of
// its own.
func Frame(w io.Writer, kind, head, body string) {
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

// Answer is one request's answer: the header's words after ok N, and the
// body; or an error, ErrQuit for quit.
func (sv *Server) Answer(line string) (head, body string, err error) {
	ws, err := SplitRequest(line)
	if err != nil {
		return "", "", err
	}
	named := 0 // @B: the buffer the request is for
	if len(ws) > 0 && strings.HasPrefix(ws[0], "@") {
		if named, err = strconv.Atoi(ws[0][1:]); err != nil || sv.bufs[named] == nil {
			return "", "", fmt.Errorf("no buffer %s", ws[0])
		}
		ws = ws[1:]
		sv.last = named
	}
	if len(ws) == 0 {
		return "", "", fmt.Errorf("an empty request")
	}
	sv.sbuf = sv.bufs[sv.last]
	// the other buffers' texts before, for the ones an edit moves
	before := map[int]string{}
	for id, x := range sv.bufs {
		if x != sv.sbuf {
			before[id] = x.b.Text
		}
	}
	touched := func(head string) string {
		var ids []int
		for id, was := range before {
			if x := sv.bufs[id]; x != nil && x.b.Text != was {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return head
		}
		sort.Ints(ids)
		head += " touched"
		for _, id := range ids {
			head += " " + strconv.Itoa(id)
		}
		return strings.TrimSpace(head)
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
		if sv.sbuf == nil || sv.b == nil {
			return fmt.Errorf("no buffer: open a view first")
		}
		return nil
	}
	switch ws[0] {
	case "quit":
		return "", "", ErrQuit
	case "ping":
		return "", "pong\n", nil
	case "open":
		into := sv.sbuf
		if named == 0 {
			into = &sbuf{id: sv.next}
		}
		if err := sv.open(into, ws[1:]); err != nil {
			return "", "", err
		}
		if named == 0 {
			sv.bufs[into.id] = into
			sv.next++
		}
		sv.last, sv.sbuf = into.id, into
		return fmt.Sprintf("buf %d", into.id), sv.b.Text, nil
	case "buffers":
		ids := make([]int, 0, len(sv.bufs))
		for id := range sv.bufs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		var out strings.Builder
		for _, id := range ids {
			x := sv.bufs[id]
			fmt.Fprintf(&out, "%d %s\n", id, x.asked)
		}
		return "", out.String(), nil
	case "close":
		if err := buf(); err != nil {
			return "", "", err
		}
		sv.b.Close()
		delete(sv.bufs, sv.id)
		sv.sbuf = nil
		return "", "", nil
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
		from, err := TextOffsetEnd(sv.b.Text, ws[1])
		if err != nil {
			return "", "", err
		}
		to, err := TextOffsetEnd(sv.b.Text, ws[2])
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
			cursor = MapPos(typed, sv.b.Text, c)
		}
		head = fmt.Sprintf("%s %d", r.Status, cursor)
		if r.Result != nil && len(r.Result.Added) > 0 {
			head += " added"
			for _, f := range r.Result.Added {
				head += " " + graph.DeclName(f)
			}
		}
		head = touched(head)
		if r.Reason != "" {
			head += " " + r.Reason // last: it has blanks
		}
		return head, sv.b.Text, nil
	case "rename":
		if err := need(2); err != nil {
			return "", "", err
		}
		if err := buf(); err != nil {
			return "", "", err
		}
		off, err := TextOffset(sv.b.Text, ws[1])
		if err != nil {
			return "", "", err
		}
		rn, d, err := sv.b.Rename(off, ws[2])
		if err != nil {
			return "", "", err
		}
		return touched(fmt.Sprintf("#%d %d declarations %d uses", d.ID, len(rn.Decls), len(rn.Uses))), sv.b.Text, nil
	case "at":
		if len(ws) < 2 || len(ws) > 3 {
			return "", "", fmt.Errorf("at takes POS [VIEW]")
		}
		if err := buf(); err != nil {
			return "", "", err
		}
		off, err := TextOffset(sv.b.Text, ws[1])
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
		sv.asked = name + " " + sv.b.Ix.Name(e)
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
		return touched(""), sv.b.Text, nil
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
		ix := NewIndex(sv.g)
		if sv.b != nil {
			ix = sv.b.Ix
		}
		text, _, err := Text(ix, "def", ws[1:], Options{}, Printer{}, true)
		return "", text, err
	case "write":
		if err := need(1); err != nil {
			return "", "", err
		}
		c, err := sv.g.C()
		if err != nil {
			return "", "", err
		}
		write := sv.o.Write
		if write == nil {
			write = func(name string, c []byte) error { return os.WriteFile(name, c, 0o644) }
		}
		if err := write(ws[1], c); err != nil {
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
	return "", "", fmt.Errorf("no request %s (open, buffers, close, text, change, rename, at, undo, revert, c, write, edits, stats, ping, quit)", ws[0])
}

// open puts buffer into on a view: its flags as whim view's, then its name
// and argument; the buffer it was on is closed.
func (sv *Server) open(into *sbuf, ws []string) error {
	var opt Options
	var p Printer
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
				sh, err := ParseShow(v)
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
	old := into.b
	sv.sbuf = into
	sv.name, sv.vargs, sv.opt, sv.p = rest[0], rest[1:], opt, p
	sv.asked = strings.Join(rest, " ")
	eo := EditOptions{IDs: p.IDs}
	if fallout {
		// a deletion's dangling uses closed over, wherever they are: asked
		// for, since a function deleted takes its calls in other functions
		if sv.o.FallOut == nil {
			return fmt.Errorf("--fallout: the server has no fall-out options")
		}
		fo := *sv.o.FallOut
		eo.FallOut = &fo
	}
	b, err := sv.s.Open(sv.render(), eo)
	if err != nil {
		return err
	}
	if old != nil {
		old.Close()
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
func (sv *sbuf) render() Render {
	name, args, opt, p := sv.name, sv.vargs, sv.opt, sv.p
	pinned := len(args) > 0 && strings.HasPrefix(args[len(args)-1], "#")
	last := ""
	return func(ix *Index) (string, []Span, error) {
		text, spans, err := Text(ix, name, args, opt, p, false)
		if err != nil && pinned && last != "" {
			byName := append(append([]string(nil), args[:len(args)-1]...), last)
			text, spans, err = Text(ix, name, byName, opt, p, false)
		}
		if err == nil && pinned {
			if rs, ferr := ix.Find(args[len(args)-1]); ferr == nil && len(rs) == 1 {
				last = ix.Name(rs[0])
			}
		}
		return text, spans, err
	}
}

// TextOffsetEnd is TextOffset, the text's end allowed: where a change may
// end, or insert at.
func TextOffsetEnd(text, pos string) (int, error) {
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
	return TextOffset(text, pos)
}

// SplitRequest is a request's words: blank-separated, '...' a word as it
// is, "..." a Go string.
func SplitRequest(line string) ([]string, error) {
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

// Text is the view name of args ([ARG] or [STEPS ROOT]) on ix, printed
// by p, with its span table: one text for each root the argument names.
func Text(ix *Index, name string, args []string, opt Options, p Printer, cOut bool) (string, []Span, error) {
	roots, err := ix.Find(args[len(args)-1])
	if err == nil && name == "member" && !isMemberNode(ix, roots[0]) {
		err = fmt.Errorf("%s is not a member: member is S.M", args[len(args)-1])
	}
	if err != nil {
		return "", nil, err
	}
	var out strings.Builder
	var spans []Span
	add := func(s string, sp []Span) {
		base, first := out.Len(), len(spans)
		for _, x := range sp {
			x.Start += base
			x.End += base
			if x.Parent >= 0 {
				x.Parent += first
			}
			spans = append(spans, x)
		}
		out.WriteString(s)
	}
	for _, root := range roots {
		var t *Tree
		switch name {
		case "callers":
			t = Callers(ix, root, opt)
		case "callees":
			t = Callees(ix, root, opt)
		case "uses":
			t = Uses(ix, root, opt)
		case "member":
			t = Member(ix, root, opt)
		case "type":
			s, err := ix.Aggregate(args[0])
			if err != nil {
				return "", nil, err
			}
			t = Type(ix, s, opt)
		case "def":
			d := Def(ix, root)
			if cOut {
				c, err := clisp.Print([]*clisp.Node{graph.Lisp(d)})
				if err != nil {
					return "", nil, err
				}
				out.Write(c)
			} else {
				add(p.RenderForm(d))
			}
			continue
		case "follow":
			steps, err := ParseSteps(args[0])
			if err != nil {
				return "", nil, err
			}
			t = Build(ix, root, Spec{Name: "follow", Child: "to", Rel: Steps(steps),
				Depth: followDepth(opt.Depth), Stop: opt.Stop, Show: opt.Show})
			t.Notes = append(t.Notes, "steps: "+args[0])
		default:
			return "", nil, ErrUsage
		}
		add(p.Render(ix, t))
	}
	return out.String(), spans, nil
}

// TextOffset is a position in text: a byte offset, or LINE:COL from 1:1
// (COL a byte in the line).
func TextOffset(text, pos string) (int, error) {
	l, c, ok := strings.Cut(pos, ":")
	if !ok {
		n, err := strconv.Atoi(pos)
		if err != nil || n < 0 || n >= len(text) {
			return 0, fmt.Errorf("--at %s: not a byte of the view's %d", pos, len(text))
		}
		return n, nil
	}
	line, err1 := strconv.Atoi(l)
	col, err2 := strconv.Atoi(c)
	if err1 != nil || err2 != nil || line < 1 || col < 1 {
		return 0, fmt.Errorf("--at %s: not LINE:COL", pos)
	}
	off := 0
	for i := 1; i < line; i++ {
		k := strings.IndexByte(text[off:], '\n')
		if k < 0 {
			return 0, fmt.Errorf("--at %s: the view has %d lines", pos, i)
		}
		off += k + 1
	}
	end := strings.IndexByte(text[off:], '\n')
	if end < 0 {
		end = len(text) - off
	}
	if col > end {
		return 0, fmt.Errorf("--at %s: line %d has %d bytes", pos, line, end)
	}
	return off + col - 1, nil
}

func followDepth(d int) int {
	switch {
	case d == 0:
		return 1
	case d < 0:
		return 0
	}
	return d
}
