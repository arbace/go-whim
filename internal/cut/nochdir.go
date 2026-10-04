package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// nochdirFullNameNote is what stands above mch_FullName.
//
// Written as an interpreted string and not a raw one because it CONTAINS
// BACKTICKS, in the comment it carries into the C.  Generated from the
// Python's own constant rather than retyped, so the two cannot drift.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const nochdirFullNameNote = "// The dance that used to be here chdir'd into the leading directory of a\n" +
	"// relative name, asked getcwd() where that landed, and chdir'd back -- so\n" +
	"// that `..` and a symlinked directory were resolved on the way.  Nothing\n" +
	"// moves this process any more, so a full name is the working directory\n" +
	"// with the name appended, and a `..` in it survives into the answer.\n" +
	"//\n" +
	"// `force` asked for that re-resolution even when the name was already\n" +
	"// absolute.  There is nothing left to re-resolve, so an absolute name is\n" +
	"// its own answer -- and prepending the cwd to one was the whole of the\n" +
	"// first attempt at this, which moved :read, :write and :wq.\n"

// nochdirFullName is mch_FullName with the chdir dance gone.
const nochdirFullName = "    int buflen = 0;\n" +
	"\n" +
	"    if (!mch_isFullName(fname))\n" +
	"    {\n" +
	"        if (mch_dirname(buf, len) == FAIL)\n" +
	"        {\n" +
	"            *buf = NUL;\n" +
	"            return FAIL;\n" +
	"        }\n" +
	"        buflen = (int)strlen((char *)(buf));\n" +
	"        if (buflen >= len - 1)\n" +
	"        {\n" +
	"            return FAIL;\n" +
	"        }\n" +
	"        if (buflen > 0 && buf[buflen - 1] !=  ((char_u)'/')  && *fname != NUL &&  strcmp((char *)(fname), (char *)(\".\"))  != 0)\n" +
	"        {\n" +
	"             strcpy((char *)(buf + buflen), (char *)( \"/\" )) ;\n" +
	"            buflen += sizeof( ((char_u)'/') );\n" +
	"        }\n" +
	"    }\n" +
	"    else\n" +
	"    {\n" +
	"        *buf = NUL;\n" +
	"    }\n" +
	"\n" +
	"    if ((int)(buflen +  strlen((char *)(fname)) ) >= len)\n" +
	"    {\n" +
	"        return FAIL;\n" +
	"    }\n" +
	"\n" +
	"    if (strcmp((char *)(fname), (char *)(\".\")) != 0)\n" +
	"    {\n" +
	"        strcpy((char *)(buf + buflen), (char *)(fname));\n" +
	"    }\n" +
	"\n" +
	"    return OK;"

// nochdirDirname is mch_dirname, asked once.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const nochdirDirnameNote = "// Asked once.  Nothing can move this process -- :cd, :lcd and :tcd are\n" +
	"// ex_ni, :! does not fork, and mch_FullName() no longer chdirs -- so every\n" +
	"// later call is asking the kernel a question whose answer cannot have\n" +
	"// changed since the first one.\n"

const nochdirDirname = "    static char_u   cwd[ PATH_MAX ];\n" +
	"    static int      cwd_len = -1;\n" +
	"\n" +
	"    if (cwd_len < 0)\n" +
	"    {\n" +
	"        if (getcwd((char *)cwd, sizeof(cwd)) == nullptr)\n" +
	"        {\n" +
	"             strcpy((char *)(buf), (char *)(strerror(errno))) ;\n" +
	"            return FAIL;\n" +
	"        }\n" +
	"        cwd_len = (int) strlen((char *)(cwd)) ;\n" +
	"    }\n" +
	"    if (cwd_len >= len)\n" +
	"    {\n" +
	"        return FAIL;\n" +
	"    }\n" +
	"     strcpy((char *)(buf), (char *)(cwd)) ;\n" +
	"    return OK;"

// NoChdir stops anything moving this process between directories.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the two bodies are the text's
// literals made nodes in one unit (FRAG: BodyC, Together) -- their static
// locals, getcwd, errno and strerror resolved as the importer resolves them
// -- the free cut as an item; the lengths and the mentions left are the
// text's numbers, on the C view (history keeps the text version).
func NoChdir(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nochdir", e, w)
	bodies := []struct{ name, body, tag string }{
		{"mch_FullName", nochdirFullName, "nothing moves this process"},
		{"mch_dirname", nochdirDirname, "the answer cannot change"},
	}
	v.Together(func(v *graph.Verbs) {
		for _, b := range bodies {
			if e.Defn(b.name) == nil {
				v.Die("%s is not defined at file scope", b.name)
				return
			}
			was := 0
			v.InFunction(b.name, func(v *graph.Verbs) { was = bodyLines(v.Text()) })
			v.BodyC(b.name, b.body, fmt.Sprintf("%-14s was %3d lines -- %s", b.name, was, b.tag))
		}
	})

	// The window- and tab-local directory restore in aucmd_restbuf() went
	// with the autocommand window's switch at phase 4 (whim4c, phase 4c's
	// program), and win_fix_current_dir()'s unconditional call with
	// win_enter_ext(), whose callers phases 5b and 5c take at phase 5: both
	// run before this phase now.

	// `globaldir` remembers the directory to come back to when a window-local
	// one is in force.  aucmd_prepbuf()/aucmd_restbuf() saved and restored it
	// around the autocommand window's switch, and both went with that switch
	// at phase 4 (whim4c); the global and the function are the collection's.

	// edit_buffers(), the -o window walk that returned to `cwd`, went with
	// the windows at phase 3 (nowindows, the reform's D9).  start_dir, which
	// nothing assigns, is left with its free, which goes here, and the global
	// is the collection's.
	v.Cut("(call vim_free start_dir)", 1, "start_dir's free, of a directory nothing ever names")
	if v.Failed() {
		return v.Done()
	}
	text := v.Text()
	for _, g := range []string{"mch_chdir", "fchdir", "getcwd"} {
		v.Sayf("%-10s %d mentions left for the sweep", g, len(regexp.MustCompile(`\b`+g+`\b`).FindAll(text, -1)))
	}
	return v.Done()
}
