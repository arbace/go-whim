package p094

// The multi-line literals internal/phase/094/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w94lit1 = "    int save_exiting = exiting;\n    exiting = TRUE;\n    getout(0);\n    not_exiting(save_exiting);\n"
	w94lit2 = "    getout(0);\n"
	w94lit4 = "    wp->w_topline_was_set = true;\n"
	w94lit5 = "    int wi_changelistidx;\n"
	w94lit6 = "        wip->wi_changelistidx = win->w_changelistidx;\n"
)
