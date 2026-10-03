package p043

// Whim phase 43 (formerly 110) -- THE MOVE.  The first `#include` becomes the boundary.
// See GOALS.md II.4c, GOALS.md, and .claude/briefs/zero-reorg.md 1, 4, 5, 6 and 7.
//
// GOALS.md II.4c's design, which the user settled and which this phase performs:
//
// whim-vim.c  upper part  the core editor.  NO PREPROCESSOR SYNTAX AT ALL.  At its
// top, the musl_-prefixed prototypes: its calls to the host.
// ----------  the first #include IS the boundary, and nothing marks it
// lower part  the host.  The #includes, then the musl_ definitions,
// host_exit, host_message, and main().
//
// Everything stays `static` except `main`.  One translation unit, one gcc invocation,
// no tool changes.  What this phase produces is a POSITION in the file, and the check
// that falls out of that position is the boundary itself, enumerated by the compiler.
//
// WHAT IT DOES, in the order the constraint forces:
//
// 1  THE ELEVEN `#include`s GO DOWN, to just above the host block phases 38b and 39
// put at the bottom of the file.  Nothing else marks the line.
// 2  THE VARIADIC LAYER GOES WITH THEM.  `va_list` is <stdarg.h>'s and the core
// cannot declare it, so every function that holds one is on the host's side:
// vim_snprintf, vim_vsnprintf, vim_vsnprintf_typval and skip_to_arg -- FOUR, not
// three; skip_to_arg is the positional-argument walker and the plan (GOALS.md II.4c) missed it
// until phase 0b counted.  Their two prototypes go with them, for the same reason.
// 3  AND WHATEVER ONLY THEY USE, COMPUTED TO A FIXPOINT rather than listed.  The
// brief's cut reported five functions and two objects after ONE round; this edit
// compiles the cut, moves what gcc calls unused, and compiles again until nothing
// above the boundary is dead.  MEASURED: FIVE rounds, 15 functions and 18 objects
// -- the formatter's whole private island, not its first layer.  The stopping
// rule is `vim_main` and `deathtrap`, the two the HOST calls, which are unused
// above the cut by construction and must stay there.
// 4  THE DERIVED CONSTANTS GO UP, AND THIS IS THE ONLY MOMENT THEY CAN.  `enum : int
// { INT_MAX = (int)(~0u >> 1) };` placed AFTER `#include <limits.h>` is
// `enum : int { 0x7fffffff = ... };`, a syntax error.  So the constants cannot be
// written before the move and need no renaming after it: they are exactly this
// phase's, and that is why phase 42 left them and said so.
//
// THE TWELVE CONSTANTS ARE NOT A LIST THIS PROGRAM REMEMBERS, THEY ARE WHAT THE
// COMPILER ASKS FOR.  The edit performs the move, compiles the cut ALONE, and collects
// every `'X' undeclared` and `unknown type name 'X'` it reports.  That set must be
// exactly the twelve below, or the phase stops: a thirteenth name would mean the core
// still takes something from a header and phase 42 did not finish, and a missing one
// would mean this program is declaring something nobody needs.  MEASURED on the input:
// 23 errors naming exactly these twelve.
//
// derived   INT_MAX 14   INT_MIN 2   LONG_MAX 51  LONG_MIN 1   LLONG_MAX 3
// LLONG_MIN 1  ULLONG_MAX 10  SIZE_MAX 1
// asserted  PATH_MAX 12  EXIT_FAILURE 1  SIGHUP 2  SIGTERM 2
//
// `PATH_MAX` IS AN ARRAY BOUND, which is why all twelve are enumerators and not
// `static const int`: a `static const int` cannot appear in an array bound, a case
// label or an enumerator initialiser, and this one does.  MEASURED, 12 mentions above
// the cut, `char NameBuff[PATH_MAX];` and `vim_strncpy(..., PATH_MAX - 1)` among them.
//
// AN ENUMERATOR IS NOT A TYPEDEF, AND THAT IS WHY THE FAMILIAR NAMES CAN STAY.  A
// later `#define INT_MAX 0x7fffffff` governs only textual occurrences AFTER it, so the
// enumerator above the boundary and the macro below are silent together -- which is a
// different position from `size_t`, where a typedef redefinition has to be
// type-identical, and is the whole reason phase 0a renamed that one and this one
// renames none of these.
//
// AND IT IS ALSO WHY THE CROSS-CHECK HAS TO RESTATE THE DERIVATION.  Below the
// includes the name `INT_MAX` IS the macro, so `static_assert(INT_MAX == INT_MAX)`
// would be a tautology about <limits.h> and would say nothing about the core.  What
// the twelve asserts this phase writes into the file compare is the DERIVING
// EXPRESSION against the header:
//
// static_assert((int)(~0u >> 1) == INT_MAX, "INT_MAX");
//
// and the left-hand side is not typed twice -- it is the enumerator's own initialiser,
// emitted from the same table, and the check reads both out of the output and requires
// them equal text.  That is the same cross-check phase 42 used, in the only shape the
// move leaves available, and unlike phase 42's it lives in the PRODUCT: the core
// declares, the host verifies, and the verification is the ordinary build.
//
// THE FOUR ASSERTED CONSTANTS ARE ASSERTED AND NOT DERIVED, and the file says so by
// writing them as plain numbers with the same assert beside them.  `PATH_MAX`,
// `EXIT_FAILURE`, `SIGHUP` and `SIGTERM` are policy and ABI numbers, not properties of
// the type system: nothing computes 4096 or 15 from anything, so the honest form is a
// number the host checks rather than an expression that pretends.
//
// NOTHING MOVES UP.  In ONE translation unit everything above the cut is visible below
// it, so only the core -> host direction ever needs a declaration.  MEASURED at phase
// 41: the four variadic functions call twenty distinct core functions at forty-one
// sites and read IObuff once, and not one of them costs a declaration.
//
// WHAT THE MOVE DESTROYS, said plainly, because phase 42's check was built on it.
// After this phase a wrong `void *malloc(int n);` above the boundary is no longer
// `error: conflicting types for 'malloc'` -- there is no second declaration to
// conflict with -- and a `static` one is no longer an error at the declaration but a
// LINK failure, `'malloc' used but never defined`.  That is exactly why phase 42 came
// first and wrote sixteen static_asserts against headers that were still above it.
// The check here breaks the `static` trap in its new shape.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// phase is a graph step now, and gcc is asked nothing: each of its three
// questions has an answer in the edges (the text program, which compiled
// the cut eight times, is in history, `16717ab` and before):
//
//   - THE TWELVE are what the move would leave unprovided: the names the
//     headers give the core now and would not once the includes and the
//     variadic layer are below it (crefactor/graph's MoveWouldLose, the
//     headers' rule of B2e) -- the macros the core's own tokens are, and
//     NOT ONE DECLARATION, which would mean the core still takes a function
//     or a type from a header.  It must be exactly the twelve, as gcc's
//     `undeclared` set had to be.
//   - DEFINED BUT NOT USED is a static function or object of the core that
//     no use in the core refers to, a function's own recursive calls
//     excepted -- gcc's rule exactly (c-typeck.cc: "Recursive call does not
//     count as usage"; a sizeof, an initialiser's address, a dead
//     function's call all count).  The enum blocks are asked the same of
//     their enumerators (and their tag), by edge where the text counted
//     words.
//   - USED BUT NEVER DEFINED, the boundary, is a static function the core
//     declares and uses and does not define.
//
// The move is ONE act, MoveFormsOwning: the includes and everything that
// goes with them below the core in the text's order, the twelve
// enumerators written by FRAG beneath `usize`, and every token that was a
// header's macro made a use of its enumerator, as an import of the text
// after makes it; the rule (nothing left unprovided, no new collision) is
// held against the graph before the move.  The twelve static_asserts are a
// FRAG below the last include, where their names are the headers' again.
// What compiling the cut proved -- 0 errors, nothing dead above it -- is
// the rule's answer and the fixpoint's last round.  The state directory
// the text wrote `moved` and `boundary` into is gone with it: nothing read
// them.

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim43", Edit) }

// ---- the twelve constants -----------------------------------------------
// Eight derived and four asserted.  The initialiser text is used TWICE -- as
// the enumerator above the boundary and as the left-hand side of the
// static_assert below it -- and it is written here ONCE, so the derivation and
// the thing that checks it cannot drift apart.
type w43Const struct{ ty, Name, val string }

var w43Consts = []w43Const{
	{"int", "INT_MAX", "(int)(~0u >> 1)"},
	{"int", "INT_MIN", "-(int)(~0u >> 1) - 1"},
	{"long", "LONG_MAX", "(long)(~0ul >> 1)"},
	{"long", "LONG_MIN", "-(long)(~0ul >> 1) - 1"},
	{"long long", "LLONG_MAX", "(long long)(~0ull >> 1)"},
	{"long long", "LLONG_MIN", "-(long long)(~0ull >> 1) - 1"},
	{"unsigned long long", "ULLONG_MAX", "~0ull"},
	{"usize", "SIZE_MAX", "(usize)-1"},
	{"", "PATH_MAX", "4096"},
	{"", "EXIT_FAILURE", "1"},
	{"", "SIGHUP", "1"},
	{"", "SIGTERM", "15"},
}

const (
	w43Host    = "static volatile sig_atomic_t host_winch_pending"
	w43Typedef = "typedef typeof(sizeof(0)) usize;"
)

// The four that hold a `va_list`.  They are named because `va_list` is
// <stdarg.h>'s and the core cannot declare it; EVERYTHING ELSE that moves is
// computed from them.
var w43Variadic = []string{"vim_snprintf", "vim_vsnprintf", "skip_to_arg", "vim_vsnprintf_typval"}

// The two `va_list` prototypes, adjacent, which go with them.
var w43Protos = []string{"vim_vsnprintf", "vim_vsnprintf_typval"}

// The other direction of the boundary: the host calls these, so they are
// unused ABOVE the cut by construction and stay there.  They are the stopping
// rule of the fixpoint.
var w43Keep = map[string]bool{"vim_main": true, "deathtrap": true}

// Edit is the move: the `#include`s go below the core, twelve
// header-supplied constants become enumerators asserted from below, and the
// formatter's private island follows the four `va_list` functions down.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	v := graph.NewVerbs("boundary", e, w)
	if len(args) != 0 {
		v.Die("usage: edit whim43 (no arguments)")
		return v.Done()
	}
	forms := e.Graph().Forms
	nForms := len(forms)
	at := map[*graph.Node]int{}
	for i, f := range forms {
		at[f] = i
	}

	// ---- 0. the file this edit was written against ----------------------
	// Every include form at the top, contiguous, and nothing above the
	// first: what phase 40 left and phases 0a and 42 each asserted in turn.
	// This is the LAST phase for which that is true, and making it false is
	// the point.
	incs := e.Includes()
	nInc := len(incs)
	for k, inc := range incs {
		if at[inc] != k {
			v.Die("the input's %d include forms are not its first %d forms: #%d (%s) is form %d",
				nInc, nInc, inc.ID, graph.IncludeSpec(inc), at[inc])
			return v.Done()
		}
	}
	var specs []string
	for _, inc := range incs {
		specs = append(specs, graph.IncludeSpec(inc))
	}
	var hosts []*graph.Node
	for _, d := range e.Decls("host_winch_pending") {
		if e.TopForm(d) == d && strings.HasPrefix(w43C(d), w43Host) {
			hosts = append(hosts, d)
		}
	}
	if len(hosts) != 1 {
		v.Die("the host block does not begin exactly once with '%s' -- found %d.  It is "+
			"where the includes are going and there is nowhere else to put them", w43Host, len(hosts))
		return v.Done()
	}
	hb := hosts[0]
	var usize []*graph.Node
	for _, d := range e.Decls("usize") {
		if d.Is("typedef") && w43C(d) == w43Typedef {
			usize = append(usize, d)
		}
	}
	if len(usize) != 1 {
		v.Die("`%s` is not in the input exactly once -- phase 0a put it below the last "+
			"`#include` and the constants go beneath it", w43Typedef)
		return v.Done()
	}
	v.Sayf("%d include forms, every one an `#include <...>`, the file's first %d forms, "+
		"and the host block beginning exactly once: %s", nInc, nInc, strings.Join(specs, " "))

	// ---- 1. the four kinds of thing that can move ------------------------
	defn := func(n string) *graph.Node {
		var out []*graph.Node
		for _, d := range e.Decls(n) {
			if d.Is("defn") {
				out = append(out, d)
			}
		}
		if len(out) != 1 {
			v.Die("`%s` is not defined exactly once at file scope -- found %d", n, len(out))
			return nil
		}
		return out[0]
	}
	var variadic []*graph.Node
	for _, n := range w43Variadic {
		d := defn(n)
		if d == nil {
			return v.Done()
		}
		variadic = append(variadic, d)
	}
	var protos []*graph.Node
	for _, n := range w43Protos {
		var ps []*graph.Node
		for _, d := range e.Decls(n) {
			if d.Is("def") && w43IsFunc(d) {
				ps = append(ps, d)
			}
		}
		if len(ps) != 1 {
			v.Die("`%s` has %d prototypes, not the one this edit lifts", n, len(ps))
			return v.Done()
		}
		protos = append(protos, ps[0])
	}
	if at[protos[1]] != at[protos[0]]+1 {
		v.Die("the two `va_list` prototypes are not the adjacent pair of forms this edit lifts")
		return v.Done()
	}
	region := forms[nInc:at[hb]] // the core-to-be, before anything moves

	// ---- 2. the move, and the twelve names the headers stop providing -----
	// THE LIST IS NOT REMEMBERED, IT IS ASKED FOR: what moving the includes
	// and the variadic layer below the core would leave unprovided.  That
	// set must be exactly the twelve this phase declares: a declaration in
	// it, or a thirteenth name, would mean the core still takes something
	// from a header and phase 42 did not finish, and a missing one would
	// mean this program declares something nobody needs.
	first := append(append(append([]*graph.Node{}, incs...), protos...), variadic...)
	lost, err := e.MoveWouldLose(hb, false, first)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	askedSet := map[string]bool{}
	for _, u := range lost {
		if !u.Macro {
			v.Die("the move would leave the core without a header's declaration: %s", u)
			return v.Done()
		}
		askedSet[u.Name] = true
	}
	asked := edit.SortedKeys(askedSet)
	var want []string
	for _, c := range w43Consts {
		want = append(want, c.Name)
	}
	sort.Strings(want)
	if !slices.Equal(asked, want) {
		shown := strings.Join(asked, " ")
		if shown == "" {
			shown = "(none)"
		}
		v.Die("the move leaves %d names unprovided and this phase declares %d.  Asked for: %s.  "+
			"Declared: %s.  A name in the first list and not the second is something "+
			"the core still takes from a header; one in the second and not the first "+
			"is a declaration nobody needs", len(asked), len(want), shown, strings.Join(want, " "))
		return v.Done()
	}
	core0, err := graph.FormsC(region)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	counts := map[string]int{}
	tokens := 0
	for _, c := range w43Consts {
		counts[c.Name] = edit.MentionCount(core0, c.Name)
	}
	moving0 := map[*graph.Node]bool{}
	for _, f := range first {
		moving0[f] = true
	}
	for _, f := range region {
		if moving0[f] {
			continue
		}
		graph.Walk(f, func(n *graph.Node) bool {
			if !n.IsList() && askedSet[n.Atom] && n.Ref() == nil {
				tokens++
			}
			return true
		})
	}
	var missing []string
	for _, c := range w43Consts {
		if counts[c.Name] < 1 {
			missing = append(missing, c.Name)
		}
	}
	if len(missing) > 0 {
		v.Die("a constant this phase declares does not occur above the host block at all: %s",
			strings.Join(missing, " "))
		return v.Done()
	}
	v.Sayf("the move alone leaves %d tokens of the core provided by no include above them, "+
		"naming EXACTLY the twelve constants this phase declares and nothing else", tokens)
	var shownCounts []string
	for _, c := range w43Consts {
		shownCounts = append(shownCounts, fmt.Sprintf("%s %d", c.Name, counts[c.Name]))
	}
	v.Sayf("their counts above the host block, re-measured here: %s", strings.Join(shownCounts, "  "))

	// ---- 3. the fixpoint: whatever only the moved code uses --------------
	moving := map[*graph.Node]bool{}
	for _, f := range first {
		moving[f] = true
	}
	type round struct {
		nf, nv []string
		ne     []*graph.Node
	}
	var rounds []round
	settled := false
	for r := 0; r < 16; r++ {
		nf, nv, ne := w43Unused(e, region, moving)
		rounds = append(rounds, round{nf.names, nv.names, ne})
		if len(nf.forms) == 0 && len(nv.forms) == 0 && len(ne) == 0 {
			settled = true
			break
		}
		for _, f := range append(append(nf.forms, nv.forms...), ne...) {
			moving[f] = true
		}
	}
	if !settled {
		v.Die("the fixpoint did not settle in 16 rounds")
		return v.Done()
	}
	// the moved forms in the text's order below the includes: the enum
	// blocks, the objects, the two prototypes, the functions, each kind in
	// the file's order
	var enums, objs, funcs []*graph.Node
	for _, f := range region {
		switch {
		case !moving[f] || slices.Contains(protos, f):
		case f.Is("enum"):
			enums = append(enums, f)
		case f.Is("defn"):
			funcs = append(funcs, f)
		default:
			objs = append(objs, f)
		}
	}
	all := append(append([]*graph.Node{}, incs...), enums...)
	all = append(append(append(all, objs...), protos...), funcs...)

	// ---- 4. the move, owning the twelve ------------------------------------
	var up strings.Builder
	for _, c := range w43Consts {
		if c.ty == "" {
			fmt.Fprintf(&up, "enum { %s = %s };\n", c.Name, c.val)
		} else {
			fmt.Fprintf(&up, "enum : %s { %s = %s };\n", c.ty, c.Name, c.val)
		}
	}
	rebound, err := e.MoveFormsOwning(hb, false, all, func(lost []graph.HeaderUse) error {
		for _, u := range lost {
			if !askedSet[u.Name] {
				return fmt.Errorf("the whole move leaves %s unprovided, which the variadic layer's did not", u)
			}
		}
		_, err := e.SpliceC(graph.Frag{At: e.SpotAfter(usize[0]), Src: up.String()})
		return err
	})
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	incs = e.Includes()
	var asserts strings.Builder
	for _, c := range w43Consts {
		fmt.Fprintf(&asserts, "static_assert(%s == %s, \"%s\");\n", c.val, c.Name, c.Name)
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotAfter(incs[len(incs)-1]), Src: asserts.String()}); err != nil {
		v.Die("the static_asserts: %v", err)
		return v.Done()
	}

	// ---- 5. what the file is now -----------------------------------------
	forms = e.Graph().Forms
	at = map[*graph.Node]int{}
	for i, f := range forms {
		at[f] = i
	}
	incs = e.Includes()
	for k, inc := range incs {
		if at[inc] != at[incs[0]]+k {
			v.Die("the output's %d include forms are not contiguous: #%d (%s) is form %d",
				len(incs), inc.ID, graph.IncludeSpec(inc), at[inc])
			return v.Done()
		}
	}
	if len(incs) != nInc {
		v.Die("the output has %d include forms where the input had %d", len(incs), nInc)
		return v.Done()
	}
	core := e.Core()
	uses, err := e.HeaderUses()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	for _, u := range uses {
		if e.InCore(u.First) {
			v.Die("the core still takes %s from the headers", u)
			return v.Done()
		}
	}
	if cs, err := e.Collisions(); err != nil || len(cs) > 0 {
		v.Die("an include's macro is over a name of the file's own below it: %v %v", cs, err)
		return v.Done()
	}
	nf, nv, ne := w43Unused(e, core, nil)
	if left := append(append(nf.names, nv.names...), w43EnumNames(ne)...); len(left) > 0 {
		v.Die("the fixpoint left %s unused above the boundary", strings.Join(left, " "))
		return v.Done()
	}
	boundary := w43Boundary(e, core)

	v.Sayf("the fixpoint settled in %d rounds: %d functions, %d objects and %d enum "+
		"blocks go below the boundary, and every one after the four that hold a "+
		"`va_list` was named by the edges, never by this program",
		len(rounds), len(funcs), len(objs), len(enums))
	for i, r := range rounds {
		if len(r.nf) > 0 || len(r.nv) > 0 || len(r.ne) > 0 {
			var parts []string
			parts = append(parts, r.nf...)
			parts = append(parts, r.nv...)
			for _, n := range w43EnumNames(r.ne) {
				parts = append(parts, "enum{"+n+"}")
			}
			v.Sayf("  round %d: %s", i, strings.Join(parts, " "))
		}
	}
	v.Sayf("%d forms -> %d, the %d include forms now forms %d-%d, %d tokens made uses of the "+
		"twelve, and NOTHING above the first include is a directive",
		nForms, len(forms), len(incs), at[incs[0]]+1, at[incs[len(incs)-1]]+1, len(rebound))
	v.Sayf("THE CORE IS %d FORMS, TAKES NOTHING FROM THE HEADERS AND NOTHING IN IT IS DEAD, "+
		"and its boundary is %d names, every one used and never defined there -- %s",
		len(core), len(boundary), strings.Join(boundary, " "))
	return v.Done()
}

// w43C is a form's C, trimmed.
func w43C(f *graph.Node) string {
	b, err := graph.FormsC([]*graph.Node{f})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// w43IsFunc says a def declares a function: its type is a fn form.
func w43IsFunc(d *graph.Node) bool {
	t := graph.DeclType(d)
	return t != nil && t.Is("fn")
}

// w43Static says a top-level def or defn is `static`.
func w43Static(d *graph.Node) bool {
	name := graph.DeclName(d)
	for _, k := range d.Kids[1:] {
		if k.IsList() {
			continue
		}
		if k.Atom == "static" {
			return true
		}
		if k.Atom == name {
			break
		}
	}
	return false
}

type w43Found struct {
	names []string
	forms []*graph.Node
}

// w43Unused is what gcc says `defined but not used` of the forms of
// region, those moving left out: the static functions and objects no use
// in what stays refers to -- a function's own recursive calls not counting,
// c-typeck.cc's rule -- and the enum blocks none of whose enumerators, and
// not its tag, is used there outside the block.  Names sorted; the blocks
// in the file's order.
func w43Unused(e *graph.Editor, region []*graph.Node, moving map[*graph.Node]bool) (fns, objs w43Found, enums []*graph.Node) {
	decls := w43Decls(e)
	stays := map[*graph.Node]bool{}
	for _, f := range region {
		if !moving[f] && !graph.IsInclude(f) {
			stays[f] = true
		}
	}
	usedFrom := func(decls []*graph.Node, self *graph.Node) bool {
		for _, d := range decls {
			for _, u := range e.Uses(d) {
				t := e.TopForm(u)
				if stays[t] && t != self {
					return true
				}
			}
		}
		return false
	}
	add := func(f *w43Found, n *graph.Node) {
		f.names = append(f.names, graph.DeclName(n))
		f.forms = append(f.forms, n)
	}
	for _, f := range region {
		if !stays[f] {
			continue
		}
		name := graph.DeclName(f)
		switch {
		case f.Is("defn"):
			if w43Static(f) && !w43Keep[name] && !usedFrom(decls[name], f) {
				add(&fns, f)
			}
		case f.Is("def"):
			if w43Static(f) && !w43IsFunc(f) && !w43Keep[name] && !usedFrom(decls[name], nil) {
				add(&objs, f)
			}
		case f.Is("enum"):
			ens := graph.Enumerators(f)
			if len(ens) == 0 {
				continue
			}
			used := false
			for _, en := range append([]*graph.Node{f}, ens...) {
				for _, u := range e.Uses(en) {
					if t := e.TopForm(u); stays[t] && t != f {
						used = true
					}
				}
			}
			if !used {
				enums = append(enums, f)
			}
		}
	}
	sortFound := func(f *w43Found) {
		sort.Sort(w43ByName(*f))
	}
	sortFound(&fns)
	sortFound(&objs)
	return fns, objs, enums
}

type w43ByName w43Found

func (b w43ByName) Len() int           { return len(b.names) }
func (b w43ByName) Less(i, j int) bool { return b.names[i] < b.names[j] }
func (b w43ByName) Swap(i, j int) {
	b.names[i], b.names[j] = b.names[j], b.names[i]
	b.forms[i], b.forms[j] = b.forms[j], b.forms[i]
}

// w43Decls are the file's top-level declarations by the ordinary name
// they declare: a function's prototypes and definition, an object's.
func w43Decls(e *graph.Editor) map[string][]*graph.Node {
	out := map[string][]*graph.Node{}
	for _, f := range e.Graph().Forms {
		if n := graph.DeclName(f); n != "" {
			out[n] = append(out[n], f)
		}
	}
	return out
}

// w43EnumNames are the enum blocks' first enumerators' names.
func w43EnumNames(ens []*graph.Node) []string {
	var out []string
	for _, f := range ens {
		out = append(out, graph.EnumeratorName(graph.Enumerators(f)[0]))
	}
	return out
}

// w43Boundary is what gcc says `used but never defined` of the core: the
// static functions it declares and uses and does not define, sorted.
func w43Boundary(e *graph.Editor, core []*graph.Node) []string {
	in := map[*graph.Node]bool{}
	for _, f := range core {
		in[f] = true
	}
	decls := w43Decls(e)
	set := map[string]bool{}
	for _, f := range core {
		if !f.Is("def") || !w43IsFunc(f) || !w43Static(f) {
			continue
		}
		name := graph.DeclName(f)
		defined, used := false, false
		for _, d := range decls[name] {
			if d.Is("defn") && in[d] {
				defined = true
			}
			for _, u := range e.Uses(d) {
				if in[e.TopForm(u)] {
					used = true
				}
			}
		}
		if used && !defined {
			set[name] = true
		}
	}
	return edit.SortedKeys(set)
}
