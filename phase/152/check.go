package p152

// Whim phase 152, the check -- the option variables are typed.
// See phase/152/edit.go, and GOALS.md.
//
// phase/152/check.go computes every row's typed variable from the
// input's, requires no punning left, and probes options of every kind and scope.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim152", Check) }

// w152Table is options[]'s own text, head to matching brace: the span the
// punning patterns that are about an OPTION VARIABLE are read in.
func w152Table(text string) string {
	const head = "static struct vimoption options[] =\n{\n"
	i := strings.Index(text, head)
	if i < 0 {
		return ""
	}
	end := cutil.Match(cutil.Blank([]byte(text)), i+len(head)-2)
	if end < 0 {
		return ""
	}
	return text[i : end+1]
}

// Whim152 is phase 152's check: the option variables are typed.
//
//  1. EVERY ROW: the input row's `(char_u *)&X` is the output row's X in the
//     slot of its kind -- ov_int for P_BOOL, ov_long for P_NUM, ov_str for
//     P_STRING -- NULL is no variable, (char_u *)-1 is the window flag; row
//     for row, computed from the input.
//  2. NO PUNNING IS LEFT: no `(char_u *)&` of an option variable, no
//     `(char_u *)-1`, no cast of varp or of an option's var to a typed pointer,
//     and no `+ sizeof(winopt_T)`; the type and helpers are W152Type.
//  3. THE GATE, the libc surface unchanged: every typed constructor's argument
//     and every read's type is checked by the compiler, silently.
//  4. THE PROBES: a boolean, a number and a string option set and read back;
//     window-local, buffer-local and global-local options through :setlocal and
//     :setglobal -- the paths get_varp_scope() and get_varp_allbuf() take --
//     a terminal option, and :set all.  Each the same on both binaries, each
//     CONTROL moving.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim152", "optvar")
	if err != nil {
		return err
	}
	r := c.R
	in, Out := check.W152Vars(c.Old), check.W152Vars(c.New)
	if len(in) == 0 || len(in) != len(Out) {
		r.Bad("options[] has %d rows on the input and %d on the output", len(in), len(Out))
		return r.Done()
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
		if strings.ReplaceAll(Out[k][1], " ", "") != strings.ReplaceAll(want, " ", "") {
			r.Bad("row %d (%s): %q where the input's %q gives %q", k, fl, Out[k][1], in[k][1], want)
			break
		}
	}
	for _, re := range []string{`\(char_u \*\)&\s*\(?\s*(p_|curwin|curbuf|term_strings)`, `\((int|long) \*\)\s*\(?\s*varp`, `\(char_u \*\*\)\s*\(?\s*varp\b`, `sizeof\(winopt_T\)`, `\(char_u \*\*\)\s*\(?\s*(p|opp)->var`, `options\[[^\]]+\]\.var\s*==\s*\(char_u`} {
		if m := regexp.MustCompile(re).FindString(c.New); m != "" {
			r.Bad("an option variable is still punned: %q", m)
		}
	}
	// `(char_u *)-1` IS A PUN INSIDE options[] AND A SENTINEL OUTSIDE IT, so it
	// is read inside the table and not over the file.  The regex size pass
	// writes `regcode == ((char_u *)-1)` fourteen times and it is phase 156's,
	// not this one's; the residue spelled that one `(char_u *) -1`, with the
	// space macro expansion left, so a file-wide pattern told the two apart by
	// accident.  The canonical text spells both the same way, and the table is
	// where the claim was always about.
	tbl := w152Table(c.New)
	if tbl == "" {
		r.Bad("options[] is not in the output in the shape this check reads it")
	} else if m := regexp.MustCompile(`\(char_u \*\)\s*-1`).FindString(tbl); m != "" {
		r.Bad("an option variable is still punned: %q", m)
	}
	if !strings.Contains(c.New, W152Type) {
		r.Bad("optvar_T and its helpers are not the ones this phase writes")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("each of the %d rows of options[] holds its variable in the slot of its kind, computed from the input; no option variable is punned through char_u * any more", len(in))
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	more := strings.Repeat(" ", 40)
	probes := []struct{ What, Keys, ctl string }{
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
		keys := [][]byte{[]byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, [][]byte{[]byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.Bad("%s is written differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: a boolean, a number, a string, window-local, buffer-local and global-local options through :set, :setlocal and :setglobal, a terminal option and :set all are drawn the same by both binaries; each CONTROL moves")
	return nil
}
