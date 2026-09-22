package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim152", Whim152) }

// w152Vars is each row of options[] as its flags and its variable field, in
// order -- the fourth top-level field of the row.
func w152Vars(text string) [][2]string {
	head := "static struct vimoption options[] =\n{\n"
	i := strings.Index(text, head)
	if i < 0 {
		return nil
	}
	b := cutil.Blank([]byte(text))
	end := cutil.Match(b, i+len(head)-2)
	var out [][2]string
	for k := i + len(head) - 1; k < end; k++ {
		if b[k] != '{' {
			continue
		}
		re := cutil.Match(b, k)
		depth, start := 0, k+1
		var fs []string
		for j := k + 1; j < re; j++ {
			switch b[j] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
			case ',':
				if depth == 0 {
					fs = append(fs, strings.Join(strings.Fields(text[start:j]), " "))
					start = j + 1
				}
			}
		}
		if len(fs) >= 4 {
			out = append(out, [2]string{fs[2], fs[3]})
		}
		k = re
	}
	return out
}

// Whim152 is phase 152's check: the option variables are typed.
//
//  1. EVERY ROW: the input row's `(char_u *)&X` is the output row's X in the
//     slot of its kind -- ov_int for P_BOOL, ov_long for P_NUM, ov_str for
//     P_STRING -- NULL is no variable, (char_u *)-1 is the window flag; row
//     for row, computed from the input.
//  2. NO PUNNING IS LEFT: no `(char_u *)&` of an option variable, no
//     `(char_u *)-1`, no cast of varp or of an option's var to a typed pointer,
//     and no `+ sizeof(winopt_T)`; the type and helpers are edit.W152Type.
//  3. THE GATE, the libc surface unchanged: every typed constructor's argument
//     and every read's type is checked by the compiler, silently.
//  4. THE PROBES: a boolean, a number and a string option set and read back;
//     window-local, buffer-local and global-local options through :setlocal and
//     :setglobal -- the paths get_varp_scope() and get_varp_allbuf() take --
//     a terminal option, and :set all.  Each the same on both binaries, each
//     CONTROL moving.
func Whim152(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim152", "optvar")
	if err != nil {
		return err
	}
	r := c.r
	in, out := w152Vars(c.old), w152Vars(c.new)
	if len(in) == 0 || len(in) != len(out) {
		r.bad("options[] has %d rows on the input and %d on the output", len(in), len(out))
		return r.done()
	}
	for k := range in {
		fl, v := in[k][0], strings.ReplaceAll(in[k][1], " ", "")
		var want string
		switch {
		case v == "nullptr" || v == "(char_u*)nullptr":
			want = "{nullptr, nullptr, nullptr, 0}"
		case v == "((char_u*)-1)" || v == "(char_u*)((char_u*)-1)":
			want = "{nullptr, nullptr, nullptr, 1}"
		default:
			addr := "&" + strings.TrimPrefix(v, "(char_u*)&")
			switch {
			case strings.Contains(fl, "P_BOOL"):
				want = "{" + addr + ", nullptr, nullptr, 0}"
			case strings.Contains(fl, "P_NUM"):
				want = "{nullptr, " + addr + ", nullptr, 0}"
			default:
				want = "{nullptr, nullptr, " + addr + ", 0}"
			}
		}
		if strings.ReplaceAll(out[k][1], " ", "") != strings.ReplaceAll(want, " ", "") {
			r.bad("row %d (%s): %q where the input's %q gives %q", k, fl, out[k][1], in[k][1], want)
			break
		}
	}
	for _, re := range []string{`\(char_u \*\)&\s*\(?\s*(p_|curwin|curbuf|term_strings)`, `\(char_u \*\)-1`, `\((int|long) \*\)\s*\(?\s*varp`, `\(char_u \*\*\)\s*\(?\s*varp\b`, `sizeof\(winopt_T\)`, `\(char_u \*\*\)\s*\(?\s*(p|opp)->var`, `options\[[^\]]+\]\.var\s*==\s*\(char_u`} {
		if m := regexp.MustCompile(re).FindString(c.new); m != "" {
			r.bad("an option variable is still punned: %q", m)
		}
	}
	if !strings.Contains(c.new, edit.W152Type) {
		r.bad("optvar_T and its helpers are not the ones this phase writes")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("each of the %d rows of options[] holds its variable in the slot of its kind, computed from the input; no option variable is punned through char_u * any more", len(in))
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	more := strings.Repeat(" ", 40)
	probes := []struct{ what, keys, ctl string }{
		{"a boolean", ":set ai\r:set ai?\r:set invai\r:set ai?\r", ":set noai\r:set ai?\r:set invai\r:set ai?\r"},
		{"a number", ":set sw=3 ts=5\r:set sw? ts?\r", ":set sw=4 ts=5\r:set sw? ts?\r"},
		{"a string", ":set bs=indent\r:set bs+=eol\r:set bs?\r", ":set bs=start\r:set bs+=eol\r:set bs?\r"},
		{"a window-local option", ":set list\r:setlocal nolist\r:setglobal list?\r:setlocal list?\r", ":set nolist\r:setlocal nolist\r:setglobal list?\r:setlocal list?\r"},
		{"a window-local number", ":set nu\r:setglobal nonu\r:set nu?\r:setglobal nu?\r", ":set nonu\r:setglobal nonu\r:set nu?\r:setglobal nu?\r"},
		{"a global-local window option", ":set so=3\r:setlocal so=5\r:set so?\r:setglobal so?\r:setlocal so?\r", ":set so=2\r:setlocal so=5\r:set so?\r:setglobal so?\r:setlocal so?\r"},
		{"a global-local window string", ":set lcs=tab:>-\r:setlocal lcs=eol:$\r:setglobal lcs?\r:setlocal lcs?\r", ":set lcs=tab:>.\r:setlocal lcs=eol:$\r:setglobal lcs?\r:setlocal lcs?\r"},
		{"a buffer-local option", ":setlocal sw=2\r:setglobal sw?\r:setlocal sw?\r", ":setlocal sw=6\r:setglobal sw?\r:setlocal sw?\r"},
		{"a global-local buffer number", ":set ul=5\r:setlocal ul=7\r:setglobal ul?\r:setlocal ul?\r", ":set ul=6\r:setlocal ul=7\r:setglobal ul?\r:setlocal ul?\r"},
		{"a terminal option", ":set t_ZH=x\r:set t_ZH?\r", ":set t_ZH=y\r:set t_ZH?\r"},
		{"every option", ":set all\r" + more, ":set ts=3\r:set all\r" + more},
	}
	for _, pr := range probes {
		keys := [][]byte{[]byte(pr.keys), []byte(":q!\r")}
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, [][]byte{[]byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: a boolean, a number, a string, window-local, buffer-local and global-local options through :set, :setlocal and :setglobal, a terminal option and :set all are drawn the same by both binaries; each CONTROL moves")
	return nil
}
