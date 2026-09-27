package togo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// THE TABLES AS DATA (doc/CLOJURE-IDIOMS.md item 4).  A file-scope array of
// structs whose initializer is all constants -- numbers, booleans, strings,
// nulls, functions' names: the Unicode intervals, the case tables, the
// command tables -- was written as a store per member, (.set-first_ ^S_interval
// (aget (g ed t) 0) 768), a few thousand lines of it.  Such a table is now
// its rows, data the namespace holds once and every editor's objects are
// filled from: (def ^:private table-t (read-string "[[768 879] ...]")), and
// (fill-S_interval! (g ed t) table-t) at new-editor, a function written once
// per struct type that sets each column with its typed setter.  The values
// are the ones the stores wrote -- the same printer makes each -- and each
// editor still has its own objects; the rows are read from a string, as
// the enumerators are, so that no method of the namespace builds them.

// a column of a table: how a row's value becomes the member's
type ccol struct {
	set  string // the member's setter, (.set-m o v)
	kind byte   // 'l' a number, 'z' a boolean, 's' a string or nil, 'f' a function's name or nil, 0 not set
}

var (
	cljIntRe = regexp.MustCompile(`^-?\d+$`)
	cljLitRe = regexp.MustCompile(`^\(BytePtr/lit ("(?:[^"\\]|\\.)*")\)$`)
	cljSymRe = regexp.MustCompile(`^[a-zA-Z][A-Za-z0-9_?!*<>=-]*$`)
	cljEnRe  = regexp.MustCompile(`^\(e ([A-Za-z_][A-Za-z0-9_]*)\)$`)
)

// tableData writes p, a file-scope array of structs with initializer in,
// as a table's rows and a fill of p from them, and says whether it did: it
// does not when a value is anything but a constant a row can hold.
func (f *cfn) tableData(p cplace, x *cc.ArrayType, in *cc.Initializer) (ok bool) {
	st, isStruct := x.Elem().(*cc.StructType)
	if !isStruct || p.obj.s == "" {
		return false
	}
	fs := members(st)
	rows := map[int64][]string{}
	cols := make([]ccol, len(fs))
	var max int64 = -1
	defer func() {
		if r := recover(); r != nil {
			if _, is := r.(unsupported); !is {
				panic(r)
			}
			ok = false
		}
	}()
	i := int64(0)
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if d := f.designator(l); d != nil {
			if d.Case != cc.DesignatorIndex {
				return false
			}
			v, isInt := d.ConstantExpression.Value().(cc.Int64Value)
			if !isInt {
				return false
			}
			i = int64(v)
		}
		k := i
		i++
		if k > max {
			max = k
		}
		el := l.Initializer
		if zeroInit(el) {
			continue
		}
		if el.Case != cc.InitializerInitList {
			return false
		}
		row := make([]string, len(fs))
		for j := range row {
			row[j] = "nil"
		}
		n := 0
		for m := el.InitializerList; m != nil; m = m.InitializerList {
			if m.Designation != nil || n >= len(fs) || fs[n] == nil {
				return false
			}
			fl := fs[n]
			n++
			if zeroInit(m.Initializer) {
				continue
			}
			if m.Initializer.Case != cc.InitializerExpr || fl.IsBitfield() || f.c.j.boxedField[fieldKey(fl)] ||
				isAggr(fl.Type()) || fl.Type().Kind() == cc.Array {
				return false
			}
			key := fieldKey(fl)
			jt := f.jt(fl.Type(), key, "a member", fl.Name())
			var text string
			if steps := f.capture(func() {
				v := f.exprTo(m.Initializer.AssignmentExpression, jt)
				text = f.conv(v, jt, fl.Type())
			}); len(steps) > 0 {
				return false
			}
			var kind byte
			switch {
			case isIntJ(jt) && cljIntRe.MatchString(text):
				kind = 'l'
			case isIntJ(jt) && cljEnRe.MatchString(text):
				kind, text = 'l', cljEnRe.FindStringSubmatch(text)[1] // the enumerator's name: row-long reads it
			case jt == "boolean" && (text == "true" || text == "false"):
				kind = 'z'
			case jt == "boolean" && cljIntRe.MatchString(text):
				kind, text = 'z', fmt.Sprint(text != "0")
			case jt == "BytePtr" && cljLitRe.MatchString(text):
				kind, text = 's', cljLitRe.FindStringSubmatch(text)[1]
			case text == "nil" && jt == "BytePtr":
				kind = 's'
			case cljSymRe.MatchString(text) && f.c.fnValueName(text):
				kind = 'f'
			default:
				return false
			}
			if cols[n-1].kind != 0 && cols[n-1].kind != kind {
				return false
			}
			cols[n-1] = ccol{set: f.c.memberName(fl), kind: kind}
			row[n-1] = text
		}
		rows[k] = row
	}
	if len(rows) == 0 {
		return false
	}
	// a member a row leaves zero is its column's zero
	for _, r := range rows {
		for j, c := range cols {
			if r[j] != "nil" {
				continue
			}
			switch c.kind {
			case 'l':
				r[j] = "0"
			case 'z':
				r[j] = "false"
			}
		}
	}
	// the rows, and the columns set: those some row gives a value
	last := 0
	for j, c := range cols {
		if c.kind != 0 {
			last = j + 1
		}
	}
	var lines []string
	var line []string
	for k := int64(0); k <= max; k++ {
		r, has := rows[k]
		cell := "nil"
		if has {
			var vs []string
			for j := 0; j < last; j++ {
				vs = append(vs, r[j])
			}
			cell = "[" + strings.Join(vs, " ") + "]"
		}
		line = append(line, cell)
		if len(line) == 8 {
			lines = append(lines, strings.Join(line, " "))
			line = nil
		}
	}
	if len(line) > 0 {
		lines = append(lines, strings.Join(line, " "))
	}
	elem := elemJ(p.obj.t)
	filler := f.c.filler(elem, cols[:last])
	name := "table-" + strings.TrimPrefix(strings.TrimPrefix(p.obj.s, "(g ed "), "(")
	name = strings.TrimSuffix(name, ")")
	data := "[" + strings.Join(lines, "\n   ") + "]"
	f.c.tables = append(f.c.tables, fmt.Sprintf("(def ^:private %s\n  (read-string \"%s\"))\n", name, cljDataQuote(data)))
	f.effectOf("(" + filler + " " + tagged(p.obj) + " " + name + ")")
	return true
}

// cljDataQuote is data as the contents of a Clojure string literal: its
// backslashes and quotes escaped, its newlines kept.
func cljDataQuote(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// fnValueName says s is a function of the namespace written as a value.
func (c *cgen) fnValueName(s string) bool {
	for name := range c.defined {
		if c.fnName(name) == s {
			return true
		}
	}
	return false
}

// filler is the name of the function that fills an array of struct class
// elem from a table's rows, column by column -- written once per class and
// set of columns.
func (c *cgen) filler(elem string, cols []ccol) string {
	var sig []string
	for _, cl := range cols {
		sig = append(sig, cl.set+":"+string(rune('0'+cl.kind)))
	}
	key := elem + "|" + strings.Join(sig, ",")
	if n, ok := c.fillerNames[key]; ok {
		return n
	}
	n := "fill-" + elem + "!"
	for i := 2; c.fillerUsed[n]; i++ {
		n = fmt.Sprintf("fill-%s-%d!", elem, i)
	}
	c.fillerNames[key] = n
	c.fillerUsed[n] = true
	var sets []string
	for j, cl := range cols {
		v := fmt.Sprintf("(nth r %d)", j)
		switch cl.kind {
		case 0:
			continue
		case 'l':
			v = "(row-long " + v + ")"
		case 'z':
			v = "(boolean " + v + ")"
		case 's':
			v = "(when-some [s " + v + "] (BytePtr/lit s))"
		case 'f':
			v = "(when-some [s " + v + "] @(ns-resolve '" + c.ns + " s))"
		}
		sets = append(sets, fmt.Sprintf("(.set-%s o %s)", cl.set, v))
	}
	c.fillers = append(c.fillers, fmt.Sprintf(`(defn- %s
  "Fill the %s objects of a from rows, a table's: one row an element, nil one left zero."
  [^objects a rows]
  (loop [i 0
         rows (seq rows)]
    (when rows
      (when-some [r (first rows)]
        (let [^%s o (aget a i)]
          %s))
      (recur (inc i) (next rows)))))
`, n, elem, elem, strings.Join(sets, "\n          ")))
	return n
}

// tablesText is the fillers and the tables, before the functions that use them.
func (c *cgen) tablesText() string {
	if len(c.tables) == 0 {
		return ""
	}
	fs := append([]string{}, c.fillers...)
	sort.Strings(fs)
	return ";; The tables the file-scope arrays of structs start with, as data (clj_tables.go).\n\n" +
		"(defn- row-long\n  \"A number of a table's row: itself, or an enumerator's by its name.\"\n  ^long [v]\n  (if (symbol? v) (long (get enumerators v)) (long v)))\n\n" +
		strings.Join(fs, "\n") + "\n" + strings.Join(c.tables, "\n") + "\n"
}
