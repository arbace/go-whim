package p088

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

// z5Probes is section 6: 22 command lines recorded on BOTH binaries, six of
// which the declared delta says move.  Each of the six is a way of naming a
// FILE or a STREAM to edit and is required to do that on the OLD binary -- a
// probe that only looks at the new binary passes on a phase that did nothing.
func z5Probes(r *check.Rep, old, bin string) error {
	quit := []byte("\x1b:q!\r")
	esc := []byte("\x1b")
	only := [][]byte{quit}
	alpha := []byte("alpha\rbeta")
	typed := func(seed []byte, keys ...[]byte) ([]string, [][]byte) {
		k := [][]byte{append(append([]byte("i"), seed...), esc...), []byte(":set nopaste\r")}
		return []string{"+set paste"}, append(append(k, keys...), quit)
	}
	ea, ek := typed(alpha, []byte("0dwA-tail\x1b"), []byte("u"), []byte("\x12"), []byte("yyp"))
	probes := []check.Z4Probe{
		{Name: "argv_file", Args: []string{"f.txt"}, Keys: only, Differ: true},
		{Name: "argv_files", Args: []string{"f.txt", "g.txt"}, Keys: only, Differ: true},
		// A bare `-` read the keystroke file itself as the buffer, closed fd 0
		// and waited on fd 2 for keys that never came: the old record is
		// `blocked`.
		{Name: "argv_minus", Args: []string{"-"}, Keys: [][]byte{[]byte("text on stdin\r"), quit}, Differ: true},
		{Name: "argv_minmin", Args: []string{"--"}, Keys: only, Differ: true},
		{Name: "argv_minmin_plus", Args: []string{"--", "+q!"}, Keys: only, Differ: true},
		{Name: "argv_plus_file", Args: []string{"+q!", "f.txt"}, Keys: only, Differ: true},
		{Name: "argv_none", Args: nil, Keys: only, Differ: false},
		{Name: "argv_plus", Args: []string{"+"}, Keys: only, Differ: false},
		{Name: "argv_plus_q", Args: []string{"+q!"}, Keys: only, Differ: false},
		{Name: "argv_plus_nu", Args: []string{"+set nu"}, Keys: only, Differ: false},
		{Name: "argv_plus_two", Args: []string{"+set nu", "+q!"}, Keys: only, Differ: false},
		// 'paste' is what every case of the corpus seeds itself under, and it
		// goes in as a +{command}: this phase is where both could have been
		// lost together.
		{Name: "argv_plus_paste", Args: []string{"+set paste"}, Keys: [][]byte{[]byte("ihello\x1b"), quit}, Differ: false},
		{Name: "argv_T_xterm", Args: []string{"-T", "xterm"}, Keys: only, Differ: false},
		{Name: "argv_T", Args: []string{"-T"}, Keys: only, Differ: false},
		{Name: "argv_Txterm", Args: []string{"-Txterm"}, Keys: only, Differ: false},
		{Name: "argv_T_unknown", Args: []string{"-T", "no-such-term-9x"}, Keys: only, Differ: false},
		{Name: "argv_R", Args: []string{"-R"}, Keys: only, Differ: false},
		{Name: "argv_e", Args: []string{"-e"}, Keys: only, Differ: false},
		{Name: "argv_help", Args: []string{"--help"}, Keys: only, Differ: false},
		{Name: "argv_version", Args: []string{"--version"}, Keys: only, Differ: false},
		{Name: "argv_ttyfail", Args: []string{"--ttyfail"}, Keys: only, Differ: false},
		{Name: "editing", Args: ea, Keys: ek, Differ: false}}
	type outcome struct {
		Name   string
		o, n   check.ZRec
		Differ bool
	}
	outs := make([]outcome, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		go func(i int, p check.Z4Probe) {
			defer wg.Done()
			outs[i] = outcome{p.Name, check.ZRecord(old, p.Args, p.Keys), check.ZRecord(bin, p.Args, p.Keys), p.Differ}
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
	// Each of the six must have moved FOR ITS OWN REASON, visible on the old
	// binary.
	for _, name := range []string{"argv_file", "argv_minmin", "argv_minmin_plus", "argv_plus_file"} {
		o := by[name]
		if strings.Contains(o.o.Text, z5Unk) {
			fail = append(fail, fmt.Sprintf("%s: the input binary already refused it, so this proves nothing", name))
		}
		if !strings.Contains(o.n.Text, z5Unk) {
			fail = append(fail, fmt.Sprintf("%s: it is not an unknown option now", name))
		}
	}
	if o := by["argv_file"]; !strings.Contains(o.o.Text, "exit 0") || len(o.o.Out) < 500 {
		fail = append(fail, fmt.Sprintf("argv_file: the input binary did not open a buffer and draw (%d bytes)", len(o.o.Out)))
	}
	o := by["argv_files"]
	if !strings.Contains(o.o.Text, "Too many edit arguments") {
		fail = append(fail, "argv_files: the input binary did not answer ME_TOO_MANY_ARGS, so removing that row proves nothing")
	}
	if strings.Contains(o.n.Text, "Too many edit arguments") || !strings.Contains(o.n.Text, z5Unk) {
		fail = append(fail, "argv_files: the second file argument is not an unknown option now")
	}
	o = by["argv_minus"]
	if !strings.Contains(o.o.Text, "blocked") {
		fail = append(fail, "argv_minus: the input binary did not take stdin over, so this proves nothing")
	}
	if strings.Contains(o.n.Text, "blocked") || !strings.Contains(o.n.Text, z5Unk) {
		fail = append(fail, "argv_minus: a bare `-` is not an unknown option now")
	}
	// `--` and `-- +q!` disagreed with each other before, because `+q!` after
	// `--` was a file name; they agree now, and that is the whole of what `--`
	// did.
	if by["argv_minmin"].o.Text == by["argv_minmin_plus"].o.Text {
		fail = append(fail, "the input binary read `--` and `-- +q!` the same, so `--` was not ending the options")
	}
	if by["argv_minmin"].n.Text != by["argv_minmin_plus"].n.Text {
		fail = append(fail, "`--` and `-- +q!` still differ, so something still ends the options")
	}
	for _, name := range []string{"argv_T_xterm", "argv_none"} {
		if o := by[name]; !strings.Contains(o.n.Text, "exit 0") || len(o.n.Out) < 500 {
			fail = append(fail, fmt.Sprintf("%s: the new binary did not start and draw", name))
		}
	}
	if o := by["argv_plus_q"]; !strings.Contains(o.n.Text, "exit 0") {
		fail = append(fail, "argv_plus_q: `+q!` no longer runs and quits")
	}
	if o := by["argv_T"]; !strings.Contains(o.n.Text, "Argument missing after") {
		fail = append(fail, "argv_T: a bare -T is not mainerr_arg_missing now")
	}
	if o := by["argv_Txterm"]; !strings.Contains(o.n.Text, "Garbage after option argument") {
		fail = append(fail, "argv_Txterm: -Txterm is not ME_GARBAGE now -- the renumbering is wrong")
	}
	if o := by["argv_plus_paste"]; !strings.Contains(o.n.Text, "exit 0") || !strings.Contains(o.n.Text, "hello") {
		fail = append(fail, "argv_plus_paste: `+set paste` and typing under it no longer work")
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
	return nil
}

// z5Pty is section 7: a file on the command line on a real terminal, where the
// old binary edits it and the new one refuses before the screen exists.
func z5Pty(r *check.Rep, old, bin string) error {
	home, err := os.MkdirTemp("", "whim88-home-")
	if err != nil {
		return err
	}
	session := func(binary string, args []string, keys [][]byte) (string, int, error) {
		d, err := os.MkdirTemp("", "whim88-pty-")
		if err != nil {
			return "", -1, err
		}
		os.WriteFile(d+"/f.txt", []byte("one\ntwo\nthree\n"), 0o644)
		text, status, err := harness.Session(binary, args, keys, "xterm",
			20*time.Second, 600*time.Millisecond, d, check.Z2Env(home), 0, 0)
		return string(text), status, err
	}
	say := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	fileKeys := [][]byte{[]byte("Gdd"), []byte(":q!\r")}
	oTxt, _, err := session(old, []string{"f.txt"}, fileKeys)
	if err != nil {
		return err
	}
	nTxt, nRC, err := session(bin, []string{"f.txt"}, fileKeys)
	if err != nil {
		return err
	}
	if !strings.Contains(oTxt, "three") {
		return say("the input binary did not open the file on a pty -- this proves nothing")
	}
	if !strings.Contains(nTxt, "Unknown option argument") {
		return say("the new binary did not refuse the file argument on a pty")
	}
	// THE TWO HARNESSES REPORT DIFFERENT THINGS and the message names the
	// Python's.  ptyrun returns the raw wait status, so mainerr()'s mch_exit(1)
	// arrives there as 256; harness.Session returns the EXIT CODE, which is 1.
	// Shifted rather than compared against 1, so that the refusal says 256 as
	// the shell's does instead of the message and the test disagreeing.
	if nRC<<8 != 256 {
		return say("the refused file argument left wait status %d, expected 256 (exit 1)", nRC)
	}
	edKeys := [][]byte{[]byte("ityped here\x1b"), []byte("0dw"), []byte(":set ruler?\r"), []byte(":q!\r")}
	eo, eoRC, err := session(old, nil, edKeys)
	if err != nil {
		return err
	}
	en, enRC, err := session(bin, nil, edKeys)
	if err != nil {
		return err
	}
	if !strings.Contains(eo, "here") || !strings.Contains(eo, "ruler") {
		return say("the editing pty session did not edit on the input binary")
	}
	if eo != en || eoRC != enRC {
		return say("the editing pty session moved: %d -> %d", eoRC, enRC)
	}
	r.Say("pty: the file argument edited by the old binary and refused by the new; the editing session identical either side")
	return nil
}
