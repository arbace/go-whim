#!/bin/sh
# The guest editor on the Mac, Hypervisor.framework on Apple silicon
# (doc/GUEST.md, *Running on the Mac*): run from the repository's root,
# on the Mac, with Go and Xcode's command-line tools.  Every Go build is
# native and CGO_ENABLED=0 (purego, no cgo); every program that creates a VM
# is signed ad hoc with the hypervisor entitlement.  The guest images are
# built on Linux (go tool whim guest --arch arm64 --image; Apple's clang has
# no ELF linker) and copied to bin/.
#
#   guest/mac/mac.sh check          what the Mac is: macOS, the chip, kern.hv_support, the tools
#   guest/mac/mac.sh hv             hv's arm64 tests on the framework: the smallest guests
#   guest/mac/mac.sh build          bin/whim-guest, signed
#   guest/mac/mac.sh hello          bin/whim-guest on the hello image: prints hello, exits 3
#   guest/mac/mac.sh run [ARG...]   the editor: bin/whim-guest on bin/whim-guest.elf
#   guest/mac/mac.sh c              the C reference, bin/whim-vim-mac, by clang with guest/mac/shim.h
#   guest/mac/mac.sh suite [--wide] the suite: the guest held to the C, with its control
set -eu
cd "$(dirname "$0")/../.."
export TMPDIR="$PWD/.tmp" CGO_ENABLED=0
mkdir -p "$TMPDIR" bin
ent=guest/mac/whim-guest.entitlements
sign() { codesign --sign - --entitlements "$ent" --force "$1"; }
need() { [ -f "$1" ] || { echo "mac.sh: no $1: $2" >&2; exit 1; }; }

case "${1:-}" in
check)
	sw_vers
	uname -m
	sysctl kern.hv_support
	go version
	clang --version | head -1
	;;
hv)
	go test -c -o bin/hv.test ./hv
	sign bin/hv.test
	bin/hv.test -test.v
	;;
build)
	go build -o bin/whim-guest ./vmm/cmd/whim-guest
	sign bin/whim-guest
	codesign --display --entitlements - bin/whim-guest
	[ -f bin/whim-guest.elf ] || echo "mac.sh: now copy the arm64 image to bin/whim-guest.elf" >&2
	;;
hello)
	need bin/whim-guest "mac.sh build"
	need bin/whim-guest-hello-arm64.elf "go tool whim guest --arch arm64 --hello --image, on Linux"
	s=0
	WHIM_GUEST_IMAGE=bin/whim-guest-hello-arm64.elf bin/whim-guest || s=$?
	echo "exit $s (3 expected)"
	;;
run)
	shift
	need bin/whim-guest "mac.sh build"
	need bin/whim-guest.elf "go tool whim guest --arch arm64 --image, on Linux, copied here"
	exec bin/whim-guest "$@"
	;;
c)
	# The host asserts the core's PATH_MAX, 4096, equal to the header's; macOS's
	# is 1024. The host uses PATH_MAX nowhere else (the core's paths are its
	# own buffers), so the Mac's copy drops that one Linux guard.
	mkdir -p .tmp
	sed '/^static_assert(4096 == PATH_MAX, "PATH_MAX");$/d' src/whim-vim.c > .tmp/whim-vim-mac.c
	clang -std=gnu23 -O0 -w -include guest/mac/shim.h -o bin/whim-vim-mac .tmp/whim-vim-mac.c
	;;
suite)
	need bin/whim-guest "mac.sh build"
	need bin/whim-guest.elf "go tool whim guest --arch arm64 --image, on Linux, copied here"
	need bin/whim-vim-mac "mac.sh c"
	go test -c -o bin/suite.test ./internal/suite
	if [ "${2:-}" = --wide ]; then
		export WHIM_SUITE_WIDE=1 WHIM_SUITE_SRC=src/whim-vim.c
	fi
	WHIM_SUITE_C=bin/whim-vim-mac WHIM_SUITE_GUEST=bin/whim-guest WHIM_SUITE_GUEST_IMAGE=bin/whim-guest.elf \
		bin/suite.test -test.run TestGuestPrebuilt -test.v
	;;
*)
	sed -n '10,16s/^# //p' "$0" >&2
	exit 2
	;;
esac
