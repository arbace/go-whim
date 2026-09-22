#!/bin/sh
# Run a unit of a pipeline's tier-2 program: a whole phase, or a STAGE.
#
# Usage: tools/phaserun.sh <pipeline> <unit> <work-dir>   (run from the root)
#        tools/phaserun.sh --parts <pipeline> <unit>
#
# A unit is a phase N or a stage A-B (tools/stages.sh).  The second form prints the
# program files of every phase in it, one per line, and nothing for a phase that
# has none -- which is how memo.sh, implhash.sh, specpass.sh and residue.sh ask
# "is this a program?" without each knowing the shapes.
#
# A phase program has one of two shapes:
#
#   phase/NNN/make.sh                          WHOLE: run with the work dir, as a
#                                              unit of its own (phase 0, 83, 84, 86,
#                                              116 and 123).
#   phase/NNN/edit.sh                          SPLIT: the cut, and
#   phase/NNN/check.sh                         the proof.  The sweep between is HERE.
#
# NNN is the phase number in three digits (phasedir, tools/pipeline.sh).
#
# A STAGE is a run of split phases: every edit in order, on text no sweep has
# touched since the stage began; ONE tools/sweep.sh; every check in order, on the
# one swept text and its one binary.  A single split phase is a stage of one.
#
# WHY.  70-90% of a whim phase was its final sweep, a fixed cost paid in full by a
# phase that deletes one line, measured when the stages were planned.  Eighty-two
# sweeps became twelve.  Which phases may share a sweep is phase/stages, which says
# what each edit needs of its input and which checks need a boundary before a later
# phase; tools/stages.sh --check refuses a schedule that breaks it, and runs here.
#
# THE CONTRACT, which is what makes a check runnable after other phases' edits and a
# shared sweep: the check part reads NOTHING from the edit part's shell.  No
# variable, no function, no trap, no background job.  What passes between them is
# files, in a state directory this driver makes fresh for each phase and hands to
# both parts as their second argument:
#
#   $state/input-lines   written HERE: the line count of the text THIS phase's edit
#                        is handed (unswept, inside a stage).  The check's "N -> M".
#   $state/symbols/      written HERE, once per stage, by tools/symbols.sh from the
#                        text the stage's FIRST edit is handed: the symbol snapshot
#                        tools/phasecheck.sh compares with (and removes).  Inside a
#                        stage every check compares with the stage's start, so "this
#                        phase must lower the count" means the stage must.
#   anything else        written by the EDIT part, named in it, and read by its own
#                        check: phase 80's `words` and `old`, 81's `old`, 82's
#                        `total`, `keep` and `old/whim-vim.c`.  An edit that starts
#                        a background job waits for it before it exits.
#
# THE DELTA IS THE STAGE'S, and no check part states one.  phase/NNN/delta declares
# what each phase changes; the declarations up to a phase are the whole difference
# from the pipeline's baselines at that phase, so the stage's last phase's contains
# every earlier one's, and this driver runs the pipeline's checker (PDELTA) with
# --phase <last> once, after the checks.
#
# The state directories are .cache/state/<tag><N>, and the stage's own
# .cache/state/<tag><unit>.stage, relative to where the unit runs -- never inside
# the work tree, whose every file is part of the boundary digest.  They are removed
# when every check passes and kept when anything fails.
set -eu

if [ "${1:-}" = "--parts" ]; then
    . tools/pipeline.sh "${2:?usage: phaserun.sh --parts <pipeline> <unit>}"
    u=${3:?usage: phaserun.sh --parts <pipeline> <unit>}
    for phase in $(seq "${u%-*}" "${u#*-}"); do
        if [ -f "$(phasedir "$phase")/edit.sh" ] && [ -f "$(phasedir "$phase")/check.sh" ]; then
            echo "$(phasedir "$phase")/edit.sh"
            echo "$(phasedir "$phase")/check.sh"
        elif [ -f "$(phasedir "$phase")/make.sh" ]; then
            echo "$(phasedir "$phase")/make.sh"
        fi
    done
    exit 0
fi

. tools/pipeline.sh "${1:?usage: phaserun.sh <pipeline> <unit> <work-dir>}"
unit=${2:?usage: phaserun.sh <pipeline> <unit> <work-dir>}
work=${3:?usage: phaserun.sh <pipeline> <unit> <work-dir>}
first=${unit%-*}
last=${unit#*-}

# A whole program is a unit of its own and runs as it always did.
if [ "$first" = "$last" ] && [ -f "$(phasedir "$first")/make.sh" ] \
        && ! { [ -f "$(phasedir "$first")/edit.sh" ] && [ -f "$(phasedir "$first")/check.sh" ]; }; then
    exec "$(phasedir "$first")/make.sh" "$work"
fi

phases=$(seq "$first" "$last")
for p in $phases; do
    if [ ! -f "$(phasedir "$p")/edit.sh" ] || [ ! -f "$(phasedir "$p")/check.sh" ]; then
        echo "  phaserun     $PIPE phase $p has no edit and check to run in stage $unit" >&2
        exit 2
    fi
done
if [ -z "$PSOURCE" ]; then
    echo "  phaserun     the $PIPE pipeline names no single source a sweep can run on" >&2
    exit 2
fi
tools/stages.sh "$PIPE" --check
f=$work/$PSOURCE

# ---- an EACH stage (tools/stages.sh): its own sweep per edit, the checks at once ----
#
# Every phase's edit runs on the swept tree the phase before it left, and is swept
# before the next edit sees it -- exactly the sequence a stage of one per phase
# runs, so every edit is handed the text it was written against and `need` cannot
# fail.  After its sweep each phase's TREE and STATE are copied into a root of its
# own, and once every edit has run, every check and every delta runs at once, each
# in its own root, handed exactly the arguments, the tree, the symbol snapshot and
# the input line count it would have had as a stage of one -- so no check can see
# another phase's work and `apart` cannot fail.  What the stage shares is the wall
# time of the checks, which are most of a phase from 83 on, and the boundary, which
# is its end.
#
# A ROOT OF ITS OWN, because checks are not independent in the working directory:
# tools/phasecheck.sh writes .cache/symbols/last and thirty checks read it back by
# that path, and the sweep's object is reused from .cache/compile.  A root is a
# directory with the tools, the phase programs, cmd/, internal/, go.mod and go.sum
# linked from this
# one, the baselines linked read-only, the Go build cache linked so whimtools is not
# rebuilt, and a .cache/ of its own -- tools/verifypass.sh's construct, one level
# down.  The phase's tree is its .tmp/whim-stage/ and its state is
# .cache/state/<tag><N>, the same relative paths the check is always given.
#
# THE REPORTS ARE PRINTED IN PHASE ORDER, each whole, after they have all finished:
# order is output, and a report interleaved with another is not one.  A phase that
# fails keeps its root; a stage that passes removes them.
#
# The edit and its sweep are CACHED together per phase, keyed on the phase, the
# digest of the tree the edit is handed and tools/implhash.sh --edit -- the edit
# cache below, with `each` in the key since what it holds differs.  The checks are
# not cached: they are the stage's evidence and always run.
if [ "$(tools/stages.sh "$PIPE" --mode "$unit")" = each ]; then
    here=$(pwd -P)
    jobs=${CHECK_JOBS:-8}
    each_digest() {
        ( cd "$work" && find . -type f -print0 | sort -z | xargs -0 sha256sum ) \
            | grep -Ev '/objects/|\.(o|d)$|/(whim-|zero-)?vim$' | sha256sum | cut -c1-32
    }
    roots=.cache/state/$TAG$unit.each
    rm -rf "$roots"
    mkdir -p "$roots"
    for p in $phases; do
        state=.cache/state/$TAG$p
        rm -rf "$state"
        mkdir -p "$state"
        grep -c '' "$f" > "$state/input-lines"
        tools/symbols.sh "$f" "$state/symbols"
        name=$(tools/phasename.sh "$p" "$PIPE" 2>/dev/null || true)
        ekey=$(printf '%s\neach\n%s\n%s\n' "$p" "$(each_digest)" "$(tools/implhash.sh --edit "$p" "$PIPE")" \
               | sha256sum | cut -c1-32)
        ecache=.cache/edit/$TAG$p/$ekey
        if [ -f "$ecache.tree.tar" ] && [ -f "$ecache.state.tar" ]; then
            tools/restore.sh "$ecache.tree.tar" "$work"
            tar --extract --file "$ecache.state.tar" -C "$state"
            printf '  %-12s %s  (edit and sweep cached for this input)\n' "edit $p" "$name"
        else
            printf '  %-12s %s\n' "edit $p" "$name"
            "$(phasedir "$p")/edit.sh" "$work" "$state"
            tools/sweep.sh "$f"
            mkdir -p ".cache/edit/$TAG$p"
            tar --create --file "$ecache.state.part" -C "$state" --exclude=./input-lines --exclude=./symbols .
            tar --create --file "$ecache.tree.part" -C "$work" .
            mv "$ecache.state.part" "$ecache.state.tar"
            mv "$ecache.tree.part" "$ecache.tree.tar"
        fi
        r=$roots/$p
        mkdir -p "$r/.cache/state" "$r/.reference" "$r/$PWORK"
        for x in tools phase cmd internal go.mod go.sum; do ln -s "$here/$x" "$r/$x"; done
        for x in .cache/gobin .cache/gofork; do [ -e "$here/$x" ] && ln -s "$here/$x" "$r/$x"; done
        for b in baselines zero-baselines; do
            [ -e "$here/.reference/$b" ] && ln -s "$here/.reference/$b" "$r/.reference/$b"
        done
        [ -e "$here/slim-vim.c" ] && ln -s "$here/slim-vim.c" "$r/slim-vim.c"
        tar --create --file - -C "$work" . | tar --extract --file - -C "$r/$PWORK"
        mv "$state" "$r/.cache/state/$TAG$p"
    done

    # Every check, then its delta, in its own root, $jobs at a time -- the delta
    # on the phase's own binary (delta_binary below).
    printf '%s\n' $phases | xargs -P "$jobs" -I{} sh -c '
        r=$1/$2
        rm -f "$r/$4/$7"
        ( cd "$r" && "phase/$(printf %03d "$2")/check.sh" "$4" ".cache/state/$5$2" \
          && { [ -f "$4/$7" ] || make -C "$4" >/dev/null 2>&1 \
               || { echo "  build        FAILED -- the binary the delta measures: make -C $4"; exit 1; }; } \
          && "$6" "$4/$7" "$4/$8" --phase "$2" ) > "$r.log" 2>&1
        echo $? > "$r.rc"' sh "$roots" {} "$IMPL" "$PWORK" "$TAG" "$PDELTA" "${PSOURCE%.c}" "$PSOURCE"

    fail=
    for p in $phases; do
        printf '  %-12s %s\n' "check $p" "$(tools/phasename.sh "$p" "$PIPE" 2>/dev/null || true)"
        cat "$roots/$p.log"
        [ "$(cat "$roots/$p.rc" 2>/dev/null)" = 0 ] || fail="$fail $p"
    done
    if [ -n "$fail" ]; then
        echo "  phaserun     stage $unit: the check or delta of phase(s)$fail FAILED; each root is kept in $roots"
        exit 1
    fi
    # The boundary is the last phase's tree as its check left it, which is what a
    # stage of one snapshots.
    rm -rf "$work"
    mv "$roots/$last/$PWORK" "$work"
    rm -rf "$roots"
    exit 0
fi

# The stage's start: the symbol snapshot, once, of the text the first edit is
# handed -- the one text in a stage that is certain to compile.
stage_state=.cache/state/$TAG$unit.stage
rm -rf "$stage_state"
mkdir -p "$stage_state"
tools/symbols.sh "$f" "$stage_state/symbols"

# The edits, in order, on text no sweep has touched since the stage began.  Each
# phase's state directory is made fresh, gets the line count of the text ITS edit
# is handed, and keeps whatever that edit leaves for its check.
#
# EACH EDIT'S RESULT IS CACHED, keyed like any boundary: the phase, the digest of
# the tree its edit is handed, and the implementation digest of the edit part alone
# (tools/implhash.sh --edit).  Its value is the tree the edit leaves and its state
# directory.  So editing phase K's program re-runs K's edit, and after it only the
# edits whose input really moved -- then the one sweep and the checks, which always
# run -- instead of every edit in the stage.  It is the same construct as tier 3,
# one level down, and it is as safe: a cached edit answers exactly one input and
# one implementation.  A scratch root with a .cache of its own (verifypass.sh)
# recomputes every edit.
tree_digest() {
    ( cd "$work" && find . -type f -print0 | sort -z | xargs -0 sha256sum ) \
        | grep -Ev '/objects/|\.(o|d)$|/(whim-|zero-)?vim$' | sha256sum | cut -c1-32
}
for p in $phases; do
    state=.cache/state/$TAG$p
    rm -rf "$state"
    mkdir -p "$state"
    grep -c '' "$f" > "$state/input-lines"
    name=$(tools/phasename.sh "$p" "$PIPE" 2>/dev/null || true)
    ekey=$(printf '%s\n%s\n%s\n' "$p" "$(tree_digest)" "$(tools/implhash.sh --edit "$p" "$PIPE")" \
           | sha256sum | cut -c1-32)
    ecache=.cache/edit/$TAG$p/$ekey
    if [ -f "$ecache.tree.tar" ] && [ -f "$ecache.state.tar" ]; then
        tools/restore.sh "$ecache.tree.tar" "$work"
        tar --extract --file "$ecache.state.tar" -C "$state"
        printf '  %-12s %s  (edit cached for this input)\n' "edit $p" "$name"
        continue
    fi
    [ "$first" = "$last" ] || printf '  %-12s %s\n' "edit $p" "$name"
    "$(phasedir "$p")/edit.sh" "$work" "$state"
    mkdir -p ".cache/edit/$TAG$p"
    tar --create --file "$ecache.state.part" -C "$state" --exclude=./input-lines .
    tar --create --file "$ecache.tree.part" -C "$work" .
    mv "$ecache.state.part" "$ecache.state.tar"
    mv "$ecache.tree.part" "$ecache.tree.tar"
done

tools/sweep.sh "$f"

# The checks, in order, all on the stage's one swept text and one binary.  Every
# check compares symbols with the stage's start.
# THE DELTA MEASURES THE STAGE'S OWN BINARY, and nothing else guarantees it: a
# boundary's tar carries the binary the last check to build one left, and five
# checks (109 110 111 113 115) build theirs elsewhere and never rebuild the tree's --
# so their delta measured the PREVIOUS boundary's binary, and passed.  Found when an
# each stage handed them one from further back and declared tokens stopped moving.
# So the binary the stage was handed goes before the checks, and one no check
# rebuilt is built from the tree's own makefile before the delta runs.
rm -f "$work/${PSOURCE%.c}"
for p in $phases; do
    state=.cache/state/$TAG$p
    rm -rf "$state/symbols"
    cp -r "$stage_state/symbols" "$state/symbols"
    [ "$first" = "$last" ] || printf '  %-12s %s\n' "check $p" "$(tools/phasename.sh "$p" "$PIPE" 2>/dev/null || true)"
    "$(phasedir "$p")/check.sh" "$work" "$state"
done

# The declared delta, once: every phase's declaration up to the stage's last phase
# is the whole difference from the pipeline's baselines there, and so holds every
# earlier phase's.  The checker is the pipeline's (PDELTA in tools/pipeline.sh),
# which reads the declarations and hands a phase from ZERO_FROM on to the checker of
# the second baselines.  Never name that checker's path in this file: every edit's
# key reads what this file names, so the name would put that tool in all of them.
if [ -n "$PDELTA" ]; then
    if [ ! -f "$work/${PSOURCE%.c}" ] && ! make -C "$work" >/dev/null 2>&1; then
        echo "  build        FAILED -- the binary the delta measures: make -C $work"
        exit 1
    fi
    "$PDELTA" "$work/${PSOURCE%.c}" "$f" --phase "$last"
fi

for p in $phases; do rm -rf ".cache/state/$TAG$p"; done
rm -rf "$stage_state"
