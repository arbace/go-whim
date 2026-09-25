package whim.host;

import sun.misc.Signal;
import sun.misc.SignalHandler;

/**
 * The signals the terminal host catches, through {@code sun.misc.Signal}
 * (module jdk.unsupported, which every JDK has and which needs no flag to be
 * used; javac warns that it is internal API).  The JVM runs a handler on a
 * thread of its own, not on the thread it interrupted -- Java has no
 * asynchronous handler -- so a handler here only hands the signal to
 * {@link Term#signal}, which does what the C handler did or queues it.
 *
 * <p>The JVM refuses a handler for the signals it uses itself (SIGSEGV,
 * SIGBUS, SIGFPE, SIGILL, SIGQUIT and its internal ones); the editor needs
 * none of them.  It lets SIGINT, SIGTERM and SIGHUP be taken from its
 * shutdown hooks unless run with -Xrs, which the launcher does not pass.
 */
final class Signals {
    private static final String[] CAUGHT = {"HUP", "TERM", "WINCH", "CONT", "TSTP", "INT"};
    private static final String[] IGNORED = {"PIPE", "ALRM"};

    private final SignalHandler handler;

    Signals(Term term) {
        this.handler = s -> term.signal(s.getNumber());
    }

    /** Catch the signals the C catches, and ignore the two it ignores. */
    void catchAll() {
        for (String s : CAUGHT) {
            handle(s, handler);
        }
        for (String s : IGNORED) {
            handle(s, SignalHandler.SIG_IGN);
        }
    }

    /** SIGTSTP at its default action, to stop: false when the JVM refuses. */
    boolean defaultTstp() {
        return handle("TSTP", SignalHandler.SIG_DFL);
    }

    void catchTstp() {
        handle("TSTP", handler);
    }

    private static boolean handle(String name, SignalHandler h) {
        try {
            Signal.handle(new Signal(name), h);
            return true;
        } catch (IllegalArgumentException e) {
            return false; // the JVM's own, or unknown here
        }
    }
}
