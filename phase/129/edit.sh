#!/bin/sh
# Whim phase 129 -- p_emoji is an int.  See GOAL.md.
#
# Usage: phase/129/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# The first phase that comes of transpiling editor.c to Go (tx/FINDINGS.md, 1).
# 'emoji' is a boolean option, and the options table writes and reads every
# boolean option through an `int *` -- set_option_default() stores
# `*(int *)varp`, do_set_option_bool() and set_bool_option() likewise -- but
# p_emoji was declared `char_u *`.  The C got away with it: the static starts
# zeroed and the int lands in the pointer's low bytes, so utf_char2cells()'s
# `if (p_emoji && ...)` tests the right thing.  The Go transpilation cannot say
# that, and panicked on its first run.  This phase declares the variable what
# every writer and the reader already take it to be.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c: the check's probe runs both.
set -eu

work=${1:?usage: phase/129/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/129/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim129 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { echo "  emoji        the input binary did not build with '$cflags' '$ldflags'"; exit 1; }
