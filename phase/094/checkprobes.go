package p094

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

const w94E37 = "E37: No write since last change (add ! to override)"

// w94Probes: the corpus sees ONE case and cannot tell "the refusal was removed"
// from "a message changed" -- every zcases case ends with a trailing `:q!`,
// which quits the old binary too, so the exit status is 0 either side there.
// q_alone is the probe: `:q` with nothing after it.
func w94Probes(r *check.Rep, old, bin string) error {
	esc, cr := []byte("\x1b"), []byte("\r")
	quit := []byte("\x1b:q!\r")
	hello := []byte("hello")
	ctrlG := []byte("\x07")
	typed := func(seed []byte, keys ...[]byte) ([]string, [][]byte) {
		k := [][]byte{append(append([]byte("i"), seed...), esc...), []byte(":set nopaste\r")}
		return []string{"+set paste"}, append(append(k, keys...), quit)
	}
	seeded := func(keys ...[]byte) ([]string, [][]byte) {
		k := [][]byte{append(append([]byte("i"), hello...), esc...), []byte(":set nopaste\r")}
		return []string{"+set paste"}, append(k, keys...)
	}
	var probes []check.W89Probe
	one := func(name string, seed []byte, diff bool, keys ...[]byte) {
		a, k := typed(seed, keys...)
		probes = append(probes, check.W89Probe{Name: name, Args: a, Keys: k, Differ: diff})
	}
	a, k := seeded([]byte(":q\r"))
	probes = append(probes, check.W89Probe{Name: "q_alone", Args: a, Keys: k, Differ: true})
	one("q_modified", hello, true, []byte(":q\r"))
	one("q_range", hello, true, []byte(":1q\r"))
	one("q_spell_qu", hello, true, []byte(":qu\r"))
	one("q_spell_quit", hello, true, []byte(":quit\r"))
	one("q_after_undo", hello, true, []byte("x"), []byte(":q\r"))
	probes = append(probes, check.W89Probe{Name: "q_clean", Args: []string{"+set paste"}, Keys: [][]byte{[]byte(":set nopaste\r"), []byte(":q\r")}, Differ: false})
	a, k = seeded([]byte(":q!\r"))
	probes = append(probes, check.W89Probe{Name: "q_bang", Args: a, Keys: k, Differ: false})
	one("zz_key", hello, false, []byte("ZZ"))
	one("zq_key", hello, false, []byte("ZQ"))
	a, k = seeded([]byte(":cq\r"))
	probes = append(probes, check.W89Probe{Name: "cquit", Args: a, Keys: k, Differ: false})
	one("ctrl_g", append(append([]byte("a"), cr...), []byte("b")...), false, ctrlG)
	one("cmd_set_ro", hello, false, []byte(":set ro?\r"))
	one("cmd_set_mod", hello, false, []byte(":set modified?\r"))
	one("reg_list", hello, false, []byte("yy"), []byte(":registers\r"))
	one("cmd_undo", []byte("alpha"), false, []byte("x"), []byte("u"))
	one("editing", append(append([]byte("alpha"), cr...), []byte("beta")...), false,
		[]byte("0dwA-tail\x1b"), []byte("u"), []byte("yyp"))

	type outcome struct {
		Name           string
		oT, nT, oS, nS string
		oSn, nSn       int
		oRC, nRC       string
		oB, nB         int
		Differ         bool
	}
	outs := make([]outcome, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		go func(i int, p check.W89Probe) {
			defer wg.Done()
			ot, os_, osn, orc, ob := check.CoreRecordFull(old, p.Args, p.Keys, 10*time.Second)
			nt, ns, nsn, nrc, nb := check.CoreRecordFull(bin, p.Args, p.Keys, 10*time.Second)
			outs[i] = outcome{p.Name, ot, nt, os_, ns, osn, nsn, orc, nrc, ob, nb, p.Differ}
		}(i, p)
	}
	wg.Wait()

	var fail, moved, static []string
	by := map[string]outcome{}
	for _, o := range outs {
		by[o.Name] = o
		same := o.oT == o.nT
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
	o := by["q_alone"]
	if !strings.Contains(o.oT, w94E37) {
		fail = append(fail, "q_alone: the input binary did not refuse, so this proves nothing about a refusal being removed")
	}
	if o.oRC != "1" {
		fail = append(fail, fmt.Sprintf("q_alone: the input binary exited %s, expected 1 -- it draws E37, runs out of stdin and gives up", check.CutilRepr(o.oRC)))
	}
	if !strings.Contains(o.oS, "Vim: Finished.") && !strings.Contains(o.oT, "Vim: Finished.") {
		fail = append(fail, "q_alone: the input binary did not print `Vim: Finished.`, so it did not reach end of input after refusing")
	}
	if strings.Contains(o.nT, w94E37) {
		fail = append(fail, "q_alone: this binary still refuses")
	}
	if o.nRC != "0" {
		fail = append(fail, fmt.Sprintf("q_alone: this binary exited %s, expected 0 -- `:q` quits now", check.CutilRepr(o.nRC)))
	}
	for _, name := range []string{"q_modified", "q_range", "q_spell_qu", "q_spell_quit", "q_after_undo"} {
		o := by[name]
		if !strings.Contains(o.oT, w94E37) {
			fail = append(fail, fmt.Sprintf("%s: the input binary did not refuse, so \"it moved\" is not evidence of anything", name))
		}
		if strings.Contains(o.nT, w94E37) {
			fail = append(fail, fmt.Sprintf("%s: this binary still refuses", name))
		}
		if o.nSn >= o.oSn {
			fail = append(fail, fmt.Sprintf("%s: the record did not lose a snapshot (%d -> %d), and the E37 screen is what it loses", name, o.oSn, o.nSn))
		}
	}
	if o := by["q_modified"]; o.oB != 1 || o.nB != 0 {
		fail = append(fail, fmt.Sprintf("q_modified: the bells went %d -> %d, expected 1 -> 0 -- the refusal beeped and nothing here does", o.oB, o.nB))
	}
	o = by["q_clean"]
	if strings.Contains(o.oT, w94E37) || strings.Contains(o.nT, w94E37) || o.oRC != "0" || o.nRC != "0" {
		fail = append(fail, fmt.Sprintf("q_clean: `:q` on an UNMODIFIED buffer must quit with status 0 on both binaries and refuse on neither -- it took the else arm before this phase and takes it now, which is what makes it the pair of q_alone (%s, %s)",
			check.CutilRepr(o.oRC), check.CutilRepr(o.nRC)))
	}
	if o := by["cquit"]; o.oRC != "1" || o.nRC != "1" {
		fail = append(fail, fmt.Sprintf("cquit: `:cq` exits 1 on both binaries and this phase does not touch it (%s, %s)", check.CutilRepr(o.oRC), check.CutilRepr(o.nRC)))
	}
	if o := by["ctrl_g"]; !strings.Contains(o.nT, "[Modified]") {
		fail = append(fail, "ctrl_g: CTRL-G no longer says [Modified], and the state is exactly what this phase does NOT remove")
	}
	if o := by["cmd_set_mod"]; !strings.Contains(o.nS, "modified") {
		fail = append(fail, "cmd_set_mod: `:set modified?` answered nothing, so \"it did not move\" is two failures agreeing")
	}
	for _, p := range []struct{ Name, want string }{{"editing", "alpha"}, {"reg_list", "hello"}, {"cmd_undo", "alpha"}} {
		o := by[p.Name]
		if !strings.Contains(o.nT, p.want) && !strings.Contains(o.nS, p.want) {
			fail = append(fail, fmt.Sprintf("%s: the new binary no longer shows %s, so \"it did not move\" is two failures agreeing", p.Name, check.CutilRepr(p.want)))
		}
	}
	if by["zz_key"].oT != by["zq_key"].oT {
		fail = append(fail, "ZZ and ZQ do not leave the same record, and they have run the same command string since phase 89")
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("the corpus sees ONE case and cannot tell \"the refusal was")
		r.Cont("removed\" from \"a message changed\": every zcases.py case ends")
		r.Cont("with a trailing `:q!`, which quits the old binary too, so the")
		r.Cont("exit status is 0 either side there.  q_alone is the probe.")
		return harness.ErrReported
	}
	r.Say("probes: %d moved (%s), %d unchanged", len(moved), strings.Join(moved, " "), len(static))
	r.Cont("q_alone is the phase: `:q` with nothing after it draws E37, runs out of stdin, prints `Vim: Finished.` and exits 1 on the binary this phase was handed, and quits with status 0 here -- which no recording can see")
	r.Cont("and what did not move is doing its work: `:q` on an unmodified buffer (0 either side), `:q!`, ZZ and ZQ identical to each other, `:cq` exiting 1, CTRL-G still saying [Modified], `:set modified?`, :registers and an ordinary editing session")
	return nil
}

func w94Pty(r *check.Rep, old, bin string) error {
	home, err := os.MkdirTemp("", "whim94-home-")
	if err != nil {
		return err
	}
	session := func(binary string, keys [][]byte) (string, int, error) {
		d, err := os.MkdirTemp("", "whim94-pty-")
		if err != nil {
			return "", -1, err
		}
		text, status, err := harness.Session(binary, nil, keys, "xterm",
			20*time.Second, 600*time.Millisecond, d, check.W85Env(home), 0, 0)
		return string(text), status, err
	}
	quitKeys := [][]byte{[]byte("ityped on a terminal\x1b"), []byte(":q\r"), []byte(":q!\r")}
	edit := [][]byte{[]byte("ialpha\rbeta\x1b"), []byte("ggdwA-tail\x1b"), []byte("u"), []byte(":q!\r")}
	var fail []string
	o, _, err := session(old, quitKeys)
	if err != nil {
		return err
	}
	n, _, err := session(bin, quitKeys)
	if err != nil {
		return err
	}
	if !strings.Contains(o, "E37: No write since last change") {
		fail = append(fail, "the pty session did not refuse on the input binary, so it proves nothing")
	}
	if strings.Contains(n, "E37") {
		fail = append(fail, "the pty session still refuses on the new binary")
	}
	if !strings.Contains(o, ":q!") {
		fail = append(fail, "the `:q!` did not reach the input binary, so the `:q` there did not leave the editor running -- which is the evidence that it refused rather than quitting")
	}
	eo, eos, err := session(old, edit)
	if err != nil {
		return err
	}
	en, ens, err := session(bin, edit)
	if err != nil {
		return err
	}
	// The blinded field, and the guard that keeps the blinding from becoming a
	// blinding of nothing: see whim93, where the same measurement is written up.
	if !strings.Contains(eo, "change; before #") || !check.W93AGO.MatchString(eo) {
		fail = append(fail, fmt.Sprintf("the undo report with its `N seconds ago` is not in the pty session on the input binary, so blinding the clock blinds nothing and the comparison below is not the one described: %s", check.CutilRepr(check.Tail200(eo))))
	}
	if check.W93AGO.ReplaceAllString(eo, "<ago>") != check.W93AGO.ReplaceAllString(en, "<ago>") || eos != ens {
		fail = append(fail, "an ordinary pty editing session moved, and nothing here may move it")
	}
	if !strings.Contains(en, "alpha") {
		fail = append(fail, "the pty editing session did nothing, so \"identical\" is two failures agreeing")
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("a real terminal: `:q` draws E37 and leaves the editor running on the binary this phase was handed, so its `:q!` is what ends the session, and here there is no E37 at all -- an ordinary editing session is identical either side")
	return nil
}
