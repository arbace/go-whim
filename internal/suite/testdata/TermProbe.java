import java.nio.charset.StandardCharsets;
import whim.host.Host;
import whim.host.Term;

/**
 * The terminal host (braaam/host/Term.java) driven by hand, one line of
 * output per thing it is asked: what java_test.go's TestJavaTermHost runs on a
 * pseudo-terminal and on a file, and compares with what the C host answers.
 */
public class TermProbe {
    static Term t;

    static void say(String s) {
        byte[] b = (s + "\r\n").getBytes(StandardCharsets.ISO_8859_1);
        t.write(b, 0, b.length);
    }

    static String read() {
        byte[] b = new byte[64];
        int n = t.readInput(b, 0, b.length);
        if (n < 0) {
            return "-1";
        }
        StringBuilder s = new StringBuilder();
        for (int i = 0; i < n; i++) {
            int c = b[i] & 0xff;
            s.append(c < 32 ? "^" + (char) (c + 64) : String.valueOf((char) c));
        }
        return n + " " + s;
    }

    static void kill(String sig) throws Exception {
        new ProcessBuilder("kill", "-" + sig, String.valueOf(ProcessHandle.current().pid())).inheritIO().start().waitFor();
        // Delivered, not yet handled: the JVM runs the handler on a thread of
        // its own, soon after.  A sleep ends when it has (the wake-up pipe).
        t.delay(300, false);
    }

    public static void main(String[] a) throws Exception {
        t = new Term();
        int[] died = {0};
        t.init(sig -> died[0] = sig);
        int[] ws = t.winSize();
        say("winsize " + (ws == null ? "none" : ws[0] + "x" + ws[1]));
        Host.TtyKeys k = t.ttyKeys(0);
        say("ttykeys " + (k == null ? "none" : k.erase() + " " + k.intr() + " " + k.icrnl() + " " + k.onlcr()));
        t.termStart();
        k = t.ttyKeys(0);
        say("raw " + (k == null ? "none" : k.icrnl() + " " + k.onlcr()));
        t.termStop();
        say("wait " + t.waitForInput(2000));
        say("read " + read());
        long t0 = t.nowMs();
        boolean w = t.waitForInput(150);
        long dt = t.nowMs() - t0;
        say("idle " + w + " " + (dt >= 140 && dt < 2000));
        t.raise(28); // SIGWINCH, raised: the size report
        say("winch " + t.waitForInput(0) + " " + read());
        kill("WINCH"); // delivered: the handler thread wakes the wait
        say("sigwinch " + t.waitForInput(5000) + " " + read());
        kill("INT");
        say("sigint " + t.waitForInput(5000) + " " + read());
        kill("TSTP");
        say("sigtstp " + t.waitForInput(5000) + " " + read());
        kill("TERM"); // deathtrap, on this thread, at the next wait
        long end = t.nowMs() + 5000;
        while (died[0] == 0 && t.nowMs() < end) {
            t.waitForInput(10);
        }
        say("sigterm " + died[0]);
        t0 = t.nowMs();
        t.delay(120, false);
        dt = t.nowMs() - t0;
        say("delay " + (dt >= 110 && dt < 2000));
        // :suspend -- stopped, where a shell (Run's reap) continues it; on the
        // pseudo-terminal the probe leads its own session, an orphaned group,
        // which the kernel does not stop.  SIGCONT is then read as a resize.
        t.suspend();
        t.delay(300, false);
        boolean cont = t.waitForInput(0);
        say("suspend " + cont + (cont ? " " + read() : ""));
        byte[] m = "to stderr\r\n".getBytes(StandardCharsets.ISO_8859_1);
        t.message(m, 0, m.length, true);
        t.exit(3);
    }
}
