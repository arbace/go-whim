package p105

// Whim phase 105 -- the variadic collapse: the seven wrappers that walk a va_list are
// expanded at their 129 call sites.  See GOALS.md II.4c and GOALS.md.
//
// GOALS.md II.4c settled the variadic question on 2026-09-18: the formatter goes to the
// host rather than `__builtin_va_list` going into `editor.c`.  It also said the move
// splits in two and that ONLY THE SECOND NEEDS TWO FILES.  This is the first: everything
// that can be done about `va_start` inside one translation unit, done here, where the
// declared delta can be nothing at all and a byte-identical recording can say so.
//
// WHAT IT DOES.  C cannot forward `...` -- which is why `vsnprintf` exists beside
// `snprintf` -- so a wrapper that takes `...`, opens a `va_list` and hands it to
// `vim_vsnprintf` cannot survive a split unless the formatter goes with it.  There are
// eight such functions.  Seven of them are wrappers over the eighth, and every one of
// their call sites is rewritten here into `vim_snprintf(...)` plus the tail the wrapper
// ran afterwards.  `va_start` goes from EIGHT functions to ONE.
//
// THE PHASE'S WHOLE PRODUCT IS THAT `va_start` APPEARS ONCE.  It frees no libc symbol,
// removes no Ex command, removes no option and changes no message -- and the binary gets
// BIGGER, because 129 call sites now carry a format call and a tail call where they
// carried one call, which at -O0 is the expected sign.  `nm -u` is required to be THE
// SAME SET both ways, asserted as a `comm` that is empty in both directions rather than
// as a count: a reader meeting a 129-site phase expects a symbol to fall, and none can.
// A PURE RESTRUCTURE INSIDE ONE TRANSLATION UNIT FREES NOTHING.  `vim_vsnprintf_typval`
// still does every conversion, in the same file, and `<stdarg.h>`'s three names are
// macros and a compiler builtin type, which contribute no symbol at all.  The symbols
// and the header go when `vim_snprintf`, `vim_vsnprintf`, `vim_vsnprintf_typval` and
// `skip_to_arg` LEAVE THE FILE, and that is the split.  What this phase moves is not
// code across a boundary but the POSSIBILITY of drawing one: eight functions calling
// `va_start` cannot be split, one can.
//
// ------------------------------------------------------------------------------------
// THE INVENTORY -- 129 SITES, AND THE COUNT IS THE PHASE'S FIRST ASSERTION
//
// wrapper                 mentions  protos  own def  CALL SITES
// smsg                          12       1        1          10
// smsg_attr                      4       1        1           2
// smsg_attr_keep                 2       0        1           1
// semsg                         96       1        1          94
// siemsg                        12       1        1          10
// vim_snprintf_add               3       1        1           1
// vim_snprintf_safelen          13       1        1          11
// 129
//
// `vim_snprintf`'s own 73 mentions are NOT touched: it is the survivor, the one function
// left calling `va_start`, and its rename to `musl_snprintf` belongs to the split.
//
// ------------------------------------------------------------------------------------
// THE TAILS ALREADY EXIST, WHICH IS WHAT MAKES THIS SMALL
//
// Read each wrapper beside its non-variadic twin and the wrapper is the twin with a
// format in front of it:
//
// semsg(s, ...)                        emsg(s)
// if (emsg_not_now()) return TRUE;     if (emsg_not_now()) return TRUE;
// if (IObuff == NULL) return emsg_core(s);
// format into IObuff
// return emsg_core((char *)IObuff);    return emsg_core(s);
//
// `semsg`'s tail IS `emsg()`.  `siemsg`'s IS `iemsg()`.  `smsg`'s is `msg()`,
// `smsg_attr`'s `msg_attr()`, `smsg_attr_keep`'s `msg_attr_keep(..., TRUE)`.  All five
// already exist and all five are already called from elsewhere, so THIS PHASE WRITES NO
// MESSAGE LOGIC AT ALL.  What is left over is the two guards, and those become helpers.
//
// THE SIZE-ZERO TRICK IS WHAT MAKES THE EXPANSION EXACTLY FAITHFUL, and it was measured
// rather than assumed.  `vim_vsnprintf_typval` guards every write with
// `if (str_l < str_m)` and terminates with `if (str_m > 0)`, so `vim_snprintf(buf, 0,
// ...)` writes nothing and does not fault -- measured with a build whose first act is
// `vim_snprintf(canary, 0, "%s %d %ld %c %x", ...)`, `vim_snprintf(NULL, 0, ...)` and
// `vim_snprintf(NULL, 0, "%s", (char *)NULL)`: all eight canary bytes untouched, no
// fault on the NULL destination.  So a helper returning 0 reproduces BOTH of the
// wrapper's guards -- `emsg_off > 0` and `IObuff == NULL` -- with no conditional at the
// site.  That is why there are five helpers and not an `if`/`else` written out 117
// times.
//
// THE FIVE HELPERS, against SEVEN deleted definitions: two fewer, 1,758 -> 1,756.
//
// iobuff_room()        0 if IObuff is NULL, else IOSIZE
// emsg_iobuff_room()   0 if IObuff is NULL OR emsg_not_now(), else IOSIZE
// iobuff_or(s)         IObuff, or s when IObuff is NULL -- what the wrapper handed
// emsg_core() in that arm
// safelen_result()     vim_snprintf_safelen()'s arithmetic, lifted out unchanged
// append_room()        vim_snprintf_add()'s, likewise
//
// Trace `semsg`'s three cases against the original and nothing is lost.  `emsg_off > 0`:
// room 0, nothing written, `emsg()` returns TRUE having consulted `emsg_not_now()`
// itself.  `IObuff == NULL`: room 0, nothing written, `iobuff_or` hands `emsg()` the
// unformatted format, exactly as the wrapper handed it to `emsg_core`.  Otherwise:
// formatted, and `emsg()` calls `emsg_core((char *)IObuff)`.  `siemsg` against `iemsg`
// is the same three cases.
//
// TWO MICRO-DIVERGENCES, STATED RATHER THAN HIDDEN.  (i) `vim_snprintf` is CALLED in the
// two suppressed cases where the wrapper called nothing; with `str_m == 0` it writes
// nothing but does run `parse_fmt_types`, which mallocs and frees an `ap_types` array.
// (ii) `vim_snprintf_safelen`'s `str_m == 0` early return now happens AFTER the
// formatter has been entered rather than before, with the same result.  Neither is
// observable -- 263 probes and two full recordings say so -- and both are the price of
// not putting a conditional at 129 sites.
//
// THE `emsg_not_now()` HALF OF `emsg_iobuff_room()` IS KEPT ALTHOUGH IT IS MEASURED
// UNOBSERVABLE.  A build of this phase's own output with that half of the guard removed
// moves 0 of 263 probes and 0 recording files: with `emsg_off > 0` the formatter would
// write into `IObuff`, and nothing anywhere reads `IObuff` before the next thing writes
// it.  It is kept because the phase claims THE SAME COMPUTATION and not merely the same
// output, and a phase whose declaration is "nothing at all" should not knowingly compute
// something new.  The user's decision, and it is recorded here rather than dropped
// quietly.
//
// `safelen_result`'s CLAMP IS UNTESTABLE BY ANYTHING AVAILABLE, and that is said plainly
// rather than papered over with a probe that cannot fail.  A build with
// `return ((size_t)str_l >= str_m) ? str_m - 1 : (size_t)str_l;` reduced to
// `return (size_t)str_l;` moves 0 of 263 probes and 0 recording files -- the clamp never
// fires in anything this pipeline can drive, because reaching it means overflowing
// `fileinfo`'s 1,025-byte line.  The clamp is copied verbatim from the wrapper, so the
// risk is nil; the evidence simply does not reach it, and the check says so instead of
// claiming otherwise.
//
// ------------------------------------------------------------------------------------
// THE THING A READER EXPECTS TO BE A PROBLEM AND IS NOT: NON-LITERAL FORMATS
//
// MEASURED, and it is the opposite of the expected answer.  EVERY ONE of `semsg`'s 94
// formats is `_(e_name)` or `(const char *)(_(e_name))`, where `e_name` is a
// `static char e_name[] = "E123: ...";` array -- whim's constant fold turned upstream's
// string macros into arrays, so THERE IS NO LITERAL AT A `semsg` SITE ANYWHERE IN THE
// FILE.  It changes nothing, because the expansion does not need to know the format: it
// copies the format EXPRESSION verbatim into `vim_snprintf`'s third argument, and
// `vim_snprintf` is itself variadic and reads the format at run time.
//
// The two things that could have gone wrong were both measured.
//
// -Wformat COVERAGE DOES NOT DISAPPEAR, IT MOVES.  The wrappers carry
// `__attribute__((format(printf, 1, 2)))` / `(2, 3)` / `(3, 4)`, and `vim_snprintf`
// carries `format(printf, 3, 4)` -- and every expansion puts the format expression at
// `vim_snprintf`'s third parameter, so gcc checks exactly what it checked before.
// `-Wformat=2` gives 115 `-Wformat-nonliteral` warnings in 53 functions before this
// phase and THE IDENTICAL 115 IN THE IDENTICAL 53 after it.  `_()` and `NGETTEXT()`
// are `static inline __attribute__((format_arg(1)))`, so gcc sees through them either
// side.  That equality is the check a mis-expanded argument list would fail and the
// build would not, and internal/phase/105/check.go makes it.
//
// DOUBLE EVALUATION OF THE FORMAT EXPRESSION.  The expansion mentions the format twice
// -- once in `vim_snprintf`, once in the tail's `iobuff_or(F)`.  MEASURED over all 129
// sites: every format expression is side-effect-free.  118 are `_(e_name)` / a bare
// `e_name` / a literal, 8 are `NGETTEXT(a, b, n)` (a pure inline `return`), 2 are a
// `? :` over two `_()`s, 1 is a parameter.  None contains an assignment, an increment,
// or a call to anything but `_` and `NGETTEXT`.
//
// ------------------------------------------------------------------------------------
// THE TRAPS, ALL MEASURED, AND EACH ONE CAN PRODUCE A WRONG PHASE SILENTLY
//
// 1. `emsg(` IS A SUBSTRING OF `semsg(` AND OF `siemsg(`.  A textual replace of
// `emsg(` corrupts 106 lines.  Every search here is by word boundary --
// `(?<![A-Za-z0-9_])NAME(?![A-Za-z0-9_])\s*\(` -- and the same applies to `smsg` as
// a prefix of `smsg_attr` and `smsg_attr_keep`, and `vim_snprintf` of
// `vim_snprintf_add` and `vim_snprintf_safelen`.  `grep -c vim_snprintf` counts all
// three; `grep -ow` counts one.
// 2. SIX SITE TEXTS ARE DUPLICATED ACROSS 13 SITES, so an exact-text anchor with
// `count == 1` FAILS ON A CORRECT PHASE.  The remedy is not to count anchors: THIS
// PHASE IS A RULE.  It finds every call by word boundary, splits its arguments by
// balanced parens with string and character literals honoured, and rewrites.  The
// residue is 0 and the program does not care what upstream renamed.
// 3. THE SITES COME IN THREE SHAPES AND AN EDIT THAT EMITS TWO STATEMENTS
// UNCONDITIONALLY GETS TWO OF THEM WRONG.  92 are a plain statement alone on its
// line, and become two lines at the same indentation.  7 are a WHOLE BLOCK ON ONE
// LINE -- `{   semsg(...);         goto error;     }   ;` inside `parse_fmt_types`
// -- where two lines would put a statement in front of the closing brace, so the
// expansion goes inline on the same line.  30 are in VALUE POSITION: 18 `semsg`es
// inside `return (..., rc_did_emsg = TRUE, (void *)NULL) ;` comma expressions in
// the regexp engine, all eleven `vim_snprintf_safelen`s (whose value is consumed at
// every site, five of them `+=`), and `vim_snprintf_add`'s one.  A statement is
// told from an operand by the character after the closing paren.  THE COMMA SHAPE
// ALREADY EXISTS IN THE FILE -- `return (iemsg((e_internal_error_in_regexp)),
// rc_did_emsg = TRUE, (void *)NULL) ;` -- so the expansion invents no idiom.
// 4. ONE NEW PROTOTYPE IS REQUIRED and it is not one of the five helpers'.
// `emsg_iobuff_room()` calls `emsg_not_now()`, which is defined 130 lines below the
// place the helpers go and had no forward declaration.  Without
// `static int emsg_not_now(void);` the build fails with *"static declaration of
// 'emsg_not_now' follows non-static declaration"* -- loud, not silent -- and
// `deadprotos.py` does not take it away once added.
// 5. `smsg_attr_keep` HAS NO PROTOTYPE AND `vim_snprintf` HAS TWO.  A phase that
// deletes "the prototype and the definition" for each of seven names fails on the
// first and leaves one behind on the second.  Measured: 1, 1, 0, 1, 1, 1, 1.
// 6. NO SITE PASSES ZERO VARIADIC ARGUMENTS, so the question "does
// `vim_snprintf(buf, n, s)` differ from a plain copy when there is nothing to
// substitute" never arises.  It would if a future edit added such a site, because
// `vim_snprintf` interprets `%` in the format and a copy does not.
//
// `vim_snprintf`'s SECOND PROTOTYPE GOES WITH THE WRAPPERS.  Line 35776,
// `static int vim_snprintf(char *str, size_t str_m, const char *fmt, ...);`, exists only
// because the five message wrappers are defined above `vim_snprintf` and needed to call
// it.  All five are leaving, the declaration block at the top already has the same
// prototype, and the sweep does not take a redundant one -- so it is removed here by
// name.  It is this phase's own residue and not a tidy smuggled in.
//
// NO `need 105 swept`, AND IT WAS CHECKED RATHER THAN ASSUMED: the edit finds its sites
// by word boundary and balanced parens over the whole file, not by counted anchors, so
// it gives the same answer on swept and unswept text.  The only counts it asserts are
// the seven wrappers' whole-file mention totals, which no sweep moves.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"github.com/arbace/go-whim/internal/cutil"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim105", Edit) }

// w105Wrap is the seven wrappers that walk a va_list, and their whole-file
// mention totals in q104.  `vim_snprintf`'s OWN count is deliberately NOT
// asserted up front: phase 104 formats its host message with it, so the number
// before the edit is the message layer's business and not this phase's.
var w105Wrap = []struct {
	Name string
	want int
}{
	{"smsg", 12}, {"smsg_attr", 4}, {"smsg_attr_keep", 2}, {"semsg", 96},
	{"siemsg", 12}, {"vim_snprintf_add", 3}, {"vim_snprintf_safelen", 13},
}

var w105Room = map[string]string{
	"smsg": "iobuff_room()", "smsg_attr": "iobuff_room()",
	"smsg_attr_keep": "iobuff_room()", "semsg": "emsg_iobuff_room()",
	"siemsg": "emsg_iobuff_room()",
}

var w105Lead = map[string]int{
	"smsg": 0, "smsg_attr": 1, "smsg_attr_keep": 1, "semsg": 0, "siemsg": 0,
}

// Whim105 is the variadic collapse: the seven wrappers expanded at their 129 call
// sites into vim_snprintf plus the tail each already had, so `va_start` appears
// ONCE in the whole file.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "format", W: w}
	t := string(text)

	mentions := func(s, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(s, -1))
	}
	sub := func(old, new string, n int, tag string) error {
		c := cutil.CountAnchor(t, old)
		if c != n {
			return p.Die("%s: `%s` occurs %d times, expected %d",
				tag, edit.CoreHead(strings.Split(strings.TrimSpace(old), "\n")[0], 70), c, n)
		}
		t = cutil.ReplaceAnchor(t, old, new, -1)
		return nil
	}
	// delfunc deletes a whole definition by brace matching from its name line.
	delfunc := func(sigline, tag string) error {
		if k := strings.Count(t, sigline); k != 1 {
			return p.Die("%s: the definition line `%s` occurs %d times, expected 1",
				tag, edit.CoreHead(strings.TrimSpace(sigline), 60), k)
		}
		i := strings.Index(t, sigline)
		k := strings.Index(t[i:], "{") + i
		d := 0
		for {
			if t[k] == '{' {
				d++
			} else if t[k] == '}' {
				d--
				if d == 0 {
					break
				}
			}
			k++
		}
		a := strings.LastIndex(t[:i], "\n")
		st := strings.LastIndex(t[:a], "\n") + 1
		t = t[:st] + t[strings.Index(t[k:], "\n")+k+1:]
		return nil
	}
	linesBefore := len(strings.Split(t, "\n"))

	// ---- 0. this is the file the phase was written against --------------------
	for _, wr := range w105Wrap {
		if k := mentions(t, wr.Name); k != wr.want {
			return nil, p.Die("the input has %d mentions of `%s`, expected %d -- this is not the tree "+
				"this phase was written against", k, wr.Name, wr.want)
		}
	}
	for _, v := range []struct {
		Name string
		want int
	}{{"va_start", 8}, {"va_list", 15}, {"va_end", 10}} {
		if k := mentions(t, v.Name); k != v.want {
			return nil, p.Die("the input has %d mentions of `%s`, expected %d", k, v.Name, v.want)
		}
	}
	vsBefore := mentions(t, "vim_snprintf")
	p.Say("the input is r21: eight functions call `va_start`, seven of them wrappers over the " +
		"eighth, and their mention totals are 12 4 2 96 12 3 13")

	// ---- 1. the seven definitions ---------------------------------------------
	for _, sig := range []string{
		"smsg(const char *s, ...)", "smsg_attr(int attr, const char *s, ...)",
		"smsg_attr_keep(int attr, const char *s, ...)", "semsg(const char *s, ...)",
		"siemsg(const char *s, ...)",
		"vim_snprintf_add(char *str, size_t str_m, const char *fmt, ...)",
		"vim_snprintf_safelen(char *str, size_t str_m, const char *fmt, ...)",
	} {
		if err := delfunc("\n"+sig+"\n", "D"); err != nil {
			return nil, err
		}
	}

	// ---- 2. the prototypes -----------------------------------------------------
	// Six, not seven: `smsg_attr_keep` never had one.  And `vim_snprintf`'s SECOND
	// prototype, which existed only because the wrappers sit above its definition.
	if err := sub(w105lit1, "", 1, "P0"); err != nil {
		return nil, err
	}
	for _, pr := range []string{w105lit2, w105lit3, w105lit4, w105lit5, w105lit6, w105lit7} {
		if err := sub(pr, "", 1, "P"); err != nil {
			return nil, err
		}
	}
	if err := sub(w105lit8, w105lit9, 1, "P1"); err != nil {
		return nil, err
	}

	// ---- 3. the five helpers, where the message wrappers were -----------------
	if err := sub(w105Anchor, w105Helpers+w105Anchor, 1, "H"); err != nil {
		return nil, err
	}

	// ---- 4. the 129 call sites -------------------------------------------------
	names := make([]string, len(w105Wrap))
	for i, wr := range w105Wrap {
		names[i] = wr.Name
	}
	// The Python spells the boundaries `(?<![A-Za-z0-9_])name(?![A-Za-z0-9_])`.
	// RE2 has no lookaround: the trailing one is implied by the `\s*\(` that
	// follows -- `smsg` cannot match inside `smsg_attr(` because `_` is not `(`,
	// and the engine then takes the longer alternative -- and the leading one is
	// the byte before, tested.
	call := regexp.MustCompile(`(` + strings.Join(names, "|") + `)\s*\(`)

	closeParen := func(s string, i int) (int, error) {
		d := 0
		for i < len(s) {
			c := s[i]
			if c == '"' || c == '\'' {
				q := c
				i++
				for s[i] != q {
					if s[i] == '\\' {
						i += 2
					} else {
						i++
					}
				}
			} else if c == '(' {
				d++
			} else if c == ')' {
				d--
				if d == 0 {
					return i, nil
				}
			}
			i++
		}
		return 0, p.Die("unbalanced parentheses")
	}
	splitArgs := func(s string) []string {
		var args []string
		var cur strings.Builder
		d, i := 0, 0
		for i < len(s) {
			c := s[i]
			if c == '"' || c == '\'' {
				q := c
				j := i + 1
				for s[j] != q {
					if s[j] == '\\' {
						j += 2
					} else {
						j++
					}
				}
				cur.WriteString(s[i : j+1])
				i = j + 1
				continue
			}
			if c == '(' || c == '[' || c == '{' {
				d++
			} else if c == ')' || c == ']' || c == '}' {
				d--
			}
			if c == ',' && d == 0 {
				args = append(args, strings.TrimSpace(cur.String()))
				cur.Reset()
			} else {
				cur.WriteByte(c)
			}
			i++
		}
		args = append(args, strings.TrimSpace(cur.String()))
		return args
	}

	shapes := map[string]int{"plain": 0, "inline": 0, "value": 0}
	counts := map[string]int{}
	for _, n := range names {
		counts[n] = 0
	}
	pos := 0
	for {
		var m []int
		for {
			mm := call.FindStringSubmatchIndex(t[pos:])
			if mm == nil {
				break
			}
			for i := range mm {
				if mm[i] >= 0 {
					mm[i] += pos
				}
			}
			if mm[0] > 0 && edit.IsWordByte(t[mm[0]-1]) {
				pos = mm[0] + 1
				continue
			}
			m = mm
			break
		}
		if m == nil {
			break
		}
		name := t[m[2]:m[3]]
		op := m[1] - 1
		cp, err := closeParen(t, op)
		if err != nil {
			return nil, err
		}
		args := splitArgs(t[op+1 : cp])
		counts[name]++

		var rep string
		switch name {
		case "vim_snprintf_safelen":
			// ALWAYS AN EXPRESSION: the value is consumed at every one of the
			// eleven sites, five of them `+=`, so two statements cannot say this.
			if len(args) < 4 {
				return nil, p.Die("vim_snprintf_safelen with %d arguments", len(args))
			}
			rep = fmt.Sprintf("safelen_result(%s, %s, vim_snprintf(%s))",
				args[0], args[1], strings.Join(args, ", "))
			shapes["value"]++
		case "vim_snprintf_add":
			if len(args) < 4 {
				return nil, p.Die("vim_snprintf_add with %d arguments", len(args))
			}
			rep = fmt.Sprintf("vim_snprintf(%s +  musl_strlen((char *)(%s)) , append_room(%s, %s), %s)",
				args[0], args[0], args[0], args[1], strings.Join(args[2:], ", "))
			shapes["value"]++
		default:
			nl := w105Lead[name]
			if len(args) < nl+2 {
				return nil, p.Die("`%s` with %d arguments -- no site passes zero variadic arguments",
					name, len(args))
			}
			f := args[nl]
			a := fmt.Sprintf("vim_snprintf((char *)IObuff, %s, %s)",
				w105Room[name], strings.Join(append([]string{f}, args[nl+1:]...), ", "))
			var b string
			switch name {
			case "smsg":
				b = fmt.Sprintf("msg(iobuff_or(%s))", f)
			case "smsg_attr":
				b = fmt.Sprintf("msg_attr(iobuff_or(%s), %s)", f, args[0])
			case "smsg_attr_keep":
				b = fmt.Sprintf("msg_attr_keep(iobuff_or(%s), %s, TRUE)", f, args[0])
			case "semsg":
				b = fmt.Sprintf("emsg(iobuff_or(%s))", f)
			default:
				b = fmt.Sprintf("iemsg(iobuff_or(%s))", f)
			}
			if cp+1 < len(t) && t[cp+1] == ';' {
				bol := strings.LastIndex(t[:m[0]], "\n") + 1
				eol := strings.Index(t[cp:], "\n") + cp
				if strings.TrimSpace(t[bol:m[0]]) == "" && strings.TrimSpace(t[cp+2:eol]) == "" {
					rep = a + ";\n" + t[bol:m[0]] + b + ";"
					shapes["plain"]++
				} else {
					// A WHOLE BLOCK ON ONE LINE.  Two lines here would leave a
					// statement in front of the closing brace.
					rep = a + "; " + b + ";"
					shapes["inline"]++
				}
				t = t[:m[0]] + rep + t[cp+2:]
				pos = m[0] + len(rep)
				continue
			}
			// VALUE POSITION: the comma shape the regexp engine already uses.
			rep = fmt.Sprintf("(%s, %s)", a, b)
			shapes["value"]++
		}
		t = t[:m[0]] + rep + t[cp+1:]
		pos = m[0] + len(rep)
	}

	total := 0
	for _, n := range names {
		total += counts[n]
	}
	tally := func() string {
		Out := make([]string, len(names))
		for i, n := range names {
			Out[i] = fmt.Sprintf("%s %d", n, counts[n])
		}
		return strings.Join(Out, " ")
	}
	if total != 129 {
		return nil, p.Die("%d call sites, expected 129 -- %s", total, tally())
	}
	for _, wr := range w105Wrap {
		if k := mentions(t, wr.Name); k != 0 {
			return nil, p.Die("`%s` still has %d mentions after the expansion", wr.Name, k)
		}
	}
	for _, v := range []struct {
		Name string
		want int
	}{{"va_start", 1}, {"va_list", 8}, {"va_end", 3}} {
		if k := mentions(t, v.Name); k != v.want {
			return nil, p.Die("`%s` has %d mentions after the expansion, expected %d",
				v.Name, k, v.want)
		}
	}
	if k := mentions(t, "vim_snprintf"); k != vsBefore-1+total {
		return nil, p.Die("`vim_snprintf` went from %d to %d, expected %d -- one prototype away and one "+
			"mention at each of the %d sites", vsBefore, k, vsBefore-1+total, total)
	}

	commaTally := make([]string, len(names))
	for i, n := range names {
		commaTally[i] = fmt.Sprintf("%s %d", n, counts[n])
	}
	p.Sayf("129 call sites expanded: %s", strings.Join(commaTally, ", "))
	p.Sayf("%d plain statements (two lines), %d whole blocks on one line (inline), %d in value "+
		"position -- the 18 `return (semsg(...), rc_did_emsg = TRUE, NULL)` comma "+
		"expressions, the 11 safelens whose value is consumed, and the one append",
		shapes["plain"], shapes["inline"], shapes["value"])
	p.Sayf("`va_start` 8 -> 1, `va_list` 15 -> 8, `va_end` 10 -> 3; `vim_snprintf` %d -> %d; "+
		"seven definitions and six prototypes gone, five helpers and six declarations in; "+
		"lines %d -> %d",
		vsBefore, mentions(t, "vim_snprintf"), linesBefore, len(strings.Split(t, "\n")))
	return []byte(t), nil
}
