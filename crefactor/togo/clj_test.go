package togo

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The Clojure backend on C that is not vim: each program's functions are
// lowered and written as one namespace, whim.editor, which is loaded with the
// runtime (jeditor/rt) by clojure.main -- *warn-on-reflection* on, a
// reflection warning a failure -- and run; it must print what the gcc-built
// C prints.  The C calls host functions it only declares -- out(v) prints a
// number, outs(s) a string, outf(fmt, ...) a printf -- which the C harness
// defines with printf and the Clojure one, whim.cljhost, as functions of the
// editor and the C's arguments: the host behind a line, as the editor's is.

// cljHarness is whim.cljhost for the tests: out, outs and a printf of d, u,
// x, c, s and l, reading the arguments as the backend boxes them -- an int
// an Integer, an unsigned int or a long a Long, a pointer its class.
const cljHarness = `(ns whim.cljhost
  (:import [whim.rt BytePtr]))

(set! *warn-on-reflection* true)

(defn out [ed v] (println (long v)) nil)

(defn outs [ed ^BytePtr s] (println (BytePtr/str s)) nil)

(defn outf [ed ^BytePtr fmt ^objects args]
  (let [f (BytePtr/str fmt)
        b (StringBuilder.)
        n (count f)]
    (loop [i 0 k 0]
      (if (>= i n)
        (do (when (not= k (alength args))
              (throw (IllegalArgumentException. (str "unused arguments: " f))))
            (println (str b)))
        (let [c (.charAt f i)]
          (if (not= c \%)
            (do (.append b c) (recur (inc i) k))
            (let [[l i] (loop [l 0 i (inc i)] (if (= (.charAt f i) \l) (recur (inc l) (inc i)) [l i]))
                  conv (.charAt f (int i))
                  a (when (not= conv \%) (aget args k))
                  v (if (or (= conv \s) (= conv \%)) 0 (.longValue ^Number a))]
              (case conv
                \d (.append b (if (pos? l) v (long (unchecked-int v))))
                \u (.append b (if (pos? l) (Long/toUnsignedString v) (Integer/toUnsignedString (unchecked-int v))))
                \x (.append b (if (pos? l) (Long/toHexString v) (Integer/toHexString (unchecked-int v))))
                \c (.append b (char (bit-and v 0xff)))
                \s (.append b (BytePtr/str ^BytePtr a))
                \% (.append b \%))
              (recur (inc (long i)) (if (= conv \%) k (inc k))))))))
    nil))
`

var cljTools struct {
	once      sync.Once
	cp, skip  string
	classes   string
	clojureCP string
}

// requireClj skips without gcc, javac, java or clojure; it compiles the
// runtime once and returns the class path the tests load whim.editor with.
func requireClj(t *testing.T) string {
	for _, tool := range []string{"gcc", "javac", "java", "clojure"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	cljTools.once.Do(func() {
		dir, err := os.MkdirTemp("", "cljrt.")
		if err != nil {
			cljTools.skip = err.Error()
			return
		}
		rt, _ := filepath.Glob(filepath.Join(runtimeDir, "*.java"))
		if o, err := exec.Command("javac", append([]string{"-nowarn", "-d", dir}, rt...)...).CombinedOutput(); err != nil {
			cljTools.skip = "javac: " + string(o)
			return
		}
		cp, err := bounded(60*time.Second, "", "clojure", "-Spath")
		if err != nil {
			cljTools.skip = "clojure -Spath: " + err.Error()
			return
		}
		cljTools.classes = dir
		cljTools.clojureCP = strings.TrimSpace(string(cp))
	})
	if cljTools.skip != "" {
		t.Skip(cljTools.skip)
	}
	return cljTools.classes + string(os.PathListSeparator) + cljTools.clojureCP
}

// bounded runs a program that may loop for ever: it dies after d, or with
// the test process (Pdeathsig), and returns its stdout; stderr, when it
// fails, is in the error.
func bounded(d time.Duration, stdin string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		return out, &runError{err, stderr.String()}
	}
	if stderr.Len() > 0 {
		return out, &runError{nil, stderr.String()}
	}
	return out, nil
}

type runError struct {
	err    error
	stderr string
}

func (e *runError) Error() string {
	if e.err == nil {
		return "stderr: " + e.stderr
	}
	return e.err.Error() + "\n" + e.stderr
}

// cljProgram translates src to whim/editor.clj under dir and returns it,
// with the refusals.
func cljProgram(t *testing.T, dir, src string, prof Profile) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "whim"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "src", "whim", "editor.clj")
	if rc := Run([]string{c, dir, "-clj", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// cljOutput loads prog (whim.editor) with the harness, runs the C's run on
// a new editor and returns what it prints; a reflection warning fails.
func cljOutput(t *testing.T, cp, dir, prog string) string {
	src := filepath.Join(dir, "clj")
	os.RemoveAll(src)
	if err := os.MkdirAll(filepath.Join(src, "whim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "whim", "editor.clj"), []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "whim", "cljhost.clj"), []byte(cljHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := bounded(120*time.Second, "", "java", "-cp", src+string(os.PathListSeparator)+cp, "clojure.main", "-e",
		"(require 'whim.editor) (whim.editor/run (whim.editor/new-editor nil))")
	if err != nil {
		t.Fatalf("clojure: %v\n%s", err, numbered(prog))
	}
	return string(out)
}

// cljSame translates src, requires every function written, and requires the
// Clojure to print what the C prints; it returns the Clojure.
func cljSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	cp := requireClj(t)
	dir := t.TempDir()
	prog, refused := cljProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := cljOutput(t, cp, dir, prog); got != want {
		t.Errorf("the Clojure prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

func TestCljIntegers(t *testing.T)  { cljSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestCljFlow(t *testing.T)      { cljSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestCljStrings(t *testing.T)   { cljSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestCljStructs(t *testing.T)   { cljSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestCljExtra(t *testing.T)     { cljSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestCljVarargs(t *testing.T)   { cljSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestCljPointers(t *testing.T)  { cljSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestCljGoto(t *testing.T)      { cljSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestCljDeadLabel(t *testing.T) { cljSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }

var cljProfileProf = Profile{Allocators: []string{"alloc"}, Frees: []string{"vim_free"},
	Bytes: ByteFuncs{Move: []string{"memmove"}, Set: "memset", Cmp: "memcmp"}}

func TestCljProfile(t *testing.T) {
	cljSame(t, javaProfileC, cljProfileProf, "#include <stdlib.h>\nvoid *alloc(unsigned long n) { return calloc(1, n); }\nvoid vim_free(void *p) { free(p); }\n"+javaHarnessC)
}

// cljGrowProfile is javaGrowProfile with ga_grow_inner's Clojure body.
var cljGrowProfile = Profile{Allocators: []string{"alloc"}, Frees: []string{"vim_free"},
	Bytes:     ByteFuncs{Move: []string{"memmove"}, Set: "memset", Cmp: "memcmp"},
	GrowArray: GrowArray{Data: "ga_data", MaxLen: "ga_maxlen"},
	RuntimeBodies: []RuntimeBody{{Name: "ga_grow_inner", Clj: func(string) string {
		return "(let [n (if (< n (.ga_growsize gap)) (.ga_growsize gap) n)]\n  (.set_ga_maxlen gap (+ (.ga_len gap) n))\n  1)"
	}}}}

func TestCljGrowArray(t *testing.T) {
	cljSame(t, javaGrowC, cljGrowProfile, javaGrowHarnessC)
}

// cljMatch fails unless every pattern is in prog.
func cljMatch(t *testing.T, prog string, pats ...string) {
	for _, p := range pats {
		if !regexp.MustCompile(p).MatchString(prog) {
			t.Errorf("no %q in:\n%s", p, numbered(prog))
		}
	}
}
