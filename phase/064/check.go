package p064

// Whim phase 64, the check -- no formatting, comment or nroff-macro options.
// See phase/064/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim64", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim64", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  noformatopts ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_fo", "p_flp", "p_com", "p_para", "p_sections", "b_p_fo", "b_p_flp", "b_p_com", "has_format_option", "get_leader_len", "get_last_leader_offset",
		"auto_format", "check_auto_format", "did_add_space", "paragraph_start", "same_leader", "skip_comment", "get_number_indent", "ends_in_white",
		"inmacro", "buf_has_cstyle_comments", "end_comment_pending", "Insstart_textlen", "Insstart_blank_vcol", "did_set_formatoptions",
		"did_set_comments", "OPENLINE_DO_COM", "OPENLINE_COM_LIST", "OPENLINE_FORMAT", "OPENLINE_KEEPTRAIL", "INSCHAR_DO_COM", "INSCHAR_COM_LIST",
		"COM_MAX_LEN", "FO_WRAP", "FO_AUTO",
		"op_format", "format_lines", "fmt_check_par", "nv_gd", "find_decl", "OP_FORMAT", "OP_FORMAT2", "INSCHAR_FORMAT", "INSCHAR_NO_FEX", "cursor_start",
		"op_reindent", "bangredo", "OP_INDENT", "OP_FILTER", "CPO_FILTER",
		"cindent_on", "can_cindent", "set_can_cindent") {
		return harness.ErrReported
	}
	if !s.KeptE("  noformatopts ", " went too -- it was not the formatter's", "internal_format", "comp_textwidth", "startPS", "findpar", "check_linecomment", "op_shift", "op_colon", "do_bang", "do_filter",
		"may_do_si", "did_si", "can_si", "can_si_back") {
		return harness.ErrReported
	}
	s.Echo("  noformatopts no leader, format flag, nroff macro, formatter or declaration search is left; the wrap is")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  noformatopts ", "formatoptions", "formatlistpat", "comments", "paragraphs", "sections") {
		return harness.ErrReported
	}
	wt := filepath.Join(d, "w.txt")
	check.Put(wt, "")
	check.InD(d, "-e", "-s", "+set tw=10", "+normal! Aaaa bbb ccc ddd", "+wq", "w.txt")
	if check.Bar(wt) != "aaa bbb|ccc ddd|" {
		s.Echo("  noformatopts typing did not wrap at 'textwidth': '%s'", check.Bar(wt))
		return harness.ErrReported
	}
	g := filepath.Join(d, "g.txt")
	check.Put(g, "one two three four five six seven eight nine ten\nshort\n")
	check.InD(d, "-e", "-s", "+1", "+set tw=10", "+normal! gqq", "+wq", "g.txt")
	if check.Bar(g) != "one two three four five six seven eight nine ten|short|" {
		s.Echo("  noformatopts gqq still formatted: '%s'", check.Bar(g))
		return harness.ErrReported
	}
	check.InD(d, "-e", "-s", "+1", "+set tw=10", "+normal! gqj", "+wq", "g.txt")
	if check.Bar(g) != "one two three four five six seven eight nine ten|short|" {
		s.Echo("  noformatopts gq with a motion still formatted: '%s'", check.Bar(g))
		return harness.ErrReported
	}
	gd := filepath.Join(d, "gd.txt")
	check.Put(gd, "int x;\n\nvoid f(void)\n{\n    int x;\n    x = x + 1;\n}\n")
	check.InD(d, "-e", "-s", "+6", "+normal! 0fxgd", "+s/^/HERE /", "+wq", "gd.txt")
	if !check.GrepQ(check.ReadFile(gd), `^HERE     x = x + 1;$`, check.GBRE) {
		s.Echo("  noformatopts gd moved the cursor: %s", check.GrepNum(gd, "HERE", check.GBRE))
		return harness.ErrReported
	}
	op := filepath.Join(d, "op.txt")
	for _, k := range []string{"=", "!"} {
		check.Put(op, "a\nb\nc\n")
		check.InD(d, "-e", "-s", "+1", "+normal! "+k+"jix", "+wq", "op.txt")
		if check.Bar(op) != "a|b|c|" {
			s.Echo("  noformatopts %s still ran as an operator: '%s'", k, check.Bar(op))
			return harness.ErrReported
		}
	}
	check.Put(op, "a\nb\nc\n")
	check.InD(d, "-e", "-s", "+1", "+normal! ix", "+wq", "op.txt")
	if check.Bar(op) != "xa|b|c|" {
		s.Echo("  noformatopts the control insert failed: '%s'", check.Bar(op))
		return harness.ErrReported
	}
	si := filepath.Join(d, "si.txt")
	check.Put(si, "if (x) {\n")
	check.InD(d, "-e", "-s", "+set si sw=4", "+normal! GA\ry;", "+normal! o}", "+wq", "si.txt")
	if strings.ReplaceAll(check.CatA(si), "\n", "|") != "if (x) {$|    y;$|}$|" {
		s.Echo("  noformatopts smartindent stopped indenting: '%s'", check.Bar(si))
		return harness.ErrReported
	}
	p := filepath.Join(d, "p.txt")
	check.Put(p, "a\n.PP\nb\n")
	check.InD(d, "-e", "-s", "+1", "+normal! }", "+s/^/X/", "+wq", "p.txt")
	if check.Bar(p) != "a|.PP|Xb|" {
		s.Echo("  noformatopts } stopped somewhere other than the last line: '%s'", check.Bar(p))
		return harness.ErrReported
	}
	s.Echo("  noformatopts the five are unknown; typing wraps at 'textwidth'; gqq, gqj, gd, = and ! do nothing; } passes .PP")
	return nil
}
