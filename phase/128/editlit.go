package p128

// Every block and every single line of C phase/128/edit.go writes into the
// tree, in source order, EXTRACTED from the heredoc by an AST walk rather than
// retyped.
//
// TWO ROWS CARRY A PLACEHOLDER and are formatted by the Go: the two block ids,
// `(('p' << 8) + 't')` and its twin, are CARRIED Out of the definitions being
// replaced and never spelled -- they are phase 92's macro expansion, and an
// edit that wrote them Out would be quoting somebody else's text.  The
// placeholders are the fmt verbs %[1]s (the data id) and %[2]s (the pointer
// id), and the three blocks that carry one are formatted before they are spliced.
//
// ONE ROW KEEPS ITS %d: the page size is read off the INPUT, so the row that
// asserts the fanout is formatted with it rather than with the 4096 this
// generator happened to see.  The "4096-byte" inside the message is the
// Python's own literal text and is not the same number twice.
//
// One assignment is left Out because every row of it is computed from a match --
// `'%shp = %s;' % (m.group(1), m.group(2))` in step 15 -- and the Go writes that
// one with fmt, as the Python does.
var (
	w128b0 = []string{
		"struct block_hdr",
		"{",
		"    short_u     bh_id;",
		"};",
	}
	w128b1 = []string{
		"enum { PB_COUNT_MAX = 255 };",
		"",
		"struct pointer_block",
		"{",
		"    bhdr_T      pb_hdr;",
		"    short_u     pb_count;",
		"    PTR_EN      pb_pointer[PB_COUNT_MAX];",
		"};",
	}
	w128b2 = []string{
		"static_assert(sizeof(PTR_EN) == 16, \"a pointer entry is one node reference and one line count\");",
		"static_assert(PB_COUNT_MAX == (%d - 8) / sizeof(PTR_EN), \"the fanout the 4096-byte page gave, kept when the page went\");",
		"static_assert(sizeof(DATA_BL) == 16 + DB_LINE_MAX * sizeof(DATA_LN), \"a leaf is its tag, its count and its records\");",
	}
	w128b3 = []string{
		"    static bhdr_T *",
		"ml_new_data(void)",
		"{",
		"    DATA_BL     *dp;",
		"",
		"    dp =  (DATA_BL *)alloc_clear(sizeof(DATA_BL)) ;",
		"    if (dp == nullptr)",
		"    {",
		"        return nullptr;",
		"    }",
		"",
		"    dp->db_hdr.bh_id = %[1]s;",
		"    dp->db_line_count = 0;",
		"",
		"    return (bhdr_T *)dp;",
		"}",
	}
	w128b4 = []string{
		"    static bhdr_T *",
		"ml_new_ptr(void)",
		"{",
		"    PTR_BL      *pp;",
		"",
		"    pp =  (PTR_BL *)alloc_clear(sizeof(PTR_BL)) ;",
		"    if (pp == nullptr)",
		"    {",
		"        return nullptr;",
		"    }",
		"",
		"    pp->pb_hdr.bh_id = %[2]s;",
		"    pp->pb_count = 0;",
		"",
		"    return (bhdr_T *)pp;",
		"}",
	}
	w128b5 = []string{
		"    static void",
		"ml_free_tree(bhdr_T *hp)",
		"{",
		"    PTR_BL      *pp;",
		"    int         i;",
		"",
		"    if (hp == nullptr)",
		"    {",
		"        return;",
		"    }",
		"    if (hp->bh_id == %[2]s)",
		"    {",
		"        pp = (PTR_BL *)(hp);",
		"        for (i = 0; i < (int)pp->pb_count; ++i)",
		"        {",
		"            ml_free_tree(pp->pb_pointer[i].pe_block);",
		"        }",
		"    }",
		"    vim_free(hp);",
		"}",
		"",
	}
	w128b6 = []string{
		"error:",
		"    ml_free_tree(buf->b_ml.ml_root);",
		"    buf->b_ml.ml_root = nullptr;",
	}
	w128b7 = []string{
		"    if (buf->b_ml.ml_root == nullptr)",
		"    {",
		"        return FAIL;",
		"    }",
	}
	w128b8 = []string{
		"                pp_new->pb_count = pp->pb_count;",
		"                 musl_memmove((char *)(&pp_new->pb_pointer[0]), (char *)(&pp->pb_pointer[0]), (usize)pp->pb_count * sizeof(PTR_EN)) ;",
	}
)

const (
	w128s0  = "    bhdr_T      db_hdr;"
	w128s1  = "static bhdr_T *ml_new_data(void);"
	w128s2  = "static bhdr_T *ml_new_ptr(void);"
	w128s3  = "    if ((hp = ml_new_ptr()) == nullptr)"
	w128s4  = "    pp = (PTR_BL *)(hp);"
	w128s5  = "    if ((hp = ml_new_data()) == nullptr)"
	w128s6  = "    dp = (DATA_BL *)(hp);"
	w128s7  = "    if (buf->b_ml.ml_root == nullptr)"
	w128s8  = "    ml_free_tree(buf->b_ml.ml_root);"
	w128s9  = "    buf->b_ml.ml_root = nullptr;"
	w128s10 = "        if ((hp_new = ml_new_data()) == nullptr)"
	w128s11 = "                hp_new = ml_new_ptr();"
)
