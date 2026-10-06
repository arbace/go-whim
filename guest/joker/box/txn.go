package box

import (
	"strings"
	"unsafe"

	. "github.com/candid82/joker/core"
)

// A TRANSACTION is one write of a ref, as an affine capability
// (doc/LISP-SANDBOX.md, *Capabilities used once*): (box/txn name) holds the
// ref as it is; (box/commit! t s) stores s and points the ref at it only if
// the ref is still what t saw -- true, or false when another write came
// first; (box/abort! t) gives it up.  Either spends t, and a spent t is an
// error to commit: it cannot be used twice.  (box/with-txn [t name] ...)
// aborts t on every way out of its body, so a transaction that escapes its
// scope is spent -- released where the scope ends, not when a collector
// finds it.
type txn struct {
	name  string
	old   [32]byte // the ref's hash when t began; all zero: no ref
	spent bool
}

var txnType = RegRefType("Txn", (*txn)(nil), "One write of a ref of the store, used once (box/txn).")

func (t *txn) Equals(o interface{}) bool   { return o == t }
func (t *txn) ToString(bool) string        { return "#object[Txn " + t.name + "]" }
func (t *txn) GetInfo() *ObjectInfo        { return nil }
func (t *txn) WithInfo(*ObjectInfo) Object { return t }
func (t *txn) GetType() *Type              { return txnType }
func (t *txn) Hash() uint32                { return HashPtr(uintptr(unsafe.Pointer(t))) }

func txnArg(args []Object, i int, fn string) *txn {
	t, ok := args[i].(*txn)
	if !ok {
		panic(RT.NewError("box/" + fn + ": not a transaction: " + args[i].ToString(true)))
	}
	return t
}

// installTxn interns the transactions' functions in ns, and the macro.
func installTxn(s Store, ns *Namespace, def func(string, func([]Object) Object)) {
	def("txn", func(args []Object) Object {
		CheckArity(args, 1, 1)
		name := EnsureArgIsString(args, 0).S
		t := &txn{name: name}
		if h, ok := s.Ref(name); ok {
			t.old = h
		}
		return t
	})
	def("txn-value", func(args []Object) Object {
		CheckArity(args, 1, 1)
		t := txnArg(args, 0, "txn-value")
		if t.old == ([32]byte{}) {
			return NIL
		}
		if b, ok := get(s, t.old); ok {
			return MakeString(string(b))
		}
		return NIL
	})
	def("commit!", func(args []Object) Object {
		CheckArity(args, 2, 2)
		t := txnArg(args, 0, "commit!")
		if t.spent {
			panic(RT.NewError("box/commit!: the transaction on " + t.name + " is spent"))
		}
		t.spent = true
		h, err := s.Put([]byte(EnsureArgIsString(args, 1).S))
		if err != nil {
			panic(RT.NewError("box/commit!: " + err.Error()))
		}
		old := t.old
		err = s.SetRef(t.name, h, &old)
		if c, ok := err.(conflict); ok && c.Conflict() {
			return MakeBoolean(false)
		}
		if err != nil {
			panic(RT.NewError("box/commit!: " + err.Error()))
		}
		return MakeBoolean(true)
	})
	def("abort!", func(args []Object) Object {
		CheckArity(args, 1, 1)
		txnArg(args, 0, "abort!").spent = true
		return NIL
	})
	def("spent?", func(args []Object) Object {
		CheckArity(args, 1, 1)
		return MakeBoolean(txnArg(args, 0, "spent?").spent)
	})
	// the macro is defined where the core is referred, box.txn, and
	// referred into box, whose own names (get, load) the core's would
	// shadow
	helper := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol("box.txn"))
	helper.ReferAll(GLOBAL_ENV.CoreNamespace)
	cur := GLOBAL_ENV.CurrentNamespace()
	GLOBAL_ENV.SetCurrentNamespace(helper)
	defer GLOBAL_ENV.SetCurrentNamespace(cur)
	PanicOnErr(ProcessReader(NewReader(strings.NewReader(withTxn), "<box>"), "", EVAL))
	vr, ok := GLOBAL_ENV.Resolve(MakeSymbol("box.txn/with-txn"))
	if !ok {
		panic(RT.NewError("box: with-txn not defined"))
	}
	ns.Refer(MakeSymbol("with-txn"), vr)
}

const withTxn = "(defmacro with-txn\n" +
	"  \"A transaction on ref name bound to t for body, aborted on every way\n" +
	"  out: committed in body or not, t is spent after.\"\n" +
	"  [[t name] & body]\n" +
	"  `(let [~t (box/txn ~name)]\n" +
	"     (try (do ~@body) (finally (box/abort! ~t)))))\n"
