package check

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/harness"
)

// The shell a whim check runs in: one check's work tree and state, and the
// primitives -- grep, od, cat, cut and a headless vim -- its transcription
// needs.
//
// Each check is a line-for-line transcription of its phase/NNN/check.sh:
// the same assertions in the same order, the same text on the same line, the
// same first refusal ending the run.  These are the shell's primitives, and
// the one that is easy to get wrong is the first: grep without -E is a BASIC
// regular expression, where ( ) { } | + ? are LITERALS, and RE2 reads every
// one of them as an operator -- so a BRE handed to Go unconverted is either a
// compile error or, worse, a pattern that matches something else.

// Wsh is one check's shell: its work tree, its state directory, the source
// and the line count the edit was handed.
type Wsh struct {
	W              io.Writer
	Work, State, F string
	before         string
	srcCache       string
	srcRead        bool
}

func NewWsh(w io.Writer, name string, args []string) (*Wsh, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: check %s <work-dir> <state-dir>", name)
	}
	s := &Wsh{W: w, Work: args[0], State: args[1]}
	s.F = filepath.Join(s.Work, "whim-vim.c")
	s.before = strings.TrimSpace(ReadFile(filepath.Join(s.State, "input-lines")))
	return s, nil
}

// Src is the source, read once: nothing a check does after its first grep
// writes whim-vim.c.
func (s *Wsh) Src() string {
	if !s.srcRead {
		s.srcCache, s.srcRead = ReadFile(s.F), true
	}
	return s.srcCache
}

func (s *Wsh) Echo(format string, a ...any) { fmt.Fprintf(s.W, format+"\n", a...) }

func (s *Wsh) Phasecheck() error {
	if PhaseCheck(s.W, s.Work, s.F, filepath.Join(s.State, "symbols")) != nil {
		return harness.ErrReported
	}
	return nil
}

func (s *Wsh) Phasebuild() error {
	if PhaseBuild(s.W, s.Work, s.before) != nil {
		return harness.ErrReported
	}
	return nil
}

// St runs `tools/st.sh <args>` with its output passed through, as a check's
// own_checks does, and reports whether it succeeded.
func (s *Wsh) St(args ...string) bool {
	return Run(s.W, "tools/st.sh", args...) == nil
}

// ---- grep --------------------------------------------------------------

type Gmode int

const (
	GBRE  Gmode = iota // grep
	GERE               // grep -E
	GBREw              // grep -w
	GEREw              // grep -Ew
	GFix               // grep -F
)

// bre2re translates a GNU basic regular expression into RE2.
func bre2re(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '\\' && i+1 < len(p):
			i++
			switch d := p[i]; d {
			case '(', ')', '{', '}', '|', '+', '?':
				b.WriteByte(d)
			case '<', '>':
				b.WriteString(`\b`)
			default:
				b.WriteByte('\\')
				b.WriteByte(d)
			}
		case c == '[':
			j := i + 1
			if j < len(p) && p[j] == '^' {
				j++
			}
			if j < len(p) && p[j] == ']' {
				j++
			}
			for j < len(p) && p[j] != ']' {
				if p[j] == '[' && j+1 < len(p) && (p[j+1] == ':' || p[j+1] == '.' || p[j+1] == '=') {
					if k := strings.Index(p[j+2:], string(p[j+1])+"]"); k >= 0 {
						j += k + 4
						continue
					}
				}
				j++
			}
			if j >= len(p) {
				b.WriteString(regexp.QuoteMeta(p[i:]))
				return b.String()
			}
			b.WriteString(strings.ReplaceAll(p[i:j+1], `\`, `\\`))
			i = j
		case strings.IndexByte("(){}|+?", c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ere2re is GNU ERE into RE2: the same but for \< and \>.
func ere2re(p string) string {
	return strings.NewReplacer(`\<`, `\b`, `\>`, `\b`).Replace(p)
}

func Gre(pat string, m Gmode) *regexp.Regexp {
	var r string
	switch m {
	case GBRE:
		r = bre2re(pat)
	case GERE:
		r = ere2re(pat)
	case GBREw:
		r = `\b(?:` + bre2re(pat) + `)\b`
	case GEREw:
		r = `\b(?:` + ere2re(pat) + `)\b`
	case GFix:
		r = regexp.QuoteMeta(pat)
	}
	return regexp.MustCompile(r)
}

func Lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// GrepLines is every line of text grep would print, with its number.
func GrepLines(text, pat string, m Gmode) (nums []int, Out []string) {
	re := Gre(pat, m)
	for i, l := range Lines(text) {
		if re.MatchString(l) {
			nums, Out = append(nums, i+1), append(Out, l)
		}
	}
	return
}

func GrepC(text, pat string, m Gmode) int  { n, _ := GrepLines(text, pat, m); return len(n) }
func GrepQ(text, pat string, m Gmode) bool { return GrepC(text, pat, m) > 0 }

// GrepO is `grep -o`: every match, in order.
func GrepO(text, pat string, m Gmode) []string {
	re := Gre(pat, m)
	var Out []string
	for _, l := range Lines(text) {
		Out = append(Out, re.FindAllString(l, -1)...)
	}
	return Out
}

// CutC is `cut -c1-N`, which counts bytes.
func CutC(l string, n int) string {
	if len(l) > n {
		return l[:n]
	}
	return l
}

// show is `grep -n PAT f | head -3 | sed 's/^/               /' | cut -c1-100`.
func (s *Wsh) Show(pat string, m Gmode) {
	nums, ls := GrepLines(s.Src(), pat, m)
	for i := 0; i < len(nums) && i < 3; i++ {
		fmt.Fprintln(s.W, CutC(fmt.Sprintf("               %d:%s", nums[i], ls[i]), 100))
	}
}

// gone is the loop nearly every check opens with: each pattern must have no
// line in the source, and the first that does is named, with whatever the
// check says next and three matching lines.  count and shown are separate
// modes because one check counts with one grep and shows with another.
func (s *Wsh) Gone(prefix string, count, shown Gmode, display bool, extra []string, pats ...string) bool {
	return s.GoneW(prefix, "%s", count, shown, display, extra, pats...)
}

// GoneW is gone where the loop greps a pattern built around the name --
// `grep -cE "\b$g\b"` -- and names the bare $g when it refuses.
func (s *Wsh) GoneW(prefix, wrap string, count, shown Gmode, display bool, extra []string, names ...string) bool {
	for _, g := range names {
		pat := strings.ReplaceAll(wrap, "%s", g)
		if n := GrepC(s.Src(), pat, count); n != 0 {
			s.Echo("%s%s still has %d mentions after the sweep", prefix, g, n)
			for _, e := range extra {
				s.Echo("%s", e)
			}
			if display {
				s.Show(pat, shown)
			}
			return false
		}
	}
	return true
}

// undefined is `grep -qx SYM .cache/symbols/last/undefined`.
func undefined(sym string) bool {
	for _, l := range Lines(ReadFile(".cache/symbols/last/undefined")) {
		if l == sym {
			return true
		}
	}
	return false
}

func SymNum(which string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(ReadFile(".cache/symbols/last/" + which)))
	return n
}

// AwkRanges is `awk '/FROM/,/TO/' f`: every range, each from a line FROM
// matches to the next line TO matches, the start line included in both tests.
func AwkRanges(text, from, to string) string {
	f, t := regexp.MustCompile(from), regexp.MustCompile(to)
	var Out []string
	in := false
	for _, l := range Lines(text) {
		if !in && f.MatchString(l) {
			in = true
		}
		if in {
			Out = append(Out, l)
			if t.MatchString(l) {
				in = false
			}
		}
	}
	if len(Out) == 0 {
		return ""
	}
	return strings.Join(Out, "\n") + "\n"
}

// ---- files --------------------------------------------------------------

// CatS is "$(cat f)": the contents with trailing newlines removed, empty when
// there is no file.
func CatS(p string) string { return strings.TrimRight(ReadFile(p), "\n") }

// Bar is "$(tr '\n' '|' < f)".
func Bar(p string) string { return strings.ReplaceAll(ReadFile(p), "\n", "|") }

func Put(p, s string) { os.WriteFile(p, []byte(s), 0o644) }

// OdX is "$(od -An -tx1 f | tr -d ' \n')".
func OdX(p string) string {
	var b strings.Builder
	for _, c := range []byte(ReadFile(p)) {
		fmt.Fprintf(&b, "%02x", c)
	}
	return b.String()
}

// OdC is "$(od -An -c f | tr -d ' \n')": printable bytes as themselves, the
// C escapes od knows by name, and every other byte as three octal digits.
func OdC(p string) string {
	var b strings.Builder
	for _, c := range []byte(ReadFile(p)) {
		switch c {
		case 0:
			b.WriteString(`\0`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\v':
			b.WriteString(`\v`)
		default:
			if c >= 0x20 && c < 0x7f {
				if c != ' ' {
					b.WriteByte(c)
				}
			} else {
				fmt.Fprintf(&b, "%03o", c)
			}
		}
	}
	return b.String()
}

// OdCs is "$(od -An -c f | tr -s ' ')", which only a refusal prints; close
// enough to read, and never compared.
func OdCs(p string) string {
	o, _ := exec.Command("sh", "-c", `od -An -c "$1" | tr -s ' '`, "sh", p).Output()
	return strings.TrimRight(string(o), "\n")
}

// CatA is `cat -A`: $ at every line end, ^I for tab, ^X and M- for the rest.
func CatA(p string) string {
	var b strings.Builder
	for _, c := range []byte(ReadFile(p)) {
		switch {
		case c == '\n':
			b.WriteString("$\n")
		case c == '\t':
			b.WriteString("^I")
		case c >= 0x80:
			b.WriteString("M-")
			c -= 0x80
			fallthrough
		default:
			switch {
			case c < 0x20:
				b.WriteByte('^')
				b.WriteByte(c + 64)
			case c == 0x7f:
				b.WriteString("^?")
			default:
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// ---- running the editor ---------------------------------------------------

// Scratch is `d=$(mktemp -d); cp "$work/whim-vim" "$d/vim"`.
func (s *Wsh) Scratch() (string, func()) {
	d, _ := os.MkdirTemp("", "whimchk")
	CopyExec(filepath.Join(s.Work, "whim-vim"), filepath.Join(d, "vim"))
	return d, func() { os.RemoveAll(d) }
}

func EnvWith(kv ...string) []string {
	drop := map[string]bool{}
	for _, x := range kv {
		drop[x[:strings.IndexByte(x, '=')]] = true
	}
	var env []string
	for _, x := range os.Environ() {
		if i := strings.IndexByte(x, '='); i >= 0 && drop[x[:i]] {
			continue
		}
		env = append(env, x)
	}
	return append(env, kv...)
}

// VimRC is `(cd DIR && [HOME=HOME] BIN ARGS... </dev/null >/dev/null 2>&1)`
// and its exit status.
func VimRC(dir, home, bin string, args ...string) int {
	_, rc := VimOut(dir, home, bin, false, args...)
	return rc
}

// VimOut is the same with stdout and stderr captured together, as
// `$(cd DIR && BIN ARGS </dev/null 2>&1)` captures them -- minus the trailing
// newlines $( ) strips.
func VimOut(dir, home, bin string, capture bool, args ...string) (string, int) {
	c := exec.Command(bin, args...)
	c.Dir = dir
	if home != "" {
		c.Env = EnvWith("HOME=" + home)
	}
	var o bytes.Buffer
	if capture {
		c.Stdout, c.Stderr = &o, &o
	}
	err := c.Run()
	rc := 0
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			rc = ExitCode(err)
		} else {
			rc = 127
		}
	}
	return strings.TrimRight(o.String(), "\n"), rc
}

// InD runs the scratch copy the way the later checks do:
// `(cd "$d" && HOME="$d" ./vim ARGS </dev/null >/dev/null 2>&1)`.
func InD(d string, args ...string) int { return VimRC(d, d, "./vim", args...) }

// InWork is `(cd "$work" && ./whim-vim ARGS </dev/null >/dev/null 2>&1)`.
func (s *Wsh) InWork(args ...string) int { return VimRC(s.Work, "", "./whim-vim", args...) }

// OutWork is `$(cd "$work" && ./whim-vim ARGS </dev/null 2>&1)` and its status.
func (s *Wsh) OutWork(args ...string) (string, int) {
	return VimOut(s.Work, "", "./whim-vim", true, args...)
}

// sub makes `$work/<name>` afresh, as `rm -rf .x && mkdir .x`.
func (s *Wsh) Sub(name string) string {
	p := filepath.Join(s.Work, name)
	os.RemoveAll(p)
	os.MkdirAll(p, 0o755)
	return p
}

// LsA is "$(ls -A | tr '\n' ' ')".
func LsA(dir string) string {
	e, _ := os.ReadDir(dir)
	var n []string
	for _, x := range e {
		n = append(n, x.Name())
	}
	sort.Strings(n)
	var b strings.Builder
	for _, x := range n {
		b.WriteString(x + " ")
	}
	return b.String()
}

// SedN is `sed -n Np f`.
func SedN(p string, n int) string {
	ls := Lines(ReadFile(p))
	if n-1 < len(ls) {
		return ls[n-1]
	}
	return ""
}

// std is the whole of a check whose Body is the two tools and nothing else.
func stdWhim(name string) Func {
	return func(w io.Writer, args []string) error {
		s, err := NewWsh(w, name, args)
		if err != nil {
			return err
		}
		if err := s.Phasecheck(); err != nil {
			return err
		}
		return s.Phasebuild()
	}
}

// OdXs is "$(od -An -tx1 f)" as a refusal prints it.
func OdXs(p string) string {
	o, _ := exec.Command("od", "-An", "-tx1", p).Output()
	return strings.TrimRight(string(o), "\n")
}
