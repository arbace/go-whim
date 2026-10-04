package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// The four replacement bodies are generated from the Python module's own
// constants: LALLOC's comment contains BACKTICKS, so a Go raw string cannot
// hold it and hand-escaping is the kind of transcription this port keeps
// avoiding.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const mfOpenNote = "// No caller can name a file: ml_open() passes nothing, and the recovery\n" +
	"// reader that passed a name went with the rest of recovery, above.  So\n" +
	"// there is no descriptor, no block is ever in a file, and the page size is\n" +
	"// ours to choose.\n"

const mfOpenBody = "    memfile_T           *mfp;\n" +
	"\n" +
	"    if ((mfp = (memfile_T *)alloc(sizeof(memfile_T))) == nullptr)\n" +
	"    {\n" +
	"        return nullptr;\n" +
	"    }\n" +
	"\n" +
	"    mfp->mf_free_first = nullptr;\n" +
	"    mfp->mf_used_first = nullptr;\n" +
	"    mfp->mf_used_last = nullptr;\n" +
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
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const mfSyncNote = "// Nothing to sync to.  Reporting the buffer clean is what the fd-less arm\n" +
	"// of this always did; it is now the whole function.\n"

const mfSyncBody = "    mfp->mf_dirty = MF_DIRTY_NO;\n" +
	"    return FAIL;"

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const mfGetMissNote = "// A block that is not in the hash is not anywhere: it could only\n" +
	"// ever have come back from the file, and there is no file.\n"

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const lallocNote = "// The scrollback is the only memory left to reclaim.  This used to be\n" +
	"// a retry loop, because mf_release_all() could page buffer blocks out\n" +
	"// to the swap file and free them; it cannot, so there is nothing to\n" +
	"// retry with.  `releasing` stays, because clear_sb_text() allocates.\n"

const lallocBody = "    p = malloc(size);\n" +
	"    if (p == nullptr && !releasing)\n" +
	"    {\n" +
	"        releasing = TRUE;\n" +
	"        clear_sb_text(TRUE);\n" +
	"        releasing = FALSE;\n" +
	"    }"

// NoMemfile takes the memfile's file away: there is no descriptor, no block is
// ever in a file, and the page size is ours to choose.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the bodies FRAG (BodyC, a
// loop's items by ReplaceC), mf_open's two parameters PARAM's, the
// statements cut by form and run, a block rebuilt, the folds by condition,
// the operands dropped; the old lengths and the mentions the text's on the
// C view.  mf_release's definition goes once its last caller has (the
// graph refuses it before; the text deleted it first).  PARAM drops the
// arguments at EVERY call: the text left ml_recover's `mf_open(fname_used,
// O_RDONLY)` passing two arguments to a function of none, which the sweep
// took with ml_recover; so before the collection the C view differs from
// the text's at that call alone (history keeps the text version).
func NoMemfile(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nomemfile", e, w)
	quiet := func(acts func(q *graph.Verbs)) {
		if v.Failed() {
			return
		}
		q := graph.NewVerbs("nomemfile", e, io.Discard)
		acts(q)
		if q.Err != nil {
			v.Die("%v", q.Err)
		}
	}
	body := func(name, src, tag string) {
		if v.Failed() {
			return
		}
		if e.Defn(name) == nil {
			v.Die("%s is not defined at file scope", name)
			return
		}
		was := 0
		v.InFunction(name, func(v *graph.Verbs) { was = bodyLines(v.Text()) })
		quiet(func(q *graph.Verbs) { q.BodyC(name, src, name) })
		if !v.Failed() {
			v.Sayf("%-16s was %3d lines -- %s", name, was, tag)
		}
	}
	body("mf_open", mfOpenBody, "no caller can name a file")
	quiet(func(q *graph.Verbs) {
		f := e.Defn("mf_open")
		q.DropParams([]graph.ParamDrop{{Decl: f, I: 0}, {Decl: f, I: 1}}, graph.ParamOptions{}, "mf_open's parameters")
	})
	body("mf_sync", mfSyncBody, "nothing to sync to")

	quiet(func(q *graph.Verbs) {
		q.Cut("(call mf_fullname (. (-> buf b_ml) ml_mfp))", 1, "mf_fullname's caller")
		q.InFunction("mf_close", func(q *graph.Verbs) {
			q.CutRun("mf_close's descriptor and unlink",
				"(if (>= (-> mfp mf_fd) 0) (block (if (< (call close (-> mfp mf_fd)) 0) (block (call emsg (call _ e_close_error_on_swap_file))))))",
				"(if (&& del_file (!= (-> mfp mf_fname) nullptr)) (block (call unlink (cast (ptr char) (paren (-> mfp mf_fname))))))")
			q.CutRun("mf_close's two names", "(call vim_free (-> mfp mf_fname))", "(call vim_free (-> mfp mf_ffname))")
		})
	})

	// buf_write matched the swap file's permissions and group to the file's.
	// mf_fname is NULL for ever, so the block never ran; it is the last
	// mention of mf_fd, and swap_mode exists only for it.
	quiet(func(q *graph.Verbs) {
		q.DropIf("(&& (> swap_mode 0) (!= (. (-> curbuf b_ml) ml_mfp) nullptr) (!= (-> (. (-> curbuf b_ml) ml_mfp) mf_fname) nullptr))", 1, "the swap file's permissions")
		q.Cut("(= swap_mode (| (paren (& (. st st_mode) 0644)) 0600))", 1, "swap_mode's one assignment")
	})
	if !v.Failed() {
		v.Say("buf_write stops matching a swap file's permissions")
	}

	// mf_get's cache miss: the block could only have come back from the file.
	quiet(func(q *graph.Verbs) {
		q.InFunction("mf_get", func(q *graph.Verbs) {
			miss := "(if (== hp nullptr) ?b _*)"
			var it []*graph.Node
			for _, n := range q.Find(miss) {
				ok := false
				q.In(n, func(q *graph.Verbs) { ok = q.Count("(|| (< nr 0) (>= nr (-> mfp mf_infile_count)))") == 1 })
				if ok {
					it = append(it, n)
				}
			}
			if len(it) != 1 {
				q.Die("mf_get's cache miss is not where this expects")
				return
			}
			blk := it[0].Kids[2]
			with, err := e.Build(blk, "(block (return nullptr))", nil)
			if err == nil {
				err = e.Replace(blk, with...)
			}
			if err != nil {
				q.Die("the cache miss -- %v", err)
			}
		})
	})
	if !v.Failed() {
		v.Say("mf_get stops trying to read a block back")
	}
	// mf_release reads p_mmt, which dropoptions --strict refuses later in this
	// phase with no sweep between; the rest of the file back-end is the
	// sweep's.
	quiet(func(q *graph.Verbs) {
		q.Rewrite("(= hp (call mf_release mfp page_count))", "(= hp nullptr)", 1, "mf_release's caller")
		q.DeleteDefinition("mf_release", "mf_release")
	})

	quiet(func(q *graph.Verbs) {
		q.InFunction("lalloc", func(q *graph.Verbs) {
			q.ReplaceC("(for () () () (block (if (!= (= p (call malloc size)) nullptr) (block (goto theend))) _*))",
				lallocBody, 1, "lalloc's retry loop")
			// The label was the loop's only exit; -Wunused-label is not a shape
			// the dead-code sweep deletes, so it is named here.
			q.Cut("(label theend)", 1, "lalloc's theend label")
		})
	})
	if !v.Failed() {
		v.Say("lalloc stops retrying: there is nothing to page out")
	}

	quiet(func(q *graph.Verbs) {
		q.InFunction("mf_ins_used", func(q *graph.Verbs) {
			q.CutRun("mf_ins_used's accounting", "(+= (-> mfp mf_used_count) (-> hp bh_page_count))",
				"(+= total_mem_used (* (cast long_u (-> hp bh_page_count)) (-> mfp mf_page_size)))")
		})
		q.InFunction("mf_rem_used", func(q *graph.Verbs) {
			q.CutRun("mf_rem_used's accounting", "(-= (-> mfp mf_used_count) (-> hp bh_page_count))",
				"(-= total_mem_used (* (cast long_u (-> hp bh_page_count)) (-> mfp mf_page_size)))")
		})
		q.Cut("(-= total_mem_used (* (cast long_u (-> hp bh_page_count)) (-> mfp mf_page_size)))", 1, "mf_close's accounting")
		// set_init_default_maxmemtot looks "maxmem" up by name, which
		// dropoptions --strict refuses later in this phase with no sweep
		// between; mch_total_mem is the sweep's.  Its call first: the
		// definition is refused while it is called.
		q.Cut("(call set_init_default_maxmemtot)", 1, "its call")
		q.DeleteDefinition("set_init_default_maxmemtot", "set_init_default_maxmemtot")
	})
	if !v.Failed() {
		v.Say("'maxmem' and 'maxmemtot' sized a cache that never " +
			"evicts; sysinfo and getrlimit go with them")
	}

	quiet(func(q *graph.Verbs) {
		q.CutRun("block zero's host name", "(call mch_get_host_name (-> b0p b0_hname) B0_HNAME_SIZE)",
			"(= (index (-> b0p b0_hname) (- B0_HNAME_SIZE 1)) NUL)")
	})
	if !v.Failed() {
		v.Say("the machine name in block zero; uname goes with it")
	}

	// check_overwrite's `is another vim editing this` warning, the last
	// reader of p_dir, went with :write at phase 1 (filefront, D4)

	// Stubbed rather than deleted, so ml_upd_block0()'s UB_SAME_DIR arm keeps
	// its shape.
	body("set_b0_dir_flag", "", "the swap file is nowhere, let alone beside the file")

	// SCOPED TO THE FUNCTION: FOR_ALL_BUFFERS's expansion appears dozens of
	// times; preserve_exit's first is the mf_fname one.
	quiet(func(q *graph.Verbs) {
		if e.Defn("preserve_exit") == nil {
			q.Die("preserve_exit is not defined at file scope")
			return
		}
		q.InFunction("preserve_exit", func(q *graph.Verbs) {
			loops := q.Find("(for (= (paren buf) firstbuf) _*)")
			if len(loops) == 0 {
				q.Die("preserve_exit has no FOR_ALL_BUFFERS loop")
				return
			}
			ok := false
			q.In(loops[0], func(q *graph.Verbs) { ok = q.Count("(-> _ mf_fname)") > 0 })
			if !ok {
				q.Die("preserve_exit loop is not the mf_fname one")
				return
			}
			q.In(loops[0], func(q *graph.Verbs) { q.Cut("(for (= (paren buf) firstbuf) _*)", 1, "the loop") })
		})
	})
	if !v.Failed() {
		v.Say("preserve_exit stops announcing what it cannot preserve")
	}

	quiet(func(q *graph.Verbs) {
		q.Rewrite("(|| (== varp (cast (ptr char_u) (addr p_dir))) ?x)", "?x", 1, "the '>' test")
		// on the seed p_path's term follows (phase 1, the reform's D5)
		q.DropOperand("(== p (cast (ptr char_u) (addr p_dir)))", 1, "the directory list test")
	})
	if v.Failed() {
		return v.Done()
	}
	v.Say("the two `is this option a directory list?` tests")

	for _, g := range []string{"mf_fd", "total_mem_used", "p_mmt"} {
		v.Sayf("%-14s %d mentions left for the sweep", g, v.TextCount(`\b`+g+`\b`))
	}
	return v.Done()
}
