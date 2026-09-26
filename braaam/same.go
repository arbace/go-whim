package braaam

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SAME CLASSES: the proof of a change to how the Java is spelled.  javac
// folds a constant expression and drops parentheses, so an Editor.java that
// names a constant the old one wrote as its value, or loses a pair of
// parentheses, compiles to the same code -- the Java's SOURCE_DATE_EPOCH
// comparison, with nothing run (doc/JAVA-IDIOMS.md).  Not always to the same
// bytes: a constant named for the first time is a field of its own and moves
// the constant pool.  So a class that is not byte for byte the same is
// compared as its code: javap's listing of every method with the pool's
// indices dropped (the value each loads is in the listing), ldc_w read as
// ldc, a jump's target as the number of the instruction it reaches, and the
// constant fields' declarations and the blank lines left out.

// Same compiles the Java editor twice -- with oldJava as its Editor.java,
// then newJava -- each with the embedded sources and no debugging
// information (-g:none), under dir, and returns nil when every class of the
// two is the same: its bytes, or its code.  It writes a line of counts to w.
func Same(oldJava, newJava, dir string, w io.Writer) error {
	var classes [2]string
	for i, src := range []string{oldJava, newJava} {
		d := filepath.Join(dir, []string{"old", "new"}[i])
		files, err := WriteSources(filepath.Join(d, "src"))
		if err != nil {
			return err
		}
		ed := filepath.Join(d, "src", "Editor.java")
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(ed, b, 0o644); err != nil {
			return err
		}
		classes[i] = filepath.Join(d, "classes")
		args := append([]string{"-g:none", "-nowarn", "-encoding", "UTF-8", "-d", classes[i]}, append(files, ed)...)
		if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("javac %s: %v\n%s", src, err, out)
		}
	}
	names := [2][]string{}
	for i := range classes {
		err := filepath.WalkDir(classes[i], func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(classes[i], p)
				names[i] = append(names[i], rel)
			}
			return err
		})
		if err != nil {
			return err
		}
		sort.Strings(names[i])
	}
	if strings.Join(names[0], "\n") != strings.Join(names[1], "\n") {
		return fmt.Errorf("the two compile to different classes: %d against %d", len(names[0]), len(names[1]))
	}
	bytesSame, codeSame := 0, 0
	for _, n := range names[0] {
		a, err := os.ReadFile(filepath.Join(classes[0], n))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(classes[1], n))
		if err != nil {
			return err
		}
		if bytes.Equal(a, b) {
			bytesSame++
			continue
		}
		la, err := code(filepath.Join(classes[0], n))
		if err != nil {
			return err
		}
		lb, err := code(filepath.Join(classes[1], n))
		if err != nil {
			return err
		}
		if la != lb {
			return fmt.Errorf("%s: the code differs\n%s", n, firstDiff(la, lb))
		}
		codeSame++
	}
	fmt.Fprintf(w, "  same classes %d classes: %d byte for byte, %d the same code\n", len(names[0]), bytesSame, codeSame)
	return nil
}

var (
	// an instruction: its offset, its opcode and the rest
	jInsn = regexp.MustCompile(`^\s+(\d+): ([a-z]\w*)(.*)$`)
	// a switch's case or default: its target
	jCase = regexp.MustCompile(`^(\s+(?:-?\d+|default): )(\d+)$`)
	// an exception table's row: from, to, target, type
	jExc = regexp.MustCompile(`^(\s+)(\d+)(\s+)(\d+)(\s+)(\d+)(\s+.*)$`)
	// a constant field's declaration
	jConstField = regexp.MustCompile(`^\s+(?:(?:public|private|protected) )?static final (?:int|long|short|byte|char|boolean) \w+;$`)
	jPoolRef    = regexp.MustCompile(`#\d+(?:[,.:]#?\d+)*`)
	jumpOps     = regexp.MustCompile(`^(?:if\w*|goto(?:_w)?|jsr(?:_w)?)$`)
)

// code is javap's listing of the class file p, normalized as Same compares
// it.
func code(p string) (string, error) {
	out, err := exec.Command("javap", "-c", "-p", p).Output()
	if err != nil {
		return "", fmt.Errorf("javap %s: %v", p, err)
	}
	lines := strings.Split(string(out), "\n")
	// each method's instructions numbered in order -- a method's listing
	// starts at offset 0 -- so that a jump's target reads as the number of
	// the instruction it reaches, whatever the offsets between
	type method struct {
		index map[string]int
		n     int
	}
	owner := make([]*method, len(lines))
	var cur *method
	for i, l := range lines {
		if m := jInsn.FindStringSubmatch(l); m != nil {
			if m[1] == "0" || cur == nil {
				cur = &method{index: map[string]int{}}
			}
			cur.index[m[1]] = cur.n
			cur.n++
		}
		owner[i] = cur
	}
	var b strings.Builder
	for i, l := range lines {
		target := func(off string) string {
			if owner[i] != nil {
				if n, ok := owner[i].index[off]; ok {
					return "@" + strconv.Itoa(n)
				}
			}
			return "@end"
		}
		switch {
		case jConstField.MatchString(l), strings.TrimSpace(l) == "":
			continue
		case jInsn.MatchString(l):
			m := jInsn.FindStringSubmatch(l)
			op, rest := m[2], strings.TrimSpace(jPoolRef.ReplaceAllString(m[3], "#"))
			if op == "ldc_w" {
				op = "ldc"
			}
			if jumpOps.MatchString(op) {
				rest = target(rest)
			}
			fmt.Fprintf(&b, "%s %s\n", op, rest)
		case jCase.MatchString(l):
			m := jCase.FindStringSubmatch(l)
			fmt.Fprintf(&b, "%s%s\n", strings.TrimSpace(m[1]), target(m[2]))
		case jExc.MatchString(l):
			m := jExc.FindStringSubmatch(l)
			fmt.Fprintf(&b, "exc %s %s %s%s\n", target(m[2]), target(m[4]), target(m[6]), jPoolRef.ReplaceAllString(m[7], "#"))
		default:
			b.WriteString(jPoolRef.ReplaceAllString(l, "#") + "\n")
		}
	}
	return b.String(), nil
}

// firstDiff is the first line where a and b part, with a few lines before.
func firstDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			from := max(i-5, 0)
			return fmt.Sprintf("old:\n  %s\nnew:\n  %s", strings.Join(la[from:i+1], "\n  "), strings.Join(lb[from:i+1], "\n  "))
		}
	}
	return fmt.Sprintf("one listing is longer: %d lines against %d", len(la), len(lb))
}
