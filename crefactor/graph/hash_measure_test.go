package graph

import (
	"bytes"
	"crypto/sha256"
	varint "encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// GRAPH_BOUNDARIES is a directory of the pipeline's boundaries, qNNN.c and
// the graph snapshots in their store (.cache/boundaries); the hash
// measurements read each boundary's graph from its snapshot when it is
// there and of that text, and import the text otherwise.  GRAPH_JOBS is how
// many at once (8; 1 for the times).
func boundaries(t *testing.T) (dir string, ns []int, jobs int) {
	dir = os.Getenv("GRAPH_BOUNDARIES")
	if dir == "" {
		t.Skip("GRAPH_BOUNDARIES is not set")
	}
	for n := 0; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("q%03d.c", n))); err != nil {
			break
		}
		ns = append(ns, n)
	}
	if len(ns) == 0 {
		t.Fatalf("GRAPH_BOUNDARIES=%s: no q000.c", dir)
	}
	jobs = 8
	if s := os.Getenv("GRAPH_JOBS"); s != "" {
		jobs, _ = strconv.Atoi(s)
	}
	return dir, ns, jobs
}

// boundary is boundary n's text and graph: read from the store when its
// snapshot is of qNNN.c (its header names the text's digest), else
// imported.  read says which; gsize is the snapshot's bytes.
func boundary(dir string, n int) (src []byte, g *Graph, read bool, gsize int, err error) {
	src, err = os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
	if err != nil {
		return nil, nil, false, 0, err
	}
	sum := sha256.Sum256(src)
	if b, e := ReadStoreLisp(dir, fmt.Sprintf("q%03d", n)); e == nil &&
		bytes.HasPrefix(b, fmt.Appendf(nil, ";; the graph of q%03d.c, sha256 %s\n", n, hex.EncodeToString(sum[:]))) {
		g, err = Read(b)
		return src, g, true, len(b), err
	}
	g, _, err = Import(fmt.Sprintf("q%03d.c", n), src)
	return src, g, false, 0, err
}

// unitLen is a top-level node's bytes as the Lisp writes it.
func unitLen(n *Node) int {
	var b bytes.Buffer
	layout(&b, n, 0)
	return b.Len() + 1
}

type hashSnap struct {
	n        int
	read     bool
	gsize    int
	nodes    int
	keys     []uint64         // every node's hash, its first 8 bytes
	size     map[uint64]int32 // a node's serialisation, by key
	forms    []uint64         // the top-level forms' hashes, in order
	untyped  []uint64         // the same, the typed edges left out
	nominal  []uint64         // the same, the types' definitions by name
	units    map[uint64]int   // forms, type and external nodes: their Lisp bytes, by hash
	nunits   int
	hashTime time.Duration
	cyclic   int
	inCycles int
	largest  int
}

func key(h Hash) uint64 { return varint.BigEndian.Uint64(h[:8]) }

// TestMeasureHashes measures what content addresses would share across the
// pipeline's boundaries (doc/GRAPH.md, *Content hashes, measured*):
// distinct hashes against the nodes summed over the snapshots; per phase,
// the top-level forms that keep their hash; a store keyed by hash against
// the graph snapshots; the hashing's cost.  GRAPH_HASH_IMPORT=1 also imports
// every text whose graph was read and holds the two to the same hashes,
// form by form and node by node: the hashes do not see the ids, which the
// snapshot carried from phase to phase and the import gives afresh.
func TestMeasureHashes(t *testing.T) {
	dir, ns, jobs := boundaries(t)
	cross := os.Getenv("GRAPH_HASH_IMPORT") != ""
	snaps := make([]*hashSnap, len(ns))
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for i, n := range ns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			src, g, read, gsize, err := boundary(dir, n)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			s := &hashSnap{n: n, read: read, gsize: gsize, size: map[uint64]int32{}, units: map[uint64]int{}}
			runtime.GC()
			start := time.Now()
			h, err := g.Hash(HashOptions{})
			s.hashTime = time.Since(start)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			u, err := g.Hash(HashOptions{Untyped: true})
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			v, err := g.Hash(HashOptions{Nominal: true})
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			s.nodes = len(h.Nodes)
			s.cyclic, s.inCycles, s.largest = h.Cyclic, h.InCycles, h.Largest
			s.keys = make([]uint64, len(h.Of))
			for j, x := range h.Of {
				k := key(x)
				s.keys[j] = k
				s.size[k] = h.Size[j]
			}
			for _, f := range g.Forms {
				x, _ := h.Hash(f)
				s.forms = append(s.forms, key(x))
				y, _ := u.Hash(f)
				s.untyped = append(s.untyped, key(y))
				z, _ := v.Hash(f)
				s.nominal = append(s.nominal, key(z))
			}
			for _, sec := range g.Sections() {
				for _, f := range sec {
					x, _ := h.Hash(f)
					s.units[key(x)] = unitLen(f)
					s.nunits++
				}
			}
			if cross && read {
				ig, _, err := Import(fmt.Sprintf("q%03d.c", n), src)
				if err == nil {
					var ih *Hashes
					if ih, err = ig.Hash(HashOptions{}); err == nil {
						err = sameHashesErr(g, h, ig, ih)
					}
				}
				if err != nil {
					t.Errorf("q%03d: the graph read and the import hash differently: %v", n, err)
				}
			}
			snaps[i] = s
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}

	// distinct hashes against the nodes summed
	all := map[uint64]int32{}
	units := map[uint64]int{}
	sum, unitsSum := 0, 0
	gStore, gFiles, nodeStore, manifests := 0, 0, 0, 0
	unitStore := 0
	seenUnit := map[uint64]bool{}
	seenNode := map[uint64]bool{}
	var times []time.Duration
	for _, s := range snaps {
		sum += s.nodes
		unitsSum += s.nunits
		for k, z := range s.size {
			all[k] = z
		}
		for k, l := range s.units {
			units[k] = l
		}
		times = append(times, s.hashTime)
		if !s.read {
			continue
		}
		// the store set against the graph snapshots: the boundaries with one
		gFiles++
		gStore += s.gsize
		manifests += 32 * s.nunits
		for k, z := range s.size {
			if !seenNode[k] {
				seenNode[k] = true
				nodeStore += int(z) + 32
			}
		}
		for k, l := range s.units {
			if !seenUnit[k] {
				seenUnit[k] = true
				unitStore += l + 32
			}
		}
	}
	t.Logf("%d snapshots: %d nodes summed, %d distinct hashes (%.1f%%); %d top-level units summed (forms, types, externs), %d distinct (%.1f%%)",
		len(snaps), sum, len(all), 100*float64(len(all))/float64(sum), unitsSum, len(units), 100*float64(len(units))/float64(unitsSum))
	t.Logf("against the %d graph snapshots, %d bytes: a store of nodes by hash %d bytes (%d distinct, each its serialisation and its key) and %d of manifests (%.1f%%); a store of top-level units by hash, each its Lisp, %d bytes (%d distinct) and the manifests (%.1f%%)",
		gFiles, gStore, nodeStore, len(seenNode), manifests, 100*float64(nodeStore+manifests)/float64(gStore),
		unitStore, len(seenUnit), 100*float64(unitStore+manifests)/float64(gStore))
	slices.Sort(times)
	var total time.Duration
	for _, d := range times {
		total += d
	}
	t.Logf("hashing: median %v, min %v, max %v, total %v", ms(times[len(times)/2]), ms(times[0]), ms(times[len(times)-1]), ms(total))

	// per phase: the top-level forms of qN whose hash q(N-1) holds
	keptAll, formsAll, ukeptAll, nkeptAll := 0, 0, 0, 0
	for i := 1; i < len(snaps); i++ {
		a, b := snaps[i-1], snaps[i]
		prev := map[uint64]bool{}
		for _, k := range a.forms {
			prev[k] = true
		}
		uprev := map[uint64]bool{}
		for _, k := range a.untyped {
			uprev[k] = true
		}
		vprev := map[uint64]bool{}
		for _, k := range a.nominal {
			vprev[k] = true
		}
		kept, ukept, vkept := 0, 0, 0
		for j, k := range b.forms {
			if prev[k] {
				kept++
			}
			if uprev[b.untyped[j]] {
				ukept++
			}
			if vprev[b.nominal[j]] {
				vkept++
			}
		}
		nkeptAll += vkept
		nprev := map[uint64]bool{}
		for _, k := range a.keys {
			nprev[k] = true
		}
		nkept := 0
		for _, k := range b.keys {
			if nprev[k] {
				nkept++
			}
		}
		keptAll += kept
		ukeptAll += ukept
		formsAll += len(b.forms)
		t.Logf("phase %3d: %5d forms, %5d kept their hash (%5.1f%%), %5d changed or new; untyped %5d kept (%5.1f%%); nominal %5d (%5.1f%%); nodes %6d, %6d with a hash q%03d holds (%5.1f%%); %d cyclic components, %d nodes, the largest %d; hashed in %v",
			b.n, len(b.forms), kept, 100*float64(kept)/float64(len(b.forms)), len(b.forms)-kept,
			ukept, 100*float64(ukept)/float64(len(b.forms)), vkept, 100*float64(vkept)/float64(len(b.forms)),
			b.nodes, nkept, a.n, 100*float64(nkept)/float64(b.nodes), b.cyclic, b.inCycles, b.largest, ms(b.hashTime))
	}
	t.Logf("phases 1-%d: %d forms, %d kept their hash (%.1f%%); untyped %d (%.1f%%); nominal %d (%.1f%%)",
		snaps[len(snaps)-1].n, formsAll, keptAll, 100*float64(keptAll)/float64(formsAll), ukeptAll, 100*float64(ukeptAll)/float64(formsAll),
		nkeptAll, 100*float64(nkeptAll)/float64(formsAll))
}

// sameHashesErr is sameHashes as an error.
func sameHashesErr(ga *Graph, a *Hashes, gb *Graph, b *Hashes) error {
	if len(ga.Forms) != len(gb.Forms) {
		return fmt.Errorf("%d forms against %d", len(ga.Forms), len(gb.Forms))
	}
	for i := range ga.Forms {
		var xs, ys []*Node
		Walk(ga.Forms[i], func(n *Node) bool { xs = append(xs, n); return true })
		Walk(gb.Forms[i], func(n *Node) bool { ys = append(ys, n); return true })
		if len(xs) != len(ys) {
			return fmt.Errorf("form %d: %d nodes against %d", i, len(xs), len(ys))
		}
		for j := range xs {
			hx, okx := a.Hash(xs[j])
			hy, oky := b.Hash(ys[j])
			if okx != oky || hx != hy {
				return fmt.Errorf("form %d: %s hashes %s against %s", i, label(xs[j]), hx.Short(), hy.Short())
			}
		}
	}
	return nil
}
