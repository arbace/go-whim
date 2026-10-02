package sweep

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// TestWalkGen holds the generated walks (walk_gen.go) to reflection's: the
// same fields of every node type, and on whole parses -- the module's C test
// files, and $WHIM_C when it is set -- the same nodes and the same tokens in
// the same order.
func TestWalkGen(t *testing.T) {
	files, _ := filepath.Glob("../*/testdata/*.c")
	if f := os.Getenv("WHIM_C"); f != "" {
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no C files to parse")
	}
	seen := map[reflect.Type]bool{}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := cc.NewConfig("linux", "amd64")
		if err != nil {
			t.Fatal(err)
		}
		ast, err := cc.Parse(cfg, []cc.Source{
			{Name: "<predefined>", Value: cfg.Predefined},
			{Name: "<builtin>", Value: cc.Builtin},
			{Name: file, Value: src},
		})
		if err != nil {
			t.Logf("%s: %v (skipped)", file, err)
			continue
		}
		var want, got []cc.Node
		walkReflectOnly(ast.TranslationUnit, func(n cc.Node) bool {
			want = append(want, n)
			seen[reflect.TypeOf(n)] = true
			return true
		})
		walk(ast.TranslationUnit, func(n cc.Node) bool { got = append(got, n); return true })
		if len(got) != len(want) {
			t.Fatalf("%s: walk visits %d nodes, reflection %d", file, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: node %d is %T, reflection's %T", file, i, got[i], want[i])
			}
		}
		// Pruned the same way: no descent below every third node.
		var wp, gp []cc.Node
		k := 0
		walkReflectOnly(ast.TranslationUnit, func(n cc.Node) bool { wp = append(wp, n); k++; return k%3 != 0 })
		k = 0
		walk(ast.TranslationUnit, func(n cc.Node) bool { gp = append(gp, n); k++; return k%3 != 0 })
		if len(wp) != len(gp) {
			t.Fatalf("%s: pruned walk visits %d nodes, reflection %d", file, len(gp), len(wp))
		}
		for i := range gp {
			if gp[i] != wp[i] {
				t.Fatalf("%s: pruned walk: node %d differs", file, i)
			}
		}
		var wt, gt []cc.Token
		walkTokReflectOnly(ast.TranslationUnit, func(tk cc.Token) { wt = append(wt, tk) })
		walkTok(ast.TranslationUnit, func(tk cc.Token) { gt = append(gt, tk) })
		if len(wt) != len(gt) {
			t.Fatalf("%s: walkTok visits %d tokens, reflection %d", file, len(gt), len(wt))
		}
		for i := range wt {
			if wt[i] != gt[i] {
				t.Fatalf("%s: token %d differs", file, i)
			}
		}
	}
	for typ := range seen {
		e := typ
		if e.Kind() == reflect.Ptr {
			e = e.Elem()
		}
		var fs []string
		for _, fa := range fieldsOf(e) {
			k := "p"
			switch {
			case fa.token:
				k = "t"
			case e.Field(fa.i).Type.Kind() == reflect.Interface:
				k = "i"
			}
			fs = append(fs, e.Field(fa.i).Name+":"+k)
		}
		g, ok := genFields[e.Name()]
		if !ok {
			t.Errorf("%s: not generated (go generate ./sweep)", e.Name())
			continue
		}
		if !reflect.DeepEqual(append([]string{}, g...), append([]string{}, fs...)) {
			t.Errorf("%s: generated %v, reflection %v", e.Name(), g, fs)
		}
	}
}

// walkReflectOnly and walkTokReflectOnly are the reflective walks all the way
// down, never entering the generated ones.
func walkReflectOnly(n cc.Node, f func(cc.Node) bool) {
	v := reflect.ValueOf(n)
	if n == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return
	}
	if !f(n) {
		return
	}
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		if fa.token {
			continue
		}
		if x, ok := e.Field(fa.i).Interface().(cc.Node); ok {
			walkReflectOnly(x, f)
		}
	}
}

func walkTokReflectOnly(n cc.Node, f func(cc.Token)) {
	v := reflect.ValueOf(n)
	if n == nil || v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		fv := e.Field(fa.i)
		if fa.token {
			f(fv.Interface().(cc.Token))
			continue
		}
		if x, ok := fv.Interface().(cc.Node); ok {
			walkTokReflectOnly(x, f)
		}
	}
}
