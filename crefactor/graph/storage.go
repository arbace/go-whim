package graph

import "fmt"

// SetStorage gives a file-scope declaration, and every other declaration
// of its entity, the storage class `static` or `extern`, or none (""):
// the token in the declaration's prefix, ahead of its attributes, the
// other storage class taken out.  It changes no type, no use and no id; an
// act `storage` names the declarations.  Refused for anything but a
// top-level def or defn, for a typedef, and where a class would repeat.
func (e *Editor) SetStorage(d *Node, class string) error {
	if class != "" && class != "static" && class != "extern" {
		return fmt.Errorf("storage of #%d: `%s` is not static, extern or none", d.ID, class)
	}
	if !e.Live(d) || d.up != e.top[0] || !(d.Is("def") || d.Is("defn")) || defNameAt(d) == 0 {
		return fmt.Errorf("storage of #%d (%s): not a file-scope def or defn", d.ID, label(d))
	}
	name := topName(d)
	var decls []*Node
	for _, f := range e.g.Forms {
		if (f.Is("def") || f.Is("defn")) && defNameAt(f) > 0 && topName(f) == name {
			decls = append(decls, f)
		}
	}
	act := Act{Op: "storage"}
	for _, f := range decls {
		i := defNameAt(f)
		var kids []*Node
		kids = append(kids, f.Kids[0])
		if class != "" {
			kids = append(kids, NewAtom(class))
		}
		for _, k := range f.Kids[1:i] {
			if !k.list && (k.Atom == "static" || k.Atom == "extern") {
				continue
			}
			if !k.list && k.Atom == "typedef" {
				return fmt.Errorf("storage of %s: #%d is a typedef", name, f.ID)
			}
			kids = append(kids, k)
		}
		kids = append(kids, f.Kids[i:]...)
		for _, k := range kids[1:] {
			k.up = f
		}
		f.Kids = kids
		act.Moved = append(act.Moved, f.ID)
	}
	e.Log = append(e.Log, act)
	return nil
}
