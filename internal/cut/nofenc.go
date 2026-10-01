package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
)

var nofencStubs = []struct{ name, body string }{
	{"bomb_size", "    return 0;"},
	{"add_b0_fenc", ""},
}

// nofencEdits: a nil pattern means the edit is brace-matched below, because
// the block holds inner blocks and a lazy line scan would stop at the first
// one's closing brace.
var nofencEdits = []struct {
	what, pat, repl string
	want            int
}{
	{"buf_write taking the buffer's 'fileencoding' as its target",
		`(?m)([ \t]*else\n[ \t]*\{\n)[ \t]*fenc = buf->b_p_fenc;\n`,
		"${1}        fenc = (char_u *)\"\";\n", 1},
	// readfile's 'fileencoding', its 'bomb' set and cleared and its BOM test went
	// with readfile, which dies at phase 13 since ml_recover went at phase 1
	// (norecover, the reform's D5); so did the swap file's block-zero
	// encoding and its restore.
	{"save_file_ff remembering the BOM and the encoding",
		`(?m)[ \t]*buf->b_start_bomb = buf->b_p_bomb;\n` +
			`[ \t]*if \(buf->b_start_fenc == NULL \|\| strcmp[^\n]*\n` +
			`[ \t]*\{\n` +
			`[ \t]*vim_free\(buf->b_start_fenc\);\n` +
			`[ \t]*buf->b_start_fenc = vim_strsave\(buf->b_p_fenc\);\n[ \t]*\}\n`, "", 1},
	{"file_ff_differs comparing them",
		`(?m)[ \t]*if \(!buf->b_p_bin && buf->b_start_bomb != buf->b_p_bomb\)\n` +
			`[ \t]*\{\n` +
			`[ \t]*return TRUE;\n` +
			`[ \t]*\}\n` +
			`[ \t]*if \(buf->b_start_fenc == NULL\)\n` +
			`[ \t]*\{\n` +
			`[ \t]*return \(\*buf->b_p_fenc != NUL\);\n[ \t]*\}\n[ \t]*return \(strcmp[^\n]*\n`, "    return FALSE;\n", 1},
	{"g8 converting to the buffer's 'fileencoding' to find an illegal byte",
		`(?m)[ \t]*if \(enc_utf8 && \(enc_canon_props\(curbuf->b_p_fenc\) & ENC_8BIT\)\)\n` +
			`[ \t]*\{\n[ \t]*convert_setup\(&vimconv, p_enc, curbuf->b_p_fenc\);\n[ \t]*\}\n`,
		"", 1},
	{"buf_write writing a BOM it no longer makes", "", "", 0},
	// did_set_encoding's arm for 'fileencoding', the empty test phase 15 left
	// there and gvarp went with the encoding rows, dropped at phase 1
	// (optfront, the reform's D3).
	{"freeing the remembered encoding",
		`(?m)^[ \t]* vim_free\(buf->b_start_fenc\);\n[ \t]* \(buf->b_start_fenc\) = NULL;\n`,
		"", 1},
	// three: at phase 1 (the reform's D7) readfile and the recovery are
	// still in the text, and two of them are theirs
	{"clearing the remembered BOM",
		`(?m)^[ \t]*(?:cur)?buf->b_start_bomb = FALSE;\n`, "", 3},
	// LOOKUPS BY NAME, and the reason this phase needed two attempts (three,
	// before readfile's and the recovered swap file's went with them).
	// set_string_option_direct((char_u *)"fenc", ...) resolves the option
	// through findoption(), which answers -1 for a row that is not there; the
	// caller does not check, so silent Ex mode exits 1 without printing
	// anything, and every recorded exit status in the harness moves at once.
	{"`:e ++enc=` forcing one",
		`(?m)[ \t]*char_u \*fenc = enc_canonize\(eap->cmd \+ eap->force_enc\);\n` +
			`[ \t]*if \(fenc != NULL\)\n` +
			`[ \t]*\{\n` +
			`[ \t]*set_string_option_direct\(\(char_u \*\)"fenc",[^\n]*\n` +
			`[ \t]*\}\n[ \t]*vim_free\(fenc\);\n`, "", 1},
}

// dropIfBlock removes an `if` and the block it guards, found by BRACE
// MATCHING from the condition's own parenthesis.
func dropIfBlock(text []byte, pat, what string) ([]byte, error) {
	blanked := edit.Blank(text)
	m := regexp.MustCompile(pat).FindIndex(text)
	if m == nil {
		return nil, fmt.Errorf("nofenc: %s is not where this expects", what)
	}
	lp := m[0] + bytes.IndexByte(text[m[0]:], '(')
	rp := edit.Match(blanked, lp)
	i := rp + 1
	for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n') {
		i++
	}
	closing := edit.Match(blanked, i)
	if closing < 0 {
		return nil, fmt.Errorf("nofenc: %s is unbalanced", what)
	}
	end := closing + 1
	for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
		end++
	}
	out := make([]byte, 0, len(text))
	out = append(out, text[:m[0]]...)
	return append(out, text[end:]...), nil
}

// NoFenc takes 'fileencoding' and 'bomb' away.
func NoFenc(text []byte, w io.Writer) ([]byte, error) {
	var err error
	for _, e := range nofencEdits {
		if e.pat == "" {
			// the one block found by brace matching
			text, err = dropIfBlock(text,
				`(?m)^[ \t]*if \(buf->b_p_bomb && !write_bin`,
				"buf_write no longer writes a BOM")
			if err != nil {
				return nil, fmt.Errorf("nofenc: buf_write no longer writes a BOM")
			}
		} else {
			re := regexp.MustCompile(e.pat)
			n := len(re.FindAll(text, -1))
			if n != e.want {
				return nil, fmt.Errorf("nofenc: %s -- expected %d, matched %d",
					e.what, e.want, n)
			}
			text = re.ReplaceAll(text, []byte(e.repl))
		}
		fmt.Fprintf(w, "  nofenc       %s\n", e.what)
	}

	for _, s := range nofencStubs {
		o, c, found, balanced := edit.Body(text, s.name)
		if !found || !balanced {
			return nil, fmt.Errorf("nofenc: %s is not defined at file scope any more", s.name)
		}
		var buf []byte
		buf = append(buf, text[:o]...)
		buf = append(buf, "{\n"...)
		if s.body != "" {
			buf = append(buf, s.body...)
			buf = append(buf, '\n')
		}
		buf = append(buf, '}')
		text = append(buf, text[c+1:]...)
		fmt.Fprintf(w, "  nofenc       %s answers for a file that has no BOM\n", s.name)
	}
	return text, nil
}
