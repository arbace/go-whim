package cut

import (
	"reflect"
	"regexp"
	"testing"
)

// TestInLines holds inLines to FindAllIndex on the whole text.
func TestInLines(t *testing.T) {
	text := []byte("struct b {\n    int b_p_x;\n    char_u *b_p_xy;\n};\n" +
		"    buf->b_p_x = 1;\n    buf->b_p_x = 2;\nfoo b_p_x\n" +
		"    case PV_X:\n    return (char_u *)&(curbuf->b_p_x);\n" +
		"    case PV_X:\n    case PV_Y:\n    return (char_u *)&(curbuf->b_p_x);\nb_p_x")
	for _, p := range []struct {
		before int
		re     string
	}{
		{0, `(?m)^[ \t]*(?:char_u[ \t]*\*|int[ \t]+)b_p_x;\n`},
		{0, `(?m)^[ \t]*buf->b_p_x = [^\n]*;\n`},
		{1, `(?m)^[ \t]*case[^\n]*\n[ \t]*return \(char_u \*\)&\(curbuf->b_p_x\);\n`},
		{0, `(?m)^b_p_x`},
	} {
		re := regexp.MustCompile(p.re)
		var want [][2]int
		for _, m := range re.FindAllIndex(text, -1) {
			want = append(want, [2]int{m[0], m[1]})
		}
		if got := inLines(re, text, []byte("b_p_x"), p.before); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, FindAllIndex %v", p.re, got, want)
		}
	}
}
