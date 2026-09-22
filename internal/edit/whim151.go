package edit

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
)

func init() { register("whim151", Whim151) }

var (
	w151Cast    = regexp.MustCompile(`^\(\s*char_u\s*\*\s*\)\s*`)
	w151NumUse  = regexp.MustCompile(`\((int)\)\(long\)\(long_i\)((?:options\[[^\]]+\])|p)(\.|->)def_val\[`)
	w151LongUse = regexp.MustCompile(`\(long\)\(long_i\)((?:options\[[^\]]+\])|p)(\.|->)def_val\[`)
	w151StrUse  = regexp.MustCompile(`(\.|->)def_val\[`)
)

// W151Pair splits one row's default pair into the string pair and the number
// pair, by the row's kind: exported so the check applies the identical rule to
// the input's rows and compares with the output's.
func W151Pair(flags, a, b string) (str, num string) {
	strip := func(e string) string { return strings.TrimSpace(w151Cast.ReplaceAllString(strings.TrimSpace(e), "")) }
	if strings.Contains(flags, "P_STRING") || strings.TrimSpace(flags) == "0" {
		conv := func(e string) string {
			e = strings.TrimSpace(e)
			if e == "nullptr" || strip(e) == "0L" {
				return "nullptr"
			}
			return e
		}
		return "{" + conv(a) + ", " + conv(b) + "}", "{0L, 0L}"
	}
	return "{nullptr, nullptr}", "{" + strip(a) + ", " + strip(b) + "}"
}

// W151Rows is every row of options[] as (flags, default a, default b, the
// span of the default pair's braces), in order.
func W151Rows(text string) ([][4]string, [][2]int, error) {
	head := "static struct vimoption options[] =\n{\n"
	i := strings.Index(text, head)
	if i < 0 {
		return nil, nil, fmt.Errorf("options[] is not where this phase expects it")
	}
	b := cutil.Blank([]byte(text))
	open := i + len(head) - 2
	end := cutil.Match(b, open)
	var rows [][4]string
	var spans [][2]int
	for k := open + 1; k < end; k++ {
		if b[k] != '{' {
			continue
		}
		re := cutil.Match(b, k)
		// the default pair is the row's last brace pair
		dp := strings.LastIndex(string(b[k+1:re]), "{") + k + 1
		dq := cutil.Match(b, dp)
		inner := text[dp+1 : dq]
		bi := string(b[dp+1 : dq])
		c := strings.Index(bi, ",")
		// the third field of the row is its flags
		fields := strings.SplitN(string(b[k+1:re]), ",", 4)
		fl := ""
		if len(fields) >= 3 {
			off := k + 1 + len(fields[0]) + 1 + len(fields[1]) + 1
			fl = strings.TrimSpace(text[off : off+len(fields[2])])
		}
		rows = append(rows, [4]string{fl, inner[:c], inner[c+1:], ""})
		spans = append(spans, [2]int{dp, dq + 1})
		k = re
	}
	return rows, spans, nil
}

// Whim151 gives the option table typed defaults.
//
// vimoption_T.def_val[2] held each option's default, for Vi and for Vim: a
// string for a string option, and for a number or boolean option the number
// itself cast to char_u * -- `(char_u *)80L`, `(char_u *)TRUE` -- cast back
// with (long)(long_i) where it was read.  The Go transpilation held them as
// `any` (tx/FINDINGS.md, 2).  Now a row has def_str[2] and def_num[2]: a string
// option's defaults in the first, a number's or a boolean's in the second, the
// other pair empty; every read and write names the one it means.
func Whim151(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "defaults", w: w}
	s := string(text)
	rows, spans, err := W151Rows(s)
	if err != nil {
		return nil, p.die("%v", err)
	}
	for k := len(rows) - 1; k >= 0; k-- {
		str, num := W151Pair(rows[k][0], rows[k][1], rows[k][2])
		s = s[:spans[k][0]] + str + ", " + num + s[spans[k][1]:]
	}
	p.say(fmt.Sprintf("the %d rows of options[] give their defaults as a string pair and a number pair", len(rows)))
	var o []byte
	if o, err = p.literal([]byte(s), "    char_u      *def_val[2];\n", "    char_u      *def_str[2];\n    long        def_num[2];\n", "a row holds its string defaults and its number defaults apart", 1); err != nil {
		return nil, err
	}
	s = string(o)
	if o, err = p.literal([]byte(s), "options[opt_idx].def_val[VI_DEFAULT] = (char_u *)(long_i)val;", "options[opt_idx].def_num[VI_DEFAULT] = val;", "a number default is stored as a number", 1); err != nil {
		return nil, err
	}
	s = string(o)
	n1 := len(w151NumUse.FindAllString(s, -1))
	s = w151NumUse.ReplaceAllString(s, "(int)${2}${3}def_num[")
	n2 := len(w151LongUse.FindAllString(s, -1))
	s = w151LongUse.ReplaceAllString(s, "${1}${2}def_num[")
	n3 := len(w151StrUse.FindAllString(s, -1))
	s = w151StrUse.ReplaceAllString(s, "${1}def_str[")
	if n1+n2 != 6 || n3 != 12 {
		return nil, p.die("%d number reads and %d string uses of def_val, and this phase was written against 6 and 12", n1+n2, n3)
	}
	p.say(fmt.Sprintf("%d reads take the number without a cast, and %d uses of a string default name def_str", n1+n2, n3))
	return []byte(s), nil
}
