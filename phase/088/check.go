package p088

// Whim phase 88, the check -- argv is `+{command}` and `-T {term}`, and nothing else.
// See phase/088/edit.go, and GOALS.md.
//
// Runs after phase/088/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `enums-before`, that binary's DWARF enumerator values.
//
// FOUR THINGS ARE PROVED HERE, and only the first is a grep.
//
// 1. THE CUT.  Six identifiers at zero mentions, the two strings that went with
// them, and -- against the phases that come after -- `read_stdin`'s 23 remaining
// mentions, every one of them the PARAMETER that the "nothing reads a byte"
// phase owns, and `read_cmd_fd`'s twelve, which the same phase owns.  A phase
// that reached past its own boundary would fail here rather than quietly widen.
//
// 2. THE RENUMBERING.  `main_errors[]` is indexed by the ME_* enumerators, so
// removing ME_TOO_MANY_ARGS moves the three after it.  That is exactly what
// CLAUDE.md says a build is perfectly happy to do wrongly, so it is checked
// against DWARF and not against the build: every enumerator in the binary this
// phase was handed must be in the one it made with the same value, except
// ME_TOO_MANY_ARGS and the three EDIT_*, which must be gone, and ME_ARG_MISSING,
// ME_GARBAGE and ME_EXTRA_CMD, which must each be exactly one lower.  There are
// 1,327 of them and a wrong table index would not show up anywhere else.
//
// 3. THE PROBES, in two halves, run on BOTH binaries.  The declared delta
// (tools/st.sh delta) says exactly six of the 30 command lines moved and
// nothing else did, against baselines recorded from whim-vim -- but the
// baselines are one recording of one binary, so they cannot say "the old one
// opened the file".  These say it:
//
// MUST DIFFER   a file argument, two file arguments, a bare `-` with text on
// stdin, `--`, `-- +q!` and `+q! f.txt`, each also required to
// show the OLD behaviour on the OLD binary -- a probe that only
// looks at the new binary passes on a phase that did nothing.
// MUST NOT      every `+{command}` form including `+set paste`, the three `-T`
// spellings and an unknown terminal name, the options that were
// already unknown, an ordinary keystroke edit, and a pty session.
//
// 4. THE INSTRUMENT SWAP, which this phase is the cause of.  termcheck is
// whim's, and it asks its question with a file argument.  From this boundary on
// that is an unknown option and all nineteen of its rows read `(none)` -- so
// the core's recording now uses `ztermcheck`, which is termcheck.py with its
// ask() replaced and nothing else.  Both halves are measured here: the new tool
// records the baseline's nineteen rows byte for byte from the binary this phase
// was handed, and the old tool records nothing but `(none)` from the one it made.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py): mainerr() prints the version banner, which carries __DATE__ and
// __TIME__, so two binaries built a minute apart disagree on stderr for a reason
// that is not the editor's behaviour.

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
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim88", Check) }

var (
	z5Gone     = []string{"had_minmin", "edit_type", "EDIT_NONE", "EDIT_FILE", "EDIT_STDIN", "ME_TOO_MANY_ARGS", "buflist_add"}
	z5GoneText = []string{"Too many edit arguments", "read_stdin(void)", "read_stdin();"}
	z5Kept     = []string{"MAX_ARG_CMDS", "ME_EXTRA_CMD", "ME_GARBAGE", "ME_UNKNOWN_OPTION",
		"ME_ARG_MISSING", "mainerr_arg_missing", "want_argument", "case 'T':",
		"if (argv[0][0] == '+')", "p_paste"}
	z5MEWant = [][2]string{{"ME_UNKNOWN_OPTION", "0"}, {"ME_ARG_MISSING", "1"},
		{"ME_GARBAGE", "2"}, {"ME_EXTRA_CMD", "3"}}
	z5MERe   = regexp.MustCompile(`(?m)^enum \{ (ME_\w+) = (\d+) \};$`)
	z5RowsRe = regexp.MustCompile(`(?s)(?m)^static char \*\(main_errors\[\]\) =\n\{\n(.*?)^\};\n`)
	z5Unk    = "Unknown option argument"
)

// Whim88 is phase 88's check: argv ends as `+{command}` and `-T {term}`.
//
// FOUR THINGS ARE PROVED HERE and only the first is a grep.  The cut.  The
// RENUMBERING, against DWARF and not against the build, because main_errors[]
// is indexed by the ME_* enumerators and CLAUDE.md's own warning is that a
// build is perfectly happy to renumber a table index wrongly.  The probes, on
// both binaries, because the baselines are one recording of one binary and
// cannot say "the old one opened the file".  And the INSTRUMENT SWAP this phase
// causes: termcheck asks with a file argument, which is an unknown option from
// here on.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim88 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noargv", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim88")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. what the cut removed ---------------------------------------------
	for _, g := range z5Gone {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range z5GoneText {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	// AND WHAT IT DID NOT TOUCH: `read_stdin` as a parameter and `read_cmd_fd`
	// are the stdin phase's, counted so that taking them HERE fails here.
	if n := check.Occurrences(`\bread_stdin\b`, src); n != 23 {
		return stop("read_stdin has %d mentions, expected the 23 that are the parameter the stdin phase owns", n)
	}
	if n := check.Occurrences(`\bread_cmd_fd\b`, src); n != 12 {
		return stop("read_cmd_fd has %d mentions, expected 12: the assignment was argv's, the readers are not", n)
	}
	r.Say("0 mentions of all seven; read_stdin's 23 parameter mentions and read_cmd_fd's 12 readers untouched")

	// --- 2. what it deliberately kept ----------------------------------------
	for _, k := range z5Kept {
		if !strings.Contains(string(src), k) {
			return stop("'%s' went, and argv ends as +{command} and -T {term}", k)
		}
	}
	if !check.HasLinePrefix(src, "exe_commands(") {
		return stop("exe_commands went, and with it every +{command}")
	}
	// The ME_* enumerators are main_errors[]'s indices: 0..3, in table order.
	pairs := z5MERe.FindAllStringSubmatch(string(src), -1)
	got := make([][2]string, len(pairs))
	for i, p := range pairs {
		got[i] = [2]string{p[1], p[2]}
	}
	if !z5PairsEq(got, z5MEWant) {
		return stop("the ME_* enumerators are %s, expected %s", z5PairsRepr(got), z5PairsRepr(z5MEWant))
	}
	rows := z5RowsRe.FindStringSubmatch(string(src))
	if rows == nil {
		return stop("main_errors[] has no rows, expected 5: four the enumerators index and the one whim left unreachable")
	}
	if n := len(strings.Split(strings.TrimRight(rows[1], "\n"), "\n")); n != 5 {
		return stop("main_errors[] has %d rows, expected 5: four the enumerators index and the one whim left unreachable", n)
	}
	if strings.Contains(rows[1], "Too many edit arguments") {
		return stop("main_errors[] still carries the ME_TOO_MANY_ARGS row")
	}
	r.Say("kept: +{command} with MAX_ARG_CMDS, -T with want_argument, ME_UNKNOWN_OPTION for the rest, exe_commands, 'paste'")

	// --- 3. the compile, the linkage and the libc surface --------------------
	// NOTHING IS FREED HERE, stated as an equality so that a symbol ARRIVING --
	// which a fold can do -- fails.
	before := check.ReadFile(filepath.Join(state, "symbols", "undefined"))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := check.ReadFile(".cache/symbols/last/undefined")
	if before != after {
		r.Say("the libc surface moved, and this phase frees nothing:")
		for _, l := range check.DiffLines(before, after) {
			r.Cont("  %s", l)
		}
		return harness.ErrReported
	}
	r.Say("symbols %s, the same set: nothing this cut removed was libc's last caller",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the enumerators, from DWARF --------------------------------------
	enumsAfter := filepath.Join(tmp, "enums-after")
	if err := exec.Command("sh", "tools/enumvals.sh", f, enumsAfter).Run(); err != nil {
		return fmt.Errorf("tools/enumvals.sh refused")
	}
	if err := z5Enums(r, check.ReadFile(filepath.Join(state, "enums-before")), check.ReadFile(enumsAfter)); err != nil {
		return err
	}

	// --- 5. the binary -------------------------------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	now, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes", beforeLines, check.CountLines(now), check.SizeOf(bin))

	// --- 6. the probes, both halves ------------------------------------------
	if err := z5Probes(r, old, bin); err != nil {
		return err
	}

	// --- 7. a real terminal --------------------------------------------------
	if err := z5Pty(r, old, bin); err != nil {
		return err
	}

	// --- 8. the instrument this phase broke, and the one that replaced it ----
	base := ".reference/core-baselines/ref-term.txt"
	termOld := filepath.Join(tmp, "term-old")
	if err := exec.Command("sh", "tools/st.sh", "ztermcheck", old, termOld).Run(); err != nil {
		return fmt.Errorf("ztermcheck refused")
	}
	if check.ReadFile(base) != check.ReadFile(termOld) {
		r.Say("ztermcheck does not record the baseline from the input binary:")
		for i, l := range check.DiffLines(check.ReadFile(base), check.ReadFile(termOld)) {
			if i >= 5 {
				break
			}
			r.Cont("  %s", l)
		}
		return harness.ErrReported
	}
	termFile := filepath.Join(tmp, "term-file")
	_ = exec.Command("sh", "tools/st.sh", "termcheck", bin, termFile).Run()
	if !strings.Contains(check.ReadFile(termFile), "(none)") {
		r.Say("termcheck still works on this binary -- then the file")
		r.Cont("  argument was not removed, and the recording need not have changed")
		return harness.ErrReported
	}
	r.Say("terminal table: ztermcheck records the baseline's %d rows from the input binary; termcheck's file argument now gives %d empty ones",
		check.CountLines([]byte(check.ReadFile(base))), check.CountLinesWith([]byte(check.ReadFile(termFile)), "(none)"))
	return nil
}

func z5PairsEq(a, b [][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// z5PairsRepr is Python's %r of a list of tuples, because the refusal message
// is compared byte for byte against the shell's.
func z5PairsRepr(p [][2]string) string {
	q := make([]string, len(p))
	for i, v := range p {
		q[i] = fmt.Sprintf("('%s', '%s')", v[0], v[1])
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// z5Enums is the DWARF comparison: 1,327 enumerators, of which four must be
// gone, three must be exactly one lower, and every other must be unmoved.  A
// wrong table index would not show up anywhere else.
func z5Enums(r *check.Rep, beforeTxt, afterTxt string) error {
	load := func(s string) map[string]string {
		m := map[string]string{}
		for _, l := range strings.Split(s, "\n") {
			if i := strings.IndexByte(l, '='); i > 0 {
				m[l[:i]] = strings.TrimRight(l[i+1:], "\n")
			}
		}
		return m
	}
	before, after := load(beforeTxt), load(afterTxt)
	goneSet := map[string]bool{"ME_TOO_MANY_ARGS": true, "EDIT_NONE": true, "EDIT_FILE": true, "EDIT_STDIN": true}
	moved := map[string][2]string{"ME_ARG_MISSING": {"2", "1"}, "ME_GARBAGE": {"3", "2"}, "ME_EXTRA_CMD": {"4", "3"}}
	var fail []string
	gs := make([]string, 0, len(goneSet))
	for n := range goneSet {
		gs = append(gs, n)
	}
	sort.Strings(gs)
	for _, n := range gs {
		if _, ok := before[n]; !ok {
			fail = append(fail, fmt.Sprintf("%s was not in the input binary at all, so its removal proves nothing", n))
		}
		if v, ok := after[n]; ok {
			fail = append(fail, fmt.Sprintf("%s survives with value %s", n, v))
		}
	}
	ms := make([]string, 0, len(moved))
	for n := range moved {
		ms = append(ms, n)
	}
	sort.Strings(ms)
	for _, n := range ms {
		wasNow := moved[n]
		if before[n] != wasNow[0] || after[n] != wasNow[1] {
			fail = append(fail, fmt.Sprintf("%s is %s -> %s, expected %s -> %s",
				n, pyNone(before[n]), pyNone(after[n]), wasNow[0], wasNow[1]))
		}
	}
	var still, movedN, lost []string
	for n := range before {
		if goneSet[n] {
			continue
		}
		if _, ok := moved[n]; ok {
			continue
		}
		still = append(still, n)
		if v, ok := after[n]; ok && v != before[n] {
			movedN = append(movedN, n)
		} else if !ok {
			lost = append(lost, n)
		}
	}
	sort.Strings(movedN)
	sort.Strings(lost)
	if len(movedN) > 0 {
		fail = append(fail, fmt.Sprintf("%d enumerators renumbered and were not to: %s", len(movedN), strings.Join(head8(movedN), " ")))
	}
	if len(lost) > 0 {
		fail = append(fail, fmt.Sprintf("%d enumerators left the binary and were not to: %s", len(lost), strings.Join(head8(lost), " ")))
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("main_errors[] is indexed by these, and the build cannot see a")
		r.Cont("wrong index.  DWARF can.")
		return harness.ErrReported
	}
	r.Say("enumerators: %d in, %d out; 4 gone, 3 renumbered by one, %d unmoved",
		len(before), len(after), len(still)-len(movedN))
	return nil
}

func pyNone(s string) string {
	if s == "" {
		return "None"
	}
	return s
}

func head8(s []string) []string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

var _ = sha256.Sum256
var _ = hex.EncodeToString
var _ = sync.WaitGroup{}
var _ = time.Second
var _ = io.Discard
