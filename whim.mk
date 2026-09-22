# The pipeline: whim-vim.c = G(slim-vim.c).
#
# Included by the root Makefile, and the same construct as arbace/slim-vim's slim.mk -- phases as
# targets, boundaries as content digests, results memoized in three tiers.  The
# driver, the oracle and the synthesiser take the pipeline as an argument;
# tools/pipeline.sh is where its parameters are stated.
#
# What differs from slim is what the phases DO.  arbace/slim-vim's SLIM-GOAL.md
# removes files and preprocessor and changes nothing about the editor; this
# pipeline removes capability on purpose, so every phase states its delta in
# advance and the harness shows exactly that set and no more.  Phases 0-82
# (GOALS.md Part I) leave an editor with no runtime to install; phases 83 on
# (GOALS.md Part II) turn it into an embeddable core -- no filesystem, the host
# behind a line in the file, no libc the core names, the text a tree -- and then
# remove from it what translating it to Go had to work around.
#
# The input is slim-vim.c, fetched by the root Makefile from arbace/slim-vim at
# the commit in upstream.sha -- one file, not that repository's pipeline.  The
# memoize key is slim-vim.c's DIGEST and the implementation's, so a slim-vim
# commit that leaves slim-vim.c alone produces nothing here.
#
# THE COMPILE LINE IS THE BOUNDARY'S.  Phase 0 starts from tools/templates/whim.mk,
# gcc -O0 -static -s (a static-PIE); phase 83 replaces it with
# tools/templates/zero.mk, -no-pie; phase 84 adds -fno-stack-protector.  A phase
# changes the flags by editing the work tree's Makefile (.tmp/whim-stage/Makefile),
# never a template, which is the pipeline's input.  The product rule below cannot
# read the work tree, which does not exist
# in a checkout that only builds the committed whim-vim.c, so it states them once,
# as WHIMCFLAGS and WHIMLDFLAGS, and whim-pass refuses to copy whim-vim.c out when
# they differ from the last boundary's makefile.

WHIMWORK   = .tmp/whim-stage
WHIMBUILD  = .build
WHIMORACLE = .reference/whim-phases
WHIMCFLAGS  = -O0 -fno-stack-protector
WHIMLDFLAGS = -static -no-pie -s

# The phase list is the `phases` line of phase/stages, read through
# tools/pipeline.sh -- not written out here and not in tools/pipeline.sh, whose every
# byte is in every stage's key.  One list, so it cannot disagree with itself.
WHIMPHASES := $(shell . tools/pipeline.sh whim && echo $$PHASE_LIST)

# --- the chain ------------------------------------------------------------
# The chain is of STAGES, read from phase/stages by tools/stages.sh: a stage
# is a run of phases whose edits share one sweep, and only a stage's end is a
# boundary -- q41 is the boundary of stage 13-41, and nothing between q12 and q41
# exists.  Each boundary depends on the one before it, as each phase's did.
WHIMSTAGES := $(shell tools/stages.sh whim)
ifeq ($(WHIMSTAGES),)
$(error phase/stages does not hold -- tools/stages.sh whim --check says why)
endif
WHIMENDS   := $(foreach u,$(WHIMSTAGES),$(lastword $(subst -, ,$(u))))
WHIMSTARTS := input.sha256 $(patsubst %,q%.sha256,$(filter-out $(lastword $(WHIMENDS)),$(WHIMENDS)))
WHIMLAST   := $(lastword $(WHIMENDS))
$(foreach i,$(shell seq 1 $(words $(WHIMENDS))),$(eval \
    $(WHIMBUILD)/q$(word $(i),$(WHIMENDS)).sha256: $(WHIMBUILD)/$(word $(i),$(WHIMSTARTS))))

$(WHIMBUILD)/q%.sha256:
	@tools/restore.sh $(patsubst %.sha256,%.tar,$<) $(WHIMWORK)
	@tools/memo.sh $$(tools/stages.sh whim --of $*) $(WHIMWORK) $(WHIMBUILD) whim
	@tools/oracle.sh $* $(WHIMBUILD) $(WHIMORACLE) whim | sed 's/^  /      /'

# --- the input ------------------------------------------------------------
# A directory holding one file and a makefile, so the boundary machinery -- a
# tar and a digest over a tree -- applies unchanged.
$(WHIMBUILD)/input.sha256: slim-vim.c tools/templates/whim.mk
	@rm -rf $(WHIMWORK)
	@mkdir -p $(WHIMWORK) $(WHIMBUILD)
	@cp slim-vim.c $(WHIMWORK)/whim-vim.c
	@cp tools/templates/whim.mk $(WHIMWORK)/Makefile
	@date +%s > $(WHIMBUILD)/pass-start
	@printf '\n\033[1m  whim-vim\033[0m  from slim-vim.c: an editor with no runtime\n'
	@tools/snapshot.sh $(WHIMWORK) $(WHIMBUILD)/input.tar $(WHIMBUILD)/input.sha256

# --- the product ----------------------------------------------------------
# The same content-keyed dependency the other pipeline uses, for the same
# reason: whim-vim.c and slim-vim.c are both tracked, and a fresh clone writes
# them at checkout time in arbitrary order, so an mtime dependency would run a
# pass on a tree that is exactly right.  slim.sha records the slim-vim.c this
# whim-vim.c was produced from.
whim-vim: whim-vim.c
	@printf '  %-12s %s\n' "compiling" "$(CC) $(WHIMCFLAGS) $(WHIMLDFLAGS) -o $@ $<"
	@t0=`date +%s`; $(CC) $(WHIMCFLAGS) $(WHIMLDFLAGS) -o $@ $<; \
	 printf '  %-12s %s bytes, static, not PIE, %ss\n' "$@" \
	     "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	     "$$((`date +%s` - t0))"

whim-vim.c: force
	@set -e; \
	live=`sha256sum slim-vim.c | cut -c1-64`; \
	if [ -f $@ ] && [ "$$live" = "`cat slim.sha 2>/dev/null`" ]; then \
	    printf '  %-12s %s unchanged -- whim-vim.c is current\n' "slim-vim.c" "`echo $$live | cut -c1-12`"; \
	    exit 0; \
	fi; \
	printf '  %-12s %s -- whim-vim.c must be produced\n' "slim-vim.c" "`echo $$live | cut -c1-12`"; \
	$(MAKE) --no-print-directory whim-pass; \
	echo "$$live" > slim.sha

# --- what a whim pass is --------------------------------------------------
.PHONY: whim-pass
whim-pass: $(WHIMBUILD)/q$(WHIMLAST).sha256
	@$(MAKE) --no-print-directory whim-baselines-check
	@m=$$(tar -xOf $(WHIMBUILD)/q$(WHIMLAST).tar ./Makefile 2>/dev/null \
	      || tar -xOf $(WHIMBUILD)/q$(WHIMLAST).tar Makefile); \
	 c=$$(printf '%s\n' "$$m" | sed -n 's/^CFLAGS  *= *//p'); \
	 l=$$(printf '%s\n' "$$m" | sed -n 's/^LDFLAGS  *= *//p'); \
	 if [ "$$c" != "$(WHIMCFLAGS)" ] || [ "$$l" != "$(WHIMLDFLAGS)" ]; then \
	     echo "  flags        whim.mk builds whim-vim with '$(WHIMCFLAGS)' '$(WHIMLDFLAGS)',"; \
	     echo "               but the last boundary's makefile says '$$c' '$$l'."; \
	     echo "               WHIMCFLAGS and WHIMLDFLAGS must state the boundary's flags."; \
	     exit 1; \
	 fi
	@tar -xOf $(WHIMBUILD)/q$(WHIMLAST).tar ./whim-vim.c > whim-vim.c 2>/dev/null \
	 || tar -xOf $(WHIMBUILD)/q$(WHIMLAST).tar whim-vim.c > whim-vim.c
	@echo
	@printf '  %-12s %s lines, from slim-vim.c\n' "whim-vim.c" \
	    "`grep -c '' whim-vim.c | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"
	@$(MAKE) --no-print-directory whim-editor

# The same per-phase handles arbace/slim-vim's slim.mk has, and one semantic change: a phase is run
# by running THE STAGE THAT CONTAINS IT, because only a stage's end is a boundary
# and nothing else has an input to start from.  `make whim-phase-50` re-runs stage
# 42-63; within it, the edits before 50 come from the edit cache when nothing they
# read has changed (tools/phaserun.sh).
.PHONY: $(WHIMPHASES:%=whim-phase-%)
$(WHIMPHASES:%=whim-phase-%): whim-phase-%:
	@u=$$(tools/stages.sh whim --of $*) && last=$${u#*-} && \
	 rm -f $(WHIMBUILD)/q$$last.sha256 && \
	 $(MAKE) --no-print-directory $(WHIMBUILD)/q$$last.sha256 && \
	 $(MAKE) --no-print-directory whim-baselines-check

# Only a stage's end can be replayed: nothing else was ever a tree on disk.
.PHONY: $(WHIMPHASES:%=whim-replay-%)
$(WHIMPHASES:%=whim-replay-%): whim-replay-%:
	@if [ ! -f $(WHIMBUILD)/q$*.tar ]; then \
	     echo "  replay       q$* is not a boundary: phase $* is inside stage $$(tools/stages.sh whim --of $*)," \
	          "and only a stage's end is kept"; exit 1; fi
	@tools/restore.sh $(WHIMBUILD)/q$*.tar $(WHIMWORK)
	@echo "  replay       $(WHIMWORK)/ is the tree after whim phase $*"

.PHONY: whim-times
whim-times:
	@total=0; for u in $(WHIMSTAGES); do q=$${u#*-}; \
	    [ -f $(WHIMBUILD)/q$$q.seconds ] || continue; \
	    s=$$(cat $(WHIMBUILD)/q$$q.seconds); total=$$((total + s)); \
	    printf '  whim %-6s %4s s  by %s\n' "$$u" "$$s" "$$(cat $(WHIMBUILD)/q$$q.kind)"; \
	done; \
	printf '  %-12s %4s s\n' "total" "$$total"

# Force one when the recorded input digest already matches -- slim-repass's twin.
.PHONY: whim-repass
whim-repass:
	@rm -rf $(WHIMBUILD)
	@$(MAKE) --no-print-directory whim-pass

# Every phase is a program and this pipeline has no agent in it, so a boundary
# it produced is reproducible by construction -- there is no advisory stage to
# pass through.  Recorded only after a run that built, swept to silence and
# showed exactly the declared delta.
# ADDING A PHASE DOES NOT RE-RUN THE STAGES BEFORE IT: their boundary files are
# older than nothing that changed, so make skips them, and this target runs only
# the last stage.  Their tier 3 entries are a different matter.  tools/implhash.sh
# reads a phase's own program, the tools it names and the tools those name -- not
# whim.mk, but tools/pipeline.sh, named by tools/phaserun.sh.  Measured, when the
# phase list was written there: one byte moved all 12 split stage keys and all 82
# edit keys, so the list is the `phases` line of phase/stages now, which no
# key reads.
#
# So the loop while you are trying ideas out is: make phase/NNN with edit.sh,
# check.sh, GOAL.md and its declared delta, add N to the `phases` line of
# phase/stages and put it in the last stage or a new one and in a package there
# (GOALS.md, "Adding a phase"), and `make whim-tip`.  Only the last stage runs,
# and its earlier edits come from the edit cache.
#
# WHAT THIS DOES NOT DO, and must not be mistaken for: falsify the boundaries
# before it.  A tier 3 replay COPIES the recorded digest rather than recomputing
# it, so a warm pass agrees with the oracle whatever the oracle says -- which is
# how a wrong boundary went unnoticed for eleven phases once.  Only a run that
# recomputes every digest can do that: `make whim-verify`, every phase at once on
# the recorded boundary before it, or `make whim-repass` after `make clean-cache`,
# which is sequential and is the one that records.  Do it before a push, and
# whenever a SHARED tool changes (sweep.sh, canon.sh, deadsweep.py, typereach.py,
# funcreach.py, deadfields.py, deadenums.py, phasecheck.sh, whimdelta.sh,
# cutil.py) -- those are in every phase's implhash, so everything re-runs then
# anyway.
#
# Both whim-tip and whim-verify first run tools/packages.sh whim --check: a new
# phase must be placed in a package, and a `uses` line must still point backwards.
# It is here and nowhere a phase runs -- whim.mk is in no implementation digest,
# while phaserun.sh, memo.sh and stages.sh are -- and it reads only the manifest,
# so it costs nothing and moves no key.  whim-pass and whim-repass do not run it:
# a package mistake is a documentation mistake and must not stop a build.
.PHONY: whim-tip
whim-tip:
	@tools/packages.sh whim --check && \
	 $(MAKE) --no-print-directory whim-phase-$(WHIMLAST) && \
	 $(MAKE) --no-print-directory whim-record | tail -1
	@$(MAKE) --no-print-directory whim-product-check

# Is the TRACKED product the boundary the pipeline just recorded?
#
# `whim-tip` records a boundary; `whim-pass` copies the product out of the last
# boundary's tar into the repository root.  They are different targets and
# running only the first leaves whim-vim.c behind -- measured, the product of the
# old zero pipeline went TWO phases stale that way, and it was PUSHED.
#
# Nothing else can catch it.  A boundary is a tar and a digest under .build,
# and tools/verifypass.sh reproduces each one from the boundary before it in a
# scratch root -- none of that reads the tracked product.  It is an OUTPUT of the
# memoize and an input to nothing, so it can be arbitrarily wrong while `make
# whim-verify` reports every boundary reproducing.
#
# So this says so, and says the fix, and is deliberately a WARNING rather than a
# failure: mid-arc the product is expected to lag its boundary between a phase
# landing and `whim-pass` running, and a target that refused there would be
# refusing the normal case.  whim.mk is in no implementation digest, so none of
# this is in any key.
.PHONY: whim-product-check
whim-product-check:
	@t=$(WHIMBUILD)/q$(WHIMLAST).tar; \
	 if [ -f "$$t" ] && [ -f whim-vim.c ]; then \
	     if ! (tar -xOf "$$t" ./whim-vim.c 2>/dev/null || tar -xOf "$$t" whim-vim.c) \
	          | cmp -s - whim-vim.c; then \
	         echo "  product      the tracked whim-vim.c is NOT q$(WHIMLAST) -- `grep -c '' whim-vim.c` lines here"; \
	         echo "               against the boundary just recorded.  Run: make whim-pass"; \
	     fi; \
	 fi

# Every recorded boundary, checked at once.  Each stage is run on the recorded
# boundary before it, in a scratch root of its own, and must reproduce the one it
# recorded -- by induction the same proof as a repass from an empty cache, in the
# wall time of the slowest stage instead of the sum of them all.
.PHONY: whim-verify
whim-verify:
	@tools/packages.sh whim --check && \
	 tools/verifypass.sh whim
	@$(MAKE) --no-print-directory whim-editor-check

# A repass that waits only where it has to.  Every stage first runs at once on
# the previous pass's boundary before it, and its result goes into the tier 3
# cache under the key memo.sh will look up; then the ordinary sequential pass
# runs, and is a cache hit wherever that guess about its input was right.  A
# change to a tool rather than to what a phase produces costs the wall time of
# the slowest phase; a change to phase K's output still runs K onwards in
# sequence.  See tools/specpass.sh.  The previous .build is its input, so
# this target reads it before whim-repass removes it.
.PHONY: whim-specpass
whim-specpass:
	@tools/specpass.sh whim
	@$(MAKE) --no-print-directory whim-repass

.PHONY: whim-record
whim-record:
	@mkdir -p $(WHIMORACLE)
	@for q in $(WHIMENDS); do \
	    [ -f $(WHIMBUILD)/q$$q.sha256 ] || continue; \
	    cp $(WHIMBUILD)/q$$q.sha256 $(WHIMORACLE)/q$$q.sha256; \
	    cp $(WHIMBUILD)/q$$q.sha256.files $(WHIMORACLE)/q$$q.sha256.files; \
	    echo "  record       q$$q: $$(cut -c1-12 $(WHIMORACLE)/q$$q.sha256)"; \
	done

.PHONY: whim-clean
whim-clean:
	rm -rf $(WHIMBUILD) $(WHIMWORK) whim-vim editor.c

# How much of each phase is still a recorded diff rather than a rule.  The twin
# of slim-residue; the scoreboard is the same question either side.
.PHONY: whim-residue
whim-residue:
	@tools/residue.sh whim

# --- editor.c, the upper part on its own ----------------------------------
# What phases 83 onwards are for.  whim-vim.c is one translation unit with two parts:
# above, the core editor, with no preprocessor syntax at all; below, the host,
# beginning with the #includes -- and that first directive IS the boundary, marked
# by nothing else (GOALS.md II.4c).  The product of the whole project is the upper
# part, and this is the rule that takes it.
#
# The cut is `stop at the first #include`, which is one awk clause and no judgement.
# The rest of the awk drops trailing blank lines, so the file ends on its last line
# of code rather than on whatever blank separated the core from the host.
#
# THE PATTERN IS `^ *# *include `, NEVER `^#include`, and tools/macros.py already
# carries the reason: whitespace between `#` and the keyword is insignificant to C,
# and this tree has had plenty of `# define` -- what the conditional-resolution pass
# left when it dedented `#  define` by one level and stopped.  A cut that missed
# ` # include` would not fail, it would run PAST the boundary and take the host with
# it.  So the rule also refuses if the file it wrote holds a DIRECTIVE -- a line whose
# first non-blank character is `#`: the defining property of the upper part is that it
# has no directive, and a cut that produced one has found the wrong line.
#
# THE GUARD SAID `grep -q '#'` UNTIL PHASE 110 (Phase 110), AND IT COULD ONLY EVER HAVE BEEN
# WRITTEN AGAINST AN EMPTY FILE.  A `#` is also an ordinary character, and the editor is
# full of them: measured on the first cut this rule ever produced, 63 lines hold one --
# `enum { CPO_HASH = '#' };`, `if (ptr[0] == '#')`, the two latin1 case tables, the
# `"E1281: Atom '\%%#=%c'"` message.  None is a directive and the rule refused all the
# same.  `^ *#` is what the paragraph above already says it means, and it is measured
# at 0 on the cut and at 11 on the whole file, which is the eleven `#include`s.
#
# IT DOES NOT COMPILE ON ITS OWN, AND THAT IS CORRECT, not a defect to fix: the core
# calls the musl_ functions the host defines below, so the upper part declares them
# and defines none.  What it must do is PARSE -- `gcc -fsyntax-only` with no errors
# and a warning set equal to the declared boundary -- and that is the reorganisation
# phase's check, not this rule's.
#
# THIS RULE WROTE AN EMPTY FILE UNTIL PHASE 110 (Phase 110), and that was the honest answer
# rather than a defect: the includes were the first eleven lines, so there was nothing
# above the first one.  It was written before the phase that fills it precisely so that
# the phase would change the SOURCE and not the makefile -- and it did.  Phase 110 moved
# the includes to 78,360 and the rule now writes the 78,358 lines above them, which is
# the product of this whole project.
# The cut, as a canned recipe: editor.c's rule runs it after whim-vim.c is
# current, and whim-pass and whim-editor-check run it on the tracked whim-vim.c
# as it stands -- through whim-vim.c's own rule they would start a pass.
define cut-editor
	@awk '/^ *# *include / { exit } { a[NR] = $$0; if (NF) last = NR } \
	      END { for (i = 1; i <= last; i++) print a[i] }' whim-vim.c > editor.c
	@if grep -q '^ *#' editor.c; then \
	    echo "  editor.c     REFUSED -- the cut holds a directive, so it found the wrong line:"; \
	    grep -n '^ *#' editor.c | head -3 | sed 's/^/               /'; \
	    rm -f editor.c; exit 1; \
	 fi
	@printf '  %-12s %s lines, cut at the first #include of %s\n' editor.c \
	    "`grep -c '' editor.c | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	    "`grep -c '' whim-vim.c | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"
endef

.PHONY: editor.c
editor.c: whim-vim.c
	$(cut-editor)

# --- editor/editor.go, the core in Go -----------------------------------------
# GENERATED: tx/skel writes it whole from editor.c (tx/gen.sh), and it is
# tracked, so it must be what the program writes from the tracked whim-vim.c.
# tx/gen.sh writes it only when that differs, so a current file keeps its mtime.
#
#   make editor/editor.go    write it again from whim-vim.c
#   make whim-editor-check   refuse if the tracked file is not what it would be
#
# whim-pass writes it after copying whim-vim.c out, since a phase that changes
# the C changes it; whim-verify, run before a push, refuses a stale one.
.PHONY: editor/editor.go
editor/editor.go: editor.c
	@sh tx/gen.sh

.PHONY: whim-editor
whim-editor:
	$(cut-editor)
	@sh tx/gen.sh

.PHONY: whim-editor-check
whim-editor-check:
	$(cut-editor)
	@sh tx/gen.sh --check

# There are two sets of baselines (tools/pipeline.sh, ZERO_FROM).  Phase 0 records
# .reference/baselines from slim-vim.c, and every delta before phase 83 is measured
# against them; phase 83 records .reference/zero-baselines from the tree it is
# handed, and every delta from it on is measured against those.  Each is a side
# effect of running, so a tier 3 hit on either phase records nothing -- and a fresh
# .reference/ beside a warm .cache/ would end a pass looking fine, with every later
# delta check running its baseline-free half.  So every target that can end a pass
# on a cached phase 0 or 83 asks afterwards, and refuses with the fix.  It is here,
# in a makefile no implementation digest reads, so it moves no key.
#
# BOTH PATHS in the second fix, and the first is not redundant.  This target only
# sees the MISSING case, but the other way to get here is a CHANGED recording --
# a harness that asks something new, or a phase before 83 that changed what it
# produces -- and phase/083/make.sh REFUSES a set that differs rather than
# overwriting it, naming the file that moved and exiting 1.  Which one it was must
# be NAMED before the recording is thrown away.
.PHONY: whim-baselines-check
whim-baselines-check:
	@for x in behaviour ref-term.txt ref-exsweep.txt; do \
	    [ -e .reference/baselines/$$x ] && continue; \
	    echo "  baselines    .reference/baselines/$$x is missing, so no delta before phase 83 was checked in full."; \
	    echo "               Phase 0 records them from slim-vim.c; a cached phase 0 records nothing:"; \
	    echo "                 rm -rf .reference/baselines .cache/q0 && make whim-phase-0"; \
	    exit 1; \
	done
	@b=.reference/zero-baselines; \
	 if [ -d $$b/screen ] && [ -n "$$(ls -A $$b/screen 2>/dev/null)" ] \
	    && [ -d $$b/memline ] && [ -n "$$(ls -A $$b/memline 2>/dev/null)" ] \
	    && [ -s $$b/ref-excmds.txt ] && [ -s $$b/ref-argv.txt ] \
	    && [ -s $$b/ref-pty.txt ] && [ -s $$b/ref-term.txt ]; then exit 0; fi; \
	 echo "  baselines    $$b is missing or of the old shape: it must hold screen/, memline/,"; \
	 echo "               ref-excmds.txt, ref-argv.txt, ref-pty.txt and ref-term.txt"; \
	 echo "               (tools/zrecord.sh), so no delta from phase 83 on was checked in full."; \
	 echo "               Phase 83 records them; a cached phase 83 records nothing:"; \
	 echo "                 rm -rf .reference/zero-baselines .cache/q83 && make whim-phase-83"; \
	 exit 1
