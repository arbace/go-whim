package p099

// Whim phase 99 -- the includes nothing names.  See GOAL.md.
//
// `whim-vim.c` inherited EIGHTEEN preprocessor directives from `whim-vim.c`, every one
// an `#include` of a system header, and thirteen phases removed none of them.  Six are
// now needed by nothing, and this phase takes them.
//
// THREE HAVE BEEN DEAD SINCE BEFORE THE PIPELINE STARTED, and one of the three was
// missed twice:
//
// <sys/stat.h>  supplies nothing.  Its ONE user is `typedef struct stat stat_T;`,
// and nothing uses `stat_T`.  Phase 96 named this header and declined
// it, because GOALS.md's charter stated the directive count as a
// property of the pipeline.
// <fcntl.h>     supplies nothing at all: O_RDONLY, O_WRONLY, O_CREAT, O_APPEND,
// O_NONBLOCK and fcntl() are at zero mentions, and have been since
// phase 92 freed the symbol.
// <iconv.h>     supplies nothing at all, and NOBODY HAD NOTICED: `iconv` occurs
// exactly once in whim-vim.c and that once is its own `#include`
// line.  whim removed the conversion layer and left the header
// behind.  GOALS.md II.4 says "16 directives" for this cut; it is
// 15, and this header is why.
//
// THREE MORE DIED IN PHASES 97 AND 98, which moved what they supplied inside the file:
//
// <string.h>    the sixteen `mem*`/`str*` functions, vendored by phase 97.
// <ctype.h>     TEN identifiers, not five.  isalnum, iscntrl, ispunct, tolower and
// toupper are real calls; isalpha, isdigit, isgraph, islower and
// isupper are musl MACROS -- `#define isalpha(a) (0 ? isalpha(a) :
// (((unsigned)(a)|32)-'a') < 26)` -- so they are in no `nm -u` and a
// survey driven by the symbol list cannot see them.  Phase 98 took
// all ten.
// <wctype.h>    towlower and towupper, and `iswupper`, which was a NAME the header
// had to supply while being no symbol at all: its one occurrence sat
// directly after a `return` inside vim_isupper(), so gcc never
// emitted it.  Phase 98 deleted that statement rather than vendoring
// a function nothing calls.
//
// WHAT THIS PHASE ASSERTS IS NOT THE LIST.  The edit below states, for each of the six,
// every identifier `whim-vim.c` took from it and requires all of them at ZERO -- but
// that is the pre-flight, not the argument.  The argument is in the check, which drops
// each SURVIVING `#include` in turn and requires the compile to fail, and runs the
// identical loop on the source this phase was handed, where it must name exactly these
// six.  A list can go stale; a computation cannot.
//
// THE ONE HEADER THAT MUST NOT BE TOUCHED, and the reason is worth writing down because
// nothing else in the tree records it.  `<sys/param.h>`'s OWN contribution is `MIN` and
// `MAX`.  Everything else it supplies arrives through three levels of musl-internal
// inclusion -- measured with `gcc -E -H`:
//
// sys/param.h -> sys/resource.h -> sys/time.h -> sys/select.h
//
// and `select`, `gettimeofday`, `fd_set`, `FD_SET`, `FD_ZERO`, `FD_ISSET`,
// `struct timeval` and every `*_MAX` are supplied by NO OTHER HEADER IN THIS FILE --
// measured, one probe per identifier against each of the eighteen.  `whim-vim.c` has no
// `<limits.h>`, no `<sys/time.h>` and no `<sys/select.h>`.  That is a real fragility and
// it is recorded rather than repaired: repairing it means ADDING three directives, and
// the charter says no phase adds one.  If musl ever reorganises those headers the build
// breaks outright, which is the loud failure and the acceptable one.
//
// THE TYPEDEF GOES IN THE SAME EDIT AS ITS HEADER, and that is the one thing here that
// is not optional.  `typedef struct stat stat_T;` with no `<sys/stat.h>` COMPILES
// CLEANLY -- it simply declares a new, incomplete `struct stat` at file scope -- and is
// a lie: `sizeof(stat_T)` is then an error.  It is the only silent drop in this file.
//
// AND THE SWEEP CANNOT TAKE IT, for two textual reasons, both measured.
// tools/typereach.py takes as roots every identifier mentioned outside a type
// definition, and this definition's name set is {stat, stat_T}.  It is kept alive by
// (a) update_search_stat()'s local variable `searchstat_T stat;` and (b) THE
// `#include <sys/stat.h>` LINE ITSELF, whose text contains the token `stat`.  Measured:
// typereach.py says `0 unreachable` on the committed file, `0 unreachable` with only the
// include gone, `0 unreachable` with only the local renamed, and `1 unreachable --
// stat,stat_T` only when both are gone.  Thirteen sweeps have left it.  It goes here.
//
// ONE BLANK LINE GOES WITH IT.  The typedef sits between two blank lines, so deleting
// the line alone leaves a run of two, which CLAUDE.md states this tree does not have.
// is handed is already right.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and that is this phase's
// whole evidence.  Nothing below changes a line of code, so the check does not offer
// behavioural probes: it rebuilds the output the same way and requires the two binaries
// to be THE SAME BYTES.  That is tier 1 of CLAUDE.md's verification table, and it
// subsumes every probe a recording could make.  SOURCE_DATE_EPOCH is required because
// version.c's `__DATE__ " " __TIME__` otherwise moves between any two builds -- measured,
// two ordinary builds of the same bytes differ at char 633.  The file name is not
// required: whim-vim.c names no __FILE__ and no __LINE__ -- measured, the same source
// built under two different names is identical.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim99", Edit) }

var whim99Inc = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
var whim99StructStat = regexp.MustCompile(`\bstruct\s+stat\b`)

// whim99Gone is the six headers this phase takes, each with why it is dead and
// the identifiers it supplies that the file was MEASURED not to name.
//
// EVERY LIST IS THE MEASURED SET AND NOTHING SPECULATIVE, and there is a trap
// behind that rule.  An obvious "while we are here" addition to the ctype list
// is `isprint`, `isspace`, `isblank`, `isxdigit` -- and `isprint` OCCURS IN
// THIS FILE, as the name of the 'isprint' option in a string literal.  A list
// written from what a header offers rather than from what this file was
// measured to take refuses on a correct phase, and the message would be about
// ctype.
var whim99Gone = []struct{ header, why, ids string }{
	{"sys/stat.h", "nothing: `struct stat` is named once, in a typedef nothing uses",
		"fstat lstat chmod fchmod ftruncate mkdir umask st_mode st_size st_mtim st_ino " +
			"st_dev S_ISDIR S_IFMT S_IRUSR S_IWUSR"},
	{"fcntl.h", "nothing at all -- phase 92 freed the symbol and left the header",
		"fcntl creat openat O_RDONLY O_WRONLY O_RDWR O_CREAT O_TRUNC O_APPEND O_EXCL " +
			"O_NONBLOCK O_NOFOLLOW F_GETFD F_SETFD F_GETFL F_SETFL FD_CLOEXEC AT_FDCWD"},
	{"iconv.h", "nothing at all -- whim removed the conversion layer and left the " +
		"header, and nobody had noticed",
		"iconv iconv_t iconv_open iconv_close"},
	{"string.h", "the sixteen mem*/str* functions, which phase 97 vendored",
		"memchr memcmp memcpy memmove memset strcasecmp strcat strchr strcmp strcpy " +
			"strlen strncasecmp strncmp strncpy strpbrk strstr"},
	{"ctype.h", "TEN identifiers, which phase 98 vendored -- five real calls and the " +
		"five musl MACROS a survey driven by nm -u cannot see",
		"isalnum isalpha iscntrl isdigit isgraph islower ispunct isupper tolower " +
			"toupper"},
	{"wctype.h", "towlower and towupper, vendored by phase 98, and iswupper, which " +
		"phase 98 deleted: a NAME with no symbol, on a line gcc never emitted",
		"iswupper towlower towupper"},
}

var whim99Keep = []string{"stdio.h", "stdlib.h", "unistd.h", "sys/param.h", "time.h",
	"signal.h", "errno.h", "stdint.h", "stdarg.h", "stddef.h", "sys/ioctl.h", "termios.h"}

// Whim99 removes the six `#include`s nothing names, and the stat_T typedef no
// sweep could take.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "includes", W: w}

	// ---- 0. the file this edit was written against -----------------------
	// EVERY DIRECTIVE IS AN #include OF A SYSTEM HEADER, they are the first
	// lines of the file, and there are eighteen.  GOALS.md's charter is
	// that sentence, and this is where it is checked rather than believed.
	lines := bytes.Split(text, []byte{'\n'})
	var dirIdx []int
	var dirLine []string
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#")) {
			dirIdx = append(dirIdx, i)
			dirLine = append(dirLine, string(l))
		}
	}
	if len(dirIdx) != 18 {
		return nil, p.Die("the file has %d preprocessor directives, expected 18 -- the charter says "+
			"whim-vim.c inherited eighteen from whim-vim.c and that no phase adds one", len(dirIdx))
	}
	for k, i := range dirIdx {
		if i != k {
			return nil, p.Die("the eighteen directives are not the first eighteen lines of the file")
		}
	}
	var headers []string
	for _, l := range dirLine {
		m := whim99Inc.FindStringSubmatch(l)
		if m == nil {
			return nil, p.Die("not an #include of a system header, and no phase may add one: %s",
				cutil.PyRepr(l))
		}
		headers = append(headers, m[1])
	}
	if len(edit.Uniq(headers)) != 18 {
		s := append([]string(nil), headers...)
		sort.Strings(s)
		return nil, p.Die("a header is included twice: %s", strings.Join(s, " "))
	}
	Body := bytes.Join(lines[18:], []byte{'\n'})
	p.Say("eighteen directives, every one an `#include <...>` of a system header and every " +
		"one of them among the first eighteen lines -- the charter, checked rather than " +
		"believed")

	// ---- 1. what whim-vim.c takes from each of the six, and it must be
	// NOTHING.  This is the pre-flight and not the argument: the argument is
	// the check's loop, which drops every surviving include in turn and
	// requires the compile to fail.  A list of identifiers can go stale and a
	// compile cannot.  The list is here because it says WHY each header is
	// dead, which a compiler error does not.
	for _, g := range whim99Gone {
		line := "#include <" + g.header + ">\n"
		if k := bytes.Count(text, []byte(line)); k != 1 {
			return nil, p.Die("`%s` occurs %d times, expected 1", strings.TrimSpace(line), k)
		}
		var live []string
		for _, id := range strings.Fields(g.ids) {
			if n := p.Mentions(Body, id); n > 0 {
				live = append(live, fmt.Sprintf("%s (%d)", id, n))
			}
		}
		if len(live) > 0 {
			return nil, p.Die("<%s> is NOT unused: %s -- this phase removes a header only when "+
				"whim-vim.c names nothing it supplies, and the phase that was to take "+
				"these has not run or did not finish", g.header, strings.Join(live, ", "))
		}
		p.Sayf("<%-12s %s", g.header+">", g.why)
	}

	// THE ONE EXCEPTION, and it is the only silent drop in this file.
	// `struct stat` IS named once, by a typedef, and a typedef of an
	// undeclared struct tag compiles cleanly -- it declares a new, incomplete
	// type -- so removing <sys/stat.h> alone would leave a lie that only
	// `sizeof(stat_T)` could expose.
	if k := p.Mentions(Body, "stat_T"); k != 1 {
		return nil, p.Die("stat_T has %d mentions, expected 1 -- its own typedef and no user", k)
	}
	if k := len(whim99StructStat.FindAll(Body, -1)); k != 1 {
		return nil, p.Die("`struct stat` occurs %d times, expected 1 -- the typedef", k)
	}

	// ---- 2. the twelve that stay, and the one that must not be touched ---
	want := append([]string(nil), whim99Keep...)
	for _, g := range whim99Gone {
		want = append(want, g.header)
	}
	got := append([]string(nil), headers...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return nil, p.Die("the eighteen headers are not the twelve this phase keeps and the six it "+
			"takes: %s", strings.Join(got, " "))
	}
	// TWO OF THE TWELVE ARE HELD BY ALMOST NOTHING, and those are counted
	// exactly, because the count IS the statement: a later phase that took
	// them would find this check's loop reporting a dead include.  The other
	// ten are checked for presence only -- an exact count of `select` or
	// `stderr` here would be a number this phase has no argument for.
	for _, thin := range []struct {
		Name string
		want int
	}{{"SIZE_MAX", 1}, {"offsetof", 9}, {"uintptr_t", 1}} {
		if k := p.Mentions(Body, thin.Name); k != thin.want {
			return nil, p.Die("%s has %d mentions, expected %d -- it is one of the two or three things "+
				"holding its header, so the count is the statement", thin.Name, k, thin.want)
		}
	}
	for _, name := range []string{"MIN", "MAX", "select", "gettimeofday", "va_arg",
		"tcgetattr", "ioctl", "nanosleep", "errno", "malloc", "printf", "sigaction"} {
		if p.Mentions(Body, name) == 0 {
			return nil, p.Die("%s is named nowhere, so a header this phase KEEPS may be dead too -- "+
				"the check drops every survivor in turn and would say which", name)
		}
	}
	p.Say("the twelve that stay, each held by something this file still names: <stddef.h> " +
		"by offsetof alone, <stdint.h> by SIZE_MAX and by the uintptr_t phase 97 brought, " +
		"and <sys/param.h> by MIN and " +
		"MAX -- plus, through sys/resource.h -> sys/time.h -> sys/select.h and no other " +
		"header in this file, select, gettimeofday, fd_set, struct timeval and every " +
		"*_MAX.  That chain is musl's and is recorded rather than repaired: repairing it " +
		"means ADDING <limits.h>, <sys/time.h> and <sys/select.h>, and no phase adds a " +
		"directive")

	// ---- 3. the cut: six lines, the typedef, and one blank ---------------
	runsBefore := p.BlankRuns(text)
	for _, g := range whim99Gone {
		text = bytes.Replace(text, []byte("#include <"+g.header+">\n"), nil, 1)
	}
	p.Say("the six `#include` lines")

	// The typedef AND one of its two blank lines: deleting the line alone
	// leaves a run of two blank lines, which CLAUDE.md states this tree does
	// not have.  canon.sh in the sweep would collapse it; doing it here means
	// the text the sweep is handed is right.
	const old = "\ntypedef struct stat stat_T;\n\n"
	if k := bytes.Count(text, []byte(old)); k != 1 {
		return nil, p.Die("the stat_T typedef is not one line between two blank lines, so the blank "+
			"that goes with it cannot be identified: %d matches", k)
	}
	text = bytes.Replace(text, []byte(old), []byte("\n"), 1)
	p.Say("and `typedef struct stat stat_T;` with one of its two blank lines -- the sweep " +
		"has never been able to take it, because typereach.py reads the token `stat` in " +
		"the `#include <sys/stat.h>` line itself, and in update_search_stat()'s local " +
		"variable, as roots")

	// ---- 4. what the file is now -----------------------------------------
	lines = bytes.Split(text, []byte{'\n'})
	dirIdx, dirLine = nil, nil
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#")) {
			dirIdx = append(dirIdx, i)
			dirLine = append(dirLine, string(l))
		}
	}
	bad := len(dirIdx) != 12
	for k, i := range dirIdx {
		if i != k {
			bad = true
		}
	}
	if bad {
		var at []string
		for _, i := range dirIdx {
			at = append(at, strconv.Itoa(i))
		}
		return nil, p.Die("the file does not have exactly twelve directives on its first twelve lines "+
			"after the cut: %d directives at lines %s", len(dirIdx), strings.Join(at, " "))
	}
	var left []string
	for _, l := range dirLine {
		if m := whim99Inc.FindStringSubmatch(l); m != nil {
			left = append(left, m[1])
		} else {
			left = append(left, l)
		}
	}
	if strings.Join(left, "\x00") != strings.Join(whim99Keep, "\x00") {
		return nil, p.Die("the twelve that are left are not the twelve this phase keeps, in order")
	}
	Body = bytes.Join(lines[12:], []byte{'\n'})
	if p.Mentions(Body, "stat_T") != 0 {
		return nil, p.Die("stat_T survives the cut")
	}
	if whim99StructStat.Match(Body) {
		return nil, p.Die("`struct stat` survives the cut, and with no <sys/stat.h> it would be an " +
			"incomplete type nothing declares")
	}
	if r := p.BlankRuns(text); r != runsBefore {
		return nil, p.Die("the cut left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	p.Sayf("twelve directives, every one an `#include <...>`, on the first twelve lines; "+
		"stat_T and `struct stat` at zero; and %d runs of two blank lines, exactly as "+
		"before", p.BlankRuns(text))
	return text, nil
}
