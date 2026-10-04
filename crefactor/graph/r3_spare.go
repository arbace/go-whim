package graph

import "strings"

// The include rule's answer as a step reports it (R3, doc/GRAPH-MIGRATION.md):
// which include forms can go on their own, which go together, and whether
// those alone went together -- the questions crefactor/xform's Includes
// asked the compiler, in its order, asked of the rule instead.

// IncludeSpares is the rule's account of the file's include forms.
type IncludeSpares struct {
	All      []*Node // the include forms, in the file's order
	Alone    []*Node // each one whose deletion alone leaves every name provided
	Together bool    // the Alone ones can all go at once
	Spare    []*Node // what goes: Alone, or when not Together the fold from the bottom
	// Unprovided are the names the file takes from the headers that no
	// include above their first use provides already, before any deletion
	// (a tag the file declares nowhere, and no header has, is the file's
	// own incomplete type, and not here).
	Unprovided []HeaderUse
	Collisions []Collision // the include forms' macros over the file's own names, already
}

// Spares is SpareIncludes with its account: Spare is what SpareIncludes
// returns.  The forms are not deleted.
func (e *Editor) Spares() (*IncludeSpares, error) {
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	r := &IncludeSpares{All: append([]*Node(nil), s.incs...)}
	for _, u := range s.uses(nil) {
		if u.From == nil && !(strings.Contains(u.Name, " ") && !s.anyProvides(u.Name)) {
			r.Unprovided = append(r.Unprovided, u)
		}
	}
	r.Collisions = s.collisions()
	for _, inc := range s.incs {
		if len(s.missing(map[*Node]bool{inc: true})) == 0 {
			r.Alone = append(r.Alone, inc)
		}
	}
	all := map[*Node]bool{}
	for _, inc := range r.Alone {
		all[inc] = true
	}
	if len(s.missing(all)) == 0 {
		r.Together = true
		r.Spare = r.Alone
		return r, nil
	}
	keep := map[*Node]bool{}
	for i := len(r.Alone) - 1; i >= 0; i-- {
		keep[r.Alone[i]] = true
		if len(s.missing(keep)) > 0 {
			delete(keep, r.Alone[i])
		}
	}
	for _, inc := range r.Alone {
		if keep[inc] {
			r.Spare = append(r.Spare, inc)
		}
	}
	return r, nil
}

// DeleteIncludes deletes include forms at once, refused -- the graph left
// as it was -- where a name one of them provides would be left unprovided
// (Missing of them together): DeleteInclude for many, the headers' state
// computed once.
func (e *Editor) DeleteIncludes(incs ...*Node) error {
	miss, err := e.Missing(incs...)
	if err != nil {
		return err
	}
	if len(miss) > 0 {
		return &deleteIncludesError{unprovided(miss)}
	}
	for _, inc := range incs {
		if err := e.Delete(inc); err != nil {
			return err
		}
	}
	return nil
}

type deleteIncludesError struct{ what string }

func (d *deleteIncludesError) Error() string { return "delete includes: " + d.what }
