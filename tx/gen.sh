#!/bin/sh
# tx/gen.sh -- generate editor/editor.go from whim-vim.c.
#
# Usage: sh tx/gen.sh            write editor/editor.go and tx/sigs.txt
#        sh tx/gen.sh --check    refuse if they are not what the program writes
#
# make editor.c cuts the core out of whim-vim.c; tx/skel, built against the
# patched modernc.org/cc/v4 (tools/gobuild.sh composes it), writes the types,
# the globals, their initial values and every function's body.  crt.go and
# host.go are not generated: they are the runtime and the host.
set -eu
cd "$(dirname "$0")/.."
export TMPDIR="$PWD/.tmp"
mkdir -p .tmp
make -s editor.c >/dev/null
sh tools/gobuild.sh >/dev/null
# the fork tools/gobuild.sh composed: named by the pinned version and the patch
ccver=$(awk '{ for (i = 1; i <= NF; i++) if ($i == "modernc.org/cc/v4") { print $(i + 1); exit } }' go.mod)
fork=.cache/gofork/cc-v4-$ccver-$(sha256sum tools/patches/cc-v4-c23.patch | cut -c1-12)
[ -d "$fork" ] || { echo "tx/gen.sh: $fork is missing; tools/gobuild.sh composes it"; exit 1; }
sed "\$a replace modernc.org/cc/v4 => ./$fork" go.mod > .tmp/gen-fork.mod
cp go.sum .tmp/gen-fork.sum
go build -trimpath -modfile=.tmp/gen-fork.mod -o .tmp/gen-skel ./tx/skel
out=$(mktemp -d)
.tmp/gen-skel editor.c "$out" -editor "$out/editor.go" 2>/dev/null
if [ "${1:-}" = --check ]; then
    fail=0
    cmp -s "$out/editor.go" editor/editor.go || { echo "tx/gen.sh: editor/editor.go is not what tx/skel writes from whim-vim.c"; fail=1; }
    cmp -s "$out/sigs.txt" tx/sigs.txt || { echo "tx/gen.sh: tx/sigs.txt is not what tx/skel writes"; fail=1; }
    rm -rf "$out"
    [ $fail = 0 ] && echo "tx/gen.sh: editor/editor.go and tx/sigs.txt are what tx/skel writes"
    exit $fail
fi
cp "$out/editor.go" editor/editor.go
cp "$out/sigs.txt" tx/sigs.txt
rm -rf "$out"
echo "tx/gen.sh: wrote editor/editor.go ($(wc -l < editor/editor.go) lines) and tx/sigs.txt"
