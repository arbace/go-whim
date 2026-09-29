package togo

import (
	"strings"
	"testing"
)

// jtidy drops the parentheses precedence makes redundant, keeps those it
// needs and those a reader wants, and leaves what it cannot read.
func TestJtidy(t *testing.T) {
	for _, c := range [][2]string{
		// redundant
		{"if (((a != 0)) || (b != 0)) {", "if (a != 0 || b != 0) {"},
		{"x = (a + b);", "x = a + b;"},
		{"return (a * b) + c;", "return a * b + c;"},
		{"f((a), (b + 1));", "f(a, b + 1);"},
		{"y = (a - b) - c;", "y = a - b - c;"},
		{"z = (s).add(1);", "z = s.add(1);"},
		{"case ('z' - 'a') + 1:", "case 'z' - 'a' + 1:"},
		{"n = (long) ((int) len);", "n = (long) (int) len;"},
		{"b = !(p.eq(q));", "b = !p.eq(q);"},
		{"v = (c ? a : b);", "v = c ? a : b;"},
		{"for (i = 0; (i < n) && (p != null); i++) {", "for (i = 0; i < n && p != null; i++) {"},
		// needed
		{"y = a - (b - c);", "y = a - (b - c);"},
		{"y = (a + b) * c;", "y = (a + b) * c;"},
		{"if ((x & F) != 0) {", "if ((x & F) != 0) {"},
		{"y = -(-x);", "y = -(-x);"},
		{"y = ((BytePtr) o).add(1);", "y = ((BytePtr) o).add(1);"},
		{"y = (Obj) (-x);", "y = (Obj) (-x);"},
		{"while ((c = next()) != 0) {", "while ((c = next()) != 0) {"},
		// kept for the reader
		{"if ((a && b) || c) {", "if ((a && b) || c) {"},
		{"m = (a | b) & c;", "m = (a | b) & c;"},
		{"m = (a + 1) & c;", "m = (a + 1) & c;"},
		{"m = a << (b + 1);", "m = a << (b + 1);"},
		{"t = (a < b) == c;", "t = (a < b) == c;"},
		// not read
		{"x = new Ptr<BytePtr>(p, 0);", "x = new Ptr<BytePtr>(p, 0);"},
		{"s = \"(a)\" + (b);", "s = \"(a)\" + b;"},
		{"// (a)", "// (a)"},
		{"x = (a) + b; // (c)", "x = (a) + b; // (c)"},
		{"k = new int[] {(a), b};", "k = new int[] {a, b};"},
	} {
		if got := jtidy(c[0]); got != c[1] {
			t.Errorf("jtidy(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

// jbreak breaks a line wider than jwidth after an argument's comma or
// before a && or a ||, at the lowest nesting, and leaves one it cannot.
func TestJBreak(t *testing.T) {
	long := "        if (alpha_beta_gamma_delta(one, two, three) != 0 && epsilon_zeta_eta_theta(four, five, six) != 0 && iota_kappa(7) != 0) {"
	got := jbreak(long, 0)
	if len(got) < 2 || !strings.HasPrefix(strings.TrimLeft(got[1], " "), "&&") {
		t.Errorf("jbreak(%q) = %q, want a break before a &&", long, got)
	}
	for _, g := range got {
		if len(g) > jwidth {
			t.Errorf("a line still wider than %d: %q", jwidth, g)
		}
	}
	if strings.Join(strings.Fields(strings.Join(got, " ")), " ") != strings.Join(strings.Fields(long), " ") {
		t.Errorf("the tokens moved: %q", got)
	}
	str := `        f("` + strings.Repeat("x, y && z ", 14) + `");`
	if got := jbreak(str, 0); len(got) != 1 {
		t.Errorf("a string was broken: %q", got)
	}
	cmt := "        int x = 1; // " + strings.Repeat("a, b && c ", 14)
	if got := jbreak(cmt, 0); len(got) != 1 {
		t.Errorf("a comment's line was broken: %q", got)
	}
}
