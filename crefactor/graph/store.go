package graph

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	be "encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// A STORE OF GRAPHS, CONTENT-ADDRESSED.  Many graphs that share most of
// their top-level nodes -- the pipeline's boundaries, a form a phase did not
// touch the same bytes before and after it, since its ids carry across --
// kept as the distinct units of their Lisp (LispUnits), each keyed by the
// SHA-256 of its bytes, and a manifest per graph that lists its units' keys
// in order.  A graph read back is the Lisp written, byte for byte, so it is
// the graph written: the same ids, edges and sections.
//
// The keys are of the bytes, not the content hashes of hash.go: a unit's
// Lisp names its ids and its edges' targets' ids, so a struct's change
// leaves the bytes of the forms that use it as they were, where it moves
// their content hashes (doc/GRAPH.md, *The snapshots as a store*).
//
// In a directory:
//
//	graphs.pack   the units, each once, one after another
//	graphs.idx    "whim graph store 1\n", then for each unit its key, its
//	              offset in the pack and its length (32 + 8 + 4 bytes)
//	NAME.gm       a graph's manifest: "whim graph manifest 1 DIGEST COUNT\n",
//	              DIGEST the SHA-256 of the whole Lisp, then COUNT keys
//
// The reader trusts nothing it is handed: a key the index does not hold, an
// index or pack cut short, or a Lisp whose digest is not the manifest's is
// an error, never a graph.

const (
	storePack    = "graphs.pack"
	storeIdx     = "graphs.idx"
	storeIdxHead = "whim graph store 1\n"
	manifestHead = "whim graph manifest 1 %x %d\n"
	manifestExt  = ".gm"
	idxEntry     = sha256.Size + 8 + 4
)

type span struct {
	off int64
	n   uint32
}

// StoreWriter writes a store: graphs put one after another, the index last.
type StoreWriter struct {
	dir   string
	pack  *os.File
	buf   *bufio.Writer
	off   int64
	have  map[Hash]span
	order []Hash
	// Graphs counts the graphs put, Units their units and New those the
	// pack did not hold; Bytes is the Lisp put, Pack what went into the pack.
	Graphs, Units, New int
	Bytes, Pack        int64
	manifestSize       int64
}

// CreateStore empties dir's store -- its pack, its index and every manifest
// -- and begins a new one.
func CreateStore(dir string) (*StoreWriter, error) {
	if err := RemoveStore(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, storePack))
	if err != nil {
		return nil, err
	}
	return &StoreWriter{dir: dir, pack: f, buf: bufio.NewWriterSize(f, 1<<20), have: map[Hash]span{}}, nil
}

// RemoveStore removes dir's store, what of it there is.
func RemoveStore(dir string) error {
	ms, err := filepath.Glob(filepath.Join(dir, "*"+manifestExt))
	if err != nil {
		return err
	}
	for _, f := range append(ms, filepath.Join(dir, storeIdx), filepath.Join(dir, storePack)) {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Put stores g's Lisp, headed by head (a comment line, or nothing), as the
// graph NAME.
func (w *StoreWriter) Put(name string, head []byte, g *Graph) error {
	return w.PutUnits(name, head, g.LispUnits())
}

// PutUnits is Put of a graph's LispUnits, made already.
func (w *StoreWriter) PutUnits(name string, head []byte, units [][]byte) error {
	w.Graphs++
	if len(head) > 0 {
		units = append([][]byte{head}, units...)
	}
	whole := sha256.New()
	keys := make([]byte, 0, len(units)*sha256.Size)
	for _, u := range units {
		whole.Write(u)
		h := Hash(sha256.Sum256(u))
		keys = append(keys, h[:]...)
		w.Units++
		w.Bytes += int64(len(u))
		if _, ok := w.have[h]; ok {
			continue
		}
		if _, err := w.buf.Write(u); err != nil {
			return err
		}
		w.have[h] = span{w.off, uint32(len(u))}
		w.order = append(w.order, h)
		w.off += int64(len(u))
		w.New++
		w.Pack += int64(len(u))
	}
	m := append(fmt.Appendf(nil, manifestHead, whole.Sum(nil), len(units)), keys...)
	w.manifestSize += int64(len(m))
	return os.WriteFile(filepath.Join(w.dir, name+manifestExt), m, 0o644)
}

// Close writes the pack out and then the index, which makes the store
// readable: until then no graph in it reads.
func (w *StoreWriter) Close() error {
	if err := w.buf.Flush(); err != nil {
		w.pack.Close()
		return err
	}
	if err := w.pack.Close(); err != nil {
		return err
	}
	idx := make([]byte, 0, len(storeIdxHead)+len(w.order)*idxEntry)
	idx = append(idx, storeIdxHead...)
	for _, h := range w.order {
		s := w.have[h]
		idx = append(idx, h[:]...)
		idx = be.BigEndian.AppendUint64(idx, uint64(s.off))
		idx = be.BigEndian.AppendUint32(idx, s.n)
	}
	return os.WriteFile(filepath.Join(w.dir, storeIdx), idx, 0o644)
}

// Discard abandons the store unfinished: with no index, none of it reads.
func (w *StoreWriter) Discard() { w.pack.Close() }

// Size is what the store holds on disk once closed: pack, index and
// manifests.
func (w *StoreWriter) Size() int64 {
	return w.Pack + int64(len(storeIdxHead)+len(w.order)*idxEntry) + w.manifestSize
}

// StoreHas says whether dir holds a manifest for NAME.
func StoreHas(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name+manifestExt))
	return err == nil
}

// ReadStoreLisp is the Lisp stored as NAME in dir, byte for byte as it was
// put, or why it cannot be had.
func ReadStoreLisp(dir, name string) ([]byte, error) {
	m, err := os.ReadFile(filepath.Join(dir, name+manifestExt))
	if err != nil {
		return nil, err
	}
	nl := bytes.IndexByte(m, '\n')
	var digest []byte
	var count int
	if nl > 0 {
		f := bytes.Fields(m[:nl])
		if len(f) == 6 && string(bytes.Join(f[:4], []byte(" "))) == "whim graph manifest 1" {
			digest, _ = hex.DecodeString(string(f[4]))
			count, _ = strconv.Atoi(string(f[5]))
		}
	}
	if len(digest) != sha256.Size || len(m)-nl-1 != count*sha256.Size {
		return nil, fmt.Errorf("%s%s: not a whole manifest", name, manifestExt)
	}
	keys := m[nl+1:]
	idx, err := readIdx(dir)
	if err != nil {
		return nil, err
	}
	spans := make([]span, count)
	total := int64(0)
	for i := range spans {
		var h Hash
		copy(h[:], keys[i*sha256.Size:])
		s, ok := idx[h]
		if !ok {
			return nil, fmt.Errorf("%s%s: unit %d, %s, is not in the store", name, manifestExt, i, h.Short())
		}
		spans[i] = s
		total += int64(s.n)
	}
	pack, err := os.Open(filepath.Join(dir, storePack))
	if err != nil {
		return nil, err
	}
	defer pack.Close()
	out := make([]byte, total)
	// one read for each run of units the pack holds one after another
	at := int64(0)
	for i := 0; i < len(spans); {
		j, end := i+1, spans[i].off+int64(spans[i].n)
		for j < len(spans) && spans[j].off == end {
			end += int64(spans[j].n)
			j++
		}
		n := end - spans[i].off
		if _, err := pack.ReadAt(out[at:at+n], spans[i].off); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("%s: %w", storePack, err)
		}
		at += n
		i = j
	}
	if sum := sha256.Sum256(out); !bytes.Equal(sum[:], digest) {
		return nil, fmt.Errorf("%s%s: the units read are not the graph stored (its digest differs)", name, manifestExt)
	}
	return out, nil
}

// readIdx is dir's index.
func readIdx(dir string) (map[Hash]span, error) {
	b, err := os.ReadFile(filepath.Join(dir, storeIdx))
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(b, []byte(storeIdxHead)) || (len(b)-len(storeIdxHead))%idxEntry != 0 {
		return nil, fmt.Errorf("%s: not a whole index", storeIdx)
	}
	b = b[len(storeIdxHead):]
	idx := make(map[Hash]span, len(b)/idxEntry)
	for ; len(b) > 0; b = b[idxEntry:] {
		var h Hash
		copy(h[:], b)
		idx[h] = span{int64(be.BigEndian.Uint64(b[sha256.Size:])), be.BigEndian.Uint32(b[sha256.Size+8:])}
	}
	return idx, nil
}
