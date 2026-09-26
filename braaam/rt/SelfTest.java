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

    /** A struct as the generated classes are: set() copies, zero() clears. */
    private static final class Box implements Struct<Box> {
        int v;

        Box(int v) {
            this.v = v;
        }

        @Override
        public Box set(Box o) {
            v = o.v;
            return this;
        }

        @Override
        public Box zero() {
            v = 0;
            return this;
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

        // the functions of bytes in elements
        IntPtr ip = IntPtr.alloc(5);
        for (int k = 0; k < 5; k++) {
            ip.set(k, k + 1);
        }
        Rt.memmove(ip.add(1), ip, 3); // overlapping
        check(ip.at(0) == 1 && ip.at(1) == 1 && ip.at(3) == 3 && ip.at(4) == 5, "memmove of ints");
        Rt.fill(ip, -1, 2);
        check(ip.at(0) == -1 && ip.at(1) == -1 && ip.at(2) == 2, "a fill of ints");
        Rt.zero(pp, 2);
        check(pp.get() == null && pp.at(1) == null, "a fill of pointers with NULL");
        Box[] boxes = {new Box(1), new Box(2), new Box(3)};
        Ptr<Box> bx = new Ptr<>(boxes, 0);
        Box first = boxes[0];
        Rt.moveStructs(bx.add(1), bx, 2); // overlapping, from the end
        check(boxes[0] == first && boxes[1].v == 1 && boxes[2].v == 2 && boxes[1] != boxes[0], "structs moved are copied");
        Rt.moveStructs(bx, bx.add(1), 2);
        check(boxes[0].v == 1 && boxes[1].v == 2, "structs moved the other way");
        Rt.zeroStructs(bx.add(1), 2);
        check(boxes[0].v == 1 && boxes[1].v == 0 && boxes[2].v == 0, "structs zeroed in place");

        // the growarray's storage, typed where it is used
        Object data = null;
        IntPtr gi = Ga.ints(data, 3);
        check(gi.len() == 3, "a growarray made at its first use");
        gi.set(2, 7);
        IntPtr gi2 = Ga.ints(gi, 6);
        check(gi2.len() == 6 && gi2.at(2) == 7 && gi2 != gi, "a growarray grown keeps its elements");
        check(Ga.ints(gi2, 4) == gi2, "a growarray large enough is itself");
        Ptr<Box> gb = Ga.ptrs(null, 2, Box[]::new);
        check(gb.len() == 2 && gb.get() == null, "a growarray of pointers");
        Box kept = new Box(9);
        gb.put(kept);
        Ptr<Box> gb2 = Ga.ptrs(gb, 3, m -> {
            Box[] a = new Box[m];
            for (int k = 0; k < m; k++) {
                a[k] = new Box(0);
            }
            return a;
        });
        check(gb2.get() == kept && gb2.at(2) != null, "a growarray of structs grown");
        boolean threw = false;
        try {
            Ga.bytes(gi2, 1);
        } catch (ClassCastException e) {
            threw = true;
        }
        check(threw, "a growarray used as two element types");

        // a struct held plainly against a Ptr over structs
        check(Ptr.is(bx.add(1), boxes[1]) && !Ptr.is(bx, boxes[1]) && !Ptr.is(bx.add(-1), boxes[0])
            && !Ptr.is(bx.add(3), boxes[0]) && Ptr.is(null, null) && !Ptr.is(bx, null), "Ptr.is");

        // a void * read back
        check(Rt.obj(bx.add(1)) == boxes[1] && Rt.obj(first) == first, "a void * as the one object");
        check(Rt.ptr(first).get() == first && Rt.ptr(bx) == bx && Rt.ptr(null) == null, "a void * as a Ptr");

        if (failed > 0) {
            System.exit(1);
        }
        System.out.println("ok");
    }
}
