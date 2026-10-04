// The terminal host: a Host on the process's own file descriptors 0, 1 and
// 2 -- the C++ of the half of whim-vim.c's host that asks the operating
// system for something, from host_winch_pending to host_write, function by
// function, as whimsy's term.rs ports them.  It is the Host bin/whimpp runs
// the editor with.
//
// Nothing stands between the editor and the signals: the handlers are real,
// installed with sigaction(2) as the C installs them (no SA_RESTART, so a
// caught signal interrupts select(2), read(2) and nanosleep(2) with EINTR,
// as in the C), and the libc calls are the C's own.  The handlers store to
// atomics (lock-free, so async-signal-safe), and the SIGHUP/SIGTERM one
// writes one byte to the non-blocking, close-on-exec pipe, exactly as the
// C's do.  Signal dispositions are the process's, so the pending flags and
// the pipe are the file's, as they are in the C: one process has one
// terminal.  What is the instance's -- the saved terminal modes, raw or
// not, the clock's base, the core's deathtrap -- is the Term's.
//
// Deviations from the C are marked DEVIATION where they happen.

#include "term.hpp"

#include <atomic>
#include <cerrno>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <ctime>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <sys/select.h>
#include <sys/time.h>
#include <termios.h>
#include <unistd.h>

namespace whimpp {

namespace {

// What the C keeps at file scope and its handlers touch.
std::atomic<bool> winch_pending{false};
std::atomic<bool> tstp_pending{false};
std::atomic<bool> int_pending{false};
std::atomic<int> death_pending{0};
std::atomic<int> death_pipe[2] = {-1, -1};

// host_catch: sig handled by f, the mask empty, no flags.
void host_catch(int sig, void (*f)(int))
{
    struct sigaction sa = {};
    sa.sa_handler = f;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    sigaction(sig, &sa, nullptr);
}

extern "C" void on_winch(int)
{
    winch_pending = true;
}

extern "C" void on_tstp(int)
{
    tstp_pending = true;
}

extern "C" void on_int(int)
{
    int_pending = true;
}

// SIGHUP and SIGTERM: the signal recorded and a byte down the pipe, errno
// kept as it was.
extern "C" void on_death(int sig)
{
    const int e = errno;
    death_pending = sig;
    if (const int w = death_pipe[1]; w >= 0)
    {
        [[maybe_unused]] auto r = ::write(w, "", 1);
    }
    errno = e;
}

class Term final : public Host
{
public:
    void init(std::function<void(int)> deathtrap) override
    {
        deathtrap_ = std::move(deathtrap);
        int fds[2] = {-1, -1};
        if (pipe2(fds, O_NONBLOCK | O_CLOEXEC) != 0)
        {
            fds[0] = fds[1] = -1;
        }
        for (int i = 0; i < 2; ++i)
        {
            // DEVIATION: a second init's pipe replaces the first, which is
            // closed; the C makes one and would leak the first.
            if (const int old = death_pipe[i].exchange(fds[i]); old >= 0)
            {
                close(old);
            }
        }
        host_catch(SIGHUP, on_death);
        host_catch(SIGTERM, on_death);
        host_catch(SIGWINCH, on_winch);
        host_catch(SIGCONT, on_winch);
        host_catch(SIGTSTP, on_tstp);
        host_catch(SIGINT, on_int);
        host_catch(SIGPIPE, SIG_IGN);
        host_catch(SIGALRM, SIG_IGN);
    }

    // musl_get_winsize: TIOCGWINSZ on fd 1.
    std::optional<WinSize> win_size() override
    {
        struct winsize ws = {};
        if (ioctl(1, TIOCGWINSZ, &ws) != 0 || ws.ws_row == 0 || ws.ws_col == 0)
        {
            return std::nullopt;
        }
        return WinSize{ws.ws_row, ws.ws_col};
    }

    void term_start() override
    {
        tty_raw_ = true;
        tty_set(true, false);
    }

    void term_stop() override
    {
        tty_raw_ = false;
        tty_set(false, false);
    }

    // musl_tty_keys: fd's erase and interrupt characters, ICRNL and ONLCR.
    std::optional<TtyKeys> tty_keys(int fd) override
    {
        struct termios keys = {};
        if (tcgetattr(fd, &keys) == -1)
        {
            return std::nullopt;
        }
        return TtyKeys{keys.c_cc[VERASE], keys.c_cc[VINTR], (keys.c_iflag & ICRNL) != 0, (keys.c_oflag & ONLCR) != 0};
    }

    // musl_now_ms: gettimeofday's milliseconds from the first call's second.
    long now_ms() override
    {
        struct timeval tv = {};
        gettimeofday(&tv, nullptr);
        if (!now_based_)
        {
            now_based_ = true;
            now_base_ = tv.tv_sec;
        }
        return (tv.tv_sec - now_base_) * 1000L + tv.tv_usec / 1000L;
    }

    // host_time: WHIM_TIME when set and not empty (a clock held still,
    // phase 99), read by atol; time(2) otherwise.
    long time() override
    {
        if (const char *pinned = std::getenv("WHIM_TIME"); pinned != nullptr && *pinned != '\0')
        {
            return std::atol(pinned);
        }
        return std::time(nullptr);
    }

    // musl_delay: nanosleep, a raw terminal relaxed for a long
    // interruptible one.
    void delay(long ms, bool interruptible) override
    {
        const bool relax = interruptible && tty_raw_ && ms > 500;
        if (relax)
        {
            tty_set(false, true);
        }
        struct timespec ts = {};
        ts.tv_sec = ms / 1000;
        ts.tv_nsec = (ms % 1000) * 1000000;
        nanosleep(&ts, nullptr);
        if (relax)
        {
            tty_set(true, false);
        }
    }

    // musl_wait_for_input: select on fd 0 and the pipe, for ms or for ever;
    // a pending SIGHUP/SIGTERM delivered first, a pending SIGWINCH, SIGTSTP
    // or SIGINT an answer.  Linux's select leaves the time remaining in tv,
    // so a wait resumed after EINTR waits only what is left, as the C's.
    bool wait_for_input(long ms) override
    {
        struct timeval tv = {};
        struct timeval *tvp = nullptr;
        if (ms >= 0)
        {
            tv.tv_sec = ms / 1000;
            tv.tv_usec = (ms % 1000) * 1000;
            tvp = &tv;
        }
        for (;;)
        {
            deliver_death();
            if (winch_pending || tstp_pending || int_pending)
            {
                return true;
            }
            const int p = death_pipe[0];
            fd_set rfds;
            FD_ZERO(&rfds);
            FD_SET(0, &rfds);
            if (p >= 0)
            {
                FD_SET(p, &rfds);
            }
            const int ret = select(p >= 0 ? p + 1 : 1, &rfds, nullptr, nullptr, tvp);
            if (ret == -1 && errno == EINTR)
            {
                continue;
            }
            if (ret > 0 && p >= 0 && FD_ISSET(p, &rfds))
            {
                continue;
            }
            return ret > 0 && FD_ISSET(0, &rfds);
        }
    }

    // musl_read_input: the signals the core reads as input first -- SIGINT
    // as Ctrl-C, SIGWINCH as the size report, SIGTSTP as the suspend key --
    // then read(0), which a caught signal interrupts (-1, EINTR) as in the C.
    int read_input(std::span<char> buf) override
    {
        deliver_death();
        const std::size_t len = buf.size();
        if (int_pending)
        {
            int_pending = false;
            if (len >= 1)
            {
                buf[0] = 3;
                return 1;
            }
        }
        if (winch_pending)
        {
            winch_pending = false;
            if (auto ws = win_size(); ws && len >= 32)
            {
                // DEVIATION: snprintf where the C calls vim_snprintf; the
                // report (at most 22 bytes) fits, so both write it whole.
                return std::snprintf(buf.data(), len, "\033[48;%d;%d;0;0t", ws->rows, ws->cols);
            }
        }
        if (tstp_pending)
        {
            tstp_pending = false;
            if (len >= 5)
            {
                std::string_view s = "\033[?1z";
                std::copy(s.begin(), s.end(), buf.begin());
                return 5;
            }
        }
        return static_cast<int>(::read(0, buf.data(), len));
    }

    // host_raise: kill(getpid(), sig) -- the installed handler runs.
    void raise(int sig) override
    {
        kill(getpid(), sig);
    }

    // musl_suspend: SIGTSTP at its default action to the process group,
    // the handler back after.
    void suspend() override
    {
        host_catch(SIGTSTP, SIG_DFL);
        kill(0, SIGTSTP);
        host_catch(SIGTSTP, on_tstp);
    }

    // host_message: all of msg to fd 2 when err, else fd 1, until a write
    // writes nothing or fails.
    void message(std::string_view msg, bool err) override
    {
        const int fd = err ? 2 : 1;
        std::size_t off = 0;
        while (off < msg.size())
        {
            const ssize_t w = ::write(fd, msg.data() + off, msg.size() - off);
            if (w <= 0)
            {
                return;
            }
            off += static_cast<std::size_t>(w);
        }
    }

    // host_write: one write(1).
    int write(std::string_view p) override
    {
        return static_cast<int>(::write(1, p.data(), p.size()));
    }

private:
    // host_deliver_death: the pipe drained, then a pending SIGHUP or
    // SIGTERM handed to the core's deathtrap, here on the editor's stack.
    void deliver_death()
    {
        if (const int r = death_pipe[0]; r >= 0)
        {
            char b[16];
            while (::read(r, b, sizeof b) > 0)
            {
            }
        }
        if (const int sig = death_pending.exchange(0); sig != 0 && deathtrap_)
        {
            // copied first: deathtrap may enter the host again, and does not
            // return when it ends the editor
            auto f = deathtrap_;
            f(sig);
        }
    }

    // host_tty_set: fd 0's saved modes, made raw, or relaxed for a sleep
    // (no canonical input, no echo), or as saved.
    void tty_set(bool raw, bool sleep)
    {
        int n = 10;
        if (!tty_valid_)
        {
            if (tcgetattr(0, &tty_saved_) == -1)
            {
                return;
            }
            tty_valid_ = true;
        }
        struct termios tnew = tty_saved_;
        if (raw)
        {
            tnew.c_iflag &= ~static_cast<tcflag_t>(ICRNL | IXON);
            tnew.c_lflag &= ~static_cast<tcflag_t>(ICANON | ECHO | ISIG | ECHOE | IEXTEN);
            tnew.c_oflag &= ~static_cast<tcflag_t>(ONLCR | XTABS);
            tnew.c_cc[VMIN] = 1;
            tnew.c_cc[VTIME] = 0;
        }
        else if (sleep)
        {
            tnew.c_lflag &= ~static_cast<tcflag_t>(ICANON | ECHO);
            tnew.c_cc[VMIN] = 1;
            tnew.c_cc[VTIME] = 0;
        }
        while (tcsetattr(0, TCSANOW, &tnew) == -1 && errno == EINTR && n > 0)
        {
            --n;
        }
    }

    struct termios tty_saved_ = {};
    bool tty_valid_ = false;
    bool tty_raw_ = false;
    long now_base_ = 0;
    bool now_based_ = false;
    std::function<void(int)> deathtrap_;
};

} // namespace

std::unique_ptr<Host> new_term()
{
    return std::make_unique<Term>();
}

} // namespace whimpp
