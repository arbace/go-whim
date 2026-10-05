package guest

import (
	"fmt"
	"os"
	"path/filepath"
)

// THE RUST GUEST (doc/GUEST.md, *The Rust guest*): whimsy's core --
// whimsy/src/editor.rs, generated, with whimsy's runtime and printf --
// built without std as a static library by rustc (guest/whimsy/lib.rs, the
// "guest" feature), and linked with the C guest's runtime (rt/rt.c compiled
// apart, -DWHIM_CORE_APART: its entry, vectors, hypercalls and arena), so
// that the same monitor runs it as it runs the C guest.  amd64 alone: the
// rustc here has core and alloc for its own target only, whose code is
// x86-64's baseline -- SSE, which rt.c turns on for a core compiled apart.

// RustCrate is the guest crate's root, from the repository's.
var RustCrate = filepath.Join("guest", "whimsy", "lib.rs")

// rustFlags build the core as a static library without std: aborting on a
// panic (the guest's end), linked where the image is (no PIC).
var rustFlags = []string{"--edition", "2021", "--crate-type", "staticlib", "--crate-name", "whimsy_guest",
	"-C", "panic=abort", "-C", "opt-level=2", "-C", "relocation-model=static", "-C", "debuginfo=0",
	"--cfg", `feature="guest"`}

// rustCrateFile is RustCrate from the working directory or the nearest one
// above it that has it.
func rustCrateFile() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, RustCrate)); err == nil {
			return filepath.Join(d, RustCrate), nil
		}
		up := filepath.Dir(d)
		if up == d {
			return "", fmt.Errorf("no %s here or above: run from the repository", RustCrate)
		}
		d = up
	}
}

// RustImage builds the Rust guest's image for a, into dir, and returns the
// ELF: the crate by rustc, rt.c apart and the entry by clang, linked by the
// C guest's script.
func RustImage(a Arch, dir string) ([]byte, error) {
	if a.Name != "amd64" {
		return nil, fmt.Errorf("the Rust guest is amd64's alone: rustc here has core and alloc for its own target only")
	}
	crate, err := rustCrateFile()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for _, n := range []string{"rt.c", "entry_" + a.Name + ".S", "link_" + a.Name + ".ld"} {
		b, err := sources.ReadFile("rt/" + n)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			return nil, err
		}
	}
	if err := run(command(dir, "rustc", append(append([]string(nil), rustFlags...), "-o", "libwhimsy_guest.a", crate)...)); err != nil {
		return nil, err
	}
	cc := append(append([]string{"--target=" + a.Target}, cflags...), a.Flags...)
	if err := run(command(dir, "clang", append(cc, "-DWHIM_CORE_APART", "-c", "rt.c", "-o", "rt.o")...)); err != nil {
		return nil, err
	}
	if err := run(command(dir, "clang", append(cc, "-c", "entry_"+a.Name+".S", "-o", "entry.o")...)); err != nil {
		return nil, err
	}
	if err := run(command(dir, a.LD, "-m", a.LDEmu, "-static", "-nostdlib", "--no-dynamic-linker", "-T", "link_"+a.Name+".ld",
		"-o", "guest.elf", "entry.o", "rt.o", "libwhimsy_guest.a")); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "guest.elf"))
}

// BuildRust is the whole Rust guest: the image, the monitor, and the
// launcher at out.
func BuildRust(a Arch, dir, out string) error {
	img, err := RustImage(a, filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	mon, err := Monitor(a, dir)
	if err != nil {
		return err
	}
	return Launcher(mon, img, out)
}

// BuildRustImage is the Rust guest's image alone, at out.
func BuildRustImage(a Arch, dir, out string) error {
	img, err := RustImage(a, dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, img, 0o644)
}
