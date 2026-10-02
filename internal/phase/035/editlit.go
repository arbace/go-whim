package p035

// The multi-line literals internal/phase/035/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w35lit1  = "    if ((!(State & (MODE_INSERT | MODE_CMDLINE)) || arrow_used) && scriptin[curscript] == nullptr)\n"
	w35lit2  = "    if (!(State & (MODE_INSERT | MODE_CMDLINE)) || arrow_used)\n"
	w35lit3  = " && scriptin[curscript] == nullptr"
	w35lit4  = "    script_char = -1;\n    while (scriptin[curscript] != nullptr && script_char < 0)\n    {\n        if (got_int || (script_char = getc(scriptin[curscript])) < 0)\n        {\n            closescript();\n            if (got_int)\n            {\n                retesc = TRUE;\n            }\n            else\n            {\n                return -1;\n            }\n        }\n        else\n        {\n            buf[0] = script_char;\n            len = 1;\n        }\n    }\n"
	w35lit5  = "            return retesc;\n"
	w35lit6  = "            return FALSE;\n"
	w35lit9  = "                    redir_write(p, -1);\n"
	w35lit10 = "                redir_write((char_u *)s, -1);\n"
	w35lit11 = "    redir_write((char_u *)str, maxlen);\n"
	w35lit14 = "        did_return = TRUE;\n"
	w35lit16 = "static void ui_write(char_u *s, int len, int console);\n"
	w35lit17 = "static void ui_write(char_u *s, int len);\n"
	w35lit18 = "ui_write(char_u *s, int len, int console)\n{\n    mch_write(s, len);\n    if (console && s[len - 1] == '\\n')\n    {\n        vim_fsync(1);\n    }\n}\n"
	w35lit19 = "ui_write(char_u *s, int len)\n{\n    mch_write(s, len);\n}\n"
	w35lit20 = "    ui_write(out_buf, len, FALSE);\n"
	w35lit21 = "    ui_write(out_buf, len);\n"
)
