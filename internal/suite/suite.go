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
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/build"
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
func Run(bin string, keys []byte) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = bytes.NewReader(keys)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), -1, fmt.Errorf("no exit within 10 s")
	}
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		return nil, -1, err
	}
	return out.Bytes(), code, nil
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
func Check(w io.Writer, rev, candSrc string) error {
	cases, err := Cases()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "suite.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	refSrc := filepath.Join(dir, "ref.c")
	show, err := exec.Command("git", "show", rev+":src/whim-vim.c").Output()
	if err != nil {
		return fmt.Errorf("git show %s:src/whim-vim.c: %w", rev, err)
	}
	if err := os.WriteFile(refSrc, show, 0o644); err != nil {
		return err
	}
	cand, err := os.ReadFile(candSrc)
	if err != nil {
		return err
	}
	if bytes.Count(cand, []byte(controlOld)) != 1 {
		return fmt.Errorf("suite: the control string %s is not in %s exactly once", controlOld, candSrc)
	}
	ctlSrc := filepath.Join(dir, "control.c")
	if err := os.WriteFile(ctlSrc, bytes.Replace(cand, []byte(controlOld), []byte(controlNew), 1), 0o644); err != nil {
		return err
	}
	// The three builds are the cost of a run, and independent: side by side.
	srcs, names := []string{refSrc, candSrc, ctlSrc}, []string{"ref", "cand", "control"}
	bins, errs := make([]string, 3), make([]error, 3)
	var wg sync.WaitGroup
	for i := range srcs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bins[i], errs[i] = Build(srcs[i], dir, names[i])
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	ref, cb, ctl := bins[0], bins[1], bins[2]
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
	goBin := filepath.Join(dir, "whim")
	if out, err := exec.Command("go", "build", "-o", goBin, "./editor").CombinedOutput(); err != nil {
		return fmt.Errorf("go build ./editor: %v\n%s", err, out)
	}
	goDiff, err := Compare(cases, cb, goBin)
	if err != nil {
		return err
	}
	if len(goDiff) > 0 {
		fmt.Fprintf(w, "  test         %d of %d cases: the Go editor differs from the C: %s\n", len(goDiff), len(cases), strings.Join(goDiff, " "))
		return fmt.Errorf("suite: the Go editor moved")
	}
	fmt.Fprintf(w, "  test         the Go editor (editor/) answers all %d exactly as the C does; %dms\n",
		len(cases), time.Since(goStart).Milliseconds())
	return nil
}
