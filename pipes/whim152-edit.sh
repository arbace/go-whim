#!/bin/sh
# Whim phase 152 -- the option variables are typed.  See WHIM-GOAL.md.
#
# Usage: pipes/whim152-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# vimoption_T.var, optset_T.os_varp, get_varp() and every varp held the address
# of an int, a long or a char_u * as a char_u * (tx/FINDINGS.md, 2).  They are
# an optvar_T, a pointer of each kind and a window-local flag, and a
# window-local option's global value is get_varp_allbuf(), not a byte offset.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim152-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim152-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim152 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' optvar "'$cflags'" "'$ldflags'"; exit 1; }
