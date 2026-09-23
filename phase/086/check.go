package p086

// Whim phase 86 -- the instrument becomes the screen.  See GOAL.md, and GOALS.md II.2.
//
// NO SOURCE CHANGE AT ALL: q86's whim-vim.c is q85's, byte for byte, and this phase
// asserts it.  What changes is how every later phase is measured.
//
// The core's editor is on its way to having no file to write, no file to read and no
// stream to print on, so `tools/behaviour.py` -- which ends every case with
// `+w! <file>` and reads the file back -- and `tools/exsweep.py` -- which runs a
// command on a file and records the exit status -- stop being instruments the
// moment the phases they are meant to measure land.  A whole-program phase is
// right here because there is no source edit for a sweep to follow.
//
// The instrument they are replaced with is `tools/zrecord.sh`: keystrokes in on
// stdin, escape sequences out on stdout, and a screen per redraw rebuilt from them
// (GOALS.md II.2).  Five parts -- 102 keystroke cases, every Ex command typed at
// `:`, every command line the parser may see, four pty scenarios for what only a
// terminal shows, and whim's own terminal table.
//
// WHAT THIS PHASE PROVES, in order, each depending on the one before:
//
// 1. the tree is untouched: whim-vim.c is what the phase was handed;
// 2. it builds with the boundary's flags, and is still absolutely static;
// 3. THE INSTRUMENT IS DETERMINISTIC: three recordings of that binary, byte for
// byte identical, digests included;
// 4. THE INSTRUMENT CAN FAIL: a copy of the source with do_addsub() returning
// FAIL -- CLAUDE.md's canonical break -- must move EXACTLY the eleven cases
// that increment or decrement, and no others.  A corpus that cannot fail is
// not evidence;
// 5. the declared delta holds: tools/coredelta.sh --phase 86 against
// .reference/core-baselines, which phase 83 records from whim-vim.  Those
// baselines are the INPUT's behaviour, so the delta is cumulative -- phase 85
// removed the two "not to a terminal" warnings, and `stderr-moved` is that,
// declared once and checked at every phase after it;
// 6. the old instrument still reaches whim's baselines: tools/whimdelta.sh on the
// same binary against .reference/baselines, which is the only bridge between
// the two pipelines' recordings and is kept for exactly that.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
	"github.com/arbace/go-whim/internal/verify"
)

func init() { check.Register("whim86", Check) }

// Whim86 is phase 86, whole: the instrument becomes the screen.
func Check(w io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: check whim86 <work-dir>")
	}
	work := args[0]
	f := filepath.Join(work, "whim-vim.c")
	tmp, err := os.MkdirTemp("", "whim86")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. the tree is untouched ----------------------------------------
	before := check.Sha256File(f)

	// --- 2. the build, with the boundary's flags ------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin := filepath.Join(work, "whim-vim")
	if !check.StaticFacts(w, bin) {
		return harness.ErrReported
	}

	// --- 3. the instrument is deterministic -------------------------------
	if !check.RecordThrice(w, bin, f, tmp) {
		return harness.ErrReported
	}
	run1 := filepath.Join(tmp, "run1")
	cases := check.DirCount(filepath.Join(run1, "screen"))
	(&check.Rep{Tag: "instrument", W: w}).Say("%d cases, %d commands, %d command lines, %d pty scenarios: 3 identical runs, digests included",
		cases, check.CountHeaders(filepath.Join(run1, "ref-excmds.txt")), check.CountHeaders(filepath.Join(run1, "ref-argv.txt")), check.CountHeaders(filepath.Join(run1, "ref-pty.txt")))

	// --- 4. the instrument can fail ---------------------------------------
	af := &check.Rep{Tag: "ablefail", W: w}
	const expected = "decr_dec decr_hex incr_alpha incr_bin incr_count incr_dec incr_hex incr_midword incr_oct incr_unsigned mb_incr"
	broken := filepath.Join(tmp, "broken")
	os.MkdirAll(broken, 0o755)
	t := check.ReadFile(f)
	i := strings.Index(t, "\ndo_addsub(")
	if i < 0 {
		return fmt.Errorf("do_addsub( is not in %s", f)
	}
	j := strings.Index(t[i:], "{\n")
	if j < 0 {
		return fmt.Errorf("do_addsub has no body in %s", f)
	}
	j += i + 2
	os.WriteFile(filepath.Join(broken, "whim-vim.c"), []byte(t[:j]+"    return FAIL;\n"+t[j:]), 0o644)
	if err := check.CopyExec(filepath.Join(work, "Makefile"), filepath.Join(broken, "Makefile")); err != nil {
		return err
	}
	if exec.Command("make", "-C", broken).Run() != nil {
		af.Say("the patched copy did not build -- the break is wrong, not the corpus")
		return harness.ErrReported
	}
	bs := filepath.Join(tmp, "broken-screen")
	if err := exec.Command("tools/st.sh", "zcases", filepath.Join(broken, "whim-vim"), bs).Run(); err != nil {
		return harness.ErrReported
	}
	moved := movedNames(filepath.Join(run1, "screen"), bs)
	if moved != expected+" " {
		m := moved
		if m == "" {
			m = "(none)"
		}
		af.Say("a broken do_addsub() moved a different set of cases:")
		fmt.Fprintf(w, "                 got      %s\n", m)
		fmt.Fprintf(w, "                 expected %s\n", expected)
		af.Cont("A corpus that cannot fail is not evidence, and one that")
		af.Cont("fails differently is not this corpus.")
		return harness.ErrReported
	}
	af.Say("do_addsub() returning FAIL moves exactly 11 of %d cases, and nothing else", cases)

	// --- 5. the declared delta --------------------------------------------
	if err := verify.CoreDelta(bin, f, 86, w); err != nil {
		return harness.ErrReported
	}

	// --- 6. the bridge to whim's baselines --------------------------------
	br := &check.Rep{Tag: "bridge", W: w}
	// The last phase measured against whim's own baselines: the phase before
	// the core's line (internal/build.CoreFrom).
	whimLast := build.CoreFrom - 1
	if fi, e := os.Stat(".reference/baselines/behaviour"); e == nil && fi.IsDir() {
		var ob bytes.Buffer
		e := verify.Delta(bin, f, whimLast, &ob)
		o := ob.Bytes()
		if e != nil {
			w.Write(o)
			br.Say("whim-vim does NOT show whim's declared delta to phase %d", whimLast)
			return harness.ErrReported
		}
		held := ""
		re := regexp.MustCompile(`^ *delta  *exactly as declared: (.*)$`)
		for _, l := range strings.Split(string(o), "\n") {
			if m := re.FindStringSubmatch(l); m != nil {
				held = m[1]
			}
		}
		cmds := held
		if k := strings.Index(held, ";"); k >= 0 {
			cmds = held[:k]
		}
		cs := held
		if k := strings.Index(held, "cases:"); k >= 0 {
			cs = held[k+len("cases:"):]
		}
		br.Say("%d commands and %d cases against slim-vim's baselines, exactly whim's declared delta to phase %d", len(strings.Fields(cmds)), len(strings.Fields(cs)), whimLast)
	} else {
		br.Say("no slim baselines at .reference/baselines -- whim's delta not rechecked")
	}

	// --- 1, concluded -----------------------------------------------------
	if check.Sha256File(f) != before {
		(&check.Rep{Tag: "source", W: w}).Say("whim-vim.c was modified by a phase that must not modify it")
		return harness.ErrReported
	}
	(&check.Rep{Tag: "source", W: w}).Say("whim-vim.c unchanged, %d lines: r3 is r2's tree, and only the instrument moved", check.CountLines([]byte(check.ReadFile(f))))
	return nil
}

// movedNames is the shell's
//
//	diff -rq a b | grep -E '^(Files|Only in)' | sed 's/^Only in [^:]*: //; s/ and .*//; s/.*screen\///' | sort -u | tr '\n' ' '
//
// over two flat screen directories.
func movedNames(a, b string) string {
	set := map[string]bool{}
	only := regexp.MustCompile(`^Only in [^:]*: `)
	and := regexp.MustCompile(` and .*`)
	scr := regexp.MustCompile(`.*screen/`)
	for _, l := range check.DiffRQ(a, b) {
		if !strings.HasPrefix(l, "Files") && !strings.HasPrefix(l, "Only in") {
			continue
		}
		l = only.ReplaceAllString(l, "")
		l = and.ReplaceAllString(l, "")
		l = scr.ReplaceAllString(l, "")
		set[l] = true
	}
	var s []string
	for k := range set {
		s = append(s, k)
	}
	sort.Strings(s)
	Out := ""
	for _, k := range s {
		Out += k + " "
	}
	return Out
}
