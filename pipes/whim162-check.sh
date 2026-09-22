#!/bin/sh
# Whim phase 162, the check -- no two function pointers are compared.
# See pipes/whim162-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim162-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim162.go requires internal/ccx's FuncCompares to leave
# nothing, the flag set exactly where getexline is passed, and probes @:.

# THE BODY IS GO: internal/check/whim162.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim162-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim162-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim162 "$work" "$state"
