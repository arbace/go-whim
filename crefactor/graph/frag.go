package graph

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
)

// FRAG (doc/GRAPH-MIGRATION.md, B2a): a fragment of C TEXT -- statements, a
// function's body, an expression, external declarations: what the phases'
// `editlit.go` literals and the text verbs' replacement strings are -- made
// graph nodes IN CONTEXT, at a place in the graph, exactly as the importer
// would make them had the text been spliced there and the file imported.
//
// HOW.  The importer is the only thing that knows how cc resolves and types
// C, so FRAG uses it, on a SYNTHESIZED TRANSLATION UNIT: the graph's C view
// with the fragments' texts written at their places, and pared to what a
// fragment can see -- every function's body left out but those a fragment
// goes into, a definition its prototype declared first left out where no
// use the unit resolves comes after it, and a table's rows left out (its
// length written where its initialiser said it, from the typed edge) --
// everything else in its order, so that every name is declared where it
// was.  A fragment of items is written between two marker statements
// (`__whim_frag_mark;`, declared at the top), external declarations between
// two marker declarations, an expression or a body in its own place.  The
// unit is parsed and type checked by crefactor/cc and imported (Import's own
// code); then
//
//   - the synthesized graph and this one are walked side by side, and every
//     node of the context maps to the node it is a printing of -- the
//     markers say where each fragment's nodes are, and nothing else differs;
//   - the fragment's nodes are kept, their ids cleared (the edit that puts
//     them in gives them fresh ones, the id rule), and their edges carried
//     over: a refers edge into the context to the node it maps to (a local,
//     a parameter, the declaration cc resolves the use to, a member by
//     type, a label, a tag); one to the headers' declarations to this
//     graph's external node of it, made if it has none (`strlen`, `errno`,
//     `__builtin_setjmp`, a member of a header's struct); a typed edge to
//     this graph's type node of the same structure, made if it has none;
//   - a macro's invocation in the fragment is a `(macro "...")` node (or an
//     identifier-like atom) with a refers edge to every name its expansion
//     uses, as the importer makes it (MACROX, macrox.go);
//   - a use OUTSIDE the fragments that the synthesized import resolves to
//     one of their declarations -- a later use a new local now shadows, a
//     goto to a label a fragment brings, a call of a function a top-level
//     fragment declares or defines anew -- is retargeted when the fragments
//     are spliced (for a top-level fragment that declares a name the file
//     declares, the unit is made again with every form using the name
//     printed whole, so that each use is resolved), and the expressions
//     above an expression's place, and above a retargeted use, are typed as
//     the import types them: the graph after the splice is what an import of
//     its C view would give (SameGraph, same.go, is how the tests hold it).
//
// What it refuses, at the spot: a fragment that does not parse, or that cc's
// check complains of on the fragment's own lines (the complaint, with the
// fragment's line); a name declared nowhere visible; a fragment that does
// not stand as one node in an expression's place, or that would change the
// context's own structure (`x * $a` with the fragment `a, b`); spots that
// overlap, or one inside what another replaces.  `#` lines are INCLUDE's
// (B2e).
//
// HOLES are written `$name` in the text (outside string and character
// literals) and stand for existing expression nodes, Bindings as Build's:
// the node is moved in (it must be in what the fragment replaces, or out of
// the graph -- a copy, Clone); used twice, the second use is a Clone.  The
// hole is written into the unit as its own C in parentheses, so that cc
// types the fragment with the hole's type; the node replaces what those
// parentheses made, and the C view writes the parentheses an operator needs
// around it (`$x * 2` with x `a + b` is `(a + b) * 2`, where a text splice
// would have written `a + b * 2`).
//
// THE COST is one parse, check and import of the synthesized unit, a fifth
// of the file or less: measured in doc/GRAPH-MIGRATION.md, *B2a as built*.
// Many fragments are made in one call (SpliceC(fs...)), one unit for them
// all; the verbs' Together defers a phase's FRAG acts to one.

// A Spot is a place a fragment of C takes: a run of items replaced (an
// insertion when the run is empty), or one node's place.
type Spot struct {
	p      *Node // the container; the forms' sentinel for the file
	lo, hi int
	one    bool // exactly one node's place: an expression, an initialiser element, a body
	err    error
}

// A Frag is a fragment of C and its spot.
type Frag struct {
	At    Spot
	Src   string   // C: items, an expression, external declarations
	Holes Bindings // the nodes `$name` stands for
}

func spotErr(format string, a ...any) Spot { return Spot{err: fmt.Errorf(format, a...)} }

// Err is why a spot could not be made, or nil: SpotOf, SpotRun and the
// others refuse a place FRAG cannot take by a spot that carries the
// refusal, which a caller may ask before it splices.
func (s Spot) Err() error { return s.err }

// Items says the spot is a run of items (an insertion when empty), not
// one node's place: its fragment is statements or declarations.
func (s Spot) Items() bool { return s.err == nil && !s.one }

// SpotOf is n's own place: an item's (the fragment's items replace it), an
// expression's or an initialiser element's (the fragment is one expression),
// a body's or an else's (the fragment is a block, or for an else an if).
func (e *Editor) SpotOf(n *Node) Spot {
	p, i := e.index(n)
	if i < 0 {
		return spotErr("#%d (%s) is not in the graph", n.ID, label(n))
	}
	switch e.place(p, i) {
	case placeItem:
		return Spot{p: p, lo: i, hi: i + 1}
	case placeFixed, placeBody, placeOptional:
		return Spot{p: p, lo: i, hi: i + 1, one: true}
	}
	return spotErr("#%d (%s): FRAG takes items, expressions and bodies; a member, an enumerator or a section's node is not one", n.ID, label(n))
}

// SpotRun is the run of items first through last, which one list holds in
// that order.
func (e *Editor) SpotRun(first, last *Node) Spot {
	p, lo := e.index(first)
	q, hi := e.index(last)
	switch {
	case lo < 0 || hi < 0:
		return spotErr("the run #%d..#%d: not in the graph", first.ID, last.ID)
	case p != q:
		return spotErr("the run #%d..#%d: not in one list", first.ID, last.ID)
	case hi < lo:
		return spotErr("the run #%d..#%d: its end comes before its start", first.ID, last.ID)
	}
	for j := lo; j <= hi; j++ {
		if e.place(p, j) != placeItem {
			return spotErr("the run #%d..#%d: #%d (%s) is not an item", first.ID, last.ID, e.kids(p)[j].ID, label(e.kids(p)[j]))
		}
	}
	return Spot{p: p, lo: lo, hi: hi + 1}
}

// SpotBefore is the place before the item at; SpotAfter the place after it.
func (e *Editor) SpotBefore(at *Node) Spot { return e.spotBeside(at, 0) }

// SpotAfter is the place after the item at.
func (e *Editor) SpotAfter(at *Node) Spot { return e.spotBeside(at, 1) }

func (e *Editor) spotBeside(at *Node, k int) Spot {
	p, i := e.index(at)
	if i < 0 {
		return spotErr("#%d (%s) is not in the graph", at.ID, label(at))
	}
	if e.insertPlace(p, i+k) != placeItem {
		return spotErr("#%d (%s) has no place for an item there", p.ID, label(p))
	}
	return Spot{p: p, lo: i + k, hi: i + k}
}

// SpotBody is a function definition's whole body: its items.
func (e *Editor) SpotBody(fn *Node) Spot {
	if !fn.Is("defn") || !e.Live(fn) {
		return spotErr("#%d (%s) is not a function's definition in the graph", fn.ID, label(fn))
	}
	return Spot{p: fn, lo: defnItemsAt(fn), hi: len(fn.Kids)}
}

// SpotEnd is the place after the last item of p -- a block, a statement
// expression, a function's definition -- or of the file, for p nil.
func (e *Editor) SpotEnd(p *Node) Spot {
	if p == nil {
		return Spot{p: e.top[0], lo: len(e.g.Forms), hi: len(e.g.Forms)}
	}
	if !e.Live(p) || e.insertPlace(p, len(p.Kids)) != placeItem {
		return spotErr("#%d (%s) has no place for an item at its end", p.ID, label(p))
	}
	return Spot{p: p, lo: len(p.Kids), hi: len(p.Kids)}
}

// SpliceC makes each fragment's nodes at its spot and puts them there --
// the spot's items or node replaced -- then retargets the uses the
// fragments now declare (above).  It returns each fragment's nodes.  A
// refusal while the nodes are made (a parse, cc's check, a name, a place
// the text did not stand in) changes nothing in the forms; one the editor
// makes at a splice (a body that is not one block) leaves the splices
// before it made, as a run of edits does.  The type and external nodes the
// fragments needed are added either way, and collected when nothing uses
// them.
func (e *Editor) SpliceC(fs ...Frag) ([][]*Node, error) {
	s, err := e.frag(fs)
	if err != nil {
		return nil, err
	}
	// the splices: in each list, the last spot first, so that the others'
	// indexes hold (lists are independent: no spot is in what another
	// replaces); an edge from one fragment into another is held whichever
	// is spliced first
	e.pending = map[*Node]bool{}
	for _, j := range s.jobs {
		for _, n := range j.nodes {
			Walk(n, func(x *Node) bool { e.pending[x] = true; return true })
		}
	}
	defer func() { e.pending = nil }()
	for _, j0 := range s.jobs {
		js := s.spots[j0.f.At.p]
		if js[0] != j0 {
			continue
		}
		for k := len(js) - 1; k >= 0; k-- {
			j := js[k]
			at := j.f.At
			if at.one && len(j.nodes) != 1 {
				return nil, fmt.Errorf("frag %d: %d nodes where one node's place is", j.k, len(j.nodes))
			}
			op := "replace"
			if at.lo == at.hi {
				op = "insert"
			}
			if err := e.splice(op, at.p, at.lo, at.hi, j.nodes); err != nil {
				return nil, fmt.Errorf("frag %d: %w", j.k, err)
			}
		}
	}
	if err := s.retarget(); err != nil {
		return nil, err
	}
	// an expression's place: the expressions above it typed as the
	// synthesized import types them (the splice cleared what it could not
	// say)
	for _, j := range s.jobs {
		if j.kind != kindExpr {
			continue
		}
		for q := j.f.At.p; q != nil && q.list && !IsStatement(q) && q.up != e.top[0]; q = e.Parent(q) {
			if err := s.retype(q); err != nil {
				return nil, err
			}
		}
	}
	out := make([][]*Node, len(s.jobs))
	for i, j := range s.jobs {
		out[i] = j.nodes
	}
	return out, nil
}

// MakeC is the fragments' nodes, made at their spots and not put there:
// for a caller that places them itself (Replace, InsertBefore, a node it
// builds around them).  The uses outside the fragments are not retargeted:
// SpliceC does that.
func (e *Editor) MakeC(fs ...Frag) ([][]*Node, error) {
	s, err := e.frag(fs)
	if err != nil {
		return nil, err
	}
	out := make([][]*Node, len(s.jobs))
	for i, j := range s.jobs {
		out[i] = j.nodes
	}
	return out, nil
}

// The kinds of placeholder a spot is printed with.
const (
	kindItems  = iota // markers around the fragment's items
	kindExpr          // one expression in its place
	kindBraced        // a body or an else: the printer braces the placeholder
	kindTop           // external declarations, markers around them
)

const (
	fragMark = "__whim_frag_mark"
	fragPath = "/whim-fragment.c"
)

// a job is one fragment as it is made.
type fragJob struct {
	f                  Frag
	k                  int // its number, for messages
	kind               int
	text               string // the fragment's text, its holes written in
	holes              []fragHole
	start              int // where its text starts in the synthesized unit
	startLine, endLine int
	nodes              []*Node
}

type fragHole struct {
	name     string
	rel, off int // of the `(` written for it: in the job's text, in the unit
}

// synth is one synthesized translation unit and what it maps.
type synth struct {
	e     *Editor
	jobs  []*fragJob
	spots map[*Node][]*fragJob  // a container -> the jobs in it, by lo
	keep  map[*Node]bool        // the top-level forms printed whole
	omit  map[*Node]bool        // the top-level forms left out
	trim  map[*Node]*clisp.Node // the tables printed without their rows

	im     *importer
	orig   map[*Node]*Node // a synthesized node -> the graph's node it prints
	pairs  [][2]*Node      // (graph, synthesized) in the forms printed whole
	rev    map[*Node]*Node // the pairs, the graph's node first
	inF    map[*Node]bool  // the fragments' nodes
	mis    []string        // where the walk found the two differ
	err    error           // the first fragment found out of its place
	types  map[*Node]*Node // a synthesized type node -> the graph's
	okeys  map[string]*Node
	omemo  map[*Node]string
	rmemo  map[*Node]string
	oext   map[string]*Node
	rowner map[*Node]*Node // a synthesized extern struct's member -> the struct
}

// frag makes the fragments' nodes: the synthesized unit, its import, the
// two walked side by side, the fragments' edges carried over.
func (e *Editor) frag(fs []Frag) (*synth, error) {
	s := &synth{e: e, spots: map[*Node][]*fragJob{}, keep: map[*Node]bool{}}
	for i, f := range fs {
		if f.At.err != nil {
			return nil, fmt.Errorf("frag %d: %w", i+1, f.At.err)
		}
		if f.At.p == nil || !e.Live(f.At.p) && f.At.p != e.top[0] {
			return nil, fmt.Errorf("frag %d: its spot is not in the graph", i+1)
		}
		j := &fragJob{f: f, k: i + 1}
		switch {
		case f.At.p == e.top[0]:
			j.kind = kindTop
		case f.At.one && e.place(f.At.p, f.At.lo) != placeFixed:
			j.kind = kindBraced
		case f.At.one:
			j.kind = kindExpr
		default:
			j.kind = kindItems
		}
		if err := j.prepare(); err != nil {
			return nil, err
		}
		s.jobs = append(s.jobs, j)
		s.spots[f.At.p] = append(s.spots[f.At.p], j)
		if f.At.p != e.top[0] {
			s.keep[e.topOf(f.At.p)] = true
		}
	}
	for p, js := range s.spots {
		sort.SliceStable(js, func(a, b int) bool { return js[a].f.At.lo < js[b].f.At.lo })
		for i := 1; i < len(js); i++ {
			if js[i].f.At.lo < js[i-1].f.At.hi || js[i].f.At.lo == js[i-1].f.At.lo {
				return nil, fmt.Errorf("frag %d and frag %d: their spots in #%d (%s) overlap", js[i-1].k, js[i].k, p.ID, label(p))
			}
		}
	}
	// no spot inside what another replaces
	for _, j := range s.jobs {
		for x := j.f.At.p; x != nil && x != e.top[0]; x = x.up {
			p := x.up
			for _, o := range s.spots[p] {
				if i := indexIn(e.kids(p), x); i >= o.f.At.lo && i < o.f.At.hi {
					return nil, fmt.Errorf("frag %d is inside what frag %d replaces", j.k, o.k)
				}
			}
		}
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	// a top-level fragment that declares a name the file declares: every
	// form holding a use of it printed whole, so that the import says what
	// each use resolves to now
	if extra := s.colliding(); len(extra) > 0 {
		for _, f := range extra {
			s.keep[f] = true
		}
		for _, j := range s.jobs {
			j.nodes = nil
		}
		s.orig, s.pairs, s.mis, s.err = nil, nil, nil, nil
		if err := s.load(); err != nil {
			return nil, err
		}
	}
	if err := s.carry(); err != nil {
		return nil, err
	}
	return s, nil
}

// load synthesizes the unit, parses, checks and imports it, and walks it
// beside the graph.
func (s *synth) load() error {
	text, err := s.synthesize()
	if err != nil {
		return err
	}
	ast, bsrc, err := cemit.Parse(fragPath, text)
	if err != nil {
		return s.where(err)
	}
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return err
	}
	rep := &Report{Unresolved: map[string]int{}}
	rep.Check = ast.Check(cfg)
	if err := s.checked(rep.Check); err != nil {
		return err
	}
	if s.im, err = importAST(ast, fragPath, bsrc, rep); err != nil {
		return s.where(err)
	}
	return s.walk()
}

// colliding are the top-level forms not printed whole that hold a use of a
// file-scope declaration whose name a top-level fragment declares.
func (s *synth) colliding() []*Node {
	names := map[string]bool{}
	for _, j := range s.jobs {
		if j.kind != kindTop {
			continue
		}
		for _, n := range j.nodes {
			if name := topName(n); name != "" {
				names[name] = true
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	seen := map[*Node]bool{}
	var out []*Node
	for _, f := range s.e.g.Forms {
		if !names[topName(f)] {
			continue
		}
		for _, u := range s.e.Uses(f) {
			if t := s.e.topOf(u); !s.keep[t] && !seen[t] && s.e.Live(t) {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

func indexIn(ks []*Node, x *Node) int {
	for i, k := range ks {
		if k == x {
			return i
		}
	}
	return -1
}

// topOf is the top-level form n is in.
func (e *Editor) topOf(n *Node) *Node {
	for n.up != nil && n.up != e.top[0] {
		n = n.up
	}
	return n
}

// prepare writes the job's holes into its text: `$name` as the hole's own
// C, in parentheses, outside literals.
func (j *fragJob) prepare() error {
	src := j.f.Src
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			return fmt.Errorf("frag %d: a `#` line is INCLUDE's (B2e): %s", j.k, strings.TrimSpace(line))
		}
	}
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '"' || c == '\'':
			k := skipLiteral(src, i)
			b.WriteString(src[i:k])
			i = k - 1
		case c == '$' && i+1 < len(src) && identStart(src[i+1]):
			k := i + 1
			for k < len(src) && identChar(src[k]) {
				k++
			}
			name := src[i+1 : k]
			h, ok := j.f.Holes[name]
			if !ok || h == nil {
				return fmt.Errorf("frag %d: the hole $%s is bound to nothing", j.k, name)
			}
			if IsStatement(h) {
				return fmt.Errorf("frag %d: the hole $%s is a %s: a hole is an expression", j.k, name, h.Head())
			}
			c, err := clisp.PrintExpr(Lisp(h))
			if err != nil {
				return fmt.Errorf("frag %d: the hole $%s: %w", j.k, name, err)
			}
			j.holes = append(j.holes, fragHole{name: name, rel: b.Len()})
			b.WriteString("(" + c + ")")
			i = k - 1
		default:
			b.WriteByte(c)
		}
	}
	j.text = b.String()
	return nil
}

// skipLiteral is the index after the string or character literal at i.
func skipLiteral(s string, i int) int {
	q := s[i]
	for k := i + 1; k < len(s); k++ {
		switch s[k] {
		case '\\':
			k++
		case q:
			return k + 1
		case '\n':
			return k
		}
	}
	return len(s)
}

func identStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func identChar(c byte) bool  { return identStart(c) || c >= '0' && c <= '9' }

func placeholder(k int) string { return "__whim_frag_" + strconv.Itoa(k) + "_" }

// synthesize prints the unit: a marker's declaration, then the forms, a
// function's body only where a fragment goes, each spot a placeholder; and
// writes the fragments' texts over the placeholders.
func (s *synth) synthesize() ([]byte, error) {
	forms := []*clisp.Node{clisp.L(clisp.A("def"), clisp.A("extern"), clisp.A(fragMark), clisp.A("int"))}
	top := s.e.top[0]
	i := 0
	declared := map[string]bool{}
	s.omit = map[*Node]bool{}
	s.trim = map[*Node]*clisp.Node{}
	last := 0 // after the last form printed whole, or the last spot in the file
	for k, f := range s.e.g.Forms {
		if s.keep[f] {
			last = k + 1
		}
	}
	for _, j := range s.spots[top] {
		last = max(last, j.f.At.hi)
	}
	emit := func(k int, f *Node) {
		name := topName(f)
		first := name == "" || !declared[name]
		declared[name] = true
		switch {
		case s.keep[f]:
			forms = append(forms, s.lisp(f))
		case f.Is("defn") && !first && k >= last:
			// a definition a prototype declared first, after everything a
			// use is resolved in: the unit is the smaller (cc resolves a use
			// after a definition to it when its prototype names no
			// parameter, so one before the last use is not left out)
			s.omit[f] = true
		case f.Is("def") && s.trimmed(f) != nil:
			forms = append(forms, s.trimmed(f))
		case f.Is("defn"):
			kids := make([]*clisp.Node, defnItemsAt(f))
			for k := range kids {
				kids[k] = Lisp(f.Kids[k])
			}
			forms = append(forms, clisp.L(kids...))
		default:
			forms = append(forms, Lisp(f))
		}
	}
	for _, j := range s.spots[top] {
		for ; i < j.f.At.lo; i++ {
			emit(i, s.e.g.Forms[i])
		}
		mark := clisp.L(clisp.A("def"), clisp.A("extern"), clisp.A(fragMark), clisp.A("int"))
		forms = append(forms, mark, clisp.L(clisp.A("def"), clisp.A("extern"), clisp.A(placeholder(j.k)), clisp.A("int")), mark)
		i = j.f.At.hi
	}
	for ; i < len(s.e.g.Forms); i++ {
		emit(i, s.e.g.Forms[i])
	}
	printed, err := clisp.Print(forms)
	if err != nil {
		return nil, err
	}
	type span struct {
		a, z int
		j    *fragJob
	}
	var spans []span
	for _, j := range s.jobs {
		ph := placeholder(j.k)
		var a, z int
		switch j.kind {
		case kindTop:
			a, z = findOnce(printed, "extern int "+ph+";")
		case kindItems:
			a, z = findOnce(printed, ph+";")
		case kindExpr:
			a, z = findOnce(printed, ph)
		case kindBraced:
			a, z = findOnce(printed, ph+";")
			if a >= 0 {
				a = bytes.LastIndexByte(printed[:a], '{')
				if k := bytes.IndexByte(printed[z:], '}'); k >= 0 && a >= 0 {
					z += k + 1
				} else {
					a = -1
				}
			}
		}
		if a < 0 || bytes.Count(printed, []byte(ph)) != 1 {
			return nil, fmt.Errorf("frag %d: its placeholder is not once in the synthesized text", j.k)
		}
		spans = append(spans, span{a, z, j})
	}
	sort.Slice(spans, func(x, y int) bool { return spans[x].a < spans[y].a })
	var out bytes.Buffer
	at := 0
	for _, sp := range spans {
		out.Write(printed[at:sp.a])
		sp.j.start = out.Len()
		sp.j.startLine = 1 + bytes.Count(out.Bytes(), []byte{'\n'})
		out.WriteString(sp.j.text)
		sp.j.endLine = sp.j.startLine + strings.Count(sp.j.text, "\n")
		for h := range sp.j.holes {
			sp.j.holes[h].off = sp.j.holes[h].rel + sp.j.start
		}
		at = sp.z
	}
	out.Write(printed[at:])
	return out.Bytes(), nil
}

// trimmed is a table's definition printed without its initialiser -- an
// array's length written where the initialiser said it, from the typed
// edge -- or nil for a form printed as it is.  The tables are most of what
// is left when the bodies are: their rows are not what a fragment names.
func (s *synth) trimmed(f *Node) *clisp.Node {
	if c, ok := s.trim[f]; ok {
		return c
	}
	var out *clisp.Node
	defer func() { s.trim[f] = out }()
	at := defNameAt(f)
	if at == 0 || at+1 >= len(f.Kids) || !f.Kids[len(f.Kids)-1].Is("init") || f.Type == nil || !f.Type.Is("array") {
		return nil
	}
	ty := f.Kids[at+1]
	if !ty.Is("array") {
		return nil
	}
	kids := make([]*clisp.Node, 0, len(f.Kids))
	for _, k := range f.Kids[:at+1] {
		kids = append(kids, Lisp(k))
	}
	switch {
	case len(ty.Kids) == 3: // a length written: kept
		kids = append(kids, Lisp(ty))
	case len(ty.Kids) == 2 && len(f.Type.Kids) == 3: // the length the initialiser gave
		kids = append(kids, clisp.L(clisp.A("array"), clisp.A(f.Type.Kids[1].Atom), Lisp(ty.Kids[1])))
	default:
		return nil
	}
	for _, k := range f.Kids[at+2 : len(f.Kids)-1] {
		kids = append(kids, Lisp(k))
	}
	out = clisp.L(kids...)
	return out
}

// pairTrimmed pairs a table printed without its initialiser.
func (s *synth) pairTrimmed(o, r *Node) {
	at := defNameAt(o)
	if len(r.Kids) != len(o.Kids)-1 {
		s.differ(o, "its trimmed import has another number of elements")
		return
	}
	for i := 0; i < len(o.Kids)-1; i++ {
		if i != at+1 {
			s.pair(o.Kids[i], r.Kids[i], false)
			continue
		}
		ot, rt := o.Kids[i], r.Kids[i]
		if len(ot.Kids) == len(rt.Kids) {
			s.pair(ot, rt, false)
			continue
		}
		if len(rt.Kids) != 3 || len(ot.Kids) != 2 || !rt.Is("array") {
			s.differ(o, "its trimmed type differs")
			return
		}
		s.orig[rt] = ot
		s.pair(ot.Kids[1], rt.Kids[2], false)
	}
}

// findOnce is where s is in b, once, and where it ends: -1 if it is not there.
func findOnce(b []byte, s string) (int, int) {
	a := bytes.Index(b, []byte(s))
	if a < 0 {
		return -1, -1
	}
	return a, a + len(s)
}

// lisp is a form printed whole, its spots placeholders.
func (s *synth) lisp(n *Node) *clisp.Node {
	if !n.list {
		return clisp.A(n.Atom)
	}
	js := s.spots[n]
	kids := make([]*clisp.Node, 0, len(n.Kids)+3*len(js))
	i := 0
	for _, j := range js {
		for ; i < j.f.At.lo; i++ {
			kids = append(kids, s.lisp(n.Kids[i]))
		}
		ph := clisp.A(placeholder(j.k))
		if j.kind == kindItems {
			kids = append(kids, clisp.A(fragMark), ph, clisp.A(fragMark))
		} else {
			kids = append(kids, ph)
		}
		i = j.f.At.hi
	}
	for ; i < len(n.Kids); i++ {
		kids = append(kids, s.lisp(n.Kids[i]))
	}
	return clisp.L(kids...)
}

var posRE = regexp.MustCompile(regexp.QuoteMeta(fragPath) + `:(\d+):(\d+)`)

// where says the synthesized unit's positions in an error as the
// fragments' lines.
func (s *synth) where(err error) error {
	msg := posRE.ReplaceAllStringFunc(err.Error(), func(m string) string {
		sub := posRE.FindStringSubmatch(m)
		line, _ := strconv.Atoi(sub[1])
		for _, j := range s.jobs {
			if line >= j.startLine && line <= j.endLine {
				return fmt.Sprintf("frag %d line %d:%s", j.k, line-j.startLine+1, sub[2])
			}
		}
		return "the context, line " + sub[1] + ":" + sub[2]
	})
	return fmt.Errorf("frag: %s", msg)
}

// checked refuses what cc's check says of the fragments' lines; what it
// says of the context is the context's (a text between an edit and its
// sweep may name what the edit removed).
func (s *synth) checked(err error) error {
	if err == nil {
		return nil
	}
	var bad []string
	for _, line := range strings.Split(err.Error(), "\n") {
		m := posRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		for _, j := range s.jobs {
			if n >= j.startLine && n <= j.endLine {
				bad = append(bad, line)
			}
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return s.where(fmt.Errorf("cc's check: %s", strings.Join(bad, "; ")))
}

// isMark says a synthesized node is a marker: the statement, or the
// file-scope declaration.
func isMark(n *Node) bool {
	return !n.list && n.Atom == fragMark || n.Is("def") && topName(n) == fragMark
}

// walk maps the synthesized unit's nodes to the graph's, and finds each
// fragment's nodes between its markers.
func (s *synth) walk() error {
	s.orig = map[*Node]*Node{}
	rf := s.im.g.Forms
	if len(rf) == 0 || !isMark(rf[0]) {
		return fmt.Errorf("frag: the synthesized unit does not begin with its marker's declaration")
	}
	if err := s.kids(s.e.top[0], s.e.g.Forms, rf[1:], false); err != nil {
		return err
	}
	if s.err != nil {
		return s.err
	}
	for _, j := range s.jobs {
		if len(j.nodes) == 0 && (j.f.At.one || strings.TrimSpace(j.f.Src) != "") {
			return fmt.Errorf("frag %d: its text did not stand where it was written (%s): write the parentheses it needs",
				j.k, strings.Join(s.mis, ", "))
		}
	}
	return nil
}

// kids pairs the elements of a graph list o that holds spots (the file's
// forms for the sentinel), ol, with the synthesized list's, rl.
func (s *synth) kids(o *Node, ol, rl []*Node, keep bool) error {
	ri, oi := 0, 0
	stray := func() error {
		return fmt.Errorf("frag: in #%d (%s) the fragments' text did not stand where it was written (%d elements against %d): write the parentheses it needs", o.ID, label(o), len(rl), len(ol))
	}
	pairTo := func(hi int) error {
		for ; oi < hi; oi++ {
			if s.omit[ol[oi]] {
				continue
			}
			if ri >= len(rl) {
				return stray()
			}
			s.pair(ol[oi], rl[ri], keep || o == s.e.top[0] && s.keep[ol[oi]])
			ri++
		}
		return nil
	}
	for _, j := range s.spots[o] {
		if err := pairTo(j.f.At.lo); err != nil {
			return err
		}
		if j.kind == kindItems || j.kind == kindTop {
			if ri >= len(rl) || !isMark(rl[ri]) {
				return fmt.Errorf("frag %d: its first marker is not where it was written: the fragment is not items in its place", j.k)
			}
			ri++
			for ri < len(rl) && !isMark(rl[ri]) {
				j.nodes = append(j.nodes, rl[ri])
				ri++
			}
			if ri >= len(rl) {
				return fmt.Errorf("frag %d: its second marker is not where it was written", j.k)
			}
			ri++
		} else {
			if ri >= len(rl) {
				return fmt.Errorf("frag %d: no node where it was written", j.k)
			}
			j.nodes = []*Node{rl[ri]}
			ri++
		}
		oi = j.f.At.hi
	}
	if err := pairTo(len(ol)); err != nil {
		return err
	}
	if ri != len(rl) {
		return stray()
	}
	return nil
}

// differ records a difference between the graph and its C view's import,
// which only matters if a fragment's edge goes there.
func (s *synth) differ(o *Node, why string) {
	if len(s.mis) < 5 {
		s.mis = append(s.mis, fmt.Sprintf("#%d (%s): %s", o.ID, label(o), why))
	}
}

// pair maps r, a synthesized node, to o, and their elements.
func (s *synth) pair(o, r *Node, keep bool) {
	if o.list != r.list || !o.list && o.Atom != r.Atom || o.list && o.Head() != r.Head() {
		s.differ(o, "the import of its C view differs")
		return
	}
	s.orig[r] = o
	if keep {
		s.pairs = append(s.pairs, [2]*Node{o, r})
	}
	if !o.list {
		return
	}
	if s.trim[o] != nil {
		s.pairTrimmed(o, r)
		return
	}
	if len(s.spots[o]) > 0 {
		if err := s.kids(o, o.Kids, r.Kids, keep); err != nil && s.err == nil {
			s.err = err
		}
		return
	}
	n := len(o.Kids)
	if o.Is("defn") && !keep && o.up == s.e.top[0] {
		n = defnItemsAt(o) // its body was left out
	}
	if len(r.Kids) != n {
		s.differ(o, "the import of its C view has another number of elements")
		return
	}
	for i := 0; i < n; i++ {
		s.pair(o.Kids[i], r.Kids[i], keep)
	}
}

// carry makes the fragments' nodes this graph's: the holes put in, the ids
// cleared, the edges carried over.
func (s *synth) carry() error {
	// the holes
	used := map[*Node]bool{}
	for _, j := range s.jobs {
		for _, h := range j.holes {
			if err := s.hole(j, h, used); err != nil {
				return err
			}
		}
	}
	// the fragments' nodes, holes aside
	holes := map[*Node]bool{}
	for _, j := range s.jobs {
		for _, h := range j.f.Holes {
			holes[h] = true
		}
	}
	s.inF = map[*Node]bool{}
	var all []*Node
	for _, j := range s.jobs {
		for _, n := range j.nodes {
			Walk(n, func(x *Node) bool {
				if holes[x] || used[x] {
					return false
				}
				s.inF[x] = true
				all = append(all, x)
				return true
			})
		}
	}
	s.types = map[*Node]*Node{}
	s.rowner = map[*Node]*Node{}
	for _, x := range s.im.g.Externs {
		for _, m := range x.Kids {
			if m.Is("member") {
				s.rowner[m] = x
			}
		}
	}
	for _, x := range all {
		for i, t := range x.Refs {
			o, err := s.target(t, x)
			if err != nil {
				return err
			}
			s.e.g.save(x)
			x.Refs[i] = o
		}
		if x.Type != nil {
			t, err := s.typ(x.Type)
			if err != nil {
				return err
			}
			s.e.g.save(x)
			x.Type = t
		}
	}
	for _, x := range all {
		if x.list && x.Type == nil {
			if _, ok := s.im.from[x].(cc.ExpressionNode); ok && !x.Is("macro") {
				s.e.untype(x)
			}
		}
	}
	for _, x := range all {
		s.e.g.save(x)
		x.ID = 0
		x.up = nil
	}
	return nil
}

// hole puts the hole's node where its text was.  The text was its C in
// parentheses: where they were not needed the forms keep them, `(paren
// ...)` at the `(`, and the hole replaces that, parentheses and all; where
// they were (an operand that binds less tightly than its operator) the
// forms do not have them, the hole's copy starting just after the `(`, and
// the hole replaces the copy: the C view writes the parentheses it needs.
func (s *synth) hole(j *fragJob, h fragHole, used map[*Node]bool) error {
	node := j.f.Holes[h.name]
	want := Lisp(node).String()
	var found, parent *Node
	idx := -1
	var look func(p *Node, ks []*Node) bool
	look = func(p *Node, ks []*Node) bool {
		for i, k := range ks {
			if c := s.im.from[k]; c != nil && !used[k] {
				pos := c.Position()
				switch {
				case pos.Filename != fragPath:
				case pos.Offset == h.off && k.Is("paren") && len(k.Kids) == 2 && Lisp(k.Kids[1]).String() == want,
					pos.Offset == h.off+1 && Lisp(k).String() == want:
					found, parent, idx = k, p, i
					return true
				}
			}
			if k.list && look(k, k.Kids) {
				return true
			}
		}
		return false
	}
	look(nil, j.nodes)
	if found == nil {
		return fmt.Errorf("frag %d: the hole $%s is not where it was written", j.k, h.name)
	}
	if used[node] {
		node = Clone(node)
	}
	used[node] = true
	if parent == nil {
		j.nodes[idx] = node
	} else {
		s.e.g.save(parent)
		parent.Kids[idx] = node
	}
	return nil
}

// target is the graph's node a fragment's refers edge to the synthesized
// node t goes to.
func (s *synth) target(t, from *Node) (*Node, error) {
	if s.inF[t] {
		return t, nil
	}
	if o := s.orig[t]; o != nil {
		return o, nil
	}
	if isUndeclared(t) {
		name := ""
		if len(t.Kids) > 1 {
			name = t.Kids[1].Atom
		}
		return nil, fmt.Errorf("frag: `%s` is declared nowhere visible from its place", name)
	}
	if x, err := s.external(t); x != nil || err != nil {
		return x, err
	}
	why := ""
	if len(s.mis) > 0 {
		why = "; the graph and the import of its C view differ at " + strings.Join(s.mis, ", ")
	}
	return nil, fmt.Errorf("frag: %s refers to #%d (%s), which maps to nothing in the graph%s", label(from), t.ID, label(t), why)
}

// external is this graph's node of a synthesized external one -- made if
// the graph has none -- or nil when t is not external.
func (s *synth) external(t *Node) (*Node, error) {
	if s.oext == nil {
		s.oext = map[string]*Node{}
		for _, x := range s.e.g.Externs {
			s.oext[externKey(x)] = x
		}
	}
	if t.Is("member") {
		owner := s.rowner[t]
		if owner == nil {
			return nil, nil
		}
		o, err := s.external(owner)
		if err != nil || o == nil {
			return nil, err
		}
		name := t.Kids[1].Atom
		for _, m := range o.Kids {
			if m.Is("member") && m.Kids[1].Atom == name {
				return m, nil
			}
		}
		m := NewList(NewAtom("member"), NewAtom(name))
		if t.Type != nil {
			ty, err := s.typ(t.Type)
			if err != nil {
				return nil, err
			}
			s.e.g.save(m)
			m.Type = ty
		}
		s.e.addMember(o, m)
		return m, nil
	}
	isExt := false
	for _, x := range s.im.g.Externs {
		if x == t {
			isExt = true
			break
		}
	}
	if !isExt {
		return nil, nil
	}
	k := externKey(t)
	if o := s.oext[k]; o != nil {
		return o, nil
	}
	n := NewList()
	for _, a := range t.Kids {
		if !a.list {
			s.e.g.save(n)
			n.Kids = append(n.Kids, NewAtom(a.Atom))
		}
	}
	s.oext[k] = n // before its type: a struct's members may be typed with it
	if t.Type != nil {
		ty, err := s.typ(t.Type)
		if err != nil {
			return nil, err
		}
		s.e.g.save(n)
		n.Type = ty
	}
	s.e.addTo(2, n)
	return n, nil
}

// externKey names an external node by its head and its name (a tag, or
// none for an anonymous one).
func externKey(x *Node) string {
	k := x.Head()
	if len(x.Kids) > 1 && !x.Kids[1].list {
		k += " " + x.Kids[1].Atom
	}
	return k
}

// typ is this graph's type node of the same structure as the synthesized
// t, made if it has none: a struct's, union's or enum's is the definition
// it maps to.
func (s *synth) typ(t *Node) (*Node, error) {
	if s.inF[t] {
		return t, nil
	}
	if o, ok := s.types[t]; ok {
		return o, nil
	}
	if o := s.orig[t]; o != nil {
		return o, nil
	}
	if !t.Is("basic") && !t.Is("pointer") && !t.Is("array") && !t.Is("function") {
		x, err := s.external(t)
		if err != nil {
			return nil, err
		}
		if x == nil {
			return nil, fmt.Errorf("frag: a type #%d (%s) maps to nothing in the graph", t.ID, label(t))
		}
		return x, nil
	}
	if s.okeys == nil {
		s.okeys = map[string]*Node{}
		s.omemo = map[*Node]string{}
		s.rmemo = map[*Node]string{}
		for _, o := range s.e.g.Types {
			if k := typeKey(o, s.omemo, identOf); s.okeys[k] == nil {
				s.okeys[k] = o
			}
		}
	}
	var kerr error
	k := typeKey(t, s.rmemo, func(x *Node) string {
		o, err := s.typ(x)
		if err != nil {
			kerr = err
			return "?"
		}
		return identOf(o)
	})
	if kerr != nil {
		return nil, kerr
	}
	if o := s.okeys[k]; o != nil {
		s.types[t] = o
		return o, nil
	}
	// a type the graph has not: made, its operands this graph's
	n := NewList()
	for _, a := range t.Kids {
		switch {
		case a.list: // a function's parameters
			ps := NewList()
			for _, p := range a.Kids {
				if p.Type == nil {
					s.e.g.save(ps)
					ps.Kids = append(ps.Kids, NewAtom(p.Atom))
					continue
				}
				pt, err := s.typ(p.Type)
				if err != nil {
					return nil, err
				}
				s.e.g.save(ps)
				ps.Kids = append(ps.Kids, &Node{Type: pt})
			}
			s.e.g.save(n)
			n.Kids = append(n.Kids, ps)
		case a.Type != nil:
			at, err := s.typ(a.Type)
			if err != nil {
				return nil, err
			}
			s.e.g.save(n)
			n.Kids = append(n.Kids, &Node{Type: at})
		default:
			s.e.g.save(n)
			n.Kids = append(n.Kids, NewAtom(a.Atom))
		}
	}
	s.e.addTo(1, n)
	s.types[t] = n
	s.okeys[k] = n
	return n, nil
}

func identOf(x *Node) string { return fmt.Sprintf("#%p", x) }

// typeKey is a type node's structure: its words, and its operands' keys; a
// node that is not a type node's (a definition, an external) is ident's.
func typeKey(t *Node, memo map[*Node]string, ident func(*Node) string) string {
	if t == nil {
		return "nil"
	}
	if k, ok := memo[t]; ok {
		return k
	}
	var k string
	switch {
	case t.Is("basic"), t.Is("pointer"), t.Is("array"), t.Is("function"):
		var b strings.Builder
		b.WriteString("(")
		for _, a := range t.Kids {
			switch {
			case a.list:
				b.WriteString(" (")
				for _, p := range a.Kids {
					if p.Type == nil {
						b.WriteString(" " + p.Atom)
					} else {
						b.WriteString(" " + typeKey(p.Type, memo, ident))
					}
				}
				b.WriteString(")")
			case a.Type != nil:
				b.WriteString(" " + typeKey(a.Type, memo, ident))
			default:
				b.WriteString(" " + a.Atom)
			}
		}
		b.WriteString(")")
		k = b.String()
	default:
		k = ident(t)
	}
	memo[t] = k
	return k
}

// addTo puts a new node in the types (1) or externs (2) section, with
// fresh ids.
func (e *Editor) addTo(sec int, n *Node) {
	n.up = e.top[sec]
	act := Act{Op: "add"}
	Walk(n, func(x *Node) bool {
		for _, k := range x.Kids {
			k.up = x
		}
		if x.ID == 0 && (x.list || len(x.Refs) > 0) {
			e.g.Fresh(x)
			act.New = append(act.New, x.ID)
		}
		return true
	})
	if sec == 1 {
		e.g.Types = append(e.g.Types, n)
	} else {
		e.g.Externs = append(e.g.Externs, n)
	}
	e.Log = append(e.Log, act)
}

// addMember puts `(member NAME)` in an external struct or union.
func (e *Editor) addMember(s, m *Node) {
	m.up = s
	for _, k := range m.Kids {
		k.up = m
	}
	e.g.save(s)
	s.Kids = append(s.Kids, m)
	e.g.Fresh(m)
	e.Log = append(e.Log, Act{Op: "add", New: []ID{m.ID}})
}

// retarget makes the uses outside the fragments that the synthesized
// import resolves to a fragment's declaration refer to it: in the forms
// printed whole, which hold every use that can (a top-level fragment's
// names colliding with the file's had every form using them printed whole).
func (s *synth) retarget() error {
	var moved []*Node
	for _, pr := range s.pairs {
		o, r := pr[0], pr[1]
		if !s.e.Live(o) {
			continue
		}
		for i, t := range r.Refs {
			if !s.inF[t] || i >= len(o.Refs) || o.Refs[i] == t {
				continue
			}
			s.e.retargetTo(o, i, t)
			moved = append(moved, o)
		}
	}
	// the types above a retargeted use, as the synthesized import has them
	for _, u := range moved {
		for q := u; q != nil && !IsStatement(q); q = s.e.Parent(q) {
			if err := s.retype(q); err != nil {
				return err
			}
		}
	}
	return nil
}

// retype gives a node of the forms printed whole the typed edge the
// synthesized import gave its printing, and takes it off Untyped.
func (s *synth) retype(q *Node) error {
	if s.rev == nil {
		s.rev = map[*Node]*Node{}
		for _, pr := range s.pairs {
			s.rev[pr[0]] = pr[1]
		}
	}
	r := s.rev[q]
	if r == nil || !q.list {
		return nil
	}
	var t *Node
	if r.Type != nil {
		var err error
		if t, err = s.typ(r.Type); err != nil {
			return err
		}
	}
	s.e.g.save(q)
	q.Type = t
	if t != nil {
		for i, u := range s.e.Untyped {
			if u == q {
				s.e.Untyped = append(s.e.Untyped[:i:i], s.e.Untyped[i+1:]...)
				break
			}
		}
		if s.e.inFile(t) {
			s.e.typedBy[t] = append(s.e.typedBy[t], q)
		}
	}
	return nil
}

// retargetTo is Retarget without its spelling's check: the synthesized
// import, which resolved the use by its spelling, said it.
func (e *Editor) retargetTo(use *Node, i int, to *Node) {
	e.g.save(use)
	use.Refs[i] = to
	e.extra[to] = append(e.extra[to], use)
	e.Log = append(e.Log, Act{Op: "retarget", Moved: []ID{use.ID}})
}
