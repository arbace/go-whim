package guest

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

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
