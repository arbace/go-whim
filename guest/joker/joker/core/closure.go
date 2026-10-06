package core

// go-whim: a second backend.  The parser's expressions compiled to Go
// closures instead of bytecode: each expression a func of the frame it runs
// in, resolved once -- locals to slots, captures to the closure's values,
// vars to their *Var -- so that running it is calls of Go functions and no
// dispatch on opcodes.  UseClosures chooses it for the whole program: a
// top-level form evaluated, and every function compiled on its first call
// (Fn.ensureCompiled), the core library's included, since the generated
// core keeps each function's parsed source.  It is held to the bytecode
// VM's answers (doc/LISP-SANDBOX.md, *Closures*).
//
// The semantics are the VM's, piece by piece: a capture is the captured
// slot's value when the closure is made, a letfn's bindings cells shared
// by its closures (bindingCell); recur assigns the loop's slots and
// starts it again; try catches Errors whose type the catch names, and its
// finally runs on every way out; an error's position is the call site
// being called, as the VM's for a call.

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
)

// UseClosures compiles to closures instead of bytecode.  It is set before
// anything is compiled, and not changed after.
var UseClosures bool

type (
	// a cframe is one call's locals: slot 0 the function, then its
	// arguments, then its lets' bindings.
	cframe struct {
		slots []Object
		tmp   []Object // the arguments of the calls being made
		slab  []Object // where it is on its stack: popping it pops what
		base  int      // a panic caught elsewhere left above it
		depth int
		fn    *Fn
		recur bool
		site  *CallSite // the call it is making: its line in a stack trace
		fwd   *CallSite // a forwarding arity's call made from it, frameless
	}
	ccode func(f *cframe) Object

	cArity struct {
		arity    int // fixed arguments, the rest not counted
		variadic bool
		nslots   int
		ntmp     int
		fwd      *cForward // the arity only forwards its arguments to a var's function
		body     ccode
	}
	// a cForward is an arity whose body is a call of a var with its
	// arguments, in order -- (defn + ... ([x y] (add__ x y))) -- which a
	// call makes directly when the var holds a native of the core.
	cForward struct {
		vr   *Var
		site *CallSite
	}
	// a cProto is a compiled function, its arities and its captures.
	cProto struct {
		name     string
		arities  []*cArity
		variadic *cArity
		top      *cArity // a top-level form: no arity table
		captures []UpvalueInfo
	}

	clocal struct {
		b    *Binding
		slot int
	}
	ccomp struct {
		parent      *ccomp
		locals      []clocal
		next        int
		captures    *[]UpvalueInfo
		closedEnv   *LocalEnv
		loopSlots   []int // recur's targets; nil outside a loop
		tmp, maxTmp int   // the temporaries in use, and the most
	}
)

// compileClosureTop is a top-level form as a function of no arguments.
func compileClosureTop(expr Expr) (*cProto, error) {
	p := &cProto{name: "<top-level>"}
	c := &ccomp{captures: &p.captures, next: 1}
	body, err := c.compile(expr)
	if err != nil {
		return nil, err
	}
	p.top = &cArity{nslots: c.next, ntmp: c.maxTmp, body: body}
	return p, nil
}

// compileClosureFn is a function expression, compiled within parent (nil:
// at the top, its free bindings env's values).
func compileClosureFn(expr *FnExpr, parent *ccomp, env *LocalEnv) (*cProto, error) {
	p := &cProto{name: expr.functionName()}
	arity := func(a FnArityExpr, variadic bool) (*cArity, error) {
		c := &ccomp{parent: parent, captures: &p.captures, closedEnv: env, next: 1}
		if expr.selfBinding != nil {
			c.locals = append(c.locals, clocal{b: expr.selfBinding, slot: 0})
		}
		var slots []int
		for i := range a.args {
			slots = append(slots, c.next)
			c.locals = append(c.locals, clocal{b: a.bindings[i], slot: c.next})
			c.next++
		}
		c.loopSlots = slots
		body, err := c.body(a.body)
		if err != nil {
			return nil, err
		}
		fixed := len(a.args)
		if variadic {
			fixed--
		}
		return &cArity{arity: fixed, variadic: variadic, nslots: c.next, ntmp: c.maxTmp, body: loopOf(body), fwd: forwardOf(a, variadic)}, nil
	}
	for _, a := range expr.arities {
		ca, err := arity(a, false)
		if err != nil {
			return nil, err
		}
		p.arities = append(p.arities, ca)
	}
	if expr.variadic != nil {
		ca, err := arity(*expr.variadic, true)
		if err != nil {
			return nil, err
		}
		p.variadic = ca
	}
	return p, nil
}

// loopOf runs body again while it ends in a recur.
func loopOf(body ccode) ccode {
	return func(f *cframe) Object {
		for {
			v := body(f)
			if !f.recur {
				return v
			}
			f.recur = false
		}
	}
}

func (c *ccomp) resolveLocal(b *Binding) int {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].b == b {
			return c.locals[i].slot
		}
	}
	return -1
}

func (c *ccomp) resolveUpvalue(b *Binding) int {
	if c.parent == nil {
		return -1
	}
	slot := c.parent.resolveLocal(b)
	local := slot >= 0
	if !local {
		slot = c.parent.resolveUpvalue(b)
	}
	if slot < 0 {
		return -1
	}
	for i, u := range *c.captures {
		if u.Index == slot && u.IsLocal == local {
			return i
		}
	}
	*c.captures = append(*c.captures, UpvalueInfo{Index: slot, IsLocal: local})
	return len(*c.captures) - 1
}

func (c *ccomp) body(body []Expr) (ccode, error) {
	if len(body) == 0 {
		return func(*cframe) Object { return nilObject }, nil
	}
	codes := make([]ccode, len(body))
	for i, e := range body {
		code, err := c.compile(e)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	if len(codes) == 1 {
		return codes[0], nil
	}
	init, last := codes[:len(codes)-1], codes[len(codes)-1]
	return func(f *cframe) Object {
		for _, code := range init {
			code(f)
		}
		return last(f)
	}, nil
}

func (c *ccomp) all(es []Expr) ([]ccode, error) {
	codes := make([]ccode, len(es))
	for i, e := range es {
		code, err := c.compile(e)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

func (c *ccomp) compile(expr Expr) (ccode, error) {
	switch e := expr.(type) {
	case *LiteralExpr:
		o := e.obj
		return func(*cframe) Object { return o }, nil
	case *VectorExpr:
		codes, err := c.all(e.v)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			arr := make([]Object, len(codes))
			for i, code := range codes {
				arr[i] = code(f)
			}
			return &ArrayVector{arr: arr}
		}, nil
	case *MapExpr:
		keys, err := c.all(e.keys)
		if err != nil {
			return nil, err
		}
		values, err := c.all(e.values)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			var m Object
			if int64(len(keys)) > HASHMAP_THRESHOLD/2 {
				m = EmptyHashMap
			} else {
				m = EmptyArrayMap()
			}
			for i := range keys {
				k := keys[i](f)
				if hm, ok := m.(*HashMap); ok && hm.containsKey(k) {
					panic(RT.NewError("Duplicate key: " + k.ToString(false)))
				}
				v := values[i](f)
				switch mm := m.(type) {
				case *HashMap:
					m = mm.Assoc(k, v)
				case *ArrayMap:
					if !mm.Add(k, v) {
						panic(RT.NewError("Duplicate key: " + k.ToString(false)))
					}
				}
			}
			return m
		}, nil
	case *SetExpr:
		codes, err := c.all(e.elements)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			s := EmptySet()
			for _, code := range codes {
				v := code(f)
				if !s.Add(v) {
					panic(RT.NewError("Duplicate set element: " + v.ToString(false)))
				}
			}
			return s
		}, nil
	case *MetaExpr:
		meta, err := c.compile(e.meta)
		if err != nil {
			return nil, err
		}
		obj, err := c.compile(e.expr)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			m := meta(f).(Map)
			return obj(f).(Meta).WithMeta(m)
		}, nil
	case *IfExpr:
		cond, err := c.compile(e.cond)
		if err != nil {
			return nil, err
		}
		pos, err := c.compile(e.positive)
		if err != nil {
			return nil, err
		}
		neg, err := c.compile(e.negative)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			if ToBool(cond(f)) {
				return pos(f)
			}
			return neg(f)
		}, nil
	case *DoExpr:
		return c.body(e.body)
	case *LetExpr:
		return c.let(e, false)
	case *LoopExpr:
		return c.let((*LetExpr)(e), !e.recursive)
	case *RecurExpr:
		if c.loopSlots == nil {
			return nil, RT.NewError("recur outside of loop")
		}
		args, err := c.all(e.args)
		if err != nil {
			return nil, err
		}
		slots := c.loopSlots
		if len(args) != len(slots) {
			return nil, RT.NewErrorWithPos(fmt.Sprintf("recur with %d arguments for %d", len(args), len(slots)), e.Pos())
		}
		return func(f *cframe) Object {
			var buf [4]Object
			vals := buf[:0]
			for _, a := range args {
				vals = append(vals, a(f))
			}
			for i, s := range slots {
				f.slots[s] = vals[i]
			}
			f.recur = true
			return nilObject
		}, nil
	case *BindingExpr:
		if slot := c.resolveLocal(e.binding); slot >= 0 {
			return func(f *cframe) Object { return capturedValue(f.slots[slot]) }, nil
		}
		if i := c.resolveUpvalue(e.binding); i >= 0 {
			return func(f *cframe) Object { return capturedValue(f.fn.upvalues[i]) }, nil
		}
		root := c
		for root.parent != nil {
			root = root.parent
		}
		env := root.closedEnv
		for env != nil && env.frame > e.binding.frame {
			env = env.parent
		}
		if env == nil || env.frame != e.binding.frame || e.binding.index >= len(env.bindings) || env.bindings[e.binding.index] == nil {
			return nil, RT.NewErrorWithPos("Cannot resolve binding: "+e.binding.name.ToString(false), e.Pos())
		}
		o := env.bindings[e.binding.index]
		return func(*cframe) Object { return o }, nil
	case *VarRefExpr:
		vr := e.vr
		return func(*cframe) Object { return vr.Resolve() }, nil
	case *CallExpr:
		callee, err := c.compile(e.callable)
		if err != nil {
			return nil, err
		}
		off, n := c.tmp, len(e.args)
		c.tmp += n
		c.maxTmp = maxInt(c.maxTmp, c.tmp)
		args, err := c.all(e.args)
		c.tmp = off
		if err != nil {
			return nil, err
		}
		site := &CallSite{Position: e.Pos(), name: e.Name()}
		check := e.callable.Pos()
		// an inline cache: the arity the last closure called here took
		var lastP *cProto
		var lastA *cArity
		return func(f *cframe) Object {
			fn := callee(f)
			if _, ok := fn.(Callable); !ok {
				panic(RT.NewErrorWithPos(fn.ToString(false)+" is not a Fn", check))
			}
			vals := f.tmp[off : off+n : off+n]
			for i, a := range args {
				vals[i] = a(f)
			}
			f.site = site
			if x, ok := fn.(*Fn); ok && x.cproto != nil {
				a := lastA
				if x.cproto != lastP {
					if a = x.cproto.arity(n); a != nil {
						lastP, lastA = x.cproto, a
					}
				}
				if a != nil {
					return callArityAt(site, x, a, vals)
				}
			}
			return callAt(site, fn, vals) // the caller's frame clears vals when it returns
		}, nil
	case *FnExpr:
		p, err := compileClosureFn(e, c, nil)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			fn := &Fn{cproto: p, upvalues: make([]Object, len(p.captures))}
			for i, u := range p.captures {
				var v Object
				if u.IsLocal {
					v = f.slots[u.Index]
				} else {
					v = f.fn.upvalues[u.Index]
				}
				if v == nil {
					panic(RT.NewError("closure invariant: uninitialized capture"))
				}
				fn.upvalues[i] = v
			}
			return fn
		}, nil
	case *DefExpr:
		vr := e.vr
		var value, meta ccode
		var err error
		if e.value != nil {
			if value, err = c.compile(e.value); err != nil {
				return nil, err
			}
		}
		m := EmptyArrayMap()
		m.Add(KEYWORDS.line, Int{I: e.startLine})
		m.Add(KEYWORDS.column, Int{I: e.startColumn})
		m.Add(KEYWORDS.file, String{S: e.Position.Filename()})
		m.Add(KEYWORDS.ns, e.vr.ns)
		m.Add(KEYWORDS.name, e.vr.name)
		if e.meta != nil {
			if meta, err = c.compile(e.meta); err != nil {
				return nil, err
			}
		}
		return func(f *cframe) Object {
			if value != nil {
				vr.Value = value(f)
			}
			vr.meta = m
			if meta != nil {
				vr.meta = vr.meta.Merge(meta(f).(Map))
			}
			if vr.isMacro {
				vr.meta = vr.meta.Assoc(KEYWORDS.macro, Boolean{B: true}).(Map)
			}
			return vr
		}, nil
	case *SetMacroExpr:
		vr := e.vr
		return func(*cframe) Object {
			vr.isMacro = true
			vr.isUsed = false
			if fn, ok := vr.Value.(*Fn); ok {
				fn.isMacro = true
			}
			setMacroMeta(vr)
			return vr
		}, nil
	case *ThrowExpr:
		v, err := c.compile(e.e)
		if err != nil {
			return nil, err
		}
		return func(f *cframe) Object {
			x := v(f)
			if err, ok := x.(Error); ok {
				panic(err)
			}
			panic(RT.NewError("Cannot throw " + x.ToString(false)))
		}, nil
	case *TryExpr:
		return c.try(e)
	case *MacroCallExpr:
		macro := e.macro.(Object)
		args := e.args
		site := &CallSite{Position: e.Pos(), name: e.Name()}
		return func(f *cframe) Object {
			f.site = site
			return callAt(site, macro, append([]Object(nil), args...))
		}, nil
	}
	return nil, RT.NewErrorWithPos(fmt.Sprintf("Invalid runtime expression %T", expr), expr.Pos())
}

func (c *ccomp) let(e *LetExpr, loop bool) (ccode, error) {
	n := len(c.locals)
	slots := make([]int, len(e.names))
	if e.recursive {
		for i := range e.names {
			slots[i] = c.next
			c.locals = append(c.locals, clocal{b: e.bindings[i], slot: c.next})
			c.next++
		}
	}
	values := make([]ccode, len(e.values))
	for i, v := range e.values {
		code, err := c.compile(v)
		if err != nil {
			return nil, err
		}
		values[i] = code
		if !e.recursive {
			slots[i] = c.next
			c.locals = append(c.locals, clocal{b: e.bindings[i], slot: c.next})
			c.next++
		}
	}
	saved := c.loopSlots
	if loop {
		c.loopSlots = slots
	}
	body, err := c.body(e.body)
	c.loopSlots = saved
	c.locals = c.locals[:n]
	if err != nil {
		return nil, err
	}
	if loop {
		body = loopOf(body)
	}
	recursive := e.recursive
	return func(f *cframe) Object {
		if recursive {
			for _, s := range slots {
				f.slots[s] = &bindingCell{Object: nilObject}
			}
			for i, v := range values {
				f.slots[slots[i]].(*bindingCell).Object = v(f)
			}
		} else {
			for i, v := range values {
				f.slots[slots[i]] = v(f)
			}
		}
		return body(f)
	}, nil
}

func (c *ccomp) try(e *TryExpr) (ccode, error) {
	body, err := c.body(e.body)
	if err != nil {
		return nil, err
	}
	type catch struct {
		t    *Type
		slot int
		body ccode
	}
	catches := make([]catch, len(e.catches))
	for i, k := range e.catches {
		slot := c.next
		c.next++
		n := len(c.locals)
		c.locals = append(c.locals, clocal{b: k.binding, slot: slot})
		code, err := c.body(k.body)
		c.locals = c.locals[:n]
		if err != nil {
			return nil, err
		}
		catches[i] = catch{t: k.excType, slot: slot, body: code}
	}
	var finally ccode
	if e.finallyExpr != nil {
		if finally, err = c.body(e.finallyExpr); err != nil {
			return nil, err
		}
	}
	caught := func(f *cframe) (result Object) {
		if len(catches) == 0 {
			return body(f)
		}
		s := RT.closureStack()
		depth, top, vals := s.mark()
		site := RT.currentExpr
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			s.reset(depth, top, vals)
			RT.currentExpr = site
			if err, ok := r.(Error); ok {
				for _, k := range catches {
					if IsInstance(k.t, err) {
						f.recur = false
						f.slots[k.slot] = err
						result = k.body(f)
						return
					}
				}
			}
			panic(r)
		}()
		return body(f)
	}
	if finally == nil {
		return caught, nil
	}
	return func(f *cframe) Object {
		defer finally(f)
		return caught(f)
	}, nil
}

// callAt calls fn with args, the error's call site site meanwhile.  args
// are the caller's temporaries: a callee that may keep them gets a copy.
func callAt(site *CallSite, fn Object, args []Object) (res Object) {
	prev := RT.currentExpr
	RT.currentExpr = site
	defer func() { RT.currentExpr = prev }() // on a panic too: a native that wraps it is where the VM's is
	switch x := fn.(type) {
	case *Fn:
		if x.cproto != nil {
			res = callClosure(x, args) // copies them
		} else {
			res = x.Call(append([]Object(nil), args...))
		}
	case Proc:
		if x.Package == "" {
			res = x.Fn(args) // the core's keep none, as the VM passes its stack
		} else {
			res = x.Fn(append([]Object(nil), args...))
		}
	case Callable:
		res = x.Call(append([]Object(nil), args...))
	default:
		panic(RT.NewError("Cannot call " + fn.ToString(false)))
	}
	return res
}

func (p *cProto) arity(argc int) *cArity {
	if p.top != nil {
		if argc == 0 {
			return p.top
		}
		return nil
	}
	for _, a := range p.arities {
		if a.arity == argc {
			return a
		}
	}
	if a := p.variadic; a != nil && argc >= a.arity {
		return a
	}
	return nil
}

func (p *cProto) arityError(argc int) *ArityError {
	err := &ArityError{Actual: argc, VariadicMin: -1, name: p.name}
	for _, a := range p.arities {
		err.Fixed = append(err.Fixed, a.arity)
	}
	if p.variadic != nil {
		err.VariadicMin = p.variadic.arity
	} else if p.top != nil {
		err.Fixed = []int{0}
	}
	return err
}

// callClosure is a call of a closure-compiled function.
func callClosure(fn *Fn, args []Object) Object {
	p := fn.cproto
	a := p.arity(len(args))
	if a == nil {
		if fn.isMacro {
			fn.panicClosureMacroArity(len(args))
		}
		panic(RT.NewError(p.arityError(len(args)).Error()))
	}
	return callArity(fn, a, args)
}

// callArityAt is callAt of a closure whose arity a call site knows.
func callArityAt(site *CallSite, fn *Fn, a *cArity, args []Object) Object {
	prev := RT.currentExpr
	RT.currentExpr = site
	defer func() { RT.currentExpr = prev }()
	return callArity(fn, a, args)
}

// callArity is a call of fn's arity a.
func callArity(fn *Fn, a *cArity, args []Object) Object {
	yieldTick()
	if fw := a.fwd; fw != nil {
		if p, ok := fw.vr.Resolve().(Proc); ok && p.Package == "" {
			prev := RT.currentExpr
			RT.currentExpr = fw.site
			var in *cframe // the frame a trace shows it after
			var was *CallSite
			if s := RT.cstack; s != nil && s.depth > 0 {
				in = s.frames[s.depth-1]
				was, in.fwd = in.fwd, fw.site
			}
			res := p.Fn(args) // keeps none, as the VM passes its stack
			if in != nil {
				in.fwd = was
			}
			RT.currentExpr = prev
			return res
		}
	}
	s := RT.closureStack()
	f := s.push(fn, a.nslots, a.ntmp)
	f.slots[0] = fn
	if a.variadic {
		copy(f.slots[1:], args[:a.arity])
		if n := len(args) - a.arity; n > 0 {
			f.slots[1+a.arity] = &ArraySeq{arr: append([]Object(nil), args[a.arity:]...)}
		} else {
			f.slots[1+a.arity] = nilObject
		}
	} else {
		copy(f.slots[1:], args)
	}
	res := a.body(f)
	s.pop(f)
	return res
}

// panicClosureMacroArity is panicMacroArity on a closure's arities.
func (fn *Fn) panicClosureMacroArity(argCount int) {
	min, max := 1<<31-1, -1
	for _, a := range fn.cproto.arities {
		min, max = minInt(min, a.arity), maxInt(max, a.arity)
	}
	if a := fn.cproto.variadic; a != nil {
		min, max = minInt(min, a.arity+1), 1<<31-1
	}
	min -= 2
	if max != 1<<31-1 {
		max -= 2
	}
	PanicArityMinMax(argCount-2, min, max)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// EvaluateClosure is Evaluate on closures.
func EvaluateClosure(expr Expr) Object {
	p, err := compileClosureTop(expr)
	PanicOnErr(err)
	s := RT.closureStack()
	depth, top, vals := s.mark()
	defer func() {
		if r := recover(); r != nil {
			s.reset(depth, top, vals)
			panic(r)
		}
	}()
	return callClosure(&Fn{cproto: p}, nil)
}

//go:embed data/core.joke
var coreSource string

// LoadCoreClosures compiles to closures from now on and evaluates
// joker.core's source again, so that its functions -- shipped as bytecode
// (a_code.go) -- are closures too: a program and the library it calls on
// one backend.  Joker's environment must be initialised (InitEnv,
// ProcessCoreData) and its lock held.
func LoadCoreClosures() error {
	UseClosures = true
	ns := GLOBAL_ENV.CurrentNamespace()
	GLOBAL_ENV.SetCurrentNamespace(GLOBAL_ENV.CoreNamespace)
	defer GLOBAL_ENV.SetCurrentNamespace(ns)
	return ProcessReader(NewReader(strings.NewReader(coreSource), "<joker.core>"), "", EVAL)
}

// A cstack is the closures' frames and their values, reused call after
// call: a call allocates nothing.  It is one execution's -- the runtime's
// while it holds the GIL, kept aside by Suspend and a fresh one for
// LockIndependent, as the VM's context is -- so that executions
// interleaved by a native call's release of the GIL never share one.  A
// panic leaves its frames pushed; try and the top level pop them to where
// they were.
type cstack struct {
	frames []*cframe
	depth  int
	vals   []Object
	top    int
}

func (rt *Runtime) closureStack() *cstack {
	if rt.cstack == nil {
		rt.cstack = &cstack{vals: make([]Object, 1024)}
	}
	return rt.cstack
}

func (s *cstack) push(fn *Fn, nslots, ntmp int) *cframe {
	if s.depth == len(s.frames) {
		s.frames = append(s.frames, &cframe{})
	}
	f := s.frames[s.depth]
	s.depth++
	n := nslots + ntmp
	if s.top+n > len(s.vals) {
		// a new slab: the frames on the old keep it
		s.vals, s.top = make([]Object, max(2*len(s.vals), n)), 0
	}
	all := s.vals[s.top : s.top+n : s.top+n]
	s.top += n
	f.fn, f.recur, f.site, f.fwd = fn, false, nil, nil
	f.slots, f.tmp = all[:nslots], all[nslots:]
	f.slab, f.base, f.depth = s.vals, s.top-n, s.depth-1
	return f
}

func (s *cstack) pop(f *cframe) {
	clear(f.slots[:len(f.slots)+len(f.tmp)])
	s.vals, s.top, s.depth = f.slab, f.base, f.depth
	f.fn, f.slots, f.tmp, f.slab = nil, nil, nil, nil
}

// mark is where the stack is; reset pops back to it after a panic.
func (s *cstack) mark() (int, int, []Object) { return s.depth, s.top, s.vals }

func (s *cstack) reset(depth, top int, vals []Object) {
	for i := depth; i < s.depth; i++ {
		f := s.frames[i]
		clear(f.slots[:len(f.slots)+len(f.tmp)])
		f.fn, f.slots, f.tmp, f.slab = nil, nil, nil, nil
	}
	s.depth, s.top, s.vals = depth, top, vals
}

// forwardOf is a's forwarding, when its body is one call of a var with its
// arguments in order.
func forwardOf(a FnArityExpr, variadic bool) *cForward {
	if variadic || len(a.body) != 1 {
		return nil
	}
	call, ok := a.body[0].(*CallExpr)
	if !ok || len(call.args) != len(a.bindings) {
		return nil
	}
	vr, ok := call.callable.(*VarRefExpr)
	if !ok {
		return nil
	}
	for i, x := range call.args {
		if b, ok := x.(*BindingExpr); !ok || b.binding != a.bindings[i] {
			return nil
		}
	}
	return &cForward{vr: vr.vr, site: &CallSite{Position: call.Pos(), name: call.Name()}}
}

// appendTrace is the closures' frames in a stack trace, as the VM's
// (vmContext.appendTrace): each frame's call, but the innermost's when it
// is the call being made -- the trace's last line, rt.currentExpr -- and
// after a frame the forwarding call it is making, which has no frame.
func (s *cstack) appendTrace(stack *Callstack, current Traceable) {
	var sites []*CallSite
	for i := 0; i < s.depth; i++ {
		if f := s.frames[i]; f.site != nil {
			sites = append(sites, f.site)
			if f.fwd != nil {
				sites = append(sites, f.fwd)
			}
		}
	}
	if n := len(sites); n > 0 && Traceable(sites[n-1]) == current {
		sites = sites[:n-1]
	}
	for _, site := range sites {
		stack.pushFrame(Frame{traceable: site})
	}
}

// calls counts the calls made, both backends'.
var calls uint32

// yieldTick yields the processor every 65,536 calls.  A Go runtime with
// no sysmon -- TamaGo's -- preempts nothing, so on one vCPU a collection
// started while a long computation runs has its mark workers waiting for
// the goroutine to block, and the computation runs with the write barrier
// on and assisting, four times slower (doc/LISP-SANDBOX.md, *Unboxed
// numbers*); a yield now and then lets the workers finish the cycle.
func yieldTick() {
	if calls++; calls&0xffff == 0 {
		runtime.Gosched()
	}
}
