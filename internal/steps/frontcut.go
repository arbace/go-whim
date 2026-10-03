package steps

// FrontCut is front phase n's cuts (1, 2 or 3) without the fall-out
// closure their step wraps them in, and what that closure holds: so that
// the closure can be run apart -- crefactor/graph's FoldX held to
// xform.FallOutOf on the same cut (internal/graphcheck), and the front,
// once its cutters are on the graph, ending there (doc/GRAPH-MIGRATION.md,
// B4).  Phase 1's cut takes the rows phase 1 declares as its arguments,
// as its step does.
func FrontCut(n int) (Step, []string) {
	switch n {
	case 1:
		return front, frontHold
	case 2:
		return front2, frontHold
	case 3:
		return front3, frontHold
	}
	return nil, nil
}
