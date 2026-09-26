package editor

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/braaam"
)

// A printfCase is one call of vim_snprintf: the format, the size handed to
// it, and the arguments -- int32 (i), uint32 (u), int64 (l), uint64 (L), byte
// (c), a string (s) and NULL (nil).
type printfCase struct {
	fmt  string
	m    int
	args []any
}

var printfCases = []printfCase{
	{"hello", 20, nil},
	{"%d", 20, []any{int32(42)}},
	{"%d|%i", 20, []any{int32(-42), int32(7)}},
	{"%5d|%-5d|%05d|%+d|% d|%.3d", 60, []any{int32(42), int32(42), int32(-42), int32(42), int32(42), int32(7)}},
	{"%d %d", 30, []any{int32(-2147483648), int32(2147483647)}},
	{"%x %X %#x %#X %o %#o %#o", 60, []any{uint32(255), uint32(255), uint32(255), uint32(0), uint32(8), uint32(8), uint32(0)}},
	{"%u %x", 40, []any{uint32(4294967295), uint32(4294967295)}},
	{"%ld %lu %lx", 80, []any{int64(-9223372036854775808), uint64(18446744073709551615), uint64(18446744073709551615)}},
	{"%lld %llu %llX", 80, []any{int64(-1), uint64(1) << 63, uint64(0xdeadbeef)}},
	{"%hd %hu %hx", 40, []any{int32(70000), uint32(70000), uint32(0x12345)}},
	{"%b %#b %B %#B %08b", 60, []any{uint64(5), uint64(5), uint64(0), uint64(0), uint64(3)}},
	{"%D %U %O", 60, []any{int64(-5), uint64(5), uint64(8)}},
	{"%c%c%c", 10, []any{int32('A'), byte(0xe9), int32(0x141)}},
	{"[%s] [%.2s] [%10s] [%-10s] [%.0s]", 80, []any{"abc", "abcdef", "right", "left", "gone"}},
	{"[%s] [%5s]", 30, []any{nil, nil}},
	{"[%S] [%.3S] [%6S] [%-6S]", 80, []any{"a\u00e9\u4e2d", "a\u4e2d\u4e2d", "\u4e2db", "\u4e2db"}},
	{"100%% %%d", 20, nil},
	{"[%*d] [%-*d] [%*d] [%.*s] [%.*d]", 80, []any{int32(5), int32(42), int32(5), int32(42), int32(-5), int32(42), int32(2), "abcdef", int32(-1), int32(7)}},
	{"%1$s %2$d %1$s", 40, []any{"x", int32(9)}},
	{"%2$s-%1$s", 40, []any{"a", "b"}},
	{"[%1$*2$d] [%1$-*2$d]", 40, []any{int32(42), int32(6)}},
	{"[%3$.*2$s] %1$d", 40, []any{int32(1), int32(3), "abcdef"}},
	{"hello world", 5, nil},
	{"%s and more", 4, []any{"abcdef"}},
	{"%d", 0, []any{int32(123)}},
	{"%d", 1, []any{int32(123)}},
	{"%08d|%-08d|%8.3d", 40, []any{int32(-42), int32(42), int32(-7)}},
	{"%y%d", 20, []any{int32(1)}},
	{"abc%", 20, nil},
	{"%'d %#d", 20, []any{int32(1234), int32(5)}},
	{"%.0d|%.0x|%#.0o", 20, []any{int32(0), uint32(0), uint32(0)}},
	{"%1048577d|", 10, []any{int32(1)}},
	// the format errors: nothing written, 0 returned, and one message each
	{"%1$s %s", 20, []any{"a", "b"}},
	{"%2$s", 20, []any{"a", "b"}},
	{"%0$d", 20, []any{int32(1)}},
	{"%1$d %1$s", 20, []any{int32(1)}},
	{"%1$*1$s", 20, []any{int32(1)}},
}

// printfErrors are the messages the format errors report, by format: the
// Java's, which the probe prints.  The Go's emsg needs an editor that has
// started, so the test turns it off (emsg_off) and compares the rest.
var printfErrors = map[string]string{
	"%1$s %s":   "emsg E1500: Cannot mix positional and non-positional arguments: %1$s %s",
	"%2$s":      "emsg E1501: format argument 1 unused in $-style format: %2$s",
	"%0$d":      "emsg E1505: Invalid format specifier: %0$d",
	"%1$d %1$s": "emsg E1504: Positional argument 1 type used inconsistently: string/int",
	"%1$*1$s":   "emsg E1502: Positional argument 1 used as field width reused as different type: int/string",
}

// javaPrintf runs editor/testdata/PrintfProbe.java, with braaam's runtime and
// host, on the cases, and returns what it prints.
func javaPrintf(t *testing.T, input string) string {
	for _, tool := range []string{"javac", "java"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	dir := t.TempDir()
	files, err := braaam.WriteSources(filepath.Join(dir, "src"))
	if err != nil {
		t.Fatal(err)
	}
	var srcs []string
	for _, f := range files {
		if !strings.HasSuffix(f, "Whim.java") { // the glue needs Editor.java
			srcs = append(srcs, f)
		}
	}
	probe, _ := filepath.Abs("testdata/PrintfProbe.java")
	classes := filepath.Join(dir, "classes")
	if out, err := exec.Command("javac", append([]string{"-nowarn", "-d", classes, probe}, srcs...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	cmd := exec.Command("java", "-cp", classes, "PrintfProbe")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("java: %v\n%s", err, out)
	}
	return string(out)
}

// braaam's vim_snprintf (host/Printf.java) prints exactly what the Go's
// (format.go) prints, byte for byte, the return value too, on a table of
// formats: flags, widths and precisions given and taken from arguments,
// every length and conversion, positional arguments, NULL strings, the
// double-width %S, and truncation at every size down to 0.
func TestJavaPrintf(t *testing.T) {
	ed := New(&fakeHost{})
	ed.emsg_off = 1
	var in, want strings.Builder
	for _, c := range printfCases {
		fmt.Fprintf(&in, "F %x\nM %d\n", c.fmt, c.m)
		var args []any
		for _, a := range c.args {
			switch x := a.(type) {
			case int32:
				fmt.Fprintf(&in, "A i %d\n", x)
			case uint32:
				fmt.Fprintf(&in, "A u %d\n", x)
			case int64:
				fmt.Fprintf(&in, "A l %d\n", x)
			case uint64:
				fmt.Fprintf(&in, "A L %d\n", x)
			case byte:
				fmt.Fprintf(&in, "A c %d\n", x)
			case string:
				fmt.Fprintf(&in, "A s %x\n", x)
				a = S(x)
			case nil:
				in.WriteString("A n\n")
				a = Ptr[byte]{}
			}
			args = append(args, a)
		}
		in.WriteString("E\n")
		n := max(c.m, 1) + 8
		buf := Mk[byte](n)
		copy(buf.Slice(n), bytes.Repeat([]byte("."), n))
		r := ed.vim_snprintf(buf, usize(c.m), S(c.fmt), args...)
		fmt.Fprintf(&want, "%d %s\n", r, hex.EncodeToString(buf.Slice(n)))
	}
	got := javaPrintf(t, in.String())
	wl := strings.Split(strings.TrimSuffix(want.String(), "\n"), "\n")
	i := 0
	var msgs []string
	for _, l := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if strings.HasPrefix(l, "emsg ") || strings.HasPrefix(l, "iemsg ") {
			msgs = append(msgs, l)
			continue
		}
		if i >= len(wl) {
			t.Fatalf("the Java printed more than the %d cases:\n%s", len(wl), got)
		}
		c := printfCases[i]
		if l != wl[i] {
			t.Errorf("%q (size %d): the Java returns and writes\n  %s\nthe Go\n  %s", c.fmt, c.m, l, wl[i])
		}
		if m := strings.Join(msgs, "\n"); m != printfErrors[c.fmt] {
			t.Errorf("%q: the Java reports %q, want %q", c.fmt, m, printfErrors[c.fmt])
		}
		msgs, i = nil, i+1
	}
	if i != len(wl) {
		t.Errorf("the Java answered %d of the %d cases:\n%s", i, len(wl), got)
	}
}
