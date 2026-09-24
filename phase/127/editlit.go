package p127

// Every block and every single line of C phase/127/edit.go writes into the
// tree, in source order, EXTRACTED from the heredoc by an AST walk rather than
// retyped.  Several are 200-column musl_memmove() calls written as adjacent
// Python literals, where a changed byte would be invisible to the compile and
// to a filtered read alike.
//
// One row is COMPUTED and not a literal -- `'enum { DB_LINE_MAX = %d };' %
// DB_LINE_MAX` -- and the generator evaluates it with the phase's own 64, so
// the constant is still stated once.
var (
	w127b0 = []string{
		"typedef struct data_line        DATA_LN;",
	}
	w127b1 = []string{
		"enum { DB_LINE_MAX = 64 };",
		"",
		"struct data_line",
		"{",
		"    char_u      *dl_text;",
		"    colnr_T     dl_len;",
		"    char        dl_marked;",
		"};",
		"",
		"struct data_block",
		"{",
		"    short_u     db_id;",
		"    linenr_T    db_line_count;",
		"    DATA_LN     db_line[DB_LINE_MAX];",
		"};",
		"",
		"static_assert(sizeof(DATA_BL) <= MEMFILE_PAGE_SIZE, \"a leaf is one memfile page\");",
	}
	w127b2 = []string{
		"    static char_u *",
		"ml_alloc_line(char_u *line, colnr_T len)",
		"{",
		"    char_u      *text;",
		"",
		"    text = alloc((usize)len);",
		"    if (text != nullptr)",
		"    {",
		"         musl_memmove((char *)(text), (char *)(line), (usize)(len)) ;",
		"    }",
		"",
		"    return text;",
		"}",
		"",
	}
	w127b3 = []string{
		"    dp->db_line[0].dl_text = ml_alloc_line((char_u *)\"\", 1);",
		"    if (dp->db_line[0].dl_text == nullptr)",
		"    {",
		"        goto error;",
		"    }",
		"    dp->db_line[0].dl_len = 1;",
		"    dp->db_line_count = 1;",
	}
	w127b4 = []string{
		"        idx = lnum - buf->b_ml.ml_locked_low;",
		"",
		"        buf->b_ml.ml_line_ptr = dp->db_line[idx].dl_text;",
		"        buf->b_ml.ml_line_len = dp->db_line[idx].dl_len;",
	}
	w127b5 = []string{
		"    char_u      *text;",
	}
	w127b6 = []string{
		"    text = ml_alloc_line(line, len);",
		"    if (text == nullptr)",
		"    {",
		"        goto theend;",
		"    }",
	}
	w127b7 = []string{
		"    if (dp->db_line_count < DB_LINE_MAX)",
		"    {",
		"        if (line_count > db_idx + 1)",
		"        {",
		"             musl_memmove((char *)(&dp->db_line[db_idx + 2]), (char *)(&dp->db_line[db_idx + 1]), (usize)(line_count - db_idx - 1) * sizeof(DATA_LN)) ;",
		"        }",
		"        dp->db_line[db_idx + 1].dl_text = text;",
		"        dp->db_line[db_idx + 1].dl_len = len;",
		"        dp->db_line[db_idx + 1].dl_marked = FALSE;",
		"        ++(dp->db_line_count);",
	}
	w127b8 = []string{
		"            lines_moved = line_count - db_idx - 1;",
		"            in_left = (lines_moved != 0);",
		"        }",
		"",
	}
	w127b9 = []string{
		"        if (!in_left)",
		"        {",
		"            dp_right->db_line[0].dl_text = text;",
		"            dp_right->db_line[0].dl_len = len;",
		"            dp_right->db_line[0].dl_marked = FALSE;",
		"            ++line_count_right;",
		"        }",
		"        if (lines_moved)",
		"        {",
		"             musl_memmove((char *)(&dp_right->db_line[line_count_right]), (char *)(&dp->db_line[db_idx + 1]), (usize)(lines_moved) * sizeof(DATA_LN)) ;",
		"            line_count_right += lines_moved;",
		"            line_count_left -= lines_moved;",
		"        }",
		"",
		"        if (in_left)",
		"        {",
		"            dp_left->db_line[line_count_left].dl_text = text;",
		"            dp_left->db_line[line_count_left].dl_len = len;",
		"            dp_left->db_line[line_count_left].dl_marked = FALSE;",
		"            ++line_count_left;",
		"        }",
	}
	w127b10 = []string{
		"        if (idx < count - 1)",
		"        {",
		"             musl_memmove((char *)(&dp->db_line[idx]), (char *)(&dp->db_line[idx + 1]), (usize)(count - idx - 1) * sizeof(DATA_LN)) ;",
		"        }",
		"        --(dp->db_line_count);",
	}
	w127b11 = []string{
		"            if (dp->db_line[i].dl_marked)",
		"            {",
		"                dp->db_line[i].dl_marked = FALSE;",
	}
	w127b12 = []string{
		"            idx = lnum - buf->b_ml.ml_locked_low;",
		"",
		"            dp->db_line[idx].dl_text = new_line;",
		"            dp->db_line[idx].dl_len = buf->b_ml.ml_line_len;",
	}
)

const (
	w127s0 = "    if (dp->db_line_count >= DB_LINE_MAX && db_idx == line_count - 1 && lnum < buf->b_ml.ml_line_count)"
	w127s1 = "    dp->db_line[lnum - curbuf->b_ml.ml_locked_low].dl_marked = TRUE;"
)
