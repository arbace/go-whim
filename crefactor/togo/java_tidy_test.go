package togo

import "testing"

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
