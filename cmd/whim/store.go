package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/vmm"
)

// runStore is the box's store on the host (vmm.DirStore, doc/LISP-SANDBOX.md,
// *The store*): what a guest run with WHIM_GUEST_STORE=DIR reads and
// writes by its hypercalls, and the Joker guest as the namespace box.
// put stores FILE's bytes and prints the hash; get writes blob HASH to
// stdout; ref prints NAME's hash, or with HASH points NAME at it; graph
// stores FILE's graph as EDN -- a C file imported, or a graph's Lisp read,
// as `whim graph --edn` writes it -- and points NAME at it, so that the
// Joker guest's (box/load NAME) is the EDN its views read; cc stores what
// the host's C compiler says and the headers FILE reads (crefactor/graph's
// HostBundle) and points NAME at it, the compiler the guest's editor
// (ed/start) has.
//
//	whim store DIR put FILE | get HASH | ref NAME [HASH] | graph NAME FILE | cc NAME FILE
func runStore(args []string) int {
	const usage = "usage: whim store DIR put FILE | get HASH | ref NAME [HASH] | graph NAME FILE | cc NAME FILE"
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(os.Stderr, "whim store:", err)
		return 1
	}
	s, err := vmm.OpenDirStore(args[0])
	if err != nil {
		return fail(err)
	}
	hash := func(x string) ([32]byte, error) {
		var h [32]byte
		b, err := hex.DecodeString(x)
		if err != nil || len(b) != 32 {
			return h, fmt.Errorf("not a hash: %s", x)
		}
		copy(h[:], b)
		return h, nil
	}
	switch op := args[1]; {
	case op == "put" && len(args) == 3:
		b, err := os.ReadFile(args[2])
		if err != nil {
			return fail(err)
		}
		h, err := s.Put(b)
		if err != nil {
			return fail(err)
		}
		fmt.Println(hex.EncodeToString(h[:]))
	case op == "get" && len(args) == 3:
		h, err := hash(args[2])
		if err != nil {
			return fail(err)
		}
		n, ok := s.Size(h)
		if !ok {
			return fail(fmt.Errorf("no blob %s", args[2]))
		}
		b := make([]byte, n)
		if k, ok := s.Get(h, 0, b); !ok || int64(k) != n {
			return fail(fmt.Errorf("blob %s unreadable", args[2]))
		}
		os.Stdout.Write(b)
	case op == "ref" && len(args) == 3:
		h, ok := s.Ref(args[2])
		if !ok {
			return fail(fmt.Errorf("no ref %s", args[2]))
		}
		fmt.Println(hex.EncodeToString(h[:]))
	case op == "ref" && len(args) == 4:
		h, err := hash(args[3])
		if err != nil {
			return fail(err)
		}
		if err := s.SetRef(args[2], h, nil); err != nil {
			return fail(err)
		}
	case op == "cc" && len(args) == 4:
		src, err := os.ReadFile(args[3])
		if err != nil {
			return fail(err)
		}
		b, err := graph.HostBundle(args[3], src)
		if err != nil {
			return fail(err)
		}
		h, err := s.Put(b)
		if err != nil {
			return fail(err)
		}
		if err := s.SetRef(args[2], h, nil); err != nil {
			return fail(err)
		}
		fmt.Printf("  store        %s -> %s: the C host for %s, %d bytes\n", args[2], hex.EncodeToString(h[:12]), args[3], len(b))
	case op == "graph" && len(args) == 4:
		src, err := os.ReadFile(args[3])
		if err != nil {
			return fail(err)
		}
		g, err := graphOf(args[3], src)
		if err != nil {
			return fail(err)
		}
		edn, err := g.EDN()
		if err != nil {
			return fail(err)
		}
		h, err := s.Put(edn)
		if err != nil {
			return fail(err)
		}
		if err := s.SetRef(args[2], h, nil); err != nil {
			return fail(err)
		}
		fmt.Printf("  store        %s -> %s: %s's graph, %d bytes of EDN\n", args[2], hex.EncodeToString(h[:12]), args[3], len(edn))
	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	return 0
}
