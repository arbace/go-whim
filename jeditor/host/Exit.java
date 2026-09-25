package whim.host;

/**
 * What a Host that must not end the process throws in its exit: the
 * editor's main catches it and returns the code -- the Go's editor.Exit panic.
 * No stack trace is taken: it is control flow, not an error.
 */
public final class Exit extends RuntimeException {
    private static final long serialVersionUID = 1L;

    public final int code;

    public Exit(int code) {
        super("exit " + code, null, false, false);
        this.code = code;
    }
}
