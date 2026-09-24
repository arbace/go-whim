package p096

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

// w96Evidence is sections 6, 7 and 8.  Section 6 is the instrumented pair:
// scriptin[] is assigned once in the whole file -- to NULL, inside the function
// the phase removes -- and redir_fd only by its declaration, so neither FILE *
// has been opened in any build and the phase removes the POSSIBILITY.
func w96Evidence(r *check.Rep, tmp, inst, old, bin string) error {
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, n := range []string{"probe", "ctl"} {
		wg.Add(1)
		go func(i int, n string) {
			defer wg.Done()
			errs[i] = check.RecCore(filepath.Join(inst, n),
				filepath.Join(inst, n+".c"), filepath.Join(tmp, "REC."+n))
		}(i, n)
	}
	wg.Wait()
	if check.RecReport(r.W, errs...) {
		return stop("a harness failed on one of the two recordings")
	}
	total := len(check.WalkFiles(filepath.Join(tmp, "REC.probe")))
	probeWith, _ := check.Marked(filepath.Join(tmp, "REC.probe"), w96Mark)
	ctlWith, _ := check.Marked(filepath.Join(tmp, "REC.ctl"), w96Mark)
	if total < 100 {
		r.Say("a recording is %d files, and a comparison of two things", total)
		r.Cont("nothing wrote passes.  The COUNT is reported and not pinned:")
		r.Cont("a recording held 106 records when this phase was written and")
		r.Cont("holds 122 since phase 123 added the memline corpus, so")
		r.Cont("what is asserted is 0 marked here and the identical")
		r.Cont("instrument marking nearly all of them on ui_write().")
		return harness.ErrReported
	}
	if len(probeWith) != 0 {
		r.Say("%d of %d records ENTERED one of the five sites on the binary this phase was handed:", len(probeWith), total)
		for _, p := range probeWith {
			r.Cont("  %s", filepath.Join(tmp, "REC.probe", p))
		}
		r.Cont("so this phase removes code that CAN run, and the cut is wrong")
		return harness.ErrReported
	}
	if len(ctlWith) < 100 {
		r.Say("the control marked only %d of %d records through the", len(ctlWith), total)
		r.Cont("identical instrument on ui_write(), which every byte the")
		r.Cont("editor draws goes through.  Without it the zero above is a")
		r.Cont("probe that cannot fail.")
		return harness.ErrReported
	}
	r.Say("the instrumented pair: the five sites entered by 0 of %d records, ui_write() by %d of %d through the identical instrument -- that, and nothing else, is what says this phase removed code that could not run",
		total, len(ctlWith), total)

	// --- 7. eighteen adversarial sessions, each a way of making the editor
	// PRINT, which is where redir_write() sat.
	if err := w96Adversarial(r, filepath.Join(inst, "probe"), filepath.Join(inst, "ctl")); err != nil {
		return err
	}
	// --- 8. and the ordinary sessions, byte-identical either side.
	return w96Ordinary(r, old, bin)
}

func w96Adversarial(r *check.Rep, probe, ctl string) error {
	esc := []byte("\x1b")
	quit := []byte("\x1b:q!\r")
	seed := [][]byte{append([]byte("ialpha"), esc...), []byte(":set nopaste\r")}
	s := func(keys ...string) [][]byte {
		Out := append([][]byte{}, seed...)
		for _, k := range keys {
			Out = append(Out, []byte(k))
		}
		return append(Out, quit)
	}
	sessions := []struct {
		Name string
		Keys [][]byte
	}{
		{"messages", s(":messages\r")}, {"verbose", s(":verbose set ai?\r")},
		{"silent", s(":silent echo\r")}, {"history", s(":history\r")},
		{"registers", s("yy", ":registers\r")}, {"display", s("yy", ":display\r")},
		{"ga", s("ga")}, {"unknown", s(":nosuchcommand\r")}, {"set_all", s(":set all\r")},
		{"marks", s(":marks\r")}, {"undolist", s(":undolist\r")}, {"changes", s(":changes\r")},
		{"map", s(":map\r")}, {"highlight", s(":highlight\r")}, {"normal", s(":normal ihi\r")},
		{"global_p", s(":g/a/p\r")}, {"replay", s("qaxq", "@a")},
		{"verbose9", s(":set verbose=9\r", "yy")},
	}
	type res struct {
		marks   int
		blocked bool
	}
	got := make([][2]res, len(sessions))
	var wg sync.WaitGroup
	for i, x := range sessions {
		for j, b := range []string{probe, ctl} {
			wg.Add(1)
			go func(i, j int, b string, keys [][]byte) {
				defer wg.Done()
				_, _, errb, _, err := harness.CoreSession(b, keys, "xterm", []string{"+set paste"}, 24, 80, 8*time.Second)
				if err == harness.ErrBlocked {
					got[i][j] = res{0, true}
					return
				}
				got[i][j] = res{strings.Count(string(errb), w96Mark), false}
			}(i, j, b, x.Keys)
		}
	}
	wg.Wait()
	var fail []string
	for i, x := range sessions {
		if got[i][0].blocked || got[i][1].blocked {
			fail = append(fail, fmt.Sprintf("%s blocked, so it says nothing either way", x.Name))
		}
		if got[i][0].marks > 0 {
			fail = append(fail, fmt.Sprintf("%s entered one of the five sites %d times on the binary this phase was handed", x.Name, got[i][0].marks))
		}
		if got[i][1].marks == 0 {
			fail = append(fail, fmt.Sprintf("%s never reached ui_write() either, so it proves nothing: a session that draws nothing is not an adversary", x.Name))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("eighteen adversarial sessions -- :messages, :verbose, :silent, :history, :registers, :display, ga, an unknown command, :set all, :marks, :undolist, :changes, :map, :highlight, :normal, :g/a/p, a replayed register and :set verbose=9 -- each reached ui_write() and not one reached any of the five")
	return nil
}

func w96Ordinary(r *check.Rep, old, bin string) error {
	esc, cr := []byte("\x1b"), []byte("\r")
	quit := []byte("\x1b:q!\r")
	hello := []byte("hello")
	typed := func(seed []byte, keys ...[]byte) ([]string, [][]byte) {
		k := [][]byte{append(append([]byte("i"), seed...), esc...), []byte(":set nopaste\r")}
		return []string{"+set paste"}, append(append(k, keys...), quit)
	}
	type kase struct {
		Name string
		Args []string
		Keys [][]byte
		want string
	}
	var cases []kase
	one := func(name, want string, seed []byte, keys ...[]byte) {
		a, k := typed(seed, keys...)
		cases = append(cases, kase{name, a, k, want})
	}
	one("editing", "alpha", append(append([]byte("alpha"), cr...), []byte("beta")...),
		[]byte("0dwA-tail\x1b"), []byte("u"), []byte("yyp"))
	one("ctrl_g", "[Modified]", append(append([]byte("a"), cr...), []byte("b")...), []byte("\x07"))
	one("registers", "Type Name Content", hello, []byte("yy"), []byte(":registers\r"))
	one("messages", "hello", hello, []byte(":messages\r"))
	one("silent", "hello", hello, []byte(":silent echo\r"))
	one("verbose", "autoindent", hello, []byte(":verbose set ai?\r"))
	one("replay", "pha", []byte("alpha"), []byte("qaxq"), []byte("@a"))
	cases = append(cases,
		kase{"quit", []string{"+set paste"}, [][]byte{append(append([]byte("i"), hello...), esc...), []byte(":set nopaste\r"), []byte(":q\r")}, ""},
		kase{"quit_bang", []string{"+set paste"}, [][]byte{append(append([]byte("i"), hello...), esc...), []byte(":set nopaste\r"), []byte(":q!\r")}, ""})
	type Out struct{ oT, nT, nS string }
	outs := make([]Out, len(cases))
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func(i int, c kase) {
			defer wg.Done()
			ot, _ := check.CoreRecordStream(old, c.Args, c.Keys, 10*time.Second)
			nt, ns := check.CoreRecordStream(bin, c.Args, c.Keys, 10*time.Second)
			outs[i] = Out{ot, nt, ns}
		}(i, c)
	}
	wg.Wait()
	var fail []string
	for i, c := range cases {
		if outs[i].oT != outs[i].nT {
			fail = append(fail, fmt.Sprintf("%s moved, and NOTHING in this phase may move a record", c.Name))
		}
		if c.want != "" && !strings.Contains(outs[i].nT, c.want) && !strings.Contains(outs[i].nS, c.want) {
			fail = append(fail, fmt.Sprintf("%s: the new binary no longer shows %s, so \"it did not move\" is two failures agreeing", c.Name, check.CutilRepr(c.want)))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("nine ordinary sessions byte-identical either side and each doing its work: an editing session, CTRL-G with [Modified], :registers with its table, :messages, :silent, :verbose, a recorded register replayed with @a, :q and :q!")
	return nil
}
