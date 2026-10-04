// The host: what the editor core asks of the world it runs in, behind an
// interface.  In whim-vim.c the host is everything from the first
// `#include` to the end of the file, and the core calls it by name --
// host_write, musl_read_input and the rest.  Here the core (editor.hpp's
// class Editor, generated) still calls those names, members of the editor,
// and each is a line of glue (host.cpp) to the Host its editor runs on: the
// C signature on the editor's side, C++'s types on the interface's
// (editor/host.go's Host, whimsy's trait).  What is the editor's and not the
// Host's -- the arena, which the C host keeps as the core's memory -- is the
// glue's, per editor.  So a process holds any number of editors, each on
// its own Host: the terminal (term.hpp) or anything else.
//
// This header names nothing of the generated one: the terminal host and a
// program that runs editors need only it.

#pragma once

#include <functional>
#include <memory>
#include <optional>
#include <span>
#include <string>
#include <string_view>
#include <vector>

namespace whimpp {

// A terminal's size.
struct WinSize
{
    int rows;
    int cols;
};

// A terminal's erase and interrupt characters, and whether it maps CR to NL
// on input and NL to CR NL on output.
struct TtyKeys
{
    int erase;
    int intr;
    bool icrnl;
    bool onlcr;
};

// What the core needs of the world it runs in: a terminal, a clock, input
// with a timeout, the signals, output.  The core may call the host again
// from inside a call of it (a SIGHUP's deathtrap, run where the host
// waits).
class Host
{
public:
    virtual ~Host() = default;
    // Start catching the signals the editor handles; deathtrap is the core's
    // handler for SIGHUP and SIGTERM, which the host calls on the editor's
    // thread, where the C handler would have run.
    virtual void init(std::function<void(int)> deathtrap) = 0;
    // The terminal's rows and columns, when it has them.
    virtual std::optional<WinSize> win_size() = 0;
    // Raw mode, and out of it.
    virtual void term_start() = 0;
    virtual void term_stop() = 0;
    // fd's keys, when fd is a terminal.
    virtual std::optional<TtyKeys> tty_keys(int fd) = 0;
    // Milliseconds of a clock that starts at the first call, and the Unix
    // time.
    virtual long now_ms() = 0;
    virtual long time() = 0;
    // Sleep ms milliseconds; interruptible lets the terminal relax meanwhile.
    virtual void delay(long ms, bool interruptible) = 0;
    // Whether input -- or a signal the core reads as input -- is there
    // within ms milliseconds (for ever when negative).
    virtual bool wait_for_input(long ms) = 0;
    // Up to buf.size() bytes of input: the count, 0 at its end, -1 when a
    // signal came first.  A signal the core reads as input is written as
    // the keys that stand for it.
    virtual int read_input(std::span<char> buf) = 0;
    // Raise sig in this editor's process, and suspend it.
    virtual void raise(int sig) = 0;
    virtual void suspend() = 0;
    // A message outside the screen, to the error stream when err.
    virtual void message(std::string_view msg, bool err) = 0;
    // The screen's output: the count written, or -1.
    virtual int write(std::string_view p) = 0;
};

// Runs an editor on host with the command line args (args[0] the program's
// name) to its end, and returns its exit status: vim_main's, or
// host_exit's.  The editor is freed after; another may run meanwhile, on a
// thread of its own.
int run(std::unique_ptr<Host> host, const std::vector<std::string> &args);

} // namespace whimpp
