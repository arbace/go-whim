import java.io.IOException;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import whim.host.Exit;
import whim.host.Host;
import whim.host.Printf;
import whim.host.Term;
import whim.rt.BytePtr;
import whim.rt.IntPtr;
import whim.rt.Ptr;

/**
 * The editor in Java: the generated core ({@code Editor}, abstract where the
 * C calls its host) and the glue that makes it whole -- editor/host.go's, in
 * Java.  Each of the C's host functions is a method here in the C's
 * signature, and a line of glue to the {@link Host} the editor was made on,
 * in Java's types.  vim's printf is {@link Printf}, which needs no operating
 * system.  A process holds any number of editors, each on its own Host.
 *
 * <p>It is in the unnamed package because {@code Editor} is, and its host
 * methods are package-private.
 */
final class Whim extends Editor implements Printf.Core {
    private final Host host;
    private final Printf printf = new Printf(this);

    /** An editor on h, its state as the C's file-scope objects start. */
    Whim(Host h) {
        this.host = h;
    }

    /**
     * Run the editor on h with the command line args, args[0] the program's
     * name, and return its exit status: vim_main's, or the code of an
     * {@link Exit} thrown by h.  The arguments are bytes, as C's are.
     */
    static int main(Host h, byte[][] args) {
        try {
            Whim ed = new Whim(h);
            BytePtr[] argv = new BytePtr[args.length + 1];
            for (int i = 0; i < args.length; i++) {
                byte[] z = new byte[args[i].length + 1];
                System.arraycopy(args[i], 0, z, 0, args[i].length);
                argv[i] = new BytePtr(z, 0);
            }
            return ed.vim_main(args.length, new Ptr<BytePtr>(argv, 0));
        } catch (Exit e) {
            return e.code;
        }
    }

    /** main with the arguments as Java strings, encoded as the platform encodes them. */
    static int main(Host h, String[] args) {
        Charset cs = nativeCharset();
        byte[][] b = new byte[args.length][];
        for (int i = 0; i < args.length; i++) {
            b[i] = args[i].getBytes(cs);
        }
        return main(h, b);
    }

    private static Charset nativeCharset() {
        String n = System.getProperty("sun.jnu.encoding");
        try {
            return n == null ? StandardCharsets.UTF_8 : Charset.forName(n);
        } catch (RuntimeException e) {
            return StandardCharsets.UTF_8;
        }
    }

    /**
     * The bytes of the arguments Java decoded into args: the last args.length
     * entries of /proc/self/cmdline, which are exactly what the launcher was
     * handed -- a decoding by the platform's charset is not, for bytes it
     * cannot decode.  Encoded again from the strings when /proc says
     * something else (another system, or a count that does not match).
     */
    static byte[][] argBytes(String[] args) {
        Charset cs = nativeCharset();
        byte[][] out = new byte[args.length][];
        try {
            byte[] c = Files.readAllBytes(Path.of("/proc/self/cmdline"));
            List<byte[]> all = new ArrayList<>();
            int from = 0;
            for (int i = 0; i < c.length; i++) {
                if (c[i] == 0) {
                    all.add(Arrays.copyOfRange(c, from, i));
                    from = i + 1;
                }
            }
            if (all.size() >= args.length) {
                boolean same = true;
                for (int i = 0; i < args.length; i++) {
                    out[i] = all.get(all.size() - args.length + i);
                    same &= new String(out[i], cs).equals(args[i]);
                }
                if (same) {
                    return out;
                }
            }
        } catch (IOException | RuntimeException e) {
            // no /proc: encode them again
        }
        for (int i = 0; i < args.length; i++) {
            out[i] = args[i].getBytes(cs);
        }
        return out;
    }

    /**
     * The launcher: the editor on the terminal, as bin/whim is.  The program's
     * name is the system property whim.argv0 (the launcher script's $0), since
     * Java hands main none.  The core runs on a thread of its own with a stack
     * as large as a C process's could grow: vim recurses (the regexp engine,
     * the syntax of an expression) and an interpreted frame is larger than a
     * compiled one.
     */
    public static void main(String[] args) throws InterruptedException {
        byte[][] argv = new byte[args.length + 1][];
        argv[0] = System.getProperty("whim.argv0", "braaam").getBytes(nativeCharset());
        System.arraycopy(argBytes(args), 0, argv, 1, args.length);
        int[] status = {0};
        Thread core = new Thread(null, () -> {
            try {
                status[0] = main(new Term(), argv);
            } catch (Throwable t) {
                // the editor failed: say where, as a crash would, and fail
                System.err.println("braaam: " + t);
                StackTraceElement[] st = t.getStackTrace();
                for (int i = 0; i < Math.min(st.length, 12); i++) {
                    System.err.println("\tat " + st[i]);
                }
                status[0] = 70;
            }
        }, "whim", 1L << 30);
        core.start();
        core.join();
        System.exit(status[0]);
    }

    // --- the glue: the host functions the core calls, in the C's signatures

    @Override
    void musl_host_init() {
        host.init(this::deathtrap);
    }

    @Override
    int musl_get_winsize(IntPtr rows, IntPtr cols) {
        int[] ws = host.winSize();
        if (ws == null) {
            return FAIL;
        }
        rows.put(ws[0]);
        cols.put(ws[1]);
        return OK;
    }

    @Override
    void musl_term_start() {
        host.termStart();
    }

    @Override
    void musl_term_stop() {
        host.termStop();
    }

    @Override
    int musl_tty_keys(int fd, IntPtr bs, IntPtr intr, IntPtr cr, IntPtr nlcr) {
        Host.TtyKeys k = host.ttyKeys(fd);
        if (k == null) {
            return FAIL;
        }
        bs.put(k.erase());
        intr.put(k.intr());
        cr.put(k.icrnl() ? 1 : 0);
        nlcr.put(k.onlcr() ? 1 : 0);
        return OK;
    }

    @Override
    long musl_now_ms() {
        return host.nowMs();
    }

    @Override
    long host_time() {
        return host.time();
    }

    @Override
    void musl_delay(long ms, int interruptible) {
        host.delay(ms, interruptible != 0);
    }

    @Override
    int musl_wait_for_input(long ms) {
        return host.waitForInput(ms) ? 1 : 0;
    }

    /**
     * The host gets no room for a negative length, and -1 is the answer for
     * it, as read(2) of (size_t)len gives; the host still takes the signals
     * it reads as input, as the C did before its read.
     */
    @Override
    int musl_read_input(BytePtr buf, int len) {
        int n = host.readInput(buf.a, buf.i, Math.max(len, 0));
        return len < 0 ? -1 : n;
    }

    @Override
    void host_raise(int sig) {
        host.raise(sig);
    }

    @Override
    void musl_suspend() {
        host.suspend();
    }

    @Override
    void host_exit(int r) {
        host.exit(r);
    }

    @Override
    void host_message(BytePtr msg, int len, int err) {
        int n = len < 0 ? BytePtr.strlen(msg) : len;
        host.message(msg.a, msg.i, n, err != 0);
    }

    @Override
    int host_write(BytePtr s, int len) {
        if (len < 0) {
            return -1;
        }
        if (len == 0) {
            return 0;
        }
        return host.write(s.a, s.i, len);
    }

    // The C host allocates from a static 1 GiB arena and never frees; the
    // garbage collector is the allocator here, but the arena's accounting
    // (and its exhaustion message) are kept.  They are the core's, not a
    // Host's.
    static final long HOST_ARENA_BYTES = 1024L * 1024 * 1024;
    private long hostArenaUsed;

    private void hostArenaExhausted(long n) {
        String m = "whim-vim: host arena exhausted: " + Long.toUnsignedString(HOST_ARENA_BYTES) + " bytes, "
                + Long.toUnsignedString(hostArenaUsed) + " used, request " + Long.toUnsignedString(n) + "\n";
        BytePtr b = BytePtr.alloc(m.length() + 1);
        for (int i = 0; i < m.length(); i++) {
            b.set(i, (byte) m.charAt(i));
        }
        host_message(b, m.length(), TRUE);
        host_exit(1);
    }

    /** n zeroed bytes, a BytePtr as Object: the storage a C allocation is. */
    @Override
    Object host_alloc(long n) {
        long want = (n + 15) & ~15L; // alignof(max_align_t) is 16
        if (Long.compareUnsigned(want, n) < 0 || Long.compareUnsigned(want, HOST_ARENA_BYTES - hostArenaUsed) > 0) {
            hostArenaExhausted(n);
        }
        hostArenaUsed += want;
        return BytePtr.alloc(n);
    }

    @Override
    int vim_snprintf(BytePtr str, long str_m, BytePtr fmt, Object... args) {
        return printf.snprintf(str, str_m, fmt, args);
    }

    // --- what vim_snprintf needs of the core

    @Override
    public BytePtr gettext(BytePtr msg) {
        return gettext_(msg);
    }

    @Override
    public void error(BytePtr msg) {
        emsg(msg);
    }

    @Override
    public void internalError(BytePtr msg) {
        iemsg(msg);
    }

    @Override
    public BytePtr iobuff() {
        return IObuff;
    }

    @Override
    public long emsgIobuffRoom() {
        return emsg_iobuff_room();
    }

    @Override
    public BytePtr iobuffOr(BytePtr s) {
        return iobuff_or(s);
    }

    @Override
    public BytePtr eValTooLarge() {
        return new BytePtr(e_val_too_large, 0);
    }

    @Override
    public int utfcPtr2len(BytePtr p) {
        return utfc_ptr2len(p);
    }

    @Override
    public int utfPtr2cells(BytePtr p) {
        return utf_ptr2cells(p);
    }
}
