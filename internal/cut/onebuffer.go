package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// OneBuffer leaves one buffer: the old one is wiped, nothing is hidden,
// nothing is the alternate.
func OneBuffer(text []byte, w io.Writer) ([]byte, error) {
	e := ed{"onebuffer", w}
	var err error

	text, err = e.inFunction(text, "parse_command_modifiers", func(s []byte) ([]byte, error) {
		s, err := e.subOnce(s,
			`^[ \t]*case 'h':\n[ \t]*if \(p != eap->cmd \|\| !checkforcmd_noparen\(&p, "hide", 3\) \|\| \*p == NUL \|\| ends_excmd\(\*p\)\)\n`+
				`[ \t]*\{\n[ \t]*break;\n[ \t]*\}\n[ \t]*eap->cmd = p;\n`+
				`[ \t]*cmod->cmod_flags \|= CMOD_HIDE;\n[ \t]*continue;\n`,
			"the :hide modifier")
		if err != nil {
			return nil, err
		}
		return e.dropIf(s, `^[ \t]*if \(checkforcmd_noparen\(&eap->cmd, "keepalt", 5\)\)$`,
			"the :keepalt modifier")
	})
	if err != nil {
		return nil, err
	}

	text, err = e.inFunction(text, "set_context_by_cmdname", func(s []byte) ([]byte, error) {
		return e.subCount(s, `^[ \t]*case CMD_(?:hide|keepalt):\n`,
			"completion for :hide and :keepalt", 2)
	})
	if err != nil {
		return nil, err
	}

	// do_argfile, :next, can_abandon, alist_add and alist_add_list died with
	// the argument-list and buffer commands, retired at phase 1 (exfront, the
	// reform's D2)

	// set_curbuf, which hid the buffer it left, went with ex_quit's refusal
	// at phase 1 (quitfront, the reform's move of phase 33)

	text, err = e.inFunction(text, "getfile", func(s []byte) ([]byte, error) {
		s, err := e.literal(s, " && !buf_hide(curbuf) && ", " && ",
			"getfile writing before it leaves", 1)
		if err != nil {
			return nil, err
		}
		return e.literal(s, "(buf_hide(curbuf) ? ECMD_HIDE : 0) + ", "",
			"getfile hiding the buffer", 1)
	})
	if err != nil {
		return nil, err
	}

	// :quit's refusal, and with it its buf_hide test, folded at phase 1
	// (quitfront, phase 33's move)
	text, err = e.inFunction(text, "ex_quit", func(s []byte) ([]byte, error) {
		return e.literal(s, "win_close(wp, !buf_hide(wp->w_buffer) || eap->forceit)",
			"win_close(wp, TRUE)", ":quit freeing the buffer", 1)
	})
	if err != nil {
		return nil, err
	}

	// ex_exit, do_exedit, rename_buffer, ex_read and do_write went with :exit,
	// :edit, :file, :read and :write at phase 1 (filefront, the reform's D4)

	text, err = e.inFunction(text, "nv_gotofile", func(s []byte) ([]byte, error) {
		s, err := e.literal(s, " && !buf_hide(curbuf))", ")", "gf refusing a changed buffer", 1)
		if err != nil {
			return nil, err
		}
		return e.literal(s, "buf_hide(curbuf) ? ECMD_HIDE : 0", "0", "gf hiding the buffer", 1)
	})
	if err != nil {
		return nil, err
	}
	if n := len(edit.CallsNotAfterWord(text, "buf_hide")); n != 2 {
		return nil, fmt.Errorf("onebuffer: buf_hide is still called -- %d mentions, expected "+
			"its prototype and definition", n)
	}

	text, err = e.inFunction(text, "close_buffer", func(s []byte) ([]byte, error) {
		var err error
		for _, b := range []struct{ ch, what string }{
			{"d", "delete"}, {"w", "wipe"}, {"u", "unload"},
		} {
			if s, err = e.foldNever(s, `^[ \t]*if \(buf->b_p_bh\[0\] == '`+b.ch+`'\)$`,
				"close_buffer's bufhidden="+b.what); err != nil {
				return nil, err
			}
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	text, err = e.inFunction(text, "do_ecmd", func(s []byte) ([]byte, error) {
		s, err := e.literal(s, "(flags & ECMD_HIDE) ? 0 : DOBUF_UNLOAD", "DOBUF_WIPE",
			"do_ecmd wiping the buffer it leaves", 1)
		if err != nil {
			return nil, err
		}
		if s, err = e.dropIf(s, `^[ \t]*if \(fnum == 0 && other_file && ffname != nullptr\)$`,
			"do_ecmd naming a refused file the alternate"); err != nil {
			return nil, err
		}
		if s, err = e.dropIf(s, `^[ \t]*if \(\(cmdmod\.cmod_flags & CMOD_KEEPALT\) == 0\)$`,
			"do_ecmd making the old buffer the alternate"); err != nil {
			return nil, err
		}
		if s, err = e.subOnce(s,
			`^[ \t]*if \(oldwin != nullptr\)\n[ \t]*\{\n[ \t]*buflist_altfpos\(oldwin\);\n[ \t]*\}\n`,
			"do_ecmd saving the old window position"); err != nil {
			return nil, err
		}
		return e.dropIf(s,
			`^[ \t]*if \(curwin->w_alt_fnum == buf->b_fnum && prev_alt_fnum != 0\)$`,
			"do_ecmd restoring the alternate")
	})
	if err != nil {
		return nil, err
	}

	// set_curbuf went with ex_quit's refusal at phase 1 (quitfront)

	text, err = e.inFunction(text, "win_init", func(s []byte) ([]byte, error) {
		return e.literal(s, "    newp->w_alt_fnum = oldp->w_alt_fnum;\n", "",
			"win_init copying the alternate", 1)
	})
	if err != nil {
		return nil, err
	}
	text, err = e.inFunction(text, "buflist_findnr", func(s []byte) ([]byte, error) {
		return e.dropIf(s, `^[ \t]*if \(nr == 0\)$`, "buffer 0 meaning the alternate")
	})
	if err != nil {
		return nil, err
	}
	text, err = e.inFunction(text, "buflist_findpat", func(s []byte) ([]byte, error) {
		return e.literal(s, "match = curwin->w_alt_fnum;", "match = 0;",
			"'#' finding the alternate", 1)
	})
	if err != nil {
		return nil, err
	}

	// A ROW IS NEVER DELETED FROM nv_cmds[], IT IS POINTED AT nv_error.
	if text, err = e.subCountRepl(text,
		`(?m)^([ \t]*\{Ctrl_HAT, )nv_hat(, NV_NCW, 0\},)$`, "${1}nv_error${2}",
		"CTRL-^'s row points at nv_error", 1); err != nil {
		return nil, err
	}

	// setaltfname() and buf_hide() have no caller now and still name the
	// alternate and the two modifier flags; the sweep takes them, and record 42's
	// program counted again after it.  Everything else is counted here.
	blanked := edit.Blank(text)
	var dying [][2]int
	for _, n := range []string{"setaltfname", "buf_hide"} {
		if a, z, ok := edit.FindDefinition(text, blanked, n); ok {
			dying = append(dying, [2]int{a, z})
		}
	}
	count := func(pattern string) int {
		n := 0
		for _, m := range edit.AllIndex(regexp.MustCompile(pattern), text) {
			in := false
			for _, sp := range dying {
				if sp[0] <= m[0] && m[0] < sp[1] {
					in = true
					break
				}
			}
			if !in {
				n++
			}
		}
		return n
	}
	var left []string
	for _, c := range oneBufferLeft {
		if n := count(c.pattern); n != c.want {
			left = append(left, fmt.Sprintf("(%s, %d)", edit.PyRepr(c.what), n))
		}
	}
	if len(left) > 0 {
		return nil, fmt.Errorf("onebuffer: still present: [%s]", strings.Join(left, ", "))
	}

	e.say("one buffer: the old one is wiped, nothing is hidden, nothing is the alternate")
	return text, nil
}

// oneBufferLeft is what OneBuffer counts outside the two dying functions,
// and how many of each it requires.
var oneBufferLeft = []struct {
	what, pattern string
	want          int
}{
	{"w_alt_fnum outside its field", `\bw_alt_fnum\b`, 2},
	{"CMOD_KEEPALT or CMOD_HIDE outside their enumerators", `\bCMOD_(?:KEEPALT|HIDE)\b`, 2},
	{"buflist_altfpos called", `\bbuflist_altfpos\(curwin\)|\bbuflist_altfpos\(oldwin\)`, 0},
	{"nv_hat in the key table", `\{Ctrl_HAT, nv_hat`, 0},
}
