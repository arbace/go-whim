package editor

import (
	"bytes"
	"testing"
)

// fakeHost is a Host with no operating system under it: keys from a slice,
// the screen into a buffer, a fixed size, and Exit as the panic Main
// recovers -- what an embedding program would write.
type fakeHost struct {
	keys   []byte
	screen bytes.Buffer
	msgs   bytes.Buffer
}

func (f *fakeHost) Init(func(int32))              {}
func (f *fakeHost) WinSize() (int32, int32, bool) { return 24, 80, true }
func (f *fakeHost) TermStart()                    {}
func (f *fakeHost) TermStop()                     {}
func (f *fakeHost) TTYKeys(int32) (int32, int32, bool, bool, bool) {
	return 0x7f, 3, true, true, true
}
func (f *fakeHost) NowMs() int64             { return 0 }
func (f *fakeHost) Time() int64              { return 0 }
func (f *fakeHost) Delay(int64, bool)        {}
func (f *fakeHost) WaitForInput(int64) bool  { return len(f.keys) > 0 }
func (f *fakeHost) Raise(int32)              {}
func (f *fakeHost) Suspend()                 {}
func (f *fakeHost) Exit(code int32)          { panic(Exit(code)) }
func (f *fakeHost) Message(m []byte, _ bool) { f.msgs.Write(m) }
func (f *fakeHost) Write(p []byte) int32     { f.screen.Write(p); return int32(len(p)) }
func (f *fakeHost) ReadInput(buf []byte) int32 {
	n := copy(buf, f.keys)
	f.keys = f.keys[n:]
	return int32(n)
}

// The editor runs in this process on a Host of the test's own: it types
// text, draws it, and quits with its status handed back by Main -- not by
// ending the process.  One run: the core's state is the package's, so a
// process holds one editor (AGENDA.md, step 3, is the instance).
func TestMainOnAFakeHost(t *testing.T) {
	h := &fakeHost{keys: []byte("ihello, embedded\x1b:q!\r")}
	status := Main(h, []string{"whim"})
	if status != 0 {
		t.Fatalf("status %d, want 0; messages: %q", status, h.msgs.String())
	}
	if !bytes.Contains(h.screen.Bytes(), []byte("hello, embedded")) {
		t.Errorf("the screen never showed the typed text; it wrote %d bytes: %q", h.screen.Len(), h.screen.String())
	}
}
