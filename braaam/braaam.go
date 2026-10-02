// Package braaam builds the editor in Java: the core, the package
// whim.editor (Editor.java and a file for each of its classes, tracked in
// editor/), written by crefactor/togo's Java backend from the core half of a
// whim-vim.c, and
// beside it the Java sources kept here by hand -- the runtime (rt/, package
// whim.rt), the host (host/, package whim.host: the Host interface, the
// terminal host, vim_snprintf) and the glue and launcher (Whim.java) --
// compiled with javac into a directory of classes, and a launcher script that
// runs it as a binary is run: `braaam [args]`.  doc/JAVA.md is the design.
//
// The Java sources are embedded, so a build needs no checkout beside it; the
// generator is handed in (Gen), since it is cmd/whim's profile that says
// what the core's names are.
package braaam

import (
	"embed"
	"fmt"
	"github.com/arbace/go-whim/internal/whim"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed rt/*.java host/*.java Whim.java
var sources embed.FS

// Gen writes the Java class of the C core editorC to javaOut, as `whim skel
// <editorC> <dir> -java <javaOut>` does: with the profile's JavaFiles, the
// package's files beside it.
type Gen func(editorC, dir, javaOut string) error

// MainClass is the launcher's class: the glue, in the generated package.
const MainClass = "whim.editor.Whim"

// Generated are the Java files the backend wrote in dir, sorted.
func Generated(dir string) ([]string, error) {
	fs, err := filepath.Glob(filepath.Join(dir, "*.java"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fs {
		out = append(out, filepath.Base(f))
	}
	return out, nil
}

// Build is the editor in Java from the C file src, in dir: dir/src/ the
// sources (the generated package in dir/src/editor/), dir/classes/ what
// javac made of them, and the launcher at the path launcher, which it
// returns.  edit, when not nil, is applied to the generated Editor.java
// before it is compiled -- the suite's control.
func Build(gen Gen, src, dir, launcher string, edit func([]byte) ([]byte, error)) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := whim.Cut(c)
	if err != nil {
		return "", err
	}
	srcDir := filepath.Join(dir, "src")
	// written afresh: a file of a build before (the one-file Editor.java)
	// would otherwise stay beside what this one writes
	if err := os.RemoveAll(srcDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	genDir := filepath.Join(srcDir, "editor")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return "", err
	}
	javaOut := filepath.Join(genDir, "Editor.java")
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
	return Compile(genDir, dir, launcher)
}

// Compile compiles the generated package in genDir with the embedded
// sources into dir/classes and writes the launcher at the path launcher,
// which it returns.
func Compile(genDir, dir, launcher string) (string, error) {
	srcDir := filepath.Join(dir, "src")
	files, err := WriteSources(srcDir)
	if err != nil {
		return "", err
	}
	gen, err := Generated(genDir)
	if err != nil {
		return "", err
	}
	for _, g := range gen {
		abs, err := filepath.Abs(filepath.Join(genDir, g))
		if err != nil {
			return "", err
		}
		files = append(files, abs)
	}
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
// another editor on the JVM (vijure/, the editor in Clojure) is written
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
//   - -XX:TieredStopAtLevel=1: a short-lived program starts faster with
//     it (measured in doc/JAVA.md).
//   - -XX:+UseParallelGC: the regex engine runs on every core in a :%s
//     (match_lines, phase 96), each thread allocating; the serial collector
//     stopped them all while one thread collected -- 3.1 s of a 100,000-line
//     :%s/\v(a|b)+c/X/g, against 0.98 s with this -- and it starts as fast
//     (0.28 s against 0.27 on a short session).
//   - -XX:-DontCompileHugeMethods: HotSpot never compiles a method of more
//     than 8,000 bytes of bytecode by default, and the generated editors
//     have hot ones that big (the Java editor regmatch, the Clojure editor 60
//     of its editing functions); interpreted forever, a heavy session took
//     the Clojure editor 21 s where it takes 3.7 s with the flag, and the
//     Java 1.5 s where it takes 1.0 (doc/CLOJURE-IDIOMS.md, item 0; whim
//     test's heavy case, which reports the time).
//   - -XX:Tier3BackEdgeThreshold=6000: with C1 alone, a loop running in
//     the interpreter is compiled after 60,000 turns by default, and the
//     editors' big loops -- ex_substitute over a buffer's lines, match_chunk
//     over a chunk's -- are called a handful of times and turn fewer: they
//     ran interpreted, the Clojure editor's ex_substitute at 115 times the
//     C's time (doc/CLOJURE-PROFILE.md). With it, and the Clojure's state
//     machines split at 50,000, the heavy case 1.93 -> 1.69 s on the
//     Clojure editor, as before on the Java, a short session as before.
//   - -Xss is not needed: the core runs on a thread of its own with a stack
//     of 1 GiB (Whim.main).
var JVMFlags = []string{
	"--enable-native-access=ALL-UNNAMED",
	"-XX:-UsePerfData",
	"-XX:+UseParallelGC",
	"-XX:TieredStopAtLevel=1",
	"-XX:-DontCompileHugeMethods",
	"-XX:Tier3BackEdgeThreshold=6000",
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
	b.WriteString("#!/bin/sh\n# The editor in Java (braaam/): written by braaam.WriteLauncher.\n")
	fmt.Fprintf(&b, "exec %s %s -cp %s -Dwhim.argv0=\"$0\" %s \"$@\"\n",
		ShellQuote(java), strings.Join(JVMFlags, " "), ShellQuote(abs), MainClass)
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
