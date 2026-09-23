package dead

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// EnumVals writes every enumerator of src and its value, one `NAME=VALUE` per
// line, sorted -- the format tools/enumvals.sh produced from DWARF, and the
// format LoadVals reads.
//
// It replaces a -g build and a readelf dump with the front end, which has done
// the same arithmetic the compiler did.  The values are still READ rather than
// re-derived by hand: several are `1 << 3`, `0x80000000L` or defined in terms
// of another enumerator, and the front end has already folded all of that.
//
// WHY THIS IS SAFE, measured rather than assumed.  The value string lands
// VERBATIM in a pin (`NAME = VALUE` spliced into the text), so a different
// spelling of the same number would move the product.  readelf's spelling was
// measured on the product's 1,155 enumerators: 29 are hex and every one of
// them is >= 0x10000, no decimal value >= 65,536 exists, and negatives are
// always decimal -- `LLONG_MIN=-9223372036854775808`, `SIZE_MAX=0xffff…`.
// formatVal reproduces exactly that rule, and enumsAgree is the check that it
// still does.
//
// WHAT DWARF COULD NOT SEE.  gcc emits DWARF only for enum types something
// USES, so its dump was a function of the uses and not of the declarations: on
// the committed product it omits 59 dead enumerators the tree declares. Those
// are the names AnalyseEnums would have found no value for, kept the run back
// for, and counted Unpinnable -- a class measured to fire 0 times in 200
// rounds, which is why replacing the source of the values changes no decision
// while removing the hole. Header enumerators are filtered out on the file
// name, as DWARF filtered them by never mentioning them.
func EnumVals(src, out string) error {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return fmt.Errorf("enumvals: %w", err)
	}
	ast, err := cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: src},
	})
	if err != nil {
		return fmt.Errorf("enumvals: %w", err)
	}

	seen := map[string]string{}
	var walk func(s *cc.Scope)
	walk = func(s *cc.Scope) {
		if s == nil {
			return
		}
		for _, nodes := range s.Nodes {
			for _, n := range nodes {
				e, ok := n.(*cc.Enumerator)
				if !ok {
					continue
				}
				if e.Position().Filename != src {
					continue
				}
				name := e.Token.SrcStr()
				if name == "" {
					continue
				}
				v, ok := formatVal(e.Value())
				if !ok {
					continue
				}
				seen[name] = v
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(ast.Scope)

	lines := make([]string, 0, len(seen))
	for name, v := range seen {
		lines = append(lines, name+"="+v)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return os.WriteFile(out, nil, 0o644)
	}
	return os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// formatVal spells a value the way readelf's DW_AT_const_value did: decimal
// below 65,536 and for anything negative, lowercase hex at or above it.  The
// threshold is not a guess -- see EnumVals.
func formatVal(v cc.Value) (string, bool) {
	const hexFrom = 65536
	switch x := v.(type) {
	case cc.Int64Value:
		if x >= hexFrom {
			return fmt.Sprintf("0x%x", int64(x)), true
		}
		return fmt.Sprintf("%d", int64(x)), true
	case cc.UInt64Value:
		if x >= hexFrom {
			return fmt.Sprintf("0x%x", uint64(x)), true
		}
		return fmt.Sprintf("%d", uint64(x)), true
	}
	return "", false
}
