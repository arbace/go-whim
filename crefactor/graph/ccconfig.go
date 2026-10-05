package graph

import (
	"sync"

	"github.com/arbace/go-whim/crefactor/cc"
)

// ccConfig is cc's configuration for linux/amd64, made once a process: a
// new one asks the host's C compiler for its predefined macros and include
// paths, a process of its own -- 40-50 ms of every FRAG an edit through a
// view made, which a profile of the process's own CPU does not see.  Each
// caller gets a copy of its own; what it shares (the paths, the macros'
// text, the ABI) cc reads and does not write.
func ccConfig() (*cc.Config, error) {
	ccBase.once.Do(func() { ccBase.cfg, ccBase.err = cc.NewConfig("linux", "amd64") })
	if ccBase.err != nil {
		return nil, ccBase.err
	}
	c := *ccBase.cfg
	return &c, nil
}

var ccBase struct {
	once sync.Once
	cfg  *cc.Config
	err  error
}
