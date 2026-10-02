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
#                        (the core in Go), bin/whim-vim and bin/slim-vim, bin/braaam
#                        and braaam.jar (Java), bin/vijure and vijure.jar (Clojure),
#                        bin/caprice (Haskell), bin/whimsy (Rust)
#   make whim-build      the 145 phases in one process: slim-vim.c -> whim-vim.c,
#                        about fifteen minutes, no cache and no checks
#   make whim-build-check  the same build, required to give the committed bytes back
#   make bin/whim-vim    the C product's binary
#   make bin/slim-vim    the input's binary
#   make editor/editor.go  the translations, generated from whim-vim.c's core
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
all: bin/whim bin/whim-vim bin/slim-vim bin/braaam braaam.jar bin/vijure vijure.jar bin/caprice bin/whimsy  ## everything: fetch if the upstream moved, then the six editors and the jars

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
# bin/slim-vim and bin/whim-vim each record, when built, the digest of the .c
# they were built from, src/slim-vim.c and src/whim-vim.c
# (.cache/stamps/<binary>.sha).  A binary whose source no longer has that digest
# -- fetched, produced again, checked out, edited by hand -- is DELETED, and
# never rebuilt behind anyone's back: `make bin/slim-vim` or `make bin/whim-vim`
# builds it again when it is wanted.  A binary with no stamp was built
# from nobody knows what, and goes the same way.
#
# Checked twice: as make reads this file, which catches every change made
# outside make, and by the two rules that rewrite a source during the run
# (slim-vim.c's fetch, whim-build), since by then the first check has happened.
STAMPS = .cache/stamps

# $(call drop-stale,BINARY,SOURCE), a shell command: remove BINARY when SOURCE is
# not what its stamp says it was built from, and say so.
drop-stale = if [ -e $(1) ] && [ "`sha256sum $(2) 2>/dev/null | cut -c1-64`" != "`cat $(STAMPS)/$(notdir $(1)).sha 2>/dev/null`" ]; then rm -f $(1); printf '  %-12s %s\n' "stale" "$(2) is not what $(1) was built from -- removed; make $(1) builds it again"; fi

# $(call stamp,BINARY,SOURCE), a shell command: record the digest of SOURCE,
# which BINARY was just built from.
stamp = mkdir -p $(STAMPS) && sha256sum $(2) | cut -c1-64 > $(STAMPS)/$(notdir $(1)).sha

empty :=
$(foreach b,slim-vim whim-vim,$(eval _stale := $(shell $(call drop-stale,bin/$(b),src/$(b).c)))$(if $(_stale),$(info $(empty)  $(_stale))))

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
	$(call drop-stale,bin/slim-vim,src/slim-vim.c)

bin/slim-vim: src/slim-vim.c  ## the input's binary, compiled with the one line
	@mkdir -p bin
	@printf '  %-12s %s\n' "compiling" "$(CC) $(CFLAGS) $(LDFLAGS) -o $@ $<"
	@t0=`date +%s`; $(CC) $(CFLAGS) $(LDFLAGS) -o $@ $< && $(call stamp,$@,$<); \
	 printf '  %-12s %s bytes, static, not PIE, %ss\n' "$@" \
	     "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	     "$$((`date +%s` - t0))"

# ==== the product
# whim-vim.c is keyed on slim-vim.c's content, not on mtimes: both are tracked,
# and a fresh clone writes them at checkout time in arbitrary order, so an mtime
# dependency would run a pass on a tree that is exactly right.  src/slim.sha records
# the slim-vim.c the committed whim-vim.c was produced from.
# The goals that generate the editors themselves once whim-vim.c is current --
# `make` (all), bin/whim, editor/editor.go.  When one is being made,
# whim-vim.c's rule builds without whim-build's own editor step, or a build
# from a fresh clone generated twice.
editor-follows := $(if $(MAKECMDGOALS),$(filter all bin/whim editor/editor.go,$(MAKECMDGOALS)),all)

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

# The pipeline in one process: 145 phases, in order, in memory -- internal/build's
# plan and internal/steps' transformations, every boundary printed canonically.  It
# writes whim-vim.c (and editor.go after it), and keeps every boundary in
# .cache/boundaries/.  It proves the text, not the behaviour.  Measured:
# 145 phases, 958 s, 75,381 lines.
# whim-build-check, given those snapshots, proves every phase from its own
# snapshot at once: 65 s at --jobs 32.
.PHONY: whim-build whim-build-check
whim-build:  ## the 145 phases in one process: slim-vim.c -> whim-vim.c
	@printf '\n\033[1m  whim-vim\033[0m  from slim-vim.c: an editor with no runtime\n'
	@go tool whim build --out src/whim-vim.c
	@$(call drop-stale,bin/whim-vim,src/whim-vim.c)
	@$(if $(filter no,$(WHIM_BUILD_EDITOR)),,$(MAKE) --no-print-directory whim-editor)

whim-build-check:  ## the same build, required to give the committed bytes back
	@go tool whim build --check

bin/whim-vim: src/whim-vim.c  ## the C product's binary, compiled with the one line
	@mkdir -p bin
	@printf '  %-12s %s\n' "compiling" "$(CC) $(CFLAGS) $(LDFLAGS) -o $@ $<"
	@t0=`date +%s`; $(CC) $(CFLAGS) $(LDFLAGS) -o $@ $< && $(call stamp,$@,$<); \
	 printf '  %-12s %s bytes, static, not PIE, %ss\n' "$@" \
	     "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" \
	     "$$((`date +%s` - t0))"

# ==== the core, and its translations
# whim-vim.c is one translation unit with two parts: above, the core editor, with
# no preprocessor syntax at all; below, the host, beginning with the #includes --
# and that first directive IS the boundary, marked by nothing else (GOALS.md
# II.4c).  Every translation is written from the core, and each cuts it for
# itself (internal/whim's Cut: everything before the first `^ *# *include `,
# trailing blank lines dropped, a result holding a directive refused): `whim gen`
# for editor.go and the tracked braaam/editor/ and editor.clj, `whim java` and
# `whim clj` for their builds.  Nothing is written under src/; `go tool whim cut`
# prints the core, to read.
#
# editor/editor.go is GENERATED, and so are braaam/editor/ (package whim.editor),
# vijure/src/whim/editor.clj, caprice/Caprice/Editor.hs and its parts, and
# whimsy/src/editor.rs: `go tool whim gen` writes them whole from
# whim-vim.c's core, and they are tracked, so they must be what the program
# writes from the tracked whim-vim.c.  Each is written only when that differs, so
# a current file keeps its mtime.  whim-build writes them after producing
# whim-vim.c; whim-editor-check refuses a stale one.  whim-editor and
# whim-editor-check run on whim-vim.c as it stands: through whim-vim.c's own rule
# they would start a build.
.PHONY: editor/editor.go
editor/editor.go: src/whim-vim.c  ## the translations, generated from whim-vim.c's core: editor.go, braaam/editor/, editor.clj, Editor.hs, editor.rs
	@go tool whim gen

.PHONY: whim-editor
whim-editor:
	@go tool whim gen

.PHONY: whim-editor-check
whim-editor-check:  ## refuse if a tracked editor.go, braaam/editor/, editor.clj, Editor.hs or editor.rs is not what the generator writes
	@go tool whim gen --check

# ==== the editor in Go
# The editor: editor/cmd/whim built -- package editor (editor.go as internal/gen
# writes it from whim-vim.c, and its runtime and Host) on the terminal host.  Go's own build cache decides what compiles again, so the
# rule runs every time and costs nothing when nothing moved.
bin/whim: editor/editor.go force  ## the editor in Go: editor/ built, and its binary
	@mkdir -p bin
	@go build -o $@ ./editor/cmd/whim
	@printf '  %-12s %s bytes, the core in Go (editor/)\n' "$@" \
	    "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`"

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

# ==== the editor in Java
# The editor in Java (braaam/, doc/JAVA.md): the core cut from whim-vim.c and
# written as package whim.editor (braaam/editor/) by crefactor/togo's Java backend, compiled with javac
# beside braaam's runtime, host and glue into lib/braaam/classes, and bin/braaam
# a launcher script that runs it as a binary is run.  It needs a JDK (22 or
# later, for the Foreign Function & Memory API).
.PHONY: bin/braaam
bin/braaam: src/whim-vim.c force  ## the editor in Java: whim.editor generated, compiled, and a launcher
	@go tool whim java

# braaam.jar is the same classes as one executable jar, at the top of the
# tree: `java -jar braaam.jar [args]`.  Its manifest names the main class and
# grants the terminal host its native access (Enable-Native-Access, JDK 22 and
# later), so no flag is needed.  The launcher's -XX flags have no manifest
# form, so java -jar runs without them -- -XX:-DontCompileHugeMethods among
# them, which only a heavy session feels: `java -XX:-DontCompileHugeMethods
# -jar braaam.jar` for one (whim test's heavy case: 1.5 times faster here, 5.7
# times for vijure.jar).
.PHONY: braaam.jar
braaam.jar: bin/braaam  ## the editor in Java as one jar: java -jar braaam.jar [args]
	@printf 'Main-Class: whim.editor.Whim\nEnable-Native-Access: ALL-UNNAMED\n' > lib/braaam/manifest.txt
	@jar --create --file $@ --manifest lib/braaam/manifest.txt -C lib/braaam/classes .
	@printf '  %-12s %s bytes, the core in Java (braaam/): java -jar %s\n' "$@" \
	    "`stat -c%s $@ | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta'`" "$@"

.PHONY: whim-test-java
whim-test-java:  ## the quick suite with the Java editor too, required to answer as the C does
	@go tool whim test --java

# ==== the editor in Clojure
# The editor in Clojure (vijure/, doc/CLOJURE.md): the core cut from
# whim-vim.c and written as the namespace whim.editor by crefactor/togo's
# Clojure backend, AOT-compiled with vijure's glue and launcher on
# braaam's runtime and host into lib/vijure/classes, merged with Clojure's jars
# into lib/vijure/vijure.jar, its AOT cache trained (JDK 25 and later), and
# bin/vijure a launcher script that runs it as a binary is run.  It needs a
# JDK (22 or later) and the `clojure` command, whose jars it copies.  CLJ_EDITOR=F compiles the namespace in F instead of generating one.
.PHONY: bin/vijure
bin/vijure: src/whim-vim.c force  ## the editor in Clojure: whim.editor generated, AOT-compiled, and a launcher
	@go tool whim clj $(if $(CLJ_EDITOR),--editor $(CLJ_EDITOR))

# vijure.jar is the same as one executable jar at the top of the tree:
# `java -jar vijure.jar [args]` (Main-Class whim.cljmain, and the native
# access granted in its manifest, as braaam.jar's; and, as braaam.jar,
# without the launcher's -XX flags).
.PHONY: vijure.jar
vijure.jar: bin/vijure  ## the editor in Clojure as one jar: java -jar vijure.jar [args]
	@go tool whim clj --pack $@

.PHONY: whim-test-clj
whim-test-clj:  ## the quick suite with the Clojure editor too, required to answer as the C does
	@go tool whim test $(if $(CLJ_EDITOR),--clojure-editor $(CLJ_EDITOR),--clojure)

# The editor in Haskell (caprice/, doc/HASKELL.md): the core cut from
# src/whim-vim.c and written as the module Caprice.Editor by the Haskell
# backend, compiled by GHC with caprice's runtime, host and launcher into
# lib/caprice and the program bin/caprice.  The core is one module, some
# three minutes of GHC's time; a build whose core has not moved skips it.
.PHONY: bin/caprice
bin/caprice: src/whim-vim.c force  ## the editor in Haskell: Caprice.Editor generated, compiled by GHC
	@go tool whim caprice

.PHONY: whim-test-hs
whim-test-hs:  ## the quick suite with the Haskell editor too, required to answer as the C does
	@go tool whim test --haskell

# The editor in Rust (whimsy/, doc/RUST.md): the core cut from
# src/whim-vim.c and written as the module `editor` by the Rust backend,
# compiled by cargo -- offline, std alone -- with whimsy's runtime, host and
# launcher in lib/whimsy into the program bin/whimsy.  Forty seconds of
# rustc's time when the core moved; a build whose sources have not moved
# skips it.
.PHONY: bin/whimsy
bin/whimsy: src/whim-vim.c force  ## the editor in Rust: the module editor generated, compiled by cargo
	@go tool whim whimsy

.PHONY: whim-test-rs
whim-test-rs:  ## the quick suite with the Rust editor too, required to answer as the C does
	@go tool whim test --rust

# ==== the tests
.PHONY: whim-test
whim-test:  ## the quick suite: 80 key sessions, required to behave as HEAD's whim-vim.c does
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

# ==== housekeeping
.PHONY: clean
clean:  ## remove the built binaries and jars
	rm -f bin/slim-vim bin/whim-vim bin/whim bin/braaam bin/vijure bin/caprice bin/whimsy braaam.jar vijure.jar editor.lgo
	rm -rf lib/braaam lib/vijure lib/caprice lib/whimsy .cache/caprice-suite .cache/whimsy-suite

.PHONY: clean-cache
clean-cache:  ## remove .cache/ (the Go build cache, the sweep's compiles, the stamps)
	rm -rf .cache

# Bytes to store and symbols to provide, the input and the product side by side.
.PHONY: score
score:  ## bytes to store and symbols to provide: the input beside the product
	@go tool whim score

force: ;

.PHONY: force
