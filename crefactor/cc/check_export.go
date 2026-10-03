package cc // import "github.com/arbace/go-whim/crefactor/cc"

import "fmt"

// Check type checks n, a tree Parse returned, as Translate checks the tree
// its own parse made, and leaves in it whatever it resolved -- the types, a
// PrimaryExpression's ResolvedTo, a selection's Field, an Initializer's
// Field -- whether or not the check succeeds.  The error is the check's.
//
// (go-whim: added, for crefactor/graph, which imports texts the pipeline
// holds between a phase's edit and its sweep: such a text parses, and may
// not type check -- a use of a member the edit removed -- and what does
// resolve in it is still worth having.  Translate refuses such a text whole.)
func (n *AST) Check(cfg *Config) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("type check: %v", e)
		}
	}()
	c, err := n.check(cfg)
	if c != nil {
		c.cfg = nil
	}
	return err
}
