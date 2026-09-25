package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/dead"
)

// The four replacement bodies are generated from the Python module's own
// constants: LALLOC's comment contains BACKTICKS, so a Go raw string cannot
// hold it and hand-escaping is the kind of transcription this port keeps
// avoiding.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
const mfOpenNote = "// No caller can name a file: ml_open() passes nothing, and the recovery\n" +
	"// reader that passed a name went with the rest of recovery, above.  So\n" +
	"// there is no descriptor, no block is ever in a file, and the page size is\n" +
	"// ours to choose.\n"

const mfOpenBody = "    memfile_T           *mfp;\n" +
	"\n" +
	"    if ((mfp = (memfile_T *)alloc(sizeof(memfile_T))) == NULL)\n" +
	"    {\n" +
	"        return NULL;\n" +
	"    }\n" +
	"\n" +
	"    mfp->mf_free_first = NULL;\n" +
	"    mfp->mf_used_first = NULL;\n" +
	"    mfp->mf_used_last = NULL;\n" +
	"    mfp->mf_dirty = MF_DIRTY_NO;\n" +
	"    mf_hash_init(&mfp->mf_hash);\n" +
	"    mf_hash_init(&mfp->mf_trans);\n" +
	"    mfp->mf_page_size = MEMFILE_PAGE_SIZE;\n" +
	"    mfp->mf_blocknr_max = 0;\n" +
	"    mfp->mf_blocknr_min = -1;\n" +
	"    mfp->mf_neg_count = 0;\n" +
	"\n" +
	"    return mfp;"

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
const mfSyncNote = "// Nothing to sync to.  Reporting the buffer clean is what the fd-less arm\n" +
	"// of this always did; it is now the whole function.\n"

const mfSyncBody = "    mfp->mf_dirty = MF_DIRTY_NO;\n" +
	"    return FAIL;"

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
const mfGetMissNote = "// A block that is not in the hash is not anywhere: it could only\n" +
	"// ever have come back from the file, and there is no file.\n"

const mfGetMissBody = "            return NULL;"

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
const lallocNote = "// The scrollback is the only memory left to reclaim.  This used to be\n" +
	"// a retry loop, because mf_release_all() could page buffer blocks out\n" +
	"// to the swap file and free them; it cannot, so there is nothing to\n" +
	"// retry with.  `releasing` stays, because clear_sb_text() allocates.\n"

const lallocBody = "    p = malloc(size);\n" +
	"    if (p == NULL && !releasing)\n" +
	"    {\n" +
	"        releasing = TRUE;\n" +
	"        clear_sb_text(TRUE);\n" +
	"        releasing = FALSE;\n" +
	"    }"

// nomemfileBody replaces a definition's body and reports its old line count.
func nomemfileBody(text []byte, name, replacement, tag string, w io.Writer) ([]byte, error) {
	o, c, found, balanced := cutil.Body(text, name)
	if !found || !balanced {
		return nil, fmt.Errorf("nomemfile: %s is not defined at file scope", name)
	}
	was := bytes.Count(text[o:c], []byte{'\n'})
	fmt.Fprintf(w, "  nomemfile    %-16s was %3d lines -- %s\n", name, was, tag)
	out := make([]byte, 0, len(text))
	out = append(out, text[:o]...)
	out = append(out, "{\n"...)
	if replacement != "" {
		out = append(out, replacement...)
		out = append(out, '\n')
	}
	out = append(out, '}')
	return append(out, text[c+1:]...), nil
}

// NoMemfile takes the memfile's file away: there is no descriptor, no block is
// ever in a file, and the page size is ours to choose.
func NoMemfile(text []byte, w io.Writer) ([]byte, error) {
	for _, r := range []struct{ old, new string }{
		{"static memfile_T *mf_open(char_u *fname, int flags);", "static memfile_T *mf_open(void);"},
		{"mf_open(char_u *fname, int flags)", "mf_open(void)"},
		{"mfp = mf_open(NULL, 0);", "mfp = mf_open();"},
	} {
		text = bytes.Replace(text, []byte(r.old), []byte(r.new), 1)
	}
	text, err := nomemfileBody(text, "mf_open", mfOpenBody, "no caller can name a file", w)
	if err != nil {
		return nil, err
	}
	if text, err = nomemfileBody(text, "mf_sync", mfSyncBody, "nothing to sync to", w); err != nil {
		return nil, err
	}

	// mf_release reads p_mmt, which dropoptions --strict refuses later in this
	// phase with no sweep between; the rest of the file back-end is the sweep's.
	var ok bool
	if text, ok = cutil.DeleteDefinition(text, "mf_release"); !ok {
		return nil, fmt.Errorf("nomemfile: mf_release is not defined at file scope")
	}

	for _, c := range []struct{ pat, what string }{
		{cutil.Line("mf_fullname(buf->b_ml.ml_mfp);"), "mf_fullname's caller"},
		{cutil.Line("if (mfp->mf_fd >= 0)") +
			`[ \t]*\{\n` +
			`[ \t]*if \(close\(mfp->mf_fd\) < 0\)\n` +
			`[ \t]*\{\n` +
			`[ \t]*emsg\(_\(e_close_error_on_swap_file\)\);\n` +
			`[ \t]*\}\n` +
			`[ \t]*\}\n` +
			`[ \t]*if \(del_file && mfp->mf_fname != NULL\)\n` +
			`[ \t]*\{\n[ \t]* unlink\(\(char \*\)\(mfp->mf_fname\)\);\n[ \t]*\}\n`,
			"mf_close's descriptor and unlink"},
		{cutil.Line("vim_free(mfp->mf_fname);", "vim_free(mfp->mf_ffname);"),
			"mf_close's two names"},
	} {
		if text, err = cutCounted(text, c.pat, "nomemfile", c.what, 1); err != nil {
			return nil, err
		}
	}

	// buf_write matched the swap file's permissions and group to the file's.
	// mf_fname is NULL for ever, so the block never ran; it is the last
	// mention of mf_fd, and swap_mode exists only for it.
	if text, err = cutil.DropIf(text,
		`(?m)^[ \t]*if \(swap_mode > 0 && curbuf->b_ml\.ml_mfp != NULL `+
			`&& curbuf->b_ml\.ml_mfp->mf_fname != NULL\)$`, 1); err != nil {
		return nil, err
	}
	for _, c := range []struct{ pat, what string }{
		{cutil.Line("swap_mode = (st.st_mode & 0644) | 0600;"), "swap_mode's one assignment"},
	} {
		if text, err = cutCounted(text, c.pat, "nomemfile", c.what, 1); err != nil {
			return nil, err
		}
	}
	fmt.Fprintln(w, "  nomemfile    buf_write stops matching a swap file's permissions")
	text = bytes.Replace(text, []byte("    hp = mf_release(mfp, page_count);\n"),
		[]byte("    hp = NULL;\n"), 1)

	// mf_get's cache miss: the block could only have come back from the file.
	blanked := cutil.Blank(text)
	k := bytes.Index(text, []byte("        if (nr < 0 || nr >= mfp->mf_infile_count)"))
	if k < 0 {
		return nil, fmt.Errorf("nomemfile: mf_get's cache miss is not where this expects")
	}
	at := k - 400 + bytes.Index(text[k-400:], []byte("if (hp == NULL)"))
	o := at + bytes.IndexByte(blanked[at:], '{')
	c := cutil.Match(blanked, o)
	if c < 0 {
		return nil, fmt.Errorf("nomemfile: mf_get's cache miss is unbalanced")
	}
	var buf []byte
	buf = append(buf, text[:o]...)
	buf = append(buf, "{\n"...)
	buf = append(buf, mfGetMissBody...)
	buf = append(buf, "\n        }"...)
	text = append(buf, text[c+1:]...)
	fmt.Fprintln(w, "  nomemfile    mf_get stops trying to read a block back")

	blanked = cutil.Blank(text)
	k = bytes.Index(text, []byte("    for (;;)\n    {\n        if ((p = malloc(size)) != NULL)"))
	if k < 0 {
		return nil, fmt.Errorf("nomemfile: lalloc's retry loop is not where this expects")
	}
	o = k + 12 + bytes.IndexByte(blanked[k+12:], '{')
	c = cutil.Match(blanked, o)
	if c < 0 {
		return nil, fmt.Errorf("nomemfile: lalloc's retry loop is unbalanced")
	}
	end := c + bytes.IndexByte(text[c:], '\n') + 1
	buf = nil
	buf = append(buf, text[:k]...)
	buf = append(buf, lallocBody...)
	buf = append(buf, '\n')
	text = append(buf, text[end:]...)
	for _, c := range []struct{ pat, what string }{
		// The label was the loop's only exit; -Wunused-label is not a shape
		// the dead-code sweep deletes, so it is named here.
		{`(?m)^theend:\n`, "lalloc's theend label"},
	} {
		if text, err = cutCounted(text, c.pat, "nomemfile", c.what, 1); err != nil {
			return nil, err
		}
	}
	fmt.Fprintln(w, "  nomemfile    lalloc stops retrying: there is nothing to page out")

	for _, c := range []struct{ pat, what string }{
		{cutil.Line("mfp->mf_used_count += hp->bh_page_count;") +
			`[ \t]*total_mem_used \+= \(long_u\)hp->bh_page_count \* mfp->mf_page_size;\n`,
			"mf_ins_used's accounting"},
		{cutil.Line("mfp->mf_used_count -= hp->bh_page_count;") +
			`[ \t]*total_mem_used -= \(long_u\)hp->bh_page_count \* mfp->mf_page_size;\n`,
			"mf_rem_used's accounting"},
		{cutil.Line("total_mem_used -= (long_u)hp->bh_page_count * mfp->mf_page_size;"),
			"mf_close's accounting"},
	} {
		if text, err = cutCounted(text, c.pat, "nomemfile", c.what, 1); err != nil {
			return nil, err
		}
	}
	// set_init_default_maxmemtot looks "maxmem" up by name, which dropoptions
	// --strict refuses later in this phase with no sweep between; mch_total_mem
	// is the sweep's.
	if text, ok = cutil.DeleteDefinition(text, "set_init_default_maxmemtot"); !ok {
		return nil, fmt.Errorf("nomemfile: set_init_default_maxmemtot is not defined at file scope")
	}
	if text, err = cutCounted(text, cutil.Line("set_init_default_maxmemtot();"),
		"nomemfile", "its call", 1); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  nomemfile    'maxmem' and 'maxmemtot' sized a cache that never "+
		"evicts; sysinfo and getrlimit go with them")

	if text, err = cutCounted(text,
		cutil.Line("mch_get_host_name(b0p->b0_hname, B0_HNAME_SIZE);")+
			`[ \t]*b0p->b0_hname\[B0_HNAME_SIZE - 1\] = NUL;\n`,
		"nomemfile", "block zero's host name", 1); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  nomemfile    the machine name in block zero; uname goes with it")

	if text, err = cutil.DropIf(text, cutil.Head("if (other && !emsg_silent)"), 1); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  nomemfile    check_overwrite's `is another vim editing this` "+
		"warning, the last reader of p_dir")

	// Stubbed rather than deleted, so ml_upd_block0()'s UB_SAME_DIR arm keeps
	// its shape.
	if text, err = nomemfileBody(text, "set_b0_dir_flag", "",
		"the swap file is nowhere, let alone beside the file", w); err != nil {
		return nil, err
	}

	// SCOPED TO THE FUNCTION: `for ((buf) = firstbuf; ...)` is the expansion of
	// FOR_ALL_BUFFERS and appears dozens of times, so an unanchored cut takes
	// the first one in the file -- which is somebody else's loop entirely.
	blanked = cutil.Blank(text)
	span, found := dead.FuncDefinitions(text, blanked)["preserve_exit"]
	if !found {
		return nil, fmt.Errorf("nomemfile: preserve_exit is not defined at file scope")
	}
	fn := text[span[0]:span[1]]
	fk := bytes.Index(fn, []byte(" for ((buf) = firstbuf;"))
	if fk < 0 {
		return nil, fmt.Errorf("nomemfile: preserve_exit has no FOR_ALL_BUFFERS loop")
	}
	fk = bytes.LastIndexByte(fn[:fk], '\n') + 1
	fb := cutil.Blank(fn)
	fo := fk + bytes.IndexByte(fb[fk:], '{')
	fc := cutil.Match(fb, fo)
	if fc < 0 {
		return nil, fmt.Errorf("nomemfile: preserve_exit's loop is unbalanced")
	}
	fend := fc + bytes.IndexByte(fn[fc:], '\n') + 1
	if !bytes.Contains(fn[fk:fend], []byte("mf_fname")) {
		return nil, fmt.Errorf("nomemfile: preserve_exit loop is not the mf_fname one")
	}
	buf = nil
	buf = append(buf, text[:span[0]]...)
	buf = append(buf, fn[:fk]...)
	buf = append(buf, fn[fend:]...)
	text = append(buf, text[span[1]:]...)
	fmt.Fprintln(w, "  nomemfile    preserve_exit stops announcing what it cannot preserve")

	for _, r := range []struct{ old, new string }{
		{"else if (*arg == '>' && (varp == (char_u *)&p_dir || varp == (char_u *)&p_bdir))",
			"else if (*arg == '>' && varp == (char_u *)&p_bdir)"},
		{"if (p == (char_u *)&p_bdir || p == (char_u *)&p_dir || p == (char_u *)&p_pp",
			"if (p == (char_u *)&p_bdir || p == (char_u *)&p_pp"},
	} {
		text = bytes.Replace(text, []byte(r.old), []byte(r.new), 1)
	}
	fmt.Fprintln(w, "  nomemfile    the two `is this option a directory list?` tests")

	for _, g := range []string{"mf_fd", "total_mem_used", "p_mmt"} {
		fmt.Fprintf(w, "  nomemfile    %-14s %d mentions left for the sweep\n",
			g, len(regexp.MustCompile(`\b`+g+`\b`).FindAll(text, -1)))
	}
	return text, nil
}
