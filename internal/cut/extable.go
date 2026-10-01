package cut

// extable.go is the table half of the reform's D2b (doc/PIPELINE-REFORM.md
// §7): the Ex command table cut, on the seed, to the commands the product
// has, where phase 80 cut it after 79 phases had retired the rest.
//
// exfront has just pointed every command the product has not at ex_ni, so
// the stubs are exactly the rows to go.  Their ROWS go here; their
// ENUMERATORS stay, moved after CMD_SIZE, because live code still names a
// few of them -- window_layout_locked(CMD_close), ea.cmdidx = CMD_tabnew,
// the comparisons phases 2-79 fold -- and no parsed command can reach a
// value past CMD_SIZE.  Phase 80 deletes what is left of them.
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
)

var (
	extTable  = regexp.MustCompile(`(?ms)^static struct cmdname cmdnames\[\] =\n\{\n(.*?)^\};\n`)
	extRow    = regexp.MustCompile(`^    \[CMD_(\w+)\] = \{\(char_u \*\)"([^"]*)", sizeof\("([^"]*)"\) - 1, *(\w+) *, \(long_u\)\(.*\), ADDR_\w+\},$`)
	extEnum   = regexp.MustCompile(`(?ms)^enum CMD_index\n\{\n(.*?)^    CMD_SIZE,\n`)
	extID     = regexp.MustCompile(`(?m)^    CMD_(\w+),$`)
	extIdx1   = regexp.MustCompile(`(?s)static const unsigned short cmdidxs1\[26\] =\n\{\n(.*?)\};`)
	extIdx2   = regexp.MustCompile(`(?s)static const unsigned char cmdidxs2\[26\]\[26\] =\n\{\n(.*?)\n\};`)
	extCount  = regexp.MustCompile(`static const int command_count = (\d+);`)
	extBanner = regexp.MustCompile(`(?ms)^static const unsigned short cmdidxs1\[26\] =\n.*?^static const int command_count = \d+;\n`)
	extNum    = regexp.MustCompile(`\d+`)
	extWord   = regexp.MustCompile(`^[A-Za-z]+`)
)

const (
	extChars   = "@*!=><&~#}"
	extStubs   = 489
	extHead    = "        if (((unsigned)(eap->cmd[0]) - 'a' < 26))\n"
	extLookup  = "        for (eap->cmdidx = (cmdidx_T)0; (int)eap->cmdidx < (int)CMD_SIZE; eap->cmdidx = (cmdidx_T)((int)eap->cmdidx + 1))\n        {\n            if (len >= cmdnames[(int)eap->cmdidx].cmd_minlen &&  strncmp((char *)(cmdnames[(int)eap->cmdidx].cmd_name), (char *)((char *)eap->cmd), ((size_t)len))  == 0)\n            {\n                break;\n            }\n        }\n"
	extField   = "    size_t cmd_namelen;\n"
	extMinlen  = "    int cmd_minlen;\n"
	extNameLen = "(int)cmdnames[eap->cmdidx].cmd_namelen"
)

// ExTable cuts cmdnames[] to its live rows, each with its shortest
// abbreviation, and the lookup to a scan of them.
func ExTable(text []byte, w io.Writer) ([]byte, error) {
	t := string(text)
	die := func(f string, a ...any) ([]byte, error) { return nil, fmt.Errorf("extable: "+f, a...) }

	// ---- the table, the enum and the index, as they stand ---------------------
	mt := extTable.FindStringSubmatchIndex(t)
	if mt == nil {
		return die("cmdnames[] is not where it was")
	}
	tabStart, tabEnd := mt[2], mt[3]
	rowName, rowHandler := map[string]string{}, map[string]string{}
	var rowOrder []string
	for _, line := range strings.Split(t[tabStart:tabEnd], "\n") {
		if line == "" {
			continue
		}
		r := extRow.FindStringSubmatch(line)
		if r == nil || r[2] != r[3] {
			return die("a cmdnames[] row does not have the expected shape: %s", edit.PyRepr(edit.CoreHead(line, 90)))
		}
		rowName[r[1]], rowHandler[r[1]] = r[2], r[4]
		rowOrder = append(rowOrder, r[1])
	}
	me := extEnum.FindStringSubmatchIndex(t)
	if me == nil {
		return die("enum CMD_index is not where it was")
	}
	enumStart, enumEnd := me[2], me[3]
	var ids []string
	for _, m := range extID.FindAllStringSubmatch(t[enumStart:enumEnd], -1) {
		ids = append(ids, m[1])
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
	m1, m2, mc := extIdx1.FindStringSubmatch(t), extIdx2.FindStringSubmatch(t), extCount.FindStringSubmatch(t)
	if m1 == nil || m2 == nil || mc == nil {
		return die("the prefix index is not where it was")
	}
	ints := func(s string) []int {
		var out []int
		for _, x := range extNum.FindAllString(s, -1) {
			n, _ := strconv.Atoi(x)
			out = append(out, n)
		}
		return out
	}
	idx1, idx2 := ints(m1[1]), ints(m2[1])
	if len(idx1) != 26 || len(idx2) != 676 || mc[1] != strconv.Itoa(len(names)) {
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
	fmt.Fprintf(w, "  extable      proved: all %d prefixes of the %d names resolve as before, each to at most one row\n",
		len(words), len(names))

	// ---- the table and the enum ------------------------------------------------
	var body []string
	for _, line := range strings.Split(t[tabStart:tabEnd], "\n") {
		if line == "" {
			body = append(body, "")
			continue
		}
		r := extRow.FindStringSubmatch(line)
		if n, ok := minlen[r[2]]; ok {
			body = append(body, strings.Replace(line, fmt.Sprintf("sizeof(%q) - 1", r[2]), strconv.Itoa(n), 1))
		}
	}
	var liveIDs, deadIDs []string
	for _, line := range strings.Split(t[enumStart:enumEnd], "\n") {
		r := extID.FindStringSubmatch(line)
		if r == nil {
			continue
		}
		if _, ok := minlen[rowName[r[1]]]; ok {
			liveIDs = append(liveIDs, line)
		} else {
			deadIDs = append(deadIDs, line)
		}
	}
	if enumEnd >= tabStart {
		return die("the enum is not above the table")
	}
	// the dead enumerators after CMD_SIZE: still named, never a row
	enumText := strings.Join(liveIDs, "\n") + "\n    CMD_SIZE,\n" + strings.Join(deadIDs, "\n") + "\n"
	t = t[:enumStart] + enumText + t[enumEnd+len("    CMD_SIZE,\n"):tabStart] + strings.Join(body, "\n") + t[tabEnd:]
	fmt.Fprintf(w, "  extable      %d rows kept, each with its shortest abbreviation; %d rows gone, their enumerators after CMD_SIZE\n",
		len(minlen), len(deadIDs))

	// ---- the field, the index, the lookup, the one-character commands ----------
	for _, r := range []struct{ old, new, what string }{
		{extField, extMinlen, "the row field that held the name length holds the shortest abbreviation"},
		{extNameLen, "cmdnames[eap->cmdidx].cmd_minlen", "and its one reader, in the Vim9 check phase 80 folds"},
		{fmt.Sprintf(`vim_strchr((char_u *)"%s", *p)`, extChars), fmt.Sprintf(`vim_strchr((char_u *)"%s", *p)`, newChars),
			"the one-character commands that exist: " + newChars},
	} {
		if n := strings.Count(t, r.old); n != 1 {
			return die("%s: %d occurrences, expected 1", r.what, n)
		}
		t = strings.Replace(t, r.old, r.new, 1)
		fmt.Fprintf(w, "  extable      %s\n", r.what)
	}
	loc := extBanner.FindStringIndex(t)
	if loc == nil {
		return die("the prefix index is gone")
	}
	t = t[:loc[0]] + t[loc[1]:]
	if k := strings.Count(t, extHead); k != 1 {
		return die("the index lookup head occurs %d times", k)
	}
	a := strings.Index(t, extHead)
	loop := strings.Index(t[a:], "        for (; (int)eap->cmdidx < (int)CMD_SIZE;")
	if loop < 0 {
		return die("the lookup span does not contain its for loop")
	}
	loop += a
	b := edit.Blank([]byte(t))
	lb := strings.Index(string(b[loop:]), "{") + loop
	end := edit.Match(b, lb)
	z := strings.Index(t[end:], "\n") + end + 1
	span := t[a:z]
	for _, need := range []string{"cmdidxs1", "cmdidxs2", "command_count", "CMD_Next", "CMD_bang", "strncmp"} {
		if !strings.Contains(span, need) {
			return die("the lookup span does not contain %s -- it is not the block it was", need)
		}
	}
	if k := strings.Count(span, "\n"); k != 30 {
		return die("the lookup span is %d lines, expected 30", k)
	}
	t = t[:a] + extLookup + t[z:]
	fmt.Fprintf(w, "  extable      the lookup: a prefix at least as long as the row says, over %d rows; the prefix index gone\n", len(minlen))
	return []byte(t), nil
}

// fileFrontCmds are the commands that read, write, edit or name a file:
// what the product has not of the 111 rows extable keeps (the reform's D4).
var fileFrontCmds = []string{"edit", "enew", "ex", "exit", "file", "read", "saveas",
	"update", "visual", "view", "write", "wq", "xit"}

// FileFront deletes the rows of the commands that name a file, as extable
// deleted the stubs': each enumerator moves after CMD_SIZE, where no parsed
// command reaches it, until the phases that remove its last uses.
func FileFront(text []byte, w io.Writer) ([]byte, error) {
	t := string(text)
	mt := extTable.FindStringSubmatchIndex(t)
	if mt == nil {
		return nil, fmt.Errorf("filefront: cmdnames[] is not where it was")
	}
	table := t[mt[2]:mt[3]]
	for _, n := range fileFrontCmds {
		re := regexp.MustCompile(`(?m)^    \[CMD_` + regexp.QuoteMeta(n) + `\] = \{\(char_u \*\)"` + regexp.QuoteMeta(n) + `", \d+, .*\n`)
		if k := len(re.FindAllString(table, -1)); k != 1 {
			return nil, fmt.Errorf("filefront: %d rows for :%s, expected 1", k, n)
		}
		table = re.ReplaceAllString(table, "")
	}
	t = t[:mt[2]] + table + t[mt[3]:]
	// the enumerators: out of the live run, onto the end of the dead one
	es := strings.Index(t, "enum CMD_index\n{\n")
	if es < 0 {
		return nil, fmt.Errorf("filefront: enum CMD_index is not where it was")
	}
	ee := strings.Index(t[es:], "\n};\n") + es + 1
	enum := t[es:ee]
	var moved []string
	for _, n := range fileFrontCmds {
		line := "    CMD_" + n + ",\n"
		if strings.Count(enum, line) != 1 {
			return nil, fmt.Errorf("filefront: CMD_%s is not an enumerator once", n)
		}
		enum = strings.Replace(enum, line, "", 1)
		moved = append(moved, line)
	}
	// right after CMD_SIZE: after the user-command enumerators they would
	// count up from -2 into CMD_USER's value
	if strings.Count(enum, "    CMD_SIZE,\n") != 1 {
		return nil, fmt.Errorf("filefront: CMD_SIZE is not in the enum once")
	}
	enum = strings.Replace(enum, "    CMD_SIZE,\n", "    CMD_SIZE,\n"+strings.Join(moved, ""), 1)
	t = t[:es] + enum + t[ee:]
	fmt.Fprintf(w, "  filefront    %d rows gone, their enumerators after CMD_SIZE: %s\n",
		len(fileFrontCmds), strings.Join(fileFrontCmds, " "))
	return []byte(t), nil
}

// quitHead is ex_quit's refusal on the seed: a changed buffer, more files to
// edit, or another changed buffer kept `:q` from quitting.
const quitHead = "if ((!buf_hide(wp->w_buffer) && check_changed(wp->w_buffer, (p_awa ? CCGD_AW : 0) | (eap->forceit ? CCGD_FORCEIT : 0) | CCGD_EXCMD)) || check_more(TRUE, eap->forceit) == FAIL || (only_one_window() && check_changed_any(eap->forceit, TRUE)))"

// QuitFront makes `:q` quit (the reform's front cut for phase 94's change):
// with nothing that can be written, the refusal to quit a changed buffer is a
// door onto nothing.  It folds never, the else arm -- quit -- stays, and
// check_changed_any's tail, the last caller of the editor's buffer- and
// window-switching code, goes with it.
func QuitFront(text []byte, w io.Writer) ([]byte, error) {
	a, z, ok := edit.FindDefinition(text, edit.Blank(text), "ex_quit")
	if !ok {
		return nil, fmt.Errorf("quitfront: ex_quit is not defined")
	}
	body, err := edit.FoldNever(text[a:z], edit.Head(quitHead), 1)
	if err != nil {
		return nil, fmt.Errorf("quitfront: ex_quit's refusal -- %v", err)
	}
	fmt.Fprintln(w, "  quitfront    :q quits: ex_quit's refusal for a changed buffer folds never")
	return append(append(append([]byte(nil), text[:a]...), body...), text[z:]...), nil
}
