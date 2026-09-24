// Package ccx checks editor.c for what an automatic transpilation to Go needs
// to be able to assume, and reports every place it cannot.  Each check is a
// partition: every occurrence of a construct is put in a class the emitter has
// a faithful rule for, and a leftover is a finding.  internal/gen/pre runs them on a
// file; the pipeline's checks run them on a phase's output.
package ccx

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/arbace/go-whim/internal/cc"
)

// Parse reads a C file the way internal/gen does.
func Parse(path string) (*cc.AST, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	return cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path},
	})
}

// walk calls f on every node under n, in source order, with the function it
// is in.
func walk(n cc.Node, fn string, f func(n cc.Node, fn string)) {
	if n == nil {
		return
	}
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}
	if fd, ok := n.(*cc.FunctionDefinition); ok {
		fn = fd.Declarator.Name()
	}
	f(n, fn)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		if !v.Type().Field(i).IsExported() {
			continue
		}
		fv := v.Field(i)
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface) && !fv.IsNil() {
			if c, ok := fv.Interface().(cc.Node); ok {
				walk(c, fn, f)
			}
		}
	}
}

// walkDepth visits n and, while f returns true, its descendants.
func walkDepth(n cc.Node, f func(cc.Node) bool) {
	if n == nil {
		return
	}
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}
	if !f(n) {
		return
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		if !v.Type().Field(i).IsExported() {
			continue
		}
		fv := v.Field(i)
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface) && !fv.IsNil() {
			if c, ok := fv.Interface().(cc.Node); ok {
				walkDepth(c, f)
			}
		}
	}
}

func typeOf(n cc.Node) cc.Type {
	if e, ok := n.(cc.ExpressionNode); ok && e != nil && !reflect.ValueOf(e).IsNil() {
		return e.Type()
	}
	return nil
}

// Finding is one occurrence no class covers: its function, its position and
// what it is.
type Finding struct {
	Fn, Where, What string
}

// Result is a check's partition: how many occurrences each class holds, and
// what is left over.
type Result struct {
	Title   string
	Classes map[string]int
	Left    []Finding
}

// Print writes a result, and says whether nothing was left over.
func (r Result) Print(w interface{ Write([]byte) (int, error) }) bool {
	var ks []string
	for k := range r.Classes {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	fmt.Fprintf(w, "%s\n", r.Title)
	for _, k := range ks {
		fmt.Fprintf(w, "  %6d  %s\n", r.Classes[k], k)
	}
	fmt.Fprintf(w, "  %6d  LEFT OVER\n", len(r.Left))
	for _, l := range r.Left {
		fmt.Fprintf(w, "          %s: %s  -- %s\n", l.Where, l.Fn, l.What)
	}
	return len(r.Left) == 0
}
