package graph

import "fmt"

// ReplaceByUse puts a use of the ordinary name, resolved where each site
// stands, in the place of each of sites -- an expression a name now says,
// as a constant an enumerator was made for -- and types again what the
// replacement changed above it (Rederive, then C's conversions where a
// form's type is not plain).
func (e *Editor) ReplaceByUse(sites []*Node, name string) error {
	for _, s := range sites {
		d := e.Resolve(s, name)
		if d == nil {
			return fmt.Errorf("%s is not declared where #%d (%s) stands", name, s.ID, label(s))
		}
		u := e.RefTo(d)
		if err := e.Replace(s, u); err != nil {
			return err
		}
		e.Rederive(u)
	}
	e.brTypeUntyped()
	return nil
}
