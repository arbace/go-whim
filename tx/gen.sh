#!/bin/sh
# tx/gen.sh -- generate editor/editor.go from editor.c, the core of whim-vim.c.
#
# Usage: sh tx/gen.sh            write editor/editor.go and tx/sigs.txt
#        sh tx/gen.sh --check    refuse if they are not what the program writes
#
# Run through make, which cuts editor.c first: `make editor/editor.go`, and
# `make whim-editor-check` for --check.  This script never runs make: a check
# must not be able to start a pass.  tx/skel, built against the patched
# modernc.org/cc/v4 that tools/gobuild.sh composes, writes the types, the
# globals, their initial values and every function's body.  crt.go and host.go
# are not generated: they are the runtime and the host.
set -eu
cd "$(dirname "$0")/.."
export TMPDIR="$PWD/.tmp"
mkdir -p .tmp
[ -f editor.c ] || { echo "tx/gen.sh: no editor.c; make cuts it from whim-vim.c: make editor/editor.go"; exit 1; }
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
    cmp -s "$out/editor.go" editor/editor.go || { printf '  %-12s is NOT what tx/skel writes from whim-vim.c.  Run: make editor/editor.go\n' editor.go; fail=1; }
    cmp -s "$out/sigs.txt" tx/sigs.txt || { printf '  %-12s is NOT what tx/skel writes.  Run: make editor/editor.go\n' sigs.txt; fail=1; }
    rm -rf "$out"
    [ $fail = 0 ] && printf '  %-12s is what tx/skel writes from whim-vim.c\n' editor.go
    exit $fail
fi
# written only when it differs, so a current file keeps its mtime and a build
# that depends on it does not run again
changed=0
cmp -s "$out/editor.go" editor/editor.go || { cp "$out/editor.go" editor/editor.go; changed=1; }
cmp -s "$out/sigs.txt" tx/sigs.txt || { cp "$out/sigs.txt" tx/sigs.txt; changed=1; }
rm -rf "$out"
if [ $changed = 1 ]; then
    printf '  %-12s %s lines, generated from whim-vim.c\n' editor.go "$(grep -c '' editor/editor.go)"
else
    printf '  %-12s current -- what tx/skel writes from whim-vim.c\n' editor.go
fi
