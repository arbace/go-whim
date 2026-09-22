#!/bin/sh
# Whim phase 132, the check -- nothing frees.
# See phase/132/edit.sh, and GOALS.md.
#
# Usage: phase/132/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim132.go proves the premise from the input (host_free() is
# empty, vim_free() only calls it) and COMPUTES the whole output: the input with
# every call replaced by edit.W132Rule, and vim_free() swept, is the output byte
# for byte.  It reports the blocks the calls leave empty.

# THE BODY IS GO: internal/check/whim132.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/132/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/132/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim132 "$work" "$state"
