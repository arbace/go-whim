#!/bin/sh
# Whim phase 133, the check -- one buffer needs no hash table.
# See pipes/whim133-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim133-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim133.go proves from the input that curbuf is the one buffer
# or NULL and buflist_findnr() has one caller, requires the table and b_key
# gone, and probes the marks setmark_pos() reaches through buflist_findnr().

# THE BODY IS GO: internal/check/whim133.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim133-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim133-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim133 "$work" "$state"
