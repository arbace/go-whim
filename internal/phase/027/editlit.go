package p027

// The multi-line literals internal/phase/027/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w27lit3  = "    {'Q', nv_exmode, NV_NCW, 0},\n"
	w27lit4  = "    {'Q', nv_error, NV_NCW, 0},\n"
	w27lit5  = "    case 'Q':\n        if (!check_text_locked(cap->oap) && !checkclearopq(oap))\n        {\n            do_exmode(TRUE);\n        }\n        break;\n"
	w27lit6  = "                exmode_active = EXMODE_NORMAL;\n"
	w27lit7  = "                exmode_active = EXMODE_VIM;\n"
	w27lit8  = "                if (exmode_active)\n                {\n                    silent_mode = TRUE;\n                }\n                else\n                {\n                    mainerr(ME_UNKNOWN_OPTION, (char_u *)argv[0]);\n                }\n"
	w27lit9  = "                exmode_active = 0;\n"
	w27lit12 = " || exmode_active"
	w27lit13 = "\n    check_tty();\n"
	w27lit14 = "\n    int save_silent = silent_mode;\n"
	w27lit15 = "\n    silent_mode = FALSE;\n"
	w27lit16 = "\n    silent_mode = save_silent;\n"
	w27lit18 = "\n                ex_pressedreturn = TRUE;\n"
	w27lit20 = "\n    ex_no_reprint = TRUE;\n"
	w27lit21 = "\n        ex_no_reprint = TRUE;\n"
	w27lit23 = "\n        ex_exitval = 1;\n"
	w27lit25 = "\n            previous_got_int = TRUE;\n"
	w27lit26 = "        else\n        {\n            previous_got_int = FALSE;\n        }\n"
	w27lit28 = "\nstatic void main_loop(int cmdwin, int noexmode);\n"
	w27lit29 = "\nstatic void main_loop(int cmdwin);\n"
	w27lit30 = "main_loop(int cmdwin, int noexmode)\n"
	w27lit31 = "main_loop(int cmdwin)\n"
	w27lit32 = "\n    main_loop(FALSE, FALSE);\n"
	w27lit33 = "\n    main_loop(FALSE);\n"
	w27lit34 = "\ntheend:\n    current_oap = prev_oap;\n"
	w27lit35 = "\n    current_oap = prev_oap;\n"
)
