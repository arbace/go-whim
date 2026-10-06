package graph

import "github.com/arbace/go-whim/crefactor/cc"

// HostBundle is what a program with no C compiler needs to import path and
// make its FRAGs (doc/LISP-SANDBOX.md, *The editor in the box*): the host
// compiler's configuration and every header the import of src read, as
// cc's Bundle; cc.ReadBundle and cc.SetHost make it NewConfig's answer
// elsewhere.  A fragment's pared unit has the file's include lines, so it
// reads the same headers.  It sets cc's preset while it runs: not to be
// called beside another import.
func HostBundle(path string, src []byte) ([]byte, error) {
	h, err := cc.ProbeHost()
	if err != nil {
		return nil, err
	}
	rec := &cc.RecordFS{}
	h.FS = rec
	cc.SetHost(h)
	defer cc.SetHost(nil)
	if _, _, err := Import(path, src); err != nil {
		return nil, err
	}
	return h.Bundle(rec.Files())
}
