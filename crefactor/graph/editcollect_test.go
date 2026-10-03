package graph

import (
	"bytes"
	"testing"
)

// The collection through the editor is the collection: on every sample,
// read back from its Lisp, Editor.Collect leaves the C view Collect leaves,
// the editor's index whole (Check), nothing it cut Live, and one act logged
// with the ids it superseded.
func TestEditorCollect(t *testing.T) {
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			_, _, g := importSample(t, src)
			lisp := g.Lisp()
			a, err := Read(lisp)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := Read(lisp)
			sa, err := Collect(a, testCollect)
			if err != nil {
				t.Fatal(err)
			}
			e := NewEditor(b)
			before := map[ID]*Node{}
			b.Walk(func(n *Node) bool {
				if n.ID != 0 {
					before[n.ID] = n
				}
				return true
			})
			sb, err := e.Collect(testCollect)
			if err != nil {
				t.Fatal(err)
			}
			ca, _ := a.C()
			cb, _ := b.C()
			if !bytes.Equal(ca, cb) || sa != sb {
				t.Fatalf("through the editor:\n%s\n%v\nalone:\n%s\n%v", cb, sb, ca, sa)
			}
			if err := e.Check(); err != nil {
				t.Fatalf("the index after the collection: %v", err)
			}
			act := e.Log[len(e.Log)-1]
			if act.Op != "collect" {
				t.Fatalf("the act logged is %q", act.Op)
			}
			gone := 0
			for id, n := range before {
				live := e.Live(n)
				if !live {
					gone++
				}
				if _, held := b.Index()[id]; held != live {
					t.Fatalf("#%d: Live %v, in the graph %v", id, live, held)
				}
			}
			if gone != len(act.Gone) {
				t.Fatalf("%d nodes went, the act says %d", gone, len(act.Gone))
			}
		})
	}
}

// Edit, collect, edit again: the second edit sees the collected graph
// through the same index.
func TestEditCollectEdit(t *testing.T) {
	_, g, e := editGraph(t)
	for _, d := range e.FileDecls("nothing") {
		if err := e.Delete(d); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.FallOut(FallOutOptions{KeepEmpty: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Collect(testCollect); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	// counter: its one store goes with it
	if err := e.Delete(e.FileDecls("counter")[0]); err != nil {
		t.Fatal(err)
	}
	st, err := e.FallOut(FallOutOptions{KeepEmpty: true})
	if err != nil || len(st.Removed) != 1 {
		t.Fatalf("%v: %d removed", err, len(st.Removed))
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if out, _ := g.C(); bytes.Contains(out, []byte("counter")) || bytes.Contains(out, []byte("nothing")) {
		t.Fatalf("left:\n%s", out)
	}
}
