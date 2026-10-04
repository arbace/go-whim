package guest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// THE GO GUEST (doc/GUEST.md, *A second guest*): the Go editor, editor/ as
// it stands, built with TamaGo -- GOOS=tamago, a Go distribution of its own
// -- on the board in guest/tamago/board, a module of its own (guest/tamago)
// so that this one stays standard Go; appended to the same monitor, which
// tells its image from the C guest's.

// TamaGoEnv names the TamaGo distribution: its bin/go builds the image.
const TamaGoEnv = "TAMAGO_ROOT"

// GoModule is the Go guest's module, from the repository's root.
var GoModule = filepath.Join("guest", "tamago")

// goModuleDir is GoModule from the working directory or the nearest one
// above it that has it (a test runs in its package's directory).
func goModuleDir() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, GoModule, "go.mod")); err == nil {
			return filepath.Join(d, GoModule), nil
		}
		up := filepath.Dir(d)
		if up == d {
			return "", fmt.Errorf("no %s here or above: run from the repository", GoModule)
		}
		d = up
	}
}

// goLink is where the image is linked: its first segment (the ELF header's
// page) at 2 MiB, the monitor's image base, text a page above.
const goLink = "-T 0x201000 -R 0x1000"

// TamaGo is the go command of the TamaGo distribution at root (default
// $TAMAGO_ROOT), or an error that says how to name one.
func TamaGo(root string) (string, error) {
	if root == "" {
		root = os.Getenv(TamaGoEnv)
	}
	if root == "" {
		return "", errors.New("the Go guest needs TamaGo (github.com/usbarmory/tamago-go, the release matching go.mod's Go): " +
			TamaGoEnv + "=/path/to/tamago-go, or --tamago DIR (doc/GUEST.md, *A second guest*)")
	}
	gocmd := filepath.Join(root, "bin", "go")
	if _, err := os.Stat(filepath.Join(root, "src", "runtime", "os_tamago.go")); err != nil {
		return "", fmt.Errorf("%s=%s is not a TamaGo distribution: no src/runtime/os_tamago.go", TamaGoEnv, root)
	}
	if _, err := os.Stat(gocmd); err != nil {
		return "", fmt.Errorf("%s=%s has no bin/go: build it (src/make.bash)", TamaGoEnv, root)
	}
	return gocmd, nil
}

// GoImage builds the Go guest's image for a with the TamaGo go command
// gocmd, into dir, and returns the ELF.
func GoImage(gocmd string, a Arch, dir string) ([]byte, error) {
	return GoProgram(gocmd, ".", a, dir)
}

// GoProgram builds the package pkg of the Go guest's module (".", the
// editor; "./faulty", a stand-in) as an image for a, into dir.  The arm64
// image builds, but has not been booted (doc/GUEST.md, *A second guest*).
func GoProgram(gocmd, pkg string, a Arch, dir string) ([]byte, error) {
	mod, err := goModuleDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out, err := filepath.Abs(filepath.Join(dir, "guest-go.elf"))
	if err != nil {
		return nil, err
	}
	cmd := command(mod, gocmd, "build", "-trimpath", "-ldflags", goLink, "-o", out, pkg)
	cmd.Env = append(os.Environ(), "GOOS=tamago", "GOARCH="+a.Name, "GOOSPKG=github.com/usbarmory/tamago",
		"GOROOT="+filepath.Dir(filepath.Dir(gocmd)), "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOFLAGS=")
	if err := run(cmd); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

// BuildGo is the whole Go guest: the image, the monitor, and the launcher
// at out, with the TamaGo distribution at root (default $TAMAGO_ROOT).
func BuildGo(root string, a Arch, dir, out string) error {
	gocmd, err := TamaGo(root)
	if err != nil {
		return err
	}
	img, err := GoImage(gocmd, a, filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	mon, err := Monitor(a, dir)
	if err != nil {
		return err
	}
	return Launcher(mon, img, out)
}
