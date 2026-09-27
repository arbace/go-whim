package togo

import (
	"strings"
	"testing"
)

// The rules, each on a defn: what the printer wrote, and what it becomes.
func TestCljTidy(t *testing.T) {
	for _, c := range [][2]string{
		// a step bound to _ is done; a let that binds nothing is its body
		{"(defn f [ed]\n  (let [_ (a ed)\n        _ (b ed)]\n    nil))", "(defn f [ed]\n  (a ed)\n  (b ed)\n  nil)"},
		// (if c x nil) is when; its nil, as a statement, goes
		{"(defn f [ed]\n  (do (if (c ed)\n        (let [_ (a ed)]\n          nil)\n        nil)\n    (b ed)))",
			"(defn f [ed]\n  (when (c ed)\n    (a ed))\n  (b ed))"},
		{"(defn f [ed x]\n  (if (c ed)\n    nil\n    x))", "(defn f [ed x]\n  (when-not (c ed)\n    x))"},
		// a let whose body is a let is one let; (let [x v] x) is v
		{"(defn f [ed]\n  (let [a (g ed)]\n    (let [b (h a)]\n      (+ a b))))", "(defn f [ed]\n  (let [a (g ed)\n        b (h a)]\n    (+ a b)))"},
		{"(defn f [ed n]\n  (let [n (if (< n 0)\n           (let [n 1]\n             n)\n           n)]\n    n))",
			"(defn f [ed n]\n  (if (< n 0)\n    1\n    n))"},
		// a hint moves onto a call, and stays on a let over a special form
		{"(defn f [ed]\n  (let [^BytePtr p (.get q)]\n    p))", "(defn f [ed]\n  ^BytePtr (.get q))"},
		{"(defn f [ed]\n  (.copy (let [^T h (if c a b)] h)))", "(defn f [ed]\n  (.copy (let [^T h (if c a b)] h)))"},
		{"(defn f [ed]\n  (let [^long n (g ed)]\n    n))", "(defn f [ed]\n  (let [^long n (g ed)]\n    n))"},
		// steps after the last name move into the body
		{"(defn f [ed]\n  (let [a (g ed)\n        _ (h a)]\n    a))", "(defn f [ed]\n  (let [a (g ed)]\n    (h a)\n    a))"},
		// and, or, arithmetic flattened; a conversion of a conversion
		{"(defn f [a b c]\n  (and (and a b) c))", "(defn f [a b c]\n  (and a b c))"},
		{"(defn f [a b c]\n  (+ (+ a b) c))", "(defn f [a b c]\n  (+ a b c))"},
		{"(defn f [a b c]\n  (- (- a b) c))", "(defn f [a b c]\n  (- (- a b) c))"},
		{"(defn f [x]\n  (unchecked-int (i32 x)))", "(defn f [x]\n  (unchecked-int x))"},
		{"(defn f [x]\n  (i32 (i8 x)))", "(defn f [x]\n  (i32 (i8 x)))"},
		// a conversion nothing reads is its argument
		{"(defn f [ed]\n  (let [_ (long (g ed))]\n    nil))", "(defn f [ed]\n  (g ed)\n  nil)"},
		// a machine's default, which no state reaches
		{"(defn f [st]\n  (case st\n    0\n      (a)\n    (throw (IllegalStateException. \"no state\"))))", "(defn f [st]\n  (case st\n    0\n      (a)))"},
		// a recur stays in the tail
		{"(defn f [n]\n  (loop [n n]\n    (if (> n 0)\n      (let [_ (a n)]\n        (recur (dec n)))\n      nil)))",
			"(defn f [n]\n  (loop [n n]\n    (when (> n 0)\n      (a n)\n      (recur (dec n)))))"},
		// the result's hint on the parameters
		{"(defn f ^BytePtr [ed]\n  (let [_ (a)]\n    (b)))", "(defn f ^BytePtr [ed]\n  (a)\n  (b))"},
		// what it cannot read, it leaves
		{"(defn f [ed]\n  (let [_ (a)] ; a comment\n    (b)))", "(defn f [ed]\n  (let [_ (a)] ; a comment\n    (b)))"},
	} {
		if got := strings.TrimSpace(cljTidyFile(c[0] + "\n")); got != c[1] {
			t.Errorf("cljTidyFile(%q)\n got %q\nwant %q", c[0], got, c[1])
		}
	}
}
