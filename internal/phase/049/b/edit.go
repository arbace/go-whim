package p049b

// Whim phase 49b (formerly 118) -- the core calls nothing but the host.  GOALS.md II.4c, GOALS.md.
//
// THREE LIBC FUNCTIONS ARE LEFT IN THE CORE AND THIS PHASE MOVES ALL THREE.  Everything
// else the core still asks the operating system for went out through a named call in an
// earlier phase -- the terminal, the signals, the window size and the sleep at phase 39,
// the exit at 102, the messages at 104, the clock at 109 and 111 -- and what survived is the
// three the editor uses so constantly that nobody looked at them: `malloc`, `free` and
// `write`.  The user's words, 2026-09-19: "malloc, free and write should be moved to
// host, then I guess there is no functional dependency in core beyond host."
//
// malloc   its prototype and every call of it -- lalloc(), and since phase 49a the
// malloc-copy-free that replaced ga_grow_inner's and get_keystroke's realloc
// free     its prototype and every call -- vim_free(), update_wincolor(), and phase
// 49a's two again
// write    its prototype and mch_write() -- every byte the editor draws
//
// HOW MANY OF EACH IS READ OFF THE TEXT AND NOT WRITTEN HERE.  This phase asserted the
// counts once, was handed a boundary where phase 49a had changed two of them, and refused;
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
// core's two neighbours on this boundary already name no fd: phase 39's
// `musl_read_input(char *buf, int len)` reads fd 0 inside the host, and phase 40's
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
// phase 43.  This edit does not assume what is in that run: it finds it, requires the
// three lines it owns to be in it, takes exactly those three out, and prints what is
// left.  When the run is EMPTY the core names no libc function at all, and every
// outward call it makes is a `musl_` or a `host_`.  That is the arc's claim, and the
// check states it as a measurement of the output rather than as a sentence written here.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// cut is on the program's graph, its report the text version's, which the
// plan ran until then (history keeps it); the @state argument and the two
// files it wrote there (block-before, block-after) are gone, since nothing
// read them once the checks went (448e9a8):
//
//   - the include checks are the include forms (vimtext.IncludeRun), and the
//     block of ordinary declarations is the run of core forms that are
//     non-`static` prototypes around `void *malloc(usize n);`
//     (vimtext.OrdinaryBlock), each held to the text's line by its C;
//   - "every mention above the boundary is the declaration or a call" is the
//     edges: every use of the core's prototype in the core is a callee, and
//     `\bname\b` on the core's C view is one more than those calls (the
//     text's own count, the declaration); every write is to fd 1, its first
//     argument the constant 1; the host's counts are the text's, on the C view;
//   - the three prototypes go in after host_message's by FRAG, the calls are
//     retargeted to them by edge (RetargetAs: malloc and free respelled in
//     place, their ids kept), and each write is made again by FRAG,
//     `host_write($a, $b)`, the `(int)` cast around it going with it;
//   - the three old prototypes go by DeleteForHeader: the host's own calls of
//     free and write (format_overflow_error, host_message) become uses of the
//     headers' declarations, as an import of the text after makes them, and
//     the three definitions above main(), FRAG's too, take theirs the same way;
//   - the positions (a prototype above every call, the definition below every
//     one and below the boundary) are asked of the forms; the line numbers
//     the report gives are the C view's, the canonical text's, where the
//     text's were its own unprinted lines.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim49b", Edit) }

var w49bSeed = "void *malloc(usize n);"

// w49bLineOf is the 1-based line of the C view's first line equal to l, 0
// when there is none.
func w49bLineOf(lines []string, l string) int {
	for i, x := range lines {
		if x == l {
			return i + 1
		}
	}
	return 0
}

// Edit is part 49b on the graph: malloc, free and write leave the core,
// host_alloc, host_free and host_write in their place.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("hostcall", e, w)
	incs, err := vimtext.IncludeRun(e)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	text := v.Text()
	core, host, bound := vimtext.SplitCore(text)
	L := strings.Split(string(text), "\n")
	linesBefore := len(L) - 1
	v.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", bound+1)

	block, blockBefore, err := vimtext.OrdinaryBlock(e, w49bSeed)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	for _, line := range w49bGo {
		if !edit.Contains(blockBefore, line) {
			v.Die("`%s` is not in the core's block of ordinary declarations, which is %s",
				line, strings.Join(blockBefore, " / "))
			return v.Done()
		}
	}
	names := make([]string, len(block))
	decl := map[string]*graph.Node{}
	for i, f := range block {
		names[i] = graph.DeclName(f)
		decl[names[i]] = f
	}
	v.Sayf("the core's block of ordinary declarations is lines %d-%d, %d of them, and every "+
		"one is a libc function the core calls: %s",
		w49bLineOf(L, blockBefore[0]), w49bLineOf(L, blockBefore[len(blockBefore)-1]),
		len(blockBefore), strings.Join(names, " "))

	ncalls := map[string]int{}
	coreUses := map[string][]*graph.Node{}
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
		d := decl[r.Name]
		for _, u := range e.Uses(d) {
			if !e.InCore(u) {
				continue
			}
			p := e.Parent(u)
			if p == nil || !p.Is("call") || p.Kids[1] != u {
				v.Die("`%s` is used above the boundary other than as a call's callee: %s -- every one must "+
					"be the declaration or a call, and this phase will not rewrite what it cannot classify",
					r.Name, graph.Lisp(p))
				return v.Done()
			}
			coreUses[r.Name] = append(coreUses[r.Name], u)
		}
		calls := len(coreUses[r.Name])
		if occ := edit.MentionCount(core, r.Name); occ != calls+1 {
			v.Die("`%s` has %d mentions above the boundary and only %d of them are its declaration and its "+
				"calls -- every one must be the declaration or a call, and this phase will not "+
				"rewrite what it cannot classify", r.Name, occ, calls+1)
			return v.Done()
		}
		if calls < 1 {
			v.Die("`%s` is %d calls above the boundary, and this phase needs its declaration and at "+
				"least one call -- %s", r.Name, calls, r.why)
			return v.Done()
		}
		if k := edit.MentionCount(host, r.Name); k != r.nhost {
			v.Die("`%s` has %d mentions below the boundary and this phase was written against %d -- %s",
				r.Name, k, r.nhost, r.why)
			return v.Done()
		}
		ncalls[r.Name] = calls
	}
	fd1 := 0
	for _, u := range coreUses["write"] {
		if call := e.Parent(u); len(call.Kids) == 5 && !call.Kids[2].IsList() && call.Kids[2].Atom == "1" {
			fd1++
		}
	}
	if fd1 != ncalls["write"] {
		v.Die("%d of the core's %d write() calls are to fd 1 -- host_write() takes no descriptor, so a "+
			"write to anything else is a call this phase cannot move", fd1, ncalls["write"])
		return v.Done()
	}
	for _, name := range []string{"host_alloc", "host_free", "host_write"} {
		if edit.MentionCount(text, name) > 0 {
			v.Die("`%s` is already a name in this file", name)
			return v.Done()
		}
	}
	s := "s"
	if ncalls["malloc"] == 1 {
		s = ""
	}
	v.Sayf("the partition holds: above the boundary `malloc` is its declaration and %d call%s, "+
		"`free` its declaration and %d, `write` its declaration and %d -- every one to fd 1 "+
		"-- and NOTHING above the boundary mentions any of the three in any other way.  "+
		"Below it: 0, 1 and 1, which is where they are going",
		ncalls["malloc"], s, ncalls["free"], ncalls["write"])

	var blockAfter []string
	for _, l := range blockBefore {
		if !edit.Contains(w49bGo, l) {
			blockAfter = append(blockAfter, l)
		}
	}

	// the three prototypes, at the end of the core -> host run
	var hm []*graph.Node
	for _, f := range e.Core() {
		if graph.DeclName(f) == "host_message" && f.Is("def") {
			if c, err := vimtext.FormC(f); err == nil && c == "static void host_message(const char *msg, int len, int err);" {
				hm = append(hm, f)
			}
		}
	}
	if len(hm) != 1 {
		v.Die("the last of the core -> host prototypes phase 41 left occurs %d times, expected 1 -- the "+
			"boundary is ONE block, and these three belong at the end of it rather than wherever a "+
			"declaration happened to fit", len(hm))
		return v.Done()
	}
	// ... and each write made again as host_write, in the same unit
	fs := []graph.Frag{{At: e.SpotAfter(hm[0]), Src: strings.Join(w49bProtos, "\n")}}
	cast := 0
	for _, u := range coreUses["write"] {
		call := e.Parent(u)
		at := call
		if p := e.Parent(call); p != nil && p.Is("cast") && len(p.Kids) == 3 && p.Kids[1].Atom == "int" && p.Kids[2] == call {
			at = p
			cast++
		}
		fs = append(fs, graph.Frag{At: e.SpotOf(at), Src: "host_write($a, $b)",
			Holes: graph.Bindings{"a": call.Kids[3], "b": call.Kids[4]}})
	}
	made, err := e.SpliceC(fs...)
	if err != nil {
		v.Die("the three prototypes and the write calls: %v", err)
		return v.Done()
	}
	if len(made[0]) != 3 {
		v.Die("the three prototypes made %d forms", len(made[0]))
		return v.Done()
	}
	proto := map[string]*graph.Node{"malloc": made[0][0], "free": made[0][1], "write": made[0][2]}

	// the calls of malloc and free, retargeted
	done := map[string]int{}
	for _, name := range []string{"malloc", "free"} {
		for _, u := range coreUses[name] {
			if err := e.RetargetAs(u, 0, proto[name]); err != nil {
				v.Die("%s's call: %v", name, err)
				return v.Done()
			}
			done[name]++
		}
	}
	done["write"] = len(fs) - 1
	for _, name := range []string{"malloc", "free", "write"} {
		if done[name] != ncalls[name] {
			v.Die("%d `%s` call sites were rewritten and section 2 counted %d", done[name], name, ncalls[name])
			return v.Done()
		}
		if n := len(e.Uses(decl[name])); n != map[string]int{"malloc": 0, "free": 1, "write": 1}[name] {
			v.Die("`%s` is still used %d times once its core calls were rewritten", name, n)
			return v.Done()
		}
	}
	s = "s"
	if done["malloc"] == 1 {
		s = ""
	}
	v.Sayf("%d call site%s rewritten to `host_alloc`, %d to `host_free` and %d to `host_write` "+
		"(%d of them shedding an `(int)` cast that the host now does) -- every one COMPUTED "+
		"from the edges, and no use of any of the three left above the boundary",
		done["malloc"], s, done["free"], done["write"], cast)

	// the old prototypes go; the host's own calls are the headers'
	// and the three definitions above the launcher, in the same unit
	if _, _, err := e.DeleteForHeader([]*graph.Node{decl["malloc"], decl["free"], decl["write"]}, false,
		func() []graph.Frag { return []graph.Frag{{At: e.SpotBefore(e.Defn("main")), Src: w49bDefs}} }); err != nil {
		v.Die("%v", err)
		return v.Done()
	}

	text = v.Text()
	core, host, bound = vimtext.SplitCore(text)
	L = strings.Split(string(text), "\n")
	for _, r := range []struct {
		Name         string
		ncore, nhost int
	}{{"malloc", 0, 1}, {"free", 0, 2}, {"write", 0, 2}} {
		if c, h := edit.MentionCount(core, r.Name), edit.MentionCount(host, r.Name); c != r.ncore || h != r.nhost {
			v.Die("`%s` ends at %d mentions above the boundary and %d below, expected %d and %d",
				r.Name, c, h, r.ncore, r.nhost)
			return v.Done()
		}
	}
	for _, r := range []struct{ Name, was string }{
		{"host_alloc", "malloc"}, {"host_free", "free"}, {"host_write", "write"},
	} {
		want := ncalls[r.was] + 2
		if k := edit.MentionCount(text, r.Name); k != want {
			v.Die("`%s` has %d mentions, expected %d -- its prototype, the %d call sites it took over from "+
				"`%s` and its definition", r.Name, k, want, ncalls[r.was], r.was)
			return v.Done()
		}
	}
	var have []string
	var ordinary []int
	for i, f := range e.Core() {
		if vimtext.IsOrdinaryDecl(f) {
			c, err := vimtext.FormC(f)
			if err != nil {
				v.Die("%v", err)
				return v.Done()
			}
			have = append(have, c)
			ordinary = append(ordinary, i)
		}
	}
	if strings.Join(have, "\x00") != strings.Join(blockAfter, "\x00") {
		v.Die("the ordinary declarations left above the boundary are %s and the input's block minus the "+
			"three is %s", vimtext.JoinOrNone(have), vimtext.JoinOrNone(blockAfter))
		return v.Done()
	}
	for k := 1; k < len(ordinary); k++ {
		if ordinary[k] != ordinary[0]+k {
			v.Die("what is left of the block is not one run of forms")
			return v.Done()
		}
	}
	if len(blockAfter) > 0 {
		var left []string
		for _, l := range blockAfter {
			left = append(left, vimtext.DeclNameRe.ReplaceAllString(l, "$1"))
		}
		v.Sayf("the core's block of ordinary declarations is %d lines and was %d: %s remain, "+
			"and each is a libc function some LATER phase owns",
			len(blockAfter), len(blockBefore), strings.Join(left, " "))
	} else {
		v.Sayf("THE CORE'S BLOCK OF ORDINARY DECLARATIONS IS EMPTY: it was %d lines and it is "+
			"now none.  Above the first `#include` there is no declaration that is not "+
			"`static`, so the core names no libc function at all and every outward call it "+
			"makes is a `musl_` or a `host_`", len(blockBefore))
	}

	forms := e.Graph().Forms
	pos := map[*graph.Node]int{}
	for i, f := range forms {
		pos[f] = i
	}
	first := pos[e.FirstInclude()]
	for i, name := range []string{"host_alloc", "host_free", "host_write"} {
		pr := proto[[]string{"malloc", "free", "write"}[i]]
		df := e.Defn(name)
		uses := e.Uses(pr)
		if df == nil || len(uses) == 0 || len(e.Decls(name)) != 2 {
			v.Die("`%s` has %d declarations, a definition %v and %d call sites", name, len(e.Decls(name)),
				df != nil, len(uses))
			return v.Done()
		}
		for _, u := range uses {
			if at := pos[e.TopForm(u)]; !(pos[pr] < at && at < pos[df] && pos[df] > first) {
				v.Die("`%s`: the prototype must be above every call, the definition below every one and "+
					"the definition below the boundary", name)
				return v.Done()
			}
		}
		protoLine := w49bLineOf(L, w49bProtos[i])
		var defLine int
		var where []string
		for j, l := range L {
			if strings.HasPrefix(l, name+"(") && j+1 < len(L) && L[j+1] == "{" {
				defLine = j + 1
			} else if j+1 != protoLine && len(edit.CallsNotAfterWord([]byte(l), name)) > 0 {
				where = append(where, fmt.Sprint(j+1))
			}
		}
		s := "s"
		if len(where) == 1 {
			s = ""
		}
		v.Sayf("`%s`: prototype line %d, %d call site%s at %s, definition line %d, which is "+
			"below the boundary at %d", name, protoLine, len(where), s, strings.Join(where, " "),
			defLine, bound+1)
	}
	if n := len(e.Includes()); n != len(incs) || e.FirstInclude() != incs[0] {
		v.Die("the output does not have the same %d contiguous include forms -- this phase adds "+
			"DECLARATIONS and DEFINITIONS, never a directive", len(incs))
		return v.Done()
	}
	if _, err := vimtext.IncludeRun(e); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Sayf("%d -> %d lines, and the eleven #includes untouched at line %d", linesBefore, len(L)-1, bound+1)
	return v.Done()
}
