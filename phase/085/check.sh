#!/bin/sh
# Whim phase 85, the check -- the terminal warnings, the pause and --ttyfail.
# See phase/085/edit.sh, and GOALS.md.
#
# Usage: phase/085/check.sh <work-dir> <state-dir>     (run from the repository root)
#
# Runs after phase/085/edit.sh and the sweep tools/phaserun.sh runs between them,
# and reads nothing from the edit's shell -- only the work tree and the state
# directory, as tools/phaserun.sh describes.  What the edit left there is `old`, the
# binary this phase was HANDED, built from the boundary's own makefile flags.
#
# THE PROBES ARE THIS PHASE'S EVIDENCE, and they are two-sided for the reason phase
# 84's symbol check is: the three harnesses tools/zerodelta.sh runs cannot see this
# cut at all -- behaviour.py and exsweep.py run the editor `-e -s`, where
# exmode_active takes check_tty()'s first branch, and termcheck.py drives a real pty
# where neither stream is a file.  So the declared delta is legitimately "none", and
# a delta of none from a blind harness proves nothing on its own.  Every probe below
# therefore requires the OLD binary to do the thing and the new one not to.

# THE BODY IS GO: internal/check/whim85.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/085/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/085/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim85 "$work" "$state"
