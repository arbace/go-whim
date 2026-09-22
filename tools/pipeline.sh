# The pipeline, and the only place that states its parameters.
#
# Sourced, not run:  . tools/pipeline.sh [whim]
#
#   whim-vim.c = G(slim-vim.c)       GOALS.md, work in .tmp/whim-stage/
#
# One pipeline, and the driver, the boundaries, the oracle and the synthesiser
# still take it as an argument, so a second one would be added here.
#
# The boundary tag is q (arbace/slim-vim's own pipeline, which produces this one's
# input, tags its boundaries p).
#
# A PHASE IS A DIRECTORY, phase/NNN/, numbered with three digits so that they
# sort: its program (make.sh, or edit.sh and check.sh), its GOAL.md and its
# declared delta.  phasedir N names it.  N is a plain integer everywhere else: a
# padded name in shell arithmetic is octal, and 010 is 8.
#
# PSOURCE is the one file a sweep runs on, which is what lets tools/phaserun.sh
# run a split phase's sweep itself.
#
# PDELTA is the checker a stage's declared delta goes through: tools/phaserun.sh
# runs it and tools/implhash.sh hashes it.  There are TWO SETS OF BASELINES, and
# ZERO_FROM is the line between them.  Phases before it are measured against
# .reference/baselines, recorded by phase 0 from slim-vim.c; phases from it on
# against .reference/zero-baselines, recorded by phase ZERO_FROM from the tree it
# is handed, with an instrument (tools/zrecord.sh) that an editor with no file to
# write can still be measured by.  tools/whimdelta.sh hands a phase from
# ZERO_FROM on to tools/zerodelta.sh.  Every phase declares its delta the same
# way, in its own directory; tools/declared.sh reads a run of them.
#
# THE PHASE LIST IS NOT WRITTEN HERE, and that is measured rather than tidy.  This
# file is hashed into every stage's key and every edit's (it is named by
# tools/phaserun.sh, which every split phase's key reads), so a byte changed here
# re-keys the whole pipeline.  A list written here would do that every time a
# phase is added.  It is the `phases` line of phase/stages instead, which no key
# reads.

phasedir() { printf 'phase/%03d' "$1"; }

case ${1:-whim} in
    whim) PIPE=whim; TAG=q; IMPL=whim
          PWORK=.tmp/whim-stage; PBUILD=.build; PSOURCE=whim-vim.c; PDELTA=tools/whimdelta.sh
          ZERO_FROM=83
          PHASE_LIST=$(awk '$1 == "phases" { $1 = ""; sub(/^ +/, ""); print }' phase/stages) ;;
    *)    echo "pipeline: no such pipeline: $1" >&2; return 1 2>/dev/null || exit 1 ;;
esac
