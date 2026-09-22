#!/bin/sh
# Whim phase 138 -- no parameter carries an eval value.  See GOAL.md.
#
# Usage: phase/138/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# vim_regsub_both's expr, match_add's pos_list, cursor_pos_info's dict, the
# formatter's tvs and find_ex_command's Vim9 lookup and context are passed
# nullptr by every call.  They go, each test of them folds, and the sweep takes
# typval_T, lists, dicts, type_T, class_T and the rest of the eval values.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/138/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/138/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim138 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' evalparm "'$cflags'" "'$ldflags'"; exit 1; }
