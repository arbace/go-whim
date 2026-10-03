package graph

import (
	"fmt"
	"strconv"
	"strings"
)

// THE SWEEP AS GARBAGE COLLECTION.  Collect is crefactor/sweep's Prune on
// the graph: what the roots -- main, every file-scope static_assert, what
// the caller pins -- do not reach is collected, with Prune's guards and its
// cut of the locals nothing reads, and nothing is parsed.  A reference is
// what the graph's refers edges say it is, and the keys Prune reads off the
// text are read off the nodes instead:
//
//	o:NAME  a use of a file-scope declaration of NAME -- every declaration
//	        of one name is one thing, C's linkage -- a typedef name, a
//	        label, an enumerator's own name, and a name in a text the forms
//	        keep whole (a macro's invocation, an attribute's text)
//	t:TAG   a struct, union or enum form with that tag
//	m:NAME  a member, BY NAME: Prune's rule, `p->next` keeps every member
//	        called next whose struct lives (MembersByType is the graph's own
//	        rule, the member the edge names, and is not Prune's)
//
// A use whose edge is to a local -- a parameter, a block's declaration --
// is no key, as in Prune, where the parser's scopes say the same.

// CollectOptions are what the sweep must be told about the program.
type CollectOptions struct {
	// Roots are reached whatever else is: the program's entry points.
	Roots []string
	// FreezeLayoutIf names functions whose definition makes every struct's
	// layout a format another program reads: while one is defined, no
	// member goes (vim's ml_recover).
	FreezeLayoutIf []string
	// MembersByType keeps a member when a live use refers to it, rather than
	// when a live use names it: the graph's resolution, not Prune's.
	MembersByType bool
}

// MaxRounds is Prune's: a round that deletes a local may orphan what its
// initialiser named, and the next round finds it.
const MaxRounds = 15

// CollectStats is what a collection took, Prune's Stats by kind.
type CollectStats struct {
	Rounds                                 int
	Funcs, Objects, Protos, Typedefs, Tags int
	Members, Enumerators, Locals           int
	Fallthroughs                           int // fallthrough attributes that preceded no case label
	Pinned, KeptRuns, Positional           int
	Types, Externs                         int // the type and external nodes nothing live reaches any more
}

func (s CollectStats) String() string {
	return fmt.Sprintf("functions %d, objects %d, prototypes %d, typedefs %d, tags %d, members %d, "+
		"enumerators %d (%d survivors pinned), locals %d, fallthroughs %d -- %d rounds; kept: %d enumerators before a survivor "+
		"with no computable value, the members of %d structs initialised by position; and %d type nodes, %d external nodes",
		s.Funcs, s.Objects, s.Protos, s.Typedefs, s.Tags, s.Members, s.Enumerators, s.Pinned, s.Locals, s.Fallthroughs,
		s.Rounds, s.KeptRuns, s.Positional, s.Types, s.Externs)
}

func (s *CollectStats) add(o CollectStats) {
	s.Funcs += o.Funcs
	s.Objects += o.Objects
	s.Protos += o.Protos
	s.Typedefs += o.Typedefs
	s.Tags += o.Tags
	s.Members += o.Members
	s.Enumerators += o.Enumerators
	s.Locals += o.Locals
	s.Fallthroughs += o.Fallthroughs
	s.Pinned += o.Pinned
	s.KeptRuns += o.KeptRuns
	s.Positional = max(s.Positional, o.Positional)
}

// Collect collects g in place, round after round until a round deletes no
// local, and then the type and external nodes nothing left reaches.
func Collect(g *Graph, opt CollectOptions) (CollectStats, error) {
	var total CollectStats
	for round := 1; ; round++ {
		c := newCollector(g, opt)
		st, err := c.run()
		if err != nil {
			return total, fmt.Errorf("round %d: %w", round, err)
		}
		total.add(st)
		total.Rounds = round
		if st.Locals == 0 {
			break
		}
		if round >= MaxRounds {
			return total, fmt.Errorf("not converging after %d rounds", round)
		}
	}
	total.Types, total.Externs = g.unreached()
	return total, nil
}

// ent is one thing the closure keeps or cuts (Prune's).
type ent struct {
	kind byte // F function, O object, P prototype, T typedef, S struct/union, E enum, M member, N enumerator, D a tag declared, not defined
	name string
	key  string
	refs []string
	live bool
	form *Node // what deleting it removes: the top-level form, the member, the enumerator; S, E: the definition

	owner   *ent
	members []*ent
	nested  []*ent // M: the struct and union definitions written in it
	pinAll  bool
	hold    bool

	explicit bool
	val      int64
	valOK    bool
}

// decl is one top-level form, as a unit of deletion.
type decl struct {
	form  *Node
	fn    *ent
	items []*ent
	tags  []*ent
	keep  bool
}

type collector struct {
	g   *Graph
	opt CollectOptions

	ents    []*ent
	byKey   map[string][]*ent
	byName  map[string][]*ent
	live    map[string]bool
	named   map[string]bool
	reached map[*Node]bool // MembersByType: members a live use refers to
	decls   []*decl
	roots   []string
	structs map[string][]*ent
	typedef map[string]string
	file    map[*Node]bool // the file-scope declarations: top-level forms, their enumerators
	anon    map[*Node]string
	st      CollectStats
	queue   []string
}

func newCollector(g *Graph, opt CollectOptions) *collector {
	c := &collector{
		g: g, opt: opt,
		byKey: map[string][]*ent{}, byName: map[string][]*ent{},
		live: map[string]bool{}, named: map[string]bool{}, reached: map[*Node]bool{},
		structs: map[string][]*ent{}, typedef: map[string]string{},
		file: map[*Node]bool{}, anon: map[*Node]string{},
	}
	for _, f := range g.Forms {
		c.file[f] = true
		if f.Is("defn") {
			continue
		}
		Walk(f, func(n *Node) bool {
			if n.Is("enum") {
				for _, e := range body(n) {
					c.file[e] = true
				}
			}
			return true
		})
	}
	return c
}

func (c *collector) run() (CollectStats, error) {
	c.collect()
	c.enumValues()
	c.positional()
	c.close()
	dead := c.unusedLocals()
	if err := c.cut(dead, c.orphans()); err != nil {
		return c.st, err
	}
	return c.st, nil
}

func (c *collector) add(e *ent) *ent {
	c.ents = append(c.ents, e)
	c.byKey[e.key] = append(c.byKey[e.key], e)
	if e.kind == 'M' && e.name != "" {
		c.byName[e.name] = append(c.byName[e.name], e)
	}
	return e
}

// anonKey is an anonymous definition's key, the a:N of Prune.
func (c *collector) anonKey(n *Node) string {
	if k, ok := c.anon[n]; ok {
		return k
	}
	k := fmt.Sprintf("a:%d", len(c.anon)+1)
	c.anon[n] = k
	return k
}

// ---- collecting

func (c *collector) collect() {
	for _, r := range c.opt.Roots {
		c.roots = append(c.roots, "o:"+r)
	}
	for _, f := range c.g.Forms {
		d := &decl{form: f}
		c.decls = append(c.decls, d)
		switch f.Head() {
		case "defn":
			name := f.Kids[defNameAt(f)].Atom
			e := c.add(&ent{kind: 'F', name: name, key: "o:" + name, form: f})
			e.refs = c.keys(f, nil)
			d.fn = e
		case "def", "typedef":
			c.declaration(f, d)
		case "struct", "union", "enum":
			d.tags = c.tagsIn(f, nil)
		case "declare":
			d.tags = c.tagsIn(NewList(f.Args()...), nil)
		case "static_assert":
			c.roots = append(c.roots, c.keys(f, nil)...)
			d.keep = true
		default:
			d.keep = true // include, directive, verbatim, macro-decl
		}
	}
}

// declaration makes the entities of one top-level def or typedef: the tags
// its specifiers define (or name, when they define none), and the item.
func (c *collector) declaration(f *Node, d *decl) {
	t := defType(f)
	b := base(t, nil)
	d.tags = c.tagsIn(b, nil)
	skip := map[*Node]bool{}
	var extra []string
	for _, tg := range d.tags {
		if tg.kind == 'S' || tg.kind == 'E' {
			skip[tg.form] = true
			if strings.HasPrefix(tg.key, "a:") {
				extra = append(extra, tg.key)
			}
		}
	}
	name := f.Kids[defNameAt(f)].Atom
	kind := byte('O')
	switch {
	case hasPrefix(f, "typedef"):
		kind = 'T'
	case isFuncType(t):
		kind = 'P'
	}
	e := c.add(&ent{kind: kind, name: name, key: "o:" + name, form: f})
	e.refs = append(c.keys(f, skip), extra...)
	d.items = append(d.items, e)
	if kind == 'T' {
		for _, tg := range d.tags {
			c.typedef[name] = tg.key
		}
		if len(d.tags) == 0 {
			if tn := c.typeName(b); tn != "" {
				c.typedef[name] = "o:" + tn
			}
		}
	}
}

// isFuncType says a declarator's first derivation from the name is a
// function's parameter list: a prototype.
func isFuncType(t *Node) bool {
	for t.Is("name-attr") {
		t = t.Kids[1]
	}
	return t.Is("fn") || t.Is("fn-ids")
}

// typeName is the last typedef name a base names, outside struct and enum
// forms.
func (c *collector) typeName(b *Node) (out string) {
	Walk(b, func(n *Node) bool {
		if c.isTypedefName(n) {
			out = n.Atom
		}
		return !(n.Is("struct") || n.Is("union") || n.Is("enum"))
	})
	return out
}

// isTypedefName says an atom is a typedef name's use.
func (c *collector) isTypedefName(n *Node) bool {
	if n.list || len(n.Refs) != 1 {
		return false
	}
	t := n.Refs[0]
	return t.Is("typedef") || t.Is("extern-typedef") || (t.Is("def") && hasPrefix(t, "typedef"))
}

// tagsIn finds the outermost struct, union and enum definitions in a base
// and makes their entities; when there are none, the struct and union tags
// it names are D entities.  parent is the member a definition nested in a
// struct belongs to.
func (c *collector) tagsIn(b *Node, parent *ent) (tags []*ent) {
	if b == nil {
		return nil
	}
	Walk(b, func(n *Node) bool {
		if !isDefForm(n) {
			return true
		}
		if n.Is("enum") {
			tags = append(tags, c.enumDef(n))
		} else {
			tags = append(tags, c.structDef(n, parent))
		}
		return false
	})
	if len(tags) == 0 {
		Walk(b, func(n *Node) bool {
			if (n.Is("struct") || n.Is("union")) && tagOf(n) != "" {
				tags = append(tags, c.add(&ent{kind: 'D', name: tagOf(n), key: "t:" + tagOf(n), form: n}))
			}
			return true
		})
	}
	return tags
}

func (c *collector) structDef(x *Node, parent *ent) *ent {
	tag := tagOf(x)
	key := "t:" + tag
	if tag == "" {
		key = c.anonKey(x)
	}
	s := c.add(&ent{kind: 'S', name: tag, key: key, form: x})
	if tag != "" && parent != nil {
		parent.hold = true
		s.refs = append(s.refs, parent.owner.key)
	}
	if tag != "" {
		c.structs[tag] = append(c.structs[tag], s)
	}
	c.structs[key] = append(c.structs[key], s)
	for _, m := range members(x) {
		if m.Is("static_assert") {
			s.refs = append(s.refs, c.keys(m, nil)...)
			continue
		}
		probe := &ent{owner: s}
		name := memberDeclName(m)
		var mb *Node // the member's base: its specifiers
		if name != "" {
			mb = base(m.Kids[1], nil)
		} else {
			mb = m.Kids[0]
		}
		tags := c.tagsIn(mb, probe)
		skip := map[*Node]bool{}
		var nested []*ent
		var extra []string
		for _, t := range tags {
			if t.kind == 'S' {
				nested = append(nested, t)
			}
			if t.kind == 'S' || t.kind == 'E' {
				skip[t.form] = true
				if strings.HasPrefix(t.key, "a:") {
					extra = append(extra, t.key)
				}
			}
		}
		e := c.add(&ent{kind: 'M', name: name, key: "m:" + name, owner: s, form: m, hold: probe.hold || name == ""})
		e.nested = nested
		e.refs = append(c.keys(m, skip), extra...)
		if c.opt.MembersByType {
			c.byKey[memberRefKey(m)] = append(c.byKey[memberRefKey(m)], e)
		}
		s.members = append(s.members, e)
	}
	return s
}

func (c *collector) enumDef(x *Node) *ent {
	tag := tagOf(x)
	key := "t:" + tag
	if tag == "" {
		key = c.anonKey(x)
	}
	e := c.add(&ent{kind: 'E', name: tag, key: key, form: x})
	for _, a := range x.Args() {
		if a.Is(":") {
			e.refs = c.keys(a, nil)
		}
	}
	for _, n := range body(x) {
		name := n.Kids[0].Atom
		it := c.add(&ent{kind: 'N', name: name, key: "o:" + name, owner: e, form: n, explicit: enumValue(n) != nil})
		it.refs = append([]string{key}, c.keys(n, nil)...)
		e.members = append(e.members, it)
	}
	return e
}

// enumValue is an enumerator's value form, or nil: `(NAME ATTR... VALUE)`.
func enumValue(n *Node) *Node {
	if len(n.Kids) < 2 {
		return nil
	}
	last := n.Kids[len(n.Kids)-1]
	if isAttrForm(last) {
		return nil
	}
	return last
}

// ---- the closure (Prune's)

func (c *collector) close() {
	c.queue = append(c.queue, c.roots...)
	c.drain()
	for c.guards() {
		c.drain()
	}
}

func (c *collector) drain() {
	for len(c.queue) > 0 {
		k := c.queue[len(c.queue)-1]
		c.queue = c.queue[:len(c.queue)-1]
		if strings.HasPrefix(k, "m:") {
			nm := k[2:]
			if c.named[nm] {
				continue
			}
			c.named[nm] = true
			for _, m := range c.byName[nm] {
				if m.owner.live {
					c.activate(m)
				}
			}
			continue
		}
		if strings.HasPrefix(k, "r:") {
			// MembersByType: a member a live use's edge names
			for _, m := range c.byKey[k] {
				if m.owner.live {
					c.activate(m)
				}
			}
			c.live[k] = true
			continue
		}
		if c.live[k] {
			continue
		}
		c.live[k] = true
		for _, e := range c.byKey[k] {
			if e.kind != 'M' {
				c.activate(e)
			}
		}
	}
}

func (c *collector) activate(e *ent) {
	if e.live {
		return
	}
	e.live = true
	c.queue = append(c.queue, e.refs...)
	if e.kind == 'S' {
		for _, m := range e.members {
			if e.pinAll || m.hold || c.memberNamed(m) {
				c.activate(m)
			}
		}
	}
}

func (c *collector) memberNamed(m *ent) bool {
	if c.opt.MembersByType && c.live[memberRefKey(m.form)] {
		return true
	}
	return c.named[m.name]
}

func (c *collector) guards() bool {
	grew := false
	for _, e := range c.ents {
		if !e.live || len(e.members) == 0 {
			continue
		}
		anyLive := false
		for _, m := range e.members {
			anyLive = anyLive || m.live
		}
		if !anyLive {
			for _, m := range e.members {
				c.activate(m)
			}
			grew = true
			continue
		}
		if e.kind != 'E' {
			continue
		}
		deleted := false
		var run []*ent
		for _, n := range e.members {
			if !n.live {
				deleted = true
				run = append(run, n)
				continue
			}
			if deleted && !n.explicit && !n.valOK {
				for _, r := range run {
					c.activate(r)
				}
				c.st.KeptRuns += len(run)
				grew = true
			}
			deleted, run = false, nil
		}
	}
	return grew
}

// ---- positional initialisers (Prune's guard)

func (c *collector) positional() {
	for _, name := range c.opt.FreezeLayoutIf {
		for _, e := range c.byKey["o:"+name] {
			if e.kind != 'F' {
				continue
			}
			for _, s := range c.ents {
				if s.kind == 'S' {
					s.pinAll = true
				}
			}
			return
		}
	}
	marked := map[*ent]bool{}
	var mark func(key string)
	mark = func(key string) {
		for _, s := range c.structsOf(key) {
			if marked[s] {
				continue
			}
			marked[s] = true
			s.pinAll = true
			for _, m := range s.members {
				for _, r := range c.memberTypes(m) {
					mark(r)
				}
			}
		}
	}
	for _, f := range c.g.Forms {
		Walk(f, func(n *Node) bool {
			switch {
			case n.Is("def") && !hasPrefix(n, "__auto_type"):
				t := defType(n)
				v := n.Kids[len(n.Kids)-1]
				if t == nil || v == t || !v.Is("init") {
					return true
				}
				key := c.specKey(base(t, nil))
				if key != "" && byPosition(v, arrayDepth(t)) {
					mark(key)
				}
			case n.Is("literal"):
				t := literalType(n)
				if t == nil {
					return true
				}
				key := c.specKey(base(t, nil))
				if key != "" && byPosition(NewList(append([]*Node{NewAtom("init")}, literalItems(n)...)...), 0) {
					mark(key)
				}
			}
			return true
		})
	}
	c.st.Positional = len(marked)
}

// literalType and literalItems are a compound literal's type and elements:
// `(literal STORAGE... T ITEM...)`.
func literalType(n *Node) *Node {
	for _, a := range n.Args() {
		if !a.list && prefixWords[a.Atom] {
			continue
		}
		return a
	}
	return nil
}

func literalItems(n *Node) []*Node {
	args := n.Args()
	for i, a := range args {
		if !a.list && prefixWords[a.Atom] {
			continue
		}
		return args[i+1:]
	}
	return nil
}

// memberTypes is what a member is, when it holds structs by value: the
// keys Prune scans its declaration for, unless every declarator is a
// pointer's.
func (c *collector) memberTypes(m *ent) []string {
	f := m.form
	var star bool
	if name := memberDeclName(f); name != "" {
		star = declaratorStar(f.Kids[1]) || len(f.Kids) > 2 && hasStar(NewList(f.Kids[2:]...))
	} else {
		star = hasStar(f)
	}
	if star {
		return nil
	}
	var out []string
	for _, k := range c.keys(f, nil) {
		if strings.HasPrefix(k, "t:") || strings.HasPrefix(k, "o:") {
			out = append(out, k)
		}
	}
	for _, s := range m.nested {
		if s.name == "" {
			out = append(out, s.key)
		}
	}
	return out
}

// declaratorStar says a declarator's text has a `*`: a pointer's, or one in
// an array's size or a function's parameters.
func declaratorStar(t *Node) bool {
	star := false
	base(t, func(d *Node) {
		switch d.Head() {
		case "ptr":
			star = true
		case "array":
			for _, a := range d.Kids[1 : len(d.Kids)-1] {
				star = star || hasStar(a)
			}
		case "fn", "fn-ids":
			star = star || hasStar(d.Kids[1])
		}
	})
	return star
}

// hasStar says a form's C has a `*` in it.
func hasStar(n *Node) bool {
	found := false
	Walk(n, func(x *Node) bool {
		switch {
		case found:
		case x.list && (x.Is("ptr") || x.Is("deref") || x.Is("*") || x.Is("*=")):
			found = true
		case !x.list && x.Atom == "*":
			found = true
		}
		return !found
	})
	return found
}

func (c *collector) structsOf(key string) []*ent {
	for i := 0; i < 8; i++ {
		switch {
		case strings.HasPrefix(key, "t:"):
			return c.structs[key[2:]]
		case strings.HasPrefix(key, "a:"):
			return c.structs[key]
		case strings.HasPrefix(key, "o:"):
			next, ok := c.typedef[key[2:]]
			if !ok {
				return nil
			}
			key = next
			continue
		}
		return nil
	}
	return nil
}

// specKey is what a base names as its type: the last typedef name, or a
// struct or union, outside struct and enum forms.
func (c *collector) specKey(b *Node) (key string) {
	if b == nil {
		return ""
	}
	Walk(b, func(n *Node) bool {
		switch {
		case c.isTypedefName(n):
			key = "o:" + n.Atom
		case n.Is("struct") || n.Is("union"):
			if t := tagOf(n); t != "" {
				key = "t:" + t
			} else {
				key = c.anonKey(n)
			}
			return false
		case n.Is("enum"):
			return false
		}
		return true
	})
	return key
}

// arrayDepth is how many array derivations Prune counts on a declarator:
// those of its outermost direct declarator, the ones C writes after the
// last parenthesised group -- in the forms, deeper than a `(paren ...)` or
// a pointer to an array or function, which C parenthesises.  `T *(a[3])`
// counts none, `T (*a)[3]` one.
func arrayDepth(t *Node) int {
	d := 0
	base(t, func(x *Node) {
		switch {
		case x.Is("array"):
			d++
		case x.Is("paren"), x.Is("ptr") && (x.Kids[1].Is("array") || x.Kids[1].Is("fn") || x.Kids[1].Is("fn-ids")):
			d = 0
		}
	})
	return d
}

// byPosition says a brace list, below its array levels, has an element
// that names no member.
func byPosition(in *Node, depth int) bool {
	if !in.Is("init") {
		return false
	}
	for _, el := range in.Args() {
		if depth > 0 {
			v := el
			if el.Is("at") {
				v = el.Kids[len(el.Kids)-1]
			}
			if byPosition(v, depth-1) {
				return true
			}
			continue
		}
		if !el.Is("at") {
			return true
		}
	}
	return false
}

// ---- enumerator values

func (c *collector) enumValues() {
	env := map[string]int64{}
	for _, e := range c.ents {
		if e.kind != 'E' {
			continue
		}
		var prev int64 = -1
		ok := true
		for _, n := range e.members {
			if n.explicit {
				prev, ok = evalForm(enumValue(n.form), env)
			} else {
				prev++
			}
			n.val, n.valOK = prev, ok
			if ok {
				env[n.name] = prev
			}
		}
	}
}

// memberRefKey is MembersByType's key of a member's form: a use whose edge
// names it keeps it.
func memberRefKey(m *Node) string { return "r:" + strconv.FormatUint(uint64(m.ID), 10) }

// memberKey is a member use's key: by name, Prune's; by the edge, when
// MembersByType and the edge resolved.
func (c *collector) memberKey(n *Node, name string) string {
	if c.opt.MembersByType {
		if t := n.Ref(); t != nil && !isExtern(t) {
			return memberRefKey(t)
		}
	}
	return "m:" + name
}
