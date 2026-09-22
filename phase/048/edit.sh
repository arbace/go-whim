#!/bin/sh
# Whim phase 48 -- no :noswapfile.  See GOAL.md.
#
# Usage: phase/048/edit.sh <work-dir> <state-dir>      (run from the repository root)
#
# There has been no swap file since Phase 21: the memfile is memory.  The
# modifier set CMOD_NOSWAPFILE, and its two readers, in ml_open() and
# buf_copy_options(), were already empty blocks.  The modifier is matched by name
# before the table, so its branch goes as well as its row.
#
# THE DELTA: the row, which succeeded run bare.
set -eu

work=${1:?usage: phase/048/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

tools/st.sh retire "$f" noswapfile
tools/st.sh edit whim48 "$f"

# tools/phaserun.sh sweeps next, then runs phase/048/check.sh.
