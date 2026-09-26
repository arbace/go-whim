import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import whim.host.Printf;
import whim.rt.BytePtr;

/**
 * braaam's vim_snprintf (whim.host.Printf) on the cases format_java_test.go
 * writes to stdin, one per block -- F <hex of the format>, M <size>, then A
 * lines for the arguments and E -- printing for each what the Go test prints
 * for editor/format.go's: the return value, the buffer's bytes in hex, and
 * each message an error made.
 */
public class PrintfProbe {
    static final StringBuilder out = new StringBuilder();

    static final class Core implements Printf.Core {
        final BytePtr iobuff = BytePtr.alloc(1025);

        public BytePtr gettext(BytePtr msg) { return msg; }
        public void error(BytePtr msg) { out.append("emsg ").append(BytePtr.str(msg)).append('\n'); }
        public void internalError(BytePtr msg) { out.append("iemsg ").append(BytePtr.str(msg)).append('\n'); }
        public BytePtr iobuff() { return iobuff; }
        public long emsgIobuffRoom() { return 1025; }
        public BytePtr iobuffOr(BytePtr s) { return iobuff; }
        public BytePtr eValTooLarge() { return BytePtr.lit("E1510: Value too large: %s"); }

        public int utfcPtr2len(BytePtr p) {
            int c = p.get() & 0xff;
            return c < 0x80 ? 1 : c < 0xe0 ? 2 : c < 0xf0 ? 3 : 4;
        }

        public int utfPtr2cells(BytePtr p) {
            return utfcPtr2len(p) == 3 ? 2 : 1; // the test's three-byte characters are wide
        }
    }

    public static void main(String[] a) throws Exception {
        HexFormat hex = HexFormat.of();
        Printf pf = new Printf(new Core());
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.ISO_8859_1));
        byte[] fmt = null;
        int m = 0;
        List<Object> args = new ArrayList<>();
        for (String l; (l = in.readLine()) != null;) {
            String v = l.length() > 2 ? l.substring(2) : "";
            switch (l.charAt(0)) {
                case 'F' -> {
                    byte[] f = hex.parseHex(v);
                    fmt = new byte[f.length + 1];
                    System.arraycopy(f, 0, fmt, 0, f.length);
                    args.clear();
                }
                case 'M' -> m = Integer.parseInt(v);
                case 'A' -> {
                    String[] kv = v.split(" ", 2);
                    String x = kv.length > 1 ? kv[1] : "";
                    switch (kv[0]) {
                        case "i" -> args.add(Integer.parseInt(x));
                        case "u" -> args.add(Integer.parseUnsignedInt(x));
                        case "l" -> args.add(Long.parseLong(x));
                        case "L" -> args.add(Long.parseUnsignedLong(x));
                        case "c" -> args.add((byte) Integer.parseInt(x));
                        case "s" -> {
                            byte[] s = hex.parseHex(x);
                            byte[] z = new byte[s.length + 1];
                            System.arraycopy(s, 0, z, 0, s.length);
                            args.add(new BytePtr(z, 0));
                        }
                        case "n" -> args.add(null);
                        default -> throw new IllegalArgumentException(kv[0]);
                    }
                }
                case 'E' -> {
                    BytePtr buf = BytePtr.alloc(Math.max(m, 1) + 8);
                    java.util.Arrays.fill(buf.a, (byte) '.');
                    int r = pf.snprintf(buf, m, new BytePtr(fmt, 0), args.toArray());
                    out.append(r).append(' ').append(hex.formatHex(buf.a)).append('\n');
                }
                default -> throw new IllegalArgumentException(l);
            }
        }
        System.out.print(out);
    }
}
