package treepilot

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/sweep"
	"github.com/arbace/go-whim/internal/whim"
)

// runs is TREEPILOT_RUNS, or the measurements are skipped.
func runs(t *testing.T) int {
	n, _ := strconv.Atoi(os.Getenv("TREEPILOT_RUNS"))
	if n <= 0 {
		t.Skip("TREEPILOT_RUNS is not a number of runs")
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

func ms(d time.Duration) string { return fmt.Sprintf("%d", d.Milliseconds()) }

func row(name string, c Cost) string {
	return fmt.Sprintf("| %-26s | %6s | %6s |", name, ms(c.Wall), ms(c.CPU))
}

// The A/B: each pilot phase as the plan runs it and with its tree steps on
// the tree, alternately, TREEPILOT_RUNS times each, one at a time; the
// median of each segment, wall and CPU in ms.  TREEPILOT_PROFILE=D writes a
// CPU profile of every run into D.
func TestMeasurePhases(t *testing.T) {
	dir, n := snaps(t), runs(t)
	prof := os.Getenv("TREEPILOT_PROFILE")
	for _, ph := range []int{8, 17, 18, 24} {
		p, _ := PhaseOf(ph)
		in, want := snap(t, dir, ph-1), snap(t, dir, ph)
		var text, tree []Times
		var tw, tt []Cost
		for i := 0; i < n; i++ {
			for _, which := range []string{"text", "tree"} {
				runtime.GC()
				var stop func()
				if prof != "" {
					f, err := os.Create(filepath.Join(prof, fmt.Sprintf("p%03d-%s-%d.prof", ph, which, i)))
					if err != nil {
						t.Fatal(err)
					}
					if err := pprof.StartCPUProfile(f); err != nil {
						t.Fatal(err)
					}
					stop = func() { pprof.StopCPUProfile(); f.Close() }
				}
				var out []byte
				var tm Times
				var err error
				if which == "text" {
					out, tm, err = RunText(p, in)
				} else {
					out, tm, err = RunTree(p, in, io.Discard)
				}
				if stop != nil {
					stop()
				}
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(out, want) {
					t.Fatalf("phase %d, %s: not q%03d", ph, which, ph)
				}
				if which == "text" {
					text, tw = append(text, tm), append(tw, tm.Total())
				} else {
					tree, tt = append(tree, tm), append(tt, tm.Total())
				}
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
		fmt.Fprintf(&b, "| %-26s | %6s | %6s |\n", "segment", "wall", "CPU")
		for _, s := range []struct {
			name string
			f    func(Times) Cost
		}{
			{"steps (text)", func(t Times) Cost { return t.Steps }},
			{"sweep", func(t Times) Cost { return t.Sweep }},
			{"canonical print", func(t Times) Cost { return t.Canon }},
		} {
			fmt.Fprintln(&b, row("text: "+s.name, seg(text, s.f)))
		}
		fmt.Fprintln(&b, row("text: total", median(tw)))
		for _, s := range []struct {
			name string
			f    func(Times) Cost
		}{
			{"other steps (text)", func(t Times) Cost { return t.Steps }},
			{"ToLisp", func(t Times) Cost { return t.ToLisp }},
			{"rewrite", func(t Times) Cost { return t.Rewrite }},
			{"Print", func(t Times) Cost { return t.Print }},
			{"sweep", func(t Times) Cost { return t.Sweep }},
			{"canonical print", func(t Times) Cost { return t.Canon }},
		} {
			fmt.Fprintln(&b, row("tree: "+s.name, seg(tree, s.f)))
		}
		fmt.Fprintln(&b, row("tree: total", median(tt)))
		t.Log(b.String())
	}
}

// Parse once: what each way of having the tree costs on a boundary, and what
// a phase's sweep and print cost on it -- TREEPILOT_RUNS times, medians.
func TestMeasureParse(t *testing.T) {
	dir, n := snaps(t), runs(t)
	for _, q := range []int{16, 23, 103} {
		src := snap(t, dir, q)
		path := filepath.Join(t.TempDir(), "whim-vim.c")
		fs, err := clisp.Forms(path, src)
		if err != nil {
			t.Fatal(err)
		}
		lc := clisp.Format(fs)
		segs := []struct {
			name string
			f    func()
		}{
			{"cc.Parse (cemit.Parse)", func() {
				if _, _, err := cemit.Parse(path, src); err != nil {
					t.Fatal(err)
				}
			}},
			{"ToLisp to forms (Forms)", func() {
				if _, err := clisp.Forms(path, src); err != nil {
					t.Fatal(err)
				}
			}},
			{"Read of the .lc", func() {
				if _, err := clisp.Read(lc); err != nil {
					t.Fatal(err)
				}
			}},
			{"Print (forms to C)", func() {
				if _, err := clisp.Print(fs); err != nil {
					t.Fatal(err)
				}
			}},
			{"Clone of the forms", func() {
				for _, f := range fs {
					clisp.Clone(f)
				}
			}},
			{"Walk of the forms", func() {
				k := 0
				clisp.Walk(clisp.Root(fs), func(*clisp.Cursor) bool { k++; return true }, nil)
			}},
			{"Resolve", func() { clisp.Resolve(fs) }},
			{"sweep (PruneParsed)", func() {
				if _, _, _, err := sweep.PruneParsed(src, path, whim.Profile.Sweep); err != nil {
					t.Fatal(err)
				}
			}},
			{"canonical print (parsed)", nil},
		}
		var ast *cc.AST
		if _, _, ast, err = sweep.PruneParsed(src, path, whim.Profile.Sweep); err != nil || ast == nil {
			t.Fatalf("no parse handed on: %v", err)
		}
		segs[len(segs)-1].f = func() {
			if _, err := cemit.CanonicalParsed(path, src, ast); err != nil {
				t.Fatal(err)
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "\nq%03d: %d bytes, %d lines; %d runs, load %s; median ms\n", q, len(src), bytes.Count(src, []byte("\n")), n, load())
		fmt.Fprintf(&b, "| %-26s | %6s | %6s |\n", "what", "wall", "CPU")
		for _, s := range segs {
			var cs []Cost
			for i := 0; i < n; i++ {
				runtime.GC()
				cs = append(cs, measure(s.f))
			}
			fmt.Fprintln(&b, row(s.name, median(cs)))
		}
		nodes, atoms := 0, 0
		clisp.Walk(clisp.Root(fs), func(c *clisp.Cursor) bool {
			nodes++
			if !c.Node().IsList() {
				atoms++
			}
			return true
		}, nil)
		sc := clisp.Resolve(fs)
		unres, amb := map[string]bool{}, 0
		for _, r := range sc.Refs {
			if r.Decl == nil {
				if r.Ambiguous {
					amb++
				} else {
					unres[r.Space.String()+":"+r.Atom.Atom] = true
				}
			}
		}
		var names []string
		for k := range unres {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "%d nodes (%d atoms); %d references, %d ambiguous members, %d opaque forms, unresolved: %s\n",
			nodes, atoms, len(sc.Refs), amb, len(sc.Opaque), strings.Join(names, " "))
		t.Log(b.String())
	}
}
