// Package suite is the minimal behaviour check: a corpus of editing sessions
// (cases.md) run on two builds of the editor, which must answer each one with
// the same bytes and the same exit status.
//
// THE ORACLE IS THE COMMITTED PRODUCT, not a stored recording: the reference is
// src/whim-vim.c at a git revision, built with the one compile line, so nothing
// is kept that could go stale, and a run measures exactly what a change did to
// what the editor does.  The editor needs no terminal -- keystrokes in on
// stdin, a 24x80 screen out on stdout as escape sequences -- and, measured, it
// answers byte for byte the same across runs and across rebuilds.
//
// A CHECK THAT CANNOT FAIL IS NOT ONE.  Every run also builds a CONTROL: the
// candidate with one string it prints changed, which at least one case must
// see.  A run whose control goes unseen fails, whatever the cases said.
package suite

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/arbace/go-whim/cljeditor"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/braaam"
)

//go:embed cases.md
var casesMD string

// A Case is one editing session.
type Case struct {
	Name string
	Keys []byte
}

// Cases reads the fenced block of cases.md: one `name<TAB>keys` per line.
func Cases() ([]Case, error) {
	i := strings.Index(casesMD, "```\n")
	j := strings.LastIndex(casesMD, "```")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("suite: cases.md has no fenced block")
	}
	var out []Case
	for _, l := range strings.Split(casesMD[i+4:j], "\n") {
		if l == "" {
			continue
		}
		name, keys, ok := strings.Cut(l, "\t")
		if !ok {
			return nil, fmt.Errorf("suite: %q has no tab", l)
		}
		k, err := unescape(keys)
		if err != nil {
			return nil, fmt.Errorf("suite: %s: %w", name, err)
		}
		out = append(out, Case{name, k})
	}
	return out, nil
}

// unescape reads \e \r \n \t \\ and \xHH.
func unescape(s string) ([]byte, error) {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b = append(b, s[i])
			continue
		}
		i++
		switch s[i] {
		case 'e':
			b = append(b, 0x1b)
		case 'r':
			b = append(b, '\r')
		case 'n':
			b = append(b, '\n')
		case 't':
			b = append(b, '\t')
		case '\\':
			b = append(b, '\\')
		case 'x':
			if i+2 >= len(s) {
				return nil, fmt.Errorf("a short \\x escape")
			}
			v, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil {
				return nil, err
			}
			b = append(b, byte(v))
			i += 2
		default:
			return nil, fmt.Errorf("an unknown escape \\%c", s[i])
		}
	}
	return b, nil
}

// Build compiles src with the one compile line into dir/name.
func Build(src, dir, name string) (string, error) {
	c, l, _ := build.FlagsFor(0)
	bin := filepath.Join(dir, name)
	cmd := exec.Command("gcc", append(append(c, l...), "-o", bin, src)...)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("gcc %s: %v\n%s", src, err, out)
	}
	return bin, nil
}

// Run is one session: its output and its exit status, or a hang.
func Run(bin string, keys []byte) ([]byte, int, error) { return RunArgs(bin, nil, keys) }

// RunArgs is Run with the editor's command-line arguments.
func RunArgs(bin string, args []string, keys []byte) ([]byte, int, error) {
	// THE KEYS ARE A FILE, NOT A PIPE.  The editor asks whether more typed input
	// is waiting when it decides whether to redraw, and through a pipe the answer
	// is how much the feeding goroutine has written by then: under load the
	// screen came out different 1 run in 300.  A regular file holds every key
	// from the start, so the answer is the same on every run: yes, until the end.
	in, err := os.CreateTemp("", "keys.")
	if err != nil {
		return nil, -1, err
	}
	defer os.Remove(in.Name())
	defer in.Close()
	if _, err := in.Write(keys); err != nil {
		return nil, -1, err
	}
	if _, err := in.Seek(0, 0); err != nil {
		return nil, -1, err
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = in
	// ITS OWN PROCESS GROUP.  `:suspend` and `:stop` signal the editor's whole group,
	// and in ours that stops the test and the shell that ran it.  And it dies
	// with us (Pdeathsig): a test killed from outside leaves no editor running.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	// The output is a file as well: the child is reaped here, by wait4, and no
	// goroutine copying a pipe is left waiting on it.
	out, err := os.CreateTemp("", "out.")
	if err != nil {
		return nil, -1, err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return nil, -1, err
	}
	code, err := reap(cmd.Process.Pid, 10*time.Second)
	b, rerr := os.ReadFile(out.Name())
	if err != nil {
		return b, -1, err
	}
	return b, code, rerr
}

// reap waits for pid to exit, as a shell would: a child that stops itself --
// `:suspend`, `:stop` -- is continued with SIGCONT, and one still running
// after limit is killed.  The exit status, or -1 for a signal.
func reap(pid int, limit time.Duration) (int, error) {
	timer := time.AfterFunc(limit, func() { syscall.Kill(pid, syscall.SIGKILL) })
	defer timer.Stop()
	for {
		var ws syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &ws, syscall.WUNTRACED, nil); err != nil {
			if err == syscall.EINTR {
				continue
			}
			return -1, err
		}
		switch {
		case ws.Stopped():
			syscall.Kill(pid, syscall.SIGCONT)
		case ws.Exited():
			return ws.ExitStatus(), nil
		case ws.Signaled():
			if ws.Signal() == syscall.SIGKILL {
				return -1, fmt.Errorf("no exit within %s", limit)
			}
			return -1, nil
		}
	}
}

// Compare runs every case on ref and on cand and returns the names of the
// cases whose output or exit status differ.
func Compare(cases []Case, ref, cand string) ([]string, error) {
	var diff []string
	for _, c := range cases {
		a, ea, err := Run(ref, c.Keys)
		if err != nil {
			return nil, fmt.Errorf("%s on the reference: %w", c.Name, err)
		}
		// Every case ends in :q!; one that reads to EOF left a mode too late
		// and typed the rest of its keys as text, so it tests less than it says.
		if bytes.Contains(a, []byte("Error reading input")) {
			return nil, fmt.Errorf("%s ran out of input on the reference: its keys never reach :q!", c.Name)
		}
		b, eb, err := Run(cand, c.Keys)
		if err != nil {
			diff = append(diff, c.Name+" ("+err.Error()+")")
			continue
		}
		if ea != eb || !bytes.Equal(a, b) {
			diff = append(diff, c.Name)
		}
	}
	return diff, nil
}

// The control changes a string the editor prints in insert mode, which the
// cases show on the screen.
const controlOld, controlNew = `_(" INSERT")`, `_(" INSERX")`

// Check builds the reference from src/whim-vim.c at rev and the candidate from
// candSrc, compares them on every case, and requires the control to be seen.
func Check(w io.Writer, rev, candSrc string, jvm JVM) error {
	cases, err := Cases()
	if err != nil {
		return err
	}
	b, err := prepare(rev, candSrc, jvm)
	if err != nil {
		return err
	}
	defer os.RemoveAll(b.dir)
	ref, cb, ctl := b.ref, b.cand, b.ctl
	start := time.Now()
	seen, err := Compare(cases, ref, ctl)
	if err != nil {
		return err
	}
	if len(seen) == 0 {
		return fmt.Errorf("suite: THE CONTROL WENT UNSEEN -- %s changed to %s and no case noticed, so the corpus proves nothing", controlOld, controlNew)
	}
	diff, err := Compare(cases, ref, cb)
	if err != nil {
		return err
	}
	if len(diff) > 0 {
		fmt.Fprintf(w, "  test         %d of %d cases behave differently from %s: %s\n", len(diff), len(cases), rev, strings.Join(diff, " "))
		return fmt.Errorf("suite: behaviour moved")
	}
	fmt.Fprintf(w, "  test         %d cases behave exactly as %s does; the control was seen by %d of them; %dms\n",
		len(cases), rev, len(seen), time.Since(start).Milliseconds())

	// THE GO EDITOR, the same cases: editor/ as it stands, which is what
	// internal/gen made of the C, answering exactly as the C candidate does.
	// Its control is the C one's: the string is in editor.go too.
	goStart := time.Now()
	goDiff, err := Compare(cases, cb, b.goBin)
	if err != nil {
		return err
	}
	if len(goDiff) > 0 {
		fmt.Fprintf(w, "  test         %d of %d cases: the Go editor differs from the C: %s\n", len(goDiff), len(cases), strings.Join(goDiff, " "))
		return fmt.Errorf("suite: the Go editor moved")
	}
	fmt.Fprintf(w, "  test         the Go editor (editor/) answers all %d exactly as the C does; %dms\n",
		len(cases), time.Since(goStart).Milliseconds())
	// THE EDITORS ON THE JVM, asked for (--java, --clojure): the same cases,
	// as the C does.
	wide := make([]WideCase, len(cases))
	for i, c := range cases {
		wide[i] = WideCase{Group: "keys", Name: c.Name, Keys: c.Keys}
	}
	for _, e := range b.jvm {
		if err := checkJVM(w, e.label(""), []string{"keys"}, wide, b, e); err != nil {
			return err
		}
	}
	return nil
}

// JVM is the editors on the JVM a run adds, each built from the candidate
// when its generator is given: the Java editor (--java) and the Clojure
// editor (--clojure).
type JVM struct {
	Java    braaam.Gen
	Clojure cljeditor.Gen
}

// builds is what a run compares: the C of rev (the reference), the
// candidate's C, the candidate with the control applied, and the Go editor as
// editor/ stands -- in a directory the caller removes; and, asked for, the
// editors on the JVM from the candidate, each with its control.
type builds struct {
	dir                   string
	refSrc, candSrc       []byte
	ref, cand, ctl, goBin string
	jvm                   []*jvmEditor
}

// prepare makes the four binaries, side by side: they are the cost of a run,
// and independent.
func prepare(rev, candSrc string, jvm JVM) (*builds, error) {
	dir, err := os.MkdirTemp("", "suite.")
	if err != nil {
		return nil, err
	}
	b := &builds{dir: dir}
	fail := func(err error) (*builds, error) {
		os.RemoveAll(dir)
		return nil, err
	}
	if b.refSrc, err = exec.Command("git", "show", rev+":src/whim-vim.c").Output(); err != nil {
		return fail(fmt.Errorf("git show %s:src/whim-vim.c: %w", rev, err))
	}
	if b.candSrc, err = os.ReadFile(candSrc); err != nil {
		return fail(err)
	}
	if bytes.Count(b.candSrc, []byte(controlOld)) != 1 {
		return fail(fmt.Errorf("suite: the control string %s is not in %s exactly once", controlOld, candSrc))
	}
	refSrc, ctlSrc := filepath.Join(dir, "ref.c"), filepath.Join(dir, "control.c")
	if err := os.WriteFile(refSrc, b.refSrc, 0o644); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(ctlSrc, bytes.Replace(b.candSrc, []byte(controlOld), []byte(controlNew), 1), 0o644); err != nil {
		return fail(err)
	}
	b.goBin = filepath.Join(dir, "whim")
	jobs := []func() error{
		func() (err error) { b.ref, err = Build(refSrc, dir, "ref"); return },
		func() (err error) { b.cand, err = Build(candSrc, dir, "cand"); return },
		func() (err error) { b.ctl, err = Build(ctlSrc, dir, "control"); return },
		func() error {
			if out, err := exec.Command("go", "build", "-o", b.goBin, "./editor/cmd/whim").CombinedOutput(); err != nil {
				return fmt.Errorf("go build ./editor/cmd/whim: %v\n%s", err, out)
			}
			return nil
		},
	}
	var java, clj *jvmEditor
	if jvm.Java != nil {
		jobs = append(jobs, func() (err error) { java, err = buildJava(jvm.Java, candSrc, dir); return })
	}
	if jvm.Clojure != nil {
		jobs = append(jobs, func() (err error) { clj, err = buildClojure(jvm.Clojure, candSrc, dir); return })
	}
	errs := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j func() error) {
			defer wg.Done()
			errs[i] = j()
		}(i, j)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return fail(err)
		}
	}
	for _, e := range []*jvmEditor{java, clj} {
		if e != nil {
			b.jvm = append(b.jvm, e)
		}
	}
	return b, nil
}
