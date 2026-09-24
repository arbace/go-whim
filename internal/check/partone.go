package check

// What more than one Part I phase's check uses, beyond shell.go's primitives:
// the loops and probes phases 9 to 79 share.  Each phase's own check is
// phase/NNN/check.go, written against these and against Wsh.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/harness"
)

// GlobalsExtra is what whims 16 to 20 say after a surviving global.
var GlobalsExtra = []string{
	"               a dropped row leaves its global uninitialised, and a",
	"               reader of it is a segfault before the first keystroke",
}

// SymsGone is the `grep -qx SYM undefined` loop: the first still undefined
// refuses.
func (s *Wsh) SymsGone(syms ...string) bool {
	for _, g := range syms {
		if undefined(g) {
			s.Echo("  symbols      %s is still undefined in the object", g)
			return false
		}
	}
	return true
}

// EncodingIn is whims 9 and 12's probe: `:set encoding?` redirected to a file
// in $work, with spaces and newlines taken Out.
func (s *Wsh) EncodingIn(txt, Out string) string {
	Put(filepath.Join(s.Work, txt), "x\n")
	s.InWork("-u", "NONE", "-i", "NONE", "-e", "-s", "-c", "redir! > "+Out,
		"-c", "set encoding?", "-c", "redir END", "-c", "qall!", txt)
	e := strings.NewReplacer(" ", "", "\n", "").Replace(ReadFile(filepath.Join(s.Work, Out)))
	os.Remove(filepath.Join(s.Work, txt))
	os.Remove(filepath.Join(s.Work, Out))
	return e
}

// GlobalsCheck is whims 16 to 19: a gone loop in `grep -c` with the two
// extra lines, the verdict, then the two tools.
func GlobalsCheck(name string, count, shown Gmode, verdict string, pats ...string) Func {
	return func(w io.Writer, args []string) error {
		s, err := NewWsh(w, name, args)
		if err != nil {
			return err
		}
		if !s.Gone("  globals      ", count, shown, true, GlobalsExtra, pats...) {
			return harness.ErrReported
		}
		s.Echo("%s", verdict)
		if err := s.Phasecheck(); err != nil {
			return err
		}
		return s.Phasebuild()
	}
}

// ProbeSet is `(cd "$work" && ./whim-vim -e -s -c "set $1" -c 'qa!' ...)`
// and its status.
func (s *Wsh) ProbeSet(opt string) int {
	return s.InWork("-e", "-s", "-c", "set "+opt, "-c", "qa!")
}

// GoneTools is the commonest whole check: the names gone, the verdict, and the
// two tools.
func GoneTools(name, prefix, wrap string, m Gmode, verdict string, names ...string) Func {
	return func(w io.Writer, args []string) error {
		s, err := NewWsh(w, name, args)
		if err != nil {
			return err
		}
		if !s.GoneW(prefix, wrap, m, m, true, nil, names...) {
			return harness.ErrReported
		}
		s.Echo("%s", verdict)
		if err := s.Phasecheck(); err != nil {
			return err
		}
		return s.Phasebuild()
	}
}

// kept is `grep -q "\b$g("` refusing when the name is gone.
func (s *Wsh) KeptCall(prefix, why string, names ...string) bool {
	for _, g := range names {
		if GrepC(s.Src(), `\b`+g+`(`, GBRE) == 0 {
			s.Echo("%s%s%s", prefix, g, why)
			return false
		}
	}
	return true
}

// KeptE is `grep -qE "\b$g\b" f || { echo ...; exit 1; }`.
func (s *Wsh) KeptE(prefix, why string, names ...string) bool {
	for _, g := range names {
		if !GrepQ(s.Src(), `\b`+g+`\b`, GERE) {
			s.Echo("%s%s%s", prefix, g, why)
			return false
		}
	}
	return true
}

// UnknownOpts is the scratch-copy half of whims 55 to 62: the control
// `:set sw=3` must work and every named option must be refused.
func (s *Wsh) UnknownOpts(d, prefix string, opts ...string) bool {
	if InD(d, "-e", "-s", "+set sw=3", "+q!") != 0 {
		s.Echo("%sthe control :set sw=3 failed", prefix)
		return false
	}
	for _, o := range opts {
		if InD(d, "-e", "-s", "+set "+o+"?", "+q!") == 0 {
			s.Echo("%s:set %s? was accepted", prefix, o)
			return false
		}
	}
	return true
}

// GrepNum is "$(grep -n PAT f)", as a refusal prints it.
func GrepNum(p, pat string, m Gmode) string {
	nums, ls := GrepLines(ReadFile(p), pat, m)
	var o []string
	for i := range nums {
		o = append(o, fmt.Sprintf("%d:%s", nums[i], ls[i]))
	}
	return strings.Join(o, "\n")
}

// Cnt0 is `n=$(grep -cE "\b$g\b" f); [ "$n" = 0 ] || { echo "PREFIX$g still
// has $n mentions"; exit 1; }` -- the later checks' shorter form of gone.
func (s *Wsh) Cnt0(prefix string, names ...string) bool {
	for _, g := range names {
		if n := GrepC(s.Src(), `\b`+g+`\b`, GERE); n != 0 {
			s.Echo("%s%s still has %d mentions", prefix, g, n)
			return false
		}
	}
	return true
}

// FnBody is `awk '/^NAME\(/,/^\}$/' f`.
func (s *Wsh) FnBody(start string) string { return AwkRanges(s.Src(), start, `^\}$`) }

// ProbeFile is the scratch-copy probe most later checks run: write CONTENT,
// run the editor with ARGS on FILE, and read it back through READ.
func ProbeFile(d, file, content string, args ...string) string {
	Put(filepath.Join(d, file), content)
	InD(d, append(append([]string{"-e", "-s"}, args...), file)...)
	return filepath.Join(d, file)
}

// loadsEdits is the probe pair nearly every late check opens with: the file
// loads, and a line deletes.  loadWant is what `+$ +s/^/LAST /` leaves.
func (s *Wsh) Loads(d, prefix, content, want string) bool {
	t := ProbeFile(d, "t.txt", content, "+$", "+s/^/LAST /", "+wq")
	if Bar(t) != want {
		s.Echo("%sthe file did not load: '%s'", prefix, Bar(t))
		return false
	}
	return true
}

func (s *Wsh) Edits(d, prefix, file, content, want string) bool {
	e := ProbeFile(d, file, content, "+2", "+normal! dd", "+wq")
	if Bar(e) != want {
		s.Echo("%sediting broke: '%s'", prefix, Bar(e))
		return false
	}
	return true
}

// SwitchesE is `:e h2.txt` from h1.txt, with the two refusals a check names.
func (s *Wsh) SwitchesE(d, prefix, a, b, ac, bc, bwant, add string) bool {
	Put(filepath.Join(d, a), ac)
	Put(filepath.Join(d, b), bc)
	InD(d, "-e", "-s", "+e "+b, add, "+wq", a)
	if CatS(filepath.Join(d, a)) != strings.TrimRight(ac, "\n") {
		s.Echo("%s:e wrote over the first file: %s", prefix, CatS(filepath.Join(d, a)))
		return false
	}
	if CatS(filepath.Join(d, b)) != bwant {
		s.Echo("%s:e did not load the second file: %s", prefix, CatS(filepath.Join(d, b)))
		return false
	}
	return true
}

// maps is the buffer-local and global mapping probes.
func (s *Wsh) MapsLocal(d, prefix, content, want string) bool {
	m := ProbeFile(d, "m.txt", content, "+map <buffer> Q A!", "+normal Q", "+wq")
	if CatS(m) != want {
		s.Echo("%sa buffer-local mapping stopped working: %s", prefix, CatS(m))
		return false
	}
	return true
}

func (s *Wsh) MapsGlobal(d, prefix string) bool {
	g := ProbeFile(d, "g.txt", "y\n", "+map Z A?", "+normal Z", "+wq")
	if CatS(g) != "y?" {
		s.Echo("%sa global mapping stopped working: %s", prefix, CatS(g))
		return false
	}
	return true
}
