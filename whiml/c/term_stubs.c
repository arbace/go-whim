/* The terminal host's calls into libc that OCaml's Unix library has not:
   the C host's own (whim-vim.c, from host_winch_pending to musl_suspend),
   function by function, as whimsy's term.rs declares them -- the signal
   handlers, real ones installed with sigaction(2) as the C installs them
   (no SA_RESTART, so a caught signal interrupts select(2) and read(2) with
   EINTR, as in the C), the raw mode (termios: Unix's terminal_io has no
   IEXTEN, ONLCR or XTABS), the window's size (TIOCGWINSZ) and the keys.
   What the handlers touch is the process's, as in the C: one process has
   one terminal.  The rest of the terminal host is OCaml (whiml/term.ml). */

#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdatomic.h>
#include <string.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <unistd.h>

#include <caml/alloc.h>
#include <caml/memory.h>
#include <caml/mlvalues.h>

static atomic_int host_winch_pending, host_tstp_pending, host_int_pending, host_death_pending;
static atomic_int host_death_pipe_w = -1; /* the write end of the C's host_death_pipe */

static void host_on_winch(int sig) { (void)sig; atomic_store(&host_winch_pending, 1); }
static void host_on_tstp(int sig) { (void)sig; atomic_store(&host_tstp_pending, 1); }
static void host_on_int(int sig) { (void)sig; atomic_store(&host_int_pending, 1); }
static void host_on_death(int sig)
{
    int e = errno;
    atomic_store(&host_death_pending, sig);
    int w = atomic_load(&host_death_pipe_w);
    if (w >= 0) {
        ssize_t r = write(w, "", 1);
        (void)r;
    }
    errno = e;
}

static void host_catch(int sig, void (*f)(int))
{
    struct sigaction sa;
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = f;
    sigaction(sig, &sa, NULL);
}

/* musl_host_init's: the handlers, which write to w (the write end of the
   pipe OCaml made, non-blocking and close-on-exec) */
value whiml_host_init(value w)
{
    atomic_store(&host_death_pipe_w, Int_val(w)); /* a second init's replaces the first */
    host_catch(SIGHUP, host_on_death);
    host_catch(SIGTERM, host_on_death);
    host_catch(SIGWINCH, host_on_winch);
    host_catch(SIGCONT, host_on_winch);
    host_catch(SIGTSTP, host_on_tstp);
    host_catch(SIGINT, host_on_int);
    host_catch(SIGPIPE, SIG_IGN);
    host_catch(SIGALRM, SIG_IGN);
    return Val_unit;
}

/* a pending flag, taken: 0 winch, 1 tstp, 2 int, 3 death (the signal); or
   only looked at, when peek */
value whiml_pending(value which, value peek)
{
    atomic_int *f;
    switch (Int_val(which)) {
    case 0: f = &host_winch_pending; break;
    case 1: f = &host_tstp_pending; break;
    case 2: f = &host_int_pending; break;
    default: f = &host_death_pending; break;
    }
    if (Bool_val(peek))
        return Val_int(atomic_load(f));
    return Val_int(atomic_exchange(f, 0));
}

static struct termios tty_saved;
static int tty_valid;

/* host_tty_set */
value whiml_tty_set(value raw, value sleep_)
{
    int n = 10;
    if (!tty_valid) {
        if (tcgetattr(0, &tty_saved) == -1)
            return Val_unit;
        tty_valid = 1;
    }
    struct termios tnew = tty_saved;
    if (Bool_val(raw)) {
        tnew.c_iflag &= ~(ICRNL | IXON);
        tnew.c_lflag &= ~(ICANON | ECHO | ISIG | ECHOE | IEXTEN);
        tnew.c_oflag &= ~(ONLCR | XTABS);
        tnew.c_cc[VMIN] = 1;
        tnew.c_cc[VTIME] = 0;
    } else if (Bool_val(sleep_)) {
        tnew.c_lflag &= ~(ICANON | ECHO);
        tnew.c_cc[VMIN] = 1;
        tnew.c_cc[VTIME] = 0;
    }
    while (tcsetattr(0, TCSANOW, &tnew) == -1 && errno == EINTR && n > 0)
        n--;
    return Val_unit;
}

/* musl_get_winsize's ioctl: rows * 65536 + cols, or -1 */
value whiml_win_size(value unit)
{
    (void)unit;
    struct winsize ws;
    if (ioctl(1, TIOCGWINSZ, &ws) != 0 || ws.ws_row == 0 || ws.ws_col == 0)
        return Val_int(-1);
    return Val_int((long)ws.ws_row * 65536 + ws.ws_col);
}

/* musl_tty_keys's tcgetattr: erase | intr << 8 | icrnl << 16 | onlcr << 17,
   or -1 */
value whiml_tty_keys(value fd)
{
    struct termios keys;
    if (tcgetattr(Int_val(fd), &keys) == -1)
        return Val_int(-1);
    return Val_int(keys.c_cc[VERASE] | keys.c_cc[VINTR] << 8 | ((keys.c_iflag & ICRNL) != 0) << 16 |
                   ((keys.c_oflag & ONLCR) != 0) << 17);
}

/* musl_suspend: SIGTSTP let through to its default, sent, and taken back */
value whiml_suspend(value unit)
{
    (void)unit;
    host_catch(SIGTSTP, SIG_DFL);
    kill(0, SIGTSTP);
    host_catch(SIGTSTP, host_on_tstp);
    return Val_unit;
}
