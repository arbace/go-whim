package p128

import "regexp"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	localDeclRe = regexp.MustCompile(`^\s+(?:static\s+)?[A-Za-z_][A-Za-z0-9_]*(?:\s+\*?[A-Za-z_][A-Za-z0-9_]*)*\s+\*?([A-Za-z_][A-Za-z0-9_]*)\s*(?:=[^;]*)?;$`)
)

// From phase 127.
// notDeclWords: `return OK;` has the shape of a declaration and is not one.  The
// first word of a declaration is a type, never one of these.
var notDeclWords = []string{"return", "goto", "break", "continue", "case", "else", "do"}
