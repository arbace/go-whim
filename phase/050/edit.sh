#!/bin/sh
# Whim phase 50 -- only LF text files.  See GOAL.md.
#
# Usage: phase/050/edit.sh <work-dir> <state-dir>      (run from the repository root)
#
# Every line ends with LF when it is read and when it is written, and a CR is a
# character like any other.  -b goes, and with it 'binary', 'fileformat',
# 'fileformats', 'endofline', 'fixendofline', 'endoffile', 'textmode' and
# 'textauto', and the ++bin, ++nobin and ++ff arguments.  See `lfonly`.
#
# THE DELTA: no Ex command; the behaviour cases ff_dos and binary_mode, whose
# :set ff=dos and :set binary are refused now.
set -eu

work=${1:?usage: phase/050/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

tools/st.sh dropopts "$f" -b
tools/st.sh lfonly "$f"
# The rows go before the sweep and without --strict: their callbacks, and the
# format functions the sweep has not taken yet, still read them.  The
# post-condition below is the check.
tools/st.sh dropoptions "$f" --local binary fileformat endofline fixendofline endoffile textmode
tools/st.sh dropoptions "$f" fileformats textauto

tools/sweep.sh "$f"
tools/st.sh droplocal "$f" b_p_bin b_p_ff b_p_fixeol b_p_tx

# tools/phaserun.sh sweeps next, then runs phase/050/check.sh.
