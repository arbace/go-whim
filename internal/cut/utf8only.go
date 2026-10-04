package cut

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// utf8Flags are the five encoding flags and the constant each becomes.
//
// A SLICE, not a map: the order is the report's and the refusals'.
var utf8Flags = []struct {
	name string
	mark graph.Mark
}{
	{"enc_utf8", graph.MarkTrue}, {"has_mbyte", graph.MarkTrue}, {"enc_latin1like", graph.MarkTrue},
	{"enc_dbcs", graph.MarkZero}, {"enc_unicode", graph.MarkZero},
}

// Utf8Only leaves no test of the encoding to answer.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, *R1 as built*): the five flags'
// declarations and mb_init()'s assignments go; every use becomes the
// constant it always is -- TRUE, or 0 for the two that are never set --
// marked; and each function that holds one is folded by crefactor/graph's
// MARKFOLD, which gives form for form what the text simplifier this was
// (B4's DRAFT, a rewrite a round over marker tokens) gave: the groups a
// constant stood in simplified through `||`, `&&`, `!`, `?:`, parentheses
// and comparisons with zero, and the ifs, else-if arms and whiles left on
// a constant folded -- in the text's order, so that its counts are the
// text's rounds: a group or a right side each time it changed, a statement
// each time one was folded.
func Utf8Only(e *graph.Editor, w io.Writer) error {
	var decls []*graph.Node
	for _, f := range utf8Flags {
		var ds []*graph.Node
		for _, d := range e.FileDecls(f.name) {
			if d.Is("def") {
				ds = append(ds, d)
			}
		}
		if len(ds) != 1 || !graph.HasStorage(ds[0], "static") || graph.DeclInit(ds[0]) == nil {
			return fmt.Errorf("utf8only: %s is not declared once, static, with a value", f.name)
		}
		decls = append(decls, ds[0])
	}
	mb := e.Defn("mb_init")
	if mb == nil {
		return fmt.Errorf("utf8only: mb_init is not defined at file scope")
	}
	var assigns []*graph.Node
	graph.Walk(mb, func(x *graph.Node) bool {
		if x.Is("=") && len(x.Kids) == 3 && !x.Kids[1].IsList() {
			if p := e.Parent(x); p != nil && (p.Is("block") || p.Is("defn")) {
				for _, d := range decls {
					if x.Kids[1].Ref() == d {
						assigns = append(assigns, x)
					}
				}
			}
		}
		return true
	})
	if len(assigns) != 5 {
		return fmt.Errorf("utf8only: expected five assignments in mb_init, found %d", len(assigns))
	}
	for _, a := range assigns {
		if err := e.Delete(a); err != nil {
			return fmt.Errorf("utf8only: %v", err)
		}
	}
	fmt.Fprintln(w, "  utf8only     the five flags lose their declarations and mb_init() "+
		"its assignments")

	// every use the constant it is, marked: TRUE and FALSE built where the
	// first use stands
	var t, f *graph.Node
	for _, d := range decls {
		for _, u := range e.Uses(d) {
			if u.IsList() {
				return fmt.Errorf("utf8only: %s is named inside %s, which spells it as text", graph.DeclName(d), u.Head())
			}
			if t == nil {
				ns, err := e.Build(u, "TRUE FALSE", nil)
				if err != nil {
					return fmt.Errorf("utf8only: %v", err)
				}
				t, f = ns[0], ns[1]
			}
		}
	}
	if t == nil {
		return fmt.Errorf("utf8only: the five flags are not used")
	}
	mf := &graph.MarkFold{
		Marks: map[*graph.Node]graph.Mark{},
		Make: func(k graph.Mark) *graph.Node {
			switch k {
			case graph.MarkTrue:
				return graph.Clone(t)
			case graph.MarkFalse:
				return graph.Clone(f)
			}
			return graph.Literal(0)
		},
		NonZero: func(a string) bool { return len(a) > 5 && a[:5] == "DBCS_" },
	}
	marks := 0
	for k, d := range decls {
		for _, u := range e.Uses(d) {
			n := mf.Make(utf8Flags[k].mark)
			if err := e.Replace(u, n); err != nil {
				return fmt.Errorf("utf8only: %v", err)
			}
			mf.Marks[n] = utf8Flags[k].mark
			marks++
		}
		if err := e.Delete(d); err != nil {
			return fmt.Errorf("utf8only: %v", err)
		}
	}
	fmt.Fprintf(w, "  utf8only     %d mentions become constant markers\n", marks)

	// the functions holding one, in the file's order
	var fns []*graph.Node
	for _, form := range e.Graph().Forms {
		if !form.Is("defn") {
			continue
		}
		has := false
		graph.Walk(form, func(x *graph.Node) bool {
			if _, ok := mf.Marks[x]; ok {
				has = true
			}
			return !has
		})
		if has {
			fns = append(fns, form)
		}
	}
	for _, fn := range fns {
		if err := e.FoldMarks(mf, fn); err != nil {
			return fmt.Errorf("utf8only: %v", err)
		}
	}
	fmt.Fprintf(w, "  utf8only     %d expression simplifications and %d statement folds "+
		"in %d functions\n", mf.Exprs, mf.Folds, len(fns))
	if err := labelsBeforeDecls(e); err != nil {
		return err
	}

	left := 0
	for n := range mf.Marks {
		if e.Live(n) {
			left++
		}
	}
	fmt.Fprintf(w, "  utf8only     %d constants left where they are an operand or a value, "+
		"written as TRUE, FALSE or 0\n", left)
	for _, fl := range utf8Flags {
		if len(e.FileDecls(fl.name)) > 0 {
			return fmt.Errorf("utf8only: %s is still named", fl.name)
		}
	}
	fmt.Fprintln(w, "  utf8only     no test of the encoding is left to answer")
	return nil
}

// labelsBeforeDecls gives every label or case the folds left right before a
// declaration the null statement the text's canonical print puts there
// (`case X:` then `;`), as the import of that text has it: `(case X)
// (empty)`.
func labelsBeforeDecls(e *graph.Editor) error {
	var at []*graph.Node
	for _, f := range e.Graph().Forms {
		if !f.Is("defn") {
			continue
		}
		graph.Walk(f, func(n *graph.Node) bool {
			for i := 0; i+1 < len(n.Kids); i++ {
				k := n.Kids[i]
				if (k.Is("label") || k.Is("case") || k.Is("default")) && n.Kids[i+1].Is("def") && e.Live(k) {
					at = append(at, k)
				}
			}
			return true
		})
	}
	for _, k := range at {
		if err := e.InsertAfter(k, graph.NewList(graph.NewAtom("empty"))); err != nil {
			return err
		}
	}
	return nil
}
