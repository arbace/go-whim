package p155

// Whim phase 155 -- call arguments with effects are evaluated in gcc's order.  See GOAL.md.
//
// gcc evaluates call arguments right to left; Go, left to right.  Where two
// arguments both have effects (internal/ccx), the one gcc evaluates first
// becomes a local computed before the call -- eleven calls.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim155", Edit) }

// w155Save is a copy of a line with its length, `x = vim_strnsave(get(args),
// get_len(args2));`, where the length is that line's when get_len is get's
// own and args2 is args.
const w155Save = `(?m)^( *)([\w_]+) = vim_strnsave\((ml_get\w*)\(([^()\n]*)\), (ml_get\w*_len)\(([^()\n]*)\)\);\n`

// Edit evaluates call arguments with effects in the order gcc does.
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
// the order is written, not implied (internal/gen/FINDINGS.md).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("argorder", text, w)
	saves := e.Query(w155Save, 0)
	get, args, getLen, args2 := e.Query(w155Save, 3), e.Query(w155Save, 4), e.Query(w155Save, 5), e.Query(w155Save, 6)
	for i := range saves {
		e.Expect(args[i] == args2[i] && getLen[i] == get[i]+"_len",
			"a copy of a line whose length is not that line's: %q", strings.TrimSpace(saves[i]))
	}
	e.Sub(w155Save, "${1}{\n${1}    colnr_T     len = ${5}(${4});\n\n${1}    ${2} = vim_strnsave(${3}(${4}), len);\n${1}}\n", 9,
		"the 9 copies of a line compute its length first, as gcc does")
	e.Literal("            col_print(buf2, sizeof(buf2), ml_get_curline_len(), linetabsize_str(p));\n",
		"            {\n                int         vcol = linetabsize_str(p);\n\n                col_print(buf2, sizeof(buf2), ml_get_curline_len(), vcol);\n            }\n", 1,
		"cursor_pos_info() computes the line's width before its length, as gcc does")
	e.Sub(`(?m)^( *)(.*)\(curbuf->b_flags & BF_NEW\) \? new_file_message\(\) : "", (.*)\n`,
		"${1}{\n${1}    char        *new_msg = (curbuf->b_flags & BF_NEW) ? new_file_message() : \"\";\n\n${1}    ${2}new_msg, ${3}\n${1}}\n", 1,
		"fileinfo() asks whether the file is new before whether to shorten the modified flag, as gcc does")
	return e.Done()
}
