#!/bin/sh
# Whim phase 158, the check -- a highlight's terminal font is read only from a colour entry.
# See pipes/whim158-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim158-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim158.go measures the layout that made the read harmless,
# requires internal/ccx's Unions to leave nothing, and probes both paths.

# THE BODY IS GO: internal/check/whim158.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim158-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim158-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim158 "$work" "$state"
