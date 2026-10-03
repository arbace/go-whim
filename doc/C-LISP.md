# The C product in C-lisp: whim-vim.lc

2026-10-03. [go-lisp](GO-LISP.md) writes Go as s-expressions and
[ghc-lisp](GHC-LISP.md) Haskell; C-lisp is the same treatment for C23, and
the third of its kind here. Unlike those two it is not a compiler fork: it is
a converter in `crefactor` (`crefactor/clisp`), C to s-expressions and back,
and the back is C again, which gcc compiles. `go tool whim c2lisp` and
`lisp2c` convert; `c2lisp --check` converts there and back and says `OK` only
when the bytes are the input's; `make whim-vim.lc` is to whim-vim.c what `make
editor.lgo` is to the Go editor. Every form is in `crefactor/clisp/SPEC.md`.

## Why it can be byte for byte

go-lisp's round trip needed three normalisations (comments, field pairs,
parentheses). This one needs none, because of what the pipeline already has:
whim's C has no preprocessor in the core and only `#include`s in the host
(one form each); `crefactor/cc` parses C23; and `crefactor/cemit` prints one
spelling per construct. So the converter reads cemit's tree **case for case**
-- one declarator a declaration, braces always, the source's own parentheses
kept -- and `ToC` is a printer written after cemit's, so that

    ToC(ToLisp(x)) == cemit.Canonical(x)

and on canonical text, which every boundary of the pipeline is, the round trip
is the identity. Two things cemit takes from the source text rather than the
tree -- the include lines and the macro invocations it recovers (`errno`,
`va_arg(ap, int)`, `nullptr`, which the front end predefines as a macro) -- are
exported by cemit (`recover.go`: `Parse`, `Includes`, `Macros`) rather than
written twice; `Canonical` is now `Parse` and `File`, the same code moved.

## The forms, in brief

Declarators are read in order, from the name out: `int (*(*x)[10])(int)` is
`(def x (ptr (array 10 (ptr (fn (int) int)))))`. Specifiers are a base --
`int`, `(const char)`, `(unsigned long)` -- and storage classes before it stand
before the name: `(def static name (ptr (const char)) "a")`, `(typedef usize
(typeof (sizeof 0)))`. A function is `(defn static f (fn ((c int)) bool)
ITEM...)`. Expressions are prefix forms with one head each (`(call f a)`,
`(-> p a b)`, `(index a i)`, `(cast T e)`, `(addr x)`, `(deref p)`, `(post++
i)`, `(+ a b c)` for a left-nested run); the parentheses precedence needs are
not written, the ones it does not need are `(paren ...)`, since cemit keeps
every pair the source wrote. A label is an item before what it labels, so a
switch reads as one. A literal is C's own token text -- `'\''`, `L"x"`,
`0x7fUL` -- never decoded. A macro invocation that is one identifier is that
atom; any other is `(macro "va_arg(ap, int)")`. Typedef names cost nothing:
whether a name is a type is decided by the form it stands in, never by a
symbol table.

From `whim-vim.lc`:

```
(defn static musl_strlen (fn ((s (ptr (const char)))) usize)
  (def a (ptr (const char)) s)
  (for () (deref s) (post++ s) (block))
  (return (cast usize (- s a))))

(def static term_strings (array (paren (ptr char_u))))
```

the second being vim's own `static char_u *(term_strings[]);`, whose
parentheses C does not need.

## What was done, and measured

Against go-whim `9fec73f`, on the 64-core machine:

1. **whim-vim.c round-trips byte for byte.** 2,082,233 bytes and 77,634
   lines become **`whim-vim.lc`, 2,414,213 bytes and 65,605 lines**: 4,667
   top-level forms. `c2lisp` takes 1.25 s, nearly all of it the parse
   (`whim cemit --check` on the same file: 1.4 s); `lisp2c` 0.09 s.
2. **It is the program.** `make whim-vim.lc` converts, holds the round trip
   both ways (`lisp2c --check`: the forms print as whim-vim.c, and whim-vim.c
   converts to the same forms), and compiles the C `lisp2c` prints with the
   one line, `gcc -O0 -fno-stack-protector -static -no-pie -s`, to a binary
   that is **`bin/whim-vim` byte for byte** (775,016 bytes) -- 7 s in all.
   Needing nothing outside this repository, it is the first of the three lisp
   targets that runs on a plain checkout.
3. **The control.** One character changed in `whim-vim.lc` -- the `26` of
   `ascii_isupper`'s `(< (- (cast unsigned c) 'A') 26)` made `27` --
   `lisp2c --check` refuses (*line 4: want `... < 26;`, got `... < 27;`*),
   and the binary built from it differs from `bin/whim-vim`.
   `TestControl` holds the same in the package's tests.
4. **The corpus: every boundary, and the input.** `go tool whim build --keep`
   wrote the 104 boundaries of the pipeline, q000 to q103 (q103 is the
   committed whim-vim.c); `c2lisp --check` on each: **104 of 104 byte for
   byte**, 171 s for all of them and the two below in a row (1.3-3.1 s a
   file to convert, 0.1-0.2 s back). **slim-vim.c is not in cemit's
   spelling** (180,870 lines, the macro residue's spacing): its round trip
   gives cemit's canonical text of it byte for byte (173,594 lines, 4,785,038
   bytes; 2.9 s and 0.2 s), and that canonical text round-trips byte for byte
   as itself, as does q000, which is phase 0's seed -- canonical, and parts
   0a-0c's rename, variadic collapse and attributes on it (173,552 lines,
   148,507 lines and 5.3 MB of forms).
5. **What the corpus exercises.** In `whim-vim.lc`, by count: `block`
   10,393, `->` 9,569, `call` 8,766, `paren` 7,989, `if` 7,539, `ptr` 6,255,
   `def` 6,072, `cast` 4,843, `init` 3,480, `fn` 2,852, `defn` 1,755, `case`
   1,073, `enum` 860, `typeof` 235, `typedef` 184, `goto` 49, `macro` 37 (the
   host's `va_*`), `static_assert` 17, `include` 11, `attr` 6, `literal` 5,
   `alignof-type` 2, `generic` 1; slim-vim.c adds `attr` 358 (306 of them
   `unused` on parameters), `attributed` 37
   (`__attribute__((fallthrough));`), `spec` 3 (a typedef name before a
   qualifier: `xpparam_t const`) and one `verbatim` (its for-declaration of
   two declarators, which cemit prints from the tokens). Neither has a
   bit-field, a case range, a statement expression, a computed goto,
   `alignas`, `_Atomic`, an asm label, `typeof` of a type or a `sizeof`
   without parentheses: the package's tests make each of those and every
   other form, but `macro-decl` (a declaration wholly a macro's) and
   `alignof-bare`.
6. **Nothing else moved.** `make whim-build-check` (with cemit's `Canonical`
   now `Parse` and `File`): every phase reproduces the next and whim-vim.c
   comes back byte for byte, 68 s. `make whim-test`: 80 of 80 as HEAD, the
   Go editor 80 of 80 as the C. gofmt, go vet and staticcheck are clean on
   `crefactor/clisp`, `crefactor/cemit` and `cmd/whim`.
7. **Two cemit bugs, found by writing a second printer after it**, neither in
   the corpus, and fixed in cemit and C-lisp together: `enum hue : long {
   ... }` printed as `enum : long hue`, which gcc refuses, and `- -x` as
   `--x`, a decrement (`- --x` as `---x`, `& &x` as `&&x`). The type now
   follows the tag, and `cemit.Prefix` keeps a prefix `+`, `-` or `&` apart
   from an operand that starts with the same character; both printers call
   it. Tests in both packages, which fail with the fix reverted; the
   pipeline's text does not move (`make whim-build-check` in order and in
   parallel, every editor what its generator writes, `make whim-vim.lc`).

## What it is and is not good for here

- **It is not a translation.** `.lc` is the C's syntax tree in another
  spelling, the same program -- which is why it compiles to the same bytes.
  The six editors are translations; this is not a seventh.
- **It is the parse, as data.** A tool in any language can read the C
  product without a C front end: typedef names are decided, declarators read
  in order, and every token is its own text. That is what a script, a
  diff-by-form, or a query over the tree would want.
- **It is not the representation doc/IR.md considers.** IR.md asks for a
  representation that is typed, its memory and evaluation order resolved, its
  control flow structured, and says *not a pure AST*, which keeps C's
  ambiguities and makes every backend re-derive the answers. C-lisp is
  exactly a pure AST: no expression's type, no conversion written out, no
  pointer class, `goto` still a goto. The Clojure and Scheme backends print
  s-expressions already, but from the lowered form and the shared analyses
  (`cfacts`), which C-lisp does not carry. It could be the *bottom* layer
  of such an IR -- a serialised, parse-free starting point a lift could read
  -- but the backends live in Go beside the typed tree, and would gain
  nothing from reading it back.
- **Its reader is its own.** The atoms are C's tokens, so a Lisp's reader
  will not take an `.lc` as it stands: `'A'` is a quote to Clojure and
  Scheme, `|` a symbol escape to Scheme, and a C string's escapes are not
  theirs. A consumer in a Lisp needs the dozen lines of `Read`, or a reader
  macro.
- **It is a hard test of cemit.** A second printer that must give cemit's
  bytes on 104 texts of up to 173,000 lines checks that cemit's spelling is
  a function of the tree and the two source facts it exports, and nothing
  else -- and found the two bugs of item 7, fixed since.

## Reproduce

```sh
make whim-vim.lc                          # converts, checks both ways, compiles, compares with bin/whim-vim
go tool whim c2lisp --check src/whim-vim.c
go tool whim c2lisp -o F.lc FILE.c        # any C23 file the front end parses
go tool whim lisp2c F.lc                  # and back
go tool whim lisp2c --check F.lc FILE.c   # F.lc prints as FILE.c, and FILE.c converts to F.lc

go tool whim build --keep .tmp/bounds --out .tmp/out.c   # the 104 boundaries
go tool whim c2lisp --check .tmp/bounds/q*.c

cd crefactor && go test ./clisp/          # each form, the reader, the control;
CLISP_CORPUS='../.tmp/bounds/q*.c' go test ./clisp/ -run Corpus   # and a corpus
```
