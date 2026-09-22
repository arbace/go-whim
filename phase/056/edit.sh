#!/bin/sh
# Whim phase 56 -- no shell, runtime or keyword-program options.  See GOAL.md.
#
# Usage: phase/056/edit.sh <work-dir> <state-dir>      (run from the repository root)
#
# Six options whose readers survive only in machinery with nothing to serve:
#
#   'shell', 'shellquote', 'shellredir'  no shell is ever run -- call_shell() and
#       mch_call_shell() went long ago.  'shell' only chose the default of
#       'shellredir' in set_init_3() and whether filename escaping doubled a `!`
#       for csh; 'shellquote' only wrapped do_bang()'s command line.
#   'runtimepath', 'packpath'  there is no runtime to find.  Their readers are the
#       completion of :colorscheme, :compiler, :ownsyntax, :setfiletype, :packadd
#       and :runtime -- every one of them ex_ni -- and of :set ft=, which listed
#       runtime syntax/indent/ftplugin names.
#   'keywordprg'  K is gone; only :set kp= defaulting to :help read it.
#
# THE DELTA: none the harnesses record.  The probes check the six are unknown.
set -eu

work=${1:?usage: phase/056/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

tools/st.sh edit whim56 "$f"

tools/st.sh dropoptions "$f" shell shellquote shellredir runtimepath packpath
tools/st.sh dropoptions "$f" --local keywordprg

# No sweep here.  One stood here, and the lines after it were written for swept text,
# but this phase and every stage it has run in reproduce their boundaries without
# it (GOALS.md, *The inner sweeps*; phase/stages) -- the stage's one sweep does its work.
# get_varp()'s "local if set" case for 'keywordprg' is written &curbuf->b_p_kp,
# without the parentheses droplocal.py's pattern expects, so its two mentions
# would read as readers.  It is plumbing, and goes by hand first.
tools/st.sh edit whim56kp "$f"
tools/st.sh droplocal "$f" b_p_kp

# tools/phaserun.sh sweeps next, then runs phase/056/check.sh.
