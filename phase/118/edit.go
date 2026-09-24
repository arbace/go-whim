package p118

// Whim phase 118 -- the core calls nothing but the host.  GOALS.md II.4c, GOALS.md.
//
// THREE LIBC FUNCTIONS ARE LEFT IN THE CORE AND THIS PHASE MOVES ALL THREE.  Everything
// else the core still asks the operating system for went out through a named call in an
// earlier phase -- the terminal, the signals, the window size and the sleep at phase 103,
// the exit at 102, the messages at 104, the clock at 109 and 111 -- and what survived is the
// three the editor uses so constantly that nobody looked at them: `malloc`, `free` and
// `write`.  The user's words, 2026-09-19: "malloc, free and write should be moved to
// host, then I guess there is no functional dependency in core beyond host."
//
// malloc   its prototype and every call of it -- lalloc(), and since phase 117 the
// malloc-copy-free that replaced ga_grow_inner's and get_keystroke's realloc
// free     its prototype and every call -- vim_free(), update_wincolor(), and phase
// 117's two again
// write    its prototype and mch_write() -- every byte the editor draws
//
// HOW MANY OF EACH IS READ OFF THE TEXT AND NOT WRITTEN HERE.  This phase asserted the
// counts once, was handed a boundary where phase 117 had changed two of them, and refused;
// what it asserts now is a PARTITION -- every mention above the boundary is the
// declaration or a call, and each call is rewritten -- which is the same claim about a
// file this phase has never seen (CLAUDE.md, *Rename a name across the whole file*).
//
// so the core gets three declarations and the host three definitions:
//
// static void *host_alloc(usize n);        beside host_exit and host_message,
// static void host_free(void *p);          at the end of the ONE run of
// static int host_write(const char *s, int len);   core -> host prototypes
//
// THE WRAPPERS ARE FAITHFUL AND NOT IMPROVED, which is the trap this phase could fall
// into without any recording seeing it.  mch_write() is
//
// vim_ignored = (int)write(1, (char *)s, len);
//
// -- ONE write(2), no loop, and the count assigned to the variable this tree keeps for
// results it means to ignore.  A short write therefore LOSES those bytes today, and
// host_write() must lose them too: a wrapper that looped would be a behaviour change in
// a phase that declares none, and the corpus cannot tell the two apart because nothing
// in it makes a write to fd 1 come up short.  The same rule for the other two:
// host_alloc() returns what malloc() returned, nullptr included, so lalloc()'s
// clear_sb_text()/do_outofmem_msg() failure path is reached exactly as before; and
// host_free() calls free(), so it is null-safe for the same reason free() is.  The core
// does not rely on that -- vim_free() tests `x != nullptr` and update_wincolor() frees
// only the arm it allocated -- but the wrapper inherits it rather than adding a test.
//
// WHY host_write() DROPS THE DESCRIPTOR AND THE OTHER TWO KEEP THEIR SIGNATURE.  The
// core's two neighbours on this boundary already name no fd: phase 103's
// `musl_read_input(char *buf, int len)` reads fd 0 inside the host, and phase 104's
// `host_message(const char *msg, int len, int err)` chooses between fd 2 and fd 1 from a
// FLAG, not from a number the core passes.  A descriptor is the host's idea of where the
// screen is; `host_write(s, len)` is the core's -- "these bytes go to the screen" -- and
// it is the output side of musl_read_input, spelled the same way.  host_alloc() and
// host_free() have no such question: a size and a pointer are all there ever was.
//
// THE INT RETURN IS THE ONE PLACE A CAST MOVES.  `(int)write(...)` in the core becomes
// `(int)write(...)` in the host, so the value mch_write() stores in vim_ignored is the
// same bits; what changes is which side of the boundary the narrowing happens on, and
// it happens where the libc type is visible, which is the point of the whole file split.
//
// WHAT THIS PHASE IS FOR, AND IT IS A PROPERTY OF THE BLOCK AND NOT OF THESE THREE
// NAMES.  Above the first `#include` the core carries a run of ORDINARY (non-`static`)
// declarations -- the libc it calls, declared by hand since the headers went below it at
// phase 110.  This edit does not assume what is in that run: it finds it, requires the
// three lines it owns to be in it, takes exactly those three out, and prints what is
// left.  When the run is EMPTY the core names no libc function at all, and every
// outward call it makes is a `musl_` or a `host_`.  That is the arc's claim, and the
// check states it as a measurement of the output rather than as a sentence written here.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.RegisterArgs("whim118", Edit) }

var (
	z35Seed    = "void *malloc(usize n);"
	z35CastFd1 = regexp.MustCompile(`\(int\)write\(1, `)
)

// Whim118 makes the core call nothing but the host: `malloc`, `free` and `write`
// become `host_alloc`, `host_free` and `host_write`, three prototypes above the
// boundary and three definitions below it.
//
// ITS ANCHORS ARE A PARTITION AND NOT A COUNT, and phase 117 is why.  They first
// asserted `malloc` at 2 mentions, `free` at 3 and `write` at 2 -- the counts
// measured on one boundary -- and phase 117's realloc rewrite took two of them to
// 4 and 5.  A count is a fact about a tree that WAS measured; a partition is a
// fact about the tree that arrives.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	p := edit.Ph{Tag: "hostcall", W: w}
	if len(args) != 1 {
		return nil, p.Die("usage: edit whim118 <file> <state-dir>")
	}
	state := args[0]
	t := string(text)

	mentions := func(s, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(s, -1))
	}
	blankRuns := func(s string) int {
		L := strings.Split(s, "\n")
		n := 0
		for i := 1; i < len(L); i++ {
			if L[i] == "" && L[i-1] == "" {
				n++
			}
		}
		return n
	}
	swap := func(old, new, what, why string) error {
		if c := strings.Count(t, old); c != 1 {
			return p.Die("%s occurs %d times, expected 1 -- %s", what, c, why)
		}
		t = strings.Replace(t, old, new, 1)
		return nil
	}
	shortName := func(l string) string { return edit.Z36Name.ReplaceAllString(l, "$1") }

	L := strings.Split(t, "\n")
	linesBefore := len(L) - 1
	runsBefore := blankRuns(t)

	// ---- 0. the boundary -----------------------------------------------------
	var directives []int
	for i, l := range L {
		if edit.Z36Dir.MatchString(l) {
			directives = append(directives, i)
		}
	}
	if len(directives) != 11 {
		return nil, p.Die("the file holds %d preprocessor directives and this phase was written against "+
			"the eleven `#include`s phase 104 left", len(directives))
	}
	for i := range directives {
		if directives[i] != directives[0]+i {
			return nil, p.Die("the eleven directives are not eleven consecutive lines")
		}
	}
	for _, i := range directives {
		if !edit.Z36Inc.MatchString(L[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	boundary := directives[0]
	p.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", boundary+1)

	// ---- 1. the core's block of ordinary declarations, FOUND rather than assumed
	var seed []int
	for i, l := range L[:boundary] {
		if l == z35Seed {
			seed = append(seed, i)
		}
	}
	if len(seed) != 1 {
		return nil, p.Die("the core does not declare `void *malloc(usize n);` exactly once, so this " +
			"phase has not been handed the file it was written for")
	}
	lo, hi := seed[0], seed[0]
	for lo > 0 && edit.Z36IsDecl(L[lo-1]) {
		lo--
	}
	for hi+1 < boundary && edit.Z36IsDecl(L[hi+1]) {
		hi++
	}
	blockBefore := append([]string{}, L[lo:hi+1]...)
	for _, line := range z35Go {
		if !edit.Contains(blockBefore, line) {
			return nil, p.Die("`%s` is not in the core's block of ordinary declarations, which is %s",
				line, strings.Join(blockBefore, " / "))
		}
	}
	if L[lo-1] != "" || L[hi+1] != "" {
		return nil, p.Die("the block is not a paragraph of its own -- line %d is %s and line %d is %s",
			lo, cutil.PyRepr(L[lo-1]), hi+2, cutil.PyRepr(L[hi+1]))
	}
	names := make([]string, len(blockBefore))
	for i, l := range blockBefore {
		names[i] = shortName(l)
	}
	p.Sayf("the core's block of ordinary declarations is lines %d-%d, %d of them, and every "+
		"one is a libc function the core calls: %s",
		lo+1, hi+1, len(blockBefore), strings.Join(names, " "))

	// ---- 2. the three names, above the boundary and below it -----------------
	core := strings.Join(L[:boundary], "\n")
	host := strings.Join(L[boundary:], "\n")
	ncalls := map[string]int{}
	for _, r := range []struct {
		Name  string
		nhost int
		why   string
	}{
		{"malloc", 0, "lalloc(), and whatever else has come to ask for memory"},
		{"free", 1, "vim_free(), update_wincolor() and the rest; and " +
			"format_overflow_error() below the boundary"},
		{"write", 1, "mch_write(); and host_message() below the boundary"},
	} {
		occ := mentions(core, r.Name)
		paren := len(edit.CallsNotAfterWord([]byte(core), r.Name))
		if occ != paren {
			return nil, p.Die("`%s` has %d mentions above the boundary and only %d of them are followed by "+
				"`(` -- every one must be the declaration or a call, and this phase will not "+
				"rewrite what it cannot classify", r.Name, occ, paren)
		}
		if paren < 2 {
			return nil, p.Die("`%s` is %d call-shaped mentions above the boundary, and this phase needs its "+
				"declaration and at least one call -- %s", r.Name, paren, r.why)
		}
		if k := mentions(host, r.Name); k != r.nhost {
			return nil, p.Die("`%s` has %d mentions below the boundary and this phase was written against "+
				"%d -- %s", r.Name, k, r.nhost, r.why)
		}
		ncalls[r.Name] = paren - 1
	}
	// `write` LOSES ITS FIRST ARGUMENT, so its rewrite is not a rename and the
	// descriptor has to be CHECKED: host_write(s, len) writes to the screen, and
	// the host is where fd 1 is named.
	fd1 := 0
	for _, m := range regexp.MustCompile(`write\(1, `).FindAllStringIndex(core, -1) {
		if m[0] > 0 && edit.IsWordByte(core[m[0]-1]) {
			continue
		}
		fd1++
	}
	if fd1 != ncalls["write"] {
		return nil, p.Die("%d of the core's %d write() calls are to fd 1 -- host_write() takes no "+
			"descriptor, so a write to anything else is a call this phase cannot move",
			fd1, ncalls["write"])
	}
	for _, name := range []string{"host_alloc", "host_free", "host_write"} {
		if mentions(t, name) > 0 {
			return nil, p.Die("`%s` is already a name in this file", name)
		}
	}
	s := "s"
	if ncalls["malloc"] == 1 {
		s = ""
	}
	p.Sayf("the partition holds: above the boundary `malloc` is its declaration and %d call%s, "+
		"`free` its declaration and %d, `write` its declaration and %d -- every one to fd 1 "+
		"-- and NOTHING above the boundary mentions any of the three in any other way.  "+
		"Below it: 0, 1 and 1, which is where they are going",
		ncalls["malloc"], s, ncalls["free"], ncalls["write"])

	// ---- 3. the three declarations leave the core's block --------------------
	var blockAfter []string
	for _, l := range blockBefore {
		if !edit.Contains(z35Go, l) {
			blockAfter = append(blockAfter, l)
		}
	}
	oldBlock := strings.Join(blockBefore, "\n") + "\n"
	dropped := 0
	if len(blockAfter) > 0 {
		if err := swap(oldBlock, strings.Join(blockAfter, "\n")+"\n",
			"the core's block of declarations",
			"the three this phase owns come out of it and the rest stay where they are"); err != nil {
			return nil, err
		}
		dropped = len(z35Go)
	} else {
		if err := swap(oldBlock+"\n", "", "the core's block of declarations AND its trailing "+
			"blank line",
			"the block is empty now, and a paragraph separator with nothing to separate "+
				"is the run of two blank lines this file does not have"); err != nil {
			return nil, err
		}
		dropped = len(blockBefore) + 1
	}

	// ---- 4. and three arrive at the end of the core -> host boundary block ---
	if err := swap("static void host_message(const char *msg, int len, int err);\n",
		"static void host_message(const char *msg, int len, int err);\n"+
			strings.Join(z35Protos, "\n")+"\n",
		"the last of the core -> host prototypes phase 108 left",
		"the boundary is ONE block, and these three belong at the end of it rather than "+
			"wherever a declaration happened to fit"); err != nil {
		return nil, err
	}

	// ---- 5. every call site, COMPUTED ----------------------------------------
	// The core half ONLY: the host below calls free() and write() for itself and
	// must not be touched.
	L = strings.Split(t, "\n")
	bnd := -1
	for i, l := range L {
		if edit.Z36Dir.MatchString(l) {
			bnd = i
			break
		}
	}
	core, host = strings.Join(L[:bnd], "\n"), strings.Join(L[bnd:], "\n")
	done := map[string]int{}
	var n int
	cb := []byte(core)
	cb, n = edit.SubNotAfterWord(cb, "malloc", "host_alloc")
	done["malloc"] = n
	cb, n = edit.SubNotAfterWord(cb, "free", "host_free")
	done["free"] = n
	core = string(cb)
	// The CAST FORM IS TAKEN FIRST so the bare form cannot strip the call Out
	// from under it: host_write() returns int, having narrowed inside the host
	// where the libc type is visible.
	cast := len(z35CastFd1.FindAllString(core, -1))
	core = z35CastFd1.ReplaceAllString(core, "host_write(")
	bare := 0
	var Out strings.Builder
	last := 0
	for _, m := range regexp.MustCompile(`write\(1, `).FindAllStringIndex(core, -1) {
		if m[0] > 0 && edit.IsWordByte(core[m[0]-1]) {
			continue
		}
		Out.WriteString(core[last:m[0]])
		Out.WriteString("host_write(")
		last = m[1]
		bare++
	}
	Out.WriteString(core[last:])
	core = Out.String()
	done["write"] = cast + bare
	for _, name := range []string{"malloc", "free", "write"} {
		if done[name] != ncalls[name] {
			return nil, p.Die("%d `%s` call sites were rewritten and section 2 counted %d",
				done[name], name, ncalls[name])
		}
		if mentions(core, name) > 0 {
			return nil, p.Die("`%s` still appears above the boundary after its call sites were rewritten",
				name)
		}
	}
	t = core + "\n" + host
	s = "s"
	if done["malloc"] == 1 {
		s = ""
	}
	p.Sayf("%d call site%s rewritten to `host_alloc`, %d to `host_free` and %d to `host_write` "+
		"(%d of them shedding an `(int)` cast that the host now does) -- every one COMPUTED "+
		"from the text, and no mention of any of the three left above the boundary",
		done["malloc"], s, done["free"], done["write"], cast)

	// ---- 6. the host defines the three, below the boundary -------------------
	if err := swap("    int\nmain(int argc, char **argv)\n{\n",
		z35Defs+"    int\nmain(int argc, char **argv)\n{\n",
		"the launcher's head, the last function in the file",
		"the three definitions go immediately above it, below host_exit and host_message "+
			"and in the order their prototypes are written"); err != nil {
		return nil, err
	}

	// ---- 7. what the file is now ---------------------------------------------
	L = strings.Split(t, "\n")
	boundary = -1
	for i, l := range L {
		if edit.Z36Dir.MatchString(l) {
			boundary = i
			break
		}
	}
	core, host = strings.Join(L[:boundary], "\n"), strings.Join(L[boundary:], "\n")
	for _, r := range []struct {
		Name         string
		ncore, nhost int
	}{{"malloc", 0, 1}, {"free", 0, 2}, {"write", 0, 2}} {
		if mentions(core, r.Name) != r.ncore || mentions(host, r.Name) != r.nhost {
			return nil, p.Die("`%s` ends at %d mentions above the boundary and %d below, expected %d and "+
				"%d", r.Name, mentions(core, r.Name), mentions(host, r.Name), r.ncore, r.nhost)
		}
	}
	for _, r := range []struct{ Name, was string }{
		{"host_alloc", "malloc"}, {"host_free", "free"}, {"host_write", "write"},
	} {
		want := ncalls[r.was] + 2
		if k := mentions(t, r.Name); k != want {
			return nil, p.Die("`%s` has %d mentions, expected %d -- its prototype, the %d call sites it "+
				"took over from `%s` and its definition", r.Name, k, want, ncalls[r.was], r.was)
		}
	}
	have := append([]string{}, L[lo:lo+len(blockAfter)]...)
	if strings.Join(have, "\x00") != strings.Join(blockAfter, "\x00") {
		return nil, p.Die("the ordinary declarations left above the boundary are %s and the input's "+
			"block minus the three is %s", edit.Z36Or(have), edit.Z36Or(blockAfter))
	}
	if L[lo-1] != "" || L[lo+len(blockAfter)] != "" {
		return nil, p.Die("what is left of the block is not a paragraph of its own")
	}
	// AND NOWHERE ELSE ABOVE THE BOUNDARY, which needs one more test than DECL:
	// macro expansion left ordinary STATEMENTS at column 0 that match a
	// declaration's shape exactly, and what tells them apart is the line above.
	var stray []string
	for i := 0; i < boundary; i++ {
		if edit.Z36IsDecl(L[i]) && (L[i-1] == "" || edit.Z36IsDecl(L[i-1])) &&
			!(lo <= i && i < lo+len(blockAfter)) {
			stray = append(stray, fmt.Sprintf("%d:%s", i+1, L[i]))
		}
	}
	if len(stray) > 0 {
		return nil, p.Die("an ordinary declaration is above the boundary and outside the block: %s",
			strings.Join(edit.First(stray, 4), " / "))
	}
	if err := os.WriteFile(state+"/block-before",
		[]byte(strings.Join(blockBefore, "\n")+"\n"), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	var after strings.Builder
	for _, l := range blockAfter {
		after.WriteString(l + "\n")
	}
	if err := os.WriteFile(state+"/block-after", []byte(after.String()), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	if len(blockAfter) > 0 {
		var left []string
		for _, l := range blockAfter {
			left = append(left, shortName(l))
		}
		p.Sayf("the core's block of ordinary declarations is %d lines and was %d: %s remain, "+
			"and each is a libc function some LATER phase owns",
			len(blockAfter), len(blockBefore), strings.Join(left, " "))
	} else {
		p.Sayf("THE CORE'S BLOCK OF ORDINARY DECLARATIONS IS EMPTY: it was %d lines and it is "+
			"now none.  Above the first `#include` there is no declaration that is not "+
			"`static`, so the core names no libc function at all and every outward call it "+
			"makes is a `musl_` or a `host_`", len(blockBefore))
	}

	// DECLARATION BEFORE USE, COMPUTED.
	for i, name := range []string{"host_alloc", "host_free", "host_write"} {
		proto := z35Protos[i]
		var pr, df, uses []int
		for j, l := range L {
			if l == proto {
				pr = append(pr, j)
			}
			if strings.HasPrefix(l, name+"(") && j+1 < len(L) && L[j+1] == "{" {
				df = append(df, j)
			}
		}
		for j, l := range L {
			if len(edit.CallsNotAfterWord([]byte(l), name)) > 0 && !edit.Contains(pr, j) && !edit.Contains(df, j) {
				uses = append(uses, j)
			}
		}
		if len(pr) != 1 || len(df) != 1 || len(uses) == 0 {
			return nil, p.Die("`%s` has %d prototypes, %d definitions and %d call sites",
				name, len(pr), len(df), len(uses))
		}
		lowU, highU := uses[0], uses[len(uses)-1]
		if !(pr[0] < lowU && highU < df[0] && df[0] > boundary) {
			return nil, p.Die("`%s`: prototype at %d, calls at %d..%d, definition at %d, boundary at %d "+
				"-- the prototype must be above every call, the definition below every one "+
				"and the definition below the boundary",
				name, pr[0]+1, lowU+1, highU+1, df[0]+1, boundary+1)
		}
		var where []string
		for _, u := range uses {
			where = append(where, fmt.Sprintf("%d", u+1))
		}
		s := "s"
		if len(uses) == 1 {
			s = ""
		}
		p.Sayf("`%s`: prototype line %d, %d call site%s at %s, definition line %d, which is "+
			"below the boundary at %d",
			name, pr[0]+1, len(uses), s, strings.Join(where, " "), df[0]+1, boundary+1)
	}

	if len(L)-1 != linesBefore+21-dropped {
		return nil, p.Die("the file is %d lines and the input was %d -- expected %d more: three "+
			"prototypes and three five-line definitions with a blank line after each is 21 "+
			"in, and %d out of the core's block",
			len(L)-1, linesBefore, 21-dropped, dropped)
	}
	if r := blankRuns(t); r != runsBefore {
		return nil, p.Die("the edit left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	var d []int
	for i, l := range L {
		if edit.Z36Dir.MatchString(l) {
			d = append(d, i)
		}
	}
	okd := len(d) == 11 && d[0] == boundary
	for i := range d {
		if d[i] != d[0]+i {
			okd = false
		}
	}
	if !okd {
		return nil, p.Die("the output does not have the same eleven contiguous `#include` directives -- " +
			"this phase adds DECLARATIONS and DEFINITIONS, never a directive")
	}
	p.Sayf("%d -> %d lines, the eleven #includes untouched at line %d, and no run of two "+
		"blank lines", linesBefore, len(L)-1, boundary+1)
	return []byte(t), nil
}
