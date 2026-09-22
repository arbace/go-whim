#!/bin/sh
# Whim phase 133 -- one buffer needs no hash table.  See WHIM-GOAL.md.
#
# Usage: pipes/whim133-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# Whim left one buffer, so buf_hashtab held one entry and buflist_findnr(), its
# one reader, recovered the buffer from the key inside it by subtracting the
# key's offset (tx/FINDINGS.md, 3).  buflist_findnr() becomes "the current
# buffer, if its number is nr"; the table's init, add and remove go, and the
# sweep takes the helpers, the table and b_key.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim133-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim133-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim133 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' hash "'$cflags'" "'$ldflags'"; exit 1; }
