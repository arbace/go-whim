# The pipeline, and the only place that states its parameters.
#
# Sourced, not run:  . tools/pipeline.sh [whim]
#
#   whim-vim.c = G(slim-vim.c)       WHIM-GOAL.md, work in whim/
#
# There was a second pipeline, zero, which took the committed whim-vim.c to an
# embeddable core.  It is phases 83 onwards of this one now -- zero phase N is whim
# phase N+83 -- and the driver, the boundaries, the oracle and the synthesiser
# still take the pipeline as an argument, so a second one would be added here.
#
# The boundary tag is q (arbace/slim-vim's own pipeline, which produces this one's
# input, tags its boundaries p; the old zero pipeline tagged its own r).
#
# PSOURCE is the one file a sweep runs on, which is what lets tools/phaserun.sh
# run a split phase's sweep itself.
#
# PDELTA is the checker a stage's declared delta goes through: tools/phaserun.sh
# runs it and tools/implhash.sh hashes it.  There are TWO SETS OF BASELINES and
# two delta files, and ZERO_FROM is the line between them.  Phases before it
# declare their delta in pipes/whim.delta against .reference/baselines, recorded
# by phase 0 from slim-vim.c; phases from it on declare theirs in pipes/zero.delta
# against .reference/zero-baselines, recorded by phase ZERO_FROM from the tree it is
# handed, with an instrument (tools/zrecord.sh) that an editor with no file to write
# can still be measured by.  tools/whimdelta.sh hands a phase from ZERO_FROM on to
# tools/zerodelta.sh.
#
# THE PHASE LIST IS NOT WRITTEN HERE, and that is measured rather than tidy.  This
# file is hashed into every stage's key and every edit's (it is named by
# tools/phaserun.sh, which every split phase's key reads), so a byte changed here
# re-keys the whole pipeline.  A list written here would do that every time a
# phase is added.  It is the `phases` line of pipes/whim.stages instead, which no
# key reads.

case ${1:-whim} in
    whim) PIPE=whim; TAG=q; IMPL=whim;  DOC=WHIM-GOAL.md
          PWORK=whim;     PBUILD=.build-whim; PSOURCE=whim-vim.c; PDELTA=tools/whimdelta.sh
          ZERO_FROM=83
          PHASE_LIST=$(awk '$1 == "phases" { $1 = ""; sub(/^ +/, ""); print }' pipes/whim.stages) ;;
    *)    echo "pipeline: no such pipeline: $1" >&2; return 1 2>/dev/null || exit 1 ;;
esac
