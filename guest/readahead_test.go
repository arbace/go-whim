package guest

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/vmm"
)

// scripted is a Host whose input is a script: chunks, each handed out by
// one read as a tty hands out what was typed at once.  The first gate are
// there from the start; the next arrives during the first wait after the
// editor wrote marker (when there is one), as keys typed at that moment;
// each after it arrives during a wait that waits (its ms not 0) when
// nothing else is there, as keys typed while the editor waits for them, or
// at a read that would block.  A SIGWINCH is made pending once chunk
// winchAfter is handed out (as SIGCONT makes one after :suspend), and a
// deadly signal comes at the read that would hand out chunk deadAt --
// before a byte of it, where editor/term delivers one.  Its log is what
// the host was asked, in order: the chunks handed out, the suspension, the
// signals.
type scripted struct {
	recorder
	chunks     []string
	next       int
	arrived    int
	gate       int
	marker     string
	marked     bool
	left       string
	winchAfter int
	deadAt     int
	winch      bool
	deathtrap  func(int32)
	log        []string
}

func newScripted(chunks ...string) *scripted {
	return &scripted{chunks: chunks, arrived: len(chunks), gate: -1, winchAfter: -1, deadAt: -1}
}

// gated has only the first gate chunks there at the start, and the next
// one arrive after marker is written.
func (s *scripted) gated(gate int, marker string) *scripted {
	s.arrived, s.gate, s.marker = gate, gate, marker
	return s
}

func (s *scripted) Init(d func(int32)) { s.deathtrap = d }

func (s *scripted) Suspend() { s.log = append(s.log, "suspend") }

func (s *scripted) Write(p []byte) int32 {
	if s.marker != "" && bytes.Contains(p, []byte(s.marker)) {
		s.marked = true
	}
	return s.recorder.Write(p)
}

// there is whether a read would not block: input, a signal, or the end.
func (s *scripted) there() bool {
	return s.winch || s.left != "" || s.next < s.arrived || s.next == len(s.chunks)
}

func (s *scripted) WaitForInput(ms int64) bool {
	if !s.there() {
		if s.arrived == s.gate && s.marker != "" {
			if s.marked {
				s.arrived++
			}
		} else if ms != 0 {
			s.arrived++
		}
	}
	return s.there()
}

func (s *scripted) ReadInput(buf []byte) int32 {
	if s.winch {
		s.winch = false
		s.log = append(s.log, "winch")
		if len(buf) >= 32 {
			return int32(copy(buf, "\033[48;24;80;0;0t"))
		}
	}
	if s.left == "" {
		if s.next == len(s.chunks) {
			return 0
		}
		if s.next == s.arrived {
			s.arrived++
		}
		if s.next == s.deadAt {
			s.deadAt = -1
			s.log = append(s.log, "TERM")
			s.deathtrap(15)
		}
		s.left = s.chunks[s.next]
		s.log = append(s.log, fmt.Sprintf("%q", s.left))
		if s.next == s.winchAfter {
			s.winch = true
		}
		s.next++
	}
	n := copy(buf, s.left)
	s.left = s.left[n:]
	return int32(n)
}

// The script.  'writedelay' makes every write of the screen wait for input
// and not read it; the keys after :suspend are typed during the wait of one
// of the writes it makes before it stops (stoptermcap's, the marker).  In
// the C they are still the tty's while the editor is stopped; the guests
// have read them already, at that wait, and hand them to the core after.
var (
	aheadKeys   = []string{":set wd=1\r:suspend\r", "ihello\033", "o\x16u263a\033", ":q!\r"}
	aheadMarker = "\x1b[?1004l"
	// ahead is the chunk read ahead across :suspend.
	ahead = fmt.Sprintf("%q", aheadKeys[1])
)

// readAheadRun runs the guest image img and the native Go editor on the
// same script, made by mk, and holds the guest to the native editor's
// output and exit; it returns the two logs.
func readAheadRun(t *testing.T, img []byte, name string, mk func() *scripted) (guest, native []string) {
	t.Helper()
	nat := mk()
	want := editor.Main(nat, []string{"whim"})
	g := mk()
	var stats bytes.Buffer
	code, err := vmm.Run(vmm.Config{Image: img, Host: g, Args: []string{"whim"}, Stats: &stats})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if code != want || g.out.String() != nat.out.String() {
		t.Fatalf("%s: exit %d, %d bytes; the Go editor: exit %d, %d bytes\nguest:  %v\nnative: %v",
			name, code, g.out.Len(), want, nat.out.Len(), g.log, nat.log)
	}
	if !strings.Contains(stats.String(), "call wait_read ") || strings.Contains(stats.String(), "call wait_for_input ") {
		t.Fatalf("%s: the waits not merged:\n%s", name, stats.String())
	}
	t.Logf("%s: exit %d, %d bytes; guest %v; native %v", name, code, g.out.Len(), g.log, nat.log)
	return g.log, nat.log
}

func indexOf(log []string, s string) int {
	for i, l := range log {
		if l == s {
			return i
		}
	}
	return -1
}

// checkReadAhead is the three cases, on one guest image.
//
// suspend: the keys after :suspend sit in the guest's buffer across it --
// read before the host's Suspend, where the native editor reads them after
// -- and still reach the core, in order: the same screens, the same exit.
//
// winch: a signal between -- SIGWINCH pending once the host has handed out
// those keys, which the guest holds unread across :suspend.  The core still
// takes the keys first and the resize after; that is what the native editor
// does with the signal at the same point of the script (after the read of
// those keys), so the output is the same again.
//
// term: a deadly signal during the merged call, at the read: delivered
// before a byte is read, the core's deathtrap run in the guest, and the call
// made again -- the same output and exit as the native editor's.
func checkReadAhead(t *testing.T, img []byte) {
	g, n := readAheadRun(t, img, "suspend", func() *scripted { return newScripted(aheadKeys...).gated(1, aheadMarker) })
	if gi, ni := indexOf(g, ahead), indexOf(n, ahead); gi < 0 || gi > indexOf(g, "suspend") || ni < indexOf(n, "suspend") {
		t.Fatalf("suspend: %s not read ahead across :suspend in the guest, or read before it natively", ahead)
	}
	g, _ = readAheadRun(t, img, "winch", func() *scripted {
		s := newScripted(aheadKeys...).gated(1, aheadMarker)
		s.winchAfter = 1
		return s
	})
	if indexOf(g, ahead) > indexOf(g, "suspend") || indexOf(g, "winch") < indexOf(g, "suspend") {
		t.Fatalf("winch: not the case meant: %v", g)
	}
	readAheadRun(t, img, "term", func() *scripted {
		s := newScripted(aheadKeys...)
		s.deadAt = 1
		return s
	})
}

// TestReadAheadC: the C guest, the core built from src/whim-vim.c.
func TestReadAheadC(t *testing.T) {
	a := Native()
	needTools(t, a)
	c, err := os.ReadFile("../src/whim-vim.c")
	if err != nil {
		t.Skip(err)
	}
	tu, err := Source(c)
	if err != nil {
		t.Fatal(err)
	}
	img, err := Image(tu, a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkReadAhead(t, img)
}

// TestReadAheadGo: the Go guest.
func TestReadAheadGo(t *testing.T) {
	gocmd := needTamaGo(t)
	img, err := GoImage(gocmd, Native(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkReadAhead(t, img)
}

// aheadStandIn stands in for the core to hold the runtime's buffer to its
// rules call by call: a wait reads ahead; a read takes part of it; across a
// suspension a wait answers from it, with no exit, and a read takes the
// rest; then the signal pending since -- its key sequence -- comes from the
// next wait, more keys after it, the end of input (a read's 0, kept as
// the bytes are); and a read with nothing kept is a call of its own.
const aheadStandIn = `
static int vim_main(int argc, char **argv);
static void deathtrap(int sigarg) { (void)sigarg; }
static int musl_wait_for_input(long ms);
static int musl_read_input(char *buf, int len);
static void musl_suspend(void);
static int host_write(const char *s, int len);
static char ahead_out[600];
static int ahead_at;
static void
ahead_take(int n, const char *b)
{
    for (int i = 0; i < n; i++)
        ahead_out[ahead_at++] = b[i];
    ahead_out[ahead_at++] = '|';
}
static int
vim_main(int argc, char **argv)
{
    char b[300];
    int n;
    if (!musl_wait_for_input(-1))
        return 1;
    ahead_take(musl_read_input(b, 3), b);
    musl_suspend();
    if (!musl_wait_for_input(0))
        return 2;
    ahead_take(musl_read_input(b, 250), b);
    for (int i = 0; i < 2; i++)
    {
        if (!musl_wait_for_input(-1))
            return 3;
        ahead_take(musl_read_input(b, 250), b);
    }
    if (!musl_wait_for_input(0))
        return 4;
    n = musl_read_input(b, 250);
    ahead_out[ahead_at++] = (char)('0' + n);
    n = musl_read_input(b, 250);
    ahead_out[ahead_at++] = (char)('0' + n);
    host_write(ahead_out, ahead_at);
    return 0;
}
`

// TestReadAheadRuntime: the C runtime's buffer, call by call, and the exits
// it costs: one a wait that reads, none for a wait or read the buffer
// answers.
func TestReadAheadRuntime(t *testing.T) {
	a := Native()
	needTools(t, a)
	rt, err := sources.ReadFile("rt/rt.c")
	if err != nil {
		t.Fatal(err)
	}
	img, err := Image(append([]byte(aheadStandIn), rt...), a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := newScripted("abcdefgh", "ij")
	h.winchAfter = 0
	var stats bytes.Buffer
	code, err := vmm.Run(vmm.Config{Image: img, Host: h, Args: []string{"ahead"}, Stats: &stats})
	if err != nil {
		t.Fatal(err)
	}
	want := "abc|defgh|\033[48;24;80;0;0t|ij|00"
	if code != 0 || h.out.String() != want {
		t.Fatalf("exit %d, %q, not %q", code, h.out.String(), want)
	}
	// wait_read: abcdefgh, the resize, ij, and the end; read_input: the
	// end again, its own call.
	for _, s := range []string{"exits 8 ", "call wait_read 4\n", "call read_input 1\n", "call suspend 1\n", "calls 8\n"} {
		if !strings.Contains(stats.String(), s) {
			t.Fatalf("not %q:\n%s", s, stats.String())
		}
	}
	if fmt.Sprint(h.log) != `["abcdefgh" suspend winch "ij"]` {
		t.Fatalf("the host's log: %v", h.log)
	}
}
