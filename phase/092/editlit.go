package p092

// The multi-line literals phase/092/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w92Arms = "    if (curbuf->b_ffname != NULL)\n    {\n        int old_msg_silent = msg_silent;\n        int perm;\n        perm = mch_getperm(curbuf->b_ffname);\n        if (perm >= 0 && (S_ISFIFO(perm) || S_ISSOCK(perm)))\n        {\n            read_fifo = TRUE;\n        }\n        if (shortmess(SHM_FILEINFO))\n        {\n            msg_silent = 1;\n        }\n        retval = readfile(curbuf->b_ffname, curbuf->b_fname, (linenr_T)0, (linenr_T)0, (linenr_T)LONG_MAX, eap, flags | READ_NEW | (read_fifo ? READ_FIFO : 0));\n        if (read_fifo)\n        {\n            if (retval == OK)\n            {\n                retval = read_buffer(FALSE, eap, flags);\n            }\n        }\n        msg_silent = old_msg_silent;\n        if (bt_help(curbuf))\n        {\n            fix_help_buffer();\n        }\n    }\n    else if (read_stdin)\n    {\n        retval = readfile(NULL, NULL, (linenr_T)0, (linenr_T)0, (linenr_T)LONG_MAX, NULL, flags | (READ_NEW + READ_STDIN));\n        if (retval == OK)\n        {\n            retval = read_buffer(TRUE, eap, flags);\n        }\n    }\n"
	w92lit1 = "    int read_fifo = FALSE;\n"
	w92lit2 = "    else if (retval == OK && !read_stdin && !read_fifo)\n"
	w92lit3 = "    else if (retval == OK)\n"
	w92lit4 = "open_buffer(int read_stdin, exarg_T *eap, int flags_arg)\n"
	w92lit5 = "open_buffer(void)\n"
	w92lit6 = "    int flags = flags_arg;\n"
)
