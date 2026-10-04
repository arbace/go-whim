package graph

import (
	"bytes"
	"crypto/sha256"
	varint "encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
)

// CONTENT HASHES, Unison's way, beside the sequential ids (doc/GRAPH.md,
// *Content hashes, measured*).  Every node with an id -- every list, and
// every atom an edge starts from -- gets a SHA-256 of its form: its text, its
// elements (an element with an id by its hash, a token inline), and each of
// its edges, refers and typed, by the TARGET'S HASH.  So a node's hash is a
// function of what it says and of what it names, never of its id or its
// place: two nodes that say the same thing of the same things are one hash,
// and a change moves exactly the nodes above it and, through the edges, the
// nodes that name what moved (and the nodes above those).
//
// CYCLES.  A node depends on its elements and its edges' targets; where
// those dependencies close a cycle -- a function calling itself (the call's
// name refers to the definition that holds it), two functions calling each
// other through their definitions, a struct holding a pointer to itself (the
// member's typed edge to `(pointer @:S)`, whose operand is S) -- the strongly
// connected component is hashed as ONE unit, as Unison hashes a cycle of
// definitions: each member is serialised with an edge or element inside the
// component written as the member's POSITION in it, the serialisations are
// hashed together, and a member's hash is that hash and its position.  The
// positions are those of the classes of a colour refinement of the
// component (cycle, below), sorted by their hashes, so they depend on
// nothing but what the members say: not on the order the file holds them
// in, nor on their ids.  Members the refinement cannot tell apart say the
// same of the same things, and are one hash -- two equal type nodes in one
// cycle, say, which an import interns and an edit may not.
//
// EXTERNAL NODES are the headers' names, not definitions the file holds:
// an external aggregate (`(extern-struct stat (member st_size) ...)`) is
// hashed by its head and its name, and each member it lists by its own form
// and the aggregate's name.  The members listed are a record of which the
// file uses, in the order it met them; an import and a graph edited from
// phase to phase list them differently, and neither moves a hash.
//
// The ids are not hashed, so a graph read back, imported again, or
// renumbered has the same hashes; what the sections hold is hashed node by
// node, so the order of the type nodes and the externs does not matter
// either.

// A Hash is a node's content address.
type Hash [sha256.Size]byte

func (h Hash) String() string { return hex.EncodeToString(h[:]) }

// Short is the hash's first 12 hex digits.
func (h Hash) Short() string { return hex.EncodeToString(h[:6]) }

// HashOptions say what a hash covers.
type HashOptions struct {
	// Untyped leaves the typed edges out: a node is hashed by its form
	// and its refers edges alone.  A measurement of what the typed edges'
	// cascade costs (a struct's change moves every expression of its
	// type), not the scheme.
	Untyped bool
	// Nominal writes an edge to a type's definition -- a typedef, a
	// tagged struct, union or enum -- as its name, not its hash: C's types
	// are nominal, and so a struct that loses a member moves its own forms
	// and no other.  A measurement of what the definitions' cascade costs,
	// not the scheme.
	Nominal bool
}

// Hashes are a graph's content hashes.
type Hashes struct {
	Nodes []*Node // every node with an id, in the containment's order (Walk's)
	Of    []Hash  // Of[i] is Nodes[i]'s hash
	Size  []int32 // Size[i] is the length of Nodes[i]'s serialisation: what a store keyed by hash holds for it
	// Components counts the strongly connected components; Cyclic those
	// hashed as a cycle (more than one node, or a node naming itself),
	// InCycles their nodes, Largest the largest.
	Components, Cyclic, InCycles, Largest int
	index                                 map[*Node]int32
}

// Hash is n's hash, false if n is not a node of the graph hashed.
func (h *Hashes) Hash(n *Node) (Hash, bool) {
	i, ok := h.index[n]
	if !ok {
		return Hash{}, false
	}
	return h.Of[i], true
}

// Hash computes every node's content hash.  It refuses a graph with an
// edge to a node it does not hold.
func (g *Graph) Hash(opt HashOptions) (*Hashes, error) {
	hs := &Hashes{index: map[*Node]int32{}}
	owner := map[*Node]string{} // an external aggregate's members: its name
	for _, x := range g.Externs {
		if externAggregate(x) {
			for _, k := range x.Kids[2:] {
				if k.Is("member") {
					owner[k] = x.Kids[1].Atom
				}
			}
		}
	}
	g.Walk(func(n *Node) bool {
		if hashed(n) {
			hs.index[n] = int32(len(hs.Nodes))
			hs.Nodes = append(hs.Nodes, n)
		}
		return true
	})
	nn := len(hs.Nodes)
	hs.Of = make([]Hash, nn)
	hs.Size = make([]int32, nn)
	// the dependencies, CSR: a node's elements with ids, and the targets of
	// its edges and of its tokens' edges
	off := make([]int32, nn+1)
	deps := make([]int32, 0, nn*2)
	var bad error
	var names map[*Node]string
	if opt.Nominal {
		names = nominals(g)
	}
	target := func(t *Node) {
		if names[t] != "" {
			return // not a dependency: written by its name
		}
		i, ok := hs.index[t]
		if !ok {
			if bad == nil {
				bad = fmt.Errorf("graph: an edge to a node the graph does not hold (id %d)", t.ID)
			}
			return
		}
		deps = append(deps, i)
	}
	edges := func(n *Node) {
		for _, r := range n.Refs {
			target(r)
		}
		if n.Type != nil && !opt.Untyped {
			target(n.Type)
		}
	}
	var token func(k *Node)
	token = func(k *Node) {
		edges(k)
		for _, x := range k.Kids {
			if hashed(x) {
				target(x)
			} else {
				token(x)
			}
		}
	}
	for i, n := range hs.Nodes {
		off[i] = int32(len(deps))
		for _, k := range n.Kids {
			if owner[k] != "" {
				continue
			}
			if hashed(k) {
				deps = append(deps, hs.index[k])
			} else {
				token(k)
			}
		}
		edges(n)
	}
	off[nn] = int32(len(deps))
	if bad != nil {
		return nil, bad
	}
	s := hasher{hs: hs, opt: opt, owner: owner, names: names, comp: make([]int32, nn), pos: make([]int32, nn),
		cycleOf: make([]int32, nn), refH: map[int32]Hash{}, match: map[Hash]int32{}}
	for i := range s.comp {
		s.comp[i] = -1
	}
	tarjan(nn, off, deps, func(members []int32) {
		hs.Components++
		self := false
		if len(members) == 1 {
			for _, d := range deps[off[members[0]]:off[members[0]+1]] {
				if d == members[0] {
					self = true
				}
			}
		}
		for _, m := range members {
			s.comp[m] = int32(hs.Components)
		}
		if len(members) == 1 && !self {
			if s.matched(members[0], deps[off[members[0]]:off[members[0]+1]]) {
				return
			}
			s.cur = -1
			b := s.ser(members[0], serPos)
			hs.Of[members[0]] = sha256.Sum256(b)
			hs.Size[members[0]] = int32(len(b))
			return
		}
		hs.Cyclic++
		hs.InCycles += len(members)
		hs.Largest = max(hs.Largest, len(members))
		s.cycle(members)
	})
	return hs, nil
}

// hashed says n is a node of its own (an id, or a list), not a token of its
// form.
func hashed(n *Node) bool { return n.ID != 0 || n.list }

type hasher struct {
	hs    *Hashes
	opt   HashOptions
	comp  []int32 // a node's component, once its component is done or being done
	pos   []int32 // a cycle member's position in its component
	cur   int32   // the component being hashed, -1 for none
	buf   []byte
	cls   map[int32]Hash // a cycle member's class in the refinement
	owner map[*Node]string
	names map[*Node]string // Nominal's: the types' definitions by name
	// A node of a cycle, or bisimilar to one (matched): its component, its
	// class's hash of the refinement's last round but one (how a node
	// bisimilar to it writes it), and the classes by their last round's.
	cycleOf []int32
	refH    map[int32]Hash
	match   map[Hash]int32
}

// matched gives node i, a component of its own, the hash of the cycle's
// member it is bisimilar to, if it is: a node outside a cycle that says
// what a member says of what the member names -- a type node an edit made
// equal to one in a cycle, `(pointer @:S)` beside S's own -- is one hash
// with it, whatever the graph's edges make of their components.
func (s *hasher) matched(i int32, deps []int32) bool {
	c := int32(0)
	for _, d := range deps {
		switch k := s.cycleOf[d]; {
		case k == 0:
		case c == 0:
			c = k
		case c != k:
			return false // a member names its own cycle's nodes alone
		}
	}
	if c == 0 {
		return false
	}
	s.cur = c
	h := sha256.Sum256(s.ser(i, serMatch))
	s.cur = -1
	m, ok := s.match[h]
	if !ok || s.cycleOf[m] != c {
		return false
	}
	s.hs.Of[i], s.hs.Size[i] = s.hs.Of[m], s.hs.Size[m]
	s.cycleOf[i], s.refH[i] = c, s.refH[m]
	return true
}

// cycle hashes a component of several nodes (or one naming itself) as one.
// The members' positions come from a refinement: each member is hashed
// with every reference inside the component erased, then again with each
// such reference written as its target's hash of the round before, until
// a round splits no class further (Weisfeiler and Lehman's colour
// refinement, whose stable partition is bisimilarity: the edges are
// ordered).  The classes, sorted by their last hashes, are the positions.
func (s *hasher) cycle(members []int32) {
	s.cur = s.comp[members[0]]
	type class struct {
		i int32
		h Hash
	}
	es := make([]class, len(members))
	if s.cls == nil {
		s.cls = map[int32]Hash{}
	}
	distinct := func() int {
		seen := map[Hash]bool{}
		for _, e := range es {
			seen[e.h] = true
		}
		return len(seen)
	}
	for j, m := range members {
		es[j] = class{m, sha256.Sum256(s.ser(m, serErase))}
	}
	for k := distinct(); ; {
		for _, e := range es {
			s.cls[e.i] = e.h
		}
		for j := range es {
			es[j].h = sha256.Sum256(s.ser(es[j].i, serClass))
		}
		k2 := distinct()
		if k2 == k {
			break
		}
		k = k2
	}
	// the partition is stable: a node bisimilar to a member, hashed with
	// its targets' classes of the round before, hashes as that member did
	for _, e := range es {
		s.cycleOf[e.i] = s.cur
		s.refH[e.i] = s.cls[e.i]
		s.match[e.h] = e.i
	}
	clear(s.cls)
	// the classes, in their hashes' order; a member's position is its
	// class's, and the component is hashed as its classes, one member
	// each: members no round tells apart say the same of the same things
	// (bisimilar), and are one hash, as identical forms are.
	sort.Slice(es, func(a, b int) bool { return bytes.Compare(es[a].h[:], es[b].h[:]) < 0 })
	rank := int32(-1)
	for j, e := range es {
		if j == 0 || e.h != es[j-1].h {
			rank++
		}
		s.pos[e.i] = rank
	}
	all := []byte{'S'}
	all = varint.AppendUvarint(all, uint64(rank+1))
	sizes := make([]int32, rank+1)
	for j, e := range es {
		if j > 0 && e.h == es[j-1].h {
			continue
		}
		b := s.ser(e.i, serPos)
		sizes[s.pos[e.i]] = int32(len(b))
		all = varint.AppendUvarint(all, uint64(len(b)))
		all = append(all, b...)
	}
	scc := sha256.Sum256(all)
	for _, e := range es {
		m := append([]byte{'M'}, scc[:]...)
		m = varint.AppendUvarint(m, uint64(s.pos[e.i]))
		s.hs.Of[e.i] = sha256.Sum256(m)
		s.hs.Size[e.i] = sizes[s.pos[e.i]]
	}
	s.cur = -1
}

// How a reference inside the component being hashed is written.
const (
	serPos   = iota // as its target's position
	serErase        // all alike
	serClass        // as its target's class, the refinement's last round
	serMatch        // as its target's class, a cycle's last round but one: a node matched against its members
)

// ser is node i's serialisation, its edges and its elements with ids by
// their hashes -- or, inside the component being hashed, as mode says.
func (s *hasher) ser(i int32, mode int) []byte {
	s.buf = s.node(s.buf[:0], s.hs.Nodes[i], mode)
	return s.buf
}

func (s *hasher) node(b []byte, n *Node, mode int) []byte {
	if n.list {
		b = append(b, 'L')
		kids := n.Kids
		if externAggregate(n) {
			kids = kids[:2] // its head and its name
		}
		b = varint.AppendUvarint(b, uint64(len(kids)))
		for _, k := range kids {
			if hashed(k) {
				b = s.ref(b, k, mode)
			} else {
				b = append(b, 'I')
				b = s.node(b, k, mode)
			}
		}
	} else {
		b = append(b, 'A')
		b = varint.AppendUvarint(b, uint64(len(n.Atom)))
		b = append(b, n.Atom...)
	}
	b = varint.AppendUvarint(b, uint64(len(n.Refs)))
	for _, r := range n.Refs {
		b = s.ref(b, r, mode)
	}
	if n.Type != nil && !s.opt.Untyped {
		b = append(b, 'T')
		b = s.ref(b, n.Type, mode)
	} else {
		b = append(b, '-')
	}
	if o := s.owner[n]; o != "" {
		b = append(b, 'P')
		b = varint.AppendUvarint(b, uint64(len(o)))
		b = append(b, o...)
	}
	return b
}

// nominals are the names types' definitions are known by
// (HashOptions.Nominal): a typedef's; a tagged struct's, union's or
// enum's; an untagged one's, the typedef's it is the type of.
func nominals(g *Graph) map[*Node]string {
	names := map[*Node]string{}
	g.Walk(func(t *Node) bool {
		switch h := t.Head(); h {
		case "typedef", "def":
			if h == "def" && !hasPrefix(t, "typedef") {
				break
			}
			if name := topName(t); name != "" {
				names[t] = "typedef " + name
				for _, k := range t.Kids {
					if aggregateDef(k) && k.Kids[1].list {
						names[k] = "typedef " + name + " of"
					}
				}
			}
		case "struct", "union", "enum":
			if aggregateDef(t) && !t.Kids[1].list {
				names[t] = h + " " + t.Kids[1].Atom
			}
		}
		return true
	})
	return names
}

// aggregateDef says t is a struct's, union's or enum's definition.
func aggregateDef(t *Node) bool {
	switch t.Head() {
	case "struct", "union", "enum":
		return len(t.Kids) > 2 || len(t.Kids) == 2 && t.Kids[1].list
	}
	return false
}

// externAggregate says n is an external struct, union or enum.
func externAggregate(n *Node) bool {
	switch n.Head() {
	case "extern-struct", "extern-union", "extern-enum":
		return len(n.Kids) >= 2 && !n.Kids[1].list
	}
	return false
}

func (s *hasher) ref(b []byte, t *Node, mode int) []byte {
	i := s.hs.index[t]
	if s.opt.Nominal {
		if name := s.names[t]; name != "" {
			b = append(b, 'N')
			b = varint.AppendUvarint(b, uint64(len(name)))
			return append(b, name...)
		}
	}
	if mode == serMatch {
		if s.cycleOf[i] == s.cur {
			h := s.refH[i]
			b = append(b, 'C')
			return append(b, h[:]...)
		}
	} else if s.cur >= 0 && s.comp[i] == s.cur {
		b = append(b, 'C')
		switch mode {
		case serPos:
			b = varint.AppendUvarint(b, uint64(s.pos[i]))
		case serClass:
			h := s.cls[i]
			b = append(b, h[:]...)
		}
		return b
	}
	b = append(b, 'H')
	return append(b, s.hs.Of[i][:]...)
}

// tarjan calls emit with each strongly connected component of the graph
// whose node v depends on deps[off[v]:off[v+1]], every component after the
// components it depends on.  Iterative: the dependency chains are long.
func tarjan(n int, off, deps []int32, emit func([]int32)) {
	index := make([]int32, n)
	low := make([]int32, n)
	on := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var stack []int32
	type frame struct{ v, e int32 }
	var call []frame
	next := int32(0)
	for root := int32(0); root < int32(n); root++ {
		if index[root] >= 0 {
			continue
		}
		call = append(call, frame{root, off[root]})
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		on[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			v := f.v
			if f.e < off[v+1] {
				w := deps[f.e]
				f.e++
				if index[w] < 0 {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					on[w] = true
					call = append(call, frame{w, off[w]})
				} else if on[w] && index[w] < low[v] {
					low[v] = index[w]
				}
				continue
			}
			call = call[:len(call)-1]
			if len(call) > 0 {
				if u := call[len(call)-1].v; low[v] < low[u] {
					low[u] = low[v]
				}
			}
			if low[v] == index[v] {
				j := len(stack) - 1
				for stack[j] != v {
					j--
				}
				members := stack[j:]
				for _, m := range members {
					on[m] = false
				}
				emit(members)
				stack = stack[:j]
			}
		}
	}
}
