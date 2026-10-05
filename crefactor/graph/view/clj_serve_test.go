package view

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestClojureServer holds `clj/view-clj --serve` (doc/GRAPH.md, *The
// server*) to the Go views as TestClojureViews holds --batch: one server,
// started once on whim-vim.c's graph written as EDN, is sent every case of
// TestClojureViews as a request line and must answer each with the bytes
// `whim view` prints, or its error.  Then the same requests again, answered
// from the server's cache, and a reload, after which a view is the same
// still.  It logs each request's latency, median and p99, beside the Go
// views' in process and a one-shot run of view-clj.  It needs java and
// Clojure's jars, and skips without them.
func TestClojureServer(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	needClojure(t)
	ix, _ := product(t)
	edn, err := ix.G.EDN()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ednFile := filepath.Join(dir, "graph.edn")
	if err := os.WriteFile(ednFile, edn, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := clojureCases(ix)

	start := time.Now()
	s := startServer(t, dir, ednFile)
	if _, _, err := s.ask("ping"); err != nil {
		t.Fatal(err)
	}
	t.Logf("server up, the graph read and indexed, in %v", time.Since(start).Round(time.Millisecond))

	var goLat, cold, warm []time.Duration
	wants := make([][2]string, len(cases))
	for i, c := range cases {
		t0 := time.Now()
		w, we := goView(ix, c)
		goLat = append(goLat, time.Since(t0))
		wants[i] = [2]string{w, we}
	}
	bad := 0
	for pass, lat := range []*[]time.Duration{&cold, &warm} {
		for i, c := range cases {
			t0 := time.Now()
			kind, body, err := s.ask(strings.Join(c, "\t"))
			*lat = append(*lat, time.Since(t0))
			if err != nil {
				t.Fatalf("%q: %v", c, err)
			}
			got, gotErr := body, ""
			if kind == "error" {
				got, gotErr = "", strings.TrimSuffix(body, "\n")
			}
			if got != wants[i][0] || gotErr != wants[i][1] {
				bad++
				if bad <= 5 {
					t.Errorf("pass %d, %q: the server's answer differs (%d bytes against %d; error %q against %q)", pass+1, c, len(got), len(wants[i][0]), gotErr, wants[i][1])
				}
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d answers differ", bad, 2*len(cases))
	}
	t.Logf("%d of %d views the same bytes, twice (the second time from the cache)", len(cases), len(cases))

	_, stats, _ := s.ask("stats")
	t.Logf("stats:\n%s", stats)
	t0 := time.Now()
	if kind, body, err := s.ask("reload"); err != nil || kind != "ok" || !strings.Contains(body, " 0 views") {
		t.Fatalf("reload: %s %q %v", kind, body, err)
	}
	reload := time.Since(t0)
	if kind, body, err := s.ask(strings.Join(cases[0], "\t")); err != nil || kind != "ok" || body != wants[0][0] {
		t.Fatalf("%q after reload: %s, %d bytes against %d, %v", cases[0], kind, len(body), len(wants[0][0]), err)
	}
	if kind, _, _ := s.ask("no_such_view x"); kind != "error" {
		t.Fatalf("a bad request answered %s", kind)
	}
	s.stop(t)

	// one-shot runs of the CLI, a JVM each: a few cases suffice
	var shot []time.Duration
	for _, c := range [][]string{{"uses", "p_wiv"}, {"callers", "ml_get"}, {"type", "pos_T"}} {
		t0 := time.Now()
		cmd := exec.Command("clj/view-clj", append(c, ednFile)...)
		cmd.Env = append(os.Environ(), "TMPDIR="+dir)
		if b, err := cmd.Output(); err != nil {
			t.Fatalf("view-clj %q: %v", c, err)
		} else if w, _ := goView(ix, c); string(b) != w {
			t.Fatalf("view-clj %q: one-shot output differs", c)
		}
		shot = append(shot, time.Since(t0))
	}
	t.Logf("latency a request (median, p99, max) over %d views:", len(cases))
	t.Logf("  the Go views in process   %s", spread(goLat))
	t.Logf("  the server, built         %s", spread(cold))
	t.Logf("  the server, cached        %s", spread(warm))
	t.Logf("  view-clj one-shot         %s (of %d runs)", spread(shot), len(shot))
	t.Logf("  reload                    %v", reload.Round(time.Millisecond))
}

// needClojure skips a test without java or Clojure's jar.
func needClojure(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("no java")
	}
	if os.Getenv("GRAPH_CLOJURE_CP") == "" {
		home, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(home, ".m2/repository/org/clojure/clojure/1.12.5/clojure-1.12.5.jar")); err != nil {
			t.Skip("no Clojure jar: GRAPH_CLOJURE_CP is not set and ~/.m2 has none")
		}
	}
}

// clojureCases are TestClojureViews' cases: a fixed set, and for every name
// the file declares `uses`, for every function `def` and `callers` to depth
// 1, and `type` of every struct or union a typedef names.
func clojureCases(ix *Index) [][]string {
	cases := [][]string{
		{"uses", "p_wiv"}, {"--ids", "uses", "p_wiv"}, {"callers", "ml_get"}, {"--depth", "0", "callers", "ml_get"},
		{"--ids", "callers", "ml_get_buf"}, {"callees", "main"}, {"--depth", "0", "callees", "main"},
		{"member", "buf_T.b_ml"}, {"member", "pos_T.lnum"}, {"--ids", "member", "memline_T.ml_line_count"},
		{"type", "pos_T"}, {"type", "struct vimoption"}, {"--ids", "def", "ex_substitute"}, {"def", "options"},
		{"uses", "NUL"}, {"--show", "fn", "uses", "ml_get"}, {"--show", "node", "uses", "p_wiv"},
		{"--show", "none", "callers", "ml_get"}, {"uses", "ex_substitute/lnum"}, {"--stop", "defn", "callers", "ml_get"},
		{"follow", "refers< call ^fn", "ml_get"}, {"--depth", "3", "follow", "typed<", "pos_T"},
		{"uses", "no_such_name"}, {"member", "ml_get"},
	}
	var names []string
	for name := range ix.top {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cases = append(cases, []string{"uses", name})
		for _, d := range ix.top[name] {
			switch {
			case d.Is("defn"):
				cases = append(cases, []string{"def", name}, []string{"--depth", "1", "callers", name})
			case d.Is("typedef"):
				if s, err := ix.Aggregate(name); err == nil && (s.Is("struct") || s.Is("union")) {
					cases = append(cases, []string{"type", name})
				}
			}
		}
	}
	return cases
}

// server is a running `view-clj --serve`.
type server struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func startServer(t *testing.T, dir, ednFile string) *server {
	t.Helper()
	cmd := exec.Command("clj/view-clj", "--serve", ednFile)
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &server{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return s
}

// ask sends one request and reads its answer: "ok" or "error", and the body.
func (s *server) ask(req string) (string, string, error) {
	if _, err := io.WriteString(s.in, req+"\n"); err != nil {
		return "", "", err
	}
	head, err := s.out.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("reading the header: %v", err)
	}
	kind, n, ok := strings.Cut(strings.TrimSuffix(head, "\n"), " ")
	size, err := strconv.Atoi(n)
	if !ok || err != nil || (kind != "ok" && kind != "error") {
		return "", "", fmt.Errorf("a bad header %q", head)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(s.out, body); err != nil {
		return "", "", err
	}
	end, err := s.out.ReadString('\n')
	if err != nil || (end != ";;end\n" && end != "\n") {
		return "", "", fmt.Errorf("no end line: %q %v", end, err)
	}
	if end == "\n" {
		if end, err = s.out.ReadString('\n'); err != nil || end != ";;end\n" {
			return "", "", fmt.Errorf("no end line: %q %v", end, err)
		}
	}
	return kind, string(body), nil
}

func (s *server) stop(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(s.in, "quit\n"); err != nil {
		t.Fatal(err)
	}
	s.in.Close()
	if err := s.cmd.Wait(); err != nil {
		t.Fatalf("the server's exit: %v", err)
	}
}

// spread is a sample's median, p99 and maximum.
func spread(d []time.Duration) string {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	q := func(p float64) time.Duration { return s[int(p*float64(len(s)-1))] }
	r := func(x time.Duration) string {
		switch {
		case x >= time.Second:
			return x.Round(time.Millisecond).String()
		case x >= time.Millisecond:
			return x.Round(10 * time.Microsecond).String()
		}
		return x.Round(time.Microsecond).String()
	}
	return fmt.Sprintf("%s, %s, %s", r(q(0.5)), r(q(0.99)), r(s[len(s)-1]))
}
