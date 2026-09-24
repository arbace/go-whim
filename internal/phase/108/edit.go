package p108

// Whim phase 108 -- the plain host calls.  See GOALS.md II.4c and GOALS.md.
//
// THE CORE REACHES THE HOST THROUGH TWO FUNCTION POINTERS, AND THE ONE REASON THEY ARE
// POINTERS IS GONE.  Phase 102 wrote `static void (*vim_host_exit)(int);` and phase 104
// wrote `static void (*vim_host_message)(const char *, int, int);`, each installed
// through a parameter of `vim_main()` that `main()` passes.  internal/phase/102/edit.go says
// why in as many words: "a pointer the launcher installs through a parameter adds no
// external symbol, where a `musl_exit(int)` the host defines would" -- the invariant
// being `nm --extern-only --defined-only` giving exactly `main`, and the design at the
// time being TWO TRANSLATION UNITS, where the host's definition of a function the core
// calls has external linkage by construction.
//
// THE DESIGN CHANGED ON 2026-09-18 AND THE INDIRECTION DID NOT FOLLOW IT.  GOALS.md
// II.4c is now one file with two parts and the first `#include` as the boundary, so the
// host's definitions sit BELOW the core in the SAME translation unit.  A `static`
// forward declaration above and a `static` definition below is all a direct call needs,
// and nothing becomes external: the global the indirection existed to avoid does not
// appear.  So this phase spends two prototypes and gets back two objects, two
// parameters, two assignments and two arguments:
//
// static void host_exit(int r);                            beside the nine musl_
// static void host_message(const char *, int, int);        prototypes phase 103 left
// host_exit(r);            in mch_exit                     1 call site
// host_message(...);       in five core functions          8 call sites
// vim_main(int argc, char **argv)                          the signature phase 101
// wrote, back again
//
// WHERE THE DECLARATIONS GO, AND IT IS NOT "THE TOP OF THE FILE".  Phase 103 left NINE
// `musl_` prototypes in one run -- musl_host_init, musl_get_winsize, musl_term_start,
// musl_term_stop, musl_tty_keys, musl_delay, musl_wait_for_input, musl_read_input,
// musl_suspend -- and those ARE the core's declared calls to the host.  The two go at
// the end of that run, in the order their definitions appear at the bottom of the file,
// so the boundary is ELEVEN prototypes in ONE block and not nine in a block and two
// wherever an object happened to sit.  That it is above every call site is COMPUTED
// here and asserted again in the check, by line number, and not assumed; and it will
// still be above them when a later phase moves the definitions further down, because a
// forward declaration's whole job is to make definition order not matter (CLAUDE.md).
//
// THE PROTOTYPE IS BUILT OUT OF THE DEFINITION'S OWN TWO LINES, so the two cannot
// disagree.  `    static void` + `host_exit(int r)` + `;` is the text that goes in, read
// from the file rather than retyped.
//
// `static` ON BOTH, AND IT IS THE TRAP THIS PHASE CAN FALL INTO.  A prototype that
// forgets it is either a hard error -- MEASURED, `static declaration of 'host_exit'
// follows non-static declaration` -- or, if the definition forgets it too, a SILENTLY
// CORRECT BUILD with two more external symbols.  MEASURED: `nm --extern-only
// --defined-only` on that variant prints `host_exit`, `host_message` and `main` where
// it must print `main` alone.  internal/phase/108/check.go builds both variants.
//
// THE ASYMMETRY, AND IT IS WHY THIS PHASE COSTS TWO DECLARATIONS AND NOT FOUR.  In one
// translation unit everything above the cut is visible below it for free, so only the
// core -> host direction ever needs a name declared.  MEASURED on this file: the four
// functions that still hold a `va_list` -- `vim_snprintf`, `vim_vsnprintf`,
// `vim_vsnprintf_typval` and `skip_to_arg`, the ones a later phase moves below the
// boundary -- call TWENTY distinct core functions at FORTY-ONE sites, `emsg`, `iemsg`,
// `iobuff_or` and `emsg_iobuff_room` among them, and read `IObuff` once.  A two-file
// split would have had to answer for every one of those; phase 105's survey flagged
// exactly that and called it a genuine boundary question with three unattractive
// answers.  UNDER ONE FILE THERE IS NO QUESTION, and it is recorded here so that the
// earlier note does not send the next reader looking for a problem that the design
// dissolved rather than solved.
//
// THE BINARY WILL NOT BE BYTE-IDENTICAL AND THIS PHASE DOES NOT AIM FOR IT.  At -O0 a
// call through a pointer loads the pointer and calls the register; a direct call is a
// relative call to a known address.  Different instructions, and removing two file-scope
// objects moves everything after them.  MEASURED: the same 788,488 bytes, and 347,279 of
// them differ.  So the evidence is the RECORDING -- two full tools/st.sh zrecord runs,
// byte-identical -- which is how every Part II phase before 106 was checked, with a control
// for each of the two names this phase makes direct.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim108", Edit) }

var (
	whim108IncLine  = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
	whim108Boundary = regexp.MustCompile(`^static [\w ]+\**(musl_|host_)\w+\([^\n]*\);$`)
)

// Whim108 makes the two host calls plain: the function pointers the launcher
// installed become a forward declaration and a direct call, so vim_main is
// phase 101's signature again.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 167 drops the unused)
	p := edit.Ph{Tag: "hostcall", W: w}
	linesBefore := bytes.Count(text, []byte{'\n'})
	var err error

	// ---- 0. the file this edit was written against -----------------------
	// ELEVEN DIRECTIVES on the first eleven lines.  This phase adds a
	// DECLARATION and not a directive, so it must leave that where it found
	// it.
	lines := bytes.Split(text, []byte{'\n'})
	var dirIdx []int
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#")) {
			dirIdx = append(dirIdx, i)
		}
	}
	bad := len(dirIdx) != nInc
	for k, i := range dirIdx {
		if i != k {
			bad = true
		}
	}
	if bad {
		var at []string
		for _, i := range dirIdx {
			at = append(at, strconv.Itoa(i))
		}
		return nil, p.Die("the file does not have exactly eleven preprocessor directives on its first "+
			"eleven lines: %d directives at lines %s", len(dirIdx), strings.Join(at, " "))
	}
	for _, i := range dirIdx {
		if !whim108IncLine.Match(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}

	// ---- 1. the two pointers, and every mention of each ------------------
	// A PARTITION AND NOT A COUNT.  Each is taken Out below, so anything this
	// arithmetic does not account for would survive into a file where the
	// object no longer exists, and the compile would stop.
	for _, inv := range []struct {
		Name string
		want int
		why  string
	}{
		{"vim_host_exit", 3, "its declaration, the assignment in vim_main and the one " +
			"call in mch_exit"},
		{"vim_host_message", 10, "its declaration, the assignment in vim_main and " +
			"EIGHT calls"},
		{"exit_fn", 2, "vim_main's third parameter and the assignment that reads it"},
		{"message_fn", 2, "vim_main's fourth parameter and its assignment"},
		{"host_exit", 2, "the launcher's definition and the argument main() passes"},
		{"host_message", 2, "the same two, for the message call"},
		{"vim_main", 2, "its definition and the one call from the launcher"},
		{"main", 1, "the launcher's head, still the only bare `main` in the file"},
	} {
		if k := p.Mentions(text, inv.Name); k != inv.want {
			return nil, p.Die("`%s` has %d mentions, expected %d -- %s",
				inv.Name, k, inv.want, inv.why)
		}
	}
	nExitCalls := len(edit.CallsNotAfterWord(text, "vim_host_exit"))
	nMsgCalls := len(edit.CallsNotAfterWord(text, "vim_host_message"))
	if nExitCalls != 1 || nMsgCalls != 8 {
		return nil, p.Die("%d calls through vim_host_exit and %d through vim_host_message, expected 1 "+
			"and 8 -- the other two mentions of each are its declaration and its one "+
			"assignment", nExitCalls, nMsgCalls)
	}
	p.Say("`vim_host_exit` is THREE mentions and `vim_host_message` is TEN, and each " +
		"partitions into a declaration, one assignment in vim_main and its calls -- ONE " +
		"and EIGHT.  Nothing else in the file names either")

	// ---- 2. the prototypes, built Out of the definitions' own lines ------
	// The two cannot disagree with what they declare, because the text that
	// goes in is read from the text it declares.  BOTH ARE `static`: a
	// prototype that forgot the keyword is either a hard error against the
	// static definition or, with the definition changed too, a silently
	// correct build with two more external symbols.
	var protos []string
	for _, name := range []string{"host_exit", "host_message"} {
		re := regexp.MustCompile(`(?m)^    (static \w+)\n(` + regexp.QuoteMeta(name) + `\([^\n]*\))\n\{\n`)
		m := re.FindSubmatch(text)
		if m == nil {
			return nil, p.Die("`%s` is not defined in this file's shape -- `    static <type>` on one "+
				"line, the declarator on the next and `{` on the third -- so the prototype "+
				"cannot be built out of it", name)
		}
		if !bytes.HasPrefix(m[1], []byte("static ")) {
			return nil, p.Die("`%s` is not defined `static`, and a non-static definition is an external "+
				"symbol", name)
		}
		protos = append(protos, string(m[1])+" "+string(m[2])+";")
	}
	p.Sayf("the two prototypes are BUILT OUT OF THE DEFINITIONS' OWN LINES and not retyped, "+
		"so they cannot disagree with what they declare: %s", strings.Join(protos, "  /  "))

	// ---- 3. the nine prototypes phase 103 left, and the two that join them
	// The boundary is one block.  `musl_suspend` is the last of phase 103's
	// nine, and the order the two are added in is the order their definitions
	// appear at the bottom.
	text, err = p.SwapOnce(text,
		"static void musl_suspend(void);\n",
		"static void musl_suspend(void);\n"+strings.Join(protos, "\n")+"\n",
		"the last of phase 103's nine musl_ prototypes",
		"the core -> host boundary is ONE run of declarations, and these two belong in it "+
			"rather than wherever an object happened to sit")
	if err != nil {
		return nil, err
	}

	// ---- 4. the two objects go -------------------------------------------
	text, err = p.SwapOnce(text,
		"static int musl_towlower(int a);\n"+
			"\n"+
			"static void (*vim_host_message)(const char *msg, int len, int err);\n"+
			"\n",
		"static int musl_towlower(int a);\n\n",
		"the vim_host_message object, where phase 104 put it",
		"it and the blank line that separated it from what follows go together, so the "+
			"paragraphing either side of it is what it was")
	if err != nil {
		return nil, err
	}
	text, err = p.SwapOnce(text,
		"}\n"+
			"\n"+
			"static void (*vim_host_exit)(int);\n"+
			"\n"+
			"    static void\n"+
			"mch_exit(int r)\n"+
			"{\n",
		"}\n\n    static void\nmch_exit(int r)\n{\n",
		"the vim_host_exit object, where phase 102 put it -- immediately above mch_exit",
		"the same: the object and its blank line, leaving mch_exit separated from the "+
			"function above it exactly as every other function in this file is")
	if err != nil {
		return nil, err
	}

	// ---- 5. vim_main takes argc and argv, and nothing else ---------------
	text, err = p.SwapOnce(text,
		"    static int\n"+
			"vim_main(int argc, char **argv, void (*exit_fn)(int), void (*message_fn)(const char *, int, int))\n"+
			"{\n"+
			"    vim_host_exit = exit_fn;\n"+
			"    vim_host_message = message_fn;\n",
		"    static int\nvim_main(int argc, char **argv)\n{\n",
		"vim_main()'s head and the two installations",
		"the signature goes back to the one phase 101 wrote, and the two assignments and "+
			"the blank line that followed them go with the parameters they read")
	if err != nil {
		return nil, err
	}
	text, err = p.SwapOnce(text,
		"    return vim_main(argc, argv, host_exit, host_message);\n",
		"    return vim_main(argc, argv);\n",
		"main()'s one statement",
		"the launcher no longer hands the core anything but its command line")
	if err != nil {
		return nil, err
	}

	// ---- 6. the nine call sites ------------------------------------------
	text, a := edit.SubNotAfterWord(text, "vim_host_exit", "host_exit")
	text, b := edit.SubNotAfterWord(text, "vim_host_message", "host_message")
	if a != nExitCalls || b != nMsgCalls {
		return nil, p.Die("the renames took %d and %d where %d and %d were counted",
			a, b, nExitCalls, nMsgCalls)
	}
	p.Sayf("%d call site renamed to `host_exit` and %d to `host_message`, and vim_main is "+
		"`vim_main(int argc, char **argv)` again -- two objects, two parameters, two "+
		"assignments and two arguments gone, two prototypes arrived", a, b)

	// ---- 7. what the file is now -----------------------------------------
	L := bytes.Split(text, []byte{'\n'})
	if p.Mentions(text, "vim_host_exit") > 0 || p.Mentions(text, "vim_host_message") > 0 {
		return nil, p.Die("a `vim_host_*` name survives")
	}
	if p.Mentions(text, "exit_fn") > 0 || p.Mentions(text, "message_fn") > 0 {
		return nil, p.Die("a parameter name survives the signature it was written for")
	}
	for _, inv := range []struct {
		Name string
		want int
		why  string
	}{
		{"host_exit", 3, "the prototype, the one call in mch_exit and the definition"},
		{"host_message", 10, "the prototype, EIGHT calls and the definition"},
		{"vim_main", 2, "its definition and the one call from the launcher"},
		{"main", 1, "still the only bare `main` in the file"},
	} {
		if k := p.Mentions(text, inv.Name); k != inv.want {
			return nil, p.Die("`%s` has %d mentions after the edit, expected %d -- %s",
				inv.Name, k, inv.want, inv.why)
		}
	}

	// DECLARATION BEFORE USE, COMPUTED.  The prototype must be above every
	// call and the definition below every one of them; a later phase moves
	// the definitions further down and this stays true, which is the whole
	// point of a forward declaration.
	isProto := map[string]bool{}
	for _, s := range protos {
		isProto[s] = true
	}
	for _, name := range []string{"host_exit", "host_message"} {
		var proto, defn, uses []int
		for i, l := range L {
			s := string(l)
			switch {
			case isProto[s] && strings.Contains(s, name+"("):
				proto = append(proto, i)
			case strings.HasPrefix(s, name+"(") && i+1 < len(L) && string(L[i+1]) == "{":
				defn = append(defn, i)
			}
		}
		for i, l := range L {
			if len(edit.CallsNotAfterWord(l, name)) == 0 {
				continue
			}
			if edit.Contains(proto, i) || edit.Contains(defn, i) {
				continue
			}
			uses = append(uses, i)
		}
		if len(proto) != 1 || len(defn) != 1 || len(uses) == 0 {
			return nil, p.Die("`%s` has %d prototypes, %d definitions and %d call sites, and this phase "+
				"needs exactly one, one and at least one", name, len(proto), len(defn), len(uses))
		}
		minUse, maxUse := uses[0], uses[len(uses)-1]
		if proto[0] >= minUse {
			return nil, p.Die("`%s`'s prototype is at line %d and its first call at line %d -- a call "+
				"above its declaration", name, proto[0]+1, minUse+1)
		}
		if defn[0] <= maxUse {
			return nil, p.Die("`%s`'s definition is at line %d and its last call at line %d -- the "+
				"definition must be below every call, which is what makes the prototype "+
				"load-bearing", name, defn[0]+1, maxUse+1)
		}
		plural := "s"
		if len(uses) == 1 {
			plural = ""
		}
		var at []string
		for _, u := range uses {
			at = append(at, strconv.Itoa(u+1))
		}
		p.Sayf("`%s`: prototype at line %d, %d call site%s at %s, definition at line %d -- "+
			"declared above every use and defined below every one of them",
			name, proto[0]+1, len(uses), plural, strings.Join(at, " "), defn[0]+1)
	}

	var block []int
	for i, l := range L {
		if string(l) == "static void musl_suspend(void);" {
			block = append(block, i)
		}
	}
	if len(block) != 1 || string(L[block[0]+1]) != protos[0] || string(L[block[0]+2]) != protos[1] {
		return nil, p.Die("the two prototypes are not the two lines below `static void " +
			"musl_suspend(void);` -- the core -> host boundary is one block")
	}
	nBoundary := 0
	for i := block[0]; i >= 0 && whim108Boundary.Match(L[i]); i-- {
		nBoundary++
	}
	nBoundary += 2
	p.Sayf("the core -> host boundary is now ONE run of %d prototypes ending at line %d: "+
		"phase 103's nine `musl_` and these two", nBoundary, block[0]+3)

	if n := len(L) - 1; n != linesBefore-4 {
		return nil, p.Die("the file is %d lines and the input was %d -- expected exactly 4 fewer: two "+
			"objects with their blank lines is four, two prototypes back is two, and the "+
			"two assignments are two -- the canonical text writes no blank line inside a "+
			"function, so there is none here to take with them", n, linesBefore)
	}
	var d [][]byte
	for _, l := range L {
		if bytes.HasPrefix(l, []byte("#")) {
			d = append(d, l)
		}
	}
	ok := len(d) == nInc
	for _, l := range d {
		if !bytes.HasPrefix(l, []byte("#include <")) {
			ok = false
		}
	}
	for i := 0; i < 11 && ok; i++ {
		if i >= len(L) || string(L[i]) != string(d[i]) {
			ok = false
		}
	}
	if !ok {
		return nil, p.Die("the output does not have exactly the eleven `#include` directives phase 104 " +
			"left, on its first eleven lines -- this phase adds a DECLARATION and not a " +
			"directive")
	}
	p.Sayf("%d -> %d lines, the eleven #includes untouched, and no run of two blank lines",
		linesBefore, len(L)-1)
	return text, nil
}
