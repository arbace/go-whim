package edit

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	storesDecl = regexp.MustCompile(`^[ \t]+(?:static\s+)?(?:const\s+)?(?:unsigned\s+|signed\s+)?(?:struct\s+)?([A-Za-z_]\w*)\s+\**\s*([A-Za-z_]\w*)(?:\s*=\s*(.*))?;$`)
	storesKey  = map[string]bool{"return": true, "goto": true, "case": true, "else": true, "sizeof": true}
)

// DeadStores takes Out every local that is only ever given a value: a variable
// declared inside a function Body, alone on its line, whose every other mention up to
// the end of the function is a statement of its own, `name = E;`, where E -- and
// the initialiser, if it has one -- only reads (PureCond).  Such a variable
// holds a value nothing reads, and none of its lines does anything else.
//
// gcc calls it "set but not used", and the sweep, which takes what gcc calls
// unused, leaves it: the statements that set it are its uses.  A phase that
// removes the last reader of a local -- the free that was the only thing done
// with a pointer, the empty if that was the only test of a flag -- leaves one.
//
// It is exported for the checks that compute a phase's output, and applied to
// a fixpoint: taking one store can leave another variable only stored.  Any
// line whose shape it does not know -- a store spread over two lines, a
// compound assignment, a second declaration of the name in a sibling block --
// keeps the variable.  It returns the text and the names it took, in order.
func DeadStores(core []byte) ([]byte, []string) {
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
			// storesDecl's own frame, asked first: an indented line
			// ending in `;`.
			if l := lines[k]; l == "" || (l[0] != ' ' && l[0] != '\t') || !strings.HasSuffix(l, ";") {
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
			// `\bname\b` counted by WordCount, the name being word
			// characters; the store's pattern compiled for the first line
			// that mentions it.
			if WordCount(lines[k], name) != 1 {
				continue
			}
			var store *regexp.Regexp
			kill := []int{k}
			ok := true
			for j := k + 1; j < end && ok; j++ {
				if WordCount(lines[j], name) == 0 {
					continue
				}
				if store == nil {
					store = regexp.MustCompile(`^[ \t]*` + name + ` = (.*);$`)
				}
				s := store.FindStringSubmatch(lines[j])
				if s == nil || !PureCond(s[1]) || WordCount(lines[j], name) != 1 {
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

// WordCount is len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).
// FindAllIndex(s, -1)): for a name of word characters, its occurrences with
// no word character on either side, which cannot overlap, found by
// strings.Index and not the regexp machine.  TestWordCount holds it to the
// regexp.
func WordCount[T ~string | ~[]byte](s T, name string) int {
	if !isIdentWord(name) {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllIndex([]byte(s), -1))
	}
	return wordCount(string(s), name)
}

// WordPatternCount is len(regexp.MustCompile(`\b(?:`+pat+`)\b`).FindAll(s,
// -1)), the mention count the phases write for a name or an alternation of
// names: a pat that is one name of word characters is WordCount's.
func WordPatternCount[T ~string | ~[]byte](s T, pat string) int {
	if !isIdentWord(pat) {
		return len(regexp.MustCompile(`\b(?:`+pat+`)\b`).FindAllIndex([]byte(s), -1))
	}
	return wordCount(string(s), pat)
}

func wordCount(s, name string) int {
	n := 0
	for pos := 0; ; {
		rel := strings.Index(s[pos:], name)
		if rel < 0 {
			return n
		}
		i := pos + rel
		j := i + len(name)
		pos = i + 1
		if (i > 0 && identChar(s[i-1])) || (j < len(s) && identChar(s[j])) {
			continue
		}
		n++
	}
}
