package p036

// Whim phase 36 (formerly 97) -- the strings are the editor's own.  See GOAL.md.
//
// Seventeen libc symbols are string and memory work, and every one of them is pure
// computation: no descriptor, no clock, no signal, nothing the host owns.  This phase
// brings sixteen of them into `whim-vim.c` as `static musl_*` functions written from
// /root/musl/src/string/, and moves the seventeenth -- `sprintf` -- onto the printf
// this editor already carries.  `nm -u` goes 61 -> 44 and the recording does not move.
//
// THE MEASUREMENT THAT MADE THIS PHASE POSSIBLE, and it was the open question:
// gcc emits calls to `memcpy` and `memset` FOR ITSELF, for aggregate assignments and
// large zero initialisers, whatever the source calls -- so renaming every call site
// might have left both symbols undefined and forced a definition under the REAL name,
// which is external linkage and would break "nothing is global but main()".  Measured
// on this file and it does not happen: after the rename, `gcc -S` contains NOT ONE
// call to any of the seventeen, and `nm -u` loses all seventeen.  Two things bound it
// rather than luck -- gcc's -O0 inline-copy threshold is between 8 KiB and 16 KiB (a
// 8192-byte struct assignment is inlined, a 16384-byte one calls memcpy), and
// `-Wlarger-than=8192` on whim-vim.c reports exactly ONE object above 8 KiB,
// `options[]` at 13,536 bytes, which is a table nothing assigns whole, while
// `-Wframe-larger-than=8192` reports none.  The check asserts the absence from `nm -u`,
// so a later phase that adds a big aggregate and assigns it whole fails loudly.
//
// (Measured and NOT taken: `-fno-builtin` and `-ffreestanding` each ADD `abs fprintf
// labs` and remove `fputc fputs fwrite putchar`.  A different set, not a smaller
// problem, and none of it is this phase's.  ZEROCFLAGS is untouched, so this phase
// edits no makefile and whim.mk needs no change.)
//
// `sprintf` IS TWO POPULATIONS AND THE SPLIT IS THE WHOLE STORY.  Of its 22
// occurrences, THIRTEEN are ordinary call sites that become `vim_snprintf(dest, size,
// ...)`, and NINE are inside `vim_vsnprintf_typval` ITSELF -- which is what
// vim_snprintf calls.  Using the in-house printf for those nine would be circular.
// What they actually do is narrow: `f` is built twenty lines above the call and is
// `%`, an optional `h`/`l`/`ll`, and one of `p d o u x X`, with NO flags, NO width and
// NO precision -- vim does all of those itself, in `tmp[]`, before and after.  So the
// nine are "write this integer in this base", and they become `musl_fmtnum()` and
// `musl_fmtptr()`, which have no format string and are not a printf.  The `char f[6]`
// block goes with them: leaving it would draw -Wunused-but-set-variable, which is in
// -Wall and which the sweep acts on.
//
// EVERY SIZE ARGUMENT IS KNOWABLE AND NONE IS INVENTED.  Eight are `sizeof()` of a
// visible array or the constant the buffer was allocated with -- `IObuff` is
// `alloc((1024+1))` and `NameBuff` is `alloc(PATH_MAX)` -- three repeat the `alloc()`
// expression from three lines above, and ONE, `highlight_arg_to_string`'s, is a
// pointer PARAMETER where `sizeof(buf)` would be 8 and wrong.  Its bound is
// `MAX_ATTR_LEN`, and that is sound ONLY because the function has exactly one caller,
// `highlight_list_arg`, whose local is `char_u buf[MAX_ATTR_LEN]`.  Both the edit and
// the check assert that caller count: a second caller appearing later would silently
// invalidate the bound, and the assertion is what catches it.
//
// `musl_strcasecmp` AND `musl_strncasecmp` DO NOT CALL `tolower`.  musl's do, and
// musl's `tolower` in the C locale is `(unsigned)c - 'A' < 26 ? c | 32 : c` and
// nothing else, so the arithmetic is inlined here.  That is not a shortcut: `tolower`
// belongs to the character-class phase, which counts its mentions, and four new ones
// here would trip it.  The check asserts `tolower` at exactly 2 mentions after this
// phase, which is what it had before.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, and the source goes with it as $state/old.c.  The check needs the binary for
// its one MUST-DIFFER probe: `t_CF` is a user-settable option used as a FORMAT STRING
// into `char buf[20]`, `sprintf` has no bound, and the binary this phase is handed
// SEGFAULTS on it.  vim_snprintf truncates instead.  That is a bug fix and it is the
// only reachable input on which this phase changes what the editor does.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3c).  The acts are the text
// program's, made on the graph: the thirteen sites and the nine arms are its
// literal substitutions, each made at the smallest node it changes
// (crefactor/graph's SubstituteC, through FRAG); the eighteen definitions
// are a top-level fragment after the usize typedef; and the rename is no
// longer a regexp over the text but every use of the sixteen externals
// retargeted to its musl_ definition (RetargetUses), so a string literal
// cannot be reached by construction -- the literals are still read, and
// the count of uses retargeted is the text's count of identifiers renamed.
// The `\bname\b` counts are the text's, on the C view.

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim36", Edit) }

// w36Before is the seventeen as OCCURRENCES.  Nothing here is approximate: a
// rename is only safe if the count of what is about to be renamed is known
// first -- and `grep -c` counts LINES, which gives 125 for strlen where there
// are 127, so a check written against it refuses a correct phase.
var w36Before = map[string]int{
	"memmove": 159, "strlen": 127, "memset": 79, "strncmp": 82, "strcmp": 62,
	"strcpy": 51, "sprintf": 22, "memcpy": 7, "strncasecmp": 13, "strcat": 6,
	"strcasecmp": 6, "strncpy": 4, "strstr": 3, "strchr": 2, "memcmp": 2,
	"memchr": 1, "strpbrk": 2,
	"vim_snprintf": 55, "vim_vsnprintf_typval": 4,
	"tolower": 2, "highlight_arg_to_string": 2, "MAX_ATTR_LEN": 2,
}

var w36After = map[string]int{
	"sprintf": 0, "f_l": 0, "tolower": 2, "vim_snprintf": 68,
	"musl_memmove": 160, "musl_strlen": 133, "musl_memset": 80,
	"musl_strncmp": 83, "musl_strcmp": 63, "musl_strcpy": 53, "musl_memcpy": 8,
	"musl_strncasecmp": 14, "musl_strcat": 7, "musl_strcasecmp": 7,
	"musl_strncpy": 5, "musl_strstr": 4, "musl_strchr": 3, "musl_memcmp": 3,
	"musl_memchr": 2, "musl_strpbrk": 3,
	"musl_fmtnum": 9, "musl_fmtptr": 2, "musl_fmtbase": 5,
}

// w36Names are the sixteen renamed in section C.
var w36Names = []string{"memmove", "strlen", "memset", "strncmp", "strcmp", "strcpy",
	"memcpy", "strncasecmp", "strcat", "strcasecmp", "strncpy", "strstr", "strchr",
	"memcmp", "memchr", "strpbrk"}

const w36Pad = "                                "

// Edit vendors the sixteen mem*/str* of <string.h> as local `static musl_*`
// functions, with sprintf moved onto the editor's own vim_snprintf instead.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("strings", e, w)
	t := string(v.Text())
	nInc := edit.IncludeCount([]byte(t)) // the headers it was handed (phase 88 drops the unused)
	mentions := func(s, name string) int { return edit.MentionCount([]byte(s), name) }

	// ---- 0. the input's counts, READ rather than remembered ---------------------
	// w36Before was the input these anchors were first written against.  An
	// upstream that drops three memset calls this phase never touches made
	// every one of those constants refuse a correct phase, so the counts are
	// measured here and the phase's OWN additions -- w36After less w36Before,
	// per name -- are what stays fixed.  What guards the edit is the exact-once
	// anchor at every site it rewrites, below, and the partition at the end.
	before := map[string]int{}
	for name := range w36Before {
		before[name] = mentions(t, name)
	}
	sixteen := 0
	for _, name := range w36Names {
		sixteen += before[name]
	}
	if before["sprintf"] != 22 {
		v.Die("sprintf has %d mentions, and this phase rewrites exactly 22: thirteen external "+
			"call sites and nine inside vim_vsnprintf_typval", before["sprintf"])
		return v.Done()
	}
	v.Say(fmt.Sprintf("the sixteen at %d occurrences and sprintf at 22, tolower at %d, vim_snprintf at %d -- "+
		"read from this input", sixteen, before["tolower"], before["vim_snprintf"]))

	// ---- THE SINGLE CALLER, COUNTED BEFORE THE BOUND IS CHOSEN ----------------
	// highlight_arg_to_string takes a POINTER, so sizeof(buf) is 8 and the bound
	// has to come from outside the call.
	if k := mentions(t, "highlight_arg_to_string"); k != 2 {
		v.Die("highlight_arg_to_string has %d mentions and not the definition plus ONE "+
			"call: MAX_ATTR_LEN is only its buffer size while highlight_list_arg is its "+
			"only caller", k)
		return v.Done()
	}
	if strings.Count(t, w36lit5) != 1 {
		v.Die("highlight_list_arg's `char_u buf[MAX_ATTR_LEN];` is not there exactly once, " +
			"and it is where the bound for site 25443 comes from")
		return v.Done()
	}
	if strings.Count(t, w36lit6) != 1 {
		v.Die("MAX_ATTR_LEN is not the one enumerator this phase reads")
		return v.Done()
	}
	v.Say("highlight_arg_to_string has ONE caller, highlight_list_arg, whose local is " +
		"char_u buf[MAX_ATTR_LEN] with MAX_ATTR_LEN = 120 -- that, and nothing weaker, " +
		"is why the size argument at site 25443 may be a constant from another function")

	// textEdit did not report: sections A and B each make many edits and
	// then say one line about all of them, so their acts are made quietly.
	quiet := graph.NewVerbs("strings", e, io.Discard)

	// ---- D. the definitions, after the includes and the seed's usize ----------
	// Below the typedef the seed writes under the includes (whim0a, run in
	// phase 0): the definitions name usize, so they follow it.  They go
	// first: the arms below call two of them, and the rename points every
	// use at them; defined ahead of
	// every use, so no prototype is added and the prototype block is not
	// touched.
	quiet.TopAfterC("usize", w36Defs, "the eighteen definitions go after the #includes and the usize typedef")
	if quiet.Err != nil {
		return quiet.Err
	}

	// ---- A. the thirteen external sprintf sites -------------------------------
	// ---- B. the nine inside vim_vsnprintf_typval ------------------------------
	// One synthesized import for all twenty-three substitutions.  THE BLOCK
	// GOES TOO: removing the calls and leaving `f` would draw
	// -Wunused-but-set-variable, which is in -Wall.
	var subs []graph.Subst
	for _, s := range w36Sites {
		subs = append(subs, graph.Subst{Old: s.Old, New: s.New, N: 1, What: s.What})
	}
	quiet.SubstituteC(subs...)
	if quiet.Err != nil {
		return quiet.Err
	}
	v.Say("thirteen external sprintf call sites are vim_snprintf now, each with the size " +
		"its destination really has -- eight a sizeof() or the constant the buffer was " +
		"allocated with, three the alloc() expression repeated, one MAX_ATTR_LEN from " +
		"the single caller above, and term_font the one that could overflow")
	subs = []graph.Subst{
		{In: "vim_vsnprintf_typval", Old: w36lit2, New: "", N: 1,
			What: "vim_vsnprintf_typval's `char f[6]`, the two-to-five character format " +
				"string it built for the nine calls below"},
		{In: "vim_vsnprintf_typval", Old: w36lit3, New: w36lit4, N: 1,
			What: "%p, which musl's own printf renders as `0x` and sixteen zero-padded " +
				"hex digits -- musl vfprintf does `p = MAX(p, 2*sizeof(void*)); t = " +
				"'x'; fl |= ALT_FORM`, so musl_fmtptr reproduces that and not glibc's " +
				"`(nil)`"},
	}
	// The eight integer arms.  They differ in their argument and in their trailing
	// indentation, which is why these are eight one-count patterns and not one
	// pattern with a count of eight.
	for _, a := range w36Arms {
		subs = append(subs, graph.Subst{In: "vim_vsnprintf_typval",
			Old: w36Pad + "str_arg_l += sprintf(tmp + str_arg_l, f, " + a.arg,
			New: w36Pad + "str_arg_l += musl_fmtnum(tmp + str_arg_l, " + a.call,
			N:   1, What: "the " + a.arg[:len(a.arg)-2] + " arm"})
	}
	quiet.SubstituteSeq(subs...) // the arms are inside the chain the %p arm heads
	if quiet.Err != nil {
		return quiet.Err
	}
	t = string(v.Text())
	if mentions(t, "sprintf") > 0 || mentions(t, "f_l") > 0 {
		v.Die("sprintf has %d mentions and f_l %d after the nine went, and both must be 0",
			mentions(t, "sprintf"), mentions(t, "f_l"))
		return v.Done()
	}
	v.Say("the nine calls inside vim_vsnprintf_typval are musl_fmtnum() and musl_fmtptr() " +
		"now, and the char f[6] that fed them is gone -- sprintf at 0 mentions and f_l " +
		"at 0")

	// ---- C. the rename, every use of the sixteen retargeted ---------------------
	// MEASURED: not one literal in this file mentions any of the sixteen, so the
	// literal awareness is belt and braces -- but a rename that did not have it
	// would be a guess.  On the graph a literal is not a use and cannot be
	// renamed; it is still read, so that a literal naming one is said.
	pat := regexp.MustCompile(`\b(` + strings.Join(w36Names, "|") + `)\b`)
	for _, s := range v.Strings() {
		if pat.MatchString(s.Atom) {
			v.Die("a string literal mentions one of the sixteen and the rename would "+
				"change what the editor PRINTS: %s", edit.PyRepr(edit.CoreHead(s.Atom, 60)))
			return v.Done()
		}
	}
	renamed := 0
	for _, name := range w36Names {
		var from *graph.Node
		for _, d := range e.Decls(name) {
			if d.Is("extern") {
				from = d
			}
		}
		to := e.Defn("musl_" + name)
		if from == nil || to == nil {
			v.Die("%s: no external to retarget from, or no musl_%s to retarget to", name, name)
			return v.Done()
		}
		moved, err := e.RetargetUses(from, to)
		if err != nil {
			v.Die("%s -> musl_%s: %v", name, name, err)
			return v.Done()
		}
		renamed += len(moved)
	}
	// 606 in the input + 4 that section A's size expressions introduced.  THE
	// ORDER MATTERS: a phase that renamed first and edited sprintf afterwards
	// would leave four bare strlen behind, which would compile and would keep the
	// symbol.
	if renamed != sixteen+4 {
		v.Die("%d identifiers were renamed, expected %d -- %d in the input plus the four "+
			"strlen the size expressions above introduce", renamed, sixteen+4, sixteen)
		return v.Done()
	}
	v.Say(fmt.Sprintf("%d identifiers renamed to musl_*, none of them inside a literal -- %d of the "+
		"input and the four strlen the size arguments added", renamed, sixteen))

	// ---- E. what the collection is handed, as counts rather than as trust -------
	t = string(v.Text())
	for _, name := range edit.SortedKeys(w36After) {
		want := w36After[name]
		base := strings.TrimPrefix(name, "musl_")
		if b, ok := w36Before[base]; ok && base != name {
			want = before[base] + (w36After[name] - b)
		} else if b, ok := w36Before[name]; ok && name == "vim_snprintf" {
			want = before[name] + (w36After[name] - b)
		}
		if k := mentions(t, name); k != want {
			v.Die("%s has %d mentions after the cut, expected %d", name, k, want)
			return v.Done()
		}
	}
	for _, name := range append(append([]string{}, w36Names...), "sprintf") {
		if edit.WordPatternCount([]byte(t), name) > 0 {
			v.Die("%s survives as a bare name somewhere", name)
			return v.Done()
		}
	}
	var directives, bad []string
	for _, l := range strings.Split(t, "\n") {
		if strings.HasPrefix(l, "#") {
			directives = append(directives, l)
			if !strings.HasPrefix(l, "#include <") {
				bad = append(bad, l)
			}
		}
	}
	if len(directives) != nInc || len(bad) > 0 {
		v.Die("the file has %d lines starting with # and they must be the same eighteen "+
			"#includes: this phase adds no preprocessor syntax", len(directives))
		return v.Done()
	}
	v.Say("the cut is done: sprintf and f_l at 0, tolower still at 2 -- the " +
		"character-class phase's and untouched -- vim_snprintf 55 -> 68, the sixteen " +
		"musl_* at their source counts plus their own definitions, musl_fmtnum at 9 and " +
		"musl_fmtptr at 2, and eighteen #include lines and no other directive")
	return v.Done()
}
