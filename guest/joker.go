package guest

import (
	"os"
	"path/filepath"
)

// THE JOKER GUEST (doc/LISP-SANDBOX.md): Joker, a Clojure dialect in Go --
// guest/joker/joker, a fork carrying its core and the standard namespaces
// that need no system -- built with TamaGo on the Go guest's board and run
// by the same monitor: a REPL on the console, the language in the box.

// JokerModule is the Joker guest's module, from the repository's root.
var JokerModule = filepath.Join("guest", "joker")

// JokerImage builds the Joker guest's image for a with TamaGo's go command
// gocmd, into dir, and returns the ELF: the fork's core generated first
// (go generate, by this machine's go: Joker compiles its core library into
// Go data), then the guest built.
func JokerImage(gocmd string, a Arch, dir string) ([]byte, error) {
	mod, err := moduleDir(JokerModule)
	if err != nil {
		return nil, err
	}
	gen := command(filepath.Join(mod, "joker"), "go", "generate", "./...")
	gen.Env = append(os.Environ(), "GOFLAGS=")
	if err := run(gen); err != nil {
		return nil, err
	}
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
