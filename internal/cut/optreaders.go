package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
)

// optreadersLiteral: each must occur exactly once.
var optreadersLiteral = []struct{ what, old, new string }{
	// -nb's early scan and --clean's pre-scan of argv went with the command
	// line, and what --clean, --not-a-term and -n set is never written after
	// it: the fall-out closure folds its readers (argvfront, the reform's D1)
	// -p's test of the window layout went with the window layouts at phase 1
	// (nowindows, the reform's D9)
	{"-h: the pointer to it at the end of every usage error",
		`    fprintf(stderr, "%s", (_("\nMore info with: \"vim -h\"\n")));` + "\n", ""},
}

var optreadersFolds = []struct {
	what, kind, pattern string
	count               int
}{
	// -p's tab pages, and 'shortmess' restored after filling them, went with
	// the window layouts at phase 3 (nowindows, the reform's D9)
}

// optreadersAfter is what must be LEFT, counted after every edit.
var optreadersAfter = []struct {
	what, pattern string
	want          int
}{
	{"early_arg_scan outside its definition and prototype", `\bearly_arg_scan\(paramp\)`, 0},
}

// OptReaders removes what read the options a previous phase dropped.
func OptReaders(text []byte, w io.Writer) ([]byte, error) {
	for _, l := range optreadersLiteral {
		n := bytes.Count(text, []byte(l.old))
		if n != 1 {
			return nil, fmt.Errorf("optreaders: %s -- occurs %d times, not once", l.what, n)
		}
		text = bytes.ReplaceAll(text, []byte(l.old), []byte(l.new))
		fmt.Fprintf(w, "  optreaders   %s\n", l.what)
	}

	for _, f := range optreadersFolds {
		var err error
		if f.kind == "always" {
			text, err = edit.FoldAlways(text, "(?m)"+f.pattern, f.count)
		} else {
			text, err = edit.FoldNever(text, "(?m)"+f.pattern, f.count)
		}
		if err != nil {
			return nil, fmt.Errorf("optreaders: %s -- %v", f.what, err)
		}
		places := ""
		if f.count > 1 {
			places = fmt.Sprintf(", %d places", f.count)
		}
		fmt.Fprintf(w, "  optreaders   %s%s\n", f.what, places)
	}

	for _, a := range optreadersAfter {
		n := len(regexp.MustCompile(a.pattern).FindAll(text, -1))
		if n != a.want {
			return nil, fmt.Errorf("optreaders: %s -- %d left, expected %d", a.what, n, a.want)
		}
	}

	fmt.Fprintln(w, "  optreaders   nothing reads what a dropped option set")
	return text, nil
}
