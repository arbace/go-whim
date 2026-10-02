package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// keepThen is an `if (T) { A } else { B }` whose condition is always true:
// keep A, lose B.
//
// FoldAlways refuses a block with an else, rightly -- it cannot tell whether
// the else is meant.  Here it is meant, so this does the one shape by the same
// brace matching FoldNever uses.
func (e ed) keepThen(seg []byte, pattern, what string) ([]byte, error) {
	re := regexp.MustCompile("(?m)" + pattern)
	ms := re.FindAllIndex(seg, -1)
	if len(ms) != 1 {
		return nil, fmt.Errorf("%s: %s -- the condition occurs %d times, expected 1",
			e.tool, what, len(ms))
	}
	b := edit.Blank(seg)
	k, o, c, head, err := edit.Guarded(seg, b, ms[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %s -- %v", e.tool, what, err)
	}
	if head != "if" {
		return nil, fmt.Errorf("%s: %s -- not a plain if", e.tool, what)
	}
	end := c + bytes.IndexByte(seg[c:], '\n') + 1
	rest := seg[end:]
	nxt := regexp.MustCompile(`^[ \t]*else\b`).FindIndex(rest)
	if nxt == nil || regexp.MustCompile(`^[ \t]*else[ \t]+if\b`).Match(rest) {
		return nil, fmt.Errorf("%s: %s -- expected a plain else after the block", e.tool, what)
	}
	at := end + nxt[1]
	o2 := at + bytes.IndexByte(b[at:], '{')
	c2 := edit.Match(b, o2)
	if c2 < 0 {
		return nil, fmt.Errorf("%s: %s -- the else block is unbalanced", e.tool, what)
	}
	body := seg[o+bytes.IndexByte(seg[o:], '\n')+1 : bytes.LastIndexByte(seg[:c], '\n')+1]
	e.say(what)
	out := make([]byte, 0, len(seg))
	out = append(out, seg[:k]...)
	out = append(out, body...)
	return append(out, seg[c2+bytes.IndexByte(seg[c2:], '\n')+1:]...), nil
}

// lfonlyDying are the format functions and the option callbacks whose rows
// lfonly drops (record 50's cut).  Every call left must sit inside one of them.
var lfonlyDying = []string{
	"get_fileformat", "get_fileformat_force", "set_fileformat", "default_fileformat",
	"file_ff_differs", "save_file_ff", "set_file_options", "set_options_bin",
	"msg_add_fileformat", "check_ff_value", "did_set_binary", "did_set_fileformat",
	"did_set_fileformats", "did_set_textmode", "did_set_textauto",
	"did_set_eof_eol_fixeol_bomb",
}

var lfonlyHeads = regexp.MustCompile(`(?m)^(\w+)\([^;\n]*\)[ \t]*\n\{`)

// LfOnly makes every line end with LF, read and written.
func LfOnly(text []byte, w io.Writer) ([]byte, error) {
	e := ed{"lfonly", w}
	var err error

	// readfile went with open_buffer's read arms, and its other callers, by
	// phase 1 (readfront, phase 31's move): every format it chose on reading
	// went with it.

	text, err = e.inFunction(text, "buf_write", func(s []byte) ([]byte, error) {
		var err error
		if s, err = e.subOnce(s,
			`^[ \t]*if \(eap != nullptr && eap->force_bin != 0\)\n`+
				`[ \t]*\{\n`+
				`[ \t]*write_bin = \(eap->force_bin == FORCE_BIN\);\n`+
				`[ \t]*\}\n[ \t]*else\n[ \t]*\{\n[ \t]*write_bin = buf->b_p_bin;\n[ \t]*\}\n`,
			"buf_write choosing 'binary' or ++bin"); err != nil {
			return nil, err
		}
		if s, err = e.literal(s, "        fileformat = get_fileformat_force(buf, eap);\n", "",
			"buf_write choosing a format", 1); err != nil {
			return nil, err
		}
		if s, err = e.foldNever(s, `^[ \t]*else if \(c == CAR && fileformat == EOL_MAC\)$`,
			"buf_write writing CR as a line end"); err != nil {
			return nil, err
		}
		if s, err = e.literal(s,
			"if (end == 0 || (lnum == end && (write_bin || !buf->b_p_fixeol) && ((write_bin && lnum == buf->b_no_eol_lnum) || (lnum == buf->b_ml.ml_line_count && !buf->b_p_eol))))",
			"if (end == 0)", "buf_write leaving the last LF off", 1); err != nil {
			return nil, err
		}
		if s, err = e.keepThen(s, `^[ \t]*if \(fileformat == EOL_UNIX\)$`,
			"buf_write writing CR LF or CR"); err != nil {
			return nil, err
		}
		if s, err = e.foldNever(s, `^[ \t]*if \(!buf->b_p_fixeol && buf->b_p_eof\)$`,
			"buf_write appending CTRL-Z"); err != nil {
			return nil, err
		}
		return e.dropIf(s, `^[ \t]*if \(msg_add_fileformat\(fileformat\)\)$`,
			`the "[dos]" and "[mac]" write messages`)
	})
	if err != nil {
		return nil, err
	}

	// open_buffer's fifo and stdin arms, and their 'binary', went at phase 1
	// (readfront).
	text, err = e.inFunction(text, "open_buffer", func(s []byte) ([]byte, error) {
		return e.literal(s, "    save_file_ff(curbuf);\n", "",
			"open_buffer saving the format", 1)
	})
	if err != nil {
		return nil, err
	}

	if text, err = e.inFunction(text, "do_ecmd", func(s []byte) ([]byte, error) {
		return e.subOnce(s, `^[ \t]*set_file_options\(TRUE, eap\);\n`,
			"do_ecmd setting ++ff and ++bin")
	}); err != nil {
		return nil, err
	}

	// 'endofline' and 'endoffile' had no initialiser in buf_copy_options():
	// their only resets were the ones removed above.  droplocal wants an
	// initialiser to recognise the shape, so their get_varp() case goes here,
	// and the sweep takes the fields no one names after it.
	if text, err = e.inFunction(text, "get_varp", func(s []byte) ([]byte, error) {
		return e.subCount(s,
			`^[ \t]*case \(idopt_T\)\(PV_BUF \+ \(int\)\(BV_EO[LF]\)\):\n`+
				`[ \t]*return \(char_u \*\)&\(curbuf->b_p_eo[lf]\);\n`,
			"get_varp handing out 'endofline' and 'endoffile'", 2)
	}); err != nil {
		return nil, err
	}
	// one (readfile's) went at phase 1 (readfront)
	if text, err = e.subCount(text, `^[ \t]*curbuf->b_no_eol_lnum = 0;\n`,
		"resetting the no-LF line for 'binary'", 1); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "set_init_1", func(s []byte) ([]byte, error) {
		return e.literal(s, "    save_file_ff(curbuf);\n", "",
			"startup saving the format of the first buffer", 1)
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "did_set_modified", func(s []byte) ([]byte, error) {
		return e.dropIf(s, `^[ \t]*if \(!args->os_newval\.boolean\)$`,
			"'nomodified' saving the format")
	}); err != nil {
		return nil, err
	}

	if text, err = e.inFunction(text, "cursor_pos_info", func(s []byte) ([]byte, error) {
		var err error
		for _, f := range []struct{ pat, what string }{
			{`^[ \t]*if \(get_fileformat\(curbuf\) == EOL_DOS\)$`,
				"g CTRL-G counting CR LF as two bytes"},
			{`^[ \t]*if \(lnum == curbuf->b_ml\.ml_line_count && !curbuf->b_p_eol && \(curbuf->b_p_bin \|\| !curbuf->b_p_fixeol\) && [^\n]*\)$`,
				"g CTRL-G at a last line with no LF"},
			{`^[ \t]*if \(!curbuf->b_p_eol && \(curbuf->b_p_bin \|\| !curbuf->b_p_fixeol\)\)$`,
				"g CTRL-G counting a missing last LF"},
		} {
			if s, err = e.foldNever(s, f.pat, f.what); err != nil {
				return nil, err
			}
		}
		return s, nil
	}); err != nil {
		return nil, err
	}

	if text, err = e.inFunction(text, "unchanged", func(s []byte) ([]byte, error) {
		s, err := e.literal(s, "buf->b_changed || (ff && file_ff_differs(buf, FALSE))",
			"buf->b_changed", "a changed format counting as a change", 1)
		if err != nil {
			return nil, err
		}
		return e.dropIf(s, `^[ \t]*if \(ff\)$`, "unchanged saving the format")
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "bufIsChangedNotTerm", func(s []byte) ([]byte, error) {
		return e.literal(s, "(buf->b_changed || file_ff_differs(buf, TRUE))",
			"(buf->b_changed)", "a changed format counting as changed", 1)
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "buf_clear_file", func(s []byte) ([]byte, error) {
		return e.subCount(s, `^[ \t]*buf->b_(?:p|start)_eo[fl] = (?:FALSE|TRUE);\n`,
			"buf_clear_file resetting 'endofline' and 'endoffile'", 4)
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "transchar_nonprint", func(s []byte) ([]byte, error) {
		return e.foldNever(s,
			`^[ \t]*else if \(buf != nullptr && c == CAR && get_fileformat\(buf\) == EOL_MAC\)$`,
			"CR shown as a line end")
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "do_ascii", func(s []byte) ([]byte, error) {
		return e.foldNever(s, `^[ \t]*if \(c == CAR && get_fileformat\(curbuf\) == EOL_MAC\)$`,
			"ga showing CR as a line end")
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "ml_open", func(s []byte) ([]byte, error) {
		return e.subOnce(s,
			`^[ \t]*b0p->b0_fname\[B0_FNAME_SIZE_ORG - 2\] = get_fileformat\(buf\) \+ 1;\n`,
			"block 0 recording the format")
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "ml_setflags", func(s []byte) ([]byte, error) {
		return e.subOnce(s,
			`^[ \t]*b0p->b0_fname\[B0_FNAME_SIZE_ORG - 2\] = \(b0p->b0_fname\[B0_FNAME_SIZE_ORG - 2\] & ~B0_FF_MASK\) \| \(get_fileformat\(buf\) \+ 1\);\n`,
			"block 0 updating the format")
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "set_init_3", func(s []byte) ([]byte, error) {
		return e.dropIf(s,
			`^[ \t]*if \(\(curbuf->b_ml\.ml_line_count == 1 && \*ml_get\(\(linenr_T\)1\) == NUL\)\)$`,
			"startup applying 'fileformats' to an empty buffer")
	}); err != nil {
		return nil, err
	}

	if text, err = e.inFunction(text, "getargopt", func(s []byte) ([]byte, error) {
		var err error
		if s, err = e.dropIf(s,
			`^[ \t]*if \(strncmp\(\(char \*\)\(arg\), \(char \*\)\("bin"\), \(3\)\) == 0 \|\| strncmp\(\(char \*\)\(arg\), \(char \*\)\("nobin"\), \(5\)\) == 0\)$`,
			"++bin and ++nobin"); err != nil {
			return nil, err
		}
		for _, f := range []struct{ pat, what string }{
			{`^[ \t]*if \(strncmp\(\(char \*\)\(arg\), \(char \*\)\("ff"\), \(2\)\) == 0\)$`, "++ff"},
			{`^[ \t]*if \(strncmp\(\(char \*\)\(arg\), \(char \*\)\("fileformat"\), \(10\)\) == 0\)$`,
				"++fileformat"},
			{`^[ \t]*if \(pp == &eap->force_ff\)$`, "++ff checking its value"},
		} {
			if s, err = e.foldNever(s, f.pat, f.what); err != nil {
				return nil, err
			}
		}
		return s, nil
	}); err != nil {
		return nil, err
	}
	if text, err = e.inFunction(text, "prepare_help_buffer", func(s []byte) ([]byte, error) {
		return e.literal(s, "    curbuf->b_p_bin = FALSE;\n", "",
			"the help buffer clearing 'binary'", 1)
	}); err != nil {
		return nil, err
	}

	if text, err = e.inFunction(text, "buf_copy_options", func(s []byte) ([]byte, error) {
		a := bytes.Index(s, []byte("switch (*p_ffs)"))
		if a < 0 {
			return nil, fmt.Errorf("lfonly: buf_copy_options has no 'fileformats' switch")
		}
		a = bytes.LastIndexByte(s[:a], '\n') + 1
		b := edit.Blank(s)
		o := a + bytes.IndexByte(b[a:], '{')
		c := edit.Match(b, o)
		if c < 0 {
			return nil, fmt.Errorf("lfonly: buf_copy_options' switch is unbalanced")
		}
		out := make([]byte, 0, len(s))
		out = append(out, s[:a]...)
		s = append(out, s[c+bytes.IndexByte(s[c:], '\n')+1:]...)
		e.say("a new buffer's 'fileformat' from 'fileformats'")
		var err error
		if s, err = e.dropIf(s, `^[ \t]*if \(buf->b_p_ff != nullptr\)$`,
			"a new buffer's remembered format"); err != nil {
			return nil, err
		}
		return e.subCount(s, `^[ \t]*buf->b_p_(?:tw|wm|et)_nobin = p_(?:tw|wm|et)_nobin;\n`,
			"a new buffer's values saved for 'binary'", 3)
	}); err != nil {
		return nil, err
	}

	// Every call left must be inside code the sweep takes with them.  Counted
	// by WHERE each call sits, not by a tally that has to be guessed.
	blanked := edit.Blank(text)
	var spans [][2]int
	for _, n := range lfonlyDying {
		if a, z, ok := edit.FindDefinition(text, blanked, n); ok {
			spans = append(spans, [2]int{a, z})
		}
	}
	type head struct {
		at   int
		name string
	}
	var heads []head
	for _, m := range lfonlyHeads.FindAllSubmatchIndex(text, -1) {
		heads = append(heads, head{m[0], string(text[m[2]:m[3]])})
	}
	var live []string
	for _, n := range lfonlyDying {
		for _, m := range edit.CallsNotAfterWord(text, n) {
			ls := bytes.LastIndexByte(text[:m[0]], '\n') + 1
			le := bytes.IndexByte(text[m[0]:], '\n')
			var line []byte
			if le < 0 {
				line = text[ls:]
			} else {
				line = text[ls : m[0]+le]
			}
			if bytes.HasPrefix(line, []byte("static ")) ||
				bytes.HasPrefix(line, []byte(n+"(")) {
				continue
			}
			inDying := false
			for _, sp := range spans {
				if sp[0] <= m[0] && m[0] < sp[1] {
					inDying = true
					break
				}
			}
			if inDying {
				continue
			}
			owner := "?"
			for _, h := range heads {
				if h.at <= m[0] {
					owner = h.name
				}
			}
			live = append(live, n+" in "+owner)
		}
	}
	if len(live) > 0 {
		return nil, fmt.Errorf("lfonly: still called from live code: %s", strings.Join(live, ", "))
	}

	e.say("every line ends with LF, read and written")
	return text, nil
}
