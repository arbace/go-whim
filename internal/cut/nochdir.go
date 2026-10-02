package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
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

// nochdirBody replaces a definition's body and reports its old line count in
// the tool's own column.
func nochdirBody(text []byte, name, replacement, tag string, w io.Writer) ([]byte, error) {
	o, c, found, balanced := edit.Body(text, name)
	if !found || !balanced {
		return nil, fmt.Errorf("nochdir: %s is not defined at file scope", name)
	}
	fmt.Fprintf(w, "  nochdir      %-14s was %3d lines -- %s\n",
		name, bytes.Count(text[o:c], []byte{'\n'}), tag)
	out := make([]byte, 0, len(text))
	out = append(out, text[:o]...)
	out = append(out, "{\n"...)
	out = append(out, replacement...)
	out = append(out, "\n}"...)
	return append(out, text[c+1:]...), nil
}

// NoChdir stops anything moving this process between directories.
func NoChdir(text []byte, w io.Writer) ([]byte, error) {
	text, err := nochdirBody(text, "mch_FullName", nochdirFullName,
		"nothing moves this process", w)
	if err != nil {
		return nil, err
	}
	text, err = nochdirBody(text, "mch_dirname", nochdirDirname,
		"the answer cannot change", w)
	if err != nil {
		return nil, err
	}

	// The window- and tab-local directory restore in aucmd_restbuf() went
	// with the autocommand window's switch at phase 6 (whim68, phase 68's
	// program), and win_fix_current_dir()'s unconditional call with
	// win_enter_ext(), whose callers phases 72 and 73 take at phase 7: both
	// run before this phase now.

	// `globaldir` remembers the directory to come back to when a window-local
	// one is in force.  aucmd_prepbuf()/aucmd_restbuf() saved and restored it
	// around the autocommand window's switch, and both went with that switch
	// at phase 6 (whim68); the global and the function are the sweep's.

	// edit_buffers(), the -o window walk that returned to `cwd`, went with
	// the windows at phase 1 (nowindows, the reform's D9).  start_dir, which
	// nothing assigns, is left with its free, which goes here, and the global
	// is the sweep's.
	if text, err = cutCounted(text, edit.Line("vim_free(start_dir);"), "nochdir",
		"start_dir -- the free of it", 1); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  nochdir      start_dir's free, of a directory nothing ever names")

	for _, g := range []string{"mch_chdir", "fchdir", "getcwd"} {
		fmt.Fprintf(w, "  nochdir      %-10s %d mentions left for the sweep\n",
			g, len(regexp.MustCompile(`\b`+g+`\b`).FindAll(text, -1)))
	}
	return text, nil
}
