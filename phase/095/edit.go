package p095

// Whim phase 95 -- the options nothing reads.  See GOAL.md.
//
// Phases 89 to 94 took every way to reach a file and then the refusal that guarded
// the text.  What they left behind is a set of SETTINGS: `options[]` rows whose
// global nothing reads any more, so that `:set fsync?` answers a question about
// machinery that is not there.  An option that cannot do anything is a lie, and the
// same argument that removed `:write` removes `'write'`.
//
// WHICH ROWS GO IS COMPUTED, NOT LISTED.  The edit walks `options[]`, finds each
// row's `(char_u *)&p_xx` and counts readers of that global outside the row, with
// `dropoptions --strict`'s own exclusions -- another row, the row's `var`
// field, the variable's own declaration, and taking the address, which is an
// identity test and not a dereference.  Exactly SEVEN of the 114 rows have no
// reader, and the program requires that set rather than naming six of them:
//
// fsync       p_fs       PV_BOTH   goes, but needs droplocal.py b_p_fs first
// modified    p_mod      PV_BUF    STAYS -- see below
// prompt      p_prompt   PV_NONE   goes
// readonly    p_ro       PV_BUF    goes, by GOALS.md II decision 5
// undoreload  p_ur       PV_NONE   goes
// write       p_write    PV_NONE   goes
// writeany    p_wa       PV_NONE   goes
//
// `'modified'` HAS NO READER OF `p_mod` EITHER AND MUST NOT GO.  GOALS.md II.5
// decision 5 keeps it: the state it reports lives in `b_changed`, not in `p_mod`, so
// `:set modified?` answers correctly and the row is not a lie.  A computation that
// took "no reader" as the criterion would delete it, which is why the seven are
// computed and the six are chosen.  `dropoptions` refuses it anyway, on the
// PV_ guard.
//
// `'paste'` IS EXEMPT FOR EVER, and this is the comment that says so -- GOALS.md II.2d
// and II.5 decision 8, the user's standing promise.  `p_paste` has 12 mentions here and
// has them afterwards, and its five save slots `p_ai_nopaste p_et_nopaste
// p_sts_nopaste p_tw_nopaste p_wm_nopaste` are the non-pointer orphans
// `orphanopts` reports and tolerates, before and after, identically.  THE NEXT
// PERSON TO RUN THE COMPUTATION MUST NOT "FIX" THEM.  `+{command}` is likewise
// untouched, for the same promise.
//
// FOUR PARTS.  A and B are the tools' work; C is the only live code here.
//
// A  the four clean rows, `dropoptions --strict prompt undoreload write
// writeany`.  The sweep then takes the four globals as -Wunused-variable.
// B  `'fsync'`, which --strict alone REFUSES -- not on a reader but on the PV_
// guard, because the row is what initialises the global ('tagcase' taught that
// by segfaulting before the first keystroke).  `droplocal.py b_p_fs` is the
// other half and goes first: six plumbing sites, including get_varp()'s two-line
// "local if set" form.  Then `--strict --local fsync`.
// C  `'readonly'`, which is LIVE CODE and not an inert row.  `p_ro` the global has
// had no reader since whim; what survives is the buffer-local `b_p_ro`, and
// since phase 89 nothing but `:set ro` can set it -- decision 5's premise.  Five
// edits, in this order and for this reason:
// 1  the W10 warning.  `change_warning()` and its six call sites, each one
// statement on a line of its own.  There is NO PROTOTYPE -- it is defined
// above its first call -- so a program that removes one fails loudly.  This
// also takes the `ui_delay(1002L, TRUE)` that phase 85's GOAL.md named as
// one of the eight other pauses.
// 2  the `[RO]` in `fileinfo()`.  THE FORMAT STRING AND THE ARGUMENT MOVE
// TOGETHER -- `%s%s%s%s%s%s` to `%s%s%s%s%s` -- and nothing in the build
// checks a vim_snprintf_safelen count.
// 3  the `[RO]` on the status line, in `win_redr_status()`: the name-padding
// disjunct and the block that appends it.
// 4  `did_set_readonly()`, BY NAME and with the reason: it is the row's
// callback and the row is its only other reference, but droplocal.py runs
// in the same edit and would otherwise find it still reading `b_p_ro`.
// Measured without it: `droplocal: b_p_ro still has 1 mentions after the
// plumbing went`, which is the tool working.  The alternative is an inner
// sweep; this is cheaper and honest.
// 5  the row, then `droplocal.py b_p_ro` -- three plumbing sites.
//
// WHAT THE SWEEP THEN FINDS: `SHM_RO`, `BV_FS`, `BV_RO`, the static string
// `w_readonly` inside change_warning(), the `b_did_warn` field -- which becomes dead
// only after BOTH change_warning and did_set_readonly have gone, so removing one and
// not the other leaves a field with one reader and one writer that no tool reports --
// and the six globals.
//
// THE FLAG LETTERS ARE NOT TOUCHED, AND THAT IS A DECISION.  `'cpoptions'` and
// `'shortmess'` each have a validity list that is a separate string literal from the
// value, so removing a letter from a list cannot move `:set cpo?` or `:set shm?`.
// But `:set shm=F` is accepted silently and `:set shm=y` answers E539, and dropping a
// letter from the list turns the first into the second -- a behaviour change no
// corpus case, Ex row, argv row or pty scenario can see, which is exactly what
// GOALS.md core rule 2 exists to prevent.  Accepting a letter that does nothing is
// what upstream does for every feature a build lacks.  Measured: 23 of 'cpoptions'
// 60 letters and 14 of 'shortmess' 23 are inert here, and THIS PHASE MAKES EXACTLY
// ONE MORE SO -- `'shortmess'`'s `r`, whose SHM_RO the sweep takes with the `[RO]`
// indicator.  The check asserts both literals character for character.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 85 does, and the source goes with it as
// $state/old.c.  The check needs both, and needs them more than any phase so far:
// THIS PHASE DECLARES NOTHING, because `:set` is the one thing the core's instrument
// cannot read, and the probes are the whole evidence.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// ---- A. the four clean rows ----------------------------------------------------------
// --strict is the guard: it refuses a row while anything still reads its global,
// because the row is what INITIALISES that global.  The sweep takes the four
// variables afterwards as -Wunused-variable.
// ---- B. 'fsync', where --strict alone is NOT the guard --------------------------------
// The row is PV_BOTH + PV_BUF + BV_FS, so dropoptions stops on the PV_ guard before
// the reader test is ever reached, and its message talks about a segfault at startup
// rather than about readers.  droplocal.py is the other half and goes first.
// ---- C5. 'readonly': the row, then the field -------------------------------------------
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() {
	edit.Register("whim95", Edit)
	// THE SECOND HEREDOC IS A SECOND REGISTRATION, not a tail of the first, and
	// the position is the reason: two `tools/st.sh droplocal` and two
	// `tools/st.sh dropoptions` calls run BETWEEN them, and the counts this one
	// asserts are the counts after those four have run.  Folding the two into
	// one call would move the assertion to before its subject.
	edit.Register("whim95rows", Whim95Rows)
}

var (
	z12OptRow = regexp.MustCompile(`(?m)^[ \t]*\{"([a-z]+)",`)
	z12Var    = regexp.MustCompile(`\(char_u \*\)&(\w+)`)
	z12PV     = regexp.MustCompile(`PV_\w+`)
	z12Call   = regexp.MustCompile(`(?m)^[ \t]*change_warning\([^;]*\);\n`)
	z12Glob   = regexp.MustCompile(`&(p_[a-z0-9_]+)\b`)
)

var z12Before = map[string]int{
	"change_warning": 7, "did_set_readonly": 3,
	"b_p_ro": 10, "b_p_fs": 7, "b_did_warn": 4,
	"p_ro": 2, "p_fs": 2, "p_ur": 2, "p_write": 2, "p_wa": 2, "p_prompt": 2,
	"p_mod": 2, "did_set_modified": 3,
	"SHM_RO": 2, "BV_RO": 3, "BV_FS": 4, "w_readonly": 2,
	"p_paste": 12, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 6,
}

var z12After = map[string]int{
	"b_p_ro": 0, "b_p_fs": 0, "change_warning": 0, "did_set_readonly": 0,
	"p_ro": 1, "p_fs": 1, "p_ur": 1, "p_write": 1, "p_wa": 1, "p_prompt": 1,
	"b_did_warn": 1, "p_mod": 2, "did_set_modified": 3, "p_paste": 12,
	"read_cmd_fd": 12, "vim_fsync": 3, "scriptin": 8, "redir_fd": 6,
}

// z12Want is the set of rows with no reader of their own global, and the six
// this phase drops are a CHOSEN SUBSET of it: 'modified' stays, because the
// state it reports lives in b_changed and not in p_mod.
var z12Want = map[string]string{
	"fsync": "PV_BOTH", "modified": "PV_BUF", "prompt": "PV_NONE",
	"readonly": "PV_BUF", "undoreload": "PV_NONE", "write": "PV_NONE",
	"writeany": "PV_NONE",
}

var z12Nopaste = []string{"p_ai_nopaste", "p_et_nopaste", "p_sts_nopaste",
	"p_tw_nopaste", "p_wm_nopaste"}

func init() {
	for _, n := range z12Nopaste {
		z12Before[n] = 4
		z12After[n] = 4
	}
}

// Whim95 removes the options nothing reads.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noopts", W: w}
	var err error

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}
	textEdit := func(t []byte, old, new, what string, n int) ([]byte, error) {
		k := strings.Count(string(t), old)
		if k != n {
			return nil, p.Die("%s -- the text occurs %d times, expected %d: %s",
				what, k, n, cutil.PyRepr(edit.ZHead(old, 70)))
		}
		p.Say(what)
		return []byte(strings.ReplaceAll(string(t), old, new)), nil
	}

	// ---- 0. the shape every anchor below was counted against ------------------
	for _, name := range edit.SortedKeys(z12Before) {
		if k := mentions(text, name); k != z12Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, z12Before[name])
		}
	}
	p.Say("change_warning 7 (a definition and six calls, and no prototype), " +
		"did_set_readonly 3, b_p_ro 10, b_p_fs 7 -- the file the five edits of part C " +
		"were counted against")

	// ---- 1. WHICH ROWS HAVE NO READER, COMPUTED -------------------------------
	// dropoptions --strict's own test, run here over EVERY row, so that the six
	// this phase drops are a chosen subset of a computed set and not a list.
	t := string(text)
	i := strings.Index(t, "static struct vimoption options[]")
	if i < 0 {
		return nil, p.Die("options[] is not in this file")
	}
	j := strings.Index(t[i:], "\n};") + i
	b := cutil.Blank(text)
	type row struct {
		Name       string
		start, end int
	}
	var rows []row
	for _, m := range z12OptRow.FindAllStringSubmatchIndex(t[i:j], -1) {
		start := i + m[0]
		end := cutil.Match(b, strings.Index(t[start:], "{")+start)
		if end < 0 {
			return nil, p.Die("the options[] row for %s is not balanced", cutil.PyRepr(t[i+m[2]:i+m[3]]))
		}
		rows = append(rows, row{t[i+m[2] : i+m[3]], start, end})
	}
	if len(rows) != 114 {
		return nil, p.Die("options[] has %d rows, expected 114 -- the table has moved under this "+
			"phase", len(rows))
	}
	got := map[string]string{}
	for _, r := range rows {
		vm := z12Var.FindStringSubmatch(t[r.start:r.end])
		if vm == nil {
			continue // a row with no global of its own
		}
		v := vm[1]
		selfDecl := regexp.MustCompile(`^static\b[^=]*\b` + regexp.QuoteMeta(v) + `;$`)
		amp := regexp.MustCompile(`&\s*` + regexp.QuoteMeta(v) + `\b`)
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(v) + `\b`)
		read := false
		for _, h := range word.FindAllStringIndex(t, -1) {
			o := h[0]
			if r.start <= o && o <= r.end {
				continue
			}
			line := t[strings.LastIndex(t[:o], "\n")+1 : strings.Index(t[o:], "\n")+o]
			if z12IsRow(line) || z12IsAmp(line) || selfDecl.MatchString(line) {
				continue
			}
			if !word.MatchString(amp.ReplaceAllString(line, "")) {
				continue
			}
			read = true
			break
		}
		if !read {
			pv := "PV_NONE"
			if m := z12PV.FindString(t[r.start:r.end]); m != "" {
				pv = m
			}
			got[r.Name] = pv
		}
	}
	if !z12SameMap(got, z12Want) {
		return nil, p.Die("the rows with no reader are %s, expected exactly %s -- the six this phase "+
			"drops are a chosen subset of that computed set, so a different set means "+
			"the choice was made against a different file", z12Fmt(got), z12Fmt(z12Want))
	}
	p.Say("seven of the 114 rows have no reader of their own global, computed with " +
		"dropoptions --strict's own test: fsync modified prompt readonly undoreload " +
		"write writeany")
	p.Say("and 'modified' is the one that STAYS -- GOALS.md II decision 5: the state it " +
		"reports lives in b_changed and not in p_mod, so the row is not a lie.  A " +
		"computation that took \"no reader\" as the criterion would delete it")

	// THE EXEMPTION, ASSERTED RATHER THAN ONLY WRITTEN DOWN.
	if _, a := got["paste"]; a {
		return nil, p.Die("'paste' came out of the computation with no reader, and it is EXEMPT FOR " +
			"EVER (GOALS.md II.2d): nothing in this pipeline may drop it")
	}
	if _, a := z12Want["paste"]; a {
		return nil, p.Die("'paste' came out of the computation with no reader, and it is EXEMPT FOR " +
			"EVER (GOALS.md II.2d): nothing in this pipeline may drop it")
	}
	p.Say("'paste' is exempt for ever and is not in the set: p_paste 12 mentions, and its " +
		"five save slots p_ai_nopaste p_et_nopaste p_sts_nopaste p_tw_nopaste " +
		"p_wm_nopaste are the non-pointer orphans orphanopts.py reports and tolerates, " +
		"here and afterwards, identically")

	// ---- C1. the W10 warning --------------------------------------------------
	if k := len(z12Call.FindAllString(t, -1)); k != 6 {
		return nil, p.Die("change_warning has %d call sites, expected 6", k)
	}
	text = z12Call.ReplaceAll([]byte(t), nil)
	var removed bool
	if text, removed = cutil.DeleteDefinition(text, "change_warning"); !removed {
		return nil, p.Die("change_warning has no definition to remove")
	}
	if k := mentions(text, "change_warning"); k != 0 {
		return nil, p.Die("change_warning still has %d mentions; it has no prototype, so six calls "+
			"and a definition is all of it", k)
	}
	p.Say("the W10 warning: six calls to change_warning() and the definition, which takes " +
		"the static string w_readonly and the ui_delay(1002L, TRUE) with it -- GOALS.md " +
		"phase 85 named that as one of the eight other pauses.  It has NO prototype, so a " +
		"program that removed one would fail here")

	// ---- C2. the [RO] in fileinfo() -------------------------------------------
	for _, e := range []struct{ Old, New, What string }{
		{`"\"%s%s%s%s%s%s", curbufIsChanged()`, `"\"%s%s%s%s%s", curbufIsChanged()`,
			"fileinfo's CTRL-G line loses one %s, and the argument below goes with " +
				"it in the same step -- nothing in the build checks the count"},
		{`curbuf->b_p_ro ? (shortmess(SHM_RO) ? _("[RO]") : _("[readonly]")) : "", `, "",
			"the [RO]/[readonly] argument itself, which is SHM_RO's only reader"},
		{` || curbuf->b_p_ro) ? " " : "");`, `) ? " " : "");`,
			"and the trailing-space test's `|| curbuf->b_p_ro` disjunct"},
		// ---- C3. the [RO] on the status line
		{` || wp->w_buffer->b_p_ro) && plen <  PATH_MAX  - 1)`, `) && plen <  PATH_MAX  - 1)`,
			"win_redr_status: the name-padding test's `|| b_p_ro` disjunct"},
		{z12lit2, "", "and the block that appended [RO] to it -- nothing else reaches that " +
			"indicator"},
		// ---- C4. did_set_readonly, by name and with the reason
		{z12lit3, "", "did_set_readonly's prototype"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}
	if text, removed = cutil.DeleteDefinition(text, "did_set_readonly"); !removed {
		return nil, p.Die("did_set_readonly has no definition to remove")
	}
	if k := mentions(text, "did_set_readonly"); k != 1 {
		return nil, p.Die("did_set_readonly has %d mentions, expected 1 -- the row that names it as "+
			"its callback, which part C5 removes", k)
	}
	p.Say("did_set_readonly, BY NAME: it is 'readonly''s callback and the sweep would take " +
		"it, but droplocal.py runs in this same edit and would find it still reading " +
		"b_p_ro -- measured, that refusal is the tool working")
	if k := mentions(text, "b_p_ro"); k != 3 {
		return nil, p.Die("b_p_ro has %d mentions, expected 3 -- the field, buf_copy_options' write, "+
			"and get_varp's case", k)
	}
	p.Say("b_p_ro 10 -> 3, and the three that are left are plumbing: the field, " +
		"buf_copy_options' write and get_varp's case.  droplocal.py is what takes those")
	return text, nil
}

// Whim95Rows is the second heredoc: what the sweep is handed, as a count rather
// than as trust, taken AFTER the four droplocal/dropoptions calls between them.
func Whim95Rows(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noopts", W: w}
	mentions := func(name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(text, -1))
	}
	for _, name := range edit.SortedKeys(z12After) {
		if k := mentions(name); k != z12After[name] {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", name, k, z12After[name])
		}
	}
	t := string(text)
	i := strings.Index(t, "static struct vimoption options[]")
	j := strings.Index(t[i:], "\n};") + i
	var rows []string
	for _, m := range z12OptRow.FindAllStringSubmatch(t[i:j], -1) {
		rows = append(rows, m[1])
	}
	globals := map[string]bool{}
	for _, m := range z12Glob.FindAllStringSubmatch(t[i:j], -1) {
		globals[m[1]] = true
	}
	if len(rows) != 108 || len(globals) != 96 {
		return nil, p.Die("options[] has %d rows and %d distinct globals, expected 108 and 96",
			len(rows), len(globals))
	}
	for _, gone := range []string{"fsync", "prompt", "readonly", "undoreload", "write", "writeany"} {
		if edit.Contains(rows, gone) {
			return nil, p.Die("the row for '%s' is still there", gone)
		}
	}
	if !edit.Contains(rows, "modified") || !edit.Contains(rows, "paste") {
		return nil, p.Die("'modified' or 'paste' lost its row, and neither may")
	}
	p.Say("the cut is done: options[] 114 -> 108 rows and 102 -> 96 distinct " +
		"globals; 'modified' and 'paste' keep theirs, b_did_warn is a field nothing " +
		"names and the six globals are declared and unread -- all of it the sweep's " +
		"now")
	return text, nil
}

var (
	z12RowHead = regexp.MustCompile(`^[ \t]*\{"`)
	z12AmpHead = regexp.MustCompile(`^[ \t]*\(char_u \*\)&`)
)

func z12IsRow(line string) bool { return z12RowHead.MatchString(line) }
func z12IsAmp(line string) bool { return z12AmpHead.MatchString(line) }

func z12SameMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// z12Fmt is Python's `' '.join('%s(%s)' % kv for kv in sorted(d.items()))`.
func z12Fmt(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	Out := make([]string, len(keys))
	for i, k := range keys {
		Out[i] = fmt.Sprintf("%s(%s)", k, m[k])
	}
	return strings.Join(Out, " ")
}
