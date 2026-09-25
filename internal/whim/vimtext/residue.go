package vimtext

import (
	"regexp"
	"sort"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/dead"
)

// coreRowRe is the one physical line of a designated cmdnames[] row.
var coreRowRe = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)

func CoreRows(t []byte) [][]byte { return coreRowRe.FindAll(t, -1) }

// zResidue is step 5 of phases 89, 90, 91 and 92, which those phases write the same
// way because it is the same argument: THE TEXT THIS EDIT LEAVES DOES NOT
// COMPILE, and the honest form of that is a computation rather than a list of
// handlers.  Every surviving mention of a dying name must be inside a function
// definition, and no surviving cmdnames[] row may name that function -- which
// is the whole reason the sweep, and not the phase, takes the handler.
//
// It returns the number of mentions, the holding functions' names sorted, and
// the dying names actually FOUND, sorted -- phase 91 reports that third one where
// 89, 90 and 92 report the list they were given.  Each phase writes its own report
// line, because the wording is the phase's.
func CoreResidue(p edit.Ph, t []byte, dying []string) (int, []string, []string, error) {
	blanked := edit.Blank(t)
	defs := dead.FuncDefinitions(t, blanked)
	type span struct {
		a, z int
		Name string
	}
	spans := make([]span, 0, len(defs))
	for name, s := range defs {
		spans = append(spans, span{s[0], s[1], name})
	}
	// The Python sorts (a, z, name) tuples and takes who[-1], the LAST span
	// that contains the mention, so the sort has to be the same three keys in
	// the same order for the same function to be chosen.
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].a != spans[j].a {
			return spans[i].a < spans[j].a
		}
		if spans[i].z != spans[j].z {
			return spans[i].z < spans[j].z
		}
		return spans[i].Name < spans[j].Name
	})
	left := 0
	holders := map[string]bool{}
	found := map[string]bool{}
	for _, e := range dying {
		for _, m := range regexp.MustCompile(`\b`+e+`\b`).FindAllIndex(t, -1) {
			left++
			who := ""
			for _, s := range spans {
				if s.a <= m[0] && m[0] < s.z {
					who = s.Name
				}
			}
			if who == "" {
				return 0, nil, nil, p.Die("%s is still named at file scope, at offset %d -- this phase only "+
					"knows the shape where what is left is inside a function", e, m[0])
			}
			holders[who] = true
			found[e] = true
		}
	}
	names := make([]string, 0, len(holders))
	for fn := range holders {
		names = append(names, fn)
	}
	sort.Strings(names)
	survivors := CoreRows(t)
	for _, fn := range names {
		re := regexp.MustCompile(`\b` + fn + `\b`)
		for _, r := range survivors {
			if re.Match(r) {
				return 0, nil, nil, p.Die("%s still has a cmdnames[] row, so it is not the sweep's to take", fn)
			}
		}
	}
	seen := make([]string, 0, len(found))
	for e := range found {
		seen = append(seen, e)
	}
	sort.Strings(seen)
	return left, names, seen, nil
}
