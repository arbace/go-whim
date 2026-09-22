#!/bin/sh
# Whim phase 141, the check -- regrepeat() does not jump into a case.
# See pipes/whim141-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim141-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim141.go reads the opcode -> assignment table from both
# sides and requires it equal, the loop byte for byte, and no goto; it probes
# all eighteen classes against a control.

# THE BODY IS GO: internal/check/whim141.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim141-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim141-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim141 "$work" "$state"
