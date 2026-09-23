package check

import (
	"os"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/cc"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	asmLine = regexp.MustCompile(`^/.*:(\d+)( \(discriminator \d+\))?$`)
	asmCall = regexp.MustCompile(`\bcall\s+[0-9a-f]+\s+<([\w.]+)>`)
)

// asmCalls compiles src at -O0 with line information and returns, for every
// source line, the functions its code calls, in the order the code calls them
// -- gcc's evaluation order, measured.
func AsmCalls(src string) (map[int][]string, error) {
	tmp, err := os.MkdirTemp("", "asmcalls-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	obj := filepath.Join(tmp, "o.o")
	if Out, err := exec.Command("gcc", "-O0", "-g", "-fno-stack-protector", "-c", "-o", obj, src).CombinedOutput(); err != nil {
		return nil, &AsmErr{string(Out)}
	}
	Out, err := exec.Command("objdump", "-d", "-l", "--no-show-raw-insn", obj).Output()
	if err != nil {
		return nil, err
	}
	calls := map[int][]string{}
	cur := 0
	for _, l := range strings.Split(string(Out), "\n") {
		if m := asmLine.FindStringSubmatch(l); m != nil {
			cur, _ = strconv.Atoi(m[1])
			continue
		}
		if m := asmCall.FindStringSubmatch(l); m != nil && cur > 0 {
			calls[cur] = append(calls[cur], m[1])
		}
	}
	return calls, nil
}

type AsmErr struct{ Out string }

func (e *AsmErr) Error() string { return "gcc -g: " + e.Out }

// firstOf is the position of the first call to any of names in calls, or -1.
func FirstOf(calls []string, names []string) int {
	for i, c := range calls {
		for _, n := range names {
			if c == n {
				return i
			}
		}
	}
	return -1
}

// parseCore parses the core of a whim-vim.c text -- everything above its first
// #include, as `make editor.c` cuts it; line numbers are the file's own.
func ParseCore(text string) (*cc.AST, error) {
	i := strings.Index(text, "\n#include")
	if i < 0 {
		i = len(text) - 1
	}
	tmp, err := os.MkdirTemp("", "core-parse-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	f := filepath.Join(tmp, "editor.c")
	if err := os.WriteFile(f, []byte(text[:i+1]), 0o644); err != nil {
		return nil, err
	}
	return ccx.Parse(f)
}
