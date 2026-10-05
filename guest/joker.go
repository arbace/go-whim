package guest

import (
	"os"
	"path/filepath"
)

// THE JOKER GUEST (doc/LISP-SANDBOX.md): Joker, a Clojure dialect in Go --
// guest/joker/joker, a fork carrying its core and the standard namespaces
// that need no system -- built with TamaGo on the Go guest's board and run
// by the same monitor: a REPL on the console, the language in the box, with
// the graph views in Joker (guest/joker/gview) loaded and a graph's EDN
// embedded as store/graph.

// JokerModule is the Joker guest's module, from the repository's root.
var JokerModule = filepath.Join("guest", "joker")

// JokerStore is where the image's graph is written, in the module: the EDN
// the guest embeds (//go:embed), not tracked.
var JokerStore = filepath.Join("store", "graph.edn")

// JokerGenerate generates the fork's core (go generate, by this machine's
// go: Joker compiles its core library into Go data) and returns the
// module's directory.
func JokerGenerate() (string, error) {
	mod, err := moduleDir(JokerModule)
	if err != nil {
		return "", err
	}
	gen := command(filepath.Join(mod, "joker"), "go", "generate", "./...")
	gen.Env = append(os.Environ(), "GOFLAGS=")
	return mod, run(gen)
}

// JokerHost builds the Joker guest's language on the host, at out:
// guest/joker/cmd/jokerhost, the fork's core and the same namespaces and
// views, by this machine's go -- no TamaGo, no board.
func JokerHost(out string) error {
	mod, err := JokerGenerate()
	if err != nil {
		return err
	}
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
// edn written where the guest embeds it as store/graph, then the guest
// built.
func JokerImage(gocmd string, a Arch, dir string, edn []byte) ([]byte, error) {
	mod, err := JokerGenerate()
	if err != nil {
		return nil, err
	}
	store := filepath.Join(mod, JokerStore)
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(store, edn, 0o644); err != nil {
		return nil, err
	}
	return tamagoBuild(gocmd, mod, ".", a, dir, "guest-joker.elf")
}

// BuildJoker is the whole Joker guest: the image, the monitor, and the
// launcher at out, with the TamaGo distribution at root and edn its graph.
func BuildJoker(root string, a Arch, dir, out string, edn []byte) error {
	gocmd, err := TamaGo(root)
	if err != nil {
		return err
	}
	img, err := JokerImage(gocmd, a, filepath.Join(dir, "image"), edn)
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
func BuildJokerImage(root string, a Arch, dir, out string, edn []byte) error {
	gocmd, err := TamaGo(root)
	if err != nil {
		return err
	}
	img, err := JokerImage(gocmd, a, dir, edn)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, img, 0o644)
}
