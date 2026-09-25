package ccx

import (
	"github.com/arbace/go-whim/internal/cc"
)

// pureFuncs computes the functions that change no state outside their own
// frame: every assignment, increment or decrement they make is to a local
// variable by name (not through a pointer, not a global, not a static), and
// every function they call is itself such a function.  Found to a fixpoint,
// starting from every function and striking out, so recursion is allowed.
// PureFuncs is the set of functions with no effect outside their frame;
// pureCalls are the ones known to have none, defined here or not.
func PureFuncs(ast *cc.AST, pureCalls []string) map[string]bool {
	type fnInfo struct {
		writesOutside bool
		calls         []string
	}
	info := map[string]*fnInfo{}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		fd, ok := n.(*cc.FunctionDefinition)
		if !ok {
			return
		}
		fi := &fnInfo{}
		info[fd.Declarator.Name()] = fi
		local := func(e cc.ExpressionNode) bool {
			p, ok := unparen(e).(*cc.PrimaryExpression)
			if !ok || p.Case != cc.PrimaryExpressionIdent {
				return false
			}
			d, ok := p.ResolvedTo().(*cc.Declarator)
			return ok && d.StorageDuration() == cc.Automatic
		}
		walk(fd.CompoundStatement, "", func(m cc.Node, _ string) {
			switch x := m.(type) {
			case *cc.AssignmentExpression:
				if x.Case != cc.AssignmentExpressionCond && !local(x.UnaryExpression) {
					fi.writesOutside = true
				}
			case *cc.UnaryExpression:
				if (x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec) && !local(x.UnaryExpression) {
					fi.writesOutside = true
				}
			case *cc.PostfixExpression:
				switch x.Case {
				case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
					if !local(x.PostfixExpression) {
						fi.writesOutside = true
					}
				case cc.PostfixExpressionCall:
					name := callee(x)
					if name == "" {
						name = "(indirect)"
					}
					fi.calls = append(fi.calls, name)
				}
			}
		})
	})
	pure := map[string]bool{}
	for k, fi := range info {
		pure[k] = !fi.writesOutside
	}
	for _, k := range pureCalls {
		pure[k] = true
	}
	for changed := true; changed; {
		changed = false
		for k, fi := range info {
			if !pure[k] {
				continue
			}
			for _, c := range fi.calls {
				if !pure[c] {
					pure[k] = false
					changed = true
					break
				}
			}
		}
	}
	return pure
}
