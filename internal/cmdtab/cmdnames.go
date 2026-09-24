// Package cmdtab reads the Ex command table out of the source: its names
// (cmdnames[]), and the first-two-letters index derived from them
// (the ex_cmdidxs.h block), which a step regenerates and checks.
package cmdtab

import (
	"fmt"
	"os"
	"regexp"
)

// NameFloor is 80, and it was 100.  A regex that stops matching after an edit
// to the table's shape yields an EMPTY list, and from it a plausible-looking
// all-zero index -- so the floor is a deliberate number rather than a guard
// against zero, and it is the same number and the same argument as
// orphanopts' so the two stay one idea.
//
// It was lowered when phase 91 took the table from 111 rows to 98, in
// that phase's own commit and never silently.  The table has stayed at 98
// since, which leaves 18 rows of margin.
const NameFloor = 80

var (
	excmdRow    = regexp.MustCompile(`(?m)^EXCMD\(\s*CMD_\w+\s*,\s*"((?:[^"\\]|\\.)*)"`)
	cmdnamesRow = regexp.MustCompile(`\[CMD_\w+\] = \{\(char_u \*\)"((?:[^"\\]|\\.)*)"`)
)

// CommandNames returns the Ex command names in table order, whichever shape
// the table is in.
//
// It has been an X-macro of EXCMD rows and a written-out cmdnames[] with
// designated initialisers.  Both are tried and whichever parses is taken;
// asking by file name stopped working when there was only one file.
//
// The EXCMD rows appear TWICE in the merged file, once per reading of what was
// ex_cmds.h -- the header is included with EXCMD meaning two different things
// either side of an #undef -- so the first run of them is taken.
func CommandNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CommandNamesIn(data, path)
}

// CommandNamesIn is CommandNames over text already in hand, for a phase edit
// that holds the tree and not a path.  `path` is only what the refusal names.
func CommandNamesIn(data []byte, path string) ([]string, error) {
	for _, try := range []struct {
		re     *regexp.Regexp
		halves bool
	}{
		{cmdnamesRow, false},
		{excmdRow, true},
	} {
		var got []string
		for _, m := range try.re.FindAllSubmatch(data, -1) {
			got = append(got, string(m[1]))
		}
		if len(got) < NameFloor {
			continue
		}
		if try.halves && len(got) > 600 {
			got = got[:len(got)/2]
		}
		return got, nil
	}
	// The failure is not the one the name suggests: both parsers are tried
	// without their own check, so a table just below the floor comes back as
	// "no command table found in either shape" rather than as a count.
	return nil, fmt.Errorf("%s: no command table found in either shape", path)
}
