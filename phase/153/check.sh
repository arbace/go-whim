#!/bin/sh
# Whim phase 153, the check -- free_one_termoption() compares without a cast.
# See phase/153/edit.sh, and GOALS.md.
#
# Usage: phase/153/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim153.go proves from the input that both-NULL is the whole
# comparison, requires the new one, and probes the terminal option path.

# THE BODY IS GO: internal/check/whim153.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/153/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/153/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim153 "$work" "$state"
