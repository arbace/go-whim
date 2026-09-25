package whim.rt;

/**
 * The runtime's own test, with no framework: {@code java whim.rt.SelfTest}
 * prints one line per failure and "ok" when there is none, and exits 1 on a
 * failure.  crefactor/togo's java_test.go compiles and runs it.
 */
public final class SelfTest {
    private static int failed;

    private static void check(boolean ok, String what) {
        if (!ok) {
            System.out.println("FAIL " + what);
            failed++;
        }
    }

    public static void main(String[] args) {
        // literals: NUL-terminated, interned
        BytePtr s = BytePtr.lit("abc");
        check(s == BytePtr.lit("abc"), "a literal twice is one pointer");
        check(s.a.length == 4 && s.at(3) == 0, "a literal is NUL-terminated");
        check(BytePtr.strlen(s) == 3, "strlen");
        check(BytePtr.lit("\377").at(0) == (byte) 0xff && (BytePtr.lit("\377").get() & 0xff) == 255, "a byte above 127");
        check(BytePtr.str(s.add(1)).equals("bc"), "str from an offset");

        // walking, comparing, subtracting
        BytePtr p = s;
        int n = 0;
        while (p.get() != 0) {
            p = p.add(1);
            n++;
        }
        check(n == 3 && p.sub(s) == 3, "a walk to the NUL and its length");
        check(p.gt(s) && s.lt(p) && s.le(s) && p.ge(p), "ordering");
        check(BytePtr.eq(s.add(3), p) && !BytePtr.eq(s, p), "equality is the element");
        check(BytePtr.eq(null, null) && !BytePtr.eq(s, null) && !BytePtr.eq(null, s), "equality with NULL");
        check(s.add(0) == s, "p + 0 is p");
        check(s.add(2).at(-1) == 'b', "a negative index");

        // storage and the functions of bytes
        BytePtr b = BytePtr.alloc(8);
        check(b.len() == 8 && b.get() == 0, "alloc is zeroed");
        Rt.memmove(b, s, 4);
        check(BytePtr.strcmp(b, s) == 0 && b != s, "memmove copies");
        Rt.memmove(b.add(1), b, 3); // overlapping
        check(BytePtr.str(b).equals("aabc"), "memmove overlapping: " + BytePtr.str(b));
        Rt.memset(b, 'x', 2);
        check(BytePtr.str(b).equals("xxbc"), "memset");
        check(Rt.memcmp(BytePtr.lit("a\200"), BytePtr.lit("a\001"), 2) == 1, "memcmp is unsigned");
        check(BytePtr.strcmp(BytePtr.lit("ab"), BytePtr.lit("abc")) == -1, "strcmp of a prefix");
        byte[] arr = new byte[6];
        arr[5] = 9;
        Rt.init(arr, "hi");
        check(arr[0] == 'h' && arr[1] == 'i' && arr[2] == 0 && arr[5] == 0, "a char array's initial value");
        b.put((byte) 'q');
        b.set(1, (byte) 'r');
        check(b.get() == 'q' && b.at(1) == 'r', "put and set");

        // the other element kinds, and an address-taken local
        int[] x = new int[1];
        IntPtr px = new IntPtr(x, 0);
        px.put(-1);
        check(x[0] == -1 && Integer.toUnsignedLong(px.get()) == 4294967295L, "an int through its address");
        LongPtr lp = LongPtr.alloc(3);
        lp.set(2, Long.MIN_VALUE);
        check(lp.add(2).get() == Long.MIN_VALUE && lp.add(2).sub(lp) == 2, "longs");
        ShortPtr sp = ShortPtr.alloc(2);
        sp.put((short) 0xffff);
        check((sp.get() & 0xffff) == 65535 && sp.get() == -1, "an unsigned short's bits");
        BoolPtr bp = BoolPtr.alloc(1);
        bp.put(true);
        check(bp.get(), "bools");

        // pointers to references: structs and pointers
        Object[] slots = new BytePtr[2];
        Ptr<BytePtr> pp = new Ptr<>(slots, 0);
        pp.put(s);
        pp.set(1, s.add(1));
        check(pp.get() == s && pp.add(1).get().get() == 'b', "a pointer to pointers");
        check(Ptr.eq(pp.add(1), new Ptr<BytePtr>(slots, 1)) && !Ptr.eq(pp, null), "Ptr equality");
        check(Ptr.one("o").get().equals("o") && Ptr.one(null) == null && Ptr.ref(null) == null, "one and ref");

        if (failed > 0) {
            System.exit(1);
        }
        System.out.println("ok");
    }
}
