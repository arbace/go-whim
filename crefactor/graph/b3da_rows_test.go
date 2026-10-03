package graph

import "testing"

const b3daTable = `typedef struct { long a; int b; } S;
static S t[] = {{1, 2}, {3, 4}};
int f(void) { return (int)sizeof(t) + t[1].b; }
`

// ArrangeRowsTyped against ArrangeRows: a length the graph has no array
// type for, and new rows with signed constants, are typed as an import of
// the result types them.
func TestB3daArrangeRowsTyped(t *testing.T) {
	const want = `typedef struct { long a; int b; } S;
static S t[] = {{1, 2}, {5, -1}, {-5000000000, -7}, {3, 4}};
int f(void) { return (int)sizeof(t) + t[3].b; }
`
	for _, typed := range []bool{false, true} {
		e := editorOn(t, b3daTable)
		tt := tableOf(t, e, "t")
		rows, err := e.BuildRows(tt, "(init 5 (- 1)) (init (- 5000000000) (- 7))", nil)
		if err != nil {
			t.Fatal(err)
		}
		old := TableInit(tt).Args()
		order := []*Node{old[0], rows[0], rows[1], old[1]}
		if typed {
			_, err = e.ArrangeRowsTyped(tt, order, RowIndex{})
		} else {
			_, err = e.ArrangeRows(tt, order, RowIndex{})
		}
		if err != nil {
			t.Fatal(err)
		}
		isC(t, e, want)
		if !typed {
			if len(e.Untyped) == 0 {
				t.Error("ArrangeRows typed the table and its rows: the test shows nothing")
			}
			continue
		}
		if len(e.Untyped) != 0 {
			t.Errorf("untyped: %d", len(e.Untyped))
		}
		_, _, g := importSample(t, want)
		if err := SameGraph(e.Graph(), g); err != nil {
			t.Error(err)
		}
	}
}
