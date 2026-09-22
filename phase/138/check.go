package p138

// Whim phase 138, the check -- no parameter carries an eval value.
// See phase/138/edit.go, and GOALS.md.
//
// phase/138/check.go proves each parameter was only tested or handed on
// and always nullptr, requires every eval value type gone, and probes g CTRL-G,
// a \= substitution and :match against controls.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim138", Check) }

// w138Param is one parameter the phase takes Out: the function, the name, and
// every shape a line mentioning it may have on the input.  A line of another
// shape is a use this phase did not account for.
type w138Param struct {
	fn, Name string
	shapes   []string
}

var w138Params = []w138Param{
	{"vim_regsub_both", "expr", []string{
		`^vim_regsub_both\(`,
		`^if \(\(source == nullptr && expr == nullptr\) \|\| dest == nullptr\)$`,
		`^if \(expr != nullptr \|\| \(source\[0\] == '\\\\' && source\[1\] == '='\)\)$`}},
	{"match_add", "pos_list", []string{`^match_add\(`}},
	{"cursor_pos_info", "dict", []string{`^cursor_pos_info\(dict_T \*dict\)$`, `^if \(dict == nullptr\)$`}},
	{"vim_vsnprintf_typval", "tvs", []string{
		`^vim_vsnprintf_typval\(`,
		`^if \(parse_fmt_types\(&ap_types, &num_posarg, fmt, tvs\) == FAIL\)$`,
		`get_unsigned_int\([^)]*, tvs != nullptr\) == FAIL\)$`,
		`^if \(tvs != nullptr\)$`,
		`^if \(tvs != nullptr && tvs\[`}},
	{"parse_fmt_types", "tvs", []string{`^parse_fmt_types\(`, `get_unsigned_int\([^)]*, tvs != nullptr\) == FAIL\)$`}},
	{"find_ex_command", "cctx", []string{`^find_ex_command\(`}},
	{"find_ex_command", "lookup", []string{`^find_ex_command\(`}},
}

// the types the phase exists to free, and every one must be gone
var w138Types = []string{"typval_T", "list_T", "dict_T", "listitem_T", "dictitem_T", "dictitem16_T",
	"listwatch_T", "type_T", "class_T", "itf2class_T", "ocmember_T", "vartype_T", "cfunc_T", "cfunc_free_T", "cctx_T", "ufunc_T"}

// Whim138 is phase 138's check: no parameter carries an eval value.
//
//  1. THE PARTITION, on the input: every line of each function that mentions
//     the parameter is its head, a test against nullptr, or its being handed
//     on as it came -- so no function did anything with one but ask whether
//     it was there; and every call of each function passes nullptr for it.
//  2. THE CUT: no parameter is left, and every type of the eval layer's values
//     has no mention: the sweep took the cluster.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE, for the paths the parameters were tested on: g CTRL-G (its
//     message), a \= substitution (the expression branch, empty) and :match;
//     each writes the same bytes on both binaries, and each CONTROL moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim138", "evalparm")
	if err != nil {
		return err
	}
	r := c.R
	old := []byte(c.Old)
	blank := cutil.Blank(old)
	for _, p := range w138Params {
		a, z, ok := cutil.FindDefinition(old, blank, p.fn)
		if !ok {
			r.Bad("%s is not defined on the input", p.fn)
			continue
		}
		for _, l := range check.LinesWith(c.Old[a:z], p.Name) {
			hit := false
			for _, s := range p.shapes {
				if regexp.MustCompile(s).MatchString(l) {
					hit = true
					break
				}
			}
			if !hit {
				r.Bad("%s's %s is used otherwise: %s", p.fn, p.Name, l)
			}
		}
	}
	calls := []string{
		"vim_regsub_both(source, nullptr, dest, destlen, flags)",
		"match_add(curwin, g, p + 1, 10, id, nullptr, nullptr);",
		"cursor_pos_info(nullptr);",
		"vim_vsnprintf_typval(str, str_m, fmt, ap, nullptr)",
		"find_ex_command(&ea, nullptr, nullptr, nullptr)",
	}
	for _, call := range calls {
		name := call[:strings.Index(call, "(")]
		// on the text with its strings blanked: vim_regsub_both() names itself
		// in its error messages
		if n := len(regexp.MustCompile(`\b`+name+`\(`).FindAll(blank, -1)); n != 3 && n != 2 {
			r.Bad("%s is named %d times on the input: its prototype if it has one, its definition and one call are what this phase was written against", name, n)
		}
		if strings.Count(c.Old, call) != 1 {
			r.Bad("%s's one call is not %q", name, call)
		}
	}
	for _, fn := range []string{"vim_regsub_both", "cursor_pos_info", "vim_vsnprintf_typval", "parse_fmt_types", "match_add", "find_ex_command"} {
		for _, p := range w138Params {
			if p.fn == fn && inFn(c.New, fn, p.Name) {
				r.Bad("%s still mentions %s", fn, p.Name)
			}
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input each parameter is only tested against nullptr or handed on, and each function's one call passes nullptr: none carried anything")

	var left []string
	for _, t := range w138Types {
		if k := check.Word(c.New, t); k != 0 {
			left = append(left, t)
		}
	}
	if len(left) != 0 {
		r.Bad("the eval types still named: %v", left)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the parameters are gone, and so is every one of the %d eval value types -- typval_T, lists, dicts, type_T, class_T and their items (the file lost %d lines)",
		len(w138Types), check.CountLines([]byte(c.Old))-check.CountLines([]byte(c.New)))
	if err := c.Gate(true); err != nil {
		return err
	}

	ob, nb := c.Bins()
	seed := []byte("ione two\rthree two\x1bgg")
	probes := [][2]string{
		{"$g\x07", "0g\x07"},
		{":s/one/\\=1/\r", ":s/one/X/\r"},
		{":match Search /two/\r", ":match Search /three/\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr[0]), []byte(":q!\r")}
		so, _, e1 := check.Stream(ob, keys, nil)
		sn, _, e2 := check.Stream(nb, keys, nil)
		sc, _, e3 := check.Stream(nb, [][]byte{seed, []byte(pr[1]), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if so != sn {
			r.Bad("%q is written differently on the two binaries", pr[0])
		}
		if sc == sn {
			r.Bad("the CONTROL for %q did not move", pr[0])
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: g CTRL-G, a \\= substitution and :match write the same bytes on both binaries; each CONTROL moves")
	return nil
}

// inFn: name is a word in fn's definition.
func inFn(text, fn, name string) bool {
	b := []byte(text)
	a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), fn)
	return ok && check.Word(text[a:z], name) != 0
}
