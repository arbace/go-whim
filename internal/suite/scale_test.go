package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// scaleText is the eight lines the :%s is timed over, typed in Insert
// mode: prose, code, SQL, a log line, the backtracking line, an indented
// one (doc/PARALLEL-SUBSTITUTE.md's kinds, these words ours).
var scaleText = []string{
	"the quick brown fox jumps over the lazy dog",
	"for (i = 0; i < n; i++) { total += a[i] * b[i]; }",
	"SELECT name, count(*) FROM users WHERE age > 30 GROUP BY name;",
	"2026-10-05 01:23:45 INFO server started on port 8080",
	"abab abc bab cab, abba acdc cabbage",
	"    an indented line with tabs and spaces",
	"Lorem ipsum dolor sit amet, consectetur adipiscing elit",
	"func main() { fmt.Println(\"hello, world\") }",
}

// scalePattern is the :%s timed: alternation under repetition,
// backtracking at every a and b.
const scalePattern = `:%s/\v(a|b)+c/X/g` + "\r"

// scaleKeys builds lines lines (a multiple of 8) and, with sub, runs the
// :%s over them, then :q!.
func scaleKeys(lines int, sub bool) []byte {
	k := "i" + strings.Join(scaleText, "\r") + fmt.Sprintf("\x1bggVGy%dP", lines/len(scaleText)-1)
	if sub {
		k += scalePattern
	}
	return []byte(k + ":q!\r")
}

// TestGuestScale (doc/GUEST.md, *SMP*; the Mac's: guest/mac/mac.sh scale)
// times the Go guest's parallel :%s on several vCPUs against the C:
// WHIM_SUITE_SCALE the vCPU counts ("1,2,4,8"), WHIM_SUITE_C,
// WHIM_SUITE_GUEST and WHIM_SUITE_GUEST_IMAGE as TestGuestPrebuilt has
// them, WHIM_SUITE_SCALE_LINES the lines (200000), WHIM_SUITE_SCALE_RUNS the
// runs of each session (5).  Each editor runs the session that builds the
// lines and the same session with the :%s, alternately, keys from a file;
// the :%s's time is the difference of the two medians.  Every guest run is
// held to the C's answer, so a count of vCPUs that changes it fails.
func TestGuestScale(t *testing.T) {
	cpus, c, g := os.Getenv("WHIM_SUITE_SCALE"), os.Getenv("WHIM_SUITE_C"), os.Getenv("WHIM_SUITE_GUEST")
	if cpus == "" || c == "" || g == "" {
		t.Skip("WHIM_SUITE_SCALE, WHIM_SUITE_C and WHIM_SUITE_GUEST say what to time")
	}
	num := func(name string, def int) int {
		if s := os.Getenv(name); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 {
				t.Fatalf("%s=%s: not a positive count", name, s)
			}
			return n
		}
		return def
	}
	lines, runs := num("WHIM_SUITE_SCALE_LINES", 200000), num("WHIM_SUITE_SCALE_RUNS", 5)
	lines -= lines % len(scaleText)
	if lines < len(scaleText) {
		t.Fatal("WHIM_SUITE_SCALE_LINES: fewer than 8")
	}
	var ns []int
	for _, f := range strings.Split(cpus, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 {
			t.Fatalf("WHIM_SUITE_SCALE=%s: not a list of vCPU counts", cpus)
		}
		ns = append(ns, n)
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	c, g = abs(c), abs(g)
	if img := os.Getenv("WHIM_SUITE_GUEST_IMAGE"); img != "" {
		var err error
		if g, err = imageScript(t.TempDir(), "whim-guest", g, abs(img)); err != nil {
			t.Fatal(err)
		}
	}
	const limit = 10 * time.Minute
	base, sub := scaleKeys(lines, false), scaleKeys(lines, true)
	var want []byte
	wantCode := 0
	// median is the median wall times of runs sessions with the :%s and
	// runs without, alternately, the answer with it held to the C's.
	median := func(name, bin string, env []string) (with, without time.Duration) {
		var tw, to []time.Duration
		for i := range 2 * runs {
			keys := sub
			if i%2 == 1 {
				keys = base
			}
			start := time.Now()
			out, code, err := runEnv(bin, nil, keys, limit, env)
			d := time.Since(start)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if i%2 == 1 {
				to = append(to, d)
				continue
			}
			tw = append(tw, d)
			if want == nil {
				want, wantCode = out, code
				if !bytes.Contains(out, []byte("substitution")) {
					t.Fatalf("%s: no substitution reported: the keys never reached the :%%s", name)
				}
			} else if code != wantCode || !bytes.Equal(out, want) {
				t.Fatalf("%s answers the :%%s differently from the C (exit %d, %d bytes; the C's %d, %d)", name, code, len(out), wantCode, len(want))
			}
		}
		slices.Sort(tw)
		slices.Sort(to)
		return tw[runs/2], to[runs/2]
	}
	type row struct {
		name          string
		with, without time.Duration
	}
	rows := []row{{name: "C"}}
	rows[0].with, rows[0].without = median("the C", c, nil)
	for _, n := range ns {
		r := row{name: fmt.Sprintf("Go guest, %d vCPU", n)}
		if n > 1 {
			r.name += "s"
		}
		r.with, r.without = median(r.name, g, []string{"WHIM_GUEST_CPUS=" + strconv.Itoa(n)})
		rows = append(rows, r)
	}
	ms := func(d time.Duration) string { return fmt.Sprintf("%d", d.Milliseconds()) }
	var b strings.Builder
	fmt.Fprintf(&b, "%s over %d lines, keys from a file, the median of %d runs (ms); :%%s is the session less the same session without it\n",
		strings.TrimSuffix(scalePattern, "\r"), lines, runs)
	fmt.Fprintf(&b, "| editor | session | without | :%%s | vs 1 vCPU | vs the C |\n|---|---|---|---|---|---|\n")
	cs := rows[0].with - rows[0].without
	var one time.Duration
	for _, r := range rows {
		s := r.with - r.without
		vs1, vsC := "", ""
		if r.name != "C" {
			if one == 0 {
				one = s
			}
			vs1 = fmt.Sprintf("%.2fx", float64(one)/float64(s))
			vsC = fmt.Sprintf("%.2fx", float64(s)/float64(cs))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", r.name, ms(r.with), ms(r.without), ms(s), vs1, vsC)
	}
	fmt.Fprintf(&b, "(vs 1 vCPU: the first guest row's :%%s over this one's, the speedup; vs the C: this one's over the C's)\n")
	t.Log("\n" + b.String())
}
