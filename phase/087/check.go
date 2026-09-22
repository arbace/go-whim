package p087

// Whim phase 87, the check -- Ex mode, silent mode and the four options are gone.
// See phase/087/edit.go, and GOALS.md.
//
// Runs after phase/087/edit.go and the sweep tools/phaserun.sh runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// built from the boundary's own makefile flags.
//
// THE DECLARED DELTA IS NOT THE WHOLE EVIDENCE HERE, for two reasons that pull in
// opposite directions.  `phase/087/delta` says six records move -- `key_Q`,
// `key_gQ` and the argv rows `-e`, `-E`, `-e -s`, `-v` -- and tools/coredelta.sh
// proves that exactly those and nothing else did, against baselines recorded from
// whim-vim.  What it cannot show is a BEFORE: the baselines are one recording of one
// binary, so "the old one entered Ex mode and the new one beeps" is not a sentence
// it can say.  The probes below say it, by running both binaries.
//
// They are in two halves and both halves are the point:
//
// MUST DIFFER   the six records above and a pty session, each required to show Ex
// mode on the OLD binary.  A probe that only checks the new binary
// passes just as well on a phase that did nothing.
// MUST NOT      `-s` alone (already an unknown option before this phase, so it is
// not in the delta), every `+{command}` form, `:append`/`:insert`/
// `:change` (which read through getexline, not the Ex-mode reader),
// `:visual`/`:view`/`:vi`/`:ex` from Normal mode (whose Ex-mode
// escape this phase folded away), bare `-`, `--`, one and two file
// arguments, the three `-T` spellings, a plain edit and an editing
// pty session.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py): `mainerr()` prints the version banner, which carries __DATE__ and
// __TIME__, so two binaries built a minute apart disagree on stderr for a reason
// that is not the editor's behaviour -- which is exactly why `-s` reads as unchanged
// and must.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim87", Check) }

// z4Gone are the 21 identifiers the edit counted, matched BY WORD, plus the two
// strings only reachable through them.  `e_at_end_of_file`'s string is here and
// not in the edit because the variable outlives the edit by one sweep.
var z4Gone = []string{
	"exmode_active", "silent_mode", "pending_exmode_active", "exmode_plus", "exmode_was",
	"do_exmode", "getexmodeline", "nv_exmode", "EXMODE_NORMAL", "EXMODE_VIM", "BO_EX",
	"ex_pressedreturn", "ex_no_reprint", "ex_exitval", "previous_got_int", "use_plus_cmd",
	"s_vbuf", "e_at_end_of_file", "noexmode", "check_tty", "mch_input_isatty",
}

var z4GoneText = []string{"Entering Ex mode", "E501: At end-of-file"}

// z4GoneWord is by WORD and not by substring: `stdout_isatty` is phase 85's and
// survives this phase, and it contains `stdout`.
var z4GoneWord = []string{"setvbuf", "stdout"}

// z4Kept is the argv phase's, named one by one so that this phase cutting into
// it fails HERE rather than three phases later.
var z4Kept = []string{
	"EDIT_STDIN", "read_cmd_fd = 2;", "had_minmin", "buflist_add",
	"ME_TOO_MANY_ARGS", "case 'T':", "want_full_screen",
}

const (
	z4QRow  = `     {'Q', nv_error, NV_NCW, 0} ,`
	z4Enter = "Entering Ex mode"
	z4Unk   = "Unknown option argument"
)

// Whim87 is phase 87's check: Ex mode, silent mode and the four options.
//
// THE DECLARED DELTA IS NOT THE WHOLE EVIDENCE, for two reasons that pull in
// opposite directions.  phase/087/delta says six records move and
// tools/coredelta.sh proves exactly those did.  What it cannot show is a
// BEFORE: the baselines are one recording of one binary, so "the old one
// entered Ex mode and the new one beeps" is not a sentence it can say.  The
// probes say it, by running both binaries -- and they are in two halves, the
// six that MUST differ and the twenty-four that must NOT, because a probe that
// only checks the new binary passes just as well on a phase that did nothing.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim87 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noexmode", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }

	// --- 1. what the cut removed ---------------------------------------------
	for _, g := range append(append([]string{}, z4Gone...), z4GoneWord...) {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range z4GoneText {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	if n := check.Occurrences(`\bisatty\(`, src); n != 4 {
		return stop("isatty is called %d times, expected 4: mch_input_isatty's went with check_tty", n)
	}
	r.Say("0 mentions of all 21, neither Ex-mode string, isatty down to 4 calls")

	// --- 2. what it deliberately kept ----------------------------------------
	// A row is repointed, never deleted: a hole in nv_cmds[] moves every key
	// past it, and nvidx (run by phasecheck below) is the other half of this.
	if !check.HasLine(src, z4QRow) {
		return stop("the 'Q' row is not a repointed nv_error row")
	}
	for _, p := range []struct{ prefix, why string }{
		{"getexline(", "getexline went -- :append, :insert and :change read through it"},
		{"exe_commands(", "exe_commands went, and with it every +{command}"},
	} {
		if !check.HasLinePrefix(src, p.prefix) {
			return stop("%s", p.why)
		}
	}
	for _, k := range z4Kept {
		if !strings.Contains(string(src), k) {
			return stop("'%s' went, and it is the argv phase's to take", k)
		}
	}
	r.Say("kept: the 'Q' row at nv_error, getexline, exe_commands, and everything argv owns")

	// --- 3. the compile, the linkage and the libc surface --------------------
	// The symbol list the stage started with, read BEFORE phasecheck consumes
	// it: what this phase gives back is setvbuf and stdout, and NOT isatty,
	// which still has four callers.
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	gone := check.Comm23(before, after)
	if g := strings.Join(gone, " ") + " "; g != "setvbuf stdout " {
		return stop("the libc surface lost '%s', expected exactly 'setvbuf stdout '", g)
	}
	if !check.Contains(after, "isatty") {
		return stop("isatty left the binary -- four calls still name it")
	}
	r.Say("symbols %s -> %s, gone: setvbuf stdout; isatty stays",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the binary -------------------------------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	now, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes",
		beforeLines, check.CountLines(now), check.SizeOf(bin))

	// --- 5. the probes, both halves ------------------------------------------
	type res struct {
		Text  string
		Out   []byte
		bells int
	}
	rec := func(binary string, pargs []string, keys [][]byte) res {
		scr, Out, errb, rc, err := harness.ZSession(binary, keys, "xterm", pargs, 24, 80, 8*time.Second)
		if err == harness.ErrBlocked {
			// `vim -` reads the keystroke file as buffer text and then waits
			// for keys that never come.  That is a recording, not a crash.
			Body := "took the input over and never returned"
			return res{harness.Section("blocked", &Body), nil, 0}
		}
		if err != nil {
			return res{"ERROR " + err.Error(), nil, 0}
		}
		text := harness.Section(fmt.Sprintf("exit %d", rc), nil)
		text += harness.Section(fmt.Sprintf("bells %d", scr.Bells), nil)
		sum := sha256.Sum256(Out)
		text += harness.Section(fmt.Sprintf("stream %d sha=%s", len(Out), hex.EncodeToString(sum[:])[:16]), nil)
		e := strings.TrimRight(string(errb), "\n")
		text += harness.Section("stderr", &e)
		for i, s := range scr.Snaps {
			d := s.Text
			text += harness.Section(fmt.Sprintf("snap %d cursor=%d,%d bells=%d", i, s.Y, s.X, s.Bells), &d)
		}
		return res{harness.Scrub(text), Out, scr.Bells}
	}

	probes := z4Probes()
	type outcome struct {
		Name   string
		o, n   res
		Differ bool
	}
	outs := make([]outcome, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		go func(i int, p check.Z4Probe) {
			defer wg.Done()
			outs[i] = outcome{p.Name, rec(old, p.Args, p.Keys), rec(bin, p.Args, p.Keys), p.Differ}
		}(i, p)
	}
	wg.Wait()

	var fail, moved, static []string
	by := map[string]outcome{}
	for _, o := range outs {
		by[o.Name] = o
		same := o.o.Text == o.n.Text
		if o.Differ && same {
			fail = append(fail, fmt.Sprintf("%s was to move and did not", o.Name))
		}
		if !o.Differ && !same {
			fail = append(fail, fmt.Sprintf("%s moved and was not to", o.Name))
		}
		if same {
			static = append(static, o.Name)
		} else {
			moved = append(moved, o.Name)
		}
	}
	// The six that moved must have moved for the stated reason, and the reason
	// has to be visible on the OLD binary.
	for _, name := range []string{"key_Q", "key_gQ"} {
		o := by[name]
		if !strings.Contains(string(o.o.Out), z4Enter) {
			fail = append(fail, fmt.Sprintf("%s: the input binary did not enter Ex mode, so this proves nothing", name))
		}
		if strings.Contains(string(o.n.Out), z4Enter) {
			fail = append(fail, fmt.Sprintf("%s: the new binary still enters Ex mode", name))
		}
		if o.n.bells <= o.o.bells {
			fail = append(fail, fmt.Sprintf("%s: the key does not beep now (%d bells, was %d)", name, o.n.bells, o.o.bells))
		}
	}
	for _, name := range []string{"argv_e", "argv_E", "argv_e_s", "argv_v"} {
		o := by[name]
		if strings.Contains(o.o.Text, z4Unk) {
			fail = append(fail, fmt.Sprintf("%s: the input binary already rejected it, so this proves nothing", name))
		}
		if !strings.Contains(o.n.Text, z4Unk) {
			fail = append(fail, fmt.Sprintf("%s: it is not an unknown option now", name))
		}
	}
	if s := by["argv_s"]; !strings.Contains(s.o.Text, z4Unk) || !strings.Contains(s.n.Text, z4Unk) {
		fail = append(fail, "argv_s: -s was to be an unknown option on both binaries")
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("a probe that cannot fail is not evidence, and one that fails")
		r.Cont("differently is not this probe.")
		return harness.ErrReported
	}
	r.Say("probes: %d moved (%s), %d unchanged", len(moved), strings.Join(moved, " "), len(static))

	// --- 6. a real terminal, where none of this went through a pipe ----------
	if err := z4Pty(r, old, bin); err != nil {
		return err
	}
	return nil
}
