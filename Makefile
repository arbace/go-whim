# go-whim: whim-vim.c = G(slim-vim.c).
#
# The input is ONE FILE, slim-vim.c, from github.com/arbace/slim-vim -- vim 9.2 as
# a single translation unit, produced there by its own pipeline.  This makefile
# asks that repository for its head, fetches slim-vim.c (and vim's LICENSE, which
# every modified vim must carry) at exactly that commit, and records the commit in
# upstream.sha.  Everything downstream is keyed on CONTENT, not on the commit:
# whim-vim.c is produced again only when slim-vim.c's digest moved (slim.sha).
# So a slim-vim commit that does not change slim-vim.c costs a fetch and nothing
# else.
#
#   make                 fetch if the upstream moved, then bin/whim: the editor, the
#                        core in Go (editor/) built with its runtime and host
#   make whim-build      the 164 phases in one process: slim-vim.c -> whim-vim.c,
#                        twenty minutes, no cache and no checks
#   make whim-build-check  the same build, required to give the committed bytes back
#   make whim-vim        the C product's binary
#   make slim-vim        the input's binary, with the line phase 0 starts from
#   make whim-verify     every phase's check and every declared delta
#   make editor.c        the core, cut from whim-vim.c at its first #include
#   make editor/editor.go  the core in Go, generated from editor.c
#
# whim.mk is the pipeline; tools and the phases (phase/NNN) are what it runs.

# Every temporary a recipe makes -- mktemp, Go's os.MkdirTemp, the harnesses'
# scratch homes, verifypass's and specpass's scratch roots -- goes in .tmp/
# here, not the shared /tmp.  Gitignored.
export TMPDIR := $(CURDIR)/.tmp
$(shell mkdir -p $(TMPDIR))

CC      = gcc

SLIMVIM_URL    = https://github.com/arbace/slim-vim
SLIMVIM_BRANCH = main
SLIMVIM_RAW    = https://raw.githubusercontent.com/arbace/slim-vim

.DEFAULT_GOAL := all

# ==== the product
.PHONY: all
all: bin/whim  ## the editor: fetch if the upstream moved, then bin/whim

include whim.mk

# --- help -----------------------------------------------------------------
# Every target worth asking for carries its own one-line description, as a `##`
# after the colon, and this reads them back.  A target with no `##` is
# machinery -- a file rule, a guard, a helper another target calls -- and is
# deliberately not listed.
.PHONY: help
help:
	@printf '\n  \033[1mgo-whim\033[0m -- whim-vim.c = G(slim-vim.c), and editor/ is that core in Go\n\n'
	@awk 'BEGIN { FS = ":.*## " } \
	     /^# ==== / { printf "\n  \033[1m%s\033[0m\n", substr($$0, 8); next } \
	     /^[a-zA-Z0-9_.\/%-]+:.*## / { printf "    %-22s %s\n", $$1, $$2 }' \
	    $(MAKEFILE_LIST)
	@printf '\n    %-22s %s\n\n' "make -n <target>" "what a target would run, without running it"

# The editor: editor/ built -- editor.go as tx/skel writes it from whim-vim.c,
# crt.go and host.go.  Go's own build cache decides what compiles again, so the
# rule runs every time and costs nothing when nothing moved.
bin/whim: editor/editor.go force  ## the editor binary alone, from editor/
	@mkdir -p bin
	@go build -o $@ ./editor
	@printf '  %-12s %s bytes, the core in Go (editor/)\n' "$@" \
	    "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"

# --- the input ------------------------------------------------------------
# It cannot be a timestamp -- a clone writes every file at checkout time in
# arbitrary order -- so the question asked is the remote's head against
# upstream.sha.  An unreachable remote with a slim-vim.c on disk builds what is
# there and says so; with none on disk there is nothing to build from.
slim-vim.c: force  ## fetch the input at arbace/slim-vim's head, if it moved
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

# ==== housekeeping
.PHONY: clean
clean:  ## remove the built binaries
	rm -f slim-vim whim-vim bin/whim

.PHONY: clean-cache
clean-cache:  ## remove .cache/ (the Go build cache and the boundaries)
	rm -rf .cache

# Bytes to store and symbols to provide, the input and the product side by side.
.PHONY: score
score:  ## bytes to store and symbols to provide: the input beside the product
	@WHIMCFLAGS='$(WHIMCFLAGS)' WHIMLDFLAGS='$(WHIMLDFLAGS)' tools/st.sh score

force: ;

.PHONY: force
