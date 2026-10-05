package main

import (
	"errors"

	"github.com/arbace/go-whim/guest/abi"
	"github.com/arbace/go-whim/guest/tamago/board"
)

// callStore is the monitor's store (vmm/store.go) by its hypercalls, for
// the namespace box: without a store every call answers -1.
type callStore struct{}

type conflict struct{}

func (conflict) Error() string  { return "the ref is not what was said" }
func (conflict) Conflict() bool { return true }

var errStore = errors.New("the store refused")

func (callStore) Put(data []byte) ([32]byte, error) {
	var h [32]byte
	if board.Call(abi.BlobPut, board.Addr(data), int64(len(data)), board.Addr(h[:]), 0, 0).Ret != 0 {
		return h, errStore
	}
	return h, nil
}

func (callStore) Size(h [32]byte) (int64, bool) {
	n := board.Call(abi.BlobSize, board.Addr(h[:]), 0, 0, 0, 0).Ret
	return n, n >= 0
}

func (callStore) Get(h [32]byte, off int64, p []byte) (int, bool) {
	n := board.Call(abi.BlobGet, board.Addr(h[:]), board.Addr(p), int64(len(p)), off, 0).Ret
	return int(n), n >= 0
}

func (callStore) Ref(name string) ([32]byte, bool) {
	var h [32]byte
	b := []byte(name)
	return h, board.Call(abi.RefGet, board.Addr(b), int64(len(b)), board.Addr(h[:]), 0, 0).Ret == 0
}

func (callStore) SetRef(name string, h [32]byte, old *[32]byte) error {
	b := []byte(name)
	o := int64(0)
	if old != nil {
		o = board.Addr(old[:])
	}
	switch board.Call(abi.RefSet, board.Addr(b), int64(len(b)), board.Addr(h[:]), o, 0).Ret {
	case 0:
		return nil
	case -2:
		return conflict{}
	}
	return errStore
}
