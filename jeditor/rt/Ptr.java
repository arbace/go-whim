package whim.rt;

/**
 * A C pointer that walks, to anything Java holds by reference: a struct (its
 * class), or a pointer (a BytePtr, a Ptr, a struct reference) -- an
 * {@code Object[]} and an offset.  Over an array of structs, {@code get()} is
 * the struct itself, whose fields are read and written in place; over an
 * array of pointers, {@code put} replaces one.  Java's arrays of generic type
 * cannot be made, so the storage is Object[], and a T[] passes as one.
 * Immutable; NULL is {@code null}.
 */
public final class Ptr<T> {
    public final Object[] a;
    public final int i;

    public Ptr(Object[] a, int i) {
        this.a = a;
        this.i = i;
    }

    public static <T> Ptr<T> of(Object[] a) {
        return new Ptr<>(a, 0);
    }

    /** &x for an x held by reference: a one-element array that is x's only slot. */
    public static <T> Ptr<T> one(T x) {
        return x == null ? null : new Ptr<>(new Object[] {x}, 0);
    }

    /** *p for a p that may be NULL: the plain reference a pointer that walks nowhere holds. */
    public static <T> T ref(Ptr<T> p) {
        return p == null ? null : p.get();
    }

    @SuppressWarnings("unchecked")
    public T get() { return (T) a[i]; }

    @SuppressWarnings("unchecked")
    public T at(int k) { return (T) a[i + k]; }

    public T put(T v) { a[i] = v; return v; }
    public T set(int k, T v) { a[i + k] = v; return v; }

    public Ptr<T> add(int k) {
        return k == 0 ? this : new Ptr<>(a, i + k);
    }

    public long sub(Ptr<T> q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    /**
     * p == &amp;x for an x held as a plain reference: p points at x itself -- a
     * pointer outside its array points at no object, and is not x.
     */
    public static boolean is(Ptr<?> p, Object x) {
        if (p == null || x == null) {
            return p == null && x == null;
        }
        return p.i >= 0 && p.i < p.a.length && p.a[p.i] == x;
    }

    public static boolean eq(Ptr<?> p, Ptr<?> q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(Ptr<T> q) { return sub(q) < 0; }
    public boolean le(Ptr<T> q) { return sub(q) <= 0; }
    public boolean gt(Ptr<T> q) { return sub(q) > 0; }
    public boolean ge(Ptr<T> q) { return sub(q) >= 0; }

    public int len() { return a.length - i; }
}
