package edit

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestWordCount holds WordCount to the regexp `\bname\b` it stands for, on
// edge cases and, with $WHIM_C, every line of that file with every
// identifier on it.
func TestWordCount(t *testing.T) {
	lines := []string{"", "a", "a a", "aa a_ _a a", "x = a;", "9a a9 a"}
	if f := os.Getenv("WHIM_C"); f != "" {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, strings.Split(string(src), "\n")...)
	}
	ident := regexp.MustCompile(`[A-Za-z_]\w*`)
	for _, l := range lines {
		for _, name := range append(ident.FindAllString(l, -1), "a", "a a", "->a") {
			want := len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringIndex(l, -1))
			if got := WordCount(l, name); got != want {
				t.Fatalf("%q in %q: %d, the regexp %d", name, l, got, want)
			}
			if got := WordCount([]byte(l), name); got != want {
				t.Fatalf("%q in %q: %d as bytes, the regexp %d", name, l, got, want)
			}
		}
		for _, pat := range append(ident.FindAllString(l, -1), "a|b", "x?a", "int|char_u") {
			want := len(regexp.MustCompile(`\b(?:`+pat+`)\b`).FindAllStringIndex(l, -1))
			if got := WordPatternCount(l, pat); got != want {
				t.Fatalf("%q in %q: %d, the regexp %d", pat, l, got, want)
			}
		}
	}
}

// TestDeadStoresSame holds DeadStores to the version that matched every line
// by regexp (deadStoresRegexp, below, as it was), with $WHIM_C on that file.
func TestDeadStoresSame(t *testing.T) {
	f := os.Getenv("WHIM_C")
	if f == "" {
		t.Skip("WHIM_C")
	}
	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	// The file as it is, and with every line that returns or calls taken
	// out, which leaves locals only stored for the two to find.
	var cut []string
	for _, l := range strings.Split(string(src), "\n") {
		if !strings.Contains(l, "return") && !strings.Contains(l, "(") || strings.HasSuffix(l, ")") || strings.HasSuffix(l, "{") {
			cut = append(cut, l)
		}
	}
	for _, text := range [][]byte{src, []byte(strings.Join(cut, "\n"))} {
		got, gotTook := DeadStores(text)
		want, wantTook := deadStoresRegexp(text)
		if !bytes.Equal(got, want) || !reflect.DeepEqual(gotTook, wantTook) {
			t.Errorf("DeadStores took %v, the regexp's %v", gotTook, wantTook)
		}
		t.Logf("%s: %d taken", f, len(gotTook))
	}
}

func deadStoresRegexp(core []byte) ([]byte, []string) {
	var took []string
	lines := strings.Split(string(core), "\n")
	for {
		changed := false
		// the functions' bodies, by brace depth on the text with its strings
		// and characters blanked: a Body opens at depth 0, on a line whose
		// line above -- the head -- ends with its parameter list, and closes
		// where the depth is 0 again.  Not the columns: a brace in column 0
		// inside a Body is not rare enough to rely on.
		Body := make([]int, len(lines)) // the Body's last line, or -1
		depth, open := 0, -1
		for k, l := range lines {
			Body[k] = -1
			b := Blank([]byte(l))
			if depth == 0 && strings.TrimSpace(l) == "{" && k > 0 && strings.HasSuffix(lines[k-1], ")") {
				open = k
			}
			depth += bytes.Count(b, []byte("{")) - bytes.Count(b, []byte("}"))
			if depth == 0 && open >= 0 {
				for i := open; i <= k; i++ {
					Body[i] = k
				}
				open = -1
			}
		}
		for k := 0; k < len(lines) && !changed; k++ {
			end := Body[k]
			if end < 0 {
				continue
			}
			m := storesDecl.FindStringSubmatch(lines[k])
			if m == nil || storesKey[m[1]] || storesKey[m[2]] {
				continue
			}
			name := m[2]
			if m[3] != "" && !PureCond(m[3]) {
				continue
			}
			word := regexp.MustCompile(`\b` + name + `\b`)
			store := regexp.MustCompile(`^[ \t]*` + name + ` = (.*);$`)
			if len(word.FindAllStringIndex(lines[k], -1)) != 1 {
				continue
			}
			kill := []int{k}
			ok := true
			for j := k + 1; j < end && ok; j++ {
				if !word.MatchString(lines[j]) {
					continue
				}
				s := store.FindStringSubmatch(lines[j])
				if s == nil || !PureCond(s[1]) || len(word.FindAllStringIndex(lines[j], -1)) != 1 {
					ok = false
					continue
				}
				kill = append(kill, j)
			}
			if !ok {
				continue
			}
			for i := len(kill) - 1; i >= 0; i-- {
				lines = append(lines[:kill[i]], lines[kill[i]+1:]...)
			}
			took = append(took, name)
			changed = true
		}
		if !changed {
			return []byte(strings.Join(lines, "\n")), took
		}
	}
}
