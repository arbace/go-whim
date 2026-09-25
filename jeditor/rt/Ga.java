package whim.rt;

import java.util.function.IntFunction;

/**
 * A growable array's storage: C's {@code void *ga_data}, held as an
 * {@code Object} -- one of the pointer classes over an allocation of its own
 * -- and typed where the C casts it, as the Go's {@code GaData[T]} is.  The
 * element type is known only there, so the storage is made there, lazily, at
 * the first typed access, as large as the array has asked for (its
 * {@code ga_maxlen}); and grown there when it has asked for more since.  The
 * generated class writes the result back into the array.
 *
 * <p>A growarray used as two element types is C's own error; here it is a
 * {@code ClassCastException}.
 */
public final class Ga {
    private Ga() {}

    private static int want(int maxlen) {
        return Math.max(maxlen, 1);
    }

    /** ((char *)ga_data): bytes. */
    public static BytePtr bytes(Object d, int maxlen) {
        int n = want(maxlen);
        if (d == null) {
            return BytePtr.alloc(n);
        }
        BytePtr p = (BytePtr) d;
        if (p.len() < n) {
            byte[] a = new byte[n];
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new BytePtr(a, 0);
        }
        return p;
    }

    /** ((short *)ga_data) */
    public static ShortPtr shorts(Object d, int maxlen) {
        int n = want(maxlen);
        if (d == null) {
            return ShortPtr.alloc(n);
        }
        ShortPtr p = (ShortPtr) d;
        if (p.len() < n) {
            short[] a = new short[n];
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new ShortPtr(a, 0);
        }
        return p;
    }

    /** ((int *)ga_data) */
    public static IntPtr ints(Object d, int maxlen) {
        int n = want(maxlen);
        if (d == null) {
            return IntPtr.alloc(n);
        }
        IntPtr p = (IntPtr) d;
        if (p.len() < n) {
            int[] a = new int[n];
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new IntPtr(a, 0);
        }
        return p;
    }

    /** ((long *)ga_data) */
    public static LongPtr longs(Object d, int maxlen) {
        int n = want(maxlen);
        if (d == null) {
            return LongPtr.alloc(n);
        }
        LongPtr p = (LongPtr) d;
        if (p.len() < n) {
            long[] a = new long[n];
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new LongPtr(a, 0);
        }
        return p;
    }

    /** ((bool *)ga_data) */
    public static BoolPtr bools(Object d, int maxlen) {
        int n = want(maxlen);
        if (d == null) {
            return BoolPtr.alloc(n);
        }
        BoolPtr p = (BoolPtr) d;
        if (p.len() < n) {
            boolean[] a = new boolean[n];
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new BoolPtr(a, 0);
        }
        return p;
    }

    /**
     * ((T *)ga_data) for a T Java holds by reference -- a struct, whose
     * elements mk makes (each a zeroed object), or a pointer (null).  Grown,
     * the old elements keep their objects and the new ones are mk's.
     */
    @SuppressWarnings("unchecked")
    public static <T> Ptr<T> ptrs(Object d, int maxlen, IntFunction<Object[]> mk) {
        int n = want(maxlen);
        if (d == null) {
            return new Ptr<>(mk.apply(n), 0);
        }
        Ptr<T> p = (Ptr<T>) d;
        if (p.len() < n) {
            Object[] a = mk.apply(n);
            System.arraycopy(p.a, p.i, a, 0, p.len());
            return new Ptr<>(a, 0);
        }
        return p;
    }
}
