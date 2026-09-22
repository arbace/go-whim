#!/bin/sh
# Whim phase 140, the check -- highlight groups are found in their array.
# See phase/140/edit.sh, and GOALS.md.
#
# Usage: phase/140/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim140.go proves the scan finds what the table found, requires
# the hash table gone, and probes group lookup, a taken-back group and a link.

# THE BODY IS GO: internal/check/whim140.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/140/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/140/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim140 "$work" "$state"
