// Package c23conf is a conformance test of the C front end, the canonical
// printer and C-lisp against C23 (ISO/IEC 9899:2024; WG14's N3220 is the
// working draft of it), with gcc 15 as the oracle.  It has no code: its test
// is the package.
//
// testdata/ holds one small file per language feature C23 added -- the
// keywords, the attributes in every position, _BitInt, the fixed enum, the
// literals, the initializer, the labels, the parameters -- and nothing of the
// preprocessor's, since the pipeline's texts have none.  For each file the
// test asks, in order, and the first no is the file's answer:
//
//  1. gcc accepts it (`gcc -std=c23 -fsyntax-only`), or the file is wrong;
//  2. crefactor/cc parses it;
//  3. crefactor/cemit prints it, and gcc accepts the print;
//  4. the print is a fixed point: cemit of it is the same bytes;
//  5. the print is the same program: the same preprocessing tokens, in the
//     same order (the files are written in the tokens cemit prints, so any
//     token that differs is a change -- a dropped attribute or a respelled
//     constant as much as a changed value), and the same object file under
//     the one compile line (`gcc -std=c23 -O0 -fno-stack-protector -c`);
//  6. C-lisp round trips it: its forms print back as cemit's text, byte for
//     byte, and the text's forms are the same forms.
//
// It also type-checks each file with cc.Translate and reports the answer,
// without judging it: the checker must not panic, and what it says of each
// feature is in the table `go test -v` prints, with the score.
//
// A file that fails is an expected failure only with its reason written
// beside it (expected in conf_test.go); one listed there that passes fails the
// test, so the list is never stale.  doc/C23.md is the account.
package c23conf
