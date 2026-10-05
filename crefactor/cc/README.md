# crefactor/cc — the C front end, forked

modernc.org/cc/v4 v4.29.7, the fifteen non-test files and `c23.go` of its
own, with the C23 productions upstream's parser refuses added (`c23.go`, and
hooks in upstream's files, each marked `go-whim`), one lookup and the type check of a parsed tree exported, three faults
corrected, and what the host's C compiler says asked once a process
(`newConfig` memoized by its options, each `NewConfig` given copies: the
compiler's process was 40-50 ms of every edit whim's program graph made). Upstream's BSD licence is beside this file and the copyright stays
with The CC Authors.

**The delta is the source here, and nowhere else.** It was a patch under
`tools/patches/` while the fork was composed at build time; once the fork became
tracked source the patch was a second copy of the same change, so it is gone.
To see what this differs from upstream by:

    diff -ru "$(go env GOMODCACHE)/modernc.org/cc/v4@v4.29.7" crefactor/cc

**It is source here, not a composition.** It used to be assembled at build time:
`tools/gobuild.sh` copied the module out of the cache into `.cache/gofork/`,
patched it, and pointed a generated modfile at it with a `replace` -- because a
patched `vendor/` fails `go mod verify`. A fork under its own import path has
neither problem: the patched source is tracked and reviewable, `go mod verify`
still answers for every dependency that is still upstream's, and nothing has to
be composed before a build can start.

**What the patch adds** is C23 (ISO/IEC 9899:2024, the draft N3220) where
upstream's parser stops at C17 and GNU. `crefactor/c23conf` is the
conformance test that holds each to gcc 15, one file per feature, through
this parser, `crefactor/cemit`'s print and C-lisp (`doc/C23.md`). The new
functions are in `c23.go`; upstream's files carry only the hooks that call
them, and a field or a case where a node must hold what it did not:

- **A label as the last thing in a compound statement** (C23 6.8.2, `theend:`
  with nothing before the `}`), labeling a synthesised null statement
  (`labeledStatementBody`). The pipeline's input uses it, and `whim parse`
  is the smoke test that this fork, and not a pristine one, got linked.
- **Attributes, `[[...]]`, in every position** (6.7.13): held in the node
  GNU's `__attribute__((...))` is (`AttributeSpecifier`, `IsStd()` tells them
  apart; '[' '[' in Token and Token2, ']' ']' in Token4 and Token5), wherever
  a GNU list is accepted -- a declaration's specifiers, after a declarator,
  a parameter, a member, a pointer's `*`, a struct or union -- and where it
  is not: after a declarator's identifier (`DirectDeclarator`), between
  `enum` and its tag (`EnumSpecifier`), after an enumerator (`Enumerator`,
  GNU's too, which upstream parsed and discarded), after a function
  definition's declarator (`FunctionDefinition`), and before a statement:
  an expression or null statement's own (`[[fallthrough]];`), any other's
  in its `Statement`. A namespace prefix is `AttributeValue`'s `Prefix`,
  `Colon` and `Colon2`; a standard or `gnu::` attribute's arguments are an
  expression list, as GNU's are, another vendor's its tokens
  (`BalancedTokenSequence`). Where upstream parses a GNU list and discards
  it, it still parses GNU's alone (`gnuAttributeSpecifierListOpt`), so a
  `[[` there is a syntax error and never a silent drop. Until this, `[[...]]`
  parsed only in a statement's place, and was discarded there: whim's
  boundaries were printed without phase 0c's 37 `[[fallthrough]];`, a bare
  `;` in their place, until the pipeline was changed to carry them
  (`doc/C23.md`).
- **`constexpr`** (6.7.2), a storage-class specifier (`CONSTEXPR`,
  `StorageClassSpecifierConstexpr`).
- **`_BitInt(N)`** (6.7.3.1), a type specifier (`BITINT`,
  `TypeSpecifierBitInt`, N in `ExpressionList`); the `wb` and `uwb` suffixes
  are pp-numbers already.
- **`typeof_unqual`** (6.7.3.6), typeof's production, told apart by its
  text; and a typeof of a type name that begins with a qualifier,
  `typeof(const int)`, which upstream refused.
- **`static_assert` without a message** (6.7.12); and a block's static
  assertion owns its `;`, which upstream left a null statement after it --
  one more each time the text was printed.
- **Digit separators** (6.4.4, `1'000'000`) in the scanner's pp-number, and
  read through by the checker's constants.
- **`u8'a'`** (6.4.4.5), a character constant of type unsigned char.
- **Storage-class specifiers in a compound literal** (6.5.3.6, `(static
  int[]){1, 2}`), in `PostfixExpression.StorageClassSpecifiers`.

**What the type checker does with them** (`Translate`): `constexpr` is read
as no storage class (the object is not made const, its value is not a
constant expression to it); `_BitInt` is an error, *a bit-precise integer
type is parsed, not type-checked* -- the type system has no such kind, and
inventing one is beyond a fork's hooks; `typeof_unqual` drops the
qualifiers; a separator and `u8` are read through. It panics on none of the
conformance files (the test holds it to that).

And the faults. A tagged struct or union definition's trailing attributes
(`struct s { ... } __attribute__((packed))`) are stored in
`AttributeSpecifierList2`, as the untagged definition's are. Upstream stored
them in `AttributeSpecifierList`, over the leading ones, so `struct
__attribute__((packed)) s { ... }` reached a printer, and the type checker,
without its attribute (`crefactor/cemit`'s test of it; none is in whim's input).
A GNU attribute list of three or more keeps them all: upstream linked every
value after the first to the first, so a third replaced the second
(`TestGNUAttributeListKept`; whim's input has no such list). The block's
static assertion, above.

And one method, `Scope.Declares` (end of `parser.go`): the scope an identifier
resolves to where it is written, asked of the parse alone. The sweep
(`crefactor/sweep`) needs it to tell a local from the global it shadows without
type-checking a text that need not type-check; the checker's own lookup is
unexported and resolves further than a scope.

And `AST.Check` (`check_export.go`): the type check `Translate` runs after its
parse, on a tree `Parse` returned, leaving what it resolved -- types, an
identifier's declaration, a selection's field -- in the tree whether or not it
succeeds. `crefactor/graph` imports texts the pipeline holds between an edit
and its sweep, which parse and need not type-check; `Translate` refuses such a
text whole.

**Upstream is still named**, in go.mod's comment and here, so a later version can
be diffed against this tree: take the delta above, copy the new release's
non-test files over, apply it, and let the build and `whim parse` say
whether it still holds.
The test files are left behind deliberately -- they pull in modernc.org/ccorpus2,
a corpus this repository has no use for.
