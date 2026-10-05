package graph

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestMeasureStore holds the store of graphs a run kept (GRAPH_STORE, its
// .cache/boundaries) to the qNNN.g files a run of the same plan kept before
// the store (GRAPH_G): each graph read back is the .g byte for byte, so
// the same graph (Equal), its C view the same; and it times the two reads,
// one at a time, 5 runs, medians -- the bytes alone, and the bytes and
// Read, which is what the parallel check pays for a link.
func TestMeasureStore(t *testing.T) {
	store, gdir := os.Getenv("GRAPH_STORE"), os.Getenv("GRAPH_G")
	if store == "" || gdir == "" {
		t.Skip("GRAPH_STORE and GRAPH_G are not set")
	}
	gs, _ := filepath.Glob(filepath.Join(gdir, "q*.g"))
	if len(gs) == 0 {
		t.Fatalf("no .g files in %s", gdir)
	}
	median := func(f func()) time.Duration {
		var ds []time.Duration
		for range 5 {
			start := time.Now()
			f()
			ds = append(ds, time.Since(start))
		}
		slices.Sort(ds)
		return ds[2]
	}
	var sumG, sumS, sumGR, sumSR time.Duration
	var gbytes int64
	for _, f := range gs {
		name := filepath.Base(f[:len(f)-2])
		old, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		gbytes += int64(len(old))
		b, err := ReadStoreLisp(store, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(b, old) {
			t.Fatalf("%s: the store's Lisp is not %s's", name, f)
		}
		g1, err := Read(old)
		if err != nil {
			t.Fatal(err)
		}
		g2, err := Read(b)
		if err != nil {
			t.Fatal(err)
		}
		if err := Equal(g1, g2); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		c1, err1 := g1.C()
		c2, err2 := g2.C()
		if err1 != nil || err2 != nil || !bytes.Equal(c1, c2) {
			t.Fatalf("%s: the C views differ (%v, %v)", name, err1, err2)
		}
		dg := median(func() { os.ReadFile(f) })
		ds := median(func() { ReadStoreLisp(store, name) })
		dgr := median(func() { b, _ := os.ReadFile(f); Read(b) })
		dsr := median(func() { b, _ := ReadStoreLisp(store, name); Read(b) })
		sumG, sumS, sumGR, sumSR = sumG+dg, sumS+ds, sumGR+dgr, sumSR+dsr
		t.Logf("%s: %d bytes; the bytes %v from .g, %v from the store; with Read %v, %v", name, len(old), dg, ds, dgr, dsr)
	}
	n := time.Duration(len(gs))
	var disk int64
	ents, _ := os.ReadDir(store)
	for _, e := range ents {
		if filepath.Ext(e.Name()) == ".gm" || e.Name() == storePack || e.Name() == storeIdx {
			if i, err := e.Info(); err == nil {
				disk += i.Size()
			}
		}
	}
	fmt.Printf("%d graphs, the same as their .g byte for byte; .g %d bytes, the store %d (%.1f%%); mean read: the bytes %v from .g, %v from the store; with Read %v, %v\n",
		len(gs), gbytes, disk, 100*float64(disk)/float64(gbytes), sumG/n, sumS/n, sumGR/n, sumSR/n)
}
