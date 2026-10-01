package p092

// The multi-line literals internal/phase/092/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w92lit2 = "    else if (retval == OK && !read_stdin && !read_fifo)\n"
	w92lit3 = "    else if (retval == OK)\n"
	w92lit4 = "open_buffer(int read_stdin, exarg_T *eap, int flags_arg)\n"
	w92lit5 = "open_buffer(void)\n"
)
