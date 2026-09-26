package whim.host;

import java.util.function.IntConsumer;

/**
 * What the editor core needs of the world it runs in: a terminal, a clock,
 * input with a timeout, the signals, output and an exit -- editor/host.go's
 * Host, in Java's types.  The core calls the C's host functions
 * (musl_read_input, host_write and the rest) and the glue, {@code Whim}, turns
 * each into one of these; a process holds any number of editors, each on its
 * own Host.  {@link Term} is the one on a terminal.
 */
public interface Host {
    /**
     * Start catching the signals the editor handles.  deathtrap is the core's
     * handler for SIGHUP and SIGTERM: the host calls it on the thread running
     * the core, where the C handler would have run.
     */
    void init(IntConsumer deathtrap);

    /** The terminal's size, {rows, cols}, or null when it has none. */
    int[] winSize();

    /** Put the terminal in raw mode, and take it out. */
    void termStart();

    void termStop();

    /** What {@link #ttyKeys} answers. */
    record TtyKeys(int erase, int intr, boolean icrnl, boolean onlcr) {}

    /**
     * The erase and interrupt characters of the terminal on fd, and whether it
     * maps CR to NL on input and NL to CR-NL on output; null when fd is none.
     */
    TtyKeys ttyKeys(int fd);

    /** Milliseconds since the first call. */
    long nowMs();

    /** The Unix time. */
    long time();

    /** Sleep ms milliseconds; interruptible lets the terminal relax during a long one. */
    void delay(long ms, boolean interruptible);

    /**
     * Wait up to ms milliseconds (for ever when negative) and say whether
     * input -- or a signal the editor reads as input -- is there.
     */
    boolean waitForInput(long ms);

    /**
     * Read into buf[off, off+len): the count, 0 at the end of input, -1 when a
     * signal came first.  A signal the editor reads as input is written as the
     * key sequence that stands for it.
     */
    int readInput(byte[] buf, int off, int len);

    /** Send the process sig. */
    void raise(int sig);

    /** Stop the process (SIGTSTP). */
    void suspend();

    /**
     * End the editor with code, and do not return: the terminal host ends the
     * process; a host embedding the editor throws {@link Exit}, which
     * {@code Whim.main} catches and returns.
     */
    void exit(int code);

    /** Write a message, to the error stream when err. */
    void message(byte[] msg, int off, int len, boolean err);

    /** Write the screen's output; the count, or -1. */
    int write(byte[] p, int off, int len);
}
