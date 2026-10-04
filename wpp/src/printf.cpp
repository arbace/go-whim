// vim_snprintf: the C host's own, as C++ -- whim-vim.c's printf, from
// musl_memchr to vim_vsnprintf_typval, pasted into a struct whose functions
// reach the editor through ed, as the C's reach the core's functions by
// name.  It is the host's because the core names no libc and has no
// variadic function of its own: the core declares vim_snprintf, and this
// file defines it.  Changed from the C only where C++ says otherwise: the
// casts C++ wants written, the functions of the core called as the
// editor's, the C's statics the struct's.

#include "editor.hpp"

#include <cstdarg>

namespace whimpp {

namespace {

struct Printf
{
    Editor &ed;

    // what the C calls of the core's, the editor's here
    template <class... A>
    auto emsg(A... a) { return ed.emsg(a...); }
    template <class... A>
    auto iemsg(A... a) { return ed.iemsg(a...); }
    auto emsg_iobuff_room() { return ed.emsg_iobuff_room(); }
    template <class... A>
    auto iobuff_or(A... a) { return ed.iobuff_or(a...); }
    template <class... A>
    auto utfc_ptr2len(A... a) { return ed.utfc_ptr2len(a...); }
    template <class... A>
    auto utf_ptr2cells(A... a) { return ed.utf_ptr2cells(a...); }
    template <class... A>
    auto alloc_clear(A... a) { return ed.alloc_clear(a...); }
    template <class... A>
    auto musl_memcpy(A... a) { return ed.musl_memcpy(a...); }
    template <class... A>
    auto musl_memmove(A... a) { return ed.musl_memmove(a...); }
    template <class... A>
    auto musl_memset(A... a) { return ed.musl_memset(a...); }
    template <class... A>
    auto musl_strchr(A... a) { return ed.musl_strchr(a...); }
    template <class... A>
    auto musl_strlen(A... a) { return ed.musl_strlen(a...); }
    template <class... A>
    auto musl_strncpy(A... a) { return ed.musl_strncpy(a...); }
    template <class... A>
    auto _(A... a) { return ed._(a...); }
    template <class... A>
    int vim_snprintf(A... a) { return ed.vim_snprintf(a...); }
    // host_free frees nothing: the arena is the editor's
    void host_free(void *) {}
    char_u *&IObuff = ed.IObuff;
    decltype(Editor::e_val_too_large) &e_val_too_large = ed.e_val_too_large;
    decltype(Editor::e_out_of_memory_allocating_nr_bytes) &e_out_of_memory_allocating_nr_bytes = ed.e_out_of_memory_allocating_nr_bytes;
    void *host_alloc(usize n) { return ed.host_alloc(n); }

    enum { TMP_LEN = 350 };

    enum
    {
        TYPE_UNKNOWN = -1,
        TYPE_INT,
        TYPE_LONGINT,
        TYPE_LONGLONGINT,
        TYPE_UNSIGNEDINT,
        TYPE_UNSIGNEDLONGINT,
        TYPE_UNSIGNEDLONGLONGINT,
        TYPE_POINTER,
        TYPE_PERCENT,
        TYPE_CHAR,
        TYPE_STRING,
    };

    enum { MAX_ALLOWED_STRING_WIDTH = 1048576 };

    static inline char typename_unknown[] = "unknown";

    static inline char typename_int[] = "int";

    static inline char typename_longint[] = "long int";

    static inline char typename_longlongint[] = "long long int";

    static inline char typename_unsignedint[] = "unsigned int";

    static inline char typename_unsignedlongint[] = "unsigned long int";

    static inline char typename_unsignedlonglongint[] = "unsigned long long int";

    static inline char typename_pointer[] = "pointer";

    static inline char typename_percent[] = "percent";

    static inline char typename_char[] = "char";

    static inline char typename_string[] = "string";

    static inline char e_cannot_mix_positional_and_non_positional_str[] = "E1500: Cannot mix positional and non-positional arguments: %s";

    static inline char e_fmt_arg_nr_unused_str[] = "E1501: format argument %d unused in $-style format: %s";

    static inline char e_positional_num_field_spec_reused_str_str[] = "E1502: Positional argument %d used as field width reused as different type: %s/%s";

    static inline char e_positional_arg_num_type_inconsistent_str_str[] = "E1504: Positional argument %d type used inconsistently: %s/%s";

    static inline char e_invalid_format_specifier_str[] = "E1505: Invalid format specifier: %s";

    static inline char e_aptypes_is_null_nr_str[] = "E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s";



        void *
    musl_memchr(const void *src, int c, usize n)
    {
        const unsigned char *s = static_cast<const unsigned char *>(src);
        unsigned char ch = (unsigned char)c;
        for (; n && *s != ch; s++, n--)
        {
        }
        return n ? (void *)s : nullptr;
    }

        int
    musl_fmtbase(char spec)
    {
        if (spec == 'o')
        {
            return 8;
        }
        if (spec == 'x' || spec == 'X')
        {
            return 16;
        }
        return 10;
    }

        int
    musl_fmtnum(char *dest, unsigned long long v, int base, int upper, int isneg)
    {
        char digits[24];
        int n = 0;
        int out = 0;
        int i;
        int d;
        if (isneg)
        {
            v = ~v + 1ULL;
        }
        do
        {
            d = (int)(v % (unsigned long long)base);
            if (d < 10)
            {
                digits[n++] = (char)('0' + d);
            }
            else if (upper)
            {
                digits[n++] = (char)('A' + d - 10);
            }
            else
            {
                digits[n++] = (char)('a' + d - 10);
            }
            v /= (unsigned long long)base;
        }
        while (v != 0);
        if (isneg)
        {
            dest[out++] = '-';
        }
        for (i = 0; i < n; i++)
        {
            dest[out++] = digits[n - 1 - i];
        }
        dest[out] = '\0';
        return out;
    }

        int
    musl_fmtptr(char *dest, void *p)
    {
        unsigned long long v = (unsigned long long)(usize)p;
        int i;
        int d;
        dest[0] = '0';
        dest[1] = 'x';
        for (i = 0; i < 16; i++)
        {
            d = (int)((v >> (60 - 4 * i)) & 0xf);
            if (d < 10)
            {
                dest[2 + i] = (char)('0' + d);
            }
            else
            {
                dest[2 + i] = (char)('a' + d - 10);
            }
        }
        dest[18] = '\0';
        return 18;
    }


        int
    vim_vsnprintf(char *str, usize str_m, const char *fmt, va_list ap)
    {
        return vim_vsnprintf_typval(str, str_m, fmt, ap);
    }

        int
    format_typeof(const char *type)
    {
        char length_modifier = '\0';
        char fmt_spec = '\0';
        if (*type == 'h' || *type == 'l')
        {
            length_modifier = *type;
            type++;
            if (length_modifier == 'l' && *type == 'l')
            {
                length_modifier = 'L';
                type++;
            }
        }
        fmt_spec = *type;
        switch (fmt_spec)
        {
        case 'i':
            fmt_spec = 'd';
            break;
        case '*':
            fmt_spec = 'd';
            length_modifier = 'h';
            break;
        case 'D':
            fmt_spec = 'd';
            length_modifier = 'l';
            break;
        case 'U':
            fmt_spec = 'u';
            length_modifier = 'l';
            break;
        case 'O':
            fmt_spec = 'o';
            length_modifier = 'l';
            break;
        default:
            break;
        }
        switch (fmt_spec)
        {
        case '%':
            return TYPE_PERCENT;
        case 'c':
            return TYPE_CHAR;
        case 's':
        case 'S':
            return TYPE_STRING;
        case 'd':
        case 'u':
        case 'b':
        case 'B':
        case 'o':
        case 'x':
        case 'X':
        case 'p':
            {
                if (fmt_spec == 'p')
                {
                    return TYPE_POINTER;
                }
                else if (fmt_spec == 'b' || fmt_spec == 'B')
                {
                    return TYPE_UNSIGNEDLONGLONGINT;
                }
                else if (fmt_spec == 'd')
                {
                    switch (length_modifier)
                    {
                    case '\0':
                    case 'h':
                        return TYPE_INT;
                    case 'l':
                        return TYPE_LONGINT;
                    case 'L':
                        return TYPE_LONGLONGINT;
                    }
                }
                else
                {
                    switch (length_modifier)
                    {
                    case '\0':
                    case 'h':
                        return TYPE_UNSIGNEDINT;
                    case 'l':
                        return TYPE_UNSIGNEDLONGINT;
                    case 'L':
                        return TYPE_UNSIGNEDLONGLONGINT;
                    }
                }
            }
            break;
        }
        return TYPE_UNKNOWN;
    }

        char *
    format_typename(const char *type)
    {
        switch (format_typeof(type))
        {
        case TYPE_INT:
            return typename_int;
        case TYPE_LONGINT:
            return typename_longint;
        case TYPE_LONGLONGINT:
            return typename_longlongint;
        case TYPE_UNSIGNEDINT:
            return typename_unsignedint;
        case TYPE_UNSIGNEDLONGINT:
            return typename_unsignedlongint;
        case TYPE_UNSIGNEDLONGLONGINT:
            return typename_unsignedlonglongint;
        case TYPE_POINTER:
            return _(typename_pointer);
        case TYPE_PERCENT:
            return _(typename_percent);
        case TYPE_CHAR:
            return typename_char;
        case TYPE_STRING:
            return _(typename_string);
        }
        return _(typename_unknown);
    }

        int
    adjust_types(const char ***ap_types, int arg, int *num_posarg, const char *type)
    {
        if (arg <= 0)
        {
            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), type);
            emsg(iobuff_or(_(e_invalid_format_specifier_str)));
            return FAIL;
        }
        if (*ap_types == nullptr || *num_posarg < arg)
        {
            int idx;
            const char **new_types;
            if (*ap_types == nullptr)
            {
                new_types = (const char **)alloc_clear(sizeof(const char *) * (arg));
            }
            else
            {
                new_types = (const char **)host_alloc(arg * sizeof(const char *));
            }
            if (new_types == nullptr)
            {
                return FAIL;
            }
            if (*ap_types != nullptr)
            {
                musl_memcpy((char **)new_types, *ap_types, (usize)*num_posarg * sizeof(const char *));
                host_free((char **)*ap_types);
            }
            for (idx = *num_posarg; idx < arg; ++idx)
            {
                new_types[idx] = nullptr;
            }
            *ap_types = new_types;
            *num_posarg = arg;
        }
        if ((*ap_types)[arg - 1] != nullptr)
        {
            if ((*ap_types)[arg - 1][0] == '*' || type[0] == '*')
            {
                const char *pt = type;
                if (pt[0] == '*')
                {
                    pt = (*ap_types)[arg - 1];
                }
                if (pt[0] != '*')
                {
                    switch (pt[0])
                    {
                    case 'd':
                    case 'i':
                        break;
                    default:
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_positional_num_field_spec_reused_str_str), arg, format_typename((*ap_types)[arg - 1]), format_typename(type));
                        emsg(iobuff_or(_(e_positional_num_field_spec_reused_str_str)));
                        return FAIL;
                    }
                }
            }
            else
            {
                if (format_typeof(type) != format_typeof((*ap_types)[arg - 1]))
                {
                    vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_positional_arg_num_type_inconsistent_str_str), arg, format_typename(type), format_typename((*ap_types)[arg - 1]));
                    emsg(iobuff_or(_(e_positional_arg_num_type_inconsistent_str_str)));
                    return FAIL;
                }
            }
        }
        (*ap_types)[arg - 1] = type;
        return OK;
    }

        void
    format_overflow_error(const char *pstart)
    {
        usize arglen = 0;
        char *argcopy = nullptr;
        const char *p = pstart;
        while (((unsigned)((int)(*p)) - '0' < 10))
        {
            ++p;
        }
        arglen = p - pstart;
        argcopy = (char *)alloc_clear(sizeof(char) * (arglen + 1));
        if (argcopy != nullptr)
        {
            musl_strncpy(argcopy, pstart, arglen);
            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_val_too_large), argcopy);
            emsg(iobuff_or(_(e_val_too_large)));
            host_free(argcopy);
        }
        else
        {
            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_out_of_memory_allocating_nr_bytes), arglen);
            emsg(iobuff_or(_(e_out_of_memory_allocating_nr_bytes)));
        }
    }

        int
    get_unsigned_int(const char *pstart, const char **p, unsigned int *uj, int overflow_err)
    {
        *uj = **p - '0';
        ++*p;
        while (((unsigned)((int)(**p)) - '0' < 10) && *uj < MAX_ALLOWED_STRING_WIDTH)
        {
            *uj = 10 * *uj + (unsigned int)(**p - '0');
            ++*p;
        }
        if (*uj > MAX_ALLOWED_STRING_WIDTH)
        {
            if (overflow_err)
            {
                format_overflow_error(pstart);
                return FAIL;
            }
            else
            {
                *uj = MAX_ALLOWED_STRING_WIDTH;
            }
        }
        return OK;
    }

        int
    parse_fmt_types(const char ***ap_types, int *num_posarg, const char *fmt)
    {
        const char *p = fmt;
        const char *arg = nullptr;
        int any_pos = 0;
        int any_arg = 0;
        int arg_idx;
        if (p == nullptr)
        {
            return OK;
        }
        while (*p != NUL)
        {
            if (*p != '%')
            {
                const char *q = musl_strchr(p + 1, '%');
                usize n = (q == nullptr) ? musl_strlen((char *)(p)) : (usize)(q - p);
                p += n;
            }
            else
            {
                char length_modifier = '\0';
                int pos_arg = -1;
                const char *ptype = nullptr;
                const char *pstart = p + 1;
                p++;
                ptype = p;
                while (((unsigned)(*ptype) - '0' < 10))
                {
                    ++ptype;
                }
                if (*ptype == '$')
                {
                    if (*p == '0')
                    {
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), fmt);
                        emsg(iobuff_or(_(e_invalid_format_specifier_str)));
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                    unsigned int uj;
                    if (get_unsigned_int(pstart, &p, &uj, FALSE) == FAIL)
                    {
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                    pos_arg = uj;
                    any_pos = 1;
                    if (any_pos && any_arg)
                    {
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                        emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                    ;
                    ++p;
                }
                while (*p == '0' || *p == '-' || *p == '+' || *p == ' ' || *p == '#' || *p == '\'')
                {
                    switch (*p)
                    {
                    case '0':
                        break;
                    case '-':
                        break;
                    case '+':
                        break;
                    case ' ':
                        break;
                    case '#':
                        break;
                    case '\'':
                        break;
                    }
                    p++;
                }
                if (*(arg = p) == '*')
                {
                    p++;
                    if (((unsigned)((int)(*p)) - '0' < 10))
                    {
                        unsigned int uj;
                        if (get_unsigned_int(arg + 1, &p, &uj, FALSE) == FAIL)
                        {
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                        if (*p != '$')
                        {
                            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), fmt);
                            emsg(iobuff_or(_(e_invalid_format_specifier_str)));
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                        else
                        {
                            ++p;
                            any_pos = 1;
                            if (any_pos && any_arg)
                            {
                                vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                                emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                                host_free((char **)*ap_types);
                                *ap_types = nullptr;
                                *num_posarg = 0;
                                return FAIL;
                            }
                            ;
                            if (adjust_types(ap_types, uj, num_posarg, arg) == FAIL)
                            {
                                host_free((char **)*ap_types);
                                *ap_types = nullptr;
                                *num_posarg = 0;
                                return FAIL;
                            }
                        }
                    }
                    else
                    {
                        any_arg = 1;
                        if (any_pos && any_arg)
                        {
                            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                            emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                        ;
                    }
                }
                else if (((unsigned)((int)(*p)) - '0' < 10))
                {
                    const char *digstart = p;
                    unsigned int uj;
                    if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                    {
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                    if (*p == '$')
                    {
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), fmt);
                        emsg(iobuff_or(_(e_invalid_format_specifier_str)));
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                }
                if (*p == '.')
                {
                    p++;
                    if (*(arg = p) == '*')
                    {
                        p++;
                        if (((unsigned)((int)(*p)) - '0' < 10))
                        {
                            unsigned int uj;
                            if (get_unsigned_int(arg + 1, &p, &uj, FALSE) == FAIL)
                            {
                                host_free((char **)*ap_types);
                                *ap_types = nullptr;
                                *num_posarg = 0;
                                return FAIL;
                            }
                            if (*p == '$')
                            {
                                any_pos = 1;
                                if (any_pos && any_arg)
                                {
                                    vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                                    emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                                    host_free((char **)*ap_types);
                                    *ap_types = nullptr;
                                    *num_posarg = 0;
                                    return FAIL;
                                }
                                ;
                                ++p;
                                if (adjust_types(ap_types, uj, num_posarg, arg) == FAIL)
                                {
                                    host_free((char **)*ap_types);
                                    *ap_types = nullptr;
                                    *num_posarg = 0;
                                    return FAIL;
                                }
                            }
                            else
                            {
                                vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), fmt);
                                emsg(iobuff_or(_(e_invalid_format_specifier_str)));
                                host_free((char **)*ap_types);
                                *ap_types = nullptr;
                                *num_posarg = 0;
                                return FAIL;
                            }
                        }
                        else
                        {
                            any_arg = 1;
                            if (any_pos && any_arg)
                            {
                                vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                                emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                                host_free((char **)*ap_types);
                                *ap_types = nullptr;
                                *num_posarg = 0;
                                return FAIL;
                            }
                            ;
                        }
                    }
                    else if (((unsigned)((int)(*p)) - '0' < 10))
                    {
                        const char *digstart = p;
                        unsigned int uj;
                        if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                        {
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                        if (*p == '$')
                        {
                            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_invalid_format_specifier_str), fmt);
                            emsg(iobuff_or(_(e_invalid_format_specifier_str)));
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                    }
                }
                if (pos_arg != -1)
                {
                    any_pos = 1;
                    if (any_pos && any_arg)
                    {
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                        emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                    ;
                    ptype = p;
                }
                if (*p == 'h' || *p == 'l')
                {
                    length_modifier = *p;
                    p++;
                    if (length_modifier == 'l' && *p == 'l')
                    {
                        p++;
                    }
                }
                switch (*p)
                {
                case 'i':
                case '*':
                case 'd':
                case 'u':
                case 'o':
                case 'D':
                case 'U':
                case 'O':
                case 'x':
                case 'X':
                case 'b':
                case 'B':
                case 'c':
                case 's':
                case 'S':
                case 'p':
                    if (pos_arg != -1)
                    {
                        if (adjust_types(ap_types, pos_arg, num_posarg, ptype) == FAIL)
                        {
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                    }
                    else
                    {
                        any_arg = 1;
                        if (any_pos && any_arg)
                        {
                            vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                            emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                            host_free((char **)*ap_types);
                            *ap_types = nullptr;
                            *num_posarg = 0;
                            return FAIL;
                        }
                        ;
                    }
                    break;
                default:
                    if (pos_arg != -1)
                    {
                        vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_cannot_mix_positional_and_non_positional_str), fmt);
                        emsg(iobuff_or(_(e_cannot_mix_positional_and_non_positional_str)));
                        host_free((char **)*ap_types);
                        *ap_types = nullptr;
                        *num_posarg = 0;
                        return FAIL;
                    }
                }
                if (*p != NUL)
                {
                    p++;
                }
            }
        }
        for (arg_idx = 0; arg_idx < *num_posarg; ++arg_idx)
        {
            if ((*ap_types)[arg_idx] == nullptr)
            {
                vim_snprintf((char *)IObuff, emsg_iobuff_room(), _(e_fmt_arg_nr_unused_str), arg_idx + 1, fmt);
                emsg(iobuff_or(_(e_fmt_arg_nr_unused_str)));
                host_free((char **)*ap_types);
                *ap_types = nullptr;
                *num_posarg = 0;
                return FAIL;
            }
        }
        return OK;
    }

        void
    skip_to_arg(const char **ap_types, va_list ap_start, va_list *ap, int *arg_idx, int *arg_cur, const char *fmt)
    {
        int arg_min = 0;
        if (*arg_cur + 1 == *arg_idx)
        {
            ++*arg_cur;
            ++*arg_idx;
            return;
        }
        if (*arg_cur >= *arg_idx)
        {
            va_end(*ap);
            va_copy(*ap, ap_start);
        }
        else
        {
            arg_min = *arg_cur;
        }
        for (*arg_cur = arg_min; *arg_cur < *arg_idx - 1; ++*arg_cur)
        {
            const char *p;
            if (ap_types == nullptr || ap_types[*arg_cur] == nullptr)
            {
                vim_snprintf((char *)IObuff, emsg_iobuff_room(), e_aptypes_is_null_nr_str, *arg_cur, fmt);
                iemsg(iobuff_or(e_aptypes_is_null_nr_str));
                return;
            }
            p = ap_types[*arg_cur];
            int fmt_type = format_typeof(p);
            switch (fmt_type)
            {
            case TYPE_PERCENT:
            case TYPE_UNKNOWN:
                break;
            case TYPE_CHAR:
                va_arg(*ap, int);
                break;
            case TYPE_STRING:
                va_arg(*ap, char *);
                break;
            case TYPE_POINTER:
                va_arg(*ap, void *);
                break;
            case TYPE_INT:
                va_arg(*ap, int);
                break;
            case TYPE_LONGINT:
                va_arg(*ap, long int);
                break;
            case TYPE_LONGLONGINT:
                va_arg(*ap, varnumber_T);
                break;
            case TYPE_UNSIGNEDINT:
                va_arg(*ap, unsigned int);
                break;
            case TYPE_UNSIGNEDLONGINT:
                va_arg(*ap, unsigned long int);
                break;
            case TYPE_UNSIGNEDLONGLONGINT:
                va_arg(*ap, uvarnumber_T);
                break;
            }
        }
        ++*arg_cur;
        ++*arg_idx;
        return;
    }

        int
    vim_vsnprintf_typval(char *str, usize str_m, const char *fmt, va_list ap_start)
    {
        usize str_l = 0;
        const char *p = fmt;
        int arg_cur = 0;
        int num_posarg = 0;
        int arg_idx = 1;
        va_list ap;
        const char **ap_types = nullptr;
        if (parse_fmt_types(&ap_types, &num_posarg, fmt) == FAIL)
        {
            return 0;
        }
        va_copy(ap, ap_start);
        if (p == nullptr)
        {
            p = "";
        }
        while (*p != NUL)
        {
            if (*p != '%')
            {
                const char *q = musl_strchr(p + 1, '%');
                usize n = (q == nullptr) ? musl_strlen((char *)(p)) : (usize)(q - p);
                if (str_l < str_m)
                {
                    usize avail = str_m - str_l;
                    musl_memmove((char *)(str + str_l), (char *)(p), n > avail ? avail : n);
                }
                p += n;
                str_l += n;
            }
            else
            {
                usize min_field_width = 0;
                usize precision = 0;
                int zero_padding = 0;
                int precision_specified = 0;
                int justify_left = 0;
                int alternate_form = 0;
                int force_sign = 0;
                int space_for_positive = 1;
                char length_modifier = '\0';
                char tmp[TMP_LEN];
                const char *str_arg = nullptr;
                usize str_arg_l;
                unsigned char uchar_arg;
                usize number_of_zeros_to_pad = 0;
                usize zero_padding_insertion_ind = 0;
                char fmt_spec = '\0';
                char_u *tofree = nullptr;
                int pos_arg = -1;
                const char *ptype;
                p++;
                ptype = p;
                while (((unsigned)(*ptype) - '0' < 10))
                {
                    ++ptype;
                }
                if (*ptype == '$')
                {
                    const char *digstart = p;
                    unsigned int uj;
                    if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                    {
                        host_free((char *)ap_types);
                        va_end(ap);
                        return (int)str_l;
                    }
                    pos_arg = uj;
                    ++p;
                }
                while (*p == '0' || *p == '-' || *p == '+' || *p == ' ' || *p == '#' || *p == '\'')
                {
                    switch (*p)
                    {
                    case '0':
                        zero_padding = 1;
                        break;
                    case '-':
                        justify_left = 1;
                        break;
                    case '+':
                        force_sign = 1;
                        space_for_positive = 0;
                        break;
                    case ' ':
                        force_sign = 1;
                        break;
                    case '#':
                        alternate_form = 1;
                        break;
                    case '\'':
                        break;
                    }
                    p++;
                }
                if (*p == '*')
                {
                    int j;
                    const char *digstart = p + 1;
                    p++;
                    if (((unsigned)((int)(*p)) - '0' < 10))
                    {
                        unsigned int uj;
                        if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                        {
                            host_free((char *)ap_types);
                            va_end(ap);
                            return (int)str_l;
                        }
                        arg_idx = uj;
                        ++p;
                    }
                    j = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, int));
                    if (j > MAX_ALLOWED_STRING_WIDTH)
                    {
                        j = MAX_ALLOWED_STRING_WIDTH;
                    }
                    if (j >= 0)
                    {
                        min_field_width = j;
                    }
                    else
                    {
                        min_field_width = -j;
                        justify_left = 1;
                    }
                }
                else if (((unsigned)((int)(*p)) - '0' < 10))
                {
                    const char *digstart = p;
                    unsigned int uj;
                    if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                    {
                        host_free((char *)ap_types);
                        va_end(ap);
                        return (int)str_l;
                    }
                    min_field_width = uj;
                }
                if (*p == '.')
                {
                    p++;
                    precision_specified = 1;
                    if (((unsigned)((int)(*p)) - '0' < 10))
                    {
                        const char *digstart = p;
                        unsigned int uj;
                        if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                        {
                            host_free((char *)ap_types);
                            va_end(ap);
                            return (int)str_l;
                        }
                        precision = uj;
                    }
                    else if (*p == '*')
                    {
                        int j;
                        const char *digstart = p;
                        p++;
                        if (((unsigned)((int)(*p)) - '0' < 10))
                        {
                            unsigned int uj;
                            if (get_unsigned_int(digstart, &p, &uj, FALSE) == FAIL)
                            {
                                host_free((char *)ap_types);
                                va_end(ap);
                                return (int)str_l;
                            }
                            arg_idx = uj;
                            ++p;
                        }
                        j = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, int));
                        if (j > MAX_ALLOWED_STRING_WIDTH)
                        {
                            j = MAX_ALLOWED_STRING_WIDTH;
                        }
                        if (j >= 0)
                        {
                            precision = j;
                        }
                        else
                        {
                            precision_specified = 0;
                            precision = 0;
                        }
                    }
                }
                if (*p == 'h' || *p == 'l')
                {
                    length_modifier = *p;
                    p++;
                    if (length_modifier == 'l' && *p == 'l')
                    {
                        length_modifier = 'L';
                        p++;
                    }
                }
                fmt_spec = *p;
                switch (fmt_spec)
                {
                case 'i':
                    fmt_spec = 'd';
                    break;
                case 'D':
                    fmt_spec = 'd';
                    length_modifier = 'l';
                    break;
                case 'U':
                    fmt_spec = 'u';
                    length_modifier = 'l';
                    break;
                case 'O':
                    fmt_spec = 'o';
                    length_modifier = 'l';
                    break;
                default:
                    break;
                }
                if (pos_arg != -1)
                {
                    arg_idx = pos_arg;
                }
                switch (fmt_spec)
                {
                case '%':
                case 'c':
                case 's':
                case 'S':
                    str_arg_l = 1;
                    switch (fmt_spec)
                    {
                    case '%':
                        str_arg = p;
                        break;
                    case 'c':
                        {
                            int j;
                            j = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, int));
                            uchar_arg = (unsigned char)j;
                            str_arg = (char *)&uchar_arg;
                            break;
                        }
                    case 's':
                    case 'S':
                        str_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, char *));
                        if (str_arg == nullptr)
                        {
                            str_arg = "[NULL]";
                            str_arg_l = 6;
                        }
                        else if (!precision_specified)
                        {
                            str_arg_l = musl_strlen(str_arg);
                        }
                        else if (precision == 0)
                        {
                            str_arg_l = 0;
                        }
                        else
                        {
                            const char *q = (const char *)musl_memchr(str_arg, '\0', precision <= (usize)0x7fffffffL ? precision : (usize)0x7fffffffL);
                            str_arg_l = (q == nullptr) ? precision : (usize)(q - str_arg);
                        }
                        if (fmt_spec == 'S')
                        {
                            char_u *p1;
                            usize i;
                            int cell;
                            for (i = 0, p1 = (char_u *)str_arg; *p1; p1 += utfc_ptr2len(p1))
                            {
                                cell = utf_ptr2cells(p1);
                                if (precision_specified && i + cell > precision)
                                {
                                    break;
                                }
                                i += cell;
                            }
                            str_arg_l = p1 - (char_u *)str_arg;
                            if (min_field_width != 0)
                            {
                                min_field_width += str_arg_l - i;
                            }
                        }
                        break;
                    default:
                        break;
                    }
                    break;
                case 'd':
                case 'u':
                case 'b':
                case 'B':
                case 'o':
                case 'x':
                case 'X':
                case 'p':
                    {
                        int arg_sign = 0;
                        int int_arg = 0;
                        unsigned int uint_arg = 0;
                        long int long_arg = 0;
                        unsigned long int ulong_arg = 0;
                        varnumber_T llong_arg = 0;
                        uvarnumber_T ullong_arg = 0;
                        uvarnumber_T bin_arg = 0;
                        void *ptr_arg = nullptr;
                        if (fmt_spec == 'p')
                        {
                            length_modifier = '\0';
                            ptr_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, void *));
                            if (ptr_arg != nullptr)
                            {
                                arg_sign = 1;
                            }
                        }
                        else if (fmt_spec == 'b' || fmt_spec == 'B')
                        {
                            bin_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, uvarnumber_T));
                            if (bin_arg != 0)
                            {
                                arg_sign = 1;
                            }
                        }
                        else if (fmt_spec == 'd')
                        {
                            switch (length_modifier)
                            {
                            case '\0':
                            case 'h':
                                int_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, int));
                                if (int_arg > 0)
                                {
                                    arg_sign = 1;
                                }
                                else if (int_arg < 0)
                                {
                                    arg_sign = -1;
                                }
                                break;
                            case 'l':
                                long_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, long int));
                                if (long_arg > 0)
                                {
                                    arg_sign = 1;
                                }
                                else if (long_arg < 0)
                                {
                                    arg_sign = -1;
                                }
                                break;
                            case 'L':
                                llong_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, varnumber_T));
                                if (llong_arg > 0)
                                {
                                    arg_sign = 1;
                                }
                                else if (llong_arg < 0)
                                {
                                    arg_sign = -1;
                                }
                                break;
                            }
                        }
                        else
                        {
                            switch (length_modifier)
                            {
                            case '\0':
                            case 'h':
                                uint_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, unsigned int));
                                if (uint_arg != 0)
                                {
                                    arg_sign = 1;
                                }
                                break;
                            case 'l':
                                ulong_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, unsigned long int));
                                if (ulong_arg != 0)
                                {
                                    arg_sign = 1;
                                }
                                break;
                            case 'L':
                                ullong_arg = (skip_to_arg(ap_types, ap_start, &ap, &arg_idx, &arg_cur, fmt), va_arg(ap, uvarnumber_T));
                                if (ullong_arg != 0)
                                {
                                    arg_sign = 1;
                                }
                                break;
                            }
                        }
                        str_arg = tmp;
                        str_arg_l = 0;
                        if (precision_specified)
                        {
                            zero_padding = 0;
                        }
                        if (fmt_spec == 'd')
                        {
                            if (force_sign && arg_sign >= 0)
                            {
                                tmp[str_arg_l++] = space_for_positive ? ' ' : '+';
                            }
                        }
                        else if (alternate_form)
                        {
                            if (arg_sign != 0 && (fmt_spec == 'b' || fmt_spec == 'B' || fmt_spec == 'x' || fmt_spec == 'X'))
                            {
                                tmp[str_arg_l++] = '0';
                                tmp[str_arg_l++] = fmt_spec;
                            }
                        }
                        zero_padding_insertion_ind = str_arg_l;
                        if (!precision_specified)
                        {
                            precision = 1;
                        }
                        if (precision == 0 && arg_sign == 0)
                        {
                        }
                        else
                        {
                            if (fmt_spec == 'p')
                            {
                                str_arg_l += musl_fmtptr(tmp + str_arg_l, ptr_arg);
                            }
                            else if (fmt_spec == 'b' || fmt_spec == 'B')
                            {
                                char b[8 * sizeof(uvarnumber_T)];
                                usize b_l = 0;
                                uvarnumber_T bn = bin_arg;
                                do
                                {
                                    b[sizeof(b) - ++b_l] = '0' + (bn & 0x1);
                                    bn >>= 1;
                                }
                                while (bn != 0);
                                musl_memcpy(tmp + str_arg_l, b + sizeof(b) - b_l, b_l);
                                str_arg_l += b_l;
                            }
                            else if (fmt_spec == 'd')
                            {
                                switch (length_modifier)
                                {
                                case '\0':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)(long long)int_arg, 10, 0, int_arg < 0);
                                    break;
                                case 'h':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)(long long)(short)int_arg, 10, 0, (short)int_arg < 0);
                                    break;
                                case 'l':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)(long long)long_arg, 10, 0, long_arg < 0);
                                    break;
                                case 'L':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)llong_arg, 10, 0, llong_arg < 0);
                                    break;
                                }
                            }
                            else
                            {
                                switch (length_modifier)
                                {
                                case '\0':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)uint_arg, musl_fmtbase(fmt_spec), fmt_spec == 'X', 0);
                                    break;
                                case 'h':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)(unsigned short)uint_arg, musl_fmtbase(fmt_spec), fmt_spec == 'X', 0);
                                    break;
                                case 'l':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)ulong_arg, musl_fmtbase(fmt_spec), fmt_spec == 'X', 0);
                                    break;
                                case 'L':
                                    str_arg_l += musl_fmtnum(tmp + str_arg_l, (unsigned long long)ullong_arg, musl_fmtbase(fmt_spec), fmt_spec == 'X', 0);
                                    break;
                                }
                            }
                            if (zero_padding_insertion_ind < str_arg_l && tmp[zero_padding_insertion_ind] == '-')
                            {
                                zero_padding_insertion_ind++;
                            }
                            if (zero_padding_insertion_ind + 1 < str_arg_l && tmp[zero_padding_insertion_ind] == '0' && (tmp[zero_padding_insertion_ind + 1] == 'x' || tmp[zero_padding_insertion_ind + 1] == 'X'))
                            {
                                zero_padding_insertion_ind += 2;
                            }
                        }
                        {
                            usize num_of_digits = str_arg_l - zero_padding_insertion_ind;
                            if (alternate_form && fmt_spec == 'o' && !(zero_padding_insertion_ind < str_arg_l && tmp[zero_padding_insertion_ind] == '0'))
                            {
                                if (!precision_specified || precision < num_of_digits + 1)
                                {
                                    precision = num_of_digits + 1;
                                }
                            }
                            if (num_of_digits < precision)
                            {
                                number_of_zeros_to_pad = precision - num_of_digits;
                            }
                        }
                        if (!justify_left && zero_padding)
                        {
                            int n = (int)(min_field_width - (str_arg_l + number_of_zeros_to_pad));
                            if (n > 0)
                            {
                                number_of_zeros_to_pad += n;
                            }
                        }
                        break;
                    }
                default:
                    zero_padding = 0;
                    justify_left = 1;
                    min_field_width = 0;
                    str_arg = p;
                    str_arg_l = 0;
                    if (*p != NUL)
                    {
                        str_arg_l++;
                    }
                    break;
                }
                if (*p != NUL)
                {
                    p++;
                }
                if (!justify_left)
                {
                    int pn = (int)(min_field_width - (str_arg_l + number_of_zeros_to_pad));
                    if (pn > 0)
                    {
                        if (str_l < str_m)
                        {
                            usize avail = str_m - str_l;
                            musl_memset((str + str_l), (zero_padding ? '0' : ' '), ((usize)pn > avail ? avail : (usize)pn));
                        }
                        str_l += pn;
                    }
                }
                if (number_of_zeros_to_pad == 0)
                {
                    zero_padding_insertion_ind = 0;
                }
                else
                {
                    int zn = (int)zero_padding_insertion_ind;
                    if (zn > 0)
                    {
                        if (str_l < str_m)
                        {
                            usize avail = str_m - str_l;
                            musl_memmove((char *)(str + str_l), (char *)(str_arg), (usize)zn > avail ? avail : (usize)zn);
                        }
                        str_l += zn;
                    }
                    zn = (int)number_of_zeros_to_pad;
                    if (zn > 0)
                    {
                        if (str_l < str_m)
                        {
                            usize avail = str_m - str_l;
                            musl_memset((str + str_l), ('0'), ((usize)zn > avail ? avail : (usize)zn));
                        }
                        str_l += zn;
                    }
                }
                {
                    int sn = (int)(str_arg_l - zero_padding_insertion_ind);
                    if (sn > 0)
                    {
                        if (str_l < str_m)
                        {
                            usize avail = str_m - str_l;
                            musl_memmove((char *)(str + str_l), (char *)(str_arg + zero_padding_insertion_ind), (usize)sn > avail ? avail : (usize)sn);
                        }
                        str_l += sn;
                    }
                }
                if (justify_left)
                {
                    int pn = (int)(min_field_width - (str_arg_l + number_of_zeros_to_pad));
                    if (pn > 0)
                    {
                        if (str_l < str_m)
                        {
                            usize avail = str_m - str_l;
                            musl_memset((str + str_l), (' '), ((usize)pn > avail ? avail : (usize)pn));
                        }
                        str_l += pn;
                    }
                }
                host_free(tofree);
            }
        }
        if (str_m > 0)
        {
            str[str_l <= str_m - 1 ? str_l : str_m - 1] = '\0';
        }
        host_free((char *)ap_types);
        va_end(ap);
        return (int)str_l;
    }};

} // namespace

int Editor::vim_snprintf(char *str, usize str_m, const char *fmt, ...)
{
    va_list ap;
    va_start(ap, fmt);
    const int str_l = Printf{*this}.vim_vsnprintf(str, str_m, fmt, ap);
    va_end(ap);
    return str_l;
}

} // namespace whimpp
