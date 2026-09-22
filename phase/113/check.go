package p113

// Whim phase 113, the check -- the message fold.
// See phase/113/edit.go, and GOALS.md II.4c.
//
// Runs after phase/113/edit.go and the sweep tools/phaserun.sh runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old.c`, the source this phase was HANDED,
// and `old`, that source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// WHAT IS CLAIMED, in seven parts:
//
// ARITHMETIC  computed FROM THE INPUT: the fold is +1 line and the sweep takes 91,
// `msg_puts_printf` and `vim_strlen_maxlen` go 3 -> 0, TWO function
// definitions leave, `msg_use_printf` STAYS at 6 -- the test is alive
// and only the arm went -- the eleven directives are where they were,
// and tools/canon.sh is a NO-OP.
// THE PAIR    the phase's whole positive evidence, and it is phase 95's shape
// because this is phase 95's kind of dead.  The INPUT source built
// twice, once with `write(2, "PP-ENTERED\n", 11)` as the first statement
// of `msg_puts_printf()` and once with the IDENTICAL instrument in
// `msg_puts_display()`: 0 of 106 records against 103 of 106.  A
// prediction about a branch nothing takes has nothing but an instrument
// to confirm it.
// STILL TRUE  and the other half of "which kind of dead".  `msg_use_printf()` is NOT
// dead: instrumented on THIS PHASE'S OWN OUTPUT it answers TRUE 23
// times, every one at `msg_clr_eos_force()` and every one in
// ref-argv.txt's `mainerr` rows -- and `full_screen` is FALSE in all 23,
// so the body it guards does nothing.  0 of 23, measured at the arm.
// PROBES      32 stream probes and 4 deadly-signal probes, IDENTICAL on the binary
// this phase was handed and on its own.  Every one of the 32 is a way of
// making `msg_use_printf()` TRUE: at `t_TI` (`-T debug`, `:set t_ti=X`
// five ways, `+set t_ti=X`, `:set term=debug`), at `termcap_active`
// (`:stop`, `:set term=...`, which calls clear_termoptions() ->
// stoptermcap(), and the stoptermcap() inside mch_exit), at `full_screen`
// (SIGHUP/SIGTERM, whose deathtrap() clears it) and at `screen_valid`
// (`:set lines=1`, `:set columns=1`).
// THE TWO     THE FINDING THIS PHASE OWES, AND IT IS A RESULT AND NOT AN OMISSION.
// Three further folds are built HERE, from this phase's own output, and
// every one of them MUST MOVE a probe: `msg_clr_eos_force()` folded to
// its false arm, the same site guarded by `msg_check_screen()` instead,
// and `exit_scroll()`'s printf arm folded to `out_char('\n')`.  The
// first two are the bug a corpus-only check would ship; the third is
// ALIVE and phase 104 named it dead.
// SYMBOLS     `nm -u` is THE SAME SET, as a `comm` empty in BOTH directions, and
// `main` is the only external symbol.  Phase 104 already took `printf`,
// `fprintf`, `fflush` and `stderr`, so removing their last non-caller
// frees nothing: the assertion is an EQUALITY, which phase 104 predicted
// in as many words.
// THE CUT     `make editor.c`'s rule, run here: the prefix above the first
// `#include` is 0 directives, 0 errors under `-fsyntax-only`, and its
// warning set -- the core -> host boundary -- is compared WITH THE
// INPUT'S, name by name, at run time.  It is never written out: phase 111
// renames one of those names, and a check that spelled the set would
// fail on a tree that is exactly right (GOALS.md, "count them as a
// rule and not as a table of constants").
//
// THE RECORDING CANNOT FAIL THIS PHASE AND CANNOT PASS IT EITHER, and that is stated
// rather than discovered.  All three rejected folds record BYTE-IDENTICALLY too --
// `screen_fill()` returns early on `ScreenLines == nullptr`, and `ScreenLines` is NULL
// in all 23 `mainerr` cases, which are the only 23 places the predicate is ever TRUE
// in a recording.  So `diff -r` is necessary here and is not the check; the
// instrumented pair and the 36 probes are.
// tools/zpty.py
// tools/zrecord.sh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim113", Check) }

type z30Probe struct {
	Name string
	Args []string
	Keys []string
	term string
}

// Thirty-two ways of making msg_use_printf() TRUE, on a pipe.  It is TRUE when
// `!msg_check_screen()` -- full_screen FALSE or screen_valid(FALSE) FALSE -- or
// when `swapping_screen() && !termcap_active`, and swapping_screen() needs t_TI
// non-empty.  ORDER IS OUTPUT: the table file is these rows in this order.
var z30Probes = []z30Probe{
	{"T_debug", []string{"-T", "debug"}, []string{":set nosuchopt\r", ":q!\r"}, ""},
	{"T_debug_quit", []string{"-T", "debug"}, []string{"ihello\x1b", ":q!\r"}, ""},
	{"T_dumb", []string{"-T", "dumb"}, []string{":set nosuchopt\r", ":q!\r"}, ""},
	{"t_ti_set_err", nil, []string{":set t_ti=X\r", ":set nosuchopt\r", ":q!\r"}, ""},
	{"t_ti_te_quit", nil, []string{":set t_ti=X t_te=Y\r", ":q!\r"}, ""},
	{"t_ti_plus", []string{"+set t_ti=X"}, []string{":set nosuchopt\r", ":q!\r"}, ""},
	{"t_ti_reglist", nil, []string{":set t_ti=X\r", ":registers\r", "q", "\x1b", ":q!\r"}, ""},
	{"t_ti_ctrl_g", nil, []string{":set t_ti=X\r", "\x07", ":q!\r"}, ""},
	{"t_ti_modified", nil, []string{":set paste\r", "ix\x1b", ":set t_ti=X\r", ":q\r", ":q!\r"}, ""},
	{"ctrl_c_clean", nil, []string{"\x03"}, ""},
	{"ctrl_c_changed", nil, []string{":set paste\r", "ixyz\x1b", "\x03"}, ""},
	{"lines_1", nil, []string{":set lines=1\r", ":set nosuchopt\r", ":q!\r"}, ""},
	{"columns_1", nil, []string{":set columns=1\r", ":set nosuchopt\r", ":q!\r"}, ""},
	{"unknown_term", nil, []string{":set nosuchopt\r", ":q!\r"}, "no-such-term-9x"},
	{"bad_option", []string{"-y"}, []string{""}, ""},
	{"T_missing", []string{"-T"}, []string{""}, ""},
	{"t_ti_more", nil, []string{":set t_ti=X\r", ":set all\r", "q", "\x1b", ":q!\r"}, ""},
	{"t_ti_hitenter", nil, []string{":set t_ti=X\r", ":map\r", "\r", ":q!\r"}, ""},
	{"debug_more", []string{"-T", "debug"}, []string{":set all\r", "q", "\x1b", ":q!\r"}, ""},
	{"debug_hitenter", []string{"-T", "debug"}, []string{":map\r", "\r", ":q!\r"}, ""},
	{"t_ti_stopterm", nil, []string{":set t_ti=X\r", ":set t_te=Y\r", "ZQ"}, ""},
	{"empty_keys", nil, []string{""}, ""},
	{"term_debug", nil, []string{":set term=debug\r", ":set nosuchopt\r", ":q!\r"}, ""},
	{"term_debug_x2", nil, []string{":set term=debug\r", ":set term=debug\r", ":q!\r"}, ""},
	{"term_unknown", nil, []string{":set term=no-such-term-9x\r", ":q!\r"}, ""},
	{"term_after_ti", nil, []string{":set t_ti=X\r", ":set term=xterm\r", ":q!\r"}, ""},
	{"term_from_debug", []string{"-T", "debug"}, []string{":set term=debug\r", ":q!\r"}, ""},
	{"term_stop_debug", []string{"-T", "debug"}, []string{":stop\r", ":q!\r"}, ""},
	{"term_plus_bad", []string{"-T", "debug", "+set nosuchopt"}, []string{":q!\r"}, ""},
	{"term_dumb_set", nil, []string{":set term=dumb\r", ":set nosuchopt\r", ":q!\r"}, ""},
	{"term_ti_then_ti", nil, []string{":set term=debug\r", ":set t_ti=X\r", ":set all\r", "q", "\x1b", ":q!\r"}, ""},
	{"term_msg_after", nil, []string{":set term=debug\r", "\x07", ":q!\r"}, ""},
}

type z30Sig struct {
	Name string
	Keys []string
	sig  syscall.Signal
	Args []string
}

// Four deadly signals on a real pty, with fd 2 on a pipe of its own.
var z30Sigs = []z30Sig{
	{"hup_clean", []string{""}, syscall.SIGHUP, nil},
	{"hup_msg", []string{"\x07"}, syscall.SIGHUP, nil},
	{"term_msg", []string{"\x07"}, syscall.SIGTERM, nil},
	{"hup_dbg", []string{"\x07"}, syscall.SIGHUP, []string{"-T", "debug"}},
}

func z30Keys(ks []string) [][]byte {
	Out := make([][]byte, len(ks))
	for i, k := range ks {
		Out[i] = []byte(k)
	}
	return Out
}

func z30Sha12(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:12]
}

func z30Head(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

// z30Table is zprobe.py: every probe in order, one row each.
func z30Table(binary, outPath string) {
	var rows []string
	for _, p := range z30Probes {
		term := p.term
		if term == "" {
			term = "xterm"
		}
		_, so, se, rc, err := harness.ZSession(binary, z30Keys(p.Keys), term, p.Args, 24, 80, 20*time.Second)
		if err == harness.ErrBlocked {
			rows = append(rows, fmt.Sprintf("%-16s BLOCKED", p.Name))
			continue
		}
		if err != nil {
			return
		}
		so = check.Z30Stamp.ReplaceAll(so, []byte("compiled <date>"))
		se = check.Z30Stamp.ReplaceAll(se, []byte("compiled <date>"))
		rows = append(rows, fmt.Sprintf("%-16s rc=%-8s out=%-6d sha=%s err=%-4d %s",
			p.Name, strconv.Itoa(rc), len(so), z30Sha12(so), len(se), check.Z30BytesRepr(z30Head(se, 80))))
	}
	os.WriteFile(outPath, []byte(strings.Join(rows, "\n")+"\n"), 0o644)
}

// z30SigTable is zsig.py: the four signal cases in order.
func z30SigTable(binary, outPath string) {
	var rows []string
	for _, c := range z30Sigs {
		Out, errb, st, err := harness.PtySplit(binary, c.Args, z30Keys(c.Keys), c.sig,
			600*time.Millisecond, "xterm", 24, 80)
		if err != nil {
			return
		}
		rows = append(rows, fmt.Sprintf("%-12s st=%-6s out=%-6d sha=%s err=%-4d %s",
			c.Name, strconv.Itoa(st), len(Out), z30Sha12(Out), len(errb), check.Z30BytesRepr(z30Head(errb, 40))))
	}
	os.WriteFile(outPath, []byte(strings.Join(rows, "\n")+"\n"), 0o644)
}

// z30Moved is `diff a b | sed -n 's/^> \([A-Za-z_0-9]*\) .*/\1/p' | tr '\n' ' '`,
// and the count of `^>` lines beside it.
func z30Moved(a, b string) (string, int) {
	s, n := "", 0
	for _, l := range check.Z30Diff(a, b) {
		if strings.HasPrefix(l, ">") {
			n++
		}
		if m := check.Z30GT.FindStringSubmatch(l); m != nil {
			s += m[1] + " "
		}
	}
	return s, n
}

// z30Out is `sed -n 's/^NAME  *KEY=[0-9]*  *out=\([0-9]*\).*/\1/p' FILE`.
func z30Out(path, name, key string) string {
	re := regexp.MustCompile(`^` + name + ` +` + key + `=[0-9]* +out=([0-9]*)`)
	s := ""
	for _, l := range strings.Split(check.ReadFile(path), "\n") {
		if m := re.FindStringSubmatch(l); m != nil {
			s += m[1] + "\n"
		}
	}
	return strings.TrimRight(s, "\n")
}

// Whim113 is phase 113's check: the message fold.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim113 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	oldC := filepath.Join(state, "old.c")
	r := &check.Rep{Tag: "msgfold", W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	beforeRaw := strings.TrimRight(check.ReadFile(filepath.Join(state, "input-lines")), "\n")
	tmp, err := os.MkdirTemp("", "whim113-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflagsS, ldflagsS := "", ""
	if m := check.Z29CFlags.FindStringSubmatch(mk); m != nil {
		cflagsS = m[1]
	}
	if m := check.Z29LDFlags.FindStringSubmatch(mk); m != nil {
		ldflagsS = m[1]
	}
	link := func(src, Out string) error {
		a := append(append(strings.Fields(cflagsS), strings.Fields(ldflagsS)...), "-o", Out, src)
		c := exec.Command("gcc", a...)
		c.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
		return c.Run()
	}

	// The product build is written first and waited for below.
	var wgNew, wgBG sync.WaitGroup
	var errNew error
	wgNew.Add(1)
	go func() { defer wgNew.Done(); errNew = link(f, filepath.Join(tmp, "new")) }()

	// --- 0. the five other binaries this check is made of ----------------------
	newT, oldT := check.ReadFile(f), check.ReadFile(oldC)
	type ctl struct{ Name, Text string }
	var files []ctl
	// cA, cB -- THE FOLD THAT CANNOT BE MADE SAFELY, twice.
	const A = "msg_clr_eos_force(void)\n{\n    if (msg_use_printf())"
	if strings.Count(newT, A) != 1 {
		return stop("msg_clr_eos_force's test is not in the output exactly once, so the two " +
			"controls that make this phase's central finding would not be controls")
	}
	files = append(files, ctl{"cA", strings.Replace(newT, A, "msg_clr_eos_force(void)\n{\n    if (0)", 1)})
	files = append(files, ctl{"cB", strings.Replace(newT, A, "msg_clr_eos_force(void)\n{\n    if (!msg_check_screen())", 1)})
	// cC -- exit_scroll's printf arm folded to the else arm it already has.
	const S4 = `        if (msg_use_printf())
        {
            if (info_message)
            {
                host_message("\n", -1, FALSE);
            }
            else
            {
                host_message("\r\n", -1, TRUE);
            }
        }
        else
        {
            out_char('\n');
        }
`
	if strings.Count(newT, S4) != 1 {
		return stop("exit_scroll's printf arm is not in the output exactly once, so the control " +
			"that shows it is ALIVE would not be a control")
	}
	files = append(files, ctl{"cC", strings.Replace(newT, S4, "        out_char('\\n');\n", 1)})
	// i_pp, i_ctl -- THE INSTRUMENTED PAIR, on the source this phase was HANDED.
	const MARK = "    write(2, \"PP-ENTERED\\n\", 11);\n"
	const P = "msg_puts_printf(char_u *str, int maxlen)\n{\n"
	const D = "msg_puts_display(char_u      *str, int         maxlen, int         attr, " +
		"int         recurse)\n{\n"
	for _, x := range []struct{ Tag, head string }{{"i_pp", P}, {"i_ctl", D}} {
		if strings.Count(oldT, x.head) != 1 {
			return stop("%s's definition is not in the INPUT exactly once, so the instrumented "+
				"pair would not be a pair", strings.SplitN(x.head, "(", 2)[0])
		}
		files = append(files, ctl{x.Tag, strings.Replace(oldT, x.head, x.head+MARK, 1)})
	}
	// i_s3 -- the predicate on THIS PHASE'S OWN OUTPUT.
	const C = "msg_clr_eos_force(void)\n{\n    if (msg_use_printf())\n    {\n" +
		"        if (full_screen)\n        {\n"
	if strings.Count(newT, C) != 1 {
		return stop("msg_clr_eos_force's two nested tests are not in the output exactly once")
	}
	files = append(files, ctl{"i_s3", strings.Replace(newT, C, "msg_clr_eos_force(void)\n{\n    if (msg_use_printf())\n"+
		"    {\n        write(2, \"T-cleos\\n\", 8);\n"+
		"        if (full_screen)\n        {\n"+
		"            write(2, \"C-fullscreen\\n\", 13);\n", 1)})
	for _, c := range files {
		if c.Text == newT && c.Text == oldT {
			return stop("%s changed nothing", c.Name)
		}
		if err := os.WriteFile(filepath.Join(tmp, c.Name+".c"), []byte(c.Text), 0o644); err != nil {
			return err
		}
	}
	r.Say("six controls written: cA msg_clr_eos_force folded to its false arm, cB " +
		"the same site guarded by msg_check_screen() instead, cC exit_scroll's printf " +
		"arm folded to out_char, i_pp and i_ctl the instrumented pair on the INPUT, " +
		"i_s3 the predicate instrumented on this phase's own output")

	for _, v := range []string{"cA", "cB", "cC", "i_pp", "i_ctl", "i_s3"} {
		v := v
		wgBG.Add(1)
		go func() { defer wgBG.Done(); link(filepath.Join(tmp, v+".c"), filepath.Join(tmp, v)) }()
	}
	// tools/canon.sh must be a NO-OP.
	canonC := filepath.Join(tmp, "canon.c")
	os.WriteFile(canonC, []byte(newT), 0o644)
	var wgCanon sync.WaitGroup
	var errCanon error
	var canonLog []byte
	wgCanon.Add(1)
	go func() {
		defer wgCanon.Done()
		canonLog, errCanon = exec.Command("sh", "tools/canon.sh", canonC).CombinedOutput()
	}()

	// --- 1. the source, as arithmetic on the input ------------------------------
	beforeLines, _ := strconv.Atoi(strings.TrimSpace(beforeRaw))
	mentions := func(text, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllStringIndex(text, -1))
	}
	O, N := strings.Split(oldT, "\n"), strings.Split(newT, "\n")
	if len(O)-1 != beforeLines {
		r.Bad("the state directory says the edit was handed %d lines and old.c has "+
			"%d", beforeLines, len(O)-1)
	}
	const CUT = 92
	if len(O)-len(N) != CUT-1 {
		r.Bad("the output is %d lines and the input was %d, a difference of %d where "+
			"%d was expected -- the edit adds 1 and the sweep takes %d: 75 for "+
			"msg_puts_printf and 1 blank, 13 for vim_strlen_maxlen and 1 blank, and "+
			"the two prototypes", len(N)-1, len(O)-1, len(O)-len(N), CUT-1, CUT)
	}
	for _, x := range []struct {
		Name         string
		oWant, nWant int
		why          string
	}{
		{"msg_puts_printf", 3, 0, "a prototype, a definition and one call"},
		{"vim_strlen_maxlen", 3, 0, "a prototype, a definition and its ONLY call, " +
			"which was inside msg_puts_printf -- A CHECK THAT " +
			"EXPECTS ONE FUNCTION REMOVED FAILS ON A CORRECT " +
			"PHASE"},
		{"msg_use_printf", 6, 6, "UNCHANGED, and it is the point: the test stays, is " +
			"still TRUE 23 times in a recording, and only the arm " +
			"behind it went.  A check that expected it at 0 would " +
			"fail on a correct phase"},
		{"msg_puts_display", 4, 4, "the false arm, untouched"},
		{"msg_clr_eos_force", 5, 5, "untouched -- folding its test is measurably wrong"},
		{"exit_scroll", 3, 3, "untouched -- its printf arm is ALIVE"},
		{"host_message", 10, 7, "four calls inside msg_puts_printf left and one arrived"},
		{"info_message", 9, 7, "four reads inside msg_puts_printf left and two arrived " +
			"-- it is a file-scope int and is in scope at the new " +
			"call site"},
		{"msg_didout", 29, 29, "msg_puts_printf's last statement left and the " +
			"replacement writes it"},
	} {
		gO, gN := mentions(oldT, x.Name), mentions(newT, x.Name)
		if gO != x.oWant || gN != x.nWant {
			r.Bad("`%s` is %d mentions in the input and %d in the output, where %d "+
				"and %d were expected -- %s", x.Name, gO, gN, x.oWant, x.nWant, x.why)
		}
	}
	const NEWARM = "    if (msg_use_printf())\n    {\n        host_message((char *)str, maxlen, " +
		"!info_message);\n        msg_didout = TRUE;\n    }\n"
	if strings.Count(newT, NEWARM) != 1 {
		r.Bad("the folded arm is not in the output exactly once")
	}
	for _, x := range []struct{ who, Text string }{
		{"hit_return_msg", "    if (!msg_use_printf())\n"},
		{"msg_clr_eos_force", "msg_clr_eos_force(void)\n{\n    if (msg_use_printf())\n"},
		{"exit_scroll", "        if (msg_use_printf())\n"},
	} {
		if strings.Count(newT, x.Text) != 1 {
			r.Bad("the %s site is not in the output exactly once, and this phase "+
				"touches exactly ONE of the four call sites", x.who)
		}
	}
	dirs := func(L []string) []int {
		var d []int
		for i, l := range L {
			if check.Z30Dir.MatchString(l) {
				d = append(d, i)
			}
		}
		return d
	}
	od, nd := dirs(O), dirs(N)
	if len(od) != 11 || len(nd) != 11 {
		r.Bad("the input has %d directives and the output %d; both must be 11", len(od), len(nd))
	} else {
		consec := true
		for k, i := range nd {
			if i != nd[0]+k {
				consec = false
			}
		}
		same := true
		for k := range nd {
			if N[nd[k]] != O[od[k]] {
				same = false
			}
		}
		if !consec {
			r.Bad("the output's eleven directives are not on eleven consecutive lines")
		} else if !same {
			r.Bad("the eleven directives are not the eleven the input had")
		} else if nd[0]-od[0] != len(N)-len(O) {
			r.Bad("the boundary moved by %d lines and the file by %d: the whole of this "+
				"phase is above the first `#include`", nd[0]-od[0], len(N)-len(O))
		}
	}
	if check.Z27Runs(N) != check.Z27Runs(O) {
		r.Bad("the edit and the sweep left %d runs of two blank lines where there "+
			"were %d", check.Z27Runs(N), check.Z27Runs(O))
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("%d lines -> %d: the edit adds ONE and the sweep takes %d -- "+
		"msg_puts_printf's 75 and its blank, vim_strlen_maxlen's 13 and its blank, and "+
		"the two prototypes.  TWO functions, not one: vim_strlen_maxlen's only call was "+
		"inside msg_puts_printf", len(O)-1, len(N)-1, CUT)
	r.Say("msg_puts_printf 3 -> 0 and vim_strlen_maxlen 3 -> 0, while " +
		"msg_use_printf STAYS AT 6 and msg_clr_eos_force, exit_scroll and " +
		"hit_return_msg's `!msg_use_printf()` are each still exactly one site.  The " +
		"predicate is alive; ONE of its four arms was dead")
	r.Say("host_message 10 -> 7 and info_message 9 -> 7 -- four calls and four " +
		"reads left with the function and one call and two reads arrived with the fold; " +
		"msg_didout is 29 either side, the removed function's last statement being the " +
		"replacement's second line")
	r.Say("the eleven directives are the input's own, consecutive, and the "+
		"boundary moved by exactly the lines the file lost -- every line this phase "+
		"touches is above the first `#include`.  Blank-line runs unmoved at %d", check.Z27Runs(N))

	// --- 2. canon.sh, and the tools with floors --------------------------------
	wgCanon.Wait()
	if errCanon != nil {
		r.Say("tools/canon.sh failed on the output:")
		for i, l := range strings.Split(string(canonLog), "\n") {
			if i >= 10 {
				break
			}
			fmt.Fprintln(w, l)
		}
		return harness.ErrReported
	}
	if !check.Z30Same(f, canonC) {
		r.Say("tools/canon.sh is not a no-op on the output -- the two lines the fold writes are not " +
			"written the way this file writes everything else:")
		for i, l := range check.Z30Diff(f, canonC) {
			if i >= 12 {
				break
			}
			fmt.Fprintln(w, l)
		}
		return harness.ErrReported
	}
	r.Say("tools/canon.sh is a NO-OP on the output: the two lines the fold writes are written the way " +
		"this file writes everything else")
	for _, tool := range []string{"orphanopts", "nvidx", "zhostonly"} {
		c := exec.Command("sh", "tools/st.sh", tool, f)
		c.Stdout, c.Stderr = w, w
		if err := c.Run(); err != nil {
			return harness.ErrReported
		}
	}
	// tools/create_cmdidxs.py -- named as a PATH so tools/implhash.sh hashes
	// it into this phase's key.  Do not delete it.
	names, _ := harness.CommandNames(f)
	if len(names) != 98 {
		return stop("names() reads %d command rows and must read 98 -- this phase "+
			"removes no Ex command", len(names))
	}
	r.Say("create_cmdidxs's names() reads 98 rows, orphanopts " +
		"and nvidx pass, and neither floor is approached: this phase " +
		"removes no command, no option row and no normal-mode row")

	// --- 3. the symbols, and the binary ----------------------------------------
	wgNew.Wait()
	if errNew != nil {
		return stop("the output did not build with '%s' '%s'", cflagsS, ldflagsS)
	}
	for _, x := range []struct{ src, obj string }{{oldC, "old.o"}, {f, "new.o"}} {
		if Out, err := exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o",
			filepath.Join(tmp, x.obj), x.src).CombinedOutput(); err != nil {
			w.Write(Out)
			return harness.ErrReported
		}
	}
	uOld := check.NmField26(filepath.Join(tmp, "old.o"), []string{"-u"}, 1)
	uNew := check.NmField26(filepath.Join(tmp, "new.o"), []string{"-u"}, 1)
	sp := func(xs []string) string {
		s := ""
		for _, x := range xs {
			s += x + " "
		}
		return s
	}
	if gone, came := check.Minus26(uOld, uNew), check.Minus26(uNew, uOld); len(gone)+len(came) > 0 {
		return stop("`nm -u` moved: gone [%s] arrived [%s].  Phase 104 took printf, fprintf, fflush and "+
			"stderr with the calls THEMSELVES, so removing the function that no longer made them frees "+
			"nothing -- this is an EQUALITY and phase 104 predicted it", sp(gone), sp(came))
	}
	ext := check.NmField26(filepath.Join(tmp, "new.o"), []string{"--extern-only", "--defined-only"}, 2)
	if extS := sp(ext); extS != "main " {
		return stop("the output defines external symbols other than main: %s", extS)
	}
	r.Say("`nm -u` is THE SAME SET, %d names, as a `comm` empty in BOTH directions, and `main` is still "+
		"the only external symbol.  Removing 75 lines that call nothing libc has frees nothing, which is "+
		"what phase 104 said when it took the four stdio symbols with the CALLS and left the function", len(uNew))
	r.Say("the binary is %d bytes against the input's %d", check.SizeOf(filepath.Join(tmp, "new")),
		check.SizeOf(filepath.Join(state, "old")))
	pc := exec.Command("sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols"))
	pc.Stdout, pc.Stderr = w, w
	if err := pc.Run(); err != nil {
		return harness.ErrReported
	}

	// --- 4. the editor.c cut, and its warning set compared WITH THE INPUT'S ------
	bset := map[string][]string{}
	cutN := map[string]int{}
	for _, x := range []struct{ side, src string }{{"old", oldC}, {"new", f}} {
		lines := check.Z28Cut(check.ReadFile(x.src))
		var dl []string
		for i, l := range lines {
			if check.Z30Dir.MatchString(l) {
				dl = append(dl, fmt.Sprintf("%d:%s", i+1, l))
			}
		}
		if len(dl) > 0 {
			r.Say("the %s cut holds a directive, so it found the wrong line:", x.side)
			for i, l := range dl {
				if i >= 3 {
					break
				}
				fmt.Fprintln(w, l)
			}
			return harness.ErrReported
		}
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		cp := filepath.Join(tmp, "ed."+x.side+".c")
		os.WriteFile(cp, []byte(text), 0o644)
		c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-Wall", "-Wextra", "-Wno-unused-parameter",
			"-fsyntax-only", cp)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		var errs, warns []string
		for _, l := range strings.Split(eb.String(), "\n") {
			if strings.Contains(l, ": error:") {
				errs = append(errs, l)
			}
			if strings.Contains(l, ": warning: ") && !strings.Contains(l, "used but never defined") {
				warns = append(warns, l)
			}
		}
		if len(errs) > 0 {
			r.Say("the %s cut does not parse on its own:", x.side)
			for i, l := range errs {
				if i >= 4 {
					break
				}
				fmt.Fprintln(w, l)
			}
			return harness.ErrReported
		}
		set := map[string]bool{}
		for _, m := range check.Z30Undef.FindAllStringSubmatch(eb.String(), -1) {
			set[m[1]] = true
		}
		bset[x.side] = check.Z27Keys(set)
		if len(warns) > 0 {
			r.Say("the %s cut has a warning that is not a boundary name:", x.side)
			for i, l := range warns {
				if i >= 3 {
					break
				}
				fmt.Fprintln(w, l)
			}
			return harness.ErrReported
		}
		cutN[x.side] = len(lines)
	}
	if strings.Join(bset["old"], " ") != strings.Join(bset["new"], " ") {
		return stop("the core -> host boundary moved: gone [%s] arrived [%s].  This phase adds ONE call to "+
			"host_message, which was already a boundary name",
			sp(check.Minus26(bset["old"], bset["new"])), sp(check.Minus26(bset["new"], bset["old"])))
	}
	r.Say("the `make editor.c` cut: %d lines -> %d, 0 directives, 0 errors under `-fsyntax-only`, and the "+
		"WHOLE warning set is the core -> host boundary -- %d names, IDENTICAL to the input's, compared name "+
		"by name at run time and never written out here (phase 111 renames one of them)",
		cutN["old"], cutN["new"], len(bset["new"]))

	// --- 5. the probes: 32 on a pipe and 4 on a pty, both sides ------------------
	wgBG.Wait()
	oldBin, _ := filepath.Abs(filepath.Join(state, "old"))
	binOf := func(v string) string {
		if v == "old" {
			return oldBin
		}
		return filepath.Join(tmp, v)
	}
	var wg sync.WaitGroup
	for _, v := range []string{"old", "new", "cA", "cB", "cC"} {
		v := v
		wg.Add(1)
		go func() { defer wg.Done(); z30Table(binOf(v), filepath.Join(tmp, "P."+v)) }()
	}
	for _, v := range []string{"old", "new", "cA", "cC"} {
		v := v
		wg.Add(1)
		go func() { defer wg.Done(); z30SigTable(binOf(v), filepath.Join(tmp, "S."+v)) }()
	}
	// The five recordings go at the same time.  The shell's bare `wait`
	// returned 0 whatever they did; a recording that fails now says why.
	recErr := make([]error, 5)
	for k, x := range []struct{ bin, src, Out string }{
		{oldBin, oldC, "REC.old"},
		{filepath.Join(tmp, "new"), f, "REC.new"},
		{filepath.Join(tmp, "i_pp"), filepath.Join(tmp, "i_pp.c"), "REC.i_pp"},
		{filepath.Join(tmp, "i_ctl"), filepath.Join(tmp, "i_ctl.c"), "REC.i_ctl"},
		{filepath.Join(tmp, "i_s3"), filepath.Join(tmp, "i_s3.c"), "REC.i_s3"},
	} {
		k, x := k, x
		wg.Add(1)
		go func() {
			defer wg.Done()
			recErr[k] = check.RecCmd("sh", "tools/zrecord.sh", x.bin, x.src, filepath.Join(tmp, x.Out))
		}()
	}
	wg.Wait()
	if check.RecReport(w, recErr...) {
		return harness.ErrReported
	}

	pf := func(v string) string { return filepath.Join(tmp, "P."+v) }
	sf := func(v string) string { return filepath.Join(tmp, "S."+v) }
	head12 := func(ls []string) {
		for i, l := range ls {
			if i >= 12 {
				break
			}
			fmt.Fprintln(w, l)
		}
	}
	if !check.Z30Same(pf("old"), pf("new")) {
		r.Say("the declared delta is NOTHING AT ALL and the 32 stream probes differ:")
		head12(check.Z30Diff(pf("old"), pf("new")))
		return harness.ErrReported
	}
	if !check.Z30Same(sf("old"), sf("new")) {
		r.Say("the four deadly-signal probes differ, which is the one thing this phase is most likely to " +
			"have broken by accident -- exit_scroll's arm:")
		head12(check.Z30Diff(sf("old"), sf("new")))
		return harness.ErrReported
	}
	r.Say("MUST NOT DIFFER: all 32 stream probes identical on the binary this phase was handed and on its " +
		"own -- every one a way of making msg_use_printf() TRUE, at t_TI, at termcap_active, at full_screen " +
		"and at screen_valid")
	r.Say("MUST NOT DIFFER: all 4 deadly-signal probes identical, on a real pty WITH FD 2 ON A PIPE OF ITS " +
		"OWN -- hup_clean, hup_msg, term_msg, hup_dbg.  Three of the four reach exit_scroll's printf arm, so " +
		"they are the evidence that this phase did not disturb it")

	// THE FINDING, AND IT IS A RESULT AND NOT AN OMISSION: the three folds
	// that look like this one and are not safe.
	for _, x := range []struct{ v, What string }{
		{"cA", "msg_clr_eos_force's test folded to its FALSE arm"},
		{"cB", "msg_clr_eos_force's test replaced by msg_check_screen(), which drops the swapping_screen() " +
			"&& !termcap_active disjunct"},
	} {
		if check.Z30Same(pf(x.v), pf("new")) {
			return stop("%s (%s) moves NONE of the 32 stream probes, so this phase's central finding -- that "+
				"the fold cannot be made safely -- is no longer measured", x.v, x.What)
		}
		n, d := z30Moved(pf("new"), pf(x.v))
		if n != "t_ti_stopterm " {
			return stop("%s moves [%s] and was measured to move exactly t_ti_stopterm", x.v, n)
		}
		r.Say("MUST DIFFER -- %s: %s.  It moves %d of the 32 probes, and it is %s: %s bytes -> %s",
			x.v, x.What, d, n, z30Out(pf("new"), "t_ti_stopterm", "rc"), z30Out(pf(x.v), "t_ti_stopterm", "rc"))
	}
	if check.Z30Same(sf("cA"), sf("new")) {
		return stop("cA moves none of the four signal probes, and hup_clean was measured to go 2,124 -> 2,142")
	}
	r.Say("AND THE SECOND MEASUREMENT OF THE SAME HAZARD: cA moves hup_clean, %s bytes -> %s, the extra "+
		"eighteen being an escape sequence that erases the last line of a screen the editor has just declared "+
		"unusable, AFTER `Vim: Finished.`  THE CORPUS CANNOT SEE EITHER: screen_fill() returns early on "+
		"ScreenLines == nullptr and it is NULL in all 23 mainerr cases, so a phase checked only against the "+
		"recording would ship this", z30Out(sf("new"), "hup_clean", "st"), z30Out(sf("cA"), "hup_clean", "st"))
	sc, _ := z30Moved(pf("new"), pf("cC"))
	if sc != "t_ti_more debug_more term_ti_then_ti " {
		return stop("cC (exit_scroll's printf arm folded to out_char) moves [%s] of the 32 stream probes and "+
			"was measured to move exactly t_ti_more, debug_more and term_ti_then_ti", sc)
	}
	sg, _ := z30Moved(sf("new"), sf("cC"))
	if sg != "hup_msg term_msg hup_dbg " {
		return stop("cC moves [%s] of the four signal probes and was measured to move exactly hup_msg, "+
			"term_msg and hup_dbg", sg)
	}
	r.Say("MUST DIFFER -- cC: exit_scroll's printf arm folded to out_char('\\n').  IT IS ALIVE AND PHASE 104 "+
		"WAS WRONG TO NAME IT A FOLLOW-UP BESIDE msg_puts_printf.  With NO SIGNAL AT ALL it moves [%s] -- each "+
		"`:set t_ti=X` or `-T debug`, a paged `:set all`, exit -- and with a signal it moves [%s].  What moves "+
		"is two bytes from FD 2 TO FD 1: out_char('\\n') emits `\\r` first, so the bytes on the wire are the "+
		"same two, and on a pty where both descriptors are the same device the combined stream is "+
		"byte-identical.  That is why tools/zpty.py cannot see it, why folding it would be UNDECLARABLE, and "+
		"why it belongs to the phase that decides the core writes nothing to fd 2 at all", sc, sg)

	// --- 6. the recording, and the instrumented pair ----------------------------
	if dl := check.DiffRQ(filepath.Join(tmp, "REC.old"), filepath.Join(tmp, "REC.new")); len(dl) > 0 {
		r.Say("the declared delta is NOTHING AT ALL and the two recordings differ:")
		head12(dl)
		return harness.ErrReported
	}
	r.Say("the declared delta is NOTHING AT ALL and TWO FULL RECORDINGS ARE BYTE-IDENTICAL -- 102 screen " +
		"cases, every Ex command, every command line, the pty scenarios and the terminal table")

	marks := func(rec, token string) ([]string, int, int) {
		var hit []string
		total, nfiles := 0, 0
		for _, Rel := range check.WalkFiles(rec) {
			nfiles++
			d, err := os.ReadFile(filepath.Join(rec, Rel))
			if err != nil {
				continue
			}
			if c := strings.Count(string(d), token); c > 0 {
				hit = append(hit, Rel)
				total += c
			}
		}
		sort.Strings(hit)
		return hit, total, nfiles
	}
	r6 := &check.Rep{Tag: "msgfold", W: w}
	pp, ppN, nRec := marks(filepath.Join(tmp, "REC.i_pp"), "PP-ENTERED")
	ctlH, ctlN, _ := marks(filepath.Join(tmp, "REC.i_ctl"), "PP-ENTERED")
	if nRec < 100 {
		r6.Bad("a recording is %d records, and a measurement over a corpus nothing "+
			"wrote passes.  The count is REPORTED and not pinned: it was 106 when "+
			"this phase was written -- 102 screen cases and four files -- and is "+
			"122 since phase 123 added the memline corpus", nRec)
	}
	if len(pp) > 0 {
		show := pp
		if len(show) > 4 {
			show = show[:4]
		}
		r6.Bad("the instrument inside msg_puts_printf() marks %d of %d records (%s), "+
			"and the whole claim of this phase is that it marks NONE", len(pp), nRec, strings.Join(show, " "))
	}
	if len(ctlH) < 100 {
		r6.Bad("the CONTROL -- the identical instrument in msg_puts_display() -- marks "+
			"only %d of %d records, so the instrument is not working and the 0 "+
			"above proves nothing", len(ctlH), nRec)
	}
	tr, trN, _ := marks(filepath.Join(tmp, "REC.i_s3"), "T-cleos")
	_, fsN, _ := marks(filepath.Join(tmp, "REC.i_s3"), "C-fullscreen")
	if trN != 23 || strings.Join(tr, " ") != "ref-argv.txt" {
		where := strings.Join(tr, " ")
		if where == "" {
			where = "nothing"
		}
		r6.Bad("msg_use_printf() answers TRUE %d times in %s and was measured at 23, "+
			"all in ref-argv.txt -- one per mainerr row.  It is NOT dead and this "+
			"phase does not claim it is", trN, where)
	}
	if fsN > 0 {
		r6.Bad("`full_screen` is TRUE %d times inside those 23, and was measured at 0: "+
			"the body msg_use_printf() guards there does nothing", fsN)
	}
	if err := r6.Done(); err != nil {
		return err
	}
	r.Say("THE INSTRUMENTED PAIR, phase 95's shape because this is phase 95's kind "+
		"of dead: the INPUT source built twice with the identical "+
		"`write(2, \"PP-ENTERED\\n\", 11)`, first in msg_puts_printf() -- %d of %d records, "+
		"%d occurrences -- and then in msg_puts_display() -- %d of %d records, %d "+
		"occurrences.  A prediction about a branch nothing takes has nothing but an "+
		"instrument to confirm it, and a control is what says the instrument works",
		len(pp), nRec, ppN, len(ctlH), nRec, ctlN)
	r.Say("AND msg_use_printf() IS STILL ALIVE AFTER THE FOLD, measured on this "+
		"phase's OWN output: TRUE %d times, every one at msg_clr_eos_force() and every "+
		"one in %s -- one per mainerr row -- with `full_screen` FALSE %d of %d, so the "+
		"body it guards is a no-op.  THIS IS PHASE 95'S KIND OF DEAD AND NOT PHASE 92'S: "+
		"the branch CAN be taken and never is, so the evidence owed is an instrument at "+
		"the site, a control, and probes -- not an argument that the code cannot run",
		trN, strings.Join(tr, " "), trN-fsN, trN)
	return nil
}
