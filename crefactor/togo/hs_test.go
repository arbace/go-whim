package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		{"unsigned division", javaIntsC, regexp.MustCompile(`pure \(quot a b\)`), "pure (fromIntegral (quot (fromIntegral a :: Int32) (fromIntegral b :: Int32)))"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`\(fromIntegral c :: Int32\) \+ \(1 :: Int32\)`), "(fromIntegral (fromIntegral c :: Int8) :: Int32) + (1 :: Int32)"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`pure \(shiftR a \(fromIntegral \(fromIntegral n :: Int64\)\)\)`), "pure (fromIntegral (shiftR (fromIntegral a :: Int32) (fromIntegral n)))"},
		{"struct copy", javaStructsC, regexp.MustCompile(`copyMem \(pAdd fr' 80\) \(pAdd fr' 72\) 8`), "pure ()"},
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
