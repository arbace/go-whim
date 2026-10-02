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

// hsRuntimeDir is caprice's runtime, which the generated module imports.
const hsRuntimeDir = "../../caprice/rt"

// hsHarness is the tests' host: out, outs and a printf of what the tests
// print, each with the editor first; and main, which runs the C's run on a
// new editor.
const hsHarness = `module Host where

import Caprice.Rt
import qualified Data.ByteString as B
import qualified Data.ByteString.Char8 as BC
import Foreign.Marshal.Alloc (callocBytes)
import Numeric (showHex)
import System.IO (stdout)

alloc :: Ed -> Word64 -> IO P
alloc _ n = callocBytes (max 1 (fromIntegral n))

vim_free :: Ed -> P -> IO ()
vim_free _ _ = pure ()

memmove :: Ed -> P -> P -> Word64 -> IO P
memmove _ d s n = copyMem d s (fromIntegral n) >> pure d

memcpy :: Ed -> P -> P -> Word64 -> IO P
memcpy = memmove

memset :: Ed -> P -> Int32 -> Word64 -> IO P
memset _ d c n = fillMem d (fromIntegral c) (fromIntegral n) >> pure d

memcmp :: Ed -> P -> P -> Word64 -> IO Int32
memcmp _ a b n = go 0
  where
    go i | i >= fromIntegral n = pure 0
    go i = do
      x <- rdW8 a i
      y <- rdW8 b i
      if x /= y then pure (if x < y then -1 else 1) else go (i + 1)

out :: Ed -> Int64 -> IO ()
out _ v = BC.hPutStrLn stdout (BC.pack (show v))

outs :: Ed -> P -> IO ()
outs _ p = do
  bs <- cBytes p
  B.hPut stdout (B.pack bs)
  BC.hPutStrLn stdout BC.empty

outf :: Ed -> P -> [VArg] -> IO ()
outf _ fmt args = do
  f <- cBytes fmt
  s <- go (map (toEnum . fromIntegral) f) args
  BC.hPutStrLn stdout (BC.pack s)
  where
    go :: String -> [VArg] -> IO String
    go [] _ = pure ""
    go ('%' : '%' : r) as = ('%' :) <$> go r as
    go ('%' : r) as = do
      let r' = dropWhile (== 'l') r
      case (r', as) of
        (c : r'', a : as') -> (++) <$> one c a <*> go r'' as'
        _ -> pure "?"
    go (c : r) as = (c :) <$> go r as
    one 'd' (VI v) = pure (show v)
    one 'd' (VU v) = pure (show (fromIntegral v :: Int64))
    one 'i' a = one 'd' a
    one 'u' (VU v) = pure (show v)
    one 'u' (VI v) = pure (show (fromIntegral v :: Word32))
    one 'x' (VU v) = pure (showHex v "")
    one 'x' (VI v) = pure (showHex (fromIntegral v :: Word32) "")
    one 'c' (VI v) = pure [toEnum (fromIntegral v)]
    one 'c' (VU v) = pure [toEnum (fromIntegral v)]
    one 's' (VP p) = map (toEnum . fromIntegral) <$> cBytes p
    one _ _ = pure "?"
`

const hsMain = `module Main where

import Caprice.Rt (toDyn)
import qualified Editor

main :: IO ()
main = Editor.newEditor (toDyn ()) >>= Editor.run
`

func requireHs(t *testing.T) {
	for _, tool := range []string{"gcc", "ghc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// hsProgram translates src and returns the module and the refusals.
func hsProgram(t *testing.T, dir, src string, prof Profile) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Editor.hs")
	if rc := Run([]string{c, dir, "-hs", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// hsOutput compiles the module with the harness and returns what it prints.
func hsOutput(t *testing.T, dir, prog string) string {
	if err := os.WriteFile(filepath.Join(dir, "Editor.hs"), []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Host.hs"), []byte(hsHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Main.hs"), []byte(hsMain), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, _ := filepath.Abs(hsRuntimeDir)
	exe := filepath.Join(dir, "hsprog")
	o, err := bounded(300*time.Second, dir, "ghc", "-O0", "-v0", "-i"+rt, "-i"+dir, "-outputdir", filepath.Join(dir, "o"), "-o", exe, filepath.Join(dir, "Main.hs"))
	if err != nil {
		t.Fatalf("ghc: %v\n%s\n%s", err, o, numbered(prog))
	}
	b, err := bounded(60*time.Second, dir, exe)
	if err != nil {
		t.Fatalf("the Haskell: %v\n%s\n%s", err, b, numbered(prog))
	}
	return string(b)
}

// hsSame translates src, requires every function written, and requires the
// Haskell to print what the C prints; it returns the Haskell.
func hsSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireHs(t)
	dir := t.TempDir()
	prog, refused := hsProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := hsOutput(t, dir, prog); got != want {
		t.Errorf("the Haskell prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

func TestHsIntegers(t *testing.T)  { hsSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestHsFlow(t *testing.T)      { hsSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestHsStrings(t *testing.T)   { hsSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestHsStructs(t *testing.T)   { hsSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestHsExtra(t *testing.T)     { hsSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestHsVarargs(t *testing.T)   { hsSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestHsPointers(t *testing.T)  { hsSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestHsGoto(t *testing.T)      { hsSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestHsShapes(t *testing.T)    { hsSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestHsNest(t *testing.T)      { hsSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestHsMachine(t *testing.T)   { hsSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestHsDeadLabel(t *testing.T) { hsSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestHsNames(t *testing.T)     { hsSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestHsProfile(t *testing.T)   { hsSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestHsGrow(t *testing.T)      { hsSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated Haskell -- unsigned
// division done signed, an unsigned char widened with its sign, an unsigned
// shift done arithmetically, a struct's assignment not copied -- and each
// must move the output.
func TestHsControl(t *testing.T) {
	requireHs(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned division", javaIntsC, regexp.MustCompile(`quot (a\w*) (b\w*)`), "fromIntegral (quot (fromIntegral $1 :: Int32) (fromIntegral $2 :: Int32))"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`\(fromIntegral (c\w*) :: Int32\) \+ 1\b`), "(fromIntegral (fromIntegral $1 :: Int8) :: Int32) + 1"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`shiftR (a\w*) \(fromIntegral (n\w*)\)`), "fromIntegral (shiftR (fromIntegral $1 :: Int32) (fromIntegral $2))"},
		{"struct copy", javaStructsC, regexp.MustCompile(`copyMem p\w* \(addr'g1 ed'\) 56`), "pure ()"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			prog, refused := hsProgram(t, dir, c.src, Profile{})
			if refused != "" {
				t.Fatalf("refused:\n%s", refused)
			}
			if !c.from.MatchString(prog) {
				t.Fatalf("the mutation's text %s is not in the program:\n%s", c.from, numbered(prog))
			}
			want := cOutputWith(t, dir, c.src, javaHarnessC)
			if got := hsOutput(t, dir, c.from.ReplaceAllString(prog, c.repl)); got == want {
				t.Errorf("the mutation %q did not move the output", c.name)
			}
		})
	}
}

// hsTokens reads past what is not a name: a string, a character literal --
// '"' among them, which once opened a string and hid every name after it --
// and comments.
func TestHsTokens(t *testing.T) {
	got := strings.Join(hsTokens(`a (ch '"') b {- '"' -} c "x \" d" e -- f`+"\n"+`g' h'1 'q'`), " ")
	if want := "a ch b c e g' h'1"; got != want {
		t.Errorf("hsTokens = %q, want %q", got, want)
	}
}

func TestHsUnparen(t *testing.T) {
	for in, want := range map[string]string{
		`(a + b)`:             `a + b`,
		`(a) + (b)`:           `(a) + (b)`,
		`(c == (ch '"'))`:     `c == (ch '"')`,
		`(c == (ch ')')) + 1`: `(c == (ch ')')) + 1`,
		`(f x' (y'))`:         `f x' (y')`,
		`(Ptr "(("# :: P)`:    `Ptr "(("# :: P`,
		`()`:                  `()`,
		`(a, b)`:              `(a, b)`,
		`(f (a, b))`:          `f (a, b)`,
	} {
		if got := hsUnparen(in); got != want {
			t.Errorf("hsUnparen(%s) = %s, want %s", in, got, want)
		}
	}
}

// hsIdiomsC is what doc/HASKELL-IDIOMS.md's items 4-11 change the printing
// of: pure functions and their loops, called where C evaluates them lazily;
// out- and in-out parameters; struct locals by member and copied whole;
// pointers of several types, converted, compared and called through; and a
// typedef'd enumeration.
const hsIdiomsC = javaHost + `
typedef unsigned char char_u;
typedef long linenr_T;
typedef enum { ST_NONE, ST_ONE, ST_TWO } state_E;
typedef struct { linenr_T lnum; int col; } pos_T;
typedef struct { pos_T cur; char_u *name; } win_T;

static int gcount;
static pos_T gpos = {7, 2};
static win_T gwin;

void nothing(void) { }
int isdig(int c) { return (unsigned)c - '0' < 10; }
int digits(long n) { int k = 1; while (n >= 10) { n /= 10; k++; } return k; }
int sumto(int n) { int s = 0; for (int i = 0; i < n; i++) { if (i % 3 == 0) continue; s += i; } return s; }
state_E next(state_E s) { switch (s) { case ST_NONE: return ST_ONE; case ST_ONE: return ST_TWO; default: return ST_NONE; } }

void split(int v, int *q, int *r) { *q = v / 10; *r = v % 10; }
int maybe_len(const char *s, int *lenp) { int n = 0; while (s[n]) n++; if (lenp != 0) *lenp = n; return n > 3; }
void bump(int *n) { *n += 1; }
void twice(int *n) { bump(n); bump(n); }

void move_pos(pos_T *pp, int dc) { pos_T a = *pp; a.col += dc; a.lnum++; *pp = a; }
long pos_key(void) { pos_T b = {3, 4}; pos_T c; c = b; c.col = c.col * 2; return c.lnum * 100 + c.col; }

int cmpv(const void *a, const void *b) { return *(const int *)a - *(const int *)b; }
int apply(int (*f)(const void *, const void *), int x, int y) { return f(&x, &y); }
char_u *skip(char_u *p) { while (*p == ' ') p++; return p; }

void run(void) {
    char buf[] = "  word";
    int q, r, len = -1, n = 5;
    out(isdig('7') && digits(12345) == 5 ? sumto(10) : -1);
    nothing();
    out(next(next(ST_ONE)));
    split(47, &q, &r); out(q); out(r);
    out(maybe_len("abcdef", &len)); out(len);
    out(maybe_len("ab", 0));
    bump(&n); twice(&n); out(n);
    bump(&gcount); out(gcount);
    move_pos(&gpos, 3); out(gpos.lnum); out(gpos.col);
    gwin.cur = gpos; move_pos(&gwin.cur, -1); out(gwin.cur.col);
    out(pos_key());
    out(apply(cmpv, 9, 4));
    char_u *w = skip((char_u *)buf); outs((char *)w);
    void *vp = w; out((char *)vp == buf + 2);
}
`

func TestHsIdioms(t *testing.T) { hsSame(t, hsIdiomsC, Profile{}, javaHarnessC) }

// The same programs split into modules by the call graph (hssplit.go): a
// part imports only what it calls into, and the top the table.
func TestHsSplit(t *testing.T) {
	for _, c := range []struct{ name, src string }{{"idioms", hsIdiomsC}, {"structs", javaStructsC}, {"goto", javaGotoC}} {
		t.Run(c.name, func(t *testing.T) { hsSame(t, c.src, Profile{HsParts: 3}, javaHarnessC) })
	}
}

// hsTupleC is C as phase 100 writes it (crefactor/xform's LocalOut): a
// function returning a struct of its result and its out-parameters' values,
// its callers reading the struct's members back.
const hsTupleC = javaHost + `
typedef struct { int r__; int q; long rest; } divmod__out_T;
typedef struct { int lnum; int col; } pos_T;
static divmod__out_T divmod(int v, int q, long rest) { q = v / 7; rest = v % 7; { divmod__out_T out__; out__.r__ = (q > 2); out__.q = q; out__.rest = rest; return out__; } }
static pos_T mkpos(int l, int c) { pos_T p; p.lnum = l; p.col = c; return p; }
static int sum(pos_T p) { return p.lnum + p.col; }
static pos_T pick(int k) { switch (k) { case 1: return mkpos(1, 1); case 2: return mkpos(2, 2); } return mkpos(0, 0); }
static pos_T gp;
void run(void) {
    divmod__out_T divmod__o;
    int q = 0;
    long rest = 0;
    int big = (divmod__o = divmod(45, q, rest), q = divmod__o.q, rest = divmod__o.rest, divmod__o.r__);
    out(big); out(q); out(rest);
    if ((divmod__o = divmod(3, q, rest), q = divmod__o.q, rest = divmod__o.rest, divmod__o.r__) || q == 0)
        out(q);
    pos_T a = mkpos(4, 5);
    out(a.lnum * 10 + a.col);
    out(sum(mkpos(1, 2)));
    gp = mkpos(8, 9);
    out(gp.col);
    out(pick(2).col + pick(5).lnum);
    pos_T t = q > 1 ? mkpos(3, 4) : mkpos(5, 6);
    out(t.col);
}
`

func TestHsTuples(t *testing.T) { hsSame(t, hsTupleC, Profile{}, javaHarnessC) }
