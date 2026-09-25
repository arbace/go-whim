// Package splice measures internal/gen's emitted bodies against the hand-written
// editor: it replaces, in a copy of editor/, every function of editor.go that
// bodies.go also defines, builds the copy, and takes back the hand-written
// body of every function the Go compiler rejects, until the copy builds.  It
// reports how many emitted functions the build keeps, and how many of those
// are the hand-written function exactly, after gofmt.
//
//	splice <editor-dir> <bodies.go> <out-dir>
package splice

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type fn struct {
	name       string
	start, end int // byte offsets in its file
}

func funcs(src []byte) ([]fn, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "x.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var r []fn
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil {
			continue
		}
		r = append(r, fn{fd.Name.Name, fs.Position(fd.Pos()).Offset, fs.Position(fd.End()).Offset})
	}
	return r, nil
}

func norm(s []byte) string {
	b, err := format.Source(s)
	if err != nil {
		return string(s)
	}
	return string(b)
}

// Run is the program, called as `go tool whim <name> ARGS`: args are its
// arguments, and what it used to print on stderr goes to errw.  It returns
// the exit status.
func Run(args []string, errw io.Writer) int {
	osArgs := append([]string{"run"}, args...)
	if len(osArgs) != 4 {
		fmt.Fprintln(errw, "usage: splice <editor-dir> <bodies.go> <out-dir>")
		return 2
	}
	dir, bodiesPath, out := osArgs[1], osArgs[2], osArgs[3]
	hand, err := os.ReadFile(filepath.Join(dir, "editor.go"))
	check(err)
	bodies, err := os.ReadFile(bodiesPath)
	check(err)
	hf, err := funcs(hand)
	check(err)
	bf, err := funcs(bodies)
	check(err)
	emitted := map[string]string{}
	for _, f := range bf {
		emitted[f.name] = string(bodies[f.start:f.end])
	}
	handSrc := map[string]string{}
	for _, f := range hf {
		handSrc[f.name] = string(hand[f.start:f.end])
	}
	check(os.MkdirAll(out, 0o755))
	// Every hand-written file of the package goes with the copy: the runtime,
	// the host's glue, vim_snprintf, libc.  editor.go is the one written here.
	hands, err := filepath.Glob(filepath.Join(dir, "*.go"))
	check(err)
	for _, path := range hands {
		n := filepath.Base(path)
		if n == "editor.go" || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, n))
		check(err)
		check(os.WriteFile(filepath.Join(out, n), b, 0o644))
	}
	rejected := map[string]string{}
	for round := 1; ; round++ {
		// write the copy, recording each spliced function's line span
		var b bytes.Buffer
		prev := 0
		type span struct {
			name     string
			from, to int
		}
		var spans []span
		line := 1
		for _, f := range hf {
			b.Write(hand[prev:f.start])
			line += bytes.Count(hand[prev:f.start], []byte("\n"))
			src := string(hand[f.start:f.end])
			if e, ok := emitted[f.name]; ok && rejected[f.name] == "" && f.name != "init" {
				src = e
			}
			n := strings.Count(src, "\n")
			spans = append(spans, span{f.name, line, line + n})
			b.WriteString(src)
			line += n
			prev = f.end
		}
		b.Write(hand[prev:])
		check(os.WriteFile(filepath.Join(out, "editor.go"), b.Bytes(), 0o644))
		cmd := exec.Command("go", "build", "-gcflags=-e", "-o", filepath.Join(out, "editor"), "./"+out)
		o, err := cmd.CombinedOutput()
		if err == nil {
			break
		}
		re := regexp.MustCompile(`editor\.go:(\d+):\d+: (.*)`)
		bad := 0
		for _, m := range re.FindAllStringSubmatch(string(o), -1) {
			ln, _ := strconv.Atoi(m[1])
			for _, s := range spans {
				if ln >= s.from && ln <= s.to {
					if _, ok := emitted[s.name]; ok && rejected[s.name] == "" {
						rejected[s.name] = m[2]
						bad++
					}
					break
				}
			}
		}
		if bad == 0 {
			fmt.Fprintf(errw, "splice: the copy does not build, and no emitted function is to blame:\n%s", o)
			return 1
		}
		fmt.Fprintf(errw, "splice: round %d, %d functions rejected\n", round, bad)
	}
	kept, same := 0, 0
	for name, e := range emitted {
		if _, ok := handSrc[name]; !ok || rejected[name] != "" {
			continue
		}
		kept++
		if norm([]byte(e)) == norm([]byte(handSrc[name])) {
			same++
		}
	}
	var rs []string
	for n, why := range rejected {
		rs = append(rs, n+": "+why)
	}
	sort.Strings(rs)
	check(os.WriteFile(filepath.Join(out, "rejected.txt"), []byte(strings.Join(rs, "\n")+"\n"), 0o644))
	fmt.Printf("splice: %d functions in editor.go, %d emitted, %d kept by the build (%d the hand-written function exactly), %d rejected\n",
		len(hf), len(emitted), kept, same, len(rejected))
	return 0
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
