# The Haskell editor in ghc-lisp: caprice.hsl

2026-10-03. [arbace/ghc-lisp](https://github.com/arbace/ghc-lisp) is to GHC
what [go-lisp](GO-LISP.md) is to Go: a fork of the compiler (`10.1.20261002`)
with a second front end, the Haskell language written as s-expressions in
`.hsl` files. The finder takes `M.hsl` where it would take `M.hs`, and after
the parser everything is GHC's; `ghc --hs2lisp X.hs` and `--lisp2hs X.hsl`
convert between the two spellings, and `--lisp-check X.hs` converts a module
there and back and says `OK` when the two parse to the same module. `make
caprice.hsl` is to caprice what `make editor.lgo` is to the Go editor.

## What was done, and measured

At ghc-lisp `f2691c43`, its stage-1 compiler (`_build/stage1/bin/ghc`, whose
package database has every package caprice needs), against go-whim `4dc6392`:

1. **The core is one module.** caprice's `Caprice.Editor` is written in nine
   modules (the Haskell backend's `HsParts`, `doc/HASKELL.md`), since GHC's
   time grows faster than a module's size; `go tool whim hscat` writes it with
   `HsParts` set to 1 instead -- one `Caprice.Editor` of 89,757 lines and its
   hs-boot, the same program unsplit.
2. **It converts, and the round trip holds.** `--hs2lisp` writes it as one
   **`Editor.hsl`, 10.4 MB, 238,584 lines**; `--lisp-check` on the module
   says `OK`. The check is the run's peak: about 25 GB of memory, most of
   its sixteen minutes.
3. **ghc-lisp compiles it, with caprice's runtime, into the editor.** With the
   `.hs` gone, the hs-boot beside it and caprice's own `Rt.hs`, `Host.hs`,
   `Printf.hs`, `Term.hs`, `Run.hs` and `Main.hs` (`caprice.CompileStats`,
   ghc-lisp's ghc in place of GHC): `-O0`, 425 s, 3.69 GB at the peak. At
   caprice's own `-O1` the one module took nine minutes and 22.6 GB before it
   failed (on the boot file, item 4); `-O0` is what `hscat` asks for.
4. **The boot file is Lisp too, where ghc-lisp reads it.** ghc-lisp looks
   for `M.hsl-boot` beside `M.hsl`, but at `f2691c43` parsed only names
   ending in `.hsl` as s-expressions (`isLispFile` in
   `compiler/GHC/Parser/Lisp.hs`), so a converted boot file was read as
   Haskell and refused (*File name does not match module name: Saw:
   `Main`*), and the boot file had to stay Haskell under the `.hsl-boot`
   name. [arbace/ghc-lisp#1](https://github.com/arbace/ghc-lisp/pull/1),
   merged as `1e4cb11c`, has `isLispFile` take `.hsl-boot` too, with a test
   (`lisp012`: two mutually recursive `.hsl` modules through a Lisp boot
   file); `db7adf93` after it lets `ghc -c` take one on its own (`lisp013`;
   the ghclisp testsuite 13 of 13). `hscat` asks the compiler which it is --
   it type-checks two modules whose cycle a Lisp boot file breaks -- and with
   the fix converts and checks `Editor.hs-boot` as it does the module;
   without it, renames the Haskell. Measured at `1e4cb11c`: the boot file
   converted (its round trip OK), the whole run 970 s and 24.9 GB at the
   peak, the compile 432 s and 3.70 GB, and the program again 320 of 320
   cases as `bin/caprice` answers them (item 5).
5. **It is the editor.** `whim test --haskell-bin PROG` runs a caprice
   built already as the Haskell editor, held to the C candidate on every case
   as the one the suite builds is, and `--limit D` lets a run of it take D
   rather than the suite's 10 s: at `-O0`, alone, `par_mmp` takes 43 s and
   four other `par_*` cases that build 3,000 lines by keys 13-18 s, where
   caprice's `-O1` takes a second for all 80. Its control is the program
   itself with its one copy of the literal `" INSERT"` spelled `" INSERX"`
   -- one byte -- so the cases the control moves are that program's own
   answers, not those of a control compiled beside it; the suite refuses a
   program holding the literal other than once. The heavy case times it
   (81-84 times the C's time) but does not hold a program built already to
   its bound of 25. On the program of item 4, `--limit 2m`: the quick suite
   **80 of 80** as the C, its control seen by 76, in 3 min 6 s; the wide
   **240 of 240** (102 keys, 98 Ex commands, 30 command lines, 10 on a
   pseudo-terminal), its control seen by 100, in 46 s. The same program
   with `" VISUAL"` changed instead fails the wide suite on `visual_show`
   and `sel_arrows`; without `--limit` the quick suite stops at `par_branch`,
   *no exit within 10s*. `whim hscat --ghc` runs the quick suite on the
   program it compiled, as above, before it writes the `.hsl`
   (`--no-test` skips it): `make caprice.hsl` at ghc-lisp `db7adf93`, 19 min
   11 s in all, the compile 428 s and 3.70 GB, the suite 80 of 80, its
   control seen by 76, the heavy case 83 times the C's time.

## What it is and is not good for here

- **It is not a fifth translation.** `.hsl` is GHC's syntax tree in another
  spelling: the ghc-lisp editor is caprice, which is why it answers every case
  as caprice does. A `togo` backend writing `.hsl` would be `--hs2lisp` of its
  Haskell output, which it already is.
- **It is a hard test of ghc-lisp.** One generated module of 90,000 lines,
  converted, checked, compiled and run against an independent oracle -- the C
  editor, through caprice -- is a corpus a compiler's own test suite does not
  have; it found the boot-file gap of item 4, fixed in ghc-lisp since.
- **It costs.** Sixteen minutes and 25 GB, and the quick suite's three
  minutes after them, against caprice's three minutes in nine modules: not
  tracked, not part of `all`, and run by `whim test` only when its program
  is handed to it (`--haskell-bin`).

## Reproduce

```sh
git clone --branch ghc-lisp https://github.com/arbace/ghc-lisp.git ../ghc-lisp
(cd ../ghc-lisp && ./boot && ./configure && hadrian/build -j)   # its README; the compiler is _build/stage1/bin/ghc
make caprice.hsl GHCLISP_ROOT=$PWD/../ghc-lisp     # or GHCLISP=path/to/its/ghc
```

`go tool whim hscat` alone prints the one-module `Caprice.Editor` as Haskell;
`--ghc GHC --hsl FILE` converts, checks and compiles it and runs the quick
suite on the program; `--out DIR` keeps the work (the `.hsl`, the runtime, the
program `DIR/caprice`), and the program kept is checked again, either suite, by

```sh
go tool whim test --haskell-bin DIR/caprice --limit 2m          # 80 cases
go tool whim test --wide --haskell-bin DIR/caprice --limit 2m   # 240 cases
```
