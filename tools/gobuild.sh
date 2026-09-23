#!/bin/sh
# Build the whimtools binary and print its path.  Run from the repository root.
#
# Usage: bin=$(tools/gobuild.sh)
#
# NOTHING IS COMPOSED ANY MORE.  The C front end used to be assembled here --
# the pinned modernc.org/cc/v4 copied out of the module cache, patched into
# .cache/gofork/, and built against through a generated modfile, because a
# patched vendor/ fails `go mod verify`.  It is a fork now, tracked source in
# internal/cc with the patch applied in place (internal/cc/README.md), so this
# script builds the module as it stands.
#
# What is left is the content key: .cache/ is gitignored and every binary lives
# under the digest of what went into it, so a change to any input builds a new
# binary and a revert finds the old one without a rebuild.
set -eu

[ -f go.mod ] && [ -d cmd/whimtools ] || { echo "gobuild: run me from the repository root" >&2; exit 1; }

# The key is every tracked input to the build.  find is sorted so the digest
# does not depend on directory order.
#
# PHASE/ IS IN IT, and that is not optional: every phase is a package of its own
# there, linked in through phase/registry.go, so a binary built before a phase
# changed is a binary that runs the old phase.  Measured, when phase/ was left
# out: editing a check changed nothing a run could see, because tools/st.sh kept
# handing back the binary the unchanged key named.
#
# -L: a verify root LINKS cmd/, internal/ and phase/, and find does not follow a
# starting-point link without it -- so the key covered no Go file there, and a
# root found whatever binary the empty key last named.
key=$(
    {
        cat go.mod go.sum
        find -L cmd internal phase -name '*.go' -type f | LC_ALL=C sort | xargs cat
    } | sha256sum | cut -c1-16
)

bin=.cache/gobin/$key/whimtools
if [ -x "$bin" ]; then
    echo "$bin"
    exit 0
fi

# EVERY GENERATED FILE IS WRITTEN UNDER A PRIVATE NAME AND RENAMED, because more
# than one of these runs at once: tools/zrecord.sh starts six harnesses in
# parallel and each calls tools/st.sh, which calls this, and a verification runs
# its checks beside each other.  A rename within one directory is atomic, so a
# concurrent reader's `[ -x ]` and its exec see either the old binary or the new
# one and never half of either -- and a process already executing the old inode
# keeps it.
d=.cache/gobin/$key
tmp=$d/.tmp.$$
rm -rf "$tmp"
mkdir -p "$tmp"

go build -trimpath -o "$(pwd)/$tmp/whimtools" ./cmd/whimtools >&2

mv -f "$tmp/whimtools" "$bin"
rm -rf "$tmp"

echo "$bin"
