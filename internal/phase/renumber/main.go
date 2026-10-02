// Command renumber numbered the pipeline's phases afresh, once: 0, 1, 2, ...
// in the order the plan runs them, where they had been numbered with gaps
// from 0 to 184.  doc/PHASES.md is the account; internal/phase/numbers.md is
// the one table it is driven by, old number to new.
//
// It is SPENT: it read the old numbering, and run again it would read the new
// numbers as old ones.  It refuses unless the plan still names phase 184.
// It is kept as the record of how the numbers moved:
//
//	go run ./internal/phase/renumber move        the directories, with git mv;
//	                                             the package clauses, the
//	                                             GOAL.md titles, cmd/whim/phases.go
//	go run ./internal/phase/renumber cite F...   the citations in F, rewritten in
//	                                             place; what it cannot decide is
//	                                             written to stderr, for a hand
//	go run ./internal/phase/renumber table       doc/PHASES.md's table, on stdout
//
// THE CITATIONS.  A phase is named, in text, as:
//   - `phase N`, `phases N and M`, `phases N-M` (`Phase` too);
//   - its program's registered name, `whimN` (`whim95rows`), and identifiers
//     spelled from it in its package (`w110Reads`, `whim117Realloc`,
//     `Whim60EP`, `Own114`);
//   - its directory, `internal/phase/NNN`, and its package, `pNNN`;
//   - its snapshot, `qNNN` (or `qNN`).
//
// A live phase's number becomes its new one, a part's its new phase's number
// and letter (`phase 4b`), and a record's stays, written `record N` where a
// single phase was cited.  A range is rewritten only when both its ends are
// phases of the plan, which keeps it the same run; a list or range with a
// record in it, and a snapshot of a part or a record, are left and reported.
// Citations of another pipeline (`slim's phase 5`) are left alone.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const table = "internal/phase/numbers.md"

// An id is what an old number became: a phase (N, ""), a part (N, "b"), or
// a directory that joins the archive (-1).
type id struct {
	n      int
	letter string
}

func (i id) String() string { return strconv.Itoa(i.n) + i.letter }
func (i id) dir() string {
	d := fmt.Sprintf("internal/phase/%03d", i.n)
	if i.letter != "" {
		d += "/" + i.letter
	}
	return d
}
func (i id) pkg() string { return fmt.Sprintf("p%03d%s", i.n, i.letter) }

var (
	ids      = map[int]id{} // every old number of a live directory
	order    []int          // the old numbers, in the table's order
	archived = map[int]bool{}
)

func load() {
	b, err := os.ReadFile(table)
	if err != nil {
		die("%v", err)
	}
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "```") {
			in = !in
			continue
		}
		f := strings.Fields(line)
		if !in || len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			die("%s: a row is `new old`: %q", table, line)
		}
		old, err := strconv.Atoi(f[1])
		if err != nil {
			die("%s: %q", table, line)
		}
		if f[0] == "archive" {
			archived[old] = true
			continue
		}
		j := strings.IndexFunc(f[0], func(r rune) bool { return r < '0' || r > '9' })
		if j < 0 {
			j = len(f[0])
		}
		n, err := strconv.Atoi(f[0][:j])
		if err != nil {
			die("%s: %q", table, line)
		}
		ids[old] = id{n, f[0][j:]}
		order = append(order, old)
	}
}

// isPhase reports whether old was a phase of the plan, and is one still.
func isPhase(old int) bool { i, ok := ids[old]; return ok && i.letter == "" }

func main() {
	if len(os.Args) < 2 {
		die("usage: renumber move | cite FILE... | table")
	}
	load()
	switch os.Args[1] {
	case "move":
		spentGuard()
		move()
	case "cite":
		for _, f := range os.Args[2:] {
			cite(f)
		}
	case "table":
		printTable()
	default:
		die("renumber: %s?", os.Args[1])
	}
}

func spentGuard() {
	b, err := os.ReadFile("internal/build/plan.go")
	if err != nil {
		die("%v", err)
	}
	if !bytes.Contains(b, []byte("{N: 184,")) {
		die("renumber: the plan names no phase 184: the numbering is the new one already")
	}
}

// ---- move ----------------------------------------------------------------

func git(args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		die("git %s: %v", strings.Join(args, " "), err)
	}
}

func move() {
	// Two steps, through a directory of its own: a new number is often an
	// old directory's that has not moved yet.
	tmp := "internal/phase/.renumber"
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		die("%v", err)
	}
	for _, old := range order {
		git("mv", fmt.Sprintf("internal/phase/%03d", old), fmt.Sprintf("%s/%03d", tmp, old))
	}
	var olds []int
	for old := range archived {
		olds = append(olds, old)
	}
	sort.Ints(olds)
	for _, old := range olds {
		git("mv", fmt.Sprintf("internal/phase/%03d", old), fmt.Sprintf("internal/phase/archive/%03d", old))
	}
	// the phases first, so that a part's parent exists
	for _, pass := range []bool{false, true} {
		for _, old := range order {
			i := ids[old]
			if (i.letter != "") != pass {
				continue
			}
			if pass {
				if err := os.MkdirAll(filepath.Dir(i.dir()), 0o755); err != nil {
					die("%v", err)
				}
			}
			git("mv", fmt.Sprintf("%s/%03d", tmp, old), i.dir())
		}
	}
	if err := os.Remove(tmp); err != nil {
		die("%v", err)
	}
	var imports []string
	for _, old := range order {
		i := ids[old]
		gofiles, _ := filepath.Glob(i.dir() + "/*.go")
		for _, f := range gofiles {
			rewrite(f, func(b []byte) []byte {
				return regexp.MustCompile(`(?m)^package p\d{3}\b`).ReplaceAll(b, []byte("package "+i.pkg()))
			})
		}
		if len(gofiles) > 0 {
			imports = append(imports, i.dir())
		}
		rewrite(i.dir()+"/GOAL.md", func(b []byte) []byte { return goalTitle(b, old, i) })
	}
	sort.Strings(imports)
	phasesGo(imports)
}

// goalTitle renames the phase in its GOAL.md's title and says, under it,
// what it was: the rest of the file is the record of its writing, in the
// numbers it had then.
func goalTitle(b []byte, old int, i id) []byte {
	title := fmt.Sprintf("# Phase %d — ", old)
	if !bytes.HasPrefix(b, []byte(title)) {
		die("%s/GOAL.md does not open %q", i.dir(), title)
	}
	nl := bytes.IndexByte(b, '\n')
	what := "phase"
	if i.letter != "" {
		what = fmt.Sprintf("part %s of phase %d", i.letter, i.n)
	}
	note := fmt.Sprintf("\n*Formerly phase %d, now %s. The other phase numbers in this file are the old\nnumbering, as it was written: `doc/PHASES.md` maps them.*\n", old, what)
	if i.letter == "" {
		note = fmt.Sprintf("\n*Formerly phase %d. The other phase numbers in this file are the old numbering,\nas it was written: `doc/PHASES.md` maps them.*\n", old)
	}
	out := []byte(fmt.Sprintf("# Phase %s — ", i))
	out = append(out, b[len(title):nl+1]...)
	out = append(out, note...)
	return append(out, b[nl+1:]...)
}

func phasesGo(dirs []string) {
	const f = "cmd/whim/phases.go"
	rewrite(f, func(b []byte) []byte {
		start := bytes.Index(b, []byte("import (\n"))
		end := bytes.Index(b[start:], []byte("\n)\n"))
		if start < 0 || end < 0 {
			die("%s: no import block", f)
		}
		var imp bytes.Buffer
		imp.WriteString("import (\n")
		for _, d := range dirs {
			fmt.Fprintf(&imp, "\t_ \"github.com/arbace/go-whim/%s\"\n", d)
		}
		return append(append(append([]byte{}, b[:start]...), bytes.TrimSuffix(imp.Bytes(), []byte("\n"))...), b[start+end:]...)
	})
}

// ---- cite ----------------------------------------------------------------

var (
	// a gap is spaces, or a line break and what continues a comment or
	// a quotation on the next line
	gap     = `(?:[ \t]*\n[ \t]*(?://+|#|\*|>)?[ \t]*|[ \t]+)`
	reList  = regexp.MustCompile(`\b([Pp]hases?)(` + gap + `)(\d{1,3}(?:'s)?(?:(?:,` + gap + `and` + gap + `|,` + gap + `or` + gap + `|,` + gap + `|,|-|–|` + gap + `to` + gap + `|` + gap + `and` + gap + `|` + gap + `or` + gap + `)\d{1,3}(?:'s)?)*)\b`)
	reNum   = regexp.MustCompile(`\d{1,3}`)
	reIdent = regexp.MustCompile(`\b(whim|Whim|w|W|Own)(\d{1,3})(kp|ep|bl|rows|KP|EP|BL|Rows)?([A-Z_][A-Za-z0-9_]*)?\b`)
	reDir   = regexp.MustCompile(`internal/phase/(\d{3})\b`)
	rePkg   = regexp.MustCompile(`\bp(\d{3})\b`)
	reSnap  = regexp.MustCompile(`\bq(\d{2,3})\b`)
)

func cite(f string) {
	isGo := strings.HasSuffix(f, ".go")
	// the short identifier forms (w110Reads) are a phase's own package's,
	// and internal/whim's (Own114); elsewhere `w` and `W` are other words
	idents := !isGo || strings.HasPrefix(f, "internal/phase/") || strings.HasPrefix(f, "internal/whim/")
	rewrite(f, func(b []byte) []byte {
		lines := bytes.SplitAfter(b, []byte("\n"))
		for k, line := range lines {
			at := fmt.Sprintf("%s:%d", f, k+1)
			s := string(line)
			s = reDir.ReplaceAllStringFunc(s, func(m string) string {
				old, _ := strconv.Atoi(m[len("internal/phase/"):])
				if archived[old] {
					return fmt.Sprintf("internal/phase/archive/%03d", old)
				}
				if i, ok := ids[old]; ok {
					return i.dir()
				}
				return m
			})
			if isGo {
				s = rePkg.ReplaceAllStringFunc(s, func(m string) string {
					old, _ := strconv.Atoi(m[1:])
					if i, ok := ids[old]; ok {
						return i.pkg()
					}
					return m
				})
			}
			s = reIdent.ReplaceAllStringFunc(s, func(m string) string {
				sm := reIdent.FindStringSubmatch(m)
				pre := sm[1]
				if (pre == "w" || pre == "W" || pre == "Own") && !idents {
					return m
				}
				if (pre == "w" || pre == "W") && sm[3] == "" && sm[4] == "" {
					return m // w2, W12: not a phase's name
				}
				old, _ := strconv.Atoi(sm[2])
				i, ok := ids[old]
				if !ok {
					report(at, m, "names no live phase")
					return m
				}
				return pre + i.String() + sm[3] + sm[4]
			})
			s = reSnap.ReplaceAllStringFunc(s, func(m string) string {
				old, _ := strconv.Atoi(m[1:])
				if old > 184 {
					return m
				}
				if isPhase(old) {
					return fmt.Sprintf("q%03d", ids[old].n)
				}
				report(at, m, "a snapshot of no phase that runs")
				return m
			})
			lines[k] = []byte(s)
		}
		// a citation may break across lines, so the lists are read in the
		// whole text
		return []byte(rewriteLists(f, string(bytes.Join(lines, nil))))
	})
}

func rewriteLists(f, s string) string {
	var out strings.Builder
	last := 0
	for _, loc := range reList.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(s[last:loc[0]])
		last = loc[1]
		at := fmt.Sprintf("%s:%d", f, 1+strings.Count(s[:loc[0]], "\n"))
		m := s[loc[0]:loc[1]]
		word, sp, list := s[loc[2]:loc[3]], s[loc[4]:loc[5]], s[loc[6]:loc[7]]
		before := strings.ToLower(s[max(0, loc[0]-12):loc[0]])
		if strings.Contains(before, "slim") {
			out.WriteString(m)
			continue
		}
		nums := reNum.FindAllStringIndex(list, -1)
		var olds []int
		bad := false
		for _, n := range nums {
			v, _ := strconv.Atoi(list[n[0]:n[1]])
			olds = append(olds, v)
			if v > 184 {
				bad = true
			}
		}
		if bad {
			out.WriteString(m)
			continue
		}
		seps := make([]string, len(nums))
		for j := 1; j < len(nums); j++ {
			seps[j] = list[nums[j-1][1]:nums[j][0]]
		}
		// a single record: `record N`
		if len(olds) == 1 && !hasID(olds[0]) {
			rec := "record"
			if word[0] == 'P' {
				rec = "Record"
			}
			if strings.HasSuffix(word, "s") {
				report(at, m, "a record, plural")
				out.WriteString(m)
				continue
			}
			out.WriteString(rec + sp + list)
			continue
		}
		ok := true
		for j, v := range olds {
			if !hasID(v) {
				ok = false
				report(at, m, fmt.Sprintf("%d is a record, in a list", v))
				break
			}
			if j > 0 && isRange(seps[j]) && !(isPhase(olds[j-1]) && isPhase(v)) {
				ok = false
				report(at, m, "a range whose end is not a phase of the plan")
				break
			}
		}
		if !ok {
			out.WriteString(m)
			continue
		}
		var nl strings.Builder
		prev := 0
		for j, n := range nums {
			nl.WriteString(list[prev:n[0]])
			nl.WriteString(ids[olds[j]].String())
			prev = n[1]
		}
		nl.WriteString(list[prev:])
		out.WriteString(word + sp + nl.String())
	}
	out.WriteString(s[last:])
	return out.String()
}

func hasID(old int) bool { _, ok := ids[old]; return ok }
func isRange(sep string) bool {
	t := strings.TrimSpace(sep)
	return t == "-" || t == "–" || t == "to"
}

func report(at, m, why string) { fmt.Fprintf(os.Stderr, "%s: %q: %s\n", at, m, why) }

// ---- table ---------------------------------------------------------------

func printTable() {
	plan, err := os.ReadFile("internal/build/plan.go")
	if err != nil {
		die("%v", err)
	}
	reEntry := regexp.MustCompile(`\{N: (\d+),(?: Block: "([^"]*)",)? Name: "((?:[^"\\]|\\.)*)"`)
	names := map[int][2]string{}
	for _, m := range reEntry.FindAllStringSubmatch(string(plan), -1) {
		n, _ := strconv.Atoi(m[1])
		names[n] = [2]string{m[2], strings.ReplaceAll(m[3], `\"`, `"`)}
	}
	parts := map[int][]string{}
	for _, old := range order {
		if i := ids[old]; i.letter != "" {
			parts[i.n] = append(parts[i.n], fmt.Sprintf("%s (%d)", i, old))
		}
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	fmt.Fprintln(w, "| new | block | phase | old | parts: new (old) |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	block := ""
	for _, old := range order {
		i := ids[old]
		if i.letter != "" {
			continue
		}
		nm, ok := names[i.n]
		if !ok {
			die("plan.go names no phase %d", i.n)
		}
		if nm[0] != "" {
			block = "`" + nm[0] + "`"
		}
		fmt.Fprintf(w, "| %d | %s | %s | %d | %s |\n", i.n, block, nm[1], old, strings.Join(parts[i.n], ", "))
	}
}

// ---- files ---------------------------------------------------------------

func rewrite(f string, edit func([]byte) []byte) {
	b, err := os.ReadFile(f)
	if err != nil {
		die("%v", err)
	}
	out := edit(b)
	if !bytes.Equal(out, b) {
		if err := os.WriteFile(f, out, 0o644); err != nil {
			die("%v", err)
		}
	}
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
