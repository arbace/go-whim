#!/bin/sh
# Whim phase 138, the check -- no parameter carries an eval value.
# See pipes/whim138-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim138-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim138.go proves each parameter was only tested or handed on
# and always nullptr, requires every eval value type gone, and probes g CTRL-G,
# a \= substitution and :match against controls.

# THE BODY IS GO: internal/check/whim138.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim138-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim138-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim138 "$work" "$state"
