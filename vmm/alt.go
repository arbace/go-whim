package vmm

import "github.com/arbace/go-whim/hv"

// isAltExit is whether an exit is a call made some other way than the
// doorbell (milestone 5's alternatives): none yet.
func isAltExit(hv.Syndrome, uint64) bool { return false }
