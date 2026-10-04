#!/bin/sh
# aarch64.sh -- the arm64 gate's VM (doc/GUEST.md, *The aarch64 VM*): Alpine
# Edge aarch64, netbooted by qemu-system-aarch64 with an emulated EL2, so that
# KVM/arm64 runs inside it, and a job agent in its apkovl.  The host side is
# QEMU's own: user networking forwards the VM's connections to 10.0.2.100:80
# to this script's `serve`, one process a connection, so there is no server
# to run -- GET /F answers $VM/www/F, POST /out/N stores job N's output.
#
#   aarch64.sh fetch        the netboot files into $VM (dl-cdn.alpinelinux.org)
#   aarch64.sh start        boot the VM in the background; waits for its agent
#   aarch64.sh put F...     hand files to the next run: fetched into /h first
#   aarch64.sh run 'CMDS'   run CMDS by sh in the VM, in /h, the put files
#                           fetched there first; print the output, exit as it did
#   aarch64.sh stop         power the VM off
#
# $VM is the state directory (default .tmp/vm); SMP and MEM the VM's size
# (8, 6144); with $VM/serve.log present, serve logs each request in it.
# Nothing is installed on the host.
set -eu
VM=${VM:-.tmp/vm}
mkdir -p "$VM"
VM=$(cd "$VM" && pwd)
export VM
WWW=$VM/www
BASE=https://dl-cdn.alpinelinux.org/alpine/edge
self=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")

case ${1:-} in
fetch)
	for f in vmlinuz-virt initramfs-virt modloop-virt; do
		[ -s "$VM/$f" ] || curl -fsSL -o "$VM/$f" "$BASE/releases/aarch64/netboot/$f"
	done
	;;

serve) # one HTTP/1.0 request on stdin/stdout, from QEMU's guestfwd
	cr=$(printf '\r')
	read -r method path _
	len=0
	while read -r line; do
		line=${line%"$cr"}
		[ -z "$line" ] && break
		case $line in [Cc]ontent-[Ll]ength:*) len=${line#*: } ;; esac
	done
	path=${path%%\?*}
	[ ! -e "$VM/serve.log" ] || echo "$method $path $len" >>"$VM/serve.log"
	case $method$path in
	POST/out/*)
		n=${path#/out/}
		head -c "$len" >"$WWW/out/$n.tmp"
		mv "$WWW/out/$n.tmp" "$WWW/out/$n"
		printf 'HTTP/1.0 200 OK\r\nContent-Length: 0\r\n\r\n'
		;;
	GET/*)
		f=$WWW${path}
		case $path in *..*) f=/nonexistent ;; esac
		if [ -f "$f" ]; then
			printf 'HTTP/1.0 200 OK\r\nContent-Length: %d\r\n\r\n' "$(wc -c <"$f")"
			cat "$f"
		else
			printf 'HTTP/1.0 404 Not Found\r\nContent-Length: 0\r\n\r\n'
		fi
		;;
	esac
	;;

start)
	for f in vmlinuz-virt initramfs-virt modloop-virt; do
		[ -s "$VM/$f" ] || { echo "aarch64.sh: no $VM/$f: run fetch" >&2; exit 1; }
	done
	rm -rf "$WWW" && mkdir -p "$WWW/job" "$WWW/out" "$WWW/f"
	ln -s "$VM/modloop-virt" "$WWW/modloop-virt"
	# the apkovl: the default runlevels, and the agent as a local.d script
	o=$VM/ovl && rm -rf "$o"
	mkdir -p "$o/etc/local.d" "$o/etc/apk" "$o/etc/runlevels/sysinit" "$o/etc/runlevels/boot" \
		"$o/etc/runlevels/default" "$o/etc/runlevels/shutdown"
	for s in sysinit:devfs sysinit:dmesg sysinit:mdev sysinit:hwdrivers sysinit:modloop \
		boot:modules boot:sysctl boot:hostname boot:bootmisc \
		default:local shutdown:mount-ro shutdown:killprocs; do
		ln -s "/etc/init.d/${s#*:}" "$o/etc/runlevels/${s%%:*}/${s#*:}"
	done
	echo alpine-base >"$o/etc/apk/world"
	printf '%s/main\n%s/community\n' "$BASE" "$BASE" >"$o/etc/apk/repositories"
	echo whimvm >"$o/etc/hostname"
	cat >"$o/etc/local.d/whim.start" <<-'EOF'
	#!/bin/sh
	# the job agent: fetch job N, run it, post its output, N+1
	echo nameserver 10.0.2.3 >/etc/resolv.conf
	mkdir -p /h
	( n=1
	  while :; do
		if wget -q -O /tmp/job.sh http://10.0.2.100/job/$n.sh 2>/dev/null; then
			(cd /h && sh /tmp/job.sh) >/tmp/job.out 2>&1
			echo "exit $?" >>/tmp/job.out
			wget -q -O /dev/null --post-file=/tmp/job.out http://10.0.2.100/out/$n
			n=$((n + 1))
		else
			sleep 1
		fi
	  done ) </dev/null >/dev/null 2>&1 &
	EOF
	chmod +x "$o/etc/local.d/whim.start"
	tar -C "$o" -czf "$WWW/whim.apkovl.tar.gz" etc
	echo 0 >"$VM/next"
	setsid qemu-system-aarch64 -machine virt,virtualization=on,gic-version=3 -cpu max \
		-accel tcg,thread=multi -smp "${SMP:-8}" -m "${MEM:-6144}" -nographic \
		-kernel "$VM/vmlinuz-virt" -initrd "$VM/initramfs-virt" \
		-append "console=ttyAMA0 ip=dhcp alpine_repo=$BASE/main modloop=http://10.0.2.100/modloop-virt apkovl=http://10.0.2.100/whim.apkovl.tar.gz" \
		-netdev "user,id=n,guestfwd=tcp:10.0.2.100:80-cmd:sh $self serve" \
		-device virtio-net-pci,netdev=n \
		-pidfile "$VM/qemu.pid" >"$VM/console.log" 2>&1 </dev/null &
	echo "aarch64.sh: booting (console: $VM/console.log)"
	VM=$VM "$self" run 'uname -a; ls -l /dev/kvm'
	;;

put)
	shift
	d=$WWW/f/$(($(cat "$VM/next") + 1))
	mkdir -p "$d"
	for f; do
		cp "$f" "$d/$(basename "$f")"
	done
	;;

run)
	n=$(($(cat "$VM/next") + 1))
	{
		for f in "$WWW/f/$n"/*; do
			[ -e "$f" ] || continue
			b=$(basename "$f")
			echo "wget -q -O '$b' 'http://10.0.2.100/f/$n/$b' && chmod +x '$b'"
		done
		printf '%s\n' "$2"
	} >"$WWW/job/$n.sh.tmp"
	mv "$WWW/job/$n.sh.tmp" "$WWW/job/$n.sh"
	echo "$n" >"$VM/next"
	while [ ! -f "$WWW/out/$n" ]; do
		if ! kill -0 "$(cat "$VM/qemu.pid" 2>/dev/null)" 2>/dev/null; then
			echo "aarch64.sh: the VM is not running (console: $VM/console.log)" >&2
			exit 1
		fi
		sleep 2
	done
	rm -rf "$WWW/f/$n"
	sed '$d' "$WWW/out/$n"
	code=$(tail -n 1 "$WWW/out/$n")
	exit "${code#exit }"
	;;

stop)
	if [ -f "$VM/qemu.pid" ]; then
		VM=$VM timeout 60 "$self" run 'poweroff' >/dev/null 2>&1 || true
		p=$(cat "$VM/qemu.pid")
		for _ in 1 2 3 4 5 6 7 8 9 10; do kill -0 "$p" 2>/dev/null || break; sleep 2; done
		kill "$p" 2>/dev/null || true
		rm -f "$VM/qemu.pid"
	fi
	;;

*)
	sed -n '2,/^set -eu/p' "$0" | sed '$d' >&2
	exit 2
	;;
esac
