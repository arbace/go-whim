#!/bin/sh
# The guest editor on the Mac, Hypervisor.framework on Apple silicon
# (doc/GUEST.md, *Running on the Mac*): run from the repository's root,
# on the Mac, with Go and Xcode's command-line tools.  Every Go build is
# native and CGO_ENABLED=0 (purego, no cgo); every program that creates a VM
# is signed ad hoc with the hypervisor entitlement.  The guest images are
# built on Linux (go tool whim guest [--go] --arch arm64 --image; Apple's
# clang has no ELF linker) and copied to bin/.  --go takes the Go guest's
# image, bin/whim-guest-go.elf, for the C guest's, under the same monitor.
#
#   guest/mac/mac.sh check                 what the Mac is: macOS, the chip, kern.hv_support, the tools
#   guest/mac/mac.sh hv                    hv's arm64 tests on the framework: the smallest guests
#   guest/mac/mac.sh build                 bin/whim-guest, signed
#   guest/mac/mac.sh hello [--go]          the hello image: prints hello, exits 3 (--go: the Go guest types hello, exits 0)
#   guest/mac/mac.sh run [--go] [ARG...]   the editor: bin/whim-guest on bin/whim-guest.elf (--go: whim-guest-go.elf)
#   guest/mac/mac.sh c                     the C reference, bin/whim-vim-mac, by clang with guest/mac/shim.h
#   guest/mac/mac.sh suite [--go] [--wide] the suite: the guest held to the C, with its control
#   guest/mac/mac.sh heavy [--go]          the heavy case: the C, then the guest, answers compared, times reported
set -eu
cd "$(dirname "$0")/../.."
export TMPDIR="$PWD/.tmp" CGO_ENABLED=0
mkdir -p "$TMPDIR" bin
ent=guest/mac/whim-guest.entitlements
sign() { codesign --sign - --entitlements "$ent" --force "$1"; }
need() { [ -f "$1" ] || { echo "mac.sh: no $1: $2" >&2; exit 1; }; }

cmd=${1:-}
[ $# -gt 0 ] && shift
img=bin/whim-guest.elf how="go tool whim guest --arch arm64 --image, on Linux, copied here"
if [ "${1:-}" = --go ]; then
	shift
	img=bin/whim-guest-go.elf how="go tool whim guest --go --arch arm64 --image, on Linux, copied here as $img"
fi
# suite: the guest held to bin/whim-vim-mac by TestGuestPrebuilt, the image
# beside the monitor named by WHIM_SUITE_GUEST_IMAGE; its arguments the
# test's environment.
suite() {
	need bin/whim-guest "mac.sh build"
	need "$img" "$how"
	need bin/whim-vim-mac "mac.sh c"
	go test -c -o bin/suite.test ./internal/suite
	env "$@" WHIM_SUITE_C=bin/whim-vim-mac WHIM_SUITE_GUEST=bin/whim-guest WHIM_SUITE_GUEST_IMAGE="$img" \
		bin/suite.test -test.run TestGuestPrebuilt -test.v
}

case "$cmd" in
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
	[ -f bin/whim-guest-go.elf ] || echo "mac.sh: and the Go guest's to bin/whim-guest-go.elf" >&2
	;;
hello)
	need bin/whim-guest "mac.sh build"
	s=0
	if [ "$img" = bin/whim-guest-go.elf ]; then
		# The Go guest has no stand-in: the editor itself, its keys from a
		# file -- hello typed, :q! -- its screen printed.
		need "$img" "$how"
		printf 'ihello\033:q!\r' > "$TMPDIR/hello-go.keys"
		WHIM_GUEST_IMAGE=$img bin/whim-guest < "$TMPDIR/hello-go.keys" || s=$?
		echo
		echo "exit $s (0 expected, hello on the screen above)"
	else
		need bin/whim-guest-hello-arm64.elf "go tool whim guest --arch arm64 --hello --image, on Linux"
		WHIM_GUEST_IMAGE=bin/whim-guest-hello-arm64.elf bin/whim-guest || s=$?
		echo "exit $s (3 expected)"
	fi
	;;
run)
	need bin/whim-guest "mac.sh build"
	need "$img" "$how"
	WHIM_GUEST_IMAGE=$img exec bin/whim-guest "$@"
	;;
c)
	# The host asserts the core's PATH_MAX, 4096, equal to the header's; macOS's
	# is 1024. The host uses PATH_MAX nowhere else (the core's paths are its
	# own buffers), so the Mac's copy drops that one Linux guard.
	sed '/^static_assert(4096 == PATH_MAX, "PATH_MAX");$/d' src/whim-vim.c > .tmp/whim-vim-mac.c
	clang -std=gnu23 -O0 -w -include guest/mac/shim.h -o bin/whim-vim-mac .tmp/whim-vim-mac.c
	;;
suite)
	if [ "${1:-}" = --wide ]; then
		suite WHIM_SUITE_WIDE=1 WHIM_SUITE_SRC=src/whim-vim.c
	else
		suite
	fi
	;;
heavy)
	suite WHIM_SUITE_HEAVY=only
	;;
*)
	sed -n '11,18s/^# //p' "$0" >&2
	exit 2
	;;
esac
