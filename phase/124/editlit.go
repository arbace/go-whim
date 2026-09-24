package p124

// The multi-line literals phase/124/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w124AllocOld    = "    static void *\nhost_alloc(usize n)\n{\n    return malloc(n);\n}\n"
	w124FreeOld     = "    static void\nhost_free(void *p)\n{\n    free(p);\n}\n"
	w124OverflowOld = "        free(argcopy);\n"
	w124ReallocOld  = "        else\n        {\n            new_types = realloc(((char **)*ap_types), (arg * sizeof(const char *)));\n        }\n        if (new_types == nullptr)\n        {\n            return FAIL;\n        }\n"
	w124AllocNew    = "enum { HOST_ARENA_BYTES = 1024 * 1024 * 1024 };\n\nstatic max_align_t host_arena[HOST_ARENA_BYTES / sizeof(max_align_t)];\nstatic usize host_arena_used;\n\n    static int\nhost_arena_say(char *b, int at, const char *s)\n{\n    usize       n = musl_strlen(s);\n\n    musl_memcpy(b + at, s, n);\n    return at + (int)n;\n}\n\n    static int\nhost_arena_num(char *b, int at, usize v)\n{\n    char        d[24];\n    int         i = 24;\n\n    if (v == 0)\n    {\n        d[--i] = '0';\n    }\n    while (v > 0)\n    {\n        d[--i] = (char)('0' + (int)(v % 10));\n        v /= 10;\n    }\n    while (i < 24)\n    {\n        b[at++] = d[i++];\n    }\n    return at;\n}\n\n    static void\nhost_arena_exhausted(usize n)\n{\n    char        m[160];\n    int         at = 0;\n\n    at = host_arena_say(m, at, \"whim-vim: host arena exhausted: \");\n    at = host_arena_num(m, at, sizeof(host_arena));\n    at = host_arena_say(m, at, \" bytes, \");\n    at = host_arena_num(m, at, host_arena_used);\n    at = host_arena_say(m, at, \" used, request \");\n    at = host_arena_num(m, at, n);\n    at = host_arena_say(m, at, \"\\n\");\n    host_message(m, at, TRUE);\n    host_exit(1);\n}\n\n    static void *\nhost_alloc(usize n)\n{\n    usize       want = (n + (alignof(max_align_t) - 1)) & ~(usize)(alignof(max_align_t) - 1);\n    char        *p;\n\n    if (want < n || want > sizeof(host_arena) - host_arena_used)\n    {\n        host_arena_exhausted(n);\n    }\n    p = (char *)host_arena + host_arena_used;\n    host_arena_used += want;\n    return p;\n}\n"
	w124FreeNew     = "    static void\nhost_free(void *p)\n{\n    (void)p;\n}\n"
	w124ReallocNew  = "        else\n        {\n            new_types = (const char **)host_alloc(arg * sizeof(const char *));\n        }\n\n        if (new_types == nullptr)\n        {\n            return FAIL;\n        }\n\n        if (*ap_types != nullptr)\n        {\n            musl_memcpy((char **)new_types, *ap_types, (usize)*num_posarg * sizeof(const char *));\n            host_free((char **)*ap_types);\n        }\n\n"
	w124lit2        = "        host_free(argcopy);\n"
)
