package cut

import (
	"bytes"
	"fmt"
	"github.com/arbace/go-whim/internal/cutil"
	"io"
	"regexp"
)

const fnameModArm = `        else if (!skip_mod)
        {
            valid |= modify_fname(src, tilde_file, usedlen, &result, &resultbuf, &resultlen);
            if (result == NULL)
            {
                *errormsg = "";
                return NULL;
            }
        }
`

// fnameModFeeders are the assignments to the locals that existed only to be
// passed to modify_fname() or to suppress it.  The sweep takes a local nothing
// names but not a store to one, so the stores go here and the declarations,
// like modify_fname itself, are the sweep's.
var fnameModFeeders = []struct {
	pat, what string
	n         int
}{
	{cutil.Line(`tilde_file = strcmp((char *)(result), (char *)("~")) == 0;`),
		"a tilde_file assignment", 2},
	{cutil.Line("skip_mod = TRUE;"), "skip_mod's assignment", 1},
}

var modifyFname = regexp.MustCompile(`\bmodify_fname\b`)

// NoFnameMod makes % a file name and nothing more.
//
// eval_vars' modifier arm goes as exact text, then the stores to the two locals
// that only fed it.
func NoFnameMod(text []byte, w io.Writer) ([]byte, error) {
	if !bytes.Contains(text, []byte(fnameModArm)) {
		return nil, fmt.Errorf("nofnamemod: eval_vars' modifier arm is not where this expects")
	}
	text = bytes.Replace(text, []byte(fnameModArm), nil, 1)
	fmt.Fprintln(w, "  nofnamemod   %% is a file name and nothing more")

	for _, f := range fnameModFeeders {
		re := regexp.MustCompile(f.pat)
		locs := re.FindAllIndex(text, f.n)
		if len(locs) != f.n {
			return nil, fmt.Errorf("nofnamemod: %s -- expected %d, matched %d",
				f.what, f.n, len(locs))
		}
		var buf []byte
		prev := 0
		for _, l := range locs {
			buf = append(buf, text[prev:l[0]]...)
			prev = l[1]
		}
		text = append(buf, text[prev:]...)
	}
	fmt.Fprintln(w, "  nofnamemod   tilde_file and skip_mod, which only fed it")

	fmt.Fprintf(w, "  nofnamemod   %d modify_fname mentions left for the sweep\n",
		len(modifyFname.FindAll(text, -1)))
	return text, nil
}
