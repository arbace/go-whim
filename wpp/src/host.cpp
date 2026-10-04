// The glue: the host functions the core calls, members of the editor in the
// C's signatures, each a line to the editor's Host (host.hpp); the arena;
// and run.

#include "editor.hpp"

#include "host.hpp"

#include <atomic>
#include <cstddef>
#include <cstdlib>
#include <cstring>
#include <new>
#include <string>

namespace whimpp {

namespace {

// The C host's arena: 1 GiB, zeroed, never freed while the editor runs --
// calloc'd, so its pages are the kernel's zeros until touched and an editor
// that uses little costs little.
constexpr std::size_t arena_bytes = std::size_t{1} << 30;

struct Free
{
    void operator()(void *p) const { std::free(p); }
};

// host_exit: the editor ends with this status, which run catches -- the
// C's longjmp back to main.
struct HostExit
{
    int code;
};

} // namespace

// What an editor's glue_ points at: its Host, and its arena.
struct Glue
{
    std::unique_ptr<Host> host;
    std::unique_ptr<char, Free> arena;
    // counted atomically: the regex engine's chunks allocate at once
    std::atomic<std::size_t> used{0};
};

int run(std::unique_ptr<Host> host, const std::vector<std::string> &args)
{
    std::vector<std::string> strs(args);
    std::vector<char *> argv;
    for (auto &s : strs)
    {
        argv.push_back(s.data());
    }
    argv.push_back(nullptr);
    Glue glue{std::move(host), std::unique_ptr<char, Free>(static_cast<char *>(std::calloc(arena_bytes, 1))), {}};
    if (!glue.arena)
    {
        throw std::bad_alloc();
    }
    auto ed = std::make_unique<Editor>();
    ed->glue_ = &glue;
    try
    {
        return ed->vim_main(static_cast<int>(args.size()), argv.data());
    }
    catch (const HostExit &e)
    {
        return e.code;
    }
}

void Editor::musl_host_init(void)
{
    glue_->host->init([this](int sig) { deathtrap(sig); });
}

int Editor::musl_get_winsize(int *rows, int *cols)
{
    auto ws = glue_->host->win_size();
    if (!ws)
    {
        return FAIL;
    }
    *rows = ws->rows;
    *cols = ws->cols;
    return OK;
}

void Editor::musl_term_start(void)
{
    glue_->host->term_start();
}

void Editor::musl_term_stop(void)
{
    glue_->host->term_stop();
}

int Editor::musl_tty_keys(int fd, int *bs, int *intr, int *cr, int *nlcr)
{
    auto k = glue_->host->tty_keys(fd);
    if (!k)
    {
        return FAIL;
    }
    *bs = k->erase;
    *intr = k->intr;
    *cr = k->icrnl;
    *nlcr = k->onlcr;
    return OK;
}

long Editor::musl_now_ms(void)
{
    return glue_->host->now_ms();
}

long Editor::host_time(void)
{
    return glue_->host->time();
}

void Editor::musl_delay(long ms, int interruptible)
{
    glue_->host->delay(ms, interruptible != 0);
}

int Editor::musl_wait_for_input(long ms)
{
    return glue_->host->wait_for_input(ms);
}

// The host is handed no room for a negative length, and the answer is -1
// for it, as read(2) of (size_t)len gives; the host still takes the signals
// it reads as input, as the C does before its read.
int Editor::musl_read_input(char *buf, int len)
{
    const int n = glue_->host->read_input(std::span<char>(buf, static_cast<std::size_t>(len < 0 ? 0 : len)));
    return len < 0 ? -1 : n;
}

void Editor::musl_suspend(void)
{
    glue_->host->suspend();
}

void Editor::host_raise(int sig)
{
    glue_->host->raise(sig);
}

// The editor ends: unwound to run, which returns the code.
void Editor::host_exit(int r)
{
    throw HostExit{r};
}

void Editor::host_message(const char *msg, int len, int err)
{
    const std::size_t n = len < 0 ? std::strlen(msg) : static_cast<std::size_t>(len);
    glue_->host->message(std::string_view(msg, n), err != 0);
}

int Editor::host_write(const char *s, int len)
{
    if (len < 0)
    {
        return -1;
    }
    if (len == 0)
    {
        return 0;
    }
    return glue_->host->write(std::string_view(s, static_cast<std::size_t>(len)));
}

// n zeroed bytes of the editor's arena, aligned as max_align_t is.
void *Editor::host_alloc(usize n)
{
    constexpr std::size_t align = alignof(std::max_align_t);
    const std::size_t want = (n + (align - 1)) & ~(align - 1);
    const std::size_t used = glue_->used.fetch_add(want, std::memory_order_relaxed);
    if (want < n || want > arena_bytes - std::min(used, arena_bytes))
    {
        const std::string m = "whim-vim: host arena exhausted: " + std::to_string(arena_bytes) + " bytes, " +
                              std::to_string(used) + " used, request " + std::to_string(n) + "\n";
        glue_->host->message(m, true);
        host_exit(1);
    }
    return glue_->arena.get() + used;
}

} // namespace whimpp
