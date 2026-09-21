# go-whim: whim-vim.c = G(slim-vim.c), and zero-vim.c = H(whim-vim.c).
#
# The input is ONE FILE, slim-vim.c, from github.com/arbace/slim-vim -- vim 9.2 as
# a single translation unit, produced there by its own pipeline.  This makefile
# asks that repository for its head, fetches slim-vim.c (and vim's LICENSE, which
# every modified vim must carry) at exactly that commit, and records the commit in
# upstream.sha.  Everything downstream is keyed on CONTENT, not on the commit:
# whim-vim.c is produced again only when slim-vim.c's digest moved (slim.sha), and
# zero-vim.c only when whim-vim.c's did (whim.sha).  So a slim-vim commit that
# does not change slim-vim.c costs a fetch and nothing else.
#
#   make                 fetch if the upstream moved, then whim-vim and zero-vim
#   make whim-verify     every whim boundary, reproduced at once
#   make zero-verify     every zero boundary, reproduced at once
#
# whim.mk and zero.mk are the pipelines; tools/ and pipes/ are what they run.

CC      = gcc
CFLAGS  = -O0
LDFLAGS = -static -s

SLIMVIM_URL    = https://github.com/arbace/slim-vim
SLIMVIM_BRANCH = main
SLIMVIM_RAW    = https://raw.githubusercontent.com/arbace/slim-vim

.DEFAULT_GOAL := all

.PHONY: all
all: whim-vim zero-vim

include whim.mk
include zero.mk

# --- the input ------------------------------------------------------------
# It cannot be a timestamp -- a clone writes every file at checkout time in
# arbitrary order -- so the question asked is the remote's head against
# upstream.sha.  An unreachable remote with a slim-vim.c on disk builds what is
# there and says so; with none on disk there is nothing to build from.
slim-vim.c: force
	@set -e; \
	live=`GIT_TERMINAL_PROMPT=0 timeout 60 git ls-remote $(SLIMVIM_URL) $(SLIMVIM_BRANCH) 2>/dev/null | cut -f1` || true; \
	if [ -z "$$live" ]; then \
	    if [ -f $@ ]; then echo "  upstream     UNREACHABLE -- using the slim-vim.c on disk"; exit 0; fi; \
	    echo "  upstream     UNREACHABLE and there is no slim-vim.c to build from"; exit 1; \
	fi; \
	if [ -f $@ ] && [ "$$live" = "`cat upstream.sha 2>/dev/null`" ]; then \
	    printf '  %-12s %s unchanged -- slim-vim.c is current\n' "upstream" "`echo $$live | cut -c1-12`"; \
	    exit 0; \
	fi; \
	printf '  %-12s %s -- fetching slim-vim.c at that commit\n' "upstream" "`echo $$live | cut -c1-12`"; \
	tmp=`mktemp -d`; \
	curl -fsSL "$(SLIMVIM_RAW)/$$live/slim-vim.c" -o "$$tmp/slim-vim.c"; \
	curl -fsSL "$(SLIMVIM_RAW)/$$live/LICENSE" -o "$$tmp/LICENSE"; \
	[ -s "$$tmp/slim-vim.c" ] && [ -s "$$tmp/LICENSE" ] || { echo "  upstream     the fetch came back empty"; rm -rf "$$tmp"; exit 1; }; \
	mv -f "$$tmp/slim-vim.c" $@; \
	mv -f "$$tmp/LICENSE" LICENSE; \
	rm -rf "$$tmp"; \
	echo "$$live" > upstream.sha; \
	printf '  %-12s %s lines, sha256 %s\n' "slim-vim.c" "`grep -c '' $@`" "`sha256sum $@ | cut -c1-12`"

# whim.mk decides by slim-vim.c's digest; it must see the fetched file.
whim-vim.c: slim-vim.c

.PHONY: clean
clean:
	rm -f slim-vim whim-vim zero-vim

.PHONY: clean-cache
clean-cache:
	rm -rf .cache

# Bytes to store and symbols to provide, the three editors side by side --
# slim-vim.c being the input, it is measured too.
.PHONY: score
score:
	@ZEROCFLAGS='$(ZEROCFLAGS)' ZEROLDFLAGS='$(ZEROLDFLAGS)' tools/score.sh

force: ;

.PHONY: force
