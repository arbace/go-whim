#!/bin/sh
# Whim phase 142, the check -- the version names no build date or time.
# See pipes/whim142-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim142-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim142.go requires __DATE__ and __TIME__ gone, the output
# built at two SOURCE_DATE_EPOCHs identical where the input's differ, and the
# version line a command-line error prints.

# THE BODY IS GO: internal/check/whim142.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim142-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim142-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim142 "$work" "$state"
