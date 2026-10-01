package cut

// optfront.go is the reform's D3 (doc/PIPELINE-REFORM.md §7): every options[]
// row the product has not, dropped on the seed in one cut, where 35 phases
// dropped them a few at a time.

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
)

//go:embed optfront.md
var optfrontList string

// optfrontNames is the fenced block of optfront.md: the rows to drop.
func optfrontNames() []string {
	parts := strings.Split(optfrontList, "```")
	if len(parts) < 3 {
		return nil
	}
	return strings.Fields(parts[1])
}

// OptFront drops every row optfront.md lists, and its modeline_whitelist[]
// entry.  No guard is asked: a global its row initialised is left at its
// static zero until the phase that removes its readers, which is what every
// drop without --strict did, and a buffer- or window-local option's field is
// its own phase's.
func OptFront(text []byte, w io.Writer) ([]byte, error) {
	names := optfrontNames()
	if len(names) == 0 {
		return nil, fmt.Errorf("optfront: optfront.md lists no rows")
	}
	// Each table on its own: a name like "arabic" also begins a row of the
	// encoding table, which DropOptions over the whole file would take first.
	region := func(t []byte, head string) (int, int, error) {
		a := bytes.Index(t, []byte(head))
		if a < 0 || bytes.Count(t, []byte(head)) != 1 {
			return 0, 0, fmt.Errorf("optfront: %q is not there once", head)
		}
		z := bytes.Index(t[a:], []byte("\n};\n"))
		if z < 0 {
			return 0, 0, fmt.Errorf("optfront: %q does not end", head)
		}
		return a, a + z + 4, nil
	}
	a, z, err := region(text, "static struct vimoption options[] =\n{\n")
	if err != nil {
		return nil, err
	}
	table := append([]byte(nil), text[a:z]...)
	for _, n := range names {
		var ok bool
		if table, ok = DropRow(table, n); !ok {
			return nil, fmt.Errorf("optfront: no options[] row for '%s'", n)
		}
	}
	text = append(append(append([]byte(nil), text[:a]...), table...), text[z:]...)
	// A name's line goes from every list, not only modeline_whitelist[]:
	// DropOptions has always removed `"name",` wherever it begins a line, and
	// so the value lists of other options lost the words they shared with a
	// dropped option ("key" from 'selectmode''s, "debug" from the history
	// names).  The product keeps that, so this does it the same way.
	whitelisted := 0
	for _, n := range names {
		spans := indentedLit(text, `"`+n+`",`+"\n")
		if len(spans) == 0 {
			continue
		}
		var buf []byte
		last := 0
		for _, sp := range spans {
			buf = append(buf, text[last:sp[0]]...)
			last = sp[1]
		}
		text = append(buf, text[last:]...)
		whitelisted += len(spans)
	}
	out := text
	fmt.Fprintf(w, "  optfront     %d options[] rows dropped, %d of their names out of the lists that held them\n", len(names), whitelisted)
	return out, nil
}
