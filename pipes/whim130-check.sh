#!/bin/sh
# Whim phase 130, the check -- the (pos_T *)-1 tests go.
# See pipes/whim130-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim130-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim130.go proves the tests were dead from the input -- every
# (pos_T *)-1 is one of the three tests, no return of getmark(), getmark_buf(),
# getmark_buf_fnum() or movechangelist() can produce it -- counts the lines the
# fold must remove from the input, and probes 'a, `a, :'a and g; on both
# binaries, each with a control that must move.

# THE BODY IS GO: internal/check/whim130.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim130-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim130-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim130 "$work" "$state"
