#!/bin/sh
# Whim phase 136, the check -- the engine is called directly.
# See pipes/whim136-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim136-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim136.go proves every program's engine was bt_regengine,
# requires the table, the type and the field gone, and probes a search and two
# substitutions against a control that matches nothing.

# THE BODY IS GO: internal/check/whim136.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim136-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim136-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim136 "$work" "$state"
