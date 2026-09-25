package whim.rt;

/**
 * A C pointer to int, signed or unsigned (the bits): a {@code int[]} and an
 * offset, as {@link BytePtr} is for bytes.  An address-taken local is a
 * one-element array from its declaration on, and its address is one of these
 * over it.  Immutable; NULL is {@code null}.
 */
public final class IntPtr {
    public final int[] a;
    public final int i;

    public IntPtr(int[] a, int i) {
        this.a = a;
        this.i = i;
    }

    public static IntPtr of(int[] a) {
        return new IntPtr(a, 0);
    }

    /** n zeroed elements of storage of their own (at least one). */
    public static IntPtr alloc(long n) {
        return new IntPtr(new int[(int) Math.max(n, 1)], 0);
    }

    public int get() { return a[i]; }
    public int at(int k) { return a[i + k]; }
    public int put(int v) { a[i] = v; return v; }
    public int set(int k, int v) { a[i + k] = v; return v; }

    public IntPtr add(int k) {
        return k == 0 ? this : new IntPtr(a, i + k);
    }

    public long sub(IntPtr q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    public static boolean eq(IntPtr p, IntPtr q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(IntPtr q) { return sub(q) < 0; }
    public boolean le(IntPtr q) { return sub(q) <= 0; }
    public boolean gt(IntPtr q) { return sub(q) > 0; }
    public boolean ge(IntPtr q) { return sub(q) >= 0; }

    public int len() { return a.length - i; }
}
