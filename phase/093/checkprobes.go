package p093

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

const w93NoName = `"[No Name]" [Modified] 1 line --100%--`

func w93Probes(r *check.Rep, old, bin string) error {
	esc, cr := []byte("\x1b"), []byte("\r")
	quit := []byte("\x1b:q!\r")
	hello := []byte("hello")
	ctrlG, ctrlR, ctrlF, ctrlP := []byte("\x07"), []byte("\x12"), []byte("\x06"), []byte("\x10")
	typed := func(seed []byte, keys ...[]byte) ([]string, [][]byte) {
		k := [][]byte{append(append([]byte("i"), seed...), esc...), []byte(":set nopaste\r")}
		return []string{"+set paste"}, append(append(k, keys...), quit)
	}
	var probes []check.W89Probe
	one := func(name string, seed []byte, diff bool, keys ...[]byte) {
		a, k := typed(seed, keys...)
		probes = append(probes, check.W89Probe{Name: name, Args: a, Keys: k, Differ: diff})
	}
	ab := append(append([]byte("a"), cr...), []byte("b")...)
	cmdOf := func(mid ...[]byte) []byte {
		Out := []byte(":")
		for _, m := range mid {
			Out = append(Out, m...)
		}
		return append(Out, cr...)
	}
	one("file_rename", hello, true, []byte(":file NEWNAME\r"), ctrlG)
	one("file_bang", hello, true, []byte(":file! NEWNAME\r"), ctrlG)
	one("cp_missing", []byte("nosuchfile"), true, []byte("0"), cmdOf(ctrlR, ctrlP))
	one("cp_existing", []byte("keys"), false, []byte("0"), cmdOf(ctrlR, ctrlP))
	one("cf_existing", []byte("keys"), false, []byte("0"), cmdOf(ctrlR, ctrlF))
	one("ctrl_g", ab, false, ctrlG)
	one("g_ctrl_g", ab, false, append([]byte("g"), ctrlG...))
	one("registers", hello, false, []byte("yy"), []byte(":registers\r"))
	one("reg_percent", hello, false, []byte("A \x1b\"%p\x1b"))
	one("reg_hash", hello, false, []byte("A \x1b\"#p\x1b"))
	one("cmd_filter", hello, false, []byte(":filter\r"))
	one("cmd_fixdel", hello, false, []byte(":fixdel\r"))
	one("cmd_ls", hello, false, []byte(":ls\r"))
	one("quit_modified", hello, false, []byte(":q\r"))
	probes = append(probes, check.W89Probe{Name: "quit_bang", Args: []string{"+set paste"}, Keys: [][]byte{append(append([]byte("i"), hello...), esc...), []byte(":set nopaste\r"), []byte(":q!\r")}, Differ: false})
	one("editing", append(append([]byte("alpha"), cr...), []byte("beta")...), false,
		[]byte("0dwA-tail\x1b"), []byte("u"), []byte("yyp"))
	// The spellings are appended, as the Python's comprehension is.
	for _, s := range w93Spellings {
		one("spell_"+s, hello, true, []byte(":"+s+"\r"))
	}

	type outcome struct {
		Name           string
		oT, nT, oS, nS string
		Differ         bool
	}
	outs := make([]outcome, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		go func(i int, p check.W89Probe) {
			defer wg.Done()
			ot, os_ := check.CoreRecordStream(old, p.Args, p.Keys, 10*time.Second)
			nt, ns := check.CoreRecordStream(bin, p.Args, p.Keys, 10*time.Second)
			outs[i] = outcome{p.Name, ot, nt, os_, ns, p.Differ}
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
	for _, name := range []string{"file_rename", "file_bang"} {
		o := by[name]
		if !strings.Contains(o.oT, `"NEWNAME" [Modified][Not edited] 1 line --100%--`) {
			fail = append(fail, fmt.Sprintf("%s: the input binary did not name the buffer NEWNAME, so this proves nothing about naming one", name))
		}
		if strings.Contains(o.nT, `NEWNAME"`) {
			fail = append(fail, fmt.Sprintf("%s: the new binary named the buffer anyway", name))
		}
		if !strings.Contains(o.nT, w93NoName) {
			fail = append(fail, fmt.Sprintf(`%s: CTRL-G does not answer "[No Name]" now, and that is the only name a buffer has`, name))
		}
		if !strings.Contains(o.nT, check.W89E492) {
			fail = append(fail, fmt.Sprintf("%s: `:file` is not an unknown command", name))
		}
		if strings.Contains(o.nT, "Not edited") {
			fail = append(fail, fmt.Sprintf("%s: [Not edited] is still shown, and BF_NOTEDITED has no writer left", name))
		}
	}
	o := by["cp_missing"]
	if strings.Contains(o.oT, check.W89E492) {
		fail = append(fail, "cp_missing: the input binary already put the word on the command line, so it never asked the filesystem and this proves nothing")
	}
	if !strings.Contains(o.nT, "E492: Not an editor command: nosuchfile") {
		fail = append(fail, "cp_missing: CTRL-P does not yield the word under the cursor now")
	}
	const trailing = "E488: Trailing characters: eys"
	for _, name := range []string{"cp_existing", "cf_existing"} {
		o := by[name]
		if !strings.Contains(o.oT, trailing) || !strings.Contains(o.nT, trailing) {
			fail = append(fail, fmt.Sprintf("%s: `keys` did not reach the command line on both binaries, so \"it did not move\" is two failures agreeing", name))
		}
	}
	for _, s := range w93Spellings {
		o := by["spell_"+s]
		if strings.Contains(o.oT, check.W89E492) {
			fail = append(fail, fmt.Sprintf(":%s already answered E492 before this phase, so it proves nothing", s))
		}
		if !strings.Contains(o.oT, w93NoName) {
			fail = append(fail, fmt.Sprintf(":%s did not report the buffer on the input binary", s))
		}
		if !strings.Contains(o.nT, check.W89E492) {
			fail = append(fail, fmt.Sprintf(":%s does not answer E492: a removed name has been inherited", s))
		}
	}
	for _, name := range []string{"cmd_filter", "cmd_fixdel"} {
		o := by[name]
		if strings.Contains(o.nT, check.W89E492) && !strings.Contains(o.oT, check.W89E492) {
			fail = append(fail, fmt.Sprintf("%s: a surviving command became unknown", name))
		}
	}
	if o := by["ctrl_g"]; !strings.Contains(o.nT, `"[No Name]" [Modified] 2 lines --100%--`) {
		fail = append(fail, "ctrl_g: CTRL-G no longer reports the buffer, and it is what `:file` printed and what must survive")
	}
	if o := by["g_ctrl_g"]; !strings.Contains(o.nT, "Col 1 of 1") {
		fail = append(fail, "g_ctrl_g: `g CTRL-G` printed no count")
	}
	if o := by["quit_modified"]; !strings.Contains(o.nT, "E37: No write since last change") {
		fail = append(fail, "quit_modified: :q no longer refuses on a modified buffer, and that is the :q phase's to change")
	}
	for _, p := range []struct{ Name, want string }{
		{"editing", "alpha"}, {"reg_percent", "hello"}, {"reg_hash", "hello"},
	} {
		if o := by[p.Name]; !strings.Contains(o.nT, p.want) {
			fail = append(fail, fmt.Sprintf("%s: the new binary no longer shows %s, so \"it did not move\" is two failures agreeing", p.Name, check.CutilRepr(p.want)))
		}
	}
	o = by["registers"]
	if !strings.Contains(o.nS, "Type Name Content") || !strings.Contains(o.oS, "Type Name Content") {
		fail = append(fail, "registers: :registers printed no table, so \"it did not move\" is two failures agreeing")
	}
	for _, p := range []struct{ Tag, stream string }{{"the input binary", o.oS}, {"this one", o.nS}} {
		if strings.Contains(p.stream, `"%`) || strings.Contains(p.stream, `"#`) {
			fail = append(fail, fmt.Sprintf("registers: %s printed a `\"%%` or `\"#` line, and neither has been printable since phase 88", p.Tag))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("the corpus cannot see a buffer being named: `cmd_file` types")
		r.Cont("`:file` with no argument and records the CTRL-G line for")
		r.Cont("[No Name].  `file_rename` is the only probe that can tell a")
		r.Cont("removed command from a changed message, and `cp_missing` the")
		r.Cont("only one that shows the old binary asking the filesystem.")
		return harness.ErrReported
	}
	r.Say("probes: %d moved (%s), %d unchanged", len(moved), strings.Join(moved, " "), len(static))
	r.Cont("the input binary answered `\"NEWNAME\" [Modified][Not edited]` to CTRL-G after `:file NEWNAME`, and found `nosuchfile` absent from the disk through CTRL-P; five spellings answer E492 and none did before")
	r.Cont("and what did not move is doing its work: CTRL-G and g CTRL-G, :registers with its table and without a `\"%%` or `\"#` line, the %% and # registers, :filter, :fixdel, :ls, :q on a modified buffer and :q!")
	return nil
}

func w93Pty(r *check.Rep, old, bin string) error {
	home, err := os.MkdirTemp("", "whim93-home-")
	if err != nil {
		return err
	}
	session := func(binary string, keys [][]byte) (string, int, error) {
		d, err := os.MkdirTemp("", "whim93-pty-")
		if err != nil {
			return "", -1, err
		}
		text, status, err := harness.Session(binary, nil, keys, "xterm",
			20*time.Second, 600*time.Millisecond, d, check.W85Env(home), 0, 0)
		return string(text), status, err
	}
	rename := [][]byte{[]byte("ityped on a terminal\x1b"), []byte(":file NEWNAME\r"), []byte("\x07"), []byte(":q!\r")}
	edit := [][]byte{[]byte("ialpha\rbeta\x1b"), []byte("ggdwA-tail\x1b"), []byte("u"), []byte(":q!\r")}
	var fail []string
	o, _, err := session(old, rename)
	if err != nil {
		return err
	}
	n, _, err := session(bin, rename)
	if err != nil {
		return err
	}
	if !strings.Contains(o, "NEWNAME") {
		fail = append(fail, "the pty session did not rename the buffer on the input binary, so it proves nothing")
	}
	if strings.Contains(n, `NEWNAME"`) {
		fail = append(fail, "the pty session renamed the buffer on the new binary")
	}
	if !strings.Contains(n, "E492") || !strings.Contains(n, "[No Name]") {
		fail = append(fail, fmt.Sprintf("the pty session does not answer E492 and [No Name]: %s", check.CutilRepr(check.Tail200(n))))
	}
	eo, eos, err := session(old, edit)
	if err != nil {
		return err
	}
	en, ens, err := session(bin, edit)
	if err != nil {
		return err
	}
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
	r.Say("a real terminal: `:file NEWNAME` then CTRL-G renames on the binary this phase was handed and answers E492 and [No Name] here, and an ordinary editing session is identical either side")
	return nil
}
