package whim.rt;

/**
 * A C pointer to _Bool: a {@code boolean[]} and an
 * offset, as {@link BytePtr} is for bytes.  An address-taken local is a
 * one-element array from its declaration on, and its address is one of these
 * over it.  Immutable; NULL is {@code null}.
 */
public final class BoolPtr {
    public final boolean[] a;
    public final int i;

    public BoolPtr(boolean[] a, int i) {
        this.a = a;
        this.i = i;
    }

    public static BoolPtr of(boolean[] a) {
        return new BoolPtr(a, 0);
    }

    /** n zeroed elements of storage of their own (at least one). */
    public static BoolPtr alloc(long n) {
        return new BoolPtr(new boolean[(int) Math.max(n, 1)], 0);
    }

    public boolean get() { return a[i]; }
    public boolean at(int k) { return a[i + k]; }
    public boolean put(boolean v) { a[i] = v; return v; }
    public boolean set(int k, boolean v) { a[i + k] = v; return v; }

    public BoolPtr add(int k) {
        return k == 0 ? this : new BoolPtr(a, i + k);
    }

    public long sub(BoolPtr q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    public static boolean eq(BoolPtr p, BoolPtr q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(BoolPtr q) { return sub(q) < 0; }
    public boolean le(BoolPtr q) { return sub(q) <= 0; }
    public boolean gt(BoolPtr q) { return sub(q) > 0; }
    public boolean ge(BoolPtr q) { return sub(q) >= 0; }

    public int len() { return a.length - i; }
}
