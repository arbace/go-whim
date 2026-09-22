#!/bin/sh
# Whim phase 140 -- highlight groups are found in their array.  See GOAL.md.
#
# Usage: phase/140/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# syn_name2id_len() found a group's id by subtracting a key's offset in
# hlname_T (tx/FINDINGS.md, 3).  Names are unique and the id is the index + 1,
# so the lookup scans highlight_ga; highlight_ht was the last hash table, and
# the sweep takes the hash table code.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/140/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/140/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim140 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' hlname "'$cflags'" "'$ldflags'"; exit 1; }
