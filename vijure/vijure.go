// Package vijure builds the editor in Clojure: the core, the namespace
// whim.editor (src/whim/editor.clj), written by crefactor/togo's Clojure
// backend from the core half of a whim-vim.c, and beside it the Clojure kept
// here by hand -- the glue (src/whim/cljhost.clj: the C's host functions) and
// the launcher (src/whim/cljmain.clj) -- on the Java editor's runtime and
// host (braaam/rt, braaam/host, compiled with javac), AOT-compiled with
// Clojure's own jars into a directory of classes, and a launcher script that
// runs it as a binary is run: `vijure [args]`.  doc/CLOJURE.md is the
// design, and its contract says what the generated namespace provides.
//
// The Clojure sources are embedded, as braaam's Java are; Clojure itself is
// what the `clojure` command's classpath names (its jars, from ~/.m2), copied
// beside the classes so that the build runs without it.
package vijure

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/braaam"
)

//go:embed src/whim/cljhost.clj src/whim/cljmain.clj
var sources embed.FS

// Gen writes the namespace whim.editor of the C core editorC to cljOut, as
// `whim skel <editorC> <dir> -clj <cljOut>` does.
type Gen func(editorC, dir, cljOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing: the
// toolset it was handed has no Clojure backend (`whim skel ... -clj`).
var ErrNoBackend = errors.New("vijure: the generator wrote no whim/editor.clj -- this toolset has no Clojure backend yet (`whim skel <editor.c> <dir> -clj <out.clj>`, doc/CLOJURE.md milestones 1-2); hand a generated editor.clj to `whim clj --editor FILE` or `whim test --clojure-editor FILE` instead")

// Main is the launcher's class, whim.cljmain's :gen-class.
const Main = "whim.cljmain"

// command is exec.Command whose process dies with ours: a build or a run
// started by a test must not outlive it (Pdeathsig), whatever kills the test.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// Build is the editor in Clojure from the C file src, in dir: dir/editor.c
// the core, dir/editor.clj the namespace generated from it, dir/src/ the
// sources, dir/classes/ what javac and Clojure's compiler made of them,
// dir/lib/ Clojure's jars, and the launcher at the path launcher, which it
// returns.  edit, when not nil, is applied to the generated editor.clj
// before it is compiled.
func Build(gen Gen, src, dir, launcher string, edit func([]byte) ([]byte, error)) (string, error) {
	cljOut, err := Generate(gen, src, dir)
	if err != nil {
		return "", err
	}
	if edit != nil {
		b, err := os.ReadFile(cljOut)
		if err != nil {
			return "", err
		}
		if b, err = edit(b); err != nil {
			return "", err
		}
		if err := os.WriteFile(cljOut, b, 0o644); err != nil {
			return "", err
		}
	}
	return Compile(cljOut, dir, launcher)
}

// Generate cuts the core from the C file src into dir/editor.c and writes
// the namespace whim.editor of it at dir/editor.clj, whose path it returns;
// ErrNoBackend when gen writes nothing.
func Generate(gen Gen, src, dir string) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := braaam.Cut(c)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	cljOut := filepath.Join(dir, "editor.clj")
	os.Remove(cljOut)
	// The generator's by-products go in a directory of their own, removed
	// after, as braaam's do.
	scratch, err := os.MkdirTemp("", "cljgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	if err := gen(editorC, scratch, cljOut); err != nil {
		return "", err
	}
	if _, err := os.Stat(cljOut); errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoBackend
	} else if err != nil {
		return "", err
	}
	return cljOut, nil
}

// Compile compiles editorClj, the namespace whim.editor, with the embedded
// sources and the Java runtime and host into dir/classes, copies Clojure's
// jars into dir/lib, merges the two into one executable jar (dir/vijure.jar),
// trains its AOT cache (dir/vijure.aot) and writes the launcher at the path
// launcher, which it returns.  A reflection warning anywhere -- the
// generated namespace's or the glue's -- is refused: a reflective call is
// orders of magnitude slower, and the contract is that there is none.
func Compile(editorClj, dir, launcher string) (string, error) {
	srcDir, classes, libDir := filepath.Join(dir, "src"), filepath.Join(dir, "classes"), filepath.Join(dir, "lib")
	for _, d := range []string{srcDir, classes, libDir} {
		if err := os.RemoveAll(d); err != nil {
			return "", err
		}
	}
	if _, err := WriteSources(srcDir); err != nil {
		return "", err
	}
	ns, err := os.ReadFile(editorClj)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "whim", "editor.clj"), ns, 0o644); err != nil {
		return "", err
	}
	// the Java runtime and host, as braaam compiles them
	java, err := braaam.WriteRuntime(filepath.Join(dir, "java"))
	if err != nil {
		return "", err
	}
	args := append([]string{"-nowarn", "-encoding", "UTF-8", "-d", classes}, java...)
	if out, err := command("javac", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("javac: %v\n%s", err, out)
	}
	jars, err := ClojureJars(dir)
	if err != nil {
		return "", err
	}
	var local []string
	for _, j := range jars {
		dst := filepath.Join(libDir, filepath.Base(j))
		if err := copyFile(j, dst); err != nil {
			return "", err
		}
		local = append(local, dst)
	}
	if err := AOT(srcDir, classes, local); err != nil {
		return "", err
	}
	jar := filepath.Join(dir, JarName)
	if err := Jar(dir, jar); err != nil {
		return "", err
	}
	Train(jar, filepath.Join(dir, CacheName))
	return WriteLauncher(launcher, dir)
}

// CompileFlags are the Clojure compiler's options, each for a reason:
//
//   - clojure.compiler.direct-linking: a call to a function of another
//     namespace is a static call, not a deref of its var and an interface
//     call -- the core calls the glue, and itself, on nearly every line.
//   - clojure.compiler.elide-meta: no docstrings and arglists kept in the
//     classes the launcher loads.
var CompileFlags = []string{
	"-Dclojure.compiler.direct-linking=true",
	"-Dclojure.compiler.elide-meta=[:doc :file :line :added]",
}

// AOT compiles the namespaces under srcDir, from whim.cljmain down -- it
// requires whim.editor and whim.cljhost -- into classes, with
// *warn-on-reflection* and *unchecked-math* on, and refuses a reflection
// warning.
func AOT(srcDir, classes string, jars []string) error {
	if err := os.MkdirAll(classes, 0o755); err != nil {
		return err
	}
	cp := strings.Join(append([]string{srcDir, classes}, jars...), string(os.PathListSeparator))
	args := append([]string{"-Xss1g", "-cp", cp, "-Dclojure.compile.path=" + classes}, CompileFlags...)
	args = append(args, "clojure.main", "-e",
		"(set! *warn-on-reflection* true) (set! *unchecked-math* true) (compile '"+Main+")")
	out, err := command("java", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("the Clojure compiler: %v\n%s", err, out)
	}
	if bytes.Contains(out, []byte("Reflection warning")) {
		return fmt.Errorf("the Clojure compiler warns of reflection, which the contract refuses:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(classes, "whim", "cljmain.class")); err != nil {
		return fmt.Errorf("the Clojure compiler wrote no %s class: %v\n%s", Main, err, out)
	}
	return nil
}

// ClojureJars is the jars of Clojure (clojure, spec.alpha, core.specs.alpha),
// as the `clojure` command resolves them from ~/.m2 for a project that names
// nothing more: `clojure -Spath` in a directory of its own under dir, since it
// writes its cache beside the deps.edn it reads.
func ClojureJars(dir string) ([]string, error) {
	if _, err := exec.LookPath("clojure"); err != nil {
		return nil, fmt.Errorf("vijure: no `clojure` command to find Clojure's jars with: %w", err)
	}
	proj := filepath.Join(dir, "deps")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(proj, "deps.edn"), []byte("{:paths []}\n"), 0o644); err != nil {
		return nil, err
	}
	cmd := command("clojure", "-Spath")
	cmd.Dir = proj
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("clojure -Spath: %v\n%s", err, stderr.Bytes())
	}
	var jars []string
	for _, p := range filepath.SplitList(strings.TrimSpace(string(out))) {
		if strings.HasSuffix(p, ".jar") {
			jars = append(jars, p)
		}
	}
	if len(jars) == 0 {
		return nil, fmt.Errorf("clojure -Spath names no jar: %q", out)
	}
	return jars, nil
}

// WriteSources writes the embedded Clojure sources under dir and returns
// their paths.
func WriteSources(dir string) ([]string, error) {
	var out []string
	err := fs.WalkDir(sources, "src", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := sources.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "src/")))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
		out = append(out, dst)
		return nil
	})
	return out, err
}

// JVMFlags are the launcher's options to the JVM: braaam's, for the same
// reasons (the terminal host's native access, no hsperfdata, a short run's
// collector and compiler).  The stack is the core thread's own, 1 GiB
// (whim.cljmain).
var JVMFlags = braaam.JVMFlags

// JarName and CacheName are the editor as one jar and its AOT cache, in the
// build's directory: what the launcher runs.
const JarName, CacheName = "vijure.jar", "vijure.aot"

// TrainingKeys are the keys of the run that writes the AOT cache: into
// insert mode, a word, out, and :q! -- the editor's start, its drawing and its
// end, which is what a run loads.
var TrainingKeys = []byte("ihello\x1b:q!\r")

// Train runs the editor in the jar once, on TrainingKeys, with the JVM told
// to write its AOT cache (JDK 25 and later, JEP 483 and 514) at cache: the
// classes the run loaded, parsed, verified and linked, and the profiles of
// the methods it ran, mapped at the next start rather than made again.
// Measured on the stand-in namespace: 590 ms a run without it, 210 ms with.
// The cache is written at the JVM's exit, System/exit included -- unlike
// the AppCDS archive the Java editor tried (-XX:+AutoCreateSharedArchive) --
// and it needs a class path of jars only, which is why the launcher runs the
// jar and not the class directory.  It reports whether the cache
// was written: a JDK without it, or a run that fails, leaves none, and the
// launcher then runs without one.
func Train(jar, cache string) bool {
	os.Remove(cache)
	in, err := os.CreateTemp("", "cljtrain.")
	if err != nil {
		return false
	}
	defer os.Remove(in.Name())
	defer in.Close()
	if _, err := in.Write(TrainingKeys); err != nil {
		return false
	}
	if _, err := in.Seek(0, 0); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := append(append([]string{}, JVMFlags...), "-XX:AOTCacheOutput="+cache, "-cp", jar, Main)
	cmd := exec.CommandContext(ctx, "java", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Setpgid: true}
	cmd.Stdin = in
	cmd.Run() // its status is the editor's; what counts is the cache
	st, err := os.Stat(cache)
	return err == nil && st.Size() > 0
}

// WriteLauncher writes path, a shell script that runs the editor built in
// dir (its jar, and its AOT cache when there is one) as a binary is run: its
// arguments the editor's, $0 its argv[0], and exec, so that the JVM is the
// process the caller started -- its pid, its process group, its signals.  A
// cache the JVM cannot use (the jar rebuilt without it, another JDK) is used
// silently not at all: -Xlog:aot=off,cds=off, since the JVM would say so on
// the editor's screen.
func WriteLauncher(path, dir string) (string, error) {
	java, err := exec.LookPath("java")
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	flags := append([]string{}, JVMFlags...)
	if cache := filepath.Join(abs, CacheName); fileExists(cache) {
		flags = append(flags, "-XX:AOTCache="+braaam.ShellQuote(cache), "-Xlog:aot=off,cds=off")
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# The editor in Clojure (vijure/): written by vijure.WriteLauncher.\n")
	fmt.Fprintf(&b, "exec %s %s -cp %s -Dwhim.argv0=\"$0\" %s \"$@\"\n",
		braaam.ShellQuote(java), strings.Join(flags, " "), braaam.ShellQuote(filepath.Join(abs, JarName)), Main)
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Jar writes the editor built in dir as one executable jar at path: the
// classes and Clojure's jars merged, a manifest naming the main class and
// granting the terminal host its native access (JDK 22 and later), so that
// `java -jar path [args]` runs it with no flag.
//
// Every entry keeps its time: Clojure loads a namespace from its AOT class
// only when the class is NEWER than the .clj beside it, and compiles the
// source otherwise -- entries of one time made the jar start in 2.1 s
// against the class directory's 0.6 s, clojure.core compiled at every run.
func Jar(dir, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	fail := func(err error) error {
		zw.Close()
		f.Close()
		os.Remove(path)
		return err
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "META-INF/MANIFEST.MF", Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return fail(err)
	}
	fmt.Fprintf(w, "Manifest-Version: 1.0\r\nMain-Class: %s\r\nEnable-Native-Access: ALL-UNNAMED\r\n\r\n", Main)
	seen := map[string]bool{"META-INF/MANIFEST.MF": true}
	classes := filepath.Join(dir, "classes")
	err = filepath.WalkDir(classes, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(classes, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name, h.Method = filepath.ToSlash(rel), zip.Deflate
		seen[h.Name] = true
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		return fail(err)
	}
	jars, err := filepath.Glob(filepath.Join(dir, "lib", "*.jar"))
	if err != nil {
		return fail(err)
	}
	for _, j := range jars {
		zr, err := zip.OpenReader(j)
		if err != nil {
			return fail(err)
		}
		for _, e := range zr.File {
			if seen[e.Name] || strings.HasSuffix(e.Name, "/") || strings.EqualFold(e.Name, "META-INF/MANIFEST.MF") {
				continue
			}
			seen[e.Name] = true
			if err := zw.Copy(e); err != nil { // raw, its header and time kept
				zr.Close()
				return fail(err)
			}
		}
		zr.Close()
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
