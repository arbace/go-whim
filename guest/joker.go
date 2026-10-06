package guest

import (
	"os"
	"path/filepath"
	"syscall"
)

// THE JOKER GUEST (doc/LISP-SANDBOX.md): Joker, a Clojure dialect in Go --
// guest/joker/joker, a fork carrying its core and the standard namespaces
// that need no system -- built with TamaGo on the Go guest's board and run
// by the same monitor: a REPL on the console, the language in the box, with
// the graph views in Joker (guest/joker/gview) loaded and the monitor's
// store as the namespace box (guest/joker/box): the image holds no data.

// JokerModule is the Joker guest's module, from the repository's root.
var JokerModule = filepath.Join("guest", "joker")

// JokerGenerate generates the fork's core (go generate, by this machine's
// go: Joker compiles its core library into Go data) and returns the
// module's directory.
//
// It writes into the source tree, so it holds a lock on the module
// (joker/.generate.lock) until the caller's unlock: two builds at once --
// tests run side by side -- had one compile a file the other was writing.
func JokerGenerate() (mod string, unlock func(), err error) {
	if mod, err = moduleDir(JokerModule); err != nil {
		return "", nil, err
	}
	f, err := os.OpenFile(filepath.Join(mod, "joker", ".generate.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return "", nil, err
	}
	unlock = func() { f.Close() } // closing it releases the lock
	gen := command(filepath.Join(mod, "joker"), "go", "generate", "./...")
	gen.Env = append(os.Environ(), "GOFLAGS=")
	if err := run(gen); err != nil {
		unlock()
		return "", nil, err
	}
	return mod, unlock, nil
}

// JokerHost builds the Joker guest's language on the host, at out:
// guest/joker/cmd/jokerhost, the fork's core and the same namespaces and
// views, by this machine's go -- no TamaGo, no board.
func JokerHost(out string) error {
	mod, unlock, err := JokerGenerate()
	if err != nil {
		return err
	}
	defer unlock()
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cmd := command(mod, "go", "build", "-o", abs, "./cmd/jokerhost")
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	return run(cmd)
}

// JokerImage builds the Joker guest's image for a with TamaGo's go command
// gocmd, into dir, and returns the ELF: the fork's core generated first,
// then the guest built.
func JokerImage(gocmd string, a Arch, dir string) ([]byte, error) {
	mod, unlock, err := JokerGenerate()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return tamagoBuild(gocmd, mod, ".", a, dir, "guest-joker.elf")
}

// BuildJoker is the whole Joker guest: the image, the monitor, and the
// launcher at out, with the TamaGo distribution at root.
func BuildJoker(root string, a Arch, dir, out string) error {
	gocmd, err := TamaGo(root)
	if err != nil {
		return err
	}
	img, err := JokerImage(gocmd, a, filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	mon, err := Monitor(a, dir)
	if err != nil {
		return err
	}
	return Launcher(mon, img, out)
}

// BuildJokerImage is the Joker guest's image alone, at out.
func BuildJokerImage(root string, a Arch, dir, out string) error {
	gocmd, err := TamaGo(root)
	if err != nil {
		return err
	}
	img, err := JokerImage(gocmd, a, dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, img, 0o644)
}
