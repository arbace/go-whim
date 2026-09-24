package p105

// Whim phase 105, the check -- the variadic collapse.
// See phase/105/edit.go, and GOALS.md II.4c.
//
// Runs after phase/105/edit.go and the sweep internal/verify runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with the boundary's own flags.
//
// WHAT IS CLAIMED, in four parts:
//
// STRUCTURE   `va_start` appears EXACTLY ONCE, and in `vim_snprintf`.  That is the
// whole product of the phase and it is asserted as a count plus an owner
// test, which is three lines and needs no tool.  Seven names at 0
// mentions, five helpers at their measured counts, `vim_snprintf` moved by
// exactly the arithmetic of the edit.
// WARNINGS    `-Wformat=2` gives THE IDENTICAL 115 `-Wformat-nonliteral` warnings IN
// THE IDENTICAL 53 FUNCTIONS before and after.  This is the strongest
// cheap check available and the one a mangled expansion would fail while
// the build did not: the format-checking attribute moves from the wrapper
// to `vim_snprintf`, and it only lands on the same expressions if every
// argument list came out right.
// SYMBOLS     `nm -u` is THE SAME SET, asserted as a `comm` that is empty in BOTH
// directions.  It is 18 names here and 17 in the boundary's own binary,
// because tools/phasecheck.sh compiles plain -O0 and so adds
// __stack_chk_fail, which the -fno-stack-protector build does not have --
// and that is why the assertion is the equality and not the number.  A
// reader meeting a 129-site phase expects a symbol to fall and none can:
// a pure restructure inside one translation unit frees nothing.  The
// binary GROWS, which is the same fact wearing its other face, and the
// check reports the number rather than letting it look like a mistake.
// BEHAVIOUR   the declared delta is NOTHING AT ALL, so tools/st.sh delta proves the
// recording did not move.  And the recording is NEARLY BLIND to this
// phase, which is measured rather than asserted (see below), so the phase
// owes probes: 263 of them on both binaries, and FOUR DELIBERATE BREAKS
// that say what the probes can and cannot see.
//
// HOW BLIND THE RECORDING IS -- MEASURED, with the input source built again with
// `write(2, "ZW|<wrapper>|<format>\n", ...)` at the entry to each of the seven.  Across
// the 102 screen cases: `vim_snprintf_safelen` 617 entries, `smsg_attr_keep` 6,
// `vim_snprintf_add` 2, and `smsg` 0, `smsg_attr` 0, `semsg` 0, `siemsg` 0.  `semsg` IS
// 94 OF THE 129 SITES AND THE SCREEN CORPUS ENTERS IT NOT ONCE.  That is the whole
// argument for probes.  The 263 below enter `semsg` 232 times over 34 distinct formats.
//
// THE FOUR BREAKS, AND TWO OF THEM MOVE NOTHING ON PURPOSE.  Each is this phase's own
// output with one thing wrong, built and run against the input binary:
//
// b1  both room helpers return 20 instead of IOSIZE            129 of 263 differ
// b2  the wrong tail -- every `semsg` site reports as an        155 of 263 differ
// ordinary message instead of an error
// b3  `safelen_result`'s clamp reduced to `return str_l;`         0 of 263 differ
// b4  all three guards removed                                    0 of 263 differ
//
// b3 AND b4 ARE KEPT AT ZERO RATHER THAN DROPPED.  They are the honest statement of what
// the evidence cannot reach: the clamp needs a message longer than 1,025 bytes out of
// `fileinfo`, and the guards need `IObuff == NULL` (an out-of-memory failure of the
// first two allocations the process makes) or a `semsg` under `emsg_off > 0` whose
// scribble on `IObuff` somebody then reads.  Reporting them as 0 is the difference
// between "the probes prove the guards are load-bearing" -- which would be false -- and
// "the guards are correct by construction and the probes say so about b1 and b2".
//
// WHAT THE CHECK DELIBERATELY DOES NOT ASSERT: `vim_snprintf`'s mention count BEFORE the
// edit.  Phase 104 formats its host message with `vim_snprintf`, so that number is the
// message layer's and moves under it; this phase asserts the count AFTER, as the
// transformer's own arithmetic against whatever it was handed.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim105", Check) }

const (
	z22RoomC = "iobuff_room(void)\n{\n    if (IObuff == NULL)\n    {\n        return 0;\n    }\n    return  (1024+1) ;\n}"
	z22ERoom = "emsg_iobuff_room(void)\n{\n    if (IObuff == NULL || emsg_not_now())\n    {\n        return 0;\n    }\n    return  (1024+1) ;\n}"
	z22Or    = "iobuff_or(const char *s)\n{\n    if (IObuff == NULL)\n    {\n        return (char *)s;\n    }\n    return (char *)IObuff;\n}"
	z22Clamp = "    return ((size_t)str_l >= str_m) ? str_m - 1 : (size_t)str_l;"
)

// z22Tails is re.subn(r'(?<![A-Za-z0-9_])emsg\(iobuff_or\(', 'msg(iobuff_or(', t).
func z22Tails(t string) (string, int) {
	const want = "emsg(iobuff_or("
	var b strings.Builder
	n, i := 0, 0
	for {
		k := strings.Index(t[i:], want)
		if k < 0 {
			b.WriteString(t[i:])
			return b.String(), n
		}
		k += i
		if k > 0 && isWordByte(t[k-1]) {
			b.WriteString(t[i : k+1])
			i = k + 1
			continue
		}
		b.WriteString(t[i:k])
		b.WriteString("msg(iobuff_or(")
		n++
		i = k + len(want)
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// Whim105 is phase 105's check: the variadic collapse.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim105 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "format", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	tmp, err := os.MkdirTemp("", "whim105")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags, ldflags := strings.Fields(check.Z9Flag(mk, "CFLAGS")), strings.Fields(check.Z9Flag(mk, "LDFLAGS"))
	newC, oldC := check.ReadFile(f), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }

	// --- 1. the four controls ------------------------------------------------
	for _, t := range []string{z22RoomC, z22ERoom, z22Or, z22Clamp} {
		if strings.Count(newC, t) != 1 {
			return stop("the helper `%s` is not in the output exactly once, so the controls below would not be controls", strings.TrimSpace(strings.SplitN(t, "(", 2)[0]))
		}
	}
	ret20 := func(s string) string { return strings.ReplaceAll(s, "return  (1024+1) ;", "return 20;") }
	b1 := strings.ReplaceAll(strings.ReplaceAll(newC, z22RoomC, ret20(z22RoomC)), z22ERoom, ret20(z22ERoom))
	b2, n2 := z22Tails(newC)
	if n2 != 94 {
		return stop("b2 rewrote %d `emsg(iobuff_or(` tails, expected 94", n2)
	}
	b3 := strings.ReplaceAll(newC, z22Clamp, "    return (size_t)str_l;")
	b4 := strings.ReplaceAll(newC, z22RoomC, "iobuff_room(void)\n{\n    return  (1024+1) ;\n}")
	b4 = strings.ReplaceAll(b4, z22ERoom, "emsg_iobuff_room(void)\n{\n    return  (1024+1) ;\n}")
	b4 = strings.ReplaceAll(b4, z22Or, "iobuff_or(const char *s)\n{\n    (void)s;\n    return (char *)IObuff;\n}")
	for _, p := range [][2]string{{"b1", b1}, {"b2", b2}, {"b3", b3}, {"b4", b4}} {
		if p[1] == newC {
			return stop("%s changed nothing", p[0])
		}
		os.WriteFile(filepath.Join(tmp, p[0]+".c"), []byte(p[1]), 0o644)
	}
	r.Say("four controls written: b1 a 20-byte IObuff, b2 the wrong tail at all 94 `semsg` sites, b3 safelen_result's clamp, b4 all three guards")
	breaks := map[string]chan struct{}{}
	for _, b := range []string{"b1", "b2", "b3", "b4"} {
		ch := make(chan struct{})
		breaks[b] = ch
		go func(b string) {
			defer close(ch)
			a := append(append(append([]string{}, cflags...), ldflags...), "-o", filepath.Join(tmp, b), filepath.Join(tmp, b+".c"))
			c := exec.Command("gcc", a...)
			lf, _ := os.Create(filepath.Join(tmp, b+".log"))
			c.Stderr = lf
			c.Run()
			lf.Close()
		}(b)
	}
	waitBreaks := func() {
		for _, ch := range breaks {
			<-ch
		}
	}
	defer waitBreaks()

	// --- 2. the source -------------------------------------------------------
	var fail []string
	mentions := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(t, -1))
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"smsg", 12}, {"smsg_attr", 4}, {"smsg_attr_keep", 2}, {"semsg", 96}, {"siemsg", 12},
		{"vim_snprintf_add", 3}, {"vim_snprintf_safelen", 13}, {"va_start", 8}, {"va_list", 15},
		{"va_end", 10}, {"va_arg", 21}, {"va_copy", 2}} {
		if m := mentions(oldC, p.Name); m != p.want {
			fail = append(fail, fmt.Sprintf("the input has %d mentions of `%s`, expected %d", m, p.Name, p.want))
		}
	}
	var left []string
	for _, n := range strings.Fields("smsg smsg_attr smsg_attr_keep semsg siemsg vim_snprintf_add vim_snprintf_safelen") {
		if mentions(newC, n) > 0 {
			left = append(left, n)
		}
	}
	if len(left) > 0 {
		fail = append(fail, "these names should be at 0 mentions and are not: "+strings.Join(left, " "))
	}
	starts := regexp.MustCompile(`\bva_start\b`).FindAllStringIndex(newC, -1)
	if len(starts) != 1 {
		fail = append(fail, fmt.Sprintf("`va_start` has %d mentions, expected exactly 1 -- that single mention IS this phase", len(starts)))
	} else {
		owner := ""
		for _, m := range check.Z22Head.FindAllStringSubmatchIndex(newC, -1) {
			if m[0] < starts[0][0] {
				owner = newC[m[2]:m[3]]
			}
		}
		if owner != "vim_snprintf" {
			o := owner
			if o == "" {
				o = "<none>"
			}
			fail = append(fail, fmt.Sprintf("the one `va_start` is inside `%s`, not `vim_snprintf` -- the survivor must be the function the split moves", o))
		}
	}
	vsWant := mentions(oldC, "vim_snprintf") - 1 + 129
	for _, p := range []struct {
		Name string
		want int
		why  string
	}{
		{"va_list", 8, "the prototype and definition of `vim_vsnprintf`, the same of `vim_vsnprintf_typval` plus its local, `vim_snprintf`'s local and `skip_to_arg`'s two parameters -- FOUR functions, and GOALS.md II.4c names three"},
		{"va_end", 3, "vim_snprintf's, vim_vsnprintf_typval's two"},
		{"va_arg", 21, "UNCHANGED, all of them inside vim_vsnprintf_typval"},
		{"va_copy", 2, "UNCHANGED, likewise"},
		{"vim_snprintf", vsWant, "one prototype away -- the redundant second one, which existed only because the message wrappers sat above it -- and one mention at each of the 129 sites"},
		{"vim_vsnprintf", 3, "a prototype, the definition and vim_snprintf's one call: the six calls the wrappers made are gone"},
		{"vim_vsnprintf_typval", 3, "a prototype, the definition and vim_vsnprintf's call -- vim_snprintf_safelen called it directly and now goes through vim_vsnprintf, which is the same function with NULL for its fifth argument"},
		{"iobuff_room", 15, "a prototype, the definition and the 13 smsg/smsg_attr/smsg_attr_keep sites"},
		{"emsg_iobuff_room", 106, "a prototype, the definition and the 104 semsg/siemsg sites"},
		{"iobuff_or", 119, "a prototype, the definition and one at each of the 117 message sites"},
		{"safelen_result", 13, "a prototype, the definition and the 11 sites"},
		{"append_room", 3, "a prototype, the definition and the one site"},
		{"emsg_not_now", 5, "UNCHANGED, and it is a coincidence worth stating: the phase ADDS a forward declaration and a call from emsg_iobuff_room(), and REMOVES semsg's and siemsg's two calls.  5 -> 5"},
		{"emsg_core", 5, "the definition and the four calls in emsg/iemsg/internal_error -- semsg's two and siemsg's three are gone"},
		{"msg", 63, "+10 sites, -2 in smsg's deleted body"},
		{"emsg", 218, "+94 sites, and semsg's body named it not once"},
		{"iemsg", 45, "+10 sites"},
		{"msg_attr", 21, "+2 sites, -2 in smsg_attr's deleted body: UNCHANGED"},
		{"msg_attr_keep", 6, "+1 site, -2 in smsg_attr_keep's deleted body"},
		{"IObuff", 204, "the 117 message sites, the helpers' four, and the 15 the seven deleted definitions took with them"},
	} {
		if m := mentions(newC, p.Name); m != p.want {
			fail = append(fail, fmt.Sprintf("`%s` has %d mentions, expected %d -- %s", p.Name, m, p.want, p.why))
		}
	}
	for _, p := range []string{"static int emsg_not_now(void);", "static size_t iobuff_room(void);",
		"static size_t emsg_iobuff_room(void);", "static char *iobuff_or(const char *s);",
		"static size_t safelen_result(char *str, size_t str_m, int str_l);",
		"static size_t append_room(char *str, size_t str_m);"} {
		if strings.Count(newC, p+"\n") != 1 {
			fail = append(fail, fmt.Sprintf("the declaration `%s` is not in the output exactly once", p))
		}
	}
	rows := check.Z6RowRe.FindAllString(newC, -1)
	got, _ := harness.CommandNamesIn([]byte(newC), "whim-vim.c")
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, "cmdnames[] is not the 98 rows phase 93 left -- this phase touches no Ex command")
	}
	if i := strings.Index(newC, "static struct vimoption options[]"); i >= 0 {
		j := strings.Index(newC[i:], "\n};")
		if m := len(check.Z12RowRe.FindAllString(newC[i:i+j], -1)); m != 107 {
			fail = append(fail, fmt.Sprintf("options[] has %d rows, expected the 107 phase 103 left -- this phase removes no option", m))
		}
	}
	nd, allInc := 0, true
	L := strings.Split(newC, "\n")
	for _, l := range L {
		if strings.HasPrefix(l, "#") {
			nd++
			if !strings.HasPrefix(l, "#include <") {
				allInc = false
			}
		}
	}
	if nd != 11 || !allInc {
		fail = append(fail, "the output does not have exactly the eleven `#include` directives phase 104 left.  <stdarg.h> STAYS: the one surviving va_start needs it, and it leaves at the SPLIT and not here")
	}
	for k := 1; k < len(L); k++ {
		if L[k] == "" && L[k-1] == "" {
			fail = append(fail, "there is a run of two blank lines, which canon.sh should have taken")
			break
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("`va_start` 8 -> 1 AND THE ONE IS INSIDE `vim_snprintf`; `va_list` 15 -> 8, `va_end` 10 -> 3, `va_arg` and `va_copy` untouched at 21 and 2.  The eight surviving `va_list` mentions are in FOUR functions -- vim_snprintf, vim_vsnprintf, vim_vsnprintf_typval and skip_to_arg -- where GOALS.md II.4c names three")
	r.Cont("the seven wrappers are at 0 mentions; `vim_snprintf` %d -> %d, one redundant prototype away and one mention at each of the 129 sites; five helpers at 15, 106, 119, 13 and 3; the tails at msg 63, emsg 218, iemsg 45, msg_attr 21 and msg_attr_keep 6, every one of them a function that already existed", mentions(oldC, "vim_snprintf"), mentions(newC, "vim_snprintf"))
	r.Cont("cmdnames[] 98 unchanged, options[] 107 unchanged, eleven #includes unchanged -- <stdarg.h> stays for the one va_start and leaves at the SPLIT -- and no run of two blank lines")

	// --- 3. -Wformat=2 -------------------------------------------------------
	wf := func(src string) (int, []string) {
		c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-Wformat=2", "-fsyntax-only", src)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		txt := eb.String()
		n := strings.Count(txt, "[-Wformat-nonliteral]")
		var fns []string
		for _, m := range check.Z22InFunc.FindAllStringSubmatch(txt, -1) {
			if m[1] != "" {
				fns = append(fns, m[1])
			} else {
				fns = append(fns, m[2])
			}
		}
		return n, fns
	}
	type wres struct {
		n   int
		fns []string
	}
	wch := make(chan wres, 1)
	go func() { n, fns := wf(filepath.Join(state, "old.c")); wch <- wres{n, fns} }()
	nn, fn := wf(f)
	wo := <-wch
	no, fo := wo.n, wo.fns
	uniq := func(s []string) map[string]bool {
		m := map[string]bool{}
		for _, v := range s {
			m[v] = true
		}
		return m
	}
	if no != nn || strings.Join(fo, "\x00") != strings.Join(fn, "\x00") {
		r.Say("-Wformat=2 MOVED: %d warnings in %d function headings before, %d in %d after", no, len(fo), nn, len(fn))
		a, b := uniq(fo), uniq(fn)
		var ab, ba []string
		for k := range a {
			if !b[k] {
				ab = append(ab, k)
			}
		}
		for k := range b {
			if !a[k] {
				ba = append(ba, k)
			}
		}
		sort.Strings(ab)
		sort.Strings(ba)
		if len(ab) > 0 {
			r.Cont("functions that stopped warning: %s", strings.Join(ab, " "))
		}
		if len(ba) > 0 {
			r.Cont("functions that started: %s", strings.Join(ba, " "))
		}
		return harness.ErrReported
	}
	if no != 115 || len(uniq(fo)) != 53 {
		return stop("-Wformat=2 gives %d warnings in %d distinct functions, expected 115 in 53 -- the equality above still holds, but this is not the tree the phase was measured on", no, len(uniq(fo)))
	}
	r.Say("-Wformat=2: THE IDENTICAL %d `-Wformat-nonliteral` warnings in THE IDENTICAL %d functions, before and after.  Every `semsg` format in this file is `_(e_name)` over a `static char e_name[]` array -- there is not one string literal at a semsg site anywhere -- so the coverage does not disappear, it MOVES from the wrapper's attribute to vim_snprintf's, and it only lands on the same expressions if all 129 argument lists came out right", no, len(uniq(fo)))

	// --- 4. the compile, the linkage and the libc surface --------------------
	beforeU := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(beforeU, after), check.Comm23(after, beforeU)
	if len(goneU) > 0 || len(cameU) > 0 {
		r.Say("the libc surface MOVED, and this phase frees nothing and takes")
		r.Cont("nothing on:")
		r.Cont("gone: %s", check.TrSpace(goneU))
		r.Cont("came: %s", check.TrSpace(cameU))
		r.Cont("A PURE RESTRUCTURE INSIDE ONE TRANSLATION UNIT FREES NOTHING:")
		r.Cont("vim_vsnprintf_typval still does every conversion, in this file,")
		r.Cont("and <stdarg.h>'s three names are macros and a builtin type.")
		r.Cont("The symbols go when the formatter LEAVES, and that is the split.")
		return harness.ErrReported
	}
	for _, absent := range strings.Fields("open creat openat stat access fcntl getcwd strerror fopen fdopen opendir fclose getc putc fsync exit _exit") {
		if check.Contains(after, absent) {
			return stop("%s is undefined, and no phase since 96 has put it back", absent)
		}
	}
	r.Say("symbols %s -> %s, THE SAME SET as a comm that is empty in BOTH directions -- nothing left and nothing arrived.  That equality is the headline: eight functions calling va_start cannot be split and one can, but until the formatter LEAVES THE FILE no symbol can move",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")), strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 5. the enumerators --------------------------------------------------
	evo, evn := filepath.Join(tmp, "ev.old"), filepath.Join(tmp, "ev.new")
	ech := make(chan error, 1)
	go func() { ech <- exec.Command("sh", "tools/enumvals.sh", filepath.Join(state, "old.c"), evo).Run() }()
	if err := exec.Command("sh", "tools/enumvals.sh", f, evn).Run(); err != nil {
		<-ech
		return harness.ErrReported
	}
	if err := <-ech; err != nil {
		return harness.ErrReported
	}
	evRead := func(p string) map[string]string {
		m := map[string]string{}
		for _, l := range strings.Split(strings.TrimRight(check.ReadFile(p), "\n"), "\n") {
			if l == "" {
				continue
			}
			k := strings.LastIndex(l, "=")
			m[l[:k]] = l[k+1:]
		}
		return m
	}
	eo, en := evRead(evo), evRead(evn)
	var egone, ecame, emoved []string
	for k, v := range eo {
		if nv, ok := en[k]; !ok {
			egone = append(egone, k)
		} else if nv != v {
			emoved = append(emoved, k)
		}
	}
	for k := range en {
		if _, ok := eo[k]; !ok {
			ecame = append(ecame, k)
		}
	}
	if len(egone)+len(ecame)+len(emoved) > 0 {
		for _, p := range []struct {
			What  string
			names []string
		}{{"went", egone}, {"arrived", ecame}, {"renumbered", emoved}} {
			if len(p.names) > 0 {
				sort.Strings(p.names)
				r.Say("enumerators %s: %s", p.What, strings.Join(check.Head(p.names, 20), " "))
			}
		}
		return harness.ErrReported
	}
	r.Say("%d enumerators, and not one went, arrived or renumbered", len(eo))

	// --- 6. the binary -------------------------------------------------------
	b := &check.Rep{Tag: "build", W: w}
	_ = exec.Command("make", "-C", work, "clean").Run()
	if _, err := os.Stat(filepath.Join(work, "whim-vim")); err == nil {
		b.Say("the clean did not remove whim-vim")
		return harness.ErrReported
	}
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		b.Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	typ, Interp, Dyn, Rel := check.Z22Readelf(bin)
	if typ != "EXEC" || Interp != 0 || Dyn != 1 || Rel != 1 {
		(&check.Rep{Tag: "static", W: w}).Say("NOT absolutely static: type %s, INTERP %d, no-dynamic %d, no-relocations %d", typ, Interp, Dyn, Rel)
		return harness.ErrReported
	}
	ob, nb := check.SizeOf(old), check.SizeOf(bin)
	b.Say("ok, %s -> %d lines, %d -> %d bytes: EXEC, no INTERP, no dynamic section, 0 relocations", beforeLines, check.CountLines([]byte(check.ReadFile(f))), ob, nb)
	if nb <= ob {
		b.Say("THE BINARY DID NOT GROW, and it must: 129 call sites now carry")
		b.Cont("a format call and a tail call where they carried one call, and")
		b.Cont("at -O0 that is the expected sign.  A binary that shrank means")
		b.Cont("the expansion did not happen where it was counted.")
		return harness.ErrReported
	}
	waitBreaks()
	for _, bb := range []string{"b1", "b2", "b3", "b4"} {
		if fi, e := os.Stat(filepath.Join(tmp, bb)); e != nil || fi.Mode()&0o111 == 0 {
			r.Say("the control %s did not build:", bb)
			for _, l := range check.Head(strings.Split(strings.TrimRight(check.ReadFile(filepath.Join(tmp, bb+".log")), "\n"), "\n"), 5) {
				fmt.Fprintln(w, "               "+l)
			}
			return harness.ErrReported
		}
	}

	// --- 7. the probes -------------------------------------------------------
	return z22Probes(r, old, bin, filepath.Join(tmp, "b1"), filepath.Join(tmp, "b2"), filepath.Join(tmp, "b3"), filepath.Join(tmp, "b4"))
}
