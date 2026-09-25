// Package text is the generic C-text machinery the pipeline's edits stand on:
// brace matching with strings and characters blanked, definitions found and
// deleted by name, the counted folds of an `if` whose condition is decided,
// anchors matched exactly or modulo whitespace -- and, on those, the verb set
// an edit is written in.
//
// IT KNOWS C AND NOTHING ELSE.  Everything here works on any translation unit
// printed in the canonical form (crefactor/cemit: one spelling per construct,
// no comments, no preprocessor, a definition's name at column 0), and no
// string literal in it names an identifier of the program being edited: what
// an edit is about -- a function, a variable, a table -- is always the
// caller's argument.  Nothing here imports the vim side of the tree
// (internal/whim, internal/phase, internal/cut, internal/cmdtab);
// doc/VIM-VS-GENERIC.md section 4 is the layout this is part of.
//
// It was internal/cutil, the port of tools/cutil.py -- the substrate the text
// tools stood on -- and internal/cutil still forwards every name to it, so the
// code written against that package reads as it did.
package text
