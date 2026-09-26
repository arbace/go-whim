package whim.rt;

/**
 * A C pointer to long, signed or unsigned (the bits): a {@code long[]} and an
 * offset, as {@link BytePtr} is for bytes.  An address-taken local is a
 * one-element array from its declaration on, and its address is one of these
 * over it.  Immutable; NULL is {@code null}.
 */
public final class LongPtr {
    public final long[] a;
    public final int i;

    public LongPtr(long[] a, int i) {
        this.a = a;
        this.i = i;
    }

    public static LongPtr of(long[] a) {
        return new LongPtr(a, 0);
    }

    /** n zeroed elements of storage of their own (at least one). */
    public static LongPtr alloc(long n) {
        return new LongPtr(new long[(int) Math.max(n, 1)], 0);
    }

    public long get() { return a[i]; }
    public long at(int k) { return a[i + k]; }
    public long put(long v) { a[i] = v; return v; }
    public long set(int k, long v) { a[i + k] = v; return v; }

    public LongPtr add(int k) {
        return k == 0 ? this : new LongPtr(a, i + k);
    }

    public long sub(LongPtr q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    public static boolean eq(LongPtr p, LongPtr q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(LongPtr q) { return sub(q) < 0; }
    public boolean le(LongPtr q) { return sub(q) <= 0; }
    public boolean gt(LongPtr q) { return sub(q) > 0; }
    public boolean ge(LongPtr q) { return sub(q) >= 0; }

    public int len() { return a.length - i; }
}
