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

    // --- the functions of bytes on what is not bytes, in elements ----------

    /** memmove of n shorts. */
    public static ShortPtr memmove(ShortPtr d, ShortPtr s, int n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, n);
        }
        return d;
    }

    /** memmove of n ints. */
    public static IntPtr memmove(IntPtr d, IntPtr s, int n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, n);
        }
        return d;
    }

    /** memmove of n longs. */
    public static LongPtr memmove(LongPtr d, LongPtr s, int n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, n);
        }
        return d;
    }

    /** memmove of n _Bools. */
    public static BoolPtr memmove(BoolPtr d, BoolPtr s, int n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, n);
        }
        return d;
    }

    /** memmove of n pointers: the references. */
    public static <T> Ptr<T> memmove(Ptr<T> d, Ptr<T> s, int n) {
        if (n > 0) {
            System.arraycopy(s.a, s.i, d.a, d.i, n);
        }
        return d;
    }

    /**
     * memmove of n structs: each copied into the destination's own object,
     * as C copies the bytes -- from the end when the two overlap that way.
     */
    public static <T extends Struct<T>> Ptr<T> moveStructs(Ptr<T> d, Ptr<T> s, int n) {
        if (d.a == s.a && d.i > s.i) {
            for (int k = n - 1; k >= 0; k--) {
                d.at(k).set(s.at(k));
            }
        } else {
            for (int k = 0; k < n; k++) {
                d.at(k).set(s.at(k));
            }
        }
        return d;
    }

    /** memset(p, 0, n * sizeof *p) of structs: each zeroed in place. */
    public static <T extends Struct<T>> Ptr<T> zeroStructs(Ptr<T> p, int n) {
        for (int k = 0; k < n; k++) {
            p.at(k).zero();
        }
        return p;
    }

    /** memset(p, 0, n * sizeof *p) of pointers: NULL. */
    public static <T> Ptr<T> zero(Ptr<T> p, int n) {
        if (n > 0) {
            Arrays.fill(p.a, p.i, p.i + n, null);
        }
        return p;
    }

    /** memset of n shorts, each v: the fill byte repeated through it. */
    public static ShortPtr fill(ShortPtr p, short v, int n) {
        if (n > 0) {
            Arrays.fill(p.a, p.i, p.i + n, v);
        }
        return p;
    }

    /** memset of n ints, each v. */
    public static IntPtr fill(IntPtr p, int v, int n) {
        if (n > 0) {
            Arrays.fill(p.a, p.i, p.i + n, v);
        }
        return p;
    }

    /** memset of n longs, each v. */
    public static LongPtr fill(LongPtr p, long v, int n) {
        if (n > 0) {
            Arrays.fill(p.a, p.i, p.i + n, v);
        }
        return p;
    }

    /** memset of n _Bools, each v. */
    public static BoolPtr fill(BoolPtr p, boolean v, int n) {
        if (n > 0) {
            Arrays.fill(p.a, p.i, p.i + n, v);
        }
        return p;
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

    // --- a void * read back as what it holds --------------------------------

    /**
     * (T *)vp for a struct T held plainly: the object vp holds, or the one a
     * Ptr it holds points at -- a void * forgets which the C's pointer was.
     */
    public static Object obj(Object vp) {
        return vp instanceof Ptr<?> p ? p.get() : vp;
    }

    /** (T *)vp for a T walked by a Ptr: the Ptr vp holds, or one over the object. */
    public static Ptr<?> ptr(Object vp) {
        return vp == null || vp instanceof Ptr<?> ? (Ptr<?>) vp : Ptr.one(vp);
    }

    /** A value computed for what computing it did. */
    public static void use(long x) {}
    public static void use(boolean x) {}
    public static void use(Object x) {}
}
