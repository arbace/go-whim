package guest

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/vmm"
)

// recorder is a Host that keeps what the guest wrote and ends by Exit's
// panic, as an embedding host does.
type recorder struct{ out bytes.Buffer }

func (r *recorder) Init(func(int32))                               {}
func (r *recorder) WinSize() (int32, int32, bool)                  { return 24, 80, true }
func (r *recorder) TermStart()                                     {}
func (r *recorder) TermStop()                                      {}
func (r *recorder) TTYKeys(int32) (int32, int32, bool, bool, bool) { return 127, 3, true, true, true }
func (r *recorder) NowMs() int64                                   { return 0 }
func (r *recorder) Time() int64                                    { return 0 }
func (r *recorder) Delay(int64, bool)                              {}
func (r *recorder) WaitForInput(int64) bool                        { return true }
func (r *recorder) ReadInput([]byte) int32                         { return 0 }
func (r *recorder) Raise(int32)                                    {}
func (r *recorder) Suspend()                                       {}
func (r *recorder) Exit(code int32)                                { panic(editor.Exit(code)) }
func (r *recorder) Message(m []byte, _ bool)                       { r.out.Write(m) }
func (r *recorder) Write(p []byte) int32                           { r.out.Write(p); return int32(len(p)) }

func needTools(t *testing.T, a Arch) {
	t.Helper()
	for _, tool := range []string{"clang", a.LD} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Skip("no /dev/kvm")
	}
}

// TestHello is milestone 1's gate: the runtime with a stand-in core writes
// "hello" by host_write and exits 3 by host_exit -- one exit for each of the
// two calls, and no other.
func TestHello(t *testing.T) {
	a := Native()
	needTools(t, a)
	tu, err := Hello()
	if err != nil {
		t.Fatal(err)
	}
	img, err := Image(tu, a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &recorder{}
	var stats bytes.Buffer
	code, err := vmm.Run(vmm.Config{Image: img, Host: h, Args: []string{"hello"}, Stats: &stats})
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 || h.out.String() != "hello\n" {
		t.Fatalf("exit %d, output %q", code, h.out.String())
	}
	if !strings.HasPrefix(stats.String(), "exits 2 ") || !strings.Contains(stats.String(), "calls 2\n") {
		t.Fatalf("not one exit per call:\n%s", stats.String())
	}
}

// faulty stands in for the core with the two faults the C editor would die
// of a SIGSEGV for: a null pointer written, and a stack run through its
// guard page.
const faulty = `
static int vim_main(int argc, char **argv);
static void deathtrap(int sigarg) { (void)sigarg; }
static int deep(volatile char *p) { volatile char b[4096]; b[0] = *p; return deep(b) + b[1]; }
static volatile int spinning = 1;
static int
vim_main(int argc, char **argv)
{
    if (argc > 1 && argv[1][0] == 'd')
        return deep("x");
    if (argc > 1 && argv[1][0] == 's')
        while (spinning)
            ;
    *(volatile int *)0 = 1;
    return 0;
}
`

// TestFault: an exception the guest takes is reported by the runtime's own
// call and ends the run as a Fault: a page fault, vector 14, at the address
// it touched; and the stack's overflow too, on the vectors' stack.
func TestFault(t *testing.T) {
	a := Native()
	needTools(t, a)
	rt, err := sources.ReadFile("rt/rt.c")
	if err != nil {
		t.Fatal(err)
	}
	img, err := Image(append([]byte(faulty), rt...), a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ arg, want string }{
		{"null", "exception 14 (code 0x2)"},
		{"deep", "exception 14 (code 0x2)"},
	} {
		_, err := vmm.Run(vmm.Config{Image: img, Host: &recorder{}, Args: []string{"faulty", c.arg}})
		var f *vmm.Fault
		if !errors.As(err, &f) || !strings.Contains(f.Msg, c.want) {
			t.Fatalf("%s: %v, not a fault with %q", c.arg, err, c.want)
		}
		if c.arg == "null" && !strings.Contains(f.Msg, "address 0x0") {
			t.Fatalf("%s: %s", c.arg, f.Msg)
		}
		t.Logf("%s: %s", c.arg, f.Msg)
	}
}

// TestWatchdog: a guest that runs without a hypercall is ended by the
// monitor's watchdog, as hv_vcpus_exit ends it on the framework.
func TestWatchdog(t *testing.T) {
	a := Native()
	needTools(t, a)
	rt, err := sources.ReadFile("rt/rt.c")
	if err != nil {
		t.Fatal(err)
	}
	img, err := Image(append([]byte(faulty), rt...), a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = vmm.Run(vmm.Config{Image: img, Host: &recorder{}, Args: []string{"faulty", "spin"}, Watchdog: 200 * time.Millisecond})
	var f *vmm.Fault
	if !errors.As(err, &f) || !strings.Contains(f.Msg, "watchdog") {
		t.Fatalf("%v, not the watchdog", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("the watchdog took %s", d)
	}
}

func needTamaGo(t *testing.T) string {
	t.Helper()
	gocmd, err := TamaGo("")
	if err != nil {
		t.Skip(err)
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Skip("no /dev/kvm")
	}
	return gocmd
}

// TestGoGuest: the Go guest -- the Go editor built with TamaGo -- on a
// recording host answers exactly as the Go editor does natively on the same
// host: the same bytes and the same exit code, every call a hypercall.
func TestGoGuest(t *testing.T) {
	gocmd := needTamaGo(t)
	img, err := GoImage(gocmd, Native(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"whim"}
	native := &recorder{}
	want := editor.Main(native, args)
	h := &recorder{}
	var stats bytes.Buffer
	code, err := vmm.Run(vmm.Config{Image: img, Host: h, Args: args, Stats: &stats})
	if err != nil {
		t.Fatal(err)
	}
	if code != want || h.out.String() != native.out.String() {
		t.Fatalf("exit %d, %d bytes; the Go editor: exit %d, %d bytes", code, h.out.Len(), want, native.out.Len())
	}
	if !strings.Contains(stats.String(), "call random 1\n") {
		t.Fatalf("no random bytes asked for:\n%s", stats.String())
	}
	t.Logf("exit %d, %d bytes; %s", code, h.out.Len(), strings.SplitN(stats.String(), "\n", 2)[0])
}

// TestGoGuestFault: the board's own paths -- a null pointer written ends
// the run as a Fault through its vectors, as the C guest's; a panic is the
// runtime's report on the console and exit 2; a spin is ended by the
// watchdog; and a line printed, then exit 7.
func TestGoGuestFault(t *testing.T) {
	gocmd := needTamaGo(t)
	img, err := GoProgram(gocmd, "./faulty", Native(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		arg, fault, out string
		code            int
	}{
		{"null", "exception 14 (code 0x2)", "", 0},
		{"panic", "", "panic: faulty: panic", 2},
		{"spin", "watchdog", "", 0},
		{"", "", "faulty: hello\n", 7},
	} {
		h := &recorder{}
		code, err := vmm.Run(vmm.Config{Image: img, Host: h, Args: []string{"faulty", c.arg}, Watchdog: 500 * time.Millisecond})
		var f *vmm.Fault
		switch {
		case c.fault != "":
			if !errors.As(err, &f) || !strings.Contains(f.Msg, c.fault) {
				t.Fatalf("%s: %v, not a fault with %q", c.arg, err, c.fault)
			}
			t.Logf("%s: %s", c.arg, f.Msg)
		case err != nil || code != c.code || !strings.Contains(h.out.String(), c.out):
			t.Fatalf("%s: exit %d, %v, output %q", c.arg, code, err, h.out.String())
		}
	}
}
