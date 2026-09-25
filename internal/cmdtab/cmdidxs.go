package cmdtab

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// CmdIdxs is tools/create_cmdidxs.py: regenerate the ex_cmdidxs.h block and
// require it byte-identical.
//
// Upstream generates this with create_cmdidxs.vim, which needs a vim with
// +eval, and this build has none -- so the generator lives here.  Its value is
// as a canary: it parses the command table and reproduces a checked-in block
// exactly, so a pass that quietly reshapes the table, or merely re-indents a
// line of it, shows up as a diff rather than as a wrong answer months later.
//
// The row floor lives in CommandNames (NameFloor) and is not repeated here:
// a table parsed too thin never reaches this file.

const (
	cmdIdxsFirst = "static const unsigned short cmdidxs1[26] ="
	cmdIdxsLast  = "static const int command_count = "
)

const letters = "abcdefghijklmnopqrstuvwxyz"

// GenerateCmdIdxs writes the two tables and the count, in the exact shape the
// checked-in block has.
//
// THE TREE CARRIES NO COMMENTS, so neither does what is written into it: both
// tables are indexed by letter, a first and z last, and the banner and the
// per-letter labels this generator used to emit went with every other comment.
// If they ever come back, re-validate against the checked-in file rather than
// against this function.
//
// A letter no command begins with gets 0 in both tables, which is a real index
// and not a sentinel -- the lookup consults cmdidxs2 only after cmdidxs1 has
// put it somewhere, so the pair is never read for a letter that has no
// commands.
func GenerateCmdIdxs(names []string) string {
	idx1 := make([]int, 26)
	for a := 0; a < 26; a++ {
		c := letters[a]
		for i, n := range names {
			if len(n) > 0 && n[0] == c {
				idx1[a] = i
				break
			}
		}
	}

	idx2 := make([][]int, 26)
	for a := 0; a < 26; a++ {
		idx2[a] = make([]int, 26)
		for b := 0; b < 26; b++ {
			pre := string([]byte{letters[a], letters[b]})
			idx2[a][b] = 0
			for i, n := range names {
				if strings.HasPrefix(n, pre) {
					idx2[a][b] = i - idx1[a]
					break
				}
			}
		}
	}

	// THE SHAPE IS THE PRINTER'S.  The block sits in a canonically printed
	// file, so it is written the way crefactor/cemit writes a table: a blank
	// line under the banner and between the three declarations, the brace on
	// its own line under the `=`, one element per line, a trailing comma on
	// every element including the last, and a row on one line with no padding.
	var o bytes.Buffer
	o.WriteString("\nstatic const unsigned short cmdidxs1[26] =\n{\n")
	for i := 0; i < 26; i++ {
		fmt.Fprintf(&o, "    %d,\n", idx1[i])
	}
	o.WriteString("};\n\nstatic const unsigned char cmdidxs2[26][26] =\n{\n")
	for i := 0; i < 26; i++ {
		var row []string
		for j := 0; j < 26; j++ {
			row = append(row, fmt.Sprintf("%d", idx2[i][j]))
		}
		fmt.Fprintf(&o, "    {%s},\n", strings.Join(row, ", "))
	}
	o.WriteString("};\n\n")
	fmt.Fprintf(&o, "static const int command_count = %d;\n\n", len(names))
	return o.String()
}

// cmdIdxsBlock returns the generated block as it currently stands in the file,
// and the line indices of its first and last lines: from the cmdidxs1 table to
// command_count.  It was delimited by two comment banners; the canonical form
// has no comments, so it is found by its own code.
func cmdIdxsBlock(path string) (block string, lines []string, i, j int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, 0, 0, err
	}
	lines = strings.Split(string(data), "\n")
	i, j = -1, -1
	for n, l := range lines {
		switch {
		case l == cmdIdxsFirst && i < 0:
			i = n
		case strings.HasPrefix(l, cmdIdxsLast) && i >= 0 && j < 0:
			j = n
		}
	}
	if i < 0 || j < 0 {
		return "", nil, 0, 0, fmt.Errorf("%s: no ex_cmdidxs block (cmdidxs1 to command_count)", path)
	}
	return strings.Join(lines[i:j+1], "\n") + "\n", lines, i, j, nil
}

// CheckCmdIdxs requires the block in the file to be what the table generates.
func CheckCmdIdxs(path string) error {
	names, err := CommandNames(path)
	if err != nil {
		return err
	}
	got, _, _, _, err := cmdIdxsBlock(path)
	if err != nil {
		return err
	}
	if want := GenerateCmdIdxs(names); strings.Trim(want, "\n") != strings.Trim(got, "\n") {
		return fmt.Errorf("%s: the generated table does not match the source", path)
	}
	return nil
}

// UpdateCmdIdxs rewrites the block in place.
func UpdateCmdIdxs(path string) error {
	names, err := CommandNames(path)
	if err != nil {
		return err
	}
	_, lines, i, j, err := cmdIdxsBlock(path)
	if err != nil {
		return err
	}
	// The generated text ends in a newline, so splitting it leaves a trailing
	// empty field that is not a line; drop it, exactly as the Python's
	// `.split('\n')[:-1]` does.
	gen := strings.Split(strings.Trim(GenerateCmdIdxs(names), "\n"), "\n")

	out := append([]string{}, lines[:i]...)
	out = append(out, gen...)
	out = append(out, lines[j+1:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}
