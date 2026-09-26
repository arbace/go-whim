package whim.rt;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.ConcurrentHashMap;

/**
 * A C pointer to char, signed or unsigned: a {@code byte[]} and an offset
 * into it.  Immutable, as a C pointer value is: {@code p++} is
 * {@code p = p.add(1)}.  NULL is Java's {@code null}; the static methods take
 * it where C does.  A byte holds the bits: an unsigned char is read as
 * {@code b & 0xff}, which the generated code writes where C widens one.
 */
public final class BytePtr {
    /** The allocation, shared with every pointer into it. */
    public final byte[] a;
    /** The offset: C permits one past the end, and so does this. */
    public final int i;

    public BytePtr(byte[] a, int i) {
        this.a = a;
        this.i = i;
    }

    /** A pointer to a's first element: an array that decays. */
    public static BytePtr of(byte[] a) {
        return new BytePtr(a, 0);
    }

    /** n zeroed bytes of storage of their own (at least one). */
    public static BytePtr alloc(long n) {
        return new BytePtr(new byte[(int) Math.max(n, 1)], 0);
    }

    /** *p */
    public byte get() {
        return a[i];
    }

    /** p[k] */
    public byte at(int k) {
        return a[i + k];
    }

    /** *p = v, and v. */
    public byte put(byte v) {
        a[i] = v;
        return v;
    }

    /** p[k] = v, and v. */
    public byte set(int k, byte v) {
        a[i + k] = v;
        return v;
    }

    /** p + k */
    public BytePtr add(int k) {
        return k == 0 ? this : new BytePtr(a, i + k);
    }

    /** p - q, both into one allocation. */
    public long sub(BytePtr q) {
        if (q.a != a) {
            throw new IllegalStateException("pointer difference across allocations");
        }
        return i - q.i;
    }

    /** p == q, either of them NULL. */
    public static boolean eq(BytePtr p, BytePtr q) {
        return p == q || (p != null && q != null && p.a == q.a && p.i == q.i);
    }

    public boolean lt(BytePtr q) { return sub(q) < 0; }
    public boolean le(BytePtr q) { return sub(q) <= 0; }
    public boolean gt(BytePtr q) { return sub(q) > 0; }
    public boolean ge(BytePtr q) { return sub(q) >= 0; }

    /** How many bytes remain from p to the allocation's end. */
    public int len() {
        return a.length - i;
    }

    // --- C strings ----------------------------------------------------------

    private static final ConcurrentHashMap<String, BytePtr> literals = new ConcurrentHashMap<>();

    /**
     * A C string literal: s's chars are bytes (0 to 255), NUL-terminated, one
     * allocation per distinct text, so the same literal twice is the same
     * pointer, as it usually is in C.  Read-only by C's rules, not by Java's.
     */
    public static BytePtr lit(String s) {
        return literals.computeIfAbsent(s, k -> {
            byte[] b = new byte[k.length() + 1];
            for (int j = 0; j < k.length(); j++) {
                b[j] = (byte) k.charAt(j);
            }
            return new BytePtr(b, 0);
        });
    }

    /** strlen(p): the bytes before the first NUL. */
    public static int strlen(BytePtr p) {
        int n = 0;
        while (p.a[p.i + n] != 0) {
            n++;
        }
        return n;
    }

    /** strcmp(p, q), the bytes compared unsigned: -1, 0 or 1. */
    public static int strcmp(BytePtr p, BytePtr q) {
        for (int k = 0;; k++) {
            int x = p.a[p.i + k] & 0xff, y = q.a[q.i + k] & 0xff;
            if (x != y) {
                return x < y ? -1 : 1;
            }
            if (x == 0) {
                return 0;
            }
        }
    }

    /** The NUL-terminated bytes at p, as a Java string of Latin-1 chars (for the host). */
    public static String str(BytePtr p) {
        if (p == null) {
            return "";
        }
        return new String(p.a, p.i, strlen(p), StandardCharsets.ISO_8859_1);
    }

    @Override
    public String toString() {
        return str(this);
    }
}
