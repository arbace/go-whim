package whim

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/togo"
)

// What the generator is told about the core it translates to Go:
// editor/crt.go's contract with vim's
// core.  internal/gen hard-coded all of it before -- in skel.go, editor.go,
// gen.go, analyze.go and body_expr.go -- and names none of it now.

// gaGrowInner is ga_grow_inner(): a function whose body is a rule of the
// runtime's rather than a translation (match_lines is the other) (internal/gen/CONVENTIONS.md,
// Growarrays).  C allocates, copies and clears new_len bytes; GaGrowTo does it
// in elements of the storage's type.
const gaGrowInner = `func ga_grow_inner(gap *S_growarray, n int32) int32 {
	if n < gap.ga_growsize {
		n = gap.ga_growsize
	}

	if n < gap.ga_len/2 {
		n = gap.ga_len / 2
	}

	if n > 0 && usize(gap.ga_len+n) > SIZE_MAX/usize(gap.ga_itemsize) {
		return FAIL
	}
	// C allocates, copies and clears new_len bytes here; GaGrowTo does it
	// in elements of the storage's type.
	gap.ga_maxlen = gap.ga_len + n
	GaGrowTo(gap, int(gap.ga_maxlen))
	return OK
}
`

// gaGrowInnerFor is gaGrowInner with the result type the C gives it: `int`,
// OK and FAIL, until phase 166 made a success bool, and then `bool`, true and
// false.
func gaGrowInnerFor(result string) string {
	if result != "bool" {
		return gaGrowInner
	}
	s := strings.Replace(gaGrowInner, "n int32) int32 {", "n int32) bool {", 1)
	s = strings.ReplaceAll(s, "return FAIL", "return false")
	return strings.ReplaceAll(s, "return OK", "return true")
}

// gaGrowInnerJava is ga_grow_inner()'s body in Java: what the Go's is,
// without the growth -- the Java's Ga (braaam/rt) grows the storage at its
// next typed access to ga_maxlen elements, which is where it learns their
// type.
func gaGrowInnerJava(result string) string {
	fail, ok := "FAIL", "OK"
	if result == "boolean" {
		fail, ok = "false", "true"
	}
	return `        if (n < gap.ga_growsize) {
            n = gap.ga_growsize;
        }
        if (n < gap.ga_len / 2) {
            n = gap.ga_len / 2;
        }
        if (n > 0 && Long.compareUnsigned((long) (gap.ga_len + n), Long.divideUnsigned(-1L, (long) gap.ga_itemsize)) > 0) {
            return ` + fail + `;
        }
        // C allocates, copies and clears the storage here; Ga does it at the
        // next typed access, in elements of the storage's type.
        gap.ga_maxlen = gap.ga_len + n;
        return ` + ok + `;
`
}

// gaGrowInnerClj is ga_grow_inner()'s body in Clojure: the Java's, one
// expression -- the storage grows at its next typed access (Ga).
func gaGrowInnerClj(result string) string {
	fail, ok := "FAIL", "OK"
	if result == "boolean" {
		fail, ok = "false", "true"
	}
	return `(let [n (if (< n (.ga-growsize gap)) (.ga-growsize gap) n)
      n (if (< n (quot (.ga-len gap) 2)) (quot (.ga-len gap) 2) n)]
  (if (and (> n 0) (pos? (Long/compareUnsigned (+ (.ga-len gap) n) (Long/divideUnsigned -1 (.ga-itemsize gap)))))
    ` + fail + `
    (do ;; C allocates, copies and clears the storage here; Ga does it at the
        ;; next typed access, in elements of the storage's type.
        (.set-ga-maxlen gap (+ (.ga-len gap) n))
        ` + ok + `)))`
}

// matchLines is match_lines()'s body, the one parallel body: the C's is one
// loop over the range's lines, each matched alone (phase 177); these run it
// in chunks at once, each chunk on a regex engine of its own -- which the
// engine's state being a parameter (phase 176) allows -- and are held to the
// C's answers by the suite's par_* cases.  The chunks are the runtime's:
// Chunks in Go (editor/chunks.go), Rt.chunks in Java and Clojure
// (braaam/rt).
const matchLines = `func match_lines(rmp *regmmatch_T, do_all bool, buf *S_file_buffer, lines Ptr[string_T], line1 linenr_T, n linenr_T, found Ptr[linefound_T]) bool {
	return Chunks(int(n), func(from, to int) bool {
		re := new(S_regengine_S)
		match_chunk(re, rmp, do_all, buf, lines, line1, linenr_T(from), linenr_T(to), found)
		return !re.failed
	})
}
`

func matchLinesGo(string) string { return matchLines }

func matchLinesJava(string) string {
	return `        return Rt.chunks(n, (from, to) -> {
            S_regengine_S re = new S_regengine_S();
            match_chunk(re, rmp, do_all, buf, lines, line1, from, to, found);
            return !re.failed;
        });
`
}

// matchLinesHs is match_lines()'s body in Haskell: the chunks on forkIO's
// threads (Caprice.Rt's chunks), each on an engine the core's alloc_clear
// makes, the C's sizes asked of the front end.
func matchLinesHs(string) string {
	return `chunks (fromIntegral n) $ \from to -> do
  re <- alloc_clear ed' {{sizeof regengine_T}}
  match_chunk ed' (castPtr re) rmp do_all buf lines line1 (fromIntegral from) (fromIntegral to) found
  failed <- rdB re {{offsetof regengine_T failed}}
  pure (not failed)`
}

// matchLinesRs is match_lines()'s body in Rust: the chunks on scoped
// threads (whimsy/src/rt.rs's chunks), each on an engine the core's
// alloc_clear makes; the raw pointers go to the threads Shared.
func matchLinesRs(string) string {
	return `let (ed_, rmp_, buf_, lines_, found_) = (Shared(ed), Shared(rmp), Shared(buf), Shared(lines), Shared(found));
chunks(n, |from, to| {
    let re = alloc_clear(ed_.get(), core::mem::size_of::<regengine_T>() as u64) as *mut regengine_T;
    match_chunk(ed_.get(), re, rmp_.get(), do_all, buf_.get(), lines_.get(), line1, from, to, found_.get());
    !(*re).failed
})`
}

// matchLinesScm is match_lines()'s body in Scheme: the chunks on
// fork-thread's threads (whimsical/whimsical/rt.ss's chunks), each with a
// stack of its own and on an engine the core's alloc_clear makes, the C's
// sizes asked of the front end.
func matchLinesScm(string) string {
	return `(chunks ed n
  (lambda (ed from to)
    (let ([re (alloc_clear ed {{sizeof regengine_T}})])
      (match_chunk ed re rmp do_all buf lines line1 from to found)
      (not (ld-bool (fx+ re {{offsetof regengine_T failed}}))))))`
}

// javaStr is a C string function's Java body: the return of braaam/rt's
// Str method of that name, on the C's parameters.
func javaStr(call string) func(string) string {
	f := strings.Fields(call)
	return func(string) string {
		return "        return Str." + f[0] + "(" + strings.Join(f[1:], ", ") + ");\n"
	}
}

// cljStr is a C string function's Clojure body: the call of braaam/rt's
// Str method of that name, on the C's parameters.
func cljStr(call string) func(string) string {
	return func(string) string { return "(Str/" + call + ")" }
}

func matchLinesClj(string) string {
	return `(Rt/chunks n (reify whim.rt.Rt$Chunk
              (run [_ from to]
                (let [^S_regengine_S re (new-S_regengine_S)]
                  (match-chunk ed re rmp do-all buf lines line1 from to found)
                  (not (.failed re))))))`
}

const editorHeader = `// Code generated by go tool whim gen from editor.c; DO NOT EDIT.
//
// editor.go is editor.c -- the core of whim-vim.c, everything above its first
// #include -- in Go, written by a program: internal/gen reads editor.c with
// modernc.org/cc/v4, decides for every C pointer whether it is a plain *T or
// a Ptr[T] that walks, and writes the types, the globals, their initial
// values and every function's body to internal/gen/CONVENTIONS.md.  crt.go is the C
// runtime it is written against; host.go is the host the core calls.
//
// Regenerate with make editor/editor.go (go tool whim gen).

package editor

`

// Gen is what internal/gen is told about editor.c.
var Gen = togo.Profile{
	Header:  editorHeader,
	Package: "editor",
	// The editor as an instance: its state an Editor's fields, the
	// functions reaching it its methods, the receiver `ed`; the hand-written
	// host's state is editorHost, which Editor embeds.  What the hand-written
	// files declare is read from them when the generator runs (cmd/whim gen).
	Instance: &togo.Instance{Type: "Editor", Receiver: "ed", Init: "initGlobals", Embed: "editorHost"},
	// The Java editor (braaam/editor/) is package whim.editor, written as
	// Java writes a package: a file a class, the constants imported
	// statically (doc/JAVA-IDIOMS.md, item 11).
	JavaPackage: "whim.editor",
	JavaFiles:   true,
	// every method private but the host's and what braaam/Whim.java calls
	JavaPrivate: true,
	JavaGlue: []string{"vim_main", "deathtrap", "_", "emsg", "iemsg",
		"emsg_iobuff_room", "iobuff_or", "utfc_ptr2len", "utf_ptr2cells"},
	// vim's _(), the identity once gettext went; _ is Go's blank.
	Rename: map[string]string{"_": "gettext_"},
	// The C functions editor/crt.go replaces: their calls are translated,
	// their bodies are not (internal/gen/CONVENTIONS.md).
	Runtime: []string{
		"alloc", "alloc_clear", "lalloc", "lalloc_clear",
		"musl_memmove", "musl_memcpy", "musl_memset", "musl_memcmp",
		"ga_grow_inner", "match_lines",
		// the C string functions: editor/libc.go, on Go's byte functions
		"musl_strlen", "musl_strcpy", "musl_strncpy", "musl_strcat",
		"musl_strcmp", "musl_strncmp", "musl_strchr", "musl_strstr",
		"musl_strpbrk",
	},
	// ga_grow_inner()'s body is a rule of its own: it grows the storage
	// with GaGrowTo, in elements of the storage's type; match_lines()'s runs
	// the C's loop over lines in parallel chunks.
	RuntimeBodies: []togo.RuntimeBody{
		{Name: "ga_grow_inner", Body: gaGrowInnerFor, Java: gaGrowInnerJava, Clj: gaGrowInnerClj},
		{Name: "match_lines", Body: matchLinesGo, Java: matchLinesJava, Clj: matchLinesClj, Hs: matchLinesHs, Rs: matchLinesRs, Scm: matchLinesScm},
		// the C string functions in Java and Clojure: braaam/rt's Str, as the
		// Go's are editor/libc.go (Runtime above), not the musl translated a
		// byte and a BytePtr at a time (doc/CLOJURE-IDIOMS.md, item 6)
		{Name: "musl_strlen", Java: javaStr("strlen s"), Clj: cljStr("strlen s")},
		{Name: "musl_strcpy", Java: javaStr("strcpy dest src"), Clj: cljStr("strcpy dest src")},
		{Name: "musl_strncpy", Java: javaStr("strncpy dest src n"), Clj: cljStr("strncpy dest src n")},
		{Name: "musl_strcat", Java: javaStr("strcat dest src"), Clj: cljStr("strcat dest src")},
		{Name: "musl_strcmp", Java: javaStr("strcmp l r"), Clj: cljStr("strcmp l r")},
		{Name: "musl_strncmp", Java: javaStr("strncmp ls rs n"), Clj: cljStr("strncmp ls rs n")},
		{Name: "musl_strchr", Java: javaStr("strchr s c"), Clj: cljStr("strchr s c")},
		{Name: "musl_strstr", Java: javaStr("strstr h n"), Clj: cljStr("strstr h n")},
		{Name: "musl_strpbrk", Java: javaStr("strpbrk s b"), Clj: cljStr("strpbrk s b")},
	},
	// what the Clojure host's printf reads of the core's state
	CljExports: []string{"IObuff"},
	// caprice, the Haskell editor: its module, its host's, and what the host
	// calls back -- the printf's error messages and cells, and the death
	// of a SIGHUP or a SIGTERM (caprice/host)
	HsModule:  "Caprice.Editor",
	HsHost:    "Caprice.Host",
	HsExports: []string{"deathtrap", "emsg", "iemsg", "emsg_iobuff_room", "iobuff_or", "utfc_ptr2len", "utf_ptr2cells", "IObuff"},
	// whimsy, the Rust editor: what its host and printf call by name
	RsExports: []string{"vim_main", "deathtrap", "emsg", "iemsg", "emsg_iobuff_room", "iobuff_or", "utfc_ptr2len", "utf_ptr2cells"},
	// whimsical, the Scheme editor: its library, and what its host calls
	// back -- the printf's error messages and cells, the death of a SIGHUP
	// or a SIGTERM -- and runs (whimsical/whimsical/host.ss)
	ScmLibrary: "(whimsical editor)",
	ScmExports: []string{"vim_main", "deathtrap", "emsg", "iemsg", "emsg_iobuff_room", "iobuff_or", "utfc_ptr2len", "utf_ptr2cells", "IObuff"},
	// a Clojure state machine split into groups past 50,000 (the backend's
	// default is the JVM's method limit, 110,000): a group is called on
	// every jump between groups, so C1 compiles it early and cheaply, where
	// one method of ex_substitute's ran interpreted (doc/CLOJURE-PROFILE.md)
	CljSplit: 50000,
	// What vijure's glue (cljhost.clj, cljmain.clj) calls by name, the
	// editor first: they keep it though some need none.
	CljGlue: []string{"vim_main", "host_of", "deathtrap", "_", "emsg", "iemsg",
		"emsg_iobuff_room", "iobuff_or", "utfc_ptr2len", "utf_ptr2cells"},
	// The namespace's functions in 4 files it loads (clojure.core's own
	// split): load(), one method a file, had 62,441 of its 65,535 bytes
	// with every function in the one file, 24 bytes a function
	// (doc/CLOJURE-IDIOMS.md, item 8).
	CljParts: 4,
	// the functions in eight modules by the call graph: GHC holds a part,
	// not the whole (doc/HASKELL-IDIOMS.md, item 8)
	HsParts:    8,
	Allocators: allocators,
	// vim_free: the garbage collector owns memory, and what is freed walks
	// nothing.
	Frees: []string{"vim_free"},
	Bytes: togo.ByteFuncs{
		Move: []string{"musl_memmove", "musl_memcpy"},
		Set:  "musl_memset",
		Cmp:  "musl_memcmp",
	},
	// size_t, since the core named no libc type.
	SizeType: "usize",
	// garray_T's ga_data, GaData[T] in editor/crt.go.
	GrowArray: togo.GrowArray{Type: "garray_T", Data: "ga_data", MaxLen: "ga_maxlen"},
	// an option's variable, of whatever type.
	Puns: []string{"varp", "varp_arg"},
}
