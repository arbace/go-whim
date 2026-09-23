# internal/cc — the C front end, forked

modernc.org/cc/v4 v4.29.7, the fifteen non-test files, with two C23 productions
added. Upstream's BSD licence is beside this file and the copyright stays with
The CC Authors.

**The delta is the source here, and nowhere else.** It was a patch under
`tools/patches/` while the fork was composed at build time; once the fork became
tracked source the patch was a second copy of the same change, so it is gone.
To see what this differs from upstream by:

    diff -ru "$(go env GOMODCACHE)/modernc.org/cc/v4@v4.29.7" internal/cc

**It is source here, not a composition.** It used to be assembled at build time:
`tools/gobuild.sh` copied the module out of the cache into `.cache/gofork/`,
patched it, and pointed a generated modfile at it with a `replace` -- because a
patched `vendor/` fails `go mod verify`. A fork under its own import path has
neither problem: the patched source is tracked and reviewable, `go mod verify`
still answers for every dependency that is still upstream's, and nothing has to
be composed before a build can start.

**What the patch adds** is two C23 productions the upstream parser lacks, both
of which the pipeline's input uses: a label as the last thing in a compound
statement (C23 6.8.1, `theend:` with nothing before the `}`), and
`[[fallthrough]];` where a statement is expected. `whimtools parse` is the smoke
test that this fork, and not a pristine one, is what got linked: whim-vim.c does
not parse without them.

**Upstream is still named**, in go.mod's comment and here, so a later version can
be diffed against this tree: take the delta above, copy the new release's
non-test files over, apply it, and let the build and `whimtools parse` say
whether it still holds.
The test files are left behind deliberately -- they pull in modernc.org/ccorpus2,
a corpus this repository has no use for.
