package cut

// extable.go is the table half of the reform's D2b (doc/PIPELINE-REFORM.md
// §7): the Ex command table cut, on the seed, to the commands the product
// has, where phase 26 cut it after 79 phases had retired the rest.
//
// exfront has just pointed every command the product has not at ex_ni, so
// the stubs are exactly the rows to go.  Their ROWS go here; their
// ENUMERATORS stay, moved after CMD_SIZE, because live code still names a
// few of them -- window_layout_locked(CMD_close), ea.cmdidx = CMD_tabnew,
// the comparisons phases 2-25 fold -- and no parsed command can reach a
// value past CMD_SIZE.  Phase 26 deletes what is left of them.
//
// A row's name decided what every abbreviation of every other name meant: the
// lookup took the first row, in table order, that the typed word began.  So
// each surviving row carries its shortest abbreviation, computed from the
// 600-row table before a row is touched, in the field that held the name's
// length; a word names a command when it is a prefix at least that long.
// That makes a match unique and the prefix index pointless: cmdidxs1,
// cmdidxs2 and command_count go, and the lookup is a scan of the rows.  The
// old lookup and the new one are both modelled over every prefix of every
// name, and must agree wherever the old answer survives.

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

var extWord = regexp.MustCompile(`^[A-Za-z]+`)

const (
	extChars  = "@*!=><&~#}"
	extStubs  = 489
	extHead   = "        if (((unsigned)(eap->cmd[0]) - 'a' < 26))\n"
	extLookup = "        for (eap->cmdidx = (cmdidx_T)0; (int)eap->cmdidx < (int)CMD_SIZE; eap->cmdidx = (cmdidx_T)((int)eap->cmdidx + 1))\n        {\n            if (len >= cmdnames[(int)eap->cmdidx].cmd_minlen &&  strncmp((char *)(cmdnames[(int)eap->cmdidx].cmd_name), (char *)((char *)eap->cmd), ((usize)len))  == 0)\n            {\n                break;\n            }\n        }\n"
)

// ExTable cuts cmdnames[] to its live rows, each with its shortest
// abbreviation, and the lookup to a scan of them.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the table, the enum and the
// prefix index are read as forms -- each row its designator, its name's
// string and `sizeof` of it, its handler; the index's initialisers as
// numbers -- and the old lookup and the new one modelled over them as the
// text program modelled them (history keeps it).  Then the stub rows go
// (INITROW, no position to say: each row is designated), their
// enumerators move after CMD_SIZE (RENUM, renumbered), each live row's
// length is its shortest abbreviation (one FRAG unit), the member is
// retyped and renamed (RETYPE, RENAME) with its one reader's cast dropped,
// the one-character commands' string respelled, the lookup's two items a
// scan (FRAG), and the prefix index's three definitions deleted.
func ExTable(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("extable", e, w)
	die := func(f string, a ...any) error { return fmt.Errorf("extable: "+f, a...) }

	// ---- the table, the enum and the index, as they stand ---------------------
	_, rows, err := cmdRows(e)
	if err != nil {
		return die("%v", err)
	}
	var tableDef *graph.Node
	for _, d := range e.FileDecls("cmdnames") {
		if graph.TableInit(d) != nil {
			tableDef = d
		}
	}
	rowName, rowHandler := map[string]string{}, map[string]string{}
	rowOf := map[string]*graph.Node{}
	var rowOrder []string
	for _, r := range rows {
		in := cmdRowInit(r)
		id := ""
		if r.Is("at") && r.Args()[0].Is("idx") && len(r.Args()[0].Args()) == 1 {
			id = strings.TrimPrefix(r.Args()[0].Args()[0].Atom, "CMD_")
		}
		name := cmdRowName(r)
		ln := in.Args()[1]
		if id == "" || !ln.Is("-") || len(ln.Args()) != 2 || !ln.Args()[0].Is("sizeof") ||
			len(ln.Args()[0].Args()) != 1 || ln.Args()[0].Args()[0].Atom != `"`+name+`"` || ln.Args()[1].Atom != "1" ||
			in.Args()[2].IsList() {
			return die("a cmdnames[] row does not have the expected shape: %s", name)
		}
		rowName[id], rowHandler[id], rowOf[id] = name, in.Args()[2].Atom, r
		rowOrder = append(rowOrder, id)
	}
	enum, size := cmdEnum(e)
	if enum == nil || size < 0 {
		return die("enum CMD_index is not where it was")
	}
	var ids []string
	enumerators := map[string]*graph.Node{}
	for _, en := range enum.Args()[1:size] {
		n := graph.EnumeratorName(en)
		ids = append(ids, strings.TrimPrefix(n, "CMD_"))
		enumerators[ids[len(ids)-1]] = en
	}
	if strings.Join(ids, ",") != strings.Join(rowOrder, ",") {
		return die("the enum's %d names are not the %d rows, in order", len(ids), len(rowOrder))
	}
	names := make([]string, len(ids))
	handler := map[string]string{}
	for i, id := range ids {
		names[i] = rowName[id]
		handler[rowName[id]] = rowHandler[id]
	}
	var dead, live []string
	for _, n := range names {
		if handler[n] == "ex_ni" || handler[n] == "ex_script_ni" {
			dead = append(dead, n)
		} else {
			live = append(live, n)
		}
	}
	if len(dead) != extStubs {
		return die("%d stub rows, expected the %d exfront leaves", len(dead), extStubs)
	}
	idx1, ok1 := initInts(e, "cmdidxs1")
	idx2, ok2 := initInts(e, "cmdidxs2")
	count, ok3 := initInts(e, "command_count")
	if !ok1 || !ok2 || !ok3 {
		return die("the prefix index is not where it was")
	}
	if len(idx1) != 26 || len(idx2) != 676 || len(count) != 1 || count[0] != len(names) {
		return die("the prefix index has an unexpected shape")
	}

	// ---- the old lookup, the new one, and the proof ----------------------------
	isLower := func(c byte) bool { return c >= 'a' && c <= 'z' }
	isAlpha := func(c byte) bool { return isLower(c) || c >= 'A' && c <= 'Z' }
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	liveSet := map[string]bool{}
	for _, n := range live {
		liveSet[n] = true
	}
	newChars := ""
	for _, c := range extChars {
		if liveSet[string(c)] {
			newChars += string(c)
		}
	}
	startNext, startBang := edit.IndexOf(ids, "Next"), edit.IndexOf(ids, "bang")
	oldLookup := func(wd string) string {
		var start int
		switch c0 := wd[0]; {
		case isAlpha(c0):
			for i := 0; i < len(wd); i++ {
				if !isAlpha(wd[i]) && !isDigit(wd[i]) {
					return ""
				}
			}
			if isLower(c0) {
				start = idx1[int(c0)-'a']
				if len(wd) > 1 && isLower(wd[1]) {
					start += idx2[(int(c0)-'a')*26+int(wd[1])-'a']
				}
			} else {
				start = startNext
			}
		case strings.IndexByte(extChars, c0) >= 0:
			if len(wd) != 1 {
				return ""
			}
			start = startBang
		default:
			return ""
		}
		for _, n := range names[start:] {
			if strings.HasPrefix(n, wd) {
				return n
			}
		}
		return ""
	}
	minlen := map[string]int{}
	for _, n := range live {
		if oldLookup(n) != n {
			return die("%s does not resolve to itself in the old table", edit.PyRepr(n))
		}
		for i := 1; i <= len(n); i++ {
			if oldLookup(n[:i]) == n {
				minlen[n] = i
				break
			}
		}
	}
	newLookup := func(wd string) string {
		if !isAlpha(wd[0]) {
			if strings.IndexByte(newChars, wd[0]) < 0 || len(wd) != 1 {
				return ""
			}
		} else {
			wd = extWord.FindString(wd)
		}
		for _, n := range live {
			if len(wd) >= minlen[n] && strings.HasPrefix(n, wd) {
				return n
			}
		}
		return ""
	}
	wordset := map[string]bool{}
	for _, n := range names {
		for i := 1; i <= len(n); i++ {
			wordset[n[:i]] = true
		}
	}
	for _, c := range extChars + "{+-" {
		wordset[string(c)] = true
	}
	words := make([]string, 0, len(wordset))
	for k := range wordset {
		words = append(words, k)
	}
	sort.Strings(words)
	for _, wd := range words {
		o := oldLookup(wd)
		want := ""
		if _, ok := minlen[o]; ok {
			want = o
		}
		if got := newLookup(wd); got != want {
			return die("the new lookup answers %q for %q where the old answered %q", got, wd, want)
		}
		k := 0
		for _, n := range live {
			if len(wd) >= minlen[n] && strings.HasPrefix(n, wd) {
				k++
			}
		}
		if k > 1 {
			return die("%q matches more than one row", wd)
		}
	}
	v.Sayf("proved: all %d prefixes of the %d names resolve as before, each to at most one row",
		len(words), len(names))

	// ---- the table and the enum ------------------------------------------------
	if !enumAbove(e, enum, tableDef) {
		return die("the enum is not above the table")
	}
	var liveRows, deadEns []*graph.Node
	var lens []int
	for _, id := range ids {
		r := rowOf[id]
		if n, ok := minlen[rowName[id]]; ok {
			lens = append(lens, n)
			liveRows = append(liveRows, r)
		} else {
			deadEns = append(deadEns, enumerators[id])
		}
	}
	if _, err := e.MoveEnumerators(deadEns, enum.Args()[size], graph.Renumber); err != nil {
		return die("the stub rows' enumerators -- %v", err)
	}
	// the stub rows deleted once their enumerators say where the rows left
	// stand, so that the table is typed at its new length
	if _, err := e.ArrangeRowsTyped(tableDef, liveRows, graph.RowIndex{}); err != nil {
		return die("the stub rows -- %v", err)
	}
	var frags []graph.Frag
	for i, r := range liveRows {
		frags = append(frags, graph.Frag{At: e.SpotOf(cmdRowInit(r).Args()[1]), Src: strconv.Itoa(lens[i])})
	}
	if _, err := e.SpliceC(frags...); err != nil {
		return die("the rows' shortest abbreviations -- %v", err)
	}
	v.Sayf("%d rows kept, each with its shortest abbreviation; %d rows gone, their enumerators after CMD_SIZE",
		len(minlen), len(deadEns))

	// ---- the field, the index, the lookup, the one-character commands ----------
	field := cmdMember(e, "cmd_namelen")
	if field == nil {
		return die("the row field that held the name length holds the shortest abbreviation: 0 occurrences, expected 1")
	}
	q := graph.NewVerbs("extable", e, io.Discard)
	const reader = "(cast int (. (index cmdnames (-> eap cmdidx)) cmd_minlen))"
	if _, err := e.Retype(field, "int"); err != nil {
		return die("the row field -- %v", err)
	}
	if _, err := e.Rename(field, "cmd_minlen"); err != nil {
		return die("the row field -- %v", err)
	}
	if n := q.Count(reader); n != 1 {
		return die("and its one reader, in the Vim9 check phase 26 folds: %d occurrences, expected 1", n)
	}
	q.RewriteFunc(reader, 1, func(m *graph.Node, _ graph.Bindings) ([]*graph.Node, error) {
		return []*graph.Node{m.Args()[1]}, nil
	}, "the reader")
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("the row field that held the name length holds the shortest abbreviation")
	v.Say("and its one reader, in the Vim9 check phase 26 folds")
	q.InFunction("find_ex_command", func(q *graph.Verbs) {
		q.RespellString(strconv.Quote(extChars), strconv.Quote(newChars), 1, "the one-character commands that exist: "+newChars)
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("the one-character commands that exist: " + newChars)

	// the lookup span, asked of the function's C view as the text asked it
	var fnText string
	q.InFunction("find_ex_command", func(q *graph.Verbs) { fnText = string(q.Text()) })
	if err := q.Done(); err != nil {
		return err
	}
	if k := strings.Count(fnText, extHead); k != 1 {
		return die("the index lookup head occurs %d times", k)
	}
	a := strings.Index(fnText, extHead)
	loop := strings.Index(fnText[a:], "        for (; (int)eap->cmdidx < (int)CMD_SIZE;")
	if loop < 0 {
		return die("the lookup span does not contain its for loop")
	}
	loop += a
	b := edit.Blank([]byte(fnText))
	lb := strings.Index(string(b[loop:]), "{") + loop
	end := edit.Match(b, lb)
	z := strings.Index(fnText[end:], "\n") + end + 1
	span := fnText[a:z]
	for _, need := range []string{"cmdidxs1", "cmdidxs2", "command_count", "CMD_Next", "CMD_bang", "strncmp"} {
		if !strings.Contains(span, need) {
			return die("the lookup span does not contain %s -- it is not the block it was", need)
		}
	}
	if k := strings.Count(span, "\n"); k != 30 {
		return die("the lookup span is %d lines, expected 30", k)
	}
	q.InFunction("find_ex_command", func(q *graph.Verbs) {
		q.SpliceRunC("(if (paren (< (- (cast unsigned (paren (index (-> eap cmd) 0))) 'a') 26)) _*)",
			"(for () (< (cast int (-> eap cmdidx)) (cast int CMD_SIZE)) _*)", extLookup, "the lookup")
	})
	for _, n := range []string{"cmdidxs1", "cmdidxs2", "command_count"} {
		q.Cut("(def static "+n+" _*)", 1, n)
	}
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("the lookup: a prefix at least as long as the row says, over %d rows; the prefix index gone", len(minlen))
	return v.Done()
}

// cmdEnum is enum CMD_index's definition and CMD_SIZE's place among its
// elements (its tag the first), -1 when it has none.
func cmdEnum(e *graph.Editor) (*graph.Node, int) {
	for _, f := range e.Graph().Forms {
		if f.Is("enum") && graph.Tag(f) == "CMD_index" && graph.IsTypeDef(f) {
			for i, en := range f.Args() {
				if i > 0 && graph.EnumeratorName(en) == "CMD_SIZE" {
					return f, i
				}
			}
			return f, -1
		}
	}
	return nil, -1
}

// enumAbove says the enum's definition comes before the table's.
func enumAbove(e *graph.Editor, enum, table *graph.Node) bool {
	for _, f := range e.Graph().Forms {
		switch f {
		case enum:
			return true
		case table:
			return false
		}
	}
	return false
}

// cmdMember is struct cmdname's member name.
func cmdMember(e *graph.Editor, name string) *graph.Node {
	for _, f := range e.Graph().Forms {
		if f.Is("struct") && graph.Tag(f) == "cmdname" && graph.IsTypeDef(f) {
			for _, m := range graph.Members(f) {
				if len(m.Kids) > 0 && !m.Kids[0].IsList() && m.Kids[0].Atom == name {
					return m
				}
			}
		}
	}
	return nil
}

// initInts is the numbers a top-level object's initialiser holds, nested
// initialisers flattened, in order.
func initInts(e *graph.Editor, name string) ([]int, bool) {
	var def *graph.Node
	for _, d := range e.FileDecls(name) {
		if d.Is("def") && len(d.Args()) >= 4 {
			def = d
		}
	}
	if def == nil {
		return nil, false
	}
	var out []int
	ok := true
	var walk func(n *graph.Node)
	walk = func(n *graph.Node) {
		if n.Is("init") {
			for _, k := range n.Args() {
				walk(k)
			}
			return
		}
		x, err := strconv.Atoi(n.Atom)
		if n.IsList() || err != nil {
			ok = false
			return
		}
		out = append(out, x)
	}
	walk(def.Args()[len(def.Args())-1])
	return out, ok
}

// fileFrontCmds are the commands that read, write, edit or name a file:
// what the product has not of the 111 rows extable keeps (the reform's D4).
var fileFrontCmds = []string{"edit", "enew", "ex", "exit", "file", "read", "saveas",
	"update", "visual", "view", "write", "wq", "xit"}

// FileFront deletes the rows of the commands that name a file, as extable
// deleted the stubs': each enumerator moves after CMD_SIZE, where no parsed
// command reaches it, until the phases that remove its last uses.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the enumerators moved first
// (RENUM, renumbered: right after CMD_SIZE, after the user-command
// enumerators they would count up from -2 into CMD_USER's value), then the
// rows, each found by its designator and its name, deleted (INITROW), the
// table typed at its new length; B2b's TestRenumFileFront first proved it
// (history keeps the text version).
func FileFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("filefront", e, w)
	var tableDef *graph.Node
	for _, d := range e.FileDecls("cmdnames") {
		if graph.TableInit(d) != nil {
			tableDef = d
		}
	}
	if tableDef == nil {
		return fmt.Errorf("filefront: cmdnames[] is not where it was")
	}
	gone := map[*graph.Node]bool{}
	for _, n := range fileFrontCmds {
		k := 0
		for _, r := range graph.TableInit(tableDef).Args() {
			if r.Is("at") && r.Args()[0].Is("idx") && len(r.Args()[0].Args()) == 1 &&
				r.Args()[0].Args()[0].Atom == "CMD_"+n && cmdRowName(r) == n {
				gone[r] = true
				k++
			}
		}
		if k != 1 {
			return fmt.Errorf("filefront: %d rows for :%s, expected 1", k, n)
		}
	}
	enum, size := cmdEnum(e)
	if enum == nil {
		return fmt.Errorf("filefront: enum CMD_index is not where it was")
	}
	if size < 0 {
		return fmt.Errorf("filefront: CMD_SIZE is not in the enum once")
	}
	var ens []*graph.Node
	for _, n := range fileFrontCmds {
		var en *graph.Node
		k := 0
		for _, x := range enum.Args()[1:] {
			if graph.EnumeratorName(x) == "CMD_"+n {
				en = x
				k++
			}
		}
		if k != 1 {
			return fmt.Errorf("filefront: CMD_%s is not an enumerator once", n)
		}
		ens = append(ens, en)
	}
	if _, err := e.MoveEnumerators(ens, enum.Args()[size], graph.Renumber); err != nil {
		return fmt.Errorf("filefront: the enumerators -- %v", err)
	}
	var keep []*graph.Node
	for _, r := range graph.TableInit(tableDef).Args() {
		if !gone[r] {
			keep = append(keep, r)
		}
	}
	if _, err := e.ArrangeRowsTyped(tableDef, keep, graph.RowIndex{}); err != nil {
		return fmt.Errorf("filefront: the rows -- %v", err)
	}
	v.Sayf("%d rows gone, their enumerators after CMD_SIZE: %s", len(fileFrontCmds), strings.Join(fileFrontCmds, " "))
	return v.Done()
}
