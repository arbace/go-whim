package graph

// PARAM, RETYPE and MOVE as verbs (B2c): each act reported as the text's
// were, refused as the text verbs refuse -- the first refusal stops the
// rest.

// DropParam drops the parameter param of the function fn, in every
// declaration, with the argument at every call (Editor.DropParam).
func (v *Verbs) DropParam(fn, param, what string) {
	if v.Err != nil {
		return
	}
	if _, err := v.e.DropParam(fn, param, ParamOptions{}); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// DropParams drops the parameters drops names, as one edit
// (Editor.DropParams).
func (v *Verbs) DropParams(drops []ParamDrop, opt ParamOptions, what string) ParamStats {
	if v.Err != nil {
		return ParamStats{}
	}
	st, err := v.e.DropParams(drops, opt)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return st
	}
	v.Say(what)
	return st
}

// ParamToLocal makes the parameter param of fn a local of its definition
// (Editor.ParamToLocal).
func (v *Verbs) ParamToLocal(fn, param, what string) {
	if v.Err != nil {
		return
	}
	if _, err := v.e.ParamToLocal(fn, param); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// Retype gives the one declaration the pattern matches in the scope the
// type typ (Editor.Retype).
func (v *Verbs) Retype(pat, typ, what string) {
	d := v.One(pat, what)
	if d == nil {
		return
	}
	if _, err := v.e.Retype(d, typ); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// RetypeResult gives the function fn the result typ (Editor.RetypeResult).
func (v *Verbs) RetypeResult(fn, typ, what string) {
	if v.Err != nil {
		return
	}
	if _, err := v.e.RetypeResult(fn, typ); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// MoveBefore moves the one item the pattern matches before the one item
// at matches, in the scope (Editor.MoveBefore); MoveAfter after it.
func (v *Verbs) MoveBefore(pat, at, what string) { v.move(pat, at, false, what) }

// MoveAfter is MoveBefore, after.
func (v *Verbs) MoveAfter(pat, at, what string) { v.move(pat, at, true, what) }

func (v *Verbs) move(pat, at string, after bool, what string) {
	n := v.One(pat, what)
	a := v.One(at, what)
	if n == nil || a == nil {
		return
	}
	var err error
	if after {
		err = v.e.MoveAfter(n, a)
	} else {
		err = v.e.MoveBefore(n, a)
	}
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}
