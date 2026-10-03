package graphcut

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// runs is GRAPHCUT_RUNS, or the measurements are skipped.
func runs(t *testing.T) int {
	n, _ := strconv.Atoi(os.Getenv("GRAPHCUT_RUNS"))
	if n <= 0 {
		t.Skip("GRAPHCUT_RUNS is not a number of runs")
	}
	return n
}

func load() string {
	b, _ := os.ReadFile("/proc/loadavg")
	f := strings.Fields(string(b))
	if len(f) < 3 {
		return "?"
	}
	return strings.Join(f[:3], " ")
}

func median(cs []Cost) Cost {
	ws := make([]time.Duration, len(cs))
	us := make([]time.Duration, len(cs))
	for i, c := range cs {
		ws[i], us[i] = c.Wall, c.CPU
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i] < ws[j] })
	sort.Slice(us, func(i, j int) bool { return us[i] < us[j] })
	return Cost{ws[len(ws)/2], us[len(us)/2]}
}

func row(name string, c Cost) string {
	return fmt.Sprintf("| %-30s | %6d | %6d |", name, c.Wall.Milliseconds(), c.CPU.Milliseconds())
}

// The A/B: each phase as the plan runs it (text), with its graph steps on a
// graph imported from q(N-1) (graph), and -- where its first step is a
// graph step -- on q(N-1)'s graph handed in, read from its Lisp (handed:
// the read timed apart, as what a pipeline on the graph would not pay);
// alternately, GRAPHCUT_RUNS times each, one at a time; medians, ms.
func TestMeasurePhases(t *testing.T) {
	dir, n := snaps(t), runs(t)
	for _, ph := range []int{8, 17, 18, 24} {
		p, _ := PhaseOf(ph)
		in, want := snap(t, dir, ph-1), snap(t, dir, ph)
		handed := false
		for _, m := range FirstOnGraph {
			handed = handed || m == ph
		}
		var lisp []byte
		if handed {
			g, _, err := graph.Import(t.TempDir()+"/whim-vim.c", in)
			if err != nil {
				t.Fatal(err)
			}
			lisp = g.Lisp()
		}
		times := map[string][]Times{}
		var reads []Cost
		for i := 0; i < n; i++ {
			for _, which := range []string{"text", "graph", "handed"} {
				if which == "handed" && !handed {
					continue
				}
				runtime.GC()
				var out []byte
				var tm Times
				var err error
				switch which {
				case "text":
					out, tm, err = RunText(p, in, io.Discard)
				case "graph":
					out, tm, err = RunGraph(p, in, nil, io.Discard)
				case "handed":
					var g *graph.Graph
					reads = append(reads, measure(func() { g, err = graph.Read(lisp) }))
					if err != nil {
						t.Fatal(err)
					}
					runtime.GC()
					out, tm, err = RunGraph(p, nil, g, io.Discard)
				}
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(out, want) {
					t.Fatalf("phase %d, %s: not q%03d", ph, which, ph)
				}
				times[which] = append(times[which], tm)
			}
		}
		seg := func(ts []Times, f func(Times) Cost) Cost {
			var cs []Cost
			for _, x := range ts {
				cs = append(cs, f(x))
			}
			return median(cs)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "\nphase %d, %d runs each, load %s; median ms\n", ph, n, load())
		fmt.Fprintf(&b, "| %-30s | %6s | %6s |\n", "segment", "wall", "CPU")
		segs := []struct {
			name string
			f    func(Times) Cost
		}{
			{"steps (text)", func(t Times) Cost { return t.Steps }},
			{"import", func(t Times) Cost { return t.Import }},
			{"index", func(t Times) Cost { return t.Index }},
			{"cut and closure", func(t Times) Cost { return t.Edit }},
			{"collection", func(t Times) Cost { return t.Collect }},
			{"C view", func(t Times) Cost { return t.Print }},
			{"sweep", func(t Times) Cost { return t.Sweep }},
			{"canonical print", func(t Times) Cost { return t.Canon }},
			{"graph's own (index to C view)", func(t Times) Cost {
				c := t.Index
				c.add(t.Edit)
				c.add(t.Collect)
				c.add(t.Print)
				return c
			}},
			{"total", func(t Times) Cost { return t.Total() }},
		}
		for _, which := range []string{"text", "graph", "handed"} {
			ts := times[which]
			if len(ts) == 0 {
				continue
			}
			if which == "handed" {
				fmt.Fprintln(&b, row("handed: the read (apart)", median(reads)))
			}
			for _, s := range segs {
				c := seg(ts, s.f)
				if c.Wall == 0 && c.CPU == 0 {
					continue
				}
				fmt.Fprintln(&b, row(which+": "+s.name, c))
			}
		}
		t.Log(b.String())
	}
}
