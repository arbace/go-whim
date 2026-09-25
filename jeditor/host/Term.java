package whim.host;

import java.lang.foreign.Arena;
import java.lang.foreign.FunctionDescriptor;
import java.lang.foreign.Linker;
import java.lang.foreign.MemoryLayout;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.SymbolLookup;
import java.lang.foreign.ValueLayout;
import java.lang.invoke.MethodHandle;
import java.lang.invoke.VarHandle;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.ArrayDeque;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.IntConsumer;

/**
 * The editor's host on a terminal: the Java port of editor/term/term.go,
 * which is the Go port of the part of whim-vim.c from its first #include to
 * its end that asks the operating system for something.  The system calls are
 * made with the Foreign Function &amp; Memory API -- ioctl (TCGETS, TCSETS,
 * TIOCGWINSZ), select, read, write, pipe2, kill -- on the C library the JVM
 * runs on, as the Go makes them with package syscall.
 *
 * <p>Deviations from the C are marked DEVIATION where they happen; they are
 * the Go's, for the same reason -- Java cannot run a handler asynchronously on
 * the thread running the core:
 * <ul>
 * <li>The C handlers for SIGWINCH/SIGCONT, SIGTSTP and SIGINT only set a flag.
 *     Here the JVM runs {@code sun.misc.Signal}'s handler on a thread of its
 *     own, which sets the same flag and writes one byte to a wake-up pipe so a
 *     select in the host returns, as the C select/read/nanosleep returns with
 *     EINTR.  The JVM installs its handlers with SA_RESTART, so no call of ours
 *     sees EINTR from them.
 * <li>SIGHUP and SIGTERM run the core's deathtrap() in the C, at whatever point
 *     the core is.  Here the handler thread queues the signal and deathtrap()
 *     runs on the core's thread at the next point the host is entered to wait,
 *     read or sleep.
 * <li>raise() of a signal the host catches runs that handler's effect directly.
 * </ul>
 */
public final class Term implements Host {
    // --- the C library ---------------------------------------------------

    private static final Linker LINKER = Linker.nativeLinker();
    private static final SymbolLookup LIBC = LINKER.defaultLookup();
    private static final MemoryLayout ERRNO_STATE = Linker.Option.captureStateLayout();
    private static final VarHandle ERRNO =
            ERRNO_STATE.varHandle(MemoryLayout.PathElement.groupElement("errno"));
    private static final Linker.Option CAPTURE = Linker.Option.captureCallState("errno");

    private static MethodHandle fn(String name, FunctionDescriptor d, Linker.Option... o) {
        return LINKER.downcallHandle(LIBC.find(name).orElseThrow(() -> new UnsatisfiedLinkError(name)), d, o);
    }

    private static final ValueLayout.OfInt I = ValueLayout.JAVA_INT;
    private static final ValueLayout.OfLong L = ValueLayout.JAVA_LONG;
    private static final ValueLayout A = ValueLayout.ADDRESS;

    private static final MethodHandle IOCTL = fn("ioctl", FunctionDescriptor.of(I, I, L, A),
            Linker.Option.firstVariadicArg(2), CAPTURE);
    private static final MethodHandle SELECT = fn("select", FunctionDescriptor.of(I, I, A, A, A, A), CAPTURE);
    private static final MethodHandle READ = fn("read", FunctionDescriptor.of(L, I, A, L), CAPTURE);
    private static final MethodHandle WRITE = fn("write", FunctionDescriptor.of(L, I, A, L), CAPTURE);
    private static final MethodHandle PIPE2 = fn("pipe2", FunctionDescriptor.of(I, A, I));
    private static final MethodHandle KILL = fn("kill", FunctionDescriptor.of(I, I, I));
    private static final MethodHandle GETPID = fn("getpid", FunctionDescriptor.of(I));
    private static final MethodHandle SIGPENDING = fn("sigpending", FunctionDescriptor.of(I, A));

    // Linux x86-64 and arm64
    private static final long TCGETS = 0x5401, TCSETS = 0x5402, TIOCGWINSZ = 0x5413;
    private static final int EINTR = 4;
    private static final int O_NONBLOCK = 0x800, O_CLOEXEC = 0x80000;
    // struct termios as the kernel has it: four flags, c_line, c_cc[19]
    private static final int TERMIOS = 36, IFLAG = 0, OFLAG = 4, LFLAG = 12, CC = 17;
    private static final int VINTR = 0, VERASE = 2, VTIME = 5, VMIN = 6;
    private static final int ICRNL = 0x100, IXON = 0x400;
    private static final int ISIG = 0x1, ICANON = 0x2, ECHO = 0x8, ECHOE = 0x10, IEXTEN = 0x8000;
    private static final int ONLCR = 0x4, XTABS = 0x1800;
    // signal numbers, Linux
    private static final int SIGHUP = 1, SIGINT = 2, SIGPIPE = 13, SIGALRM = 14, SIGTERM = 15,
            SIGCONT = 18, SIGTSTP = 20, SIGWINCH = 28;
    private static final int FD_SET_BYTES = 128;

    /** The memory the calls are made through: the host's own, for the process. */
    private final Arena arena = Arena.ofShared();
    private final MemorySegment errno = arena.allocate(ERRNO_STATE);
    private MemorySegment io = arena.allocate(4096);

    private int errno() {
        return (int) ERRNO.get(errno, 0L);
    }

    private int ioctl(int fd, long req, MemorySegment arg) {
        try {
            int r = (int) IOCTL.invokeExact(errno, fd, req, arg);
            return r == 0 ? 0 : errno();
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    private int select(int nfd, MemorySegment r, MemorySegment tv) {
        try {
            int n = (int) SELECT.invokeExact(errno, nfd, r, MemorySegment.NULL, MemorySegment.NULL, tv);
            return n < 0 ? -errno() : n;
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    /** read(2) into the host's buffer: the count, or minus errno. */
    private long sysRead(int fd, MemorySegment buf, long n) {
        try {
            long r = (long) READ.invokeExact(errno, fd, buf, n);
            return r < 0 ? -errno() : r;
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    private long sysWrite(int fd, MemorySegment buf, long n) {
        try {
            long r = (long) WRITE.invokeExact(errno, fd, buf, n);
            return r < 0 ? -errno() : r;
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    private static int kill(int pid, int sig) {
        try {
            return (int) KILL.invokeExact(pid, sig);
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    private static int getpid() {
        try {
            return (int) GETPID.invokeExact();
        } catch (Throwable t) {
            throw new Error(t);
        }
    }

    private MemorySegment ioBuffer(long n) {
        if (io.byteSize() < n) {
            io = arena.allocate(Math.max(n, io.byteSize() * 2));
        }
        return io;
    }

    // --- the host's state ------------------------------------------------

    private final AtomicBoolean winchPending = new AtomicBoolean();
    private final AtomicBoolean tstpPending = new AtomicBoolean();
    private final AtomicBoolean intPending = new AtomicBoolean();
    private final MemorySegment ttySaved = arena.allocate(TERMIOS);
    private boolean ttyValid;
    private boolean ttyRaw;
    private long nowBase;
    private boolean nowBased;

    // the replacements for asynchronous handlers
    private boolean caught;
    private int wakeR = -1, wakeW = -1;
    private final MemorySegment wakeBuf = arena.allocate(64);
    // what the signal thread writes with, apart from the core thread's
    private final MemorySegment sigErrno = arena.allocate(ERRNO_STATE);
    private final MemorySegment sigByte = arena.allocate(1);
    // the core thread's scratch for the calls, made once
    private final MemorySegment fdset = arena.allocate(FD_SET_BYTES);
    private final MemorySegment tv = arena.allocate(16);
    private final MemorySegment tnew = arena.allocate(TERMIOS);
    private final MemorySegment scratch = arena.allocate(128); // a termios, a winsize or a sigset_t
    private final ArrayDeque<Integer> dying = new ArrayDeque<>();
    private IntConsumer deathtrap = sig -> {};
    private Signals signals;

    public Term() {}

    private void ttySet(boolean raw, boolean sleep) {
        int n = 10;
        if (!ttyValid) {
            if (ioctl(0, TCGETS, ttySaved) != 0) {
                return;
            }
            ttyValid = true;
        }
        tnew.copyFrom(ttySaved);
        if (raw) {
            and(tnew, IFLAG, ~(ICRNL | IXON));
            and(tnew, LFLAG, ~(ICANON | ECHO | ISIG | ECHOE | IEXTEN));
            and(tnew, OFLAG, ~(ONLCR | XTABS));
            tnew.set(ValueLayout.JAVA_BYTE, CC + VMIN, (byte) 1);
            tnew.set(ValueLayout.JAVA_BYTE, CC + VTIME, (byte) 0);
        } else if (sleep) {
            and(tnew, LFLAG, ~(ICANON | ECHO));
            tnew.set(ValueLayout.JAVA_BYTE, CC + VMIN, (byte) 1);
            tnew.set(ValueLayout.JAVA_BYTE, CC + VTIME, (byte) 0);
        }
        for (;;) {
            int e = ioctl(0, TCSETS, tnew); // tcsetattr(0, TCSANOW)
            if (!(e != 0 && e == EINTR && n > 0)) {
                break;
            }
            n--;
        }
    }

    private static void and(MemorySegment t, int off, int mask) {
        t.set(ValueLayout.JAVA_INT_UNALIGNED, off, t.get(ValueLayout.JAVA_INT_UNALIGNED, off) & mask);
    }

    /**
     * What stands for the C handlers: sets the flag a handler sets (or queues
     * SIGHUP/SIGTERM for deathtrap), then wakes a host select that is waiting,
     * as the signal's EINTR would.  Called on the JVM's signal thread.
     */
    void signal(int sig) {
        switch (sig) {
            case SIGWINCH, SIGCONT -> winchPending.set(true);
            case SIGTSTP -> tstpPending.set(true);
            case SIGINT -> intPending.set(true);
            case SIGHUP, SIGTERM -> {
                synchronized (dying) {
                    dying.add(sig);
                }
            }
            default -> {}
        }
        if (wakeW >= 0) {
            synchronized (sigErrno) {
                try {
                    long r = (long) WRITE.invokeExact(sigErrno, wakeW, sigByte, 1L);
                    if (r < 0) {
                        return; // a wake-up lost (the pipe full): the flag is still set
                    }
                } catch (Throwable t) {
                    throw new Error(t);
                }
            }
        }
    }

    /**
     * Empty the wake-up pipe: the flags, not the pipe, say what arrived; the
     * pipe only ends a wait.  Drained before the flags are read, so a signal
     * after the read still wakes the wait that follows.
     */
    private void drain() {
        if (wakeR < 0) {
            return;
        }
        MemorySegment b = wakeBuf.asSlice(0, 32);
        for (;;) {
            long n = sysRead(wakeR, b, 32);
            if (n <= 0) {
                return;
            }
        }
    }

    /** Run deathtrap for each SIGHUP/SIGTERM received, on the core's thread. */
    private void deliver() {
        for (;;) {
            int sig;
            synchronized (dying) {
                if (dying.isEmpty()) {
                    return;
                }
                sig = dying.poll();
            }
            deathtrap.accept(sig);
        }
    }

    private static MemorySegment clear(MemorySegment s) {
        return s.fill((byte) 0);
    }

    private static void fdSet(MemorySegment s, int fd) {
        long off = (fd / 64) * 8L;
        s.set(ValueLayout.JAVA_LONG_UNALIGNED, off, s.get(ValueLayout.JAVA_LONG_UNALIGNED, off) | (1L << (fd % 64)));
    }

    private static boolean fdIsSet(MemorySegment s, int fd) {
        return (s.get(ValueLayout.JAVA_LONG_UNALIGNED, (fd / 64) * 8L) & (1L << (fd % 64))) != 0;
    }

    /** The one timeval, set to ms. */
    private MemorySegment timeval(long ms) {
        tv.set(ValueLayout.JAVA_LONG, 0, ms / 1000);
        tv.set(ValueLayout.JAVA_LONG, 8, (ms % 1000) * 1000);
        return tv;
    }

    @Override
    public void init(IntConsumer deathtrap) {
        this.deathtrap = deathtrap;
        if (signals == null) {
            MemorySegment fds = arena.allocate(8);
            try {
                if ((int) PIPE2.invokeExact(fds, O_NONBLOCK | O_CLOEXEC) == 0) {
                    wakeR = fds.get(ValueLayout.JAVA_INT, 0);
                    wakeW = fds.get(ValueLayout.JAVA_INT, 4);
                }
            } catch (Throwable t) {
                throw new Error(t);
            }
            signals = new Signals(this);
        }
        signals.catchAll();
        caught = true;
    }

    @Override
    public int[] winSize() {
        MemorySegment ws = scratch;
        if (ioctl(1, TIOCGWINSZ, ws) != 0) {
            return null;
        }
        int row = Short.toUnsignedInt(ws.get(ValueLayout.JAVA_SHORT, 0));
        int col = Short.toUnsignedInt(ws.get(ValueLayout.JAVA_SHORT, 2));
        if (row <= 0 || col <= 0) {
            return null;
        }
        return new int[] {row, col};
    }

    @Override
    public void termStart() {
        ttyRaw = true;
        ttySet(true, false);
    }

    @Override
    public void termStop() {
        ttyRaw = false;
        ttySet(false, false);
    }

    @Override
    public TtyKeys ttyKeys(int fd) {
        MemorySegment keys = scratch;
        if (ioctl(fd, TCGETS, keys) != 0) {
            return null;
        }
        return new TtyKeys(
                Byte.toUnsignedInt(keys.get(ValueLayout.JAVA_BYTE, CC + VERASE)),
                Byte.toUnsignedInt(keys.get(ValueLayout.JAVA_BYTE, CC + VINTR)),
                (keys.get(ValueLayout.JAVA_INT_UNALIGNED, IFLAG) & ICRNL) != 0,
                (keys.get(ValueLayout.JAVA_INT_UNALIGNED, OFLAG) & ONLCR) != 0);
    }

    @Override
    public long nowMs() {
        Instant t = Instant.now();
        long sec = t.getEpochSecond();
        long usec = t.getNano() / 1000;
        if (!nowBased) {
            nowBased = true;
            nowBase = sec;
        }
        return (sec - nowBase) * 1000 + usec / 1000;
    }

    @Override
    public long time() {
        return Instant.now().getEpochSecond();
    }

    /**
     * nanosleep(ms): it ends early when a caught signal arrives, as nanosleep
     * does with EINTR.  A negative time does not sleep (nanosleep's EINVAL).
     */
    private void sleep(long ms) {
        if (ms < 0) {
            return;
        }
        if (wakeR < 0) {
            try {
                Thread.sleep(ms);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
            return;
        }
        drain();
        MemorySegment tv = timeval(ms);
        for (;;) {
            MemorySegment r = clear(fdset);
            fdSet(r, wakeR);
            // Linux's select leaves the time remaining in tv, so an EINTR
            // resumes the same sleep.
            if (select(wakeR + 1, r, tv) == -EINTR) {
                continue;
            }
            return;
        }
    }

    @Override
    public void delay(long ms, boolean interruptible) {
        boolean relax = interruptible && ttyRaw && ms > 500;
        deliver();
        if (relax) {
            ttySet(false, true);
        }
        sleep(ms);
        if (relax) {
            ttySet(true, false);
        }
        deliver();
    }

    @Override
    public boolean waitForInput(long ms) {
        MemorySegment tvp = ms >= 0 ? timeval(ms) : MemorySegment.NULL;
        for (;;) {
            drain();
            deliver();
            if (winchPending.get() || tstpPending.get() || intPending.get()) {
                return true;
            }
            MemorySegment rfds = clear(fdset);
            int nfd = 1;
            fdSet(rfds, 0);
            if (wakeR >= 0) {
                fdSet(rfds, wakeR);
                nfd = wakeR + 1;
            }
            int ret = select(nfd, rfds, tvp);
            if (ret == -EINTR) {
                continue;
            }
            if (ret < 0) {
                return false;
            }
            if (ret > 0 && fdIsSet(rfds, 0)) {
                return true;
            }
            if (ret > 0) {
                continue; // only the wake-up pipe: the C select returned EINTR
            }
            return false;
        }
    }

    private static int copy(byte[] buf, int off, int len, String s) {
        byte[] b = s.getBytes(StandardCharsets.ISO_8859_1);
        int n = Math.min(len, b.length);
        System.arraycopy(b, 0, buf, off, n);
        return n;
    }

    @Override
    public int readInput(byte[] buf, int off, int len) {
        drain();
        deliver();
        if (intPending.get()) {
            intPending.set(false);
            if (len >= 1) {
                buf[off] = 3;
                return 1;
            }
        }
        if (winchPending.get()) {
            winchPending.set(false);
            int[] ws = winSize();
            if (ws != null && len >= 32) {
                return copy(buf, off, len, "\033[48;" + ws[0] + ";" + ws[1] + ";0;0t");
            }
        }
        if (tstpPending.get()) {
            tstpPending.set(false);
            if (len >= 5) {
                return copy(buf, off, len, "\033[?1z");
            }
        }
        if (len == 0) {
            return 0;
        }
        // DEVIATION (the Go's): read(0) in the C is interrupted by a caught
        // signal and returns -1 (EINTR).  Here it would not be, so wait first
        // for either input or the wake-up pipe, and return -1 when the pipe won.
        if (wakeR >= 0) {
            for (;;) {
                MemorySegment rfds = clear(fdset);
                fdSet(rfds, 0);
                fdSet(rfds, wakeR);
                int ret = select(wakeR + 1, rfds, MemorySegment.NULL);
                if (ret == -EINTR) {
                    continue;
                }
                if (ret > 0 && !fdIsSet(rfds, 0)) {
                    drain();
                    deliver();
                    return -1;
                }
                break;
            }
        }
        MemorySegment b = ioBuffer(len);
        for (;;) {
            long n = sysRead(0, b, len);
            if (n == -EINTR) {
                continue;
            }
            if (n < 0) {
                return -1;
            }
            MemorySegment.copy(b, ValueLayout.JAVA_BYTE, 0, buf, off, (int) n);
            return (int) n;
        }
    }

    @Override
    public void raise(int sig) {
        // DEVIATION: the C kill(getpid(), sig) runs the installed handler
        // before kill returns; here a caught signal's handler effect runs
        // directly.
        if (caught) {
            switch (sig) {
                case SIGHUP, SIGTERM -> {
                    deathtrap.accept(sig);
                    return;
                }
                case SIGWINCH, SIGCONT -> {
                    winchPending.set(true);
                    return;
                }
                case SIGTSTP -> {
                    tstpPending.set(true);
                    return;
                }
                case SIGINT -> {
                    intPending.set(true);
                    return;
                }
                case SIGPIPE, SIGALRM -> {
                    return;
                }
                default -> {}
            }
        }
        kill(getpid(), sig);
    }

    /**
     * Stop the process group with SIGTSTP at its default action: the handler
     * is set to SIG_DFL for the kill, and ours restored after, as the C's
     * mch_suspend and the Go's rt_sigaction do.
     */
    @Override
    public void suspend() {
        if (signals == null || !signals.defaultTstp()) {
            // DEVIATION (fallback only): stop with SIGSTOP instead
            kill(0, 19);
            return;
        }
        kill(0, SIGTSTP);
        // DEVIATION: the C is one thread, which takes the signal and stops as
        // kill returns.  The JVM is many, and the kernel may hand the signal
        // to another -- the process's first thread -- a moment later: were
        // the handler put back at once, that thread would find it and not
        // stop.  So the handler waits until the signal is no longer pending:
        // taken, and the process stopped and continued (or, in an orphaned
        // process group, discarded by the kernel).  At most a second.
        long end = System.nanoTime() + 1_000_000_000L;
        while (pending(SIGTSTP) && System.nanoTime() < end) {
            Thread.onSpinWait();
        }
        signals.catchTstp();
    }

    /** Whether sig is pending for this thread or the process: sigpending(2). */
    private boolean pending(int sig) {
        MemorySegment set = scratch; // a sigset_t: 64 bits are the kernel's
        try {
            if ((int) SIGPENDING.invokeExact(set) != 0) {
                return false;
            }
        } catch (Throwable t) {
            throw new Error(t);
        }
        return (set.get(ValueLayout.JAVA_LONG_UNALIGNED, 0) & (1L << (sig - 1))) != 0;
    }

    /**
     * The C longjmp back to main, which returns the code: here the process
     * simply exits with it.
     */
    @Override
    public void exit(int code) {
        System.exit(code);
    }

    private void writeAll(int fd, byte[] p, int off, int len) {
        MemorySegment b = ioBuffer(len);
        MemorySegment.copy(p, off, b, ValueLayout.JAVA_BYTE, 0, len);
        long done = 0;
        while (done < len) {
            long w = sysWrite(fd, b.asSlice(done), len - done);
            if (w == -EINTR) {
                continue;
            }
            if (w <= 0) {
                return;
            }
            done += w;
        }
    }

    @Override
    public void message(byte[] msg, int off, int len, boolean err) {
        writeAll(err ? 2 : 1, msg, off, len);
    }

    @Override
    public int write(byte[] p, int off, int len) {
        MemorySegment b = ioBuffer(len);
        MemorySegment.copy(p, off, b, ValueLayout.JAVA_BYTE, 0, len);
        for (;;) {
            long n = sysWrite(1, b, len);
            if (n == -EINTR) {
                continue;
            }
            if (n < 0) {
                return -1;
            }
            return (int) n;
        }
    }
}
