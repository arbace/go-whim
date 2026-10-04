package graph

import "testing"

func TestDeleteRowsAsWritten(t *testing.T) {
	const src = `static int t[] = {10, 20, 30, 40};
static int three[] = {1, 2, 3};
int f(void) { return t[0] + *&t[1] + three[0]; }
`
	e := editorOn(t, src)
	// DeleteRows refuses: t[0] names a row that goes
	if _, err := e.DeleteRows(tableOf(t, e, "t"), rowsAt(t, e, "t", 0), RowIndex{}); err == nil {
		t.Fatal("DeleteRows took t[0]'s row")
	}
	tt := tableOf(t, e, "t")
	if err := e.DeleteRowsAsWritten(tt, rowsAt(t, e, "t", 0)); err != nil {
		t.Fatal(err)
	}
	if tt.Type == nil || tt.Type != tableOf(t, e, "three").Type || len(e.Untyped) != 0 {
		t.Errorf("the table's type %v; untyped %d", tt.Type, len(e.Untyped))
	}
	isC(t, e, `static int t[] = {20, 30, 40};
static int three[] = {1, 2, 3};
int f(void) { return t[0] + *&t[1] + three[0]; }
`)
	// a count the graph holds no type for: made and interned
	e = editorOn(t, src)
	tt = tableOf(t, e, "t")
	if err := e.DeleteRowsAsWritten(tt, rowsAt(t, e, "t", 0, 1)); err != nil {
		t.Fatal(err)
	}
	if tt.Type == nil || len(e.Untyped) != 0 {
		t.Errorf("the table's type %v; untyped %d", tt.Type, len(e.Untyped))
	}
	isC(t, e, `static int t[] = {30, 40};
static int three[] = {1, 2, 3};
int f(void) { return t[0] + *&t[1] + three[0]; }
`)
	// not a row of it
	e = editorOn(t, src)
	if err := e.DeleteRowsAsWritten(tableOf(t, e, "t"), rowsAt(t, e, "three", 0)); err == nil {
		t.Error("a row of another table was taken")
	}
}
