package sweep

import (
	"strings"
	"testing"
)

// TestPruneOrphanedFallthrough holds the rule: a fallthrough attribute stays
// where control goes next to a case or default label, and goes where it does
// not -- the switch's end, a statement, a loop's end -- in either spelling, and gcc, -Werror,
// accepts what is left.
func TestPruneOrphanedFallthrough(t *testing.T) {
	out := prune(t, `
int main(int argc, char **argv) {
	int r = 0;
	(void)argv;
	switch (argc) {
	case 1:
		r = 1;
		[[fallthrough]];
	case 2:
		r++;
		[[fallthrough]];
	L:  default:
		if (r) {
			r = 2;
			[[fallthrough]];
		}
	case 3:
		do {
			r = 3;
			[[fallthrough]];
		} while (0);
	case 4:
		r = 4;
		[[fallthrough]];
		r = 5;
	case 5:
		while (r < 9) {
			r++;
			[[fallthrough]];
		}
		break;
	case 6:
		switch (r) {
		case 0:
			r = 6;
			[[fallthrough]];
		}
		r = 7;
		__attribute__((fallthrough));
	}
	if (r == 99) goto L;
	return r;
}
`)
	if n := strings.Count(out, "[[fallthrough]]"); n != 4 {
		t.Errorf("%d fallthroughs kept, want 4 (before case 2, before L, and the ends of the if and the do-while):\n%s", n, out)
	}
	has(t, out, `r = 4;\s*r = 5;`, true)
	has(t, out, `r\+\+;\s*\}`, true)
	has(t, out, `r = 6;\s*\}`, true)
	has(t, out, `r = 7;\s*\}`, true)
	has(t, out, `__attribute__`, false)
	compiles(t, out)
}
