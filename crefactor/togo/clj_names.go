package togo

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// NAMES AS CLOJURE WRITES THEM (doc/CLOJURE-IDIOMS.md item 3): a C name's
// underscores between its letters are hyphens -- ml_get_buf is ml-get-buf,
// w_cursor w-cursor -- and a function that answers a question and changes
// nothing ends in ? (buf-valid?).  Clojure munges a hyphen back into an
// underscore in the classes it writes, so a function's, a field's and a
// method's Java name is the C's still.  A leading or trailing underscore
// stays (inc_, _anon0): .-x is Clojure's field syntax.  The enumerators keep
// the C's names, as constants, and the struct types theirs, as classes.

// cljCore is every public var of clojure.core (Clojure 1.12): a name the
// namespace writes that is one takes a trailing underscore, so that a local
// never shadows what the printer calls.
var cljCore = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`
	* *' *1 *2 *3 *agent* *allow-unresolved-vars* *assert* *clojure-version*
	*command-line-args* *compile-files* *compile-path* *compiler-options*
	*data-readers* *default-data-reader-fn* *e *err* *file* *flush-on-newline*
	*fn-loader* *in* *math-context* *ns* *out* *print-dup* *print-length*
	*print-level* *print-meta* *print-namespace-maps* *print-readably*
	*read-eval* *reader-resolver* *repl* *source-path* *suppress-read*
	*unchecked-math* *use-context-classloader* *verbose-defrecords*
	*warn-on-reflection* + +' - -' -> ->> ->ArrayChunk ->Eduction ->Vec
	->VecNode ->VecSeq -cache-protocol-fn -reset-methods .. / < <= = == > >=
	EMPTY-NODE Inst NaN? PrintWriter-on StackTraceElement->vec Throwable->map
	abs accessor aclone add-classpath add-tap add-watch agent agent-error
	agent-errors aget alength alias all-ns alter alter-meta! alter-var-root amap
	ancestors and any? apply areduce array-map as-> aset aset-boolean aset-byte
	aset-char aset-double aset-float aset-int aset-long aset-short assert assoc
	assoc! assoc-in associative? atom await await-for await1 bases bean bigdec
	bigint biginteger binding bit-and bit-and-not bit-clear bit-flip bit-not
	bit-or bit-set bit-shift-left bit-shift-right bit-test bit-xor boolean
	boolean-array boolean? booleans bound-fn bound-fn* bound? bounded-count
	butlast byte byte-array bytes bytes? case cast cat char char-array
	char-escape-string char-name-string char? chars chunk chunk-append
	chunk-buffer chunk-cons chunk-first chunk-next chunk-rest chunked-seq? class
	class? clear-agent-errors clojure-version coll? comment commute comp
	comparator compare compare-and-set! compile complement completing concat
	cond cond-> cond->> condp conj conj! cons constantly construct-proxy
	contains? count counted? create-ns create-struct cycle dec dec' decimal?
	declare dedupe default-data-readers definline definterface defmacro
	defmethod defmulti defn defn- defonce defprotocol defrecord defstruct
	deftype delay delay? deliver denominator deref derive descendants
	destructure disj disj! dissoc dissoc! distinct distinct? doall dorun doseq
	dosync dotimes doto double double-array double? doubles drop drop-last
	drop-while eduction empty empty? ensure ensure-reduced enumeration-seq
	error-handler error-mode eval even? every-pred every? ex-cause ex-data
	ex-info ex-message extend extend-protocol extend-type extenders extends?
	false? ffirst file-seq filter filterv find find-keyword find-ns
	find-protocol-impl find-protocol-method find-var first flatten float
	float-array float? floats flush fn fn? fnext fnil for force format
	frequencies future future-call future-cancel future-cancelled? future-done?
	future? gen-class gen-interface gensym get get-in get-method get-proxy-class
	get-thread-bindings get-validator group-by halt-when hash hash-combine
	hash-map hash-ordered-coll hash-set hash-unordered-coll ident? identical?
	identity if-let if-not if-some ifn? import in-ns inc inc' indexed? infinite?
	init-proxy inst-ms inst-ms* inst? instance? int int-array int? integer?
	interleave intern interpose into into-array ints io! isa? iterate iteration
	iterator-seq juxt keep keep-indexed key keys keyword keyword? last lazy-cat
	lazy-seq let letfn line-seq list list* list? load load-file load-reader
	load-string loaded-libs locking long long-array longs loop macroexpand
	macroexpand-1 make-array make-hierarchy map map-entry? map-indexed map?
	mapcat mapv max max-key memfn memoize merge merge-with meta method-sig
	methods min min-key mix-collection-hash mod munge name namespace
	namespace-munge nat-int? neg-int? neg? newline next nfirst nil? nnext not
	not-any? not-empty not-every? not= ns ns-aliases ns-imports ns-interns
	ns-map ns-name ns-publics ns-refers ns-resolve ns-unalias ns-unmap nth
	nthnext nthrest num number? numerator object-array odd? or parents
	parse-boolean parse-double parse-long parse-uuid partial partition
	partition-all partition-by partitionv partitionv-all pcalls peek persistent!
	pmap pop pop! pop-thread-bindings pos-int? pos? pr pr-str prefer-method
	prefers primitives-classnames print print-ctor print-dup print-method
	print-simple print-str printf println println-str prn prn-str promise proxy
	proxy-call-with-super proxy-mappings proxy-name proxy-super
	push-thread-bindings pvalues qualified-ident? qualified-keyword?
	qualified-symbol? quot rand rand-int rand-nth random-sample random-uuid
	range ratio? rational? rationalize re-find re-groups re-matcher re-matches
	re-pattern re-seq read read+string read-line read-string reader-conditional
	reader-conditional? realized? record? reduce reduce-kv reduced reduced?
	reductions ref ref-history-count ref-max-history ref-min-history ref-set
	refer refer-clojure reify release-pending-sends rem remove
	remove-all-methods remove-method remove-ns remove-tap remove-watch repeat
	repeatedly replace replicate require requiring-resolve reset! reset-meta!
	reset-vals! resolve rest restart-agent resultset-seq reverse reversible?
	rseq rsubseq run! satisfies? second select-keys send send-off send-via seq
	seq-to-map-for-destructuring seq? seqable? seque sequence sequential? set
	set-agent-send-executor! set-agent-send-off-executor! set-error-handler!
	set-error-mode! set-validator! set? short short-array shorts shuffle
	shutdown-agents simple-ident? simple-keyword? simple-symbol? slurp some
	some-> some->> some-fn some? sort sort-by sorted-map sorted-map-by
	sorted-set sorted-set-by sorted? special-symbol? spit split-at split-with
	splitv-at str stream-into! stream-reduce! stream-seq! stream-transduce!
	string? struct struct-map subs subseq subvec supers swap! swap-vals! symbol
	symbol? sync tagged-literal tagged-literal? take take-last take-nth
	take-while tap> test the-ns thread-bound? time to-array to-array-2d
	trampoline transduce transient tree-seq true? type unchecked-add
	unchecked-add-int unchecked-byte unchecked-char unchecked-dec
	unchecked-dec-int unchecked-divide-int unchecked-double unchecked-float
	unchecked-inc unchecked-inc-int unchecked-int unchecked-long
	unchecked-multiply unchecked-multiply-int unchecked-negate
	unchecked-negate-int unchecked-remainder-int unchecked-short
	unchecked-subtract unchecked-subtract-int underive unquote unquote-splicing
	unreduced unsigned-bit-shift-right update update-in update-keys update-proxy
	update-vals uri? use uuid? val vals var-get var-set var? vary-meta vec
	vector vector-of vector? volatile! volatile? vreset! vswap! when when-first
	when-let when-not when-some while with-bindings with-bindings* with-in-str
	with-loading-context with-local-vars with-meta with-open with-out-str
	with-precision with-redefs with-redefs-fn xml-seq zero? zipmap`) {
		cljCore[w] = true
	}
}

// kebab is s with each underscore between two other characters a hyphen.
func kebab(s string) string {
	b := []byte(s)
	for i := 1; i+1 < len(b); i++ {
		if b[i] == '_' && b[i-1] != '_' && b[i+1] != '_' && b[i-1] != '-' {
			b[i] = '-'
		}
	}
	return string(b)
}

// predicates finds the functions that are questions: a bool result, and no
// store but to their own plain locals -- no global, no member, no element,
// nothing through a pointer -- and no call but to such functions, which a
// fixpoint decides (recursion included); a call through a pointer, or to
// the host, is a store for all it knows.
func (c *cgen) predicates() map[string]bool {
	type fn struct {
		fd    *cc.FunctionDefinition
		calls []string
		store bool
	}
	fns := map[string]*fn{}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef {
			continue
		}
		fd := ed.FunctionDefinition
		f := &fn{fd: fd}
		fns[fd.Declarator.Name()] = f
		plain := func(e cc.ExpressionNode) bool {
			p, ok := unparenE(e).(*cc.PrimaryExpression)
			if !ok || p.Case != cc.PrimaryExpressionIdent {
				return false
			}
			d, ok := p.ResolvedTo().(*cc.Declarator)
			if !ok || d.StorageDuration() != cc.Automatic || c.j.isBoxed(d) {
				return false
			}
			_, scalar := scalarKind(d.Type())
			return scalar || isPtrish(d.Type())
		}
		var walk func(n cc.Node)
		walk = func(n cc.Node) {
			if n == nil || f.store {
				return
			}
			switch x := n.(type) {
			case *cc.AssignmentExpression:
				if x.Case != cc.AssignmentExpressionCond && !plain(x.UnaryExpression) {
					f.store = true
					return
				}
			case *cc.UnaryExpression:
				if (x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec) && !plain(x.UnaryExpression) {
					f.store = true
					return
				}
			case *cc.PostfixExpression:
				switch x.Case {
				case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
					if !plain(x.PostfixExpression) {
						f.store = true
						return
					}
				case cc.PostfixExpressionCall:
					p, ok := unparenE(x.PostfixExpression).(*cc.PrimaryExpression)
					if !ok || p.Case != cc.PrimaryExpressionIdent {
						f.store = true // through a pointer
						return
					}
					d, ok := p.ResolvedTo().(*cc.Declarator)
					if !ok || !c.defined[d.Name()] {
						f.store = true // the host's, or unknown
						return
					}
					f.calls = append(f.calls, d.Name())
				}
			}
			walkChildrenFn(n, walk)
		}
		walk(fd.CompoundStatement)
	}
	for changed := true; changed; {
		changed = false
		for _, f := range fns {
			if f.store {
				continue
			}
			for _, callee := range f.calls {
				if g := fns[callee]; g == nil || g.store {
					f.store = true
					changed = true
					break
				}
			}
		}
	}
	out := map[string]bool{}
	for name, f := range fns {
		ft, ok := f.fd.Declarator.Type().(*cc.FunctionType)
		if ok && !f.store && ft.Result() != nil && ft.Result().Kind() == cc.Bool {
			out[name] = true
		}
	}
	return out
}
