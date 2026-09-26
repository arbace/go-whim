package whim.host;

import whim.rt.BytePtr;
import whim.rt.Rt;

/**
 * vim's own printf, vim_snprintf, from the host half of whim-vim.c: the Java
 * port of editor/format.go, line for line where it can be, so that it prints
 * the bytes the C prints.  The core calls it for every message it formats; it
 * needs nothing of an operating system, and of the editor only what
 * {@link Core} names -- the translation of a message, the error functions and
 * the width of a character -- which the glue ({@code Whim}) hands it.
 *
 * <p>The C va_list is the {@code Object...} array and an index into it:
 * va_arg reads args[ap] and advances; va_copy(ap, ap_start) resets the index
 * to 0.  Each argument is converted to the type the C va_arg names, from what
 * it arrived as:
 * <ul>
 * <li>{@code Integer}, {@code Short}, {@code Long} are signed and
 *     sign-extend; the conversion then keeps the width it names, so an
 *     unsigned int passed as an Integer's bits prints right under %u and %x.
 *     A {@code Byte} and a {@code Character} zero-extend: the core's bytes are
 *     char_u.  A {@code Boolean} is 0 or 1.
 * <li>%s takes a {@code BytePtr} (NULL is {@code null}), a {@code byte[]}
 *     (an array that decays) or a {@code String} (its chars as bytes).
 * <li>%p prints an address no C run would: Java has none.  The core uses %p
 *     for nothing a user sees.
 * </ul>
 *
 * <p>vim_vsnprintf_typval is only ever called with tvs == NULL, so, as in the
 * Go, the typval parameter is dropped: get_unsigned_int clamps instead of
 * reporting overflow, and the "too many arguments" check never runs.
 */
public final class Printf {
    /** What the formatter needs of the editor it formats for. */
    public interface Core {
        /** gettext(): the message as the user reads it. */
        BytePtr gettext(BytePtr msg);
        /** emsg() and iemsg(): report an error. */
        void error(BytePtr msg);
        void internalError(BytePtr msg);
        /** IObuff, the room emsg leaves in it, and IObuff or s when it has none. */
        BytePtr iobuff();
        long emsgIobuffRoom();
        BytePtr iobuffOr(BytePtr s);
        /** e_val_too_large, the core's message. */
        BytePtr eValTooLarge();
        /** The bytes of the character at p, with its composing ones, and its cells. */
        int utfcPtr2len(BytePtr p);
        int utfPtr2cells(BytePtr p);
    }

    private final Core ed;

    public Printf(Core ed) {
        this.ed = ed;
    }

    static final int TMP_LEN = 350;

    static final int TYPE_UNKNOWN = -1;
    static final int TYPE_INT = 0;
    static final int TYPE_LONGINT = 1;
    static final int TYPE_LONGLONGINT = 2;
    static final int TYPE_UNSIGNEDINT = 3;
    static final int TYPE_UNSIGNEDLONGINT = 4;
    static final int TYPE_UNSIGNEDLONGLONGINT = 5;
    static final int TYPE_POINTER = 6;
    static final int TYPE_PERCENT = 7;
    static final int TYPE_CHAR = 8;
    static final int TYPE_STRING = 9;

    static final int MAX_ALLOWED_STRING_WIDTH = 1048576;

    static final BytePtr typename_unknown = BytePtr.lit("unknown");
    static final BytePtr typename_int = BytePtr.lit("int");
    static final BytePtr typename_longint = BytePtr.lit("long int");
    static final BytePtr typename_longlongint = BytePtr.lit("long long int");
    static final BytePtr typename_unsignedint = BytePtr.lit("unsigned int");
    static final BytePtr typename_unsignedlongint = BytePtr.lit("unsigned long int");
    static final BytePtr typename_unsignedlonglongint = BytePtr.lit("unsigned long long int");
    static final BytePtr typename_pointer = BytePtr.lit("pointer");
    static final BytePtr typename_percent = BytePtr.lit("percent");
    static final BytePtr typename_char = BytePtr.lit("char");
    static final BytePtr typename_string = BytePtr.lit("string");
    static final BytePtr e_cannot_mix_positional_and_non_positional_str = BytePtr.lit("E1500: Cannot mix positional and non-positional arguments: %s");
    static final BytePtr e_fmt_arg_nr_unused_str = BytePtr.lit("E1501: format argument %d unused in $-style format: %s");
    static final BytePtr e_positional_num_field_spec_reused_str_str = BytePtr.lit("E1502: Positional argument %d used as field width reused as different type: %s/%s");
    static final BytePtr e_positional_arg_num_type_inconsistent_str_str = BytePtr.lit("E1504: Positional argument %d type used inconsistently: %s/%s");
    static final BytePtr e_invalid_format_specifier_str = BytePtr.lit("E1505: Invalid format specifier: %s");
    static final BytePtr e_aptypes_is_null_nr_str = BytePtr.lit("E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s");

    // ------------------------------------------------------------------
    // the va_list

    private static final class Va {
        final Object[] args;
        int i;

        Va(Object[] args) {
            this.args = args == null ? new Object[0] : args;
        }

        Object next() {
            if (i >= args.length) {
                throw new IllegalStateException("vim_snprintf: va_arg past the last argument");
            }
            return args[i++];
        }

        int vaInt() { return (int) vaInt(next()); }
        int vaUint() { return (int) vaInt(next()); }
        long vaLong() { return vaInt(next()); }
        long vaUlong() { return vaInt(next()); }
        void vaSkip() { next(); }

        BytePtr vaString() {
            Object x = next();
            if (x == null) {
                return null;
            }
            if (x instanceof BytePtr p) {
                return p;
            }
            if (x instanceof byte[] a) {
                return new BytePtr(a, 0);
            }
            if (x instanceof String s) {
                byte[] b = new byte[s.length() + 1];
                for (int k = 0; k < s.length(); k++) {
                    b[k] = (byte) s.charAt(k);
                }
                return new BytePtr(b, 0);
            }
            throw new IllegalArgumentException("vim_snprintf: %s was given " + x.getClass().getName());
        }

        /** va_arg(ap, void *) as the address it would print: one Java makes up. */
        long vaPointer() {
            Object x = next();
            if (x == null) {
                return 0;
            }
            if (x instanceof Number || x instanceof Character || x instanceof Boolean) {
                return vaInt(x);
            }
            if (x instanceof BytePtr p) {
                return ((long) System.identityHashCode(p.a) << 4) + p.i;
            }
            return (long) System.identityHashCode(x) << 4;
        }

        /**
         * An integer argument as C would read it had it been passed at its
         * own width and read back at 64 bits; the caller truncates.
         */
        static long vaInt(Object a) {
            if (a instanceof Integer x) {
                return x;
            }
            if (a instanceof Long x) {
                return x;
            }
            if (a instanceof Short x) {
                return x;
            }
            if (a instanceof Byte x) {
                return x & 0xff;
            }
            if (a instanceof Character x) {
                return x;
            }
            if (a instanceof Boolean x) {
                return x ? 1 : 0;
            }
            throw new IllegalArgumentException("vim_snprintf: an integer conversion was given "
                    + (a == null ? "null" : a.getClass().getName()));
        }
    }

    // ------------------------------------------------------------------
    // the host's small functions

    static boolean isDigit(byte c) {
        return c >= '0' && c <= '9';
    }

    static long strlen(BytePtr s) {
        return BytePtr.strlen(s);
    }

    static BytePtr strchr(BytePtr s, byte c) {
        for (;;) {
            if (s.get() == c) {
                return s;
            }
            if (s.get() == 0) {
                return null;
            }
            s = s.add(1);
        }
    }

    static BytePtr memchr(BytePtr s, int c, long n) {
        byte ch = (byte) c;
        for (; n != 0 && s.get() != ch; s = s.add(1), n--) {
        }
        return n != 0 ? s : null;
    }

    static boolean ult(long a, long b) {
        return Long.compareUnsigned(a, b) < 0;
    }

    static long umin(long a, long b) {
        return ult(b, a) ? b : a;
    }

    static int fmtbase(byte spec) {
        if (spec == 'o') {
            return 8;
        }
        if (spec == 'x' || spec == 'X') {
            return 16;
        }
        return 10;
    }

    static int fmtnum(BytePtr dest, long v, int base, boolean upper, boolean isneg) {
        byte[] digits = new byte[24];
        int n = 0, out = 0;
        if (isneg) {
            v = ~v + 1;
        }
        do {
            int d = (int) Long.remainderUnsigned(v, base);
            if (d < 10) {
                digits[n] = (byte) ('0' + d);
            } else if (upper) {
                digits[n] = (byte) ('A' + d - 10);
            } else {
                digits[n] = (byte) ('a' + d - 10);
            }
            n++;
            v = Long.divideUnsigned(v, base);
        } while (v != 0);
        if (isneg) {
            dest.set(out++, (byte) '-');
        }
        for (int i = 0; i < n; i++) {
            dest.set(out++, digits[n - 1 - i]);
        }
        dest.set(out, (byte) 0);
        return out;
    }

    static int fmtptr(BytePtr dest, long v) {
        dest.set(0, (byte) '0');
        dest.set(1, (byte) 'x');
        for (int i = 0; i < 16; i++) {
            int d = (int) ((v >>> (60 - 4 * i)) & 0xf);
            dest.set(2 + i, (byte) (d < 10 ? '0' + d : 'a' + d - 10));
        }
        dest.set(18, (byte) 0);
        return 18;
    }

    static int formatTypeof(BytePtr type) {
        byte length_modifier = 0;
        byte fmt_spec;

        if (type.get() == 'h' || type.get() == 'l') {
            length_modifier = type.get();
            type = type.add(1);
            if (length_modifier == 'l' && type.get() == 'l') {
                length_modifier = 'L';
                type = type.add(1);
            }
        }
        fmt_spec = type.get();

        switch (fmt_spec) {
            case 'i' -> fmt_spec = 'd';
            case '*' -> {
                fmt_spec = 'd';
                length_modifier = 'h';
            }
            case 'D' -> {
                fmt_spec = 'd';
                length_modifier = 'l';
            }
            case 'U' -> {
                fmt_spec = 'u';
                length_modifier = 'l';
            }
            case 'O' -> {
                fmt_spec = 'o';
                length_modifier = 'l';
            }
            default -> {}
        }

        switch (fmt_spec) {
            case '%':
                return TYPE_PERCENT;
            case 'c':
                return TYPE_CHAR;
            case 's', 'S':
                return TYPE_STRING;
            case 'd', 'u', 'b', 'B', 'o', 'x', 'X', 'p':
                if (fmt_spec == 'p') {
                    return TYPE_POINTER;
                } else if (fmt_spec == 'b' || fmt_spec == 'B') {
                    return TYPE_UNSIGNEDLONGLONGINT;
                } else if (fmt_spec == 'd') {
                    switch (length_modifier) {
                        case 0, 'h':
                            return TYPE_INT;
                        case 'l':
                            return TYPE_LONGINT;
                        case 'L':
                            return TYPE_LONGLONGINT;
                        default:
                            break;
                    }
                } else {
                    switch (length_modifier) {
                        case 0, 'h':
                            return TYPE_UNSIGNEDINT;
                        case 'l':
                            return TYPE_UNSIGNEDLONGINT;
                        case 'L':
                            return TYPE_UNSIGNEDLONGLONGINT;
                        default:
                            break;
                    }
                }
                break;
            default:
                break;
        }
        return TYPE_UNKNOWN;
    }

    BytePtr formatTypename(BytePtr type) {
        return switch (formatTypeof(type)) {
            case TYPE_INT -> typename_int;
            case TYPE_LONGINT -> typename_longint;
            case TYPE_LONGLONGINT -> typename_longlongint;
            case TYPE_UNSIGNEDINT -> typename_unsignedint;
            case TYPE_UNSIGNEDLONGINT -> typename_unsignedlongint;
            case TYPE_UNSIGNEDLONGLONGINT -> typename_unsignedlonglongint;
            case TYPE_POINTER -> ed.gettext(typename_pointer);
            case TYPE_PERCENT -> ed.gettext(typename_percent);
            case TYPE_CHAR -> typename_char;
            case TYPE_STRING -> ed.gettext(typename_string);
            default -> ed.gettext(typename_unknown);
        };
    }

    /** The vim_snprintf-into-IObuff-then-emsg pair every format error makes. */
    private void fmtError(BytePtr msg, Object... args) {
        snprintf(ed.iobuff(), ed.emsgIobuffRoom(), ed.gettext(msg), args);
        ed.error(ed.iobuffOr(ed.gettext(msg)));
    }

    /** ap_types and num_posarg: a C array of `const char *` into fmt, and its length. */
    private static final class Types {
        BytePtr[] t; // null for the C NULL array; a null entry is a NULL one
        int num;
    }

    private boolean adjustTypes(Types ap, int arg, BytePtr type) {
        if (arg <= 0) {
            fmtError(e_invalid_format_specifier_str, type);
            return false;
        }
        if (ap.t == null || ap.num < arg) {
            BytePtr[] nt = new BytePtr[arg];
            if (ap.t != null) {
                System.arraycopy(ap.t, 0, nt, 0, ap.num);
            }
            ap.t = nt;
            ap.num = arg;
        }
        BytePtr prev = ap.t[arg - 1];
        if (prev != null) {
            if (prev.at(0) == '*' || type.at(0) == '*') {
                BytePtr pt = type;
                if (pt.at(0) == '*') {
                    pt = prev;
                }
                if (pt.at(0) != '*') {
                    switch (pt.at(0)) {
                        case 'd', 'i':
                            break;
                        default:
                            snprintf(ed.iobuff(), ed.emsgIobuffRoom(), ed.gettext(e_positional_num_field_spec_reused_str_str),
                                    arg, formatTypename(prev), formatTypename(type));
                            ed.error(ed.iobuffOr(ed.gettext(e_positional_num_field_spec_reused_str_str)));
                            return false;
                    }
                }
            } else if (formatTypeof(type) != formatTypeof(prev)) {
                snprintf(ed.iobuff(), ed.emsgIobuffRoom(), ed.gettext(e_positional_arg_num_type_inconsistent_str_str),
                        arg, formatTypename(type), formatTypename(prev));
                ed.error(ed.iobuffOr(ed.gettext(e_positional_arg_num_type_inconsistent_str_str)));
                return false;
            }
        }
        ap.t[arg - 1] = type;
        return true;
    }

    private void formatOverflowError(BytePtr pstart) {
        BytePtr p = pstart;
        while (isDigit(p.get())) {
            p = p.add(1);
        }
        long arglen = p.sub(pstart);
        BytePtr argcopy = BytePtr.alloc(arglen + 1);
        Rt.memmove(argcopy, pstart, arglen);
        snprintf(ed.iobuff(), ed.emsgIobuffRoom(), ed.gettext(ed.eValTooLarge()), argcopy);
        ed.error(ed.iobuffOr(ed.gettext(ed.eValTooLarge())));
    }

    /** A cursor into the format, and the number get_unsigned_int reads. */
    private static final class Cur {
        BytePtr p;
        int uj;
    }

    /** get_unsigned_int, with overflow_err false: tvs is always NULL. */
    private boolean getUnsignedInt(BytePtr pstart, Cur c) {
        c.uj = c.p.get() - '0';
        c.p = c.p.add(1);
        while (isDigit(c.p.get()) && Integer.compareUnsigned(c.uj, MAX_ALLOWED_STRING_WIDTH) < 0) {
            c.uj = 10 * c.uj + (c.p.get() - '0');
            c.p = c.p.add(1);
        }
        if (Integer.compareUnsigned(c.uj, MAX_ALLOWED_STRING_WIDTH) > 0) {
            c.uj = MAX_ALLOWED_STRING_WIDTH;
        }
        return true;
    }

    private boolean parseFmtTypes(Types ap, BytePtr fmt) {
        boolean ok = parseFmtTypes0(ap, fmt);
        if (!ok) {
            ap.t = null;
            ap.num = 0;
        }
        return ok;
    }

    private boolean mixed(boolean anyPos, boolean anyArg, BytePtr fmt) {
        if (anyPos && anyArg) {
            fmtError(e_cannot_mix_positional_and_non_positional_str, fmt);
            return true;
        }
        return false;
    }

    private boolean parseFmtTypes0(Types ap, BytePtr fmt) {
        BytePtr arg;
        boolean any_pos = false, any_arg = false;
        Cur c = new Cur();
        c.p = fmt;

        if (fmt == null) {
            return true;
        }
        while (c.p.get() != 0) {
            if (c.p.get() != '%') {
                BytePtr q = strchr(c.p.add(1), (byte) '%');
                long n = q == null ? strlen(c.p) : q.sub(c.p);
                c.p = c.p.add((int) n);
                continue;
            }
            byte length_modifier;
            int pos_arg = -1;
            BytePtr ptype;
            BytePtr pstart = c.p.add(1);

            c.p = c.p.add(1);
            ptype = c.p;
            while (isDigit(ptype.get())) {
                ptype = ptype.add(1);
            }

            if (ptype.get() == '$') {
                if (c.p.get() == '0') {
                    fmtError(e_invalid_format_specifier_str, fmt);
                    return false;
                }
                if (!getUnsignedInt(pstart, c)) {
                    return false;
                }
                pos_arg = c.uj;
                any_pos = true;
                if (mixed(any_pos, any_arg, fmt)) {
                    return false;
                }
                c.p = c.p.add(1);
            }

            while (c.p.get() == '0' || c.p.get() == '-' || c.p.get() == '+' || c.p.get() == ' '
                    || c.p.get() == '#' || c.p.get() == '\'') {
                c.p = c.p.add(1);
            }

            arg = c.p;
            if (arg.get() == '*') {
                c.p = c.p.add(1);
                if (isDigit(c.p.get())) {
                    if (!getUnsignedInt(arg.add(1), c)) {
                        return false;
                    }
                    if (c.p.get() != '$') {
                        fmtError(e_invalid_format_specifier_str, fmt);
                        return false;
                    }
                    c.p = c.p.add(1);
                    any_pos = true;
                    if (mixed(any_pos, any_arg, fmt)) {
                        return false;
                    }
                    if (!adjustTypes(ap, c.uj, arg)) {
                        return false;
                    }
                } else {
                    any_arg = true;
                    if (mixed(any_pos, any_arg, fmt)) {
                        return false;
                    }
                }
            } else if (isDigit(c.p.get())) {
                if (!getUnsignedInt(c.p, c)) {
                    return false;
                }
                if (c.p.get() == '$') {
                    fmtError(e_invalid_format_specifier_str, fmt);
                    return false;
                }
            }

            if (c.p.get() == '.') {
                c.p = c.p.add(1);
                arg = c.p;
                if (arg.get() == '*') {
                    c.p = c.p.add(1);
                    if (isDigit(c.p.get())) {
                        if (!getUnsignedInt(arg.add(1), c)) {
                            return false;
                        }
                        if (c.p.get() == '$') {
                            any_pos = true;
                            if (mixed(any_pos, any_arg, fmt)) {
                                return false;
                            }
                            c.p = c.p.add(1);
                            if (!adjustTypes(ap, c.uj, arg)) {
                                return false;
                            }
                        } else {
                            fmtError(e_invalid_format_specifier_str, fmt);
                            return false;
                        }
                    } else {
                        any_arg = true;
                        if (mixed(any_pos, any_arg, fmt)) {
                            return false;
                        }
                    }
                } else if (isDigit(c.p.get())) {
                    if (!getUnsignedInt(c.p, c)) {
                        return false;
                    }
                    if (c.p.get() == '$') {
                        fmtError(e_invalid_format_specifier_str, fmt);
                        return false;
                    }
                }
            }

            if (pos_arg != -1) {
                any_pos = true;
                if (mixed(any_pos, any_arg, fmt)) {
                    return false;
                }
                ptype = c.p;
            }

            if (c.p.get() == 'h' || c.p.get() == 'l') {
                length_modifier = c.p.get();
                c.p = c.p.add(1);
                if (length_modifier == 'l' && c.p.get() == 'l') {
                    c.p = c.p.add(1);
                }
            }

            switch (c.p.get()) {
                case 'i', '*', 'd', 'u', 'o', 'D', 'U', 'O', 'x', 'X', 'b', 'B', 'c', 's', 'S', 'p':
                    if (pos_arg != -1) {
                        if (!adjustTypes(ap, pos_arg, ptype)) {
                            return false;
                        }
                    } else {
                        any_arg = true;
                        if (mixed(any_pos, any_arg, fmt)) {
                            return false;
                        }
                    }
                    break;
                default:
                    if (pos_arg != -1) {
                        fmtError(e_cannot_mix_positional_and_non_positional_str, fmt);
                        return false;
                    }
                    break;
            }

            if (c.p.get() != 0) {
                c.p = c.p.add(1);
            }
        }

        for (int arg_idx = 0; arg_idx < ap.num; arg_idx++) {
            if (ap.t[arg_idx] == null) {
                fmtError(e_fmt_arg_nr_unused_str, arg_idx + 1, fmt);
                return false;
            }
        }
        return true;
    }

    /** arg_idx and arg_cur, which skip_to_arg moves together. */
    private static final class Idx {
        int argIdx = 1;
        int argCur;
    }

    private void skipToArg(Types ap, Va va, Idx x, BytePtr fmt) {
        int arg_min = 0;

        if (x.argCur + 1 == x.argIdx) {
            x.argCur++;
            x.argIdx++;
            return;
        }
        if (x.argCur >= x.argIdx) {
            va.i = 0; // va_end(*ap); va_copy(*ap, ap_start)
        } else {
            arg_min = x.argCur;
        }
        for (x.argCur = arg_min; x.argCur < x.argIdx - 1; x.argCur++) {
            // DEVIATION (bounds), as the Go's: C would read past the end of
            // ap_types here; an index past it is a NULL entry.
            if (ap.t == null || x.argCur >= ap.t.length || ap.t[x.argCur] == null) {
                snprintf(ed.iobuff(), ed.emsgIobuffRoom(), e_aptypes_is_null_nr_str, x.argCur, fmt);
                ed.internalError(ed.iobuffOr(e_aptypes_is_null_nr_str));
                return;
            }
            switch (formatTypeof(ap.t[x.argCur])) {
                case TYPE_PERCENT, TYPE_UNKNOWN -> {}
                default -> va.vaSkip(); // every other type is one va_arg, whatever its width
            }
        }
        x.argCur++;
        x.argIdx++;
    }

    /** Write n bytes of src at str + str_l, as far as str_m lets them. */
    private static void put(BytePtr str, long str_l, long str_m, BytePtr src, long n) {
        if (ult(str_l, str_m)) {
            Rt.memmove(str.add((int) str_l), src, umin(n, str_m - str_l));
        }
    }

    private static void fill(BytePtr str, long str_l, long str_m, int c, long n) {
        if (ult(str_l, str_m)) {
            Rt.memset(str.add((int) str_l), c, umin(n, str_m - str_l));
        }
    }

    /** vim_snprintf(str, str_m, fmt, ...): the length the whole would have. */
    public int snprintf(BytePtr str, long str_m, BytePtr fmt, Object... args) {
        long str_l = 0;
        BytePtr p = fmt;
        Types ap_types = new Types();
        Idx x = new Idx();

        if (!parseFmtTypes(ap_types, fmt)) {
            return 0;
        }
        Va ap = new Va(args);

        if (p == null) {
            p = BytePtr.lit("");
        }
        Cur c = new Cur();
        while (p.get() != 0) {
            if (p.get() != '%') {
                BytePtr q = strchr(p.add(1), (byte) '%');
                long n = q == null ? strlen(p) : q.sub(p);
                put(str, str_l, str_m, p, n);
                p = p.add((int) n);
                str_l += n;
                continue;
            }

            long min_field_width = 0;
            long precision = 0;
            boolean zero_padding = false;
            boolean precision_specified = false;
            boolean justify_left = false;
            boolean alternate_form = false;
            boolean force_sign = false;
            boolean space_for_positive = true;
            byte length_modifier = 0;
            BytePtr str_arg = null;
            long str_arg_l = 0;
            long number_of_zeros_to_pad = 0;
            long zero_padding_insertion_ind = 0;
            byte fmt_spec;
            int pos_arg = -1;
            BytePtr ptype;

            p = p.add(1);
            ptype = p;
            while (isDigit(ptype.get())) {
                ptype = ptype.add(1);
            }

            if (ptype.get() == '$') {
                c.p = p;
                if (!getUnsignedInt(p, c)) {
                    return (int) str_l;
                }
                p = c.p;
                pos_arg = c.uj;
                p = p.add(1);
            }

            while (p.get() == '0' || p.get() == '-' || p.get() == '+' || p.get() == ' '
                    || p.get() == '#' || p.get() == '\'') {
                switch (p.get()) {
                    case '0' -> zero_padding = true;
                    case '-' -> justify_left = true;
                    case '+' -> {
                        force_sign = true;
                        space_for_positive = false;
                    }
                    case ' ' -> force_sign = true;
                    case '#' -> alternate_form = true;
                    default -> {}
                }
                p = p.add(1);
            }

            if (p.get() == '*') {
                BytePtr digstart = p.add(1);
                p = p.add(1);
                if (isDigit(p.get())) {
                    c.p = p;
                    if (!getUnsignedInt(digstart, c)) {
                        return (int) str_l;
                    }
                    p = c.p;
                    x.argIdx = c.uj;
                    p = p.add(1);
                }
                skipToArg(ap_types, ap, x, fmt);
                int j = ap.vaInt();
                if (j > MAX_ALLOWED_STRING_WIDTH) {
                    j = MAX_ALLOWED_STRING_WIDTH;
                }
                if (j >= 0) {
                    min_field_width = j;
                } else {
                    min_field_width = (long) -j; // usize(-j): MIN_INT stays negative, as in C
                    justify_left = true;
                }
            } else if (isDigit(p.get())) {
                c.p = p;
                if (!getUnsignedInt(p, c)) {
                    return (int) str_l;
                }
                p = c.p;
                min_field_width = c.uj & 0xffffffffL;
            }

            if (p.get() == '.') {
                p = p.add(1);
                precision_specified = true;
                if (isDigit(p.get())) {
                    c.p = p;
                    if (!getUnsignedInt(p, c)) {
                        return (int) str_l;
                    }
                    p = c.p;
                    precision = c.uj & 0xffffffffL;
                } else if (p.get() == '*') {
                    BytePtr digstart = p;
                    p = p.add(1);
                    if (isDigit(p.get())) {
                        c.p = p;
                        if (!getUnsignedInt(digstart, c)) {
                            return (int) str_l;
                        }
                        p = c.p;
                        x.argIdx = c.uj;
                        p = p.add(1);
                    }
                    skipToArg(ap_types, ap, x, fmt);
                    int j = ap.vaInt();
                    if (j > MAX_ALLOWED_STRING_WIDTH) {
                        j = MAX_ALLOWED_STRING_WIDTH;
                    }
                    if (j >= 0) {
                        precision = j;
                    } else {
                        precision_specified = false;
                        precision = 0;
                    }
                }
            }

            if (p.get() == 'h' || p.get() == 'l') {
                length_modifier = p.get();
                p = p.add(1);
                if (length_modifier == 'l' && p.get() == 'l') {
                    length_modifier = 'L';
                    p = p.add(1);
                }
            }
            fmt_spec = p.get();

            switch (fmt_spec) {
                case 'i' -> fmt_spec = 'd';
                case 'D' -> {
                    fmt_spec = 'd';
                    length_modifier = 'l';
                }
                case 'U' -> {
                    fmt_spec = 'u';
                    length_modifier = 'l';
                }
                case 'O' -> {
                    fmt_spec = 'o';
                    length_modifier = 'l';
                }
                default -> {}
            }

            if (pos_arg != -1) {
                x.argIdx = pos_arg;
            }

            switch (fmt_spec) {
                case '%', 'c', 's', 'S': {
                    str_arg_l = 1;
                    switch (fmt_spec) {
                        case '%':
                            str_arg = p;
                            break;
                        case 'c': {
                            skipToArg(ap_types, ap, x, fmt);
                            int j = ap.vaInt();
                            str_arg = BytePtr.alloc(1);
                            str_arg.put((byte) j);
                            break;
                        }
                        default: { // 's', 'S'
                            skipToArg(ap_types, ap, x, fmt);
                            str_arg = ap.vaString();
                            if (str_arg == null) {
                                str_arg = BytePtr.lit("[NULL]");
                                str_arg_l = 6;
                            } else if (!precision_specified) {
                                str_arg_l = strlen(str_arg);
                            } else if (precision == 0) {
                                str_arg_l = 0;
                            } else {
                                long lim = umin(precision, 0x7fffffffL);
                                BytePtr q = memchr(str_arg, 0, lim);
                                str_arg_l = q == null ? precision : q.sub(str_arg);
                            }
                            if (fmt_spec == 'S') {
                                long i = 0;
                                BytePtr p1 = str_arg;
                                for (; p1.get() != 0; p1 = p1.add(ed.utfcPtr2len(p1))) {
                                    int cell = ed.utfPtr2cells(p1);
                                    if (precision_specified && ult(precision, i + cell)) {
                                        break;
                                    }
                                    i += cell;
                                }
                                str_arg_l = p1.sub(str_arg);
                                if (min_field_width != 0) {
                                    min_field_width += str_arg_l - i;
                                }
                            }
                            break;
                        }
                    }
                    break;
                }

                case 'd', 'u', 'b', 'B', 'o', 'x', 'X', 'p': {
                    int arg_sign = 0;
                    int int_arg = 0;
                    int uint_arg = 0;
                    long long_arg = 0;
                    long ulong_arg = 0;
                    long llong_arg = 0;
                    long ullong_arg = 0;
                    long bin_arg = 0;
                    long ptr_arg = 0;
                    BytePtr tmp = BytePtr.alloc(TMP_LEN);

                    if (fmt_spec == 'p') {
                        length_modifier = 0;
                        skipToArg(ap_types, ap, x, fmt);
                        ptr_arg = ap.vaPointer();
                        if (ptr_arg != 0) {
                            arg_sign = 1;
                        }
                    } else if (fmt_spec == 'b' || fmt_spec == 'B') {
                        skipToArg(ap_types, ap, x, fmt);
                        bin_arg = ap.vaUlong();
                        if (bin_arg != 0) {
                            arg_sign = 1;
                        }
                    } else if (fmt_spec == 'd') {
                        switch (length_modifier) {
                            case 0, 'h' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                int_arg = ap.vaInt();
                                arg_sign = Integer.signum(int_arg);
                            }
                            case 'l' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                long_arg = ap.vaLong();
                                arg_sign = Long.signum(long_arg);
                            }
                            case 'L' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                llong_arg = ap.vaLong();
                                arg_sign = Long.signum(llong_arg);
                            }
                            default -> {}
                        }
                    } else {
                        switch (length_modifier) {
                            case 0, 'h' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                uint_arg = ap.vaUint();
                                if (uint_arg != 0) {
                                    arg_sign = 1;
                                }
                            }
                            case 'l' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                ulong_arg = ap.vaUlong();
                                if (ulong_arg != 0) {
                                    arg_sign = 1;
                                }
                            }
                            case 'L' -> {
                                skipToArg(ap_types, ap, x, fmt);
                                ullong_arg = ap.vaUlong();
                                if (ullong_arg != 0) {
                                    arg_sign = 1;
                                }
                            }
                            default -> {}
                        }
                    }

                    str_arg = tmp;
                    str_arg_l = 0;

                    if (precision_specified) {
                        zero_padding = false;
                    }
                    if (fmt_spec == 'd') {
                        if (force_sign && arg_sign >= 0) {
                            tmp.set((int) str_arg_l++, (byte) (space_for_positive ? ' ' : '+'));
                        }
                    } else if (alternate_form) {
                        if (arg_sign != 0 && (fmt_spec == 'b' || fmt_spec == 'B' || fmt_spec == 'x' || fmt_spec == 'X')) {
                            tmp.set((int) str_arg_l++, (byte) '0');
                            tmp.set((int) str_arg_l++, fmt_spec);
                        }
                    }

                    zero_padding_insertion_ind = str_arg_l;
                    if (!precision_specified) {
                        precision = 1;
                    }
                    if (precision == 0 && arg_sign == 0) {
                        // the C leaves this branch empty: no digits for a zero
                    } else {
                        BytePtr at = tmp.add((int) str_arg_l);
                        if (fmt_spec == 'p') {
                            str_arg_l += fmtptr(at, ptr_arg);
                        } else if (fmt_spec == 'b' || fmt_spec == 'B') {
                            byte[] b = new byte[64];
                            int b_l = 0;
                            long bn = bin_arg;
                            do {
                                b_l++;
                                b[b.length - b_l] = (byte) ('0' + (bn & 0x1));
                                bn >>>= 1;
                            } while (bn != 0);
                            for (int k = 0; k < b_l; k++) {
                                at.set(k, b[b.length - b_l + k]);
                            }
                            str_arg_l += b_l;
                        } else if (fmt_spec == 'd') {
                            switch (length_modifier) {
                                case 0 -> str_arg_l += fmtnum(at, int_arg, 10, false, int_arg < 0);
                                case 'h' -> str_arg_l += fmtnum(at, (short) int_arg, 10, false, (short) int_arg < 0);
                                case 'l' -> str_arg_l += fmtnum(at, long_arg, 10, false, long_arg < 0);
                                case 'L' -> str_arg_l += fmtnum(at, llong_arg, 10, false, llong_arg < 0);
                                default -> {}
                            }
                        } else {
                            int base = fmtbase(fmt_spec);
                            boolean upper = fmt_spec == 'X';
                            switch (length_modifier) {
                                case 0 -> str_arg_l += fmtnum(at, uint_arg & 0xffffffffL, base, upper, false);
                                case 'h' -> str_arg_l += fmtnum(at, uint_arg & 0xffffL, base, upper, false);
                                case 'l' -> str_arg_l += fmtnum(at, ulong_arg, base, upper, false);
                                case 'L' -> str_arg_l += fmtnum(at, ullong_arg, base, upper, false);
                                default -> {}
                            }
                        }

                        if (zero_padding_insertion_ind < str_arg_l && tmp.at((int) zero_padding_insertion_ind) == '-') {
                            zero_padding_insertion_ind++;
                        }
                        if (zero_padding_insertion_ind + 1 < str_arg_l && tmp.at((int) zero_padding_insertion_ind) == '0'
                                && (tmp.at((int) zero_padding_insertion_ind + 1) == 'x'
                                        || tmp.at((int) zero_padding_insertion_ind + 1) == 'X')) {
                            zero_padding_insertion_ind += 2;
                        }
                    }

                    {
                        long num_of_digits = str_arg_l - zero_padding_insertion_ind;
                        if (alternate_form && fmt_spec == 'o'
                                && !(zero_padding_insertion_ind < str_arg_l && tmp.at((int) zero_padding_insertion_ind) == '0')) {
                            if (!precision_specified || ult(precision, num_of_digits + 1)) {
                                precision = num_of_digits + 1;
                            }
                        }
                        if (ult(num_of_digits, precision)) {
                            number_of_zeros_to_pad = precision - num_of_digits;
                        }
                    }
                    if (!justify_left && zero_padding) {
                        int n = (int) (min_field_width - (str_arg_l + number_of_zeros_to_pad));
                        if (n > 0) {
                            number_of_zeros_to_pad += n;
                        }
                    }
                    break;
                }

                default:
                    zero_padding = false;
                    justify_left = true;
                    min_field_width = 0;
                    str_arg = p;
                    str_arg_l = 0;
                    if (p.get() != 0) {
                        str_arg_l++;
                    }
                    break;
            }

            if (p.get() != 0) {
                p = p.add(1);
            }

            if (!justify_left) {
                int pn = (int) (min_field_width - (str_arg_l + number_of_zeros_to_pad));
                if (pn > 0) {
                    fill(str, str_l, str_m, zero_padding ? '0' : ' ', pn);
                    str_l += pn;
                }
            }

            if (number_of_zeros_to_pad == 0) {
                zero_padding_insertion_ind = 0;
            } else {
                int zn = (int) zero_padding_insertion_ind;
                if (zn > 0) {
                    put(str, str_l, str_m, str_arg, zn);
                    str_l += zn;
                }
                zn = (int) number_of_zeros_to_pad;
                if (zn > 0) {
                    fill(str, str_l, str_m, '0', zn);
                    str_l += zn;
                }
            }

            {
                int sn = (int) (str_arg_l - zero_padding_insertion_ind);
                if (sn > 0) {
                    put(str, str_l, str_m, str_arg.add((int) zero_padding_insertion_ind), sn);
                    str_l += sn;
                }
            }

            if (justify_left) {
                int pn = (int) (min_field_width - (str_arg_l + number_of_zeros_to_pad));
                if (pn > 0) {
                    fill(str, str_l, str_m, ' ', pn);
                    str_l += pn;
                }
            }
        }

        if (str_m > 0 || str_m < 0) { // str_m != 0, unsigned
            long k = ult(str_m - 1, str_l) ? str_m - 1 : str_l;
            str.set((int) k, (byte) 0);
        }
        return (int) str_l;
    }
}
