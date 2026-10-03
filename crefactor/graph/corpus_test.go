package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
)

// GRAPH_CORPUS is a glob of canonical texts -- the pipeline's boundaries,
// `go tool whim build --keep D` writes them -- on which steps 1 and 2 are
// held, each file a subtest (-parallel N runs N at once): the C view of
// the imported graph is the text byte for byte; the graph written as Lisp
// and read back is the same graph, ids and edges, and its C view is the
// text again.  Without it the corpus tests skip.
func corpus(t *testing.T) []string {
	glob := os.Getenv("GRAPH_CORPUS")
	if glob == "" {
		t.Skip("GRAPH_CORPUS is not set")
	}
	files, err := filepath.Glob(glob)
	if err != nil || len(files) == 0 {
		t.Fatalf("GRAPH_CORPUS=%s: no files (%v)", glob, err)
	}
	return files
}

func TestCorpus(t *testing.T) {
	for _, f := range corpus(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			g, rep, err := Import(f, src)
			if err != nil {
				t.Fatal(err)
			}
			out, err := g.C()
			if err != nil {
				t.Fatalf("C view: %v", err)
			}
			if !bytes.Equal(out, src) {
				t.Fatalf("step 1: the C view is not the text: %s", firstDiff(out, src))
			}
			text := g.Lisp()
			h, err := Read(text)
			if err != nil {
				t.Fatal(err)
			}
			if err := Equal(g, h); err != nil {
				t.Fatalf("step 2: the graph read back is not the graph: %v", err)
			}
			if out, err = h.C(); err != nil {
				t.Fatalf("C view of the graph read: %v", err)
			}
			if !bytes.Equal(out, src) {
				t.Fatalf("step 2: the C view of the graph read back is not the text: %s", firstDiff(out, src))
			}
			t.Logf("byte for byte, imported and read back; %d bytes of Lisp\n%s", len(text), rep)
		})
	}
}

// TestCorpusTimes (GRAPH_TIMES=1) times, one file at a time, the best of
// three: cc's parse and type check (Translate's two halves, cemit.Parse and
// Check), the import, and the read of the Lisp -- step 2's gate is the
// read under a tenth of the parse and check.
func TestCorpusTimes(t *testing.T) {
	if os.Getenv("GRAPH_TIMES") == "" {
		t.Skip("GRAPH_TIMES is not set")
	}
	for _, f := range corpus(t) {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		abs, _ := filepath.Abs(f)
		cfg, err := cc.NewConfig("linux", "amd64")
		if err != nil {
			t.Fatal(err)
		}
		parse := best(3, func() {
			ast, _, err := cemit.Parse(abs, src)
			if err != nil {
				t.Fatal(err)
			}
			if err := ast.Check(cfg); err != nil {
				t.Fatal(err)
			}
		})
		var g *Graph
		imp := best(3, func() {
			if g, _, err = Import(f, src); err != nil {
				t.Fatal(err)
			}
		})
		text := g.Lisp()
		read := best(3, func() {
			if _, err := Read(text); err != nil {
				t.Fatal(err)
			}
		})
		ratio := float64(parse) / float64(read)
		t.Logf("%s: %d bytes of C, %d of Lisp; parse and check %v, import %v, read %v: %.1f times faster",
			filepath.Base(f), len(src), len(text), ms(parse), ms(imp), ms(read), ratio)
		if ratio < 10 {
			t.Errorf("%s: the read is not under a tenth of the parse and check", filepath.Base(f))
		}
	}
}

func best(n int, f func()) time.Duration {
	var b time.Duration
	for i := 0; i < n; i++ {
		runtime.GC()
		start := time.Now()
		f()
		if d := time.Since(start); i == 0 || d < b {
			b = d
		}
	}
	return b
}

func ms(d time.Duration) time.Duration { return d.Round(time.Millisecond) }

// firstDiff says where two texts part.
func firstDiff(a, b []byte) string {
	line := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			s := bytes.LastIndexByte(a[:i], '\n') + 1
			ea := bytes.IndexByte(a[i:], '\n')
			eb := bytes.IndexByte(b[i:], '\n')
			if ea < 0 {
				ea = len(a) - i
			}
			if eb < 0 {
				eb = len(b) - i
			}
			return "line " + itoa(line) + ": got `" + string(a[s:i+ea]) + "`, want `" + string(b[s:i+eb]) + "`"
		}
		if a[i] == '\n' {
			line++
		}
	}
	return "lengths " + itoa(len(a)) + " and " + itoa(len(b))
}

func itoa(n int) string { return string(appendInt(nil, n)) }

func appendInt(b []byte, n int) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	var d [20]byte
	i := len(d)
	for {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	return append(b, d[i:]...)
}

// TestCorpusMembersByType (GRAPH_CORPUS) measures what the graph's own
// member rule would take that the sweep's keeps: each snapshot is swept
// already, so Collect by Prune's rules takes nothing, and by type takes
// the members no live use's edge names -- named by a use of another
// struct's member of the name.
func TestCorpusMembersByType(t *testing.T) {
	for _, f := range corpus(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := Import(f, src)
			if err != nil {
				t.Fatal(err)
			}
			before := map[*Node]string{}
			g.Walk(func(n *Node) bool {
				if (n.Is("struct") || n.Is("union")) && isDefForm(n) {
					for _, m := range members(n) {
						before[m] = tagOf(n) + "." + memberDeclName(m)
					}
				}
				return true
			})
			opt := CollectOptions{Roots: []string{"main"}, FreezeLayoutIf: []string{"ml_recover"}, MembersByType: true}
			st, err := Collect(g, opt)
			if err != nil {
				t.Fatal(err)
			}
			after := map[*Node]bool{}
			g.Walk(func(n *Node) bool { after[n] = true; return true })
			var gone []string
			for m, name := range before {
				if !after[m] {
					gone = append(gone, name)
				}
			}
			sort.Strings(gone)
			t.Logf("by type: %d members, %d locals, %d functions more: %v", st.Members, st.Locals, st.Funcs, gone)
		})
	}
}
