package graph

// What a reader of the forms outside this package asks of a declaration --
// crefactor/graph/view's views among them -- exported from the forms'
// syntax (forms.go) without moving it.

// DeclName is the ordinary name a top-level def, typedef or defn declares,
// or "".  It is the name of a local's def and of a parameter's `(NAME
// TYPE)` too, for a def; a parameter's is its first element.
func DeclName(f *Node) string { return topName(f) }

// DeclType is a def's, typedef's or defn's type form, or nil.
func DeclType(f *Node) *Node { return defType(f) }

// Tag is a struct, union or enum form's tag, or "".
func Tag(n *Node) string { return tagOf(n) }

// IsTypeDef says n is a struct, union or enum form that defines its type:
// it has a body.
func IsTypeDef(n *Node) bool { return isDefForm(n) }

// Members is a struct or union definition's members, in order: each
// `(NAME TYPE ...)`, `(TYPE)` for an anonymous struct or union, or a
// `(static_assert ...)`.
func Members(n *Node) []*Node { return members(n) }
