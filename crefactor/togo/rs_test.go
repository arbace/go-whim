package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// rsRuntime is whimsy's runtime, which the generated module uses.
const rsRuntime = "../../whimsy/src/rt.rs"

// rsHarness is the tests' host: out, outs and a printf of what the tests
// print, and the allocator of the tests that grow, each with the editor
// first.
const rsHarness = `use crate::editor::Editor;
use crate::rt::VArg;
use core::ffi::c_void;
use std::io::Write;

unsafe fn cstr(p: *const u8) -> Vec<u8> {
    let mut v = Vec::new();
    let mut i = 0;
    while *p.add(i) != 0 {
        v.push(*p.add(i));
        i += 1;
    }
    v
}

fn put(b: &[u8]) {
    let mut o = std::io::stdout().lock();
    o.write_all(b).unwrap();
    o.write_all(b"\n").unwrap();
}

pub unsafe fn out(_ed: *mut Editor, v: i64) {
    put(v.to_string().as_bytes());
}

pub unsafe fn outs(_ed: *mut Editor, s: *mut i8) {
    put(&cstr(s as *const u8));
}

pub unsafe fn alloc(_ed: *mut Editor, n: u64) -> *mut c_void {
    std::alloc::alloc_zeroed(std::alloc::Layout::from_size_align(n.max(1) as usize, 16).unwrap()) as *mut c_void
}

pub unsafe fn vim_free(_ed: *mut Editor, _p: *mut c_void) {}

pub unsafe fn memmove(_ed: *mut Editor, d: *mut c_void, s: *mut c_void, n: u64) -> *mut c_void {
    core::ptr::copy(s as *const u8, d as *mut u8, n as usize);
    d
}

pub unsafe fn memcpy(ed: *mut Editor, d: *mut c_void, s: *mut c_void, n: u64) -> *mut c_void {
    memmove(ed, d, s, n)
}

pub unsafe fn memset(_ed: *mut Editor, d: *mut c_void, c: i32, n: u64) -> *mut c_void {
    core::ptr::write_bytes(d as *mut u8, c as u8, n as usize);
    d
}

pub unsafe fn memcmp(_ed: *mut Editor, a: *mut c_void, b: *mut c_void, n: u64) -> i32 {
    for i in 0..n as usize {
        let (x, y) = (*(a as *const u8).add(i), *(b as *const u8).add(i));
        if x != y {
            return if x < y { -1 } else { 1 };
        }
    }
    0
}

/// a printf of d, i, u, x, c, s and l, reading the arguments as C does
pub unsafe fn outf(_ed: *mut Editor, fmt: *mut i8, args: &[VArg]) {
    let f = cstr(fmt as *const u8);
    let mut b: Vec<u8> = Vec::new();
    let mut k = 0;
    let mut i = 0;
    while i < f.len() {
        let c = f[i];
        i += 1;
        if c != b'%' {
            b.push(c);
            continue;
        }
        let mut l = 0;
        while f[i] == b'l' {
            l += 1;
            i += 1;
        }
        let conv = f[i];
        i += 1;
        if conv == b'%' {
            b.push(b'%');
            continue;
        }
        let a = args[k];
        k += 1;
        let v = a.bits();
        let s = match conv {
            b'd' | b'i' => if l > 0 { (v as i64).to_string() } else { (v as i32).to_string() },
            b'u' => if l > 0 { v.to_string() } else { (v as u32).to_string() },
            b'x' => if l > 0 { format!("{:x}", v) } else { format!("{:x}", v as u32) },
            b'c' => String::from_utf8_lossy(&[v as u8]).into_owned(),
            b's' => String::from_utf8_lossy(&cstr(a.ptr())).into_owned(),
            _ => panic!("outf: %{}", conv as char),
        };
        b.extend_from_slice(s.as_bytes());
    }
    assert_eq!(k, args.len(), "unused arguments");
    put(&b);
}
`

const rsLib = "pub mod rt;\npub mod host;\npub mod editor;\n"

const rsMain = "fn main() {\n    let ed = prog::editor::new_editor();\n    unsafe { prog::editor::run(ed) }\n}\n"

func requireRs(t *testing.T) {
	for _, tool := range []string{"gcc", "rustc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// rsProgram translates src and returns the module and the refusals.
func rsProgram(t *testing.T, dir, src string, prof Profile) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.rs")
	if rc := Run([]string{c, dir, "-rs", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// rsBuild compiles the module with the harness, warnings denied, into
// dir/prog.
func rsBuild(t *testing.T, dir, prog string) string {
	files := map[string]string{"editor.rs": prog, "host.rs": rsHarness, "lib.rs": rsLib, "main.rs": rsMain}
	rt, err := os.ReadFile(rsRuntime)
	if err != nil {
		t.Fatal(err)
	}
	files["rt.rs"] = string(rt)
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o, err := bounded(300*time.Second, "", "rustc", "--edition", "2021", "-D", "warnings", "--crate-type", "rlib", "--crate-name", "prog",
		"-C", "opt-level=0", "--out-dir", dir, filepath.Join(dir, "lib.rs"))
	if err != nil {
		t.Fatalf("rustc: %v\n%s\n%s", err, o, numbered(prog))
	}
	exe := filepath.Join(dir, "rsprog")
	o, err = bounded(300*time.Second, "", "rustc", "--edition", "2021", "-D", "warnings", "-C", "opt-level=0",
		"--extern", "prog="+filepath.Join(dir, "libprog.rlib"), "-o", exe, filepath.Join(dir, "main.rs"))
	if err != nil {
		t.Fatalf("rustc main: %v\n%s", err, o)
	}
	return exe
}

// rsOutput compiles the module with the harness and returns what it prints.
func rsOutput(t *testing.T, dir, prog string) string {
	exe := rsBuild(t, dir, prog)
	b, err := bounded(60*time.Second, "", exe)
	if err != nil {
		t.Fatalf("the Rust: %v\n%s\n%s", err, b, numbered(prog))
	}
	return string(b)
}

// rsSame translates src, requires every function written, and requires the
// Rust to print what the C prints; it returns the Rust.
func rsSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireRs(t)
	dir := t.TempDir()
	prog, refused := rsProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := rsOutput(t, dir, prog); got != want {
		t.Errorf("the Rust prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

func TestRsIntegers(t *testing.T)  { rsSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestRsFlow(t *testing.T)      { rsSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestRsStrings(t *testing.T)   { rsSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestRsStructs(t *testing.T)   { rsSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestRsExtra(t *testing.T)     { rsSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestRsVarargs(t *testing.T)   { rsSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestRsPointers(t *testing.T)  { rsSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestRsGoto(t *testing.T)      { rsSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestRsShapes(t *testing.T)    { rsSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestRsNest(t *testing.T)      { rsSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestRsMachine(t *testing.T)   { rsSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestRsDeadLabel(t *testing.T) { rsSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestRsNames(t *testing.T)     { rsSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestRsProfile(t *testing.T)   { rsSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestRsGrow(t *testing.T)      { rsSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }
func TestRsSteps(t *testing.T)     { rsSame(t, javaStepsC, Profile{}, javaHarnessC) }
func TestRsMasks(t *testing.T)     { rsSame(t, javaMasksC, Profile{}, javaHarnessC) }
func TestRsLocals(t *testing.T)    { rsSame(t, javaLocalsC, Profile{}, javaHarnessC) }
func TestRsTables(t *testing.T)    { rsSame(t, cljTablesC, Profile{}, javaHarnessC) }
func TestRsCljNames(t *testing.T)  { rsSame(t, cljNamesC, Profile{}, javaHarnessC) }
func TestRsSplitRet(t *testing.T)  { rsSame(t, cljSplitReturnC, Profile{}, javaHarnessC) }

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated Rust -- unsigned
// division done signed, an unsigned char widened with its sign, an unsigned
// shift done arithmetically, a struct's assignment not made -- and each
// must move the output.
func TestRsControl(t *testing.T) {
	requireRs(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned division", javaIntsC, regexp.MustCompile(`(fn udiv\(.*\n\s*return )a\.wrapping_div\(b\);`), "${1}(a as i32).wrapping_div(b as i32) as u32;"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`(fn widen_uchar\(.*\n\s*return )\(c as i32\)\.wrapping_add\(1\);`), "${1}(c as i8 as i32).wrapping_add(1);"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`(fn ushr\(.*\n\s*return )a\.wrapping_shr\(n as u32\);`), "${1}(a as i32).wrapping_shr(n as u32) as u32;"},
		{"struct copy", javaStructsC, regexp.MustCompile(`\(\*ed\)\.g2 = \(\*ed\)\.g1;`), "let _ = (*ed).g1;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			prog, _ := rsProgram(t, dir, c.src, Profile{})
			if !c.from.MatchString(prog) {
				t.Fatalf("the control's pattern %s is not in the Rust:\n%s", c.from, numbered(prog))
			}
			bad := c.from.ReplaceAllString(prog, c.repl)
			want := cOutputWith(t, dir, c.src, javaHarnessC)
			exe := rsBuild(t, dir, bad)
			got, _ := bounded(60*time.Second, "", exe) // a crash moves the output too
			if string(got) == want {
				t.Errorf("the mutation %q moved nothing", c.name)
			}
		})
	}
}

// numberedRs is unused when the tests pass; strings keeps the import.
var _ = strings.TrimSpace
