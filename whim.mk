# The pipeline: whim-vim.c = G(slim-vim.c), and editor/editor.go from it.
#
# Included by the root Makefile.  163 phases remove capability on purpose, and
# every phase declares its delta in advance: phases 0-82 (GOALS.md Part I) leave
# an editor with no runtime to install; phases 83 on (Part II) turn it into an
# embeddable core -- no filesystem, the host behind a line in the file, no libc
# the core names, the text a tree -- and then remove from it what translating it
# to Go had to work around.
#
# TWO PATHS, AND THAT IS THE WHOLE STRUCTURE.
#
#   whim-build     the phases, in one process, in memory: the product, twenty
#                  minutes, nothing verified (internal/build)
#   whim-verify    every phase's check on the tree and the state it was written
#                  against, and every stage's declared delta (internal/verify)
#
# There is no memoize between them any more.  Its key was the input boundary's
# digest and the implementation's together, so a moved slim-vim.c missed every
# entry by construction -- it paid only while the phases themselves were being
# written, and that is over.  What answers for the product now is that it is
# TRACKED: `whim-build-check` requires the committed whim-vim.c back, byte for
# byte, from the committed slim-vim.c.
#
# THE COMPILE LINE IS THE BOUNDARY'S.  Phase 0 starts from tools/templates/whim.mk,
# gcc -O0 -static -s (a static-PIE); phase 83 replaces it with
# tools/templates/core.mk, -no-pie; phase 84 adds -fno-stack-protector.  A phase
# changes the flags by writing the work tree's Makefile, never a template
# (internal/build's Makefile field).  The product rule below cannot read a work
# tree -- there is none in a checkout that only builds the committed whim-vim.c --
# so it states them once, as WHIMCFLAGS and WHIMLDFLAGS.

WHIMWORK    = .tmp/whim-stage
WHIMCFLAGS  = -O0 -fno-stack-protector
WHIMLDFLAGS = -static -no-pie -s

# ==== the product
# The same content-keyed dependency the other pipeline uses, for the same
# reason: whim-vim.c and slim-vim.c are both tracked, and a fresh clone writes
# them at checkout time in arbitrary order, so an mtime dependency would run a
# pass on a tree that is exactly right.  slim.sha records the slim-vim.c this
# whim-vim.c was produced from.
whim-vim: whim-vim.c  ## the C product, compiled with the boundary's own flags
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
	$(MAKE) --no-print-directory whim-build; \
	echo "$$live" > slim.sha

# --- the build: the pipeline in one process -------------------------------
# 163 phases, in order, in memory -- internal/build's plan and internal/steps'
# transformations, which are the phase programs' own (tools/st.sh, cmd/, internal/).
# It produces whim-vim.c and nothing else: no boundaries, no digests, no cache.
#
# WHAT HOLDS IT TO THE PHASES IS THE PRODUCT.  `build --check` requires the
# committed whim-vim.c back, byte for byte, from the committed slim-vim.c, which
# a step in the wrong order or a missing sweep cannot survive.  Measured: 163
# phases, 1,220 s, 77,306 lines.
#
# IT VERIFIES NOTHING ABOUT THE EDITOR, on purpose.  The checks, the recordings
# and the declared deltas are `make whim-verify`, and they are separate because
# they cost hours and this costs twenty minutes.
# ==== the pipeline
.PHONY: whim-build whim-build-check
whim-build:  ## the 163 phases in one process: slim-vim.c -> whim-vim.c
	@printf '\n\033[1m  whim-vim\033[0m  from slim-vim.c: an editor with no runtime\n'
	@tools/st.sh build --out whim-vim.c
	@$(MAKE) --no-print-directory whim-editor

whim-build-check:  ## the same build, required to give the committed bytes back
	@tools/st.sh build --check

# --- what a whim pass is --------------------------------------------------

# ==== verification
# --- whim-verify: the evidence ---------------------------------------------
# Every phase's check, on exactly the tree and the state directory it was
# written against, and every stage's declared delta against the baselines.  It
# builds the pipeline as it goes (the same plan whim-build runs) and keeps the
# stage arrangement, because that arrangement is what a check was written for: a
# shared stage runs every edit, ONE sweep, then every check on that one swept
# text; an each stage sweeps after every edit and gives every check its own tree.
#
#   make whim-verify             the whole pipeline, hours
#   tools/st.sh verify --to N    stop after the stage holding phase N
#   tools/st.sh verify --from N --src BOUNDARY   start from a boundary you have
.PHONY: whim-verify
whim-verify:  ## every phase's check and every declared delta, from slim-vim.c
	@$(MAKE) --no-print-directory whim-baselines-check
	@tools/st.sh verify
	@$(MAKE) --no-print-directory whim-editor-check

# --- the baselines ---------------------------------------------------------
# What phase 0's program and phase 83's recorded while a pass still ran shell:
# .reference/baselines from slim-vim.c, and .reference/core-baselines from q82,
# which this builds in order to record from.  Each set is recorded three times
# and required identical, and an existing set is COMPARED, never overwritten.
.PHONY: whim-baselines
whim-baselines:  ## record .reference/: from slim-vim.c, and from q82
	@tools/st.sh record

# ==== the editor
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
# current, and whim-editor and whim-editor-check run it on the tracked
# whim-vim.c as it stands -- through whim-vim.c's own rule they would start a
# build.
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
editor.c: whim-vim.c  ## the core, cut from whim-vim.c at its first #include
	$(cut-editor)

# --- editor/editor.go, the core in Go -----------------------------------------
# GENERATED: tx/skel writes it whole from editor.c (tx/gen.sh), and it is
# tracked, so it must be what the program writes from the tracked whim-vim.c.
# tx/gen.sh writes it only when that differs, so a current file keeps its mtime.
#
#   make editor/editor.go    write it again from whim-vim.c
#   make whim-editor-check   refuse if the tracked file is not what it would be
#
# whim-build writes it after producing whim-vim.c, since a phase that changes
# the C changes it; whim-verify, run before a push, refuses a stale one.
.PHONY: editor/editor.go
editor/editor.go: editor.c  ## the core in Go, generated whole from editor.c
	@sh tx/gen.sh

.PHONY: whim-editor
whim-editor:
	$(cut-editor)
	@sh tx/gen.sh

.PHONY: whim-editor-check
whim-editor-check:  ## refuse if the tracked editor.go is not what tx/skel writes
	$(cut-editor)
	@sh tx/gen.sh --check

# There are two sets of baselines.  Phase 0 records
# .reference/baselines from slim-vim.c, and every delta before phase 83 is measured
# against them; phase 83 records .reference/core-baselines from the tree it is
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
# produces -- and whimtools record REFUSES a set that differs rather than
# overwriting it, naming the file that moved and exiting 1.  Which one it was must

# There are two sets of baselines, and the line between them is phase 83
# (internal/build.CoreFrom).  Phase 0 records .reference/baselines from
# slim-vim.c, and every delta before 83 is measured against them; phase 83
# records .reference/core-baselines from the tree it is handed, and every delta
# from it on against those.  A verification with neither would run its
# baseline-free half and look fine, so it asks first and refuses with the fix.
#
# THE FIX IS `make whim-baselines`, and a set that DIFFERS is not the same case
# as one that is missing: whimtools record refuses a differing set rather than
# overwriting it, naming the file that moved.  Which one it was must be NAMED
# before the recording is thrown away.
.PHONY: whim-baselines-check
whim-baselines-check:
	@for x in behaviour ref-term.txt ref-exsweep.txt; do \
	    [ -e .reference/baselines/$$x ] && continue; \
	    echo "  baselines    .reference/baselines/$$x is missing, so no delta before phase 83 can be checked in full."; \
	    echo "               They are recorded from slim-vim.c:  make whim-baselines"; \
	    exit 1; \
	done
	@b=.reference/core-baselines; \
	 if [ -d $$b/screen ] && [ -n "$$(ls -A $$b/screen 2>/dev/null)" ] \
	    && [ -d $$b/memline ] && [ -n "$$(ls -A $$b/memline 2>/dev/null)" ] \
	    && [ -s $$b/ref-excmds.txt ] && [ -s $$b/ref-argv.txt ] \
	    && [ -s $$b/ref-pty.txt ] && [ -s $$b/ref-term.txt ]; then exit 0; fi; \
	 echo "  baselines    $$b is missing or of the old shape: it must hold screen/, memline/,"; \
	 echo "               ref-excmds.txt, ref-argv.txt, ref-pty.txt and ref-term.txt"; \
	 echo "               (tools/zrecord.sh), so no delta from phase 83 on can be checked in full."; \
	 echo "               They are recorded from q82:  make whim-baselines"; \
	 exit 1

# ==== housekeeping
.PHONY: whim-clean
whim-clean:  ## remove the work trees and the built product
	rm -rf $(WHIMWORK)* whim-vim editor.c
