package check

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

// What more than one phase uses.
//
// EVERY DECLARATION HERE WAS ONE PHASE'S, and is here because another phase
// reached it: when each phase became a package of its own (phase/NNN), a helper
// two phases share stopped being either one's.  The phase it was written for is
// named above each.

// From phase 54.
// run is a shell tool the check shells Out to, with its output going to the
// report exactly where the shell put it.  Its own refusal message is its
// output, so nothing is added here.
func Run(w io.Writer, name string, arg ...string) error {
	c := exec.Command(name, arg...)
	c.Stdout, c.Stderr = w, w
	return c.Run()
}

// From phase 80.
// optRepr is repr() of a file's contents, or None when there was no file.
func OptRepr(b *string) string {
	if b == nil {
		return "None"
	}
	return PyRepr(*b)
}

// From phase 85.
// occurrences counts MATCHES and not lines.  `grep -c` counts matching lines,
// and `!isatty(fd) && isatty(read_cmd_fd)` puts two calls on one -- which is how
// the five isatty calls first read as four and failed a check that was right.
func Occurrences(pat string, src []byte) int {
	return len(regexp.MustCompile(pat).FindAll(src, -1))
}

// From phase 85.
func Z2Env(home string) []string {
	env := []string{}
	for _, kv := range os.Environ() {
		k := kv[:strings.IndexByte(kv, '=')]
		switch k {
		case "VIMINIT", "EXINIT", "MYVIMRC", "HOME", "VIM", "VIMRUNTIME", "XDG_CONFIG_HOME", "TERM":
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"HOME="+home,
		"VIM="+filepath.Join(home, "novim"),
		"VIMRUNTIME="+filepath.Join(home, "novim"),
		"XDG_CONFIG_HOME="+filepath.Join(home, "xdg"),
		"TERM=xterm")
}

// From phase 86.
// countHeaders is `grep -c '^=== '`.
func CountHeaders(p string) int {
	n := 0
	for _, l := range strings.Split(ReadFile(p), "\n") {
		if strings.HasPrefix(l, "=== ") {
			n++
		}
	}
	return n
}

// From phase 86.
// recordThrice is a whole recording three times over one binary, each run
// compared with the first as `diff -r` would.
func RecordThrice(w io.Writer, bin, f, tmp string) bool {
	for i := 1; i <= 3; i++ {
		Out := filepath.Join(tmp, fmt.Sprintf("run%d", i))
		if err := RunZ(w, bin, f, Out); err != nil {
			return false
		}
		if i != 1 {
			if d := DiffRQ(filepath.Join(tmp, "run1"), Out); len(d) > 0 {
				(&Rep{Tag: "instrument", W: w}).Say("run %d differs from run 1 -- not deterministic, not an instrument:", i)
				for _, l := range Head(d, 10) {
					fmt.Fprintln(w, "               "+l)
				}
				return false
			}
		}
	}
	return true
}

// From phase 86.
// staticFacts is the four readelf facts every whole-phase program asserts,
// and prints the one line that says so.
func StaticFacts(w io.Writer, bin string) bool {
	typ, Interp, Dyn, Rel := Z22Readelf(bin)
	if typ != "EXEC" || Interp != 0 || Dyn != 1 || Rel != 1 {
		(&Rep{Tag: "static", W: w}).Say("NOT absolutely static: type %s, INTERP %d, no-dynamic %d, no-relocations %d", typ, Interp, Dyn, Rel)
		return false
	}
	(&Rep{Tag: "build", W: w}).Say("ok, %d bytes: EXEC, no INTERP, no dynamic section, 0 relocations", SizeOf(bin))
	return true
}

// From phase 86.
func DirCount(p string) int {
	e, _ := os.ReadDir(p)
	return len(e)
}

// From phase 86.
func Sha256File(p string) string {
	b, _ := os.ReadFile(p)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// From phase 87.
// z4Probe is one probe: a command line, a key sequence, and whether this phase
// is required to MOVE it.  Both halves matter -- a probe that only checks the
// new binary passes just as well on a phase that did nothing.
type Z4Probe struct {
	Name   string
	Args   []string
	Keys   [][]byte
	Differ bool
}

// From phase 89.
// z6Probe is a probe whose run directory is kept, because what this phase is
// about is whether a file reached the disk.
type Z6Probe struct {
	Name   string
	Args   []string
	Keys   [][]byte
	Differ bool
}

// From phase 89.
const Z6E492 = "E492: Not an editor command"

// From phase 89.
var Z6RowRe = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)

// From phase 90.
// READ_IN is what proves the bytes arrived: the keystroke file holds
// `...\x1b:q!\r`, which zscreen draws as `^[:q!^M`, and an Escape can only be
// in the buffer if the file was read -- nothing typed at `:` puts one there.
// The file MESSAGE is not the check: `:1r keys` reads the file and leaves the
// message line blank, measured, so only `:r keys` can be asked for `"keys"`.
const Z7ReadIn = "^[:q!^M"

// From phase 90.
var Z7QRow = regexp.MustCompile(`(?m)^ *\{'Q', nv_error,`)

// From phase 91.
var Z8Lock = regexp.MustCompile(`(?m)^.*EX_LOCK_OK.*curbuf_locked\(\).*$`)

// From phase 92.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// From phase 92.
func Z9Flag(mk, name string) string {
	m := regexp.MustCompile(`(?m)^` + name + `  *= *(.*)$`).FindStringSubmatch(mk)
	if m == nil {
		return ""
	}
	return m[1]
}

// From phase 93.
// z10AGO blinds ONE FIELD of the pty editing session, and the field is a WALL
// CLOCK.  The `u` prints `1 change; before #3  N second(s) ago`, which neither
// binary decides: ptyrun writes the keystrokes 0.6 s apart, so the elapsed time
// sits ON the one-second boundary and which side it falls is how busy the
// machine is.  Measured on the phase that owns these keys: 30 of 240 runs
// failed, every one differing in that field ALONE, and 0 of 120 with it
// blinded.  Everything else, `1 change; before #3` included, is still compared
// byte for byte -- and the guard below is what stops the blinding from quietly
// becoming a blinding of nothing.
var Z10AGO = regexp.MustCompile(`\d+ seconds? ago`)

// From phase 93.
func Tail200(s string) string {
	if len(s) > 200 {
		return s[len(s)-200:]
	}
	return s
}

// From phase 94.
var _ = io.Discard

// From phase 95.
var (
	Z12RowRe   = regexp.MustCompile(`(?m)^[ \t]*\{"([a-z]+)",`)
	Z12GlobRe  = regexp.MustCompile(`&(p_[a-z0-9_]+)\b`)
	Z12WhiteRe = regexp.MustCompile(`(?s)modeline_whitelist\[\][^;]*?\{(.*?)\n\};`)
	Z12Fmt     = regexp.MustCompile(`"\\"%s%s%s%s%s", curbufIsChanged\(\)`)
)

// From phase 100.
// z17Session starts the editor on pipes, types into it, waits for it to go
// QUIET, and only then sends the signals -- so that what was drawn is a
// property of the editor and not of the machine's load.  argv[0] decides the
// mode, so the binary is staged as `vim`; the child gets a session of its own,
// because a signal sent to a process group reaches the harness too.
func Z17Session(binary string, sigs []syscall.Signal) Z17Res {
	vim, err := harness.Stage(binary)
	if err != nil {
		return Z17Res{Rc: -1, Out: []byte("STAGE " + err.Error())}
	}
	home, _ := os.MkdirTemp("", "whim100-home-")
	defer os.RemoveAll(home)
	c := exec.Command(vim, "+set paste")
	c.Env = Z2Env(home)
	harness.Setsid(c)
	stdin, _ := c.StdinPipe()
	stdout, _ := c.StdoutPipe()
	var errb bytes.Buffer
	c.Stderr = &errb
	if err := c.Start(); err != nil {
		return Z17Res{Rc: -1, Out: []byte("START " + err.Error())}
	}
	stdin.Write([]byte("ihello"))

	chunks := make(chan []byte, 64)
	go func() {
		buf := make([]byte, 65536)
		for {
			n, e := stdout.Read(buf)
			if n > 0 {
				b := make([]byte, n)
				copy(b, buf[:n])
				chunks <- b
			}
			if e != nil {
				close(chunks)
				return
			}
		}
	}()
	var drawn []byte
	last, deadline := time.Now(), time.Now().Add(20*time.Second)
	closed := false
wait:
	for {
		if time.Now().After(deadline) {
			drawn = append(drawn, []byte("\n<<EDITOR NEVER DREW THE TYPED TEXT>>")...)
			break
		}
		select {
		case b, ok := <-chunks:
			if !ok {
				closed = true
				break wait
			}
			drawn = append(drawn, b...)
			last = time.Now()
		case <-time.After(50 * time.Millisecond):
			if bytes.Contains(drawn, []byte("hello")) && time.Since(last) > 300*time.Millisecond {
				break wait
			}
		}
	}
	for _, s := range sigs {
		if syscall.Kill(c.Process.Pid, s) != nil {
			break
		}
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	var werr error
	select {
	case werr = <-done:
	case <-time.After(20 * time.Second):
		c.Process.Kill()
		werr = <-done
	}
	if !closed {
		for b := range chunks {
			drawn = append(drawn, b...)
		}
	}
	rc := 0
	if werr != nil {
		rc = ExitCode(werr)
	}
	return Z17Res{rc, drawn, errb.Bytes()}
}

// From phase 101.
// z18Quiet runs to completion with stdin at /dev/null.  /dev/null AND NOT A
// PIPE, and that was a measurement: with a pipe the harness closes, the EOF case
// came back as a twenty-second timeout on four binaries of five and a clean 1 on
// the fifth -- a race in the HARNESS, not the editor.
func Z18Quiet(binary string, args []string) string {
	vim, err := harness.Stage(binary)
	if err != nil {
		return "STAGE"
	}
	home, _ := os.MkdirTemp("", "whim101-home-")
	defer os.RemoveAll(home)
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	c := exec.Command(vim, args...)
	c.Stdin, c.Stdout, c.Stderr, c.Env = devnull, nil, nil, Z2Env(home)
	harness.Setsid(c)
	if err := c.Start(); err != nil {
		return "START"
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case e := <-done:
		if e == nil {
			return "0"
		}
		return fmt.Sprintf("%d", ExitCode(e))
	case <-time.After(20 * time.Second):
		c.Process.Kill()
		<-done
		return "TIMEOUT"
	}
}

// From phase 101.
func Z18Signalled(binary string, sig syscall.Signal) string {
	x := Z17Session(binary, []syscall.Signal{sig})
	if strings.Contains(string(x.Out), "NEVER DREW") {
		return "NEVER DREW"
	}
	return fmt.Sprintf("%d", x.Rc)
}

// From phase 102.
// trSpace is `tr '\n' ' '` of a file holding these lines: every one followed by
// a space, so an empty set is the empty string.
func TrSpace(s []string) string {
	var b strings.Builder
	for _, v := range s {
		b.WriteString(v + " ")
	}
	return b.String()
}

// From phase 105.
// z22Readelf is the four facts the shell took from readelf: the Type word,
// the count of INTERP lines, and whether readelf says there is no dynamic
// section and no relocation.
func Z22Readelf(bin string) (typ string, Interp, Dyn, Rel int) {
	h, _ := exec.Command("readelf", "-h", bin).Output()
	sc := bufio.NewScanner(strings.NewReader(string(h)))
	for sc.Scan() {
		l := sc.Text()
		k := strings.Index(l, ":")
		if k < 0 {
			continue
		}
		if regexp.MustCompile(`^ *Type$`).MatchString(l[:k]) {
			if fs := strings.Fields(l[k+1:]); len(fs) > 0 {
				typ = fs[0]
			}
		}
	}
	lo, _ := exec.Command("readelf", "-l", bin).Output()
	Interp = CountLinesWith(lo, "INTERP")
	d, _ := exec.Command("readelf", "-d", bin).Output()
	Dyn = z22CountExact(d, "There is no dynamic section in this file.")
	rr, _ := exec.Command("readelf", "-r", bin).Output()
	Rel = z22CountExact(rr, "There are no relocations in this file.")
	return
}

// From phase 105.
func Head(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// From phase 105.
func z22CountExact(b []byte, line string) int {
	n := 0
	for _, l := range strings.Split(string(b), "\n") {
		if l == line {
			n++
		}
	}
	return n
}

// From phase 105.
var (
	Z22Head   = regexp.MustCompile(`(?m)^(\w+)\(`)
	Z22InFunc = regexp.MustCompile(`In function (?:‘(\w+)’|'(\w+)')`)
)

// From phase 106.
// z23CmpL is `cmp -l a b | wc -l`: the bytes that differ over the common length.
func Z23CmpL(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	d := 0
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			d++
		}
	}
	return d
}

// From phase 107.
// pyRepr is Python's repr() of a str: single quotes unless the text holds a
// single quote and no double one, and backslash escapes for the rest.
func PyRepr(s string) string {
	q := byte('\'')
	if strings.IndexByte(s, '\'') >= 0 && strings.IndexByte(s, '"') < 0 {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == q:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// From phase 109.
// nmField26 is `nm <flags> f | awk '{print $N}' | sort`.  The FIELD INDEX is
// the shell's and differs between the two calls here -- `$2` for `nm -u` and
// `$3` for `--extern-only --defined-only` -- because an undefined symbol has
// no address column and a defined one does.
func NmField26(path string, flags []string, field int) []string {
	Out, _ := exec.Command("nm", append(flags, path)...).Output()
	var got []string
	for _, l := range strings.Split(strings.TrimRight(string(Out), "\n"), "\n") {
		fs := strings.Fields(l)
		if len(fs) > field {
			got = append(got, fs[field])
		} else {
			got = append(got, "")
		}
	}
	sort.Strings(got)
	return got
}

// From phase 109.
// pyRepr26 is Python's `%r` for a plain string: single quotes unless the text
// holds one.
func PyRepr26(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// From phase 109.
func Minus26(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	var Out []string
	for _, x := range a {
		if x != "" && !in[x] {
			Out = append(Out, x)
		}
	}
	return Out
}

// From phase 110.
// z27Repr is Python's `repr(k[:60])`.
func Z27Repr(k string) string {
	if len(k) > 60 {
		k = k[:60]
	}
	if strings.Contains(k, "'") && !strings.Contains(k, `"`) {
		return `"` + k + `"`
	}
	return "'" + strings.ReplaceAll(k, "'", `\'`) + "'"
}

// From phase 110.
func Z27Keys(m map[string]bool) []string {
	Out := make([]string, 0, len(m))
	for k := range m {
		Out = append(Out, k)
	}
	sort.Strings(Out)
	return Out
}

// From phase 110.
func Z27ReprList(xs []string) string {
	var Out []string
	for _, x := range xs {
		Out = append(Out, Z27Repr(x))
	}
	return "[" + strings.Join(Out, ", ") + "]"
}

// From phase 110.
func Z27Runs(lines []string) int {
	n := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" && lines[i-1] == "" {
			n++
		}
	}
	return n
}

// From phase 111.
// z28Cut is `make editor.c`'s rule character for character: stop at the first
// `#include`, drop the trailing blank lines -- where BLANK means `strip()` is
// empty, so a line of spaces counts.  whim110's cut tests `== ""` instead; the
// two are different rules and each port keeps its own heredoc's.
func Z28Cut(text string) []string {
	var keep []string
	last := 0
	for _, line := range strings.Split(text, "\n") {
		if Z28Inc.MatchString(line) {
			break
		}
		keep = append(keep, line)
		if strings.TrimSpace(line) != "" {
			last = len(keep)
		}
	}
	return keep[:last]
}

// From phase 112.
func Z29RowCount(text string) int {
	i := strings.Index(text, "static struct vimoption options[]")
	if i < 0 {
		i = len(text) - 1
	}
	j := strings.Index(text[i:], "\n};")
	if j < 0 {
		return 0
	}
	return len(Z29Opt.FindAllString(text[i:i+j], -1))
}

// From phase 112.
var (
	Z29Row     = regexp.MustCompile(`(?m)^        \{(0x[0-9a-f]+),(0x[0-9a-f]+),(-?\d+),(-?\d+)\},?$`)
	Z29CFlags  = regexp.MustCompile(`(?m)^CFLAGS  *= *(.*)$`)
	Z29LDFlags = regexp.MustCompile(`(?m)^LDFLAGS  *= *(.*)$`)
	Z29Inc     = regexp.MustCompile(`^ *# *include `)
	Z29Hash    = regexp.MustCompile(`(?m)^ *#`)
	Z29Cmd     = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)
	Z29Opt     = regexp.MustCompile(`(?m)^[ \t]*\{"([a-z]+)",`)
	Z29Undef   = regexp.MustCompile(`'[A-Za-z_][A-Za-z0-9_]*' used but never defined`)
)

// From phase 113.
// z30BytesRepr is Python's repr() of a bytes object, quote choice included.
func Z30BytesRepr(b []byte) string {
	q := byte('\'')
	if strings.IndexByte(string(b), '\'') >= 0 && strings.IndexByte(string(b), '"') < 0 {
		q = '"'
	}
	var s strings.Builder
	s.WriteByte('b')
	s.WriteByte(q)
	for _, c := range b {
		switch {
		case c == '\\':
			s.WriteString(`\\`)
		case c == q:
			s.WriteByte('\\')
			s.WriteByte(c)
		case c == '\n':
			s.WriteString(`\n`)
		case c == '\r':
			s.WriteString(`\r`)
		case c == '\t':
			s.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			s.WriteByte(c)
		default:
			fmt.Fprintf(&s, `\x%02x`, c)
		}
	}
	s.WriteByte(q)
	return s.String()
}

// From phase 113.
// z30Diff runs diff(1) and returns its output lines, which is what the shell
// greps and seds.
func Z30Diff(a, b string) []string {
	Out, _ := exec.Command("diff", a, b).Output()
	s := strings.TrimRight(string(Out), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// From phase 113.
func Z30Same(a, b string) bool {
	x, e1 := os.ReadFile(a)
	y, e2 := os.ReadFile(b)
	return e1 == nil && e2 == nil && string(x) == string(y)
}

// From phase 113.
var (
	Z30Stamp = regexp.MustCompile(`compiled [A-Z][a-z][a-z] [ 0-9][0-9] [0-9]{4} [0-9:]{8}`)
	Z30Dir   = regexp.MustCompile(`^ *#`)
	Z30Undef = regexp.MustCompile(`'([A-Za-z_][A-Za-z0-9_]*)' used but never defined`)
	Z30GT    = regexp.MustCompile(`^> ([A-Za-z_0-9]*) `)
)

// From phase 114.
func Z31Words(xs []string) string {
	s := ""
	for _, x := range xs {
		s += x + " "
	}
	return s
}

// From phase 118.
// z35CallShaped is `(?<!\w)NAME\(`, counted.
func Z35CallShaped(text, name string) int {
	n := 0
	pat := name + "("
	for i := 0; ; {
		k := strings.Index(text[i:], pat)
		if k < 0 {
			return n
		}
		k += i
		if k == 0 || !Z35Word(text[k-1]) {
			n++
		}
		i = k + 1
	}
}

// From phase 118.
// z35Decl is the heredoc's DECL: an ordinary declaration, which RE2 cannot
// say with the Python's `(?!static |typedef |static_assert)` lookahead, so the
// three prefixes are refused by hand.
func Z35Decl(l string) bool {
	if strings.HasPrefix(l, "static ") || strings.HasPrefix(l, "typedef ") || strings.HasPrefix(l, "static_assert") {
		return false
	}
	return Z35DeclRe.MatchString(l)
}

// From phase 118.
func ContainsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// From phase 118.
func Z35Word(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= 0x80
}

// From phase 118.
var (
	Z35Dir     = regexp.MustCompile(`^ *# *`)
	Z35Include = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
	Z35DeclRe  = regexp.MustCompile(`^[A-Za-z_][\w *]*\**\w+\([^;]*\);$`)
	Z35FnName  = regexp.MustCompile(`^.*?\**(\w+)\(.*$`)
	Z35CanonLn = regexp.MustCompile(`^.*canon *`)
	Z35Warn    = regexp.MustCompile(`warning: '([A-Za-z_][A-Za-z0-9_]*)' used but never defined`)
	Z35CmdRow  = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)
	Z35Probe   = regexp.MustCompile(`PROBE alloc=(\d+) amax=(\d+) free=(\d+) fnull=(\d+) write=(\d+) wmax=(\d+) wshort=(\d+)`)
	Z35Probe4  = regexp.MustCompile(`PROBE alloc=(\d+) amax=(\d+) free=(\d+) fnull=(\d+)`)
)

// From phase 119.
// z36ShellRC is the status `$?` gives a subshell: the exit code, or 128 plus
// the signal that ended it.
func Z36ShellRC(err error, ps *os.ProcessState) int {
	if ps == nil {
		if err != nil {
			return 127
		}
		return 0
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

// From phase 121.
var (
	Z38Row      = regexp.MustCompile(`(?m)^[ \t]*\{\s*"([^"]*)"\s*,\s*(\w+)\s*\},\n`)
	Z38Quote    = regexp.MustCompile(`"`)
	Z38VimErr   = regexp.MustCompile(`\bE\d+:`)
	Z38ErrWord  = regexp.MustCompile(`\bE\d+\b`)
	Z38ErrTok   = regexp.MustCompile(`^E\d+$`)
	Z38Stream   = regexp.MustCompile(`stream (\d+)`)
	Z38Counted  = regexp.MustCompile(`^[\s)]*,\s*\(\d+\)`)
	Z38Prefix   = regexp.MustCompile(`musl_strncasecmp\(\(char \*\)\(name\), \(char \*\)\("([^"]*)"\), \((\d+)\)\)\s*(==|!=)`)
	Z38IfaceWrn = regexp.MustCompile(`warning: '([a-zA-Z_][a-zA-Z_0-9]*)' used but never defined`)
	Z38Inc      = regexp.MustCompile(`^ *# *include `)
)

// From phase 126.
// z43PyList is Python's repr of a sorted list of names.
func Z43PyList(xs []string) string {
	var q []string
	for _, x := range xs {
		q = append(q, "'"+x+"'")
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// From phase 127.
type Z44Mark struct{ Name, Pat, Where, Cond string }

// From phase 127.
var (
	Z44Ptr      = regexp.MustCompile(`\(char_u? \*\)dp[a-z_]* *\+`)
	Z44FnHead   = regexp.MustCompile(`(?m)^([a-zA-Z_][a-zA-Z0-9_]*)\(`)
	Z44DlText   = regexp.MustCompile(`\.dl_text\s*=[^=]`)
	Z44Flush    = regexp.MustCompile(`(?ms)^ml_flush_line\(buf_T \*buf\)\n\{.*?^\}$`)
	Z44FreeDb   = regexp.MustCompile(`free\([^)]*db_line`)
	Z44Low      = regexp.MustCompile(`(?m)^    low = 1;$`)
	Z44ZA       = regexp.MustCompile(`ZA=(\d+)`)
	Z44IfaceGrp = regexp.MustCompile(`'[a-zA-Z_][a-zA-Z0-9_]*' used but never defined`)
)

// From phase 132.
func At(ls []string, i int) string {
	if i < len(ls) {
		return ls[i]
	}
	return "<end>"
}

// From phase 143.
// w143Diff is a line diff by longest common subsequence: what a lost and what
// b gained, as multisets.
func W143Diff(a, b []string) (gone, added map[string]int) {
	n, m := len(a), len(b)
	l := make([][]int32, n+1)
	for i := range l {
		l[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				l[i][j] = l[i+1][j+1] + 1
			} else if l[i+1][j] >= l[i][j+1] {
				l[i][j] = l[i+1][j]
			} else {
				l[i][j] = l[i][j+1]
			}
		}
	}
	gone, added = map[string]int{}, map[string]int{}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			i++
			j++
		case l[i+1][j] >= l[i][j+1]:
			gone[a[i]]++
			i++
		default:
			added[b[j]]++
			j++
		}
	}
	for ; i < n; i++ {
		gone[a[i]]++
	}
	for ; j < m; j++ {
		added[b[j]]++
	}
	return gone, added
}

// From phase 143.
// w143Lines is a function's lines, trimmed, without the blank ones.
func W143Lines(s string) []string {
	var r []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			r = append(r, t)
		}
	}
	return r
}

// From phase 152.
// w152Vars is each row of options[] as its flags and its variable field, in
// order -- the fourth top-level field of the row.
func W152Vars(text string) [][2]string {
	head := "static struct vimoption options[] =\n{\n"
	i := strings.Index(text, head)
	if i < 0 {
		return nil
	}
	b := cutil.Blank([]byte(text))
	end := cutil.Match(b, i+len(head)-2)
	var Out [][2]string
	for k := i + len(head) - 1; k < end; k++ {
		if b[k] != '{' {
			continue
		}
		re := cutil.Match(b, k)
		depth, start := 0, k+1
		var fs []string
		for j := k + 1; j < re; j++ {
			switch b[j] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
			case ',':
				if depth == 0 {
					fs = append(fs, strings.Join(strings.Fields(text[start:j]), " "))
					start = j + 1
				}
			}
		}
		if len(fs) >= 4 {
			Out = append(Out, [2]string{fs[2], fs[3]})
		}
		k = re
	}
	return Out
}

// From phase 100.
type Z17Res struct {
	Rc       int
	Out, Err []byte
}

// From phase 111.
var (
	Z28CFlags  = regexp.MustCompile(`(?m)^CFLAGS  *= *(.*)$`)
	Z28LDFlags = regexp.MustCompile(`(?m)^LDFLAGS  *= *(.*)$`)
	Z28Inc     = regexp.MustCompile(`^ *# *include `)
	Z28Hash    = regexp.MustCompile(`^ *#`)
	Z28Stamp   = regexp.MustCompile(`(?m)^\s*(?:\w+\.)?start_tv = musl_now_ms\(\);$`)
	Z28Read    = regexp.MustCompile(`musl_now_ms\(\) - (?:\w+\.)?start_tv\b`)
	Z28TV      = regexp.MustCompile(`struct timeval\b`)
	Z28ErrLine = regexp.MustCompile(`(?m)^[^ ].*: error:.*$`)
	Z28Undef   = regexp.MustCompile(`'(\w+)' used but never defined`)
	Z28Scalar  = regexp.MustCompile(`^(?:const +)?(?:void|int|long|char|usize) *\**$`)
	Z28ParmNm  = regexp.MustCompile(`\b[A-Za-z_]\w* *$`)
	Z28WS      = regexp.MustCompile(`\s+`)
)
