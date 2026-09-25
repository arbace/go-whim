// Package jeditor builds the editor in Java: the core, Editor.java, written
// by crefactor/togo's Java backend from the core half of a whim-vim.c, and
// beside it the Java sources kept here by hand -- the runtime (rt/, package
// whim.rt), the host (host/, package whim.host: the Host interface, the
// terminal host, vim_snprintf) and the glue and launcher (Whim.java) --
// compiled with javac into a directory of classes, and a launcher script that
// runs it as a binary is run: `whim-java [args]`.  doc/JAVA.md is the design.
//
// The Java sources are embedded, so a build needs no checkout beside it; the
// generator is handed in (Gen), since it is cmd/whim's profile that says
// what the core's names are.
package jeditor

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed rt/*.java host/*.java Whim.java
var sources embed.FS

// Gen writes the Java class of the C core editorC to javaOut, as `whim skel
// <editorC> <dir> -java <javaOut>` does.
type Gen func(editorC, dir, javaOut string) error

// includeLine is where the core ends: the first #include, whitespace after
// the # being insignificant to C.
var includeLine = regexp.MustCompile(`^ *# *include `)

// Cut is the core half of a whim-vim.c, as `make editor.c` cuts it: every
// line before the first #include, trailing blank lines dropped; and an error
// when what is left holds a directive, since the core has none.
func Cut(c []byte) ([]byte, error) {
	lines := strings.SplitAfter(string(c), "\n")
	last := -1
	for i, l := range lines {
		if includeLine.MatchString(l) {
			break
		}
		if strings.TrimSpace(l) != "" {
			last = i
		}
		if strings.HasPrefix(strings.TrimLeft(l, " "), "#") {
			return nil, fmt.Errorf("jeditor: the cut holds a directive at line %d, so it found the wrong line", i+1)
		}
	}
	if last < 0 {
		return nil, fmt.Errorf("jeditor: no core before the first #include")
	}
	var b bytes.Buffer
	for _, l := range lines[:last+1] {
		b.WriteString(strings.TrimSuffix(l, "\n"))
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

// Build is the editor in Java from the C file src, in dir: dir/src/ the
// sources, dir/classes/ what javac made of them, and the launcher
// at the path launcher, which it returns.  edit, when not nil, is applied to the
// generated Editor.java before it is compiled -- the suite's control.
func Build(gen Gen, src, dir, launcher string, edit func([]byte) ([]byte, error)) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := Cut(c)
	if err != nil {
		return "", err
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	javaOut := filepath.Join(srcDir, "Editor.java")
	// The generator's by-products (the skeleton's .go files, the facts) go in
	// a directory of their own, removed after: under the module they would
	// be a package `go build ./...` tries.
	scratch, err := os.MkdirTemp("", "jgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	if err := gen(editorC, scratch, javaOut); err != nil {
		return "", err
	}
	if r, err := os.ReadFile(javaOut + ".refused"); err == nil {
		os.WriteFile(filepath.Join(dir, "Editor.java.refused"), r, 0o644)
		os.Remove(javaOut + ".refused")
	}
	if edit != nil {
		b, err := os.ReadFile(javaOut)
		if err != nil {
			return "", err
		}
		if b, err = edit(b); err != nil {
			return "", err
		}
		if err := os.WriteFile(javaOut, b, 0o644); err != nil {
			return "", err
		}
	}
	return Compile(javaOut, dir, launcher)
}

// Compile compiles editorJava with the embedded sources into dir/classes and
// writes the launcher at the path launcher, which it returns.
func Compile(editorJava, dir, launcher string) (string, error) {
	srcDir := filepath.Join(dir, "src")
	files, err := WriteSources(srcDir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(editorJava)
	if err != nil {
		return "", err
	}
	files = append(files, abs)
	classes := filepath.Join(dir, "classes")
	if err := os.RemoveAll(classes); err != nil {
		return "", err
	}
	// -nowarn: the generated core's lossy compound assignments and
	// fall-throughs are C's, meant (doc/JAVA.md).  What javac prints still --
	// its mandatory warnings that sun.misc.Signal is internal API, which
	// host/Signals.java explains -- is shown only when it fails.
	args := append([]string{"-nowarn", "-encoding", "UTF-8", "-d", classes}, files...)
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("javac: %v\n%s", err, out)
	}
	return WriteLauncher(launcher, classes)
}

// WriteSources writes the embedded Java sources under dir and returns their
// paths.
func WriteSources(dir string) ([]string, error) {
	return writeSources(dir, func(string) bool { return true })
}

// WriteRuntime writes the runtime (rt/, package whim.rt) and the host (host/,
// package whim.host) under dir and returns their paths: the Java sources
// without the glue, Whim.java, which needs a generated Editor.java -- what
// another editor on the JVM (cljeditor/, the editor in Clojure) is written
// against.
func WriteRuntime(dir string) ([]string, error) {
	return writeSources(dir, func(p string) bool {
		return strings.HasPrefix(p, "rt/") || strings.HasPrefix(p, "host/")
	})
}

func writeSources(dir string, want func(string) bool) ([]string, error) {
	var out []string
	err := fs.WalkDir(sources, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !want(p) {
			return err
		}
		b, err := sources.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
		abs, err := filepath.Abs(dst)
		out = append(out, abs)
		return err
	})
	return out, err
}

// JVMFlags are the launcher's options to the JVM, each for a reason:
//
//   - --enable-native-access=ALL-UNNAMED: the terminal host calls the C
//     library through java.lang.foreign, and without it the JVM prints a
//     warning on the editor's screen.
//   - -XX:-UsePerfData: no hsperfdata file under /tmp for every run.
//   - -XX:+UseSerialGC and -XX:TieredStopAtLevel=1: a short-lived,
//     single-threaded program starts faster with them (measured in
//     doc/JAVA.md).
//   - -Xss is not needed: the core runs on a thread of its own with a stack
//     of 1 GiB (Whim.main).
var JVMFlags = []string{
	"--enable-native-access=ALL-UNNAMED",
	"-XX:-UsePerfData",
	"-XX:+UseSerialGC",
	"-XX:TieredStopAtLevel=1",
}

// WriteLauncher writes path, a shell script that runs the editor in classes
// as a binary is run: its arguments the editor's, $0 its argv[0], and exec,
// so that the JVM is the process the caller started -- its pid, its process
// group, its signals.
func WriteLauncher(path, classes string) (string, error) {
	java, err := exec.LookPath("java")
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(classes)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# The editor in Java (jeditor/): written by jeditor.WriteLauncher.\n")
	fmt.Fprintf(&b, "exec %s %s -cp %s -Dwhim.argv0=\"$0\" Whim \"$@\"\n",
		ShellQuote(java), strings.Join(JVMFlags, " "), ShellQuote(abs))
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// ShellQuote is s as one word of a shell command.
func ShellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+=:") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
