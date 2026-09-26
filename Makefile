# go-whim: whim-vim.c = G(slim-vim.c), and editor/ is that core in Go.
#
# The input is ONE FILE, slim-vim.c, from github.com/arbace/slim-vim -- vim 9.2 as
# a single translation unit, produced there by its own pipeline.  This makefile
# asks that repository for its head, fetches slim-vim.c (and vim's LICENSE, which
# every modified vim must carry) at exactly that commit, and records the commit in
# src/upstream.sha; the input, the product, their binaries and both digests
# live in src/.  Everything downstream is keyed on CONTENT, not on the commit:
# whim-vim.c is produced again only when slim-vim.c's digest moved (src/slim.sha).
# So a slim-vim commit that does not change slim-vim.c costs a fetch and nothing
# else.
#
# The pipeline is 164 phases that remove capability on purpose: phases 0-82
# (GOALS.md Part I) leave an editor with no runtime to install; phases 83 on
# (Part II) turn it into an embeddable core -- no filesystem, the host behind a
# line in the file, no libc the core names, the text a tree -- and then remove
# from it what translating it to Go had to work around.  ONE PATH: whim-build
# applies them in one process, in memory (internal/build).  There is no test
# suite and no memoize; what answers for the product is that it is TRACKED, and
# whim-build-check requires the committed whim-vim.c back, byte for byte, from
# the committed slim-vim.c.  448e9a8 is the last commit with the old suite.
#
#   make                 fetch if the upstream moved, then every editor: bin/whim
#                        (the core in Go), src/whim-vim and src/slim-vim, bin/braaam
#                        and braaam.jar (Java), bin/vijure and vijure.jar (Clojure)
#   make whim-build      the 143 phases in one process: slim-vim.c -> whim-vim.c,
#                        about fifteen minutes, no cache and no checks
#   make whim-build-check  the same build, required to give the committed bytes back
#   make whim-vim        the C product's binary
#   make slim-vim        the input's binary
#   make src/editor.c    the core, cut from whim-vim.c at its first #include
#   make editor/editor.go  the core in Go, generated from src/editor.c
#   make help            every target, with a line each

# Every temporary a recipe makes -- mktemp, Go's os.MkdirTemp, a build's work
# tree -- goes in .tmp/ here, not the shared /tmp.  Gitignored.
export TMPDIR := $(CURDIR)/.tmp
$(shell mkdir -p $(TMPDIR))

CC      = gcc

SLIMVIM_URL    = https://github.com/arbace/slim-vim
SLIMVIM_BRANCH = main
SLIMVIM_RAW    = https://raw.githubusercontent.com/arbace/slim-vim

# THE COMPILE LINE IS ONE LINE, for the input, the product and every boundary:
# an ordinary static executable (EXEC, no dynamic section, no relocation), no
# stack protector, no -g.  internal/build/compile.go states it for the tools;
# this states it for the two binary rules.
CFLAGS  = -O0 -fno-stack-protector
LDFLAGS = -static -no-pie -s

.DEFAULT_GOAL := all

.PHONY: all
all: bin/whim src/whim-vim src/slim-vim bin/braaam braaam.jar bin/vijure vijure.jar  ## everything: fetch if the upstream moved, then the four editors and the jars

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

# --- a binary is only ever the build of its source as it stands ----------
# slim-vim and whim-vim each record, when built, the digest of the .c they were
# built from (.cache/stamps/<binary>.sha).  A binary whose source no longer has
# that digest -- fetched, produced again, checked out, edited by hand -- is
# DELETED, and never rebuilt behind anyone's back: `make slim-vim` or `make
# whim-vim` builds it again when it is wanted.  A binary with no stamp was built
# from nobody knows what, and goes the same way.
#
# Checked twice: as make reads this file, which catches every change made
# outside make, and by the two rules that rewrite a source during the run
# (slim-vim.c's fetch, whim-build), since by then the first check has happened.
STAMPS = .cache/stamps

# $(call drop-stale,BINARY), a shell command: remove BINARY when BINARY.c is not
# what its stamp says it was built from, and say so.
drop-stale = if [ -e $(1) ] && [ "`sha256sum $(1).c 2>/dev/null | cut -c1-64`" != "`cat $(STAMPS)/$(notdir $(1)).sha 2>/dev/null`" ]; then rm -f $(1); printf '  %-12s %s\n' "stale" "$(1).c is not what $(1) was built from -- removed; make $(notdir $(1)) builds it again"; fi

# $(call stamp,BINARY), a shell command: record the digest of BINARY.c, which
# BINARY was just built from.
stamp = mkdir -p $(STAMPS) && sha256sum $(1).c | cut -c1-64 > $(STAMPS)/$(notdir $(1)).sha

empty :=
$(foreach b,src/slim-vim src/whim-vim,$(eval _stale := $(shell $(call drop-stale,$(b))))$(if $(_stale),$(info $(empty)  $(_stale))))

# ==== the input
# It cannot be a timestamp -- a clone writes every file at checkout time in
# arbitrary order -- so the question asked is the remote's head against
# src/upstream.sha.  An unreachable remote with a slim-vim.c on disk builds what is
# there and says so; with none on disk there is nothing to build from.
src/slim-vim.c: force  ## fetch the input at arbace/slim-vim's head, if it moved
	@set -e; \
	live=`GIT_TERMINAL_PROMPT=0 timeout 60 git ls-remote $(SLIMVIM_URL) $(SLIMVIM_BRANCH) 2>/dev/null | cut -f1` || true; \
	if [ -z "$$live" ]; then \
	    if [ -f $@ ]; then echo "  upstream     UNREACHABLE -- using the slim-vim.c on disk"; exit 0; fi; \
	    echo "  upstream     UNREACHABLE and there is no slim-vim.c to build from"; exit 1; \
	fi; \
	if [ -f $@ ] && [ "$$live" = "`cat src/upstream.sha 2>/dev/null`" ]; then \
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
	echo "$$live" > src/upstream.sha; \
	printf '  %-12s %s lines, sha256 %s\n' "$@" "`grep -c '' $@`" "`sha256sum $@ | cut -c1-12`"; \
	$(call drop-stale,src/slim-vim)

src/slim-vim: src/slim-vim.c
	@printf '  %-12s %s\n' "compiling" "$(CC) $(CFLAGS) $(LDFLAGS) -o $@ $<"
	@t0=`date +%s`; $(CC) $(CFLAGS) $(LDFLAGS) -o $@ $< && $(call stamp,$@); \
	 printf '  %-12s %s bytes, static, not PIE, %ss\n' "$@" \
	     "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	     "$$((`date +%s` - t0))"

.PHONY: slim-vim
slim-vim: src/slim-vim  ## the input's binary, src/slim-vim, compiled with the one line

# ==== the product
# whim-vim.c is keyed on slim-vim.c's content, not on mtimes: both are tracked,
# and a fresh clone writes them at checkout time in arbitrary order, so an mtime
# dependency would run a pass on a tree that is exactly right.  src/slim.sha records
# the slim-vim.c the committed whim-vim.c was produced from.
# The goals that cut src/editor.c and generate the editors themselves once
# whim-vim.c is current -- `make` (all), bin/whim, src/editor.c, editor/editor.go.
# When one is being made, whim-vim.c's rule builds without whim-build's own
# editor step, or a build from a fresh clone cut and generated twice.
editor-follows := $(if $(MAKECMDGOALS),$(filter all bin/whim src/editor.c editor/editor.go,$(MAKECMDGOALS)),all)

src/whim-vim.c: src/slim-vim.c force
	@set -e; \
	live=`sha256sum src/slim-vim.c | cut -c1-64`; \
	if [ -f $@ ] && [ "$$live" = "`cat src/slim.sha 2>/dev/null`" ]; then \
	    printf '  %-12s %s unchanged -- whim-vim.c is current\n' "slim-vim.c" "`echo $$live | cut -c1-12`"; \
	    exit 0; \
	fi; \
	printf '  %-12s %s -- whim-vim.c must be produced\n' "slim-vim.c" "`echo $$live | cut -c1-12`"; \
	$(MAKE) --no-print-directory whim-build $(if $(editor-follows),WHIM_BUILD_EDITOR=no); \
	echo "$$live" > src/slim.sha

# The pipeline in one process: 164 phases, in order, in memory -- internal/build's
# plan and internal/steps' transformations, every boundary printed canonically.  It
# writes whim-vim.c (and editor.go after it), and keeps every boundary in
# .cache/boundaries/.  It proves the text, not the behaviour.  Measured: 170
# 143 phases, 920 s, 75,539 lines.
# whim-build-check, given those snapshots, proves every phase from its own
# snapshot at once: 65 s at --jobs 32.
.PHONY: whim-build whim-build-check
whim-build:  ## the 143 phases in one process: slim-vim.c -> whim-vim.c
	@printf '\n\033[1m  whim-vim\033[0m  from slim-vim.c: an editor with no runtime\n'
	@go tool whim build --out src/whim-vim.c
	@$(call drop-stale,src/whim-vim)
	@$(if $(filter no,$(WHIM_BUILD_EDITOR)),,$(MAKE) --no-print-directory whim-editor)

whim-build-check:  ## the same build, required to give the committed bytes back
	@go tool whim build --check

src/whim-vim: src/whim-vim.c
	@printf '  %-12s %s\n' "compiling" "$(CC) $(CFLAGS) $(LDFLAGS) -o $@ $<"
	@t0=`date +%s`; $(CC) $(CFLAGS) $(LDFLAGS) -o $@ $< && $(call stamp,$@); \
	 printf '  %-12s %s bytes, static, not PIE, %ss\n' "$@" \
	     "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	     "$$((`date +%s` - t0))"

.PHONY: whim-vim
whim-vim: src/whim-vim  ## the C product's binary, src/whim-vim, compiled with the one line

# ==== the editor
# whim-vim.c is one translation unit with two parts: above, the core editor, with
# no preprocessor syntax at all; below, the host, beginning with the #includes --
# and that first directive IS the boundary, marked by nothing else (GOALS.md
# II.4c).  The cut stops at the first `^ *# *include ` (whitespace after `#` is
# insignificant to C) and drops trailing blank lines, and it REFUSES a result that
# holds a directive: the defining property of the upper part is that it has none,
# so a cut that produced one found the wrong line.  editor.c does not compile on
# its own -- the core calls the musl_ functions the host defines below -- and is
# not meant to; it must parse.
#
# A canned recipe: editor.c's rule runs it after whim-vim.c is current, and
# whim-editor and whim-editor-check run it on whim-vim.c as it stands -- through
# whim-vim.c's own rule they would start a build.
define cut-editor
	@awk '/^ *# *include / { exit } { a[NR] = $$0; if (NF) last = NR } \
	      END { for (i = 1; i <= last; i++) print a[i] }' src/whim-vim.c > src/editor.c
	@if grep -q '^ *#' src/editor.c; then \
	    echo "  src/editor.c REFUSED -- the cut holds a directive, so it found the wrong line:"; \
	    grep -n '^ *#' src/editor.c | head -3 | sed 's/^/               /'; \
	    rm -f src/editor.c; exit 1; \
	 fi
	@printf '  %-12s %s lines, cut at the first #include of %s\n' src/editor.c \
	    "`grep -c '' src/editor.c | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	    "`grep -c '' src/whim-vim.c | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"
endef

.PHONY: src/editor.c
src/editor.c: src/whim-vim.c  ## the core, cut from whim-vim.c at its first #include
	$(cut-editor)

# editor/editor.go is GENERATED: internal/gen writes it whole from src/editor.c
# (go tool whim gen), and it is tracked, so it must be what the program writes
# from the tracked whim-vim.c.  It is written only when that differs, so a current
# file keeps its mtime.  whim-build writes it after producing whim-vim.c;
# whim-editor-check refuses a stale one.
.PHONY: editor/editor.go
editor/editor.go: src/editor.c  ## the core in Go, generated whole from src/editor.c
	@go tool whim gen

.PHONY: whim-editor
whim-editor:
	$(cut-editor)
	@go tool whim gen

.PHONY: whim-test
whim-test:  ## the quick suite: 45 key sessions, required to behave as HEAD's whim-vim.c does
	@go tool whim test

.PHONY: whim-test-wide
whim-test-wide:  ## the optional wide suite: 240 cases in four groups (keys, Ex commands, argv, a real terminal)
	@go tool whim test --wide

# The Go packages' own tests, in both modules: `go test ./...` at the root
# does not reach crefactor/, a module of its own, so it is run from there too.
# vet's unreachable check is off in both: its three reports are in the forked
# front end's upstream code (cc/cpp.go, cc/parser.go), kept as upstream wrote it.
.PHONY: go-test
go-test:  ## the Go tests of both modules: this one and crefactor/
	@go vet -unreachable=false ./... && go test ./...
	@cd crefactor && go vet -unreachable=false ./... && go test ./...

.PHONY: whim-editor-check
whim-editor-check:  ## refuse if a tracked editor.go, Editor.java or editor.clj is not what the generator writes
	$(cut-editor)
	@go tool whim gen --check

# The editor: editor/cmd/whim built -- package editor (editor.go as internal/gen
# writes it from whim-vim.c, and its runtime and Host) on the terminal host.  Go's own build cache decides what compiles again, so the
# rule runs every time and costs nothing when nothing moved.
bin/whim: editor/editor.go force  ## the editor binary alone, from editor/
	@mkdir -p bin
	@go build -o $@ ./editor/cmd/whim
	@printf '  %-12s %s bytes, the core in Go (editor/)\n' "$@" \
	    "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"

# The editor in Java (braaam/, doc/JAVA.md): the core cut from whim-vim.c and
# written as Editor.java by crefactor/togo's Java backend, compiled with javac
# beside braaam's runtime, host and glue into lib/braaam/classes, and bin/braaam
# a launcher script that runs it as a binary is run.  Not part of `all`: it
# needs a JDK (22 or later, for the Foreign Function & Memory API).
.PHONY: bin/braaam
bin/braaam: src/whim-vim.c force  ## the editor in Java: Editor.java generated, compiled, and a launcher
	@go tool whim java

# braaam.jar is the same classes as one executable jar, at the top of the
# tree: `java -jar braaam.jar [args]`.  Its manifest names the main class and
# grants the terminal host its native access (Enable-Native-Access, JDK 22 and
# later), so no flag is needed; the launcher's -XX flags only tune a short run
# and have no manifest form, so java -jar runs without them.
.PHONY: braaam.jar
braaam.jar: bin/braaam  ## the editor in Java as one jar: java -jar braaam.jar [args]
	@printf 'Main-Class: Whim\nEnable-Native-Access: ALL-UNNAMED\n' > lib/braaam/manifest.txt
	@jar --create --file $@ --manifest lib/braaam/manifest.txt -C lib/braaam/classes .
	@printf '  %-12s %s bytes, the core in Java (braaam/): java -jar %s\n' "$@" \
	    "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" "$@"

# The editor in Clojure (vijure/, doc/CLOJURE.md): the core cut from
# whim-vim.c and written as the namespace whim.editor by crefactor/togo's
# Clojure backend, AOT-compiled with vijure's glue and launcher on
# braaam's runtime and host into lib/vijure/classes, merged with Clojure's jars
# into lib/vijure/vijure.jar, its AOT cache trained (JDK 25 and later), and
# bin/vijure a launcher script that runs it as a binary is run.  Not part of
# `all`: it needs a JDK (22 or later) and the `clojure` command, whose jars it
# copies.  CLJ_EDITOR=F compiles the namespace in F instead of generating one.
.PHONY: bin/vijure
bin/vijure: src/whim-vim.c force  ## the editor in Clojure: whim.editor generated, AOT-compiled, and a launcher
	@go tool whim clj $(if $(CLJ_EDITOR),--editor $(CLJ_EDITOR))

# vijure.jar is the same as one executable jar at the top of the tree:
# `java -jar vijure.jar [args]` (Main-Class whim.cljmain, and the native
# access granted in its manifest, as braaam.jar's).
.PHONY: vijure.jar
vijure.jar: bin/vijure  ## the editor in Clojure as one jar: java -jar vijure.jar [args]
	@go tool whim clj --pack $@

# editor.lgo is the Go editor as ONE go-lisp file (doc/GO-LISP.md): editor/'s
# files merged into one Go file (whim gocat), converted by go-lisp's golisp,
# then compiled by go-lisp's go as the proof it is a package.  It needs the
# go-lisp toolchain, which is not this repository's: GOLISP_ROOT names it, and
# its `go tool golisp` converts -- or GOLISP names a golisp binary, and the go
# beside it compiles.  Not tracked, not part of `all`.
GOLISP ?= golisp
GOLISP_ROOT ?=
.PHONY: editor.lgo
editor.lgo:  ## the Go editor as one go-lisp file, compiled (needs go-lisp: GOLISP_ROOT=.../go-lisp)
	@if [ -n "$(GOLISP_ROOT)" ]; then \
	    golisp="$(GOLISP_ROOT)/bin/go tool golisp"; gobin="$(GOLISP_ROOT)/bin/go"; \
	    [ -x "$$gobin" ] || { echo "  editor.lgo   GOLISP_ROOT=$(GOLISP_ROOT) has no bin/go (doc/GO-LISP.md)"; exit 1; }; \
	 else \
	    golisp=`command -v $(GOLISP) 2>/dev/null`; gobin="`dirname "$$golisp" 2>/dev/null`/go"; \
	    [ -n "$$golisp" ] || { echo "  editor.lgo   needs go-lisp: make editor.lgo GOLISP_ROOT=/path/to/go-lisp (doc/GO-LISP.md)"; exit 1; }; \
	 fi; \
	 set -e; \
	 go tool whim gocat editor > $(TMPDIR)/editor-all.go; \
	 GOTOOLCHAIN=local $$golisp go2lisp $(TMPDIR)/editor-all.go > $@.tmp; \
	 rm -f $(TMPDIR)/editor-all.go; \
	 check=`mktemp -d`; \
	 cp $@.tmp $$check/editor.lgo; \
	 printf 'module lgocheck\n\ngo 1.27\n' > $$check/go.mod; \
	 (cd $$check && GOTOOLCHAIN=local GOFLAGS= "$$gobin" build ./...) \
	    || { echo "  editor.lgo   REFUSED -- go-lisp's go does not compile it"; rm -rf $$check $@.tmp; exit 1; }; \
	 rm -rf $$check; \
	 mv $@.tmp $@; \
	 printf '  %-12s %s lines, the Go editor in go-lisp, one file; go-lisp compiles it\n' $@ \
	    "`grep -c '' $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"

.PHONY: whim-test-java
whim-test-java:  ## the quick suite with the Java editor too, required to answer as the C does
	@go tool whim test --java

.PHONY: whim-test-clj
whim-test-clj:  ## the quick suite with the Clojure editor too, required to answer as the C does
	@go tool whim test $(if $(CLJ_EDITOR),--clojure-editor $(CLJ_EDITOR),--clojure)

# ==== housekeeping
.PHONY: clean
clean:  ## remove the built binaries and src/editor.c
	rm -f src/slim-vim src/whim-vim bin/whim bin/braaam bin/vijure braaam.jar vijure.jar editor.lgo src/editor.c
	rm -rf lib/braaam lib/vijure

.PHONY: clean-cache
clean-cache:  ## remove .cache/ (the Go build cache, the sweep's compiles, the stamps)
	rm -rf .cache

# Bytes to store and symbols to provide, the input and the product side by side.
.PHONY: score
score:  ## bytes to store and symbols to provide: the input beside the product
	@go tool whim score

force: ;

.PHONY: force
