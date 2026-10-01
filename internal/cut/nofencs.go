package cut

import (
	"fmt"
	"io"
	"regexp"
)

var nofencsEdits = []struct{ what, pat, repl string }{
	{"the reset that restored a unicode 'fileencodings'",
		`(?m)[ \t]*else if \(\(char_u \*\*\)varp == &p_fencs && enc_utf8\)\n` +
			`[ \t]*\{\n[ \t]*newval = fencs_utf8_default;\n[ \t]*\}\n`, ""},
	// readfile's choice between an empty list and a list went with readfile,
	// which dies at phase 13 since ml_recover went at phase 1 (norecover, the
	// reform's D5)
}

// NoFencs leaves p_fencs and p_tenc as a declaration and a row, so the rows
// can go next.
func NoFencs(text []byte, w io.Writer) ([]byte, error) {
	for _, e := range nofencsEdits {
		re := regexp.MustCompile(e.pat)
		n := len(re.FindAll(text, -1))
		if n != 1 {
			return nil, fmt.Errorf("nofencs: %s -- expected 1, matched %d", e.what, n)
		}
		text = re.ReplaceAll(text, []byte(e.repl))
		fmt.Fprintf(w, "  nofencs      %s\n", e.what)
	}

	// did_set_encoding's conversion between 'termencoding' and 'encoding'
	// went with the two rows, dropped at phase 1 (optfront, the reform's D3).

	for _, v := range []string{"p_fencs", "p_tenc"} {
		// The declaration and the options[] row are not reads.
		n := len(regexp.MustCompile(`\b`+v+`\b`).FindAll(text, -1))
		if n > 2 {
			return nil, fmt.Errorf("nofencs: %s still has %d mentions; the row cannot go "+
				"while anything reads it", v, n)
		}
	}

	fmt.Fprintln(w, "  nofencs      p_fencs and p_tenc are now declaration and row only")
	return text, nil
}
