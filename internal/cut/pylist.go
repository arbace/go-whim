package cut

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// pyList renders a []string the way Python prints a list of str.
func pyList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = edit.PyRepr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
