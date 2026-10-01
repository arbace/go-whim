package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
)

// The parser's half -- -t, -t's argument, -i -- is the command line's own,
// cut with it (argvfront, the reform's D1).

// nocmdoptsElsewhere are the fields the four options set, their now-unreachable
// readers, and restricted mode.
//
// SCOPED BY THEIR NEIGHBOUR: `char_u *tagname;` is also a field of taggy_T,
// seventeen hundred lines earlier, and an unanchored pattern takes the first -- which
// removed the tag stack's field and broke five lines in two functions this
// phase never meant to touch.  So the field is the one whose struct closes,
// with no brace between, as `} mparm_T;` -- whatever its neighbours are, which
// moved as the reform brought this cut to the seed (phase 1, D6).
var nocmdoptsElsewhere = []struct {
	what, pat, repl string
	want            int
}{
	{"the startup tag jump, which nothing can now ask for",
		`(?m)[ \t]*if \(params\.tagname != NULL\)\n[ \t]*\{\n(?:[^\n]*\n)*?` +
			`[ \t]*do_cmdline_cmd\(IObuff\);\n(?:[^\n]*\n)*?^[ \t]{4}\}\n`, "", 1},
	{"exe_pre_commands testing for one",
		`(?m)[ \t]*if \(parmp->tagname == NULL && curwin->w_cursor\.lnum <= 1\)\n` +
			`([ \t]*\{\n[ \t]*curwin->w_cursor\.lnum = 0;\n[ \t]*\}\n)`,
		"    if (curwin->w_cursor.lnum <= 1)\n${1}", 1},
	{"mparm_T's tagname field",
		`(?m)^[ \t]*char_u[ \t]*\*tagname;\n((?:[ \t]+[^\n}]*\n)*\} mparm_T;)`, "${1}", 1},
	// The flag itself was on twenty-four rows, every one deleted at phase 1
	// (extable, the reform's D2b): with the gate gone it is named by nothing.
	// The other way in, and a small find of its own: set_init_restricted_mode()
	// reads $SHELL at startup and turns the mode on when it is nologin or
	// false.  An environment read, deciding a mode that now restricts nothing.
	{"$SHELL deciding restricted mode at startup",
		edit.Line("set_init_restricted_mode();"), "", 1},
	// do_bang's restricted check went with :! and the commands that named a
	// file, at phase 1 (exfront and filefront, the reform's D2 and D4)
	{"ex_stop's restricted check",
		`(?m)[ \t]*if \(check_restricted\(\)\)\n[ \t]*\{\n[ \t]*return;\n[ \t]*\}\n`, "", 1},
	{"the EX_RESTRICT gate, which no live command reaches",
		`(?m)[ \t]*if \(restricted != 0 && \(ea\.argt & EX_RESTRICT\)\)\n` +
			`[ \t]*\{\n(?:[^\n]*\n)*?[ \t]*\}\n`, "", 1},
	{"restricted deciding whether SIGTSTP is ignored",
		`(?m)ignore_sigtstp = restricted \|\| SIG_IGN`, "ignore_sigtstp = SIG_IGN", 1},
}

// NoCmdOpts removes -t, -i, -y and -Z, the fields they set, and restricted
// mode.
func NoCmdOpts(text []byte, w io.Writer) ([]byte, error) {
	for _, e := range nocmdoptsElsewhere {
		re := regexp.MustCompile(e.pat)
		n := len(re.FindAll(text, -1))
		if e.want > 1 {
			if n != e.want {
				return nil, fmt.Errorf("nocmdopts: %s -- expected %d, matched %d",
					e.what, e.want, n)
			}
			text = re.ReplaceAll(text, []byte(e.repl))
		} else {
			if n < 1 {
				return nil, fmt.Errorf("nocmdopts: %s -- expected 1, matched 0", e.what)
			}
			text, _ = replaceFirst(re, text, e.repl)
		}
		fmt.Fprintf(w, "  nocmdopts    %s\n", e.what)
	}
	return text, nil
}
