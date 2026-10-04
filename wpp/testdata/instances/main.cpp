// Editors are instances: four at once in one process, each on a thread and
// a host of its own -- keys from a buffer, the screen into a buffer, a
// terminal of 24 by 80 -- each required to exit 0 with its own text on its
// screen and no other's (editor/host_test.go's TestEditorsAreInstances,
// whimsy's testdata/instances).  wpp_test.go compiles it against the
// objects `make bin/whim++` leaves in lib/wpp.

#include "host.hpp"

#include <algorithm>
#include <cstdio>
#include <format>
#include <iostream>
#include <memory>
#include <string>
#include <thread>
#include <vector>

namespace {

// A host of the test's own: no terminal, no signals, no clock but a still
// one.
class Buffers final : public whimpp::Host
{
public:
    Buffers(std::string keys, std::string &screen) : keys_(std::move(keys)), screen_(screen) {}
    void init(std::function<void(int)>) override {}
    std::optional<whimpp::WinSize> win_size() override { return whimpp::WinSize{24, 80}; }
    void term_start() override {}
    void term_stop() override {}
    std::optional<whimpp::TtyKeys> tty_keys(int) override { return std::nullopt; }
    long now_ms() override { return 0; }
    long time() override { return 1790000000; }
    void delay(long, bool) override {}
    bool wait_for_input(long) override { return !keys_.empty(); }
    int read_input(std::span<char> buf) override
    {
        const std::size_t n = std::min(keys_.size(), buf.size());
        std::copy_n(keys_.begin(), n, buf.begin());
        keys_.erase(0, n);
        return static_cast<int>(n);
    }
    void raise(int) override {}
    void suspend() override {}
    void message(std::string_view msg, bool) override { screen_ += msg; }
    int write(std::string_view p) override
    {
        screen_ += p;
        return static_cast<int>(p.size());
    }

private:
    std::string keys_;
    std::string &screen_;
};

} // namespace

int main()
{
    const std::vector<std::string> words = {"alpha", "bravo", "charlie", "delta"};
    std::vector<std::string> screens(words.size());
    std::vector<int> codes(words.size());
    {
        std::vector<std::jthread> threads;
        for (std::size_t i = 0; i < words.size(); ++i)
        {
            threads.emplace_back([&, i] {
                const std::string &w = words[i];
                // its word typed five hundred times, a :%s over the lines, and :q!
                std::string keys = std::format("i{}\x1byy500p:%s/{}/{}{}/\r:q!\r", w, w, w, w);
                codes[i] = whimpp::run(std::make_unique<Buffers>(keys, screens[i]), {"whim++"});
            });
        }
    }
    bool failed = false;
    for (std::size_t i = 0; i < words.size(); ++i)
    {
        const std::string &w = words[i];
        const bool mine = screens[i].find(w + w) != std::string::npos;
        std::string others;
        for (const auto &o : words)
        {
            if (o != w && screens[i].find(o) != std::string::npos)
            {
                others += " " + o;
            }
        }
        std::cout << w << ": status " << codes[i] << ", " << screens[i].size() << " bytes of screen, its own text "
                  << (mine ? "seen" : "MISSING") << ", others'" << (others.empty() ? " none" : others) << "\n";
        if (codes[i] != 0 || !mine || !others.empty())
        {
            failed = true;
        }
    }
    return failed ? 1 : 0;
}
