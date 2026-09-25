package whim.rt;

import java.util.Arrays;

/**
 * What the generated code calls that is no pointer's: the functions of bytes
 * (memmove, memset, memcmp), the initial value of a char array, and a value
 * computed for what it does and then dropped -- C's expression statement,
 * which Java allows only for a call or an assignment.
 */
public final class Rt {
    private Rt() {}

    /** memmove and memcpy: n bytes from s to d, overlapping or not; d. */
    public static BytePtr memmove(BytePtr d, BytePtr s, long n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, (int) n);
        }
        return d;
    }

    /** memset: n bytes of c's low byte from d; d. */
    public static BytePtr memset(BytePtr d, int c, long n) {
        if (n > 0) {
            Arrays.fill(d.a, d.i, d.i + (int) n, (byte) c);
        }
        return d;
    }

    /** memcmp: the first differing byte, unsigned, decides: -1, 0 or 1. */
    public static int memcmp(BytePtr p, BytePtr q, long n) {
        for (int k = 0; k < n; k++) {
            int x = p.a[p.i + k] & 0xff, y = q.a[q.i + k] & 0xff;
            if (x != y) {
                return x < y ? -1 : 1;
            }
        }
        return 0;
    }

    /**
     * A char array's initial value: s's chars as bytes from its start, and
     * NULs to its end -- what C does for {@code char a[N] = "..."}.
     */
    public static byte[] init(byte[] a, String s) {
        int n = Math.min(a.length, s.length());
        for (int k = 0; k < n; k++) {
            a[k] = (byte) s.charAt(k);
        }
        Arrays.fill(a, n, a.length, (byte) 0);
        return a;
    }

    /** A value computed for what computing it did. */
    public static void use(long x) {}
    public static void use(boolean x) {}
    public static void use(Object x) {}
}
