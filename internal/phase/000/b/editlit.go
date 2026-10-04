package p000b

// The multi-line literals the phase splices (graph.go), EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w0bHelpers = "    static usize\niobuff_room(void)\n{\n    if (IObuff == nullptr)\n    {\n        return 0;\n    }\n    return  (1024+1) ;\n}\n\n    static usize\nemsg_iobuff_room(void)\n{\n    if (IObuff == nullptr || emsg_not_now())\n    {\n        return 0;\n    }\n    return  (1024+1) ;\n}\n\n    static char *\niobuff_or(const char *s)\n{\n    if (IObuff == nullptr)\n    {\n        return (char *)s;\n    }\n    return (char *)IObuff;\n}\n\n    static usize\nsafelen_result(char *str, usize str_m, int str_l)\n{\n    if (str_m == 0)\n    {\n        return 0;\n    }\n    if (str_l < 0)\n    {\n        *str = NUL;\n        return 0;\n    }\n    return ((usize)str_l >= str_m) ? str_m - 1 : (usize)str_l;\n}\n\n    static usize\nappend_room(char *str, usize str_m)\n{\n    usize      len =  strlen((char *)(str)) ;\n\n    if (str_m <= len)\n    {\n        return 0;\n    }\n    return str_m - len;\n}\n\n"
	w0blit8    = "static int vim_snprintf(char *, usize, const char *, ...) __attribute__((format(printf, 3, 4)));\n"
	w0blit9    = "static int vim_snprintf(char *, usize, const char *, ...) __attribute__((format(printf, 3, 4)));\nstatic int emsg_not_now(void);\nstatic usize iobuff_room(void);\nstatic usize emsg_iobuff_room(void);\nstatic char *iobuff_or(const char *s);\nstatic usize safelen_result(char *str, usize str_m, int str_l);\nstatic usize append_room(char *str, usize str_m);\n"
)
