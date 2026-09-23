#!/bin/sh
# tx/gen.sh -- generate editor/editor.go from editor.c, the core of whim-vim.c.
#
# Usage: sh tx/gen.sh            write editor/editor.go and tx/sigs.md
#        sh tx/gen.sh --check    refuse if they are not what the program writes
#
# Run through make, which cuts editor.c first: `make editor/editor.go`, and
# `make whim-editor-check` for --check.  This script never runs make: a check
# must not be able to start a pass.  tx/skel, built against the patched
# internal/cc, the forked C front end, writes the types, the
# globals, their initial values and every function's body.  crt.go and host.go
# are not generated: they are the runtime and the host.
set -eu
cd "$(dirname "$0")/.."
export TMPDIR="$PWD/.tmp"
mkdir -p .tmp
[ -f editor.c ] || { echo "tx/gen.sh: no editor.c; make cuts it from whim-vim.c: make editor/editor.go"; exit 1; }
# The front end is internal/cc, a tracked fork, so this is an ordinary build.
go build -trimpath -o .tmp/gen-skel ./tx/skel
out=$(mktemp -d)
.tmp/gen-skel editor.c "$out" -editor "$out/editor.go" 2>/dev/null
if [ "${1:-}" = --check ]; then
    fail=0
    cmp -s "$out/editor.go" editor/editor.go || { printf '  %-12s is NOT what tx/skel writes from whim-vim.c.  Run: make editor/editor.go\n' editor.go; fail=1; }
    cmp -s "$out/sigs.md" tx/sigs.md || { printf '  %-12s is NOT what tx/skel writes.  Run: make editor/editor.go\n' sigs.md; fail=1; }
    rm -rf "$out"
    [ $fail = 0 ] && printf '  %-12s is what tx/skel writes from whim-vim.c\n' editor.go
    exit $fail
fi
# written only when it differs, so a current file keeps its mtime and a build
# that depends on it does not run again
changed=0
cmp -s "$out/editor.go" editor/editor.go || { cp "$out/editor.go" editor/editor.go; changed=1; }
cmp -s "$out/sigs.md" tx/sigs.md || { cp "$out/sigs.md" tx/sigs.md; changed=1; }
rm -rf "$out"
if [ $changed = 1 ]; then
    printf '  %-12s %s lines, generated from whim-vim.c\n' editor.go "$(grep -c '' editor/editor.go)"
else
    printf '  %-12s current -- what tx/skel writes from whim-vim.c\n' editor.go
fi
