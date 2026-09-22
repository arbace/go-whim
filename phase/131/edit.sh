#!/bin/sh
# Whim phase 131 -- the saved input buffer is a garray_T *.  See GOAL.md.
#
# Usage: phase/131/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# get_input_buf() makes a garray_T, casts it to char_u * for
# tasave_T.save_inputbuf, and set_input_buf() casts it back; nothing reads it as
# characters.  The field, the prototypes, the definition and the return say
# what it is (tx/FINDINGS.md, 6).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/131/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/131/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim131 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { echo "  inputbuf      the input binary did not build with '$cflags' '$ldflags'"; exit 1; }
