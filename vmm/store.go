package vmm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// THE STORE (doc/LISP-SANDBOX.md, *The store*): the box's data, kept on the
// host by the monitor, so that what a guest reads and writes is no file of
// the host's but blobs and refs.  A blob is immutable, named by its
// SHA-256; a ref is a name pointing at a blob, the only state that
// changes, by compare-and-set when a guest asks.  The guest's calls
// (guest/abi's BlobPut, BlobSize, BlobGet, RefGet, RefSet) are the
// monitor's, not the Host's.

// A Store keeps blobs and refs.  Its methods are called from any vCPU.
type Store interface {
	Put(data []byte) ([32]byte, error)
	Size(h [32]byte) (int64, bool)
	Get(h [32]byte, off int64, p []byte) (int, bool)
	Ref(name string) ([32]byte, bool)
	// SetRef points name at h; when old is not nil, only if the ref is old
	// now (all zero: if it does not exist), else ErrConflict.
	SetRef(name string, h [32]byte, old *[32]byte) error
}

// ErrConflict is a ref's compare-and-set that failed.  It says Conflict(),
// which is how a store's user that cannot import vmm (guest/joker/box)
// tells it from other errors.
var ErrConflict error = conflict{}

type conflict struct{}

func (conflict) Error() string  { return "the ref is not what the guest said it was" }
func (conflict) Conflict() bool { return true }

// RefName says a ref's name is one the store takes: 1 to 64 of
// [A-Za-z0-9._-], not beginning with a dot.
func RefName(name string) bool {
	if len(name) < 1 || len(name) > 64 || name[0] == '.' {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// A DirStore keeps the store in a directory of the host's: blobs/HASH,
// refs/NAME (the hash in hex).  It makes only the system calls the
// monitor's filter allows a running guest (openat, read, write, fstat,
// close): a blob is written to its own name, created exclusively, and
// held to its hash when first read -- one a write left short (the guest
// ended, the disk filled) is refused, not served; a ref, 65 bytes, is
// written in one write.  Blobs read are kept, verified, while the monitor
// runs: a graph's EDN is read in many BlobGets.
type DirStore struct {
	dir   string
	mu    sync.Mutex
	cache map[[32]byte][]byte // blobs read and found whole
}

// OpenDirStore is the store in dir, its directories made when missing --
// before the monitor's system-call filter, which allows no mkdir.
func OpenDirStore(dir string) (*DirStore, error) {
	for _, d := range []string{"blobs", "refs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return nil, fmt.Errorf("the store: %w", err)
		}
	}
	return &DirStore{dir: dir, cache: map[[32]byte][]byte{}}, nil
}

func (s *DirStore) blob(h [32]byte) string {
	return filepath.Join(s.dir, "blobs", hex.EncodeToString(h[:]))
}

func (s *DirStore) Put(data []byte) ([32]byte, error) {
	h := sha256.Sum256(data)
	if _, ok := s.load(h); ok {
		return h, nil // content-addressed: there already, and whole
	}
	f, err := os.OpenFile(s.blob(h), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return h, err // a short one from before is there: refused until removed by hand
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return h, err
}

// load is blob h, read once and held to its hash.
func (s *DirStore) load(h [32]byte) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.cache[h]; ok {
		return b, true
	}
	b, err := os.ReadFile(s.blob(h))
	if err != nil || sha256.Sum256(b) != h {
		return nil, false
	}
	s.cache[h] = b
	return b, true
}

func (s *DirStore) Size(h [32]byte) (int64, bool) {
	b, ok := s.load(h)
	return int64(len(b)), ok
}

func (s *DirStore) Get(h [32]byte, off int64, p []byte) (int, bool) {
	b, ok := s.load(h)
	if !ok || off < 0 {
		return 0, false
	}
	if off >= int64(len(b)) {
		return 0, true
	}
	return copy(p, b[off:]), true
}

func (s *DirStore) Ref(name string) ([32]byte, bool) {
	var h [32]byte
	if !RefName(name) {
		return h, false
	}
	b, err := os.ReadFile(filepath.Join(s.dir, "refs", name))
	if err != nil {
		return h, false
	}
	n, err := hex.Decode(h[:], bytes.TrimSpace(b))
	return h, err == nil && n == 32
}

var refMu sync.Mutex // refs' compare-and-set, across stores of one process

func (s *DirStore) SetRef(name string, h [32]byte, old *[32]byte) error {
	if !RefName(name) {
		return fmt.Errorf("ref %q: not a name the store takes", name)
	}
	if _, ok := s.load(h); !ok {
		return fmt.Errorf("ref %s: no blob %x", name, h[:4])
	}
	refMu.Lock()
	defer refMu.Unlock()
	if old != nil {
		cur, ok := s.Ref(name)
		if ok != (*old != [32]byte{}) || ok && cur != *old {
			return ErrConflict
		}
	}
	return os.WriteFile(filepath.Join(s.dir, "refs", name), []byte(hex.EncodeToString(h[:])+"\n"), 0o644)
}

// storeCall answers a store's call; -1 when the machine has no store.
func (m *machine) storeCall(nr uint64, a [5]int64) (int64, error) {
	st := m.cfg.Store
	if st == nil {
		return -1, nil
	}
	at := func(addr, n int64, what string) ([]byte, error) {
		p, ok := m.guest(addr, n)
		if !ok {
			return nil, m.fault(fmt.Sprintf("a store's %s at %#x+%d, outside the guest's memory", what, addr, n))
		}
		return p, nil
	}
	hashAt := func(addr int64) ([32]byte, []byte, error) {
		p, err := at(addr, 32, "hash")
		if err != nil {
			return [32]byte{}, nil, err
		}
		return [32]byte(p), p, nil
	}
	switch nr {
	case callBlobPut:
		data, err := at(a[0], a[1], "blob")
		if err != nil {
			return 0, err
		}
		_, out, err := hashAt(a[2])
		if err != nil {
			return 0, err
		}
		h, perr := st.Put(data)
		if perr != nil {
			return -1, nil
		}
		copy(out, h[:])
		return 0, nil
	case callBlobSize:
		h, _, err := hashAt(a[0])
		if err != nil {
			return 0, err
		}
		if n, ok := st.Size(h); ok {
			return n, nil
		}
		return -1, nil
	case callBlobGet:
		h, _, err := hashAt(a[0])
		if err != nil {
			return 0, err
		}
		buf, err := at(a[1], a[2], "buffer")
		if err != nil {
			return 0, err
		}
		if n, ok := st.Get(h, a[3], buf); ok {
			return int64(n), nil
		}
		return -1, nil
	case callRefGet:
		name, err := at(a[0], a[1], "ref's name")
		if err != nil {
			return 0, err
		}
		_, out, err := hashAt(a[2])
		if err != nil {
			return 0, err
		}
		h, ok := st.Ref(string(name))
		if !ok {
			return -1, nil
		}
		copy(out, h[:])
		return 0, nil
	case callRefSet:
		name, err := at(a[0], a[1], "ref's name")
		if err != nil {
			return 0, err
		}
		h, _, err := hashAt(a[2])
		if err != nil {
			return 0, err
		}
		var old *[32]byte
		if a[3] != 0 {
			o, _, err := hashAt(a[3])
			if err != nil {
				return 0, err
			}
			old = &o
		}
		switch err := st.SetRef(string(name), h, old); {
		case err == nil:
			return 0, nil
		case errors.Is(err, ErrConflict):
			return -2, nil
		default:
			return -1, nil
		}
	}
	return -1, nil
}
