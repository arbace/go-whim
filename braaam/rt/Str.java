package whim.rt;

/**
 * The C string functions the core calls, on the runtime's bytes instead of
 * translated from musl a byte and a {@link BytePtr} at a time: what
 * editor/libc.go is to the Go editor.  Their contracts are musl's exactly --
 * a comparison answers the difference of the first bytes that differ, read
 * unsigned, not only its sign; strchr finds the terminator when asked for 0;
 * strcpy and strncpy copy forward, a byte at a time, as the C's loops do even
 * where the two overlap.  A found pointer at the start of the string is the
 * string itself ({@code p.add(0) == p}).  SelfTest holds each to the C's
 * loop, transcribed.
 */
public final class Str {
    private Str() {
    }

    /** strlen(s): the bytes before the first NUL. */
    public static long strlen(BytePtr s) {
        byte[] a = s.a;
        int i = s.i, k = i;
        while (a[k] != 0) {
            k++;
        }
        return k - i;
    }

    /** strcpy(dest, src): src and its NUL at dest, forward; dest. */
    public static BytePtr strcpy(BytePtr dest, BytePtr src) {
        byte[] d = dest.a, s = src.a;
        int i = dest.i, j = src.i;
        byte c;
        do {
            c = s[j++];
            d[i++] = c;
        } while (c != 0);
        return dest;
    }

    /** strncpy(dest, src, n): at most n bytes of src, the rest of n NULs; dest. */
    public static BytePtr strncpy(BytePtr dest, BytePtr src, long n) {
        byte[] d = dest.a, s = src.a;
        int i = dest.i, j = src.i;
        for (; n != 0 && s[j] != 0; n--) {
            d[i++] = s[j++];
        }
        for (; n != 0; n--) {
            d[i++] = 0;
        }
        return dest;
    }

    /** strcat(dest, src): src at dest's NUL; dest. */
    public static BytePtr strcat(BytePtr dest, BytePtr src) {
        strcpy(dest.add((int) strlen(dest)), src);
        return dest;
    }

    /** strcmp(l, r): the difference of the first bytes that differ, unsigned. */
    public static int strcmp(BytePtr l, BytePtr r) {
        byte[] a = l.a, b = r.a;
        int i = l.i, j = r.i;
        while (a[i] == b[j] && a[i] != 0) {
            i++;
            j++;
        }
        return (a[i] & 0xff) - (b[j] & 0xff);
    }

    /** strncmp(ls, rs, n): strcmp of at most n bytes; 0 for none. */
    public static int strncmp(BytePtr ls, BytePtr rs, long n) {
        if (n-- == 0) {
            return 0;
        }
        byte[] a = ls.a, b = rs.a;
        int i = ls.i, j = rs.i;
        while (a[i] != 0 && b[j] != 0 && n != 0 && a[i] == b[j]) {
            i++;
            j++;
            n--;
        }
        return (a[i] & 0xff) - (b[j] & 0xff);
    }

    /** strchr(s, c): the first (unsigned char) c in s, its NUL included; null. */
    public static BytePtr strchr(BytePtr s, long c) {
        byte ch = (byte) c;
        byte[] a = s.a;
        int k = s.i;
        while (a[k] != 0 && a[k] != ch) {
            k++;
        }
        return a[k] == ch ? s.add(k - s.i) : null;
    }

    /** strstr(h, n): the first place n is in h; h for an empty n; null. */
    public static BytePtr strstr(BytePtr h, BytePtr n) {
        byte[] a = h.a, b = n.a;
        if (b[n.i] == 0) {
            return h;
        }
        for (int k = h.i; a[k] != 0; k++) {
            int i = 0;
            while (b[n.i + i] != 0 && a[k + i] == b[n.i + i]) {
                i++;
            }
            if (b[n.i + i] == 0) {
                return h.add(k - h.i);
            }
        }
        return null;
    }

    /** strpbrk(s, b): the first byte of s that is in b; null. */
    public static BytePtr strpbrk(BytePtr s, BytePtr b) {
        byte[] a = s.a, set = b.a;
        for (int k = s.i; a[k] != 0; k++) {
            for (int c = b.i; set[c] != 0; c++) {
                if (a[k] == set[c]) {
                    return s.add(k - s.i);
                }
            }
        }
        return null;
    }
}
