#!/bin/sh
# Whim phase 139, the check -- the core sorts and searches typed arrays.
# See pipes/whim139-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim139-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim139.go proves each typed search is the input's
# musl_bsearch() with its byte steps typed, requires the generic sort and
# search gone, and probes each search and the sort against controls.

# THE BODY IS GO: internal/check/whim139.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim139-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim139-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim139 "$work" "$state"
