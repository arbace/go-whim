package p031

// The multi-line literals internal/phase/031/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w31lit2 = "    else if (retval == OK && !read_stdin && !read_fifo)\n"
	w31lit3 = "    else if (retval == OK)\n"
	w31lit4 = "open_buffer(int read_stdin, exarg_T *eap, int flags_arg)\n"
	w31lit5 = "open_buffer(void)\n"
)
