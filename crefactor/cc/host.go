// go-whim: a C configuration given rather than asked of the host's
// compiler, for a program that has none -- a guest (doc/LISP-SANDBOX.md,
// *The editor in the box*).

package cc

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// A Host is what the host's C compiler says -- its name, the macros it
// predefines, its search paths -- and the headers, FS (nil: the host's own
// files).  SetHost makes NewConfig answer it, with no options, without
// asking a compiler.
type Host struct {
	CC              string
	Predefined      string
	IncludePaths    []string
	SysIncludePaths []string
	FS              fs.FS `json:"-"`
}

var preset atomic.Pointer[Host]

// SetHost makes NewConfig, with no options, answer h; nil asks the host's
// compiler again.
func SetHost(h *Host) { preset.Store(h) }

// ProbeHost is what the host's compiler says with no options, FS nil.
func ProbeHost() (*Host, error) {
	cc, pre, inc, sys, _, err := newConfig(nil)
	if err != nil {
		return nil, err
	}
	return &Host{CC: cc, Predefined: pre, IncludePaths: inc, SysIncludePaths: sys}, nil
}

// presetConfig is newConfig's answer from the preset, when there is one
// and no options.
func presetConfig(opts []string) (h *Host, cc, predefined string, includePaths, sysIncludePaths []string, keywords map[string]rune, ok bool) {
	if h = preset.Load(); h == nil || len(opts) != 0 {
		return nil, "", "", nil, nil, nil, false
	}
	keywords = make(map[string]rune, len(defaultKeywords)+len(c23Keywords))
	for k, v := range defaultKeywords {
		keywords[k] = v
	}
	if stdcVersion(h.Predefined) >= 202000 {
		for k, v := range c23Keywords {
			keywords[k] = v
		}
	}
	return h, h.CC, h.Predefined, append([]string(nil), h.IncludePaths...), append([]string(nil), h.SysIncludePaths...), keywords, true
}

// A RecordFS opens the host's files and keeps every one it opened: the
// headers a parse read, for a Host's FS elsewhere.
type RecordFS struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (r *RecordFS) Open(name string) (fs.File, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.files == nil {
		r.files = map[string][]byte{}
	}
	r.files[name] = b
	r.mu.Unlock()
	return &memFile{name: name, b: b}, nil
}

// Files are the files opened, by name.
func (r *RecordFS) Files() map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := make(map[string][]byte, len(r.files))
	for k, v := range r.files {
		m[k] = v
	}
	return m
}

// MemFS is files by their names as cc opens them, absolute paths and all.
type MemFS map[string][]byte

func (m MemFS) Open(name string) (fs.File, error) {
	b, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &memFile{name: name, b: b}, nil
}

type memFile struct {
	name string
	b    []byte
	off  int
}

func (f *memFile) Stat() (fs.FileInfo, error) { return memInfo{f}, nil }
func (f *memFile) Close() error               { return nil }
func (f *memFile) Read(p []byte) (int, error) {
	if f.off >= len(f.b) {
		return 0, io.EOF
	}
	n := copy(p, f.b[f.off:])
	f.off += n
	return n, nil
}

type memInfo struct{ f *memFile }

func (i memInfo) Name() string       { return i.f.name }
func (i memInfo) Size() int64        { return int64(len(i.f.b)) }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }

// hostBundle is a Host and its headers as one JSON document.
type hostBundle struct {
	Host
	Files map[string]string
}

// Bundle is h and files, the headers, as one document; ReadBundle reads
// it back, the headers h's FS.
func (h *Host) Bundle(files map[string][]byte) ([]byte, error) {
	b := hostBundle{Host: *h, Files: map[string]string{}}
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		b.Files[k] = string(files[k])
	}
	return json.Marshal(b)
}

// ReadBundle is the Host a Bundle wrote, its FS the headers.
func ReadBundle(data []byte) (*Host, error) {
	var b hostBundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	if b.Predefined == "" {
		return nil, errorf("not a C host's bundle")
	}
	m := MemFS{}
	for k, v := range b.Files {
		m[k] = []byte(v)
	}
	h := b.Host
	h.FS = m
	return &h, nil
}
