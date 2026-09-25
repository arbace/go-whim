package whim.rt;

/**
 * A C pointer to short, signed or unsigned (the bits): a {@code short[]} and an
 * offset, as {@link BytePtr} is for bytes.  An address-taken local is a
 * one-element array from its declaration on, and its address is one of these
 * over it.  Immutable; NULL is {@code null}.
 */
public final class ShortPtr {
    public final short[] a;
    public final int i;

    public ShortPtr(short[] a, int i) {
        this.a = a;
        this.i = i;
    }

    public static ShortPtr of(short[] a) {
        return new ShortPtr(a, 0);
    }

    /** n zeroed elements of storage of their own (at least one). */
    public static ShortPtr alloc(long n) {
        return new ShortPtr(new short[(int) Math.max(n, 1)], 0);
    }

    public short get() { return a[i]; }
    public short at(int k) { return a[i + k]; }
    public short put(short v) { a[i] = v; return v; }
    public short set(int k, short v) { a[i + k] = v; return v; }

    public ShortPtr add(int k) {
        return k == 0 ? this : new ShortPtr(a, i + k);
    }

    public long sub(ShortPtr q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    public static boolean eq(ShortPtr p, ShortPtr q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(ShortPtr q) { return sub(q) < 0; }
    public boolean le(ShortPtr q) { return sub(q) <= 0; }
    public boolean gt(ShortPtr q) { return sub(q) > 0; }
    public boolean ge(ShortPtr q) { return sub(q) >= 0; }

    public int len() { return a.length - i; }
}
