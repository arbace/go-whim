package p153

// Whim phase 153, the check -- free_one_termoption() compares without a cast.
// See phase/153/edit.go, and GOALS.md.
//
// phase/153/check.go proves from the input that both-NULL is the whole
// comparison, requires the new one, and probes the terminal option path.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim153", Check) }

// Whim153 is phase 153's check: free_one_termoption() compares without a cast.
//
//  1. WHY BOTH-NULL IS THE WHOLE COMPARISON, on the input: every row of
//     options[] holds as ov_str either nullptr or `&X` of a p_* global or a
//     term_strings slot -- the address of a char_u * variable -- and
//     free_one_termoption() has one caller, which passes the value
//     term_strings[KS_CCO].  No code takes the address of a term_strings slot
//     or of a p_* string as a string, so an address and a value are equal only
//     when both are NULL.
//  2. THE CUT: the comparison is `p->var.ov_str == nullptr && var ==
//     nullptr`, and no pointer to a variable is cast to char_u * anywhere.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: ttest(), which calls it, is reached by setting a terminal
//     option and by t_Co; both draw the same on both binaries, each CONTROL
//     moving.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim153", "termopt")
	if err != nil {
		return err
	}
	r := c.R
	for _, v := range check.W152Vars(c.Old) {
		f := strings.ReplaceAll(v[1], " ", "")
		if f == "{nullptr,nullptr,nullptr,0}" || f == "{nullptr,nullptr,nullptr,1}" {
			continue
		}
		if !regexp.MustCompile(`^\{[^,]*,[^,]*,(nullptr|&\(?term_strings\[\(int\)\(KS_\w+\)\]\)?|&p_\w+),[01]\}$`).MatchString(f) && !regexp.MustCompile(`^\{&\w+,nullptr,nullptr,0\}$|^\{nullptr,&\w+,nullptr,0\}$`).MatchString(f) {
			r.Bad("a row's variable is not nullptr or the address of a variable: %s", v[1])
		}
	}
	if n := len(regexp.MustCompile(`\bfree_one_termoption\(`).FindAllString(c.Old, -1)); n != 2 || !strings.Contains(c.Old, "free_one_termoption( ( term_strings[(int)(KS_CCO)] ) );") {
		r.Bad("free_one_termoption() is not its definition and one call with term_strings[KS_CCO]")
	}
	if regexp.MustCompile(`\(char_u \*\*?\)\s*&\s*\(?\s*term_strings|=\s*\(char_u \*\)\s*&\s*\(?\s*term_strings`).MatchString(c.Old) {
		r.Bad("the address of a term_strings slot is taken as a string somewhere on the input")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input every option variable is NULL or the address of a variable, and the one caller passes a string value: the two are equal only when both are NULL")
	if !strings.Contains(c.New, "        if (p->var.ov_str == nullptr && var == nullptr)\n") || strings.Contains(c.New, "(char_u *)p->var.ov_str") {
		r.Bad("the comparison is not the one this phase writes")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("free_one_termoption() compares both-NULL, with no pointer cast to another pointer type")
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	probes := []struct{ What, Keys, ctl string }{
		{"a terminal option set", ":set t_AB=\r:set t_AF=\r:set t_Co?\r", ":set t_AB=\r:set t_Co=16\r:set t_Co?\r"},
		{"the colour count", ":set t_Co=256\r:set t_Co?\r", ":set t_Co=8\r:set t_Co?\r"},
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
	r.Say("PROBE: clearing terminal colour options and setting t_Co draw the same on both binaries; each CONTROL moves")
	return nil
}
