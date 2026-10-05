#!/bin/sh
# The guest editor on the Mac, Hypervisor.framework on Apple silicon
# (doc/GUEST.md, *Running on the Mac*): run from the repository's root,
# on the Mac, with Go and Xcode's command-line tools.  Every Go build is
# native and CGO_ENABLED=0 (purego, no cgo); every program that creates a VM
# is signed ad hoc with the hypervisor entitlement.  The guest images are
# built on Linux (make mac-images; Apple's clang has no ELF linker) and
# copied to lib/whim-guest/ under the same names; the monitor is told which
# by WHIM_GUEST_IMAGE.  --go takes the Go guest's image,
# lib/whim-guest/whim-guest-go-arm64.elf, for the C guest's, and --rs the Rust
# guest's, lib/whim-guest/whim-guest-rs-arm64.elf (built in an arm64 Linux:
# doc/GUEST.md, *The Rust guest on arm64*); --cpus N gives
# the Go or the Rust guest N vCPUs (WHIM_GUEST_CPUS: 1 to 32, or host; 4 by
# default, as measured -- doc/GUEST.md, *Running on the Mac*), the C guest
# always one.
# The options come after the step's name, in any order.
#
#   guest/mac/mac.sh check                 what the Mac is: macOS, the chip, kern.hv_support, the tools
#   guest/mac/mac.sh hv                    hv's arm64 tests on the framework: the smallest guests
#   guest/mac/mac.sh build                 bin/whim-guest, signed
#   guest/mac/mac.sh hello [--go|--rs]     the hello image: prints hello, exits 3 (--go, --rs: the Go or the Rust guest types hello, exits 0)
#   guest/mac/mac.sh run [--go|--rs] [--cpus N] [ARG...]  the editor: bin/whim-guest on lib/whim-guest/whim-guest-arm64.elf (--go, --rs: the Go or the Rust guest's)
#   guest/mac/mac.sh c                     the C reference, bin/whim-vim-mac, by clang with guest/mac/shim.h
#   guest/mac/mac.sh suite [--go|--rs] [--cpus N] [--wide]  the suite: the guest held to the C, with its control
#   guest/mac/mac.sh heavy [--go|--rs] [--cpus N]  the heavy case: the C, then the guest, answers compared, times reported
#   guest/mac/mac.sh scale [--rs] [LIST]   the Go (--rs: the Rust) guest's :%s over 200,000 lines on 1,2,4,8 vCPUs (or LIST) against the C: a table
set -eu
cd "$(dirname "$0")/../.."
export TMPDIR="$PWD/.tmp" CGO_ENABLED=0
mkdir -p "$TMPDIR" bin
ent=guest/mac/whim-guest.entitlements
sign() { codesign --sign - --entitlements "$ent" --force "$1"; }
need() { [ -f "$1" ] || { echo "mac.sh: no $1: $2" >&2; exit 1; }; }

cmd=${1:-}
[ $# -gt 0 ] && shift
L=lib/whim-guest
img=$L/whim-guest-arm64.elf how="make mac-images, on Linux, copied to $L/"
wide= rs=
while [ $# -gt 0 ]; do
	case $1 in
	--go) img=$L/whim-guest-go-arm64.elf ;;
	--rs)
		img=$L/whim-guest-rs-arm64.elf rs=1
		how="go tool whim guest --rust --image -o $img, in an arm64 Linux (doc/GUEST.md), copied here"
		;;
	--wide) wide=1 ;;
	--cpus)
		[ $# -ge 2 ] || { echo "mac.sh: --cpus N" >&2; exit 2; }
		export WHIM_GUEST_CPUS="$2"
		shift
		;;
	*) break ;;
	esac
	shift
done
# suite TEST VAR=VAL...: TEST, the guest held to bin/whim-vim-mac (TestGuestPrebuilt,
# TestGuestScale), the image beside the monitor named by
# WHIM_SUITE_GUEST_IMAGE; the rest the test's environment.
suite() {
	t=$1
	shift
	need bin/whim-guest "mac.sh build"
	need "$img" "$how"
	need bin/whim-vim-mac "mac.sh c"
	go test -c -o bin/suite.test ./internal/suite
	echo "mac.sh: $t, $img, WHIM_GUEST_CPUS=${WHIM_GUEST_CPUS:-default}" >&2
	env "$@" WHIM_SUITE_C=bin/whim-vim-mac WHIM_SUITE_GUEST=bin/whim-guest WHIM_SUITE_GUEST_IMAGE="$img" \
		bin/suite.test -test.run "$t" -test.v -test.timeout 3h
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
	ls $L/*.elf >/dev/null 2>&1 || echo "mac.sh: now copy the arm64 images (make mac-images, on Linux) to $L/" >&2
	;;
hello)
	need bin/whim-guest "mac.sh build"
	s=0
	if [ "$img" = $L/whim-guest-go-arm64.elf ] || [ -n "$rs" ]; then
		# The Go and Rust guests have no stand-in: the editor itself, its keys from a
		# file -- hello typed, :q! -- its screen printed.
		need "$img" "$how"
		printf 'ihello\033:q!\r' > "$TMPDIR/hello-go.keys"
		WHIM_GUEST_IMAGE=$img bin/whim-guest < "$TMPDIR/hello-go.keys" || s=$?
		echo
		echo "exit $s (0 expected, hello on the screen above)"
	else
		need $L/whim-guest-hello-arm64.elf "$how"
		WHIM_GUEST_IMAGE=$L/whim-guest-hello-arm64.elf bin/whim-guest || s=$?
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
	if [ -n "$wide" ]; then
		suite TestGuestPrebuilt WHIM_SUITE_WIDE=1 WHIM_SUITE_SRC=src/whim-vim.c
	else
		suite TestGuestPrebuilt
	fi
	;;
heavy)
	suite TestGuestPrebuilt WHIM_SUITE_HEAVY=only
	;;
scale)
	# The Go guest's parallel :%s/\v(a|b)+c/X/g over 200,000 lines
	# (WHIM_SUITE_SCALE_LINES), the median of 5 runs (WHIM_SUITE_SCALE_RUNS)
	# with it and 5 without, on each count of vCPUs, beside the C's;
	# every answer held to the C's.
	if [ -n "$rs" ]; then
		suite TestGuestScale WHIM_SUITE_SCALE="${1:-1,2,4,8}" WHIM_SUITE_GUEST_NAME="Rust guest"
	else
		img=$L/whim-guest-go-arm64.elf
		suite TestGuestScale WHIM_SUITE_SCALE="${1:-1,2,4,8}"
	fi
	;;
*)
	sed -n '/^#   guest/s/^# //p' "$0" >&2
	exit 2
	;;
esac
