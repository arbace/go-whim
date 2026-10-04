package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// envCopy is expand_env_esc with the $ arm gone.  Written out rather than cut,
// because what survives is the loop's tail and it reads better as its own
// function than as a `copy_char` flag that is now always true.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const envNote = `// $VAR is part of a name, not a place to look one up.  There is no
// environment to ask, so what is left of this is the escape handling and
// the bound on dstlen: a name reaches its caller as it was written.
`

const envCopy = `    char_u      *src;
    char_u      *dst_start = dst;

    src = skipwhite(srcp);
    --dstlen;
    while (*src && dstlen > 0)
    {
        if (src[0] == '\\' && src[1] != NUL)
        {
            *dst++ = *src++;
            --dstlen;
        }
        if (dstlen > 0)
        {
            *dst++ = *src++;
            --dstlen;
        }
    }
    *dst = NUL;

    return (usize)(dst - dst_start);`

var envLeft = regexp.MustCompile(`\bgetenv\b|\bsetenv\b|\benviron\b`)

// NoGetEnv takes the environment away: nothing the editor does is decided by a
// variable any more.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): expand_env_esc's body the
// text's literal made nodes (FRAG: BodyC), two calls cut, two ifs dropped and
// the $COLORFGBG operand dropped as the text's cut left its neighbour; the
// lengths and the mentions left are the text's numbers, on the C view
// (history keeps the text version).
func NoGetEnv(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nogetenv", e, w)
	if e.Defn("expand_env_esc") == nil {
		return fmt.Errorf("nogetenv: expand_env_esc is not defined at file scope")
	}
	was := 0
	v.InFunction("expand_env_esc", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	v.BodyC("expand_env_esc", envCopy, fmt.Sprintf("expand_env_esc was %d lines, and now copies a name", was))

	// expand_shellcmd(), which looked a command name up in $PATH, went with
	// every completion context but files at phase 4 (whim4f, phase 4f's
	// program, which runs before this phase now).

	// The local-additions scan went with fix_help_buffer, open_buffer's read
	// arm its one caller, at phase 1 (readfront, phase 31's move).

	for _, d := range []struct{ name, call string }{
		{"set_init_default_shell", "$SHELL for 'shell'"},
		{"set_init_default_cdpath", "$CDPATH for 'cdpath'"},
	} {
		v.Cut("(call "+d.name+")", 1, d.call)
	}

	v.DropIf(`(!= (cast (ptr char_u) (call getenv (cast (ptr char) (paren (cast (ptr char_u) "VIM_POSIX"))))) nullptr)`, 1,
		"$VIM_POSIX, which chose a stricter 'cpoptions'")

	// 'backupskip' was $TMPDIR, $TEMP, $TMP and always /tmp: its default,
	// set_init_default_backupskip(), went with the backup at phase 5
	// (nobackup, phase 12's cut, which runs before this phase now).

	// vimrc_found() and do_source_ext()'s two DOSO_VIMRC arms, which kept
	// vim_setenv, export_myvimdir and $MYVIMDIR alive, died with :source,
	// retired at phase 1, and the startup scripts (exfront, the reform's D2)
	v.DropIf("(== varp (addr p_rtp))", 1, "$VIM, $VIMRUNTIME and $MYVIMDIR stop being published")

	// did_set_helpfile went with 'helpfile''s row, dropped at phase 1
	// (optfront, the reform's D3)

	// $VAR completion -- the EXPAND_ENV_VARS row and its context, the one
	// mention of environ -- went with every completion context but files at
	// phase 4 (whim4f); `:command -complete=environment`'s table row went
	// with `:command` at phase 3 (noucmd, the reform's D10).

	v.DropOperandAsText(`(paren (&& (!= (= p (cast (ptr char_u) (call getenv (cast (ptr char) (paren (cast (ptr char_u) "COLORFGBG")))))) nullptr) _*))`, 1,
		"$COLORFGBG; 'background' is what the table says")

	// vim_localtime(), whose tzset cache read $TZ, went with get_ctime() and
	// add_time() once the swap file's messages went at phase 1 (noswap and
	// norecover, the reform's D5).

	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d environment mentions left for the sweep", len(envLeft.FindAll(v.Text(), -1)))
	return v.Done()
}
