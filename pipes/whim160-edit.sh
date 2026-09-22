#!/bin/sh
# Whim phase 160 -- no line getter takes a cookie.  See WHIM-GOAL.md.
#
# Usage: pipes/whim160-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# do_cmdline() handed its line getter a void * cookie that every call passed
# as nullptr and no getter read.  It goes, with find_func_t, a typedef nothing
# names: what is left of void * in the core is memory.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim160-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim160-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim160 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' cookie "'$cflags'" "'$ldflags'"; exit 1; }
