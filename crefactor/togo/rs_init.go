package togo

// rs_init.go is an initializer as Rust: the object zeroed -- the editor is
// made zeroed, a local zeroed where C declares it -- and each of the
// initializer's values that is not a zero stored at the member or element
// it initializes, the front end's offset of it read back as the path to
// it: `(*ed).cmdnames[3].cmd_name = b"append\0".as_ptr() as *mut u8;`.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// initLocal gives a local its initializer's value where C declares it.
func (f *rfn) initLocal(l *rlocal, in *cc.Initializer) {
	t := l.t
	if in.Case == cc.InitializerExpr && t.Kind() != cc.Array {
		pre, post := f.hoist(in.AssignmentExpression, true, true)
		for _, p := range pre {
			f.line("%s", p)
		}
		v := f.conv(f.expr(in.AssignmentExpression), l.ty(f.r, f.r.ty(t)))
		f.done()
		f.line("%s = %s;", l.name, unparenRs(v.s))
		for _, p := range post {
			f.line("%s", p)
		}
		return
	}
	if in.Case == cc.InitializerExpr && t.Kind() == cc.Array && rsStringInit(in, t) {
		f.initInto(l.name, t, in) // the whole array, from the string: no zero first
		return
	}
	f.line("%s = %s;", l.name, f.zero(t))
	f.initInto(l.name, t, in)
}

// rsStringInit says a char array's initializer is a string initInto
// stores whole, str_u8::<N>(...): one with a byte not NUL.
func rsStringInit(in *cc.Initializer, t cc.Type) bool {
	sv, ok := unparenE(in.AssignmentExpression).Value().(cc.StringValue)
	if !ok {
		return false
	}
	b := []byte(string(sv))
	if int64(len(b)) > t.Size() {
		b = b[:t.Size()]
	}
	for _, c := range b {
		if c != 0 {
			return true
		}
	}
	return false
}

// initInto stores the initializer in's values into the object of type t at
// place, which is zeroed.  The front end gives each value its offset in the
// outermost object, and its type.
func (f *rfn) initInto(place string, t cc.Type, in *cc.Initializer) {
	if in == nil {
		return
	}
	if in.Case == cc.InitializerExpr {
		e := in.AssignmentExpression
		it := in.Type()
		if it == nil {
			it = t
		}
		path := f.r.pathTo(t, in.Offset(), it, in.Field())
		if path == nil {
			f.no(e, "an initializer at offset %d of %v", in.Offset(), t)
		}
		dst := place + strings.Join(path, "")
		if fl := in.Field(); fl != nil && fl.IsBitfield() {
			f.no(e, "a bit field's initializer")
		}
		if sv, ok := unparenE(e).Value().(cc.StringValue); ok && it.Kind() == cc.Array {
			// a char array from a string: its bytes, as many as fit
			n := it.Size()
			b := []byte(string(sv))
			if int64(len(b)) > n {
				b = b[:n]
			}
			for len(b) > 0 && b[len(b)-1] == 0 {
				b = b[:len(b)-1]
			}
			if len(b) == 0 {
				return
			}
			fn := "str_u8"
			if et := f.r.ty(elemOf(it)); et == "i8" {
				fn = "str_i8"
			}
			f.line("%s = %s::<%d>(%s);", dst, fn, n, rsBytes(b))
			return
		}
		x := f.expr(e)
		if (x.konst && x.kv == 0 && x.spell == "") || x.null {
			return // the memory is zeroed
		}
		v := f.conv(x, f.r.ty(it))
		f.line("%s = %s;", dst, unparenRs(v.s))
		return
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		f.initInto(place, t, l.Initializer)
	}
}

// pathTo is the members and elements, from an object of type t, to the one
// at offset off of type want: `.a`, `[3]`, `.b`.  fl, when known, is the
// member the value initializes, which says which of a union's.
func (r *rgen) pathTo(t cc.Type, off int64, want cc.Type, fl *cc.Field) []string {
	path := []string{}
	wty := r.ty(want)
	for steps := 0; steps < 64; steps++ {
		if off == 0 && r.ty(t) == wty {
			return path
		}
		switch t.Kind() {
		case cc.Array:
			e := elemOf(t)
			es := e.Size()
			if es <= 0 {
				return nil
			}
			i := off / es
			path = append(path, fmt.Sprintf("[%d]", i))
			off -= i * es
			t = e
		case cc.Struct, cc.Union:
			var pick *cc.Field
			for _, fi := range fields(t) {
				if fi.Offset() > off || off >= fi.Offset()+max(fi.Type().Size(), 1) {
					continue
				}
				if fl != nil && fieldWithin(fi, fl, 8) {
					pick = fi
					break
				}
				if pick == nil {
					pick = fi
				}
				if t.Kind() == cc.Struct && fl == nil {
					break
				}
			}
			if pick == nil {
				return nil
			}
			path = append(path, "."+rsName(pick.Name()))
			off -= pick.Offset()
			t = pick.Type()
		default:
			if off == 0 {
				return path // a scalar of another spelling: an enum's int
			}
			return nil
		}
	}
	return nil
}

// fieldWithin says fl is fi, or a member of fi's type, to depth levels.
func fieldWithin(fi, fl *cc.Field, depth int) bool {
	if fi == fl || fi.Name() == fl.Name() && fi.Offset() == fl.Offset() && fi.ParentType() == fl.ParentType() {
		return true
	}
	if depth == 0 || !isAggr(fi.Type()) {
		return false
	}
	for _, x := range fields(fi.Type()) {
		if fieldWithin(x, fl, depth-1) {
			return true
		}
	}
	return false
}

// rsInitChunk is how many statements one of init_globals's functions holds.
const rsInitChunk = 400

// initializers is init_globals: the objects' initial values written into a
// new editor, in functions of rsInitChunk statements; and the objects whose
// initializer it cannot write.
func (r *rgen) initializers() (string, []string) {
	var failed []string
	f := &rfn{r: r, name: "init_globals", local: map[*cc.Declarator]*rlocal{}, taken: map[string]bool{},
		gotos: map[*cc.JumpStatement]string{}, out: &strings.Builder{}, ind: 1, global: true}
	var lines []string
	for i := 0; i < len(r.objects); i++ { // a compound literal adds one
		o := r.objects[i]
		if o.in == nil {
			continue
		}
		var b string
		func() {
			defer func() {
				if e := recover(); e != nil {
					u, ok := e.(unsupported)
					if !ok {
						panic(e)
					}
					failed = append(failed, o.what+": "+u.why)
					b = ""
				}
			}()
			b = f.capture(func() { f.initInto("(*ed)."+o.field, o.t, o.in) })
		}()
		for _, l := range strings.Split(b, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
	}
	var b strings.Builder
	n := (len(lines) + rsInitChunk - 1) / rsInitChunk
	b.WriteString("/// The objects' initial values, written into a new editor, which is zeroed.\npub unsafe fn init_globals(ed: *mut Editor) {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "    init_globals_%d(ed);\n", i)
	}
	if n == 0 {
		b.WriteString("    let _ = ed;\n")
	}
	b.WriteString("}\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "unsafe fn init_globals_%d(ed: *mut Editor) {\n", i)
		for _, l := range lines[i*rsInitChunk : min(len(lines), (i+1)*rsInitChunk)] {
			b.WriteString("    " + l + "\n")
		}
		b.WriteString("}\n\n")
	}
	return b.String(), failed
}

// declTy is a local's Rust type as its declaration says it.
func (l *rlocal) declTy(r *rgen) string {
	if l.rty != "" {
		return l.rty
	}
	return l.ty(r, r.declType(l.t))
}

// ty is ty, the local's type, as *const where it is read-only (rs_const.go).
func (l *rlocal) ty(r *rgen, ty string) string {
	if l.d == nil {
		return ty
	}
	return r.constTy(declSlot(l.d), ty)
}

// zeroOf is a local's zero.
func (l *rlocal) zeroOf(r *rgen) string {
	if l.rty != "" {
		switch {
		case isPtrTy(l.rty):
			if isConstPtr(l.rty) {
				return "null()"
			}
			return "null_mut()"
		case isFnTy(l.rty):
			return "None"
		case l.rty == "bool":
			return "false"
		case isIntTy(l.rty):
			return "0"
		}
		return "core::mem::zeroed()"
	}
	return l.zeroIn(r)
}

// zeroIn is a local's zero by its C type: null() for a *const.
func (l *rlocal) zeroIn(r *rgen) string {
	z := r.zero(l.t)
	if z == "null_mut()" && isConstPtr(l.declTy(r)) {
		return "null()"
	}
	return z
}
