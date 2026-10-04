// Package guest builds whim's core as a bare-metal guest (doc/GUEST.md):
// the core cut from a whim-vim.c as every translation cuts it, the C host's
// vim_snprintf (which needs no system), and the guest runtime (rt.c: the 15
// hypercalls, the allocator, the fault report; entry_<isa>.S: the entry and
// the exception vectors), compiled freestanding into one ELF image, which
// the monitor (vmm/) loads.  `go tool whim guest` is its command.
package guest

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/arbace/go-whim/internal/whim"
	"github.com/arbace/go-whim/vmm"
)

//go:embed rt
var sources embed.FS

// An Arch is an ISA the guest is built for, and how.
type Arch struct {
	Name   string // GOARCH
	Target string // clang's
	Flags  []string
	LD     string
	LDEmu  string
}

// Arches is the two the monitor runs.
var Arches = map[string]Arch{
	"amd64": {Name: "amd64", Target: "x86_64-none-elf", LD: "ld", LDEmu: "elf_x86_64",
		Flags: []string{"-mno-red-zone", "-mgeneral-regs-only", "-mcmodel=small"}},
	"arm64": {Name: "arm64", Target: "aarch64-none-elf", LD: "aarch64-none-elf-ld", LDEmu: "aarch64elf",
		Flags: []string{"-mgeneral-regs-only"}},
}

// Native is the Arch of this machine.
func Native() Arch { return Arches[runtime.GOARCH] }

// cflags is the compile line of the guest's one translation unit: the core
// freestanding, no SIMD (so there is no vector state to set up or save),
// no stack protector, nothing to unwind.  -O2: the C reference is -O0, and
// what the suite compares is behaviour.
var cflags = []string{"-std=c23", "-O2", "-ffreestanding", "-fno-builtin", "-nostdlib", "-fno-pic", "-fno-pie",
	"-fno-stack-protector", "-fno-asynchronous-unwind-tables", "-fno-unwind-tables", "-w"}

// Source is the guest's translation unit for the whim-vim.c in c: the core,
// the C host's formatting (everything between its #includes and its first
// object that is the system's), and rt.c.
func Source(c []byte) ([]byte, error) {
	core, err := whim.Cut(c)
	if err != nil {
		return nil, err
	}
	fmtPart, err := hostFormat(c)
	if err != nil {
		return nil, err
	}
	rt, err := sources.ReadFile("rt/rt.c")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(core)
	b.WriteString("\n#include <stdarg.h>\n#include <stddef.h>\n")
	b.Write(fmtPart)
	b.WriteString("\n")
	b.Write(rt)
	return b.Bytes(), nil
}

// hostEnd is the first line of the C host that is the system's: its signal
// flags.  What is between the #includes and it is vim_snprintf and its
// helpers, which call nothing but the core.
const hostEnd = "static volatile sig_atomic_t "

// hostFormat is the C host's formatting part, its static_asserts (about the
// system's headers) left out.
func hostFormat(c []byte) ([]byte, error) {
	lines := strings.SplitAfter(string(c), "\n")
	start, end := -1, -1
	for i, l := range lines {
		if start < 0 {
			if strings.HasPrefix(strings.TrimLeft(l, " "), "#") {
				start = i
			}
			continue
		}
		if strings.HasPrefix(l, hostEnd) {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("guest: no host part in the C, from its first #include to %q", hostEnd)
	}
	var b bytes.Buffer
	for _, l := range lines[start:end] {
		t := strings.TrimLeft(l, " ")
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "static_assert") {
			continue
		}
		b.WriteString(l)
	}
	return b.Bytes(), nil
}

// Hello is milestone 1's translation unit: rt.c with hello.c standing in
// for the core.
func Hello() ([]byte, error) {
	h, err := sources.ReadFile("rt/hello.c")
	if err != nil {
		return nil, err
	}
	rt, err := sources.ReadFile("rt/rt.c")
	if err != nil {
		return nil, err
	}
	return append(append(h, '\n'), rt...), nil
}

func command(dir, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

func run(cmd *exec.Cmd) error {
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	return nil
}

// Image compiles tu for a in dir and returns the linked ELF.
func Image(tu []byte, a Arch, dir string) ([]byte, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for _, n := range []string{"entry_" + a.Name + ".S", "link_" + a.Name + ".ld"} {
		b, err := sources.ReadFile("rt/" + n)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "guest.c"), tu, 0o644); err != nil {
		return nil, err
	}
	cc := append(append([]string{"--target=" + a.Target}, cflags...), a.Flags...)
	if err := run(command(dir, "clang", append(cc, "-c", "guest.c", "-o", "guest.o")...)); err != nil {
		return nil, err
	}
	if err := run(command(dir, "clang", append(cc, "-c", "entry_"+a.Name+".S", "-o", "entry.o")...)); err != nil {
		return nil, err
	}
	if err := run(command(dir, a.LD, "-m", a.LDEmu, "-static", "-nostdlib", "--no-dynamic-linker", "-T", "link_"+a.Name+".ld",
		"-o", "guest.elf", "entry.o", "guest.o")); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "guest.elf"))
}

// Monitor builds the launcher for a (cmd/whim-guest), in dir.
func Monitor(a Arch, dir string) (string, error) {
	out, err := filepath.Abs(filepath.Join(dir, "whim-guest-"+a.Name))
	if err != nil {
		return "", err
	}
	cmd := command("", "go", "build", "-o", out, "./vmm/cmd/whim-guest")
	cmd.Env = append(os.Environ(), "GOARCH="+a.Name, "CGO_ENABLED=0")
	if err := run(cmd); err != nil {
		return "", err
	}
	return out, nil
}

// Launcher writes out: the monitor with img appended, executable.
func Launcher(monitor string, img []byte, out string) error {
	prog, err := os.ReadFile(monitor)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, vmm.Append(prog, img), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}

// Build is the whole: the image of src (a whim-vim.c) for a, the monitor,
// and the launcher at out, using dir for the work.
func Build(src string, a Arch, dir, out string) error {
	c, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tu, err := Source(c)
	if err != nil {
		return err
	}
	img, err := Image(tu, a, filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	mon, err := Monitor(a, dir)
	if err != nil {
		return err
	}
	return Launcher(mon, img, out)
}

// BuildHello is milestone 1's launcher: the runtime and hello.c.
func BuildHello(a Arch, dir, out string) error {
	tu, err := Hello()
	if err != nil {
		return err
	}
	img, err := Image(tu, a, filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	mon, err := Monitor(a, dir)
	if err != nil {
		return err
	}
	return Launcher(mon, img, out)
}
