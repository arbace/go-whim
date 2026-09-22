#!/bin/sh
# Build the whimtools binary and print its path.  Run from the repository root.
#
# Usage: bin=$(tools/gobuild.sh)
#
# The C front end is carried the way this tree carries everything else it did
# not write: the pristine module is pinned in go.mod and verified by
# go.sum, and the delta from it is one tracked file,
# tools/patches/cc-v4-c23.patch.  Neither is edited in place.  This script
# composes them -- it copies the pinned module out of the module cache, applies
# the patch to the copy, and builds against that copy through a GENERATED
# modfile, so the tracked go.mod keeps naming the pristine upstream and `go mod
# verify` still means what it says.  `go mod vendor` was the alternative and is
# wrong here: a patched vendor/ fails verification and is silently overwritten
# by the next re-vendor.
#
# Why the patch is needed at all: cc/v4 v4.29.7 has no production for a C23
# attribute in statement position ('[[fallthrough]];', 20 sites in zero-vim.c)
# nor for a label at the end of a compound statement (one site each in
# whim-vim.c and zero-vim.c).  Without it those two products do not parse.  The
# patch touches parser.go only -- no new AST node types, so the `yy` generator
# that writes ast.go never has to run -- and upstream's own test suite is
# unchanged by it, 324 pass either side.
#
# Everything generated lands in .cache/, which is gitignored and content-keyed:
# the key below covers the module files, the patch and every .go file, so a
# change to any of them builds a new binary and a revert restores the old one
# without a rebuild.
set -eu

[ -f go.mod ] && [ -d cmd/whimtools ] || { echo "gobuild: run me from the repository root" >&2; exit 1; }

# The pinned version, read from go.mod rather than written here twice.
# Matched by field rather than by column, because go.mod writes a requirement
# two ways: "require <mod> <ver>" on one line, and "<mod> <ver>" inside a
# require block.
ccver=$(awk '{ for (i = 1; i <= NF; i++) if ($i == "modernc.org/cc/v4") { print $(i + 1); exit } }' go.mod)
[ -n "$ccver" ] || { echo "gobuild: no modernc.org/cc/v4 requirement in go.mod" >&2; exit 1; }

patch=tools/patches/cc-v4-c23.patch
[ -f "$patch" ] || { echo "gobuild: $patch is missing" >&2; exit 1; }

# The key is every tracked input to the build.  find is sorted so the digest
# does not depend on directory order.
# -L: a verify root or an each stage's check root LINKS cmd/ and internal/, and
# find does not follow a starting-point link without it -- so the key covered no
# Go file there, and a root found whatever binary the empty key last named.
key=$(
    {
        cat go.mod go.sum "$patch"
        find -L cmd internal -name '*.go' -type f | LC_ALL=C sort | xargs cat
    } | sha256sum | cut -c1-16
)

bin=.cache/gobin/$key/whimtools
if [ -x "$bin" ]; then
    echo "$bin"
    exit 0
fi

# The patched copy of the front end.  Keyed by the version and the patch alone,
# so editing a tool does not rebuild it.
psha=$(sha256sum "$patch" | cut -c1-12)
fork=.cache/gofork/cc-v4-$ccver-$psha
if [ ! -d "$fork" ]; then
    go mod download modernc.org/cc/v4 >&2
    src=$(go env GOMODCACHE)/modernc.org/cc/v4@$ccver
    [ -d "$src" ] || { echo "gobuild: $src is not in the module cache" >&2; exit 1; }
    tmp=$fork.tmp.$$
    rm -rf "$tmp"
    mkdir -p "$tmp"
    cp -R "$src/." "$tmp/"
    chmod -R u+w "$tmp"
    # --forward so a re-run is not asked to apply an applied patch in reverse.
    patch -p1 -d "$tmp" --forward --silent < "$patch"
    # Rename last: a reader either sees a complete fork or no fork at all.
    #
    # AND ONLY IF IT IS STILL MISSING.  `mv a b` where b is a DIRECTORY that
    # exists moves a INSIDE b, so two concurrent builders would leave
    # $fork/$fork.tmp.NNN and the second one's patched copy where nothing looks
    # for it.  The loser throws its copy away instead; the two are identical by
    # construction, being the same pinned version and the same patch.
    if [ -d "$fork" ]; then rm -rf "$tmp"; else mv "$tmp" "$fork" || rm -rf "$tmp"; fi
fi

# The generated modfile.  Go looks for the matching .sum beside it, so both are
# written; the replace is a directory, whose contents go.sum cannot cover, which
# is exactly why the patch is tracked and hashed into the key above.
# EVERY GENERATED FILE IS WRITTEN UNDER A PRIVATE NAME AND RENAMED, because
# more than one of these runs at once.  tools/zrecord.sh starts six harnesses in
# parallel and each calls tools/st.sh, which calls this, and a verification runs
# its checks beside each other.  Writing fork.mod and fork.sum at their final names
# meant one builder truncating with `cp` what another was already reading.  Two
# units of one cold verification failed that way, 44 of 46 passing around
# them, and NEITHER MESSAGE NAMES CONCURRENCY -- which is why the two states
# were reproduced by hand rather than guessed at, each giving its unit's message
# byte for byte:
#
#   an EMPTY fork.mod                -> `fork.mod: missing module declaration`
#                                       (q83, caught inside `cp go.mod`)
#   fork.mod copied, fork.sum empty  -> `missing go.sum entry for module
#                                       providing package modernc.org/cc/v4`
#                                       (q84, caught between the two `cp`s)
#
# The window is the first few milliseconds of a `go build`, which reads both
# files and then never looks again, so the race is not reproducible on demand --
# 40 staggered cold builders did not fire it once.  The fix is structural for
# exactly that reason: a retry would be tuned against something unmeasurable.
d=.cache/gobin/$key
tmp=$d/.tmp.$$
rm -rf "$tmp"
mkdir -p "$tmp"
mod=$tmp/fork.mod
cp go.mod "$mod"
cp go.sum "$tmp/fork.sum"
printf '\nreplace modernc.org/cc/v4 => %s\n' "$(cd "$fork" && pwd)" >> "$mod"

go build -trimpath -modfile="$(pwd)/$mod" -o "$(pwd)/$tmp/whimtools" ./cmd/whimtools >&2

# A rename within one directory is atomic, so a concurrent reader's `[ -x ]`
# and its exec see either the old binary or the new one and never half of
# either -- and a process already executing the old inode keeps it.
mv -f "$tmp/whimtools" "$bin"
rm -rf "$tmp"

echo "$bin"
