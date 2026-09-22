package edit

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

func init() { register("whim155", Whim155) }

var w155Save = regexp.MustCompile(`(?m)^( *)([\w_]+) = vim_strnsave\((ml_get\w*)\(([^()\n]*)\), (ml_get\w*_len)\(([^()\n]*)\)\);\n`)

// W155Saves is how many vim_strnsave(line, its length) statements this phase
// sequences.
const W155Saves = 9

// Whim155 evaluates call arguments with effects in the order gcc does.
//
// C leaves the order of a call's arguments unspecified, Go evaluates them left
// to right, and gcc -- measured on this file's code, line by line in its
// disassembly -- evaluates them right to left.  Where two arguments both call
// a function with an effect outside its frame (internal/ccx's Order), the
// order is part of what the program does, and a translation that evaluated
// them left to right would not be this program.  Eleven calls are such: nine
// vim_strnsave(ml_get...(), ml_get..._len()), which gcc evaluates length
// first; col_print(..., ml_get_curline_len(), linetabsize_str(p)), which it
// evaluates linetabsize_str first; and fileinfo()'s message, whose
// new_file_message() it calls before the shortmess() of an earlier argument.
// Each first-evaluated argument becomes a local computed before the call, so
// the order is written, not implied (tx/FINDINGS.md).
func Whim155(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "argorder", w: w}
	s := string(text)
	n := 0
	var bad string
	s = w155Save.ReplaceAllStringFunc(s, func(m string) string {
		sm := w155Save.FindStringSubmatch(m)
		ind, lhs, get, args, getLen, args2 := sm[1], sm[2], sm[3], sm[4], sm[5], sm[6]
		if args != args2 || getLen != get+"_len" {
			bad = m
			return m
		}
		n++
		return fmt.Sprintf("%s{\n%s    colnr_T     len = %s(%s);\n\n%s    %s = vim_strnsave(%s(%s), len);\n%s}\n",
			ind, ind, getLen, args, ind, lhs, get, args, ind)
	})
	if bad != "" {
		return nil, p.die("a copy of a line whose length is not that line's: %q", strings.TrimSpace(bad))
	}
	if n != W155Saves {
		return nil, p.die("%d copies of a line with its length, and this phase was written against %d", n, W155Saves)
	}
	p.say(fmt.Sprintf("the %d copies of a line compute its length first, as gcc does", n))
	var err error
	if o, e := p.literal([]byte(s), "            col_print(buf2, sizeof(buf2), ml_get_curline_len(), linetabsize_str(p));\n",
		"            {\n                int         vcol = linetabsize_str(p);\n\n                col_print(buf2, sizeof(buf2), ml_get_curline_len(), vcol);\n            }\n",
		"cursor_pos_info() computes the line's width before its length, as gcc does", 1); e != nil {
		return nil, e
	} else {
		s = string(o)
	}
	old := `(curbuf->b_flags & BF_NEW) ? new_file_message() : "", `
	i := strings.Index(s, old)
	if i < 0 || strings.Count(s, old) != 1 {
		return nil, p.die("fileinfo()'s new-file argument is not where this phase expects it")
	}
	ls := strings.LastIndex(s[:i], "\n") + 1
	le := i + strings.Index(s[i:], "\n") + 1
	line := s[ls:le]
	ind := line[:len(line)-len(strings.TrimLeft(line, " "))]
	nl := strings.Replace(line, old, "new_msg, ", 1)
	s = s[:ls] + ind + "{\n" + ind + `    char        *new_msg = (curbuf->b_flags & BF_NEW) ? new_file_message() : "";` + "\n\n" + ind + "    " + strings.TrimLeft(nl, " ") + ind + "}\n" + s[le:]
	p.say("fileinfo() asks whether the file is new before whether to shorten the modified flag, as gcc does")
	return []byte(s), err
}
